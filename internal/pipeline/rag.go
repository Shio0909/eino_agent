// Package pipeline implements the knowledge-base RAG flow as an Eino compose
// Graph:
//
//	START → rewrite → retrieve → rerank → evidence_gate ─┬→ build_context → generate → END
//	                                                     └→ reject ─────────────────→ END
//
// Run (Invoke) and RunStream (Stream) execute the same compiled graph, so the
// evidence gate, context construction and tracing behave identically in both
// modes.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/tracing"
)

const (
	nodeRewrite      = "rewrite"
	nodeRetrieve     = "retrieve"
	nodeRerank       = "rerank"
	nodeEvidenceGate = "evidence_gate"
	nodeReject       = "reject"
	nodeBuildContext = "build_context"
	nodeGenerate     = "generate"

	defaultRerankTopK = 5
	graphName         = "rag_pipeline"
)

// RAGPipeline is a compiled RAG graph plus its pluggable components.
type RAGPipeline struct {
	config    *Config
	retriever retriever.Retriever
	rewriter  QueryRewriter
	reranker  Reranker
	generator Generator

	compileOnce sync.Once
	runnable    compose.Runnable[*RAGRequest, *RAGResponse]
	compileErr  error
}

// Config controls optional stages of the pipeline.
type Config struct {
	EnableRewrite bool
	EnableRerank  bool
	// TopK is the retrieval depth; it is applied by the retriever itself and
	// kept here so callers can configure both in one place.
	TopK int
	// RerankTopK is the number of evidence chunks passed to generation.
	RerankTopK int
	// SystemPrompt is informational; the generator owns the prompt it sends.
	SystemPrompt string
}

// DefaultConfig returns the default pipeline configuration.
func DefaultConfig() *Config {
	return &Config{
		EnableRewrite: true,
		EnableRerank:  true,
		TopK:          10,
		RerankTopK:    defaultRerankTopK,
	}
}

// QueryRewriter rewrites a user query into a retrieval-friendly form.
type QueryRewriter interface {
	Rewrite(ctx context.Context, query string) (string, error)
}

// Reranker returns passage indices ordered from most to least relevant.
type Reranker interface {
	Rerank(ctx context.Context, query string, passages []string) ([]int, error)
}

// RankedPassage is one reranked passage with its relevance score.
type RankedPassage struct {
	Index int
	Score float64
}

// ScoredReranker is an optional Reranker extension that also returns scores.
// When available, the pipeline records rerank scores on documents and
// sources instead of keeping retrieval scores only.
type ScoredReranker interface {
	RerankWithScores(ctx context.Context, query string, passages []string) ([]RankedPassage, error)
}

// Generator produces an answer from a query and a rendered evidence context.
type Generator interface {
	Generate(ctx context.Context, query string, context string) (string, error)
	GenerateStream(ctx context.Context, query string, context string) (<-chan string, error)
}

// NewRAGPipeline creates a pipeline. The graph is compiled lazily on first use.
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

// Option configures a RAGPipeline.
type Option func(*RAGPipeline)

func WithRetriever(r retriever.Retriever) Option { return func(p *RAGPipeline) { p.retriever = r } }
func WithRewriter(r QueryRewriter) Option        { return func(p *RAGPipeline) { p.rewriter = r } }
func WithReranker(r Reranker) Option             { return func(p *RAGPipeline) { p.reranker = r } }
func WithGenerator(g Generator) Option           { return func(p *RAGPipeline) { p.generator = g } }

// RAGRequest is the pipeline input.
type RAGRequest struct {
	Query       string
	SessionID   string
	Metadata    map[string]string
	SkipRewrite bool
	SkipRerank  bool
	// GenerationInstruction is appended to the question for generation only;
	// it never influences retrieval.
	GenerationInstruction string
}

// RAGResponse is the pipeline output.
type RAGResponse struct {
	Answer   string
	Sources  []Source
	RewriteQ string
	Metadata map[string]string
	Trace    RetrievalTrace

	contextText string
}

// Source is one evidence chunk passed to generation.
type Source struct {
	Content  string
	DocID    string
	Score    float64
	Metadata map[string]interface{}
}

// ragState flows through the graph nodes. Each run owns its own state.
type ragState struct {
	req    *RAGRequest
	query  string
	ranked []*schema.Document
	gate   evidenceGateResult
	resp   *RAGResponse
}

