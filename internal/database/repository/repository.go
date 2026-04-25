// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"

	"eino_agent/internal/database/postgres"
)

// ============================================================================
// Models (数据模型)
// ============================================================================

// Tenant 租户
type Tenant struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	APIKey       string    `json:"api_key"`
	StorageQuota int64     `json:"storage_quota"`
	UsedStorage  int64     `json:"used_storage"`
	Config       JSON      `json:"config"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// KnowledgeBase 知识库
type KnowledgeBase struct {
	ID                    string    `json:"id"`
	TenantID              int       `json:"tenant_id"`
	Name                  string    `json:"name"`
	Description           string    `json:"description"`
	Mode                  string    `json:"mode"`               // "vector"(默认) 或 "wiki"(LLM编译的Wiki知识库)
	EmbeddingModelID      *string   `json:"embedding_model_id"` // nullable FK → models(id)
	EmbeddingDimensions   int       `json:"embedding_dimensions"`
	EmbedModelFingerprint string    `json:"embed_model_fingerprint"` // "provider:model_id:dimensions" at last index time
	ChunkingConfig        JSON      `json:"chunking_config"`
	ExtractConfig         JSON      `json:"extract_config"`
	DocumentCount         int       `json:"document_count"`
	ChunkCount            int       `json:"chunk_count"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// IsWikiMode 是否为 Wiki (Karpathy) 模式
func (kb *KnowledgeBase) IsWikiMode() bool {
	return kb.Mode == "wiki"
}

// Knowledge 知识文档
type Knowledge struct {
	ID              string    `json:"id"`
	KnowledgeBaseID string    `json:"knowledge_base_id"`
	TagID           *string   `json:"tag_id"`
	Name            string    `json:"name"`
	SourceType      string    `json:"source_type"` // file, faq, url
	FileName        string    `json:"file_name"`
	FileType        string    `json:"file_type"`
	FileSize        int64     `json:"file_size"`
	FilePath        *string   `json:"file_path"`
	ParseStatus     string    `json:"parse_status"` // pending, processing, completed, failed
	ParseError      *string   `json:"parse_error"`
	ChunkCount      int       `json:"chunk_count"`
	ContentHash     string    `json:"content_hash"` // SHA256 of raw document content
	Metadata        JSON      `json:"metadata"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Chunk 文本块
type Chunk struct {
	ID              string    `json:"id"`
	KnowledgeID     string    `json:"knowledge_id"`
	KnowledgeBaseID string    `json:"knowledge_base_id"`
	ChunkIndex      int       `json:"chunk_index"`
	Content         string    `json:"content"`
	ContentLength   int       `json:"content_length"`
	ContentHash     string    `json:"content_hash"` // SHA256 of chunk content
	ParentChunkID   *string   `json:"parent_chunk_id"`
	StartPos        int       `json:"start_pos"`
	EndPos          int       `json:"end_pos"`
	ImageInfo       JSON      `json:"image_info"`
	Flags           int       `json:"flags"`
	Metadata        JSON      `json:"metadata"`
	CreatedAt       time.Time `json:"created_at"`
}

// Embedding 向量嵌入
type Embedding struct {
	ID              string          `json:"id"`
	ChunkID         string          `json:"chunk_id"`
	KnowledgeID     string          `json:"knowledge_id"`
	KnowledgeBaseID string          `json:"knowledge_base_id"`
	TagID           *string         `json:"tag_id"`
	Content         string          `json:"content"`
	Vector          pgvector.Vector `json:"-"`
	CreatedAt       time.Time       `json:"created_at"`
}

// AccessRole 知识库/空间访问角色。
type AccessRole string

const (
	AccessRoleViewer AccessRole = "viewer"
	AccessRoleEditor AccessRole = "editor"
	AccessRoleAdmin  AccessRole = "admin"
)

func (r AccessRole) Rank() int {
	switch r {
	case AccessRoleAdmin:
		return 3
	case AccessRoleEditor:
		return 2
	case AccessRoleViewer:
		return 1
	default:
		return 0
	}
}

func (r AccessRole) Allows(required AccessRole) bool {
	return r.Rank() >= required.Rank()
}

func MinAccessRole(a, b AccessRole) AccessRole {
	if a.Rank() <= b.Rank() {
		return a
	}
	return b
}

// Organization 组织/空间。
type Organization struct {
	ID          string    `json:"id"`
	TenantID    int       `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerUserID string    `json:"owner_user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// OrganizationMember 组织成员。
type OrganizationMember struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	TenantID       int        `json:"tenant_id"`
	UserID         string     `json:"user_id"`
	Role           AccessRole `json:"role"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// KnowledgeBaseShare 知识库分享授权。
type KnowledgeBaseShare struct {
	ID              string     `json:"id"`
	KnowledgeBaseID string     `json:"knowledge_base_id"`
	OrganizationID  string     `json:"organization_id"`
	SourceTenantID  int        `json:"source_tenant_id"`
	SharedByUserID  string     `json:"shared_by_user_id"`
	Permission      AccessRole `json:"permission"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Session 会话
type Session struct {
	ID                  string    `json:"id"`
	TenantID            int       `json:"tenant_id"`
	UserID              string    `json:"user_id"`
	AgentID             string    `json:"agent_id"`
	Title               string    `json:"title"`
	KnowledgeBaseIDs    []string  `json:"knowledge_base_ids"`
	RetrievalConfig     JSON      `json:"retrieval_config"`
	SimilarityThreshold float64   `json:"similarity_threshold"`
	TopK                int       `json:"top_k"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Message 消息
type Message struct {
	ID                  string    `json:"id"`
	SessionID           string    `json:"session_id"`
	Role                string    `json:"role"` // user, assistant, system
	Content             string    `json:"content"`
	KnowledgeReferences JSON      `json:"knowledge_references"`
	AgentSteps          JSON      `json:"agent_steps"`
	MentionedItems      JSON      `json:"mentioned_items"`
	TokensUsed          int       `json:"tokens_used"`
	LatencyMs           int       `json:"latency_ms"`
	Metadata            JSON      `json:"metadata"`
	CreatedAt           time.Time `json:"created_at"`
}

// JSON 辅助类型
type JSON map[string]interface{}

// Scan 实现 sql.Scanner 接口
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSON)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("invalid type for JSON: %T", value)
	}
	return json.Unmarshal(bytes, j)
}

