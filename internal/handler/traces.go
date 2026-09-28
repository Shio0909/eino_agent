package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/database/repository"
)

func (h *Handler) GetTrace(c *gin.Context) {
	if h.requestTraceRepo == nil {
		respondError(c, http.StatusServiceUnavailable, "request trace repository not configured")
		return
	}
	traceID := strings.TrimSpace(c.Param("trace_id"))
	if traceID == "" {
		respondError(c, http.StatusBadRequest, "缺少 trace_id")
		return
	}
	trace, err := h.requestTraceRepo.GetByTraceID(c.Request.Context(), traceID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if trace == nil {
		respondError(c, http.StatusNotFound, "trace 不存在")
		return
	}
	if !h.canAccessTrace(c, trace) {
		respondError(c, http.StatusForbidden, "无权访问该 trace")
		return
	}
	c.JSON(http.StatusOK, trace)
}

func (h *Handler) ListSessionTraces(c *gin.Context) {
	if h.requestTraceRepo == nil {
		respondError(c, http.StatusServiceUnavailable, "request trace repository not configured")
		return
	}
	sessionID := c.Param("id")
	if h.sessionRepo != nil {
		if _, ok := h.ensureSessionAccess(c, sessionID); !ok {
			return
		}
	}
	page, pageSize := parsePagination(c, 20, 100)
	filter := repository.RequestTraceFilter{
		TenantID:  h.getTenantID(c),
		SessionID: sessionID,
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
	}
	if h.getUserRole(c) != "admin" {
		filter.UserID = h.getUserID(c)
	}
	traces, total, err := h.requestTraceRepo.List(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"traces": traces, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) canAccessTrace(c *gin.Context, trace *repository.RequestTrace) bool {
	if trace == nil || trace.TenantID != h.getTenantID(c) {
		return false
	}
	if h.getUserRole(c) == "admin" {
		return true
	}
	return trace.UserID == h.getUserID(c)
}
