-- 为知识库添加 mode 字段，支持 markdown 纯文本模式（无需 embedding）
ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS mode VARCHAR(20) DEFAULT 'vector' NOT NULL;

COMMENT ON COLUMN knowledge_bases.mode IS '知识库模式: vector(向量检索) / markdown(全文检索，无需embedding)';

-- 为 chunks 表添加全文搜索索引（markdown 模式使用）
CREATE INDEX IF NOT EXISTS idx_chunks_content_fts
    ON chunks USING gin (to_tsvector('simple', content));

-- 为 chunks 表添加 trigram 索引（ILIKE 模糊搜索回退）
CREATE INDEX IF NOT EXISTS idx_chunks_content_trgm
    ON chunks USING gin (content gin_trgm_ops);

-- 为 chunks 表添加 knowledge_base_id + content 联合索引（按 KB 范围搜索）
CREATE INDEX IF NOT EXISTS idx_chunks_kb_fts
    ON chunks (knowledge_base_id);
