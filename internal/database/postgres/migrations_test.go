package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKnowledgeContentHashIndexIsCreatedOnlyByDedicatedMigration(t *testing.T) {
	migrationDir := "../../../migrations"
	matches := make([]string, 0)

	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		path := filepath.Join(migrationDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) returned error: %v", path, err)
		}
		if strings.Contains(string(content), "idx_knowledges_kb_source_hash") {
			matches = append(matches, entry.Name())
		}
	}

	if len(matches) != 1 || matches[0] != "000009_add_knowledge_content_hash_index.up.sql" {
		t.Fatalf("idx_knowledges_kb_source_hash should only be created by 000009, got %#v", matches)
	}
}

func TestListMigrationFilesReturnsAllUpMigrations(t *testing.T) {
	files, err := ListMigrationFiles("../../../migrations")
	if err != nil {
		t.Fatalf("ListMigrationFiles returned error: %v", err)
	}

	want := []string{
		"000001_init.up.sql",
		"000002_add_wiki_mode.up.sql",
		"000003_add_wiki_pages.up.sql",
		"000004_create_llm_audit_logs.up.sql",
		"000005_add_embed_fingerprint.up.sql",
		"000006_add_content_hash.up.sql",
		"000007_add_kb_access_control.up.sql",
		"000008_create_request_traces.up.sql",
		"000009_add_knowledge_content_hash_index.up.sql",
		"000010_add_enrichment_status.up.sql",
	}
	if len(files) != len(want) {
		t.Fatalf("got %d migration files, want %d: %#v", len(files), len(want), files)
	}
	for i := range want {
		if files[i].Name != want[i] {
			t.Fatalf("files[%d] = %q, want %q", i, files[i].Name, want[i])
		}
	}
}
