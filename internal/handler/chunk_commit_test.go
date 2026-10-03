package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/cache"
	"eino_agent/internal/config"
	"eino_agent/internal/database/repository"
)

// editableChunkRepo adds the manual-edit surface (and its revision history) to
// the in-memory chunk table.
type editableChunkRepo struct {
	*statefulChunkRepo
	revisions map[string][]*repository.ChunkRevision
}

func newEditableChunkRepo() *editableChunkRepo {
	return &editableChunkRepo{statefulChunkRepo: newStatefulChunkRepo(), revisions: map[string][]*repository.ChunkRevision{}}
}

func (r *editableChunkRepo) UpdateContent(_ context.Context, id, content string) (*repository.Chunk, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[id]
	if !ok {
		return nil, fmt.Errorf("更新 chunk 失败: no rows")
	}
	r.revisions[id] = append(r.revisions[id], &repository.ChunkRevision{
		ID: fmt.Sprintf("rev-%s-%d", id, len(r.revisions[id])+1), ChunkID: id, Content: row.Content, ContentHash: row.ContentHash, Reason: "manual_edit",
	})
	row.Content, row.ContentLength, row.ContentHash = content, len(content), contentHash(content)
	cp := *row
	return &cp, nil
}

func (r *editableChunkRepo) ListRevisions(_ context.Context, chunkID string) ([]*repository.ChunkRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*repository.ChunkRevision(nil), r.revisions[chunkID]...), nil
}

func (r *editableChunkRepo) RestoreRevision(ctx context.Context, chunkID, revisionID string) (*repository.Chunk, error) {
	r.mu.Lock()
	var content string
	found := false
	for _, rev := range r.revisions[chunkID] {
		if rev.ID == revisionID {
			content, found = rev.Content, true
		}
	}
	r.mu.Unlock()
	if !found {
		return nil, fmt.Errorf("读取 chunk revision 失败: no rows")
	}
	return r.UpdateContent(ctx, chunkID, content)
}

// deletableKnowledgeRepo is a document table where Delete really removes the
// row (the real one soft-deletes, which reads the same: GetByID finds nothing).
type deletableKnowledgeRepo struct {
	*fakeKnowledgeRepo
	deleteErr error // returned once by the next Delete
	deleted   []string
}

func (r *deletableKnowledgeRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleteErr != nil {
		err := r.deleteErr
		r.deleteErr = nil
		return err
	}
	delete(r.items, id)
	r.deleted = append(r.deleted, id)
	return nil
}

type chunkEnv struct {
	h         *Handler
	chunks    *editableChunkRepo
	vectors   *statefulVectorDB
	embedder  *countingEmbedder
	knowledge *deletableKnowledgeRepo
	kbs       *fakeKnowledgeBaseRepo
}

func newChunkEnv(t *testing.T) *chunkEnv {
	t.Helper()
	env := &chunkEnv{
		chunks:   newEditableChunkRepo(),
		vectors:  newStatefulVectorDB(),
		embedder: &countingEmbedder{},
		kbs:      &fakeKnowledgeBaseRepo{items: map[string]*repository.KnowledgeBase{syncKB: {ID: syncKB, TenantID: 1}}},
	}
	env.knowledge = &deletableKnowledgeRepo{fakeKnowledgeRepo: newFakeKnowledgeRepo(map[string]*repository.Knowledge{
		syncDoc: {ID: syncDoc, KnowledgeBaseID: syncKB, SourceType: "file", FileName: syncName},
	})}
	env.h = &Handler{
		cfg:           &config.Config{RAG: config.RAGConfig{ChunkStrategy: "markdown", ChunkSize: 400, ChunkOverlap: 0}},
		embedding:     env.embedder,
		vectorDB:      env.vectors,
		chunkRepo:     env.chunks,
		knowledgeRepo: env.knowledge,
		kbRepo:        env.kbs,
	}
	if _, err := env.h.processPlainTextDocument(context.Background(), syncKB, syncDoc, syncName, []byte(syncTestDoc(numberedSections(6)))); err != nil {
		t.Fatalf("seed import: %v", err)
	}
	env.embedder.reset()
	return env
}

