-- Migration: 000001_init
-- Description: Initial schema for Eino RAG system (PostgreSQL + pgvector)
-- Inspired by WeKnora database design

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ============================================================================
-- Core Tables
-- ============================================================================

-- 租户表 (多租户支持)
CREATE TABLE IF NOT EXISTS tenants (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    api_key VARCHAR(64) UNIQUE NOT NULL,
    storage_quota BIGINT DEFAULT 10737418240,  -- 10GB default
    used_storage BIGINT DEFAULT 0,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_tenants_api_key ON tenants(api_key);
CREATE INDEX idx_tenants_deleted_at ON tenants(deleted_at);

-- 模型配置表
CREATE TABLE IF NOT EXISTS models (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,  -- 'llm', 'embedding', 'rerank'
    provider VARCHAR(100) NOT NULL,  -- 'openai', 'azure', 'local'
    model_name VARCHAR(255) NOT NULL,
    config JSONB DEFAULT '{}',
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_models_tenant_id ON models(tenant_id);
CREATE INDEX idx_models_type ON models(type);

-- 知识库表
CREATE TABLE IF NOT EXISTS knowledge_bases (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    embedding_model_id VARCHAR(36) REFERENCES models(id),
    embedding_dimensions INTEGER DEFAULT 1536,
    chunking_config JSONB DEFAULT '{"chunk_size": 500, "chunk_overlap": 50}',
    extract_config JSONB DEFAULT '{"enable_ocr": false, "enable_vlm": false}',
    document_count INTEGER DEFAULT 0,
    chunk_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_knowledge_bases_tenant_id ON knowledge_bases(tenant_id);
CREATE INDEX idx_knowledge_bases_deleted_at ON knowledge_bases(deleted_at);

-- 标签表 (知识分类)
CREATE TABLE IF NOT EXISTS knowledge_tags (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    knowledge_base_id VARCHAR(36) NOT NULL REFERENCES knowledge_bases(id),
    name VARCHAR(255) NOT NULL,
    parent_id VARCHAR(36) REFERENCES knowledge_tags(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_knowledge_tags_kb_id ON knowledge_tags(knowledge_base_id);
CREATE INDEX idx_knowledge_tags_parent_id ON knowledge_tags(parent_id);

-- 知识文档表
CREATE TABLE IF NOT EXISTS knowledges (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    knowledge_base_id VARCHAR(36) NOT NULL REFERENCES knowledge_bases(id),
    tag_id VARCHAR(36) REFERENCES knowledge_tags(id),
    name VARCHAR(512) NOT NULL,
    source_type VARCHAR(50) NOT NULL DEFAULT 'file',  -- 'file', 'faq', 'url'
    file_name VARCHAR(512),
    file_type VARCHAR(50),
    file_size BIGINT DEFAULT 0,
    file_path VARCHAR(1024),
    parse_status VARCHAR(50) DEFAULT 'pending',  -- 'pending', 'processing', 'completed', 'failed'
    parse_error TEXT,
    chunk_count INTEGER DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_knowledges_kb_id ON knowledges(knowledge_base_id);
CREATE INDEX idx_knowledges_tag_id ON knowledges(tag_id);
CREATE INDEX idx_knowledges_parse_status ON knowledges(parse_status);
CREATE INDEX idx_knowledges_deleted_at ON knowledges(deleted_at);

-- 文本块表
CREATE TABLE IF NOT EXISTS chunks (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    knowledge_id VARCHAR(36) NOT NULL REFERENCES knowledges(id),
    knowledge_base_id VARCHAR(36) NOT NULL REFERENCES knowledge_bases(id),
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    content_length INTEGER DEFAULT 0,
    parent_chunk_id VARCHAR(36) REFERENCES chunks(id),
    start_pos INTEGER DEFAULT 0,
    end_pos INTEGER DEFAULT 0,
    image_info JSONB,  -- 图片信息 (多模态)
    flags INTEGER DEFAULT 1,  -- 位标志: 1=recommended
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_chunks_knowledge_id ON chunks(knowledge_id);
CREATE INDEX idx_chunks_kb_id ON chunks(knowledge_base_id);
CREATE INDEX idx_chunks_parent_id ON chunks(parent_chunk_id);

-- 向量嵌入表
CREATE TABLE IF NOT EXISTS embeddings (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    chunk_id VARCHAR(36) NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    knowledge_id VARCHAR(36) NOT NULL REFERENCES knowledges(id),
    knowledge_base_id VARCHAR(36) NOT NULL REFERENCES knowledge_bases(id),
    tag_id VARCHAR(36) REFERENCES knowledge_tags(id),
    content TEXT NOT NULL,
    embedding vector(1536),  -- 默认 1536 维，可根据模型调整
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 向量相似度搜索索引 (HNSW)
CREATE INDEX idx_embeddings_vector ON embeddings 
USING hnsw (embedding vector_cosine_ops) 
WITH (m = 16, ef_construction = 64);

CREATE INDEX idx_embeddings_kb_id ON embeddings(knowledge_base_id);
CREATE INDEX idx_embeddings_knowledge_id ON embeddings(knowledge_id);
CREATE INDEX idx_embeddings_tag_id ON embeddings(tag_id);

-- 全文搜索索引
CREATE INDEX idx_embeddings_content_trgm ON embeddings USING gin (content gin_trgm_ops);

-- ============================================================================
-- Chat & Session Tables
-- ============================================================================

-- 会话表
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    user_id VARCHAR(36),
    agent_id VARCHAR(36),
    title VARCHAR(512),
    knowledge_base_ids TEXT[],  -- 关联的知识库 ID 列表
    retrieval_config JSONB DEFAULT '{}',
    similarity_threshold FLOAT DEFAULT 0.7,
    top_k INTEGER DEFAULT 5,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_sessions_tenant_id ON sessions(tenant_id);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_agent_id ON sessions(agent_id);
CREATE INDEX idx_sessions_deleted_at ON sessions(deleted_at);

-- 消息表
CREATE TABLE IF NOT EXISTS messages (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    session_id VARCHAR(36) NOT NULL REFERENCES sessions(id),
    role VARCHAR(50) NOT NULL,  -- 'user', 'assistant', 'system'
    content TEXT NOT NULL,
    knowledge_references JSONB,  -- 引用的知识块
    agent_steps JSONB,  -- Agent 执行步骤
    mentioned_items JSONB DEFAULT '[]',  -- @提及的项目
    tokens_used INTEGER DEFAULT 0,
    latency_ms INTEGER DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_messages_session_id ON messages(session_id);
CREATE INDEX idx_messages_role ON messages(role);
CREATE INDEX idx_messages_created_at ON messages(created_at);

-- ============================================================================
-- User & Auth Tables
-- ============================================================================

-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    username VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    password_hash VARCHAR(255),
    is_admin BOOLEAN DEFAULT false,
    status VARCHAR(50) DEFAULT 'active',  -- 'active', 'disabled'
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(tenant_id, username),
    UNIQUE(tenant_id, email)
);

CREATE INDEX idx_users_tenant_id ON users(tenant_id);
CREATE INDEX idx_users_email ON users(email);

-- 认证令牌表
CREATE TABLE IF NOT EXISTS auth_tokens (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    token_type VARCHAR(50) DEFAULT 'access',  -- 'access', 'refresh'
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_auth_tokens_user_id ON auth_tokens(user_id);
CREATE INDEX idx_auth_tokens_hash ON auth_tokens(token_hash);
CREATE INDEX idx_auth_tokens_expires_at ON auth_tokens(expires_at);

-- ============================================================================
-- Agent & Tools Tables
-- ============================================================================

-- 自定义 Agent 表 (GPTs 风格)
CREATE TABLE IF NOT EXISTS custom_agents (
    id VARCHAR(36) NOT NULL DEFAULT uuid_generate_v4()::text,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    avatar VARCHAR(64),
    is_builtin BOOLEAN DEFAULT false,
    config JSONB NOT NULL DEFAULT '{}',
    -- config 结构:
    -- {
    --   "agent_mode": "quick-answer" | "deep-research",
    --   "system_prompt": "...",
    --   "model_id": "...",
    --   "temperature": 0.7,
    --   "max_tokens": 2048,
    --   "allowed_tools": ["search", "calculator"],
    --   "kb_selection_mode": "all" | "selected",
    --   "knowledge_bases": ["kb_id_1", "kb_id_2"],
    --   "web_search_enabled": false
    -- }
    created_by VARCHAR(36),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    PRIMARY KEY (id, tenant_id)
);

CREATE INDEX idx_custom_agents_tenant_id ON custom_agents(tenant_id);
CREATE INDEX idx_custom_agents_is_builtin ON custom_agents(is_builtin);
CREATE INDEX idx_custom_agents_deleted_at ON custom_agents(deleted_at);

-- MCP 服务配置表
CREATE TABLE IF NOT EXISTS mcp_services (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    endpoint VARCHAR(512) NOT NULL,
    transport_type VARCHAR(50) DEFAULT 'stdio',  -- 'stdio', 'http', 'websocket'
    tools JSONB DEFAULT '[]',  -- 可用工具列表
    config JSONB DEFAULT '{}',
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_mcp_services_tenant_id ON mcp_services(tenant_id);
CREATE INDEX idx_mcp_services_enabled ON mcp_services(enabled);

-- ============================================================================
-- Helper Functions
-- ============================================================================

-- 更新 updated_at 触发器函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 为需要的表添加触发器
CREATE TRIGGER update_tenants_updated_at BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_models_updated_at BEFORE UPDATE ON models
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_knowledge_bases_updated_at BEFORE UPDATE ON knowledge_bases
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_knowledges_updated_at BEFORE UPDATE ON knowledges
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sessions_updated_at BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_custom_agents_updated_at BEFORE UPDATE ON custom_agents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_mcp_services_updated_at BEFORE UPDATE ON mcp_services
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- Initial Data
-- ============================================================================

-- 创建默认租户
INSERT INTO tenants (name, api_key, config) VALUES 
('default', 'sk-eino-default-key-' || substr(md5(random()::text), 1, 16), '{}')
ON CONFLICT DO NOTHING;
