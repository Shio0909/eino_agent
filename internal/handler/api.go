// Package handler 提供 Gin HTTP 处理器
package handler

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	einoembedding "github.com/cloudwego/eino/components/embedding"
	"github.com/gin-gonic/gin"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/codegraph"
	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/database/postgres"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/docreader"
	"eino_agent/internal/document"
	"eino_agent/internal/graphrag"
	"eino_agent/internal/importqueue"
	mcpmanager "eino_agent/internal/mcp"
	"eino_agent/internal/rediscache"
	"eino_agent/internal/security"
	"eino_agent/internal/service"
	"eino_agent/internal/wiki"
)

// Handler HTTP 处理器
type Handler struct {
	cfg              *config.Config
	configPath       string
	chatService      *service.ChatService
	embedding        einoembedding.Embedder
	vectorDB         container.VectorDBProvider
	docReaderCli     *docreader.Client
	db               *postgres.DB
	mcpMgr           *mcpmanager.Manager
	importQueue      importqueue.Queue
	redisClient      *rediscache.Client
	sessionCache     cachepkg.SessionCache
	retrievalCache   cachepkg.RetrievalCache
	importStateStore cachepkg.ImportStateStore
	auditLogger      *AuditLogger
	graphRAGService  *graphrag.Service
	codeGraphRepo    codegraph.CodeGraphRepository
	codeIndexer      *codegraph.Indexer
	wikiCompiler     *wiki.Compiler

	// Repositories
	kbRepo        repository.KnowledgeBaseRepository
	knowledgeRepo repository.KnowledgeRepository
	chunkRepo     repository.ChunkRepository
	wikiRepo      repository.WikiPageRepository
	embeddingRepo repository.EmbeddingRepository
	sessionRepo   repository.SessionRepository
	messageRepo   repository.MessageRepository
}

// SetMCPManager 设置 MCP 管理器
func (h *Handler) SetMCPManager(mgr *mcpmanager.Manager) {
	h.mcpMgr = mgr
}

// SetRedisClient 设置 Redis 客户端状态提供者。
func (h *Handler) SetRedisClient(client *rediscache.Client) {
	h.redisClient = client
}

// SetSessionCache 设置会话短期记忆缓存。
func (h *Handler) SetSessionCache(sessionCache cachepkg.SessionCache) {
	h.sessionCache = sessionCache
}

// SetRetrievalCache 设置检索缓存，用于知识库内容变更后的失效处理。
func (h *Handler) SetRetrievalCache(retrievalCache cachepkg.RetrievalCache) {
	h.retrievalCache = retrievalCache
}

// SetImportStateStore 设置异步导入任务状态存储。
func (h *Handler) SetImportStateStore(importStateStore cachepkg.ImportStateStore) {
	h.importStateStore = importStateStore
}

// SetGraphRAGService 设置 GraphRAG 服务。
func (h *Handler) SetGraphRAGService(svc *graphrag.Service) {
	h.graphRAGService = svc
}

// SetCodeGraph 设置代码知识图谱组件。
func (h *Handler) SetCodeGraph(repo codegraph.CodeGraphRepository, indexer *codegraph.Indexer) {
	h.codeGraphRepo = repo
	h.codeIndexer = indexer
}

// SetWikiCompiler 设置 Wiki 编译器（Wiki 模式 KB 上传时使用）
func (h *Handler) SetWikiCompiler(compiler *wiki.Compiler) {
	h.wikiCompiler = compiler
}

// NewHandler 创建新的处理器
func NewHandler(
	cfg *config.Config,
	configPath string,
	chatService *service.ChatService,
	embedding einoembedding.Embedder,
	vectorDB container.VectorDBProvider,
	docReaderCli *docreader.Client,
	db *postgres.DB,
	importQueue importqueue.Queue,
) *Handler {
	if configPath == "" {
		configPath = "configs/config.yaml"
	}

	h := &Handler{
		cfg:              cfg,
		configPath:       configPath,
		chatService:      chatService,
		embedding:        embedding,
		vectorDB:         vectorDB,
		docReaderCli:     docReaderCli,
		db:               db,
		importQueue:      importQueue,
		retrievalCache:   cachepkg.NewNoopRetrievalCache(),
		importStateStore: cachepkg.NewNoopImportStateStore(),
	}

	if logger, err := NewAuditLogger("data/audit/audit.log"); err == nil {
		h.auditLogger = logger
	} else {
		log.Printf("[Audit] init logger failed: %v", err)
	}

	// 初始化 repositories (如果有数据库连接)
	if db != nil {
		h.kbRepo = repository.NewKnowledgeBaseRepository(db)
		h.knowledgeRepo = repository.NewKnowledgeRepository(db)
		h.chunkRepo = repository.NewChunkRepository(db)
		h.wikiRepo = repository.NewWikiPageRepository(db)
		h.embeddingRepo = repository.NewEmbeddingRepository(db)
		h.sessionRepo = repository.NewSessionRepository(db)
		h.messageRepo = repository.NewMessageRepository(db)
		h.chatService.SetRepositories(h.sessionRepo, h.messageRepo)
	}

	return h
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// 健康检查
	r.GET("/health", h.HealthCheck)

	// API v1
	v1 := r.Group("/api/v1")
	{
		// 鉴权
		auth := v1.Group("/auth")
		{
			auth.POST("/login", h.Login)
			auth.GET("/me", h.AuthRequired(), h.Me)
		}

		// 受保护路由（auth 关闭时自动放行）
		protected := v1.Group("")
		protected.Use(h.AuthRequired())

		// 聊天
		chat := protected.Group("/chat")
		{
			chat.POST("", h.Chat)
			chat.POST("/stream", h.ChatStream)
		}

		// 知识库管理
		kb := protected.Group("/knowledge-bases")
		{
			kb.GET("", h.ListKnowledgeBases)
			kb.POST("", h.CreateKnowledgeBase)
			kb.GET("/:id", h.GetKnowledgeBase)
			kb.PUT("/:id", h.UpdateKnowledgeBase)
			kb.DELETE("/:id", h.DeleteKnowledgeBase)

			// 知识文档
			kb.POST("/:id/documents", h.UploadDocument)
			kb.POST("/:id/documents/url", h.UploadDocumentURL)
			kb.GET("/:id/documents", h.ListDocuments)
			kb.GET("/:id/documents/:docId/status", h.GetDocumentImportStatus)
			kb.DELETE("/:id/documents/:docId", h.DeleteDocument)
		}

		// 会话管理
		sessions := protected.Group("/sessions")
		{
			sessions.GET("", h.ListSessions)
			sessions.POST("", h.CreateSession)
			sessions.GET("/:id", h.GetSession)
			sessions.DELETE("/:id", h.DeleteSession)
			sessions.GET("/:id/messages", h.GetSessionMessages)
		}

		// 模型管理
		models := protected.Group("/models")
		{
			models.GET("", h.ListModels)
			models.POST("", h.CreateModel)
			models.DELETE("/:id", h.DeleteModel)
		}

		// 系统设置
		settings := protected.Group("/settings")
		{
			settings.GET("", h.GetSettings)
			settings.PUT("", h.RequireRole("admin"), h.UpdateSettings)
		}

		// 系统信息
		protected.GET("/system/info", h.GetSystemInfo)

		// MCP 管理
		mcp := protected.Group("/mcp")
		{
			mcp.GET("", h.GetMCPStatus)
			mcp.POST("/import", h.RequireRole("admin"), h.ImportMCPServer)
		}

		eval := protected.Group("/eval")
		{
			eval.GET("/reports", h.ListEvalReports)
		}

		// GraphRAG 管理
		graphragAPI := protected.Group("/graphrag")
		{
			graphragAPI.GET("/status", h.GetGraphRAGStatus)
			graphragAPI.POST("/build/:kbId", h.BuildGraphForKB)
			graphragAPI.DELETE("/:kbId", h.DeleteGraphForKB)
		}

		// 代码仓库管理
		codeRepos := protected.Group("/code-repos")
		{
			codeRepos.GET("", h.ListCodeRepos)
			codeRepos.POST("/clone", h.CloneCodeRepo)
			codeRepos.POST("/:name/index", h.IndexCodeRepo)
			codeRepos.POST("/:name/pull", h.PullCodeRepo)
			codeRepos.DELETE("/:name", h.DeleteCodeRepo)
		}
	}

	// 兼容旧 API
	legacy := r.Group("/api")
	legacy.Use(h.AuthRequired())
	legacy.POST("/chat", h.Chat)
	legacy.POST("/chat/stream", h.ChatStream)
}

// HealthCheck 健康检查
// @Summary 健康检查
// @Description 检查服务及各组件状态
// @Tags 系统
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func (h *Handler) HealthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
		"components": gin.H{
			"llm":          h.chatService != nil,
			"embedding":    h.embedding != nil,
			"vectordb":     h.vectorDB != nil,
			"database":     h.db != nil,
			"docreader":    h.docReaderCli != nil,
			"redis":        h.redisClient != nil && h.redisClient.Status(c.Request.Context()).Available,
			"import_queue": h.importQueue != nil,
			"graphrag":     h.graphRAGService != nil,
		},
	}
	if h.redisClient != nil {
		status["redis"] = h.redisClient.Status(c.Request.Context())
	}
	c.JSON(http.StatusOK, status)
}

// ── GraphRAG Endpoints ──

// GetGraphRAGStatus 获取 GraphRAG 服务状态
func (h *Handler) GetGraphRAGStatus(c *gin.Context) {
	if h.graphRAGService == nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false, "message": "GraphRAG 未启用"})
		return
	}
	c.JSON(http.StatusOK, h.graphRAGService.GetStatus())
}

