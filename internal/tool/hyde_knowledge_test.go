package tool

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type hydeTestRetriever struct {
	query string
	docs  []*schema.Document
}

func (r *hydeTestRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	r.query = query
	return r.docs, nil
}

type hydeTestModel struct {
	content string
}

func (m hydeTestModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: m.content}, nil
}

func (m hydeTestModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (m hydeTestModel) BindTools(tools []*schema.ToolInfo) error {
	return nil
}

func TestHyDEKnowledgeToolRequiresPlainSearchFirst(t *testing.T) {
	base := NewKnowledgeTool(&hydeTestRetriever{}, 3, 1000, 3000)
	hyde := NewHyDEKnowledgeTool(base, hydeTestModel{content: "假想文档"})

	output, err := hyde.InvokableRun(context.Background(), `{"query":"redis 持久化"}`)
	if err != nil {
		t.Fatalf("InvokableRun error = %v", err)
	}
	if !strings.Contains(output, `"mode":"hyde_skipped"`) {
		t.Fatalf("output = %s, want hyde_skipped", output)
	}
}

func TestHyDEKnowledgeToolStripsThinkTagsBeforeRetrieval(t *testing.T) {
	retriever := &hydeTestRetriever{docs: []*schema.Document{{ID: "doc-1", Content: "真实文档"}}}
	base := NewKnowledgeTool(retriever, 3, 1000, 3000)
	base.searched = true
	hyde := NewHyDEKnowledgeTool(base, hydeTestModel{content: "<think>内部推理</think>Redis AOF 持久化"})

	_, err := hyde.InvokableRun(context.Background(), `{"query":"redis 怎么保证数据不丢"}`)
	if err != nil {
		t.Fatalf("InvokableRun error = %v", err)
	}
	if strings.Contains(retriever.query, "think") || strings.Contains(retriever.query, "内部推理") {
		t.Fatalf("retriever query = %q, should strip think tags", retriever.query)
	}
	if retriever.query != "Redis AOF 持久化" {
		t.Fatalf("retriever query = %q", retriever.query)
	}
}

func TestHyDEKnowledgeToolRetrievesWithHypotheticalDocument(t *testing.T) {
	retriever := &hydeTestRetriever{docs: []*schema.Document{{ID: "doc-1", Content: "真实文档", MetaData: map[string]any{"source_filename": "redis.md"}}}}
	base := NewKnowledgeTool(retriever, 3, 1000, 3000)
	base.searched = true
	hyde := NewHyDEKnowledgeTool(base, hydeTestModel{content: "Redis RDB AOF 持久化机制说明"})

	output, err := hyde.InvokableRun(context.Background(), `{"query":"redis 怎么保证数据不丢"}`)
	if err != nil {
		t.Fatalf("InvokableRun error = %v", err)
	}
	if retriever.query != "Redis RDB AOF 持久化机制说明" {
		t.Fatalf("retriever query = %q", retriever.query)
	}
	if !strings.Contains(output, `"mode":"hyde"`) || !strings.Contains(output, "真实文档") {
		t.Fatalf("unexpected output: %s", output)
	}
	if got := len(base.LastDocs()); got != 1 {
		t.Fatalf("last docs len = %d, want 1", got)
	}
}