// VectorSearchResult 向量搜索结果
type VectorSearchResult struct {
	Embedding
	Score float64 `json:"score"`
}

// ============================================================================
// Repository 接口
// ============================================================================

// KnowledgeBaseRepository 知识库仓储接口
type KnowledgeBaseRepository interface {
	Create(ctx context.Context, kb *KnowledgeBase) error
	GetByID(ctx context.Context, id string) (*KnowledgeBase, error)
	List(ctx context.Context, tenantID int, offset, limit int) ([]*KnowledgeBase, error)
	ListAccessible(ctx context.Context, tenantID int, userID string, offset, limit int) ([]*KnowledgeBase, error)
	Count(ctx context.Context, tenantID int) (int, error)
	CountAccessible(ctx context.Context, tenantID int, userID string) (int, error)
	Update(ctx context.Context, kb *KnowledgeBase) error
	Delete(ctx context.Context, id string) error
	IncrementCounts(ctx context.Context, id string, docDelta, chunkDelta int) error
	UpdateEmbedFingerprint(ctx context.Context, id, fingerprint string) error
}

// AccessControlRepository 管理组织成员与知识库分享权限。
type AccessControlRepository interface {
	CreateOrganization(ctx context.Context, org *Organization) error
	AddOrganizationMember(ctx context.Context, member *OrganizationMember) error
	ShareKnowledgeBase(ctx context.Context, share *KnowledgeBaseShare) error
	GetKnowledgeBaseRole(ctx context.Context, tenantID int, userID, kbID string) (AccessRole, error)
	ListAccessibleKnowledgeBaseIDs(ctx context.Context, tenantID int, userID string) ([]string, error)
}

