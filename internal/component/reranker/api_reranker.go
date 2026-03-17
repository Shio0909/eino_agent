// Package reranker 提供重排序器实现
package reranker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
)

// APIReranker 基于 API 的重排序器
// 兼容 Jina、Cohere 等重排序 API
type APIReranker struct {
	config *RerankerConfig
	client *http.Client
}

// NewAPIReranker 创建 API 重排序器
func NewAPIReranker(config *RerankerConfig) *APIReranker {
	return &APIReranker{
		config: config,
		client: &http.Client{},
	}
}

// Rerank 执行重排序
func (r *APIReranker) Rerank(ctx context.Context, query string, passages []string) ([]RankResult, error) {
	if len(passages) == 0 {
		return nil, nil
	}

	// 构建请求
	reqBody := map[string]any{
		"model":     r.config.ModelID,
		"query":     query,
		"documents": passages,
		"top_n":     r.config.TopK,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.config.BaseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if r.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.config.APIKey)
	}

	// 发送请求
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rerank API error: %s - %s", resp.Status, string(body))
	}

	// 解析响应
	var result struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 转换结果并过滤
	rankResults := make([]RankResult, 0, len(result.Results))
	for _, r := range result.Results {
		rankResults = append(rankResults, RankResult{
			Index:          r.Index,
			RelevanceScore: r.RelevanceScore,
		})
	}

	// 按分数排序
	sort.Slice(rankResults, func(i, j int) bool {
		return rankResults[i].RelevanceScore > rankResults[j].RelevanceScore
	})

	// 应用阈值过滤
	if r.config.Threshold > 0 {
		filtered := make([]RankResult, 0)
		for _, rr := range rankResults {
			if rr.RelevanceScore >= r.config.Threshold {
				filtered = append(filtered, rr)
			}
		}
		rankResults = filtered
	}

	return rankResults, nil
}

// LocalReranker 本地重排序器（基于简单的相似度计算）
// 用于没有 Rerank API 时的 fallback
type LocalReranker struct {
	config *RerankerConfig
}

// NewLocalReranker 创建本地重排序器
func NewLocalReranker(config *RerankerConfig) *LocalReranker {
	return &LocalReranker{config: config}
}

// Rerank 本地重排序（保持原顺序，仅作为 fallback）
func (r *LocalReranker) Rerank(ctx context.Context, query string, passages []string) ([]RankResult, error) {
	results := make([]RankResult, len(passages))
	for i := range passages {
		// 简单的衰减分数，保持原有顺序
		results[i] = RankResult{
			Index:          i,
			RelevanceScore: 1.0 - float64(i)*0.05,
		}
	}
	return results, nil
}
