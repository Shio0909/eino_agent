// Package container - Retriever 提供者实现
//
// 【Eino 特点】组合向量检索和关键词检索
package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	einoembedding "github.com/cloudwego/eino/components/embedding"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/config"
	"eino_agent/internal/retrieval"
	"eino_agent/internal/tracing"
)

// CompositeRetriever 组合检索器
// 【Eino 特点】支持向量检索 + 关键词检索 + 图谱检索的混合模式
type CompositeRetriever struct {
	cfg                   *config.RAGConfig
	embeddingCfg          *config.EmbeddingConfig
	embedding             einoembedding.Embedder
	vectorDB              VectorDBProvider
	retrievalCache        cachepkg.RetrievalCache
	graphRetriever        einoretriever.Retriever
	graphRetrieverFactory GraphRetrieverFactory // 用于按 KB 动态创建图谱检索器
	filter                SearchFilter
}

// GraphRetrieverFactory 按命名空间创建图谱检索器（由 graphRAG Service 实现）
type GraphRetrieverFactory interface {
	CreateScopedGraphRetriever(knowledgeBaseID string, topK int) einoretriever.Retriever
}

// NewRetrieverProvider 创建检索器提供者
func NewRetrieverProvider(
	ctx context.Context,
	cfg *config.RAGConfig,
	embeddingCfg *config.EmbeddingConfig,
	embedding einoembedding.Embedder,
	vectorDB VectorDBProvider,
	retrievalCache cachepkg.RetrievalCache,
) (einoretriever.Retriever, CleanupFunc, error) {
	retriever := &CompositeRetriever{
		cfg:            cfg,
		embeddingCfg:   embeddingCfg,
		embedding:      embedding,
		vectorDB:       vectorDB,
		retrievalCache: retrievalCache,
	}
	return retriever, nil, nil
}

// SetGraphRetriever 注入图谱检索器（全局兜底，当无法按 KB 动态创建时使用）
func (r *CompositeRetriever) SetGraphRetriever(gr einoretriever.Retriever) {
	r.graphRetriever = gr
}

// SetGraphRetrieverFactory 注入图谱检索器工厂（按 KB 动态创建作用域图谱检索器）
func (r *CompositeRetriever) SetGraphRetrieverFactory(f GraphRetrieverFactory) {
	r.graphRetrieverFactory = f
}

// Retrieve 执行检索。retrieval.WithMode 可指定 semantic / exact / graph，
// 其余（含未设置、auto 和未知取值）走默认策略。
func (r *CompositeRetriever) Retrieve(ctx context.Context, query string, opts ...einoretriever.Option) ([]*schema.Document, error) {
	switch mode := retrieval.ModeFromOptions(opts...); mode {
	case retrieval.ModeSemantic, retrieval.ModeExact, retrieval.ModeGraph:
		return r.retrieveByMode(ctx, query, mode)
	default:
		return r.retrieveAuto(ctx, query, opts...)
	}
}

// retrieveAuto 是默认策略：开启 hybrid 时走 RRF 融合，否则纯向量。
func (r *CompositeRetriever) retrieveAuto(ctx context.Context, query string, opts ...einoretriever.Option) ([]*schema.Document, error) {
	if r.cfg.EnableHybrid {
		return r.retrieveWithHybrid(ctx, query, opts...)
	}

	topK := r.cfg.TopK
	if topK <= 0 {
		topK = 10
	}

	tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "retrieval_mode", Summary: "semantic", Metadata: map[string]any{"mode": "semantic", "top_k": topK}})
	queryVector, err := r.getQueryEmbedding(ctx, query)
	if err != nil {
		tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "embedding", Level: "error", Error: err.Error(), Metadata: map[string]any{"query": query}})
		return nil, fmt.Errorf("生成查询向量失败: %w", err)
	}

	vectorStart := time.Now()
	docs, err := r.vectorDB.Search(ctx, queryVector, topK*2, r.filter)
	tracing.Emit(ctx, retrievalSourceEvent("vector", query, docs, vectorStart, err))
	if err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}

	if len(docs) > topK {
		docs = docs[:topK]
	}

	return toSchemaDocuments(docs), nil
}