// BuildGraphForKB 为知识库构建图谱（手动触发，接受 chunks）
func (h *Handler) BuildGraphForKB(c *gin.Context) {
	if h.graphRAGService == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "GraphRAG 未启用"})
		return
	}
	kbID := c.Param("kbId")
	if kbID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少知识库 ID"})
		return
	}

	var req struct {
		Chunks []struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"chunks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Chunks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 chunks 数组"})
		return
	}

	chunks := make([]*graphrag.ChunkForGraph, 0, len(req.Chunks))
	for _, ch := range req.Chunks {
		chunks = append(chunks, &graphrag.ChunkForGraph{ID: ch.ID, Content: ch.Content})
	}

	result, err := h.graphRAGService.BuildGraph(c.Request.Context(), &graphrag.BuildGraphRequest{
		Namespace: &graphrag.NameSpace{KnowledgeBase: kbID},
		Chunks:    chunks,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("构建图谱失败: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "图谱构建完成", "result": result})
}

// DeleteGraphForKB 删除知识库对应的图谱
func (h *Handler) DeleteGraphForKB(c *gin.Context) {
	if h.graphRAGService == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "GraphRAG 未启用"})
		return
	}
	kbID := c.Param("kbId")
	if kbID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少知识库 ID"})
		return
	}

	if err := h.graphRAGService.DeleteGraph(c.Request.Context(), &graphrag.NameSpace{KnowledgeBase: kbID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除图谱失败: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "图谱已删除"})
}

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
	SessionID  string              `json:"session_id,omitempty"`
	TokensUsed int                 `json:"tokens_used,omitempty"`
	LatencyMs  int64               `json:"latency_ms"`
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
// @Description 发送消息并获取 AI 回答，支持 Pipeline/Agent/Agentic RAG 三种模式
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg := req.GetMessage()
	if msg == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query 或 message 不能为空"})
		return
	}
	decision := h.evaluatePromptRisk(msg)
	if decision.Block {
		log.Printf("[Security][Chat] blocked request: level=%s rules=%v", decision.Level, decision.MatchedRules)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求触发安全策略，请调整问题后重试"})
		return
	}

	startTime := time.Now()
	if req.SessionID != "" && h.sessionRepo != nil {
		if _, ok := h.ensureSessionAccess(c, req.SessionID); !ok {
			return
		}
	}

	// 调用聊天服务
	// "agent", "agentic", "agentic_rag" 统一走 Agentic 模式
	useAgent := req.UseAgent || strings.EqualFold(req.Mode, "agent") || strings.EqualFold(req.Mode, "agentic") || strings.EqualFold(req.Mode, "agentic_rag")
	if decision.DisableToolCalls {
		log.Printf("[Security][Chat] downgrade to pipeline: level=%s rules=%v", decision.Level, decision.MatchedRules)
		useAgent = false
		if strings.EqualFold(req.Mode, "agent") || strings.EqualFold(req.Mode, "agentic") || strings.EqualFold(req.Mode, "agentic_rag") {
			req.Mode = "pipeline"
		}
	}
	serviceReq := &service.ChatRequest{
		Message:          msg,
		SessionID:        req.SessionID,
		UseAgent:         useAgent,
		Mode:             req.Mode,
		TenantID:         h.getTenantID(c),
		UserID:           h.getUserID(c),
		ForceCitation:    req.ForceCitation || decision.ForceCitation,
		KnowledgeBaseIDs: req.KnowledgeBaseIDs,
		DocumentIDs:      req.DocumentIDs,
		EnableLongTerm:   req.EnableLongTerm,
	}
	resp, err := h.chatService.Chat(c.Request.Context(), serviceReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ChatResponse{
		Answer:     resp.Answer,
		References: h.toReferences(resp.Sources),
		SessionID:  resp.SessionID,
		LatencyMs:  time.Since(startTime).Milliseconds(),
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg := req.GetMessage()
	if msg == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query 或 message 不能为空"})
		return
	}
	decision := h.evaluatePromptRisk(msg)
	if decision.Block {
		log.Printf("[Security][ChatStream] blocked request: level=%s rules=%v", decision.Level, decision.MatchedRules)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求触发安全策略，请调整问题后重试"})
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
	// "agent", "agentic", "agentic_rag" 统一走 Agentic 模式
	useAgent := req.UseAgent || strings.EqualFold(req.Mode, "agent") || strings.EqualFold(req.Mode, "agentic") || strings.EqualFold(req.Mode, "agentic_rag")
	if decision.DisableToolCalls {
		log.Printf("[Security][ChatStream] downgrade to pipeline: level=%s rules=%v", decision.Level, decision.MatchedRules)
		useAgent = false
		if strings.EqualFold(req.Mode, "agent") || strings.EqualFold(req.Mode, "agentic") || strings.EqualFold(req.Mode, "agentic_rag") {
			req.Mode = "pipeline"
		}
	}
	serviceReq := &service.ChatRequest{
		Message:          msg,
		SessionID:        req.SessionID,
		UseAgent:         useAgent,
		Mode:             req.Mode,
		TenantID:         h.getTenantID(c),
		UserID:           h.getUserID(c),
		ForceCitation:    req.ForceCitation || decision.ForceCitation,
		KnowledgeBaseIDs: req.KnowledgeBaseIDs,
		DocumentIDs:      req.DocumentIDs,
		EnableLongTerm:   req.EnableLongTerm,
	}
	ch, err := h.chatService.ChatStream(c.Request.Context(), serviceReq)
	if err != nil {
		c.SSEvent("error", gin.H{"error": err.Error()})
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
			if event.Error != "" {
				data["error"] = event.Error
			}
			if event.DocID != "" {
				data["doc_id"] = event.DocID
			}
			c.SSEvent("message", data)
			return true
		}
		c.SSEvent("done", gin.H{})
		return false
	})
}

func (h *Handler) ensureKnowledgeBaseAccess(c *gin.Context, id string) (*repository.KnowledgeBase, bool) {
	kb, err := h.kbRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return nil, false
	}
	if kb == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "知识库不存在"})
		return nil, false
	}
	if kb.TenantID != h.getTenantID(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该知识库"})
		return nil, false
	}
	return kb, true
}

func (h *Handler) ensureSessionAccess(c *gin.Context, id string) (*repository.Session, bool) {
	session, err := h.sessionRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return nil, false
	}
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return nil, false
	}
	if session.TenantID != h.getTenantID(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该会话"})
		return nil, false
	}
	if h.getUserRole(c) != "admin" && session.UserID != h.getUserID(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该会话"})
		return nil, false
	}
	return session, true
}

func (h *Handler) toReferences(sources []service.Source) []ReferenceDocument {
	if len(sources) == 0 {
		return nil
	}
	refs := make([]ReferenceDocument, 0, len(sources))
	for _, src := range sources {
		refs = append(refs, ReferenceDocument{
			ID:      src.DocID,
			Source:  src.DocID,
			Content: src.Content,
		})
	}
	return refs
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

// KnowledgeBaseRequest 知识库请求
type KnowledgeBaseRequest struct {
	Name                string         `json:"name" binding:"required"`
	Description         string         `json:"description"`
	Mode                string         `json:"mode"` // "vector"(默认) 或 "wiki"(LLM编译Wiki)
	EmbeddingModelID    string         `json:"embedding_model_id"`
	EmbeddingDimensions int            `json:"embedding_dimensions"`
	ChunkingConfig      map[string]any `json:"chunking_config"`
	ExtractConfig       map[string]any `json:"extract_config"`
}

// ListKnowledgeBases 获取知识库列表
// @Summary 知识库列表
// @Description 获取当前租户下所有知识库
// @Tags 知识库
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /knowledge-bases [get]
func (h *Handler) ListKnowledgeBases(c *gin.Context) {
	if h.kbRepo == nil {
		c.JSON(http.StatusOK, gin.H{"knowledge_bases": []any{}, "message": "数据库未连接"})
		return
	}

	tenantID := h.getTenantID(c)

	kbs, err := h.kbRepo.List(c.Request.Context(), tenantID, 0, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"knowledge_bases": kbs})
}

// CreateKnowledgeBase 创建知识库
// @Summary 创建知识库
// @Description 创建新的知识库
// @Tags 知识库
// @Accept json
// @Produce json
// @Param request body KnowledgeBaseRequest true "知识库信息"
// @Success 201 {object} repository.KnowledgeBase
// @Failure 400 {object} map[string]string
// @Router /knowledge-bases [post]
func (h *Handler) CreateKnowledgeBase(c *gin.Context) {
	if h.kbRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "数据库未连接"})
		return
	}

	var req KnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 处理 embedding_model_id: 空字符串转为 nil (NULL) 避免 FK 约束错误
	var embeddingModelID *string
	if req.EmbeddingModelID != "" {
		embeddingModelID = &req.EmbeddingModelID
	}

	// 默认使用配置中的 embedding 维度
	dimensions := req.EmbeddingDimensions
	if dimensions == 0 {
		dimensions = h.cfg.Embedding.Dimensions
	}

	// 确定 KB 模式
	mode := req.Mode
	if mode == "" {
		mode = "vector"
	}
	if mode != "vector" && mode != "wiki" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode 只能是 'vector' 或 'wiki'"})
		return
	}

	kb := &repository.KnowledgeBase{
		TenantID:            h.getTenantID(c),
		Name:                req.Name,
		Description:         req.Description,
		Mode:                mode,
		EmbeddingModelID:    embeddingModelID,
		EmbeddingDimensions: dimensions,
		ChunkingConfig:      repository.JSON(req.ChunkingConfig),
		ExtractConfig:       repository.JSON(req.ExtractConfig),
	}

	if err := h.kbRepo.Create(c.Request.Context(), kb); err != nil {
		h.audit(c, "kb.create", req.Name, false, map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.audit(c, "kb.create", kb.ID, true, map[string]interface{}{"name": kb.Name})

	c.JSON(http.StatusCreated, kb)
}

// GetKnowledgeBase 获取知识库详情
// @Summary 知识库详情
// @Description 获取指定知识库的详细信息
// @Tags 知识库
// @Produce json
// @Param id path string true "知识库 ID"
// @Success 200 {object} repository.KnowledgeBase
// @Failure 404 {object} map[string]string
// @Router /knowledge-bases/{id} [get]
func (h *Handler) GetKnowledgeBase(c *gin.Context) {
	if h.kbRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "数据库未连接"})
		return
	}

	id := c.Param("id")
	kb, ok := h.ensureKnowledgeBaseAccess(c, id)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, kb)
}