// KnowledgeRepository 知识文档仓储接口
type KnowledgeRepository interface {
	Create(ctx context.Context, k *Knowledge) error
	GetByID(ctx context.Context, id string) (*Knowledge, error)
	ListByKnowledgeBase(ctx context.Context, kbID string, offset, limit int) ([]*Knowledge, error)
	CountByKnowledgeBase(ctx context.Context, kbID string) (int, error)
	UpdateParseStatus(ctx context.Context, id, status, errorMsg string, chunkCount int) error
	UpdateContentHash(ctx context.Context, id, hash string) error
	Delete(ctx context.Context, id string) error
}

// ChunkRepository 文本块仓储接口
type ChunkRepository interface {
	BatchCreate(ctx context.Context, chunks []*Chunk) error
	GetByKnowledgeID(ctx context.Context, knowledgeID string) ([]*Chunk, error)
	GetHashesByKnowledgeID(ctx context.Context, knowledgeID string) (map[string]string, error) // content_hash → chunk_id
	DeleteByIDs(ctx context.Context, ids []string) error
	DeleteByKnowledgeID(ctx context.Context, knowledgeID string) error
}

// EmbeddingRepository 向量嵌入仓储接口
type EmbeddingRepository interface {
	BatchCreate(ctx context.Context, embeddings []*Embedding) error
	Search(ctx context.Context, kbIDs []string, vector []float32, topK int, threshold float64) ([]*VectorSearchResult, error)
	SearchByKeyword(ctx context.Context, kbIDs []string, keyword string, topK int, minSimilarity float64) ([]*VectorSearchResult, error)
	DeleteByKnowledgeID(ctx context.Context, knowledgeID string) error
}

// SessionRepository 会话仓储接口
type SessionRepository interface {
	Create(ctx context.Context, s *Session) error
	GetByID(ctx context.Context, id string) (*Session, error)
	List(ctx context.Context, tenantID int, userID string, offset, limit int) ([]*Session, error)
	Delete(ctx context.Context, id string) error
	TouchUpdatedAt(ctx context.Context, id string) error
}

// MessageRepository 消息仓储接口
type MessageRepository interface {
	Create(ctx context.Context, m *Message) error
	ListBySession(ctx context.Context, sessionID string, limit int) ([]*Message, error)
}

// ============================================================================
// PostgreSQL 实现
// ============================================================================

// pgKnowledgeBaseRepo PostgreSQL 知识库仓储实现
type pgKnowledgeBaseRepo struct {
	db *postgres.DB
}

// NewKnowledgeBaseRepository 创建知识库仓储
func NewKnowledgeBaseRepository(db *postgres.DB) KnowledgeBaseRepository {
	return &pgKnowledgeBaseRepo{db: db}
}

func (r *pgKnowledgeBaseRepo) Create(ctx context.Context, kb *KnowledgeBase) error {
	chunkingConfig, _ := json.Marshal(kb.ChunkingConfig)
	extractConfig, _ := json.Marshal(kb.ExtractConfig)

	mode := kb.Mode
	if mode == "" {
		mode = "vector"
	}

	return r.db.QueryRow(ctx, `
		INSERT INTO knowledge_bases (tenant_id, name, description, mode, embedding_model_id, embedding_dimensions, embed_model_fingerprint, chunking_config, extract_config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`, kb.TenantID, kb.Name, kb.Description, mode, kb.EmbeddingModelID, kb.EmbeddingDimensions, kb.EmbedModelFingerprint, chunkingConfig, extractConfig).
		Scan(&kb.ID, &kb.CreatedAt, &kb.UpdatedAt)
}

