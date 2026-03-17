-- Migration: 000001_init (rollback)
-- Description: Remove all tables

-- Drop triggers first
DROP TRIGGER IF EXISTS update_tenants_updated_at ON tenants;
DROP TRIGGER IF EXISTS update_models_updated_at ON models;
DROP TRIGGER IF EXISTS update_knowledge_bases_updated_at ON knowledge_bases;
DROP TRIGGER IF EXISTS update_knowledges_updated_at ON knowledges;
DROP TRIGGER IF EXISTS update_sessions_updated_at ON sessions;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_custom_agents_updated_at ON custom_agents;
DROP TRIGGER IF EXISTS update_mcp_services_updated_at ON mcp_services;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order of creation (respect foreign keys)
DROP TABLE IF EXISTS mcp_services;
DROP TABLE IF EXISTS custom_agents;
DROP TABLE IF EXISTS auth_tokens;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS embeddings;
DROP TABLE IF EXISTS chunks;
DROP TABLE IF EXISTS knowledges;
DROP TABLE IF EXISTS knowledge_tags;
DROP TABLE IF EXISTS knowledge_bases;
DROP TABLE IF EXISTS models;
DROP TABLE IF EXISTS tenants;

-- Note: Extensions are kept as they might be used by other databases
-- DROP EXTENSION IF EXISTS pg_trgm;
-- DROP EXTENSION IF EXISTS vector;
-- DROP EXTENSION IF EXISTS "uuid-ossp";
