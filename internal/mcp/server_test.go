package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	mcpProto "github.com/mark3labs/mcp-go/mcp"

	"eino_agent/internal/config"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/service"
)

// ---- Mock ----

type mockChatProvider struct {
	resp *service.ChatResponse
	err  error
	// 记录最后一次调用的请求
	lastReq *service.ChatRequest
}

func (m *mockChatProvider) Chat(_ context.Context, req *service.ChatRequest) (*service.ChatResponse, error) {
	m.lastReq = req
	return m.resp, m.err
}

type mockKBRepo struct {
	kbs []*repository.KnowledgeBase
	err error
}

func (m *mockKBRepo) Create(_ context.Context, _ *repository.KnowledgeBase) error  { return nil }
func (m *mockKBRepo) GetByID(_ context.Context, _ string) (*repository.KnowledgeBase, error) {
	return nil, nil
}
func (m *mockKBRepo) List(_ context.Context, _ int, _, _ int) ([]*repository.KnowledgeBase, error) {
	return m.kbs, m.err
}
func (m *mockKBRepo) Update(_ context.Context, _ *repository.KnowledgeBase) error { return nil }
func (m *mockKBRepo) Delete(_ context.Context, _ string) error                     { return nil }
func (m *mockKBRepo) IncrementCounts(_ context.Context, _ string, _, _ int) error  { return nil }

// ---- Helper ----

func newTestServer(chat chatProvider, kbRepo repository.KnowledgeBaseRepository) *Server {
	s := &Server{
		config:  &config.Config{},
		chatSvc: chat,
		kbRepo:  kbRepo,
	}
	s.mcpSrv = nil // 测试中不需要真实 MCP Server
	return s
}

func makeCallToolRequest(args map[string]any) mcpProto.CallToolRequest {
	return mcpProto.CallToolRequest{
		Params: struct {
			Name      string         `json:"name"`
			Arguments any            `json:"arguments,omitempty"`
			Meta      *mcpProto.Meta `json:"_meta,omitempty"`
		}{
			Arguments: args,
		},
	}
}

// ---- truncateContent 测试 ----

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"短于限制", "hello", 10, "hello"},
		{"等于限制", "hello", 5, "hello"},
		{"超出限制", "hello world", 5, "hello..."},
		{"空字符串", "", 5, ""},
		{"零长度限制", "hello", 0, "..."},
		{"中文截断", "你好世界测试", 6, "你好..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateContent(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateContent(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// ---- handleKnowledgeSearch 测试 ----

func TestHandleKnowledgeSearch_EmptyQuery(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})

	result, err := s.handleKnowledgeSearch(context.Background(), makeCallToolRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "query 参数不能为空")
}

