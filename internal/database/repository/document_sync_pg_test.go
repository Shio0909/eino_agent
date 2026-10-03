package repository

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"eino_agent/internal/database/postgres"
)

func migratedTestDB(t *testing.T, maxConns int32) *postgres.DB {
	t.Helper()
	db := pgTestDBWithPool(t, maxConns)
	if _, err := db.RunMigrations(context.Background(), "../../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// lockDocumentOrFail returns an unlock that is safe to call twice and also runs
// at cleanup, so a failing test cannot leave a connection held and block the
// pool from closing.
func lockDocumentOrFail(t *testing.T, ctx context.Context, l DocumentLocker, id string) func() {
	t.Helper()
	unlock, err := l.LockDocument(ctx, id)
	if err != nil {
		t.Errorf("LockDocument(%s): %v", id, err)
		return func() {}
	}
	var once sync.Once
	release := func() { once.Do(unlock) }
	t.Cleanup(release)
	return release
}

func TestLockDocumentExcludesOtherSessionsOnTheSameDocument(t *testing.T) {
	ctx := context.Background()
	id := fmt.Sprintf("doc-%d", rand.Int63())
	// Two pools stand for two processes, an API server and an import worker.
	a := NewKnowledgeRepository(migratedTestDB(t, 4)).(DocumentLocker)
	b := NewKnowledgeRepository(migratedTestDB(t, 4)).(DocumentLocker)

	unlockA := lockDocumentOrFail(t, ctx, a, id)

	var acquired atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		unlock := lockDocumentOrFail(t, ctx, b, id)
		acquired.Store(true)
		unlock()
	}()

	time.Sleep(300 * time.Millisecond)
	if acquired.Load() {
		t.Fatal("a second session took the lock while the first still held it")
	}
	unlockA()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the second session never got the lock after the first released it")
	}

	other := lockDocumentOrFail(t, ctx, b, id+"-other")
	other()
}

// Every held lock keeps a pooled connection. With more simultaneous updates
// than the pool has connections, the updates must still be able to run their
// own queries instead of all waiting for a connection that never frees up.
func TestLockDocumentLeavesConnectionsForTheWorkItGuards(t *testing.T) {
	db := migratedTestDB(t, 4)
	locker := NewKnowledgeRepository(db).(DocumentLocker)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			unlock, err := locker.LockDocument(ctx, fmt.Sprintf("pool-%d-%d", rand.Int63(), i))
			if err != nil {
				errs <- err
				return
			}
			defer unlock()
			for range 3 {
				if err := db.Exec(ctx, `SELECT 1`); err != nil {
					errs <- err
					return
				}
			}
			time.Sleep(20 * time.Millisecond)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("update could not finish while others held locks: %v", err)
	}
}

func TestLockDocumentGivesUpWhenTheContextEndsWhileWaiting(t *testing.T) {
	db := migratedTestDB(t, 4)
	locker := NewKnowledgeRepository(db).(DocumentLocker)
	ctx := context.Background()

	held := []func(){}
	for i := 0; i < 2; i++ { // a pool of 4 allows 2 held locks
		held = append(held, lockDocumentOrFail(t, ctx, locker, fmt.Sprintf("slot-%d-%d", rand.Int63(), i)))
	}
	short, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if _, err := locker.LockDocument(short, fmt.Sprintf("slot-%d-late", rand.Int63())); err == nil {
		t.Fatal("expected the waiting caller to give up with its context")
	}
	for _, unlock := range held {
		unlock()
	}
	again := lockDocumentOrFail(t, ctx, locker, "slot-after")
	again()
}

func TestUpdateChunkIndexesChangesOnlyTheIndexPostgres(t *testing.T) {
	db := migratedTestDB(t, 0)
	ctx := context.Background()
	suffix := rand.Int63()

	var tenantID int
	if err := db.QueryRow(ctx, `INSERT INTO tenants (name, api_key) VALUES ($1, $2) RETURNING id`, "t", fmt.Sprintf("k%d", suffix)).Scan(&tenantID); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	kbID, docID := fmt.Sprintf("kb-%d", suffix), fmt.Sprintf("doc-%d", suffix)
	t.Cleanup(func() {
		_ = db.Exec(context.Background(), `DELETE FROM chunks WHERE knowledge_id = $1`, docID)
		_ = db.Exec(context.Background(), `DELETE FROM knowledges WHERE id = $1`, docID)
		_ = db.Exec(context.Background(), `DELETE FROM knowledge_bases WHERE id = $1`, kbID)
		_ = db.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tenantID)
	})
	if err := db.Exec(ctx, `INSERT INTO knowledge_bases (id, tenant_id, name) VALUES ($1, $2, 'kb')`, kbID, tenantID); err != nil {
		t.Fatalf("kb: %v", err)
	}
	if err := db.Exec(ctx, `INSERT INTO knowledges (id, knowledge_base_id, name) VALUES ($1, $2, 'doc')`, docID, kbID); err != nil {
		t.Fatalf("knowledge: %v", err)
	}

	repo := NewChunkRepository(db)
	rows := []*Chunk{}
	for i := 0; i < 4; i++ {
		rows = append(rows, &Chunk{
			ID: fmt.Sprintf("00000000-0000-0000-0000-%012d", suffix%1000000+int64(i)), KnowledgeID: docID, KnowledgeBaseID: kbID,
			ChunkIndex: i, Content: fmt.Sprintf("content %d", i), ContentHash: fmt.Sprintf("h%d", i),
		})
	}
	if err := repo.BatchCreate(ctx, rows); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Text was inserted in front of the first two chunks and the third was removed.
	if err := repo.UpdateChunkIndexes(ctx, map[string]int{rows[0].ID: 2, rows[1].ID: 3, rows[3].ID: 1}); err != nil {
		t.Fatalf("UpdateChunkIndexes: %v", err)
	}
	got, err := repo.GetByKnowledgeID(ctx, docID)
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{rows[2].ID, rows[3].ID, rows[0].ID, rows[1].ID}
	wantIndex := []int{2, 1, 2, 3}
	if len(got) != 4 {
		t.Fatalf("got %d chunks", len(got))
	}
	byID := map[string]*Chunk{}
	for _, c := range got {
		byID[c.ID] = c
	}
	for i, id := range wantOrder {
		c := byID[id]
		if c == nil {
			t.Fatalf("chunk %s missing", id)
		}
		if c.ChunkIndex != wantIndex[i] {
			t.Errorf("chunk %s index = %d, want %d", id, c.ChunkIndex, wantIndex[i])
		}
	}
	for _, r := range rows {
		if byID[r.ID].Content != r.Content || byID[r.ID].ContentHash != r.ContentHash {
			t.Errorf("chunk %s lost content or hash", r.ID)
		}
	}
	if err := repo.UpdateChunkIndexes(ctx, nil); err != nil {
		t.Errorf("empty update: %v", err)
	}
}
