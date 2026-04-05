// Package pipeline implements Agentic RAG with adaptive query analysis,
// knowledge refinement, and self-reflective generation verification.
//
// Deprecated: This fixed DAG pipeline has been superseded by the unified Agentic
// mode which uses ReAct Agent + tool-based retrieval (knowledge_search, query_decompose,
// web_search). Kept for reference; will be removed in a future version.
//
// Graph topology:
//
//	START → query_analyze → retrieve → knowledge_refine → generate → self_reflect → [END | query_analyze]
//
// Compared with the original Corrective RAG (rewrite→retrieve→evaluate→retry),
// this version adds:
//   - Query Router: classify query as simple / complex / direct
//   - Query Decomposition: break complex multi-hop queries into sub-queries
//   - Knowledge Refinement: per-document relevance filtering (replaces bulk evaluate)
//   - Self-Reflection: post-generation faithfulness + usefulness check
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/config"
)

// QueryType classifies the user query for routing.
type QueryType string

const (
	QueryTypeSimple  QueryType = "simple"  // 简单事实型，直接检索即可
	QueryTypeComplex QueryType = "complex" // 复杂多跳，需要子问题分解
	QueryTypeDirect  QueryType = "direct"  // 无需检索，LLM 可直接回答（闲聊/常识）
)

// directQueryPattern 匹配明显不需要检索的查询（问候、闲聊、简单计算等），
// 命中后直接标记为 direct，省去 classify LLM 调用。
var directQueryPattern = regexp.MustCompile(
	`(?i)^(你好|hello|hi|嗨|早上好|晚上好|下午好|hey|哈喽)[\s!！。.?？]*$|` +
		`(?i)(天气怎么样|几点了|今天星期几)|` +
		`(?i)^(谢谢|再见|拜拜|bye|感谢|辛苦了)[\s!！。.?？]*$|` +
		`(?i)^计算\s*[\d+\-*/().]+$`,
)

// AgenticRAGState carries shared state across graph nodes.
type AgenticRAGState struct {
	// Inputs
	OriginalQuery string // The original user query.
	CurrentQuery  string // The current rewritten query.
	SystemPrompt  string // The runtime system prompt.

	// Query Analysis
	QueryType  QueryType // Router classification result.
	SubQueries []string  // Decomposed sub-queries for complex questions.
	MergedDocs bool      // Whether docs were merged from sub-query retrieval.

	// Retrieval
	Documents      []*schema.Document // Retrieved documents for the current attempt.
	RefinedDocs    []*schema.Document // Documents after knowledge refinement filtering.
	RetrievalScore float64            // Retrieval quality score in the range [0, 1].
	RetryCount     int                // Number of retries already attempted.

	// Generation & Reflection
	GeneratedAnswer string // The most recent generated answer (before reflection).
	ReflectPass     bool   // Whether self-reflection passed.
	ReflectReason   string // Reason if reflection failed.

	// Control
	MaxRetries       int     // Maximum number of rewrite/retrieve retries.
	QualityThreshold float64 // Minimum retrieval quality required to continue.
	UseWebSearch     bool    // Whether web search is enabled for fallback.

	// Outputs
	FinalAnswer  string // Final generated answer.
	Sources      []AgenticSource
	QueryHistory []string // History of rewritten queries.
}

// AgenticSource describes a source used by the final answer.
type AgenticSource struct {
	Content string  `json:"content"`
	DocID   string  `json:"doc_id"`
	Score   float64 `json:"score"`
	Type    string  `json:"type"` // "knowledge" or "web"
}

// AgenticRAGPipeline is a graph-based corrective RAG pipeline.
type AgenticRAGPipeline struct {
	runnable     compose.Runnable[string, string]
	chatModel    model.ChatModel
	lightModel   model.ChatModel // 轻量模型，用于 classify / refine 等低推理节点
	retriever    retriever.Retriever
	reranker     Reranker // 可选重排序器，用于检索后对文档排序
	rerankerTopK int      // 重排序后保留的文档数
	cfg          *config.AgenticRAGConfig
	systemPrompt string

	mu          sync.Mutex // 保护 lastRunMeta
	lastRunMeta *runMeta   // 最近一次 Run 的元信息（供 Run() 填充响应）
}

// runMeta stores metadata from the last graph execution for Run() to read.
type runMeta struct {
	retryCount   int
	queryHistory []string
}

// nodeCtx returns a context with the configured per-node timeout.
// If no timeout is configured, returns the original context.
func (p *AgenticRAGPipeline) nodeCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if p.cfg != nil && p.cfg.NodeTimeoutSec > 0 {
		return context.WithTimeout(ctx, time.Duration(p.cfg.NodeTimeoutSec)*time.Second)
	}
	return ctx, func() {}
}

