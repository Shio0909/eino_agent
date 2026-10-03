package container

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"

	"eino_agent/internal/config"
	"eino_agent/internal/database/postgres"
)

// PgVectorDB PostgreSQL + pgvector 实现。
//
// knowledge_base_id / knowledge_id 作为由 metadata 派生的存储列并建索引，
// 检索时范围过滤在 SQL 中完成，而不是取回全局候选后在内存里过滤。
type PgVectorDB struct {
	pg             *postgres.DB
	dimensions     int
	tableName      string
	iterativeScan  bool
	pgvectorVerStr string
}

func newPgVectorDB(ctx context.Context, cfg *config.DatabaseConfig, dimensions int) (*PgVectorDB, CleanupFunc, error) {
	if dimensions <= 0 {
		return nil, nil, fmt.Errorf("无效向量维度: %d", dimensions)
	}

	pg, err := postgres.New(ctx, &postgres.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		Database: cfg.DBName,
		SSLMode:  cfg.SSLMode,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}

	db := &PgVectorDB{pg: pg, dimensions: dimensions, tableName: "rag_vectors"}
	if err := db.initSchema(ctx, cfg.ResetVectorsOnDimensionChange); err != nil {
		pg.Close()
		return nil, nil, fmt.Errorf("初始化 pgvector schema 失败: %w", err)
	}

	cleanup := func(context.Context) error {
		db.pg.Close()
		return nil
	}
	return db, cleanup, nil
}

func (db *PgVectorDB) initSchema(ctx context.Context, resetOnDimensionChange bool) error {
	if err := db.pg.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		return err
	}
	if err := db.detectCapabilities(ctx); err != nil {
		return err
	}
	if err := db.ensureTable(ctx); err != nil {
		return err
	}

	currentDim, err := db.currentEmbeddingDimension(ctx)
	if err != nil {
		return err
	}
	if currentDim > 0 && currentDim != db.dimensions {
		if !resetOnDimensionChange {
			return fmt.Errorf(
				"向量表 %s 维度为 %d，但当前 embedding 维度为 %d；请恢复原 embedding 配置，"+
					"或设置 database.reset_vectors_on_dimension_change=true 清空向量后重新索引",
				db.tableName, currentDim, db.dimensions)
		}
		log.Printf("[VectorDB] 维度变更 %d -> %d，按配置清空向量表 %s（需要重新索引）", currentDim, db.dimensions, db.tableName)
		if err := db.pg.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, db.tableName)); err != nil {
			return fmt.Errorf("删除旧向量表失败: %w", err)
		}
		if err := db.ensureTable(ctx); err != nil {
			return fmt.Errorf("重建向量表失败: %w", err)
		}
	}
	return db.ensureIndexes(ctx)
}

func (db *PgVectorDB) detectCapabilities(ctx context.Context) error {
	var version string
	if err := db.pg.QueryRow(ctx, `SELECT extversion FROM pg_extension WHERE extname = 'vector'`).Scan(&version); err != nil {
		return fmt.Errorf("读取 pgvector 版本失败: %w", err)
	}
	db.pgvectorVerStr = version
	db.iterativeScan = versionAtLeast(version, 0, 8)
	return nil
}

func versionAtLeast(version string, major, minor int) bool {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return false
	}
	ma, err1 := strconv.Atoi(parts[0])
	mi, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return ma > major || (ma == major && mi >= minor)
}

func (db *PgVectorDB) ensureTable(ctx context.Context) error {
	stmts := []string{
		fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	id TEXT PRIMARY KEY,
	content TEXT NOT NULL,
	metadata JSONB NOT NULL DEFAULT '{}',
	embedding vector(%d) NOT NULL,
	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
)`, db.tableName, db.dimensions),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS knowledge_base_id TEXT GENERATED ALWAYS AS (metadata->>'knowledge_base_id') STORED`, db.tableName),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS knowledge_id TEXT GENERATED ALWAYS AS (metadata->>'knowledge_id') STORED`, db.tableName),
	}
	for _, stmt := range stmts {
		if err := db.pg.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (db *PgVectorDB) ensureIndexes(ctx context.Context) error {
	t := db.tableName
	stmts := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_embedding_hnsw_idx ON %s USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64)`, t, t),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_kb_idx ON %s (knowledge_base_id)`, t, t),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_knowledge_idx ON %s (knowledge_id)`, t, t),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_content_fts_idx ON %s USING gin (to_tsvector('simple', content))`, t, t),
	}
	for _, stmt := range stmts {
		if err := db.pg.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (db *PgVectorDB) currentEmbeddingDimension(ctx context.Context) (int, error) {
	const query = `
