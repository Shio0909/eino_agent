package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/database/repository"
	"eino_agent/internal/pipeline"
	"eino_agent/internal/security"
	"eino_agent/internal/service"
)

// ChatRequest 聊天请求
type ChatRequest struct {
	Query            string   `json:"query"`
	Message          string   `json:"message"`
	SessionID        string   `json:"session_id"`
	UseAgent         bool     `json:"use_agent,omitempty"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	DocumentIDs      []string `json:"document_ids,omitempty"`
	Mode             string   `json:"mode"`
	TopK             int      `json:"top_k"`
	Temperature      float64  `json:"temperature"`
	ForceCitation    bool     `json:"force_citation,omitempty"`
	EnableLongTerm   *bool    `json:"enable_long_term,omitempty"`
	// EnableSkills / SelectedSkills 已废弃：skills 由 Eino 原生中间件自动管理
}

// GetMessage 兼容 query 和 message 两个字段
func (r *ChatRequest) GetMessage() string {
	if r.Query != "" {
		return r.Query
	}
	return r.Message
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Answer     string              `json:"answer"`
	References []ReferenceDocument `json:"references,omitempty"`
	Sources    []ReferenceDocument `json:"sources,omitempty"`
	Evidence   []EvidenceDocument  `json:"evidence,omitempty"`
	Grounding  GroundingResult     `json:"grounding"`
	SessionID  string              `json:"session_id,omitempty"`
	TraceID    string              `json:"trace_id,omitempty"`
	TokensUsed int                 `json:"tokens_used,omitempty"`
	LatencyMs  int64               `json:"latency_ms"`
	Trace      []service.TraceStep `json:"trace,omitempty"`
}

type messageResponse struct {
	*repository.Message
	AgentSteps any                 `json:"agent_steps,omitempty"`
	Trace      []service.TraceStep `json:"trace,omitempty"`
}

func (h *Handler) toMessageResponses(messages []*repository.Message) []messageResponse {
	out := make([]messageResponse, 0, len(messages))
	for _, msg := range messages {
		item := messageResponse{Message: msg, AgentSteps: msg.AgentSteps}
		if rawTrace, ok := msg.AgentSteps["trace"]; ok {
			if steps, ok := rawTrace.([]any); ok {
				item.Trace = make([]service.TraceStep, 0, len(steps))
				for _, raw := range steps {
					if m, ok := raw.(map[string]any); ok {
						item.Trace = append(item.Trace, traceStepFromMap(m))
					}
				}
				item.AgentSteps = item.Trace
			}
		}
		out = append(out, item)
	}
	return out
}

func traceStepFromMap(m map[string]any) service.TraceStep {
	step := service.TraceStep{
		TraceID:   stringValue(m["trace_id"]),
		Type:      stringValue(m["type"]),
		Stage:     stringValue(m["stage"]),
		Level:     stringValue(m["level"]),
		Summary:   stringValue(m["summary"]),
		Content:   stringValue(m["content"]),
		ToolName:  stringValue(m["tool_name"]),
		ToolInput: stringValue(m["tool_input"]),
		DocID:     stringValue(m["doc_id"]),
		Error:     stringValue(m["error"]),
		Metadata:  mapValue(m["metadata"]),
	}
	step.Seq = intValue(m["seq"])
	step.LatencyMs = int64Value(m["latency_ms"])
	step.TokenCount = intValue(m["token_count"])
	return step
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func int64Value(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}

func intValue(v any) int {
	return int(int64Value(v))
}

func mapValue(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// EvidenceDocument 结构化证据
type EvidenceDocument struct {
	ID       string                 `json:"id"`
	DocID    string                 `json:"doc_id"`
	Source   string                 `json:"source"`
	Snippet  string                 `json:"snippet"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// GroundingResult 证据支撑状态
type GroundingResult struct {
	Status        string `json:"status"`
	EvidenceCount int    `json:"evidence_count"`
	Reason        string `json:"reason,omitempty"`
}

// ReferenceDocument 引用文档
type ReferenceDocument struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Source   string                 `json:"source"`
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Chat 聊天接口
// @Summary 聊天问答
// @Description 发送消息并获取 AI 回答，支持 Pipeline 和 Agentic 两种模式
// @Tags 聊天
// @Accept json
// @Produce json
// @Param request body ChatRequest true "聊天请求"
// @Success 200 {object} ChatResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /chat [post]
func (h *Handler) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	msg := req.GetMessage()
	if msg == "" {
		respondError(c, http.StatusBadRequest, "query 或 message 不能为空")
		return
	}
	decision := h.evaluatePromptRisk(msg)
	if decision.Block {
		log.Printf("[Security][Chat] blocked request: level=%s rules=%v", decision.Level, decision.MatchedRules)
		respondError(c, http.StatusBadRequest, "请求触发安全策略，请调整问题后重试")
		return
	}

	startTime := time.Now()
	if req.SessionID != "" && h.sessionRepo != nil {
		if _, ok := h.ensureSessionAccess(c, req.SessionID); !ok {
			return
		}
	}

	// 调用聊天服务
	var useAgent bool
	req.Mode, useAgent = normalizeChatMode(req.Mode, req.UseAgent)
	if decision.DisableToolCalls {
		log.Printf("[Security][Chat] downgrade to pipeline: level=%s rules=%v", decision.Level, decision.MatchedRules)
		useAgent = false
		req.Mode = "pipeline"
	}
	allowedKBIDs, ok := h.resolveAuthorizedKnowledgeBaseIDs(c, req.KnowledgeBaseIDs)
	if !ok {
		return
	}
	serviceReq := &service.ChatRequest{
		Message:           msg,
		SessionID:         req.SessionID,
		UseAgent:          useAgent,
		Mode:              req.Mode,
		TenantID:          h.getTenantID(c),
		UserID:            h.getUserID(c),
		ForceCitation:     req.ForceCitation || decision.ForceCitation,
		KnowledgeBaseIDs:  allowedKBIDs,
		DocumentIDs:       req.DocumentIDs,
		RestrictRetrieval: true,
		EnableLongTerm:    req.EnableLongTerm,
	}
	resp, err := h.chatService.Chat(c.Request.Context(), serviceReq)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	references := h.toReferences(resp.Sources)
	evidence := h.toEvidence(resp.Sources)
	grounding := groundingFromEvidence(resp, evidence)
	c.JSON(http.StatusOK, ChatResponse{
		Answer:     resp.Answer,
		References: references,
		Sources:    references,
		Evidence:   evidence,
		Grounding:  grounding,
		SessionID:  resp.SessionID,
		TraceID:    resp.TraceID,
		LatencyMs:  time.Since(startTime).Milliseconds(),
		Trace:      resp.Trace,
	})
}

