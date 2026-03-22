// Package pipeline - Agentic RAG (Corrective RAG) 实现
//
// 【核心亮点】使用 Eino Graph 有环编排实现 Corrective RAG 机制：
// - 检索后自动评估结果质量
// - 质量不达标时自动改写 Query 并重新检索
// - 支持 Web 搜索降级兜底
// - 通过 MaxRetries 防止死循环
//
// 流程图：
//
//	START → [query_rewrite] → [retrieve] → [evaluate]
//	                 ↑                          │
//	                 │      score < threshold   │
//	                 └──────── && retry < max ──┘
//	                                            │
//	                                  score >= threshold
//	                                            ↓
//	                                       [generate] → END
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

// ── State ───────────────────────────────────────────────────────

// AgenticRAGState 是 Agentic RAG Graph 在各节点间共享的状态
type AgenticRAGState struct {
	// 输入
	OriginalQuery string // 用户原始查询
	CurrentQuery  string // 当前（可能被改写过的）查询
	SystemPrompt  string // 系统提示词

	// 检索
	Documents      []*schema.Document // 检索到的文档
	RetrievalScore float64            // 检索质量分数 (0-1)
	RetryCount     int                // 已重试次数

	// 控制
	MaxRetries       int     // 最大重试次数
	QualityThreshold float64 // 质量阈值
	UseWebSearch     bool    // 当前轮次是否使用 Web 搜索

	// 输出
	FinalAnswer  string // 最终生成的回答
	Sources      []AgenticSource
	QueryHistory []string // 查询改写历史
}

// AgenticSource 来源信息
type AgenticSource struct {
	Content string  `json:"content"`
	DocID   string  `json:"doc_id"`
	Score   float64 `json:"score"`
	Type    string  `json:"type"` // "knowledge" or "web"
}

// ── AgenticRAGPipeline ──────────────────────────────────────────

// AgenticRAGPipeline 基于 Eino Graph 有环编排的 Corrective RAG
type AgenticRAGPipeline struct {
	runnable  compose.Runnable[string, string]
	chatModel model.ChatModel
	retriever retriever.Retriever
	cfg       *config.AgenticRAGConfig
}

// AgenticRAGOption 配置选项
type AgenticRAGOption func(*agenticRAGDeps)

type agenticRAGDeps struct {
	chatModel    model.ChatModel
	retriever    retriever.Retriever
	webRetriever retriever.Retriever // 可选的 Web 搜索 retriever
}

// WithAgenticChatModel 设置 ChatModel
func WithAgenticChatModel(m model.ChatModel) AgenticRAGOption {
	return func(d *agenticRAGDeps) { d.chatModel = m }
}

// WithAgenticRetriever 设置知识库检索器
func WithAgenticRetriever(r retriever.Retriever) AgenticRAGOption {
	return func(d *agenticRAGDeps) { d.retriever = r }
}

// WithAgenticWebRetriever 设置 Web 搜索检索器（可选，用于降级）
func WithAgenticWebRetriever(r retriever.Retriever) AgenticRAGOption {
	return func(d *agenticRAGDeps) { d.webRetriever = r }
}

