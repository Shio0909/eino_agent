package pipeline

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
)

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
	metadata := cloneMetadata(doc.MetaData)
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

func cloneMetadata(metadata map[string]any) map[string]any {
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func stringMetadata(metadata map[string]interface{}, key string) string {
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

type nodeStartKey struct{ name string }

// newTimingHandler logs per-node latency for every graph run. Stage-specific
// trace events carrying payloads are emitted by the nodes themselves.
func newTimingHandler() callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackInput) context.Context {
			if info == nil {
				return ctx
			}
			return context.WithValue(ctx, nodeStartKey{info.Name}, time.Now())
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackOutput) context.Context {
			logNodeDuration(ctx, info, "")
			return ctx
		}).
		OnEndWithStreamOutputFn(func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
			output.Close()
			logNodeDuration(ctx, info, " (stream opened)")
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			logNodeDuration(ctx, info, " error="+err.Error())
			return ctx
		}).
		Build()
}

func logNodeDuration(ctx context.Context, info *callbacks.RunInfo, suffix string) {
	if info == nil {
		return
	}
	started, ok := ctx.Value(nodeStartKey{info.Name}).(time.Time)
	if !ok {
		return
	}
	log.Printf("[Timing][Pipeline] node=%s duration_ms=%d%s", info.Name, time.Since(started).Milliseconds(), suffix)
}
