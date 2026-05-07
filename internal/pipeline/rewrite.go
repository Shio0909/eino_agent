package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type ctxKey string

const ctxKeyQueryHistory ctxKey = "query_history_context"

// LLMRewriter 基于 LLM 的查询重写器
type LLMRewriter struct {
	model    model.ChatModel
	template prompt.ChatTemplate
}

// NewLLMRewriter 创建 LLM 重写器
func NewLLMRewriter(chatModel model.ChatModel) *LLMRewriter {
	tpl := prompt.FromMessages(
		schema.FString,
		schema.UserMessage(`{query_context}
请将以下用户查询优化为更适合检索的形式。
要求：
1. 保持原意
2. 扩展关键词
3. 移除口语化表达
4. 如果查询已经很清晰，直接返回原查询
5. 如果上文讨论主题与当前问题明显相关，则结合上文把指代词（"它"、"那个"等）替换为具体实体名称
6. 如果上文与当前问题不相关，忽略上文

用户查询: {query}

优化后的查询:`),
	)
	return &LLMRewriter{model: chatModel, template: tpl}
}

// Rewrite 重写查询
// 从 ctx 中读取可选的历史上下文（通过 WithQueryHistory 注入），自动织入 rewrite prompt。
func (r *LLMRewriter) Rewrite(ctx context.Context, query string) (string, error) {
	if r.model == nil {
		return query, nil
	}

	queryCtx := ""
	if v := ctx.Value(ctxKeyQueryHistory); v != nil {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			queryCtx = fmt.Sprintf("【上文讨论主题】\n%s\n", strings.TrimSpace(s))
		}
	}

	messages, err := r.template.Format(ctx, map[string]any{
		"query":         query,
		"query_context": queryCtx,
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

// WithQueryHistory 将上文上下文注入 ctx，供 Rewrite 使用。
// 传入最近几轮对话的简短摘要（建议 50-80 字），空字符串会被忽略。
func WithQueryHistory(ctx context.Context, summary string) context.Context {
	if strings.TrimSpace(summary) == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyQueryHistory, strings.TrimSpace(summary))
}