func init() {
	compose.RegisterStreamChunkConcatFunc(concatResponses)
}

func (p *RAGPipeline) compiled() (compose.Runnable[*RAGRequest, *RAGResponse], error) {
	p.compileOnce.Do(func() {
		p.runnable, p.compileErr = p.buildGraph(context.Background())
	})
	return p.runnable, p.compileErr
}

func (p *RAGPipeline) buildGraph(ctx context.Context) (compose.Runnable[*RAGRequest, *RAGResponse], error) {
	g := compose.NewGraph[*RAGRequest, *RAGResponse]()

	generate, err := compose.AnyLambda(p.generateInvoke, p.generateStream, nil, nil)
	if err != nil {
		return nil, err
	}
	nodes := []struct {
		key    string
		lambda *compose.Lambda
	}{
		{nodeRewrite, compose.InvokableLambda(p.rewrite)},
		{nodeRetrieve, compose.InvokableLambda(p.retrieve)},
		{nodeRerank, compose.InvokableLambda(p.rerank)},
		{nodeEvidenceGate, compose.InvokableLambda(p.evidenceGate)},
		{nodeReject, compose.InvokableLambda(p.reject)},
		{nodeBuildContext, compose.InvokableLambda(p.buildContext)},
		{nodeGenerate, generate},
	}
	for _, node := range nodes {
		if err := g.AddLambdaNode(node.key, node.lambda, compose.WithNodeName(node.key)); err != nil {
			return nil, fmt.Errorf("add node %s: %w", node.key, err)
		}
	}

	edges := [][2]string{
		{compose.START, nodeRewrite},
		{nodeRewrite, nodeRetrieve},
		{nodeRetrieve, nodeRerank},
		{nodeRerank, nodeEvidenceGate},
		{nodeBuildContext, nodeGenerate},
		{nodeGenerate, compose.END},
		{nodeReject, compose.END},
	}
	for _, edge := range edges {
		if err := g.AddEdge(edge[0], edge[1]); err != nil {
			return nil, fmt.Errorf("add edge %s→%s: %w", edge[0], edge[1], err)
		}
	}

	gateBranch := compose.NewGraphBranch(func(_ context.Context, st *ragState) (string, error) {
		if st.gate.Status == evidenceGateOK {
			return nodeBuildContext, nil
		}
		return nodeReject, nil
	}, map[string]bool{nodeBuildContext: true, nodeReject: true})
	if err := g.AddBranch(nodeEvidenceGate, gateBranch); err != nil {
		return nil, fmt.Errorf("add evidence gate branch: %w", err)
	}

	return g.Compile(ctx, compose.WithGraphName(graphName))
}

// Run executes the graph and returns the full answer.
func (p *RAGPipeline) Run(ctx context.Context, req *RAGRequest) (*RAGResponse, error) {
	runnable, err := p.compiled()
	if err != nil {
		return nil, fmt.Errorf("compile rag graph: %w", err)
	}
	return runnable.Invoke(ctx, req, compose.WithCallbacks(newTimingHandler()))
}

// RunStream executes the graph in streaming mode. The channel yields rewrite,
// trace and source chunks first, then answer content, then a done chunk.
// Failures are delivered as a ChunkTypeError chunk.
func (p *RAGPipeline) RunStream(ctx context.Context, req *RAGRequest) (<-chan StreamChunk, error) {
	runnable, err := p.compiled()
	if err != nil {
		return nil, fmt.Errorf("compile rag graph: %w", err)
	}

	ch := make(chan StreamChunk, 64)
	send := func(chunk StreamChunk) bool {
		select {
		case ch <- chunk:
			return true
		case <-ctx.Done():
			return false
		}
	}

	go func() {
		defer close(ch)

		reader, err := runnable.Stream(ctx, req, compose.WithCallbacks(newTimingHandler()))
		if err != nil {
			send(StreamChunk{Type: ChunkTypeError, Content: err.Error()})
			return
		}
		defer reader.Close()

		first := true
		for {
			part, err := reader.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				send(StreamChunk{Type: ChunkTypeError, Content: err.Error()})
				return
			}
			if part == nil {
				continue
			}
			if first {
				first = false
				for _, chunk := range headerChunks(req, part) {
					if !send(chunk) {
						return
					}
				}
			}
			if part.Answer != "" {
				if !send(StreamChunk{Type: ChunkTypeContent, Content: part.Answer}) {
					return
				}
			}
		}
		send(StreamChunk{Type: ChunkTypeDone})
	}()

	return ch, nil
}

