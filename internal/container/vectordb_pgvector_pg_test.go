package container

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"sort"
	"strconv"
	"testing"

	"eino_agent/internal/database/postgres"
)

// pgVectorTestDB opens the database named by EINO_TEST_PG_DSN
// (postgres://user:pass@host:port/dbname?sslmode=disable) with pgvector
// available, and skips when it is unset. It works in its own table, which is
// dropped afterwards, so a development database is safe to point it at.
func pgVectorTestDB(t *testing.T, dims int) *PgVectorDB {
	t.Helper()
	raw := os.Getenv("EINO_TEST_PG_DSN")
	if raw == "" {
		t.Skip("EINO_TEST_PG_DSN not set")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("EINO_TEST_PG_DSN: %v", err)
	}
	port := 5432
	if p := u.Port(); p != "" {
		if port, err = strconv.Atoi(p); err != nil {
			t.Fatalf("EINO_TEST_PG_DSN port: %v", err)
		}
	}
	pw, _ := u.User.Password()
	sslmode := u.Query().Get("sslmode")
	if sslmode == "" {
		sslmode = "disable"
	}
	ctx := context.Background()
	pg, err := postgres.New(ctx, &postgres.Config{
		Host: u.Hostname(), Port: port, User: u.User.Username(), Password: pw,
		Database: u.Path[1:], SSLMode: sslmode,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	db := &PgVectorDB{pg: pg, dimensions: dims, tableName: fmt.Sprintf("rag_vectors_test_%d", rand.Int63())}
	t.Cleanup(func() {
		_ = pg.Exec(context.Background(), "DROP TABLE IF EXISTS "+db.tableName)
		pg.Close()
	})
	if err := db.initSchema(ctx, false); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return db
}

func ids(docs []*Document) []string {
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.ID)
	}
	sort.Strings(out)
	return out
}

// Choosing a document has to find that document's chunks even when a much
// larger number of chunks from other documents of the same knowledge base lie
// closer to the query. The filter is applied by the database, over all rows;
// applying it to a top-K taken first would return nothing here.
func TestPgVectorSearchScopedToDocumentsFindsFarAwayChunks(t *testing.T) {
	db := pgVectorTestDB(t, 3)
	ctx := context.Background()

	meta := func(kb, doc string) map[string]interface{} {
		return map[string]interface{}{"knowledge_base_id": kb, "knowledge_id": doc}
	}
	var docs []*Document
	for i := 0; i < 200; i++ {
		docs = append(docs, &Document{
			ID: fmt.Sprintf("crowd-%03d", i), Content: fmt.Sprintf("crowd chunk %d about the same subject", i),
			Vector: []float32{1, float32(i) * 0.001, 0}, Metadata: meta("kb-1", fmt.Sprintf("crowd-doc-%d", i%10)),
		})
	}
	docs = append(docs,
		&Document{ID: "t-1", Content: "the target chunk with ZEBRA", Vector: []float32{0, 1, 0}, Metadata: meta("kb-1", "target-doc")},
		&Document{ID: "t-2", Content: "another target chunk", Vector: []float32{0, 0.9, 0.1}, Metadata: meta("kb-1", "target-doc")},
		&Document{ID: "o-1", Content: "a chunk of another knowledge base with ZEBRA", Vector: []float32{0, 1, 0}, Metadata: meta("kb-2", "target-doc-2")},
	)
	if err := db.Upsert(ctx, docs); err != nil {
		t.Fatal(err)
	}
	query := []float32{1, 0, 0}

	unscoped, err := db.Search(ctx, query, 5, SearchFilter{KnowledgeBaseIDs: []string{"kb-1"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range unscoped {
		if d.Metadata["knowledge_id"] == "target-doc" {
			t.Fatalf("the knowledge base alone should be crowded out, got %v", ids(unscoped))
		}
	}

	for name, filter := range map[string]SearchFilter{
		"document":                {KnowledgeIDs: []string{"target-doc"}},
		"knowledge base+document": {KnowledgeBaseIDs: []string{"kb-1"}, KnowledgeIDs: []string{"target-doc"}},
	} {
		got, err := db.Search(ctx, query, 5, filter)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := []string{"t-1", "t-2"}; fmt.Sprint(ids(got)) != fmt.Sprint(want) {
			t.Errorf("%s: vector search returned %v, want %v", name, ids(got), want)
		}
	}

	// The same document under another knowledge base is not in scope.
	got, err := db.Search(ctx, query, 5, SearchFilter{KnowledgeBaseIDs: []string{"kb-2"}, KnowledgeIDs: []string{"target-doc"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a document outside the chosen knowledge base was returned: %v", ids(got))
	}

	kw, err := db.SearchKeyword(ctx, "ZEBRA", 5, SearchFilter{KnowledgeIDs: []string{"target-doc"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"t-1"}; fmt.Sprint(ids(kw)) != fmt.Sprint(want) {
		t.Errorf("keyword search returned %v, want %v", ids(kw), want)
	}
}
