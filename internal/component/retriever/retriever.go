// Package retriever 提供检索器的抽象和实现
// 参考 WeKnora 的混合检索策略，支持向量检索、关键词检索、图谱检索
package retriever

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// SearchResult 检索结果
type SearchResult struct {
	Document  *schema.Document // 文档内容
	Score     float64          // 相关性分数
	MatchType MatchType        // 匹配类型
	Metadata  map[string]any   // 元数据
}

// MatchType 匹配类型
type MatchType string

const (
	MatchTypeVector  MatchType = "vector"   // 向量匹配
	MatchTypeKeyword MatchType = "keyword"  // 关键词匹配
	MatchTypeGraph   MatchType = "graph"    // 图谱匹配
	MatchTypeHybrid  MatchType = "hybrid"   // 混合匹配
)

// SearchParams 检索参数
type SearchParams struct {
	Query            string   // 查询文本
	KnowledgeBaseIDs []string // 知识库 ID 列表
	TopK             int      // 返回数量
	VectorThreshold  float64  // 向量相似度阈值
	KeywordThreshold float64  // 关键词匹配阈值
	EnableRewrite    bool     // 是否启用查询改写
}

// HybridRetriever 混合检索器接口
// 【Eino 特点】实现 Eino 的 retriever.Retriever 接口，可以直接用于 Graph 编排
type HybridRetriever interface {
	// Retrieve 执行混合检索
	Retrieve(ctx context.Context, query string, opts ...Option) ([]*SearchResult, error)

	// RetrieveWithParams 使用详细参数检索
	RetrieveWithParams(ctx context.Context, params *SearchParams) ([]*SearchResult, error)
}

// Option 检索选项
type Option func(*searchOptions)

type searchOptions struct {
	topK             int
	vectorThreshold  float64
	keywordThreshold float64
	knowledgeBaseIDs []string
}

// WithTopK 设置返回数量
func WithTopK(k int) Option {
	return func(o *searchOptions) {
		o.topK = k
	}
}

// WithVectorThreshold 设置向量阈值
func WithVectorThreshold(threshold float64) Option {
	return func(o *searchOptions) {
		o.vectorThreshold = threshold
	}
}

// WithKeywordThreshold 设置关键词阈值
func WithKeywordThreshold(threshold float64) Option {
	return func(o *searchOptions) {
		o.keywordThreshold = threshold
	}
}

// WithKnowledgeBases 设置知识库
func WithKnowledgeBases(ids ...string) Option {
	return func(o *searchOptions) {
		o.knowledgeBaseIDs = ids
	}
}

func defaultOptions() *searchOptions {
	return &searchOptions{
		topK:             10,
		vectorThreshold:  0.7,
		keywordThreshold: 0.5,
	}
}

func applyOptions(opts ...Option) *searchOptions {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}
