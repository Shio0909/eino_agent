DROP INDEX IF EXISTS idx_knowledges_enrichment_status;

ALTER TABLE knowledges
    DROP COLUMN IF EXISTS enriched_chunk_count,
    DROP COLUMN IF EXISTS enrichment_error,
    DROP COLUMN IF EXISTS enrichment_status;
