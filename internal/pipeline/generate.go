// Package pipeline 生成器实现
package pipeline

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// LLMGenerator 基于 Eino ChatModel 的生成器
// 【Eino 特点】直接使用 Eino 的 model.ChatModel 接口
type LLMGenerator struct {
	model        model.ChatModel
	systemPrompt string
	template     prompt.ChatTemplate
}

// NewLLMGenerator 创建 LLM 生成器
func NewLLMGenerator(m model.ChatModel, systemPrompt string) *LLMGenerator {
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}
	tpl := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage("{system_prompt}"),
		schema.UserMessage(`上下文信息：
{rag_context}

用户问题：{query}

请根据上下文信息回答用户问题：`),
	)
	return &LLMGenerator{
		model:        m,
		systemPrompt: systemPrompt,
		template:     tpl,
	}
}

const defaultSystemPrompt = `你是一个专业的知识库问答助手。

## 回答原则
- **积极回答**：基于上下文信息尽可能给出有帮助的回答，允许总结和归纳。
- **禁止编造**具体信息。
- 只有在上下文与问题完全无关时才说明信息不足。

## 回答格式
1. 每个要点附带 [来源X] 标注
2. 优先使用上下文中的原始措辞`

// Generate 生成回答
func (g *LLMGenerator) Generate(ctx context.Context, query string, ragContext string) (string, error) {
	if g.model == nil {
		return "", fmt.Errorf("model not configured")
	}

	messages, err := g.template.Format(ctx, map[string]any{
		"system_prompt": g.systemPrompt,
		"rag_context":   ragContext,
		"query":         query,
	})
	if err != nil {
		return "", fmt.Errorf("format prompt: %w", err)
	}

	resp, err := g.model.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	return resp.Content, nil
}

// GenerateStream 流式生成回答
// 【Eino 特点】使用 Eino 的 StreamReader 实现流式输出
func (g *LLMGenerator) GenerateStream(ctx context.Context, query string, ragContext string) (<-chan string, error) {
	if g.model == nil {
		return nil, fmt.Errorf("model not configured")
	}

	messages, err := g.template.Format(ctx, map[string]any{
		"system_prompt": g.systemPrompt,
		"rag_context":   ragContext,
		"query":         query,
	})
	if err != nil {
		return nil, fmt.Errorf("format prompt: %w", err)
	}

	// 【Eino 特点】调用 Stream 方法获取流式响应
	reader, err := g.model.Stream(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("stream: %w", err)
	}

	ch := make(chan string, 100)

	go func() {
		defer close(ch)
		defer reader.Close()

		for {
			chunk, err := reader.Recv()
			if err != nil {
				break
			}
			if chunk.Content != "" {
				ch <- chunk.Content
			}
		}
	}()

	return ch, nil
}