// AgenticRAGOption configures AgenticRAGPipeline dependencies.
type AgenticRAGOption func(*agenticRAGDeps)

type agenticRAGDeps struct {
	chatModel    model.ChatModel
	lightModel   model.ChatModel // Optional light model for classify/refine.
	retriever    retriever.Retriever
	reranker     Reranker            // Optional reranker for post-retrieval ranking.
	rerankerTopK int                 // Number of docs to keep after reranking.
	webRetriever retriever.Retriever // Optional web retriever for fallback.
	systemPrompt string
}

// WithAgenticChatModel sets the chat model dependency.
func WithAgenticChatModel(m model.ChatModel) AgenticRAGOption {
	return func(d *agenticRAGDeps) { d.chatModel = m }
}

// WithAgenticRetriever sets the primary knowledge retriever.
func WithAgenticRetriever(r retriever.Retriever) AgenticRAGOption {
	return func(d *agenticRAGDeps) { d.retriever = r }
}

// WithAgenticWebRetriever sets the optional web retriever fallback.
func WithAgenticWebRetriever(r retriever.Retriever) AgenticRAGOption {
	return func(d *agenticRAGDeps) { d.webRetriever = r }
}

// WithAgenticSystemPrompt sets the runtime system prompt.
func WithAgenticSystemPrompt(systemPrompt string) AgenticRAGOption {
	return func(d *agenticRAGDeps) { d.systemPrompt = systemPrompt }
}

// WithAgenticLightModel sets an optional lightweight model for classify/refine nodes.
func WithAgenticLightModel(m model.ChatModel) AgenticRAGOption {
	return func(d *agenticRAGDeps) { d.lightModel = m }
}

// WithAgenticReranker sets an optional reranker for post-retrieval document ranking.
func WithAgenticReranker(r Reranker, topK int) AgenticRAGOption {
	return func(d *agenticRAGDeps) {
		d.reranker = r
		d.rerankerTopK = topK
	}
}

