// Package container 提供 Markdown 模式的全文检索器
package container

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"eino_agent/internal/database/postgres"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// MarkdownRetriever 基于 PostgreSQL 全文检索的 Markdown 模式检索器
// 直接查询 chunks 表（无需 embedding），使用 tsvector + ILIKE 回退
type MarkdownRetriever struct {
	db   *postgres.DB
	topK int
}

// NewMarkdownRetriever 创建 Markdown 检索器
func NewMarkdownRetriever(db *postgres.DB, topK int) *MarkdownRetriever {
	if topK <= 0 {
		topK = 10
	}
	return &MarkdownRetriever{db: db, topK: topK}
}

// Retrieve 全文检索 chunks 表
func (r *MarkdownRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	topK := r.topK

	docs, err := r.searchChunksFTS(ctx, nil, query, topK)
	if err != nil {
		return nil, fmt.Errorf("markdown 全文检索失败: %w", err)
	}

	return docs, nil
}

// RetrieveScoped 按知识库 ID 范围检索
func (r *MarkdownRetriever) RetrieveScoped(ctx context.Context, kbIDs []string, query string, topK int) ([]*schema.Document, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if topK <= 0 {
		topK = r.topK
	}
	return r.searchChunksFTS(ctx, kbIDs, query, topK)
}

// searchChunksFTS 在 chunks 表上执行全文检索
func (r *MarkdownRetriever) searchChunksFTS(ctx context.Context, kbIDs []string, keyword string, topK int) ([]*schema.Document, error) {
	var (
		sqlQuery string
		args     []interface{}
	)

	if len(kbIDs) > 0 {
		sqlQuery = `
SELECT id, knowledge_id, knowledge_base_id, chunk_index, content, metadata,
	(ts_rank_cd(to_tsvector('simple', content), plainto_tsquery('simple', $1))
	 + CASE WHEN content ILIKE '%' || $1 || '%' THEN 0.2 ELSE 0 END) AS score
FROM chunks
WHERE knowledge_base_id = ANY($3)
  AND (to_tsvector('simple', content) @@ plainto_tsquery('simple', $1)
       OR content ILIKE '%' || $1 || '%')
ORDER BY score DESC
LIMIT $2`
		args = []interface{}{keyword, topK, kbIDs}
	} else {
		sqlQuery = `
SELECT id, knowledge_id, knowledge_base_id, chunk_index, content, metadata,
	(ts_rank_cd(to_tsvector('simple', content), plainto_tsquery('simple', $1))
	 + CASE WHEN content ILIKE '%' || $1 || '%' THEN 0.2 ELSE 0 END) AS score
FROM chunks
WHERE to_tsvector('simple', content) @@ plainto_tsquery('simple', $1)
      OR content ILIKE '%' || $1 || '%'
ORDER BY score DESC
LIMIT $2`
		args = []interface{}{keyword, topK}
	}

	rows, err := r.db.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []*schema.Document
	for rows.Next() {
		var (
			id      string
			knID    string
			kbID    string
			chunkIdx int
			content string
			metaRaw []byte
			score   float64
		)
		if err := rows.Scan(&id, &knID, &kbID, &chunkIdx, &content, &metaRaw, &score); err != nil {
			return nil, err
		}

		metadata := map[string]interface{}{
			"knowledge_id":      knID,
			"knowledge_base_id": kbID,
			"chunk_index":       chunkIdx,
			"match_type":        "markdown_fts",
			"score":             score,
		}
		if len(metaRaw) > 0 {
			var extra map[string]interface{}
			if err := json.Unmarshal(metaRaw, &extra); err == nil {
				for k, v := range extra {
					metadata[k] = v
				}
			}
		}

		docs = append(docs, &schema.Document{
			ID:       id,
			Content:  content,
			MetaData: metadata,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	log.Printf("[MarkdownRetriever] 检索到 %d 个结果 (query=%q)", len(docs), keyword)
	return docs, nil
}
