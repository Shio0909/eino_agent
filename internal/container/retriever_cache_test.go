package container

import (
	"context"
	"testing"
	"time"

	einoembedding "github.com/cloudwego/eino/components/embedding"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/config"
)

type fakeEmbedder struct {
	callCount int
	vector    []float32
}

func (f *fakeEmbedder) EmbedStrings(context.Context, []string, ...einoembedding.Option) ([][]float64, error) {
	f.callCount++
	result := make([]float64, len(f.vector))
	for i, value := range f.vector {
		result[i] = float64(value)
	}
	return [][]float64{result}, nil
}

type fakeVectorDB struct {
	searchCalls        int
	searchKeywordCalls int
	docs               []*Document
	keywordDocs        []*Document
}

func (f *fakeVectorDB) Upsert(context.Context, []*Document) error { return nil }
func (f *fakeVectorDB) Delete(context.Context, []string) error    { return nil }
func (f *fakeVectorDB) Close() error                              { return nil }

func (f *fakeVectorDB) Search(context.Context, []float32, int) ([]*Document, error) {
	f.searchCalls++
	return cloneDocuments(f.docs), nil
}

func (f *fakeVectorDB) SearchKeyword(context.Context, string, int) ([]*Document, error) {
	f.searchKeywordCalls++
	return cloneDocuments(f.keywordDocs), nil
}

type memoryRetrievalCache struct {
	embeddings map[string][]float32
	results    map[string]*cachepkg.RetrievalResult
}

func newMemoryRetrievalCache() *memoryRetrievalCache {
	return &memoryRetrievalCache{
		embeddings: make(map[string][]float32),
		results:    make(map[string]*cachepkg.RetrievalResult),
	}
}

func (m *memoryRetrievalCache) GetEmbedding(_ context.Context, modelID, queryHash string) ([]float32, bool, error) {
	vector, ok := m.embeddings[modelID+":"+queryHash]
	if !ok {
		return nil, false, nil
	}
	cloned := make([]float32, len(vector))
	copy(cloned, vector)
	return cloned, true, nil
}

func (m *memoryRetrievalCache) SetEmbedding(_ context.Context, modelID, queryHash string, vector []float32, _ time.Duration) error {
	cloned := make([]float32, len(vector))
	copy(cloned, vector)
	m.embeddings[modelID+":"+queryHash] = cloned
	return nil
}

func (m *memoryRetrievalCache) GetRetrievalResult(_ context.Context, cacheKey string) (*cachepkg.RetrievalResult, bool, error) {
	result, ok := m.results[cacheKey]
	if !ok {
		return nil, false, nil
	}
	cloned := *result
	cloned.Documents = append([]cachepkg.RetrievalDocument(nil), result.Documents...)
	cloned.DocIDs = append([]string(nil), result.DocIDs...)
	cloned.Scores = append([]float64(nil), result.Scores...)
	return &cloned, true, nil
}

func (m *memoryRetrievalCache) SetRetrievalResult(_ context.Context, cacheKey string, result *cachepkg.RetrievalResult, _ time.Duration) error {
	cloned := *result
	cloned.Documents = append([]cachepkg.RetrievalDocument(nil), result.Documents...)
	cloned.DocIDs = append([]string(nil), result.DocIDs...)
	cloned.Scores = append([]float64(nil), result.Scores...)
	m.results[cacheKey] = &cloned
	return nil
}

func (m *memoryRetrievalCache) InvalidateKnowledgeBase(_ context.Context, knowledgeBaseID string) error {
	for key, result := range m.results {
		for _, doc := range result.Documents {
			if kbID, _ := doc.Metadata["knowledge_base_id"].(string); kbID == knowledgeBaseID {
				delete(m.results, key)
				break
			}
		}
	}
	return nil
}

