// Package pipeline 查询重写节点实现
package pipeline

import (
	"context"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// LLMRewriter 基于 LLM 的查询重写器
// 【Eino 特点】可作为 Graph 中的 Lambda 节点使用
type LLMRewriter struct {
	model    model.ChatModel
	template prompt.ChatTemplate
}

// NewLLMRewriter 创建 LLM 重写器
func NewLLMRewriter(model model.ChatModel) *LLMRewriter {
	tpl := prompt.FromMessages(
		schema.FString,
		schema.UserMessage(`请将以下用户查询优化为更适合检索的形式。
要求：
1. 保持原意
2. 扩展关键词
3. 移除口语化表达
4. 如果查询已经很清晰，直接返回原查询

用户查询: {query}

优化后的查询:`),
	)
	return &LLMRewriter{model: model, template: tpl}
}

// Rewrite 重写查询
func (r *LLMRewriter) Rewrite(ctx context.Context, query string) (string, error) {
	if r.model == nil {
		return query, nil
	}

	messages, err := r.template.Format(ctx, map[string]any{
		"query": query,
	})
	if err != nil {
		return query, err
	}

	resp, err := r.model.Generate(ctx, messages)
	if err != nil {
		return query, err
	}

	return resp.Content, nil
}

// splitLines 分割文本为行
func splitLines(text string) []string {
	var lines []string
	var current []byte
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			if len(current) > 0 {
				lines = append(lines, string(current))
				current = current[:0]
			}
		} else {
			current = append(current, text[i])
		}
	}
	if len(current) > 0 {
		lines = append(lines, string(current))
	}
	return lines
}
