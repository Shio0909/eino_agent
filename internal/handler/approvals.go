package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/approval"
)

func (h *Handler) approvalRuntime() *approval.Manager {
	if h.approvalManager != nil {
		return h.approvalManager
	}
	ttl := 5 * time.Minute
	if h.cfg != nil && h.cfg.HITL.ApprovalTimeoutSeconds > 0 {
		ttl = time.Duration(h.cfg.HITL.ApprovalTimeoutSeconds) * time.Second
	}
	h.approvalManager = approval.NewManager(ttl)
	if h.chatService != nil {
		h.chatService.SetApprovalManager(h.approvalManager)
	}
	return h.approvalManager
}

func (h *Handler) canAccessApproval(c *gin.Context, item approval.Approval) bool {
	if item.TenantID > 0 && item.TenantID != h.getTenantID(c) {
		return false
	}
	if h.getUserRole(c) == "admin" {
		return true
	}
	return item.UserID == "" || item.UserID == h.getUserID(c)
}

func (h *Handler) ListPendingApprovals(c *gin.Context) {
	items := h.approvalRuntime().ListPending(c.Request.Context(), h.getTenantID(c))
	filtered := make([]approval.Approval, 0, len(items))
	for _, item := range items {
		if h.canAccessApproval(c, item) {
			filtered = append(filtered, item)
		}
	}
	c.JSON(http.StatusOK, gin.H{"approvals": filtered})
}

func (h *Handler) GetApproval(c *gin.Context) {
	approvalID := strings.TrimSpace(c.Param("id"))
	if approvalID == "" {
		respondError(c, http.StatusBadRequest, "缺少 approval_id")
		return
	}
	item, ok := h.approvalRuntime().Get(c.Request.Context(), approvalID)
	if !ok {
		respondError(c, http.StatusNotFound, "approval 不存在")
		return
	}
	if !h.canAccessApproval(c, item) {
		respondError(c, http.StatusForbidden, "无权访问该 approval")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) SubmitApprovalDecision(c *gin.Context) {
	approvalID := strings.TrimSpace(c.Param("id"))
	if approvalID == "" {
		respondError(c, http.StatusBadRequest, "缺少 approval_id")
		return
	}
	item, ok := h.approvalRuntime().Get(c.Request.Context(), approvalID)
	if !ok {
		respondError(c, http.StatusNotFound, "approval 不存在")
		return
	}
	if !h.canAccessApproval(c, item) {
		respondError(c, http.StatusForbidden, "无权审批该 approval")
		return
	}
	var req struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	decisionValue := approval.DecisionValue(strings.TrimSpace(req.Decision))
	if decisionValue != approval.DecisionApprove && decisionValue != approval.DecisionReject {
		respondError(c, http.StatusBadRequest, "decision 必须是 approve 或 reject")
		return
	}
	decision := approval.Decision{Decision: decisionValue, DeciderUserID: h.getUserID(c), Reason: req.Reason}
	if err := h.approvalRuntime().Decide(c.Request.Context(), approvalID, decision); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	updated, _ := h.approvalRuntime().Get(c.Request.Context(), approvalID)
	c.JSON(http.StatusOK, gin.H{
		"type":        "approval_decision",
		"approval_id": updated.ID,
		"status":      string(updated.Status),
		"decision":    string(decisionValue),
	})
}
