package pipeline

import (
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type expansionTestRetriever struct {
	queries []string
}

func (r *expansionTestRetriever) Retrieve(_ context.Context, query string, _ ...retriever.Option) ([]*schema.Document, error) {
	r.queries = append(r.queries, query)
	if query == "crimson-log 持久化 恢复 完整性" {
		return []*schema.Document{{ID: "doc-expansion", Content: "crimson-log 通过追加写日志、落盘确认、启动恢复扫描和完整性校验保证可靠性。"}}, nil
	}
	return nil, nil
}

func TestQueryExpansionRetrievesWhenInitialRecallLow(t *testing.T) {
	retriever := &expansionTestRetriever{}
	generator := &captureGenerator{}
	pipeline := NewRAGPipeline(&Config{
		EnableRewrite:        false,
		EnableQueryExpansion: true,
		RerankTopK:           2,
		Fallback:             FallbackConfig{Enabled: true, RefuseWhenNoSources: true},
		QueryExpansion:       QueryExpansionConfig{Enabled: true, MinDocs: 1, MaxQueries: 5, MaxTotalDocs: 5},
	},
		WithRetriever(retriever),
		WithGenerator(generator),
	)

	resp, err := pipeline.Run(context.Background(), &RAGRequest{Query: "什么是 crimson-log 持久化 恢复 完整性"})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}

	if got := len(retriever.queries); got < 2 {
		t.Fatalf("queries = %#v, want initial query plus expansion", retriever.queries)
	}
	if want := []string{"crimson-log 持久化 恢复 完整性"}; !reflect.DeepEqual(resp.Trace.ExpansionQueries, want) {
		t.Fatalf("expansion queries = %#v, want %#v", resp.Trace.ExpansionQueries, want)
	}
	if len(resp.Sources) != 1 || resp.Sources[0].DocID != "doc-expansion" {
		t.Fatalf("sources = %#v, want expansion source", resp.Sources)
	}
	if resp.Answer == "" || generator.context == "" {
		t.Fatalf("answer/context should be generated from expansion result")
	}
}

func TestBuildQueryExpansionsKeepsCodeTerms(t *testing.T) {
	got := buildQueryExpansions("怎么配置 gpt-4o-mini max_tokens?", "怎么配置 gpt-4o-mini max_tokens?", 5)
	if len(got) == 0 || got[0] != "gpt-4o-mini max_tokens" {
		t.Fatalf("expansions = %#v, want code-like terms preserved", got)
	}
}
