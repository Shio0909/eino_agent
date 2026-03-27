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
)

// CompositeRetriever 组合检索器
// 【Eino 特点】支持向量检索 + 关键词检索 + 图谱检索的混合模式
type CompositeRetriever struct {
	cfg                  *config.RAGConfig
	embeddingCfg         *config.EmbeddingConfig
	embedding            einoembedding.Embedder
	vectorDB             VectorDBProvider
	retrievalCache       cachepkg.RetrievalCache
	graphRetriever       einoretriever.Retriever
	graphRetrieverFactory GraphRetrieverFactory // 用于按 KB 动态创建图谱检索器
}

// GraphRetrieverFactory 按命名空间创建图谱检索器（由 graphRAG Service 实现）
type GraphRetrieverFactory interface {
	CreateScopedGraphRetriever(knowledgeBaseID string, topK int) einoretriever.Retriever
}

type scopedKeywordSearcher interface {
	SearchKeyword(ctx context.Context, query string, topK int) ([]*Document, error)
}

type keywordSearcher interface {
	SearchKeyword(ctx context.Context, query string, topK int) ([]*Document, error)
}

type knowledgeBaseScopedVectorDB struct {
	base  VectorDBProvider
	kbSet map[string]struct{}
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

// Retrieve 执行检索
func (r *CompositeRetriever) Retrieve(ctx context.Context, query string, opts ...einoretriever.Option) ([]*schema.Document, error) {
	if cachedDocs, ok, err := r.getCachedRetrievalDocuments(ctx, query, false); err != nil {
		log.Printf("[Retriever] 读取检索缓存失败，降级到实时检索: %v", err)
	} else if ok {
		return toSchemaDocuments(cachedDocs), nil
	}

	if r.cfg.EnableHybrid {
		return r.retrieveWithHybrid(ctx, query, opts...)
	}

	topK := r.cfg.TopK
	if topK <= 0 {
		topK = 10
	}

	// 1. 生成查询向量
	queryVector, err := r.getQueryEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("生成查询向量失败: %w", err)
	}

	// 2. 向量检索
	docs, err := r.vectorDB.Search(ctx, queryVector, topK*2) // 多取一些用于后续过滤
	if err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}

	// 4. 限制返回数量
	if len(docs) > topK {
		docs = docs[:topK]
	}

	r.setCachedRetrievalDocuments(ctx, query, false, docs)

	return toSchemaDocuments(docs), nil
}

func (r *CompositeRetriever) retrieveWithHybrid(ctx context.Context, query string, opts ...einoretriever.Option) ([]*schema.Document, error) {
	topK := r.cfg.TopK
	if topK <= 0 {
		topK = 10
	}

	if cachedDocs, ok, err := r.getCachedRetrievalDocuments(ctx, query, true); err != nil {
		log.Printf("[Retriever] 读取检索缓存失败，降级到实时检索: %v", err)
	} else if ok {
		return toSchemaDocuments(cachedDocs), nil
	}

	queryVector, err := r.getQueryEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("生成查询向量失败: %w", err)
	}

	vectorDocs, err := r.vectorDB.Search(ctx, queryVector, topK*2)
	if err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}

	var keywordDocs []*Document
	if ks, ok := r.vectorDB.(keywordSearcher); ok {
		keywordDocs, err = ks.SearchKeyword(ctx, query, topK*2)
		if err != nil {
			return nil, fmt.Errorf("关键词检索失败: %w", err)
		}
	}

	// 图谱检索（如果启用）
	var graphDocs []*Document
	var graphContextDoc *Document // 图谱上下文（实体/关系描述），不参与 RRF 竞争
	if r.graphRetriever != nil {
		gDocs, gErr := r.graphRetriever.Retrieve(ctx, query)
		if gErr != nil {
			log.Printf("[Retriever] 图谱检索失败（降级继续）: %v", gErr)
		} else {
			for _, d := range gDocs {
				if d == nil {
					continue
				}
				// graph-context 是实体/关系的摘要文档，从 RRF 中分离出来直接注入
				if d.ID == "graph-context" && strings.TrimSpace(d.Content) != "" {
					graphContextDoc = &Document{
						ID:       d.ID,
						Content:  d.Content,
						Metadata: d.MetaData,
					}
					continue
				}
				graphDocs = append(graphDocs, &Document{
					ID:       d.ID,
					Content:  d.Content,
					Metadata: d.MetaData,
				})
			}
		}
	}

	fused := r.rrfFuse(vectorDocs, keywordDocs, graphDocs, topK)

	// 将图谱上下文作为补充信息追加到检索结果中（不占 topK 名额）
	if graphContextDoc != nil {
		if graphContextDoc.Metadata == nil {
			graphContextDoc.Metadata = map[string]interface{}{}
		}
		graphContextDoc.Metadata["match_type"] = "graph_context"
		fused = append(fused, graphContextDoc)
		log.Printf("[Retriever] 已注入图谱上下文文档（%d 字符）", len(graphContextDoc.Content))
	}

	r.setCachedRetrievalDocuments(ctx, query, true, fused)

	return toSchemaDocuments(fused), nil
}

