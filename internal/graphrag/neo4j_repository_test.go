package graphrag

import (
	"context"
	"strings"
	"testing"
)

func TestBuildSearchNodeQueryHopValidation(t *testing.T) {
	for _, hops := range []int{-1, 0, MaxAllowedHops + 1, 100} {
		if _, err := buildSearchNodeQuery("`ENTITYkb`", hops); err == nil {
			t.Fatalf("hops=%d should be rejected", hops)
		}
	}
	for hops := 1; hops <= MaxAllowedHops; hops++ {
		q, err := buildSearchNodeQuery("`ENTITYkb`", hops)
		if err != nil {
			t.Fatalf("hops=%d: %v", hops, err)
		}
		want := "-[*1.." + string(rune('0'+hops)) + "]-"
		if !strings.Contains(q, want) {
			t.Fatalf("hops=%d: query missing %q:\n%s", hops, want, q)
		}
	}
}

func TestBuildSearchNodeQueryIsParameterized(t *testing.T) {
	q, err := buildSearchNodeQuery("`ENTITYkb`", DefaultMaxHops)
	if err != nil {
		t.Fatal(err)
	}
	for _, param := range []string{"$nodes", "$maxSeeds", "$maxResults"} {
		if !strings.Contains(q, param) {
			t.Fatalf("query missing parameter %s", param)
		}
	}
	if !strings.Contains(q, "x:`ENTITYkb`") || !strings.Contains(q, "n:`ENTITYkb`") {
		t.Fatalf("namespace label not applied to seeds and path nodes:\n%s", q)
	}
	if !strings.Contains(q, "ORDER BY hops ASC, weight DESC") {
		t.Fatalf("results must be ordered by path length then weight:\n%s", q)
	}

	unscoped, err := buildSearchNodeQuery("", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(unscoped, "STARTS WITH 'ENTITY'") {
		t.Fatalf("unscoped query must restrict to ENTITY labels:\n%s", unscoped)
	}
}

func TestLabelQuotesNamespace(t *testing.T) {
	repo := NewNeo4jRepository(nil).(*Neo4jRepository)
	got := repo.Label(NameSpace{KnowledgeBase: "kb-1", Knowledge: "x`) DETACH DELETE n //"})
	want := "`ENTITYkb_1`:`ENTITYx``) DETACH DELETE n //`"
	if got != want {
		t.Fatalf("Label = %s, want %s", got, want)
	}
}

func TestWithMaxHopsNormalization(t *testing.T) {
	cases := map[int]int{0: DefaultMaxHops, -3: DefaultMaxHops, 1: 1, 3: 3, 9: MaxAllowedHops}
	for in, want := range cases {
		repo := NewNeo4jRepository(nil, WithMaxHops(in)).(*Neo4jRepository)
		if repo.maxHops != want {
			t.Fatalf("WithMaxHops(%d) = %d, want %d", in, repo.maxHops, want)
		}
	}
}

func TestSearchAccumulatorMergesPaths(t *testing.T) {
	acc := newSearchAccumulator()
	seed := map[string]any{"name": "Eino", "entity_type": "Technology", "chunks": []any{"c1"}}
	near := map[string]any{"name": "CloudWeGo", "chunks": []any{"c2"}}
	far := map[string]any{"name": "ByteDance", "chunks": []any{"c3"}}
	rel := func(src, typ, dst string) map[string]any {
		return map[string]any{"source": src, "type": typ, "target": dst}
	}

	acc.add(seed, far, 2, 1.0, []any{rel("Eino", "BELONGS_TO", "CloudWeGo"), rel("CloudWeGo", "OWNED_BY", "ByteDance")})
	acc.add(seed, near, 1, 1.0, []any{rel("Eino", "BELONGS_TO", "CloudWeGo")})
	acc.add(near, far, 1, 0.5, []any{rel("CloudWeGo", "OWNED_BY", "ByteDance")})
	acc.add(map[string]any{"name": "Lonely"}, nil, 0, 1.0, nil)

	g := acc.graph()
	gotNames := make([]string, len(g.Node))
	for i, n := range g.Node {
		gotNames[i] = n.Name
	}
	wantNames := []string{"CloudWeGo", "Eino", "Lonely", "ByteDance"}
	if strings.Join(gotNames, ",") != strings.Join(wantNames, ",") {
		t.Fatalf("node order = %v, want %v", gotNames, wantNames)
	}
	for _, n := range g.Node {
		if n.Name == "ByteDance" && (n.Hops != 1 || n.Weight != 0.5) {
			t.Fatalf("ByteDance should keep shortest path (hops=1, weight=0.5), got %+v", n)
		}
	}
	if len(g.Relation) != 2 {
		t.Fatalf("relations should be deduplicated, got %d", len(g.Relation))
	}
}

func TestFilterSeedsByType(t *testing.T) {
	nodes := []*GraphNode{
		{Name: "Go", Type: "Language"},
		{Name: "Go", Type: "Game"},
		{Name: "Gopher", Type: "Mascot", Hops: 1},
		{Name: "Untyped"},
	}
	// "Go" 带类型约束 Language；"Untyped" 由无类型查询实体命中
	entities := []QueryEntity{{Name: "Go", Type: "Language"}, {Name: "Untyped"}}
	got := filterSeedsByType(nodes, entities)
	if len(got) != 3 {
		t.Fatalf("expected 3 nodes after filtering, got %d", len(got))
	}
	for _, n := range got {
		if n.Type == "Game" {
			t.Fatalf("type-mismatched seed should be filtered: %+v", n)
		}
	}
}

func TestNilDriverIsNoop(t *testing.T) {
	repo := NewNeo4jRepository(nil)
	g, err := repo.SearchNode(context.Background(), NameSpace{KnowledgeBase: "kb"}, []QueryEntity{{Name: "x"}})
	if err != nil || g != nil {
		t.Fatalf("expected nil graph without error, got %+v, %v", g, err)
	}
}

func TestInitNeo4jDriverRequiresCredentialsFromConfig(t *testing.T) {
	t.Setenv("NEO4J_URI", "bolt://should-not-be-used:7687")
	t.Setenv("NEO4J_PASSWORD", "should-not-be-used")

	if _, err := InitNeo4jDriver(context.Background(), &Config{Neo4jPass: "secret"}); err == nil ||
		!strings.Contains(err.Error(), "uri is required") {
		t.Fatalf("expected missing uri error, got %v", err)
	}
	if _, err := InitNeo4jDriver(context.Background(), &Config{Neo4jURI: "bolt://127.0.0.1:1"}); err == nil ||
		!strings.Contains(err.Error(), "password is required") {
		t.Fatalf("expected missing password error, got %v", err)
	}
}

func TestServiceCloseWithoutDriver(t *testing.T) {
	svc := NewService(&Config{Enabled: true}, nil, NewNeo4jRepository(nil))
	if err := svc.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := svc.Close(context.Background()); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	var nilSvc *Service
	if err := nilSvc.Close(context.Background()); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
}
