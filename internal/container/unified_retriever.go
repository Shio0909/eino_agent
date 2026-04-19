// Package container - 统一检索器
//
// UnifiedRetriever 聚合 vector 和 wiki 两种检索来源，
// 根据请求中的知识库 mode 自动路由到对应检索器并合并结果。
package container

import (
	"context"
	"log"
	"strings"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/database/repository"
)

// UnifiedRetriever 统一检索器，聚合 vector + wiki 两种来源
type UnifiedRetriever struct {
	vectorRetriever *CompositeRetriever           // 向量检索（可为 nil）
	wikiRetriever   *WikiRetriever                // Wiki 全文检索（可为 nil）
	kbRepo          repository.KnowledgeBaseRepository // 用于查询 KB mode
	topK            int
}

// NewUnifiedRetriever 创建统一检索器
func NewUnifiedRetriever(
	vectorRetriever *CompositeRetriever,
	wikiRetriever *WikiRetriever,
	kbRepo repository.KnowledgeBaseRepository,
	topK int,
) *UnifiedRetriever {
	if topK <= 0 {
		topK = 10
	}
	return &UnifiedRetriever{
		vectorRetriever: vectorRetriever,
		wikiRetriever:   wikiRetriever,
		kbRepo:          kbRepo,
		topK:            topK,
	}
}

// Retrieve 全局检索（不限定 KB）— 仅使用向量检索
func (u *UnifiedRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	if u.vectorRetriever == nil {
		return nil, nil
	}
	return u.vectorRetriever.Retrieve(ctx, query, opts...)
}

// WithKnowledgeBaseScope 按知识库范围创建作用域检索器
// 自动将 KB IDs 按 mode 分组，路由到不同检索器
func (u *UnifiedRetriever) WithKnowledgeBaseScope(ids []string) retriever.Retriever {
	if len(ids) == 0 {
		return u
	}

	// 如果没有 kbRepo，无法查 mode，全部走 vector
	if u.kbRepo == nil {
		if u.vectorRetriever != nil {
			return u.vectorRetriever.WithKnowledgeBaseScope(ids)
		}
		return u
	}

	var vectorIDs, wikiIDs []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		// 使用 background context 因为 WithKnowledgeBaseScope 接口不接受 ctx
		kb, err := u.kbRepo.GetByID(context.Background(), id)
		if err != nil {
			log.Printf("[UnifiedRetriever] 查询 KB %s mode 失败: %v, 默认走 vector", id, err)
			vectorIDs = append(vectorIDs, id)
			continue
		}
		if kb.IsWikiMode() {
			wikiIDs = append(wikiIDs, id)
		} else {
			vectorIDs = append(vectorIDs, id)
		}
	}

	return &scopedUnifiedRetriever{
		vectorRetriever: u.vectorRetriever,
		wikiRetriever:   u.wikiRetriever,
		vectorKBIDs:     vectorIDs,
		wikiKBIDs:       wikiIDs,
		topK:            u.topK,
	}
}

// scopedUnifiedRetriever 已按 KB mode 分组的作用域检索器
type scopedUnifiedRetriever struct {
	vectorRetriever *CompositeRetriever
	wikiRetriever   *WikiRetriever
	vectorKBIDs     []string
	wikiKBIDs       []string
	topK            int
}

func (s *scopedUnifiedRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	var allDocs []*schema.Document

	// 1) 向量检索（vector-mode KBs）
	if len(s.vectorKBIDs) > 0 && s.vectorRetriever != nil {
		scoped := s.vectorRetriever.WithKnowledgeBaseScope(s.vectorKBIDs)
		docs, err := scoped.Retrieve(ctx, query, opts...)
		if err != nil {
			log.Printf("[UnifiedRetriever] vector 检索失败: %v", err)
		} else {
			allDocs = append(allDocs, docs...)
		}
	}

	// 2) Wiki 全文检索（wiki-mode KBs）
	if len(s.wikiKBIDs) > 0 && s.wikiRetriever != nil {
		docs, err := s.wikiRetriever.RetrieveScoped(ctx, s.wikiKBIDs, query, s.topK)
		if err != nil {
			log.Printf("[UnifiedRetriever] wiki 检索失败: %v", err)
		} else {
			allDocs = append(allDocs, docs...)
		}
	}

	// 3) 去重 + 截断
	allDocs = deduplicateDocs(allDocs)
	if len(allDocs) > s.topK {
		allDocs = allDocs[:s.topK]
	}

	log.Printf("[UnifiedRetriever] 检索完成: vector_kbs=%d wiki_kbs=%d total_docs=%d",
		len(s.vectorKBIDs), len(s.wikiKBIDs), len(allDocs))
	return allDocs, nil
}

// deduplicateDocs 按 Document ID 去重，保留先出现的
func deduplicateDocs(docs []*schema.Document) []*schema.Document {
	seen := make(map[string]struct{}, len(docs))
	result := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		key := strings.TrimSpace(doc.ID)
		if key == "" {
			result = append(result, doc) // 没有 ID 的文档保留
			continue
		}
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, doc)
		}
	}
	return result
}

// GetVectorRetriever 返回底层的向量检索器（供需要直接访问的场景使用）
func (u *UnifiedRetriever) GetVectorRetriever() *CompositeRetriever {
	return u.vectorRetriever
}

// ensure interfaces
var _ retriever.Retriever = (*UnifiedRetriever)(nil)
var _ retriever.Retriever = (*scopedUnifiedRetriever)(nil)
