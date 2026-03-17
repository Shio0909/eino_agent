package cache

import (
	"context"
	"time"
)

// SessionMessage 表示缓存中的会话消息快照。
type SessionMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// RetrievalDocument 表示缓存中的检索文档快照。
type RetrievalDocument struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Score    float64        `json:"score,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// RetrievalResult 表示缓存中的检索结果。
type RetrievalResult struct {
	DocIDs    []string            `json:"doc_ids"`
	Scores    []float64           `json:"scores,omitempty"`
	Documents []RetrievalDocument `json:"documents,omitempty"`
	CachedAt  time.Time           `json:"cached_at"`
	ExpiresAt *time.Time          `json:"expires_at,omitempty"`
}

// ImportTaskState 表示导入任务的实时状态。
type ImportTaskState struct {
	Status     string    `json:"status"`
	Stage      string    `json:"stage,omitempty"`
	ChunkCount int       `json:"chunk_count,omitempty"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

// SessionCache 定义会话热数据缓存能力。
type SessionCache interface {
	GetRecentMessages(ctx context.Context, sessionID string, limit int) ([]SessionMessage, bool, error)
	SetRecentMessages(ctx context.Context, sessionID string, messages []SessionMessage, ttl time.Duration) error
	GetSummary(ctx context.Context, sessionID string) (string, bool, error)
	SetSummary(ctx context.Context, sessionID, summary string, ttl time.Duration) error
	InvalidateSession(ctx context.Context, sessionID string) error
}

// RetrievalCache 定义检索链路缓存能力。
type RetrievalCache interface {
	GetEmbedding(ctx context.Context, modelID, queryHash string) ([]float32, bool, error)
	SetEmbedding(ctx context.Context, modelID, queryHash string, vector []float32, ttl time.Duration) error
	GetRetrievalResult(ctx context.Context, cacheKey string) (*RetrievalResult, bool, error)
	SetRetrievalResult(ctx context.Context, cacheKey string, result *RetrievalResult, ttl time.Duration) error
	InvalidateKnowledgeBase(ctx context.Context, knowledgeBaseID string) error
}

// ImportStateStore 定义异步导入任务实时状态存储能力。
type ImportStateStore interface {
	GetTaskState(ctx context.Context, taskID string) (*ImportTaskState, bool, error)
	SetTaskState(ctx context.Context, taskID string, state *ImportTaskState, ttl time.Duration) error
	DeleteTaskState(ctx context.Context, taskID string) error
}

// Lock 表示一个分布式锁句柄。
type Lock interface {
	Release(ctx context.Context) error
}

// Locker 定义分布式锁能力。
type Locker interface {
	TryLock(ctx context.Context, key string, ttl time.Duration) (Lock, bool, error)
}
