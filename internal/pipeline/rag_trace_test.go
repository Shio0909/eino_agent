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
		{ID: "chunk-a", Content: "alpha content explains the project architecture with enough factual details for retrieval evidence", MetaData: map[string]any{"source": "doc-a.md", "match_type": "vector"}},
		{ID: "chunk-b", Content: "beta content describes the runtime pipeline and gives enough context for grounded answers", MetaData: map[string]any{"source": "doc-b.md", "match_type": "hybrid"}},
		{ID: "chunk-c", Content: "gamma content documents the code search flow and provides concrete implementation evidence", MetaData: map[string]any{"source": "doc-c.md", "match_type": "keyword"}},
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

type lowQualityRetriever struct{}

func (lowQualityRetriever) Retrieve(context.Context, string, ...retriever.Option) ([]*schema.Document, error) {
	return []*schema.Document{
		{ID: "junk-a", Content: "example", MetaData: map[string]any{"source": "junk-a.md"}},
		{ID: "junk-b", Content: `astro-wu23bvmt"><a href="https://github">provided by this API</a>`, MetaData: map[string]any{"source": "junk-b.md"}},
	}, nil
}

func TestRAGPipelineBlocksLowQualityEvidence(t *testing.T) {
	p := NewRAGPipeline(&Config{EnableRewrite: false, EnableRerank: false, RerankTopK: 2},
		WithRetriever(lowQualityRetriever{}),
		WithGenerator(traceTestGenerator{}),
	)

	resp, err := p.Run(context.Background(), &RAGRequest{Query: "这个项目是什么"})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if len(resp.Sources) != 0 {
		t.Fatalf("sources len = %d, want 0", len(resp.Sources))
	}
	if got := resp.Metadata["retrieval_gate"]; got != string(evidenceGateInsufficientProjectContext) {
		t.Fatalf("retrieval_gate = %q, want %q", got, evidenceGateInsufficientProjectContext)
	}
}

type irrelevantRetriever struct{}

func (irrelevantRetriever) Retrieve(context.Context, string, ...retriever.Option) ([]*schema.Document, error) {
	return []*schema.Document{
		{ID: "doc-a", Content: "This document describes the Eino Agent architecture, retrieval pipeline, code search implementation, and trace observability in detail.", MetaData: map[string]any{"source": "architecture.md"}},
	}, nil
}

func TestRAGPipelineBlocksIrrelevantEvidence(t *testing.T) {
	p := NewRAGPipeline(&Config{EnableRewrite: false, EnableRerank: false, RerankTopK: 1},
		WithRetriever(irrelevantRetriever{}),
		WithGenerator(traceTestGenerator{}),
	)

	resp, err := p.Run(context.Background(), &RAGRequest{Query: "2026 上海 天气 股票 涨跌"})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if len(resp.Sources) != 0 {
		t.Fatalf("sources len = %d, want 0", len(resp.Sources))
	}
	if got := resp.Metadata["retrieval_gate"]; got != string(evidenceGateIrrelevantEvidence) {
		t.Fatalf("retrieval_gate = %q, want %q", got, evidenceGateIrrelevantEvidence)
	}
}

type projectMetadataOnlyRetriever struct{}

func (projectMetadataOnlyRetriever) Retrieve(context.Context, string, ...retriever.Option) ([]*schema.Document, error) {
	return []*schema.Document{
		{ID: "meta-a", Content: "Instruction Description table schema fields for a generic dataset entry with no concrete system content", MetaData: map[string]any{"source": "metadata.md"}},
	}, nil
}

func TestRAGPipelineBlocksProjectOverviewWithoutProjectContext(t *testing.T) {
	p := NewRAGPipeline(&Config{EnableRewrite: false, EnableRerank: false, RerankTopK: 1},
		WithRetriever(projectMetadataOnlyRetriever{}),
		WithGenerator(traceTestGenerator{}),
	)

	resp, err := p.Run(context.Background(), &RAGRequest{Query: "请说明这个项目是什么"})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if len(resp.Sources) != 0 {
		t.Fatalf("sources len = %d, want 0", len(resp.Sources))
	}
	if got := resp.Metadata["retrieval_gate"]; got != string(evidenceGateInsufficientProjectContext) {
		t.Fatalf("retrieval_gate = %q, want %q", got, evidenceGateInsufficientProjectContext)
	}
	if resp.Answer == "answer" {
		t.Fatal("expected fixed state-machine answer, got generator output")
	}
}

func docIDs(chunks []TraceChunk) []string {
	ids := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		ids = append(ids, chunk.DocID)
	}
	return ids
}
