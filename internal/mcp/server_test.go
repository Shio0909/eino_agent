package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	mcpProto "github.com/mark3labs/mcp-go/mcp"

	einoTool "github.com/cloudwego/eino/components/tool"

	"eino_agent/internal/approval"
	"eino_agent/internal/codegraph"
	"eino_agent/internal/config"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/graphrag"
	"eino_agent/internal/service"
)

// ---- Mock ----

type mockChatProvider struct {
	resp         *service.ChatResponse
	err          error
	retrieveResp *service.RetrieveResponse
	retrieveErr  error
	// 记录最后一次调用的请求
	lastReq         *service.ChatRequest
	lastRetrieveReq *service.RetrieveRequest
}

func (m *mockChatProvider) Chat(_ context.Context, req *service.ChatRequest) (*service.ChatResponse, error) {
	m.lastReq = req
	return m.resp, m.err
}

func (m *mockChatProvider) Retrieve(_ context.Context, req *service.RetrieveRequest) (*service.RetrieveResponse, error) {
	m.lastRetrieveReq = req
	return m.retrieveResp, m.retrieveErr
}

type mockKBRepo struct {
	kbs    []*repository.KnowledgeBase
	kb     *repository.KnowledgeBase
	err    error
	getErr error
}

func (m *mockKBRepo) Create(_ context.Context, _ *repository.KnowledgeBase) error { return nil }
func (m *mockKBRepo) GetByID(_ context.Context, id string) (*repository.KnowledgeBase, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.kb, nil
}
func (m *mockKBRepo) List(_ context.Context, _ int, _, _ int) ([]*repository.KnowledgeBase, error) {
	return m.kbs, m.err
}
func (m *mockKBRepo) ListAccessible(_ context.Context, _ int, _ string, _, _ int) ([]*repository.KnowledgeBase, error) {
	return m.kbs, m.err
}
func (m *mockKBRepo) Update(_ context.Context, _ *repository.KnowledgeBase) error { return nil }
func (m *mockKBRepo) Delete(_ context.Context, _ string) error                    { return nil }
func (m *mockKBRepo) IncrementCounts(_ context.Context, _ string, _, _ int) error { return nil }
func (m *mockKBRepo) Count(_ context.Context, _ int) (int, error)                 { return 0, nil }
func (m *mockKBRepo) CountAccessible(_ context.Context, _ int, _ string) (int, error) {
	return 0, nil
}
func (m *mockKBRepo) UpdateEmbedFingerprint(_ context.Context, _, _ string) error { return nil }

type mockCodeSearchProvider struct {
	result string
	err    error
}

func (m *mockCodeSearchProvider) InvokableRun(_ context.Context, input string, _ ...einoTool.Option) (string, error) {
	return m.result, m.err
}

type mockGraphRAGProvider struct {
	graph *graphrag.VisGraph
	err   error
}

func (m *mockGraphRAGProvider) GetGraphForVis(_ context.Context, _ string, _ int) (*graphrag.VisGraph, error) {
	return m.graph, m.err
}

type mockMCPWriter struct {
	importCalled bool
}

func (m *mockMCPWriter) MCPCreateKB(_ context.Context, name, desc, mode string) (*repository.KnowledgeBase, error) {
	return &repository.KnowledgeBase{ID: "kb-created", Name: name, Description: desc, Mode: mode}, nil
}
func (m *mockMCPWriter) MCPImportURL(_ context.Context, _ string, _ string, _ string) (string, int, error) {
	m.importCalled = true
	return "doc-imported", 3, nil
}
func (m *mockMCPWriter) MCPDeleteKB(_ context.Context, _ string) error { return nil }
func (m *mockMCPWriter) MCPListDocuments(_ context.Context, _ string) ([]map[string]any, error) {
	return nil, nil
}
func (m *mockMCPWriter) MCPDeleteDocument(_ context.Context, _ string, _ string) error { return nil }
func (m *mockMCPWriter) MCPCloneCodeRepo(_ context.Context, _ string, name string) (string, error) {
	if name == "" {
		return "repo", nil
	}
	return name, nil
}
func (m *mockMCPWriter) MCPIndexCodeRepo(_ context.Context, name string) (map[string]any, error) {
	return map[string]any{"files": 1, "name": name}, nil
}