// NewAgenticRAGPipeline builds the Agentic RAG graph.
//
// Graph: START → query_analyze → retrieve → knowledge_refine → generate → self_reflect → [END | query_analyze]
//
//   - query_analyze: first pass does routing (simple/complex/direct) + optional decomposition;
//     retry passes do query rewriting based on reflection feedback.
//   - retrieve: executes retrieval; for complex queries, retrieves per sub-query and merges.
//   - knowledge_refine: per-document relevance filtering to remove noise.
//   - generate: produces grounded answer from refined documents.
//   - self_reflect: checks faithfulness (answer supported by docs) and usefulness;
//     branches to END if pass, or back to query_analyze for retry.
func NewAgenticRAGPipeline(ctx context.Context, cfg *config.AgenticRAGConfig, opts ...AgenticRAGOption) (*AgenticRAGPipeline, error) {
	deps := &agenticRAGDeps{}
	for _, opt := range opts {
		opt(deps)
	}
	if deps.chatModel == nil {
		return nil, fmt.Errorf("chatModel is required")
	}
	if deps.retriever == nil {
		return nil, fmt.Errorf("retriever is required")
	}

	p := &AgenticRAGPipeline{
		chatModel:    deps.chatModel,
		lightModel:   deps.lightModel,
		retriever:    deps.retriever,
		reranker:     deps.reranker,
		rerankerTopK: deps.rerankerTopK,
		cfg:          cfg,
		systemPrompt: strings.TrimSpace(deps.systemPrompt),
	}
	// 如果未配置轻量模型，降级使用主模型
	if p.lightModel == nil {
		p.lightModel = p.chatModel
	}

	// Build the graph.
	graph := compose.NewGraph[string, string](
		compose.WithGenLocalState(func(ctx context.Context) *AgenticRAGState {
			return &AgenticRAGState{
				MaxRetries:       cfg.MaxRetries,
				QualityThreshold: cfg.QualityThreshold,
				SystemPrompt:     strings.TrimSpace(deps.systemPrompt),
			}
		}),
	)

	// ── Node 1: query_analyze ──
	// First pass: classify query type (simple/complex/direct) + decompose if complex.
	// Retry passes: rewrite query using reflection feedback.
	analyzeNode := compose.InvokableLambda(p.queryAnalyze)
	if err := graph.AddLambdaNode("query_analyze", analyzeNode,
		compose.WithStatePreHandler(func(ctx context.Context, input string, state *AgenticRAGState) (string, error) {
			if state.OriginalQuery == "" {
				state.OriginalQuery = input
				state.CurrentQuery = input
				state.QueryHistory = []string{input}
			}
			return input, nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, output string, state *AgenticRAGState) (string, error) {
			state.CurrentQuery = output
			state.QueryHistory = append(state.QueryHistory, output)
			return output, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add query_analyze node: %w", err)
	}

	// ── Node 2: retrieve ──
	// For simple queries: single retrieval.
	// For complex queries: per-sub-query retrieval + merge & dedup.
	retrieveNode := compose.InvokableLambda(p.retrieve)
	if err := graph.AddLambdaNode("retrieve", retrieveNode,
		compose.WithStatePreHandler(func(ctx context.Context, input string, state *AgenticRAGState) (string, error) {
			if state.RetryCount >= 2 && cfg.EnableWebFallback {
				state.UseWebSearch = true
			}
			return state.CurrentQuery, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add retrieve node: %w", err)
	}

	// ── Node 3: knowledge_refine ──
	// Per-document relevance scoring; filters out irrelevant docs.
	refineNode := compose.InvokableLambda(p.knowledgeRefine)
	if err := graph.AddLambdaNode("knowledge_refine", refineNode); err != nil {
		return nil, fmt.Errorf("add knowledge_refine node: %w", err)
	}

	// ── Node 4: generate ──
	generateNode := compose.InvokableLambda(p.generate)
	if err := graph.AddLambdaNode("generate", generateNode); err != nil {
		return nil, fmt.Errorf("add generate node: %w", err)
	}

	// ── Node 5: self_reflect ──
	// Post-generation check: is the answer faithful to docs & useful to user?
	reflectNode := compose.InvokableLambda(p.selfReflect)
	if err := graph.AddLambdaNode("self_reflect", reflectNode,
		compose.WithStatePostHandler(func(ctx context.Context, output string, state *AgenticRAGState) (string, error) {
			// 将 graph 内 state 的关键字段写入 pipeline 级别结构体，供 Run() 读取
			p.mu.Lock()
			p.lastRunMeta = &runMeta{
				retryCount:   state.RetryCount,
				queryHistory: append([]string(nil), state.QueryHistory...),
			}
			p.mu.Unlock()
			return output, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add self_reflect node: %w", err)
	}

	// ── Edges ──
	if err := graph.AddEdge(compose.START, "query_analyze"); err != nil {
		return nil, fmt.Errorf("add edge START -> query_analyze: %w", err)
	}
	if err := graph.AddEdge("query_analyze", "retrieve"); err != nil {
		return nil, fmt.Errorf("add edge query_analyze -> retrieve: %w", err)
	}
	if err := graph.AddEdge("retrieve", "knowledge_refine"); err != nil {
		return nil, fmt.Errorf("add edge retrieve -> knowledge_refine: %w", err)
	}
	if err := graph.AddEdge("knowledge_refine", "generate"); err != nil {
		return nil, fmt.Errorf("add edge knowledge_refine -> generate: %w", err)
	}
	if err := graph.AddEdge("generate", "self_reflect"); err != nil {
		return nil, fmt.Errorf("add edge generate -> self_reflect: %w", err)
	}

	// ── Conditional branch: self_reflect → END or retry ──
	reflectBranch := compose.NewGraphBranch(func(ctx context.Context, reflectOutput string) (string, error) {
		var reflectPass bool
		var retryCount, maxRetries int

		err := compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
			reflectPass = state.ReflectPass
			retryCount = state.RetryCount
			maxRetries = state.MaxRetries
			return nil
		})
		if err != nil {
			log.Printf("[AgenticRAG] cannot read state in reflect branch, finishing: %v", err)
			return compose.END, nil
		}

		if reflectPass {
			log.Printf("[AgenticRAG] self-reflection passed; finishing")
			return compose.END, nil
		}

		if retryCount >= maxRetries {
			log.Printf("[AgenticRAG] self-reflection failed but max retries reached (%d); finishing", maxRetries)
			return compose.END, nil
		}

		log.Printf("[AgenticRAG] self-reflection failed; retrying query_analyze (attempt %d)", retryCount)
		return "query_analyze", nil

	}, map[string]bool{
		compose.END:     true,
		"query_analyze": true,
	})

	if err := graph.AddBranch("self_reflect", reflectBranch); err != nil {
		return nil, fmt.Errorf("add branch self_reflect -> {END, query_analyze}: %w", err)
	}

	// ── Compile ──
	runnable, err := graph.Compile(ctx,
		compose.WithMaxRunSteps(cfg.MaxRunSteps),
		compose.WithGraphName("agentic_rag"),
	)
	if err != nil {
		return nil, fmt.Errorf("compile graph: %w", err)
	}

	p.runnable = runnable
	return p, nil
}

// ──────────────────────────────────────────────────────────────
// Node implementations
// ──────────────────────────────────────────────────────────────

// queryAnalyze handles both first-pass analysis (routing + decomposition) and
// retry-pass rewriting. It replaces the old queryRewrite node.
func (p *AgenticRAGPipeline) queryAnalyze(ctx context.Context, query string) (string, error) {
	startTime := time.Now()
	var retryCount int
	var originalQuery string
	var reflectReason string

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		retryCount = state.RetryCount
		originalQuery = state.OriginalQuery
		reflectReason = state.ReflectReason
		return nil
	})

	// ── Retry pass: rewrite query using reflection feedback ──
	if retryCount > 0 {
		rewritten, err := p.rewriteWithFeedback(ctx, originalQuery, query, reflectReason)
		log.Printf("[Timing][AgenticRAG] stage=query_analyze_rewrite duration_ms=%d retry=%d",
			time.Since(startTime).Milliseconds(), retryCount)
		if err != nil {
			log.Printf("[AgenticRAG][Analyze] rewrite failed, using original: %v", err)
			return query, nil
		}
		log.Printf("[AgenticRAG][Analyze] rewritten_query=%q", rewritten)
		return rewritten, nil
	}

	// ── First pass: classify + optional decomposition ──

	// 正则快速判断：明显的闲聊/问候直接标记为 direct，省掉 classify LLM 调用
	if directQueryPattern.MatchString(strings.TrimSpace(query)) {
		log.Printf("[AgenticRAG][Analyze] regex matched direct query, skipping LLM classify")
		_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
			state.QueryType = QueryTypeDirect
			state.SubQueries = nil
			return nil
		})
		return query, nil
	}

	queryType, subQueries, err := p.classifyAndDecompose(ctx, query)
	log.Printf("[Timing][AgenticRAG] stage=query_analyze_classify duration_ms=%d type=%s subs=%d",
		time.Since(startTime).Milliseconds(), queryType, len(subQueries))

	if err != nil {
		log.Printf("[AgenticRAG][Analyze] classification failed, treating as simple: %v", err)
		queryType = QueryTypeSimple
	}

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		state.QueryType = queryType
		state.SubQueries = subQueries
		return nil
	})

	log.Printf("[AgenticRAG][Analyze] query_type=%s sub_queries=%v", queryType, subQueries)

	// For direct-answer queries, return a marker; generate node handles it.
	// For simple/complex, return the query as-is; retrieve handles sub-queries.
	return query, nil
}

