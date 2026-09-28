package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/config"
	"eino_agent/internal/container"
)

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

// CreateModel 创建/更新模型配置
// @Summary 创建模型
// @Description 添加或更新模型配置（llm / embedding / reranker）。由于系统每种类型只有一个槽位，
//
//	实际效果等同于更新对应配置并热重载 Provider。
//
// @Tags 模型
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /models [post]
func (h *Handler) CreateModel(c *gin.Context) {
	h.settingsMu.Lock()
	defer h.settingsMu.Unlock()

	var req struct {
		Type        string  `json:"type" binding:"required"` // "llm" | "embedding" | "reranker"
		Provider    string  `json:"provider" binding:"required"`
		Model       string  `json:"model" binding:"required"`
		BaseURL     string  `json:"base_url"`
		APIKey      string  `json:"api_key"`
		Temperature float64 `json:"temperature"`
		Dimensions  int     `json:"dimensions"`
		TopK        int     `json:"top_k"`
		Threshold   float64 `json:"threshold"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx := c.Request.Context()
	var newID string

	switch req.Type {
	case "llm":
		h.cfg.LLM.Provider = req.Provider
		h.cfg.LLM.ModelID = req.Model
		if req.BaseURL != "" {
			h.cfg.LLM.BaseURL = req.BaseURL
		}
		if req.APIKey != "" && !strings.Contains(req.APIKey, "****") {
			h.cfg.LLM.APIKey = req.APIKey
		}
		if req.Temperature > 0 {
			h.cfg.LLM.Temperature = req.Temperature
		}
		// 重建 LLM Provider
		newModel, _, err := container.NewLLMProvider(ctx, &h.cfg.LLM)
		if err != nil {
			respondError(c, http.StatusBadRequest, fmt.Sprintf("LLM 初始化失败: %v", err))
			return
		}
		if h.chatService != nil {
			h.chatService.SetChatModel(newModel)
			if err := h.chatService.Reload(); err != nil {
				log.Printf("[Model] ChatService Reload 失败: %v", err)
			}
		}
		newID = "llm-default"

	case "embedding":
		h.cfg.Embedding.Provider = req.Provider
		h.cfg.Embedding.ModelID = req.Model
		if req.BaseURL != "" {
			h.cfg.Embedding.BaseURL = req.BaseURL
		}
		if req.APIKey != "" && !strings.Contains(req.APIKey, "****") {
			h.cfg.Embedding.APIKey = req.APIKey
		}
		if req.Dimensions > 0 {
			h.cfg.Embedding.Dimensions = req.Dimensions
		}
		// 重建 Embedding Provider
		newEmb, _, err := container.NewEmbeddingProvider(ctx, &h.cfg.Embedding)
		if err != nil {
			respondError(c, http.StatusBadRequest, fmt.Sprintf("Embedding 初始化失败: %v", err))
			return
		}
		h.setEmbedder(newEmb)
		newID = "embedding-default"

	case "reranker":
		h.cfg.Reranker.Provider = req.Provider
		h.cfg.Reranker.ModelID = req.Model
		h.cfg.Reranker.Enabled = true
		if req.BaseURL != "" {
			h.cfg.Reranker.BaseURL = req.BaseURL
		}
		if req.APIKey != "" && !strings.Contains(req.APIKey, "****") {
			h.cfg.Reranker.APIKey = req.APIKey
		}
		if req.TopK > 0 {
			h.cfg.Reranker.TopK = req.TopK
		}
		if req.Threshold > 0 {
			h.cfg.Reranker.Threshold = req.Threshold
		}
		newID = "reranker-default"

	default:
		respondError(c, http.StatusBadRequest, "type 必须为 llm / embedding / reranker")
		return
	}

	if err := config.Save(h.configPath, h.cfg); err != nil {
		log.Printf("[Model] 持久化配置失败: %v", err)
	}
	h.audit(c, "model.create", newID, true, map[string]interface{}{"type": req.Type, "provider": req.Provider, "model": req.Model})
	c.JSON(http.StatusCreated, gin.H{
		"message": fmt.Sprintf("%s 模型配置已更新", req.Type),
		"id":      newID,
	})
}

// DeleteModel 删除模型配置
// @Summary 删除模型
// @Description 删除指定模型配置。llm-default 和 embedding-default 不可删除；
//
//	reranker-default 删除等同于禁用 Reranker。
//
// @Tags 模型
// @Param id path string true "模型 ID"
// @Success 200 {object} map[string]string
// @Router /models/{id} [delete]
func (h *Handler) DeleteModel(c *gin.Context) {
	h.settingsMu.Lock()
	defer h.settingsMu.Unlock()

	id := c.Param("id")
	switch id {
	case "llm-default":
		respondError(c, http.StatusBadRequest, "LLM 主模型不可删除，可通过 PUT /settings 更新配置")
	case "embedding-default":
		respondError(c, http.StatusBadRequest, "Embedding 模型不可删除，可通过 PUT /settings 更新配置")
	case "reranker-default":
		h.cfg.Reranker.Enabled = false
		if err := config.Save(h.configPath, h.cfg); err != nil {
			log.Printf("[Model] 持久化配置失败: %v", err)
		}
		h.audit(c, "model.delete", id, true, nil)
		c.JSON(http.StatusOK, gin.H{"message": "Reranker 已禁用"})
	default:
		respondError(c, http.StatusNotFound, "模型不存在")
	}
}