// ChatStream 流式聊天接口
// @Summary 流式聊天 (SSE)
// @Description 发送消息并通过 Server-Sent Events 获取流式 AI 回答
// @Tags 聊天
// @Accept json
// @Produce text/event-stream
// @Param request body ChatRequest true "聊天请求"
// @Success 200 {string} string "SSE 流"
// @Failure 400 {object} map[string]string
// @Router /chat/stream [post]
func (h *Handler) ChatStream(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	msg := req.GetMessage()
	if msg == "" {
		respondError(c, http.StatusBadRequest, "query 或 message 不能为空")
		return
	}
	decision := h.evaluatePromptRisk(msg)
	if decision.Block {
		log.Printf("[Security][ChatStream] blocked request: level=%s rules=%v", decision.Level, decision.MatchedRules)
		respondError(c, http.StatusBadRequest, "请求触发安全策略，请调整问题后重试")
		return
	}

	// 设置 SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	if req.SessionID != "" && h.sessionRepo != nil {
		if _, ok := h.ensureSessionAccess(c, req.SessionID); !ok {
			return
		}
	}

	// 使用流式响应
	var useAgent bool
	req.Mode, useAgent = normalizeChatMode(req.Mode, req.UseAgent)
	if decision.DisableToolCalls {
		log.Printf("[Security][ChatStream] downgrade to pipeline: level=%s rules=%v", decision.Level, decision.MatchedRules)
		useAgent = false
		req.Mode = "pipeline"
	}
	allowedKBIDs, ok := h.resolveAuthorizedKnowledgeBaseIDs(c, req.KnowledgeBaseIDs)
	if !ok {
		return
	}
	serviceReq := &service.ChatRequest{
		Message:           msg,
		SessionID:         req.SessionID,
		UseAgent:          useAgent,
		Mode:              req.Mode,
		TenantID:          h.getTenantID(c),
		UserID:            h.getUserID(c),
		ForceCitation:     req.ForceCitation || decision.ForceCitation,
		KnowledgeBaseIDs:  allowedKBIDs,
		DocumentIDs:       req.DocumentIDs,
		RestrictRetrieval: true,
		EnableLongTerm:    req.EnableLongTerm,
	}
	ch, err := h.chatService.ChatStream(c.Request.Context(), serviceReq)
	if err != nil {
		c.SSEvent("error", gin.H{"type": "error", "error": newErrorResponse(c, http.StatusInternalServerError, err.Error(), nil).Error})
		return
	}

	c.Stream(func(w io.Writer) bool {
		if event, ok := <-ch; ok {
			data := gin.H{"type": event.Type}
			if event.Content != "" {
				data["content"] = event.Content
			}
			if event.SessionID != "" {
				data["session_id"] = event.SessionID
			}
			if event.TraceID != "" {
				data["trace_id"] = event.TraceID
			}
			if event.Error != "" {
				data["error"] = event.Error
			}
			if event.DocID != "" {
				data["doc_id"] = event.DocID
			}
			if event.ToolName != "" {
				data["tool_name"] = event.ToolName
			}
			if event.ToolInput != "" {
				data["tool_input"] = event.ToolInput
			}
			if event.ApprovalID != "" {
				data["approval_id"] = event.ApprovalID
			}
			if event.ApprovalStatus != "" {
				data["approval_status"] = event.ApprovalStatus
			}
			if event.Action != "" {
				data["action"] = event.Action
			}
			if event.RiskLevel != "" {
				data["risk_level"] = event.RiskLevel
			}
			if event.Reason != "" {
				data["reason"] = event.Reason
			}
			if event.ExpiresAt != nil {
				data["expires_at"] = event.ExpiresAt
			}
			if len(event.Metadata) > 0 {
				data["metadata"] = event.Metadata
			}
			if len(event.Sources) > 0 {
				data["sources"] = h.toReferences(event.Sources)
			}
			if event.LatencyMs > 0 {
				data["latency_ms"] = event.LatencyMs
			}
			if event.SourceCount > 0 {
				data["source_count"] = event.SourceCount
			}
			if event.RetryCount > 0 {
				data["retry_count"] = event.RetryCount
			}
			if event.TraceStep != nil {
				data["trace_step"] = event.TraceStep
			}
			if len(event.TraceSnapshot) > 0 {
				data["trace_snapshot"] = event.TraceSnapshot
			}
			c.SSEvent("message", data)
			return true
		}
		c.SSEvent("done", gin.H{})
		return false
	})
}

