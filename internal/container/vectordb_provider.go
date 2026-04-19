// Package container - 向量数据库提供者实现
//
// 【Eino 特点】支持多种向量数据库后端
// 支持: PostgreSQL+pgvector, Milvus, 内存存储
package container

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"eino_agent/internal/config"
	"eino_agent/internal/database/postgres"

	"github.com/pgvector/pgvector-go"
)

// NewVectorDBProvider 创建向量数据库提供者
func NewVectorDBProvider(ctx context.Context, cfg *config.DatabaseConfig, dimensions int) (VectorDBProvider, CleanupFunc, error) {
	// 优先使用 Milvus
	if cfg.MilvusAddr != "" {
		db, cleanup, err := newMilvusVectorDB(ctx, cfg, dimensions)
		if err == nil {
			return db, cleanup, nil
		}
		fmt.Printf("[VectorDB] Milvus 初始化失败，降级到内存存储: %v\n", err)
	}

	// 其次使用 PostgreSQL + pgvector
	if cfg.Host != "" && cfg.DBName != "" {
		db, cleanup, err := newPgVectorDB(ctx, cfg, dimensions)
		if err == nil {
			return db, cleanup, nil
		}
		fmt.Printf("[VectorDB] pgvector 初始化失败，降级到内存存储: %v\n", err)
	}

	// 默认使用内存存储（开发环境）
	return newMemoryVectorDB(ctx, dimensions)
}

// MemoryVectorDB 内存向量数据库（用于开发测试）
type MemoryVectorDB struct {
	mu         sync.RWMutex
	docs       map[string]*Document
	dimensions int
	dataFile   string
}

// newMemoryVectorDB 创建内存向量数据库
func newMemoryVectorDB(ctx context.Context, dimensions int) (*MemoryVectorDB, CleanupFunc, error) {
	db := &MemoryVectorDB{
		docs:       make(map[string]*Document),
		dimensions: dimensions,
		dataFile:   "data/vectors.json",
	}

	// 尝试从文件加载
	if err := db.loadFromFile(); err != nil {
		// 文件不存在是正常的
		fmt.Printf("向量数据文件不存在，使用空数据库\n")
	}

	cleanup := func(ctx context.Context) error {
		return db.saveToFile()
	}

	return db, cleanup, nil
}

// Upsert 插入或更新文档
func (db *MemoryVectorDB) Upsert(ctx context.Context, docs []*Document) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	for _, doc := range docs {
		db.docs[doc.ID] = doc
	}

	return nil
}

// Search 向量搜索
func (db *MemoryVectorDB) Search(ctx context.Context, vector []float32, topK int) ([]*Document, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if len(db.docs) == 0 {
		return []*Document{}, nil
	}

	// 计算余弦相似度并排序
	type scored struct {
		doc   *Document
		score float64
	}
	results := make([]scored, 0, len(db.docs))

	for _, doc := range db.docs {
		if doc.Vector == nil {
			continue
		}
		score := cosineSimilarity(vector, doc.Vector)
		results = append(results, scored{doc: doc, score: score})
	}

	// 按分数降序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// 取 TopK
	if topK > len(results) {
		topK = len(results)
	}

	docs := make([]*Document, topK)
	for i := 0; i < topK; i++ {
		docs[i] = &Document{
			ID:       results[i].doc.ID,
			Content:  results[i].doc.Content,
			Score:    results[i].score,
			Metadata: results[i].doc.Metadata,
		}
	}

	return docs, nil
}

