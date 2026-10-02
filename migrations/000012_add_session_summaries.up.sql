-- Rolling summary of a session's older messages (context compaction).
-- Messages stay untouched in messages; the summary stands in for everything
-- up to and including covered_until_message_id when building prompt memory.
CREATE TABLE IF NOT EXISTS session_summaries (
    session_id VARCHAR(36) PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    summary TEXT NOT NULL,
    covered_until_message_id VARCHAR(36) NOT NULL,
    covered_until_at TIMESTAMP WITH TIME ZONE NOT NULL,
    covered_count INTEGER NOT NULL DEFAULT 0,
    degraded BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
