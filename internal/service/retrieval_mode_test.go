package service

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/embedding"

	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/retrieval"
	internalTool "eino_agent/internal/tool"
)

// searchLog is a vector store that records which kind of search reached it.
type searchLog struct {
	*container.MemoryVectorDB
	mu    sync.Mutex
	calls []string
}

func (d *searchLog) Search(ctx context.Context, v []float32, k int, f container.SearchFilter) ([]*container.Document, error) {
	d.record("vector")
	return d.MemoryVectorDB.Search(ctx, v, k, f)
}

func (d *searchLog) SearchKeyword(ctx context.Context, q string, k int, f container.SearchFilter) ([]*container.Document, error) {
	d.record("keyword")
	return d.MemoryVectorDB.SearchKeyword(ctx, q, k, f)
}

func (d *searchLog) record(kind string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, kind)
}

func (d *searchLog) take() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := d.calls
	d.calls = nil
	return out
}

type unitEmbedder struct{}

func (unitEmbedder) EmbedStrings(_ context.Context, in []string, _ ...embedding.Option) ([][]float64, error) {
	out := make([][]float64, len(in))
	for i := range in {
		out[i] = []float64{1, 0, 0}
	}
	return out, nil
}

// vectorKBs answers the one lookup UnifiedRetriever makes: every knowledge base
// is a vector-mode one.
type vectorKBs struct {
	repository.KnowledgeBaseRepository
}

func (vectorKBs) GetByID(_ context.Context, id string) (*repository.KnowledgeBase, error) {
	return &repository.KnowledgeBase{ID: id, Mode: "vector"}, nil
}

// productionRetriever builds the retriever the server wires up: the composite
// retriever behind a UnifiedRetriever, with hybrid search switched on. The
// server passes a knowledge-base repository; with one, a scoped request is
// routed through a different retriever type than without.
func productionRetriever(t *testing.T, db container.VectorDBProvider, kbRepo repository.KnowledgeBaseRepository) *ChatService {
	t.Helper()
	r, _, err := container.NewRetrieverProvider(context.Background(),
		&config.RAGConfig{TopK: 5, EnableHybrid: true}, &config.EmbeddingConfig{}, unitEmbedder{}, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	unified := container.NewUnifiedRetriever(r.(*container.CompositeRetriever), nil, kbRepo, 5)
	return &ChatService{config: testChatConfig(), retriever: unified}
}

// knowledge_search offers semantic / exact / graph modes to the model. The
// retriever it holds in a real request is wrapped for scoping and tracing, and
// the mode must still decide which store is searched.
func TestKnowledgeSearchModeReachesTheStoreThroughTheRequestWrappers(t *testing.T) {
	db := &searchLog{MemoryVectorDB: container.NewMemoryVectorDB(3, "")}
	services := map[string]*ChatService{
		"without kb repo": productionRetriever(t, db, nil),
		"with kb repo":    productionRetriever(t, db, vectorKBs{}),
	}

	requests := map[string]*ChatRequest{
		"no scope":    {Message: "q"},
		"kb scope":    {Message: "q", KnowledgeBaseIDs: []string{"kb-1"}},
		"doc scope":   {Message: "q", DocumentIDs: []string{"doc-1"}},
		"restricted":  {Message: "q", KnowledgeBaseIDs: []string{"kb-1"}, RestrictRetrieval: true},
		"kb and docs": {Message: "q", KnowledgeBaseIDs: []string{"kb-1"}, DocumentIDs: []string{"doc-1"}},
	}
	want := map[string][]string{
		"exact":    {"keyword"},
		"semantic": {"vector"},
		"auto":     {"vector", "keyword"},
		"":         {"vector", "keyword"},
	}
	for name, req := range requests {
		for svcName, svc := range services {
			for _, traced := range []bool{false, true} {
				for mode, expect := range want {
					rt := svc.getRuntimeRetriever(req)
					if traced {
						rt = newTracedRetriever(rt, newTraceCollector("t"))
					}
					db.take()
					kt := internalTool.NewKnowledgeTool(rt, 5, 1500, 15000)
					if _, err := kt.InvokableRun(context.Background(), `{"query":"ERR_42","mode":"`+mode+`"}`); err != nil {
						t.Fatalf("%s/%s traced=%v mode=%q: %v", name, svcName, traced, mode, err)
					}
					if got := db.take(); !reflect.DeepEqual(got, expect) {
						t.Errorf("%s/%s traced=%v mode=%q: searches %v, want %v", name, svcName, traced, mode, got, expect)
					}
				}
			}
		}
	}
}

// The project-context recovery tries "exact" and "semantic" passes on purpose.
func TestRecoveryModesReachTheStore(t *testing.T) {
	db := &searchLog{MemoryVectorDB: container.NewMemoryVectorDB(3, "")}
	svc := productionRetriever(t, db, nil)
	rt := newTracedRetriever(svc.getRuntimeRetriever(&ChatRequest{Message: "q"}), newTraceCollector("t"))

	for mode, expect := range map[string][]string{"exact": {"keyword"}, "semantic": {"vector"}} {
		db.take()
		if _, err := retrieveForRecoveryMode(context.Background(), rt, "README", mode); err != nil {
			t.Fatal(err)
		}
		if got := db.take(); !reflect.DeepEqual(got, expect) {
			t.Errorf("mode %q: searches %v, want %v", mode, got, expect)
		}
	}
}

func TestTracedRetrieverRecordsTheMode(t *testing.T) {
	trace := newTraceCollector("t")
	rt := newTracedRetriever(fakeRetriever{}, trace)
	if _, err := rt.Retrieve(context.Background(), "q", retrieval.WithMode(retrieval.ModeExact)); err != nil {
		t.Fatal(err)
	}
	var mode any
	for _, step := range trace.snapshot() {
		if step.Stage == "retrieve" {
			mode = step.Metadata["mode"]
		}
	}
	if mode != retrieval.ModeExact {
		t.Fatalf("retrieve step mode = %v, want %q", mode, retrieval.ModeExact)
	}
}
