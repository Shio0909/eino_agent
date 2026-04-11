// Package container - Wiki 模式检索器
// 通过 index.md 索引导航 + 全文搜索 wiki 页面
package container

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/database/repository"
)

// WikiRetriever 基于 wiki 页面的检索器
// 检索策略：先搜索 wiki 页面 → 展开交叉引用页面
type WikiRetriever struct {
	wikiRepo repository.WikiPageRepository
	topK     int
}

// NewWikiRetriever 创建 wiki 检索器
func NewWikiRetriever(wikiRepo repository.WikiPageRepository, topK int) *WikiRetriever {
	if topK <= 0 {
		topK = 5
	}
	return &WikiRetriever{wikiRepo: wikiRepo, topK: topK}
}

// Retrieve 全局检索（不限定 KB）
func (r *WikiRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	// wiki 检索必须有 KB 范围，全局检索不适用
	return nil, nil
}

// RetrieveScoped 按知识库范围检索 wiki 页面
func (r *WikiRetriever) RetrieveScoped(ctx context.Context, kbIDs []string, query string, topK int) ([]*schema.Document, error) {
	query = strings.TrimSpace(query)
	if query == "" || len(kbIDs) == 0 {
		return nil, nil
	}
	if topK <= 0 {
		topK = r.topK
	}

	var allDocs []*schema.Document
	seen := make(map[string]bool)

	for _, kbID := range kbIDs {
		docs, err := r.retrieveFromKB(ctx, kbID, query, topK)
		if err != nil {
			log.Printf("[WikiRetriever] KB %s 检索失败: %v", kbID, err)
			continue
		}
		for _, doc := range docs {
			if !seen[doc.ID] {
				seen[doc.ID] = true
				allDocs = append(allDocs, doc)
			}
		}
	}

	// 限制总结果数
	if len(allDocs) > topK {
		allDocs = allDocs[:topK]
	}

	log.Printf("[WikiRetriever] 检索到 %d 个 wiki 页面 (query=%q, kbs=%d)", len(allDocs), query, len(kbIDs))
	return allDocs, nil
}

// retrieveFromKB 从单个知识库检索
func (r *WikiRetriever) retrieveFromKB(ctx context.Context, kbID, query string, topK int) ([]*schema.Document, error) {
	// Step 1: 搜索匹配的 wiki 页面
	pages, err := r.wikiRepo.SearchPages(ctx, kbID, query, topK)
	if err != nil {
		return nil, fmt.Errorf("搜索 wiki 页面失败: %w", err)
	}

	// Step 2: 展开交叉引用（获取链接的相关页面）
	var linkedPages []*repository.WikiPage
	for _, page := range pages {
		linked, err := r.wikiRepo.GetLinkedPages(ctx, page.ID)
		if err != nil {
			continue
		}
		linkedPages = append(linkedPages, linked...)
	}

	// Step 3: 转换为 schema.Document
	seen := make(map[string]bool)
	var docs []*schema.Document

	// 主结果
	for _, page := range pages {
		if seen[page.ID] {
			continue
		}
		seen[page.ID] = true
		docs = append(docs, wikiPageToDocument(page, "wiki_search"))
	}

	// 交叉引用结果（附加到主结果后面）
	for _, page := range linkedPages {
		if seen[page.ID] {
			continue
		}
		seen[page.ID] = true
		docs = append(docs, wikiPageToDocument(page, "wiki_linked"))
	}

	return docs, nil
}

// wikiPageToDocument 将 wiki 页面转换为 Eino Document
func wikiPageToDocument(page *repository.WikiPage, matchType string) *schema.Document {
	return &schema.Document{
		ID:      page.ID,
		Content: page.Content,
		MetaData: map[string]interface{}{
			"knowledge_base_id": page.KnowledgeBaseID,
			"wiki_path":         page.Path,
			"wiki_title":        page.Title,
			"wiki_page_type":    page.PageType,
			"match_type":        matchType,
		},
	}
}