func (r *CompositeRetriever) WithKnowledgeBaseScope(ids []string) einoretriever.Retriever {
	if len(ids) == 0 {
		return r
	}

	kbSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		kbSet[id] = struct{}{}
	}
	if len(kbSet) == 0 {
		return r
	}

	clone := *r
	clone.vectorDB = &knowledgeBaseScopedVectorDB{
		base:  r.vectorDB,
		kbSet: kbSet,
	}

	// 如果有 GraphRetriever 工厂，按第一个 KB ID 创建作用域图谱检索器
	if r.graphRetrieverFactory != nil && len(ids) > 0 {
		topK := r.cfg.TopK
		if topK <= 0 {
			topK = 10
		}
		clone.graphRetriever = r.graphRetrieverFactory.CreateScopedGraphRetriever(ids[0], topK)
		log.Printf("[Retriever] 为 KB=%s 创建了作用域图谱检索器", ids[0])
	}

	return &clone
}

func (r *CompositeRetriever) getQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	if r.retrievalCache != nil {
		queryHash := hashQuery(query)
		modelID := r.embeddingModelID()
		cachedVector, hit, err := r.retrievalCache.GetEmbedding(ctx, modelID, queryHash)
		if err != nil {
			log.Printf("[Retriever] 读取 embedding 缓存失败，降级到实时向量化: %v", err)
		} else if hit && len(cachedVector) > 0 {
			log.Printf("[Cache][Embedding] hit model=%s", modelID)
			return cachedVector, nil
		} else {
			log.Printf("[Cache][Embedding] miss model=%s", modelID)
		}

		vector, err := EmbedFloat32(ctx, r.embedding, query)
		if err != nil {
			return nil, err
		}
		if err := r.retrievalCache.SetEmbedding(ctx, modelID, queryHash, vector, r.embeddingCacheTTL()); err != nil {
			log.Printf("[Retriever] 写入 embedding 缓存失败，继续使用实时结果: %v", err)
		}
		return vector, nil
	}

	return EmbedFloat32(ctx, r.embedding, query)
}

func (r *CompositeRetriever) getCachedRetrievalDocuments(ctx context.Context, query string, hybrid bool) ([]*Document, bool, error) {
	if r.retrievalCache == nil {
		return nil, false, nil
	}
	cacheKey := r.retrievalCacheKey(query, hybrid)
	result, hit, err := r.retrievalCache.GetRetrievalResult(ctx, cacheKey)
	if err != nil {
		return nil, false, err
	}
	if !hit || result == nil || len(result.Documents) == 0 {
		log.Printf("[Cache][Retrieval] miss mode=%s", map[bool]string{true: "hybrid", false: "vector"}[hybrid])
		return nil, false, nil
	}
	log.Printf("[Cache][Retrieval] hit mode=%s docs=%d", map[bool]string{true: "hybrid", false: "vector"}[hybrid], len(result.Documents))
	return retrievalCacheDocsToDocuments(result.Documents), true, nil
}

func (r *CompositeRetriever) setCachedRetrievalDocuments(ctx context.Context, query string, hybrid bool, docs []*Document) {
	if r.retrievalCache == nil || len(docs) == 0 {
		return
	}
	result := &cachepkg.RetrievalResult{
		DocIDs:    make([]string, 0, len(docs)),
		Scores:    make([]float64, 0, len(docs)),
		Documents: documentsToRetrievalCacheDocs(docs),
		CachedAt:  time.Now(),
	}
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		result.DocIDs = append(result.DocIDs, doc.ID)
		result.Scores = append(result.Scores, doc.Score)
	}
	if err := r.retrievalCache.SetRetrievalResult(ctx, r.retrievalCacheKey(query, hybrid), result, r.retrievalCacheTTL()); err != nil {
		log.Printf("[Retriever] 写入检索缓存失败，继续返回实时结果: %v", err)
	}
}

func (r *CompositeRetriever) retrievalCacheKey(query string, hybrid bool) string {
	mode := "vector"
	if hybrid {
		mode = "hybrid"
	}
	return fmt.Sprintf("%s:%d:%s:%s", mode, r.cfg.TopK, r.scopeCacheKey(), hashQuery(query))
}

