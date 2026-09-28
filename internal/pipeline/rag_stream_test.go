package pipeline

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

func collectStream(t *testing.T, p *RAGPipeline, query string) []StreamChunk {
	t.Helper()
	ch, err := p.RunStream(context.Background(), &RAGRequest{Query: query})
	if err != nil {
		t.Fatalf("RunStream error = %v", err)
	}
	var chunks []StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}
	return chunks
}

func chunksOfType(chunks []StreamChunk, typ ChunkType) []StreamChunk {
	var out []StreamChunk
	for _, chunk := range chunks {
		if chunk.Type == typ {
			out = append(out, chunk)
		}
	}
	return out
}

type multiDeltaGenerator struct{ traceTestGenerator }

func (multiDeltaGenerator) GenerateStream(context.Context, string, string) (<-chan string, error) {
	ch := make(chan string, 3)
	ch <- "hello "
	ch <- "stream "
	ch <- "world"
	close(ch)
	return ch, nil
}

func TestRunStreamEmitsSourcesBeforeContent(t *testing.T) {
	p := NewRAGPipeline(&Config{EnableRerank: true, RerankTopK: 2},
		WithRetriever(traceTestRetriever{}),
		WithReranker(traceTestReranker{}),
		WithGenerator(multiDeltaGenerator{}),
	)
	chunks := collectStream(t, p, "question")

	if errs := chunksOfType(chunks, ChunkTypeError); len(errs) > 0 {
		t.Fatalf("unexpected error chunk: %#v", errs[0])
	}
	if last := chunks[len(chunks)-1]; last.Type != ChunkTypeDone {
		t.Fatalf("last chunk = %s, want done", last.Type)
	}

	sources := chunksOfType(chunks, ChunkTypeSource)
	gotIDs := []string{}
	for _, source := range sources {
		gotIDs = append(gotIDs, source.DocID)
	}
	if want := []string{"chunk-c", "chunk-a"}; !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("source ids = %#v, want %#v", gotIDs, want)
	}

	var answer strings.Builder
	firstContent := -1
	lastSource := -1
	for i, chunk := range chunks {
		switch chunk.Type {
		case ChunkTypeContent:
			if firstContent < 0 {
				firstContent = i
			}
			answer.WriteString(chunk.Content)
		case ChunkTypeSource:
			lastSource = i
		}
	}
	if answer.String() != "hello stream world" {
		t.Fatalf("answer = %q", answer.String())
	}
	if firstContent < lastSource {
		t.Fatalf("content chunk at %d arrived before last source at %d", firstContent, lastSource)
	}
}

func TestRunStreamAppliesEvidenceGate(t *testing.T) {
	p := NewRAGPipeline(&Config{RerankTopK: 2},
		WithRetriever(lowQualityRetriever{}),
		WithGenerator(multiDeltaGenerator{}),
	)
	chunks := collectStream(t, p, "这个项目是什么")

	if sources := chunksOfType(chunks, ChunkTypeSource); len(sources) != 0 {
		t.Fatalf("sources = %d, want 0", len(sources))
	}
	content := chunksOfType(chunks, ChunkTypeContent)
	if len(content) != 1 || content[0].Content != insufficientProjectInfoAnswer {
		t.Fatalf("content = %#v, want gate answer", content)
	}
	gateSeen := false
	for _, chunk := range chunksOfType(chunks, ChunkTypeTrace) {
		if chunk.Metadata["stage"] == "retrieval_gate" && chunk.Metadata["status"] == string(evidenceGateInsufficientProjectContext) {
			gateSeen = true
		}
	}
	if !gateSeen {
		t.Fatal("expected retrieval_gate trace chunk")
	}
}

type scoredTestReranker struct{ traceTestReranker }

func (scoredTestReranker) RerankWithScores(context.Context, string, []string) ([]RankedPassage, error) {
	return []RankedPassage{{Index: 1, Score: 0.9}, {Index: 2, Score: 0.4}, {Index: 0, Score: 0.1}}, nil
}

func TestRunUsesRerankScoresForSources(t *testing.T) {
	p := NewRAGPipeline(&Config{EnableRerank: true, RerankTopK: 2},
		WithRetriever(traceTestRetriever{}),
		WithReranker(scoredTestReranker{}),
		WithGenerator(traceTestGenerator{}),
	)
	resp, err := p.Run(context.Background(), &RAGRequest{Query: "question"})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if len(resp.Sources) != 2 {
		t.Fatalf("sources = %d, want 2", len(resp.Sources))
	}
	if resp.Sources[0].DocID != "chunk-b" || resp.Sources[0].Score != 0.9 {
		t.Fatalf("first source = %s/%v, want chunk-b/0.9", resp.Sources[0].DocID, resp.Sources[0].Score)
	}
	if resp.Sources[1].Score != 0.4 {
		t.Fatalf("second source score = %v, want 0.4", resp.Sources[1].Score)
	}
	if resp.Trace.Retrieved[1].Score != 0 {
		t.Fatalf("retrieval trace score mutated: %v", resp.Trace.Retrieved[1].Score)
	}
	if resp.Metadata[MetadataKeyGroundingStatus] != GroundingStatusGrounded {
		t.Fatalf("grounding status = %q", resp.Metadata[MetadataKeyGroundingStatus])
	}
}

type emptyRetriever struct{}

func (emptyRetriever) Retrieve(context.Context, string, ...retriever.Option) ([]*schema.Document, error) {
	return nil, nil
}

func TestRunBlocksWhenNothingRetrieved(t *testing.T) {
	p := NewRAGPipeline(&Config{RerankTopK: 2},
		WithRetriever(emptyRetriever{}),
		WithGenerator(traceTestGenerator{}),
	)
	resp, err := p.Run(context.Background(), &RAGRequest{Query: "anything at all"})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if resp.Metadata[MetadataKeyRetrievalGate] != string(evidenceGateNoEvidence) {
		t.Fatalf("gate = %q, want no_evidence", resp.Metadata[MetadataKeyRetrievalGate])
	}
	if resp.Answer == "answer" {
		t.Fatal("generator must not run without evidence")
	}
}

func TestRunRejectsEmptyQuery(t *testing.T) {
	p := NewRAGPipeline(nil, WithRetriever(traceTestRetriever{}), WithGenerator(traceTestGenerator{}))
	if _, err := p.Run(context.Background(), &RAGRequest{Query: "  "}); err == nil {
		t.Fatal("expected error for empty query")
	}
}
