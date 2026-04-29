package postgres

import "testing"

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
