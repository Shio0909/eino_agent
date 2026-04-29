package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"eino_agent/internal/container"
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
	started := time.Now()
	// Convert passages to container.Document slice
	docs := make([]*container.Document, len(passages))
	for i, p := range passages {
		docs[i] = &container.Document{ID: fmt.Sprintf("%d", i), Content: p}
	}

	// Call the underlying provider (returns top docs sorted by score)
	ranked, err := a.provider.Rerank(ctx, query, docs, len(passages))
	if err != nil {
		tracing.Emit(ctx, tracing.Event{Type: "rerank", Stage: "rerank_scores", Level: "error", Error: err.Error(), LatencyMs: time.Since(started).Milliseconds(), Metadata: map[string]any{"passage_count": len(passages)}})
		return nil, err
	}

	// Build a score map from the ranked results
	scoreMap := make(map[int]float64, len(ranked))
	for _, doc := range ranked {
		// Find original index by matching content
		for i, p := range passages {
			if doc.Content == p {
				if _, exists := scoreMap[i]; !exists {
					scoreMap[i] = doc.Score
				}
				break
			}
		}
	}

	// Build indices sorted by score descending
	type scored struct {
		idx   int
		score float64
	}
	items := make([]scored, 0, len(passages))
	for i := range passages {
		s, ok := scoreMap[i]
		if !ok {
			s = 0
		}
		items = append(items, scored{idx: i, score: s})
	}
	sort.Slice(items, func(a, b int) bool {
		return items[a].score > items[b].score
	})

	result := make([]int, len(items))
	traceItems := make([]map[string]any, 0, len(items))
	for i, it := range items {
		result[i] = it.idx
		traceItems = append(traceItems, map[string]any{
			"rank":           i + 1,
			"original_index": it.idx,
			"score":          it.score,
			"preview":        previewPassage(passages[it.idx]),
		})
	}
	tracing.Emit(ctx, tracing.Event{Type: "rerank", Stage: "rerank_scores", Summary: "rerank score mapping", LatencyMs: time.Since(started).Milliseconds(), Metadata: map[string]any{"query": query, "passage_count": len(passages), "scores": traceItems}})
	return result, nil
}

func previewPassage(passage string) string {
	trimmed := strings.TrimSpace(passage)
	runes := []rune(trimmed)
	if len(runes) > 160 {
		return string(runes[:160]) + "..."
	}
	return trimmed
}