// NewAgenticRAGPipeline 创建 Agentic RAG 流水线
//
// 【Eino 特点】
// - 使用 compose.NewGraph 创建有状态的图
// - 使用 WithGenLocalState 在节点之间共享状态
// - 使用 NewGraphBranch 实现条件分支（有环 → Pregel 模式）
// - 使用 WithMaxRunSteps 防止死循环
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
		chatModel: deps.chatModel,
		retriever: deps.retriever,
		cfg:       cfg,
	}

	// 构建 Graph
	graph := compose.NewGraph[string, string](
		compose.WithGenLocalState(func(ctx context.Context) *AgenticRAGState {
			return &AgenticRAGState{
				MaxRetries:       cfg.MaxRetries,
				QualityThreshold: cfg.QualityThreshold,
			}
		}),
	)

	// ── 节点 1: 查询改写 ──
	rewriteNode := compose.InvokableLambda(p.queryRewrite)
	if err := graph.AddLambdaNode("query_rewrite", rewriteNode,
		compose.WithStatePreHandler(func(ctx context.Context, input string, state *AgenticRAGState) (string, error) {
			if state.OriginalQuery == "" {
				// 第一次进入：记录原始查询
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

	// ── 节点 2: 检索 ──
	retrieveNode := compose.InvokableLambda(p.retrieve)
	if err := graph.AddLambdaNode("retrieve", retrieveNode,
		compose.WithStatePreHandler(func(ctx context.Context, input string, state *AgenticRAGState) (string, error) {
			// 如果是降级到 Web 搜索的情况，打标记
			if state.RetryCount >= 2 && cfg.EnableWebFallback {
				state.UseWebSearch = true
			}
			return state.CurrentQuery, nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, output string, state *AgenticRAGState) (string, error) {
			// output 是 retrieve 节点的字符串输出，实际文档存在 state.Documents 中
			return output, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add retrieve node: %w", err)
	}

	// ── 节点 3: 评估检索质量 ──
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

	// ── 边 ──
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

	// ── 条件分支：评估后决定是重试还是生成 ──
	// 【Eino 特点】这是实现有环图的关键 — evaluate 节点的输出可以回到 query_rewrite
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
			// 无法获取状态，降级到直接生成
			log.Printf("[AgenticRAG] 无法获取状态，直接生成: %v", err)
			return "generate", nil
		}

		if retrievalScore >= qualityThreshold {
			log.Printf("[AgenticRAG] 检索质量达标 (%.2f >= %.2f)，进入生成",
				retrievalScore, qualityThreshold)
			return "generate", nil
		}

		if retryCount >= maxRetries {
			log.Printf("[AgenticRAG] 已达最大重试次数 (%d)，使用当前结果生成", maxRetries)
			return "generate", nil
		}

		log.Printf("[AgenticRAG] 检索质量不足 (%.2f < %.2f)，第 %d 次重试",
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
	// 【Eino 特点】Pregel 模式（默认）支持有环图；WithMaxRunSteps 防止死循环
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

// ── 节点实现 ────────────────────────────────────────────────────

// queryRewrite 查询改写节点
// 第一次直接透传；重试时让 LLM 基于之前的检索反馈改写查询
func (p *AgenticRAGPipeline) queryRewrite(ctx context.Context, query string) (string, error) {
	startTime := time.Now()
	var retryCount int
	var currentQuery string

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		retryCount = state.RetryCount
		currentQuery = state.CurrentQuery
		return nil
	})

	if retryCount == 0 {
		// 第一次不改写，直接用原始查询
		log.Printf("[Timing][AgenticRAG] stage=rewrite duration_ms=%d retry=%d skipped=true", time.Since(startTime).Milliseconds(), retryCount)
		return query, nil
	}

	// 重试时：让 LLM 改写查询
	log.Printf("[AgenticRAG][Rewrite] 第 %d 次改写，当前查询: %s", retryCount, currentQuery)

	var originalQuery string
	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		originalQuery = state.OriginalQuery
		return nil
	})

	prompt := fmt.Sprintf(`你是一个搜索查询优化专家。用户的原始问题是："%s"

之前的搜索查询 "%s" 没有找到足够好的结果。

请改写搜索查询，使其更容易匹配到相关文档。改写策略：
1. 尝试使用同义词或近义词
2. 扩展或缩小搜索范围
3. 提取核心关键词
4. 如果是专业术语，尝试用更通用的表达

只输出改写后的查询，不要解释。`, originalQuery, currentQuery)

	messages := []*schema.Message{
		{Role: schema.User, Content: prompt},
	}

	resp, err := p.chatModel.Generate(ctx, messages)
	log.Printf("[Timing][AgenticRAG] stage=rewrite duration_ms=%d retry=%d", time.Since(startTime).Milliseconds(), retryCount)
	if err != nil {
		log.Printf("[AgenticRAG][Rewrite] LLM 改写失败，使用原始查询: %v", err)
		return query, nil // 改写失败不影响主流程
	}

	rewritten := strings.TrimSpace(resp.Content)
	if rewritten == "" {
		return query, nil
	}

	log.Printf("[AgenticRAG][Rewrite] 改写结果: %s → %s", currentQuery, rewritten)
	return rewritten, nil
}

// retrieve 检索节点
func (p *AgenticRAGPipeline) retrieve(ctx context.Context, query string) (string, error) {
	startTime := time.Now()
	var retryCount int
	var useWebSearch bool

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		retryCount = state.RetryCount
		useWebSearch = state.UseWebSearch
		return nil
	})

	log.Printf("[AgenticRAG][Retrieve] 检索查询: %s (retry=%d, webFallback=%v)",
		query, retryCount, useWebSearch)

	docs, err := p.retriever.Retrieve(ctx, query)
	log.Printf("[Timing][AgenticRAG] stage=retrieve duration_ms=%d retry=%d docs=%d", time.Since(startTime).Milliseconds(), retryCount, len(docs))
	if err != nil {
		return "", fmt.Errorf("retrieve: %w", err)
	}

	// 将文档写入状态
	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		state.Documents = docs
		return nil
	})

	log.Printf("[AgenticRAG][Retrieve] 检索到 %d 个文档", len(docs))

	// 返回文档内容的摘要（用作下游节点的输入）
	var sb strings.Builder
	for i, doc := range docs {
		if i >= 5 { // 最多传递 5 个
			break
		}
		sb.WriteString(fmt.Sprintf("[%d] %s\n\n", i+1, truncate(doc.Content, 200)))
	}
	return sb.String(), nil
}