// classifyAndDecompose asks the LLM to route the query and optionally decompose it.
func (p *AgenticRAGPipeline) classifyAndDecompose(ctx context.Context, query string) (QueryType, []string, error) {
	prompt := fmt.Sprintf(`分析以下用户查询，判断其类型并返回 JSON。

用户查询：%s

返回格式（严格 JSON，不要附加说明）：
{"type": "simple|complex|direct", "sub_queries": ["子问题1", "子问题2"]}

分类规则：
- simple：单一事实性问题，用一次检索即可回答（如"X 是什么"、"Y 怎么用"）
- complex：多跳推理或需要聚合多个信息源的问题（如"A 和 B 的区别"、"X 导致了什么后果"）。请拆解为 2-4 个子问题放入 sub_queries
- direct：闲聊、问候、数学计算等不需要文档检索的问题。sub_queries 留空

只返回 JSON。`, query)

	llmCtx, llmCancel := p.nodeCtx(ctx)
	defer llmCancel()
	resp, err := p.lightModel.Generate(llmCtx, []*schema.Message{
		{Role: schema.User, Content: prompt},
	})
	if err != nil {
		return QueryTypeSimple, nil, err
	}

	content := strings.TrimSpace(resp.Content)
	// Strip markdown code fences if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result struct {
		Type       string   `json:"type"`
		SubQueries []string `json:"sub_queries"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		log.Printf("[AgenticRAG][Analyze] JSON parse failed (%q), fallback to simple: %v", content, err)
		return QueryTypeSimple, nil, nil
	}

	switch result.Type {
	case "complex":
		if len(result.SubQueries) == 0 {
			return QueryTypeSimple, nil, nil
		}
		return QueryTypeComplex, result.SubQueries, nil
	case "direct":
		return QueryTypeDirect, nil, nil
	default:
		return QueryTypeSimple, nil, nil
	}
}

// rewriteWithFeedback rewrites the query using reflection feedback from the previous attempt.
func (p *AgenticRAGPipeline) rewriteWithFeedback(ctx context.Context, originalQuery, currentQuery, reflectReason string) (string, error) {
	prompt := fmt.Sprintf(`你正在为知识检索系统改进检索查询。

用户原始问题：%s

上一轮查询：%s

上一轮回答的问题反馈：%s

请根据反馈重写查询以提高检索质量。
要求：
- 保持用户的原始意图
- 针对反馈中提到的不足，调整检索方向
- 使用更精确或更常用的术语
- 只返回一条重写后的查询，不要附加解释`, originalQuery, currentQuery, reflectReason)

	llmCtx, llmCancel := p.nodeCtx(ctx)
	defer llmCancel()
	resp, err := p.lightModel.Generate(llmCtx, []*schema.Message{
		{Role: schema.User, Content: prompt},
	})
	if err != nil {
		return currentQuery, err
	}

	rewritten := strings.TrimSpace(resp.Content)
	if rewritten == "" {
		return currentQuery, nil
	}
	return rewritten, nil
}

// retrieve runs knowledge retrieval. For complex queries with sub-queries,
// it retrieves per sub-query and merges/deduplicates results.
func (p *AgenticRAGPipeline) retrieve(ctx context.Context, query string) (string, error) {
	startTime := time.Now()
	var retryCount int
	var queryType QueryType
	var subQueries []string
	var useWebSearch bool

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		retryCount = state.RetryCount
		queryType = state.QueryType
		subQueries = state.SubQueries
		useWebSearch = state.UseWebSearch
		return nil
	})

	log.Printf("[AgenticRAG][Retrieve] query=%q type=%s subs=%d retry=%d web=%v",
		query, queryType, len(subQueries), retryCount, useWebSearch)

	var allDocs []*schema.Document

	// For direct queries: skip retrieval
	if queryType == QueryTypeDirect {
		log.Printf("[AgenticRAG][Retrieve] direct query, skipping retrieval")
		_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
			state.Documents = nil
			state.RefinedDocs = nil
			return nil
		})
		return "", nil
	}

	// For complex queries with sub-queries: retrieve per sub-query in parallel and merge
	if queryType == QueryTypeComplex && len(subQueries) > 0 && retryCount == 0 {
		// 并行检索所有子查询 + 原始查询
		type retrieveResult struct {
			docs []*schema.Document
			sq   string
		}
		queries := append([]string{query}, subQueries...)
		results := make([]retrieveResult, len(queries))
		var wg sync.WaitGroup
		for i, sq := range queries {
			wg.Add(1)
			go func(idx int, q string) {
				defer wg.Done()
				docs, err := p.retriever.Retrieve(ctx, q)
				if err != nil {
					log.Printf("[AgenticRAG][Retrieve] sub-query %q failed: %v", q, err)
					return
				}
				results[idx] = retrieveResult{docs: docs, sq: q}
			}(i, sq)
		}
		wg.Wait()

		// 合并去重
		seen := make(map[string]bool)
		for _, r := range results {
			for _, doc := range r.docs {
				key := doc.ID
				if key == "" {
					key = truncate(doc.Content, 100)
				}
				if !seen[key] {
					seen[key] = true
					allDocs = append(allDocs, doc)
				}
			}
		}
		_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
			state.MergedDocs = true
			return nil
		})
	} else {
		// Simple query or retry: single retrieval
		docs, err := p.retriever.Retrieve(ctx, query)
		if err != nil {
			return "", fmt.Errorf("retrieve: %w", err)
		}
		allDocs = docs
	}

	// Apply reranking if available
	if p.reranker != nil && len(allDocs) > 0 {
		rerankStart := time.Now()
		passages := make([]string, len(allDocs))
		for i, doc := range allDocs {
			passages[i] = doc.Content
		}
		ranked, err := p.reranker.Rerank(ctx, query, passages)
		if err != nil {
			log.Printf("[AgenticRAG][Retrieve] rerank failed, keeping original order: %v", err)
		} else {
			topK := p.rerankerTopK
			if topK <= 0 {
				topK = 5
			}
			if topK > len(ranked) {
				topK = len(ranked)
			}
			rerankedDocs := make([]*schema.Document, 0, topK)
			for i := 0; i < topK; i++ {
				idx := ranked[i]
				if idx < len(allDocs) {
					rerankedDocs = append(rerankedDocs, allDocs[idx])
				}
			}
			log.Printf("[Timing][AgenticRAG] stage=rerank duration_ms=%d docs_in=%d docs_out=%d",
				time.Since(rerankStart).Milliseconds(), len(allDocs), len(rerankedDocs))
			allDocs = rerankedDocs
		}
	}

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		state.Documents = allDocs
		return nil
	})

	log.Printf("[Timing][AgenticRAG] stage=retrieve duration_ms=%d retry=%d docs=%d",
		time.Since(startTime).Milliseconds(), retryCount, len(allDocs))

	var sb strings.Builder
	for i, doc := range allDocs {
		if i >= 10 {
			break
		}
		sb.WriteString(fmt.Sprintf("[%d] %s\n\n", i+1, truncate(doc.Content, 200)))
	}
	return sb.String(), nil
}

// knowledgeRefine filters retrieved documents by per-document relevance scoring.
// This replaces the old bulk evaluate node with more granular filtering.
func (p *AgenticRAGPipeline) knowledgeRefine(ctx context.Context, retrieveOutput string) (string, error) {
	startTime := time.Now()
	var docs []*schema.Document
	var currentQuery string
	var originalQuery string
	var queryType QueryType

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		docs = state.Documents
		currentQuery = state.CurrentQuery
		originalQuery = state.OriginalQuery
		queryType = state.QueryType
		return nil
	})

	// Direct queries or no docs: skip refinement
	if queryType == QueryTypeDirect || len(docs) == 0 {
		_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
			state.RefinedDocs = docs
			state.RetrievalScore = 0
			return nil
		})
		return retrieveOutput, nil
	}

	// If only 1-2 documents, keep all (not enough signal to filter)
	if len(docs) <= 2 {
		_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
			state.RefinedDocs = docs
			state.RetrievalScore = 0.5
			return nil
		})
		log.Printf("[AgenticRAG][Refine] only %d docs, keeping all", len(docs))
		return retrieveOutput, nil
	}

	// Build batch evaluation prompt for per-document relevance
	evalQuery := originalQuery
	if currentQuery != originalQuery {
		evalQuery = fmt.Sprintf("%s（改写后：%s）", originalQuery, currentQuery)
	}

	var docsText strings.Builder
	maxDocs := 8
	if len(docs) < maxDocs {
		maxDocs = len(docs)
	}
	for i := 0; i < maxDocs; i++ {
		docsText.WriteString(fmt.Sprintf("文档%d：\n%s\n\n", i+1, truncate(docs[i].Content, 300)))
	}

	prompt := fmt.Sprintf(`你是一个检索质量评估专家。请逐条判断每篇文档与用户问题的相关性。

用户问题：%s

检索到的文档：
%s
对每篇文档返回 relevant 或 irrelevant 标签。
返回格式（严格 JSON 数组）：
[{"doc": 1, "label": "relevant"}, {"doc": 2, "label": "irrelevant"}, ...]

只返回 JSON。`, evalQuery, docsText.String())

	llmCtx, llmCancel := p.nodeCtx(ctx)
	defer llmCancel()
	resp, err := p.lightModel.Generate(llmCtx, []*schema.Message{
		{Role: schema.User, Content: prompt},
	})
	log.Printf("[Timing][AgenticRAG] stage=knowledge_refine duration_ms=%d docs_in=%d",
		time.Since(startTime).Milliseconds(), len(docs))

	if err != nil {
		// LLM failed: keep all docs (conservative fallback)
		log.Printf("[AgenticRAG][Refine] LLM failed, keeping all docs: %v", err)
		_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
			state.RefinedDocs = docs
			state.RetrievalScore = 0.5
			return nil
		})
		return retrieveOutput, nil
	}

	// Parse per-document labels
	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var labels []struct {
		Doc   int    `json:"doc"`
		Label string `json:"label"`
	}

	var refined []*schema.Document
	if err := json.Unmarshal([]byte(content), &labels); err != nil {
		log.Printf("[AgenticRAG][Refine] JSON parse failed, keeping all: %v", err)
		refined = docs
	} else {
		relevant := make(map[int]bool)
		for _, l := range labels {
			if strings.Contains(strings.ToLower(l.Label), "relevant") && !strings.Contains(strings.ToLower(l.Label), "irrelevant") {
				relevant[l.Doc] = true
			}
		}
		for i, doc := range docs {
			if i >= maxDocs {
				// 超出 LLM 评估范围的文档无条件保留（未评估 ≠ 不相关）
				refined = append(refined, doc)
				continue
			}
			if relevant[i+1] {
				refined = append(refined, doc)
			}
		}
		// If filtering removed all docs, keep top 2 as fallback
		if len(refined) == 0 && len(docs) > 0 {
			log.Printf("[AgenticRAG][Refine] all docs filtered out, keeping top 2")
			end := 2
			if len(docs) < end {
				end = len(docs)
			}
			refined = docs[:end]
		}
	}

	score := float64(len(refined)) / float64(maxDocs)
	if score > 0.9 {
		score = 0.9
	}

	// 检索质量低于阈值时标记，让 selfReflect 更倾向触发重试
	qualityThreshold := 0.3 // 默认阈值
	lowQuality := false
	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		if state.QualityThreshold > 0 {
			qualityThreshold = state.QualityThreshold
		}
		return nil
	})
	if score < qualityThreshold {
		lowQuality = true
		log.Printf("[AgenticRAG][Refine] ⚠ retrieval quality %.2f below threshold %.2f", score, qualityThreshold)
	}

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		state.RefinedDocs = refined
		state.RetrievalScore = score
		if lowQuality && state.RetryCount < state.MaxRetries {
			// 质量不达标时，直接触发重试（跳过 generate+reflect 节省时间）
			state.ReflectPass = false
			state.ReflectReason = fmt.Sprintf("retrieval quality %.2f below threshold %.2f", score, qualityThreshold)
			state.RetryCount++
			log.Printf("[AgenticRAG][Refine] low quality → forcing retry (count=%d)", state.RetryCount)
		}
		return nil
	})

	log.Printf("[AgenticRAG][Refine] docs_in=%d docs_out=%d score=%.2f lowQuality=%v", len(docs), len(refined), score, lowQuality)
	return retrieveOutput, nil
}

// generate produces the final grounded answer.
// generate produces the final answer. Uses RefinedDocs (post-knowledge-refine)
// when available, otherwise falls back to raw Documents.
// For direct queries, generates without any document context.
func (p *AgenticRAGPipeline) generate(ctx context.Context, refineOutput string) (string, error) {
	startTime := time.Now()
	var docs []*schema.Document
	var originalQuery string
	var systemPrompt string
	var queryType QueryType
	var retryCount int

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		// Use refined docs if available, otherwise raw
		if len(state.RefinedDocs) > 0 {
			docs = state.RefinedDocs
		} else {
			docs = state.Documents
		}
		originalQuery = state.OriginalQuery
		systemPrompt = state.SystemPrompt
		queryType = state.QueryType
		retryCount = state.RetryCount
		return nil
	})

	// Direct queries: answer without document context
	if queryType == QueryTypeDirect {
		llmCtx, llmCancel := p.nodeCtx(ctx)
		defer llmCancel()
		resp, err := p.chatModel.Generate(llmCtx, []*schema.Message{
			{Role: schema.User, Content: originalQuery},
		})
		if err != nil {
			return "", fmt.Errorf("generate (direct): %w", err)
		}
		_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
			state.GeneratedAnswer = resp.Content
			return nil
		})
		log.Printf("[Timing][AgenticRAG] stage=generate_direct duration_ms=%d", time.Since(startTime).Milliseconds())
		return resp.Content, nil
	}

	var contextBuilder strings.Builder
	for i, doc := range docs {
		if i >= 8 {
			break
		}
		contextBuilder.WriteString(fmt.Sprintf("[来源%d] %s\n\n", i+1, doc.Content))
	}

	if systemPrompt == "" {
		systemPrompt = `你是一个专业的知识库问答助手。
回答必须且只能基于下方的来源资料，逐条引用原文支撑论点。
不得添加资料中未出现的事实或数据。信息不足时明确说明，不要猜测。
引用证据时使用 [来源X] 标注。优先使用资料原文措辞。`
	}

	retryNote := ""
	if retryCount > 0 {
		retryNote = fmt.Sprintf("\n系统经过 %d 轮检索优化。", retryCount+1)
	}

	userPrompt := fmt.Sprintf(`参考资料：
%s

用户问题：
%s%s

请基于上述参考资料尽可能完整地回答。每个要点附带[来源X]。允许总结归纳。`, contextBuilder.String(), originalQuery, retryNote)

	llmCtx, llmCancel := p.nodeCtx(ctx)
	defer llmCancel()
	resp, err := p.chatModel.Generate(llmCtx, []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	})
	log.Printf("[Timing][AgenticRAG] stage=generate duration_ms=%d docs=%d context_chars=%d retry=%d",
		time.Since(startTime).Milliseconds(), len(docs), contextBuilder.Len(), retryCount)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		state.GeneratedAnswer = resp.Content
		return nil
	})

	return resp.Content, nil
}

// selfReflect evaluates the generated answer for faithfulness and usefulness.
// If the answer fails the check, it sets ReflectPass=false and ReflectReason
// to guide the next retry's query rewriting.
func (p *AgenticRAGPipeline) selfReflect(ctx context.Context, answer string) (string, error) {
	startTime := time.Now()
	var originalQuery string
	var docs []*schema.Document
	var queryType QueryType

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		originalQuery = state.OriginalQuery
		if len(state.RefinedDocs) > 0 {
			docs = state.RefinedDocs
		} else {
			docs = state.Documents
		}
		queryType = state.QueryType
		return nil
	})

	// Direct queries: always pass
	if queryType == QueryTypeDirect || len(docs) == 0 {
		_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
			state.ReflectPass = true
			state.ReflectReason = ""
			return nil
		})
		log.Printf("[Timing][AgenticRAG] stage=self_reflect duration_ms=%d pass=true (direct/no-docs)",
			time.Since(startTime).Milliseconds())
		return answer, nil
	}

	// Build document summary for reflection
	var docsSummary strings.Builder
	for i, doc := range docs {
		if i >= 5 {
			break
		}
		docsSummary.WriteString(fmt.Sprintf("文档%d：%s\n", i+1, truncate(doc.Content, 200)))
	}

	prompt := fmt.Sprintf(`你是回答质量审查员。请评估以下回答的质量。

用户问题：%s

检索到的参考文档：
%s

生成的回答：
%s

请从两个维度进行评估：
1. 忠实性（Faithfulness）：回答是否由参考文档支撑？是否存在编造内容？
2. 有用性（Usefulness）：回答是否充分回答了用户的问题？

返回格式（严格 JSON）：
{"pass": true/false, "reason": "评估理由（如不通过，说明具体哪方面不足以指导检索改进）"}

只返回 JSON。`, originalQuery, docsSummary.String(), truncate(answer, 500))

	// 反思是二分类任务（pass/fail），使用轻量模型即可
	llmCtx, llmCancel := p.nodeCtx(ctx)
	defer llmCancel()
	resp, err := p.lightModel.Generate(llmCtx, []*schema.Message{
		{Role: schema.User, Content: prompt},
	})
	log.Printf("[Timing][AgenticRAG] stage=self_reflect duration_ms=%d",
		time.Since(startTime).Milliseconds())

	if err != nil {
		// LLM failed: track consecutive failures; pass on first, fail on second
		log.Printf("[AgenticRAG][Reflect] LLM failed: %v", err)
		_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
			if state.RetryCount > 0 {
				// 已重试过，再次失败说明 LLM 不稳定，不再盲目通过
				state.ReflectPass = false
				state.ReflectReason = "reflection LLM failed after retry"
				log.Printf("[AgenticRAG][Reflect] LLM failed after retry, rejecting")
			} else {
				state.ReflectPass = true
			}
			return nil
		})
		return answer, nil
	}

	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result struct {
		Pass   bool   `json:"pass"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		log.Printf("[AgenticRAG][Reflect] JSON parse failed (%q): %v", content, err)
		_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
			if state.RetryCount > 0 {
				state.ReflectPass = false
				state.ReflectReason = "reflection JSON parse failed after retry"
			} else {
				state.ReflectPass = true
			}
			return nil
		})
		return answer, nil
	}

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		state.ReflectPass = result.Pass
		state.ReflectReason = result.Reason
		if !result.Pass {
			state.RetryCount++
		}
		return nil
	})

	log.Printf("[AgenticRAG][Reflect] pass=%v reason=%q", result.Pass, result.Reason)
	return answer, nil
}

