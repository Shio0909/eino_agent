package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsDotEnvBeforeExpandingConfig(t *testing.T) {
	t.Setenv("EINO_TEST_DOTENV_VALUE", "")
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("EINO_TEST_DOTENV_VALUE=from-dotenv\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("llm:\n  api_key: ${EINO_TEST_DOTENV_VALUE:-}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if cfg.LLM.APIKey != "from-dotenv" {
		t.Fatalf("LLM.APIKey = %q, want from-dotenv", cfg.LLM.APIKey)
	}
}

func TestLoadDotEnvDoesNotOverrideEnvironment(t *testing.T) {
	t.Setenv("EINO_TEST_DOTENV_OVERRIDE", "from-env")
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("EINO_TEST_DOTENV_OVERRIDE=from-dotenv\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("llm:\n  api_key: ${EINO_TEST_DOTENV_OVERRIDE:-}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if cfg.LLM.APIKey != "from-env" {
		t.Fatalf("LLM.APIKey = %q, want from-env", cfg.LLM.APIKey)
	}
}
