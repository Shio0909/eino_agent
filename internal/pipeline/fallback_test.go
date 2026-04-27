package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type fallbackTestRetriever struct {
	docs []*schema.Document
}

func (r fallbackTestRetriever) Retrieve(context.Context, string, ...retriever.Option) ([]*schema.Document, error) {
	return r.docs, nil
}

type fallbackTestProvider struct {
	calls   int
	request ExternalSearchRequest
	results []ExternalSearchResult
}

func (p *fallbackTestProvider) Search(_ context.Context, req ExternalSearchRequest) ([]ExternalSearchResult, error) {
	p.calls++
	p.request = req
	return p.results, nil
}

type captureGenerator struct {
	context string
}

func (g *captureGenerator) Generate(_ context.Context, _ string, context string) (string, error) {
	g.context = context
	return "answer", nil
}

func (g *captureGenerator) GenerateStream(context.Context, string, string) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "answer"
	close(ch)
	return ch, nil
}

func TestFallbackStateMachineUsesKnowledgeWhenConfident(t *testing.T) {
	provider := &fallbackTestProvider{}
	generator := &captureGenerator{}
	pipeline := NewRAGPipeline(&Config{EnableRewrite: false, RerankTopK: 1, Fallback: FallbackConfig{Enabled: true, MinKnowledgeDocs: 1, MinContextChars: 10}},
		WithRetriever(fallbackTestRetriever{docs: []*schema.Document{{ID: "kb-1", Content: "knowledge content that is long enough"}}}),
		WithExternalFallbackProvider(provider),
		WithGenerator(generator),
	)

	resp, err := pipeline.Run(context.Background(), &RAGRequest{Query: "what is this"})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
	if resp.Fallback.AllowExternal {
		t.Fatalf("fallback decision = %#v, want no external", resp.Fallback)
	}
	if !strings.Contains(generator.context, "knowledge content") {
		t.Fatalf("context = %q, want knowledge content", generator.context)
	}
}

func TestFallbackStateMachineUsesExternalWhenKnowledgeEmpty(t *testing.T) {
	provider := &fallbackTestProvider{results: []ExternalSearchResult{
		{Provider: "web", Title: "Result A", URL: "https://example.com/a", Content: "external content A", Score: 1},
		{Provider: "web", Title: "Result B", URL: "https://example.com/b", Content: "external content B", Score: 2},
	}}
	generator := &captureGenerator{}
	pipeline := NewRAGPipeline(&Config{EnableRewrite: false, RerankTopK: 2, Fallback: FallbackConfig{Enabled: true, MinKnowledgeDocs: 1, MaxExternalResults: 5, MaxExternalContext: 1, AllowedProviders: []string{"web"}}},
		WithRetriever(fallbackTestRetriever{}),
		WithExternalFallbackProvider(provider),
		WithGenerator(generator),
	)

	resp, err := pipeline.Run(context.Background(), &RAGRequest{Query: "latest topic"})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if resp.Fallback.Reason != FallbackReasonNoDocuments || resp.Fallback.State != StateGenerate {
		t.Fatalf("fallback decision = %#v, want no_documents -> generate", resp.Fallback)
	}
	if len(resp.Sources) != 1 {
		t.Fatalf("sources len = %d, want 1", len(resp.Sources))
	}
	if resp.Sources[0].Metadata["source_type"] != "external" || resp.Sources[0].Metadata["provider"] != "web" {
		t.Fatalf("source metadata = %#v, want external web", resp.Sources[0].Metadata)
	}
	if !strings.Contains(generator.context, "external content B") {
		t.Fatalf("context = %q, want top-scored external content", generator.context)
	}
}

func TestFallbackStateMachineRefusesWhenNoSources(t *testing.T) {
	provider := &fallbackTestProvider{}
	generator := &captureGenerator{}
	pipeline := NewRAGPipeline(&Config{EnableRewrite: false, RerankTopK: 2, Fallback: FallbackConfig{Enabled: true, RefuseWhenNoSources: true}},
		WithRetriever(fallbackTestRetriever{}),
		WithExternalFallbackProvider(provider),
		WithGenerator(generator),
	)

	resp, err := pipeline.Run(context.Background(), &RAGRequest{Query: "unknown"})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if resp.Fallback.State != StateRefuseOrClarify {
		t.Fatalf("fallback state = %s, want %s", resp.Fallback.State, StateRefuseOrClarify)
	}
	if !strings.Contains(resp.Answer, "未在当前知识库") {
		t.Fatalf("answer = %q, want refusal", resp.Answer)
	}
	if generator.context != "" {
		t.Fatalf("generator context = %q, want generator not called", generator.context)
	}
}
