-- 回滚 wiki mode 支持
DROP INDEX IF EXISTS idx_chunks_kb_fts;
DROP INDEX IF EXISTS idx_chunks_content_trgm;
DROP INDEX IF EXISTS idx_chunks_content_fts;

ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS mode;
