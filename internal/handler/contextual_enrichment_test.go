package handler

import (
	"context"
	"strings"
	"testing"

	einoembedding "github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/cache"
	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/docreader"
)

type enrichmentTestChatModel struct{}

func (enrichmentTestChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return &schema.Message{Content: "本段介绍 MySQL 索引结构的上下文"}, nil
}

func (enrichmentTestChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (enrichmentTestChatModel) BindTools([]*schema.ToolInfo) error { return nil }

type enrichmentTestEmbedder struct{}

func (enrichmentTestEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...einoembedding.Option) ([][]float64, error) {
	vectors := make([][]float64, len(texts))
	for i := range texts {
		vectors[i] = []float64{1, 0, 0}
	}
	return vectors, nil
}

type capturingVectorDB struct {
	docs []*container.Document
}

func (v *capturingVectorDB) Upsert(_ context.Context, docs []*container.Document) error {
	v.docs = append(v.docs, docs...)
	return nil
}
func (v *capturingVectorDB) Search(context.Context, []float32, int) ([]*container.Document, error) { return nil, nil }
func (v *capturingVectorDB) Delete(context.Context, []string) error { return nil }
func (v *capturingVectorDB) DeleteByKnowledgeID(context.Context, string) error { return nil }
func (v *capturingVectorDB) DeleteByKnowledgeBaseID(context.Context, string) error { return nil }
func (v *capturingVectorDB) Close() error { return nil }

func TestProcessAndStoreChunksAppliesContextualEnrichment(t *testing.T) {
	vectorDB := &capturingVectorDB{}
	h := &Handler{
		cfg: &config.Config{RAG: config.RAGConfig{EnableContextualEnrichment: true}},
		embedding: enrichmentTestEmbedder{},
		vectorDB: vectorDB,
		chatModelFactory: func(context.Context) (model.ChatModel, error) {
			return enrichmentTestChatModel{}, nil
		},
		importStateStore: cache.NewNoopImportStateStore(),
		retrievalCache: cache.NewNoopRetrievalCache(),
	}

	err := h.processAndStoreChunks(context.Background(), "kb-1", "doc-1", "mysql.pdf", []docreader.ParsedChunk{
		{Seq: 1, Content: "B+Tree 索引适合范围查询", Start: 0, End: 20},
	})
	if err != nil {
		t.Fatalf("processAndStoreChunks error = %v", err)
	}
	if len(vectorDB.docs) != 1 {
		t.Fatalf("stored docs = %d, want 1", len(vectorDB.docs))
	}
	if !strings.HasPrefix(vectorDB.docs[0].Content, "[上下文: 本段介绍 MySQL 索引结构的上下文]") {
		t.Fatalf("content was not enriched: %q", vectorDB.docs[0].Content)
	}
	if vectorDB.docs[0].Metadata["enriched"] != true {
		t.Fatalf("enriched metadata = %v, want true", vectorDB.docs[0].Metadata["enriched"])
	}
}
