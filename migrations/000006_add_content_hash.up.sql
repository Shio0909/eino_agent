-- 增量更新支持：文档级和 Chunk 级内容哈希
-- 用于检测内容变化，避免重复向量化

-- 文档级内容哈希 (SHA256 of raw content)
ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS content_hash VARCHAR(64) DEFAULT '';

-- Chunk 级内容哈希 (SHA256 of chunk content)
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS content_hash VARCHAR(64) DEFAULT '';


-- 加速 chunk diff 查询：按 knowledge_id + content_hash 查找
CREATE INDEX IF NOT EXISTS idx_chunks_knowledge_hash ON chunks(knowledge_id, content_hash);
