package pipeline

import (
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type traceTestRetriever struct{}

func (traceTestRetriever) Retrieve(context.Context, string, ...retriever.Option) ([]*schema.Document, error) {
	return []*schema.Document{
		{ID: "chunk-a", Content: "alpha content", MetaData: map[string]any{"source": "doc-a.md", "match_type": "vector"}},
		{ID: "chunk-b", Content: "beta content", MetaData: map[string]any{"source": "doc-b.md", "match_type": "hybrid"}},
		{ID: "chunk-c", Content: "gamma content", MetaData: map[string]any{"source": "doc-c.md", "match_type": "keyword"}},
	}, nil
}

type traceTestReranker struct{}

func (traceTestReranker) Rerank(context.Context, string, []string) ([]int, error) {
	return []int{2, 0, 1}, nil
}

type traceTestGenerator struct{}

func (traceTestGenerator) Generate(context.Context, string, string) (string, error) {
	return "answer", nil
}

func (traceTestGenerator) GenerateStream(context.Context, string, string) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "answer"
	close(ch)
	return ch, nil
}

func TestRAGPipelineReturnsRetrievalTrace(t *testing.T) {
	p := NewRAGPipeline(&Config{EnableRewrite: false, EnableRerank: true, RerankTopK: 2},
		WithRetriever(traceTestRetriever{}),
		WithReranker(traceTestReranker{}),
		WithGenerator(traceTestGenerator{}),
	)

	resp, err := p.Run(context.Background(), &RAGRequest{Query: "question"})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}

	if len(resp.Trace.Retrieved) != 3 {
		t.Fatalf("retrieved len = %d, want 3", len(resp.Trace.Retrieved))
	}
	if resp.Trace.Retrieved[0].DocID != "chunk-a" || resp.Trace.Retrieved[0].Rank != 1 || resp.Trace.Retrieved[0].MatchType != "vector" {
		t.Fatalf("unexpected first retrieved trace: %#v", resp.Trace.Retrieved[0])
	}

	gotBefore := docIDs(resp.Trace.RerankBefore)
	if want := []string{"chunk-a", "chunk-b", "chunk-c"}; !reflect.DeepEqual(gotBefore, want) {
		t.Fatalf("rerank before = %#v, want %#v", gotBefore, want)
	}

	gotAfter := docIDs(resp.Trace.RerankAfter)
	if want := []string{"chunk-c", "chunk-a", "chunk-b"}; !reflect.DeepEqual(gotAfter, want) {
		t.Fatalf("rerank after = %#v, want %#v", gotAfter, want)
	}

	gotContext := docIDs(resp.Trace.Context)
	if want := []string{"chunk-c", "chunk-a"}; !reflect.DeepEqual(gotContext, want) {
		t.Fatalf("context = %#v, want %#v", gotContext, want)
	}
}

func docIDs(chunks []TraceChunk) []string {
	ids := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		ids = append(ids, chunk.DocID)
	}
	return ids
}