// target returns the chunk the tests edit, taken from the middle of the document.
func (e *chunkEnv) target(t *testing.T) *repository.Chunk {
	t.Helper()
	rows, err := e.chunks.GetByKnowledgeID(context.Background(), syncDoc)
	if err != nil || len(rows) < 3 {
		t.Fatalf("seed chunks: %d err=%v", len(rows), err)
	}
	return rows[len(rows)/2]
}

func (e *chunkEnv) call(handler gin.HandlerFunc, method, chunkID, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/chunk", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: syncKB}, {Key: "docId", Value: syncDoc}, {Key: "chunkId", Value: chunkID}}
	c.Set("tenant_id", 1)
	handler(c)
	return w
}

func (e *chunkEnv) edit(chunkID, content string) *httptest.ResponseRecorder {
	return e.call(e.h.UpdateDocumentChunk, http.MethodPatch, chunkID, fmt.Sprintf(`{"content":%q}`, content))
}

func (e *chunkEnv) reindex(chunkID string) *httptest.ResponseRecorder {
	return e.call(e.h.ReindexEditedChunk, http.MethodPost, chunkID, "")
}

func (e *chunkEnv) vectorContent(chunkID string) (string, bool) {
	e.vectors.mu.Lock()
	defer e.vectors.mu.Unlock()
	d, ok := e.vectors.docs[chunkID]
	if !ok {
		return "", false
	}
	return d.Content, true
}

// holdFirstEmbedding makes the next embedding request wait for release, which
// is what a slow model call looks like to a request that has already read the
// chunk. entered fires when that request is parked.
func (e *chunkEnv) holdFirstEmbedding() (entered <-chan struct{}, release func()) {
	enteredCh, gate := make(chan struct{}), make(chan struct{})
	var first atomic.Bool
	e.embedder.onEmbed = func() {
		if first.CompareAndSwap(false, true) {
			close(enteredCh)
			<-gate
		}
	}
	return enteredCh, func() { close(gate) }
}

func (e *chunkEnv) reindexInBackground(chunkID string) <-chan *httptest.ResponseRecorder {
	out := make(chan *httptest.ResponseRecorder, 1)
	go func() { out <- e.reindex(chunkID) }()
	return out
}

func awaitRecorder(t *testing.T, ch <-chan *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	t.Helper()
	select {
	case w := <-ch:
		return w
	case <-time.After(3 * time.Second):
		t.Fatal("request did not finish")
		return nil
	}
}

func TestStaleChunkReindexCannotOverwriteANewerEdit(t *testing.T) {
	env := newChunkEnv(t)
	target := env.target(t)
	entered, release := env.holdFirstEmbedding()

	slow := env.reindexInBackground(target.ID)
	<-entered // the slow request has read the old text and is waiting on the model

	const edited = "REVISED: the newer text the user saved while the first request was waiting."
	if w := env.edit(target.ID, edited); w.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", w.Code, w.Body.String())
	}
	if w := env.reindex(target.ID); w.Code != http.StatusOK {
		t.Fatalf("second reindex: %d %s", w.Code, w.Body.String())
	}
	if got, _ := env.vectorContent(target.ID); got != edited {
		t.Fatalf("index does not carry the edit: %q", got)
	}

	release()
	w := awaitRecorder(t, slow)
	if got, _ := env.vectorContent(target.ID); got != edited {
		t.Fatalf("a late stale reindex overwrote the newer edit: index has %q", got)
	}
	if w.Code != http.StatusConflict {
		t.Fatalf("stale reindex status = %d, want 409; body=%s", w.Code, w.Body.String())
	}
}

func TestChunkReindexDoesNotResurrectAChunkRemovedMeanwhile(t *testing.T) {
	env := newChunkEnv(t)
	target := env.target(t)
	entered, release := env.holdFirstEmbedding()

	slow := env.reindexInBackground(target.ID)
	<-entered

	// What an incremental sync does to a chunk whose text left the document.
	if err := env.chunks.DeleteByIDs(context.Background(), []string{target.ID}); err != nil {
		t.Fatal(err)
	}
	if err := env.vectors.Delete(context.Background(), []string{target.ID}); err != nil {
		t.Fatal(err)
	}

	release()
	w := awaitRecorder(t, slow)
	if _, ok := env.vectorContent(target.ID); ok {
		t.Fatal("reindex wrote a vector for a chunk that no longer exists")
	}
	if w.Code != http.StatusNotFound && w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 404 or 409; body=%s", w.Code, w.Body.String())
	}
}

