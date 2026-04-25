package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"eino_agent/internal/database/postgres"
)

type pgAccessControlRepo struct {
	db *postgres.DB
}

func NewAccessControlRepository(db *postgres.DB) AccessControlRepository {
	return &pgAccessControlRepo{db: db}
}

func (r *pgAccessControlRepo) CreateOrganization(ctx context.Context, org *Organization) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO organizations (tenant_id, name, description, owner_user_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`, org.TenantID, org.Name, org.Description, org.OwnerUserID).Scan(&org.ID, &org.CreatedAt, &org.UpdatedAt)
}

func (r *pgAccessControlRepo) AddOrganizationMember(ctx context.Context, member *OrganizationMember) error {
	role := member.Role
	if role == "" {
		role = AccessRoleViewer
	}
	return r.db.QueryRow(ctx, `
		INSERT INTO organization_members (organization_id, tenant_id, user_id, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (organization_id, tenant_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, updated_at = CURRENT_TIMESTAMP
		RETURNING id, created_at, updated_at
	`, member.OrganizationID, member.TenantID, member.UserID, role).Scan(&member.ID, &member.CreatedAt, &member.UpdatedAt)
}

func (r *pgAccessControlRepo) ShareKnowledgeBase(ctx context.Context, share *KnowledgeBaseShare) error {
	permission := share.Permission
	if permission == "" {
		permission = AccessRoleViewer
	}
	return r.db.QueryRow(ctx, `
		INSERT INTO knowledge_base_shares (knowledge_base_id, organization_id, source_tenant_id, shared_by_user_id, permission)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (knowledge_base_id, organization_id)
		DO UPDATE SET permission = EXCLUDED.permission, shared_by_user_id = EXCLUDED.shared_by_user_id, updated_at = CURRENT_TIMESTAMP
		RETURNING id, created_at, updated_at
	`, share.KnowledgeBaseID, share.OrganizationID, share.SourceTenantID, share.SharedByUserID, permission).Scan(&share.ID, &share.CreatedAt, &share.UpdatedAt)
}

func (r *pgAccessControlRepo) GetKnowledgeBaseRole(ctx context.Context, tenantID int, userID, kbID string) (AccessRole, error) {
	var ownerTenantID int
	err := r.db.QueryRow(ctx, `
		SELECT tenant_id FROM knowledge_bases WHERE id = $1 AND deleted_at IS NULL
	`, kbID).Scan(&ownerTenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if ownerTenantID == tenantID {
		return AccessRoleAdmin, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT s.permission, m.role
		FROM knowledge_base_shares s
		JOIN organization_members m ON m.organization_id = s.organization_id
		WHERE s.knowledge_base_id = $1 AND m.tenant_id = $2 AND m.user_id = $3
	`, kbID, tenantID, userID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	best := AccessRole("")
	for rows.Next() {
		var shareRole, memberRole AccessRole
		if err := rows.Scan(&shareRole, &memberRole); err != nil {
			return "", err
		}
		effective := MinAccessRole(shareRole, memberRole)
		if effective.Rank() > best.Rank() {
			best = effective
		}
	}
	return best, rows.Err()
}

func (r *pgAccessControlRepo) ListAccessibleKnowledgeBaseIDs(ctx context.Context, tenantID int, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT kb.id
		FROM knowledge_bases kb
		LEFT JOIN knowledge_base_shares s ON s.knowledge_base_id = kb.id
		LEFT JOIN organization_members m ON m.organization_id = s.organization_id AND m.tenant_id = $1 AND m.user_id = $2
		WHERE kb.deleted_at IS NULL
		  AND (kb.tenant_id = $1 OR m.id IS NOT NULL)
		ORDER BY kb.id
	`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
