package container

import (
	"context"
	"testing"

	"eino_agent/internal/config"
)

func TestNewLLMProviderSupportsClaudeProvider(t *testing.T) {
	_, _, err := NewLLMProvider(context.Background(), &config.LLMConfig{
		Provider:  "claude",
		BaseURL:   "https://token-plan-cn.xiaomimimo.com/anthropic",
		APIKey:    "test-key",
		ModelID:   "mimo-v2.5-pro",
		MaxTokens: 1024,
	})
	if err != nil {
		t.Fatalf("NewLLMProvider claude error = %v", err)
	}
}
