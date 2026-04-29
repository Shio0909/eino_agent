package pipeline

import (
	"context"
	"strings"
	"unicode"

	"github.com/cloudwego/eino/schema"
)

func (p *RAGPipeline) queryExpansionConfig() QueryExpansionConfig {
	cfg := p.config.QueryExpansion
	if !cfg.Enabled {
		cfg.Enabled = p.config.EnableQueryExpansion
	}
	if cfg.MinDocs <= 0 {
		cfg.MinDocs = p.fallbackConfig().MinKnowledgeDocs
	}
	if cfg.MaxQueries <= 0 {
		cfg.MaxQueries = 5
	}
	if cfg.MaxTotalDocs <= 0 {
		cfg.MaxTotalDocs = maxInt(p.config.TopK*2, cfg.MinDocs*2)
	}
	return cfg
}

func (p *RAGPipeline) shouldExpandQuery(docs []*schema.Document) bool {
	cfg := p.queryExpansionConfig()
	if !cfg.Enabled || p.retriever == nil {
		return false
	}
	if len(docs) < cfg.MinDocs {
		return true
	}
	if cfg.MinContextChars > 0 && totalDocumentChars(docs) < cfg.MinContextChars {
		return true
	}
	if cfg.MinScore > 0 && maxDocumentScore(docs) < cfg.MinScore {
		return true
	}
	return false
}

func (p *RAGPipeline) expandAndRetrieve(ctx context.Context, originalQuery, retrievalQuery string, docs []*schema.Document) ([]*schema.Document, []string, error) {
	cfg := p.queryExpansionConfig()
	queries := buildQueryExpansions(originalQuery, retrievalQuery, cfg.MaxQueries)
	if len(queries) == 0 {
		return docs, nil, nil
	}

	merged := append([]*schema.Document(nil), docs...)
	seen := seenDocumentKeys(merged)
	usedQueries := make([]string, 0, len(queries))
	for _, query := range queries {
		if len(merged) >= cfg.MaxTotalDocs {
			break
		}
		results, err := p.retriever.Retrieve(ctx, query)
		if err != nil {
			return merged, usedQueries, err
		}
		used := false
		for _, doc := range results {
			if doc == nil {
				continue
			}
			key := documentDedupeKey(doc)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, doc)
			used = true
			if len(merged) >= cfg.MaxTotalDocs {
				break
			}
		}
		if used {
			usedQueries = append(usedQueries, query)
		}
	}
	return merged, usedQueries, nil
}

func buildQueryExpansions(originalQuery, retrievalQuery string, limit int) []string {
	seen := map[string]struct{}{}
	for _, query := range []string{originalQuery, retrievalQuery} {
		query = strings.TrimSpace(query)
		if query != "" {
			seen[strings.ToLower(query)] = struct{}{}
		}
	}

	queries := make([]string, 0, limit)
	add := func(query string) {
		if limit > 0 && len(queries) >= limit {
			return
		}
		query = strings.TrimSpace(query)
		if len([]rune(query)) < 2 {
			return
		}
		key := strings.ToLower(query)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		queries = append(queries, query)
	}

	base := strings.TrimSpace(retrievalQuery)
	if base == "" {
		base = strings.TrimSpace(originalQuery)
	}
	cleaned := removeExpansionQuestionWords(base)
	keywords := extractExpansionKeywords(cleaned)
	if len(keywords) >= 2 {
		add(strings.Join(keywords, " "))
	}
	for _, segment := range splitExpansionSegments(cleaned) {
		add(segment)
	}
	if cleaned != base {
		add(cleaned)
	}
	return queries
}

func extractExpansionKeywords(query string) []string {
	tokens := tokenizeExpansionQuery(query)
	keywords := make([]string, 0, len(tokens))
	for _, token := range tokens {
		lower := strings.ToLower(token)
		if _, ok := expansionStopwords[lower]; ok {
			continue
		}
		if len([]rune(token)) > 1 || containsASCII(token) {
			keywords = append(keywords, token)
		}
	}
	return keywords
}

func tokenizeExpansionQuery(query string) []string {
	tokens := make([]string, 0)
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, r := range query {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' || r == '/' || r == '#' || r == '+' {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func splitExpansionSegments(query string) []string {
	fields := strings.FieldsFunc(query, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '、', '。', '？', '?', '！', '!', '\n', '\t', '(', ')', '（', '）', '[', ']', '【', '】':
			return true
		default:
			return false
		}
	})
	segments := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len([]rune(field)) >= 3 {
			segments = append(segments, field)
		}
	}
	return segments
}

func removeExpansionQuestionWords(query string) string {
	query = strings.TrimSpace(query)
	for _, prefix := range expansionQuestionPrefixes {
		if strings.HasPrefix(query, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(query, prefix))
		}
	}
	return query
}

func seenDocumentKeys(docs []*schema.Document) map[string]struct{} {
	seen := make(map[string]struct{}, len(docs))
	for _, doc := range docs {
		if doc != nil {
			seen[documentDedupeKey(doc)] = struct{}{}
		}
	}
	return seen
}

func documentDedupeKey(doc *schema.Document) string {
	if doc.ID != "" {
		return "id:" + doc.ID
	}
	return "content:" + strings.TrimSpace(doc.Content)
}

func maxDocumentScore(docs []*schema.Document) float64 {
	maxScore := 0.0
	for _, doc := range docs {
		if doc != nil && doc.Score() > maxScore {
			maxScore = doc.Score()
		}
	}
	return maxScore
}

func containsASCII(value string) bool {
	for _, r := range value {
		if r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var expansionStopwords = map[string]struct{}{
	"的": {}, "是": {}, "了": {}, "和": {}, "与": {}, "或": {}, "在": {},
	"配置": {}, "查询": {}, "检索": {}, "回答": {}, "介绍": {}, "说明": {}, "一下": {},
	"a": {}, "an": {}, "the": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"to": {}, "of": {}, "in": {}, "for": {}, "on": {}, "with": {}, "at": {}, "by": {}, "from": {},
	"what": {}, "how": {}, "why": {}, "when": {}, "where": {}, "which": {}, "who": {},
}

var expansionQuestionPrefixes = []string{
	"什么是", "请问", "请告诉我", "帮我查一下", "帮我找", "帮我", "我想知道", "我想了解", "如何", "怎么", "怎样", "为什么", "为何", "哪个", "哪些", "谁", "何时", "何地",
}
