DROP INDEX IF EXISTS idx_chunks_knowledge_hash;
ALTER TABLE chunks DROP COLUMN IF EXISTS content_hash;
ALTER TABLE knowledges DROP COLUMN IF EXISTS content_hash;
