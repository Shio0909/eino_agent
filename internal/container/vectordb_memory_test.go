package container

import (
	"context"
	"testing"
)

func seedMemoryDB(t *testing.T) *MemoryVectorDB {
	t.Helper()
	db := NewMemoryVectorDB(2, "")
	docs := []*Document{
		{ID: "a1", Content: "alpha shared token", Vector: []float32{1, 0}, Metadata: map[string]interface{}{"knowledge_base_id": "kbA", "knowledge_id": "docA"}},
		{ID: "b1", Content: "beta shared token", Vector: []float32{0.9, 0.1}, Metadata: map[string]interface{}{"knowledge_base_id": "kbB", "knowledge_id": "docB"}},
	}
	if err := db.Upsert(context.Background(), docs); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	return db
}

func TestMemorySearchAppliesKnowledgeBaseFilter(t *testing.T) {
	db := seedMemoryDB(t)
	ctx := context.Background()

	got, err := db.Search(ctx, []float32{1, 0}, 10, SearchFilter{KnowledgeBaseIDs: []string{"kbB"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "b1" {
		t.Fatalf("vector search leaked other KB: %+v", got)
	}

	got, err = db.SearchKeyword(ctx, "shared", 10, SearchFilter{KnowledgeBaseIDs: []string{"kbA"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a1" {
		t.Fatalf("keyword search leaked other KB: %+v", got)
	}
}

func TestMemorySearchScopedWithoutIDsReturnsNothing(t *testing.T) {
	db := seedMemoryDB(t)
	got, err := db.Search(context.Background(), []float32{1, 0}, 10, SearchFilter{Scoped: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no results for empty scoped filter, got %d", len(got))
	}
}

func TestMemorySearchKnowledgeIDFilter(t *testing.T) {
	db := seedMemoryDB(t)
	got, err := db.Search(context.Background(), []float32{1, 0}, 10, SearchFilter{KnowledgeIDs: []string{"docB"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "b1" {
		t.Fatalf("unexpected docs: %+v", got)
	}
}

func TestScopeClauseNumbering(t *testing.T) {
	where, args := scopeClause(SearchFilter{KnowledgeBaseIDs: []string{"k"}, KnowledgeIDs: []string{"d"}}, 3)
	want := "knowledge_base_id = ANY($3) AND (knowledge_id = ANY($4) OR id = ANY($4))"
	if where != want {
		t.Fatalf("where = %q, want %q", where, want)
	}
	if len(args) != 2 {
		t.Fatalf("args = %d", len(args))
	}
}

func TestEscapeLike(t *testing.T) {
	if got := escapeLike(`50%_a\b`); got != `50\%\_a\\b` {
		t.Fatalf("escapeLike = %q", got)
	}
}
