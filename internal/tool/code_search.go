package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// CodeSearchTool 代码仓库检索工具 (MVP)
// 采用类 Claude Code 的 agentic retrieval 策略：grep/find/read，而非向量 RAG
type CodeSearchTool struct {
	reposDir  string // 仓库根目录，如 data/test_repos
	callCount int    // 调用计数，防止 agent 无限循环
	maxCalls  int    // 最大调用次数
}

type codeSearchInput struct {
	Query    string `json:"query"`
	Pattern  string `json:"pattern,omitempty"`
	Repo     string `json:"repo,omitempty"`
	FileGlob string `json:"file_glob,omitempty"`
	Action   string `json:"action,omitempty"`
	Path     string `json:"path,omitempty"`
}

type codeSearchOutput struct {
	Action     string             `json:"action"`
	Results    []codeSearchResult `json:"results"`
	Summary    string             `json:"summary"`
	Retryable  bool               `json:"retryable,omitempty"`
	ErrorCode  string             `json:"error_code,omitempty"`
	Suggestion string             `json:"suggestion,omitempty"`
}

type codeSearchResult struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Content string `json:"content"`
}

func NewCodeSearchTool(reposDir string) *CodeSearchTool {
	return &CodeSearchTool{reposDir: reposDir, maxCalls: 20}
}

func (t *CodeSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "code_search",
		Desc: `在本地代码仓库中搜索代码。支持三种操作：
- grep: 在代码中搜索匹配模式的内容（正则或字面量）
- find: 按文件名模式查找文件
- read: 读取指定文件内容（支持行范围）
适用于：查找函数定义、理解代码结构、追踪调用关系、阅读实现细节。`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type: schema.String,
				Desc: "操作类型：grep(搜索内容), find(查找文件), read(读取文件)。默认 grep",
			},
			"query": {
				Type: schema.String,
				Desc: "自然语言描述你要找什么（用于日志，非必须）",
			},
			"pattern": {
				Type: schema.String,
				Desc: "grep: 搜索的正则/字面量模式; find: 文件名匹配模式(如 *.py, *test*)",
			},
			"repo": {
				Type: schema.String,
				Desc: "指定仓库目录名。当前项目可用 eino_agent 或留空；测试仓库如 deer-flow、pydantic-ai 必须显式填写对应 repo。",
			},
			"file_glob": {
				Type: schema.String,
				Desc: "grep 时限定文件类型，如 *.py, *.go, *.ts",
			},
			"path": {
				Type: schema.String,
				Desc: "read 操作时的文件路径（相对于仓库根目录）",
			},
		}),
	}, nil
}

func (t *CodeSearchTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	// 强制调用次数限制，防止 agent 上下文爆炸
	t.callCount++
	if t.callCount > t.maxCalls {
		return `{"action":"limit","results":[],"summary":"已达到 code_search 调用次数上限，请基于已有信息作答"}`, nil
	}

	var params codeSearchInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	action := strings.ToLower(strings.TrimSpace(params.Action))
	if action == "" {
		action = "grep"
	}

	start := time.Now()
	var result codeSearchOutput
	var err error

	switch action {
	case "grep":
		result, err = t.doGrep(ctx, params)
	case "find":
		result, err = t.doFind(ctx, params)
	case "read":
		result, err = t.doRead(ctx, params)
	default:
		return "", fmt.Errorf("unknown action: %s (supported: grep, find, read)", action)
	}

	log.Printf("[Timing][CodeSearch] action=%s duration_ms=%d pattern=%q repo=%q",
		action, time.Since(start).Milliseconds(), params.Pattern, params.Repo)

	if err != nil {
		return "", err
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

func (t *CodeSearchTool) doGrep(ctx context.Context, params codeSearchInput) (codeSearchOutput, error) {
	if params.Pattern == "" {
		return codeSearchOutput{}, fmt.Errorf("pattern is required for grep action")
	}

	searchDir := t.resolveDir(params.Repo)
	if _, err := os.Stat(searchDir); os.IsNotExist(err) {
		return codeSearchOutput{}, fmt.Errorf("directory not found: %s", searchDir)
	}

	// 构建 grep 命令，优先用 ripgrep (rg)，降级到系统 grep
	args := t.buildGrepArgs(params)

	var cmd *exec.Cmd
	if rgPath, err := exec.LookPath("rg"); err == nil {
		cmd = exec.CommandContext(ctx, rgPath, args...)
	} else {
		// 降级：Windows 用 findstr，Unix 用 grep
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "findstr", "/s", "/n", "/i", params.Pattern, t.buildFindstrGlob(params, searchDir))
		} else {
			grepArgs := []string{"-rn", "--include=" + params.FileGlob, params.Pattern, "."}
			if params.FileGlob == "" {
				grepArgs = []string{"-rn", params.Pattern, "."}
			}
			cmd = exec.CommandContext(ctx, "grep", grepArgs...)
		}
	}
	cmd.Dir = searchDir

	output, err := cmd.Output()
	// grep 返回 exit 1 表示没找到，不算错误
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return codeSearchOutput{
				Action:  "grep",
				Results: nil,
				Summary: fmt.Sprintf("No matches found for pattern '%s'", params.Pattern),
			}, nil
		}
		// 其他错误也尝试返回已有输出
		if len(output) == 0 {
			return codeSearchOutput{
				Action:  "grep",
				Results: nil,
				Summary: fmt.Sprintf("Search completed with no results for '%s'", params.Pattern),
			}, nil
		}
	}

	results := t.parseGrepOutput(string(output), searchDir, 15) // 最多15条
	// 截断总输出防止上下文爆炸
	const maxGrepChars = 3000
	totalChars := 0
	var trimmedResults []codeSearchResult
	for _, r := range results {
		totalChars += len(r.File) + len(r.Content) + 20
		if totalChars > maxGrepChars && len(trimmedResults) > 0 {
			break
		}
		trimmedResults = append(trimmedResults, r)
	}
	return codeSearchOutput{
		Action:  "grep",
		Results: trimmedResults,
		Summary: fmt.Sprintf("Found %d matches for '%s' (showing %d)", len(results), params.Pattern, len(trimmedResults)),
	}, nil
}