func (r *pgKnowledgeBaseRepo) GetByID(ctx context.Context, id string) (*KnowledgeBase, error) {
	kb := &KnowledgeBase{}
	var chunkingConfig, extractConfig []byte

	err := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, mode, embedding_model_id, embedding_dimensions,
		       embed_model_fingerprint, chunking_config, extract_config, document_count, chunk_count, created_at, updated_at
		FROM knowledge_bases WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&kb.ID, &kb.TenantID, &kb.Name, &kb.Description, &kb.Mode, &kb.EmbeddingModelID, &kb.EmbeddingDimensions,
		&kb.EmbedModelFingerprint, &chunkingConfig, &extractConfig, &kb.DocumentCount, &kb.ChunkCount, &kb.CreatedAt, &kb.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(chunkingConfig, &kb.ChunkingConfig)
	_ = json.Unmarshal(extractConfig, &kb.ExtractConfig)
	return kb, nil
}

func (r *pgKnowledgeBaseRepo) List(ctx context.Context, tenantID int, offset, limit int) ([]*KnowledgeBase, error) {
	query := `
		SELECT id, tenant_id, name, description, mode, embedding_model_id, embedding_dimensions,
		       embed_model_fingerprint, chunking_config, extract_config, document_count, chunk_count, created_at, updated_at
		FROM knowledge_bases WHERE deleted_at IS NULL`
	args := []any{limit, offset}
	if tenantID > 0 {
		query += ` AND tenant_id = $3`
		args = append(args, tenantID)
	}
	query += ` ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	return scanKnowledgeBases(ctx, r.db, query, args...)
}

func (r *pgKnowledgeBaseRepo) ListAccessible(ctx context.Context, tenantID int, userID string, offset, limit int) ([]*KnowledgeBase, error) {
	if tenantID <= 0 {
		return r.List(ctx, tenantID, offset, limit)
	}
	query := `
		SELECT DISTINCT kb.id, kb.tenant_id, kb.name, kb.description, kb.mode, kb.embedding_model_id, kb.embedding_dimensions,
		       kb.embed_model_fingerprint, kb.chunking_config, kb.extract_config, kb.document_count, kb.chunk_count, kb.created_at, kb.updated_at
		FROM knowledge_bases kb
		LEFT JOIN knowledge_base_shares s ON s.knowledge_base_id = kb.id
		LEFT JOIN organization_members m ON m.organization_id = s.organization_id AND m.tenant_id = $1 AND m.user_id = $2
		WHERE kb.deleted_at IS NULL
		  AND (kb.tenant_id = $1 OR m.id IS NOT NULL)
		ORDER BY kb.created_at DESC
		LIMIT $3 OFFSET $4`
	return scanKnowledgeBases(ctx, r.db, query, tenantID, userID, limit, offset)
}

func scanKnowledgeBases(ctx context.Context, db *postgres.DB, query string, args ...any) ([]*KnowledgeBase, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*KnowledgeBase
	for rows.Next() {
		kb := &KnowledgeBase{}
		var chunkingConfig, extractConfig []byte
		if err := rows.Scan(
			&kb.ID, &kb.TenantID, &kb.Name, &kb.Description, &kb.Mode, &kb.EmbeddingModelID, &kb.EmbeddingDimensions,
			&kb.EmbedModelFingerprint, &chunkingConfig, &extractConfig, &kb.DocumentCount, &kb.ChunkCount, &kb.CreatedAt, &kb.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(chunkingConfig, &kb.ChunkingConfig)
		_ = json.Unmarshal(extractConfig, &kb.ExtractConfig)
		result = append(result, kb)
	}
	return result, rows.Err()
}

func (r *pgKnowledgeBaseRepo) Count(ctx context.Context, tenantID int) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM knowledge_bases WHERE deleted_at IS NULL`
	args := []any{}
	if tenantID > 0 {
		query += ` AND tenant_id = $1`
		args = append(args, tenantID)
	}
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *pgKnowledgeBaseRepo) CountAccessible(ctx context.Context, tenantID int, userID string) (int, error) {
	if tenantID <= 0 {
		return r.Count(ctx, tenantID)
	}
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT kb.id)
		FROM knowledge_bases kb
		LEFT JOIN knowledge_base_shares s ON s.knowledge_base_id = kb.id
		LEFT JOIN organization_members m ON m.organization_id = s.organization_id AND m.tenant_id = $1 AND m.user_id = $2
		WHERE kb.deleted_at IS NULL
		  AND (kb.tenant_id = $1 OR m.id IS NOT NULL)
	`, tenantID, userID).Scan(&count)
	return count, err
}

func (r *pgKnowledgeBaseRepo) Update(ctx context.Context, kb *KnowledgeBase) error {
	chunkingConfig, _ := json.Marshal(kb.ChunkingConfig)
	extractConfig, _ := json.Marshal(kb.ExtractConfig)

	_, err := r.db.Pool().Exec(ctx, `
		UPDATE knowledge_bases SET name = $2, description = $3, chunking_config = $4, extract_config = $5
		WHERE id = $1 AND deleted_at IS NULL
	`, kb.ID, kb.Name, kb.Description, chunkingConfig, extractConfig)
	return err
}

func (r *pgKnowledgeBaseRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Pool().Exec(ctx, `
		UPDATE knowledge_bases SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1
	`, id)
	return err
}

func (r *pgKnowledgeBaseRepo) IncrementCounts(ctx context.Context, id string, docDelta, chunkDelta int) error {
	_, err := r.db.Pool().Exec(ctx, `
		UPDATE knowledge_bases SET document_count = document_count + $2, chunk_count = chunk_count + $3
		WHERE id = $1
	`, id, docDelta, chunkDelta)
	return err
}

func (r *pgKnowledgeBaseRepo) UpdateEmbedFingerprint(ctx context.Context, id, fingerprint string) error {
	_, err := r.db.Pool().Exec(ctx, `
		UPDATE knowledge_bases SET embed_model_fingerprint = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND deleted_at IS NULL
	`, id, fingerprint)
	return err
}

// pgKnowledgeRepo PostgreSQL 知识文档仓储实现
type pgKnowledgeRepo struct {
	db *postgres.DB
}

// NewKnowledgeRepository 创建知识文档仓储
func NewKnowledgeRepository(db *postgres.DB) KnowledgeRepository {
	return &pgKnowledgeRepo{db: db}
}

func (r *pgKnowledgeRepo) Create(ctx context.Context, k *Knowledge) error {
	metadata, _ := json.Marshal(k.Metadata)
	return r.db.QueryRow(ctx, `
		INSERT INTO knowledges (knowledge_base_id, tag_id, name, source_type, file_name, file_type, file_size, file_path, parse_status, chunk_count, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`, k.KnowledgeBaseID, k.TagID, k.Name, k.SourceType, k.FileName, k.FileType, k.FileSize, k.FilePath, k.ParseStatus, k.ChunkCount, metadata).
		Scan(&k.ID, &k.CreatedAt, &k.UpdatedAt)
}

func (r *pgKnowledgeRepo) GetByID(ctx context.Context, id string) (*Knowledge, error) {
	k := &Knowledge{}
	var metadata []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, knowledge_base_id, tag_id, name, source_type, file_name, file_type, file_size, file_path,
		       parse_status, parse_error, chunk_count, content_hash, metadata, created_at, updated_at
		FROM knowledges WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&k.ID, &k.KnowledgeBaseID, &k.TagID, &k.Name, &k.SourceType, &k.FileName, &k.FileType, &k.FileSize, &k.FilePath,
		&k.ParseStatus, &k.ParseError, &k.ChunkCount, &k.ContentHash, &metadata, &k.CreatedAt, &k.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(metadata, &k.Metadata)
	return k, nil
}

func (r *pgKnowledgeRepo) ListByKnowledgeBase(ctx context.Context, kbID string, offset, limit int) ([]*Knowledge, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, knowledge_base_id, tag_id, name, source_type, file_name, file_type, file_size, file_path,
		       parse_status, parse_error, chunk_count, content_hash, metadata, created_at, updated_at
		FROM knowledges WHERE knowledge_base_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`, kbID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Knowledge
	for rows.Next() {
		k := &Knowledge{}
		var metadata []byte
		if err := rows.Scan(
			&k.ID, &k.KnowledgeBaseID, &k.TagID, &k.Name, &k.SourceType, &k.FileName, &k.FileType, &k.FileSize, &k.FilePath,
			&k.ParseStatus, &k.ParseError, &k.ChunkCount, &k.ContentHash, &metadata, &k.CreatedAt, &k.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metadata, &k.Metadata)
		result = append(result, k)
	}
	return result, rows.Err()
}

