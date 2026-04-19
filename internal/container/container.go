// Package container 依赖注入容器
//
// 【Eino 特点】提供集中化的组件生命周期管理
// 参考 WeKnora 的 dig 容器，但使用更轻量的手动注入方式
package container

import (
	"context"
	"fmt"
	"sync"

	einoembedding "github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	einoretriever "github.com/cloudwego/eino/components/retriever"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/config"
)

// Container 依赖注入容器
// 管理所有组件的创建和生命周期
type Container struct {
	cfg *config.Config
	mu  sync.RWMutex

	// 核心组件（懒加载）
	chatModel           model.ChatModel
	embedding           einoembedding.Embedder
	embeddingDimensions int
	retriever           einoretriever.Retriever
	reranker            RerankerProvider
	vectorDB            VectorDBProvider
	retrievalCache      cachepkg.RetrievalCache

	// 清理函数
	cleanups []CleanupFunc
}

// CleanupFunc 清理函数类型
type CleanupFunc func(ctx context.Context) error

// RerankerProvider 重排序提供者接口
type RerankerProvider interface {
	Rerank(ctx context.Context, query string, docs []*Document, topK int) ([]*Document, error)
}

// VectorDBProvider 向量数据库提供者接口
type VectorDBProvider interface {
	Upsert(ctx context.Context, docs []*Document) error
	Search(ctx context.Context, vector []float32, topK int) ([]*Document, error)
	Delete(ctx context.Context, ids []string) error
	// DeleteByKnowledgeID 删除指定文档的所有向量 chunk
	DeleteByKnowledgeID(ctx context.Context, knowledgeID string) error
	// DeleteByKnowledgeBaseID 删除指定知识库的所有向量 chunk
	DeleteByKnowledgeBaseID(ctx context.Context, kbID string) error
	Close() error
}

// Document 文档
type Document struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Vector   []float32              `json:"vector,omitempty"`
	Score    float64                `json:"score,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// New 创建新的容器
func New(cfg *config.Config) *Container {
	return &Container{
		cfg:            cfg,
		cleanups:       make([]CleanupFunc, 0),
		retrievalCache: cachepkg.NewNoopRetrievalCache(),
	}
}

// SetRetrievalCache 设置检索链路缓存。
func (c *Container) SetRetrievalCache(retrievalCache cachepkg.RetrievalCache) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if retrievalCache == nil {
		c.retrievalCache = cachepkg.NewNoopRetrievalCache()
		return
	}
	c.retrievalCache = retrievalCache
	c.retriever = nil
}

// Config 获取配置
func (c *Container) Config() *config.Config {
	return c.cfg
}

// GetChatModel 获取 Eino 原生 ChatModel（懒加载）
func (c *Container) GetChatModel(ctx context.Context) (model.ChatModel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.chatModel != nil {
		return c.chatModel, nil
	}

	chatModel, cleanup, err := NewLLMProvider(ctx, &c.cfg.LLM)
	if err != nil {
		return nil, fmt.Errorf("创建 ChatModel 失败: %w", err)
	}

	c.chatModel = chatModel
	if cleanup != nil {
		c.cleanups = append(c.cleanups, cleanup)
	}

	return c.chatModel, nil
}

// GetEmbedding 获取 Eino 原生 Embedder（懒加载）
func (c *Container) GetEmbedding(ctx context.Context) (einoembedding.Embedder, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.embedding != nil {
		return c.embedding, nil
	}

	embedding, cleanup, err := NewEmbeddingProvider(ctx, &c.cfg.Embedding)
	if err != nil {
		return nil, fmt.Errorf("创建 Embedder 失败: %w", err)
	}

	c.embedding = embedding
	c.embeddingDimensions = c.cfg.Embedding.Dimensions
	if cleanup != nil {
		c.cleanups = append(c.cleanups, cleanup)
	}

	return c.embedding, nil
}

// GetRetriever 获取 Eino 原生 Retriever（懒加载）
func (c *Container) GetRetriever(ctx context.Context) (einoretriever.Retriever, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.retriever != nil {
		return c.retriever, nil
	}

	// 检索器需要 embedding 和 vectorDB（必须已经在外部初始化）
	if c.embedding == nil {
		return nil, fmt.Errorf("Embedding 尚未初始化，请先调用 GetEmbedding()")
	}
	if c.vectorDB == nil {
		return nil, fmt.Errorf("VectorDB 尚未初始化，请先调用 GetVectorDB()")
	}

	retriever, cleanup, err := NewRetrieverProvider(ctx, &c.cfg.RAG, &c.cfg.Embedding, c.embedding, c.vectorDB, c.retrievalCache)
	if err != nil {
		return nil, fmt.Errorf("创建 Retriever 失败: %w", err)
	}

	c.retriever = retriever
	if cleanup != nil {
		c.cleanups = append(c.cleanups, cleanup)
	}

	return c.retriever, nil
}

// GetReranker 获取重排序提供者（懒加载）
func (c *Container) GetReranker(ctx context.Context) (RerankerProvider, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.reranker != nil {
		return c.reranker, nil
	}

	if !c.cfg.Reranker.Enabled {
		return nil, nil // 返回 nil 表示未启用
	}

	reranker, cleanup, err := NewRerankerProvider(ctx, &c.cfg.Reranker)
	if err != nil {
		return nil, fmt.Errorf("创建 Reranker 失败: %w", err)
	}

	c.reranker = reranker
	if cleanup != nil {
		c.cleanups = append(c.cleanups, cleanup)
	}

	return c.reranker, nil
}

// GetVectorDB 获取向量数据库提供者（懒加载）
func (c *Container) GetVectorDB(ctx context.Context) (VectorDBProvider, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.vectorDB != nil {
		return c.vectorDB, nil
	}

	// 需要 embedding 来获取维度（此时 embedding 必须已经初始化，避免重入死锁）
	if c.embedding == nil {
		return nil, fmt.Errorf("Embedding 尚未初始化，请先调用 GetEmbedding()")
	}
	dimensions := c.embeddingDimensions
	if dimensions <= 0 {
		dimensions = c.cfg.Embedding.Dimensions
	}

	vectorDB, cleanup, err := NewVectorDBProvider(ctx, &c.cfg.Database, dimensions)
	if err != nil {
		return nil, fmt.Errorf("创建向量数据库失败: %w", err)
	}

	c.vectorDB = vectorDB
	if cleanup != nil {
		c.cleanups = append(c.cleanups, cleanup)
	}

	return c.vectorDB, nil
}

// GetEmbeddingDimensions 获取当前 Embedding 向量维度
func (c *Container) GetEmbeddingDimensions() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.embeddingDimensions > 0 {
		return c.embeddingDimensions
	}
	return c.cfg.Embedding.Dimensions
}

// Cleanup 清理所有资源
func (c *Container) Cleanup(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error
	for i := len(c.cleanups) - 1; i >= 0; i-- {
		if err := c.cleanups[i](ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("清理资源时发生 %d 个错误: %v", len(errs), errs)
	}

	return nil
}