SELECT COALESCE((
	SELECT a.atttypmod
	FROM pg_attribute a
	JOIN pg_class c ON a.attrelid = c.oid
	JOIN pg_namespace n ON c.relnamespace = n.oid
	WHERE c.relname = $1
	  AND n.nspname = current_schema()
	  AND a.attname = 'embedding'
	  AND a.attnum > 0
	  AND NOT a.attisdropped
	LIMIT 1
), 0)`
	var dimension int
	if err := db.pg.QueryRow(ctx, query, db.tableName).Scan(&dimension); err != nil {
		return 0, fmt.Errorf("查询向量列维度失败: %w", err)
	}
	return dimension, nil
}

func (db *PgVectorDB) Ping(ctx context.Context) error {
	return db.pg.Ping(ctx)
}

// Upsert 在单个事务中批量写入，任一 chunk 失败则整体回滚。
func (db *PgVectorDB) Upsert(ctx context.Context, docs []*Document) error {
	if len(docs) == 0 {
		return nil
	}
	query := fmt.Sprintf(`
INSERT INTO %s (id, content, metadata, embedding, updated_at)
VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO UPDATE SET
	content = EXCLUDED.content,
	metadata = EXCLUDED.metadata,
	embedding = EXCLUDED.embedding,
	updated_at = CURRENT_TIMESTAMP`, db.tableName)

	batch := &pgx.Batch{}
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if len(doc.Vector) != db.dimensions {
			return fmt.Errorf("文档 %s 向量维度不匹配: got=%d expect=%d", doc.ID, len(doc.Vector), db.dimensions)
		}
		metadataJSON, err := json.Marshal(doc.Metadata)
		if err != nil {
			return fmt.Errorf("序列化 metadata 失败: %w", err)
		}
		batch.Queue(query, doc.ID, doc.Content, metadataJSON, pgvector.NewVector(doc.Vector))
	}
	if batch.Len() == 0 {
		return nil
	}

	return db.pg.WithTx(ctx, func(tx pgx.Tx) error {
		results := tx.SendBatch(ctx, batch)
		for i := 0; i < batch.Len(); i++ {
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				return fmt.Errorf("upsert 向量失败: %w", err)
			}
		}
		return results.Close()
	})
}

// scopeClause 生成范围过滤 SQL，参数从 argStart 开始编号。
func scopeClause(filter SearchFilter, argStart int) (string, []any) {
	var (
		conds []string
		args  []any
	)
	if len(filter.KnowledgeBaseIDs) > 0 {
		conds = append(conds, fmt.Sprintf("knowledge_base_id = ANY($%d)", argStart+len(args)))
		args = append(args, filter.KnowledgeBaseIDs)
	}
	if len(filter.KnowledgeIDs) > 0 {
		idx := argStart + len(args)
		conds = append(conds, fmt.Sprintf("(knowledge_id = ANY($%d) OR id = ANY($%d))", idx, idx))
		args = append(args, filter.KnowledgeIDs)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return strings.Join(conds, " AND "), args
}

func (db *PgVectorDB) Search(ctx context.Context, vector []float32, topK int, filter SearchFilter) ([]*Document, error) {
	filter = filter.Normalized()
	if filter.DeniesAll() {
		return []*Document{}, nil
	}
	if topK <= 0 {
		topK = 10
	}
	if len(vector) != db.dimensions {
		return nil, fmt.Errorf("查询向量维度不匹配: got=%d expect=%d", len(vector), db.dimensions)
	}

	where, scopeArgs := scopeClause(filter, 3)
	if where != "" {
		where = "WHERE " + where
	}
	query := fmt.Sprintf(`
