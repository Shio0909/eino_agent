// Package container - Reranker 提供者实现
//
// 【Eino 特点】支持 API 和本地重排序
package container

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"eino_agent/internal/config"
)

// APIReranker API 重排序器
type APIReranker struct {
	cfg    *config.RerankerConfig
	client *http.Client
}

// NewRerankerProvider 创建 Reranker 提供者
func NewRerankerProvider(ctx context.Context, cfg *config.RerankerConfig) (RerankerProvider, CleanupFunc, error) {
	switch cfg.Provider {
	case "jina", "cohere", "bge":
		return newAPIReranker(ctx, cfg)
	case "local":
		return newLocalReranker(ctx, cfg)
	default:
		return newAPIReranker(ctx, cfg)
	}
}

// newAPIReranker 创建 API 重排序器
func newAPIReranker(ctx context.Context, cfg *config.RerankerConfig) (*APIReranker, CleanupFunc, error) {
	reranker := &APIReranker{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	return reranker, nil, nil
}

// Rerank 执行重排序
func (r *APIReranker) Rerank(ctx context.Context, query string, docs []*Document, topK int) ([]*Document, error) {
	if len(docs) == 0 {
		return docs, nil
	}

	// 准备文档内容
	passages := make([]string, len(docs))
	for i, doc := range docs {
		passages[i] = doc.Content
	}

	// 调用重排序 API
	scores, err := r.callRerankAPI(ctx, query, passages)
	if err != nil {
		return nil, fmt.Errorf("重排序 API 调用失败: %w", err)
	}

	// 更新分数并排序
	type scoredDoc struct {
		doc   *Document
		score float64
	}
	scored := make([]scoredDoc, len(docs))
	for i, doc := range docs {
		scored[i] = scoredDoc{
			doc:   doc,
			score: scores[i],
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// 返回 TopK
	if topK > len(scored) {
		topK = len(scored)
	}

	result := make([]*Document, topK)
	for i := 0; i < topK; i++ {
		result[i] = &Document{
			ID:       scored[i].doc.ID,
			Content:  scored[i].doc.Content,
			Score:    scored[i].score,
			Metadata: scored[i].doc.Metadata,
		}
	}

	return result, nil
}

// callRerankAPI 调用重排序 API
func (r *APIReranker) callRerankAPI(ctx context.Context, query string, passages []string) ([]float64, error) {
	// Jina Reranker API 格式
	type rerankRequest struct {
		Model  string   `json:"model"`
		Query  string   `json:"query"`
		Documents []string `json:"documents"`
		TopN   int      `json:"top_n,omitempty"`
	}

	type rerankResult struct {
		Index float64 `json:"index"`
		Score float64 `json:"relevance_score"`
	}

	type rerankResponse struct {
		Results []rerankResult `json:"results"`
	}

	// 构建请求
	reqBody := rerankRequest{
		Model:     r.cfg.ModelID,
		Query:     query,
		Documents: passages,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// 发送请求
	req, err := http.NewRequestWithContext(ctx, "POST", r.cfg.BaseURL+"/v1/rerank", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.cfg.APIKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result rerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// 按原始顺序返回分数
	scores := make([]float64, len(passages))
	for _, r := range result.Results {
		idx := int(r.Index)
		if idx >= 0 && idx < len(scores) {
			scores[idx] = r.Score
		}
	}

	return scores, nil
}

// LocalReranker 本地重排序器
// 使用简单的 BM25 或 TF-IDF 算法
type LocalReranker struct {
	cfg *config.RerankerConfig
}

func newLocalReranker(ctx context.Context, cfg *config.RerankerConfig) (*LocalReranker, CleanupFunc, error) {
	return &LocalReranker{cfg: cfg}, nil, nil
}

// Rerank 本地重排序（简化实现，使用字符串匹配）
func (r *LocalReranker) Rerank(ctx context.Context, query string, docs []*Document, topK int) ([]*Document, error) {
	if len(docs) == 0 {
		return docs, nil
	}

	// 简单的关键词匹配打分
	type scoredDoc struct {
		doc   *Document
		score float64
	}

	scored := make([]scoredDoc, len(docs))
	queryRunes := []rune(query)

	for i, doc := range docs {
		// 简单的包含度评分
		contentRunes := []rune(doc.Content)
		matchCount := 0
		for _, qr := range queryRunes {
			for _, cr := range contentRunes {
				if qr == cr {
					matchCount++
					break
				}
			}
		}
		score := float64(matchCount) / float64(len(queryRunes))
		// 结合原始分数
		scored[i] = scoredDoc{
			doc:   doc,
			score: (score + doc.Score) / 2,
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if topK > len(scored) {
		topK = len(scored)
	}

	result := make([]*Document, topK)
	for i := 0; i < topK; i++ {
		result[i] = &Document{
			ID:       scored[i].doc.ID,
			Content:  scored[i].doc.Content,
			Score:    scored[i].score,
			Metadata: scored[i].doc.Metadata,
		}
	}

	return result, nil
}
