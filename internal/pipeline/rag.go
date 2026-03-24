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
		SystemPrompt:  "你是一个智能助手，请根据提供的上下文回答用户问题。",
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
}

// Source 来源信息
type Source struct {
	Content string  // 内容片段
	DocID   string  // 文档 ID
	Score   float64 // 相关性分数
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

	// Step 3: 重排序
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
	topK := p.config.RerankTopK
	if topK > len(rerankedIdx) {
		topK = len(rerankedIdx)
	}

	contextStart := time.Now()
	var contextBuilder string
	for i := 0; i < topK; i++ {
		idx := rerankedIdx[i]
		if idx < len(docs) {
			contextBuilder += fmt.Sprintf("[%d] %s\n\n", i+1, docs[idx].Content)
			resp.Sources = append(resp.Sources, Source{
				Content: docs[idx].Content,
				DocID:   docs[idx].ID,
				Score:   float64(topK - i), // 简单的位置分数
			})
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
	Type    ChunkType // 块类型
	Content string    // 内容
	DocID   string    // 文档 ID（仅 source 类型）
}

// ChunkType 块类型
type ChunkType string

const (
	ChunkTypeRewrite ChunkType = "rewrite" // 重写结果
	ChunkTypeSource  ChunkType = "source"  // 来源信息
	ChunkTypeContent ChunkType = "content" // 生成内容
	ChunkTypeDone    ChunkType = "done"    // 完成
	ChunkTypeError   ChunkType = "error"   // 错误
)
