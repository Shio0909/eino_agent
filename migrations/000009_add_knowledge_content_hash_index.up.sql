CREATE INDEX IF NOT EXISTS idx_knowledges_kb_source_hash ON knowledges(knowledge_base_id, source_type, content_hash)
WHERE deleted_at IS NULL AND content_hash <> '';
