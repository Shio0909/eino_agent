package document

import (
	"context"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"eino_agent/internal/container"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ContextualEnricher 上下文富化器
// 参考 Anthropic Contextual Retrieval：为每个 chunk 添加 LLM 生成的上下文前缀
// 使 chunk 在被单独检索时仍能理解其在原文中的位置和含义
type ContextualEnricher struct {
	LLM           model.ChatModel
	MaxDocContext int // 传给 LLM 的文档上下文最大字符数
}

// NewContextualEnricher 创建上下文富化器
func NewContextualEnricher(llm model.ChatModel) *ContextualEnricher {
	return &ContextualEnricher{
		LLM:           llm,
		MaxDocContext:  4000,
	}
}

const enrichPromptTemplate = `<document>
%s
</document>

以上是一篇完整文档。下面是文档中的一个片段（chunk）：

<chunk>
%s
</chunk>

请用1-2句简短的话描述这个片段在文档中的位置和上下文，帮助读者理解这个片段的背景。
要求：
- 只输出描述文字，不要加任何前缀标记
- 不要复述片段内容，只说明"这段内容位于文档的哪个部分，讲的是什么主题"
- 控制在50字以内`

// Enrich 为一组 chunks 添加上下文前缀
// docContent: 原始完整文档内容
// chunks: 待富化的 chunk 列表
// 返回新的 chunk 列表（Content 被添加了上下文前缀）
func (e *ContextualEnricher) Enrich(ctx context.Context, docContent string, chunks []*container.Document) ([]*container.Document, error) {
	// 截断文档上下文
	docCtx := truncateText(docContent, e.MaxDocContext)

	enriched := make([]*container.Document, len(chunks))
	for i, chunk := range chunks {
		prefix, err := e.generateContext(ctx, docCtx, chunk.Content)
		if err != nil {
			log.Printf("[ContextualEnricher] chunk %d 富化失败，保留原文: %v", i, err)
			enriched[i] = chunk
			continue
		}

		// 复制 chunk，添加前缀
		newChunk := &container.Document{
			ID:       chunk.ID,
			Content:  fmt.Sprintf("[上下文: %s]\n%s", prefix, chunk.Content),
			Vector:   chunk.Vector,
			Metadata: copyMetadata(chunk.Metadata),
		}
		newChunk.Metadata["contextual_prefix"] = prefix
		newChunk.Metadata["enriched"] = true
		enriched[i] = newChunk
	}

	return enriched, nil
}

// generateContext 调用 LLM 生成单个 chunk 的上下文描述
func (e *ContextualEnricher) generateContext(ctx context.Context, docContext, chunkContent string) (string, error) {
	prompt := fmt.Sprintf(enrichPromptTemplate, docContext, chunkContent)

	msg := &schema.Message{
		Role:    schema.User,
		Content: prompt,
	}

	resp, err := e.LLM.Generate(ctx, []*schema.Message{msg})
	if err != nil {
		return "", fmt.Errorf("LLM 生成上下文失败: %w", err)
	}

	result := strings.TrimSpace(resp.Content)
	// 限制前缀长度
	if utf8.RuneCountInString(result) > 100 {
		runes := []rune(result)
		result = string(runes[:100])
	}

	return result, nil
}

// truncateText 截断文本到指定字符数
func truncateText(text string, maxChars int) string {
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}
	return string(runes[:maxChars]) + "\n...[文档已截断]"
}

// copyMetadata 深拷贝 metadata
func copyMetadata(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
