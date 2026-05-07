DROP INDEX IF EXISTS idx_chunks_knowledge_hash;
DROP INDEX IF EXISTS idx_knowledges_kb_source_hash;
ALTER TABLE chunks DROP COLUMN IF EXISTS content_hash;
ALTER TABLE knowledges DROP COLUMN IF EXISTS content_hash;
