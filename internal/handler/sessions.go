package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/database/repository"
)

// ListSessions 获取会话列表
// @Summary 会话列表
// @Description 获取所有聊天会话
// @Tags 会话
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /sessions [get]
func (h *Handler) ListSessions(c *gin.Context) {
	if h.sessionRepo == nil {
		c.JSON(http.StatusOK, gin.H{"sessions": []any{}, "message": "数据库未连接"})
		return
	}

	tenantID := h.getTenantID(c)
	userID := ""
	if h.getUserRole(c) != "admin" {
		userID = h.getUserID(c)
	}

	sessions, err := h.sessionRepo.List(c.Request.Context(), tenantID, userID, 0, 100)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// CreateSession 创建会话
// @Summary 创建会话
// @Description 创建新的聊天会话
// @Tags 会话
// @Accept json
// @Produce json
// @Success 201 {object} repository.Session
// @Failure 400 {object} map[string]string
// @Router /sessions [post]
func (h *Handler) CreateSession(c *gin.Context) {
	if h.sessionRepo == nil {
		respondError(c, http.StatusServiceUnavailable, "数据库未连接")
		return
	}

	var req struct {
		Title            string   `json:"title"`
		KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	session := &repository.Session{
		TenantID:            h.getTenantID(c),
		UserID:              h.getUserID(c),
		Title:               req.Title,
		KnowledgeBaseIDs:    req.KnowledgeBaseIDs,
		SimilarityThreshold: 0.7,
		TopK:                5,
	}

	if err := h.sessionRepo.Create(c.Request.Context(), session); err != nil {
		h.audit(c, "session.create", req.Title, false, map[string]interface{}{"error": err.Error()})
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, "session.create", session.ID, true, map[string]interface{}{"title": req.Title})

	c.JSON(http.StatusCreated, session)
}

// GetSession 获取会话详情
// @Summary 会话详情
// @Description 获取指定会话的详细信息
// @Tags 会话
// @Produce json
// @Param id path string true "会话 ID"
// @Success 200 {object} repository.Session
// @Failure 404 {object} map[string]string
// @Router /sessions/{id} [get]
func (h *Handler) GetSession(c *gin.Context) {
	if h.sessionRepo == nil {
		respondError(c, http.StatusServiceUnavailable, "数据库未连接")
		return
	}

	id := c.Param("id")
	session, ok := h.ensureSessionAccess(c, id)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, session)
}

// DeleteSession 删除会话
// @Summary 删除会话
// @Description 删除指定聊天会话
// @Tags 会话
// @Param id path string true "会话 ID"
// @Success 200 {object} map[string]string
// @Router /sessions/{id} [delete]
func (h *Handler) DeleteSession(c *gin.Context) {
	if h.sessionRepo == nil {
		respondError(c, http.StatusServiceUnavailable, "数据库未连接")
		return
	}

	id := c.Param("id")
	if _, ok := h.ensureSessionAccess(c, id); !ok {
		return
	}
	if err := h.sessionRepo.Delete(c.Request.Context(), id); err != nil {
		h.audit(c, "session.delete", id, false, map[string]interface{}{"error": err.Error()})
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if h.sessionCache != nil {
		if err := h.sessionCache.InvalidateSession(c.Request.Context(), id); err != nil {
			log.Printf("[Handler] 会话缓存失效失败: session_id=%s err=%v", id, err)
		}
	}
	h.audit(c, "session.delete", id, true, nil)

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetSessionMessages 获取会话消息
// @Summary 会话消息列表
// @Description 获取指定会话的所有消息记录
// @Tags 会话
// @Produce json
// @Param id path string true "会话 ID"
// @Success 200 {object} map[string]interface{}
// @Router /sessions/{id}/messages [get]
func (h *Handler) GetSessionMessages(c *gin.Context) {
	if h.messageRepo == nil {
		c.JSON(http.StatusOK, gin.H{"messages": []any{}, "message": "数据库未连接"})
		return
	}

	sessionID := c.Param("id")
	if h.sessionRepo != nil {
		if _, ok := h.ensureSessionAccess(c, sessionID); !ok {
			return
		}
	}
	messages, err := h.messageRepo.ListBySession(c.Request.Context(), sessionID, 100)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": h.toMessageResponses(messages)})
}