func (r *pgKnowledgeRepo) CountByKnowledgeBase(ctx context.Context, kbID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM knowledges WHERE knowledge_base_id = $1 AND deleted_at IS NULL`,
		kbID,
	).Scan(&count)
	return count, err
}

func (r *pgKnowledgeRepo) UpdateParseStatus(ctx context.Context, id, status, errorMsg string, chunkCount int) error {
	_, err := r.db.Pool().Exec(ctx, `
		UPDATE knowledges SET parse_status = $2, parse_error = $3, chunk_count = $4, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, id, status, errorMsg, chunkCount)
	return err
}

func (r *pgKnowledgeRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Pool().Exec(ctx, `
		UPDATE knowledges SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1
	`, id)
	return err
}

func (r *pgKnowledgeRepo) UpdateContentHash(ctx context.Context, id, hash string) error {
	_, err := r.db.Pool().Exec(ctx, `
		UPDATE knowledges SET content_hash = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1
	`, id, hash)
	return err
}

// pgEmbeddingRepo PostgreSQL 向量嵌入仓储实现
type pgEmbeddingRepo struct {
	db *postgres.DB
}

// NewEmbeddingRepository 创建向量嵌入仓储
func NewEmbeddingRepository(db *postgres.DB) EmbeddingRepository {
	return &pgEmbeddingRepo{db: db}
}

