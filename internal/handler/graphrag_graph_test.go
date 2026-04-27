package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/config"
	"eino_agent/internal/graphrag"
)

type fakeGraphRepository struct {
	graph *graphrag.VisGraph
}

func (r fakeGraphRepository) AddGraph(context.Context, graphrag.NameSpace, []*graphrag.GraphData) error {
	return nil
}

func (r fakeGraphRepository) DelGraph(context.Context, []graphrag.NameSpace) error {
	return nil
}

func (r fakeGraphRepository) SearchNode(context.Context, graphrag.NameSpace, []graphrag.QueryEntity) (*graphrag.GraphData, error) {
	return nil, errors.New("not implemented")
}

func (r fakeGraphRepository) GetGraphForVis(context.Context, graphrag.NameSpace, int) (*graphrag.VisGraph, error) {
	return r.graph, nil
}

func TestGetGraphRAGGraphReturnsVisualizationData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{
		cfg: &config.Config{},
		graphRAGService: graphrag.NewService(&graphrag.Config{Enabled: true}, nil, fakeGraphRepository{graph: &graphrag.VisGraph{
			Nodes: []graphrag.VisNode{{ID: "go", Label: "Go", Degree: 2, ChunkCount: 3}},
			Edges: []graphrag.VisEdge{{Source: "go", Target: "gc", Label: "HAS_CONCEPT"}},
		}}),
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/graphrag/kb-1/graph?limit=50", nil)
	ctx.Params = gin.Params{{Key: "kbId", Value: "kb-1"}}

	h.GetGraphRAGGraph(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var resp graphrag.VisGraph
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if len(resp.Nodes) != 1 || resp.Nodes[0].ID != "go" {
		t.Fatalf("nodes = %#v, want Go node", resp.Nodes)
	}
	if len(resp.Edges) != 1 || resp.Edges[0].Target != "gc" {
		t.Fatalf("edges = %#v, want go -> gc edge", resp.Edges)
	}
}
