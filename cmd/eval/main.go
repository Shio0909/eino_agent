package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type EvalSample struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	GoldDocs []string `json:"gold_docs,omitempty"`
	Category string   `json:"category,omitempty"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
}

type chatRequest struct {
	Message          string   `json:"message"`
	Mode             string   `json:"mode,omitempty"`
	UseAgent         bool     `json:"use_agent,omitempty"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
}

type reference struct {
	ID      string `json:"id"`
	Source  string `json:"source,omitempty"`
	Content string `json:"content,omitempty"`
}

type source struct {
	ID    string `json:"id,omitempty"`
	DocID string `json:"doc_id,omitempty"`
}

type chatResponse struct {
	Answer     string      `json:"answer"`
	References []reference `json:"references,omitempty"`
	Sources    []source    `json:"sources,omitempty"`
	LatencyMs  int64       `json:"latency_ms"`
}

type sampleResult struct {
	ID                string
	Category          string
	LatencyMs         int64
	Recall            float64
	Precision         float64
	Hit               float64
	MRR               float64
	NDCG             float64
	RetrievalLabeled bool
	Error             string
}

func main() {
	input := flag.String("input", "data/eval_set.jsonl", "评测集文件（jsonl）")
	mode := flag.String("mode", "pipeline", "评测模式: pipeline|agent|agentic_rag")
	baseURL := flag.String("base-url", "http://localhost:8080", "服务地址")
	kbIDsFlag := flag.String("knowledge-base-ids", "", "知识库 ID，多个用逗号分隔（可选）")
	token := flag.String("token", "", "Bearer token（可选）")
	username := flag.String("username", "", "登录用户名（未传 token 时可用）")
	password := flag.String("password", "", "登录密码（未传 token 时可用）")
	timeoutSec := flag.Int("timeout", 60, "单请求超时（秒）")
	reportPath := flag.String("report", "", "报告输出路径（默认为 docs/eval_reports/<timestamp>.md）")
	strategy := flag.String("strategy", "", "检索策略: vector|hybrid|hybrid_rerank|full (通过 API 自动切换)")
	noRAG := flag.Bool("no-rag", false, "No-RAG baseline：传入不存在的知识库 ID，LLM 纯靠自身知识回答")
	flag.Parse()

	samples, err := loadSamples(*input)
	if err != nil {
		fmt.Printf("[eval] 读取评测集失败: %v\n", err)
		os.Exit(1)
	}

	if len(samples) == 0 {
		fmt.Println("[eval] 评测集为空")
		os.Exit(1)
	}

	client := &http.Client{Timeout: time.Duration(*timeoutSec) * time.Second}
	knowledgeBaseIDs := parseCSVList(*kbIDsFlag)
	if *noRAG {
		// 使用不存在的 KB ID，scoped retriever 会过滤掉所有文档
		knowledgeBaseIDs = []string{"00000000-0000-0000-0000-000000000000"}
		fmt.Println("[eval] No-RAG baseline 模式：LLM 将不依赖检索结果回答")
	}
	bearer := strings.TrimSpace(*token)
	if bearer == "" && *username != "" && *password != "" {
		bearer, err = login(client, strings.TrimRight(*baseURL, "/"), *username, *password)
		if err != nil {
			fmt.Printf("[eval] 登录失败: %v\n", err)
			os.Exit(1)
		}
	}

	strategyName := *strategy
	if strategyName != "" {
		if err := applyStrategy(client, strings.TrimRight(*baseURL, "/"), bearer, strategyName); err != nil {
			fmt.Printf("[eval] 切换策略失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[eval] 检索策略已切换为: %s\n", strategyName)
	}

	// 如果是 agentic_rag 模式，自动启用
	if *mode == "agentic_rag" {
		if err := setAgenticRAG(client, strings.TrimRight(*baseURL, "/"), bearer, true); err != nil {
			fmt.Printf("[eval] 启用 agentic_rag 失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[eval] agentic_rag 已启用")
	}

	results := make([]sampleResult, 0, len(samples))
	for _, sample := range samples {
		res := runSample(client, strings.TrimRight(*baseURL, "/"), bearer, *mode, sample, knowledgeBaseIDs)
		results = append(results, res)
		if res.Error != "" {
			fmt.Printf("[eval] %s error: %s\n", sample.ID, res.Error)
		} else {
			fmt.Printf("[eval] %s ok latency=%dms recall=%.3f hit=%.0f\n", sample.ID, res.LatencyMs, res.Recall, res.Hit)
		}
	}

	reportStrategy := strategyName
	if *noRAG {
		reportStrategy = "no-rag"
	}
	report := buildReport(*mode, reportStrategy, *baseURL, results)
	out := *reportPath
	if strings.TrimSpace(out) == "" {
		out = filepath.Join("docs", "eval_reports", time.Now().Format("20060102_150405")+".md")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Printf("[eval] 创建报告目录失败: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, []byte(report), 0o644); err != nil {
		fmt.Printf("[eval] 写入报告失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("========================================")
	fmt.Println("Eino RAG Eval")
	fmt.Println("========================================")
	fmt.Printf("mode: %s\n", *mode)
	if strategyName != "" {
		fmt.Printf("strategy: %s\n", strategyName)
	}
	fmt.Printf("samples: %d\n", len(samples))
	fmt.Printf("report: %s\n", out)
}

func loadSamples(path string) ([]EvalSample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	results := make([]EvalSample, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimPrefix(line, "\ufeff")
		if line == "" {
			continue
		}
		var s EvalSample
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			continue
		}
		if s.ID == "" {
			s.ID = fmt.Sprintf("sample_%d", len(results)+1)
		}
		if s.Category == "" {
			s.Category = "uncategorized"
		}
		results = append(results, s)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func login(client *http.Client, baseURL, username, password string) (string, error) {
	body, _ := json.Marshal(loginRequest{Username: username, Password: password})
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var lr loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return "", err
	}
	if lr.AccessToken == "" {
		return "", fmt.Errorf("登录响应缺少 access_token")
	}
	return lr.AccessToken, nil
}

// applyStrategy 通过 API 切换检索策略
func applyStrategy(client *http.Client, baseURL, bearer, strategy string) error {
	type ragSettings struct {
		EnableHybrid  *bool `json:"enable_hybrid,omitempty"`
		EnableRewrite *bool `json:"enable_rewrite,omitempty"`
		EnableRerank  *bool `json:"enable_rerank,omitempty"`
	}
	type settingsReq struct {
		RAG *ragSettings `json:"rag"`
	}

	boolPtr := func(v bool) *bool { return &v }

	var s settingsReq
	switch strategy {
	case "vector":
		s.RAG = &ragSettings{EnableHybrid: boolPtr(false), EnableRewrite: boolPtr(false), EnableRerank: boolPtr(false)}
	case "hybrid":
		s.RAG = &ragSettings{EnableHybrid: boolPtr(true), EnableRewrite: boolPtr(false), EnableRerank: boolPtr(false)}
	case "hybrid_rerank":
		s.RAG = &ragSettings{EnableHybrid: boolPtr(true), EnableRewrite: boolPtr(false), EnableRerank: boolPtr(true)}
	case "full":
		s.RAG = &ragSettings{EnableHybrid: boolPtr(true), EnableRewrite: boolPtr(true), EnableRerank: boolPtr(true)}
	default:
		return fmt.Errorf("未知策略: %s (可选: vector|hybrid|hybrid_rerank|full)", strategy)
	}

	body, _ := json.Marshal(s)
	req, err := http.NewRequest(http.MethodPut, baseURL+"/api/v1/settings", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

func setAgenticRAG(client *http.Client, baseURL, bearer string, enabled bool) error {
	type agenticRAGSettings struct {
		Enabled *bool `json:"enabled"`
	}
	type settingsReq struct {
		AgenticRAG *agenticRAGSettings `json:"agentic_rag"`
	}
	s := settingsReq{AgenticRAG: &agenticRAGSettings{Enabled: &enabled}}
	body, _ := json.Marshal(s)
	req, err := http.NewRequest(http.MethodPut, baseURL+"/api/v1/settings", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

func runSample(client *http.Client, baseURL, bearer, mode string, sample EvalSample, knowledgeBaseIDs []string) sampleResult {
	result := sampleResult{ID: sample.ID, Category: sample.Category}
	request := chatRequest{Message: sample.Question, KnowledgeBaseIDs: knowledgeBaseIDs}
	switch mode {
	case "agent":
		request.UseAgent = true
	case "agentic_rag":
		request.Mode = "agentic_rag"
	default:
		request.Mode = "pipeline"
	}
	body, _ := json.Marshal(request)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/chat", bytes.NewReader(body))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		result.Error = fmt.Sprintf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
		return result
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		result.Error = err.Error()
		return result
	}

	latency := cr.LatencyMs
	if latency <= 0 {
		latency = time.Since(started).Milliseconds()
	}
	result.LatencyMs = latency
	refs := make([]reference, 0, len(cr.References)+len(cr.Sources))
	refs = append(refs, cr.References...)
	for _, src := range cr.Sources {
		id := strings.TrimSpace(src.ID)
		if id == "" {
			id = strings.TrimSpace(src.DocID)
		}
		if id != "" {
			refs = append(refs, reference{ID: id})
		}
	}

	result.Recall, result.Precision, result.Hit, result.MRR, result.NDCG, result.RetrievalLabeled = retrievalMetrics(sample.GoldDocs, refs)
	return result
}

func retrievalMetrics(goldDocs []string, refs []reference) (recall, precision, hit, mrr, ndcg float64, labeled bool) {
	if len(goldDocs) == 0 {
		return 0, 0, 0, 0, 0, false
	}
	labeled = true
	goldSet := make(map[string]struct{}, len(goldDocs))
	for _, raw := range goldDocs {
		key := normalizeForMatch(raw)
		if key != "" {
			goldSet[key] = struct{}{}
		}
	}
	if len(goldSet) == 0 {
		return 0, 0, 0, 0, 0, false
	}

	retrieved := make([]reference, 0, len(refs))
	seen := map[string]struct{}{}
	for _, ref := range refs {
		key := referenceKey(ref)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		retrieved = append(retrieved, ref)
	}
	if len(retrieved) == 0 {
		return 0, 0, 0, 0, 0, true
	}

	relevantRetrieved := 0
	firstRank := -1
	relList := make([]int, 0, len(retrieved))
	matchedGold := map[string]struct{}{}
	for idx, ref := range retrieved {
		matched := matchGoldToReference(goldSet, ref)
		hasNewGold := false
		for _, g := range matched {
			if _, ok := matchedGold[g]; !ok {
				hasNewGold = true
				break
			}
		}
		if hasNewGold {
			relevantRetrieved++
			relList = append(relList, 1)
			if firstRank == -1 {
				firstRank = idx + 1
			}
			for _, g := range matched {
				matchedGold[g] = struct{}{}
			}
		} else {
			relList = append(relList, 0)
		}
	}

	recall = float64(len(matchedGold)) / float64(len(goldSet))
	precision = float64(relevantRetrieved) / float64(len(retrieved))
	if relevantRetrieved > 0 {
		hit = 1
		mrr = 1.0 / float64(firstRank)
	}
	ndcg = calcNDCG(relList, len(goldSet))
	return recall, precision, hit, mrr, ndcg, true
}

func normalizeForMatch(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func referenceKey(ref reference) string {
	id := normalizeForMatch(ref.ID)
	source := normalizeForMatch(ref.Source)
	if id != "" {
		return id
	}
	if source != "" {
		return source
	}
	content := normalizeForMatch(ref.Content)
	if content == "" {
		return ""
	}
	if len(content) > 256 {
		return content[:256]
	}
	return content
}

func matchGoldToReference(goldSet map[string]struct{}, ref reference) []string {
	targets := []string{
		normalizeForMatch(ref.ID),
		normalizeForMatch(ref.Source),
		normalizeForMatch(ref.Content),
	}
	matches := make([]string, 0)
	for gold := range goldSet {
		for _, t := range targets {
			if t == "" {
				continue
			}
			if t == gold || strings.Contains(t, gold) {
				matches = append(matches, gold)
				break
			}
		}
	}
	return matches
}



func parseCSVList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildReport(mode, strategy, baseURL string, results []sampleResult) string {
	latencies := make([]int64, 0, len(results))
	var recallSum, precisionSum, hitSum, mrrSum, ndcgSum float64
	errCount := 0
	retrievalLabeled := 0

	for _, r := range results {
		if r.Error != "" {
			errCount++
			continue
		}
		latencies = append(latencies, r.LatencyMs)
		if r.RetrievalLabeled {
			retrievalLabeled++
			recallSum += r.Recall
			precisionSum += r.Precision
			hitSum += r.Hit
			mrrSum += r.MRR
			ndcgSum += r.NDCG
		}
	}

	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)
	avgLatency := avgInt64(latencies)
	denom := float64(max(1, len(results)-errCount))
	retrievalDenom := float64(max(1, retrievalLabeled))
	recallAtK := recallSum / denom
	precisionAtK := precisionSum / denom
	hitAtK := hitSum / denom
	mrrAtK := mrrSum / denom
	ndcgAtK := ndcgSum / denom
	if retrievalLabeled > 0 {
		recallAtK = recallSum / retrievalDenom
		precisionAtK = precisionSum / retrievalDenom
		hitAtK = hitSum / retrievalDenom
		mrrAtK = mrrSum / retrievalDenom
		ndcgAtK = ndcgSum / retrievalDenom
	}
	errorRate := float64(errCount) / float64(max(1, len(results)))

	var b strings.Builder
	b.WriteString("# Eino RAG 评测报告\n\n")
	b.WriteString(fmt.Sprintf("- 时间: %s\n", time.Now().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- 模式: %s\n", mode))
	if strategy != "" {
		b.WriteString(fmt.Sprintf("- 策略: %s\n", strategy))
	}
	b.WriteString(fmt.Sprintf("- 服务: %s\n", strings.TrimRight(baseURL, "/")))
	b.WriteString(fmt.Sprintf("- 样本数: %d\n\n", len(results)))

	b.WriteString("## 总体指标\n\n")
	b.WriteString(fmt.Sprintf("- Recall@K: %.4f\n", recallAtK))
	b.WriteString(fmt.Sprintf("- Precision@K: %.4f\n", precisionAtK))
	b.WriteString(fmt.Sprintf("- Hit@K: %.4f\n", hitAtK))
	b.WriteString(fmt.Sprintf("- MRR@K: %.4f\n", mrrAtK))
	b.WriteString(fmt.Sprintf("- nDCG@K: %.4f\n", ndcgAtK))
	b.WriteString(fmt.Sprintf("- Avg Latency (ms): %.2f\n", avgLatency))
	b.WriteString(fmt.Sprintf("- P50 Latency (ms): %d\n", p50))
	b.WriteString(fmt.Sprintf("- P95 Latency (ms): %d\n", p95))
	b.WriteString(fmt.Sprintf("- Error Rate: %.4f\n", errorRate))
	b.WriteString(fmt.Sprintf("- Retrieval标注样本: %d\n", retrievalLabeled))
	b.WriteString(fmt.Sprintf("- Retrieval标注覆盖率: %.2f%%\n\n", 100*float64(retrievalLabeled)/float64(max(1, len(results)-errCount))))

	if retrievalLabeled == 0 {
		b.WriteString("> ⚠️ 当前评测集中未提供 gold_docs，Recall/Precision/Hit/MRR/nDCG 仅为占位值，不可用于检索效果结论。\n\n")
	}

	b.WriteString("## 明细\n\n")
	b.WriteString("| id | category | latency_ms | recall | precision | hit | mrr | ndcg | retrieval_labeled | status |\n")
	b.WriteString("|---|---|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for _, r := range results {
		status := "ok"
		if r.Error != "" {
			status = "error"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %.3f | %.3f | %.0f | %.3f | %.3f | %t | %s |\n",
			r.ID, r.Category, r.LatencyMs, r.Recall, r.Precision, r.Hit, r.MRR, r.NDCG, r.RetrievalLabeled, status))
	}

	return b.String()
}

func calcNDCG(relevance []int, totalRelevant int) float64 {
	if len(relevance) == 0 || totalRelevant <= 0 {
		return 0
	}
	dcg := 0.0
	for i, rel := range relevance {
		if rel <= 0 {
			continue
		}
		dcg += float64(rel) / math.Log2(float64(i+2))
	}
	idealCount := totalRelevant
	if idealCount > len(relevance) {
		idealCount = len(relevance)
	}
	idcg := 0.0
	for i := 0; i < idealCount; i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	ndcg := dcg / idcg
	if ndcg > 1 {
		return 1
	}
	return ndcg
}

func percentile(values []int64, p int) int64 {
	if len(values) == 0 {
		return 0
	}
	arr := make([]int64, len(values))
	copy(arr, values)
	sort.Slice(arr, func(i, j int) bool { return arr[i] < arr[j] })
	if p <= 0 {
		return arr[0]
	}
	if p >= 100 {
		return arr[len(arr)-1]
	}
	rank := int(math.Ceil(float64(p)/100*float64(len(arr)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(arr) {
		rank = len(arr) - 1
	}
	return arr[rank]
}

func avgInt64(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum int64
	for _, v := range values {
		sum += v
	}
	return float64(sum) / float64(len(values))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
