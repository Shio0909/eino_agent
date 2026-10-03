package container

import (
	"context"
	"testing"

	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/config"
	"eino_agent/internal/retrieval"
)

type countingGraphRetriever struct{ calls int }

func (g *countingGraphRetriever) Retrieve(context.Context, string, ...einoretriever.Option) ([]*schema.Document, error) {
	g.calls++
	return []*schema.Document{{ID: "from-graph", Content: "graph hit"}}, nil
}

func newModeTestRetriever(t *testing.T, hybrid bool, graph einoretriever.Retriever) (*CompositeRetriever, *fakeVectorDB) {
	t.Helper()
	db := &fakeVectorDB{
		docs:        []*Document{{ID: "v1", Content: "vector hit"}},
		keywordDocs: []*Document{{ID: "k1", Content: "keyword hit"}},
	}
	r, _, err := NewRetrieverProvider(context.Background(), &config.RAGConfig{TopK: 5, EnableHybrid: hybrid},
		&config.EmbeddingConfig{}, &fakeEmbedder{vector: []float32{1, 0}}, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := r.(*CompositeRetriever)
	if graph != nil {
		c.SetGraphRetriever(graph)
	}
	return c, db
}

// The mode a caller asks for has to decide which store is searched. It used to
// travel through a side interface that the wrappers around the retriever hid, so
// every mode ended up as the default search.
func TestRetrieveModeDecidesWhichSearchRuns(t *testing.T) {
	cases := []struct {
		mode                      string
		vector, keyword, graphHit int
	}{
		{"", 1, 1, 1},
		{retrieval.ModeAuto, 1, 1, 1},
		{"no-such-mode", 1, 1, 1},
		{retrieval.ModeSemantic, 1, 0, 0},
		{retrieval.ModeExact, 0, 1, 0},
		{retrieval.ModeGraph, 0, 0, 1},
	}
	for _, tc := range cases {
		graph := &countingGraphRetriever{}
		r, db := newModeTestRetriever(t, true, graph)
		var opts []einoretriever.Option
		if tc.mode != "" {
			opts = append(opts, retrieval.WithMode(tc.mode))
		}
		if _, err := r.Retrieve(context.Background(), "ERR_42", opts...); err != nil {
			t.Fatalf("mode %q: %v", tc.mode, err)
		}
		if db.searchCalls != tc.vector || db.searchKeywordCalls != tc.keyword || graph.calls != tc.graphHit {
			t.Errorf("mode %q: vector=%d keyword=%d graph=%d, want %d/%d/%d",
				tc.mode, db.searchCalls, db.searchKeywordCalls, graph.calls, tc.vector, tc.keyword, tc.graphHit)
		}
	}
}

func TestRetrieveGraphModeWithoutGraphFallsBackToTheDefault(t *testing.T) {
	r, db := newModeTestRetriever(t, true, nil)
	docs, err := r.Retrieve(context.Background(), "q", retrieval.WithMode(retrieval.ModeGraph))
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) == 0 || db.searchCalls != 1 || db.searchKeywordCalls != 1 {
		t.Fatalf("docs=%d vector=%d keyword=%d, want the hybrid default", len(docs), db.searchCalls, db.searchKeywordCalls)
	}
}

func TestRetrieveDefaultIsVectorOnlyWithoutHybrid(t *testing.T) {
	r, db := newModeTestRetriever(t, false, nil)
	if _, err := r.Retrieve(context.Background(), "q"); err != nil {
		t.Fatal(err)
	}
	if db.searchCalls != 1 || db.searchKeywordCalls != 0 {
		t.Fatalf("vector=%d keyword=%d, want 1/0", db.searchCalls, db.searchKeywordCalls)
	}
}