func (t *CodeSearchTool) doFind(ctx context.Context, params codeSearchInput) (codeSearchOutput, error) {
	if params.Pattern == "" {
		return codeSearchOutput{}, fmt.Errorf("pattern is required for find action")
	}

	searchDir := t.resolveDir(params.Repo)

	var files []string
	maxFiles := 30

	err := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过权限错误等
		}
		if len(files) >= maxFiles {
			return filepath.SkipAll
		}
		// 跳过 .git 等隐藏目录
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}
		if info.IsDir() || strings.Contains(path, "node_modules") || strings.Contains(path, "__pycache__") {
			if info.IsDir() && (info.Name() == "node_modules" || info.Name() == "__pycache__" || info.Name() == ".git") {
				return filepath.SkipDir
			}
			return nil
		}

		matched, _ := filepath.Match(params.Pattern, info.Name())
		if matched || strings.Contains(strings.ToLower(info.Name()), strings.ToLower(params.Pattern)) {
			rel, _ := filepath.Rel(searchDir, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})

	if err != nil {
		return codeSearchOutput{}, fmt.Errorf("walk directory: %w", err)
	}

	results := make([]codeSearchResult, len(files))
	for i, f := range files {
		results[i] = codeSearchResult{File: f, Content: ""}
	}

	return codeSearchOutput{
		Action:  "find",
		Results: results,
		Summary: fmt.Sprintf("Found %d files matching '%s'", len(files), params.Pattern),
	}, nil
}

func (t *CodeSearchTool) doRead(ctx context.Context, params codeSearchInput) (codeSearchOutput, error) {
	if params.Path == "" {
		return codeSearchOutput{}, fmt.Errorf("path is required for read action")
	}

	cleanPath := t.normalizeReadPath(params.Repo, params.Path)
	searchDir := t.resolveDir(params.Repo)
	fullPath := filepath.Join(searchDir, filepath.FromSlash(cleanPath))

	absPath, _ := filepath.Abs(fullPath)
	absSearchDir, _ := filepath.Abs(searchDir)
	if !isPathInside(absPath, absSearchDir) {
		return codeSearchOutput{}, fmt.Errorf("path traversal not allowed")
	}

	// 如果是目录，列出内容而不是尝试读取
	info, statErr := os.Stat(fullPath)
	if statErr == nil && info.IsDir() {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return codeSearchOutput{}, fmt.Errorf("list directory: %w", err)
		}
		var results []codeSearchResult
		for _, e := range entries {
			if len(results) >= 50 {
				break
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			if e.IsDir() {
				name += "/"
			}
			results = append(results, codeSearchResult{File: name})
		}
		return codeSearchOutput{
			Action:  "read",
			Results: results,
			Summary: fmt.Sprintf("Listed directory: %s (%d entries)", cleanPath, len(results)),
		}, nil
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return codeSearchOutput{
			Action:     "read",
			Results:    nil,
			Summary:    fmt.Sprintf("File not found or unreadable: %s", cleanPath),
			Retryable:  true,
			ErrorCode:  "path_not_found",
			Suggestion: "Use action=find or grep to locate the file, then call read with a path relative to the repo root.",
		}, nil
	}

	content := string(data)
	const maxFileChars = 2000
	if utf8.RuneCountInString(content) > maxFileChars {
		runes := []rune(content)
		content = string(runes[:maxFileChars]) + "\n... (truncated, total " + fmt.Sprintf("%d", len(runes)) + " chars)"
	}

	return codeSearchOutput{
		Action: "read",
		Results: []codeSearchResult{
			{File: cleanPath, Content: content},
		},
		Summary: fmt.Sprintf("Read file: %s (%d bytes)", cleanPath, len(data)),
	}, nil
}