// UpdateKnowledgeBase 更新知识库
// @Summary 更新知识库
// @Description 更新知识库信息
// @Tags 知识库
// @Accept json
// @Produce json
// @Param id path string true "知识库 ID"
// @Param request body KnowledgeBaseRequest true "更新信息"
// @Success 200 {object} map[string]string
// @Router /knowledge-bases/{id} [put]
func (h *Handler) UpdateKnowledgeBase(c *gin.Context) {
	if h.kbRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "数据库未连接"})
		return
	}

	id := c.Param("id")
	if _, ok := h.ensureKnowledgeBaseAccess(c, id); !ok {
		return
	}
	var req KnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	kb := &repository.KnowledgeBase{
		ID:             id,
		Name:           req.Name,
		Description:    req.Description,
		ChunkingConfig: repository.JSON(req.ChunkingConfig),
		ExtractConfig:  repository.JSON(req.ExtractConfig),
	}

	if err := h.kbRepo.Update(c.Request.Context(), kb); err != nil {
		h.audit(c, "kb.update", id, false, map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.audit(c, "kb.update", id, true, map[string]interface{}{"name": req.Name})

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteKnowledgeBase 删除知识库
// @Summary 删除知识库
// @Description 删除指定知识库及其所有文档
// @Tags 知识库
// @Param id path string true "知识库 ID"
// @Success 200 {object} map[string]string
// @Router /knowledge-bases/{id} [delete]
func (h *Handler) DeleteKnowledgeBase(c *gin.Context) {
	if h.kbRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "数据库未连接"})
		return
	}

	id := c.Param("id")
	if _, ok := h.ensureKnowledgeBaseAccess(c, id); !ok {
		return
	}
	h.clearKnowledgeBaseImportStates(c.Request.Context(), id)
	if err := h.kbRepo.Delete(c.Request.Context(), id); err != nil {
		h.audit(c, "kb.delete", id, false, map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.retrievalCache != nil {
		if err := h.retrievalCache.InvalidateKnowledgeBase(c.Request.Context(), id); err != nil {
			log.Printf("[Cache] 知识库删除后失效检索缓存失败: kb=%s err=%v", id, err)
		}
	}
	h.audit(c, "kb.delete", id, true, nil)

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// UploadDocument 上传文档
// @Summary 上传文档
// @Description 上传文档到指定知识库，自动解析、分块、向量化
// @Tags 知识库
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "知识库 ID"
// @Param file formData file true "文档文件"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /knowledge-bases/{id}/documents [post]
func (h *Handler) UploadDocument(c *gin.Context) {
	kbID := c.Param("id")
	var kb *repository.KnowledgeBase
	if h.kbRepo != nil {
		var ok bool
		kb, ok = h.ensureKnowledgeBaseAccess(c, kbID)
		if !ok {
			return
		}
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": "请上传文件"})
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传文件"})
		return
	}
	defer file.Close()

	// Wiki 模式: LLM 编译为结构化 wiki 页面
	if kb != nil && kb.IsWikiMode() {
		h.uploadWikiDocument(c, kb, header, file)
		return
	}

	if h.importQueue != nil {
		knowledge, err := h.enqueueFileImport(c.Request.Context(), kbID, header.Filename, header.Size, file)
		if err != nil {
			h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error(), "filename": header.Filename})
			c.JSON(http.StatusInternalServerError, gin.H{"error": "加入异步导入队列失败: " + err.Error()})
			return
		}

		h.audit(c, "doc.upload", kbID, true, map[string]interface{}{"filename": header.Filename, "knowledge_id": knowledge.ID, "async": true})
		c.JSON(http.StatusAccepted, gin.H{
			"message":      "文档已加入异步导入队列",
			"knowledge_id": knowledge.ID,
			"status":       knowledge.ParseStatus,
		})
		return
	}

	knowledge, err := h.createKnowledgeRecord(c.Request.Context(), h.newFileKnowledge(kbID, header.Filename, header.Size, "processing"))
	if err != nil {
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error(), "filename": header.Filename})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文档记录失败: " + err.Error()})
		return
	}

	// 检查 docreader 是否可用
	if h.docReaderCli != nil {
		// 从文件名推导文件类型
		fileType := ""
		if dotIdx := strings.LastIndex(header.Filename, "."); dotIdx >= 0 {
			fileType = strings.ToLower(header.Filename[dotIdx+1:])
		}
		// 使用 docreader 解析文件
		result, err := h.docReaderCli.ParseReader(
			c.Request.Context(),
			file,
			header.Filename,
			fileType,
			docreader.DefaultParseOptions(),
		)
		if err != nil {
			h.markKnowledgeFailed(c.Request.Context(), knowledge.ID, 0, err)
			h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error(), "filename": header.Filename})
			c.JSON(http.StatusInternalServerError, gin.H{"error": "文档解析失败: " + err.Error()})
			return
		}

		// 向量化并存储
		chunkCount, err := h.storeParsedChunks(c.Request.Context(), kbID, knowledge.ID, header.Filename, result.Chunks)
		if err != nil {
			log.Printf("[Upload] 向量化失败（文档已解析 %d 块）: %v", chunkCount, err)
			h.markKnowledgeFailed(c.Request.Context(), knowledge.ID, chunkCount, err)
			h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error(), "filename": header.Filename})
			c.JSON(http.StatusOK, gin.H{
				"message":      "文档已解析，但向量化失败（请在设置中配置有效的 API Key）",
				"chunk_count":  chunkCount,
				"status":       "failed",
				"knowledge_id": knowledge.ID,
				"error":        err.Error(),
			})
			return
		}

		h.markKnowledgeCompleted(c.Request.Context(), knowledge, chunkCount)
		h.audit(c, "doc.upload", kbID, true, map[string]interface{}{"filename": header.Filename, "chunk_count": chunkCount})

		c.JSON(http.StatusOK, gin.H{
			"message":      "文档上传成功",
			"chunk_count":  chunkCount,
			"knowledge_id": knowledge.ID,
		})
		return
	}

	// 回退到本地处理: 读取文件内容
	content, err := io.ReadAll(file)
	if err != nil {
		h.markKnowledgeFailed(c.Request.Context(), knowledge.ID, 0, err)
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败: " + err.Error()})
		return
	}

	chunkCount, err := h.processPlainTextDocument(c.Request.Context(), kbID, knowledge.ID, header.Filename, content)
	if err != nil {
		h.markKnowledgeFailed(c.Request.Context(), knowledge.ID, chunkCount, err)
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文档处理失败: " + err.Error()})
		return
	}

	h.markKnowledgeCompleted(c.Request.Context(), knowledge, chunkCount)
	h.audit(c, "doc.upload", kbID, true, map[string]interface{}{"filename": header.Filename, "chunk_count": chunkCount})

	c.JSON(http.StatusOK, gin.H{
		"message":      "文档上传成功",
		"chunk_count":  chunkCount,
		"knowledge_id": knowledge.ID,
	})
}