func (r *CompositeRetriever) retrieveWithHybrid(ctx context.Context, query string, opts ...einoretriever.Option) ([]*schema.Document, error) {
	topK := r.cfg.TopK
	if topK <= 0 {
		topK = 10
	}
	tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "retrieval_mode", Summary: "hybrid", Metadata: map[string]any{"mode": "hybrid", "top_k": topK, "candidate_k": topK * 2}})

	queryVector, err := r.getQueryEmbedding(ctx, query)
	if err != nil {
		tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "embedding", Level: "error", Error: err.Error(), Metadata: map[string]any{"query": query}})
		return nil, fmt.Errorf("生成查询向量失败: %w", err)
	}

	vectorStart := time.Now()
	vectorDocs, err := r.vectorDB.Search(ctx, queryVector, topK*2, r.filter)
	tracing.Emit(ctx, retrievalSourceEvent("vector", query, vectorDocs, vectorStart, err))
	if err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}

	keywordStart := time.Now()
	keywordDocs, err := r.vectorDB.SearchKeyword(ctx, query, topK*2, r.filter)
	tracing.Emit(ctx, retrievalSourceEvent("keyword", query, keywordDocs, keywordStart, err))
	if err != nil {
		log.Printf("[Retriever] 关键词检索失败（降级继续）: %v", err)
		keywordDocs = nil
	}

	// 图谱检索（如果启用）
	// 图谱不参与 RRF 融合，而是作为独立的强信号源：
	//   - 图谱命中的 chunk ID 如果在 RRF 结果中存在，给对应文档加分（多信号确认）
	//   - graph_context（实体/关系摘要）直接注入最终结果，作为 LLM 的附加上下文
	var graphChunkIDs map[string]struct{}
	var graphContextDoc *Document
	if r.graphRetriever != nil {
		graphStart := time.Now()
		gDocs, gErr := r.graphRetriever.Retrieve(ctx, query)
		tracing.Emit(ctx, graphRetrievalEvent(query, gDocs, graphStart, gErr))
		if gErr != nil {
			log.Printf("[Retriever] 图谱检索失败（降级继续）: %v", gErr)
		} else {
			graphChunkIDs = make(map[string]struct{})
			for _, d := range gDocs {
				if d == nil {
					continue
				}
				// graph-context 是实体/关系的摘要文档，直接注入最终结果
				if d.ID == "graph-context" && strings.TrimSpace(d.Content) != "" {
					graphContextDoc = &Document{
						ID:       d.ID,
						Content:  d.Content,
						Metadata: d.MetaData,
					}
					continue
				}
				// 收集图谱命中的 chunk ID（不填充 Content，由向量/关键词检索结果补充）
				if d.ID != "" {
					graphChunkIDs[d.ID] = struct{}{}
				}
			}
		}
	} else {
		tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "graph_recall", Summary: "graph retriever unavailable", Metadata: map[string]any{"source": "graph", "available": false}})
	}

	fused := r.rrfFuse(ctx, vectorDocs, keywordDocs, topK)

	// 图谱信号增强：RRF 结果中与图谱命中重叠的文档获得得分加成（多源确认信号）
	if len(graphChunkIDs) > 0 {
		for _, doc := range fused {
			if _, ok := graphChunkIDs[doc.ID]; ok {
				doc.Score = clampScore(doc.Score * 1.2)
				if doc.Metadata == nil {
					doc.Metadata = map[string]interface{}{}
				}
				if mt, _ := doc.Metadata["match_type"].(string); mt == "vector" || mt == "keyword" {
					doc.Metadata["match_type"] = "hybrid"
				}
				doc.Metadata["graph_boost"] = true
			}
		}
		// 重新按得分排序
		sort.Slice(fused, func(i, j int) bool {
			return fused[i].Score > fused[j].Score
		})
	}

	// 将图谱上下文作为补充信息追加到检索结果中（不占 topK 名额）
	if graphContextDoc != nil {
		if graphContextDoc.Metadata == nil {
			graphContextDoc.Metadata = map[string]interface{}{}
		}
		graphContextDoc.Metadata["match_type"] = "graph_context"
		fused = append(fused, graphContextDoc)
		log.Printf("[Retriever] 已注入图谱上下文文档（%d 字符）", len(graphContextDoc.Content))
	}

	return toSchemaDocuments(fused), nil
}

