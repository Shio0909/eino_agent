DROP TRIGGER IF EXISTS update_knowledge_base_shares_updated_at ON knowledge_base_shares;
DROP TRIGGER IF EXISTS update_organization_members_updated_at ON organization_members;
DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;

DROP TABLE IF EXISTS knowledge_base_shares;
DROP TABLE IF EXISTS organization_members;
DROP TABLE IF EXISTS organizations;
