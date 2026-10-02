package repository

import (
	"context"
	"sync"
	"testing"

	"eino_agent/internal/usage"
)

func TestUsageStorePostgres(t *testing.T) {
	db := pgTestDB(t)
	ctx := context.Background()
	reset := func() { _ = db.Exec(ctx, `DELETE FROM llm_usage_totals`) }
	reset()
	t.Cleanup(reset)
	store := NewUsageStore(db)

	got, err := store.Add(ctx, usage.Totals{})
	if err != nil || got != (usage.Totals{}) {
		t.Fatalf("an empty delta on a new database must create the row at zero, got %+v, %v", got, err)
	}
	if got, err = store.Add(ctx, usage.Totals{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}); err != nil || got.TotalTokens != 15 {
		t.Fatalf("first add: %+v, %v", got, err)
	}

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Add(ctx, usage.Totals{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if got, err = store.Add(ctx, usage.Totals{}); err != nil {
		t.Fatal(err)
	}
	if want := (usage.Totals{PromptTokens: 50, CompletionTokens: 25, TotalTokens: 75}); got != want {
		t.Fatalf("concurrent adds lost updates: %+v, want %+v", got, want)
	}
	var rows int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM llm_usage_totals`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("expected a single row, got %d (%v)", rows, err)
	}
}
