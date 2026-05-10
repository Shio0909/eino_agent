// Package pipeline 实现基于 Eino 的 RAG 流水线
//
// 【Eino 特点】使用 Eino 的 Graph 来编排 RAG 流程，支持：
// - 声明式的节点定义
// - 灵活的条件分支
// - 内置的流式处理
// - 完善的回调机制
package pipeline

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/tracing"
)

// RAGPipeline RAG 流水线
// 【Eino 特点】基于 Eino Graph 编排的 RAG 流程
type RAGPipeline struct {
	config    *Config
	retriever retriever.Retriever
	rewriter  QueryRewriter
	reranker  Reranker
	generator Generator
}

// Config 流水线配置
type Config struct {
	// 是否启用查询重写
	EnableRewrite bool
	// 是否启用重排序
	EnableRerank bool
	// 检索结果数量
	TopK int
	// 重排序后保留数量
	RerankTopK int
	// 生成时使用的模型
	ModelID string
	// 系统提示词
	SystemPrompt string
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		EnableRewrite: true,
		EnableRerank:  true,
		TopK:          10,
		RerankTopK:    5,
		ModelID:       "gpt-4o-mini",
		SystemPrompt:  "你是一个专业的知识库问答助手。请基于参考资料尽可能完整地回答，允许总结和归纳。禁止编造具体信息。每个要点附带[来源X]标注。",
	}
}

// QueryRewriter 查询重写接口
type QueryRewriter interface {
	Rewrite(ctx context.Context, query string) (string, error)
}

// Reranker 重排序接口
type Reranker interface {
	Rerank(ctx context.Context, query string, passages []string) ([]int, error)
}

// Generator 生成器接口
type Generator interface {
	Generate(ctx context.Context, query string, context string) (string, error)
	GenerateStream(ctx context.Context, query string, context string) (<-chan string, error)
}