func (r *pgEmbeddingRepo) BatchCreate(ctx context.Context, embeddings []*Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		for _, e := range embeddings {
			_, err := tx.Exec(ctx, `
				INSERT INTO embeddings (chunk_id, knowledge_id, knowledge_base_id, tag_id, content, embedding)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, e.ChunkID, e.KnowledgeID, e.KnowledgeBaseID, e.TagID, e.Content, e.Vector)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *pgEmbeddingRepo) Search(ctx context.Context, kbIDs []string, vector []float32, topK int, threshold float64) ([]*VectorSearchResult, error) {
	queryVector := pgvector.NewVector(vector)

	rows, err := r.db.Query(ctx, `
		SELECT id, chunk_id, knowledge_id, knowledge_base_id, tag_id, content, created_at,
		       1 - (embedding <=> $1) as score
		FROM embeddings
		WHERE knowledge_base_id = ANY($2)
		  AND 1 - (embedding <=> $1) >= $3
		ORDER BY embedding <=> $1
		LIMIT $4
	`, queryVector, kbIDs, threshold, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*VectorSearchResult
	for rows.Next() {
		r := &VectorSearchResult{}
		if err := rows.Scan(
			&r.ID, &r.ChunkID, &r.KnowledgeID, &r.KnowledgeBaseID, &r.TagID, &r.Content, &r.CreatedAt, &r.Score,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (r *pgEmbeddingRepo) DeleteByKnowledgeID(ctx context.Context, knowledgeID string) error {
	_, err := r.db.Pool().Exec(ctx, `DELETE FROM embeddings WHERE knowledge_id = $1`, knowledgeID)
	return err
}

func (r *pgEmbeddingRepo) SearchByKeyword(ctx context.Context, kbIDs []string, keyword string, topK int, minSimilarity float64) ([]*VectorSearchResult, error) {
	// 使用 PostgreSQL 全文搜索 + ILIKE 回退
	rows, err := r.db.Query(ctx, `
		SELECT id, chunk_id, knowledge_id, knowledge_base_id, tag_id, content, created_at,
		       ts_rank(to_tsvector('simple', content), plainto_tsquery('simple', $1)) as score
		FROM embeddings
		WHERE knowledge_base_id = ANY($2)
		  AND content ILIKE '%' || $1 || '%'
		ORDER BY score DESC
		LIMIT $3
	`, keyword, kbIDs, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*VectorSearchResult
	for rows.Next() {
		r := &VectorSearchResult{}
		if err := rows.Scan(
			&r.ID, &r.ChunkID, &r.KnowledgeID, &r.KnowledgeBaseID, &r.TagID, &r.Content, &r.CreatedAt, &r.Score,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// pgSessionRepo PostgreSQL 会话仓储实现
type pgSessionRepo struct {
	db *postgres.DB
}

// NewSessionRepository 创建会话仓储
func NewSessionRepository(db *postgres.DB) SessionRepository {
	return &pgSessionRepo{db: db}
}

func (r *pgSessionRepo) Create(ctx context.Context, s *Session) error {
	retrievalConfig, _ := json.Marshal(s.RetrievalConfig)

	return r.db.QueryRow(ctx, `
		INSERT INTO sessions (tenant_id, user_id, agent_id, title, knowledge_base_ids, retrieval_config, similarity_threshold, top_k)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`, s.TenantID, s.UserID, s.AgentID, s.Title, s.KnowledgeBaseIDs, retrievalConfig, s.SimilarityThreshold, s.TopK).
		Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *pgSessionRepo) GetByID(ctx context.Context, id string) (*Session, error) {
	s := &Session{}
	var retrievalConfig []byte

	err := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, user_id, agent_id, title, knowledge_base_ids, retrieval_config, similarity_threshold, top_k, created_at, updated_at
		FROM sessions WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&s.ID, &s.TenantID, &s.UserID, &s.AgentID, &s.Title, &s.KnowledgeBaseIDs, &retrievalConfig, &s.SimilarityThreshold, &s.TopK, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(retrievalConfig, &s.RetrievalConfig)
	return s, nil
}

func (r *pgSessionRepo) List(ctx context.Context, tenantID int, userID string, offset, limit int) ([]*Session, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, user_id, agent_id, title, knowledge_base_ids, retrieval_config, similarity_threshold, top_k, created_at, updated_at
		FROM sessions WHERE tenant_id = $1 AND (user_id = $2 OR $2 = '') AND deleted_at IS NULL
		ORDER BY updated_at DESC LIMIT $3 OFFSET $4
	`, tenantID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Session
	for rows.Next() {
		s := &Session{}
		var retrievalConfig []byte
		if err := rows.Scan(
			&s.ID, &s.TenantID, &s.UserID, &s.AgentID, &s.Title, &s.KnowledgeBaseIDs, &retrievalConfig, &s.SimilarityThreshold, &s.TopK, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(retrievalConfig, &s.RetrievalConfig)
		result = append(result, s)
	}
	return result, rows.Err()
}

// ============================================================================
// ChunkRepository 实现 (Markdown 模式使用)
// ============================================================================

type pgChunkRepo struct {
	db *postgres.DB
}

// NewChunkRepository 创建文本块仓储
func NewChunkRepository(db *postgres.DB) ChunkRepository {
	return &pgChunkRepo{db: db}
}

func (r *pgChunkRepo) BatchCreate(ctx context.Context, chunks []*Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	for _, chunk := range chunks {
		if chunk.ID == "" {
			chunk.ID = fmt.Sprintf("%s_%d", chunk.KnowledgeID, chunk.ChunkIndex)
		}
		metadata, _ := json.Marshal(chunk.Metadata)

		err := r.db.QueryRow(ctx, `
			INSERT INTO chunks (knowledge_id, knowledge_base_id, chunk_index, content, content_length, content_hash, metadata)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET content = EXCLUDED.content, content_hash = EXCLUDED.content_hash, metadata = EXCLUDED.metadata
			RETURNING id, created_at
		`, chunk.KnowledgeID, chunk.KnowledgeBaseID, chunk.ChunkIndex, chunk.Content, len(chunk.Content), chunk.ContentHash, metadata).
			Scan(&chunk.ID, &chunk.CreatedAt)
		if err != nil {
			return fmt.Errorf("插入 chunk %d 失败: %w", chunk.ChunkIndex, err)
		}
	}
	return nil
}

func (r *pgChunkRepo) GetByKnowledgeID(ctx context.Context, knowledgeID string) ([]*Chunk, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, knowledge_id, knowledge_base_id, chunk_index, content, content_length, content_hash, metadata, created_at
		FROM chunks WHERE knowledge_id = $1
		ORDER BY chunk_index
	`, knowledgeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Chunk
	for rows.Next() {
		c := &Chunk{}
		var metaRaw []byte
		if err := rows.Scan(&c.ID, &c.KnowledgeID, &c.KnowledgeBaseID, &c.ChunkIndex, &c.Content, &c.ContentLength, &c.ContentHash, &metaRaw, &c.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metaRaw, &c.Metadata)
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *pgChunkRepo) DeleteByKnowledgeID(ctx context.Context, knowledgeID string) error {
	_, err := r.db.Pool().Exec(ctx, `DELETE FROM chunks WHERE knowledge_id = $1`, knowledgeID)
	return err
}

func (r *pgChunkRepo) GetHashesByKnowledgeID(ctx context.Context, knowledgeID string) (map[string]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, content_hash FROM chunks WHERE knowledge_id = $1 AND content_hash != ''
	`, knowledgeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string) // content_hash → chunk_id
	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return nil, err
		}
		result[hash] = id
	}
	return result, rows.Err()
}

