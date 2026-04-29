package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type MigrationFile struct {
	Name string
	Path string
}

type MigrationResult struct {
	Applied []string
	Skipped []string
}

func ListMigrationFiles(dir string) ([]MigrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取迁移目录失败: %w", err)
	}

	files := make([]MigrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		files = append(files, MigrationFile{
			Name: entry.Name(),
			Path: filepath.Join(dir, entry.Name()),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})
	return files, nil
}

func (db *DB) RunMigrations(ctx context.Context, dir string) (MigrationResult, error) {
	files, err := ListMigrationFiles(dir)
	if err != nil {
		return MigrationResult{}, err
	}

	if err := db.bootstrapMigrationState(ctx); err != nil {
		return MigrationResult{}, err
	}

	result := MigrationResult{}
	for _, file := range files {
		applied, err := db.isMigrationApplied(ctx, file.Name)
		if err != nil {
			return result, err
		}
		if applied {
			result.Skipped = append(result.Skipped, file.Name)
			continue
		}
		content, err := os.ReadFile(file.Path)
		if err != nil {
			return result, fmt.Errorf("读取迁移文件 %s 失败: %w", file.Name, err)
		}
		if err := db.applyMigration(ctx, file.Name, string(content)); err != nil {
			return result, err
		}
		result.Applied = append(result.Applied, file.Name)
	}
	return result, nil
}

func (db *DB) bootstrapMigrationState(ctx context.Context) error {
	if err := db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
)`); err != nil {
		return fmt.Errorf("创建 schema_migrations 失败: %w", err)
	}

	baselineChecks := []struct {
		version string
		query   string
	}{
		{"000001_init.up.sql", "SELECT to_regclass('public.knowledge_bases') IS NOT NULL"},
		{"000002_add_wiki_mode.up.sql", "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'knowledge_bases' AND column_name = 'mode')"},
		{"000003_add_wiki_pages.up.sql", "SELECT to_regclass('public.wiki_pages') IS NOT NULL"},
		{"000004_create_llm_audit_logs.up.sql", "SELECT to_regclass('public.llm_audit_logs') IS NOT NULL"},
		{"000005_add_embed_fingerprint.up.sql", "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'knowledge_bases' AND column_name = 'embed_model_fingerprint')"},
		{"000006_add_content_hash.up.sql", "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'knowledges' AND column_name = 'content_hash')"},
		{"000007_add_kb_access_control.up.sql", "SELECT to_regclass('public.knowledge_base_shares') IS NOT NULL"},
		{"000008_create_request_traces.up.sql", "SELECT to_regclass('public.request_traces') IS NOT NULL"},
	}

	for _, check := range baselineChecks {
		var exists bool
		if err := db.QueryRow(ctx, check.query).Scan(&exists); err != nil {
			return fmt.Errorf("检查迁移 %s 状态失败: %w", check.version, err)
		}
		if !exists {
			continue
		}
		if err := db.Exec(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2) ON CONFLICT (version) DO NOTHING", check.version, time.Now()); err != nil {
			return fmt.Errorf("记录迁移 %s 状态失败: %w", check.version, err)
		}
	}
	return nil
}

func (db *DB) isMigrationApplied(ctx context.Context, version string) (bool, error) {
	var applied bool
	err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&applied)
	if err != nil {
		return false, fmt.Errorf("查询迁移 %s 状态失败: %w", version, err)
	}
	return applied, nil
}

func (db *DB) applyMigration(ctx context.Context, version, sql string) error {
	return db.WithTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, sql); err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %w", version, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)", version, time.Now()); err != nil {
			return fmt.Errorf("记录迁移 %s 失败: %w", version, err)
		}
		return nil
	})
}
