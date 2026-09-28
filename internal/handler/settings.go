package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	einoembedding "github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/gin-gonic/gin"

	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/service"
)

// GetSettings 获取系统设置
// @Summary 获取设置
// @Description 获取系统配置信息（RAG参数、Agent 配置等）
// @Tags 设置
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /settings [get]
func (h *Handler) GetSettings(c *gin.Context) {
	cfg := h.cfg
	settings := gin.H{
		"rag": gin.H{
			"top_k":                        cfg.RAG.TopK,
			"enable_hybrid":                cfg.RAG.EnableHybrid,
			"enable_rewrite":               cfg.RAG.EnableRewrite,
			"enable_rerank":                cfg.RAG.EnableRerank,
			"chunk_size":                   cfg.RAG.ChunkSize,
			"chunk_overlap":                cfg.RAG.ChunkOverlap,
			"chunk_strategy":               cfg.RAG.ChunkStrategy,
			"enable_contextual_enrichment": cfg.RAG.EnableContextualEnrichment,
		},
		"llm": gin.H{
			"provider":    cfg.LLM.Provider,
			"model":       cfg.LLM.ModelID,
			"base_url":    cfg.LLM.BaseURL,
			"api_key":     maskSecret(cfg.LLM.APIKey),
			"temperature": cfg.LLM.Temperature,
			"max_tokens":  cfg.LLM.MaxTokens,
		},
		"embedding": gin.H{
			"provider": cfg.Embedding.Provider,
			"model":    cfg.Embedding.ModelID,
			"base_url": cfg.Embedding.BaseURL,
			"api_key":  maskSecret(cfg.Embedding.APIKey),
		},
		"reranker": gin.H{
			"enabled":   cfg.Reranker.Enabled,
			"provider":  cfg.Reranker.Provider,
			"model":     cfg.Reranker.ModelID,
			"base_url":  cfg.Reranker.BaseURL,
			"api_key":   maskSecret(cfg.Reranker.APIKey),
			"top_k":     cfg.Reranker.TopK,
			"threshold": cfg.Reranker.Threshold,
		},
		"agent": gin.H{
			"enabled":               cfg.Agent.Enabled,
			"max_steps":             cfg.Agent.MaxSteps,
			"system_prompt":         cfg.Agent.SystemPrompt,
			"enable_knowledge_tool": cfg.Agent.EnableKnowledgeTool,
			"enable_web_search":     cfg.Agent.EnableWebSearch,
		},
		"mcp": gin.H{
			"enabled":      cfg.MCP.Enabled,
			"server_count": len(cfg.MCP.Servers),
		},
		"docreader": h.docReaderStatus(),
		"graphrag": gin.H{
			"enabled":             cfg.GraphRAG.Enabled,
			"neo4j_uri":           cfg.GraphRAG.Neo4jURI,
			"extract_temperature": cfg.GraphRAG.ExtractTemperature,
			"top_k":               cfg.GraphRAG.TopK,
		},
	}

	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func maskSecret(secret string) string {
	switch {
	case secret == "":
		return ""
	case len(secret) > 8:
		return secret[:4] + "****" + secret[len(secret)-4:]
	default:
		return "****"
	}
}

// SettingsUpdateRequest 设置更新请求
type SettingsUpdateRequest struct {
	RAG       *RAGSettingsUpdate       `json:"rag,omitempty"`
	LLM       *LLMSettingsUpdate       `json:"llm,omitempty"`
	Embedding *EmbeddingSettingsUpdate `json:"embedding,omitempty"`
	Reranker  *RerankerSettingsUpdate  `json:"reranker,omitempty"`
	Agent     *AgentSettingsUpdate     `json:"agent,omitempty"`
	GraphRAG  *GraphRAGSettingsUpdate  `json:"graphrag,omitempty"`
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

// UpdateSettings 更新系统设置
// @Summary 更新设置
// @Description 运行时更新系统配置。所有字段先校验，任何校验或依赖重建失败都不会改动运行时配置。
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
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	h.settingsMu.Lock()
	defer h.settingsMu.Unlock()

	next := *h.cfg
	changed, err := applySettingsUpdate(&next, &req)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(changed) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "无变更", "changed": changed})
		return
	}

	ctx := c.Request.Context()
	var warnings []string

	// Dependents are rebuilt from the candidate config before it is committed,
	// so a failing provider leaves the running system untouched.
	var newEmbedder einoembedding.Embedder
	if anyChanged(changed, "embedding.", "llm.api_key") {
		newEmbedder, _, err = container.NewEmbeddingProvider(ctx, &next.Embedding)
		if err != nil {
			respondError(c, http.StatusBadGateway, "重建 Embedding 失败: "+err.Error())
			return
		}
		if next.Embedding.ModelID != h.cfg.Embedding.ModelID || next.Embedding.Provider != h.cfg.Embedding.Provider {
			warnings = append(warnings, "Embedding 模型已变更，已有向量需要重新索引后才能正确检索")
		}
	}
	var newModel model.ChatModel
	if anyChanged(changed, "llm.provider", "llm.model", "llm.base_url", "llm.api_key") {
		newModel, _, err = container.NewLLMProvider(ctx, &next.LLM)
		if err != nil {
			respondError(c, http.StatusBadGateway, "重建 LLM 失败: "+err.Error())
			return
		}
	}
	var newReranker container.RerankerProvider
	rerankerChanged := anyChanged(changed, "reranker.")
	if rerankerChanged && next.Reranker.Enabled {
		newReranker, _, err = container.NewRerankerProvider(ctx, &next.Reranker)
		if err != nil {
			respondError(c, http.StatusBadRequest, "重建 Reranker 失败: "+err.Error())
			return
		}
	}

	*h.cfg = next
	if newEmbedder != nil {
		h.setEmbedder(newEmbedder)
	}
	if h.chatService != nil {
		if newModel != nil {
			h.chatService.SetChatModel(newModel)
		}
		if rerankerChanged {
			if newReranker != nil {
				h.chatService.SetReranker(service.NewRerankerAdapter(newReranker))
			} else {
				h.chatService.SetReranker(nil)
			}
		}
		if anyChanged(changed, "llm.", "rag.", "reranker.", "agent.") {
			if err := h.chatService.Reload(); err != nil {
				log.Printf("[Settings] ChatService reload failed: %v", err)
				warnings = append(warnings, "对话服务重载失败: "+err.Error())
			}
		}
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
		"message":  fmt.Sprintf("已更新 %d 项设置", len(changed)),
		"changed":  changed,
		"warnings": warnings,
	})
}

