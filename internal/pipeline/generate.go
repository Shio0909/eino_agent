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

const defaultSystemPrompt = `你是一个专业的知识库问答助手。请根据提供的上下文信息回答用户的问题。

## 忠实性原则（最高优先级）
- 你的回答**必须且只能**基于提供的上下文信息，**严禁**使用自身训练知识补充或推断
- 如果上下文信息不足以回答问题，请直接说明信息不足
- 禁止编造文档名称、命令、版本号等任何具体信息

## 回答规范
1. 回答要简洁、准确、有条理
2. 引用来源时使用 [来源X] 标记`

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

// ChatGenerator 对话式生成器（带历史上下文）
type ChatGenerator struct {
	model        model.ChatModel
	systemPrompt string
	maxHistory   int
	template     prompt.ChatTemplate
}

// NewChatGenerator 创建对话生成器
func NewChatGenerator(m model.ChatModel, systemPrompt string, maxHistory int) *ChatGenerator {
	if maxHistory <= 0 {
		maxHistory = 10
	}
	tpl := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage("{system_prompt}"),
		schema.MessagesPlaceholder("history", true),
		schema.UserMessage(`上下文信息：
{rag_context}

用户问题：{query}`),
	)
	return &ChatGenerator{
		model:        m,
		systemPrompt: systemPrompt,
		maxHistory:   maxHistory,
		template:     tpl,
	}
}

// GenerateWithHistory 带历史上下文的生成
func (g *ChatGenerator) GenerateWithHistory(
	ctx context.Context,
	query string,
	ragContext string,
	history []*schema.Message,
) (string, error) {
	if g.model == nil {
		return "", fmt.Errorf("model not configured")
	}

	// 保留最近的 N 条历史消息
	startIdx := 0
	if len(history) > g.maxHistory {
		startIdx = len(history) - g.maxHistory
	}

	messages, err := g.template.Format(ctx, map[string]any{
		"system_prompt": g.systemPrompt,
		"history":       history[startIdx:],
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
