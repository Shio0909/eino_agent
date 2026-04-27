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
	"time"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// RAGPipeline RAG 流水线
// 【Eino 特点】基于 Eino Graph 编排的 RAG 流程
type RAGPipeline struct {
	config    *Config
	retriever retriever.Retriever
	rewriter  QueryRewriter
	reranker  Reranker
	generator Generator
	fallback  ExternalFallbackProvider
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
	// 外部降级策略
	Fallback FallbackConfig
}

type FallbackConfig struct {
	Enabled             bool
	MinKnowledgeDocs    int
	MinContextChars     int
	MaxExternalResults  int
	MaxExternalContext  int
	AllowedProviders    []string
	ProviderByKeyword   map[string][]string
	RefuseWhenNoSources bool
}

type AnswerState string

const (
	StateKBRetrieve           AnswerState = "kb_retrieve"
	StateKBAssess             AnswerState = "kb_assess"
	StateKBContextBuild       AnswerState = "kb_context_build"
	StateExternalPlan         AnswerState = "external_plan"
	StateExternalSearch       AnswerState = "external_search"
	StateExternalAssess       AnswerState = "external_assess"
	StateExternalContextBuild AnswerState = "external_context_build"
	StateGenerate             AnswerState = "generate"
	StateRefuseOrClarify      AnswerState = "refuse_or_clarify"
)

type FallbackReason string

const (
	FallbackReasonNone          FallbackReason = "none"
	FallbackReasonNoDocuments   FallbackReason = "no_documents"
	FallbackReasonShortContext  FallbackReason = "short_context"
	FallbackReasonNoExternal    FallbackReason = "no_external_results"
	FallbackReasonProviderError FallbackReason = "provider_error"
)

type FallbackDecision struct {
	State         AnswerState    `json:"state"`
	Reason        FallbackReason `json:"reason"`
	AllowExternal bool           `json:"allow_external"`
	Providers     []string       `json:"providers,omitempty"`
	DocsCount     int            `json:"docs_count"`
	ContextChars  int            `json:"context_chars"`
}

type ExternalSearchRequest struct {
	Query      string
	Providers  []string
	MaxResults int
}

type ExternalSearchResult struct {
	Provider string
	Title    string
	URL      string
	Content  string
	Score    float64
	Metadata map[string]interface{}
}

