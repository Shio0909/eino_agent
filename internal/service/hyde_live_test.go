package service

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/config"
	"eino_agent/internal/container"
)

type liveHyDERetriever struct {
	queries []string
}

func (r *liveHyDERetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	r.queries = append(r.queries, query)
	lower := strings.ToLower(query)
	if strings.Contains(lower, "crimson-log") && (strings.Contains(query, "持久化") || strings.Contains(query, "恢复") || strings.Contains(query, "完整性")) {
		return []*schema.Document{{
			ID:      "crimson-log-reliability-note",
			Content: "crimson-log 的可靠性方案由追加写入日志、落盘确认、启动恢复扫描和记录完整性校验组成。写入路径先追加日志记录，再等待持久化确认；进程重启后根据日志尾部校验结果截断不完整记录，并回放已确认记录恢复状态。",
			MetaData: map[string]any{
				"source_filename":   "crimson-log-reliability.md",
				"knowledge_base_id": "kb-live-hyde",
			},
		}}, nil
	}
	return nil, nil
}

func TestLiveMiniMaxReActHyDE(t *testing.T) {
	if os.Getenv("RUN_LIVE_HYDE") != "1" {
		t.Skip("set RUN_LIVE_HYDE=1 to run live MiniMax HyDE verification")
	}

	cfg, err := config.Load("../../configs/config.yaml")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Agent.EnableKnowledgeTool = true
	cfg.Agent.EnableHyDE = true
	cfg.Agent.EnableWebSearch = false
	cfg.Agent.EnableCodeSearch = false
	cfg.Agent.EnableCodeGraph = false
	cfg.Agent.EnableSkills = false
	cfg.Agent.MaxSteps = 6
	cfg.Agent.SystemPrompt = `你是知识库问答助手。回答前必须先调用 knowledge_search。若普通 knowledge_search 没有结果或结果不足，必须再调用 knowledge_search_hyde。最终回答只能依据工具返回的真实知识库文档。`

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	chatModel, cleanup, err := container.NewLLMProvider(ctx, &cfg.LLM)
	if err != nil {
		t.Fatalf("new llm provider: %v", err)
	}
	if cleanup != nil {
		defer cleanup(ctx)
	}

	retriever := &liveHyDERetriever{}
	svc := &ChatService{config: cfg, chatModel: chatModel, lightModel: chatModel, retriever: retriever}
	agent, kt, err := svc.buildRuntimeAgentForRequest(ctx, retriever, &ChatRequest{Message: "用知识库回答：crimson-log 的可靠性方案是什么？"}, nil)
	if err != nil {
		t.Fatalf("build runtime agent: %v", err)
	}
	if kt == nil {
		t.Fatal("knowledge tool is nil")
	}

	resp, err := agent.Generate(ctx, []*schema.Message{{Role: schema.User, Content: "用知识库回答：crimson-log 的可靠性方案是什么？请先普通检索；如果没有资料，再用 HyDE 重试。"}})
	if err != nil {
		t.Fatalf("agent generate: %v", err)
	}
	if len(retriever.queries) < 2 {
		t.Fatalf("retriever queries = %#v, want ordinary search then HyDE search", retriever.queries)
	}
	if len(kt.LastDocs()) == 0 || kt.LastDocs()[0].ID != "crimson-log-reliability-note" {
		t.Fatalf("last docs = %#v, want HyDE-hit crimson-log-reliability-note", kt.LastDocs())
	}
	answer := ""
	if resp != nil {
		answer = resp.Content
	}
	if !strings.Contains(answer, "追加") && !strings.Contains(answer, "落盘") && !strings.Contains(answer, "恢复") {
		t.Fatalf("answer = %q, want answer grounded in HyDE-hit document; queries=%#v", answer, retriever.queries)
	}
	t.Logf("retriever queries: %#v", retriever.queries)
	t.Logf("answer: %s", answer)
}
