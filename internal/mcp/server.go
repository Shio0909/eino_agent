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

	einoTool "github.com/cloudwego/eino/components/tool"

	"eino_agent/internal/codegraph"
	"eino_agent/internal/config"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/graphrag"
	"eino_agent/internal/service"
)

// chatProvider 定义 MCP Server 需要的聊天能力（用于解耦和测试）
type chatProvider interface {
	Chat(ctx context.Context, req *service.ChatRequest) (*service.ChatResponse, error)
}

// codeSearchProvider 定义代码搜索能力
type codeSearchProvider interface {
	InvokableRun(ctx context.Context, input string, opts ...einoTool.Option) (string, error)
}

// graphRAGProvider 定义 GraphRAG 查询能力
type graphRAGProvider interface {
	GetGraphForVis(ctx context.Context, kbID string, limit int) (*graphrag.VisGraph, error)
}

// Server 封装 MCP Server，暴露项目核心能力
type Server struct {
	mcpSrv    *mcpServer.MCPServer
	config    *config.Config
	chatSvc   chatProvider
	kbRepo    repository.KnowledgeBaseRepository
	codeTool  codeSearchProvider  // 可选：代码搜索工具
	graphRAG  graphRAGProvider    // 可选：GraphRAG 服务
	codeGraph codegraph.CodeGraphRepository // 可选：代码知识图谱
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

	return s
}

// SetCodeSearchTool 注入代码搜索工具（可选）
func (s *Server) SetCodeSearchTool(t codeSearchProvider) { s.codeTool = t }

// SetGraphRAGService 注入 GraphRAG 服务（可选）
func (s *Server) SetGraphRAGService(svc graphRAGProvider) { s.graphRAG = svc }

// SetCodeGraph 注入代码知识图谱仓储（可选）
func (s *Server) SetCodeGraph(repo codegraph.CodeGraphRepository) { s.codeGraph = repo }

// Init 注册所有工具并初始化 MCP Server（在设置好所有可选依赖后调用）
func (s *Server) Init() {
	s.registerTools()
}