func TestCompositeRetrieverCachesVectorSearch(t *testing.T) {
	embedder := &fakeEmbedder{vector: []float32{0.1, 0.2}}
	vectorDB := &fakeVectorDB{docs: []*Document{{ID: "d1", Content: "doc1", Score: 0.9, Metadata: map[string]interface{}{"knowledge_base_id": "kb1"}}}}
	cacheStore := newMemoryRetrievalCache()

	retriever, _, err := NewRetrieverProvider(context.Background(), &config.RAGConfig{TopK: 5, EmbeddingCacheTTLMinutes: 60, RetrievalCacheTTLMinutes: 10}, &config.EmbeddingConfig{ModelID: "embed-1"}, embedder, vectorDB, cacheStore)
	if err != nil {
		t.Fatalf("NewRetrieverProvider error = %v", err)
	}
	composite := retriever.(*CompositeRetriever)

	first, err := composite.Retrieve(context.Background(), "redis 是什么")
	if err != nil {
		t.Fatalf("first Retrieve error = %v", err)
	}
	if len(first) != 1 || first[0].ID != "d1" {
		t.Fatalf("unexpected first result: %#v", first)
	}
	if embedder.callCount != 1 || vectorDB.searchCalls != 1 {
		t.Fatalf("expected initial embedding/search calls to be 1, got embed=%d search=%d", embedder.callCount, vectorDB.searchCalls)
	}

	second, err := composite.Retrieve(context.Background(), "redis 是什么")
	if err != nil {
		t.Fatalf("second Retrieve error = %v", err)
	}
	if len(second) != 1 || second[0].ID != "d1" {
		t.Fatalf("unexpected second result: %#v", second)
	}
	if embedder.callCount != 1 {
		t.Fatalf("expected embedding cache hit, got embed call count %d", embedder.callCount)
	}
	if vectorDB.searchCalls != 1 {
		t.Fatalf("expected retrieval cache hit, got search call count %d", vectorDB.searchCalls)
	}
}

func TestCompositeRetrieverCachesEmbeddingAcrossQueries(t *testing.T) {
	embedder := &fakeEmbedder{vector: []float32{0.3, 0.4}}
	vectorDB := &fakeVectorDB{docs: []*Document{{ID: "d1", Content: "doc1", Metadata: map[string]interface{}{"knowledge_base_id": "kb1"}}}}
	cacheStore := newMemoryRetrievalCache()

	retriever, _, err := NewRetrieverProvider(context.Background(), &config.RAGConfig{TopK: 5, EmbeddingCacheTTLMinutes: 60, RetrievalCacheTTLMinutes: 10}, &config.EmbeddingConfig{ModelID: "embed-1"}, embedder, vectorDB, cacheStore)
	if err != nil {
		t.Fatalf("NewRetrieverProvider error = %v", err)
	}
	composite := retriever.(*CompositeRetriever)
	composite.retrievalCache = retrievalMissEmbeddingHitCache{base: cacheStore}

	_, err = composite.Retrieve(context.Background(), "golang")
	if err != nil {
		t.Fatalf("first Retrieve error = %v", err)
	}
	_, err = composite.Retrieve(context.Background(), "golang")
	if err != nil {
		t.Fatalf("second Retrieve error = %v", err)
	}
	if embedder.callCount != 1 {
		t.Fatalf("expected embedding cache hit on second query, got %d calls", embedder.callCount)
	}
	if vectorDB.searchCalls != 2 {
		t.Fatalf("expected retrieval result cache bypassed in this test, got search=%d", vectorDB.searchCalls)
	}
}

type retrievalMissEmbeddingHitCache struct {
	base *memoryRetrievalCache
}

func (c retrievalMissEmbeddingHitCache) GetEmbedding(ctx context.Context, modelID, queryHash string) ([]float32, bool, error) {
	return c.base.GetEmbedding(ctx, modelID, queryHash)
}

func (c retrievalMissEmbeddingHitCache) SetEmbedding(ctx context.Context, modelID, queryHash string, vector []float32, ttl time.Duration) error {
	return c.base.SetEmbedding(ctx, modelID, queryHash, vector, ttl)
}

func (c retrievalMissEmbeddingHitCache) GetRetrievalResult(context.Context, string) (*cachepkg.RetrievalResult, bool, error) {
	return nil, false, nil
}

func (c retrievalMissEmbeddingHitCache) SetRetrievalResult(context.Context, string, *cachepkg.RetrievalResult, time.Duration) error {
	return nil
}

func (c retrievalMissEmbeddingHitCache) InvalidateKnowledgeBase(context.Context, string) error {
	return nil
}

func cloneDocuments(docs []*Document) []*Document {
	result := make([]*Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		metadata := make(map[string]interface{}, len(doc.Metadata))
		for key, value := range doc.Metadata {
			metadata[key] = value
		}
		result = append(result, &Document{ID: doc.ID, Content: doc.Content, Score: doc.Score, Metadata: metadata})
	}
	return result
}
