package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCodeSearchTool_DeerFlow(t *testing.T) {
	// 检查 deer-flow 仓库是否存在
	reposDir := filepath.Join("..", "..", "data", "test_repos")
	deerFlowDir := filepath.Join(reposDir, "deer-flow")
	if _, err := os.Stat(deerFlowDir); os.IsNotExist(err) {
		t.Skip("deer-flow repo not found, run: git clone https://github.com/bytedance/deer-flow.git data/test_repos/deer-flow")
	}

	cs := NewCodeSearchTool(reposDir)
	ctx := context.Background()

	// Test 1: find Python files
	t.Run("find_python_files", func(t *testing.T) {
		out, err := cs.InvokableRun(ctx, `{"action":"find","pattern":"*.py","repo":"deer-flow"}`)
		if err != nil {
			t.Fatalf("find failed: %v", err)
		}
		var result codeSearchOutput
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		t.Logf("Summary: %s", result.Summary)
		if len(result.Results) == 0 {
			t.Error("expected to find Python files")
		}
		for i, r := range result.Results {
			if i >= 10 {
				break
			}
			t.Logf("  %s", r.File)
		}
	})

	// Test 2: grep for class definitions
	t.Run("grep_class_agent", func(t *testing.T) {
		out, err := cs.InvokableRun(ctx, `{"action":"grep","pattern":"class.*Agent","file_glob":"*.py","repo":"deer-flow"}`)
		if err != nil {
			t.Fatalf("grep failed: %v", err)
		}
		var result codeSearchOutput
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		t.Logf("Summary: %s", result.Summary)
		for _, r := range result.Results {
			t.Logf("  %s:%d -> %s", r.File, r.Line, r.Content)
		}
	})

	// Test 3: grep for import statements
	t.Run("grep_imports", func(t *testing.T) {
		out, err := cs.InvokableRun(ctx, `{"action":"grep","pattern":"from langchain","file_glob":"*.py","repo":"deer-flow"}`)
		if err != nil {
			t.Fatalf("grep failed: %v", err)
		}
		var result codeSearchOutput
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		t.Logf("Summary: %s", result.Summary)
		for i, r := range result.Results {
			if i >= 8 {
				break
			}
			t.Logf("  %s:%d -> %s", r.File, r.Line, r.Content)
		}
	})

	// Test 4: read a specific file
	t.Run("read_readme", func(t *testing.T) {
		out, err := cs.InvokableRun(ctx, `{"action":"read","path":"README.md","repo":"deer-flow"}`)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		var result codeSearchOutput
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		t.Logf("Summary: %s", result.Summary)
		if len(result.Results) == 0 || result.Results[0].Content == "" {
			t.Error("expected non-empty file content")
		}
		// 只打前200字符
		content := result.Results[0].Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		t.Logf("Content preview: %s", content)
	})

	// Test 5: grep for function definitions
	t.Run("grep_def_search", func(t *testing.T) {
		out, err := cs.InvokableRun(ctx, `{"action":"grep","pattern":"def search","file_glob":"*.py","repo":"deer-flow"}`)
		if err != nil {
			t.Fatalf("grep failed: %v", err)
		}
		var result codeSearchOutput
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		t.Logf("Summary: %s", result.Summary)
		for _, r := range result.Results {
			t.Logf("  %s:%d -> %s", r.File, r.Line, r.Content)
		}
	})

	// Test 6: path traversal protection
	t.Run("path_traversal_blocked", func(t *testing.T) {
		_, err := cs.InvokableRun(ctx, `{"action":"read","path":"../../configs/config.yaml","repo":"deer-flow"}`)
		if err == nil {
			t.Error("expected path traversal to be blocked")
		} else {
			t.Logf("Correctly blocked: %v", err)
		}
	})
}