func TestChunkReindexOfAnUnchangedChunkStillWorks(t *testing.T) {
	env := newChunkEnv(t)
	target := env.target(t)
	const edited = "REVISED: plain edit followed by an explicit reindex."
	if w := env.edit(target.ID, edited); w.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", w.Code, w.Body.String())
	}
	w := env.reindex(target.ID)
	if w.Code != http.StatusOK {
		t.Fatalf("reindex: %d %s", w.Code, w.Body.String())
	}
	if got, _ := env.vectorContent(target.ID); got != edited {
		t.Fatalf("index content = %q", got)
	}
	d := env.vectors.forKnowledge(syncDoc)[target.ID]
	if d == nil || d.Metadata["chunk_index"] != target.ChunkIndex || d.Metadata["knowledge_base_id"] != syncKB {
		t.Fatalf("vector metadata lost its position or owner: %+v", d)
	}
}

// Editing, restoring a revision and committing a reindex are all changes to one
// document's index, so they take the same per-document lock as a sync.
func TestChunkEditsWaitForTheDocumentLock(t *testing.T) {
	env := newChunkEnv(t)
	target := env.target(t)
	if w := env.edit(target.ID, "REVISED: first edit, so that a revision exists."); w.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", w.Code, w.Body.String())
	}
	revisions, _ := env.chunks.ListRevisions(context.Background(), target.ID)
	if len(revisions) != 1 {
		t.Fatalf("revisions = %d", len(revisions))
	}

	cases := []struct {
		name string
		run  func() *httptest.ResponseRecorder
	}{
		{"edit", func() *httptest.ResponseRecorder { return env.edit(target.ID, "REVISED: edit while locked.") }},
		{"restore", func() *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/restore", nil)
			c.Params = gin.Params{{Key: "id", Value: syncKB}, {Key: "docId", Value: syncDoc}, {Key: "chunkId", Value: target.ID}, {Key: "revisionId", Value: revisions[0].ID}}
			c.Set("tenant_id", 1)
			env.h.RestoreChunkRevision(c)
			return w
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := env.target(t).Content
			unlock, err := env.h.lockDocument(context.Background(), syncDoc)
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() { done <- tc.run() }()
			select {
			case <-done:
				unlock()
				t.Fatal("the change ran while another update held the document lock")
			case <-time.After(150 * time.Millisecond):
			}
			if env.target(t).Content != before {
				unlock()
				t.Fatal("content changed while the document was locked")
			}
			unlock()
			if w := awaitRecorder(t, done); w.Code != http.StatusOK {
				t.Fatalf("after unlock: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestChunkEditHonoursExpectedContentHash(t *testing.T) {
	env := newChunkEnv(t)
	target := env.target(t)

	// Two editors loaded the same text; the second to save must be told.
	first := fmt.Sprintf(`{"content":%q,"expected_content_hash":%q}`, "REVISED: first editor.", target.ContentHash)
	if w := env.call(env.h.UpdateDocumentChunk, http.MethodPatch, target.ID, first); w.Code != http.StatusOK {
		t.Fatalf("first edit: %d %s", w.Code, w.Body.String())
	}
	second := fmt.Sprintf(`{"content":%q,"expected_content_hash":%q}`, "REVISED: second editor, based on the old text.", target.ContentHash)
	w := env.call(env.h.UpdateDocumentChunk, http.MethodPatch, target.ID, second)
	if w.Code != http.StatusConflict {
		t.Fatalf("stale edit status = %d, want 409; body=%s", w.Code, w.Body.String())
	}
	if got := env.target(t).Content; got != "REVISED: first editor." {
		t.Fatalf("stale edit overwrote the first one: %q", got)
	}

	// The hash is optional, so existing clients keep working.
	if w := env.edit(target.ID, "REVISED: client that sends no hash."); w.Code != http.StatusOK {
		t.Fatalf("edit without hash: %d %s", w.Code, w.Body.String())
	}
}

func TestConcurrentChunkEditsLeaveDatabaseAndIndexInAgreement(t *testing.T) {
	env := newChunkEnv(t)
	target := env.target(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			env.edit(target.ID, fmt.Sprintf("REVISED: concurrent edit number %d.", i))
			env.reindex(target.ID)
		}(i)
	}
	wg.Wait()
	final := env.target(t).Content
	// One last explicit reindex is what a user does after the dust settles; the
	// point is that no in-flight request can undo it afterwards.
	if w := env.reindex(target.ID); w.Code != http.StatusOK {
		t.Fatalf("final reindex: %d %s", w.Code, w.Body.String())
	}
	if got, _ := env.vectorContent(target.ID); got != final {
		t.Fatalf("index %q disagrees with the stored chunk %q", got, final)
	}
}

func TestChunkReindexRecordsTheChunksCurrentPosition(t *testing.T) {
	env := newChunkEnv(t)
	target := env.target(t)
	entered, release := env.holdFirstEmbedding()

	slow := env.reindexInBackground(target.ID)
	<-entered
	// A sync inserted text before this chunk while the model call was running.
	moved := target.ChunkIndex + 7
	if err := env.chunks.UpdateChunkIndexes(context.Background(), map[string]int{target.ID: moved}); err != nil {
		t.Fatal(err)
	}
	release()
	if w := awaitRecorder(t, slow); w.Code != http.StatusOK {
		t.Fatalf("reindex: %d %s", w.Code, w.Body.String())
	}
	d := env.vectors.forKnowledge(syncDoc)[target.ID]
	if d == nil || d.Metadata["chunk_index"] != moved {
		t.Fatalf("vector carries a stale position: %+v, want %d", d, moved)
	}
}

func TestChunkReindexCommitWaitsForTheDocumentLock(t *testing.T) {
	env := newChunkEnv(t)
	target := env.target(t)
	const edited = "REVISED: committed only once the lock is free."
	if w := env.edit(target.ID, edited); w.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", w.Code, w.Body.String())
	}
	unlock, err := env.h.lockDocument(context.Background(), syncDoc)
	if err != nil {
		t.Fatal(err)
	}
	done := env.reindexInBackground(target.ID)
	select {
	case <-done:
		unlock()
		t.Fatal("the vector was written while another update held the document lock")
	case <-time.After(150 * time.Millisecond):
	}
	if got, _ := env.vectorContent(target.ID); got == edited {
		unlock()
		t.Fatal("index changed while the document was locked")
	}
	unlock()
	if w := awaitRecorder(t, done); w.Code != http.StatusOK {
		t.Fatalf("after unlock: %d %s", w.Code, w.Body.String())
	}
	if got, _ := env.vectorContent(target.ID); got != edited {
		t.Fatalf("index content = %q", got)
	}
}

func TestChunkReindexRefusesOnceTheDocumentIsDeleted(t *testing.T) {
	env := newChunkEnv(t)
	target := env.target(t)
	const edited = "REVISED: must not reach the index of a deleted document."
	if w := env.edit(target.ID, edited); w.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", w.Code, w.Body.String())
	}
	before, _ := env.vectorContent(target.ID)
	entered, release := env.holdFirstEmbedding()
	slow := env.reindexInBackground(target.ID)
	<-entered
	if err := env.knowledge.Delete(context.Background(), syncDoc); err != nil {
		t.Fatal(err)
	}
	release()
	w := awaitRecorder(t, slow)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
	if got, _ := env.vectorContent(target.ID); got != before {
		t.Fatalf("a deleted document's vector was rewritten: %q", got)
	}
}

type invalidationCounter struct {
	cache.RetrievalCache
	invalidated []string
}

func (c *invalidationCounter) InvalidateKnowledgeBase(_ context.Context, kbID string) error {
	c.invalidated = append(c.invalidated, kbID)
	return nil
}

func TestChunkReindexInvalidatesCachedRetrievals(t *testing.T) {
	env := newChunkEnv(t)
	counter := &invalidationCounter{RetrievalCache: cache.NewNoopRetrievalCache()}
	env.h.retrievalCache = counter
	target := env.target(t)
	if w := env.reindex(target.ID); w.Code != http.StatusOK {
		t.Fatalf("reindex: %d %s", w.Code, w.Body.String())
	}
	if len(counter.invalidated) != 1 || counter.invalidated[0] != syncKB {
		t.Fatalf("cache invalidations = %v, want one for %s", counter.invalidated, syncKB)
	}
}