SELECT id, content, metadata, (1 - (embedding <=> $1)) AS score
FROM %s
%s
ORDER BY embedding <=> $1
LIMIT $2`, db.tableName, where)
	args := append([]any{pgvector.NewVector(vector), topK}, scopeArgs...)

	var docs []*Document
	err := db.pg.WithTx(ctx, func(tx pgx.Tx) error {
		// HNSW 在带过滤条件时只在 ef_search 个候选内过滤，可能召回不足；
		// 提高 ef_search，并在 pgvector>=0.8 时启用 iterative scan 持续扩展候选。
		efSearch := topK * 4
		if efSearch < 64 {
			efSearch = 64
		}
		if efSearch > 1000 {
			efSearch = 1000
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL hnsw.ef_search = %d", efSearch)); err != nil {
			return err
		}
		if where != "" && db.iterativeScan {
			if _, err := tx.Exec(ctx, "SET LOCAL hnsw.iterative_scan = strict_order"); err != nil {
				return err
			}
		}
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("pgvector 检索失败: %w", err)
		}
		docs, err = scanDocuments(rows, topK)
		return err
	})
	if err != nil {
		return nil, err
	}
	return docs, nil
}

// SearchKeyword 关键词检索（PostgreSQL 全文检索 + ILIKE 子串匹配）。
func (db *PgVectorDB) SearchKeyword(ctx context.Context, keyword string, topK int, filter SearchFilter) ([]*Document, error) {
	filter = filter.Normalized()
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || filter.DeniesAll() {
		return []*Document{}, nil
	}
	if topK <= 0 {
		topK = 10
	}

	scope, scopeArgs := scopeClause(filter, 4)
	if scope != "" {
		scope = "AND " + scope
	}
	query := fmt.Sprintf(`
SELECT id, content, metadata,
	(ts_rank_cd(to_tsvector('simple', content), plainto_tsquery('simple', $1))
	 + CASE WHEN content ILIKE $3 ESCAPE '\' THEN 0.2 ELSE 0 END) AS score
FROM %s
WHERE (to_tsvector('simple', content) @@ plainto_tsquery('simple', $1) OR content ILIKE $3 ESCAPE '\')
%s
ORDER BY score DESC, id
LIMIT $2`, db.tableName, scope)
	args := append([]any{keyword, topK, "%" + escapeLike(keyword) + "%"}, scopeArgs...)

	rows, err := db.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("关键词检索失败: %w", err)
	}
	return scanDocuments(rows, topK)
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func scanDocuments(rows pgx.Rows, capacity int) ([]*Document, error) {
	defer rows.Close()
	results := make([]*Document, 0, capacity)
	for rows.Next() {
		var (
			id      string
			content string
			metaRaw []byte
			score   float64
		)
		if err := rows.Scan(&id, &content, &metaRaw, &score); err != nil {
			return nil, err
		}
		metadata := map[string]interface{}{}
		if len(metaRaw) > 0 {
			if err := json.Unmarshal(metaRaw, &metadata); err != nil {
				return nil, fmt.Errorf("解析向量 metadata 失败 id=%s: %w", id, err)
			}
		}
		results = append(results, &Document{ID: id, Content: content, Score: score, Metadata: metadata})
	}
	return results, rows.Err()
}

func (db *PgVectorDB) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := db.pg.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ANY($1)`, db.tableName), ids); err != nil {
		return fmt.Errorf("删除向量文档失败: %w", err)
	}
	return nil
}

// UpdateChunkIndexes 只改 metadata 里的 chunk_index，不动向量与内容。
func (db *PgVectorDB) UpdateChunkIndexes(ctx context.Context, indexes map[string]int) error {
	if len(indexes) == 0 {
		return nil
	}
	ids := make([]string, 0, len(indexes))
	positions := make([]int32, 0, len(indexes))
	for id, index := range indexes {
		ids = append(ids, id)
		positions = append(positions, int32(index))
	}
	query := fmt.Sprintf(`
UPDATE %s AS t
SET metadata = jsonb_set(t.metadata, '{chunk_index}', to_jsonb(v.idx)), updated_at = CURRENT_TIMESTAMP
FROM (SELECT unnest($1::text[]) AS id, unnest($2::int[]) AS idx) AS v
WHERE t.id = v.id`, db.tableName)
	if err := db.pg.Exec(ctx, query, ids, positions); err != nil {
		return fmt.Errorf("更新向量 chunk 序号失败: %w", err)
	}
	return nil
}

func (db *PgVectorDB) DeleteByKnowledgeID(ctx context.Context, knowledgeID string) error {
	if err := db.pg.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE knowledge_id = $1`, db.tableName), knowledgeID); err != nil {
		return fmt.Errorf("按文档ID删除向量失败: %w", err)
	}
	return nil
}

func (db *PgVectorDB) DeleteByKnowledgeBaseID(ctx context.Context, kbID string) error {
	if err := db.pg.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE knowledge_base_id = $1`, db.tableName), kbID); err != nil {
		return fmt.Errorf("按知识库ID删除向量失败: %w", err)
	}
	return nil
}

func (db *PgVectorDB) Close() error {
	if db.pg != nil {
		db.pg.Close()
	}
	return nil
}