// UploadDocumentURL 从网页 URL 导入文档
// @Summary 上传网页 URL
// @Description 从指定 URL 抓取内容到知识库，自动解析、分块、向量化
// @Tags 知识库
// @Accept json
// @Produce json
// @Param id path string true "知识库 ID"
// @Param request body object{url=string,title=string,enable_multimodal=bool} true "URL 导入请求"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /knowledge-bases/{id}/documents/url [post]
func (h *Handler) UploadDocumentURL(c *gin.Context) {
	kbID := c.Param("id")
	if h.kbRepo != nil {
		if _, ok := h.ensureKnowledgeBaseAccess(c, kbID); !ok {
			return
		}
	}

	var req struct {
		URL              string `json:"url" binding:"required"`
		Title            string `json:"title"`
		EnableMultimodal bool   `json:"enable_multimodal"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parsedURL, err := security.ValidateExternalURL(strings.TrimSpace(req.URL), h.cfg.Security.URLPolicy)
	if err != nil {
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": req.URL})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = parsedURL.String()
	}

	if h.importQueue != nil {
		knowledge, err := h.enqueueURLImport(c.Request.Context(), kbID, title, parsedURL.String(), req.EnableMultimodal)
		if err != nil {
			h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": parsedURL.String()})
			c.JSON(http.StatusInternalServerError, gin.H{"error": "加入异步导入队列失败: " + err.Error()})
			return
		}

		h.audit(c, "doc.upload_url", kbID, true, map[string]interface{}{"url": parsedURL.String(), "knowledge_id": knowledge.ID, "async": true})
		c.JSON(http.StatusAccepted, gin.H{
			"message":      "网页已加入异步导入队列",
			"knowledge_id": knowledge.ID,
			"status":       knowledge.ParseStatus,
		})
		return
	}

	parseOpts := docreader.DefaultParseOptions()
	parseOpts.EnableMultimodal = req.EnableMultimodal

	knowledge, err := h.createKnowledgeRecord(c.Request.Context(), h.newURLKnowledge(kbID, title, parsedURL.String(), "processing"))
	if err != nil {
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": parsedURL.String()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文档记录失败: " + err.Error()})
		return
	}

	result, err := h.docReaderCli.ParseURL(c.Request.Context(), parsedURL.String(), title, parseOpts)
	if err != nil {
		h.markKnowledgeFailed(c.Request.Context(), knowledge.ID, 0, err)
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": parsedURL.String()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "网页解析失败: " + err.Error()})
		return
	}

	chunkCount, err := h.storeParsedChunks(c.Request.Context(), kbID, knowledge.ID, title, result.Chunks)
	if err != nil {
		log.Printf("[UploadURL] 向量化失败（URL 已解析 %d 块）: %v", chunkCount, err)
		h.markKnowledgeFailed(c.Request.Context(), knowledge.ID, chunkCount, err)
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": parsedURL.String()})
		c.JSON(http.StatusOK, gin.H{
			"message":      "网页已解析，但向量化失败（请在设置中配置有效的 API Key）",
			"chunk_count":  chunkCount,
			"status":       "failed",
			"knowledge_id": knowledge.ID,
			"error":        err.Error(),
		})
		return
	}

	h.markKnowledgeCompleted(c.Request.Context(), knowledge, chunkCount)
	h.audit(c, "doc.upload_url", kbID, true, map[string]interface{}{"url": parsedURL.String(), "chunk_count": chunkCount})

	c.JSON(http.StatusOK, gin.H{
		"message":      "网页上传成功",
		"chunk_count":  chunkCount,
		"knowledge_id": knowledge.ID,
	})
}

func (h *Handler) newFileKnowledge(kbID, filename string, fileSize int64, status string) *repository.Knowledge {
	return &repository.Knowledge{
		KnowledgeBaseID: kbID,
		Name:            filename,
		SourceType:      "file",
		FileName:        filename,
		FileType:        inferFileType(filename),
		FileSize:        fileSize,
		ParseStatus:     status,
		ChunkCount:      0,
	}
}

func (h *Handler) newURLKnowledge(kbID, title, sourceURL, status string) *repository.Knowledge {
	if title == "" {
		title = sourceURL
	}
	filePath := sourceURL
	return &repository.Knowledge{
		KnowledgeBaseID: kbID,
		Name:            title,
		SourceType:      "url",
		FileName:        title,
		FileType:        "url",
		FilePath:        &filePath,
		ParseStatus:     status,
		ChunkCount:      0,
		Metadata: repository.JSON{
			"url": sourceURL,
		},
	}
}

func (h *Handler) createKnowledgeRecord(ctx context.Context, knowledge *repository.Knowledge) (*repository.Knowledge, error) {
	if h.knowledgeRepo == nil {
		return knowledge, nil
	}
	if err := h.knowledgeRepo.Create(ctx, knowledge); err != nil {
		return nil, err
	}
	if h.kbRepo != nil {
		if err := h.kbRepo.IncrementCounts(ctx, knowledge.KnowledgeBaseID, 1, 0); err != nil {
			return nil, err
		}
	}
	return knowledge, nil
}

func (h *Handler) markKnowledgeCompleted(ctx context.Context, knowledge *repository.Knowledge, chunkCount int) {
	if knowledge == nil || h.knowledgeRepo == nil {
		return
	}
	_ = h.knowledgeRepo.UpdateParseStatus(ctx, knowledge.ID, "completed", "", chunkCount)
	h.writeImportTaskState(ctx, knowledge.ID, func(state *cachepkg.ImportTaskState) {
		state.Status = "completed"
		state.Stage = "completed"
		state.ChunkCount = chunkCount
		state.Error = ""
	})
	if h.kbRepo != nil {
		_ = h.kbRepo.IncrementCounts(ctx, knowledge.KnowledgeBaseID, 0, chunkCount)
	}
	if h.retrievalCache != nil {
		if err := h.retrievalCache.InvalidateKnowledgeBase(ctx, knowledge.KnowledgeBaseID); err != nil {
			log.Printf("[Cache] 文档完成导入后失效检索缓存失败: kb=%s knowledge=%s err=%v", knowledge.KnowledgeBaseID, knowledge.ID, err)
		}
	}
}

func (h *Handler) markKnowledgeFailed(ctx context.Context, knowledgeID string, chunkCount int, err error) {
	if knowledgeID == "" || h.knowledgeRepo == nil || err == nil {
		return
	}
	_ = h.knowledgeRepo.UpdateParseStatus(ctx, knowledgeID, "failed", err.Error(), chunkCount)
	h.writeImportTaskState(ctx, knowledgeID, func(state *cachepkg.ImportTaskState) {
		state.Status = "failed"
		state.Stage = "failed"
		state.ChunkCount = chunkCount
		state.Error = err.Error()
	})
}

func (h *Handler) enqueueFileImport(ctx context.Context, kbID, filename string, fileSize int64, reader io.Reader) (*repository.Knowledge, error) {
	knowledge, err := h.createKnowledgeRecord(ctx, h.newFileKnowledge(kbID, filename, fileSize, "pending"))
	if err != nil {
		return nil, err
	}
	h.writeImportTaskState(ctx, knowledge.ID, func(state *cachepkg.ImportTaskState) {
		state.Status = "pending"
		state.Stage = "queued"
		state.ChunkCount = 0
		state.Error = ""
	})

	tempPath, err := h.persistUploadedFile(filename, reader)
	if err != nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		return nil, err
	}

	task := importqueue.Task{
		KnowledgeID:     knowledge.ID,
		KnowledgeBaseID: kbID,
		SourceType:      "file",
		FilePath:        tempPath,
		FileName:        filename,
		FileType:        inferFileType(filename),
	}
	if err := h.importQueue.Enqueue(ctx, task); err != nil {
		_ = os.Remove(tempPath)
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		return nil, err
	}

	return knowledge, nil
}

func (h *Handler) enqueueURLImport(ctx context.Context, kbID, title, sourceURL string, enableMultimodal bool) (*repository.Knowledge, error) {
	knowledge, err := h.createKnowledgeRecord(ctx, h.newURLKnowledge(kbID, title, sourceURL, "pending"))
	if err != nil {
		return nil, err
	}
	h.writeImportTaskState(ctx, knowledge.ID, func(state *cachepkg.ImportTaskState) {
		state.Status = "pending"
		state.Stage = "queued"
		state.ChunkCount = 0
		state.Error = ""
	})

	task := importqueue.Task{
		KnowledgeID:      knowledge.ID,
		KnowledgeBaseID:  kbID,
		SourceType:       "url",
		Title:            title,
		SourceURL:        sourceURL,
		EnableMultimodal: enableMultimodal,
	}
	if err := h.importQueue.Enqueue(ctx, task); err != nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		return nil, err
	}

	return knowledge, nil
}

func (h *Handler) persistUploadedFile(filename string, reader io.Reader) (string, error) {
	tempDir := h.cfg.ImportQueue.TempDir
	if tempDir == "" {
		if h.cfg.RAG.DocumentsPath != "" {
			tempDir = filepath.Join(h.cfg.RAG.DocumentsPath, "import-jobs")
		} else {
			tempDir = filepath.Join(os.TempDir(), "eino-agent-import-jobs")
		}
	}
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("create import temp dir: %w", err)
	}

	safeName := sanitizeFilename(filename)
	if safeName == "" {
		safeName = "upload.bin"
	}
	tempPath := filepath.Join(tempDir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), safeName))

	tempFile, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, reader); err != nil {
		return "", fmt.Errorf("write temp file: %w", err)
	}

	return tempPath, nil
}

func sanitizeFilename(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(base)
}

func inferFileType(filename string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if ext == "" {
		return "unknown"
	}
	return ext
}

// uploadWikiDocument 处理 Wiki 模式的文档上传
// LLM 将原始文档编译为结构化 wiki 页面，存入 wiki_pages 表
func (h *Handler) uploadWikiDocument(c *gin.Context, kb *repository.KnowledgeBase, header *multipart.FileHeader, file multipart.File) {
	kbID := kb.ID
	ctx := c.Request.Context()

	knowledge, err := h.createKnowledgeRecord(ctx, h.newFileKnowledge(kbID, header.Filename, header.Size, "processing"))
	if err != nil {
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error(), "filename": header.Filename})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文档记录失败: " + err.Error()})
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败: " + err.Error()})
		return
	}

	if h.wikiCompiler == nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, fmt.Errorf("wiki compiler 未初始化"))
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Wiki 编译器未初始化（需要 LLM 配置）"})
		return
	}

	result, err := h.wikiCompiler.Compile(ctx, kbID, knowledge.ID, header.Filename, string(content))
	if err != nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error(), "filename": header.Filename})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Wiki 编译失败: " + err.Error()})
		return
	}

	pageCount := len(result.Pages)
	h.markKnowledgeCompleted(ctx, knowledge, pageCount)
	h.audit(c, "doc.upload", kbID, true, map[string]interface{}{
		"filename":   header.Filename,
		"page_count": pageCount,
		"link_count": result.LinkCount,
		"mode":       "wiki",
	})

	c.JSON(http.StatusOK, gin.H{
		"message":      "文档上传成功（Wiki模式，LLM已编译为结构化知识页面）",
		"page_count":   pageCount,
		"link_count":   result.LinkCount,
		"knowledge_id": knowledge.ID,
		"mode":         "wiki",
	})
}

func (h *Handler) processPlainTextDocument(ctx context.Context, kbID, knowledgeID, filename string, content []byte) (int, error) {
	rawDoc := &document.RawDocument{
		ID:      knowledgeID,
		Source:  filename,
		Content: string(content),
		Metadata: map[string]interface{}{
			"filename":     filename,
			"size":         len(content),
			"knowledge_id": knowledgeID,
		},
	}

	// 构建分块器选项（语义分块需要 embedder）
	var chunkerOpts []document.ChunkerOption
	if (h.cfg.RAG.ChunkStrategy == "semantic" || h.cfg.RAG.ChunkStrategy == "auto") && h.embedding != nil {
		chunkerOpts = append(chunkerOpts, document.WithEmbedder(h.embedding))
		if h.cfg.RAG.SemanticSimilarityPct > 0 {
			chunkerOpts = append(chunkerOpts, document.WithSimilarityPct(h.cfg.RAG.SemanticSimilarityPct))
		}
	}

	chunker := document.NewChunker(
		h.cfg.RAG.ChunkStrategy,
		h.cfg.RAG.ChunkSize,
		h.cfg.RAG.ChunkOverlap,
		filename,
		chunkerOpts...,
	)
	chunks, err := chunker.Chunk(ctx, rawDoc)
	if err != nil {
		return 0, fmt.Errorf("文档分块失败: %w", err)
	}

	// 上下文富化（可选）
	if h.cfg.RAG.EnableContextualEnrichment && h.cfg.Agent.AgenticRAG.LightLLM != nil && h.cfg.Agent.AgenticRAG.LightLLM.BaseURL != "" {
		lightLLM, _, llmErr := container.NewLLMProvider(ctx, h.cfg.Agent.AgenticRAG.LightLLM)
		if llmErr != nil {
			log.Printf("[Enricher] 创建 LLM 失败，跳过富化: %v", llmErr)
		} else {
			enricher := document.NewContextualEnricher(lightLLM)
			enrichedChunks, enrichErr := enricher.Enrich(ctx, string(content), chunks)
			if enrichErr != nil {
				log.Printf("[Enricher] 富化失败，使用原始 chunks: %v", enrichErr)
			} else {
				chunks = enrichedChunks
				log.Printf("[Enricher] 成功富化 %d 个 chunks", len(chunks))
			}
		}
	}

	contents := make([]string, len(chunks))
	for i, chunk := range chunks {
		contents[i] = chunk.Content
	}

	vectors, err := container.BatchEmbedFloat32(ctx, h.embedding, contents)
	if err != nil {
		return len(chunks), fmt.Errorf("向量化失败: %w", err)
	}

	for i, chunk := range chunks {
		chunk.Vector = vectors[i]
		chunk.Metadata["knowledge_base_id"] = kbID
		chunk.Metadata["knowledge_id"] = knowledgeID
		chunk.Metadata["source_filename"] = filename
		chunk.Metadata["uploaded_at"] = time.Now().Format(time.RFC3339)
	}

	if err := h.vectorDB.Upsert(ctx, chunks); err != nil {
		return len(chunks), fmt.Errorf("存储失败: %w", err)
	}

	return len(chunks), nil
}

func (h *Handler) storeParsedChunks(ctx context.Context, kbID, knowledgeID, sourceFilename string, chunks []docreader.ParsedChunk) (int, error) {
	if err := h.processAndStoreChunks(ctx, kbID, knowledgeID, sourceFilename, chunks); err != nil {
		return len(chunks), err
	}
	return len(chunks), nil
}

// ProcessImportTask 处理异步导入任务。
func (h *Handler) ProcessImportTask(ctx context.Context, task importqueue.Task) error {
	if h.knowledgeRepo == nil {
		return fmt.Errorf("knowledge repository unavailable")
	}

	start := time.Now()
	log.Printf("[ImportWorker] 开始处理: %s (type=%s, id=%s)", task.FileName, task.SourceType, task.KnowledgeID)

	if err := h.knowledgeRepo.UpdateParseStatus(ctx, task.KnowledgeID, "processing", "", 0); err != nil {
		return err
	}
	h.writeImportTaskState(ctx, task.KnowledgeID, func(state *cachepkg.ImportTaskState) {
		state.Status = "processing"
		state.Stage = "parsing"
		state.Error = ""
	})

	var (
		chunkCount int
		err        error
	)

	switch task.SourceType {
	case "url":
		chunkCount, err = h.processQueuedURLImport(ctx, task)
	case "file":
		chunkCount, err = h.processQueuedFileImport(ctx, task)
	default:
		err = fmt.Errorf("unsupported import source type: %s", task.SourceType)
	}

	if task.SourceType == "file" && task.FilePath != "" {
		_ = os.Remove(task.FilePath)
	}

	elapsed := time.Since(start)

	if err != nil {
		log.Printf("[ImportWorker] 失败: %s (%v) [%v]", task.FileName, err, elapsed)
		h.markKnowledgeFailed(ctx, task.KnowledgeID, chunkCount, err)
		return nil
	}

	log.Printf("[ImportWorker] 完成: %s → %d chunks [%v]", task.FileName, chunkCount, elapsed)
	h.markKnowledgeCompleted(ctx, &repository.Knowledge{ID: task.KnowledgeID, KnowledgeBaseID: task.KnowledgeBaseID}, chunkCount)
	return nil
}

func (h *Handler) processQueuedFileImport(ctx context.Context, task importqueue.Task) (int, error) {
	if h.docReaderCli != nil {
		file, err := os.Open(task.FilePath)
		if err != nil {
			return 0, fmt.Errorf("open temp file: %w", err)
		}
		defer file.Close()

		result, err := h.docReaderCli.ParseReader(ctx, file, task.FileName, task.FileType, docreader.DefaultParseOptions())
		if err != nil {
			return 0, fmt.Errorf("文档解析失败: %w", err)
		}
		return h.storeParsedChunks(ctx, task.KnowledgeBaseID, task.KnowledgeID, task.FileName, result.Chunks)
	}

	content, err := os.ReadFile(task.FilePath)
	if err != nil {
		return 0, fmt.Errorf("read temp file: %w", err)
	}
	return h.processPlainTextDocument(ctx, task.KnowledgeBaseID, task.KnowledgeID, task.FileName, content)
}

func (h *Handler) processQueuedURLImport(ctx context.Context, task importqueue.Task) (int, error) {
	if h.docReaderCli != nil {
		parseOpts := docreader.DefaultParseOptions()
		parseOpts.EnableMultimodal = task.EnableMultimodal

		result, err := h.docReaderCli.ParseURL(ctx, task.SourceURL, task.Title, parseOpts)
		if err != nil {
			return 0, fmt.Errorf("网页解析失败: %w", err)
		}
		return h.storeParsedChunks(ctx, task.KnowledgeBaseID, task.KnowledgeID, task.Title, result.Chunks)
	}

	// docreader 不可用时，直接 HTTP 抓取并按纯文本处理
	log.Printf("[ImportWorker] docreader 不可用，使用 HTTP 直接抓取: %s", task.SourceURL)
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, task.SourceURL, nil)
	if err != nil {
		return 0, fmt.Errorf("构建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; EinoAgent/1.0)")
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP 抓取失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP 抓取返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return 0, fmt.Errorf("读取响应失败: %w", err)
	}
	return h.processPlainTextDocument(ctx, task.KnowledgeBaseID, task.KnowledgeID, task.FileName, body)
}

// processAndStoreChunks 处理并存储文档块
func (h *Handler) processAndStoreChunks(ctx context.Context, kbID, knowledgeID, sourceFilename string, chunks []docreader.ParsedChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	h.writeImportTaskState(ctx, knowledgeID, func(state *cachepkg.ImportTaskState) {
		state.Status = "processing"
		state.Stage = "vectorizing"
		state.ChunkCount = len(chunks)
		state.Error = ""
	})

	// 转换为内部格式
	docs := make([]*container.Document, len(chunks))
	contents := make([]string, len(chunks))
	batchPrefix := time.Now().UnixNano()

	for i, chunk := range chunks {
		chunkID := fmt.Sprintf("%s_chunk_%d_%d_%d", kbID, batchPrefix, chunk.Seq, i)
		docs[i] = &container.Document{
			ID:      chunkID,
			Content: chunk.Content,
			Metadata: map[string]interface{}{
				"knowledge_base_id": kbID,
				"knowledge_id":      knowledgeID,
				"chunk_id":          chunkID,
				"chunk_index":       chunk.Seq,
				"start_pos":         chunk.Start,
				"end_pos":           chunk.End,
				"source_filename":   sourceFilename,
				"uploaded_at":       time.Now().Format(time.RFC3339),
			},
		}
		contents[i] = chunk.Content
	}

	// 批量向量化
	const maxEmbedBatchSize = 64
	vectors := make([][]float32, len(contents))
	for start := 0; start < len(contents); start += maxEmbedBatchSize {
		end := start + maxEmbedBatchSize
		if end > len(contents) {
			end = len(contents)
		}

		batchVectors, err := container.BatchEmbedFloat32(ctx, h.embedding, contents[start:end])
		if err != nil {
			return err
		}
		copy(vectors[start:end], batchVectors)
	}

	for i, doc := range docs {
		doc.Vector = vectors[i]
	}

	// 存储
	if err := h.vectorDB.Upsert(ctx, docs); err != nil {
		return err
	}

	// 异步构建 GraphRAG 图谱
	if h.graphRAGService != nil {
		graphChunks := make([]*graphrag.ChunkForGraph, len(docs))
		for i, d := range docs {
			graphChunks[i] = &graphrag.ChunkForGraph{ID: d.ID, Content: d.Content}
		}
		go func() {
			bgCtx := context.Background()
			if _, err := h.graphRAGService.BuildGraph(bgCtx, &graphrag.BuildGraphRequest{
				Namespace: &graphrag.NameSpace{KnowledgeBase: kbID},
				Chunks:    graphChunks,
			}); err != nil {
				log.Printf("[GraphRAG] 异步构建图谱失败 kbID=%s: %v", kbID, err)
			}
		}()
	}

	return nil
}

// ListDocuments 获取知识库文档列表
// @Summary 文档列表
// @Description 获取知识库下的文档列表
// @Tags 知识库
// @Produce json
// @Param id path string true "知识库 ID"
// @Success 200 {object} map[string]interface{}
// @Router /knowledge-bases/{id}/documents [get]
func (h *Handler) ListDocuments(c *gin.Context) {
	kbID := c.Param("id")
	if h.kbRepo != nil {
		if _, ok := h.ensureKnowledgeBaseAccess(c, kbID); !ok {
			return
		}
	}

	if h.knowledgeRepo == nil {
		c.JSON(http.StatusOK, gin.H{"documents": []any{}, "message": "数据库未连接"})
		return
	}

	knowledges, err := h.knowledgeRepo.ListByKnowledgeBase(c.Request.Context(), kbID, 0, 1000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	docs := make([]gin.H, 0, len(knowledges))
	for _, k := range knowledges {
		stage := ""
		updatedAt := k.UpdatedAt
		if state, ok := h.getImportTaskState(c.Request.Context(), k.ID); ok {
			if state.Status != "" {
				k.ParseStatus = state.Status
			}
			if state.Error != "" {
				parseError := state.Error
				k.ParseError = &parseError
			}
			if state.ChunkCount > 0 {
				k.ChunkCount = state.ChunkCount
			}
			stage = state.Stage
			if !state.UpdatedAt.IsZero() {
				updatedAt = state.UpdatedAt
			}
		}
		docs = append(docs, gin.H{
			"id":           k.ID,
			"filename":     k.FileName,
			"name":         k.Name,
			"source_type":  k.SourceType,
			"file_size":    k.FileSize,
			"file_type":    k.FileType,
			"file_path":    k.FilePath,
			"metadata":     k.Metadata,
			"parse_status": k.ParseStatus,
			"parse_error":  k.ParseError,
			"chunk_count":  k.ChunkCount,
			"created_at":   k.CreatedAt,
			"updated_at":   updatedAt,
			"stage":        stage,
		})
	}

	c.JSON(http.StatusOK, gin.H{"documents": docs})
}

// GetDocumentImportStatus 获取单个导入任务的实时状态。
func (h *Handler) GetDocumentImportStatus(c *gin.Context) {
	kbID := c.Param("id")
	docID := c.Param("docId")

	if h.kbRepo != nil {
		if _, ok := h.ensureKnowledgeBaseAccess(c, kbID); !ok {
			return
		}
	}
	if h.knowledgeRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "数据库未连接"})
		return
	}

	knowledge, err := h.knowledgeRepo.GetByID(c.Request.Context(), docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if knowledge == nil || knowledge.KnowledgeBaseID != kbID {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}

	resp := gin.H{
		"knowledge_id":      knowledge.ID,
		"knowledge_base_id": knowledge.KnowledgeBaseID,
		"status":            knowledge.ParseStatus,
		"stage":             "",
		"chunk_count":       knowledge.ChunkCount,
		"error":             knowledge.ParseError,
		"created_at":        knowledge.CreatedAt,
		"updated_at":        knowledge.UpdatedAt,
	}
	if state, ok := h.getImportTaskState(c.Request.Context(), knowledge.ID); ok {
		if state.Status != "" {
			resp["status"] = state.Status
		}
		resp["stage"] = state.Stage
		resp["chunk_count"] = state.ChunkCount
		if state.Error != "" {
			resp["error"] = state.Error
		}
		if !state.StartedAt.IsZero() {
			resp["started_at"] = state.StartedAt
		}
		if !state.UpdatedAt.IsZero() {
			resp["updated_at"] = state.UpdatedAt
		}
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteDocument 删除文档
// @Summary 删除文档
// @Description 删除知识库中的指定文档
// @Tags 知识库
// @Param id path string true "知识库 ID"
// @Param docId path string true "文档 ID"
// @Success 200 {object} map[string]string
// @Router /knowledge-bases/{id}/documents/{docId} [delete]
func (h *Handler) DeleteDocument(c *gin.Context) {
	kbID := c.Param("id")
	docID := c.Param("docId")

	if h.kbRepo != nil {
		if _, ok := h.ensureKnowledgeBaseAccess(c, kbID); !ok {
			return
		}
	}

	if h.knowledgeRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "数据库未连接"})
		return
	}

	k, err := h.knowledgeRepo.GetByID(c.Request.Context(), docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if k == nil || k.KnowledgeBaseID != kbID {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}

	if err := h.knowledgeRepo.Delete(c.Request.Context(), docID); err != nil {
		h.audit(c, "doc.delete", docID, false, map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新知识库计数
	if h.kbRepo != nil {
		_ = h.kbRepo.IncrementCounts(c.Request.Context(), kbID, -1, 0)
	}
	h.deleteImportTaskState(c.Request.Context(), docID)
	if h.retrievalCache != nil {
		if err := h.retrievalCache.InvalidateKnowledgeBase(c.Request.Context(), kbID); err != nil {
			log.Printf("[Cache] 文档删除后失效检索缓存失败: kb=%s doc=%s err=%v", kbID, docID, err)
		}
	}

	h.audit(c, "doc.delete", docID, true, map[string]interface{}{"kb_id": kbID})
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (h *Handler) getImportTaskState(ctx context.Context, taskID string) (*cachepkg.ImportTaskState, bool) {
	if h.importStateStore == nil || strings.TrimSpace(taskID) == "" {
		return nil, false
	}
	state, hit, err := h.importStateStore.GetTaskState(ctx, taskID)
	if err != nil {
		log.Printf("[ImportState] 读取任务状态失败: task=%s err=%v", taskID, err)
		return nil, false
	}
	return state, hit && state != nil
}

func (h *Handler) writeImportTaskState(ctx context.Context, taskID string, mutate func(state *cachepkg.ImportTaskState)) {
	if h.importStateStore == nil || strings.TrimSpace(taskID) == "" || mutate == nil {
		return
	}
	now := time.Now()
	state := &cachepkg.ImportTaskState{
		StartedAt: now,
		UpdatedAt: now,
	}
	if existing, hit := h.getImportTaskState(ctx, taskID); hit {
		state = existing
		if state.StartedAt.IsZero() {
			state.StartedAt = now
		}
	}
	mutate(state)
	state.UpdatedAt = now
	if err := h.importStateStore.SetTaskState(ctx, taskID, state, h.importStateTTL()); err != nil {
		log.Printf("[ImportState] 写入任务状态失败: task=%s err=%v", taskID, err)
	}
}

func (h *Handler) deleteImportTaskState(ctx context.Context, taskID string) {
	if h.importStateStore == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	if err := h.importStateStore.DeleteTaskState(ctx, taskID); err != nil {
		log.Printf("[ImportState] 删除任务状态失败: task=%s err=%v", taskID, err)
	}
}

func (h *Handler) clearKnowledgeBaseImportStates(ctx context.Context, knowledgeBaseID string) {
	if h.importStateStore == nil || h.knowledgeRepo == nil || strings.TrimSpace(knowledgeBaseID) == "" {
		return
	}
	knowledges, err := h.knowledgeRepo.ListByKnowledgeBase(ctx, knowledgeBaseID, 0, 1000)
	if err != nil {
		log.Printf("[ImportState] 列出知识库文档失败，跳过状态清理: kb=%s err=%v", knowledgeBaseID, err)
		return
	}
	for _, knowledge := range knowledges {
		if knowledge == nil {
			continue
		}
		h.deleteImportTaskState(ctx, knowledge.ID)
	}
}

func (h *Handler) importStateTTL() time.Duration {
	minutes := h.cfg.ImportQueue.StateTTLMinutes
	if minutes <= 0 {
		minutes = 1440
	}
	return time.Duration(minutes) * time.Minute
}

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "数据库未连接"})
		return
	}

	var req struct {
		Title            string   `json:"title"`
		KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "数据库未连接"})
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
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "数据库未连接"})
		return
	}

	id := c.Param("id")
	if _, ok := h.ensureSessionAccess(c, id); !ok {
		return
	}
	if err := h.sessionRepo.Delete(c.Request.Context(), id); err != nil {
		h.audit(c, "session.delete", id, false, map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// ============================================================================
// 模型管理
// ============================================================================

// ListModels 获取模型列表
// @Summary 模型列表
// @Description 获取所有已配置的 AI 模型（LLM、Embedding、Reranker）
// @Tags 模型
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /models [get]
func (h *Handler) ListModels(c *gin.Context) {
	// 返回当前配置中的模型信息
	models := []gin.H{
		{
			"id":         "llm-default",
			"name":       h.cfg.LLM.ModelID,
			"type":       "llm",
			"provider":   h.cfg.LLM.Provider,
			"is_default": true,
			"config": gin.H{
				"base_url":    h.cfg.LLM.BaseURL,
				"temperature": h.cfg.LLM.Temperature,
				"max_tokens":  h.cfg.LLM.MaxTokens,
			},
		},
		{
			"id":         "embedding-default",
			"name":       h.cfg.Embedding.ModelID,
			"type":       "embedding",
			"provider":   h.cfg.Embedding.Provider,
			"is_default": true,
			"config": gin.H{
				"base_url":   h.cfg.Embedding.BaseURL,
				"dimensions": h.cfg.Embedding.Dimensions,
			},
		},
	}

	if h.cfg.Reranker.Enabled {
		models = append(models, gin.H{
			"id":         "reranker-default",
			"name":       h.cfg.Reranker.ModelID,
			"type":       "reranker",
			"provider":   h.cfg.Reranker.Provider,
			"is_default": true,
			"config": gin.H{
				"base_url":  h.cfg.Reranker.BaseURL,
				"top_k":     h.cfg.Reranker.TopK,
				"threshold": h.cfg.Reranker.Threshold,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

// CreateModel 创建模型配置
// @Summary 创建模型
// @Description 添加新的模型配置
// @Tags 模型
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /models [post]
func (h *Handler) CreateModel(c *gin.Context) {
	// TODO: 持久化到数据库
	c.JSON(http.StatusOK, gin.H{"message": "模型管理功能开发中，当前通过配置文件管理"})
}

// DeleteModel 删除模型配置
// @Summary 删除模型
// @Description 删除指定模型配置
// @Tags 模型
// @Param id path string true "模型 ID"
// @Success 200 {object} map[string]string
// @Router /models/{id} [delete]
func (h *Handler) DeleteModel(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "模型管理功能开发中"})
}

// ============================================================================
// 系统设置
// ============================================================================

// GetSettings 获取系统设置
// @Summary 获取设置
// @Description 获取系统配置信息（RAG参数、Agent 配置等）
// @Tags 设置
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /settings [get]
func (h *Handler) GetSettings(c *gin.Context) {
	// API Key 脱敏显示
	maskedAPIKey := ""
	if k := h.cfg.LLM.APIKey; len(k) > 8 {
		maskedAPIKey = k[:4] + "****" + k[len(k)-4:]
	} else if k != "" {
		maskedAPIKey = "****"
	}

	maskedEmbAPIKey := ""
	if k := h.cfg.Embedding.APIKey; len(k) > 8 {
		maskedEmbAPIKey = k[:4] + "****" + k[len(k)-4:]
	} else if k != "" {
		maskedEmbAPIKey = "****"
	}

	maskedRerankAPIKey := ""
	if k := h.cfg.Reranker.APIKey; len(k) > 8 {
		maskedRerankAPIKey = k[:4] + "****" + k[len(k)-4:]
	} else if k != "" {
		maskedRerankAPIKey = "****"
	}

	settings := gin.H{
		"rag": gin.H{
			"enabled":            true,
			"top_k":              h.cfg.RAG.TopK,
			"score_threshold":    0.5,
			"enable_hybrid":      h.cfg.RAG.EnableHybrid,
			"enable_rewrite":     h.cfg.RAG.EnableRewrite,
			"enable_rerank":      h.cfg.RAG.EnableRerank,
			"chunk_size":                   h.cfg.RAG.ChunkSize,
			"chunk_overlap":                h.cfg.RAG.ChunkOverlap,
			"chunk_strategy":               h.cfg.RAG.ChunkStrategy,
			"enable_contextual_enrichment": h.cfg.RAG.EnableContextualEnrichment,
			"embedding_model":    h.cfg.Embedding.ModelID,
			"knowledge_base_ids": []string{},
		},
		"llm": gin.H{
			"provider":    h.cfg.LLM.Provider,
			"model":       h.cfg.LLM.ModelID,
			"base_url":    h.cfg.LLM.BaseURL,
			"api_key":     maskedAPIKey,
			"temperature": h.cfg.LLM.Temperature,
			"max_tokens":  h.cfg.LLM.MaxTokens,
			"top_p":       0.9,
		},
		"embedding": gin.H{
			"provider": h.cfg.Embedding.Provider,
			"model":    h.cfg.Embedding.ModelID,
			"base_url": h.cfg.Embedding.BaseURL,
			"api_key":  maskedEmbAPIKey,
		},
		"reranker": gin.H{
			"enabled":  h.cfg.Reranker.Enabled,
			"provider": h.cfg.Reranker.Provider,
			"model":    h.cfg.Reranker.ModelID,
			"base_url": h.cfg.Reranker.BaseURL,
			"api_key":  maskedRerankAPIKey,
			"top_k":    h.cfg.Reranker.TopK,
		},
		"agent": gin.H{
			"enabled":       h.cfg.Agent.Enabled,
			"max_steps":     h.cfg.Agent.MaxSteps,
			"system_prompt": h.cfg.Agent.SystemPrompt,
			"tools":         []string{},
			"agentic_rag": gin.H{
				"enabled":             h.cfg.Agent.AgenticRAG.Enabled,
				"max_retries":         h.cfg.Agent.AgenticRAG.MaxRetries,
				"quality_threshold":   h.cfg.Agent.AgenticRAG.QualityThreshold,
				"enable_web_fallback": h.cfg.Agent.AgenticRAG.EnableWebFallback,
				"max_run_steps":       h.cfg.Agent.AgenticRAG.MaxRunSteps,
			},
		},
		"mcp": gin.H{
			"enabled":      h.cfg.MCP.Enabled,
			"server_count": len(h.cfg.MCP.Servers),
		},
		"docreader": gin.H{
			"enabled":     h.cfg.DocReader.Enabled,
			"mode":        h.cfg.DocReader.Mode,
			"endpoint":    h.cfg.DocReader.Endpoint,
			"render_mode": h.cfg.DocReader.RenderMode,
		},
		// 前端使用 graph_rag 作为 key
		"graph_rag": gin.H{
			"enabled":             h.cfg.GraphRAG.Enabled,
			"max_depth":           h.cfg.GraphRAG.TopK,
			"community_detection": false,
			"neo4j_uri":           h.cfg.GraphRAG.Neo4jURI,
			"extract_temperature": h.cfg.GraphRAG.ExtractTemperature,
		},
		// 同时保留 graphrag key 兼容旧前端
		"graphrag": gin.H{
			"enabled":             h.cfg.GraphRAG.Enabled,
			"neo4j_uri":           h.cfg.GraphRAG.Neo4jURI,
			"extract_temperature": h.cfg.GraphRAG.ExtractTemperature,
			"top_k":               h.cfg.GraphRAG.TopK,
		},
	}

	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

// SettingsUpdateRequest 设置更新请求
type SettingsUpdateRequest struct {
	RAG        *RAGSettingsUpdate        `json:"rag,omitempty"`
	LLM        *LLMSettingsUpdate        `json:"llm,omitempty"`
	Embedding  *EmbeddingSettingsUpdate  `json:"embedding,omitempty"`
	Reranker   *RerankerSettingsUpdate   `json:"reranker,omitempty"`
	Agent      *AgentSettingsUpdate      `json:"agent,omitempty"`
	AgenticRAG *AgenticRAGSettingsUpdate `json:"agentic_rag,omitempty"`
	GraphRAG   *GraphRAGSettingsUpdate   `json:"graphrag,omitempty"`
	GraphRAGV2 *GraphRAGSettingsUpdate   `json:"graph_rag,omitempty"`
}

// EmbeddingSettingsUpdate Embedding 设置更新
type EmbeddingSettingsUpdate struct {
	Provider *string `json:"provider,omitempty"`
	Model    *string `json:"model,omitempty"`
	BaseURL  *string `json:"base_url,omitempty"`
	APIKey   *string `json:"api_key,omitempty"`
}

// GraphRAGSettingsUpdate GraphRAG 设置更新
type GraphRAGSettingsUpdate struct {
	Enabled            *bool    `json:"enabled,omitempty"`
	ExtractTemperature *float64 `json:"extract_temperature,omitempty"`
	TopK               *int     `json:"top_k,omitempty"`
	MaxDepth           *int     `json:"max_depth,omitempty"`
}

// RAGSettingsUpdate RAG 设置更新
type RAGSettingsUpdate struct {
	TopK          *int  `json:"top_k,omitempty"`
	EnableHybrid  *bool `json:"enable_hybrid,omitempty"`
	EnableRewrite *bool `json:"enable_rewrite,omitempty"`
	EnableRerank  *bool `json:"enable_rerank,omitempty"`
	ChunkSize     *int  `json:"chunk_size,omitempty"`
	ChunkOverlap  *int  `json:"chunk_overlap,omitempty"`
}

// LLMSettingsUpdate LLM 设置更新
type LLMSettingsUpdate struct {
	Provider    *string  `json:"provider,omitempty"`
	Model       *string  `json:"model,omitempty"`
	BaseURL     *string  `json:"base_url,omitempty"`
	APIKey      *string  `json:"api_key,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
}

// RerankerSettingsUpdate Reranker 设置更新
type RerankerSettingsUpdate struct {
	Enabled   *bool    `json:"enabled,omitempty"`
	Provider  *string  `json:"provider,omitempty"`
	Model     *string  `json:"model,omitempty"`
	BaseURL   *string  `json:"base_url,omitempty"`
	APIKey    *string  `json:"api_key,omitempty"`
	TopK      *int     `json:"top_k,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`
}

// AgentSettingsUpdate Agent 设置更新
type AgentSettingsUpdate struct {
	Enabled             *bool `json:"enabled,omitempty"`
	MaxSteps            *int  `json:"max_steps,omitempty"`
	EnableKnowledgeTool *bool `json:"enable_knowledge_tool,omitempty"`
	EnableWebSearch     *bool `json:"enable_web_search,omitempty"`
}

// AgenticRAGSettingsUpdate Agentic RAG 设置更新
type AgenticRAGSettingsUpdate struct {
	Enabled           *bool    `json:"enabled,omitempty"`
	MaxRetries        *int     `json:"max_retries,omitempty"`
	QualityThreshold  *float64 `json:"quality_threshold,omitempty"`
	EnableWebFallback *bool    `json:"enable_web_fallback,omitempty"`
	MaxRunSteps       *int     `json:"max_run_steps,omitempty"`
}

// UpdateSettings 更新系统设置
// @Summary 更新设置
// @Description 运行时更新系统配置（RAG、Agent、Agentic RAG）
// @Tags 设置
// @Accept json
// @Produce json
// @Param request body SettingsUpdateRequest true "要更新的设置"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /settings [put]
func (h *Handler) UpdateSettings(c *gin.Context) {
	var req SettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	changed := []string{}

	// 更新 RAG 设置
	if req.RAG != nil {
		if req.RAG.TopK != nil {
			h.cfg.RAG.TopK = *req.RAG.TopK
			changed = append(changed, "rag.top_k")
		}
		if req.RAG.EnableHybrid != nil {
			h.cfg.RAG.EnableHybrid = *req.RAG.EnableHybrid
			changed = append(changed, "rag.enable_hybrid")
		}
		if req.RAG.EnableRewrite != nil {
			h.cfg.RAG.EnableRewrite = *req.RAG.EnableRewrite
			changed = append(changed, "rag.enable_rewrite")
		}
		if req.RAG.EnableRerank != nil {
			h.cfg.RAG.EnableRerank = *req.RAG.EnableRerank
			changed = append(changed, "rag.enable_rerank")
		}
		if req.RAG.ChunkSize != nil {
			h.cfg.RAG.ChunkSize = *req.RAG.ChunkSize
			changed = append(changed, "rag.chunk_size")
		}
		if req.RAG.ChunkOverlap != nil {
			h.cfg.RAG.ChunkOverlap = *req.RAG.ChunkOverlap
			changed = append(changed, "rag.chunk_overlap")
		}
	}

	// 更新 Agent 设置
	if req.Agent != nil {
		if req.Agent.Enabled != nil {
			h.cfg.Agent.Enabled = *req.Agent.Enabled
			changed = append(changed, "agent.enabled")
		}
		if req.Agent.MaxSteps != nil {
			h.cfg.Agent.MaxSteps = *req.Agent.MaxSteps
			changed = append(changed, "agent.max_steps")
		}
		if req.Agent.EnableKnowledgeTool != nil {
			h.cfg.Agent.EnableKnowledgeTool = *req.Agent.EnableKnowledgeTool
			changed = append(changed, "agent.enable_knowledge_tool")
		}
		if req.Agent.EnableWebSearch != nil {
			h.cfg.Agent.EnableWebSearch = *req.Agent.EnableWebSearch
			changed = append(changed, "agent.enable_web_search")
		}
	}

	// 更新 LLM 设置
	if req.LLM != nil {
		if req.LLM.Provider != nil && *req.LLM.Provider != "" {
			h.cfg.LLM.Provider = *req.LLM.Provider
			changed = append(changed, "llm.provider")
		}
		if req.LLM.Model != nil && *req.LLM.Model != "" {
			h.cfg.LLM.ModelID = *req.LLM.Model
			changed = append(changed, "llm.model")
		}
		if req.LLM.BaseURL != nil {
			h.cfg.LLM.BaseURL = *req.LLM.BaseURL
			changed = append(changed, "llm.base_url")
		}
		if req.LLM.APIKey != nil && *req.LLM.APIKey != "" {
			// 跳过脱敏后的 key（包含 ****）
			if !strings.Contains(*req.LLM.APIKey, "****") {
				h.cfg.LLM.APIKey = *req.LLM.APIKey
				// 同步更新 Embedding APIKey（通常相同）
				if h.cfg.Embedding.APIKey == "" || h.cfg.Embedding.Provider == h.cfg.LLM.Provider {
					h.cfg.Embedding.APIKey = *req.LLM.APIKey
					h.cfg.Embedding.BaseURL = h.cfg.LLM.BaseURL
				}
				// 重建 Embedding Provider 使新 key 生效
				newEmb, _, err := container.NewEmbeddingProvider(c.Request.Context(), &h.cfg.Embedding)
				if err != nil {
					log.Printf("[Settings] 重建 Embedding 失败: %v", err)
				} else {
					h.embedding = newEmb
					log.Printf("[Settings] Embedding Provider 已用新 API Key 重建")
				}
				changed = append(changed, "llm.api_key")
			}
		}
		if req.LLM.Temperature != nil {
			if *req.LLM.Temperature < 0 || *req.LLM.Temperature > 2 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "temperature 应在 0-2 之间"})
				return
			}
			h.cfg.LLM.Temperature = *req.LLM.Temperature
			changed = append(changed, "llm.temperature")
		}
		if req.LLM.MaxTokens != nil {
			if *req.LLM.MaxTokens < 1 || *req.LLM.MaxTokens > 128000 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "max_tokens 应在 1-128000 之间"})
				return
			}
			h.cfg.LLM.MaxTokens = *req.LLM.MaxTokens
			changed = append(changed, "llm.max_tokens")
		}
	}

	// 更新 Embedding 设置
	if req.Embedding != nil {
		if req.Embedding.Provider != nil && *req.Embedding.Provider != "" {
			h.cfg.Embedding.Provider = *req.Embedding.Provider
			changed = append(changed, "embedding.provider")
		}
		if req.Embedding.Model != nil && *req.Embedding.Model != "" {
			h.cfg.Embedding.ModelID = *req.Embedding.Model
			changed = append(changed, "embedding.model")
		}
		if req.Embedding.BaseURL != nil {
			h.cfg.Embedding.BaseURL = *req.Embedding.BaseURL
			changed = append(changed, "embedding.base_url")
		}
		if req.Embedding.APIKey != nil && *req.Embedding.APIKey != "" {
			if !strings.Contains(*req.Embedding.APIKey, "****") {
				h.cfg.Embedding.APIKey = *req.Embedding.APIKey
				changed = append(changed, "embedding.api_key")
			}
		}
		// 重建 Embedding Provider
		if len(changed) > 0 {
			newEmb, _, err := container.NewEmbeddingProvider(c.Request.Context(), &h.cfg.Embedding)
			if err != nil {
				log.Printf("[Settings] 重建 Embedding 失败: %v", err)
			} else {
				h.embedding = newEmb
				log.Printf("[Settings] Embedding Provider 已重建")
			}
		}
	}

	// 更新 Reranker 设置
	if req.Reranker != nil {
		if req.Reranker.Enabled != nil {
			h.cfg.Reranker.Enabled = *req.Reranker.Enabled
			changed = append(changed, "reranker.enabled")
		}
		if req.Reranker.Provider != nil && *req.Reranker.Provider != "" {
			h.cfg.Reranker.Provider = *req.Reranker.Provider
			changed = append(changed, "reranker.provider")
		}
		if req.Reranker.Model != nil && *req.Reranker.Model != "" {
			h.cfg.Reranker.ModelID = *req.Reranker.Model
			changed = append(changed, "reranker.model")
		}
		if req.Reranker.BaseURL != nil {
			h.cfg.Reranker.BaseURL = *req.Reranker.BaseURL
			changed = append(changed, "reranker.base_url")
		}
		if req.Reranker.APIKey != nil && *req.Reranker.APIKey != "" {
			if !strings.Contains(*req.Reranker.APIKey, "****") {
				h.cfg.Reranker.APIKey = *req.Reranker.APIKey
				changed = append(changed, "reranker.api_key")
			}
		}
		if req.Reranker.TopK != nil {
			if *req.Reranker.TopK < 1 || *req.Reranker.TopK > 50 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "reranker.top_k 应在 1-50 之间"})
				return
			}
			h.cfg.Reranker.TopK = *req.Reranker.TopK
			changed = append(changed, "reranker.top_k")
		}
		if req.Reranker.Threshold != nil {
			if *req.Reranker.Threshold < 0 || *req.Reranker.Threshold > 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "reranker.threshold 应在 0-1 之间"})
				return
			}
			h.cfg.Reranker.Threshold = *req.Reranker.Threshold
			changed = append(changed, "reranker.threshold")
		}
		// 重建 Reranker Provider (如果需要的话，可以在这里触发重建)
		if len(changed) > 0 {
			log.Printf("[Settings] Reranker 配置已更新: %v", changed)
		}
	}

	// 更新 Agentic RAG 设置
	if req.AgenticRAG != nil {
		if req.AgenticRAG.Enabled != nil {
			h.cfg.Agent.AgenticRAG.Enabled = *req.AgenticRAG.Enabled
			changed = append(changed, "agentic_rag.enabled")
		}
		if req.AgenticRAG.MaxRetries != nil {
			h.cfg.Agent.AgenticRAG.MaxRetries = *req.AgenticRAG.MaxRetries
			changed = append(changed, "agentic_rag.max_retries")
		}
		if req.AgenticRAG.QualityThreshold != nil {
			h.cfg.Agent.AgenticRAG.QualityThreshold = *req.AgenticRAG.QualityThreshold
			changed = append(changed, "agentic_rag.quality_threshold")
		}
		if req.AgenticRAG.EnableWebFallback != nil {
			h.cfg.Agent.AgenticRAG.EnableWebFallback = *req.AgenticRAG.EnableWebFallback
			changed = append(changed, "agentic_rag.enable_web_fallback")
		}
		if req.AgenticRAG.MaxRunSteps != nil {
			h.cfg.Agent.AgenticRAG.MaxRunSteps = *req.AgenticRAG.MaxRunSteps
			changed = append(changed, "agentic_rag.max_run_steps")
		}
	}

	// 更新 GraphRAG 设置（兼容 graphrag 与 graph_rag）
	graphReq := req.GraphRAG
	if graphReq == nil {
		graphReq = req.GraphRAGV2
	}
	if graphReq != nil {
		if graphReq.Enabled != nil {
			h.cfg.GraphRAG.Enabled = *graphReq.Enabled
			changed = append(changed, "graphrag.enabled")
		}
		if graphReq.ExtractTemperature != nil {
			if *graphReq.ExtractTemperature < 0 || *graphReq.ExtractTemperature > 2 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "graphrag.extract_temperature 应在 0-2 之间"})
				return
			}
			h.cfg.GraphRAG.ExtractTemperature = *graphReq.ExtractTemperature
			changed = append(changed, "graphrag.extract_temperature")
		}

		topK := graphReq.TopK
		if topK == nil {
			topK = graphReq.MaxDepth
		}
		if topK != nil {
			if *topK < 1 || *topK > 50 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "graphrag.top_k 应在 1-50 之间"})
				return
			}
			h.cfg.GraphRAG.TopK = *topK
			changed = append(changed, "graphrag.top_k")
		}
	}

	if len(changed) == 0 {
		h.audit(c, "settings.update", "settings", true, map[string]interface{}{"changed": changed})
		c.JSON(http.StatusOK, gin.H{"message": "无变更", "changed": changed})
		return
	}

	if err := config.Save(h.configPath, h.cfg); err != nil {
		log.Printf("[Settings] 持久化配置失败: %v", err)
		h.audit(c, "settings.update", "settings", false, map[string]interface{}{"error": err.Error(), "changed": changed})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "配置已在运行时更新，但持久化到配置文件失败",
			"changed": changed,
		})
		return
	}
	h.audit(c, "settings.update", "settings", true, map[string]interface{}{"changed": changed})

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("已更新 %d 项设置（运行时生效并已持久化）", len(changed)),
		"changed": changed,
	})
}

