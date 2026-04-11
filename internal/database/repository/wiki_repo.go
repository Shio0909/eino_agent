// Package repository - Wiki 页面数据访问层
package repository

import (
	"context"
	"fmt"
	"time"

	"eino_agent/internal/database/postgres"
)

// ============================================================================
// Wiki Models
// ============================================================================

// WikiPage LLM 编译的 wiki 页面
type WikiPage struct {
	ID                string    `json:"id"`
	KnowledgeBaseID   string    `json:"knowledge_base_id"`
	SourceKnowledgeID *string   `json:"source_knowledge_id"`
	Path              string    `json:"path"`      // 如 'index.md', 'kubernetes/pods.md'
	Title             string    `json:"title"`
	Content           string    `json:"content"`
	PageType          string    `json:"page_type"` // index, topic, entity
	Metadata          JSON      `json:"metadata"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// WikiLink 页面间的交叉引用
type WikiLink struct {
	ID           string    `json:"id"`
	SourcePageID string    `json:"source_page_id"`
	TargetPath   string    `json:"target_path"`
	TargetPageID *string   `json:"target_page_id"`
	LinkText     *string   `json:"link_text"`
	CreatedAt    time.Time `json:"created_at"`
}

// ============================================================================
// Wiki Repository Interface
// ============================================================================

// WikiPageRepository wiki 页面存储接口
type WikiPageRepository interface {
	// UpsertPage 创建或更新 wiki 页面（按 knowledge_base_id + path 唯一）
	UpsertPage(ctx context.Context, page *WikiPage) error
	// BatchUpsertPages 批量创建或更新 wiki 页面
	BatchUpsertPages(ctx context.Context, pages []*WikiPage) error
	// GetPageByPath 按路径获取页面
	GetPageByPath(ctx context.Context, kbID, path string) (*WikiPage, error)
	// ListPages 列出知识库的所有 wiki 页面
	ListPages(ctx context.Context, kbID string) ([]*WikiPage, error)
	// SearchPages 全文搜索 wiki 页面
	SearchPages(ctx context.Context, kbID string, query string, limit int) ([]*WikiPage, error)
	// DeletePagesByKnowledgeBase 删除知识库的所有 wiki 页面
	DeletePagesByKnowledgeBase(ctx context.Context, kbID string) error
	// DeletePagesBySourceKnowledge 删除某个源文档生成的所有 wiki 页面
	DeletePagesBySourceKnowledge(ctx context.Context, sourceKnowledgeID string) error

	// UpsertLinks 批量创建交叉引用（先删除旧链接）
	UpsertLinks(ctx context.Context, pageID string, links []*WikiLink) error
	// GetLinkedPages 获取与指定页面交叉引用的所有页面
	GetLinkedPages(ctx context.Context, pageID string) ([]*WikiPage, error)
	// ResolveLinks 解析 wiki 链接，将 target_path 匹配到 target_page_id
	ResolveLinks(ctx context.Context, kbID string) error
}

// ============================================================================
// PostgreSQL Implementation
// ============================================================================

type pgWikiRepo struct {
	db *postgres.DB
}

// NewWikiPageRepository 创建 wiki 页面 repository
func NewWikiPageRepository(db *postgres.DB) WikiPageRepository {
	return &pgWikiRepo{db: db}
}

func (r *pgWikiRepo) UpsertPage(ctx context.Context, page *WikiPage) error {
	query := `
INSERT INTO wiki_pages (knowledge_base_id, source_knowledge_id, path, title, content, page_type, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (knowledge_base_id, path) DO UPDATE SET
    title = EXCLUDED.title,
    content = EXCLUDED.content,
    page_type = EXCLUDED.page_type,
    source_knowledge_id = EXCLUDED.source_knowledge_id,
    metadata = EXCLUDED.metadata,
    updated_at = CURRENT_TIMESTAMP
RETURNING id`
	return r.db.QueryRow(ctx, query,
		page.KnowledgeBaseID, page.SourceKnowledgeID, page.Path,
		page.Title, page.Content, page.PageType, page.Metadata,
	).Scan(&page.ID)
}

func (r *pgWikiRepo) BatchUpsertPages(ctx context.Context, pages []*WikiPage) error {
	for _, page := range pages {
		if err := r.UpsertPage(ctx, page); err != nil {
			return fmt.Errorf("upsert page %q: %w", page.Path, err)
		}
	}
	return nil
}

func (r *pgWikiRepo) GetPageByPath(ctx context.Context, kbID, path string) (*WikiPage, error) {
	query := `
SELECT id, knowledge_base_id, source_knowledge_id, path, title, content, page_type, metadata, created_at, updated_at
FROM wiki_pages
WHERE knowledge_base_id = $1 AND path = $2`

	p := &WikiPage{}
	err := r.db.QueryRow(ctx, query, kbID, path).Scan(
		&p.ID, &p.KnowledgeBaseID, &p.SourceKnowledgeID,
		&p.Path, &p.Title, &p.Content, &p.PageType,
		&p.Metadata, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *pgWikiRepo) ListPages(ctx context.Context, kbID string) ([]*WikiPage, error) {
	query := `
SELECT id, knowledge_base_id, source_knowledge_id, path, title, content, page_type, metadata, created_at, updated_at
FROM wiki_pages
WHERE knowledge_base_id = $1
ORDER BY page_type, path`

	rows, err := r.db.Query(ctx, query, kbID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []*WikiPage
	for rows.Next() {
		p := &WikiPage{}
		if err := rows.Scan(
			&p.ID, &p.KnowledgeBaseID, &p.SourceKnowledgeID,
			&p.Path, &p.Title, &p.Content, &p.PageType,
			&p.Metadata, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

func (r *pgWikiRepo) SearchPages(ctx context.Context, kbID string, query string, limit int) ([]*WikiPage, error) {
	sqlQuery := `
SELECT id, knowledge_base_id, source_knowledge_id, path, title, content, page_type, metadata, created_at, updated_at,
    (ts_rank_cd(to_tsvector('simple', content), plainto_tsquery('simple', $2))
     + CASE WHEN content ILIKE '%' || $2 || '%' THEN 0.2 ELSE 0 END
     + CASE WHEN title ILIKE '%' || $2 || '%' THEN 0.5 ELSE 0 END) AS score
FROM wiki_pages
WHERE knowledge_base_id = $1
  AND (to_tsvector('simple', content) @@ plainto_tsquery('simple', $2)
       OR content ILIKE '%' || $2 || '%'
       OR title ILIKE '%' || $2 || '%')
ORDER BY score DESC
LIMIT $3`

	rows, err := r.db.Query(ctx, sqlQuery, kbID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []*WikiPage
	for rows.Next() {
		p := &WikiPage{}
		var score float64
		if err := rows.Scan(
			&p.ID, &p.KnowledgeBaseID, &p.SourceKnowledgeID,
			&p.Path, &p.Title, &p.Content, &p.PageType,
			&p.Metadata, &p.CreatedAt, &p.UpdatedAt,
			&score,
		); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

func (r *pgWikiRepo) DeletePagesByKnowledgeBase(ctx context.Context, kbID string) error {
	// wiki_links 通过 ON DELETE CASCADE 自动清理
	return r.db.Exec(ctx, `DELETE FROM wiki_pages WHERE knowledge_base_id = $1`, kbID)
}

func (r *pgWikiRepo) DeletePagesBySourceKnowledge(ctx context.Context, sourceKnowledgeID string) error {
	return r.db.Exec(ctx, `DELETE FROM wiki_pages WHERE source_knowledge_id = $1`, sourceKnowledgeID)
}

func (r *pgWikiRepo) UpsertLinks(ctx context.Context, pageID string, links []*WikiLink) error {
	// 先删除旧链接
	if err := r.db.Exec(ctx, `DELETE FROM wiki_links WHERE source_page_id = $1`, pageID); err != nil {
		return err
	}
	// 插入新链接
	for _, link := range links {
		err := r.db.Exec(ctx,
			`INSERT INTO wiki_links (source_page_id, target_path, link_text) VALUES ($1, $2, $3)`,
			pageID, link.TargetPath, link.LinkText,
		)
		if err != nil {
			return fmt.Errorf("insert link %q: %w", link.TargetPath, err)
		}
	}
	return nil
}

func (r *pgWikiRepo) GetLinkedPages(ctx context.Context, pageID string) ([]*WikiPage, error) {
	query := `
SELECT wp.id, wp.knowledge_base_id, wp.source_knowledge_id, wp.path, wp.title, wp.content,
       wp.page_type, wp.metadata, wp.created_at, wp.updated_at
FROM wiki_pages wp
INNER JOIN wiki_links wl ON wl.target_page_id = wp.id
WHERE wl.source_page_id = $1`

	rows, err := r.db.Query(ctx, query, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []*WikiPage
	for rows.Next() {
		p := &WikiPage{}
		if err := rows.Scan(
			&p.ID, &p.KnowledgeBaseID, &p.SourceKnowledgeID,
			&p.Path, &p.Title, &p.Content, &p.PageType,
			&p.Metadata, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

func (r *pgWikiRepo) ResolveLinks(ctx context.Context, kbID string) error {
	return r.db.Exec(ctx, `
UPDATE wiki_links wl
SET target_page_id = wp.id
FROM wiki_pages wp
WHERE wp.knowledge_base_id = $1
  AND wp.path = wl.target_path
  AND wl.target_page_id IS NULL
  AND wl.source_page_id IN (SELECT id FROM wiki_pages WHERE knowledge_base_id = $1)
`, kbID)
}
