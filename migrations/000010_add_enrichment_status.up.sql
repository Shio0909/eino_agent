ALTER TABLE knowledges
    ADD COLUMN IF NOT EXISTS enrichment_status VARCHAR(50) DEFAULT 'skipped',
    ADD COLUMN IF NOT EXISTS enrichment_error TEXT,
    ADD COLUMN IF NOT EXISTS enriched_chunk_count INTEGER DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_knowledges_enrichment_status ON knowledges(enrichment_status)
WHERE deleted_at IS NULL;