// SearchKeyword 关键词检索（内存版）
func (db *MemoryVectorDB) SearchKeyword(ctx context.Context, query string, topK int) ([]*Document, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return []*Document{}, nil
	}
	if topK <= 0 {
		topK = 10
	}

	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		tokens = []string{query}
	}

	type scored struct {
		doc   *Document
		score float64
	}

	results := make([]scored, 0, len(db.docs))
	for _, doc := range db.docs {
		if doc == nil {
			continue
		}
		content := strings.ToLower(doc.Content)
		matchCount := 0
		for _, token := range tokens {
			if token == "" {
				continue
			}
			if strings.Contains(content, token) {
				matchCount++
			}
		}
		if matchCount == 0 {
			continue
		}

		score := float64(matchCount) / float64(len(tokens))
		results = append(results, scored{doc: doc, score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if topK > len(results) {
		topK = len(results)
	}

	docs := make([]*Document, 0, topK)
	for i := 0; i < topK; i++ {
		item := results[i]
		docs = append(docs, &Document{
			ID:       item.doc.ID,
			Content:  item.doc.Content,
			Score:    item.score,
			Metadata: item.doc.Metadata,
		})
	}

	return docs, nil
}

// Delete 删除文档
func (db *MemoryVectorDB) Delete(ctx context.Context, ids []string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	for _, id := range ids {
		delete(db.docs, id)
	}
	return nil
}

// DeleteByKnowledgeID 删除指定文档的所有向量 chunk
func (db *MemoryVectorDB) DeleteByKnowledgeID(ctx context.Context, knowledgeID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	for id, doc := range db.docs {
		if kid, ok := doc.Metadata["knowledge_id"].(string); ok && kid == knowledgeID {
			delete(db.docs, id)
		}
	}
	return nil
}

// DeleteByKnowledgeBaseID 删除指定知识库的所有向量 chunk
func (db *MemoryVectorDB) DeleteByKnowledgeBaseID(ctx context.Context, kbID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	for id, doc := range db.docs {
		if kid, ok := doc.Metadata["knowledge_base_id"].(string); ok && kid == kbID {
			delete(db.docs, id)
		}
	}
	return nil
}

// Close 关闭数据库
func (db *MemoryVectorDB) Close() error {
	return db.saveToFile()
}

// loadFromFile 从文件加载数据
func (db *MemoryVectorDB) loadFromFile() error {
	data, err := os.ReadFile(db.dataFile)
	if err != nil {
		return err
	}

	var docs []*Document
	if err := json.Unmarshal(data, &docs); err != nil {
		return err
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	for _, doc := range docs {
		db.docs[doc.ID] = doc
	}

	return nil
}

// saveToFile 保存数据到文件
func (db *MemoryVectorDB) saveToFile() error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	docs := make([]*Document, 0, len(db.docs))
	for _, doc := range db.docs {
		docs = append(docs, doc)
	}

	data, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(db.dataFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(db.dataFile, data, 0644)
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// PgVectorDB PostgreSQL + pgvector 实现
type PgVectorDB struct {
	pg         *postgres.DB
	dimensions int
	tableName  string
}

func newPgVectorDB(ctx context.Context, cfg *config.DatabaseConfig, dimensions int) (*PgVectorDB, CleanupFunc, error) {
	if dimensions <= 0 {
		return nil, nil, fmt.Errorf("无效向量维度: %d", dimensions)
	}

	pgCfg := &postgres.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		Database: cfg.DBName,
		SSLMode:  cfg.SSLMode,
	}

	pg, err := postgres.New(ctx, pgCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}

	db := &PgVectorDB{
		pg:         pg,
		dimensions: dimensions,
		tableName:  "rag_vectors",
	}

	if err := db.initSchema(ctx); err != nil {
		pg.Close()
		return nil, nil, fmt.Errorf("初始化 pgvector schema 失败: %w", err)
	}

	cleanup := func(ctx context.Context) error {
		db.pg.Close()
		return nil
	}

	return db, cleanup, nil
}

func (db *PgVectorDB) initSchema(ctx context.Context) error {
	if err := db.pg.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		return err
	}

	if err := db.ensureSchemaObjects(ctx); err != nil {
		return err
	}

	currentDim, err := db.currentEmbeddingDimension(ctx)
	if err != nil {
		return err
	}

	if currentDim > 0 && currentDim != db.dimensions {
		fmt.Printf("[VectorDB] 检测到向量维度变更，重建表 %s: old=%d new=%d\n", db.tableName, currentDim, db.dimensions)
		dropSQL := fmt.Sprintf(`DROP TABLE IF EXISTS %s CASCADE`, db.tableName)
		if err := db.pg.Exec(ctx, dropSQL); err != nil {
			return fmt.Errorf("删除旧向量表失败: %w", err)
		}
		if err := db.ensureSchemaObjects(ctx); err != nil {
			return fmt.Errorf("重建向量表失败: %w", err)
		}
	}

	return nil
}

func (db *PgVectorDB) ensureSchemaObjects(ctx context.Context) error {
	createSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	id TEXT PRIMARY KEY,
	content TEXT NOT NULL,
	metadata JSONB DEFAULT '{}',
	embedding vector(%d) NOT NULL,
	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
)`, db.tableName, db.dimensions)

	if err := db.pg.Exec(ctx, createSQL); err != nil {
		return err
	}

	indexSQL := fmt.Sprintf(`
CREATE INDEX IF NOT EXISTS %s_embedding_hnsw_idx
ON %s USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64)`, db.tableName, db.tableName)

	if err := db.pg.Exec(ctx, indexSQL); err != nil {
		return err
	}

	return nil
}

func (db *PgVectorDB) currentEmbeddingDimension(ctx context.Context) (int, error) {
	query := `
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

func (db *PgVectorDB) Upsert(ctx context.Context, docs []*Document) error {
	if len(docs) == 0 {
		return nil
	}

	query := fmt.Sprintf(`
INSERT INTO %s (id, content, metadata, embedding, updated_at)
VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
ON CONFLICT (id)
DO UPDATE SET
	content = EXCLUDED.content,
	metadata = EXCLUDED.metadata,
	embedding = EXCLUDED.embedding,
	updated_at = CURRENT_TIMESTAMP`, db.tableName)

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

		if err := db.pg.Exec(ctx, query, doc.ID, doc.Content, metadataJSON, pgvector.NewVector(doc.Vector)); err != nil {
			return fmt.Errorf("upsert 文档 %s 失败: %w", doc.ID, err)
		}
	}

	return nil
}

