package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"eino_agent/internal/config"
	"eino_agent/internal/docreader"
)

func TestParseOptionsFollowRAGConfig(t *testing.T) {
	h := &Handler{cfg: &config.Config{RAG: config.RAGConfig{ChunkSize: 120, ChunkOverlap: 0, ChunkStrategy: "recursive"}}}

	opts := h.parseOptions()

	if opts.ChunkSize != 120 {
		t.Errorf("ChunkSize = %d, want 120", opts.ChunkSize)
	}
	if opts.ChunkOverlap != 0 {
		t.Errorf("ChunkOverlap = %d, want 0 (an explicit zero must stay zero)", opts.ChunkOverlap)
	}
	if opts.Strategy != "recursive" {
		t.Errorf("Strategy = %q, want recursive", opts.Strategy)
	}
}

func TestParseOptionsWithoutConfigKeepDefaults(t *testing.T) {
	want := docreader.DefaultParseOptions()
	got := (&Handler{}).parseOptions()
	if got.ChunkSize != want.ChunkSize || got.ChunkOverlap != want.ChunkOverlap {
		t.Fatalf("got %d/%d, want %d/%d", got.ChunkSize, got.ChunkOverlap, want.ChunkSize, want.ChunkOverlap)
	}
}

func TestUploadedFileIsChunkedWithConfiguredSize(t *testing.T) {
	env := newSyncEnv()
	withDocReader(t, env)
	env.h.cfg.RAG.ChunkSize = 150
	env.h.cfg.RAG.ChunkOverlap = 10
	env.h.cfg.RAG.ChunkStrategy = "auto"
	text := strings.Repeat("今天天气很好，适合出门散步。我们一起去公园看看花。", 60)

	chunks, err := env.h.parseFileChunks(context.Background(), []byte(text), "note.txt", "text")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 5 {
		t.Fatalf("expected the text to need several 150-rune chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if n := utf8.RuneCountInString(c.Content); n > 150 {
			t.Errorf("chunk %d has %d runes, over the configured 150", i, n)
		}
	}
}

func TestFetchedPageIsChunkedWithConfiguredSize(t *testing.T) {
	var page strings.Builder
	page.WriteString("<html><body><h1>Handbook</h1>")
	for i := 1; i <= 6; i++ {
		fmt.Fprintf(&page, "<h2>Part %d</h2><p>%s</p>", i, strings.Repeat("This part explains one procedure in detail. ", 10))
	}
	page.WriteString("</body></html>")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page.String()))
	}))
	defer server.Close()

	env := newSyncEnv()
	docCfg := docreader.DefaultConfig()
	docCfg.AllowPrivateNetworks = true
	docCfg.RenderMode = "disabled"
	cli, err := docreader.NewClient(docCfg)
	if err != nil {
		t.Fatal(err)
	}
	env.h.docReaderCli = cli
	env.h.cfg.RAG.ChunkSize = 200
	env.h.cfg.RAG.ChunkOverlap = 20
	env.h.cfg.RAG.ChunkStrategy = "auto"

	chunks, _, err := env.h.parseURLChunks(context.Background(), server.URL, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 6 {
		t.Fatalf("expected at least one chunk per part, got %d", len(chunks))
	}
	for i, c := range chunks {
		// The heading path rides along on top of the 200-rune body.
		if n := utf8.RuneCountInString(c.Content); n > 200+len("# Handbook\n## Part 6\n\n") {
			t.Errorf("chunk %d has %d runes, far over the configured 200", i, n)
		}
	}
}
