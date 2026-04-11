// Package mcp 提供 MCP Server 实现
//
// 将 eino_agent 的核心能力暴露为 MCP 工具，供外部 Agent（Claude Desktop、
// Cursor、其他 MCP 客户端）直接调用。
//
// 暴露的工具：
//   - knowledge_search: 知识库语义检索
//   - chat: 完整的 RAG 问答
//   - list_knowledge_bases: 列出知识库
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	mcpProto "github.com/mark3labs/mcp-go/mcp"
	mcpServer "github.com/mark3labs/mcp-go/server"

	"eino_agent/internal/config"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/service"
)

// chatProvider 定义 MCP Server 需要的聊天能力（用于解耦和测试）
type chatProvider interface {
	Chat(ctx context.Context, req *service.ChatRequest) (*service.ChatResponse, error)
}

// Server 封装 MCP Server，暴露项目核心能力
type Server struct {
	mcpSrv  *mcpServer.MCPServer
	config  *config.Config
	chatSvc chatProvider
	kbRepo  repository.KnowledgeBaseRepository
}

// NewServer 创建 MCP Server
func NewServer(cfg *config.Config, chatSvc *service.ChatService, kbRepo repository.KnowledgeBaseRepository) *Server {
	s := &Server{
		config:  cfg,
		chatSvc: chatSvc,
		kbRepo:  kbRepo,
	}

	s.mcpSrv = mcpServer.NewMCPServer(
		"eino-rag-agent",
		"1.0.0",
		mcpServer.WithToolCapabilities(false),
	)

	s.registerTools()
	return s
}

// registerTools 注册所有对外暴露的 MCP 工具
func (s *Server) registerTools() {
	// 1. knowledge_search — 知识库检索
	s.mcpSrv.AddTool(
		mcpProto.NewTool(
			"knowledge_search",
			mcpProto.WithDescription("在知识库中检索与查询相关的文档片段。支持语义检索（向量）和全文检索（wiki）。返回最相关的文档内容及其来源。"),
			mcpProto.WithString("query",
				mcpProto.Required(),
				mcpProto.Description("检索查询文本"),
			),
			mcpProto.WithString("knowledge_base_ids",
				mcpProto.Description("知识库 ID 列表，逗号分隔。留空则搜索所有知识库。"),
			),
			mcpProto.WithNumber("top_k",
				mcpProto.Description("返回结果数量，默认 5"),
			),
		),
		s.handleKnowledgeSearch,
	)

	// 2. chat — 完整 RAG 问答
	s.mcpSrv.AddTool(
		mcpProto.NewTool(
			"chat",
			mcpProto.WithDescription("基于知识库的智能问答。自动检索相关文档，结合 LLM 生成有引用的回答。支持 Pipeline（线性 RAG）和 Agentic（ReAct Agent + 工具调用）两种模式。"),
			mcpProto.WithString("message",
				mcpProto.Required(),
				mcpProto.Description("用户问题"),
			),
			mcpProto.WithString("session_id",
				mcpProto.Description("会话 ID，用于多轮对话。留空则创建临时会话。"),
			),
			mcpProto.WithBoolean("use_agent",
				mcpProto.Description("是否使用 Agentic 模式（ReAct Agent + 工具调用）。默认 false 使用 Pipeline 模式。"),
			),
		),
		s.handleChat,
	)

	// 3. list_knowledge_bases — 列出知识库
	s.mcpSrv.AddTool(
		mcpProto.NewTool(
			"list_knowledge_bases",
			mcpProto.WithDescription("列出所有可用的知识库及其基本信息（名称、模式、文档数量）。"),
		),
		s.handleListKnowledgeBases,
	)
}

