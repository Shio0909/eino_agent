package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// MemoryVectorDB 内存向量数据库（仅用于本地开发与测试）。
type MemoryVectorDB struct {
	mu         sync.RWMutex
	docs       map[string]*Document
	dimensions int
	dataFile   string
}

func newMemoryVectorDB(_ context.Context, dimensions int) (*MemoryVectorDB, CleanupFunc, error) {
	db := NewMemoryVectorDB(dimensions, "data/vectors.json")
	if err := db.loadFromFile(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("[VectorDB] 读取内存向量快照失败，使用空数据库: %v", err)
	}
	cleanup := func(ctx context.Context) error {
		return db.saveToFile()
	}
	return db, cleanup, nil
}

// NewMemoryVectorDB 创建内存向量库；dataFile 为空时不做持久化。
func NewMemoryVectorDB(dimensions int, dataFile string) *MemoryVectorDB {
	return &MemoryVectorDB{
		docs:       make(map[string]*Document),
		dimensions: dimensions,
		dataFile:   dataFile,
	}
}

func (db *MemoryVectorDB) Ping(context.Context) error { return nil }

func (db *MemoryVectorDB) Upsert(_ context.Context, docs []*Document) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if db.dimensions > 0 && len(doc.Vector) != 0 && len(doc.Vector) != db.dimensions {
			return fmt.Errorf("文档 %s 向量维度不匹配: got=%d expect=%d", doc.ID, len(doc.Vector), db.dimensions)
		}
		db.docs[doc.ID] = doc
	}
	return nil
}

type memoryScored struct {
	doc   *Document
	score float64
}

func (db *MemoryVectorDB) Search(_ context.Context, vector []float32, topK int, filter SearchFilter) ([]*Document, error) {
	filter = filter.Normalized()
	if filter.DeniesAll() {
		return []*Document{}, nil
	}
	if topK <= 0 {
		topK = 10
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	results := make([]memoryScored, 0, len(db.docs))
	for _, doc := range db.docs {
		if doc.Vector == nil || !filter.Matches(doc) {
			continue
		}
		results = append(results, memoryScored{doc: doc, score: cosineSimilarity(vector, doc.Vector)})
	}
	return topScored(results, topK), nil
}

func (db *MemoryVectorDB) SearchKeyword(_ context.Context, query string, topK int, filter SearchFilter) ([]*Document, error) {
	filter = filter.Normalized()
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" || filter.DeniesAll() {
		return []*Document{}, nil
	}
	if topK <= 0 {
		topK = 10
	}

	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		tokens = []string{query}
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	results := make([]memoryScored, 0)
	for _, doc := range db.docs {
		if !filter.Matches(doc) {
			continue
		}
		content := strings.ToLower(doc.Content)
		matched := 0
		for _, token := range tokens {
			if strings.Contains(content, token) {
				matched++
			}
		}
		if matched == 0 {
			continue
		}
		results = append(results, memoryScored{doc: doc, score: float64(matched) / float64(len(tokens))})
	}
	return topScored(results, topK), nil
}

func topScored(results []memoryScored, topK int) []*Document {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].score == results[j].score {
			return results[i].doc.ID < results[j].doc.ID
		}
		return results[i].score > results[j].score
	})
	if topK > len(results) {
		topK = len(results)
	}
	docs := make([]*Document, 0, topK)
	for _, item := range results[:topK] {
		docs = append(docs, &Document{
			ID:       item.doc.ID,
			Content:  item.doc.Content,
			Score:    item.score,
			Metadata: cloneMetadata(item.doc.Metadata),
		})
	}
	return docs
}

func cloneMetadata(meta map[string]interface{}) map[string]interface{} {
	if meta == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

func (db *MemoryVectorDB) Delete(_ context.Context, ids []string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	for _, id := range ids {
		delete(db.docs, id)
	}
	return nil
}

func (db *MemoryVectorDB) UpdateChunkIndexes(_ context.Context, indexes map[string]int) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	for id, index := range indexes {
		doc, ok := db.docs[id]
		if !ok {
			continue
		}
		meta := cloneMetadata(doc.Metadata)
		meta["chunk_index"] = index
		doc.Metadata = meta
	}
	return nil
}

func (db *MemoryVectorDB) DeleteByKnowledgeID(_ context.Context, knowledgeID string) error {
	return db.deleteWhere("knowledge_id", knowledgeID)
}

func (db *MemoryVectorDB) DeleteByKnowledgeBaseID(_ context.Context, kbID string) error {
	return db.deleteWhere("knowledge_base_id", kbID)
}

func (db *MemoryVectorDB) deleteWhere(key, value string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	for id, doc := range db.docs {
		if v, ok := doc.Metadata[key].(string); ok && v == value {
			delete(db.docs, id)
		}
	}
	return nil
}

func (db *MemoryVectorDB) Close() error {
	return db.saveToFile()
}

func (db *MemoryVectorDB) loadFromFile() error {
	if db.dataFile == "" {
		return nil
	}
	data, err := os.ReadFile(db.dataFile)
	if err != nil {
		return err
	}
	var docs []*Document
	if err := json.Unmarshal(data, &docs); err != nil {
		return err
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	for _, doc := range docs {
		if doc != nil {
			db.docs[doc.ID] = doc
		}
	}
	return nil
}

func (db *MemoryVectorDB) saveToFile() error {
	if db.dataFile == "" {
		return nil
	}
	db.mu.RLock()
	docs := make([]*Document, 0, len(db.docs))
	for _, doc := range db.docs {
		docs = append(docs, doc)
	}
	db.mu.RUnlock()

	sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
	data, err := json.Marshal(docs)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(db.dataFile), 0o755); err != nil {
		return err
	}
	tmp := db.dataFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, db.dataFile)
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