// WithKnowledgeBaseScope 返回限定到指定知识库的检索器；ids 为空时返回受限且无结果的检索器。
func (r *CompositeRetriever) WithKnowledgeBaseScope(ids []string) einoretriever.Retriever {
	return r.WithRetrievalScope(ids, nil)
}

// WithRetrievalScope 返回限定到指定知识库和/或文档的检索器。范围由存储层执行，
// 所以被选中的文档即使排不进整个库的前 topK 也能检索到；两者都给时取交集。
func (r *CompositeRetriever) WithRetrievalScope(kbIDs, docIDs []string) einoretriever.Retriever {
	return r.WithScope(SearchFilter{KnowledgeBaseIDs: kbIDs, KnowledgeIDs: docIDs, Scoped: true})
}

// WithScope 返回带存储层过滤条件的检索器副本。
func (r *CompositeRetriever) WithScope(filter SearchFilter) *CompositeRetriever {
	filter = filter.Normalized()
	clone := *r
	clone.filter = filter

	if r.graphRetrieverFactory != nil && len(filter.KnowledgeBaseIDs) > 0 {
		topK := r.cfg.TopK
		if topK <= 0 {
			topK = 10
		}
		// 图谱按知识库命名空间隔离，只能针对单个 KB 建立作用域；多 KB 时禁用图谱召回，避免跨库泄漏。
		if len(filter.KnowledgeBaseIDs) == 1 {
			clone.graphRetriever = r.graphRetrieverFactory.CreateScopedGraphRetriever(filter.KnowledgeBaseIDs[0], topK)
		} else {
			clone.graphRetriever = nil
		}
	} else if !filter.IsZero() {
		clone.graphRetriever = nil
	}
	return &clone
}

func (r *CompositeRetriever) getQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	started := time.Now()
	if r.retrievalCache != nil {
		queryHash := hashQuery(query)
		modelID := r.embeddingModelID()
		cachedVector, hit, err := r.retrievalCache.GetEmbedding(ctx, modelID, queryHash)
		if err != nil {
			log.Printf("[Retriever] 读取 embedding 缓存失败，降级到实时向量化: %v", err)
			tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "embedding_cache", Level: "warning", Summary: "embedding cache read failed", Error: err.Error(), Metadata: map[string]any{"model": modelID, "query_hash": queryHash}})
		} else if hit && len(cachedVector) > 0 {
			log.Printf("[Cache][Embedding] hit model=%s", modelID)
			tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "embedding", Summary: "cache hit", LatencyMs: time.Since(started).Milliseconds(), Metadata: map[string]any{"model": modelID, "query_hash": queryHash, "cache_hit": true, "dimensions": len(cachedVector)}})
			return cachedVector, nil
		} else {
			log.Printf("[Cache][Embedding] miss model=%s", modelID)
			tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "embedding_cache", Summary: "cache miss", Metadata: map[string]any{"model": modelID, "query_hash": queryHash, "cache_hit": false}})
		}

		vector, err := EmbedFloat32(ctx, r.embedding, query)
		if err != nil {
			tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "embedding", Level: "error", Error: err.Error(), LatencyMs: time.Since(started).Milliseconds(), Metadata: map[string]any{"model": modelID, "query_hash": queryHash, "cache_hit": false}})
			return nil, err
		}
		if err := r.retrievalCache.SetEmbedding(ctx, modelID, queryHash, vector, r.embeddingCacheTTL()); err != nil {
			log.Printf("[Retriever] 写入 embedding 缓存失败，继续使用实时结果: %v", err)
			tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "embedding_cache", Level: "warning", Summary: "embedding cache write failed", Error: err.Error(), Metadata: map[string]any{"model": modelID, "query_hash": queryHash}})
		}
		tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "embedding", Summary: "generated", LatencyMs: time.Since(started).Milliseconds(), Metadata: map[string]any{"model": modelID, "query_hash": queryHash, "cache_hit": false, "dimensions": len(vector)}})
		return vector, nil
	}

	vector, err := EmbedFloat32(ctx, r.embedding, query)
	metadata := map[string]any{"model": r.embeddingModelID(), "cache_enabled": false}
	if err != nil {
		tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "embedding", Level: "error", Error: err.Error(), LatencyMs: time.Since(started).Milliseconds(), Metadata: metadata})
		return nil, err
	}
	metadata["dimensions"] = len(vector)
	tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "embedding", Summary: "generated", LatencyMs: time.Since(started).Milliseconds(), Metadata: metadata})
	return vector, nil
}

