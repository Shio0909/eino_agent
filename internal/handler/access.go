package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/database/repository"
)

func (h *Handler) ensureKnowledgeBaseAccess(c *gin.Context, id string) (*repository.KnowledgeBase, bool) {
	return h.ensureKnowledgeBaseRole(c, id, repository.AccessRoleViewer)
}

func (h *Handler) ensureKnowledgeBaseWriteAccess(c *gin.Context, id string) (*repository.KnowledgeBase, bool) {
	return h.ensureKnowledgeBaseRole(c, id, repository.AccessRoleEditor)
}

func (h *Handler) ensureKnowledgeBaseAdminAccess(c *gin.Context, id string) (*repository.KnowledgeBase, bool) {
	return h.ensureKnowledgeBaseRole(c, id, repository.AccessRoleAdmin)
}

func (h *Handler) ensureKnowledgeBaseRole(c *gin.Context, id string, required repository.AccessRole) (*repository.KnowledgeBase, bool) {
	kb, err := h.kbRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if kb == nil {
		respondError(c, http.StatusNotFound, "知识库不存在")
		return nil, false
	}
	role := repository.AccessRole("")
	if h.accessRepo != nil {
		role, err = h.accessRepo.GetKnowledgeBaseRole(c.Request.Context(), h.getTenantID(c), h.getUserID(c), id)
		if err != nil {
			respondError(c, http.StatusInternalServerError, err.Error())
			return nil, false
		}
	} else if kb.TenantID == h.getTenantID(c) {
		role = repository.AccessRoleAdmin
	}
	if !role.Allows(required) {
		respondError(c, http.StatusForbidden, "无权访问该知识库")
		return nil, false
	}
	return kb, true
}

func (h *Handler) resolveAuthorizedKnowledgeBaseIDs(c *gin.Context, requested []string) ([]string, bool) {
	if h.kbRepo == nil {
		// 无数据库时不存在任何知识库记录，fail-closed：返回空范围而不是放行。
		log.Printf("[Security] kbRepo 不可用，知识库授权解析为空范围")
		return []string{}, true
	}

	seen := make(map[string]struct{}, len(requested))
	ids := make([]string, 0, len(requested))
	for _, rawID := range requested {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		if h.accessRepo != nil {
			allowed, err := h.accessRepo.ListAccessibleKnowledgeBaseIDs(c.Request.Context(), h.getTenantID(c), h.getUserID(c))
			if err != nil {
				respondError(c, http.StatusInternalServerError, err.Error())
				return nil, false
			}
			return allowed, true
		}
		kbs, err := h.kbRepo.List(c.Request.Context(), h.getTenantID(c), 0, 1000)
		if err != nil {
			respondError(c, http.StatusInternalServerError, err.Error())
			return nil, false
		}
		allowed := make([]string, 0, len(kbs))
		for _, kb := range kbs {
			allowed = append(allowed, kb.ID)
		}
		return allowed, true
	}

	for _, id := range ids {
		if _, ok := h.ensureKnowledgeBaseAccess(c, id); !ok {
			return nil, false
		}
	}
	return ids, true
}

func (h *Handler) ensureSessionAccess(c *gin.Context, id string) (*repository.Session, bool) {
	session, err := h.sessionRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if session == nil {
		respondError(c, http.StatusNotFound, "会话不存在")
		return nil, false
	}
	if session.TenantID != h.getTenantID(c) {
		respondError(c, http.StatusForbidden, "无权访问该会话")
		return nil, false
	}
	if h.getUserRole(c) != "admin" && session.UserID != h.getUserID(c) {
		respondError(c, http.StatusForbidden, "无权访问该会话")
		return nil, false
	}
	return session, true
}