func (p *RAGPipeline) rewrite(ctx context.Context, req *RAGRequest) (*ragState, error) {
	if req == nil || strings.TrimSpace(req.Query) == "" {
		return nil, errors.New("empty query")
	}
	st := &ragState{req: req, query: req.Query, resp: &RAGResponse{Metadata: make(map[string]string)}}
	if !p.config.EnableRewrite || req.SkipRewrite || p.rewriter == nil {
		return st, nil
	}
	rewritten, err := p.rewriter.Rewrite(ctx, req.Query)
	if err != nil {
		st.resp.Metadata[MetadataKeyRewriteError] = err.Error()
		return st, nil
	}
	if rewritten = strings.TrimSpace(rewritten); rewritten != "" {
		st.query = rewritten
		st.resp.RewriteQ = rewritten
	}
	return st, nil
}

func (p *RAGPipeline) retrieve(ctx context.Context, st *ragState) (*ragState, error) {
	if p.retriever == nil {
		return nil, errors.New("retriever not configured")
	}
	started := time.Now()
	docs, err := p.retriever.Retrieve(ctx, st.query)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		tracing.Emit(ctx, tracing.Event{Type: "error", Stage: nodeRetrieve, Summary: err.Error(), Error: err.Error(), LatencyMs: latency})
		return nil, fmt.Errorf("retrieve: %w", err)
	}
	st.ranked = docs
	st.resp.Trace.Retrieved = traceChunksFromDocs(docs)
	st.resp.Trace.RerankBefore = st.resp.Trace.Retrieved
	tracing.Emit(ctx, tracing.Event{
		Type:      "retrieval",
		Stage:     "retrieved_candidates",
		Summary:   "retrieved candidate documents",
		LatencyMs: latency,
		Metadata:  map[string]any{"query": st.query, "count": len(docs), "chunks": st.resp.Trace.Retrieved},
	})
	return st, nil
}

func (p *RAGPipeline) rerank(ctx context.Context, st *ragState) (*ragState, error) {
	if !p.config.EnableRerank || st.req.SkipRerank || p.reranker == nil || len(st.ranked) == 0 {
		return st, nil
	}
	passages := make([]string, len(st.ranked))
	for i, doc := range st.ranked {
		if doc != nil {
			passages[i] = doc.Content
		}
	}

	var ranked []RankedPassage
	var err error
	if scored, ok := p.reranker.(ScoredReranker); ok {
		ranked, err = scored.RerankWithScores(ctx, st.query, passages)
	} else {
		var indices []int
		indices, err = p.reranker.Rerank(ctx, st.query, passages)
		for _, idx := range indices {
			ranked = append(ranked, RankedPassage{Index: idx, Score: -1})
		}
	}
	if err != nil {
		st.resp.Metadata[MetadataKeyRerankError] = err.Error()
		return st, nil
	}
	if reordered := applyRerank(st.ranked, ranked); len(reordered) > 0 {
		st.ranked = reordered
	}
	return st, nil
}

// applyRerank reorders docs and, when a score is known, stores it on a copy of
// the document so the retrieval trace keeps the original retrieval score.
func applyRerank(docs []*schema.Document, ranked []RankedPassage) []*schema.Document {
	ordered := make([]*schema.Document, 0, len(ranked))
	seen := make(map[int]struct{}, len(ranked))
	for _, item := range ranked {
		if item.Index < 0 || item.Index >= len(docs) || docs[item.Index] == nil {
			continue
		}
		if _, dup := seen[item.Index]; dup {
			continue
		}
		seen[item.Index] = struct{}{}
		doc := docs[item.Index]
		if item.Score >= 0 {
			cp := *doc
			cp.MetaData = cloneMetadata(doc.MetaData)
			cp.MetaData[metadataKeyRetrievalScore] = doc.Score()
			cp.MetaData[metadataKeyRerankScore] = item.Score
			doc = cp.WithScore(item.Score)
		}
		ordered = append(ordered, doc)
	}
	return ordered
}

