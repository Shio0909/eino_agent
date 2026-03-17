// Package retriever 提供检索器实现
package retriever

import (
	"context"
	"sort"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// HybridRetrieverImpl 混合检索器实现
// 【Eino 特点】实现 Eino 的 retriever.Retriever 接口，可直接嵌入 Graph/Chain
type HybridRetrieverImpl struct {
	vectorRetriever  retriever.Retriever // 向量检索器
	keywordRetriever retriever.Retriever // 关键词检索器（可选）
	graphRetriever   retriever.Retriever // 图谱检索器（可选）

	// RRF 融合参数
	rrfK int // RRF 常数，默认 60
}

// HybridConfig 混合检索器配置
type HybridConfig struct {
	VectorRetriever  retriever.Retriever
	KeywordRetriever retriever.Retriever
	GraphRetriever   retriever.Retriever
	RRFK             int
}

// NewHybridRetriever 创建混合检索器
func NewHybridRetriever(config *HybridConfig) *HybridRetrieverImpl {
	rrfK := config.RRFK
	if rrfK <= 0 {
		rrfK = 60 // 默认 RRF 常数
	}

	return &HybridRetrieverImpl{
		vectorRetriever:  config.VectorRetriever,
		keywordRetriever: config.KeywordRetriever,
		graphRetriever:   config.GraphRetriever,
		rrfK:             rrfK,
	}
}

// Retrieve 实现 Eino retriever.Retriever 接口
// 【Eino 特点】这个方法让 HybridRetriever 可以直接用于 Graph.AddRetrieverNode()
func (h *HybridRetrieverImpl) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	results, err := h.RetrieveWithScore(ctx, query, opts...)
	if err != nil {
		return nil, err
	}

	// 转换为 Document 列表
	docs := make([]*schema.Document, 0, len(results))
	for _, r := range results {
		docs = append(docs, r.Document)
	}
	return docs, nil
}

// RetrieveWithScore 带分数的检索
func (h *HybridRetrieverImpl) RetrieveWithScore(ctx context.Context, query string, opts ...retriever.Option) ([]*SearchResult, error) {
	// 并行执行多种检索
	type retrieverResult struct {
		results []*schema.Document
		typ     MatchType
		err     error
	}

	resultsChan := make(chan retrieverResult, 3)

	// 向量检索
	if h.vectorRetriever != nil {
		go func() {
			docs, err := h.vectorRetriever.Retrieve(ctx, query, opts...)
			resultsChan <- retrieverResult{results: docs, typ: MatchTypeVector, err: err}
		}()
	} else {
		resultsChan <- retrieverResult{}
	}

	// 关键词检索
	if h.keywordRetriever != nil {
		go func() {
			docs, err := h.keywordRetriever.Retrieve(ctx, query, opts...)
			resultsChan <- retrieverResult{results: docs, typ: MatchTypeKeyword, err: err}
		}()
	} else {
		resultsChan <- retrieverResult{}
	}

	// 图谱检索
	if h.graphRetriever != nil {
		go func() {
			docs, err := h.graphRetriever.Retrieve(ctx, query, opts...)
			resultsChan <- retrieverResult{results: docs, typ: MatchTypeGraph, err: err}
		}()
	} else {
		resultsChan <- retrieverResult{}
	}

	// 收集结果
	allResults := make(map[MatchType][]*schema.Document)
	for i := 0; i < 3; i++ {
		r := <-resultsChan
		if r.err == nil && len(r.results) > 0 {
			allResults[r.typ] = r.results
		}
	}

	// RRF 融合
	return h.rrfFusion(allResults), nil
}

// rrfFusion 使用 RRF (Reciprocal Rank Fusion) 算法融合多路检索结果
// 参考 WeKnora 的实现：score = Σ 1/(k + rank_i)
func (h *HybridRetrieverImpl) rrfFusion(results map[MatchType][]*schema.Document) []*SearchResult {
	// 计算每个文档的 RRF 分数
	docScores := make(map[string]*SearchResult)

	for matchType, docs := range results {
		for rank, doc := range docs {
			id := doc.ID
			if id == "" {
				continue
			}

			score := 1.0 / float64(h.rrfK+rank+1)

			if existing, ok := docScores[id]; ok {
				existing.Score += score
				existing.MatchType = MatchTypeHybrid // 多路命中
			} else {
				docScores[id] = &SearchResult{
					Document:  doc,
					Score:     score,
					MatchType: matchType,
					Metadata:  make(map[string]any),
				}
			}
		}
	}

	// 转为列表并排序
	results_list := make([]*SearchResult, 0, len(docScores))
	for _, r := range docScores {
		results_list = append(results_list, r)
	}

	sort.Slice(results_list, func(i, j int) bool {
		return results_list[i].Score > results_list[j].Score
	})

	return results_list
}

// GetType 返回组件类型（Eino 接口要求）
func (h *HybridRetrieverImpl) GetType() string {
	return "HybridRetriever"
}

// IsCallbacksEnabled 是否启用回调（Eino 接口要求）
func (h *HybridRetrieverImpl) IsCallbacksEnabled() bool {
	return true
}
