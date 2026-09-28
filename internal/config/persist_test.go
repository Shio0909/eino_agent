package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveKeepsEnvReferencesAndComments(t *testing.T) {
	t.Setenv("PERSIST_TEST_LLM_KEY", "sk-live-secret")
	t.Setenv("PERSIST_TEST_MODEL", "model-a")

	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := `# top comment
llm:
  provider: openai
  # key comes from the environment
  api_key: ${PERSIST_TEST_LLM_KEY}
  model_id: ${PERSIST_TEST_MODEL}
  temperature: 0.7
custom_extension:
  keep: me
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{}
	cfg.LLM.Provider = "openai"
	cfg.LLM.APIKey = "sk-changed-at-runtime"
	cfg.LLM.ModelID = "model-a"
	cfg.LLM.Temperature = 0.2

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save error = %v", err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)

	for _, want := range []string{
		"# top comment",
		"# key comes from the environment",
		"api_key: ${PERSIST_TEST_LLM_KEY}",
		"model_id: ${PERSIST_TEST_MODEL}",
		"temperature: 0.2",
		"keep: me",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("saved config missing %q:\n%s", want, text)
		}
	}
	for _, leaked := range []string{"sk-live-secret", "sk-changed-at-runtime"} {
		if strings.Contains(text, leaked) {
			t.Errorf("saved config leaked secret %q", leaked)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config perm = %o, want 600", perm)
	}
}

func TestSaveOverridesNonSecretEnvReferenceWhenChanged(t *testing.T) {
	t.Setenv("PERSIST_TEST_MODEL", "model-a")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("llm:\n  model_id: ${PERSIST_TEST_MODEL}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{}
	cfg.LLM.ModelID = "model-b"
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "model_id: model-b") {
		t.Fatalf("expected changed model to persist:\n%s", out)
	}
}
