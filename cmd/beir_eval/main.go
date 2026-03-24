package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	einoembedding "github.com/cloudwego/eino/components/embedding"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/config"
	"eino_agent/internal/container"
)

type beirQuery struct {
	ID             string   `json:"id"`
	Question       string   `json:"question"`
	GoldOrigDocIDs []string `json:"gold_orig_doc_ids"`
	Category       string   `json:"category,omitempty"`
}

type sampleResult struct {
	ID         string   `json:"id"`
	Category   string   `json:"category"`
	LatencyMs  int64    `json:"latency_ms"`
	Retrieved  []string `json:"retrieved"`
	Gold       []string `json:"gold"`
	Recall     float64  `json:"recall"`
	Precision  float64  `json:"precision"`
	Hit        float64  `json:"hit"`
	MRR        float64  `json:"mrr"`
	NDCG       float64  `json:"ndcg"`
	Error      string   `json:"error,omitempty"`
}

type summary struct {
	Dataset      string  `json:"dataset"`
	Strategy     string  `json:"strategy"`
	Queries      int     `json:"queries"`
	TopK         int     `json:"top_k"`
	Recall       float64 `json:"recall"`
	Precision    float64 `json:"precision"`
	Hit          float64 `json:"hit"`
	MRR          float64 `json:"mrr"`
	NDCG         float64 `json:"ndcg"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	P50LatencyMs int64   `json:"p50_latency_ms"`
	P95LatencyMs int64   `json:"p95_latency_ms"`
	ErrorRate    float64 `json:"error_rate"`
}

type report struct {
	Summary summary        `json:"summary"`
	Results []sampleResult `json:"results"`
}

func main() {
	ctx := context.Background()
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	datasetDir := flag.String("dataset", "data/beir_scifact_small", "BEIR 数据集目录")
	strategy := flag.String("strategy", "vector", "检索策略: vector|hybrid|hybrid_rerank")
	topK := flag.Int("top-k", 10, "返回前 K 条结果")
	reportPath := flag.String("report", "", "Markdown 报告输出路径")
	jsonPath := flag.String("json", "", "JSON 结果输出路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("[beir_eval] 读取配置失败: %v\n", err)
		os.Exit(1)
	}
	if *topK > 0 {
		cfg.RAG.TopK = *topK
	}
	applyStrategy(&cfg.RAG, *strategy)

	queries, err := loadQueries(filepath.Join(*datasetDir, "queries.json"))
	if err != nil {
		fmt.Printf("[beir_eval] 读取 queries 失败: %v\n", err)
		os.Exit(1)
	}
	if len(queries) == 0 {
		fmt.Println("[beir_eval] queries 为空")
		os.Exit(1)
	}

	embedder, embedCleanup, err := container.NewEmbeddingProvider(ctx, &cfg.Embedding)
	if err != nil {
		fmt.Printf("[beir_eval] 初始化 embedder 失败: %v\n", err)
		os.Exit(1)
	}
	if embedCleanup != nil {
		defer embedCleanup(nil)
	}

	vectorDB, vectorCleanup, err := container.NewVectorDBProvider(ctx, &config.DatabaseConfig{}, cfg.Embedding.Dimensions)
	if err != nil {
		fmt.Printf("[beir_eval] 初始化 vector db 失败: %v\n", err)
		os.Exit(1)
	}
	if vectorCleanup != nil {
		defer vectorCleanup(nil)
	}

	docs, err := loadDocs(filepath.Join(*datasetDir, "docs"))
	if err != nil {
		fmt.Printf("[beir_eval] 读取 docs 失败: %v\n", err)
		os.Exit(1)
	}
	if err := indexDocs(ctx, embedder, vectorDB, docs); err != nil {
		fmt.Printf("[beir_eval] 建索引失败: %v\n", err)
		os.Exit(1)
	}

	retriever, cleanup, err := container.NewRetrieverProvider(ctx, &cfg.RAG, &cfg.Embedding, embedder, vectorDB, cachepkg.NewNoopRetrievalCache())
	if err != nil {
		fmt.Printf("[beir_eval] 初始化 retriever 失败: %v\n", err)
		os.Exit(1)
	}
	if cleanup != nil {
		defer cleanup(nil)
	}

	var reranker container.RerankerProvider
	if *strategy == "hybrid_rerank" {
		reranker, _, err = container.NewRerankerProvider(ctx, &cfg.Reranker)
		if err != nil {
			fmt.Printf("[beir_eval] 初始化 reranker 失败: %v\n", err)
			os.Exit(1)
		}
	}

	results := make([]sampleResult, 0, len(queries))
	for _, q := range queries {
		res := runQuery(ctx, retriever, reranker, q, cfg.RAG.TopK)
		results = append(results, res)
		if res.Error != "" {
			fmt.Printf("[beir_eval] %s error: %s\n", q.ID, res.Error)
		} else {
			fmt.Printf("[beir_eval] %s ok latency=%dms recall=%.3f hit=%.0f\n", q.ID, res.LatencyMs, res.Recall, res.Hit)
		}
	}

	s := buildSummary(*datasetDir, *strategy, cfg.RAG.TopK, results)
	output := report{Summary: s, Results: results}

	md := buildMarkdown(output)
	if *reportPath == "" {
		*reportPath = filepath.Join("docs", "eval_reports", fmt.Sprintf("%s_beir_%s.md", time.Now().Format("20060102_150405"), *strategy))
	}
	if *jsonPath == "" {
		*jsonPath = strings.TrimSuffix(*reportPath, filepath.Ext(*reportPath)) + ".json"
	}
	if err := os.MkdirAll(filepath.Dir(*reportPath), 0o755); err != nil {
		fmt.Printf("[beir_eval] 创建报告目录失败: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*reportPath, []byte(md), 0o644); err != nil {
		fmt.Printf("[beir_eval] 写入 Markdown 报告失败: %v\n", err)
		os.Exit(1)
	}
	body, _ := json.MarshalIndent(output, "", "  ")
	if err := os.WriteFile(*jsonPath, body, 0o644); err != nil {
		fmt.Printf("[beir_eval] 写入 JSON 报告失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("========================================")
	fmt.Println("Eino BEIR Eval")
	fmt.Println("========================================")
	fmt.Printf("dataset: %s\n", *datasetDir)
	fmt.Printf("strategy: %s\n", *strategy)
	fmt.Printf("queries: %d\n", len(results))
	fmt.Printf("report: %s\n", *reportPath)
	fmt.Printf("json: %s\n", *jsonPath)
}

func applyStrategy(cfg *config.RAGConfig, strategy string) {
	switch strategy {
	case "hybrid":
		cfg.EnableHybrid = true
		cfg.EnableRerank = false
		cfg.EnableRewrite = false
	case "hybrid_rerank":
		cfg.EnableHybrid = true
		cfg.EnableRerank = true
		cfg.EnableRewrite = false
	default:
		cfg.EnableHybrid = false
		cfg.EnableRerank = false
		cfg.EnableRewrite = false
	}
}

func loadQueries(path string) ([]beirQuery, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var queries []beirQuery
	if err := json.Unmarshal(data, &queries); err != nil {
		return nil, err
	}
	return queries, nil
}

func loadDocs(dir string) ([]*container.Document, error) {
	docs := make([]*container.Document, 0)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".txt" {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		docs = append(docs, &container.Document{ID: id, Content: string(body), Metadata: map[string]interface{}{"source": filepath.Base(path)}})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
	return docs, nil
}

func indexDocs(ctx context.Context, embedder einoembedding.Embedder, vectorDB container.VectorDBProvider, docs []*container.Document) error {
	if len(docs) == 0 {
		return nil
	}
	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		texts = append(texts, doc.Content)
	}
	vectors, err := container.BatchEmbedFloat32(ctx, embedder, texts)
	if err != nil {
		return err
	}
	vecIdx := 0
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if vecIdx >= len(vectors) {
			return fmt.Errorf("embedding 结果数量不足")
		}
		doc.Vector = vectors[vecIdx]
		vecIdx++
	}
	return vectorDB.Upsert(ctx, docs)
}

func runQuery(ctx context.Context, r einoretriever.Retriever, reranker container.RerankerProvider, q beirQuery, topK int) sampleResult {
	res := sampleResult{ID: q.ID, Category: q.Category, Gold: append([]string(nil), q.GoldOrigDocIDs...)}
	started := time.Now()
	docs, err := r.Retrieve(ctx, q.Question)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if reranker != nil && len(docs) > 0 {
		cand := make([]*container.Document, 0, len(docs))
		for _, doc := range docs {
			if doc == nil {
				continue
			}
			cand = append(cand, &container.Document{ID: doc.ID, Content: doc.Content, Metadata: doc.MetaData})
		}
		reranked, rerankErr := reranker.Rerank(ctx, q.Question, cand, topK)
		if rerankErr != nil {
			res.Error = rerankErr.Error()
			return res
		}
		docs = make([]*schema.Document, 0, len(reranked))
		for _, doc := range reranked {
			if doc == nil {
				continue
			}
			docs = append(docs, &schema.Document{ID: doc.ID, Content: doc.Content, MetaData: doc.Metadata})
		}
	}
	res.LatencyMs = time.Since(started).Milliseconds()
	limit := topK
	if limit <= 0 || limit > len(docs) {
		limit = len(docs)
	}
	res.Retrieved = make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		if docs[i] == nil {
			continue
		}
		res.Retrieved = append(res.Retrieved, docs[i].ID)
	}
	res.Recall, res.Precision, res.Hit, res.MRR, res.NDCG = retrievalMetrics(q.GoldOrigDocIDs, res.Retrieved)
	return res
}

func buildSummary(dataset, strategy string, topK int, results []sampleResult) summary {
	latencies := make([]int64, 0, len(results))
	var recallSum, precisionSum, hitSum, mrrSum, ndcgSum float64
	errCount := 0
	for _, r := range results {
		if r.Error != "" {
			errCount++
			continue
		}
		latencies = append(latencies, r.LatencyMs)
		recallSum += r.Recall
		precisionSum += r.Precision
		hitSum += r.Hit
		mrrSum += r.MRR
		ndcgSum += r.NDCG
	}
	denom := float64(max(1, len(results)-errCount))
	return summary{
		Dataset:      dataset,
		Strategy:     strategy,
		Queries:      len(results),
		TopK:         topK,
		Recall:       recallSum / denom,
		Precision:    precisionSum / denom,
		Hit:          hitSum / denom,
		MRR:          mrrSum / denom,
		NDCG:         ndcgSum / denom,
		AvgLatencyMs: avgInt64(latencies),
		P50LatencyMs: percentile(latencies, 50),
		P95LatencyMs: percentile(latencies, 95),
		ErrorRate:    float64(errCount) / float64(max(1, len(results))),
	}
}

func buildMarkdown(r report) string {
	var b strings.Builder
	b.WriteString("# Eino BEIR 检索评测报告\n\n")
	b.WriteString(fmt.Sprintf("- 时间: %s\n", time.Now().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- 数据集: %s\n", r.Summary.Dataset))
	b.WriteString(fmt.Sprintf("- 策略: %s\n", r.Summary.Strategy))
	b.WriteString(fmt.Sprintf("- Query 数: %d\n", r.Summary.Queries))
	b.WriteString(fmt.Sprintf("- TopK: %d\n\n", r.Summary.TopK))
	b.WriteString("## 总体指标\n\n")
	b.WriteString(fmt.Sprintf("- Recall@K: %.4f\n", r.Summary.Recall))
	b.WriteString(fmt.Sprintf("- Precision@K: %.4f\n", r.Summary.Precision))
	b.WriteString(fmt.Sprintf("- Hit@K: %.4f\n", r.Summary.Hit))
	b.WriteString(fmt.Sprintf("- MRR@K: %.4f\n", r.Summary.MRR))
	b.WriteString(fmt.Sprintf("- nDCG@K: %.4f\n", r.Summary.NDCG))
	b.WriteString(fmt.Sprintf("- Avg Latency (ms): %.2f\n", r.Summary.AvgLatencyMs))
	b.WriteString(fmt.Sprintf("- P50 Latency (ms): %d\n", r.Summary.P50LatencyMs))
	b.WriteString(fmt.Sprintf("- P95 Latency (ms): %d\n", r.Summary.P95LatencyMs))
	b.WriteString(fmt.Sprintf("- Error Rate: %.4f\n\n", r.Summary.ErrorRate))
	b.WriteString("## 明细\n\n")
	b.WriteString("| id | category | latency_ms | recall | precision | hit | mrr | ndcg | retrieved_topk | status |\n")
	b.WriteString("|---|---|---:|---:|---:|---:|---:|---:|---|---|\n")
	for _, item := range r.Results {
		status := "ok"
		if item.Error != "" {
			status = "error"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %.3f | %.3f | %.0f | %.3f | %.3f | %s | %s |\n",
			item.ID, item.Category, item.LatencyMs, item.Recall, item.Precision, item.Hit, item.MRR, item.NDCG, strings.Join(item.Retrieved, ", "), status))
	}
	return b.String()
}

func retrievalMetrics(goldDocs, retrieved []string) (recall, precision, hit, mrr, ndcg float64) {
	if len(goldDocs) == 0 {
		return 0, 0, 0, 0, 0
	}
	goldSet := make(map[string]struct{}, len(goldDocs))
	for _, g := range goldDocs {
		g = normalizeForMatch(g)
		if g != "" {
			goldSet[g] = struct{}{}
		}
	}
	if len(goldSet) == 0 || len(retrieved) == 0 {
		return 0, 0, 0, 0, 0
	}
	matched := make(map[string]struct{})
	relevantRetrieved := 0
	firstRank := -1
	rels := make([]int, 0, len(retrieved))
	for idx, docID := range retrieved {
		docID = normalizeForMatch(docID)
		if _, ok := goldSet[docID]; ok {
			rels = append(rels, 1)
			if _, seen := matched[docID]; !seen {
				matched[docID] = struct{}{}
				relevantRetrieved++
				if firstRank == -1 {
					firstRank = idx + 1
				}
			}
		} else {
			rels = append(rels, 0)
		}
	}
	recall = float64(len(matched)) / float64(len(goldSet))
	precision = float64(relevantRetrieved) / float64(len(retrieved))
	if relevantRetrieved > 0 {
		hit = 1
		mrr = 1 / float64(firstRank)
	}
	ndcg = calcNDCG(rels, len(goldSet))
	return recall, precision, hit, mrr, ndcg
}

func normalizeForMatch(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
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
	return dcg / idcg
}

func percentile(values []int64, p int) int64 {
	if len(values) == 0 {
		return 0
	}
	copied := append([]int64(nil), values...)
	sort.Slice(copied, func(i, j int) bool { return copied[i] < copied[j] })
	idx := int(math.Ceil(float64(p)/100*float64(len(copied)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(copied) {
		idx = len(copied) - 1
	}
	return copied[idx]
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