func (r *pgChunkRepo) DeleteByIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.Pool().Exec(ctx, `DELETE FROM chunks WHERE id = ANY($1)`, ids)
	return err
}

func (r *pgSessionRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Pool().Exec(ctx, `UPDATE sessions SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1`, id)
	return err
}

func (r *pgSessionRepo) TouchUpdatedAt(ctx context.Context, id string) error {
	_, err := r.db.Pool().Exec(ctx, `UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}

// pgMessageRepo PostgreSQL 消息仓储实现
type pgMessageRepo struct {
	db *postgres.DB
}

// NewMessageRepository 创建消息仓储
func NewMessageRepository(db *postgres.DB) MessageRepository {
	return &pgMessageRepo{db: db}
}

func (r *pgMessageRepo) Create(ctx context.Context, m *Message) error {
	knowledgeRefs, _ := json.Marshal(m.KnowledgeReferences)
	agentSteps, _ := json.Marshal(m.AgentSteps)
	mentionedItems, _ := json.Marshal(m.MentionedItems)
	metadata, _ := json.Marshal(m.Metadata)

	return r.db.QueryRow(ctx, `
		INSERT INTO messages (session_id, role, content, knowledge_references, agent_steps, mentioned_items, tokens_used, latency_ms, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`, m.SessionID, m.Role, m.Content, knowledgeRefs, agentSteps, mentionedItems, m.TokensUsed, m.LatencyMs, metadata).
		Scan(&m.ID, &m.CreatedAt)
}

func (r *pgMessageRepo) ListBySession(ctx context.Context, sessionID string, limit int) ([]*Message, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, session_id, role, content, knowledge_references, agent_steps, mentioned_items, tokens_used, latency_ms, metadata, created_at
		FROM (
			SELECT id, session_id, role, content, knowledge_references, agent_steps, mentioned_items, tokens_used, latency_ms, metadata, created_at
			FROM messages
			WHERE session_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		) AS recent_messages
		ORDER BY created_at ASC
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Message
	for rows.Next() {
		m := &Message{}
		var knowledgeRefs, agentSteps, mentionedItems, metadata []byte
		if err := rows.Scan(
			&m.ID, &m.SessionID, &m.Role, &m.Content, &knowledgeRefs, &agentSteps, &mentionedItems, &m.TokensUsed, &m.LatencyMs, &metadata, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(knowledgeRefs, &m.KnowledgeReferences)
		_ = json.Unmarshal(agentSteps, &m.AgentSteps)
		_ = json.Unmarshal(mentionedItems, &m.MentionedItems)
		_ = json.Unmarshal(metadata, &m.Metadata)
		result = append(result, m)
	}
	return result, rows.Err()
}
