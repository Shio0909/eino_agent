package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/container"
)

// A knowledge base where every chunk of the other documents ranks ahead of the
// chunk the user picked: any search that takes its top results first and
// filters by document afterwards finds nothing.
func seedCrowdedKnowledgeBase(t *testing.T) *searchLog {
	t.Helper()
	db := &searchLog{MemoryVectorDB: container.NewMemoryVectorDB(3, "")}
	var docs []*container.Document
	for i := 0; i < 20; i++ {
		docs = append(docs, &container.Document{
			ID: fmt.Sprintf("crowd-%d", i), Content: "other", Vector: []float32{1, 0, 0},
			Metadata: map[string]any{"knowledge_base_id": "kb-1", "knowledge_id": fmt.Sprintf("doc-%d", i)},
		})
	}
	docs = append(docs,
		&container.Document{ID: "wanted-chunk", Content: "wanted", Vector: []float32{0, 1, 0},
			Metadata: map[string]any{"knowledge_base_id": "kb-1", "knowledge_id": "doc-wanted"}},
		&container.Document{ID: "elsewhere-chunk", Content: "wanted elsewhere", Vector: []float32{0, 1, 0},
			Metadata: map[string]any{"knowledge_base_id": "kb-2", "knowledge_id": "doc-wanted"}},
	)
	if err := db.Upsert(context.Background(), docs); err != nil {
		t.Fatal(err)
	}
	return db
}

func docIDs(t *testing.T, svc *ChatService, req *ChatRequest) []string {
	t.Helper()
	docs, err := svc.getRuntimeRetriever(req).Retrieve(context.Background(), "q")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	ids := make([]string, 0, len(docs))
	for _, d := range docs {
		ids = append(ids, d.ID)
	}
	return ids
}

func TestSelectedDocumentIsFoundEvenWhenOtherDocumentsOutrankIt(t *testing.T) {
	db := seedCrowdedKnowledgeBase(t)
	services := map[string]*ChatService{
		"without kb repo": productionRetriever(t, db, nil),
		"with kb repo":    productionRetriever(t, db, vectorKBs{}),
	}
	cases := []struct {
		name string
		req  *ChatRequest
		want []string
	}{
		{"documents inside a knowledge base",
			&ChatRequest{Message: "q", KnowledgeBaseIDs: []string{"kb-1"}, DocumentIDs: []string{"doc-wanted"}, RestrictRetrieval: true},
			[]string{"wanted-chunk"}},
		{"documents alone",
			&ChatRequest{Message: "q", DocumentIDs: []string{"doc-wanted"}, RestrictRetrieval: true},
			[]string{"wanted-chunk", "elsewhere-chunk"}},
		{"a chunk id counts as a document id",
			&ChatRequest{Message: "q", KnowledgeBaseIDs: []string{"kb-1"}, DocumentIDs: []string{"wanted-chunk"}},
			[]string{"wanted-chunk"}},
		{"the document is in a different knowledge base",
			&ChatRequest{Message: "q", KnowledgeBaseIDs: []string{"kb-2"}, DocumentIDs: []string{"doc-3"}, RestrictRetrieval: true},
			nil},
	}
	for svcName, svc := range services {
		for _, tc := range cases {
			got := docIDs(t, svc, tc.req)
			if !sameSet(got, tc.want) {
				t.Errorf("%s / %s: got %v, want %v", svcName, tc.name, got, tc.want)
			}
		}
	}
}

// The public /retrieve endpoint takes the same document ids.
func TestRetrieveAPIFindsTheSelectedDocument(t *testing.T) {
	db := seedCrowdedKnowledgeBase(t)
	svc := productionRetriever(t, db, nil)
	resp, err := svc.Retrieve(context.Background(), &RetrieveRequest{Query: "q", KnowledgeBaseIDs: []string{"kb-1"}, DocumentIDs: []string{"doc-wanted"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Fatalf("Retrieve returned %d results, want the one chunk of the selected document", resp.Total)
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

// A retriever that cannot take the scope is still held to it.
func TestDocumentScopeIsEnforcedOnRetrieversThatCannotTakeIt(t *testing.T) {
	svc := &ChatService{retriever: fakeRetriever{docs: []*schema.Document{
		{ID: "c1", Content: "mine", MetaData: map[string]any{"knowledge_id": "doc-1", "knowledge_base_id": "kb-1"}},
		{ID: "c2", Content: "not mine", MetaData: map[string]any{"knowledge_id": "doc-2", "knowledge_base_id": "kb-1"}},
	}}}
	got := docIDs(t, svc, &ChatRequest{Message: "q", KnowledgeBaseIDs: []string{"kb-1"}, DocumentIDs: []string{"doc-1"}})
	if !sameSet(got, []string{"c1"}) {
		t.Fatalf("got %v, want only c1", got)
	}
}