func (r *CompositeRetriever) scopeCacheKey() string {
	if scoped, ok := r.vectorDB.(*knowledgeBaseScopedVectorDB); ok {
		ids := make([]string, 0, len(scoped.kbSet))
		for id := range scoped.kbSet {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return strings.Join(ids, ",")
	}
	return "all"
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

func (r *CompositeRetriever) retrievalCacheTTL() time.Duration {
	minutes := r.cfg.RetrievalCacheTTLMinutes
	if minutes <= 0 {
		minutes = 10
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
		result = append(result, &schema.Document{
			ID:       doc.ID,
			Content:  doc.Content,
			MetaData: doc.Metadata,
		})
	}
	return result
}

func documentsToRetrievalCacheDocs(docs []*Document) []cachepkg.RetrievalDocument {
	result := make([]cachepkg.RetrievalDocument, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		metadata := make(map[string]any, len(doc.Metadata))
		for key, value := range doc.Metadata {
			metadata[key] = value
		}
		result = append(result, cachepkg.RetrievalDocument{
			ID:       doc.ID,
			Content:  doc.Content,
			Score:    doc.Score,
			Metadata: metadata,
		})
	}
	return result
}

func retrievalCacheDocsToDocuments(docs []cachepkg.RetrievalDocument) []*Document {
	result := make([]*Document, 0, len(docs))
	for _, doc := range docs {
		metadata := make(map[string]interface{}, len(doc.Metadata))
		for key, value := range doc.Metadata {
			metadata[key] = value
		}
		result = append(result, &Document{
			ID:       doc.ID,
			Content:  doc.Content,
			Score:    doc.Score,
			Metadata: metadata,
		})
	}
	return result
}

func (r *CompositeRetriever) rrfFuse(vectorDocs, keywordDocs, graphDocs []*Document, topK int) []*Document {
	const rrfK = 60.0

	type rankedDoc struct {
		doc   *Document
		score float64
		hits  int
	}

	ranked := make(map[string]*rankedDoc)

	add := func(docs []*Document, source string) {
		for rank, doc := range docs {
			if doc == nil || doc.ID == "" {
				continue
			}
			entry, exists := ranked[doc.ID]
			if !exists {
				entry = &rankedDoc{doc: doc}
				ranked[doc.ID] = entry
			} else if entry.doc.Content == "" && doc.Content != "" {
				// 当前 entry 无内容但新 doc 有内容时，替换为有内容的文档（保留已有得分）
				entry.doc = doc
			}
			entry.score += 1.0 / (rrfK + float64(rank+1))
			entry.hits++

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
	add(graphDocs, "graph")

	list := make([]*rankedDoc, 0, len(ranked))
	for _, item := range ranked {
		item.doc.Score = item.score
		if item.hits > 1 {
			if item.doc.Metadata == nil {
				item.doc.Metadata = map[string]interface{}{}
			}
			item.doc.Metadata["match_type"] = "hybrid"
		}
		// 跳过没有实际内容的文档（图谱返回的空 content chunk 若未被向量/关键词命中则无用）
		if strings.TrimSpace(item.doc.Content) == "" {
			continue
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
	for i := 0; i < topK; i++ {
		fused = append(fused, list[i].doc)
	}

	return fused
}

// RetrieveWithHybrid 混合检索（向量 + 关键词）
func (r *CompositeRetriever) RetrieveWithHybrid(ctx context.Context, query string, opts ...einoretriever.Option) ([]*schema.Document, error) {
	return r.retrieveWithHybrid(ctx, query, opts...)
}

func (db *knowledgeBaseScopedVectorDB) Upsert(ctx context.Context, docs []*Document) error {
	return db.base.Upsert(ctx, docs)
}

func (db *knowledgeBaseScopedVectorDB) Delete(ctx context.Context, ids []string) error {
	return db.base.Delete(ctx, ids)
}

func (db *knowledgeBaseScopedVectorDB) Close() error {
	return db.base.Close()
}

func (db *knowledgeBaseScopedVectorDB) Search(ctx context.Context, vector []float32, topK int) ([]*Document, error) {
	candidateK := topK * 50
	if candidateK < 200 {
		candidateK = 200
	}

	docs, err := db.base.Search(ctx, vector, candidateK)
	if err != nil {
		return nil, err
	}

	filtered := db.filterDocs(docs, topK)
	if len(filtered) >= topK || candidateK >= 5000 {
		return filtered, nil
	}

	docs, err = db.base.Search(ctx, vector, 5000)
	if err != nil {
		return nil, err
	}

	return db.filterDocs(docs, topK), nil
}

func (db *knowledgeBaseScopedVectorDB) SearchKeyword(ctx context.Context, query string, topK int) ([]*Document, error) {
	searcher, ok := db.base.(scopedKeywordSearcher)
	if !ok {
		return nil, nil
	}

	candidateK := topK * 20
	if candidateK < 100 {
		candidateK = 100
	}

	docs, err := searcher.SearchKeyword(ctx, query, candidateK)
	if err != nil {
		return nil, err
	}

	return db.filterDocs(docs, topK), nil
}

func (db *knowledgeBaseScopedVectorDB) filterDocs(docs []*Document, topK int) []*Document {
	limit := topK
	if limit <= 0 || limit > len(docs) {
		limit = len(docs)
	}
	filtered := make([]*Document, 0, limit)
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if !db.matchKB(doc) {
			continue
		}
		filtered = append(filtered, doc)
		if topK > 0 && len(filtered) >= topK {
			break
		}
	}
	return filtered
}

func (db *knowledgeBaseScopedVectorDB) matchKB(doc *Document) bool {
	if doc == nil || doc.Metadata == nil {
		return false
	}
	kbID, _ := doc.Metadata["knowledge_base_id"].(string)
	if kbID == "" {
		return false
	}
	_, ok := db.kbSet[kbID]
	return ok
}
