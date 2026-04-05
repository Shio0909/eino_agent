package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// codeRepoInfo represents a code repository with its indexing status.
type codeRepoInfo struct {
	Name           string          `json:"name"`
	Path           string          `json:"path"`
	Branch         string          `json:"branch"`
	LastCommit     string          `json:"last_commit"`
	LastCommitDate string          `json:"last_commit_date"`
	Indexed        bool            `json:"indexed"`
	IndexStats     *codeIndexStats `json:"index_stats,omitempty"`
}

type codeIndexStats struct {
	Files     int `json:"files"`
	Entities  int `json:"entities"`
	Relations int `json:"relations"`
}

// getCodeReposDir returns the configured code search repos directory.
func (h *Handler) getCodeReposDir() string {
	dir := h.cfg.Agent.CodeSearchReposDir
	if dir == "" {
		dir = "data/test_repos"
	}
	return dir
}

// ListCodeRepos lists all code repositories with their indexing status.
func (h *Handler) ListCodeRepos(c *gin.Context) {
	reposDir := h.getCodeReposDir()

	entries, err := os.ReadDir(reposDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"repos": []codeRepoInfo{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("read repos dir: %v", err)})
		return
	}

	ctx := c.Request.Context()
	var repos []codeRepoInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		repoPath := filepath.Join(reposDir, name)
		gitDir := filepath.Join(repoPath, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			continue
		}

		info := codeRepoInfo{
			Name: name,
			Path: repoPath,
		}

		// Get current branch
		if out, err := gitCommand(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
			info.Branch = out
		}

		// Get last commit hash (short)
		if out, err := gitCommand(ctx, repoPath, "log", "-1", "--format=%h", "--no-walk"); err == nil {
			info.LastCommit = out
		}

		// Get last commit date
		if out, err := gitCommand(ctx, repoPath, "log", "-1", "--format=%cI", "--no-walk"); err == nil {
			info.LastCommitDate = out
		}

		// Get index stats from code graph if available
		if h.codeGraphRepo != nil {
			overview, err := h.codeGraphRepo.GetRepoOverview(ctx, name)
			if err == nil && overview != nil && (overview.FileCount > 0 || overview.EntityCount > 0) {
				info.Indexed = true
				info.IndexStats = &codeIndexStats{
					Files:     overview.FileCount,
					Entities:  overview.EntityCount,
					Relations: overview.RelationCount,
				}
			}
		}

		repos = append(repos, info)
	}

	if repos == nil {
		repos = []codeRepoInfo{}
	}

	c.JSON(http.StatusOK, gin.H{"repos": repos})
}

// CloneCodeRepo clones a new repository.
func (h *Handler) CloneCodeRepo(c *gin.Context) {
	var req struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}

	repoName := req.Name
	if repoName == "" {
		repoName = extractRepoNameFromURL(req.URL)
	}
	if repoName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot extract repo name from url"})
		return
	}

	reposDir := h.getCodeReposDir()
	repoPath := filepath.Join(reposDir, repoName)

	if _, err := os.Stat(repoPath); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("repo %s already exists", repoName)})
		return
	}

	if err := os.MkdirAll(reposDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("create repos dir: %v", err)})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	args := []string{"clone", "--depth=1", req.URL, repoPath}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("git clone failed: %s", strings.TrimSpace(string(output)))})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cloning completed",
		"name":    repoName,
	})
}

// IndexCodeRepo triggers indexing for a repository.
func (h *Handler) IndexCodeRepo(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo name is required"})
		return
	}

	reposDir := h.getCodeReposDir()
	repoPath := filepath.Join(reposDir, name)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("repo %s not found", name)})
		return
	}

	if h.codeIndexer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "code graph indexer not initialized"})
		return
	}

	progress, err := h.codeIndexer.IndexRepo(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("indexing failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Indexing started",
		"stats": gin.H{
			"total_files": progress.TotalFiles,
			"processed":   progress.Processed,
			"entities":    progress.Entities,
			"relations":   progress.Relations,
			"errors":      progress.Errors,
			"elapsed_ms":  progress.ElapsedMs,
		},
	})
}

// PullCodeRepo pulls latest changes for a repository.
func (h *Handler) PullCodeRepo(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo name is required"})
		return
	}

	reposDir := h.getCodeReposDir()
	repoPath := filepath.Join(reposDir, name)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("repo %s not found", name)})
		return
	}

	ctx := c.Request.Context()

	// Fetch
	cmd := exec.CommandContext(ctx, "git", "fetch", "--depth=1")
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("git fetch failed: %s", strings.TrimSpace(string(out)))})
		return
	}

	// Count commits behind
	countStr, _ := gitCommand(ctx, repoPath, "rev-list", "--count", "HEAD..FETCH_HEAD")
	newCommits, _ := strconv.Atoi(countStr)

	if newCommits == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message":     "Already up to date",
			"new_commits": 0,
		})
		return
	}

	// Pull
	cmd = exec.CommandContext(ctx, "git", "pull", "--ff-only")
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		// Fallback to reset
		cmd = exec.CommandContext(ctx, "git", "reset", "--hard", "FETCH_HEAD")
		cmd.Dir = repoPath
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("pull failed: %s, reset failed: %s", strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)))})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Updated",
		"new_commits": newCommits,
	})
}

// DeleteCodeRepo deletes a repository and its graph data.
func (h *Handler) DeleteCodeRepo(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo name is required"})
		return
	}

	reposDir := h.getCodeReposDir()
	repoPath := filepath.Join(reposDir, name)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("repo %s not found", name)})
		return
	}

	// Delete graph data if available
	if h.codeGraphRepo != nil {
		if err := h.codeGraphRepo.DeleteRepoGraph(c.Request.Context(), name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("delete graph data failed: %v", err)})
			return
		}
	}

	// Delete directory
	if err := os.RemoveAll(repoPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("delete repo directory failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("repo %s deleted", name)})
}

// gitCommand runs a git command in the given directory and returns trimmed stdout.
func gitCommand(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// extractRepoNameFromURL extracts repository name from a git URL.
func extractRepoNameFromURL(rawURL string) string {
	rawURL = strings.TrimSuffix(rawURL, ".git")
	rawURL = strings.TrimSuffix(rawURL, "/")
	parts := strings.Split(rawURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
