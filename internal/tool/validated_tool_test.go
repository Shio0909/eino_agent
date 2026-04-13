package tool

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// mockTool 用于测试的 mock 工具
type mockTool struct {
	called bool
	input  string
}

func (m *mockTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "test_tool",
		Desc: "A test tool",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "查询内容",
				Required: true,
			},
			"mode": {
				Type: schema.String,
				Desc: "模式",
				Enum: []string{"auto", "semantic", "exact"},
			},
			"count": {
				Type: schema.Integer,
				Desc: "数量",
			},
		}),
	}, nil
}

func (m *mockTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	m.called = true
	m.input = input
	return "ok", nil
}

var _ tool.InvokableTool = (*mockTool)(nil)

// --- 基础校验测试 ---

func TestValidatedTool_ValidInput(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	result, err := wrapped.InvokableRun(context.Background(), `{"query":"hello","mode":"auto"}`)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got: %s", result)
	}
	if !mock.called {
		t.Fatal("expected inner tool to be called")
	}
}

func TestValidatedTool_MissingRequired(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	_, err := wrapped.InvokableRun(context.Background(), `{"mode":"auto"}`)
	if err == nil {
		t.Fatal("expected error for missing required field")
	}
	if mock.called {
		t.Fatal("inner tool should not be called on validation failure")
	}
	assertContains(t, err.Error(), "query")
}

func TestValidatedTool_EmptyRequiredString(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	_, err := wrapped.InvokableRun(context.Background(), `{"query":"  "}`)
	if err == nil {
		t.Fatal("expected error for empty required string")
	}
	assertContains(t, err.Error(), "空字符串")
}

func TestValidatedTool_InvalidJSON(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	_, err := wrapped.InvokableRun(context.Background(), `not json at all`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	assertContains(t, err.Error(), "JSON 格式无效")
}

func TestValidatedTool_InvalidEnum(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	_, err := wrapped.InvokableRun(context.Background(), `{"query":"hello","mode":"invalid_mode"}`)
	if err == nil {
		t.Fatal("expected error for invalid enum value")
	}
	assertContains(t, err.Error(), "invalid_mode")
	assertContains(t, err.Error(), "auto")
}

func TestValidatedTool_WrongType(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	_, err := wrapped.InvokableRun(context.Background(), `{"query":123}`)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
	assertContains(t, err.Error(), "string")
}

func TestValidatedTool_IntegerCheck(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	// 浮点数不能作为整数
	_, err := wrapped.InvokableRun(context.Background(), `{"query":"hello","count":3.5}`)
	if err == nil {
		t.Fatal("expected error for float as integer")
	}
	assertContains(t, err.Error(), "整数")

	// 整数值可以（JSON 中 3 会被解析为 float64(3.0)，整除检查应通过）
	result, err := wrapped.InvokableRun(context.Background(), `{"query":"hello","count":3}`)
	if err != nil {
		t.Fatalf("expected no error for valid integer, got: %v", err)
	}
	if result != "ok" {
		t.Fatal("expected ok")
	}
}

func TestValidatedTool_InfoPassthrough(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	info, err := wrapped.Info(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if info.Name != "test_tool" {
		t.Fatalf("expected test_tool, got: %s", info.Name)
	}
}

// --- 模拟 LLM 常见参数错误 ---

func TestValidatedTool_LLMCommonMistakes(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "正常调用",
			input:   `{"query":"什么是 Kubernetes"}`,
			wantErr: false,
		},
		{
			name:      "LLM 发送空 JSON 对象",
			input:     `{}`,
			wantErr:   true,
			errSubstr: "query",
		},
		{
			name:    "LLM 只传必填字段",
			input:   `{"query":"Go 并发模型"}`,
			wantErr: false,
		},
		{
			name:      "LLM 传 null 给必填字段",
			input:     `{"query":null}`,
			wantErr:   true,
			errSubstr: "query",
		},
		{
			name:      "LLM 传数组给 string 字段",
			input:     `{"query":["a","b"]}`,
			wantErr:   true,
			errSubstr: "string",
		},
		{
			name:      "LLM 传对象给 string 字段",
			input:     `{"query":{"text":"hello"}}`,
			wantErr:   true,
			errSubstr: "string",
		},
		{
			name:      "LLM 把 boolean 传给 string",
			input:     `{"query":true}`,
			wantErr:   true,
			errSubstr: "string",
		},
		{
			name:    "LLM 传额外字段（应该忽略）",
			input:   `{"query":"test","extra_field":"should be ignored"}`,
			wantErr: false,
		},
		{
			name:    "合法 enum 值",
			input:   `{"query":"test","mode":"semantic"}`,
			wantErr: false,
		},
		{
			name:      "LLM 拼写错误 enum 值",
			input:     `{"query":"test","mode":"sematic"}`,
			wantErr:   true,
			errSubstr: "sematic",
		},
		{
			name:      "LLM 用大写 enum 值",
			input:     `{"query":"test","mode":"AUTO"}`,
			wantErr:   true,
			errSubstr: "AUTO",
		},
		{
			name:      "LLM 传负浮点给 integer",
			input:     `{"query":"test","count":-1.5}`,
			wantErr:   true,
			errSubstr: "整数",
		},
		{
			name:    "负整数是合法的",
			input:   `{"query":"test","count":-3}`,
			wantErr: false,
		},
		{
			name:    "零是合法整数",
			input:   `{"query":"test","count":0}`,
			wantErr: false,
		},
		{
			name:      "LLM 传字符串数字给 integer",
			input:     `{"query":"test","count":"5"}`,
			wantErr:   true,
			errSubstr: "integer",
		},
		{
			name:      "LLM 发送 markdown 代码块包裹的 JSON",
			input:     "```json\n{\"query\":\"test\"}\n```",
			wantErr:   true,
			errSubstr: "JSON 格式无效",
		},
		{
			name:      "LLM 发送纯自然语言",
			input:     "Please search for Kubernetes deployment best practices",
			wantErr:   true,
			errSubstr: "JSON 格式无效",
		},
		{
			name:      "LLM 发送单引号 JSON（Python 风格）",
			input:     `{'query': 'test'}`,
			wantErr:   true,
			errSubstr: "JSON 格式无效",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.called = false
			_, err := wrapped.InvokableRun(context.Background(), tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				assertContains(t, err.Error(), tt.errSubstr)
				if mock.called {
					t.Fatal("inner tool should not be called on validation failure")
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				if !mock.called {
					t.Fatal("inner tool should be called on valid input")
				}
			}
		})
	}
}

