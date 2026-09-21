-- Manual chunk iteration history. The current chunk remains in chunks;
-- revisions preserve the previous content before an edit or rollback.
CREATE TABLE IF NOT EXISTS chunk_revisions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    chunk_id VARCHAR(36) NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chunk_revisions_chunk_id_created
    ON chunk_revisions(chunk_id, created_at DESC);
