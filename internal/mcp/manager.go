// Package mcp 提供 MCP (Model Context Protocol) 客户端集成
//
// 【Eino 特点】使用 eino-ext 的 MCP Tool 组件，将远程 MCP Server 的工具
// 自动转换为 Eino tool.BaseTool，直接注入 ReAct Agent 使用。
//
// 支持：
// - 连接多个 MCP Server（SSE 协议）
// - 自动发现并注册远程工具
// - 工具名过滤（只使用指定的工具）
// - 优雅关闭
package mcp

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	mcpClient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpProto "github.com/mark3labs/mcp-go/mcp"

	einoMCP "github.com/cloudwego/eino-ext/components/tool/mcp"

	"eino_agent/internal/config"
)

// Manager 管理多个 MCP Server 连接和工具
type Manager struct {
	config  *config.MCPConfig
	clients []mcpClient.MCPClient
	tools   []tool.BaseTool
}

// NewManager 创建 MCP 管理器
func NewManager(cfg *config.MCPConfig) *Manager {
	return &Manager{
		config:  cfg,
		clients: make([]mcpClient.MCPClient, 0),
		tools:   make([]tool.BaseTool, 0),
	}
}

// Init 连接所有配置的 MCP Server 并获取工具
func (m *Manager) Init(ctx context.Context) error {
	// 允许重复初始化：每次先关闭旧连接并重置状态
	m.Close()
	m.clients = make([]mcpClient.MCPClient, 0)
	m.tools = make([]tool.BaseTool, 0)

	if !m.config.Enabled || len(m.config.Servers) == 0 {
		return nil
	}

	for _, serverCfg := range m.config.Servers {
		tools, client, err := m.connectServer(ctx, serverCfg)
		if err != nil {
			log.Printf("[MCP] 连接服务器 %s (%s) 失败: %v", serverCfg.Name, serverCfg.Endpoint, err)
			continue
		}
		m.clients = append(m.clients, client)
		m.tools = append(m.tools, tools...)
		log.Printf("[MCP] 已连接 %s，获取 %d 个工具", serverCfg.Name, len(tools))
	}

	return nil
}

// connectServer 连接单个 MCP Server
func (m *Manager) connectServer(ctx context.Context, cfg config.MCPServerConfig) ([]tool.BaseTool, mcpClient.MCPClient, error) {
	headers := buildHeaders(cfg)

	var (
		cli *mcpClient.Client
		err error
	)
	shouldStart := true

	transportType := strings.ToLower(strings.TrimSpace(cfg.Transport))
	switch transportType {
	case "", "sse":
		opts := make([]transport.ClientOption, 0)
		if len(headers) > 0 {
			opts = append(opts, transport.WithHeaders(headers))
		}
		cli, err = mcpClient.NewSSEMCPClient(cfg.Endpoint, opts...)
		if err != nil {
			return nil, nil, fmt.Errorf("创建 SSE 客户端失败: %w", err)
		}
	case "streamable_http", "streamable-http", "http":
		opts := make([]transport.StreamableHTTPCOption, 0)
		if len(headers) > 0 {
			opts = append(opts, transport.WithHTTPHeaders(headers))
		}
		cli, err = mcpClient.NewStreamableHttpClient(cfg.Endpoint, opts...)
		if err != nil {
			return nil, nil, fmt.Errorf("创建 Streamable HTTP 客户端失败: %w", err)
		}
	case "stdio":
		command := strings.TrimSpace(cfg.Command)
		if command == "" {
			return nil, nil, fmt.Errorf("stdio 模式缺少 command 配置")
		}

		cli, err = mcpClient.NewStdioMCPClient(command, buildCommandEnv(cfg), cfg.Args...)
		if err != nil {
			return nil, nil, fmt.Errorf("创建 stdio 客户端失败: %w", err)
		}
		shouldStart = false
	default:
		return nil, nil, fmt.Errorf("不支持的 MCP 传输协议: %s", cfg.Transport)
	}

	// 启动连接
	if shouldStart {
		if err := cli.Start(ctx); err != nil {
			return nil, nil, fmt.Errorf("启动 MCP 连接失败: %w", err)
		}
	}

	// 初始化协议
	initReq := mcpProto.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcpProto.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcpProto.Implementation{
		Name:    "eino-rag-agent",
		Version: "1.0.0",
	}

	if _, err := cli.Initialize(ctx, initReq); err != nil {
		cli.Close()
		return nil, nil, fmt.Errorf("MCP 协议初始化失败: %w", err)
	}

	// 使用 Eino MCP Tool 组件获取工具
	tools, err := einoMCP.GetTools(ctx, &einoMCP.Config{
		Cli:          cli,
		ToolNameList: cfg.ToolNames,
	})
	if err != nil {
		cli.Close()
		return nil, nil, fmt.Errorf("获取 MCP 工具失败: %w", err)
	}

	return tools, cli, nil
}

func buildHeaders(cfg config.MCPServerConfig) map[string]string {
	headers := map[string]string{}

	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return headers
	}

	headerName := strings.TrimSpace(cfg.APIKeyHeader)
	if headerName == "" {
		headerName = "Authorization"
	}

	prefix := strings.TrimSpace(cfg.APIKeyPrefix)
	if prefix == "" {
		prefix = "Bearer"
	}

	if strings.EqualFold(headerName, "authorization") {
		headers[headerName] = fmt.Sprintf("%s %s", prefix, apiKey)
		return headers
	}

	headers[headerName] = apiKey
	return headers
}

func buildCommandEnv(cfg config.MCPServerConfig) []string {
	if len(cfg.Env) == 0 && strings.TrimSpace(cfg.APIKey) == "" {
		return nil
	}

	envMap := map[string]string{}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		envMap[parts[0]] = parts[1]
	}

	for _, entry := range cfg.Env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		envMap[key] = parts[1]
	}

	if strings.EqualFold(strings.TrimSpace(cfg.Name), "github") && strings.TrimSpace(cfg.APIKey) != "" {
		envMap["GITHUB_PERSONAL_ACCESS_TOKEN"] = strings.TrimSpace(cfg.APIKey)
	}

	envList := make([]string, 0, len(envMap))
	for key, value := range envMap {
		envList = append(envList, fmt.Sprintf("%s=%s", key, value))
	}

	return envList
}

// GetTools 返回所有已注册的 MCP 工具
func (m *Manager) GetTools() []tool.BaseTool {
	return m.tools
}

// Close 关闭所有 MCP 连接
func (m *Manager) Close() {
	for _, cli := range m.clients {
		if err := cli.Close(); err != nil {
			log.Printf("[MCP] 关闭连接失败: %v", err)
		}
	}
}
