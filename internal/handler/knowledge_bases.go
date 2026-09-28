package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/container"
	"eino_agent/internal/database/repository"
)

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
// @Description 获取当前租户下所有知识库，支持分页
// @Tags 知识库
// @Produce json
// @Param page query int false "页码（从1开始，默认1）"
// @Param page_size query int false "每页数量（默认20，最大100）"
// @Success 200 {object} map[string]interface{}
// @Router /knowledge-bases [get]
func (h *Handler) ListKnowledgeBases(c *gin.Context) {
	if h.kbRepo == nil {
		c.JSON(http.StatusOK, gin.H{"knowledge_bases": []any{}, "message": "数据库未连接"})
		return
	}

	page, pageSize := parsePagination(c, 20, 100)
	offset := (page - 1) * pageSize
	tenantID := h.getTenantID(c)

	var kbs []*repository.KnowledgeBase
	var err error
	if h.accessRepo != nil {
		kbs, err = h.kbRepo.ListAccessible(c.Request.Context(), tenantID, h.getUserID(c), offset, pageSize)
	} else {
		kbs, err = h.kbRepo.List(c.Request.Context(), tenantID, offset, pageSize)
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	var total int
	if h.accessRepo != nil {
		total, _ = h.kbRepo.CountAccessible(c.Request.Context(), tenantID, h.getUserID(c))
	} else {
		total, _ = h.kbRepo.Count(c.Request.Context(), tenantID)
	}
	currentFingerprint := container.EmbedFingerprint(&h.cfg.Embedding)

	// Enrich each KB with embed_stale flag (never stale if KB was never indexed)
	type kbWithStale struct {
		*repository.KnowledgeBase
		EmbedStale bool `json:"embed_stale"`
	}
	enriched := make([]kbWithStale, 0, len(kbs))
	for _, kb := range kbs {
		stale := kb.EmbedModelFingerprint != "" && kb.EmbedModelFingerprint != currentFingerprint
		enriched = append(enriched, kbWithStale{kb, stale})
	}

	c.JSON(http.StatusOK, gin.H{
		"knowledge_bases": enriched,
		"page":            page,
		"page_size":       pageSize,
		"total":           total,
	})
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
		respondError(c, http.StatusServiceUnavailable, "数据库未连接")
		return
	}

	var req KnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
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
		respondError(c, http.StatusBadRequest, "mode 只能是 'vector' 或 'wiki'")
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
		respondError(c, http.StatusInternalServerError, err.Error())
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
		respondError(c, http.StatusServiceUnavailable, "数据库未连接")
		return
	}

	id := c.Param("id")
	kb, ok := h.ensureKnowledgeBaseAccess(c, id)
	if !ok {
		return
	}

	currentFingerprint := container.EmbedFingerprint(&h.cfg.Embedding)
	stale := kb.EmbedModelFingerprint != "" && kb.EmbedModelFingerprint != currentFingerprint
	c.JSON(http.StatusOK, gin.H{
		"id":                      kb.ID,
		"tenant_id":               kb.TenantID,
		"name":                    kb.Name,
		"description":             kb.Description,
		"mode":                    kb.Mode,
		"embedding_model_id":      kb.EmbeddingModelID,
		"embedding_dimensions":    kb.EmbeddingDimensions,
		"embed_model_fingerprint": kb.EmbedModelFingerprint,
		"embed_stale":             stale,
		"chunking_config":         kb.ChunkingConfig,
		"extract_config":          kb.ExtractConfig,
		"document_count":          kb.DocumentCount,
		"chunk_count":             kb.ChunkCount,
		"created_at":              kb.CreatedAt,
		"updated_at":              kb.UpdatedAt,
	})
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
		respondError(c, http.StatusServiceUnavailable, "数据库未连接")
		return
	}

	id := c.Param("id")
	if _, ok := h.ensureKnowledgeBaseWriteAccess(c, id); !ok {
		return
	}
	var req KnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
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
		respondError(c, http.StatusInternalServerError, err.Error())
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
		respondError(c, http.StatusServiceUnavailable, "数据库未连接")
		return
	}

	id := c.Param("id")
	if _, ok := h.ensureKnowledgeBaseAdminAccess(c, id); !ok {
		return
	}
	h.clearKnowledgeBaseImportStates(c.Request.Context(), id)
	if err := h.kbRepo.Delete(c.Request.Context(), id); err != nil {
		h.audit(c, "kb.delete", id, false, map[string]interface{}{"error": err.Error()})
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 清理向量数据库中该知识库的所有 chunk
	if h.vectorDB != nil {
		if err := h.vectorDB.DeleteByKnowledgeBaseID(c.Request.Context(), id); err != nil {
			log.Printf("[VectorDB] 删除知识库向量失败（数据可能残留）: kb=%s err=%v", id, err)
		}
	}

	if h.retrievalCache != nil {
		if err := h.retrievalCache.InvalidateKnowledgeBase(c.Request.Context(), id); err != nil {
			log.Printf("[Cache] 知识库删除后失效检索缓存失败: kb=%s err=%v", id, err)
		}
	}
	h.audit(c, "kb.delete", id, true, nil)

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
