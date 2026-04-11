-- Wiki 模式：LLM 编译的结构化知识页面
-- 用于 Karpathy LLM Wiki 模式，LLM 将原始文档编译为结构化 wiki 页面

-- wiki_pages: LLM 编译后的 wiki 页面
CREATE TABLE IF NOT EXISTS wiki_pages (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    knowledge_base_id VARCHAR(36) NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    source_knowledge_id VARCHAR(36) REFERENCES knowledges(id) ON DELETE SET NULL,
    path VARCHAR(500) NOT NULL,              -- 页面路径，如 'index.md', 'kubernetes/pods.md'
    title VARCHAR(500) NOT NULL,             -- 页面标题
    content TEXT NOT NULL,                   -- 完整 Markdown 内容
    page_type VARCHAR(20) NOT NULL DEFAULT 'topic', -- 'index', 'topic', 'entity'
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(knowledge_base_id, path)
);

CREATE INDEX idx_wiki_pages_kb_id ON wiki_pages(knowledge_base_id);
CREATE INDEX idx_wiki_pages_type ON wiki_pages(page_type);
CREATE INDEX idx_wiki_pages_content_fts ON wiki_pages USING gin (to_tsvector('simple', content));

-- wiki_links: 页面间的交叉引用
CREATE TABLE IF NOT EXISTS wiki_links (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    source_page_id VARCHAR(36) NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
    target_path VARCHAR(500) NOT NULL,
    target_page_id VARCHAR(36) REFERENCES wiki_pages(id) ON DELETE SET NULL,
    link_text VARCHAR(500),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_wiki_links_source ON wiki_links(source_page_id);
CREATE INDEX idx_wiki_links_target ON wiki_links(target_page_id);