// Public API

// Run executes Agentic RAG synchronously.
func (p *AgenticRAGPipeline) Run(ctx context.Context, query string) (*AgenticRAGResponse, error) {
	answer, err := p.runnable.Invoke(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("agentic rag invoke: %w", err)
	}

	resp := &AgenticRAGResponse{Answer: answer}

	p.mu.Lock()
	if p.lastRunMeta != nil {
		resp.RetryCount = p.lastRunMeta.retryCount
		resp.QueryHistory = p.lastRunMeta.queryHistory
	}
	p.mu.Unlock()

	return resp, nil
}

// RunStream executes Agentic RAG with streaming events.
func (p *AgenticRAGPipeline) RunStream(ctx context.Context, query string) (<-chan AgenticStreamEvent, error) {
	ch := make(chan AgenticStreamEvent, 100)

	go func() {
		defer close(ch)

		// 目前使用 Invoke 模式，后续可升级??Stream
		// TODO: 利用 Eino Graph ??Stream 能力实现真正的流式输??
		ch <- AgenticStreamEvent{Type: AgenticEventStatus, Content: "正在分析问题..."}

		answer, err := p.runnable.Invoke(ctx, query)
		if err != nil {
			ch <- AgenticStreamEvent{Type: AgenticEventError, Content: err.Error()}
			return
		}

		ch <- AgenticStreamEvent{Type: AgenticEventContent, Content: answer}
		ch <- AgenticStreamEvent{Type: AgenticEventDone}
	}()

	return ch, nil
}

// Response types

// AgenticRAGResponse is the synchronous response payload.
type AgenticRAGResponse struct {
	Answer       string          `json:"answer"`
	Sources      []AgenticSource `json:"sources,omitempty"`
	RetryCount   int             `json:"retry_count"`
	QueryHistory []string        `json:"query_history,omitempty"`
}

// AgenticStreamEvent is a streaming event emitted by Agentic RAG.
type AgenticStreamEvent struct {
	Type    AgenticEventType `json:"type"`
	Content string           `json:"content,omitempty"`
}

// AgenticEventType enumerates stream event types.
type AgenticEventType string

const (
	AgenticEventStatus  AgenticEventType = "status"  // 状态信息（检索中/评估??改写中）
	AgenticEventContent AgenticEventType = "content" // 最终内??
	AgenticEventDone    AgenticEventType = "done"    // 完成
	AgenticEventError   AgenticEventType = "error"   // 错误
)

// Helpers

// truncate shortens a string to the requested rune length.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
