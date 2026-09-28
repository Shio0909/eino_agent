package handler

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// 健康检查
	r.GET("/health", h.HealthCheck)
	r.GET("/health/live", h.LivenessCheck)
	r.GET("/health/ready", h.HealthCheck)

	// API v1
	v1 := r.Group("/api/v1")
	{
		// 鉴权
		auth := v1.Group("/auth")
		{
			auth.POST("/login", AuthRateLimitMiddleware(), h.Login)
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
		protected.GET("/traces/:trace_id", h.GetTrace)
		approvals := protected.Group("/approvals")
		{
			approvals.GET("/pending", h.ListPendingApprovals)
			approvals.GET("/:id", h.GetApproval)
			approvals.POST("/:id/decision", h.SubmitApprovalDecision)
		}

		// 知识库管理
		kb := protected.Group("/knowledge-bases")
		{
			kb.GET("", h.ListKnowledgeBases)
			kb.POST("", h.CreateKnowledgeBase)
			kb.GET("/:id", h.GetKnowledgeBase)
			kb.PUT("/:id", h.UpdateKnowledgeBase)
			kb.DELETE("/:id", h.DeleteKnowledgeBase)

			// Knowledge graph
			kb.GET("/:id/graph", h.GetGraphRAGGraph)
			kb.POST("/:id/graph/build", h.BuildGraphForKB)
			kb.DELETE("/:id/graph", h.DeleteGraphForKB)

			// Wiki pages
			kb.GET("/:id/wiki/pages", h.ListWikiPages)
			kb.GET("/:id/wiki/page", h.GetWikiPage)
			kb.GET("/:id/wiki/page/links", h.GetWikiPageLinks)
			kb.GET("/:id/wiki/search", h.SearchWikiPages)

			// 知识文档
			kb.POST("/:id/documents", h.UploadDocument)
			kb.POST("/:id/documents/url", h.UploadDocumentURL)
			kb.GET("/:id/documents", h.ListDocuments)
			kb.GET("/:id/documents/:docId/status", h.GetDocumentImportStatus)
			kb.GET("/:id/documents/:docId/chunks", h.GetDocumentChunks)
			kb.PATCH("/:id/documents/:docId/chunks/:chunkId", h.UpdateDocumentChunk)
			kb.GET("/:id/documents/:docId/chunks/:chunkId/revisions", h.ListChunkRevisions)
			kb.POST("/:id/documents/:docId/chunks/:chunkId/revisions/:revisionId/restore", h.RestoreChunkRevision)
			kb.POST("/:id/documents/:docId/reindex", h.ReindexDocument)
			kb.POST("/:id/documents/:docId/chunks/:chunkId/reindex", h.ReindexEditedChunk)
			kb.POST("/:id/documents/:docId/sync", h.SyncDocument)
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
			sessions.GET("/:id/traces", h.ListSessionTraces)
		}

		// 模型管理
		models := protected.Group("/models")
		{
			models.GET("", h.ListModels)
			models.POST("", h.RequireRole("admin"), h.CreateModel)
			models.DELETE("/:id", h.RequireRole("admin"), h.DeleteModel)
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
			eval.GET("/reports/:name", h.GetEvalReport)
		}

		protected.GET("/graphrag/status", h.GetGraphRAGStatus)

		// 代码仓库管理
		codeRepos := protected.Group("/code-repos")
		{
			codeRepos.GET("", h.ListCodeRepos)
			codeRepos.POST("/clone", h.RequireRole("admin"), h.CloneCodeRepo)
			codeRepos.POST("/:name/index", h.RequireRole("admin"), h.IndexCodeRepo)
			codeRepos.POST("/:name/pull", h.RequireRole("admin"), h.PullCodeRepo)
			codeRepos.DELETE("/:name", h.RequireRole("admin"), h.DeleteCodeRepo)
		}
	}
}
