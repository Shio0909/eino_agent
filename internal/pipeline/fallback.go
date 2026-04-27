package pipeline

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
)

func (p *RAGPipeline) assessKnowledgeResults(query string, docs []*schema.Document) FallbackDecision {
	cfg := p.fallbackConfig()
	contextChars := totalDocumentChars(docs)
	decision := FallbackDecision{
		State:        StateKBAssess,
		Reason:       FallbackReasonNone,
		DocsCount:    len(docs),
		ContextChars: contextChars,
	}
	if !cfg.Enabled || p.fallback == nil {
		return decision
	}
	minDocs := cfg.MinKnowledgeDocs
	if minDocs <= 0 {
		minDocs = 1
	}
	if len(docs) < minDocs {
		decision.Reason = FallbackReasonNoDocuments
		decision.AllowExternal = true
		decision.Providers = p.planExternalProviders(query)
		return decision
	}
	if cfg.MinContextChars > 0 && contextChars < cfg.MinContextChars {
		decision.Reason = FallbackReasonShortContext
		decision.AllowExternal = true
		decision.Providers = p.planExternalProviders(query)
	}
	return decision
}

func (p *RAGPipeline) fallbackConfig() FallbackConfig {
	cfg := p.config.Fallback
	if cfg.MinKnowledgeDocs <= 0 {
		cfg.MinKnowledgeDocs = 1
	}
	if cfg.MaxExternalResults <= 0 {
		cfg.MaxExternalResults = 5
	}
	if cfg.MaxExternalContext <= 0 {
		cfg.MaxExternalContext = 3
	}
	if len(cfg.AllowedProviders) == 0 {
		cfg.AllowedProviders = []string{"web"}
	}
	return cfg
}

func (p *RAGPipeline) planExternalProviders(query string) []string {
	cfg := p.fallbackConfig()
	allowed := make(map[string]struct{}, len(cfg.AllowedProviders))
	for _, provider := range cfg.AllowedProviders {
		provider = strings.TrimSpace(strings.ToLower(provider))
		if provider != "" {
			allowed[provider] = struct{}{}
		}
	}
	var planned []string
	lowerQuery := strings.ToLower(query)
	keywords := cfg.ProviderByKeyword
	if len(keywords) == 0 {
		keywords = map[string][]string{
			"github":   {"github", "repo", "仓库", "issue", "pr", "代码"},
			"bilibili": {"b站", "bilibili", "视频", "up主", "教程", "课程"},
			"xhs":      {"小红书", "xhs", "笔记", "种草", "攻略"},
		}
	}
	for provider, words := range keywords {
		provider = strings.TrimSpace(strings.ToLower(provider))
		if _, ok := allowed[provider]; !ok {
			continue
		}
		for _, word := range words {
			if strings.Contains(lowerQuery, strings.ToLower(word)) || strings.Contains(query, word) {
				planned = append(planned, provider)
				break
			}
		}
	}
	if _, ok := allowed["web"]; ok {
		planned = append(planned, "web")
	}
	return uniqueStrings(planned)
}

func totalDocumentChars(docs []*schema.Document) int {
	total := 0
	for _, doc := range docs {
		if doc != nil {
			total += len(strings.TrimSpace(doc.Content))
		}
	}
	return total
}

func externalResultsToDocuments(results []ExternalSearchResult, limit int) []*schema.Document {
	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}
	docs := make([]*schema.Document, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, result := range rankExternalResults(results) {
		if len(docs) >= limit {
			break
		}
		content := strings.TrimSpace(result.Content)
		if content == "" {
			content = strings.TrimSpace(result.Title)
		}
		if content == "" {
			continue
		}
		key := strings.TrimSpace(result.URL)
		if key == "" {
			key = strings.ToLower(result.Provider + ":" + result.Title)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		metadata := map[string]any{
			"source_type": "external",
			"provider":    result.Provider,
			"title":       result.Title,
			"url":         result.URL,
		}
		for key, value := range result.Metadata {
			metadata[key] = value
		}
		docID := result.URL
		if docID == "" {
			docID = fmt.Sprintf("external:%s:%d", result.Provider, len(docs)+1)
		}
		docs = append(docs, &schema.Document{ID: docID, Content: content, MetaData: metadata})
	}
	return docs
}

func rankExternalResults(results []ExternalSearchResult) []ExternalSearchResult {
	out := append([]ExternalSearchResult(nil), results...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Score > out[j].Score
	})
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