// resolveDir 解析搜索目录
func (t *CodeSearchTool) resolveDir(repo string) string {
	if repo == "" {
		if root, ok := workspaceRoot(); ok && isPathInsideAbs(t.reposDir, root) {
			return root
		}
		return t.reposDir
	}

	cleanRepo := filepath.Clean(filepath.FromSlash(strings.TrimSpace(repo)))
	if filepath.IsAbs(cleanRepo) {
		return cleanRepo
	}

	if dir, ok := t.resolveNamedRepo(cleanRepo); ok {
		return dir
	}

	return filepath.Join(t.reposDir, cleanRepo)
}

func (t *CodeSearchTool) resolveNamedRepo(repo string) (string, bool) {
	repoName := filepath.Base(repo)
	if repoName == "." || repoName == string(filepath.Separator) {
		return "", false
	}

	cwd, err := os.Getwd()
	if err == nil {
		if dir, ok := findAncestorNamed(cwd, repoName); ok {
			return dir, true
		}
	}

	reposDirAbs, err := filepath.Abs(t.reposDir)
	if err == nil {
		if dir, ok := findAncestorNamed(reposDirAbs, repoName); ok {
			return dir, true
		}
	}

	return "", false
}

func workspaceRoot() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	return findGitRoot(cwd)
}

func findGitRoot(startDir string) (string, bool) {
	dir := filepath.Clean(startDir)
	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func isPathInsideAbs(path, root string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	return isPathInside(absPath, absRoot)
}

func findAncestorNamed(startDir, name string) (string, bool) {
	dir := filepath.Clean(startDir)
	for {
		if filepath.Base(dir) == name {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func (t *CodeSearchTool) normalizeReadPath(repo, inputPath string) string {
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(inputPath))))
	cleaned = strings.TrimPrefix(cleaned, "./")

	if repo != "" {
		repoName := strings.Trim(strings.TrimSpace(repo), `/\\`)
		parts := strings.Split(cleaned, "/")
		for i, part := range parts {
			if part == repoName {
				return strings.Join(parts[i+1:], "/")
			}
		}
	}

	reposDir := filepath.ToSlash(filepath.Clean(filepath.FromSlash(t.reposDir)))
	reposDir = strings.TrimPrefix(reposDir, "./")
	return strings.TrimPrefix(cleaned, reposDir+"/")
}

func isPathInside(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// buildGrepArgs 构建 ripgrep 参数
func (t *CodeSearchTool) buildGrepArgs(params codeSearchInput) []string {
	args := []string{
		"--no-heading",
		"--line-number",
		"--max-count=5",     // 每个文件最多5个匹配
		"--max-filesize=1M", // 跳过大文件
		"-i",                // 忽略大小写
	}

	// 排除常见无用目录
	args = append(args,
		"--glob=!.git",
		"--glob=!node_modules",
		"--glob=!__pycache__",
		"--glob=!*.min.js",
		"--glob=!*.map",
		"--glob=!dist/",
		"--glob=!build/",
		"--glob=!*.lock",
		"--glob=!package-lock.json",
		"--glob=!yarn.lock",
	)

	if params.FileGlob != "" {
		args = append(args, "--glob="+params.FileGlob)
	}

	args = append(args, params.Pattern)
	return args
}

// buildFindstrGlob Windows findstr 的文件模式
func (t *CodeSearchTool) buildFindstrGlob(params codeSearchInput, dir string) string {
	glob := "*.*"
	if params.FileGlob != "" {
		glob = params.FileGlob
	}
	return filepath.Join(dir, glob)
}

// parseGrepOutput 解析 ripgrep/grep 输出
func (t *CodeSearchTool) parseGrepOutput(output, baseDir string, maxResults int) []codeSearchResult {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var results []codeSearchResult

	for _, line := range lines {
		if len(results) >= maxResults {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// rg 格式: file:line:content
		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 3 {
			lineNum := 0
			fmt.Sscanf(parts[1], "%d", &lineNum)
			file := filepath.ToSlash(parts[0])
			content := strings.TrimSpace(parts[2])
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			results = append(results, codeSearchResult{
				File:    file,
				Line:    lineNum,
				Content: content,
			})
		} else if len(parts) >= 2 {
			results = append(results, codeSearchResult{
				File:    filepath.ToSlash(parts[0]),
				Content: strings.TrimSpace(parts[1]),
			})
		}
	}
	return results
}

// ResetCallCount 重置调用计数（每次请求开始时调用）
func (t *CodeSearchTool) ResetCallCount() {
	t.callCount = 0
}

var _ tool.InvokableTool = (*CodeSearchTool)(nil)
