// Package container - Reranker 提供者
//
// 远程重排：Jina / Cohere / SiliconFlow(BGE) 使用相同的 rerank 请求/响应结构
// （model, query, documents -> results[{index, relevance_score}]），仅默认地址与路径不同。
// 本地重排：对候选集做 BM25 打分（英文按词、中日韩按字二元组切分），无需外部服务。
package container

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"eino_agent/internal/config"
)

type rerankPreset struct {
	baseURL string
	path    string
}

var rerankPresets = map[string]rerankPreset{
	"jina":        {baseURL: "https://api.jina.ai", path: "/v1/rerank"},
	"cohere":      {baseURL: "https://api.cohere.com", path: "/v2/rerank"},
	"bge":         {baseURL: "https://api.siliconflow.cn", path: "/v1/rerank"},
	"siliconflow": {baseURL: "https://api.siliconflow.cn", path: "/v1/rerank"},
}

// NewRerankerProvider 创建 Reranker 提供者；未知 provider 直接报错，不静默降级。
func NewRerankerProvider(_ context.Context, cfg *config.RerankerConfig) (RerankerProvider, CleanupFunc, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "local" || provider == "bm25" {
		return &LocalReranker{}, nil, nil
	}
	preset, ok := rerankPresets[provider]
	if !ok {
		return nil, nil, fmt.Errorf("不支持的 reranker provider %q（可选: jina, cohere, bge, siliconflow, local）", cfg.Provider)
	}
	endpoint, err := rerankEndpoint(cfg.BaseURL, preset)
	if err != nil {
		return nil, nil, err
	}
	return &APIReranker{
		cfg:      cfg,
		endpoint: endpoint,
		client:   &http.Client{Timeout: 30 * time.Second},
	}, nil, nil
}

// rerankEndpoint 拼接最终地址；兼容 base_url 已带版本前缀（如 https://host/v1）的写法。
func rerankEndpoint(baseURL string, preset rerankPreset) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = preset.baseURL
	}
	if strings.HasSuffix(base, "/rerank") {
		return base, nil
	}
	path := preset.path
	for _, version := range []string{"/v1", "/v2"} {
		if strings.HasSuffix(base, version) && strings.HasPrefix(path, version+"/") {
			path = strings.TrimPrefix(path, version)
			break
		}
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return "", fmt.Errorf("reranker base_url 无效: %q", baseURL)
	}
	return base + path, nil
}

// APIReranker 远程重排序器。
type APIReranker struct {
	cfg      *config.RerankerConfig
	endpoint string
	client   *http.Client
}

func (r *APIReranker) Rerank(ctx context.Context, query string, docs []*Document, topK int) ([]*Document, error) {
	if len(docs) == 0 {
		return docs, nil
	}
	passages := make([]string, len(docs))
	for i, doc := range docs {
		passages[i] = doc.Content
	}
	scores, err := r.callRerankAPI(ctx, query, passages)
	if err != nil {
		return nil, fmt.Errorf("重排序 API 调用失败: %w", err)
	}
	return rankByScores(docs, scores, topK), nil
}

func (r *APIReranker) callRerankAPI(ctx context.Context, query string, passages []string) ([]float64, error) {
	type rerankRequest struct {
		Model     string   `json:"model"`
		Query     string   `json:"query"`
		Documents []string `json:"documents"`
	}
	type rerankResponse struct {
		Results []struct {
			Index int     `json:"index"`
			Score float64 `json:"relevance_score"`
		} `json:"results"`
	}

	body, err := json.Marshal(rerankRequest{Model: r.cfg.ModelID, Query: query, Documents: passages})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.APIKey)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("API 返回错误 %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var parsed rerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("解析重排序响应失败: %w", err)
	}
	if len(parsed.Results) == 0 {
		return nil, fmt.Errorf("重排序响应为空")
	}
	scores := make([]float64, len(passages))
	for i := range scores {
		scores[i] = math.Inf(-1)
	}
	for _, item := range parsed.Results {
		if item.Index >= 0 && item.Index < len(scores) {
			scores[item.Index] = item.Score
		}
	}
	return scores, nil
}

// LocalReranker 在候选集内计算 BM25 分数进行重排（k1=1.2, b=0.75）。
type LocalReranker struct{}

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

func (r *LocalReranker) Rerank(_ context.Context, query string, docs []*Document, topK int) ([]*Document, error) {
	if len(docs) == 0 {
		return docs, nil
	}
	queryTerms := uniqueTerms(tokenizeForBM25(query))
	if len(queryTerms) == 0 {
		return rankByScores(docs, make([]float64, len(docs)), topK), nil
	}

	docTerms := make([]map[string]int, len(docs))
	lengths := make([]int, len(docs))
	df := make(map[string]int)
	totalLen := 0
	for i, doc := range docs {
		tokens := tokenizeForBM25(doc.Content)
		tf := make(map[string]int, len(tokens))
		for _, t := range tokens {
			tf[t]++
		}
		for t := range tf {
			df[t]++
		}
		docTerms[i] = tf
		lengths[i] = len(tokens)
		totalLen += len(tokens)
	}
	avgLen := float64(totalLen) / float64(len(docs))
	if avgLen == 0 {
		avgLen = 1
	}

	n := float64(len(docs))
	scores := make([]float64, len(docs))
	for i := range docs {
		var score float64
		for _, term := range queryTerms {
			freq := float64(docTerms[i][term])
			if freq == 0 {
				continue
			}
			idf := math.Log(1 + (n-float64(df[term])+0.5)/(float64(df[term])+0.5))
			norm := freq * (bm25K1 + 1) / (freq + bm25K1*(1-bm25B+bm25B*float64(lengths[i])/avgLen))
			score += idf * norm
		}
		scores[i] = score
	}
	return rankByScores(docs, scores, topK), nil
}

// tokenizeForBM25 英文/数字按连续字母数字切词并小写；CJK 字符按相邻二元组切分（单字文本保留单字）。
func tokenizeForBM25(text string) []string {
	var (
		tokens []string
		word   []rune
		cjk    []rune
	)
	flushWord := func() {
		if len(word) > 0 {
			tokens = append(tokens, strings.ToLower(string(word)))
			word = word[:0]
		}
	}
	flushCJK := func() {
		switch {
		case len(cjk) == 1:
			tokens = append(tokens, string(cjk))
		case len(cjk) > 1:
			for i := 0; i+1 < len(cjk); i++ {
				tokens = append(tokens, string(cjk[i:i+2]))
			}
		}
		cjk = cjk[:0]
	}
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul):
			flushWord()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			flushCJK()
			word = append(word, r)
		default:
			flushWord()
			flushCJK()
		}
	}
	flushWord()
	flushCJK()
	return tokens
}

func uniqueTerms(tokens []string) []string {
	seen := make(map[string]struct{}, len(tokens))
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func rankByScores(docs []*Document, scores []float64, topK int) []*Document {
	idx := make([]int, len(docs))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return scores[idx[a]] > scores[idx[b]] })
	if topK <= 0 || topK > len(idx) {
		topK = len(idx)
	}
	out := make([]*Document, 0, topK)
	for _, i := range idx[:topK] {
		if math.IsInf(scores[i], -1) {
			continue
		}
		d := docs[i]
		out = append(out, &Document{ID: d.ID, Content: d.Content, Score: scores[i], Metadata: d.Metadata})
	}
	return out
}
