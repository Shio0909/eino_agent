package service

import (
	"context"
	"testing"
)

// legacyChunkSource mimics a vector row embedded before filenames were stamped
// into chunk metadata: ids only, no name keys.
func legacyChunkSource(chunkID, docID string) Source {
	return Source{DocID: chunkID, Content: "内容", Metadata: map[string]any{
		"knowledge_id": docID, "knowledge_base_id": "kb-1", "chunk_id": chunkID,
	}}
}

func TestEnrichSourceNamesStampsFilenameForLegacyChunks(t *testing.T) {
	svc := &ChatService{}
	svc.SetDocNameResolver(func(_ context.Context, id string) string {
		if id == "doc-1" {
			return "设计文档.md"
		}
		return ""
	})
	sources := []Source{legacyChunkSource("chunk-1", "doc-1")}

	out := svc.enrichSourceNames(context.Background(), sources)

	if got := metadataStringValue(out[0].Metadata, "source_filename"); got != "设计文档.md" {
		t.Fatalf("source_filename = %q, want %q", got, "设计文档.md")
	}
}

func TestEnrichSourceNamesKeepsExistingName(t *testing.T) {
	svc := &ChatService{}
	calls := 0
	svc.SetDocNameResolver(func(_ context.Context, id string) string {
		calls++
		return "other.md"
	})
	src := legacyChunkSource("chunk-1", "doc-1")
	src.Metadata["source_filename"] = "原始.md"

	out := svc.enrichSourceNames(context.Background(), []Source{src})

	if got := metadataStringValue(out[0].Metadata, "source_filename"); got != "原始.md" {
		t.Fatalf("source_filename = %q, want existing %q", got, "原始.md")
	}
	if calls != 0 {
		t.Fatalf("resolver called %d times for an already-named source, want 0", calls)
	}
}

func TestEnrichSourceNamesDoesNotMutateInputMetadata(t *testing.T) {
	svc := &ChatService{}
	svc.SetDocNameResolver(func(_ context.Context, id string) string { return "doc.md" })
	original := map[string]any{"knowledge_id": "doc-1", "chunk_id": "chunk-1"}
	sources := []Source{{DocID: "chunk-1", Content: "x", Metadata: original}}

	out := svc.enrichSourceNames(context.Background(), sources)

	if _, ok := original["source_filename"]; ok {
		t.Fatal("enrichSourceNames mutated the shared input metadata map")
	}
	if metadataStringValue(out[0].Metadata, "source_filename") != "doc.md" {
		t.Fatal("enriched copy lost the stamped filename")
	}
}

func TestEnrichSourceNamesNilResolverIsNoop(t *testing.T) {
	svc := &ChatService{}
	sources := []Source{legacyChunkSource("chunk-1", "doc-1")}

	out := svc.enrichSourceNames(context.Background(), sources)

	if _, ok := out[0].Metadata["source_filename"]; ok {
		t.Fatal("nil resolver must not stamp anything")
	}
}

func TestEnrichSourceNamesResolvesOncePerDocument(t *testing.T) {
	svc := &ChatService{}
	calls := map[string]int{}
	svc.SetDocNameResolver(func(_ context.Context, id string) string {
		calls[id]++
		return id + ".md"
	})
	sources := []Source{
		legacyChunkSource("chunk-1", "doc-1"),
		legacyChunkSource("chunk-2", "doc-1"),
		legacyChunkSource("chunk-3", "doc-2"),
	}

	out := svc.enrichSourceNames(context.Background(), sources)

	if calls["doc-1"] != 1 || calls["doc-2"] != 1 {
		t.Fatalf("resolver calls = %v, want one per unique document", calls)
	}
	for i, want := range []string{"doc-1.md", "doc-1.md", "doc-2.md"} {
		if got := metadataStringValue(out[i].Metadata, "source_filename"); got != want {
			t.Fatalf("sources[%d] source_filename = %q, want %q", i, got, want)
		}
	}
}

func TestEnrichSourceNamesSkipsUnresolvable(t *testing.T) {
	svc := &ChatService{}
	svc.SetDocNameResolver(func(_ context.Context, id string) string { return "" })

	noDoc := Source{DocID: "chunk-0", Content: "x", Metadata: map[string]any{"chunk_id": "chunk-0"}}
	unresolved := legacyChunkSource("chunk-1", "doc-404")
	out := svc.enrichSourceNames(context.Background(), []Source{noDoc, unresolved})

	for i := range out {
		if _, ok := out[i].Metadata["source_filename"]; ok {
			t.Fatalf("sources[%d] unexpectedly stamped", i)
		}
	}
}
