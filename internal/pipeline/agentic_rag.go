// Package pipeline implements Agentic RAG (Corrective RAG).
//
// The graph rewrites the query, retrieves context, evaluates retrieval quality,
// and retries until the quality threshold is met or the retry budget is exhausted.
package pipeline

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/config"
)

// AgenticRAGState carries shared state across graph nodes.
type AgenticRAGState struct {
	// Inputs
	OriginalQuery string // The original user query.
	CurrentQuery  string // The current rewritten query.
	SystemPrompt  string // The runtime system prompt.

	// Retrieval
	Documents      []*schema.Document // Retrieved documents for the current attempt.
	RetrievalScore float64            // Retrieval quality score in the range [0, 1].
	RetryCount     int                // Number of retries already attempted.

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
	retriever    retriever.Retriever
	cfg          *config.AgenticRAGConfig
	systemPrompt string
}

// AgenticRAGOption configures AgenticRAGPipeline dependencies.
type AgenticRAGOption func(*agenticRAGDeps)

type agenticRAGDeps struct {
	chatModel    model.ChatModel
	retriever    retriever.Retriever
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

// NewAgenticRAGPipeline builds the corrective RAG graph.
//
// The graph uses local state to carry retries, retrieved documents,
// and evaluation scores across rewrite, retrieve, evaluate, and generate nodes.
// It also uses a conditional branch to either retry retrieval or continue to generation.
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
		retriever:    deps.retriever,
		cfg:          cfg,
		systemPrompt: strings.TrimSpace(deps.systemPrompt),
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

	// Node 1: query rewrite.
	rewriteNode := compose.InvokableLambda(p.queryRewrite)
	if err := graph.AddLambdaNode("query_rewrite", rewriteNode,
		compose.WithStatePreHandler(func(ctx context.Context, input string, state *AgenticRAGState) (string, error) {
			if state.OriginalQuery == "" {
				// Record the first incoming query as the baseline request.
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
		return nil, fmt.Errorf("add query_rewrite node: %w", err)
	}

	// ── 节点 2: 检??──
	retrieveNode := compose.InvokableLambda(p.retrieve)
	if err := graph.AddLambdaNode("retrieve", retrieveNode,
		compose.WithStatePreHandler(func(ctx context.Context, input string, state *AgenticRAGState) (string, error) {
			// 如果是降级到 Web 搜索的情况，打标??
			if state.RetryCount >= 2 && cfg.EnableWebFallback {
				state.UseWebSearch = true
			}
			return state.CurrentQuery, nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, output string, state *AgenticRAGState) (string, error) {
			// output ??retrieve 节点的字符串输出，实际文档存??state.Documents ??
			return output, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add retrieve node: %w", err)
	}

	// ── 节点 3: 评估检索质??──
	evaluateNode := compose.InvokableLambda(p.evaluate)
	if err := graph.AddLambdaNode("evaluate", evaluateNode,
		compose.WithStatePostHandler(func(ctx context.Context, output string, state *AgenticRAGState) (string, error) {
			state.RetryCount++
			return output, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add evaluate node: %w", err)
	}

	// ── 节点 4: 生成回答 ──
	generateNode := compose.InvokableLambda(p.generate)
	if err := graph.AddLambdaNode("generate", generateNode); err != nil {
		return nil, fmt.Errorf("add generate node: %w", err)
	}

	// ── ??──
	if err := graph.AddEdge(compose.START, "query_rewrite"); err != nil {
		return nil, fmt.Errorf("add edge START -> query_rewrite: %w", err)
	}
	if err := graph.AddEdge("query_rewrite", "retrieve"); err != nil {
		return nil, fmt.Errorf("add edge query_rewrite -> retrieve: %w", err)
	}
	if err := graph.AddEdge("retrieve", "evaluate"); err != nil {
		return nil, fmt.Errorf("add edge retrieve -> evaluate: %w", err)
	}
	if err := graph.AddEdge("generate", compose.END); err != nil {
		return nil, fmt.Errorf("add edge generate -> END: %w", err)
	}

	// ── 条件分支：评估后决定是重试还是生??──
	// 【Eino 特点】这是实现有环图的关????evaluate 节点的输出可以回??query_rewrite
	branch := compose.NewGraphBranch(func(ctx context.Context, evaluateOutput string) (string, error) {
		var retrievalScore, qualityThreshold float64
		var retryCount, maxRetries int

		err := compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
			retrievalScore = state.RetrievalScore
			qualityThreshold = state.QualityThreshold
			retryCount = state.RetryCount
			maxRetries = state.MaxRetries
			return nil
		})
		if err != nil {
			// 无法获取状态，降级到直接生??
			log.Printf("[AgenticRAG] 无法获取状态，直接生成: %v", err)
			return "generate", nil
		}

		if retrievalScore >= qualityThreshold {
			log.Printf("[AgenticRAG] retrieval score meets threshold (%.2f >= %.2f); continue to generation",
				retrievalScore, qualityThreshold)
			return "generate", nil
		}

		if retryCount >= maxRetries {
			log.Printf("[AgenticRAG] reached max retries (%d); continue to generation", maxRetries)
			return "generate", nil
		}

		log.Printf("[AgenticRAG] retrieval score below threshold (%.2f < %.2f); retry rewrite #%d",
			retrievalScore, qualityThreshold, retryCount)
		return "query_rewrite", nil

	}, map[string]bool{
		"query_rewrite": true,
		"generate":      true,
	})

	if err := graph.AddBranch("evaluate", branch); err != nil {
		return nil, fmt.Errorf("add branch evaluate -> {query_rewrite, generate}: %w", err)
	}

	// ── 编译 ──
	// 【Eino 特点】Pregel 模式（默认）支持有环图；WithMaxRunSteps 防止死循??
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

// Node implementations

// queryRewrite rewrites the query for the next retrieval attempt.
// The first attempt keeps the original query. Retries ask the LLM to improve it using retrieval feedback.
func (p *AgenticRAGPipeline) queryRewrite(ctx context.Context, query string) (string, error) {
	startTime := time.Now()
	var retryCount int
	var currentQuery string
	var originalQuery string

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		retryCount = state.RetryCount
		currentQuery = state.CurrentQuery
		originalQuery = state.OriginalQuery
		return nil
	})

	if retryCount == 0 {
		log.Printf("[Timing][AgenticRAG] stage=rewrite duration_ms=%d retry=%d skipped=true", time.Since(startTime).Milliseconds(), retryCount)
		return query, nil
	}

	log.Printf("[AgenticRAG][Rewrite] retry=%d current_query=%q", retryCount, currentQuery)

	prompt := fmt.Sprintf(`你正在为知识检索系统改进检索查询。

用户原始问题：
%s

当前查询未能检索到足够有力的证据：
%s

请重写查询以提高检索质量。
要求：
- 保持用户的原始意图
- 使用更清晰或更常用的术语
- 仅在有助于检索时扩展或缩小范围
- 只返回一条重写后的查询，不要附加解释`, originalQuery, currentQuery)

	resp, err := p.chatModel.Generate(ctx, []*schema.Message{
		{Role: schema.User, Content: prompt},
	})
	log.Printf("[Timing][AgenticRAG] stage=rewrite duration_ms=%d retry=%d", time.Since(startTime).Milliseconds(), retryCount)
	if err != nil {
		log.Printf("[AgenticRAG][Rewrite] failed to rewrite query, fallback to original: %v", err)
		return query, nil
	}

	rewritten := strings.TrimSpace(resp.Content)
	if rewritten == "" {
		return query, nil
	}

	log.Printf("[AgenticRAG][Rewrite] rewritten_query=%q", rewritten)
	return rewritten, nil
}

// retrieve runs knowledge retrieval for the current query.
func (p *AgenticRAGPipeline) retrieve(ctx context.Context, query string) (string, error) {
	startTime := time.Now()
	var retryCount int
	var useWebSearch bool

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		retryCount = state.RetryCount
		useWebSearch = state.UseWebSearch
		return nil
	})

	log.Printf("[AgenticRAG][Retrieve] query=%q retry=%d web_fallback=%v", query, retryCount, useWebSearch)

	docs, err := p.retriever.Retrieve(ctx, query)
	log.Printf("[Timing][AgenticRAG] stage=retrieve duration_ms=%d retry=%d docs=%d", time.Since(startTime).Milliseconds(), retryCount, len(docs))
	if err != nil {
		return "", fmt.Errorf("retrieve: %w", err)
	}

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		state.Documents = docs
		return nil
	})

	log.Printf("[AgenticRAG][Retrieve] documents=%d", len(docs))

	var sb strings.Builder
	for i, doc := range docs {
		if i >= 5 {
			break
		}
		sb.WriteString(fmt.Sprintf("[%d] %s\n\n", i+1, truncate(doc.Content, 200)))
	}
	return sb.String(), nil
}

