package rediscache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/config"
)

func TestRetrievalCacheRoundTripAndInvalidateKnowledgeBase(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run error = %v", err)
	}
	defer mr.Close()

	client, err := NewClient(context.Background(), config.RedisConfig{Addr: mr.Addr()})
	if err != nil {
		t.Fatalf("NewClient error = %v", err)
	}
	defer client.Close()

	store := NewRetrievalCache(client)
	ctx := context.Background()
	vector := []float32{0.1, 0.2, 0.3}
	if err := store.SetEmbedding(ctx, "embed-1", "hash-1", vector, 5*time.Minute); err != nil {
		t.Fatalf("SetEmbedding error = %v", err)
	}

	gotVector, hit, err := store.GetEmbedding(ctx, "embed-1", "hash-1")
	if err != nil {
		t.Fatalf("GetEmbedding error = %v", err)
	}
	if !hit || len(gotVector) != 3 {
		t.Fatalf("unexpected embedding cache result: hit=%v vector=%v", hit, gotVector)
	}

	result := &cachepkg.RetrievalResult{
		DocIDs: []string{"d1"},
		Documents: []cachepkg.RetrievalDocument{{
			ID:      "d1",
			Content: "doc1",
			Score:   0.9,
			Metadata: map[string]any{
				"knowledge_base_id": "kb1",
			},
		}},
		CachedAt: time.Now(),
	}
	if err := store.SetRetrievalResult(ctx, "vector:cache-key", result, 5*time.Minute); err != nil {
		t.Fatalf("SetRetrievalResult error = %v", err)
	}

	gotResult, hit, err := store.GetRetrievalResult(ctx, "vector:cache-key")
	if err != nil {
		t.Fatalf("GetRetrievalResult error = %v", err)
	}
	if !hit || gotResult == nil || len(gotResult.Documents) != 1 || gotResult.Documents[0].ID != "d1" {
		t.Fatalf("unexpected retrieval cache result: hit=%v result=%#v", hit, gotResult)
	}

	if err := store.InvalidateKnowledgeBase(ctx, "kb1"); err != nil {
		t.Fatalf("InvalidateKnowledgeBase error = %v", err)
	}

	_, hit, err = store.GetRetrievalResult(ctx, "vector:cache-key")
	if err != nil {
		t.Fatalf("GetRetrievalResult after invalidation error = %v", err)
	}
	if hit {
		t.Fatal("expected retrieval cache miss after knowledge base invalidation")
	}
}
