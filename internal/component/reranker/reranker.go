// Package reranker 提供重排序器的抽象和实现
// 参考 WeKnora 的 Rerank 实现，支持多种重排序模型
package reranker

import (
	"context"
)

// RankResult 重排序结果
type RankResult struct {
	Index          int     // 原始索引
	RelevanceScore float64 // 相关性分数
}

// Reranker 重排序器接口
type Reranker interface {
	// Rerank 对候选文档进行重排序
	Rerank(ctx context.Context, query string, passages []string) ([]RankResult, error)
}

// RerankerConfig 重排序器配置
type RerankerConfig struct {
	ModelID   string  // 模型 ID
	BaseURL   string  // API 地址
	APIKey    string  // API 密钥
	Threshold float64 // 过滤阈值
	TopK      int     // 返回数量
}
