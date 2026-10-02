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

func validConfig(mode string) *Config {
	cfg := &Config{}
	cfg.Server.Mode = mode
	setDefaults(cfg)
	return cfg
}

func TestValidateRejectsDefaultJWTInRelease(t *testing.T) {
	cfg := validConfig("release")
	cfg.Auth.Enabled = true
	cfg.Auth.AdminPassword = "a-strong-admin-password"
	if _, err := cfg.Validate(); err == nil {
		t.Fatal("expected default JWT secret to be rejected in release mode")
	}
}

func TestValidateWarnsOnlyInDebug(t *testing.T) {
	cfg := validConfig("debug")
	cfg.Auth.Enabled = true
	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("debug mode should only warn: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings for weak auth config")
	}
}

func TestValidateRejectsInvalidChunking(t *testing.T) {
	cfg := validConfig("debug")
	cfg.RAG.ChunkSize = 100
	cfg.RAG.ChunkOverlap = 100
	if _, err := cfg.Validate(); err == nil {
		t.Fatal("expected chunk_overlap >= chunk_size to be rejected")
	}
}

func TestValidateRejectsOpenMCPExportInRelease(t *testing.T) {
	cfg := validConfig("release")
	cfg.MCPExport.Enabled = true
	cfg.MCPExport.Transport = "streamable_http"
	if _, err := cfg.Validate(); err == nil {
		t.Fatal("expected unauthenticated network MCP export to be rejected in release mode")
	}
}

func TestLoadRedisDefaultsToLocalhostUnlessDisabled(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return p
	}

	cfg, err := Load(write("default.yaml", "llm:\n  model_id: m\n"))
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if cfg.Redis.Disabled || cfg.Redis.Addr != "localhost:6379" {
		t.Fatalf("default redis = %+v, want enabled at localhost:6379", cfg.Redis)
	}

	cfg, err = Load(write("off.yaml", "redis:\n  disabled: true\n"))
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if !cfg.Redis.Disabled || cfg.Redis.Addr != "" {
		t.Fatalf("disabled redis = %+v, want disabled with no address filled in", cfg.Redis)
	}
}
