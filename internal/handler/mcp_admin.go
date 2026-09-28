package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/config"
	mcpmanager "eino_agent/internal/mcp"
)

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
	h.settingsMu.Lock()
	defer h.settingsMu.Unlock()

	var req MCPImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "custom"
	}

	serverCfg, err := h.buildImportServerConfig(provider, req)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
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
		respondError(c, http.StatusInternalServerError, "MCP 配置导入成功，但持久化失败: "+err.Error())
		return
	}

	toolCount, reloadErr := h.reloadMCPAndChat(c.Request.Context())
	if reloadErr != nil {
		respondErrorWithDetails(c, http.StatusInternalServerError, "MCP 配置已保存，但运行时重载失败: "+reloadErr.Error(), gin.H{
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
			apiKey = strings.TrimSpace(h.cfg.Agent.TavilyAPIKey)
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("TAVILY_API_KEY"))
		}
		if apiKey == "" {
			return config.MCPServerConfig{}, fmt.Errorf("tavily 导入需要 api_key、agent.tavily_api_key 或环境变量 TAVILY_API_KEY")
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
