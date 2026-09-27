// Package container - 向量数据库提供者
//
// 支持 PostgreSQL+pgvector（生产）与内存存储（仅在未配置数据库时用于本地开发）。
package container

import (
	"context"
	"fmt"
	"log"
	"strings"

	"eino_agent/internal/config"
)

// SearchFilter 限定检索范围，由存储层下推执行（SQL WHERE / 内存过滤），
// 保证越权数据不会进入候选集。
type SearchFilter struct {
	KnowledgeBaseIDs []string
	// KnowledgeIDs 匹配文档 ID（knowledge_id）或 chunk ID。
	KnowledgeIDs []string
	// Scoped 为 true 表示调用方要求受限检索；此时若未给出任何 ID，直接返回空结果。
	Scoped bool
}

// IsZero 表示不限制范围（仅用于单租户/管理员场景）。
func (f SearchFilter) IsZero() bool {
	return !f.Scoped && len(f.KnowledgeBaseIDs) == 0 && len(f.KnowledgeIDs) == 0
}

// DeniesAll 表示受限检索但没有任何可访问范围。
func (f SearchFilter) DeniesAll() bool {
	return f.Scoped && len(f.KnowledgeBaseIDs) == 0 && len(f.KnowledgeIDs) == 0
}

// Normalized 去除空白与重复 ID。
func (f SearchFilter) Normalized() SearchFilter {
	return SearchFilter{
		KnowledgeBaseIDs: dedupeIDs(f.KnowledgeBaseIDs),
		KnowledgeIDs:     dedupeIDs(f.KnowledgeIDs),
		Scoped:           f.Scoped || len(f.KnowledgeBaseIDs) > 0 || len(f.KnowledgeIDs) > 0,
	}
}

// Matches 在内存中判断文档是否属于过滤范围。
func (f SearchFilter) Matches(doc *Document) bool {
	if doc == nil {
		return false
	}
	if f.IsZero() {
		return true
	}
	if f.DeniesAll() {
		return false
	}
	if len(f.KnowledgeBaseIDs) > 0 {
		kbID, _ := doc.Metadata["knowledge_base_id"].(string)
		if !containsID(f.KnowledgeBaseIDs, kbID) {
			return false
		}
	}
	if len(f.KnowledgeIDs) > 0 {
		kid, _ := doc.Metadata["knowledge_id"].(string)
		if !containsID(f.KnowledgeIDs, kid) && !containsID(f.KnowledgeIDs, doc.ID) {
			return false
		}
	}
	return true
}

func dedupeIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func containsID(ids []string, id string) bool {
	if id == "" {
		return false
	}
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

// NewVectorDBProvider 创建向量数据库提供者。
//
// 配置了 PostgreSQL 时必须连接成功，否则返回错误；不会静默降级到内存存储，
// 避免数据写入进程内存后在重启时丢失。
func NewVectorDBProvider(ctx context.Context, cfg *config.DatabaseConfig, dimensions int) (VectorDBProvider, CleanupFunc, error) {
	if cfg != nil && cfg.Host != "" && cfg.DBName != "" {
		db, cleanup, err := newPgVectorDB(ctx, cfg, dimensions)
		if err != nil {
			return nil, nil, fmt.Errorf("pgvector 初始化失败: %w", err)
		}
		return db, cleanup, nil
	}

	log.Printf("[VectorDB] 未配置 PostgreSQL，使用内存向量存储（仅适用于本地开发）")
	return newMemoryVectorDB(ctx, dimensions)
}