func (db *PgVectorDB) Search(ctx context.Context, vector []float32, topK int) ([]*Document, error) {
	if topK <= 0 {
		topK = 10
	}
	if len(vector) != db.dimensions {
		return nil, fmt.Errorf("查询向量维度不匹配: got=%d expect=%d", len(vector), db.dimensions)
	}

	query := fmt.Sprintf(`
SELECT id, content, metadata, (1 - (embedding <=> $1)) AS score
FROM %s
ORDER BY embedding <=> $1
LIMIT $2`, db.tableName)

	rows, err := db.pg.Query(ctx, query, pgvector.NewVector(vector), topK)
	if err != nil {
		return nil, fmt.Errorf("pgvector 检索失败: %w", err)
	}
	defer rows.Close()

	results := make([]*Document, 0, topK)
	for rows.Next() {
		var (
			id       string
			content  string
			metaRaw  []byte
			score    float64
		)
		if err := rows.Scan(&id, &content, &metaRaw, &score); err != nil {
			return nil, err
		}

		metadata := map[string]interface{}{}
		if len(metaRaw) > 0 {
			_ = json.Unmarshal(metaRaw, &metadata)
		}

		results = append(results, &Document{
			ID:       id,
			Content:  content,
			Score:    score,
			Metadata: metadata,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// SearchKeyword 关键词检索（PostgreSQL 全文检索 + ILIKE 回退）
func (db *PgVectorDB) SearchKeyword(ctx context.Context, keyword string, topK int) ([]*Document, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []*Document{}, nil
	}
	if topK <= 0 {
		topK = 10
	}

	query := fmt.Sprintf(`
SELECT id, content, metadata,
	(ts_rank_cd(to_tsvector('simple', content), plainto_tsquery('simple', $1))
	 + CASE WHEN content ILIKE '%%' || $1 || '%%' THEN 0.2 ELSE 0 END) AS score
FROM %s
WHERE to_tsvector('simple', content) @@ plainto_tsquery('simple', $1)
	OR content ILIKE '%%' || $1 || '%%'
ORDER BY score DESC
LIMIT $2`, db.tableName)

	rows, err := db.pg.Query(ctx, query, keyword, topK)
	if err != nil {
		return nil, fmt.Errorf("关键词检索失败: %w", err)
	}
	defer rows.Close()

	results := make([]*Document, 0, topK)
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
			_ = json.Unmarshal(metaRaw, &metadata)
		}

		results = append(results, &Document{
			ID:       id,
			Content:  content,
			Score:    score,
			Metadata: metadata,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (db *PgVectorDB) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ANY($1)`, db.tableName)
	if err := db.pg.Exec(ctx, query, ids); err != nil {
		return fmt.Errorf("删除向量文档失败: %w", err)
	}
	return nil
}

func (db *PgVectorDB) DeleteByKnowledgeID(ctx context.Context, knowledgeID string) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE metadata->>'knowledge_id' = $1`, db.tableName)
	if err := db.pg.Exec(ctx, query, knowledgeID); err != nil {
		return fmt.Errorf("按文档ID删除向量失败: %w", err)
	}
	return nil
}

func (db *PgVectorDB) DeleteByKnowledgeBaseID(ctx context.Context, kbID string) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE metadata->>'knowledge_base_id' = $1`, db.tableName)
	if err := db.pg.Exec(ctx, query, kbID); err != nil {
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

// MilvusVectorDB Milvus 实现（未完成）
//
// TODO: 实现 Milvus 支持。需要:
//   - 引入 go.milvus.io/milvus-sdk-go/v2 依赖
//   - 在 newMilvusVectorDB 中建立 gRPC 连接并创建 Collection
//   - 实现 Upsert（InsertRows + Flush）、Search（Search with ANN）
//   - 实现 Delete/DeleteByKnowledgeID/DeleteByKnowledgeBaseID（Delete by expr）
//
// 当前 newMilvusVectorDB 始终返回错误，系统会自动降级到 pgvector。
// 若不打算支持 Milvus，可删除此文件中整个 MilvusVectorDB 结构体及其方法。
type MilvusVectorDB struct {
	dimensions int
}

func newMilvusVectorDB(ctx context.Context, cfg *config.DatabaseConfig, dimensions int) (*MilvusVectorDB, CleanupFunc, error) {
	// TODO: 实现 Milvus 连接
	return nil, nil, fmt.Errorf("Milvus 暂未实现，请使用内存存储")
}

func (db *MilvusVectorDB) Upsert(ctx context.Context, docs []*Document) error {
	return fmt.Errorf("未实现")
}

func (db *MilvusVectorDB) Search(ctx context.Context, vector []float32, topK int) ([]*Document, error) {
	return nil, fmt.Errorf("未实现")
}

func (db *MilvusVectorDB) Delete(ctx context.Context, ids []string) error {
	return fmt.Errorf("未实现")
}

func (db *MilvusVectorDB) DeleteByKnowledgeID(ctx context.Context, knowledgeID string) error {
	return fmt.Errorf("未实现")
}

func (db *MilvusVectorDB) DeleteByKnowledgeBaseID(ctx context.Context, kbID string) error {
	return fmt.Errorf("未实现")
}

func (db *MilvusVectorDB) Close() error {
	return nil
}
