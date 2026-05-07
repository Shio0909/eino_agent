package service

import (
	"context"
	"slices"
	"strings"
	"testing"

	"eino_agent/internal/config"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type fakeRetriever struct {
	docs []*schema.Document
}

func (r fakeRetriever) Retrieve(context.Context, string, ...retriever.Option) ([]*schema.Document, error) {
	return r.docs, nil
}

func testChatConfig() *config.Config {
	return &config.Config{RAG: config.RAGConfig{TopK: 5}}
}

func TestRestrictedEmptyRetrievalScopeReturnsNoDocuments(t *testing.T) {
	svc := &ChatService{retriever: fakeRetriever{docs: []*schema.Document{
		{ID: "doc-1", Content: "secret", MetaData: map[string]any{"knowledge_base_id": "kb-1"}},
	}}}

	runtimeRetriever := svc.getRuntimeRetriever(&ChatRequest{RestrictRetrieval: true})
	docs, err := runtimeRetriever.Retrieve(context.Background(), "query")
	if err != nil {
		t.Fatalf("Retrieve error = %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("docs len = %d, want 0", len(docs))
	}
}

func TestKnowledgeBaseRetrievalScopeFiltersDocuments(t *testing.T) {
	svc := &ChatService{retriever: fakeRetriever{docs: []*schema.Document{
		{ID: "doc-1", Content: "allowed", MetaData: map[string]any{"knowledge_base_id": "kb-1"}},
		{ID: "doc-2", Content: "denied", MetaData: map[string]any{"knowledge_base_id": "kb-2"}},
	}}}

	runtimeRetriever := svc.getRuntimeRetriever(&ChatRequest{KnowledgeBaseIDs: []string{"kb-1"}, RestrictRetrieval: true})
	docs, err := runtimeRetriever.Retrieve(context.Background(), "query")
	if err != nil {
		t.Fatalf("Retrieve error = %v", err)
	}
	if len(docs) != 1 || docs[0].ID != "doc-1" {
		t.Fatalf("unexpected docs: %#v", docs)
	}
}

func TestAgenticProjectContextRecoveryInstruction(t *testing.T) {
	svc := &ChatService{retriever: fakeRetriever{docs: []*schema.Document{
		{ID: "doc-1", Content: "Instruction Description table schema fields for a generic dataset entry", MetaData: map[string]any{"source": "metadata.md"}},
	}}}

	instruction := svc.agenticProjectContextRecoveryInstruction(context.Background(), &ChatRequest{Message: "请介绍这个项目是什么", UseAgent: true})

	if !strings.Contains(instruction, "只是中间状态") {
		t.Fatalf("instruction should mark agentic gate as intermediate state: %s", instruction)
	}
	if !strings.Contains(instruction, "query_decompose") || !strings.Contains(instruction, "HyDE") {
		t.Fatalf("instruction missing recovery strategies: %s", instruction)
	}
	if !strings.Contains(instruction, "不要直接套用 pipeline 的固定拒答") {
		t.Fatalf("instruction should avoid pipeline terminal refusal: %s", instruction)
	}
	if !isAgenticProjectContextRecoveryActive(instruction) {
		t.Fatalf("recovery marker should be detectable: %s", instruction)
	}
}

func TestAgenticProjectContextRecoveryInstructionSkipsSupportedEvidence(t *testing.T) {
	svc := &ChatService{retriever: fakeRetriever{docs: []*schema.Document{
		{ID: "doc-1", Content: "Eino Agent 是一个 RAG-first 项目，包含 pipeline、agentic、MCP 和 code_search 等模块。"},
	}}}

	instruction := svc.agenticProjectContextRecoveryInstruction(context.Background(), &ChatRequest{Message: "请介绍这个项目是什么", UseAgent: true})

	if instruction != "" {
		t.Fatalf("instruction = %q, want empty for supported evidence", instruction)
	}
}

type modeFakeRetriever struct {
	docsByMode map[string][]*schema.Document
	queries    []string
	modes      []string
}

func (r *modeFakeRetriever) Retrieve(_ context.Context, query string, _ ...retriever.Option) ([]*schema.Document, error) {
	r.queries = append(r.queries, query)
	r.modes = append(r.modes, "auto")
	return r.docsByMode["auto"], nil
}

func (r *modeFakeRetriever) RetrieveWithMode(_ context.Context, query string, mode string) ([]*schema.Document, error) {
	r.queries = append(r.queries, query)
	r.modes = append(r.modes, mode)
	return r.docsByMode[mode], nil
}

func TestAgenticProjectContextRecoveryRunsDeterministicAttempts(t *testing.T) {
	retriever := &modeFakeRetriever{docsByMode: map[string][]*schema.Document{
		"auto":  {{ID: "meta", Content: "Instruction Description table schema fields for a generic dataset entry"}},
		"exact": {{ID: "readme", Content: "Eino Agent 是一个 RAG-first 项目，包含 pipeline、agentic、MCP 和 code_search 等模块。", MetaData: map[string]any{"source": "README.md"}}},
	}}
	svc := &ChatService{config: testChatConfig(), retriever: retriever}
	req := &ChatRequest{Message: "请介绍这个项目是什么", UseAgent: true}
	decision, ok := svc.agenticProjectContextRecoveryDecision(context.Background(), req)
	if !ok {
		t.Fatal("expected recovery decision")
	}
	trace := newTraceCollector("trace-test")
	resp, recovered := svc.runAgenticProjectContextRecovery(context.Background(), req, "s1", "trace-test", trace, decision)
	if !recovered {
		t.Fatal("expected deterministic recovery to find project context")
	}
	if len(resp.Sources) == 0 || !strings.Contains(resp.Answer, "README.md") {
		t.Fatalf("unexpected recovery response: %#v", resp)
	}
	if !slices.Contains(retriever.modes, "exact") {
		t.Fatalf("expected exact recovery attempt, got modes %#v", retriever.modes)
	}
}

func TestSourcesFromDocumentsUsesDefaultLimitForInvalidMax(t *testing.T) {
	docs := []*schema.Document{
		{ID: "doc-1", Content: "one"},
		{ID: "doc-2", Content: "two"},
	}

	sources := sourcesFromDocuments(docs, -1)

	if len(sources) != 2 {
		t.Fatalf("sources len = %d, want 2", len(sources))
	}
	if sources[0].DocID != "doc-1" || sources[1].DocID != "doc-2" {
		t.Fatalf("unexpected sources: %#v", sources)
	}
}