type MCPImportRequest struct {
	Provider    string   `json:"provider"`
	Name        string   `json:"name"`
	Endpoint    string   `json:"endpoint"`
	Transport   string   `json:"transport"`
	ToolNames   []string `json:"tool_names"`
	APIKey      string   `json:"api_key"`
	UseKeyInURL bool     `json:"use_key_in_url"`
}

type MCPServerView struct {
	Name         string   `json:"name"`
	Endpoint     string   `json:"endpoint"`
	Transport    string   `json:"transport"`
	ToolNames    []string `json:"tool_names"`
	HasAPIKey    bool     `json:"has_api_key"`
	APIKeyHeader string   `json:"api_key_header,omitempty"`
}

func (h *Handler) GetMCPStatus(c *gin.Context) {
	servers := make([]MCPServerView, 0, len(h.cfg.MCP.Servers))
	for _, s := range h.cfg.MCP.Servers {
		servers = append(servers, MCPServerView{
			Name:         s.Name,
			Endpoint:     s.Endpoint,
			Transport:    s.Transport,
			ToolNames:    s.ToolNames,
			HasAPIKey:    strings.TrimSpace(s.APIKey) != "",
			APIKeyHeader: s.APIKeyHeader,
		})
	}

	toolCount := 0
	if h.mcpMgr != nil {
		toolCount = len(h.mcpMgr.GetTools())
	}

	c.JSON(http.StatusOK, gin.H{
		"mcp": gin.H{
			"enabled":      h.cfg.MCP.Enabled,
			"server_count": len(h.cfg.MCP.Servers),
			"tool_count":   toolCount,
			"servers":      servers,
		},
	})
}