func (r *CompositeRetriever) embeddingModelID() string {
	if r.embeddingCfg != nil && strings.TrimSpace(r.embeddingCfg.ModelID) != "" {
		return strings.TrimSpace(r.embeddingCfg.ModelID)
	}
	return "default"
}

func (r *CompositeRetriever) embeddingCacheTTL() time.Duration {
	minutes := r.cfg.EmbeddingCacheTTLMinutes
	if minutes <= 0 {
		minutes = 1440
	}
	return time.Duration(minutes) * time.Minute
}

func hashQuery(query string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(query)))
	return hex.EncodeToString(sum[:])
}

func toSchemaDocuments(docs []*Document) []*schema.Document {
	result := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		result = append(result, (&schema.Document{
			ID:       doc.ID,
			Content:  doc.Content,
			MetaData: doc.Metadata,
		}).WithScore(doc.Score))
	}
	return result
}

func (r *CompositeRetriever) rrfFuse(ctx context.Context, vectorDocs, keywordDocs []*Document, topK int) []*Document {
	const rrfK = 60.0

	// 各检索源权重（向量通常最稳定，关键词补充精确匹配）
	sourceWeights := map[string]float64{
		"vector":  1.0,
		"keyword": 0.8,
	}

	type rankedDoc struct {
		doc           *Document
		score         float64
		hits          int
		contributions []map[string]any
	}

	ranked := make(map[string]*rankedDoc)

	add := func(docs []*Document, source string) {
		weight := sourceWeights[source]
		if weight == 0 {
			weight = 1.0
		}
		for rank, doc := range docs {
			if doc == nil || doc.ID == "" {
				continue
			}
			entry, exists := ranked[doc.ID]
			if !exists {
				entry = &rankedDoc{doc: doc}
				ranked[doc.ID] = entry
			} else if entry.doc.Content == "" && doc.Content != "" {
				entry.doc = doc
			}
			contribution := weight * 1.0 / (rrfK + float64(rank+1))
			entry.score += contribution
			entry.hits++
			entry.contributions = append(entry.contributions, map[string]any{
				"source":       source,
				"rank":         rank + 1,
				"weight":       weight,
				"contribution": contribution,
				"source_score": doc.Score,
			})

			if entry.doc.Metadata == nil {
				entry.doc.Metadata = map[string]interface{}{}
			}
			if current, ok := entry.doc.Metadata["match_type"].(string); ok && current != "" && current != source {
				entry.doc.Metadata["match_type"] = "hybrid"
			} else if _, ok := entry.doc.Metadata["match_type"]; !ok {
				entry.doc.Metadata["match_type"] = source
			}
		}
	}

	add(vectorDocs, "vector")
	add(keywordDocs, "keyword")

	list := make([]*rankedDoc, 0, len(ranked))
	for _, item := range ranked {
		item.doc.Score = item.score
		if item.hits > 1 {
			if item.doc.Metadata == nil {
				item.doc.Metadata = map[string]interface{}{}
			}
			item.doc.Metadata["match_type"] = "hybrid"
		}
		// 空内容文档：仅跳过单源命中的（图谱独占但无内容的 chunk）
		// 多源命中意味着 ID 被多种检索方式确认，即使内容暂空也保留其得分排位
		if strings.TrimSpace(item.doc.Content) == "" {
			if item.hits <= 1 {
				continue
			}
			log.Printf("[RRF] 保留多源命中的空内容文档 id=%s hits=%d score=%.4f", item.doc.ID, item.hits, item.score)
		}
		list = append(list, item)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].score > list[j].score
	})

	if topK > len(list) {
		topK = len(list)
	}

	fused := make([]*Document, 0, topK)
	breakdown := make([]map[string]any, 0, topK)
	for i := 0; i < topK; i++ {
		item := list[i]
		if item.doc.Metadata == nil {
			item.doc.Metadata = map[string]interface{}{}
		}
		item.doc.Metadata["rrf_score"] = item.score
		item.doc.Metadata["rrf_hits"] = item.hits
		item.doc.Metadata["rrf_contributions"] = item.contributions
		fused = append(fused, item.doc)
		breakdown = append(breakdown, map[string]any{
			"rank":          i + 1,
			"doc_id":        item.doc.ID,
			"score":         item.score,
			"hits":          item.hits,
			"match_type":    item.doc.Metadata["match_type"],
			"contributions": item.contributions,
		})
	}

	tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "rrf", Summary: "RRF fusion", Metadata: map[string]any{
		"rrf_k":          rrfK,
		"top_k":          topK,
		"source_weights": sourceWeights,
		"input_counts": map[string]int{
			"vector":  len(vectorDocs),
			"keyword": len(keywordDocs),
		},
		"results": breakdown,
	}})

	return fused
}

