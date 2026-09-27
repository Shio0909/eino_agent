package graphrag

import (
	"reflect"
	"strings"
	"testing"
)

func TestRankGraphChunksDeterministic(t *testing.T) {
	nodes := []*GraphNode{
		{Name: "Eino", Chunks: []string{"c3", "c1", "c2"}},
		{Name: "CloudWeGo", Hops: 1, Chunks: []string{"c1", "c9", "c8"}},
		{Name: "ByteDance", Hops: 2, Chunks: []string{"c9", "c7"}},
	}
	// c1: 1 + 1/2 = 1.5；c2,c3: 1；c9: 1/2 + 1/3；c8: 1/2；c7: 1/3
	want := []string{"c1", "c2", "c3", "c9", "c8"}

	for i := 0; i < 50; i++ {
		ranked := rankGraphChunks(nodes, 5)
		got := make([]string, len(ranked))
		for j, sc := range ranked {
			got[j] = sc.ID
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("iteration %d: got %v, want %v", i, got, want)
		}
	}
}

func TestRankGraphChunksWeightAndHops(t *testing.T) {
	nodes := []*GraphNode{
		{Name: "a", Hops: 1, Weight: 0.2, Chunks: []string{"weak"}},
		{Name: "b", Hops: 1, Weight: 1.0, Chunks: []string{"strong", "strong"}},
	}
	ranked := rankGraphChunks(nodes, 0)
	if len(ranked) != 2 || ranked[0].ID != "strong" || ranked[1].ID != "weak" {
		t.Fatalf("unexpected ranking: %+v", ranked)
	}
	if ranked[0].Score != 0.5 {
		t.Fatalf("duplicate chunk ids in one node must count once, score=%v", ranked[0].Score)
	}
	if ranked[0].Hops != 1 {
		t.Fatalf("hops = %d, want 1", ranked[0].Hops)
	}
}

func TestBuildGraphDocuments(t *testing.T) {
	graph := &GraphData{
		Node: []*GraphNode{
			{Name: "Eino", Type: "Technology", Chunks: []string{"c2", "c1"}},
			{Name: "CloudWeGo", Hops: 1, Chunks: []string{"c3"}},
		},
		Relation: []*GraphRelation{{Node1: "Eino", Node2: "CloudWeGo", Type: "BELONGS_TO"}},
	}
	docs := buildGraphDocuments(graph, 2)
	if len(docs) != 3 {
		t.Fatalf("expected 2 chunk docs + 1 context doc, got %d", len(docs))
	}
	if docs[0].ID != "c1" || docs[1].ID != "c2" {
		t.Fatalf("chunk docs not sorted deterministically: %s, %s", docs[0].ID, docs[1].ID)
	}
	if docs[0].Score() != 1 {
		t.Fatalf("score = %v, want 1", docs[0].Score())
	}
	ctxDoc := docs[2]
	if ctxDoc.ID != "graph-context" || !strings.Contains(ctxDoc.Content, "[Technology]Eino") ||
		!strings.Contains(ctxDoc.Content, "Eino -[BELONGS_TO]-> CloudWeGo") {
		t.Fatalf("unexpected context doc: %+v", ctxDoc)
	}
}
