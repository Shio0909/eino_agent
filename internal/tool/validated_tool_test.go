package tool

import (
	"context"
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
	t.Logf("got expected error: %v", err)
}

func TestValidatedTool_EmptyRequiredString(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	_, err := wrapped.InvokableRun(context.Background(), `{"query":"  "}`)
	if err == nil {
		t.Fatal("expected error for empty required string")
	}
	t.Logf("got expected error: %v", err)
}

func TestValidatedTool_InvalidJSON(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	_, err := wrapped.InvokableRun(context.Background(), `not json at all`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	t.Logf("got expected error: %v", err)
}

func TestValidatedTool_InvalidEnum(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	_, err := wrapped.InvokableRun(context.Background(), `{"query":"hello","mode":"invalid_mode"}`)
	if err == nil {
		t.Fatal("expected error for invalid enum value")
	}
	t.Logf("got expected error: %v", err)
}

func TestValidatedTool_WrongType(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	_, err := wrapped.InvokableRun(context.Background(), `{"query":123}`)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
	t.Logf("got expected error: %v", err)
}

func TestValidatedTool_IntegerCheck(t *testing.T) {
	mock := &mockTool{}
	wrapped := WrapWithValidation(mock)

	// 浮点数不能作为整数
	_, err := wrapped.InvokableRun(context.Background(), `{"query":"hello","count":3.5}`)
	if err == nil {
		t.Fatal("expected error for float as integer")
	}
	t.Logf("got expected error: %v", err)

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