// registerTools 注册所有对外暴露的 MCP 工具
func (s *Server) registerTools() {
	// ── 核心工具（始终注册） ──

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

	// 4. get_knowledge_base — 获取知识库详情
	s.mcpSrv.AddTool(
		mcpProto.NewTool(
			"get_knowledge_base",
			mcpProto.WithDescription("获取指定知识库的详细信息，包括名称、描述、模式（vector/wiki）、文档数量、分块数量等。"),
			mcpProto.WithString("id",
				mcpProto.Required(),
				mcpProto.Description("知识库 ID"),
			),
		),
		s.handleGetKnowledgeBase,
	)

	// ── 可选工具（按依赖注入情况注册） ──

	// 5. code_search — 代码仓库检索
	if s.codeTool != nil {
		s.mcpSrv.AddTool(
			mcpProto.NewTool(
				"code_search",
				mcpProto.WithDescription("在已索引的代码仓库中搜索代码。支持 grep（正则搜索文件内容）、find（按文件名模式查找）、read（读取文件内容）三种操作。"),
				mcpProto.WithString("action",
					mcpProto.Description("操作类型: grep（搜索内容，默认）/ find（查找文件）/ read（读取文件）"),
				),
				mcpProto.WithString("pattern",
					mcpProto.Description("搜索模式：grep 时为正则表达式，find 时为文件名 glob 模式"),
				),
				mcpProto.WithString("repo",
					mcpProto.Description("仓库名称，留空则搜索所有仓库"),
				),
				mcpProto.WithString("file_glob",
					mcpProto.Description("文件类型过滤，如 *.go, *.py"),
				),
				mcpProto.WithString("path",
					mcpProto.Description("read 操作的文件路径（相对仓库根目录）"),
				),
			),
			s.handleCodeSearch,
		)
		log.Println("[MCP Server] 已注册工具: code_search")
	}

	// 6. graphrag_query — 知识图谱可视化查询
	if s.graphRAG != nil {
		s.mcpSrv.AddTool(
			mcpProto.NewTool(
				"graphrag_query",
				mcpProto.WithDescription("获取知识库的知识图谱数据（实体和关系），用于理解知识结构和概念之间的关联。"),
				mcpProto.WithString("knowledge_base_id",
					mcpProto.Required(),
					mcpProto.Description("知识库 ID"),
				),
				mcpProto.WithNumber("limit",
					mcpProto.Description("返回的最大节点数量，默认 50"),
				),
			),
			s.handleGraphRAGQuery,
		)
		log.Println("[MCP Server] 已注册工具: graphrag_query")
	}

	// 7. code_graph_query — 代码知识图谱查询
	if s.codeGraph != nil {
		s.mcpSrv.AddTool(
			mcpProto.NewTool(
				"code_graph_query",
				mcpProto.WithDescription("查询代码知识图谱。支持查找函数调用关系（callers/callees）、符号定义、文件结构、符号搜索等。"),
				mcpProto.WithString("action",
					mcpProto.Required(),
					mcpProto.Description("操作类型: callers（查找调用者）/ callees（查找被调用者）/ definition（查找定义）/ structure（文件结构）/ search（搜索符号）/ overview（仓库概览）"),
				),
				mcpProto.WithString("repo",
					mcpProto.Required(),
					mcpProto.Description("仓库名称"),
				),
				mcpProto.WithString("name",
					mcpProto.Description("函数/符号名称（callers/callees/definition/search 操作需要）"),
				),
				mcpProto.WithString("path",
					mcpProto.Description("文件路径（structure 操作需要）"),
				),
				mcpProto.WithNumber("depth",
					mcpProto.Description("调用链深度，默认 2（callers/callees 操作）"),
				),
				mcpProto.WithNumber("limit",
					mcpProto.Description("结果数量限制，默认 20（search 操作）"),
				),
			),
			s.handleCodeGraphQuery,
		)
		log.Println("[MCP Server] 已注册工具: code_graph_query")
	}
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

// handleGetKnowledgeBase 处理知识库详情请求
func (s *Server) handleGetKnowledgeBase(ctx context.Context, req mcpProto.CallToolRequest) (*mcpProto.CallToolResult, error) {
	id, _ := req.RequireString("id")
	if strings.TrimSpace(id) == "" {
		return mcpProto.NewToolResultError("id 参数不能为空"), nil
	}

	kb, err := s.kbRepo.GetByID(ctx, id)
	if err != nil {
		return mcpProto.NewToolResultError(fmt.Sprintf("获取知识库失败: %v", err)), nil
	}
	if kb == nil {
		return mcpProto.NewToolResultError(fmt.Sprintf("知识库 %s 不存在", id)), nil
	}

	data, _ := json.MarshalIndent(kb, "", "  ")
	return mcpProto.NewToolResultText(string(data)), nil
}

// handleCodeSearch 处理代码搜索请求
func (s *Server) handleCodeSearch(ctx context.Context, req mcpProto.CallToolRequest) (*mcpProto.CallToolResult, error) {
	// 构造 CodeSearchTool 期望的 JSON 输入
	input := map[string]string{
		"action":    req.GetString("action", "grep"),
		"pattern":   req.GetString("pattern", ""),
		"repo":      req.GetString("repo", ""),
		"file_glob": req.GetString("file_glob", ""),
		"path":      req.GetString("path", ""),
	}

	inputJSON, _ := json.Marshal(input)
	result, err := s.codeTool.InvokableRun(ctx, string(inputJSON))
	if err != nil {
		return mcpProto.NewToolResultError(fmt.Sprintf("代码搜索失败: %v", err)), nil
	}

	return mcpProto.NewToolResultText(result), nil
}

// handleGraphRAGQuery 处理知识图谱查询请求
func (s *Server) handleGraphRAGQuery(ctx context.Context, req mcpProto.CallToolRequest) (*mcpProto.CallToolResult, error) {
	kbID, _ := req.RequireString("knowledge_base_id")
	if strings.TrimSpace(kbID) == "" {
		return mcpProto.NewToolResultError("knowledge_base_id 参数不能为空"), nil
	}

	limit := req.GetInt("limit", 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	graph, err := s.graphRAG.GetGraphForVis(ctx, kbID, limit)
	if err != nil {
		return mcpProto.NewToolResultError(fmt.Sprintf("图谱查询失败: %v", err)), nil
	}

	// 构造摘要 + 数据
	type graphSummary struct {
		NodeCount int               `json:"node_count"`
		EdgeCount int               `json:"edge_count"`
		Nodes     []graphrag.VisNode `json:"nodes"`
		Edges     []graphrag.VisEdge `json:"edges"`
	}

	summary := graphSummary{
		NodeCount: len(graph.Nodes),
		EdgeCount: len(graph.Edges),
		Nodes:     graph.Nodes,
		Edges:     graph.Edges,
	}

	data, _ := json.MarshalIndent(summary, "", "  ")
	return mcpProto.NewToolResultText(string(data)), nil
}

// handleCodeGraphQuery 处理代码知识图谱查询请求
func (s *Server) handleCodeGraphQuery(ctx context.Context, req mcpProto.CallToolRequest) (*mcpProto.CallToolResult, error) {
	action, _ := req.RequireString("action")
	repo, _ := req.RequireString("repo")

	if strings.TrimSpace(action) == "" {
		return mcpProto.NewToolResultError("action 参数不能为空"), nil
	}
	if strings.TrimSpace(repo) == "" {
		return mcpProto.NewToolResultError("repo 参数不能为空"), nil
	}

	switch action {
	case "callers":
		name := req.GetString("name", "")
		if name == "" {
			return mcpProto.NewToolResultError("callers 操作需要 name 参数"), nil
		}
		depth := req.GetInt("depth", 2)
		rels, err := s.codeGraph.FindCallers(ctx, repo, name, depth)
		if err != nil {
			return mcpProto.NewToolResultError(fmt.Sprintf("查询调用者失败: %v", err)), nil
		}
		return marshalToolResult(rels)

	case "callees":
		name := req.GetString("name", "")
		if name == "" {
			return mcpProto.NewToolResultError("callees 操作需要 name 参数"), nil
		}
		depth := req.GetInt("depth", 2)
		rels, err := s.codeGraph.FindCallees(ctx, repo, name, depth)
		if err != nil {
			return mcpProto.NewToolResultError(fmt.Sprintf("查询被调用者失败: %v", err)), nil
		}
		return marshalToolResult(rels)

	case "definition":
		name := req.GetString("name", "")
		if name == "" {
			return mcpProto.NewToolResultError("definition 操作需要 name 参数"), nil
		}
		entities, err := s.codeGraph.FindDefinition(ctx, repo, name)
		if err != nil {
			return mcpProto.NewToolResultError(fmt.Sprintf("查找定义失败: %v", err)), nil
		}
		return marshalToolResult(entities)

	case "structure":
		path := req.GetString("path", "")
		if path == "" {
			return mcpProto.NewToolResultError("structure 操作需要 path 参数"), nil
		}
		entities, err := s.codeGraph.GetFileStructure(ctx, repo, path)
		if err != nil {
			return mcpProto.NewToolResultError(fmt.Sprintf("获取文件结构失败: %v", err)), nil
		}
		return marshalToolResult(entities)

	case "search":
		name := req.GetString("name", "")
		if name == "" {
			return mcpProto.NewToolResultError("search 操作需要 name 参数"), nil
		}
		limit := req.GetInt("limit", 20)
		entities, err := s.codeGraph.SearchSymbol(ctx, repo, name, limit)
		if err != nil {
			return mcpProto.NewToolResultError(fmt.Sprintf("搜索符号失败: %v", err)), nil
		}
		return marshalToolResult(entities)

	case "overview":
		overview, err := s.codeGraph.GetRepoOverview(ctx, repo)
		if err != nil {
			return mcpProto.NewToolResultError(fmt.Sprintf("获取仓库概览失败: %v", err)), nil
		}
		return marshalToolResult(overview)

	default:
		return mcpProto.NewToolResultError(fmt.Sprintf("未知操作: %s（支持: callers, callees, definition, structure, search, overview）", action)), nil
	}
}

// marshalToolResult 将任意值序列化为 JSON 工具结果
func marshalToolResult(v any) (*mcpProto.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcpProto.NewToolResultError(fmt.Sprintf("序列化结果失败: %v", err)), nil
	}
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