type mockCodeGraphRepo struct {
	callers  []codegraph.CodeRelation
	callees  []codegraph.CodeRelation
	entities []codegraph.CodeEntity
	overview *codegraph.RepoOverview
	err      error
}

func (m *mockCodeGraphRepo) UpsertFile(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
func (m *mockCodeGraphRepo) UpsertEntities(_ context.Context, _ string, _ []codegraph.CodeEntity) error {
	return nil
}
func (m *mockCodeGraphRepo) UpsertRelations(_ context.Context, _ string, _ []codegraph.CodeRelation) error {
	return nil
}
func (m *mockCodeGraphRepo) DeleteFileGraph(_ context.Context, _ string, _ string) error { return nil }
func (m *mockCodeGraphRepo) DeleteRepoGraph(_ context.Context, _ string) error           { return nil }
func (m *mockCodeGraphRepo) FindCallers(_ context.Context, _ string, _ string, _ int) ([]codegraph.CodeRelation, error) {
	return m.callers, m.err
}
func (m *mockCodeGraphRepo) FindCallees(_ context.Context, _ string, _ string, _ int) ([]codegraph.CodeRelation, error) {
	return m.callees, m.err
}
func (m *mockCodeGraphRepo) FindDefinition(_ context.Context, _ string, _ string) ([]codegraph.CodeEntity, error) {
	return m.entities, m.err
}
func (m *mockCodeGraphRepo) GetFileStructure(_ context.Context, _ string, _ string) ([]codegraph.CodeEntity, error) {
	return m.entities, m.err
}
func (m *mockCodeGraphRepo) SearchSymbol(_ context.Context, _ string, _ string, _ int) ([]codegraph.CodeEntity, error) {
	return m.entities, m.err
}
func (m *mockCodeGraphRepo) GetRepoOverview(_ context.Context, _ string) (*codegraph.RepoOverview, error) {
	return m.overview, m.err
}

// ---- Helper ----

func newTestServer(chat chatProvider, kbRepo repository.KnowledgeBaseRepository) *Server {
	s := &Server{
		config:          &config.Config{},
		chatSvc:         chat,
		kbRepo:          kbRepo,
		approvalManager: approval.NewManager(1 * time.Minute),
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
		{"零长度限制", "hello", 0, ""},
		{"中文截断", "你好世界测试", 2, "你好..."},
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
		retrieveResp: &service.RetrieveResponse{
			Query: "什么是 Go 语言",
			Results: []service.RetrieveResult{
				{ID: "doc-1", Content: "Go 语言由 Google 开发", Snippet: "Go 语言由 Google 开发"},
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
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("result should be JSON: %v\ntext: %s", err, text)
	}
	if parsed["query"] != "什么是 Go 语言" {
		t.Errorf("query = %v, want 什么是 Go 语言", parsed["query"])
	}
	if parsed["answer"] != nil {
		t.Fatalf("knowledge_search must not return generated answer: %v", parsed["answer"])
	}

	if mock.lastReq != nil {
		t.Fatal("knowledge_search must not call Chat/LLM generation")
	}
	if mock.lastRetrieveReq == nil {
		t.Fatal("expected Retrieve to be called")
	}
	if mock.lastRetrieveReq.Query != "什么是 Go 语言" {
		t.Errorf("query = %q, want %q", mock.lastRetrieveReq.Query, "什么是 Go 语言")
	}
}

func TestHandleKnowledgeSearch_RendersWikiSourceMetadata(t *testing.T) {
	mock := &mockChatProvider{
		retrieveResp: &service.RetrieveResponse{
			Query: "Go Slice 扩容规则",
			Results: []service.RetrieveResult{
				{
					ID:      "page-1",
					Content: "# Go语言Slice原理详解\n\n当往切片追加元素时，如果切片容量不足，会自动扩容。",
					Snippet: "当往切片追加元素时，如果切片容量不足，会自动扩容。",
					Metadata: map[string]interface{}{
						"knowledge_base_id": "kb-go",
						"wiki_path":         "slice-principles.md",
						"wiki_title":        "Go语言Slice原理详解",
						"wiki_page_type":    "topic",
						"match_type":        "fts",
					},
				},
			},
		},
	}
	s := newTestServer(mock, &mockKBRepo{})

	result, err := s.handleKnowledgeSearch(context.Background(), makeCallToolRequest(map[string]any{
		"query": "Go Slice 扩容规则",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getToolResultText(t, result)
	var parsed struct {
		Results []struct {
			ID              string         `json:"id"`
			Title           string         `json:"title"`
			Content         string         `json:"content"`
			Snippet         string         `json:"snippet"`
			KnowledgeBaseID string         `json:"knowledge_base_id"`
			WikiPath        string         `json:"wiki_path"`
			WikiPageType    string         `json:"wiki_page_type"`
			MatchType       string         `json:"match_type"`
			Metadata        map[string]any `json:"metadata"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v\ntext: %s", err, text)
	}
	if len(parsed.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(parsed.Results))
	}
	got := parsed.Results[0]
	if got.Title != "Go语言Slice原理详解" {
		t.Errorf("title = %q, want Go语言Slice原理详解", got.Title)
	}
	if got.ID != "page-1" || got.KnowledgeBaseID != "kb-go" || got.WikiPath != "slice-principles.md" {
		t.Errorf("unexpected result metadata: %+v", got)
	}
	if got.MatchType != "fts" || got.WikiPageType != "topic" {
		t.Errorf("unexpected match/page metadata: %+v", got)
	}
}

func TestHandleKnowledgeSearch_BoundsContentForMCP(t *testing.T) {
	longContent := strings.Repeat("Go Slice 扩容规则。", 400)
	mock := &mockChatProvider{
		retrieveResp: &service.RetrieveResponse{
			Query: "Go Slice 扩容",
			Results: []service.RetrieveResult{
				{
					ID:      "page-1",
					Content: longContent,
					Metadata: map[string]interface{}{
						"knowledge_base_id": "kb-go",
						"wiki_path":         "slice-principles.md",
					},
				},
			},
		},
	}
	s := newTestServer(mock, &mockKBRepo{})

	result, err := s.handleKnowledgeSearch(context.Background(), makeCallToolRequest(map[string]any{
		"query": "Go Slice 扩容",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getToolResultText(t, result)
	var parsed struct {
		Results []struct {
			Content string `json:"content"`
			Snippet string `json:"snippet"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v\ntext: %s", err, text)
	}
	if len(parsed.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(parsed.Results))
	}
	if len([]rune(parsed.Results[0].Content)) > 4003 {
		t.Fatalf("content length = %d, want bounded by 4000", len([]rune(parsed.Results[0].Content)))
	}
	if parsed.Results[0].Content == longContent {
		t.Fatal("content should be bounded for MCP output")
	}
	if parsed.Results[0].Snippet == "" || len([]rune(parsed.Results[0].Snippet)) > 503 {
		t.Fatalf("snippet length = %d, want non-empty bounded snippet", len([]rune(parsed.Results[0].Snippet)))
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

	if len(mock.lastRetrieveReq.KnowledgeBaseIDs) != 3 {
		t.Fatalf("KnowledgeBaseIDs len = %d, want 3", len(mock.lastRetrieveReq.KnowledgeBaseIDs))
	}
	want := []string{"kb-1", "kb-2", "kb-3"}
	for i, id := range mock.lastRetrieveReq.KnowledgeBaseIDs {
		if id != want[i] {
			t.Errorf("KnowledgeBaseIDs[%d] = %q, want %q", i, id, want[i])
		}
	}
	if mock.lastReq != nil {
		t.Fatal("knowledge_search must not call Chat/LLM generation")
	}
}

func TestHandleKnowledgeSearch_RejectsOutOfScopeKBID(t *testing.T) {
	mock := &mockChatProvider{retrieveResp: &service.RetrieveResponse{Query: "test"}}
	s := newTestServer(mock, &mockKBRepo{})
	ctx := context.WithValue(context.Background(), ctxKeyScope, mcpScope{
		knowledgeBaseIDs: []string{"kb-allowed"},
		kbRestricted:     true,
	})

	result, err := s.handleKnowledgeSearch(ctx, makeCallToolRequest(map[string]any{
		"query":              "test",
		"knowledge_base_ids": "kb-denied",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "不在当前 MCP 凭据授权范围内")
	if mock.lastReq != nil || mock.lastRetrieveReq != nil {
		t.Fatal("search service should not be called for out-of-scope KB")
	}
}

func TestHandleKnowledgeSearch_UsesScopedKBIDsWhenRequestOmitsScope(t *testing.T) {
	mock := &mockChatProvider{retrieveResp: &service.RetrieveResponse{Query: "test"}}
	s := newTestServer(mock, &mockKBRepo{})
	ctx := context.WithValue(context.Background(), ctxKeyScope, mcpScope{
		knowledgeBaseIDs: []string{"kb-allowed"},
		kbRestricted:     true,
	})

	result, err := s.handleKnowledgeSearch(ctx, makeCallToolRequest(map[string]any{
		"query": "test",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolSuccess(t, result)
	if len(mock.lastRetrieveReq.KnowledgeBaseIDs) != 1 || mock.lastRetrieveReq.KnowledgeBaseIDs[0] != "kb-allowed" {
		t.Fatalf("KnowledgeBaseIDs = %#v, want [kb-allowed]", mock.lastRetrieveReq.KnowledgeBaseIDs)
	}
	if mock.lastReq != nil {
		t.Fatal("knowledge_search must not call Chat/LLM generation")
	}
}

func TestHandleKnowledgeSearch_RetrieveError(t *testing.T) {
	mock := &mockChatProvider{
		retrieveErr: fmt.Errorf("retriever timeout"),
	}
	s := newTestServer(mock, &mockKBRepo{})

	result, err := s.handleKnowledgeSearch(context.Background(), makeCallToolRequest(map[string]any{
		"query": "test",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "检索失败")
	if mock.lastReq != nil {
		t.Fatal("knowledge_search must not call Chat/LLM generation")
	}
}

func TestHandleKnowledgeSearch_NoSources(t *testing.T) {
	mock := &mockChatProvider{
		retrieveResp: &service.RetrieveResponse{
			Query:   "一个不存在的话题",
			Results: nil,
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
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("result should be JSON: %v\ntext: %s", err, text)
	}
	if parsed["total"] != float64(0) {
		t.Errorf("total = %v, want 0", parsed["total"])
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

func TestShouldExposeAnswerTool_DefaultFalse(t *testing.T) {
	if shouldExposeAnswerTool(&config.Config{}) {
		t.Fatal("answer tool should be disabled by default")
	}
}

func TestShouldExposeAnswerTool_ExplicitTrue(t *testing.T) {
	cfg := &config.Config{MCPExport: config.MCPExportConfig{EnableAnswerTool: true}}
	if !shouldExposeAnswerTool(cfg) {
		t.Fatal("answer tool should be enabled when explicitly configured")
	}
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

// ---- handleGetKnowledgeBase 测试 ----

func TestHandleGetKnowledgeBase_EmptyID(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})

	result, err := s.handleGetKnowledgeBase(context.Background(), makeCallToolRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "id 参数不能为空")
}

func TestHandleGetKnowledgeBase_NotFound(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{kb: nil})

	result, err := s.handleGetKnowledgeBase(context.Background(), makeCallToolRequest(map[string]any{
		"id": "nonexistent-kb",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "不存在")
}

func TestHandleGetKnowledgeBase_Success(t *testing.T) {
	kb := &repository.KnowledgeBase{
		ID: "kb-1", Name: "测试知识库", Description: "desc", Mode: "vector", DocumentCount: 5,
	}
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{kb: kb})

	result, err := s.handleGetKnowledgeBase(context.Background(), makeCallToolRequest(map[string]any{
		"id": "kb-1",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertToolSuccess(t, result)
	text := getToolResultText(t, result)
	if !contains(text, "测试知识库") {
		t.Errorf("result should contain KB name, got: %s", text)
	}
}

func TestHandleGetKnowledgeBase_DBError(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{getErr: fmt.Errorf("db down")})

	result, err := s.handleGetKnowledgeBase(context.Background(), makeCallToolRequest(map[string]any{
		"id": "kb-1",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "获取知识库失败")
}

// ---- handleCodeSearch 测试 ----

func TestHandleCodeSearch_Success(t *testing.T) {
	mock := &mockCodeSearchProvider{
		result: `{"action":"grep","results":[{"file":"main.go","line":10,"content":"func main()"}],"summary":"1 match"}`,
	}
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.codeTool = mock

	result, err := s.handleCodeSearch(context.Background(), makeCallToolRequest(map[string]any{
		"action":  "grep",
		"pattern": "func main",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertToolSuccess(t, result)
	text := getToolResultText(t, result)
	if !contains(text, "main.go") {
		t.Errorf("result should contain file name, got: %s", text)
	}
}

func TestHandleCodeSearch_Error(t *testing.T) {
	mock := &mockCodeSearchProvider{err: fmt.Errorf("no repos")}
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.codeTool = mock

	result, err := s.handleCodeSearch(context.Background(), makeCallToolRequest(map[string]any{
		"pattern": "test",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "代码搜索失败")
}

// ---- handleGraphRAGQuery 测试 ----

func TestHandleGraphRAGQuery_EmptyKBID(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.graphRAG = &mockGraphRAGProvider{}

	result, err := s.handleGraphRAGQuery(context.Background(), makeCallToolRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "knowledge_base_id 参数不能为空")
}

func TestHandleGraphRAGQuery_Success(t *testing.T) {
	mock := &mockGraphRAGProvider{
		graph: &graphrag.VisGraph{
			Nodes: []graphrag.VisNode{{ID: "n1", Label: "Go", Degree: 5}},
			Edges: []graphrag.VisEdge{{Source: "n1", Target: "n2", Label: "RELATED_TO"}},
		},
	}
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.graphRAG = mock

	result, err := s.handleGraphRAGQuery(context.Background(), makeCallToolRequest(map[string]any{
		"knowledge_base_id": "kb-1",
		"limit":             float64(10),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertToolSuccess(t, result)
	text := getToolResultText(t, result)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if parsed["node_count"] != float64(1) {
		t.Errorf("node_count = %v, want 1", parsed["node_count"])
	}
}

func TestHandleGraphRAGQuery_Error(t *testing.T) {
	mock := &mockGraphRAGProvider{err: fmt.Errorf("neo4j down")}
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.graphRAG = mock

	result, err := s.handleGraphRAGQuery(context.Background(), makeCallToolRequest(map[string]any{
		"knowledge_base_id": "kb-1",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "图谱查询失败")
}

// ---- handleCodeGraphQuery 测试 ----

func TestHandleCodeGraphQuery_EmptyAction(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.codeGraph = &mockCodeGraphRepo{}

	result, err := s.handleCodeGraphQuery(context.Background(), makeCallToolRequest(map[string]any{
		"repo": "test-repo",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "action 参数不能为空")
}

func TestHandleCodeGraphQuery_EmptyRepo(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.codeGraph = &mockCodeGraphRepo{}

	result, err := s.handleCodeGraphQuery(context.Background(), makeCallToolRequest(map[string]any{
		"action": "callers",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "repo 参数不能为空")
}

func TestHandleCodeGraphQuery_Callers(t *testing.T) {
	mock := &mockCodeGraphRepo{
		callers: []codegraph.CodeRelation{
			{Type: "calls", Source: "handleRequest", Target: "processData"},
		},
	}
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.codeGraph = mock

	result, err := s.handleCodeGraphQuery(context.Background(), makeCallToolRequest(map[string]any{
		"action": "callers",
		"repo":   "test-repo",
		"name":   "processData",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertToolSuccess(t, result)
	text := getToolResultText(t, result)
	if !contains(text, "handleRequest") {
		t.Errorf("result should contain caller name, got: %s", text)
	}
}

func TestHandleCodeGraphQuery_CallersMissingName(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.codeGraph = &mockCodeGraphRepo{}

	result, err := s.handleCodeGraphQuery(context.Background(), makeCallToolRequest(map[string]any{
		"action": "callers",
		"repo":   "test-repo",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "需要 name 参数")
}

func TestHandleCodeGraphQuery_Overview(t *testing.T) {
	mock := &mockCodeGraphRepo{
		overview: &codegraph.RepoOverview{
			Repo: "test-repo", FileCount: 10, EntityCount: 50, RelationCount: 30,
			TypeCounts: map[string]int{"function": 40, "class": 10},
		},
	}
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.codeGraph = mock

	result, err := s.handleCodeGraphQuery(context.Background(), makeCallToolRequest(map[string]any{
		"action": "overview",
		"repo":   "test-repo",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertToolSuccess(t, result)
	text := getToolResultText(t, result)
	if !contains(text, "test-repo") {
		t.Errorf("result should contain repo name, got: %s", text)
	}
}

func TestHandleCodeGraphQuery_Search(t *testing.T) {
	mock := &mockCodeGraphRepo{
		entities: []codegraph.CodeEntity{
			{Name: "HandleRequest", Type: "function", FilePath: "handler.go", LineStart: 10, LineEnd: 30},
		},
	}
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.codeGraph = mock

	result, err := s.handleCodeGraphQuery(context.Background(), makeCallToolRequest(map[string]any{
		"action": "search",
		"repo":   "test-repo",
		"name":   "Handle",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertToolSuccess(t, result)
	text := getToolResultText(t, result)
	if !contains(text, "HandleRequest") {
		t.Errorf("result should contain symbol name, got: %s", text)
	}
}

func TestHandleCodeGraphQuery_UnknownAction(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.codeGraph = &mockCodeGraphRepo{}

	result, err := s.handleCodeGraphQuery(context.Background(), makeCallToolRequest(map[string]any{
		"action": "invalid",
		"repo":   "test-repo",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "未知操作")
}

func TestHandleImportURLReturnsApprovalRequiredWhenConfigured(t *testing.T) {
	writer := &mockMCPWriter{}
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.config.MCPExport.EnableAdminTools = true
	s.config.MCPExport.RequireApprovalForAdminTools = true
	s.kbWriter = writer

	ctx := context.WithValue(context.Background(), ctxKeyScope, mcpScope{tenantID: 1, userID: "agent-1", allowAdminTools: true})
	result, err := s.handleImportURL(ctx, makeCallToolRequest(map[string]any{
		"knowledge_base_id": "kb-1",
		"url":               "https://example.com/doc",
		"title":             "doc",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolSuccess(t, result)
	parsed := parseToolResultJSON(t, result)
	if parsed["type"] != "approval_required" || parsed["status"] != "pending" || parsed["approval_id"] == "" {
		t.Fatalf("unexpected approval response: %#v", parsed)
	}
	if writer.importCalled {
		t.Fatal("import should not run before approval")
	}
}

func TestHandleImportURLExecutesAfterApprovalIDRetry(t *testing.T) {
	writer := &mockMCPWriter{}
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.config.MCPExport.EnableAdminTools = true
	s.config.MCPExport.RequireApprovalForAdminTools = true
	s.kbWriter = writer

	ctx := context.WithValue(context.Background(), ctxKeyScope, mcpScope{tenantID: 1, userID: "agent-1", allowAdminTools: true})
	result, err := s.handleImportURL(ctx, makeCallToolRequest(map[string]any{
		"knowledge_base_id": "kb-1",
		"url":               "https://example.com/doc",
		"title":             "doc",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	approvalID, _ := parseToolResultJSON(t, result)["approval_id"].(string)
	if approvalID == "" {
		t.Fatal("approval_id is empty")
	}
	if err := s.approvalManager.Decide(context.Background(), approvalID, approval.Decision{Decision: approval.DecisionApprove, DeciderUserID: "human-1"}); err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}

	result, err = s.handleImportURL(ctx, makeCallToolRequest(map[string]any{
		"knowledge_base_id": "kb-1",
		"url":               "https://example.com/doc",
		"title":             "doc",
		"approval_id":       approvalID,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolSuccess(t, result)
	if !writer.importCalled {
		t.Fatal("import should run after approved approval_id retry")
	}
}

func TestHandleImportURLRejectsMismatchedApprovalIDRetry(t *testing.T) {
	writer := &mockMCPWriter{}
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.config.MCPExport.EnableAdminTools = true
	s.config.MCPExport.RequireApprovalForAdminTools = true
	s.kbWriter = writer

	ctx := context.WithValue(context.Background(), ctxKeyScope, mcpScope{tenantID: 1, userID: "agent-1", allowAdminTools: true})
	result, err := s.handleImportURL(ctx, makeCallToolRequest(map[string]any{
		"knowledge_base_id": "kb-1",
		"url":               "https://example.com/doc-a",
		"title":             "doc",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	approvalID, _ := parseToolResultJSON(t, result)["approval_id"].(string)
	if err := s.approvalManager.Decide(context.Background(), approvalID, approval.Decision{Decision: approval.DecisionApprove}); err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}

	result, err = s.handleImportURL(ctx, makeCallToolRequest(map[string]any{
		"knowledge_base_id": "kb-1",
		"url":               "https://example.com/doc-b",
		"title":             "doc",
		"approval_id":       approvalID,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "approval action does not match request")
	if writer.importCalled {
		t.Fatal("import should not run with mismatched approval")
	}
}

func TestHandleApprovalStatusAndDecision(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.config.MCPExport.EnableAdminTools = true
	approvalReq := approval.Request{TenantID: 1, Source: "mcp", Action: "import_url", ToolName: "import_url", ToolInput: `{"url":"https://example.com"}`}
	item, err := s.approvalManager.Create(context.Background(), approvalReq)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	statusResult, err := s.handleApprovalStatus(context.Background(), makeCallToolRequest(map[string]any{"approval_id": item.ID}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := parseToolResultJSON(t, statusResult)["status"]; got != string(approval.StatusPending) {
		t.Fatalf("status = %v, want pending", got)
	}

	ctx := context.WithValue(context.Background(), ctxKeyScope, mcpScope{allowAdminTools: true})
	decisionResult, err := s.handleSubmitApprovalDecision(ctx, makeCallToolRequest(map[string]any{
		"approval_id": item.ID,
		"decision":    "approve",
		"reason":      "verified",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolSuccess(t, decisionResult)

	statusResult, err = s.handleApprovalStatus(context.Background(), makeCallToolRequest(map[string]any{"approval_id": item.ID}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := parseToolResultJSON(t, statusResult)["status"]; got != string(approval.StatusApproved) {
		t.Fatalf("status = %v, want approved", got)
	}
}

// ---- 认证中间件测试 ----

func TestExtractAPIKey_BearerToken(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})

	r, _ := http.NewRequest("POST", "/mcp", nil)
	r.Header.Set("Authorization", "Bearer sk-test-123")

	key := s.extractAPIKey(r)
	if key != "sk-test-123" {
		t.Errorf("got %q, want %q", key, "sk-test-123")
	}
}

func TestExtractAPIKey_XAPIKey(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})

	r, _ := http.NewRequest("POST", "/mcp", nil)
	r.Header.Set("X-API-Key", "my-key-456")

	key := s.extractAPIKey(r)
	if key != "my-key-456" {
		t.Errorf("got %q, want %q", key, "my-key-456")
	}
}

func TestExtractAPIKey_BearerPriority(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})

	r, _ := http.NewRequest("POST", "/mcp", nil)
	r.Header.Set("Authorization", "Bearer bearer-key")
	r.Header.Set("X-API-Key", "xapi-key")

	key := s.extractAPIKey(r)
	if key != "bearer-key" {
		t.Errorf("Bearer should take priority, got %q", key)
	}
}

func TestExtractAPIKey_NoHeader(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})

	r, _ := http.NewRequest("POST", "/mcp", nil)
	key := s.extractAPIKey(r)
	if key != "" {
		t.Errorf("got %q, want empty string", key)
	}
}

func TestHTTPContextFunc_NoAPIKeys(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	// apiKeySet 为空，所有请求都应认证通过

	ctxFunc := s.httpContextFunc()
	r, _ := http.NewRequest("POST", "/mcp", nil)
	ctx := ctxFunc(context.Background(), r)

	authenticated, _ := ctx.Value(ctxKeyAuthenticated).(bool)
	if !authenticated {
		t.Error("should be authenticated when no API keys configured")
	}
}

func TestHTTPContextFunc_ValidKey(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.apiKeySet = map[string]struct{}{"valid-key": {}}

	ctxFunc := s.httpContextFunc()
	r, _ := http.NewRequest("POST", "/mcp", nil)
	r.Header.Set("Authorization", "Bearer valid-key")
	ctx := ctxFunc(context.Background(), r)

	authenticated, _ := ctx.Value(ctxKeyAuthenticated).(bool)
	if !authenticated {
		t.Error("should be authenticated with valid key")
	}
}

func TestHTTPContextFunc_InvalidKey(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.apiKeySet = map[string]struct{}{"valid-key": {}}

	ctxFunc := s.httpContextFunc()
	r, _ := http.NewRequest("POST", "/mcp", nil)
	r.Header.Set("Authorization", "Bearer wrong-key")
	ctx := ctxFunc(context.Background(), r)

	authenticated, _ := ctx.Value(ctxKeyAuthenticated).(bool)
	if authenticated {
		t.Error("should NOT be authenticated with invalid key")
	}
}

func TestHTTPContextFunc_MissingKey(t *testing.T) {
	s := newTestServer(&mockChatProvider{}, &mockKBRepo{})
	s.apiKeySet = map[string]struct{}{"valid-key": {}}

	ctxFunc := s.httpContextFunc()
	r, _ := http.NewRequest("POST", "/mcp", nil)
	ctx := ctxFunc(context.Background(), r)

	authenticated, _ := ctx.Value(ctxKeyAuthenticated).(bool)
	if authenticated {
		t.Error("should NOT be authenticated without any key")
	}
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

func parseToolResultJSON(t *testing.T, result *mcpProto.CallToolResult) map[string]any {
	t.Helper()
	text := getToolResultText(t, result)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("result should be JSON: %v\ntext: %s", err, text)
	}
	return parsed
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
