package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"eino_agent/internal/pipeline"
)

func (t *WebSearchTool) Search(ctx context.Context, req pipeline.ExternalSearchRequest) ([]pipeline.ExternalSearchResult, error) {
	if len(req.Providers) > 0 && !containsProvider(req.Providers, "web") {
		return nil, nil
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	output, err := t.InvokableRun(ctx, mustJSON(WebSearchInput{Query: query}))
	if err != nil {
		return nil, err
	}
	var parsed WebSearchOutput
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return nil, fmt.Errorf("parse web search output: %w", err)
	}
	limit := req.MaxResults
	if limit <= 0 || limit > len(parsed.Results) {
		limit = len(parsed.Results)
	}
	results := make([]pipeline.ExternalSearchResult, 0, limit)
	for i, result := range parsed.Results {
		if i >= limit {
			break
		}
		results = append(results, pipeline.ExternalSearchResult{
			Provider: "web",
			Title:    result.Title,
			URL:      result.URL,
			Content:  result.Content,
			Score:    float64(limit - i),
			Metadata: map[string]interface{}{"rank": i + 1},
		})
	}
	return results, nil
}

func containsProvider(providers []string, target string) bool {
	for _, provider := range providers {
		if strings.EqualFold(strings.TrimSpace(provider), target) {
			return true
		}
	}
	return false
}

func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

var _ pipeline.ExternalFallbackProvider = (*WebSearchTool)(nil)