func (p *RAGPipeline) evidenceGate(ctx context.Context, st *ragState) (*ragState, error) {
	candidates := len(st.ranked)
	st.gate = evaluateEvidenceGate(st.req.Query, st.ranked)
	st.ranked = st.gate.Docs
	st.resp.Trace.RerankAfter = traceChunksFromDocs(st.ranked)
	if st.gate.Status == evidenceGateOK {
		return st, nil
	}
	st.resp.Metadata[MetadataKeyGroundingStatus] = GroundingStatusInsufficient
	st.resp.Metadata[MetadataKeyRetrievalGate] = string(st.gate.Status)
	tracing.Emit(ctx, tracing.Event{
		Type:     "guardrail",
		Stage:    "retrieval_gate",
		Summary:  st.gate.Summary,
		Metadata: map[string]any{"query": st.req.Query, "candidates": candidates, "status": string(st.gate.Status)},
	})
	return st, nil
}

func (p *RAGPipeline) reject(_ context.Context, st *ragState) (*RAGResponse, error) {
	st.resp.Answer = st.gate.Answer
	return st.resp, nil
}

func (p *RAGPipeline) buildContext(ctx context.Context, st *ragState) (*ragState, error) {
	topK := p.config.RerankTopK
	if topK <= 0 {
		topK = defaultRerankTopK
	}
	if topK > len(st.ranked) {
		topK = len(st.ranked)
	}

	var builder strings.Builder
	for i, doc := range st.ranked[:topK] {
		fmt.Fprintf(&builder, "[来源%d]", i+1)
		if name := firstStringMetadata(doc.MetaData, "source", "source_filename", "file_name", "title"); name != "" {
			fmt.Fprintf(&builder, " (%s)", name)
		}
		builder.WriteString("\n")
		builder.WriteString(doc.Content)
		builder.WriteString("\n\n")
		st.resp.Sources = append(st.resp.Sources, Source{
			Content:  doc.Content,
			DocID:    doc.ID,
			Score:    doc.Score(),
			Metadata: doc.MetaData,
		})
		st.resp.Trace.Context = append(st.resp.Trace.Context, traceChunkFromDoc(doc, i+1))
	}
	st.resp.contextText = builder.String()
	st.resp.Metadata[MetadataKeyGroundingStatus] = GroundingStatusGrounded
	tracing.Emit(ctx, tracing.Event{
		Type:     "context",
		Stage:    "context_build",
		Summary:  "built generation context",
		Metadata: contextTraceMetadata(st.resp),
	})
	return st, nil
}

func (p *RAGPipeline) generationQuery(req *RAGRequest) string {
	if req.GenerationInstruction == "" {
		return req.Query
	}
	return req.Query + "\n\n" + req.GenerationInstruction
}

func (p *RAGPipeline) generateInvoke(ctx context.Context, st *ragState, _ ...any) (*RAGResponse, error) {
	if p.generator == nil {
		return nil, errors.New("generator not configured")
	}
	started := time.Now()
	answer, err := p.generator.Generate(ctx, p.generationQuery(st.req), st.resp.contextText)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		tracing.Emit(ctx, tracing.Event{Type: "error", Stage: nodeGenerate, Summary: err.Error(), Error: err.Error(), LatencyMs: latency})
		return nil, fmt.Errorf("generate: %w", err)
	}
	emitGenerated(ctx, "answer generated", latency, len(st.resp.contextText), len(answer))
	st.resp.Answer = answer
	return st.resp, nil
}

func (p *RAGPipeline) generateStream(ctx context.Context, st *ragState, _ ...any) (*schema.StreamReader[*RAGResponse], error) {
	if p.generator == nil {
		return nil, errors.New("generator not configured")
	}
	started := time.Now()
	deltas, err := p.generator.GenerateStream(ctx, p.generationQuery(st.req), st.resp.contextText)
	if err != nil {
		tracing.Emit(ctx, tracing.Event{Type: "error", Stage: nodeGenerate, Summary: err.Error(), Error: err.Error()})
		return nil, fmt.Errorf("generate: %w", err)
	}

	reader, writer := schema.Pipe[*RAGResponse](16)
	go func() {
		defer writer.Close()
		header := *st.resp
		if closed := writer.Send(&header, nil); closed {
			drain(deltas)
			return
		}
		answerChars := 0
		for delta := range deltas {
			answerChars += len(delta)
			if closed := writer.Send(&RAGResponse{Answer: delta}, nil); closed {
				drain(deltas)
				return
			}
		}
		emitGenerated(ctx, "answer streamed", time.Since(started).Milliseconds(), len(st.resp.contextText), answerChars)
	}()
	return reader, nil
}