type ExternalFallbackProvider interface {
	Search(ctx context.Context, req ExternalSearchRequest) ([]ExternalSearchResult, error)
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

func WithExternalFallbackProvider(provider ExternalFallbackProvider) Option {
	return func(p *RAGPipeline) {
		p.fallback = provider
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
	Fallback FallbackDecision  // 确定性降级决策
}

type RetrievalTrace struct {
	Retrieved    []TraceChunk        `json:"retrieved,omitempty"`
	RerankBefore []TraceChunk        `json:"rerank_before,omitempty"`
	RerankAfter  []TraceChunk        `json:"rerank_after,omitempty"`
	Context      []TraceChunk        `json:"context,omitempty"`
	Fallback     []FallbackGraphStep `json:"fallback,omitempty"`
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
	log.Printf("[Timing][Pipeline] stage=retrieve duration_ms=%d docs=%d", time.Since(retrieveStart).Milliseconds(), len(docs))
	if err != nil {
		return nil, fmt.Errorf("retrieve: %w", err)
	}
	fallbackOutput, err := p.runFallbackGraph(ctx, query, docs)
	if err != nil {
		return nil, err
	}
	resp.Fallback = fallbackOutput.Decision
	resp.Trace.Fallback = fallbackOutput.Trace
	resp.Metadata["fallback_state"] = string(resp.Fallback.State)
	resp.Metadata["fallback_reason"] = string(resp.Fallback.Reason)
	if fallbackOutput.Err != nil {
		resp.Metadata["fallback_error"] = fallbackOutput.Err.Error()
	}
	docs = fallbackOutput.Docs

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
	rankedDocs := reorderDocs(docs, rerankedIdx)
	resp.Trace.RerankAfter = traceChunksFromDocs(rankedDocs)
	topK := p.config.RerankTopK
	if topK > len(rerankedIdx) {
		topK = len(rerankedIdx)
	}

	contextStart := time.Now()
	var contextBuilder string
	for i := 0; i < topK; i++ {
		idx := rerankedIdx[i]
		if idx < len(docs) {
			doc := docs[idx]
			contextBuilder += fmt.Sprintf("[来源%d] %s\n\n", i+1, doc.Content)
			resp.Sources = append(resp.Sources, Source{
				Content:  doc.Content,
				DocID:    doc.ID,
				Score:    float64(topK - i), // 简单的位置分数
				Metadata: doc.MetaData,
			})
			resp.Trace.Context = append(resp.Trace.Context, traceChunkFromDoc(doc, i+1))
		}
	}
	log.Printf("[Timing][Pipeline] stage=build_context duration_ms=%d top_k=%d", time.Since(contextStart).Milliseconds(), topK)

	// Step 5: 生成回答
	if p.generator == nil {
		return nil, fmt.Errorf("generator not configured")
	}

	generationQuery := req.Query
	if req.GenerationInstruction != "" {
		generationQuery = req.Query + "\n\n" + req.GenerationInstruction
	}

	generateStart := time.Now()
	if len(resp.Sources) == 0 && p.fallbackConfig().RefuseWhenNoSources {
		resp.Fallback.State = StateRefuseOrClarify
		resp.Answer = "⚠️ 未在当前知识库或已启用的外部来源中找到足够依据，请补充资料或放宽检索范围。"
		log.Printf("[Timing][Pipeline] stage=refuse duration_ms=%d", time.Since(generateStart).Milliseconds())
		return resp, nil
	}
	resp.Fallback.State = StateGenerate
	answer, err := p.generator.Generate(ctx, generationQuery, contextBuilder)
	log.Printf("[Timing][Pipeline] stage=generate duration_ms=%d context_chars=%d", time.Since(generateStart).Milliseconds(), len(contextBuilder))
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
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
			ch <- StreamChunk{Type: ChunkTypeError, Content: err.Error()}
			return
		}
		fallbackOutput, err := p.runFallbackGraph(ctx, query, docs)
		if err != nil {
			ch <- StreamChunk{Type: ChunkTypeError, Content: err.Error()}
			return
		}
		for _, step := range fallbackOutput.Trace {
			ch <- StreamChunk{Type: ChunkTypeTrace, Metadata: map[string]any{"stage": string(step.State), "decision": fallbackOutput.Decision, "step": step}}
		}
		docs = fallbackOutput.Docs
		ch <- StreamChunk{Type: ChunkTypeTrace, Metadata: map[string]any{"stage": "retrieved_candidates", "chunks": traceChunksFromDocs(docs), "count": len(docs)}}
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
				Type:    ChunkTypeSource,
				Content: doc.Content,
				DocID:   doc.ID,
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
		ch <- StreamChunk{Type: ChunkTypeTrace, Metadata: map[string]any{"stage": "context_build", "chunks": traceChunksFromDocs(rankedDocs[:contextCount]), "count": contextCount}}

		// 流式生成
		if p.generator == nil {
			ch <- StreamChunk{Type: ChunkTypeError, Content: "generator not configured"}
			return
		}

		generationQuery := req.Query
		if req.GenerationInstruction != "" {
			generationQuery = req.Query + "\n\n" + req.GenerationInstruction
		}

		if len(rankedDocs) == 0 && p.fallbackConfig().RefuseWhenNoSources {
			ch <- StreamChunk{Type: ChunkTypeTrace, Metadata: map[string]any{"stage": string(StateRefuseOrClarify)}}
			ch <- StreamChunk{Type: ChunkTypeContent, Content: "⚠️ 未在当前知识库或已启用的外部来源中找到足够依据，请补充资料或放宽检索范围。"}
			ch <- StreamChunk{Type: ChunkTypeDone}
			return
		}

		stream, err := p.generator.GenerateStream(ctx, generationQuery, contextBuilder)
		if err != nil {
			ch <- StreamChunk{Type: ChunkTypeError, Content: err.Error()}
			return
		}

		for chunk := range stream {
			ch <- StreamChunk{Type: ChunkTypeContent, Content: chunk}
		}

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