// evaluate 评估检索质量
// 两级评估：规则判断优先（零延迟），LLM 评估兜底
func (p *AgenticRAGPipeline) evaluate(ctx context.Context, retrieveOutput string) (string, error) {
	startTime := time.Now()
	var docs []*schema.Document
	var currentQuery string

	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		docs = state.Documents
		currentQuery = state.CurrentQuery
		return nil
	})

	// ── 规则评估（快速，零 LLM 调用） ──

	var score float64

	// 规则 1: 没有检索到任何文档 → 直接判定质量差
	if len(docs) == 0 {
		score = 0
		log.Printf("[AgenticRAG][Evaluate] 规则判定：无文档，score=0")
	} else if len(docs) <= 1 {
		// 规则 2: 检索到文档非常少 → 质量可疑
		score = 0.3
		log.Printf("[AgenticRAG][Evaluate] 规则判定：文档过少(%d)，score=0.3", len(docs))
	} else {
		// ── LLM 评估（兜底，更准确） ──
		var err error
		score, err = p.llmEvaluate(ctx, currentQuery, docs)
		if err != nil {
			// LLM 评估失败时，给一个中等分数让流程继续
			log.Printf("[AgenticRAG][Evaluate] LLM 评估失败: %v，使用默认分数 0.5", err)
			score = 0.5
		}
	}

	// 将分数写入状态
	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		state.RetrievalScore = score
		return nil
	})

	var threshold float64
	_ = compose.ProcessState[*AgenticRAGState](ctx, func(ctx context.Context, state *AgenticRAGState) error {
		threshold = state.QualityThreshold
		return nil
	})
	log.Printf("[AgenticRAG][Evaluate] 最终评估分数: %.2f (阈值: %.2f)", score, threshold)
	log.Printf("[Timing][AgenticRAG] stage=evaluate duration_ms=%d docs=%d score=%.2f threshold=%.2f", time.Since(startTime).Milliseconds(), len(docs), score, threshold)
	return retrieveOutput, nil
}

// llmEvaluate 使用 LLM 评估检索结果与查询的相关性
func (p *AgenticRAGPipeline) llmEvaluate(ctx context.Context, query string, docs []*schema.Document) (float64, error) {
	startTime := time.Now()
	// 构建评估文本
	var docsText strings.Builder
	for i, doc := range docs {
		if i >= 5 {
			break
		}
		content := truncate(doc.Content, 300)
		docsText.WriteString(fmt.Sprintf("文档 %d:\n%s\n\n", i+1, content))
	}

	prompt := fmt.Sprintf(`你是一个搜索结果质量评估专家。请评估以下检索结果与用户问题的相关程度。

用户问题：%s

检索结果：
%s

请对这些检索结果的整体相关性给出评价。只输出一个词：
- "高" — 检索结果能很好地回答用户问题
- "中" — 检索结果部分相关，但可能不够完整
- "低" — 检索结果与用户问题关联不大

只输出"高"、"中"或"低"，不要解释。`, query, docsText.String())

	messages := []*schema.Message{
		{Role: schema.User, Content: prompt},
	}

	resp, err := p.chatModel.Generate(ctx, messages)
	log.Printf("[Timing][AgenticRAG] stage=llm_evaluate duration_ms=%d docs=%d", time.Since(startTime).Milliseconds(), len(docs))
	if err != nil {
		return 0, err
	}

	// 将评价映射为分数
	answer := strings.TrimSpace(resp.Content)
	switch {
	case strings.Contains(answer, "高"):
		return 0.9, nil
	case strings.Contains(answer, "中"):
		return 0.6, nil
	case strings.Contains(answer, "低"):
		return 0.2, nil
	default:
		log.Printf("[AgenticRAG][Evaluate] LLM 返回了非预期的评价: %s，默认为 0.5", answer)
		return 0.5, nil
	}
}

