package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// RepoManagerTool 代码仓库管理工具
// 提供 clone/pull/list/status 操作，管理本地代码仓库
type RepoManagerTool struct {
	reposDir string // 仓库根目录，如 data/test_repos
}

type repoManagerInput struct {
	Action string `json:"action"`           // clone, pull, list, status
	URL    string `json:"url,omitempty"`     // clone 时的仓库 URL
	Repo   string `json:"repo,omitempty"`    // pull/status 时的仓库名
	Branch string `json:"branch,omitempty"`  // clone 时指定分支
}

type repoManagerOutput struct {
	Action  string     `json:"action"`
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Repos   []repoInfo `json:"repos,omitempty"`
}

type repoInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path,omitempty"`
	Branch     string `json:"branch,omitempty"`
	LastCommit string `json:"last_commit,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
	Indexed    bool   `json:"indexed,omitempty"`
}

func NewRepoManagerTool(reposDir string) *RepoManagerTool {
	return &RepoManagerTool{reposDir: reposDir}
}

func (t *RepoManagerTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "repo_manager",
		Desc: `管理本地代码仓库。支持以下操作：
- clone: 从远程 URL 克隆仓库到本地（自动浅克隆加速）
- pull: 拉取已有仓库的最新代码
- list: 列出所有本地已有的代码仓库
- status: 查看指定仓库的详细状态（分支、最近提交等）
在使用 code_search 或 code_graph 之前，先确保目标仓库已 clone 到本地。`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:     schema.String,
				Desc:     "操作类型: clone(克隆仓库), pull(更新仓库), list(列出仓库), status(仓库状态)",
				Required: true,
			},
			"url": {
				Type: schema.String,
				Desc: "仓库 URL（clone 时必填），如 https://github.com/bytedance/deer-flow.git",
			},
			"repo": {
				Type: schema.String,
				Desc: "仓库名称（pull/status 时使用），如 deer-flow",
			},
			"branch": {
				Type: schema.String,
				Desc: "指定分支（clone 时可选），默认 main/master",
			},
		}),
	}, nil
}

func (t *RepoManagerTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params repoManagerInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("parse params: %w", err)
	}

	var output repoManagerOutput
	var err error

	switch strings.ToLower(params.Action) {
	case "clone":
		output, err = t.doClone(ctx, params)
	case "pull":
		output, err = t.doPull(ctx, params)
	case "list":
		output, err = t.doList(ctx)
	case "status":
		output, err = t.doStatus(ctx, params)
	default:
		return "", fmt.Errorf("unknown action: %s (use clone/pull/list/status)", params.Action)
	}

	if err != nil {
		output = repoManagerOutput{
			Action:  params.Action,
			Success: false,
			Message: err.Error(),
		}
	}

	data, _ := json.Marshal(output)
	return string(data), nil
}

// doClone 克隆远程仓库
func (t *RepoManagerTool) doClone(ctx context.Context, params repoManagerInput) (repoManagerOutput, error) {
	if params.URL == "" {
		return repoManagerOutput{}, fmt.Errorf("url is required for clone action")
	}

	// 从 URL 提取仓库名
	repoName := extractRepoName(params.URL)
	if repoName == "" {
		return repoManagerOutput{}, fmt.Errorf("cannot extract repo name from URL: %s", params.URL)
	}

	repoPath := filepath.Join(t.reposDir, repoName)

	// 检查是否已存在
	if _, err := os.Stat(repoPath); err == nil {
		log.Printf("[repo_manager] repo already exists: %s, checking for updates", repoName)
		return t.doPull(ctx, repoManagerInput{Repo: repoName})
	}

	// 确保 reposDir 存在
	if err := os.MkdirAll(t.reposDir, 0755); err != nil {
		return repoManagerOutput{}, fmt.Errorf("create repos dir: %w", err)
	}

	// 构建 git clone 命令
	args := []string{"clone", "--depth=1"}
	if params.Branch != "" {
		args = append(args, "--branch", params.Branch)
	}
	args = append(args, params.URL, repoPath)

	log.Printf("[repo_manager] cloning %s → %s", params.URL, repoPath)
	start := time.Now()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	if err != nil {
		return repoManagerOutput{}, fmt.Errorf("git clone failed (%v): %s", err, string(output))
	}

	log.Printf("[repo_manager] cloned %s in %s", repoName, elapsed)

	return repoManagerOutput{
		Action:  "clone",
		Success: true,
		Message: fmt.Sprintf("Successfully cloned %s to %s in %s", repoName, repoPath, elapsed.Round(time.Millisecond)),
	}, nil
}

// doPull 更新已有仓库
func (t *RepoManagerTool) doPull(ctx context.Context, params repoManagerInput) (repoManagerOutput, error) {
	repoName := params.Repo
	if repoName == "" {
		return repoManagerOutput{}, fmt.Errorf("repo name is required for pull action")
	}

	repoPath := filepath.Join(t.reposDir, repoName)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return repoManagerOutput{}, fmt.Errorf("repo %s not found locally, use clone first", repoName)
	}

	// 先 fetch 检查是否有更新
	cmd := exec.CommandContext(ctx, "git", "fetch", "--depth=1")
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return repoManagerOutput{}, fmt.Errorf("git fetch failed: %s", string(out))
	}

	// 检查本地和远程差异
	cmd = exec.CommandContext(ctx, "git", "rev-list", "--count", "HEAD..FETCH_HEAD")
	cmd.Dir = repoPath
	countOut, err := cmd.Output()
	if err != nil {
		// 如果失败，直接 pull
		countOut = []byte("1")
	}
	behindCount := strings.TrimSpace(string(countOut))

	if behindCount == "0" {
		return repoManagerOutput{
			Action:  "pull",
			Success: true,
			Message: fmt.Sprintf("Repo %s is already up to date", repoName),
		}, nil
	}

	// 执行 pull
	cmd = exec.CommandContext(ctx, "git", "pull", "--ff-only")
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// pull 失败时尝试 reset
		cmd = exec.CommandContext(ctx, "git", "reset", "--hard", "FETCH_HEAD")
		cmd.Dir = repoPath
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			return repoManagerOutput{}, fmt.Errorf("git pull failed (%s), reset also failed: %s", string(output), string(out2))
		}
	}

	return repoManagerOutput{
		Action:  "pull",
		Success: true,
		Message: fmt.Sprintf("Updated %s (%s commits behind → pulled)", repoName, behindCount),
	}, nil
}

// doList 列出所有本地仓库
func (t *RepoManagerTool) doList(ctx context.Context) (repoManagerOutput, error) {
	entries, err := os.ReadDir(t.reposDir)
	if err != nil {
		if os.IsNotExist(err) {
			return repoManagerOutput{
				Action:  "list",
				Success: true,
				Message: "No repositories found (repos directory does not exist)",
			}, nil
		}
		return repoManagerOutput{}, fmt.Errorf("read repos dir: %w", err)
	}

	var repos []repoInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		gitDir := filepath.Join(t.reposDir, name, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			continue // 不是 git 仓库
		}

		info := repoInfo{
			Name: name,
			Path: filepath.Join(t.reposDir, name),
		}

		// 获取当前分支
		cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
		cmd.Dir = info.Path
		if out, err := cmd.Output(); err == nil {
			info.Branch = strings.TrimSpace(string(out))
		}

		// 获取最近 commit
		cmd = exec.CommandContext(ctx, "git", "log", "-1", "--format=%h %s", "--no-walk")
		cmd.Dir = info.Path
		if out, err := cmd.Output(); err == nil {
			info.LastCommit = strings.TrimSpace(string(out))
		}

		// 获取最近更新时间
		cmd = exec.CommandContext(ctx, "git", "log", "-1", "--format=%ci", "--no-walk")
		cmd.Dir = info.Path
		if out, err := cmd.Output(); err == nil {
			info.UpdatedAt = strings.TrimSpace(string(out))
		}

		repos = append(repos, info)
	}

	msg := fmt.Sprintf("Found %d repositories", len(repos))
	return repoManagerOutput{
		Action:  "list",
		Success: true,
		Message: msg,
		Repos:   repos,
	}, nil
}

// doStatus 查看仓库状态
func (t *RepoManagerTool) doStatus(ctx context.Context, params repoManagerInput) (repoManagerOutput, error) {
	repoName := params.Repo
	if repoName == "" {
		return repoManagerOutput{}, fmt.Errorf("repo name is required for status action")
	}

	repoPath := filepath.Join(t.reposDir, repoName)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return repoManagerOutput{}, fmt.Errorf("repo %s not found", repoName)
	}

	info := repoInfo{
		Name: repoName,
		Path: repoPath,
	}

	// 分支
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	if out, err := cmd.Output(); err == nil {
		info.Branch = strings.TrimSpace(string(out))
	}

	// 最近 commit
	cmd = exec.CommandContext(ctx, "git", "log", "-1", "--format=%H %s (%ci)", "--no-walk")
	cmd.Dir = repoPath
	if out, err := cmd.Output(); err == nil {
		info.LastCommit = strings.TrimSpace(string(out))
	}

	// 文件统计
	var fileCount int
	_ = filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if strings.Contains(path, ".git") {
			if fi.IsDir() && filepath.Base(path) == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !fi.IsDir() {
			fileCount++
		}
		return nil
	})

	return repoManagerOutput{
		Action:  "status",
		Success: true,
		Message: fmt.Sprintf("Repo: %s | Branch: %s | Files: %d | %s", repoName, info.Branch, fileCount, info.LastCommit),
		Repos:   []repoInfo{info},
	}, nil
}

// extractRepoName 从 URL 提取仓库名
func extractRepoName(url string) string {
	// https://github.com/bytedance/deer-flow.git → deer-flow
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