func retrievalSourceEvent(source, query string, docs []*Document, started time.Time, err error) tracing.Event {
	metadata := map[string]any{
		"source":    source,
		"query":     query,
		"count":     len(docs),
		"doc_ids":   documentIDs(docs, 10),
		"top_docs":  documentTraceItems(docs, 10),
		"available": true,
	}
	event := tracing.Event{Type: "retrieval", Stage: source + "_recall", Summary: fmt.Sprintf("%s recall", source), LatencyMs: time.Since(started).Milliseconds(), Metadata: metadata}
	if err != nil {
		event.Level = "error"
		event.Error = err.Error()
		metadata["error"] = err.Error()
	}
	return event
}

func graphRetrievalEvent(query string, docs []*schema.Document, started time.Time, err error) tracing.Event {
	metadata := map[string]any{
		"source":    "graph",
		"query":     query,
		"count":     len(docs),
		"doc_ids":   schemaDocumentIDs(docs, 10),
		"top_docs":  schemaDocumentTraceItems(docs, 10),
		"available": true,
	}
	event := tracing.Event{Type: "retrieval", Stage: "graph_recall", Summary: "graph recall", LatencyMs: time.Since(started).Milliseconds(), Metadata: metadata}
	if err != nil {
		event.Level = "error"
		event.Error = err.Error()
		metadata["error"] = err.Error()
	}
	return event
}

func documentIDs(docs []*Document, limit int) []string {
	ids := make([]string, 0, boundedDocLimit(len(docs), limit))
	for _, doc := range docs {
		if doc == nil || doc.ID == "" {
			continue
		}
		ids = append(ids, doc.ID)
		if len(ids) >= limit {
			break
		}
	}
	return ids
}

func schemaDocumentIDs(docs []*schema.Document, limit int) []string {
	ids := make([]string, 0, boundedDocLimit(len(docs), limit))
	for _, doc := range docs {
		if doc == nil || doc.ID == "" {
			continue
		}
		ids = append(ids, doc.ID)
		if len(ids) >= limit {
			break
		}
	}
	return ids
}

