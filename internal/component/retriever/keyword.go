// Package retriever - 关键词检索器实现
package retriever

import (
	"context"
	"log"

	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/database/repository"
)

// KeywordRetrieverImpl 关键词检索器
// 使用 PostgreSQL pg_trgm 扩展进行模糊文本匹配
type KeywordRetrieverImpl struct {
	embeddingRepo    repository.EmbeddingRepository
	knowledgeBaseIDs []string
	topK             int
	minSimilarity    float64
}

// KeywordRetrieverConfig 关键词检索器配置
type KeywordRetrieverConfig struct {
	EmbeddingRepo    repository.EmbeddingRepository
	KnowledgeBaseIDs []string
	TopK             int
	MinSimilarity    float64
}

// NewKeywordRetriever 创建关键词检索器
func NewKeywordRetriever(cfg *KeywordRetrieverConfig) *KeywordRetrieverImpl {
	topK := cfg.TopK
	if topK <= 0 {
		topK = 10
	}
	minSimilarity := cfg.MinSimilarity
	if minSimilarity <= 0 {
		minSimilarity = 0.1
	}

	return &KeywordRetrieverImpl{
		embeddingRepo:    cfg.EmbeddingRepo,
		knowledgeBaseIDs: cfg.KnowledgeBaseIDs,
		topK:             topK,
		minSimilarity:    minSimilarity,
	}
}

// Retrieve 实现 Eino retriever.Retriever 接口
func (k *KeywordRetrieverImpl) Retrieve(ctx context.Context, query string, opts ...einoretriever.Option) ([]*schema.Document, error) {
	kbIDs := k.knowledgeBaseIDs
	if len(kbIDs) == 0 {
		log.Println("[KeywordRetriever] 未指定知识库 ID，跳过关键词检索")
		return nil, nil
	}

	results, err := k.embeddingRepo.SearchByKeyword(ctx, kbIDs, query, k.topK, k.minSimilarity)
	if err != nil {
		log.Printf("[KeywordRetriever] 关键词检索失败: %v", err)
		return nil, err
	}

	docs := make([]*schema.Document, 0, len(results))
	for _, r := range results {
		docs = append(docs, &schema.Document{
			ID:      r.ID,
			Content: r.Content,
			MetaData: map[string]any{
				"match_type":        string(MatchTypeKeyword),
				"score":             r.Score,
				"knowledge_id":      r.KnowledgeID,
				"knowledge_base_id": r.KnowledgeBaseID,
				"chunk_id":          r.ChunkID,
			},
		})
	}

	log.Printf("[KeywordRetriever] 关键词检索返回 %d 个结果", len(docs))
	return docs, nil
}

// SetKnowledgeBaseIDs 动态设置知识库 ID
func (k *KeywordRetrieverImpl) SetKnowledgeBaseIDs(ids []string) {
	k.knowledgeBaseIDs = ids
}

// GetType 返回组件类型
func (k *KeywordRetrieverImpl) GetType() string {
	return "KeywordRetriever"
}

// IsCallbacksEnabled 是否启用回调
func (k *KeywordRetrieverImpl) IsCallbacksEnabled() bool {
	return true
}
