-- Cumulative LLM token consumption of this deployment. A single row, updated by
-- adding deltas, so that concurrent writers and restarts never lose each
-- other's counts.
CREATE TABLE IF NOT EXISTS llm_usage_totals (
    id                SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    prompt_tokens     BIGINT NOT NULL DEFAULT 0 CHECK (prompt_tokens >= 0),
    completion_tokens BIGINT NOT NULL DEFAULT 0 CHECK (completion_tokens >= 0),
    total_tokens      BIGINT NOT NULL DEFAULT 0 CHECK (total_tokens >= 0),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
