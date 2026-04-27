package service

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type fakeRetriever struct {
	docs []*schema.Document
}

func (r fakeRetriever) Retrieve(context.Context, string, ...retriever.Option) ([]*schema.Document, error) {
	return r.docs, nil
}

func TestRestrictedEmptyRetrievalScopeReturnsNoDocuments(t *testing.T) {
	svc := &ChatService{retriever: fakeRetriever{docs: []*schema.Document{
		{ID: "doc-1", Content: "secret", MetaData: map[string]any{"knowledge_base_id": "kb-1"}},
	}}}

	runtimeRetriever := svc.getRuntimeRetriever(&ChatRequest{RestrictRetrieval: true})
	docs, err := runtimeRetriever.Retrieve(context.Background(), "query")
	if err != nil {
		t.Fatalf("Retrieve error = %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("docs len = %d, want 0", len(docs))
	}
}

func TestKnowledgeBaseRetrievalScopeFiltersDocuments(t *testing.T) {
	svc := &ChatService{retriever: fakeRetriever{docs: []*schema.Document{
		{ID: "doc-1", Content: "allowed", MetaData: map[string]any{"knowledge_base_id": "kb-1"}},
		{ID: "doc-2", Content: "denied", MetaData: map[string]any{"knowledge_base_id": "kb-2"}},
	}}}

	runtimeRetriever := svc.getRuntimeRetriever(&ChatRequest{KnowledgeBaseIDs: []string{"kb-1"}, RestrictRetrieval: true})
	docs, err := runtimeRetriever.Retrieve(context.Background(), "query")
	if err != nil {
		t.Fatalf("Retrieve error = %v", err)
	}
	if len(docs) != 1 || docs[0].ID != "doc-1" {
		t.Fatalf("unexpected docs: %#v", docs)
	}
}

func TestSourcesFromDocumentsUsesDefaultLimitForInvalidMax(t *testing.T) {
	docs := []*schema.Document{
		{ID: "doc-1", Content: "one"},
		{ID: "doc-2", Content: "two"},
	}

	sources := sourcesFromDocuments(docs, -1)

	if len(sources) != 2 {
		t.Fatalf("sources len = %d, want 2", len(sources))
	}
	if sources[0].DocID != "doc-1" || sources[1].DocID != "doc-2" {
		t.Fatalf("unexpected sources: %#v", sources)
	}
}
