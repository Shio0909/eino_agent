// Package container - LLM 提供者实现
//
// 【Eino 特点】使用 Eino 的 ChatModel 接口
package container

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"eino_agent/internal/config"
)

// NewLLMProvider 创建 LLM 提供者
func NewLLMProvider(ctx context.Context, cfg *config.LLMConfig) (model.ChatModel, CleanupFunc, error) {
	switch cfg.Provider {
	case "openai", "azure", "doubao", "deepseek", "moonshot", "qwen":
		return newOpenAICompatibleLLM(ctx, cfg)
	case "ollama":
		return newOllamaLLM(ctx, cfg)
	default:
		return newOpenAICompatibleLLM(ctx, cfg)
	}
}

// newOpenAICompatibleLLM 创建 OpenAI 兼容的 LLM
func newOpenAICompatibleLLM(ctx context.Context, cfg *config.LLMConfig) (model.ChatModel, CleanupFunc, error) {
	// 【Eino 特点】使用 eino-ext 的 OpenAI 组件
	temperature := float32(cfg.Temperature)
	maxTokens := cfg.MaxTokens

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL:     cfg.BaseURL,
		APIKey:      cfg.APIKey,
		Model:       cfg.ModelID,
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("创建 OpenAI ChatModel 失败: %w", err)
	}

	return chatModel, nil, nil
}

// newOllamaLLM 创建 Ollama LLM
func newOllamaLLM(ctx context.Context, cfg *config.LLMConfig) (model.ChatModel, CleanupFunc, error) {
	// Ollama 使用 OpenAI 兼容模式
	ollamaCfg := &config.LLMConfig{
		Provider:    "openai",
		BaseURL:     cfg.BaseURL + "/v1", // Ollama 的 OpenAI 兼容端点
		APIKey:      "ollama",            // Ollama 不需要 API Key
		ModelID:     cfg.ModelID,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	}
	return newOpenAICompatibleLLM(ctx, ollamaCfg)
}