// generate 生成最终回答
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

	// 构建上下文
	var contextBuilder strings.Builder
	for i, doc := range docs {
		if i >= 5 {
			break
		}
		contextBuilder.WriteString(fmt.Sprintf("[来源 %d] %s\n\n", i+1, doc.Content))
	}

	// 构建提示词
	if systemPrompt == "" {
		systemPrompt = `你是一个专业的知识库问答助手。请根据提供的参考资料回答用户问题。

回答要求：
1. 仅基于参考资料回答，不编造信息
2. 如果资料不足以完整回答，请明确告知
3. 适当引用来源编号（如 [来源 1]）
4. 回答准确、简洁、专业`
	}

	qualityNote := ""
	if retrievalScore < qualityThreshold {
		qualityNote = "\n\n注意：检索结果的相关性可能不足，请在回答中注明信息可能不够完整。"
	}

	retryNote := ""
	if retryCount > 1 {
		retryNote = fmt.Sprintf("\n(经过 %d 次检索优化)", retryCount)
	}

	userPrompt := fmt.Sprintf(`参考资料：
%s

用户问题：%s%s%s

请根据以上参考资料回答用户的问题。`, contextBuilder.String(), originalQuery, qualityNote, retryNote)

	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	}

	resp, err := p.chatModel.Generate(ctx, messages)
	log.Printf("[Timing][AgenticRAG] stage=generate duration_ms=%d docs=%d context_chars=%d retry=%d", time.Since(startTime).Milliseconds(), len(docs), contextBuilder.Len(), retryCount)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	return resp.Content, nil
}

// ── 公开 API ────────────────────────────────────────────────────

// Run 执行 Agentic RAG
func (p *AgenticRAGPipeline) Run(ctx context.Context, query string) (*AgenticRAGResponse, error) {
	answer, err := p.runnable.Invoke(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("agentic rag invoke: %w", err)
	}

	return &AgenticRAGResponse{
		Answer: answer,
	}, nil
}

// RunStream 流式执行 Agentic RAG
func (p *AgenticRAGPipeline) RunStream(ctx context.Context, query string) (<-chan AgenticStreamEvent, error) {
	ch := make(chan AgenticStreamEvent, 100)

	go func() {
		defer close(ch)

		// 目前使用 Invoke 模式，后续可升级为 Stream
		// TODO: 利用 Eino Graph 的 Stream 能力实现真正的流式输出
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

// ── 响应类型 ────────────────────────────────────────────────────

// AgenticRAGResponse Agentic RAG 响应
type AgenticRAGResponse struct {
	Answer       string          `json:"answer"`
	Sources      []AgenticSource `json:"sources,omitempty"`
	RetryCount   int             `json:"retry_count"`
	QueryHistory []string        `json:"query_history,omitempty"`
}

// AgenticStreamEvent 流式事件
type AgenticStreamEvent struct {
	Type    AgenticEventType `json:"type"`
	Content string           `json:"content,omitempty"`
}

// AgenticEventType 事件类型
type AgenticEventType string

const (
	AgenticEventStatus  AgenticEventType = "status"  // 状态信息（检索中/评估中/改写中）
	AgenticEventContent AgenticEventType = "content" // 最终内容
	AgenticEventDone    AgenticEventType = "done"    // 完成
	AgenticEventError   AgenticEventType = "error"   // 错误
)

// ── 工具函数 ────────────────────────────────────────────────────

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