// --- 模拟真实工具 schema 的校验测试 ---

// 模拟 knowledge_search 工具的 schema
type mockKnowledgeSearchTool struct{ mockTool }

func (m *mockKnowledgeSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "knowledge_search",
		Desc: "在知识库中检索相关文档",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "要检索的问题或关键词",
				Required: true,
			},
			"mode": {
				Type: schema.String,
				Desc: "检索模式",
				Enum: []string{"auto", "semantic", "exact", "graph"},
			},
		}),
	}, nil
}

// 模拟 code_search 工具的 schema
type mockCodeSearchTool struct{ mockTool }

func (m *mockCodeSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "code_search",
		Desc: "在本地代码仓库中搜索代码",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type: schema.String,
				Desc: "操作类型：grep, find, read",
				Enum: []string{"grep", "find", "read"},
			},
			"query": {
				Type: schema.String,
				Desc: "描述要找什么",
			},
			"pattern": {
				Type: schema.String,
				Desc: "搜索模式",
			},
			"repo": {
				Type: schema.String,
				Desc: "仓库名",
			},
			"file_glob": {
				Type: schema.String,
				Desc: "文件匹配模式",
			},
			"path": {
				Type: schema.String,
				Desc: "文件路径",
			},
		}),
	}, nil
}

// 模拟 query_decompose 工具的 schema
type mockQueryDecomposeTool struct{ mockTool }

func (m *mockQueryDecomposeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "query_decompose",
		Desc: "将复杂问题拆解为子查询",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "需要分解的复杂问题",
				Required: true,
			},
		}),
	}, nil
}

