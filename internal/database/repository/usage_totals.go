package repository

import (
	"context"

	"eino_agent/internal/database/postgres"
	"eino_agent/internal/usage"
)

type pgUsageTotalsStore struct {
	db *postgres.DB
}

// NewUsageStore persists cumulative LLM usage in the llm_usage_totals row.
func NewUsageStore(db *postgres.DB) usage.Store {
	return &pgUsageTotalsStore{db: db}
}

// Add applies delta to the stored totals and returns the result. An all-zero
// delta reads the current value, creating the row on first use.
func (s *pgUsageTotalsStore) Add(ctx context.Context, d usage.Totals) (usage.Totals, error) {
	var t usage.Totals
	err := s.db.QueryRow(ctx, `
		INSERT INTO llm_usage_totals (id, prompt_tokens, completion_tokens, total_tokens)
		VALUES (1, $1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			prompt_tokens     = llm_usage_totals.prompt_tokens     + EXCLUDED.prompt_tokens,
			completion_tokens = llm_usage_totals.completion_tokens + EXCLUDED.completion_tokens,
			total_tokens      = llm_usage_totals.total_tokens      + EXCLUDED.total_tokens,
			updated_at        = CURRENT_TIMESTAMP
		RETURNING prompt_tokens, completion_tokens, total_tokens
	`, d.PromptTokens, d.CompletionTokens, d.TotalTokens).Scan(&t.PromptTokens, &t.CompletionTokens, &t.TotalTokens)
	return t, err
}