func (h *Handler) ImportMCPServer(c *gin.Context) {
	var req MCPImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "custom"
	}

	serverCfg, err := h.buildImportServerConfig(provider, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.cfg.MCP.Enabled = true
	upserted := false
	for i, s := range h.cfg.MCP.Servers {
		if strings.EqualFold(s.Name, serverCfg.Name) {
			h.cfg.MCP.Servers[i] = serverCfg
			upserted = true
			break
		}
	}
	if !upserted {
		h.cfg.MCP.Servers = append(h.cfg.MCP.Servers, serverCfg)
	}

	if err := config.Save(h.configPath, h.cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "MCP 配置导入成功，但持久化失败: " + err.Error()})
		return
	}

	toolCount, reloadErr := h.reloadMCPAndChat(c.Request.Context())
	if reloadErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":       "MCP 配置已保存，但运行时重载失败: " + reloadErr.Error(),
			"server_name": serverCfg.Name,
			"endpoint":    sanitizeMCPURL(serverCfg.Endpoint),
		})
		return
	}

	h.audit(c, "mcp.import", serverCfg.Name, true, map[string]interface{}{
		"provider":   provider,
		"transport":  serverCfg.Transport,
		"tool_count": toolCount,
	})

	c.JSON(http.StatusOK, gin.H{
		"message":     "MCP 导入并重载成功",
		"provider":    provider,
		"server_name": serverCfg.Name,
		"endpoint":    sanitizeMCPURL(serverCfg.Endpoint),
		"transport":   serverCfg.Transport,
		"tool_count":  toolCount,
	})
}

