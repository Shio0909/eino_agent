package container

import (
	"context"
	"testing"

	"eino_agent/internal/config"
)

func bm25TestDocs() []*Document {
	return []*Document{
		{ID: "d1", Content: "今天天气很好，我们去公园散步", Metadata: map[string]interface{}{}},
		{ID: "d2", Content: "向量数据库用于存储和检索 embedding 向量", Metadata: map[string]interface{}{}},
		{ID: "d3", Content: "The cat sat on the mat and watched the rain", Metadata: map[string]interface{}{}},
	}
}

func TestLocalRerankerRanksRelevantDocFirst(t *testing.T) {
	r := &LocalReranker{}
	out, err := r.Rerank(context.Background(), "向量检索", bm25TestDocs(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 || out[0].ID != "d2" {
		t.Fatalf("expected d2 first, got %+v", out)
	}
}

func TestLocalRerankerEnglish(t *testing.T) {
	r := &LocalReranker{}
	out, err := r.Rerank(context.Background(), "rain cat", bm25TestDocs(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 || out[0].ID != "d3" {
		t.Fatalf("expected d3 first, got %+v", out)
	}
}

func TestLocalRerankerRespectsTopK(t *testing.T) {
	r := &LocalReranker{}
	out, err := r.Rerank(context.Background(), "向量", bm25TestDocs(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("topK not respected: %d", len(out))
	}
}

func TestRerankEndpointPreservesVersionPrefix(t *testing.T) {
	got, err := rerankEndpoint("https://api.siliconflow.cn/v1", rerankPresets["bge"])
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://api.siliconflow.cn/v1/rerank" {
		t.Fatalf("endpoint = %q", got)
	}
	if got, _ := rerankEndpoint("", rerankPresets["jina"]); got != "https://api.jina.ai/v1/rerank" {
		t.Fatalf("default jina endpoint = %q", got)
	}
	if got, _ := rerankEndpoint("https://example.com/v2/rerank", rerankPresets["cohere"]); got != "https://example.com/v2/rerank" {
		t.Fatalf("explicit path endpoint = %q", got)
	}
}

func TestNewRerankerProviderRejectsUnknown(t *testing.T) {
	cfg := &config.RerankerConfig{Provider: "magic-cloud", ModelID: "m"}
	if _, _, err := NewRerankerProvider(context.Background(), cfg); err == nil {
		t.Fatal("unknown provider must be rejected, not silently routed to jina format")
	}
}