// handleKnowledgeSearch 处理知识库检索请求
func (s *Server) handleKnowledgeSearch(ctx context.Context, req mcpProto.CallToolRequest) (*mcpProto.CallToolResult, error) {
	query, _ := req.RequireString("query")
	if strings.TrimSpace(query) == "" {
		return mcpProto.NewToolResultError("query 参数不能为空"), nil
	}

	kbIDsStr := req.GetString("knowledge_base_ids", "")

	// 解析知识库 ID
	var kbIDs []string
	if kbIDsStr != "" {
		for _, id := range strings.Split(kbIDsStr, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				kbIDs = append(kbIDs, id)
			}
		}
	}

	// 使用 Pipeline 模式进行检索（通过 Chat 服务，只需要检索部分）
	chatReq := &service.ChatRequest{
		Message:          query,
		KnowledgeBaseIDs: kbIDs,
		ForceCitation:    true,
	}

	resp, err := s.chatSvc.Chat(ctx, chatReq)
	if err != nil {
		return mcpProto.NewToolResultError(fmt.Sprintf("检索失败: %v", err)), nil
	}

	// 格式化结果：回答 + 来源
	var sb strings.Builder
	sb.WriteString(resp.Answer)

	if len(resp.Sources) > 0 {
		sb.WriteString("\n\n---\n引用来源：\n")
		for i, src := range resp.Sources {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, src.DocID, truncateContent(src.Content, 200)))
		}
	}

	return mcpProto.NewToolResultText(sb.String()), nil
}

// handleChat 处理完整问答请求
func (s *Server) handleChat(ctx context.Context, req mcpProto.CallToolRequest) (*mcpProto.CallToolResult, error) {
	message, _ := req.RequireString("message")
	if strings.TrimSpace(message) == "" {
		return mcpProto.NewToolResultError("message 参数不能为空"), nil
	}

	sessionID := req.GetString("session_id", "")
	useAgent := req.GetBool("use_agent", false)

	chatReq := &service.ChatRequest{
		Message:   message,
		SessionID: sessionID,
		UseAgent:  useAgent,
	}

	resp, err := s.chatSvc.Chat(ctx, chatReq)
	if err != nil {
		return mcpProto.NewToolResultError(fmt.Sprintf("问答失败: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString(resp.Answer)

	if len(resp.Sources) > 0 {
		sb.WriteString("\n\n---\n引用来源：\n")
		for i, src := range resp.Sources {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, src.DocID, truncateContent(src.Content, 200)))
		}
	}

	return mcpProto.NewToolResultText(sb.String()), nil
}

// handleListKnowledgeBases 处理知识库列表请求
func (s *Server) handleListKnowledgeBases(ctx context.Context, _ mcpProto.CallToolRequest) (*mcpProto.CallToolResult, error) {
	// tenantID=0 表示全局查询，limit 设为 100 足够
	kbs, err := s.kbRepo.List(ctx, 0, 0, 100)
	if err != nil {
		return mcpProto.NewToolResultError(fmt.Sprintf("获取知识库列表失败: %v", err)), nil
	}

	if len(kbs) == 0 {
		return mcpProto.NewToolResultText("暂无知识库。"), nil
	}

	type kbInfo struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Description   string `json:"description"`
		Mode          string `json:"mode"`
		DocumentCount int    `json:"document_count"`
	}

	result := make([]kbInfo, 0, len(kbs))
	for _, kb := range kbs {
		result = append(result, kbInfo{
			ID:            kb.ID,
			Name:          kb.Name,
			Description:   kb.Description,
			Mode:          kb.Mode,
			DocumentCount: kb.DocumentCount,
		})
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return mcpProto.NewToolResultText(string(data)), nil
}

// truncateContent 截断内容到指定长度
func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// MCPServer 返回底层 MCPServer 实例（供启动 SSE/HTTP/Stdio 使用）
func (s *Server) MCPServer() *mcpServer.MCPServer {
	return s.mcpSrv
}

// ServeSSE 启动 SSE 传输的 MCP Server
func (s *Server) ServeSSE(addr string) error {
	sseServer := mcpServer.NewSSEServer(s.mcpSrv)
	log.Printf("[MCP Server] SSE 模式启动于 %s", addr)
	return sseServer.Start(addr)
}

// ServeStreamableHTTP 启动 Streamable HTTP 传输的 MCP Server
func (s *Server) ServeStreamableHTTP(addr string) error {
	httpServer := mcpServer.NewStreamableHTTPServer(s.mcpSrv)
	log.Printf("[MCP Server] Streamable HTTP 模式启动于 %s", addr)
	return httpServer.Start(addr)
}

// ServeStdio 以 Stdio 模式启动 MCP Server
func (s *Server) ServeStdio() error {
	log.Printf("[MCP Server] Stdio 模式启动")
	return mcpServer.ServeStdio(s.mcpSrv)
}
