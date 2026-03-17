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

// MultiQueryRewriter 多查询重写器
// 生成多个不同角度的查询以提高召回率
type MultiQueryRewriter struct {
	model    model.ChatModel
	numQuery int
	template prompt.ChatTemplate
}

// NewMultiQueryRewriter 创建多查询重写器
func NewMultiQueryRewriter(model model.ChatModel, numQuery int) *MultiQueryRewriter {
	if numQuery <= 0 {
		numQuery = 3
	}
	tpl := prompt.FromMessages(
		schema.FString,
		schema.UserMessage(`请从不同角度重写以下查询，生成 {num_query} 个语义相似但表述不同的检索查询。
每个查询一行，不要编号。

原始查询: {query}

重写后的查询:`),
	)
	return &MultiQueryRewriter{model: model, numQuery: numQuery, template: tpl}
}

// Rewrite 生成多个查询（返回第一个，其他通过 MultiRewrite 获取）
func (r *MultiQueryRewriter) Rewrite(ctx context.Context, query string) (string, error) {
	queries, err := r.MultiRewrite(ctx, query)
	if err != nil || len(queries) == 0 {
		return query, err
	}
	return queries[0], nil
}

// MultiRewrite 生成多个查询
func (r *MultiQueryRewriter) MultiRewrite(ctx context.Context, query string) ([]string, error) {
	if r.model == nil {
		return []string{query}, nil
	}

	messages, err := r.template.Format(ctx, map[string]any{
		"num_query": r.numQuery,
		"query":     query,
	})
	if err != nil {
		return []string{query}, err
	}

	resp, err := r.model.Generate(ctx, messages)
	if err != nil {
		return []string{query}, err
	}

	// 简单的按行分割
	queries := splitLines(resp.Content)
	if len(queries) == 0 {
		queries = []string{query}
	}

	return queries, nil
}

// HyDERewriter HyDE (Hypothetical Document Embeddings) 重写器
// 让 LLM 生成假设性答案，用答案作为检索查询
type HyDERewriter struct {
	model    model.ChatModel
	template prompt.ChatTemplate
}

// NewHyDERewriter 创建 HyDE 重写器
func NewHyDERewriter(model model.ChatModel) *HyDERewriter {
	tpl := prompt.FromMessages(
		schema.FString,
		schema.UserMessage(`请为以下问题写一个假设性的答案段落，这个段落应该包含回答该问题所需的关键信息。
不需要是真实准确的答案，只需要是一个合理的假设性回答。

问题: {query}

假设性答案:`),
	)
	return &HyDERewriter{model: model, template: tpl}
}

// Rewrite 使用 HyDE 方式重写查询
func (r *HyDERewriter) Rewrite(ctx context.Context, query string) (string, error) {
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
