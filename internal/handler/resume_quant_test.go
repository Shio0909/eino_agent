package handler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"eino_agent/internal/cache"
	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/database/postgres"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/docreader"

	einoembedding "github.com/cloudwego/eino/components/embedding"
	"github.com/gin-gonic/gin"
)

// Counting embeddings isolate index work from model latency and semantic quality.
type quantEmbedder struct{ texts, calls int }

func (e *quantEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...einoembedding.Option) ([][]float64, error) {
	e.texts += len(texts)
	e.calls++
	out := make([][]float64, len(texts))
	for i, text := range texts {
		sum := sha256.Sum256([]byte(text))
		out[i] = make([]float64, 32)
		for j, b := range sum {
			out[i][j] = float64(b) / 255
		}
	}
	return out, nil
}

func quantContext(kb, doc, chunk string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/quant", nil)
	c.Params = gin.Params{{Key: "id", Value: kb}, {Key: "docId", Value: doc}, {Key: "chunkId", Value: chunk}}
	c.Set("tenant_id", 1)
	return c, w
}

func TestResumeQuantChunkReindex(t *testing.T) {
	if os.Getenv("RESUME_QUANT_DB") != "1" {
		t.Skip("requires disposable PostgreSQL at localhost:55439; set RESUME_QUANT_DB=1")
	}
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	db, err := postgres.New(ctx, &postgres.Config{Host: "127.0.0.1", Port: 55439, User: "postgres", Password: "resume_quant_local", Database: "resume_quant", SSLMode: "disable"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.RunMigrations(ctx, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(ctx, `INSERT INTO tenants(id,name,api_key) VALUES(1,'quant','local-fixture') ON CONFLICT(id) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	vdb, cleanup, err := container.NewVectorDBProvider(ctx, &config.DatabaseConfig{Host: "127.0.0.1", Port: 55439, User: "postgres", Password: "resume_quant_local", DBName: "resume_quant", SSLMode: "disable"}, 32)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(ctx)
	if _, ok := vdb.(*container.PgVectorDB); !ok {
		t.Fatal("benchmark requires real pgvector; fallback forbidden")
	}
	for _, n := range []int{10, 100, 1000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			kb, doc := fmt.Sprintf("quant-kb-%d", n), fmt.Sprintf("quant-doc-%d", n)
			if err := db.Exec(ctx, `INSERT INTO knowledge_bases(id,tenant_id,name,description) VALUES($1,1,'quant','') ON CONFLICT(id) DO UPDATE SET description=''`, kb); err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(ctx, `INSERT INTO knowledges(id,knowledge_base_id,name,source_type) VALUES($1,$2,'quant','file') ON CONFLICT(id) DO NOTHING`, doc, kb); err != nil {
				t.Fatal(err)
			}
			repo := repository.NewChunkRepository(db)
			if err := repo.DeleteByKnowledgeID(ctx, doc); err != nil {
				t.Fatal(err)
			}
			if err := vdb.DeleteByKnowledgeID(ctx, doc); err != nil {
				t.Fatal(err)
			}
			embed := &quantEmbedder{}
			h := &Handler{cfg: &config.Config{}, embedding: embed, vectorDB: vdb, chunkRepo: repo, kbRepo: repository.NewKnowledgeBaseRepository(db), retrievalCache: cache.NewNoopRetrievalCache(), importStateStore: newMemoryImportStateStore()}
			parsed := make([]docreader.ParsedChunk, n)
			for i := range parsed {
				parsed[i] = docreader.ParsedChunk{Seq: i, Content: fmt.Sprintf("Section %d. %s", i, strings.Repeat("Technical fixture about request routing, cancellation and retry. ", 8))}
			}
			start := time.Now()
			if err := h.processAndStoreChunks(ctx, kb, doc, "quant.txt", parsed); err != nil {
				t.Fatal(err)
			}
			fullMS := float64(time.Since(start).Microseconds()) / 1000
			fullTexts, fullCalls := embed.texts, embed.calls
			chunks, err := repo.GetByKnowledgeID(ctx, doc)
			if err != nil {
				t.Fatal(err)
			}
			if len(chunks) != n {
				t.Fatalf("stored chunks %d != %d", len(chunks), n)
			}
			editor := repo.(repository.ChunkEditor)
			target := chunks[n/2]
			edited := "REVISED: cancellation must release the request slot exactly once."
			if _, err := editor.UpdateContent(ctx, target.ID, edited); err != nil {
				t.Fatal(err)
			}
			embed.texts, embed.calls = 0, 0
			c, w := quantContext(kb, doc, target.ID)
			start = time.Now()
			h.ReindexEditedChunk(c)
			partialMS := float64(time.Since(start).Microseconds()) / 1000
			if w.Code != 200 {
				t.Fatalf("reindex status=%d body=%s", w.Code, w.Body.String())
			}
			var total, oldContent int
			if err := db.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE content=$2) FROM rag_vectors WHERE metadata->>'knowledge_id'=$1`, doc, target.Content).Scan(&total, &oldContent); err != nil {
				t.Fatal(err)
			}
			payload := map[string]any{"chunks": n, "full_embedding_texts": fullTexts, "full_embedding_batches": fullCalls, "partial_embedding_texts": embed.texts, "partial_embedding_batches": embed.calls, "full_local_ms": fullMS, "partial_local_ms": partialMS, "vectors_after_edit": total, "stale_vectors": oldContent, "embedding": "deterministic_hash_32d_not_a_model"}
			b, _ := json.Marshal(payload)
			t.Log("QUANT_CHUNK " + string(b))
			if total != n || oldContent != 0 {
				t.Fatalf("index consistency failed: vectors=%d want=%d stale=%d", total, n, oldContent)
			}
			if embed.texts != 1 {
				t.Fatalf("partial embed texts=%d", embed.texts)
			}
			revisions, err := editor.ListRevisions(ctx, target.ID)
			if err != nil || len(revisions) != 1 {
				t.Fatalf("revisions=%d err=%v", len(revisions), err)
			}
			if _, err := editor.RestoreRevision(ctx, target.ID, revisions[0].ID); err != nil {
				t.Fatal(err)
			}
			c, w = quantContext(kb, doc, target.ID)
			h.ReindexEditedChunk(c)
			if w.Code != 200 {
				t.Fatal(w.Body.String())
			}
			var restored int
			if err := db.QueryRow(ctx, `SELECT count(*) FROM rag_vectors WHERE metadata->>'knowledge_id'=$1 AND content=$2`, doc, target.Content).Scan(&restored); err != nil || restored != 1 {
				t.Fatalf("restored=%d err=%v", restored, err)
			}
		})
	}
}