func (h *Handler) buildImportServerConfig(provider string, req MCPImportRequest) (config.MCPServerConfig, error) {
	switch provider {
	case "tavily":
		apiKey := strings.TrimSpace(req.APIKey)
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("TAVILY_API_KEY"))
		}
		if apiKey == "" {
			return config.MCPServerConfig{}, fmt.Errorf("tavily 导入需要 api_key 或环境变量 TAVILY_API_KEY")
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "tavily"
		}

		endpoint := strings.TrimSpace(req.Endpoint)
		if endpoint == "" {
			endpoint = "https://mcp.tavily.com/mcp/"
		}

		transport := strings.TrimSpace(req.Transport)
		if transport == "" {
			transport = "streamable_http"
		}

		s := config.MCPServerConfig{
			Name:         name,
			Endpoint:     endpoint,
			Transport:    transport,
			ToolNames:    req.ToolNames,
			APIKeyHeader: "Authorization",
			APIKeyPrefix: "Bearer",
		}

		if req.UseKeyInURL {
			u, err := url.Parse(endpoint)
			if err != nil {
				return config.MCPServerConfig{}, fmt.Errorf("endpoint 非法: %w", err)
			}
			q := u.Query()
			q.Set("tavilyApiKey", apiKey)
			u.RawQuery = q.Encode()
			s.Endpoint = u.String()
		} else {
			s.APIKey = apiKey
		}

		return s, nil
	case "custom":
		name := strings.TrimSpace(req.Name)
		endpoint := strings.TrimSpace(req.Endpoint)
		if name == "" || endpoint == "" {
			return config.MCPServerConfig{}, fmt.Errorf("custom 导入需要 name 和 endpoint")
		}

		transport := strings.TrimSpace(req.Transport)
		if transport == "" {
			transport = "sse"
		}

		return config.MCPServerConfig{
			Name:      name,
			Endpoint:  endpoint,
			Transport: transport,
			ToolNames: req.ToolNames,
			APIKey:    strings.TrimSpace(req.APIKey),
		}, nil
	default:
		return config.MCPServerConfig{}, fmt.Errorf("不支持的 provider: %s", provider)
	}
}

