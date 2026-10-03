package repository

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"eino_agent/internal/database/postgres"
)

// pgTestDB connects to the database named by EINO_TEST_PG_DSN
// (postgres://user:pass@host:port/dbname?sslmode=disable) and skips when it is
// unset. The target must already be migrated and must not hold data worth
// keeping: a throwaway database on a development server is what this is for.
func pgTestDB(t *testing.T) *postgres.DB {
	t.Helper()
	return pgTestDBWithPool(t, 0)
}

// pgTestDBWithPool is pgTestDB with a chosen pool size (0 keeps the default).
func pgTestDBWithPool(t *testing.T, maxConns int32) *postgres.DB {
	t.Helper()
	raw := os.Getenv("EINO_TEST_PG_DSN")
	if raw == "" {
		t.Skip("EINO_TEST_PG_DSN not set")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("EINO_TEST_PG_DSN: %v", err)
	}
	port := 5432
	if p := u.Port(); p != "" {
		if port, err = strconv.Atoi(p); err != nil {
			t.Fatalf("EINO_TEST_PG_DSN port: %v", err)
		}
	}
	pw, _ := u.User.Password()
	sslmode := u.Query().Get("sslmode")
	if sslmode == "" {
		sslmode = "disable"
	}
	db, err := postgres.New(context.Background(), &postgres.Config{
		Host: u.Hostname(), Port: port, User: u.User.Username(), Password: pw,
		Database: u.Path[1:], SSLMode: sslmode, MaxConns: maxConns,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func TestSessionSummaryRepositoryPostgres(t *testing.T) {
	db := pgTestDB(t)
	ctx := context.Background()
	repo := NewSessionSummaryRepository(db)

	var sessionID string
	if err := db.QueryRow(ctx, `INSERT INTO sessions (tenant_id, title) VALUES (1, 'summary-repo-test') RETURNING id`).Scan(&sessionID); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	t.Cleanup(func() { _ = db.Exec(context.Background(), `DELETE FROM sessions WHERE id = $1`, sessionID) })

	got, err := repo.Get(ctx, sessionID)
	if err != nil || got != nil {
		t.Fatalf("Get before any Upsert = %+v, %v; want nil, nil", got, err)
	}

	at := time.Now().Add(-time.Minute).UTC().Truncate(time.Microsecond)
	first := &SessionSummary{
		SessionID: sessionID, Summary: "第一版摘要", CoveredUntilMessageID: "m-10",
		CoveredUntilAt: at, CoveredCount: 10,
	}
	if err := repo.Upsert(ctx, first); err != nil {
		t.Fatalf("Upsert insert: %v", err)
	}
	got, err = repo.Get(ctx, sessionID)
	if err != nil || got == nil {
		t.Fatalf("Get after insert = %+v, %v", got, err)
	}
	if got.Summary != first.Summary || got.CoveredUntilMessageID != "m-10" || got.CoveredCount != 10 || got.Degraded {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if !got.CoveredUntilAt.Equal(at) {
		t.Fatalf("CoveredUntilAt = %v, want %v", got.CoveredUntilAt, at)
	}

	// The conflict path must replace every column, not only the summary text.
	second := &SessionSummary{
		SessionID: sessionID, Summary: "第二版摘要", CoveredUntilMessageID: "m-24",
		CoveredUntilAt: at.Add(30 * time.Second), CoveredCount: 24, Degraded: true,
	}
	if err := repo.Upsert(ctx, second); err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	got, err = repo.Get(ctx, sessionID)
	if err != nil || got == nil {
		t.Fatalf("Get after update = %+v, %v", got, err)
	}
	if got.Summary != second.Summary || got.CoveredUntilMessageID != "m-24" || got.CoveredCount != 24 || !got.Degraded {
		t.Fatalf("update did not replace all columns: %+v", got)
	}
	var rows int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM session_summaries WHERE session_id = $1`, sessionID).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("rows = %d, %v; want exactly one rolling summary per session", rows, err)
	}

	if err := db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if got, err = repo.Get(ctx, sessionID); err != nil || got != nil {
		t.Fatalf("summary survived its session: %+v, %v", got, err)
	}
}
