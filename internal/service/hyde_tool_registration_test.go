package service

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/config"
)

type toolRegistryTestModel struct{}

func (m toolRegistryTestModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: "ok"}, nil
}

func (m toolRegistryTestModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (m toolRegistryTestModel) BindTools(tools []*schema.ToolInfo) error {
	return nil
}

func TestBuildToolsRegistersHyDEOnlyWhenEnabled(t *testing.T) {
	svc := &ChatService{
		config:     &config.Config{RAG: config.RAGConfig{TopK: 3}, Agent: config.AgentConfig{EnableKnowledgeTool: true}},
		lightModel: toolRegistryTestModel{},
	}

	tools, _ := svc.buildToolsWithRetriever(fakeRetriever{})
	if hasToolNamed(t, tools, "knowledge_search_hyde") {
		t.Fatal("knowledge_search_hyde should be disabled by default")
	}

	svc.config.Agent.EnableHyDE = true
	tools, _ = svc.buildToolsWithRetriever(fakeRetriever{})
	if !hasToolNamed(t, tools, "knowledge_search_hyde") {
		t.Fatal("knowledge_search_hyde should be registered when enable_hyde is true")
	}
}

func hasToolNamed(t *testing.T, tools []einotool.BaseTool, name string) bool {
	t.Helper()
	for _, candidate := range tools {
		info, err := candidate.Info(context.Background())
		if err != nil {
			t.Fatalf("tool info error = %v", err)
		}
		if info != nil && info.Name == name {
			return true
		}
	}
	return false
}