func (h *Handler) reloadMCPAndChat(ctx context.Context) (int, error) {
	if h.mcpMgr != nil {
		h.mcpMgr.Close()
	}

	mgr := mcpmanager.NewManager(&h.cfg.MCP)
	if err := mgr.Init(ctx); err != nil {
		return 0, err
	}
	h.mcpMgr = mgr

	h.chatService.SetMCPTools(mgr.GetTools())
	if err := h.chatService.Reload(); err != nil {
		return 0, err
	}

	return len(mgr.GetTools()), nil
}

func sanitizeMCPURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if q.Has("tavilyApiKey") {
		q.Set("tavilyApiKey", "****")
		u.RawQuery = q.Encode()
	}
	return u.String()
}

type EvalReportView struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

func (h *Handler) ListEvalReports(c *gin.Context) {
	reportDir := filepath.Join("docs", "eval_reports")
	entries, err := os.ReadDir(reportDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"reports": []EvalReportView{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	reports := make([]EvalReportView, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		reports = append(reports, EvalReportView{
			Name:       entry.Name(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().Format(time.RFC3339),
		})
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].ModifiedAt > reports[j].ModifiedAt
	})

	if len(reports) > 30 {
		reports = reports[:30]
	}

	c.JSON(http.StatusOK, gin.H{"reports": reports})
}

// GetSystemInfo 获取系统信息
// @Summary 系统信息
// @Description 获取系统版本、组件状态等信息
// @Tags 系统
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /system/info [get]
func (h *Handler) GetSystemInfo(c *gin.Context) {
	info := gin.H{
		"version":   "1.0.0",
		"framework": "Eino v0.7.32",
		"runtime":   "Go 1.24",
		"features": gin.H{
			"pipeline_rag": true,
			"react_agent":  h.cfg.Agent.Enabled,
			"agentic_rag":  h.cfg.Agent.AgenticRAG.Enabled,
			"graph_rag":    h.cfg.GraphRAG.Enabled,
			"web_search":   h.cfg.Agent.EnableWebSearch,
			"mcp":          h.cfg.MCP.Enabled,
			"docreader":    h.cfg.DocReader.Enabled,
			"reranker":     h.cfg.Reranker.Enabled,
		},
		"components": gin.H{
			"database":  h.db != nil,
			"vectordb":  h.vectorDB != nil,
			"embedding": h.embedding != nil,
			"docreader": h.docReaderCli != nil,
		},
		"models": gin.H{
			"llm":       h.cfg.LLM.ModelID,
			"embedding": h.cfg.Embedding.ModelID,
			"reranker":  h.cfg.Reranker.ModelID,
		},
	}

	c.JSON(http.StatusOK, info)
}