func documentTraceItems(docs []*Document, limit int) []map[string]any {
	items := make([]map[string]any, 0, boundedDocLimit(len(docs), limit))
	for index, doc := range docs {
		if doc == nil {
			continue
		}
		items = append(items, map[string]any{
			"rank":       index + 1,
			"doc_id":     doc.ID,
			"score":      doc.Score,
			"source":     firstMetadataString(doc.Metadata, "source", "source_filename", "file_name", "wiki_path"),
			"match_type": firstMetadataString(doc.Metadata, "match_type"),
		})
		if len(items) >= limit {
			break
		}
	}
	return items
}

func schemaDocumentTraceItems(docs []*schema.Document, limit int) []map[string]any {
	items := make([]map[string]any, 0, boundedDocLimit(len(docs), limit))
	for index, doc := range docs {
		if doc == nil {
			continue
		}
		items = append(items, map[string]any{
			"rank":       index + 1,
			"doc_id":     doc.ID,
			"score":      doc.Score(),
			"source":     firstMetadataString(doc.MetaData, "source", "source_filename", "file_name", "wiki_path"),
			"match_type": firstMetadataString(doc.MetaData, "match_type"),
		})
		if len(items) >= limit {
			break
		}
	}
	return items
}

func boundedDocLimit(length, limit int) int {
	if limit <= 0 || limit > length {
		return length
	}
	return limit
}

func clampScore(s float64) float64 {
	if s > 1.0 {
		return 1.0
	}
	if s < 0 {
		return 0
	}
	return s
}

func firstMetadataString(metadata map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := metadata[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

// RetrieveWithHybrid 混合检索（向量 + 关键词）
func (r *CompositeRetriever) RetrieveWithHybrid(ctx context.Context, query string, opts ...einoretriever.Option) ([]*schema.Document, error) {
	return r.retrieveWithHybrid(ctx, query, opts...)
}

// retrieveByMode 执行 semantic / exact / graph 三种显式模式。
func (r *CompositeRetriever) retrieveByMode(ctx context.Context, query string, mode string) ([]*schema.Document, error) {
	topK := r.cfg.TopK
	if topK <= 0 {
		topK = 10
	}
	tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "retrieval_mode", Summary: mode, Metadata: map[string]any{"mode": mode, "top_k": topK}})

	switch mode {
	case "semantic":
		queryVector, err := r.getQueryEmbedding(ctx, query)
		if err != nil {
			tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "embedding", Level: "error", Error: err.Error(), Metadata: map[string]any{"query": query}})
			return nil, fmt.Errorf("生成查询向量失败: %w", err)
		}
		started := time.Now()
		docs, err := r.vectorDB.Search(ctx, queryVector, topK*2, r.filter)
		tracing.Emit(ctx, retrievalSourceEvent("vector", query, docs, started, err))
		if err != nil {
			return nil, fmt.Errorf("语义检索失败: %w", err)
		}
		if len(docs) > topK {
			docs = docs[:topK]
		}
		return toSchemaDocuments(docs), nil

	case "exact":
		started := time.Now()
		docs, err := r.vectorDB.SearchKeyword(ctx, query, topK*2, r.filter)
		tracing.Emit(ctx, retrievalSourceEvent("keyword", query, docs, started, err))
		if err != nil {
			return nil, fmt.Errorf("关键词检索失败: %w", err)
		}
		if len(docs) > topK {
			docs = docs[:topK]
		}
		return toSchemaDocuments(docs), nil

	case "graph":
		if r.graphRetriever == nil {
			log.Printf("[Retriever] graph 模式不可用（无图谱检索器），降级为 auto")
			tracing.Emit(ctx, tracing.Event{Type: "retrieval", Stage: "graph_recall", Summary: "graph retriever unavailable", Metadata: map[string]any{"source": "graph", "available": false, "fallback": "auto"}})
			return r.retrieveAuto(ctx, query)
		}
		started := time.Now()
		docs, err := r.graphRetriever.Retrieve(ctx, query)
		tracing.Emit(ctx, graphRetrievalEvent(query, docs, started, err))
		return docs, err

	default:
		return r.retrieveAuto(ctx, query)
	}
}