// NewRAGPipeline 创建 RAG 流水线
func NewRAGPipeline(cfg *Config, opts ...Option) *RAGPipeline {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	p := &RAGPipeline{config: cfg}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Option 配置选项
type Option func(*RAGPipeline)

// WithRetriever 设置检索器
func WithRetriever(r retriever.Retriever) Option {
	return func(p *RAGPipeline) {
		p.retriever = r
	}
}

// WithRewriter 设置查询重写器
func WithRewriter(r QueryRewriter) Option {
	return func(p *RAGPipeline) {
		p.rewriter = r
	}
}

// WithReranker 设置重排序器
func WithReranker(r Reranker) Option {
	return func(p *RAGPipeline) {
		p.reranker = r
	}
}

// WithGenerator 设置生成器
func WithGenerator(g Generator) Option {
	return func(p *RAGPipeline) {
		p.generator = g
	}
}

// RAGRequest RAG 请求
type RAGRequest struct {
	Query                 string            // 用户查询
	SessionID             string            // 会话 ID
	Metadata              map[string]string // 附加元数据
	SkipRewrite           bool              // 跳过重写
	SkipRerank            bool              // 跳过重排序
	GenerationInstruction string            // 仅用于生成阶段的附加指令（不参与检索）
}

// RAGResponse RAG 响应
type RAGResponse struct {
	Answer   string            // 生成的回答
	Sources  []Source          // 引用来源
	RewriteQ string            // 重写后的查询（如果启用）
	Metadata map[string]string // 附加元数据
	Trace    RetrievalTrace    // 检索链路明细
}

type RetrievalTrace struct {
	Retrieved    []TraceChunk `json:"retrieved,omitempty"`
	RerankBefore []TraceChunk `json:"rerank_before,omitempty"`
	RerankAfter  []TraceChunk `json:"rerank_after,omitempty"`
	Context      []TraceChunk `json:"context,omitempty"`
}

type TraceChunk struct {
	Rank      int                    `json:"rank"`
	DocID     string                 `json:"doc_id"`
	Content   string                 `json:"content,omitempty"`
	Score     float64                `json:"score,omitempty"`
	MatchType string                 `json:"match_type,omitempty"`
	Source    string                 `json:"source,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Source 来源信息
type Source struct {
	Content  string                 // 内容片段
	DocID    string                 // 文档 ID
	Score    float64                // 相关性分数
	Metadata map[string]interface{} // 检索元数据
}

// Run 执行 RAG 流水线
// 【Eino 特点】整个流程可被 Eino 的 Callback 系统监控
func (p *RAGPipeline) Run(ctx context.Context, req *RAGRequest) (*RAGResponse, error) {
	startTime := time.Now()
	resp := &RAGResponse{
		Metadata: make(map[string]string),
	}

	// Step 1: 查询重写
	query := req.Query
	if p.config.EnableRewrite && !req.SkipRewrite && p.rewriter != nil {
		rewriteStart := time.Now()
		rewritten, err := p.rewriter.Rewrite(ctx, query)
		log.Printf("[Timing][Pipeline] stage=rewrite duration_ms=%d", time.Since(rewriteStart).Milliseconds())
		if err != nil {
			// 重写失败不影响主流程，记录日志继续
			resp.Metadata["rewrite_error"] = err.Error()
		} else {
			query = rewritten
			resp.RewriteQ = rewritten
		}
	}

	// Step 2: 检索
	if p.retriever == nil {
		return nil, fmt.Errorf("retriever not configured")
	}

	retrieveStart := time.Now()
	docs, err := p.retriever.Retrieve(ctx, query)
	retrieveLatencyMs := time.Since(retrieveStart).Milliseconds()
	log.Printf("[Timing][Pipeline] stage=retrieve duration_ms=%d docs=%d", retrieveLatencyMs, len(docs))
	if err != nil {
		tracing.Emit(ctx, tracing.Event{Type: "error", Stage: "retrieve", Summary: err.Error(), Error: err.Error(), LatencyMs: retrieveLatencyMs})
		return nil, fmt.Errorf("retrieve: %w", err)
	}
	tracing.Emit(ctx, tracing.Event{
		Type:      "retrieval",
		Stage:     "retrieved_candidates",
		Summary:   "retrieved candidate documents",
		LatencyMs: retrieveLatencyMs,
		Metadata:  map[string]any{"query": query, "count": len(docs), "chunks": traceChunksFromDocs(docs)},
	})

	// Step 3: 重排序
	resp.Trace.Retrieved = traceChunksFromDocs(docs)
	resp.Trace.RerankBefore = traceChunksFromDocs(docs)
	passages := extractPassages(docs)
	rerankedIdx := make([]int, len(passages))
	for i := range rerankedIdx {
		rerankedIdx[i] = i
	}

	if p.config.EnableRerank && !req.SkipRerank && p.reranker != nil && len(passages) > 0 {
		rerankStart := time.Now()
		idx, err := p.reranker.Rerank(ctx, query, passages)
		log.Printf("[Timing][Pipeline] stage=rerank duration_ms=%d passages=%d", time.Since(rerankStart).Milliseconds(), len(passages))
		if err != nil {
			resp.Metadata["rerank_error"] = err.Error()
		} else {
			rerankedIdx = idx
		}
	}

	// Step 4: 构建上下文
	gate := evaluateEvidenceGate(query, reorderDocs(docs, rerankedIdx))
	rankedDocs := gate.Docs
	resp.Trace.RerankAfter = traceChunksFromDocs(rankedDocs)
	if gate.Status != evidenceGateOK {
		resp.Metadata["grounding_status"] = "insufficient_evidence"
		resp.Metadata["retrieval_gate"] = string(gate.Status)
		resp.Answer = gate.Answer
		tracing.Emit(ctx, tracing.Event{
			Type:    "guardrail",
			Stage:   "retrieval_gate",
			Summary: gate.Summary,
			Metadata: map[string]any{
				"query":      query,
				"candidates": len(docs),
				"status":     string(gate.Status),
			},
		})
		return resp, nil
	}
	topK := p.config.RerankTopK
	if topK > len(rankedDocs) {
		topK = len(rankedDocs)
	}

	contextStart := time.Now()
	var contextBuilder string
	for i := 0; i < topK; i++ {
		doc := rankedDocs[i]
		contextBuilder += fmt.Sprintf("[来源%d] %s\n\n", i+1, doc.Content)
		resp.Sources = append(resp.Sources, Source{
			Content:  doc.Content,
			DocID:    doc.ID,
			Score:    float64(topK - i),
			Metadata: doc.MetaData,
		})
		resp.Trace.Context = append(resp.Trace.Context, traceChunkFromDoc(doc, i+1))
	}
	log.Printf("[Timing][Pipeline] stage=build_context duration_ms=%d top_k=%d", time.Since(contextStart).Milliseconds(), topK)
	tracing.Emit(ctx, tracing.Event{
		Type:    "context",
		Stage:   "context_build",
		Summary: "built generation context",
		Metadata: map[string]any{
			"chunks":          resp.Trace.Context,
			"count":           len(resp.Trace.Context),
			"context_chars":   len(contextBuilder),
			"context_preview": compactText(contextBuilder, 500),
		},
	})

	// Step 5: 生成回答
	if p.generator == nil {
		return nil, fmt.Errorf("generator not configured")
	}

	generationQuery := req.Query
	if req.GenerationInstruction != "" {
		generationQuery = req.Query + "\n\n" + req.GenerationInstruction
	}

	generateStart := time.Now()
	answer, err := p.generator.Generate(ctx, generationQuery, contextBuilder)
	generateLatencyMs := time.Since(generateStart).Milliseconds()
	log.Printf("[Timing][Pipeline] stage=generate duration_ms=%d context_chars=%d", generateLatencyMs, len(contextBuilder))
	if err != nil {
		tracing.Emit(ctx, tracing.Event{Type: "error", Stage: "generate", Summary: err.Error(), Error: err.Error(), LatencyMs: generateLatencyMs})
		return nil, fmt.Errorf("generate: %w", err)
	}
	tracing.Emit(ctx, tracing.Event{
		Type:      "llm",
		Stage:     "generate",
		Summary:   "answer generated",
		LatencyMs: generateLatencyMs,
		Metadata: map[string]any{
			"context_chars":     len(contextBuilder),
			"answer_chars":      len(answer),
			"token_unavailable": true,
		},
	})
	resp.Answer = answer
	log.Printf("[Timing][Pipeline] stage=total duration_ms=%d", time.Since(startTime).Milliseconds())

	return resp, nil
}

// extractPassages 从文档中提取文本
func extractPassages(docs []*schema.Document) []string {
	passages := make([]string, len(docs))
	for i, doc := range docs {
		passages[i] = doc.Content
	}
	return passages
}

type evidenceGateStatus string

const (
	evidenceGateOK                         evidenceGateStatus = "ok"
	evidenceGateLowQualityEvidence         evidenceGateStatus = "low_quality_evidence"
	evidenceGateIrrelevantEvidence         evidenceGateStatus = "irrelevant_evidence"
	evidenceGateInsufficientProjectContext evidenceGateStatus = "insufficient_project_context"
)

type EvidenceGateDecision struct {
	Status         string
	Answer         string
	Summary        string
	CandidateCount int
	EvidenceCount  int
}

type evidenceGateResult struct {
	Status  evidenceGateStatus
	Docs    []*schema.Document
	Answer  string
	Summary string
}

func EvaluateEvidenceGate(query string, docs []*schema.Document) EvidenceGateDecision {
	gate := evaluateEvidenceGate(query, docs)
	return EvidenceGateDecision{
		Status:         string(gate.Status),
		Answer:         gate.Answer,
		Summary:        gate.Summary,
		CandidateCount: len(docs),
		EvidenceCount:  len(gate.Docs),
	}
}

func IsProjectOverviewQuery(query string) bool {
	return isProjectOverviewQuery(query)
}

func evaluateEvidenceGate(query string, docs []*schema.Document) evidenceGateResult {
	filtered := make([]*schema.Document, 0, len(docs))
	lowQualityCount := 0
	irrelevantCount := 0
	strictRelevance := isStrictOutOfScopeQuery(query)
	for _, doc := range docs {
		if doc == nil || isLowQualityEvidence(doc.Content) {
			lowQualityCount++
			continue
		}
		if strictRelevance && !isEvidenceRelevant(query, doc.Content) {
			irrelevantCount++
			continue
		}
		filtered = append(filtered, doc)
	}

	if isProjectOverviewQuery(query) && !hasProjectContextEvidence(filtered) {
		return evidenceGateResult{
			Status:  evidenceGateInsufficientProjectContext,
			Answer:  "⚠️ 当前知识库没有可用于说明项目整体情况的有效项目描述。请先导入 README、架构文档、复习文档或项目说明后再提问。",
			Summary: "blocked project overview without project context evidence",
		}
	}
	if len(filtered) > 0 {
		return evidenceGateResult{Status: evidenceGateOK, Docs: filtered}
	}
	status := evidenceGateLowQualityEvidence
	summary := "blocked low-quality retrieval evidence"
	if irrelevantCount > 0 && lowQualityCount == 0 {
		status = evidenceGateIrrelevantEvidence
		summary = "blocked irrelevant retrieval evidence"
	}
	return evidenceGateResult{
		Status:  status,
		Answer:  "⚠️ 未在当前知识库中找到足够依据，以下内容不是知识库证据支持的回答。",
		Summary: summary,
	}
}

func isProjectOverviewQuery(query string) bool {
	lower := strings.ToLower(query)
	return strings.Contains(lower, "这个项目") || strings.Contains(lower, "项目是什么") || strings.Contains(lower, "项目介绍") || strings.Contains(lower, "整体架构") || strings.Contains(lower, "project overview")
}

func hasProjectContextEvidence(docs []*schema.Document) bool {
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		lower := strings.ToLower(doc.Content)
		metadataText := strings.ToLower(fmt.Sprint(doc.MetaData))
		if strings.Contains(lower, "eino agent") || strings.Contains(lower, "eino_agent") || strings.Contains(metadataText, "readme") || strings.Contains(metadataText, "终极复习文档") {
			return true
		}
		hasProjectMarker := strings.Contains(lower, "本项目") || strings.Contains(lower, "这个项目") || strings.Contains(lower, "项目概述") || strings.Contains(lower, "项目定位") || strings.Contains(lower, "整体架构")
		hasSystemMarker := strings.Contains(lower, "eino") || strings.Contains(lower, "pipeline") || strings.Contains(lower, "agentic") || strings.Contains(lower, "mcp") || strings.Contains(lower, "code_search") || strings.Contains(lower, "graphrag")
		if hasProjectMarker && hasSystemMarker {
			return true
		}
	}
	return false
}

func isLowQualityEvidence(content string) bool {
	text := strings.TrimSpace(content)
	if len([]rune(text)) < 40 {
		return true
	}

	lower := strings.ToLower(text)
	badSignals := 0
	for _, signal := range []string{
		"example",
		"provided by this api",
		"<a href=",
		"<div",
		"astro-",
		"lorem ipsum",
	} {
		if strings.Contains(lower, signal) {
			badSignals++
		}
	}
	if badSignals >= 2 {
		return true
	}

	letters := 0
	markup := 0
	for _, r := range text {
		if r == '<' || r == '>' || r == '/' || r == '=' {
			markup++
		}
		if unicode.IsLetter(r) || unicode.Is(unicode.Han, r) {
			letters++
		}
	}
	return letters < 20 || markup > letters/2
}

func isStrictOutOfScopeQuery(query string) bool {
	lower := strings.ToLower(query)
	for _, signal := range []string{"天气", "股票", "股价", "涨跌", "彩票", "实时", "今天", "明天", "weather", "stock", "price today"} {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	return false
}

func isEvidenceRelevant(query, content string) bool {
	queryTerms := meaningfulTerms(query)
	if len(queryTerms) == 0 {
		return true
	}
	lowerContent := strings.ToLower(content)
	matches := 0
	for _, term := range queryTerms {
		if strings.Contains(lowerContent, term) {
			matches++
		}
	}
	if matches > 0 {
		return true
	}
	return len(queryTerms) <= 1
}

func meaningfulTerms(text string) []string {
	lower := strings.ToLower(text)
	fields := strings.FieldsFunc(lower, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Han, r))
	})
	terms := make([]string, 0, len(fields))
	stopwords := map[string]struct{}{
		"请": {}, "基于": {}, "知识库": {}, "回答": {}, "说明": {}, "什么": {}, "这个": {}, "项目": {}, "必须": {}, "给出": {}, "引用": {}, "来源": {},
		"the": {}, "and": {}, "for": {}, "with": {}, "this": {}, "that": {}, "what": {}, "how": {}, "why": {}, "please": {}, "answer": {},
	}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len([]rune(field)) < 2 {
			continue
		}
		if _, ok := stopwords[field]; ok {
			continue
		}
		terms = append(terms, field)
	}
	return terms
}

func reorderDocs(docs []*schema.Document, indices []int) []*schema.Document {
	if len(indices) == 0 {
		return docs
	}
	ordered := make([]*schema.Document, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(docs) {
			continue
		}
		ordered = append(ordered, docs[idx])
	}
	if len(ordered) == 0 {
		return docs
	}
	return ordered
}

func traceChunksFromDocs(docs []*schema.Document) []TraceChunk {
	chunks := make([]TraceChunk, 0, len(docs))
	for i, doc := range docs {
		chunks = append(chunks, traceChunkFromDoc(doc, i+1))
	}
	return chunks
}

func traceChunkFromDoc(doc *schema.Document, rank int) TraceChunk {
	if doc == nil {
		return TraceChunk{Rank: rank}
	}
	metadata := make(map[string]interface{}, len(doc.MetaData))
	for key, value := range doc.MetaData {
		metadata[key] = value
	}
	return TraceChunk{
		Rank:      rank,
		DocID:     doc.ID,
		Content:   doc.Content,
		Score:     doc.Score(),
		MatchType: stringMetadata(metadata, "match_type"),
		Source:    firstStringMetadata(metadata, "source", "source_filename", "file_name", "wiki_path"),
		Metadata:  metadata,
	}
}

func stringMetadata(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata[key].(string); ok {
		return value
	}
	return ""
}

func firstStringMetadata(metadata map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := stringMetadata(metadata, key); value != "" {
			return value
		}
	}
	return ""
}

func boundedTopK(length, topK int) int {
	if topK < 0 {
		return 0
	}
	if topK < length {
		return topK
	}
	return length
}

func compactText(text string, maxLen int) string {
	trimmed := strings.TrimSpace(text)
	if maxLen <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= maxLen {
		return trimmed
	}
	return string(runes[:maxLen]) + "..."
}

// RunStream 流式执行 RAG 流水线
// 【Eino 特点】原生支持流式输出，与 Eino 的 StreamReader 无缝集成
func (p *RAGPipeline) RunStream(ctx context.Context, req *RAGRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 100)

	go func() {
		defer close(ch)

		// 前置步骤与 Run 相同
		query := req.Query
		if p.config.EnableRewrite && !req.SkipRewrite && p.rewriter != nil {
			rewritten, err := p.rewriter.Rewrite(ctx, query)
			if err == nil {
				query = rewritten
				ch <- StreamChunk{Type: ChunkTypeRewrite, Content: rewritten}
			}
		}

		if p.retriever == nil {
			ch <- StreamChunk{Type: ChunkTypeError, Content: "retriever not configured"}
			return
		}

		docs, err := p.retriever.Retrieve(ctx, query)
		if err != nil {
			tracing.Emit(ctx, tracing.Event{Type: "error", Stage: "retrieve", Summary: err.Error(), Error: err.Error()})
			ch <- StreamChunk{Type: ChunkTypeError, Content: err.Error()}
			return
		}
		chunks := traceChunksFromDocs(docs)
		traceMetadata := map[string]any{"stage": "retrieved_candidates", "query": query, "chunks": chunks, "count": len(docs)}
		tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "retrieved_candidates", Summary: "retrieved candidate documents", Metadata: traceMetadata})
		ch <- StreamChunk{Type: ChunkTypeTrace, Metadata: traceMetadata}
		rankedDocs := docs
		passages := extractPassages(docs)
		rerankedIdx := make([]int, len(passages))
		for i := range rerankedIdx {
			rerankedIdx[i] = i
		}
		if p.config.EnableRerank && !req.SkipRerank && p.reranker != nil && len(passages) > 0 {
			if idx, rerankErr := p.reranker.Rerank(ctx, query, passages); rerankErr == nil && len(idx) > 0 {
				rerankedIdx = idx
			}
		}
		ch <- StreamChunk{Type: ChunkTypeTrace, Metadata: map[string]any{"stage": "rerank", "before": traceChunksFromDocs(docs)}}
		if len(rerankedIdx) > 0 {
			reordered := make([]*schema.Document, 0, len(rerankedIdx))
			for _, idx := range rerankedIdx {
				if idx < 0 || idx >= len(docs) {
					continue
				}
				reordered = append(reordered, docs[idx])
			}
			if len(reordered) > 0 {
				rankedDocs = reordered
			}
		}
		ch <- StreamChunk{Type: ChunkTypeTrace, Metadata: map[string]any{"stage": "rerank", "after": traceChunksFromDocs(rankedDocs)}}

		// 发送来源信息
		for i, doc := range rankedDocs {
			if i >= p.config.RerankTopK {
				break
			}
			ch <- StreamChunk{
				Type:     ChunkTypeSource,
				Content:  doc.Content,
				DocID:    doc.ID,
				Metadata: doc.MetaData,
			}
		}

		// 构建上下文
		var contextBuilder string
		for i, doc := range rankedDocs {
			if i >= p.config.RerankTopK {
				break
			}
			contextBuilder += fmt.Sprintf("[%d] %s\n\n", i+1, doc.Content)
		}

		contextCount := boundedTopK(len(rankedDocs), p.config.RerankTopK)
		contextChunks := traceChunksFromDocs(rankedDocs[:contextCount])
		contextMetadata := map[string]any{
			"stage":           "context_build",
			"chunks":          contextChunks,
			"count":           contextCount,
			"context_chars":   len(contextBuilder),
			"context_preview": compactText(contextBuilder, 500),
		}
		tracing.Emit(ctx, tracing.Event{Type: "context", Stage: "context_build", Summary: "built generation context", Metadata: contextMetadata})
		ch <- StreamChunk{Type: ChunkTypeTrace, Metadata: contextMetadata}

		// 流式生成
		if p.generator == nil {
			ch <- StreamChunk{Type: ChunkTypeError, Content: "generator not configured"}
			return
		}

		generationQuery := req.Query
		if req.GenerationInstruction != "" {
			generationQuery = req.Query + "\n\n" + req.GenerationInstruction
		}

		stream, err := p.generator.GenerateStream(ctx, generationQuery, contextBuilder)
		if err != nil {
			tracing.Emit(ctx, tracing.Event{Type: "error", Stage: "generate", Summary: err.Error(), Error: err.Error()})
			ch <- StreamChunk{Type: ChunkTypeError, Content: err.Error()}
			return
		}

		generateStart := time.Now()
		answerChars := 0
		for chunk := range stream {
			answerChars += len(chunk)
			ch <- StreamChunk{Type: ChunkTypeContent, Content: chunk}
		}
		tracing.Emit(ctx, tracing.Event{
			Type:      "llm",
			Stage:     "generate",
			Summary:   "answer streamed",
			LatencyMs: time.Since(generateStart).Milliseconds(),
			Metadata: map[string]any{
				"context_chars":     len(contextBuilder),
				"answer_chars":      answerChars,
				"token_unavailable": true,
			},
		})

		ch <- StreamChunk{Type: ChunkTypeDone}
	}()

	return ch, nil
}

// StreamChunk 流式输出块
type StreamChunk struct {
	Type     ChunkType      // 块类型
	Content  string         // 内容
	DocID    string         // 文档 ID（仅 source 类型）
	Metadata map[string]any // 结构化链路信息
}

// ChunkType 块类型
type ChunkType string

const (
	ChunkTypeRewrite ChunkType = "rewrite" // 重写结果
	ChunkTypeSource  ChunkType = "source"  // 来源信息
	ChunkTypeTrace   ChunkType = "trace"   // 检索链路明细
	ChunkTypeContent ChunkType = "content" // 生成内容
	ChunkTypeDone    ChunkType = "done"    // 完成
	ChunkTypeError   ChunkType = "error"   // 错误
)