func anyChanged(changed []string, prefixes ...string) bool {
	for _, item := range changed {
		for _, prefix := range prefixes {
			if strings.HasPrefix(item, prefix) {
				return true
			}
		}
	}
	return false
}

// isMaskedSecret reports whether a submitted secret is the masked value that
// GetSettings returned, meaning the client did not change it.
func isMaskedSecret(value string) bool {
	return strings.Contains(value, "****")
}

// applySettingsUpdate validates req and applies it to cfg. It returns the
// dotted names of changed settings; cfg must be a copy when err may be non-nil.
func applySettingsUpdate(cfg *config.Config, req *SettingsUpdateRequest) ([]string, error) {
	var changed []string
	set := func(name string) { changed = append(changed, name) }

	if r := req.RAG; r != nil {
		if r.TopK != nil {
			if *r.TopK < 1 || *r.TopK > 100 {
				return nil, fmt.Errorf("rag.top_k 应在 1-100 之间")
			}
			cfg.RAG.TopK = *r.TopK
			set("rag.top_k")
		}
		if r.EnableHybrid != nil {
			cfg.RAG.EnableHybrid = *r.EnableHybrid
			set("rag.enable_hybrid")
		}
		if r.EnableRewrite != nil {
			cfg.RAG.EnableRewrite = *r.EnableRewrite
			set("rag.enable_rewrite")
		}
		if r.EnableRerank != nil {
			cfg.RAG.EnableRerank = *r.EnableRerank
			set("rag.enable_rerank")
		}
		if r.ChunkSize != nil {
			if *r.ChunkSize < 50 || *r.ChunkSize > 10000 {
				return nil, fmt.Errorf("chunk_size 必须在 50-10000 之间")
			}
			cfg.RAG.ChunkSize = *r.ChunkSize
			set("rag.chunk_size")
		}
		if r.ChunkOverlap != nil {
			if *r.ChunkOverlap < 0 || *r.ChunkOverlap > 5000 {
				return nil, fmt.Errorf("chunk_overlap 必须在 0-5000 之间")
			}
			cfg.RAG.ChunkOverlap = *r.ChunkOverlap
			set("rag.chunk_overlap")
		}
		if cfg.RAG.ChunkOverlap >= cfg.RAG.ChunkSize && cfg.RAG.ChunkSize > 0 {
			return nil, fmt.Errorf("chunk_overlap 必须小于 chunk_size")
		}
	}

	if r := req.Agent; r != nil {
		if r.Enabled != nil {
			cfg.Agent.Enabled = *r.Enabled
			set("agent.enabled")
		}
		if r.MaxSteps != nil {
			if *r.MaxSteps < 1 || *r.MaxSteps > 50 {
				return nil, fmt.Errorf("agent.max_steps 应在 1-50 之间")
			}
			cfg.Agent.MaxSteps = *r.MaxSteps
			set("agent.max_steps")
		}
		if r.EnableKnowledgeTool != nil {
			cfg.Agent.EnableKnowledgeTool = *r.EnableKnowledgeTool
			set("agent.enable_knowledge_tool")
		}
		if r.EnableWebSearch != nil {
			cfg.Agent.EnableWebSearch = *r.EnableWebSearch
			set("agent.enable_web_search")
		}
	}

	if r := req.LLM; r != nil {
		if r.Provider != nil && *r.Provider != "" {
			cfg.LLM.Provider = *r.Provider
			set("llm.provider")
		}
		if r.Model != nil && *r.Model != "" {
			cfg.LLM.ModelID = *r.Model
			set("llm.model")
		}
		if r.BaseURL != nil {
			cfg.LLM.BaseURL = *r.BaseURL
			set("llm.base_url")
		}
		if r.APIKey != nil && *r.APIKey != "" && !isMaskedSecret(*r.APIKey) {
			cfg.LLM.APIKey = *r.APIKey
			// Embedding often shares the LLM account; only inherit when it has no key of its own.
			if cfg.Embedding.APIKey == "" {
				cfg.Embedding.APIKey = *r.APIKey
			}
			set("llm.api_key")
		}
		if r.Temperature != nil {
			if *r.Temperature < 0 || *r.Temperature > 2 {
				return nil, fmt.Errorf("temperature 应在 0-2 之间")
			}
			cfg.LLM.Temperature = *r.Temperature
			set("llm.temperature")
		}
		if r.MaxTokens != nil {
			if *r.MaxTokens < 1 || *r.MaxTokens > 128000 {
				return nil, fmt.Errorf("max_tokens 应在 1-128000 之间")
			}
			cfg.LLM.MaxTokens = *r.MaxTokens
			set("llm.max_tokens")
		}
	}

	if r := req.Embedding; r != nil {
		if r.Provider != nil && *r.Provider != "" {
			cfg.Embedding.Provider = *r.Provider
			set("embedding.provider")
		}
		if r.Model != nil && *r.Model != "" {
			cfg.Embedding.ModelID = *r.Model
			set("embedding.model")
		}
		if r.BaseURL != nil {
			cfg.Embedding.BaseURL = *r.BaseURL
			set("embedding.base_url")
		}
		if r.APIKey != nil && *r.APIKey != "" && !isMaskedSecret(*r.APIKey) {
			cfg.Embedding.APIKey = *r.APIKey
			set("embedding.api_key")
		}
	}

	if r := req.Reranker; r != nil {
		if r.Enabled != nil {
			cfg.Reranker.Enabled = *r.Enabled
			set("reranker.enabled")
		}
		if r.Provider != nil && *r.Provider != "" {
			cfg.Reranker.Provider = *r.Provider
			set("reranker.provider")
		}
		if r.Model != nil && *r.Model != "" {
			cfg.Reranker.ModelID = *r.Model
			set("reranker.model")
		}
		if r.BaseURL != nil {
			cfg.Reranker.BaseURL = *r.BaseURL
			set("reranker.base_url")
		}
		if r.APIKey != nil && *r.APIKey != "" && !isMaskedSecret(*r.APIKey) {
			cfg.Reranker.APIKey = *r.APIKey
			set("reranker.api_key")
		}
		if r.TopK != nil {
			if *r.TopK < 1 || *r.TopK > 50 {
				return nil, fmt.Errorf("reranker.top_k 应在 1-50 之间")
			}
			cfg.Reranker.TopK = *r.TopK
			set("reranker.top_k")
		}
		if r.Threshold != nil {
			if *r.Threshold < 0 || *r.Threshold > 1 {
				return nil, fmt.Errorf("reranker.threshold 应在 0-1 之间")
			}
			cfg.Reranker.Threshold = *r.Threshold
			set("reranker.threshold")
		}
	}

	if r := req.GraphRAG; r != nil {
		if r.Enabled != nil {
			cfg.GraphRAG.Enabled = *r.Enabled
			set("graphrag.enabled")
		}
		if r.ExtractTemperature != nil {
			if *r.ExtractTemperature < 0 || *r.ExtractTemperature > 2 {
				return nil, fmt.Errorf("graphrag.extract_temperature 应在 0-2 之间")
			}
			cfg.GraphRAG.ExtractTemperature = *r.ExtractTemperature
			set("graphrag.extract_temperature")
		}
		if r.TopK != nil {
			if *r.TopK < 1 || *r.TopK > 50 {
				return nil, fmt.Errorf("graphrag.top_k 应在 1-50 之间")
			}
			cfg.GraphRAG.TopK = *r.TopK
			set("graphrag.top_k")
		}
	}

	return changed, nil
}
