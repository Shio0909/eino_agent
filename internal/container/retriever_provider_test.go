package container

import (
	"context"
	"testing"
)

func TestRRFFusePrefersHybridHits(t *testing.T) {
	r := &CompositeRetriever{}

	vectorDocs := []*Document{
		{ID: "A", Content: "alpha", Metadata: map[string]interface{}{}},
		{ID: "B", Content: "beta", Metadata: map[string]interface{}{}},
	}
	keywordDocs := []*Document{
		{ID: "B", Content: "beta", Metadata: map[string]interface{}{}},
		{ID: "C", Content: "charlie", Metadata: map[string]interface{}{}},
	}

	fused := r.rrfFuse(context.Background(), vectorDocs, keywordDocs, 3)
	if len(fused) != 3 {
		t.Fatalf("expected 3 docs, got %d", len(fused))
	}

	if fused[0].ID != "B" {
		t.Fatalf("expected first doc to be B, got %s", fused[0].ID)
	}

	if mt, ok := fused[0].Metadata["match_type"].(string); !ok || mt != "hybrid" {
		t.Fatalf("expected B match_type=hybrid, got %#v", fused[0].Metadata["match_type"])
	}
}

func TestRRFFuseRespectsTopK(t *testing.T) {
	r := &CompositeRetriever{}

	vectorDocs := []*Document{
		{ID: "A", Content: "alpha"},
		{ID: "B", Content: "beta"},
		{ID: "C", Content: "charlie"},
	}
	keywordDocs := []*Document{
		{ID: "D", Content: "delta"},
		{ID: "E", Content: "echo"},
	}

	fused := r.rrfFuse(context.Background(), vectorDocs, keywordDocs, 2)
	if len(fused) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(fused))
	}
}