func drain(ch <-chan string) {
	for range ch {
	}
}

func emitGenerated(ctx context.Context, summary string, latencyMs int64, contextChars, answerChars int) {
	tracing.Emit(ctx, tracing.Event{
		Type:      "llm",
		Stage:     nodeGenerate,
		Summary:   summary,
		LatencyMs: latencyMs,
		Metadata: map[string]any{
			"context_chars":     contextChars,
			"answer_chars":      answerChars,
			"token_unavailable": true,
		},
	})
}

func contextTraceMetadata(resp *RAGResponse) map[string]any {
	return map[string]any{
		"stage":           "context_build",
		"chunks":          resp.Trace.Context,
		"count":           len(resp.Trace.Context),
		"context_chars":   len(resp.contextText),
		"context_preview": compactText(resp.contextText, 500),
	}
}

// headerChunks converts the non-answer part of a response into the stream
// protocol consumed by the chat service.
func headerChunks(req *RAGRequest, resp *RAGResponse) []StreamChunk {
	chunks := make([]StreamChunk, 0, 5+len(resp.Sources))
	query := req.Query
	if resp.RewriteQ != "" {
		query = resp.RewriteQ
		chunks = append(chunks, StreamChunk{Type: ChunkTypeRewrite, Content: resp.RewriteQ})
	}
	chunks = append(chunks,
		StreamChunk{Type: ChunkTypeTrace, Metadata: map[string]any{"stage": "retrieved_candidates", "query": query, "chunks": resp.Trace.Retrieved, "count": len(resp.Trace.Retrieved)}},
		StreamChunk{Type: ChunkTypeTrace, Metadata: map[string]any{"stage": "rerank", "before": resp.Trace.RerankBefore}},
		StreamChunk{Type: ChunkTypeTrace, Metadata: map[string]any{"stage": "rerank", "after": resp.Trace.RerankAfter}},
	)
	if gate := resp.Metadata[MetadataKeyRetrievalGate]; gate != "" {
		chunks = append(chunks, StreamChunk{Type: ChunkTypeTrace, Metadata: map[string]any{
			"stage":            "retrieval_gate",
			"status":           gate,
			"grounding_status": resp.Metadata[MetadataKeyGroundingStatus],
		}})
		return chunks
	}
	for _, source := range resp.Sources {
		chunks = append(chunks, StreamChunk{Type: ChunkTypeSource, Content: source.Content, DocID: source.DocID, Score: source.Score, Metadata: source.Metadata})
	}
	return append(chunks, StreamChunk{Type: ChunkTypeTrace, Metadata: contextTraceMetadata(resp)})
}

func concatResponses(parts []*RAGResponse) (*RAGResponse, error) {
	out := &RAGResponse{Metadata: make(map[string]string)}
	var answer strings.Builder
	for _, part := range parts {
		if part == nil {
			continue
		}
		answer.WriteString(part.Answer)
		if out.RewriteQ == "" {
			out.RewriteQ = part.RewriteQ
		}
		if len(out.Sources) == 0 {
			out.Sources = part.Sources
		}
		if out.contextText == "" {
			out.contextText = part.contextText
		}
		if len(out.Trace.Retrieved) == 0 && len(out.Trace.Context) == 0 && len(out.Trace.RerankAfter) == 0 {
			out.Trace = part.Trace
		}
		for key, value := range part.Metadata {
			out.Metadata[key] = value
		}
	}
	out.Answer = answer.String()
	return out, nil
}

// StreamChunk is one item of RunStream output.
type StreamChunk struct {
	Type     ChunkType
	Content  string
	DocID    string
	Score    float64
	Metadata map[string]any
}

// ChunkType identifies a StreamChunk kind.
type ChunkType string

const (
	ChunkTypeRewrite ChunkType = "rewrite"
	ChunkTypeSource  ChunkType = "source"
	ChunkTypeTrace   ChunkType = "trace"
	ChunkTypeContent ChunkType = "content"
	ChunkTypeDone    ChunkType = "done"
	ChunkTypeError   ChunkType = "error"
)
