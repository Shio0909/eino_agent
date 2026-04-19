-- Migration 000005: track which embedding model was used when indexing each knowledge base
-- embed_model_fingerprint stores "provider:model_id:dimensions" at index time.
-- Comparing against the current config fingerprint detects stale KBs.
ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS embed_model_fingerprint VARCHAR(255) NOT NULL DEFAULT '';
