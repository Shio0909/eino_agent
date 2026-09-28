package handler

import (
	"net/http"
	"runtime"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Version is the application version, overridable at build time with
// -ldflags "-X eino_agent/internal/handler.Version=v1.2.3".
var Version = "dev"

func dependencyVersion(module string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == module {
			if dep.Replace != nil {
				return dep.Replace.Version
			}
			return dep.Version
		}
	}
	return "unknown"
}

func buildRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			return setting.Value
		}
	}
	return ""
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
		"version":   Version,
		"revision":  buildRevision(),
		"framework": "Eino " + dependencyVersion("github.com/cloudwego/eino"),
		"runtime":   runtime.Version(),
		"features": gin.H{
			"pipeline_rag": true,
			"react_agent":  h.cfg.Agent.Enabled,
			"graph_rag":    h.cfg.GraphRAG.Enabled,
			"web_search":   h.cfg.Agent.EnableWebSearch,
			"mcp":          h.cfg.MCP.Enabled,
			"docreader":    h.cfg.DocReader.Enabled,
			"reranker":     h.cfg.Reranker.Enabled,
		},
		"components": gin.H{
			"database":  h.db != nil,
			"vectordb":  h.vectorDB != nil,
			"embedding": h.currentEmbedder() != nil,
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