func TestValidatedTool_RealToolSchemas(t *testing.T) {
	t.Run("knowledge_search schema", func(t *testing.T) {
		mock := &mockKnowledgeSearchTool{}
		wrapped := WrapWithValidation(mock)

		// 合法: 最小参数
		mock.called = false
		_, err := wrapped.InvokableRun(context.Background(), `{"query":"什么是 Docker 容器编排"}`)
		assertNoError(t, err)
		assertTrue(t, mock.called, "inner should be called")

		// 合法: 全部参数
		mock.called = false
		_, err = wrapped.InvokableRun(context.Background(), `{"query":"K8s vs Docker Swarm","mode":"semantic"}`)
		assertNoError(t, err)

		// 非法: 缺少 query
		mock.called = false
		_, err = wrapped.InvokableRun(context.Background(), `{"mode":"graph"}`)
		assertError(t, err, "query")

		// 非法: mode 不在枚举中
		mock.called = false
		_, err = wrapped.InvokableRun(context.Background(), `{"query":"test","mode":"hybrid"}`)
		assertError(t, err, "hybrid")
	})

	t.Run("code_search schema", func(t *testing.T) {
		mock := &mockCodeSearchTool{}
		wrapped := WrapWithValidation(mock)

		// 合法: grep 搜索
		mock.called = false
		_, err := wrapped.InvokableRun(context.Background(),
			`{"action":"grep","query":"查找 main 函数","pattern":"func main","file_glob":"*.go"}`)
		assertNoError(t, err)
		assertTrue(t, mock.called, "inner should be called")

		// 合法: read 文件
		mock.called = false
		_, err = wrapped.InvokableRun(context.Background(),
			`{"action":"read","path":"main.go","repo":"my-project"}`)
		assertNoError(t, err)

		// 合法: 没有必填字段，空 JSON 也行
		mock.called = false
		_, err = wrapped.InvokableRun(context.Background(), `{}`)
		assertNoError(t, err)

		// 非法: action 不在枚举中
		mock.called = false
		_, err = wrapped.InvokableRun(context.Background(),
			`{"action":"search","pattern":"test"}`)
		assertError(t, err, "search")
	})

	t.Run("query_decompose schema", func(t *testing.T) {
		mock := &mockQueryDecomposeTool{}
		wrapped := WrapWithValidation(mock)

		// 合法: 复杂问题
		mock.called = false
		_, err := wrapped.InvokableRun(context.Background(),
			`{"query":"比较 PostgreSQL 和 MySQL 在事务隔离级别、JSON 支持和扩展性方面的差异"}`)
		assertNoError(t, err)

		// 非法: 缺少 query
		mock.called = false
		_, err = wrapped.InvokableRun(context.Background(), `{}`)
		assertError(t, err, "query")
	})
}

// --- 边界场景测试 ---

func TestValidatedTool_EdgeCases(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	t.Run("空字符串输入", func(t *testing.T) {
		_, err := wrapped.InvokableRun(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty input")
		}
	})

	t.Run("Unicode 中文 query 正常通过", func(t *testing.T) {
		mock.called = false
		_, err := wrapped.InvokableRun(context.Background(), `{"query":"深度学习与强化学习有什么区别？🤔"}`)
		assertNoError(t, err)
		assertTrue(t, mock.called, "inner should be called")
	})

	t.Run("超长 query 正常通过（不做长度限制）", func(t *testing.T) {
		mock.called = false
		longQuery := strings.Repeat("x", 10000)
		_, err := wrapped.InvokableRun(context.Background(), `{"query":"`+longQuery+`"}`)
		assertNoError(t, err)
		assertTrue(t, mock.called, "inner should be called")
	})

	t.Run("嵌套 JSON 对象作为 string 字段值", func(t *testing.T) {
		_, err := wrapped.InvokableRun(context.Background(), `{"query":{"nested":"value"}}`)
		assertError(t, err, "string")
	})

	t.Run("Info 缓存生效", func(t *testing.T) {
		vt := WrapWithValidation(mock)
		info1, _ := vt.Info(context.Background())
		info2, _ := vt.Info(context.Background())
		if info1 != info2 {
			t.Fatal("Info should be cached and return same pointer")
		}
	})
}

// --- 无 schema 工具的容错测试 ---

type noSchemaTool struct{ mockTool }

func (m *noSchemaTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "no_schema_tool",
		Desc: "A tool with no parameter schema",
		// ParamsOneOf 为 nil
	}, nil
}

func TestValidatedTool_NoSchema(t *testing.T) {
	mock := &noSchemaTool{}
	wrapped := WrapWithValidation(mock)

	// 没有 schema 的工具应该直接放行
	mock.called = false
	_, err := wrapped.InvokableRun(context.Background(), `anything goes here`)
	assertNoError(t, err)
	assertTrue(t, mock.called, "inner should be called when no schema")
}

// --- Helper functions ---

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected string to contain %q, got: %s", substr, s)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func assertError(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Errorf("expected error to contain %q, got: %v", substr, err)
	}
}

func assertTrue(t *testing.T, b bool, msg string) {
	t.Helper()
	if !b {
		t.Fatal(msg)
	}
}