// evaluate scores retrieval quality before generation.
// It uses a lightweight rule-based check first, then falls back to an LLM-based evaluation.
func (p *AgenticRAGPipeline) evaluate(ctx context.Context, retrieveOutput string) (string, error) {
	startTime := time.Now()
	var docs []*schema.Document
	var currentQuery string

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		docs = state.Documents
		currentQuery = state.CurrentQuery
		return nil
	})

	var score float64
	if len(docs) == 0 {
		score = 0
		log.Printf("[AgenticRAG][Evaluate] no documents retrieved, score=0")
	} else if len(docs) <= 1 {
		score = 0.3
		log.Printf("[AgenticRAG][Evaluate] only %d document retrieved, score=0.3", len(docs))
	} else {
		var err error
		score, err = p.llmEvaluate(ctx, currentQuery, docs)
		if err != nil {
			log.Printf("[AgenticRAG][Evaluate] llm evaluation failed, fallback to 0.5: %v", err)
			score = 0.5
		}
	}

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		state.RetrievalScore = score
		return nil
	})

	var threshold float64
	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		threshold = state.QualityThreshold
		return nil
	})

	log.Printf("[AgenticRAG][Evaluate] score=%.2f threshold=%.2f", score, threshold)
	log.Printf("[Timing][AgenticRAG] stage=evaluate duration_ms=%d docs=%d score=%.2f threshold=%.2f", time.Since(startTime).Milliseconds(), len(docs), score, threshold)
	return retrieveOutput, nil
}