func (h *Handler) toReferences(sources []service.Source) []ReferenceDocument {
	if len(sources) == 0 {
		return nil
	}
	refs := make([]ReferenceDocument, 0, len(sources))
	for _, src := range sources {
		refs = append(refs, ReferenceDocument{
			ID:       src.DocID,
			Source:   src.DocID,
			Content:  src.Content,
			Metadata: src.Metadata,
		})
	}
	return refs
}

func (h *Handler) toEvidence(sources []service.Source) []EvidenceDocument {
	if len(sources) == 0 {
		return nil
	}
	evidence := make([]EvidenceDocument, 0, len(sources))
	for i, src := range sources {
		evidence = append(evidence, EvidenceDocument{
			ID:       fmt.Sprintf("ev-%d", i+1),
			DocID:    src.DocID,
			Source:   firstEvidenceSource(src.Metadata, src.DocID),
			Snippet:  truncateForEvidence(src.Content, 500),
			Metadata: src.Metadata,
		})
	}
	return evidence
}

func groundingFromEvidence(resp *service.ChatResponse, evidence []EvidenceDocument) GroundingResult {
	if resp != nil && resp.GroundingStatus == pipeline.GroundingStatusInsufficient {
		return GroundingResult{Status: "insufficient_evidence", EvidenceCount: len(evidence), Reason: "retrieval evidence gate rejected the candidates"}
	}
	if len(evidence) == 0 {
		return GroundingResult{Status: "insufficient_evidence", EvidenceCount: 0, Reason: "no evidence returned"}
	}
	return GroundingResult{Status: "supported_by_retrieval", EvidenceCount: len(evidence)}
}

func firstEvidenceSource(metadata map[string]interface{}, fallback string) string {
	for _, key := range []string{"source", "source_filename", "file_name", "wiki_path", "title"} {
		if value, ok := metadata[key]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return fallback
}

func truncateForEvidence(content string, maxRunes int) string {
	content = strings.TrimSpace(content)
	if maxRunes <= 0 {
		return content
	}
	runes := []rune(content)
	if len(runes) <= maxRunes {
		return content
	}
	return string(runes[:maxRunes]) + "..."
}

func (h *Handler) evaluatePromptRisk(input string) security.PromptDecision {
	guardCfg := security.DefaultGuardConfig()
	if h == nil || h.cfg == nil {
		return security.EvaluatePromptRiskWithConfig(input, guardCfg)
	}

	cfg := h.cfg.Security.PromptGuard
	if cfg.Enabled != nil {
		guardCfg.Enabled = *cfg.Enabled
	}
	if cfg.BlockOnHigh != nil {
		guardCfg.BlockOnHigh = *cfg.BlockOnHigh
	}
	if cfg.DowngradeOnMedium != nil {
		guardCfg.DowngradeOnMedium = *cfg.DowngradeOnMedium
	}
	if cfg.ForceCitationOnMedium != nil {
		guardCfg.ForceCitationOnMedium = *cfg.ForceCitationOnMedium
	}
	if len(cfg.HighRiskPatterns) > 0 {
		guardCfg.HighRiskPatterns = cfg.HighRiskPatterns
	}
	if len(cfg.MediumRiskPatterns) > 0 {
		guardCfg.MediumRiskPatterns = cfg.MediumRiskPatterns
	}

	return security.EvaluatePromptRiskWithConfig(input, guardCfg)
}