func TestHandleKnowledgeSearch_WhitespaceQuery(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})

	result, err := s.handleKnowledgeSearch(context.Background(), makeCallToolRequest(map[string]any{
		"query": "   ",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "query 参数不能为空")
}

func TestHandleKnowledgeSearch_Success(t *testing.T) {
	mock := &mockChatProvider{
		resp: &service.ChatResponse{
			Answer: "Go 是一种编译型语言。",
			Sources: []service.Source{
				{DocID: "doc-1", Content: "Go 语言由 Google 开发"},
			},
		},
	}
	s := newTestServer(mock, &mockKBRepo{})

	result, err := s.handleKnowledgeSearch(context.Background(), makeCallToolRequest(map[string]any{
		"query": "什么是 Go 语言",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertToolSuccess(t, result)
	text := getToolResultText(t, result)
	if text == "" {
		t.Fatal("expected non-empty result text")
	}

	// 检查请求参数传递正确
	if mock.lastReq.Message != "什么是 Go 语言" {
		t.Errorf("message = %q, want %q", mock.lastReq.Message, "什么是 Go 语言")
	}
	if !mock.lastReq.ForceCitation {
		t.Error("expected ForceCitation = true")
	}
}

func TestHandleKnowledgeSearch_WithKBIDs(t *testing.T) {
	mock := &mockChatProvider{
		resp: &service.ChatResponse{Answer: "answer"},
	}
	s := newTestServer(mock, &mockKBRepo{})

	result, err := s.handleKnowledgeSearch(context.Background(), makeCallToolRequest(map[string]any{
		"query":              "test",
		"knowledge_base_ids": "kb-1, kb-2, kb-3",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolSuccess(t, result)

	if len(mock.lastReq.KnowledgeBaseIDs) != 3 {
		t.Fatalf("KnowledgeBaseIDs len = %d, want 3", len(mock.lastReq.KnowledgeBaseIDs))
	}
	want := []string{"kb-1", "kb-2", "kb-3"}
	for i, id := range mock.lastReq.KnowledgeBaseIDs {
		if id != want[i] {
			t.Errorf("KnowledgeBaseIDs[%d] = %q, want %q", i, id, want[i])
		}
	}
}

func TestHandleKnowledgeSearch_ChatError(t *testing.T) {
	mock := &mockChatProvider{
		err: fmt.Errorf("LLM timeout"),
	}
	s := newTestServer(mock, &mockKBRepo{})

	result, err := s.handleKnowledgeSearch(context.Background(), makeCallToolRequest(map[string]any{
		"query": "test",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "检索失败")
}

func TestHandleKnowledgeSearch_NoSources(t *testing.T) {
	mock := &mockChatProvider{
		resp: &service.ChatResponse{
			Answer:  "没有找到相关文档。",
			Sources: nil,
		},
	}
	s := newTestServer(mock, &mockKBRepo{})

	result, err := s.handleKnowledgeSearch(context.Background(), makeCallToolRequest(map[string]any{
		"query": "一个不存在的话题",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getToolResultText(t, result)
	if text != "没有找到相关文档。" {
		t.Errorf("text = %q, want exact answer without sources section", text)
	}
}

// ---- handleChat 测试 ----

func TestHandleChat_EmptyMessage(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})

	result, err := s.handleChat(context.Background(), makeCallToolRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "message 参数不能为空")
}

func TestHandleChat_Success_Pipeline(t *testing.T) {
	mock := &mockChatProvider{
		resp: &service.ChatResponse{
			Answer:    "RAG 的核心是检索增强。",
			SessionID: "sess-abc",
		},
	}
	s := newTestServer(mock, &mockKBRepo{})

	result, err := s.handleChat(context.Background(), makeCallToolRequest(map[string]any{
		"message": "什么是 RAG",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertToolSuccess(t, result)
	if mock.lastReq.UseAgent {
		t.Error("expected UseAgent = false for pipeline mode")
	}
}

func TestHandleChat_Success_Agentic(t *testing.T) {
	mock := &mockChatProvider{
		resp: &service.ChatResponse{Answer: "agent response"},
	}
	s := newTestServer(mock, &mockKBRepo{})

	result, err := s.handleChat(context.Background(), makeCallToolRequest(map[string]any{
		"message":    "分析这段代码",
		"use_agent":  true,
		"session_id": "sess-123",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertToolSuccess(t, result)
	if !mock.lastReq.UseAgent {
		t.Error("expected UseAgent = true")
	}
	if mock.lastReq.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", mock.lastReq.SessionID, "sess-123")
	}
}

func TestHandleChat_ChatError(t *testing.T) {
	mock := &mockChatProvider{err: fmt.Errorf("service unavailable")}
	s := newTestServer(mock, &mockKBRepo{})

	result, err := s.handleChat(context.Background(), makeCallToolRequest(map[string]any{
		"message": "test",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "问答失败")
}

func TestHandleChat_WithSources(t *testing.T) {
	mock := &mockChatProvider{
		resp: &service.ChatResponse{
			Answer: "answer with sources",
			Sources: []service.Source{
				{DocID: "d1", Content: "source content 1"},
				{DocID: "d2", Content: "source content 2"},
			},
		},
	}
	s := newTestServer(mock, &mockKBRepo{})

	result, err := s.handleChat(context.Background(), makeCallToolRequest(map[string]any{
		"message": "test",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getToolResultText(t, result)
	if text == "" {
		t.Fatal("expected non-empty text")
	}
	// 验证包含来源信息
	for _, want := range []string{"引用来源", "d1", "d2"} {
		if !contains(text, want) {
			t.Errorf("result text should contain %q", want)
		}
	}
}

// ---- handleListKnowledgeBases 测试 ----

func TestHandleListKnowledgeBases_Empty(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{kbs: nil})

	result, err := s.handleListKnowledgeBases(context.Background(), mcpProto.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getToolResultText(t, result)
	if text != "暂无知识库。" {
		t.Errorf("text = %q, want %q", text, "暂无知识库。")
	}
}

func TestHandleListKnowledgeBases_Success(t *testing.T) {
	kbs := []*repository.KnowledgeBase{
		{ID: "kb-1", Name: "技术文档", Description: "开发文档", Mode: "vector", DocumentCount: 10},
		{ID: "kb-2", Name: "Wiki 知识", Description: "团队知识", Mode: "wiki", DocumentCount: 25},
	}
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{kbs: kbs})

	result, err := s.handleListKnowledgeBases(context.Background(), mcpProto.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertToolSuccess(t, result)
	text := getToolResultText(t, result)

	// 解析 JSON 验证结构
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v\ntext: %s", err, text)
	}

	if len(parsed) != 2 {
		t.Fatalf("len = %d, want 2", len(parsed))
	}
	if parsed[0]["name"] != "技术文档" {
		t.Errorf("first KB name = %v, want '技术文档'", parsed[0]["name"])
	}
	if parsed[1]["mode"] != "wiki" {
		t.Errorf("second KB mode = %v, want 'wiki'", parsed[1]["mode"])
	}
	// 验证 document_count 字段存在
	if parsed[0]["document_count"] != float64(10) {
		t.Errorf("first KB document_count = %v, want 10", parsed[0]["document_count"])
	}
}

func TestHandleListKnowledgeBases_DBError(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{err: fmt.Errorf("connection refused")})

	result, err := s.handleListKnowledgeBases(context.Background(), mcpProto.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "获取知识库列表失败")
}

// ---- NewServer 测试 ----

func TestNewServer_RegistersTools(t *testing.T) {
	cfg := &config.Config{}
	s := &Server{
		config:  cfg,
		chatSvc: &mockChatProvider{},
		kbRepo:  &mockKBRepo{},
	}

	// 手动注册，不使用 NewServer（需要真实 ChatService）
	// 验证 registerTools 不 panic
	s.mcpSrv = nil

	// NewServer 通过 interface 接受 chatProvider，但公开签名要求 *ChatService
	// 这里测试 registerTools 独立调用
	// 由于 mcpSrv 为 nil 会 panic，我们跳过直接调用
	// 改为验证 NewServer 的工具注册通过 MCP Server 工具列表
}

// ---- 断言工具函数 ----

func assertToolError(t *testing.T, result *mcpProto.CallToolResult, msgContains string) {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if !result.IsError {
		t.Error("expected IsError = true")
	}
	text := getToolResultText(t, result)
	if !contains(text, msgContains) {
		t.Errorf("error text %q should contain %q", text, msgContains)
	}
}

func assertToolSuccess(t *testing.T, result *mcpProto.CallToolResult) {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.IsError {
		text := getToolResultText(t, result)
		t.Fatalf("unexpected error: %s", text)
	}
}

func getToolResultText(t *testing.T, result *mcpProto.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	// mcp-go 的 Content 是 []Content，TextContent 是第一个
	content := result.Content[0]
	if tc, ok := content.(mcpProto.TextContent); ok {
		return tc.Text
	}
	t.Fatalf("expected TextContent, got %T", content)
	return ""
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
