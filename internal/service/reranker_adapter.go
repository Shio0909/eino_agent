package service

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"eino_agent/internal/container"
	"eino_agent/internal/pipeline"
	"eino_agent/internal/tracing"
)

// rerankerAdapter adapts container.RerankerProvider to pipeline.Reranker interface.
type rerankerAdapter struct {
	provider container.RerankerProvider
}

// NewRerankerAdapter creates an adapter from container.RerankerProvider to pipeline.Reranker.
func NewRerankerAdapter(provider container.RerankerProvider) *rerankerAdapter {
	return &rerankerAdapter{provider: provider}
}

func (a *rerankerAdapter) Rerank(ctx context.Context, query string, passages []string) ([]int, error) {
	ranked, err := a.RerankWithScores(ctx, query, passages)
	if err != nil {
		return nil, err
	}
	indices := make([]int, len(ranked))
	for i, item := range ranked {
		indices[i] = item.Index
	}
	return indices, nil
}

// RerankWithScores maps provider results back to passage indices through the
// document ID, so duplicate passages keep distinct scores.
func (a *rerankerAdapter) RerankWithScores(ctx context.Context, query string, passages []string) ([]pipeline.RankedPassage, error) {
	started := time.Now()
	docs := make([]*container.Document, len(passages))
	for i, p := range passages {
		docs[i] = &container.Document{ID: strconv.Itoa(i), Content: p}
	}

	ranked, err := a.provider.Rerank(ctx, query, docs, len(passages))
	if err != nil {
		tracing.Emit(ctx, tracing.Event{Type: "rerank", Stage: "rerank_scores", Level: "error", Error: err.Error(), LatencyMs: time.Since(started).Milliseconds(), Metadata: map[string]any{"passage_count": len(passages)}})
		return nil, err
	}

	scores := make(map[int]float64, len(ranked))
	for _, doc := range ranked {
		if doc == nil {
			continue
		}
		idx, convErr := strconv.Atoi(doc.ID)
		if convErr != nil || idx < 0 || idx >= len(passages) {
			continue
		}
		if _, exists := scores[idx]; !exists {
			scores[idx] = doc.Score
		}
	}

	items := make([]pipeline.RankedPassage, 0, len(passages))
	for i := range passages {
		items = append(items, pipeline.RankedPassage{Index: i, Score: scores[i]})
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].Score > items[b].Score })

	traceItems := make([]map[string]any, 0, len(items))
	for i, it := range items {
		traceItems = append(traceItems, map[string]any{
			"rank":           i + 1,
			"original_index": it.Index,
			"score":          it.Score,
			"preview":        previewPassage(passages[it.Index]),
		})
	}
	tracing.Emit(ctx, tracing.Event{Type: "rerank", Stage: "rerank_scores", Summary: "rerank score mapping", LatencyMs: time.Since(started).Milliseconds(), Metadata: map[string]any{"query": query, "passage_count": len(passages), "scores": traceItems}})
	return items, nil
}

func previewPassage(passage string) string {
	trimmed := strings.TrimSpace(passage)
	runes := []rune(trimmed)
	if len(runes) > 160 {
		return string(runes[:160]) + "..."
	}
	return trimmed
}
