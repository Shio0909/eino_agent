package service

import (
	"context"
	"fmt"
	"sort"

	"eino_agent/internal/container"
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
	// Convert passages to container.Document slice
	docs := make([]*container.Document, len(passages))
	for i, p := range passages {
		docs[i] = &container.Document{ID: fmt.Sprintf("%d", i), Content: p}
	}

	// Call the underlying provider (returns top docs sorted by score)
	ranked, err := a.provider.Rerank(ctx, query, docs, len(passages))
	if err != nil {
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
	for i, it := range items {
		result[i] = it.idx
	}
	return result, nil
}