// llmEvaluate asks the LLM to judge how well the retrieved documents answer the query.
func (p *AgenticRAGPipeline) llmEvaluate(ctx context.Context, query string, docs []*schema.Document) (float64, error) {
	startTime := time.Now()
	var docsText strings.Builder
	for i, doc := range docs {
		if i >= 5 {
			break
		}
		docsText.WriteString(fmt.Sprintf("文档%d：\n%s\n\n", i+1, truncate(doc.Content, 300)))
	}

	prompt := fmt.Sprintf(`你正在为问答系统评估检索质量。

用户问题：
%s

检索到的文档：
%s

请返回以下标签之一：
- high：文档能有力支撑回答该问题
- medium：文档有一定相关性但不够完整
- low：文档与问题基本无关`, query, docsText.String())

	resp, err := p.chatModel.Generate(ctx, []*schema.Message{
		{Role: schema.User, Content: prompt},
	})
	log.Printf("[Timing][AgenticRAG] stage=llm_evaluate duration_ms=%d docs=%d", time.Since(startTime).Milliseconds(), len(docs))
	if err != nil {
		return 0, err
	}

	answer := strings.ToLower(strings.TrimSpace(resp.Content))
	switch {
	case strings.Contains(answer, "high"):
		return 0.9, nil
	case strings.Contains(answer, "medium"):
		return 0.6, nil
	case strings.Contains(answer, "low"):
		return 0.2, nil
	default:
		log.Printf("[AgenticRAG][Evaluate] unexpected llm label %q, fallback to 0.5", answer)
		return 0.5, nil
	}
}

// generate produces the final grounded answer.
func (p *AgenticRAGPipeline) generate(ctx context.Context, evaluateOutput string) (string, error) {
	startTime := time.Now()
	var docs []*schema.Document
	var originalQuery string
	var systemPrompt string
	var retrievalScore float64
	var qualityThreshold float64
	var retryCount int

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		docs = state.Documents
		originalQuery = state.OriginalQuery
		systemPrompt = state.SystemPrompt
		retrievalScore = state.RetrievalScore
		qualityThreshold = state.QualityThreshold
		retryCount = state.RetryCount
		return nil
	})

	var contextBuilder strings.Builder
	for i, doc := range docs {
		if i >= 5 {
			break
		}
		contextBuilder.WriteString(fmt.Sprintf("[来源%d] %s\n\n", i+1, doc.Content))
	}

	if systemPrompt == "" {
		systemPrompt = `你是一个专业的知识库问答助手。
回答**必须且只能**基于提供的来源资料，**严禁**使用自身训练知识补充。
如果来源资料不足，请直接说明信息不足。
引用证据时使用 [来源X] 标注。`
	}

	qualityNote := ""
	if retrievalScore < qualityThreshold {
		qualityNote = "\n\n注意：检索质量未达到理想阈值，回答可能不完整。"
	}

	retryNote := ""
	if retryCount > 1 {
		retryNote = fmt.Sprintf("\n系统在回答前进行了 %d 次重试检索。", retryCount)
	}

	userPrompt := fmt.Sprintf(`参考资料：
%s

用户问题：
%s%s%s

请严格基于上述参考资料回答问题。`, contextBuilder.String(), originalQuery, qualityNote, retryNote)

	resp, err := p.chatModel.Generate(ctx, []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	})
	log.Printf("[Timing][AgenticRAG] stage=generate duration_ms=%d docs=%d context_chars=%d retry=%d", time.Since(startTime).Milliseconds(), len(docs), contextBuilder.Len(), retryCount)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	return resp.Content, nil
}

// Public API

// Run executes Agentic RAG synchronously.
func (p *AgenticRAGPipeline) Run(ctx context.Context, query string) (*AgenticRAGResponse, error) {
	answer, err := p.runnable.Invoke(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("agentic rag invoke: %w", err)
	}

	return &AgenticRAGResponse{
		Answer: answer,
	}, nil
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
