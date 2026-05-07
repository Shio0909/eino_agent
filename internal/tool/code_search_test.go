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
	cs.maxCalls = 20
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
		content := result.Results[0].Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		t.Logf("Content preview: %s", content)
	})

	t.Run("read_with_repo_prefix", func(t *testing.T) {
		out, err := cs.InvokableRun(ctx, `{"action":"read","path":"deer-flow/README.md","repo":"deer-flow"}`)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		var result codeSearchOutput
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(result.Results) == 0 || result.Results[0].File != "README.md" || result.Results[0].Content == "" {
			t.Fatalf("expected normalized README.md result, got %+v", result.Results)
		}
	})

	t.Run("read_with_repos_dir_prefix", func(t *testing.T) {
		out, err := cs.InvokableRun(ctx, `{"action":"read","path":"data/test_repos/deer-flow/README.md","repo":"deer-flow"}`)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		var result codeSearchOutput
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(result.Results) == 0 || result.Results[0].File != "README.md" || result.Results[0].Content == "" {
			t.Fatalf("expected normalized README.md result, got %+v", result.Results)
		}
	})

	t.Run("read_missing_file_retryable", func(t *testing.T) {
		out, err := cs.InvokableRun(ctx, `{"action":"read","path":"missing/nope.py","repo":"deer-flow"}`)
		if err != nil {
			t.Fatalf("expected retryable result, got error: %v", err)
		}
		var result codeSearchOutput
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !result.Retryable || result.ErrorCode != "path_not_found" {
			t.Fatalf("expected retryable path_not_found, got %+v", result)
		}
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

func TestCodeSearchTool_CurrentProjectScope(t *testing.T) {
	cs := NewCodeSearchTool(filepath.Join("..", "..", "data", "test_repos"))
	ctx := context.Background()

	t.Run("default_repo_searches_workspace", func(t *testing.T) {
		out, err := cs.InvokableRun(ctx, `{"action":"grep","pattern":"normalizeReadPath|path_not_found|isPathInside","file_glob":"*.go"}`)
		if err != nil {
			t.Fatalf("grep failed: %v", err)
		}
		var result codeSearchOutput
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(result.Results) == 0 {
			t.Fatalf("expected current project matches, got %+v", result)
		}
	})

	t.Run("repo_name_resolves_workspace", func(t *testing.T) {
		out, err := cs.InvokableRun(ctx, `{"action":"read","repo":"eino_agent","path":"internal/tool/code_search.go"}`)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		var result codeSearchOutput
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(result.Results) == 0 || result.Results[0].File != "internal/tool/code_search.go" || result.Results[0].Content == "" {
			t.Fatalf("expected current project file, got %+v", result.Results)
		}
	})
}
