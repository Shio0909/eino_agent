package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// repoNamePattern keeps a repo name to a single plain path segment: it is joined
// onto the repos directory and later handed to os.RemoveAll.
var repoNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

func validRepoName(name string) bool { return repoNamePattern.MatchString(name) }

// validRepoURL rejects anything git would read as an option rather than a location.
func validRepoURL(rawURL string) bool {
	if rawURL == "" || strings.HasPrefix(rawURL, "-") {
		return false
	}
	return !strings.ContainsFunc(rawURL, func(r rune) bool { return r < 0x20 || r == 0x7f })
}

// resolveRepoName picks the explicit name, else the last segment of the URL.
func resolveRepoName(rawURL, name string) string {
	if name == "" {
		name = extractRepoNameFromURL(rawURL)
	}
	return name
}

// gitCloneArgs ends option parsing before the URL so it cannot be taken for a flag.
func gitCloneArgs(repoURL, repoPath string) []string {
	return []string{"clone", "--depth=1", "--", repoURL, repoPath}
}

func isGitCheckout(repoPath string) bool {
	_, err := os.Stat(filepath.Join(repoPath, ".git"))
	return err == nil
}

// existingRepoPath resolves a repo name from the URL to its checkout, writing the
// error response itself when the name is unusable or the repo does not exist.
func (h *Handler) existingRepoPath(c *gin.Context) (name, repoPath string, ok bool) {
	name = c.Param("name")
	if !validRepoName(name) {
		respondError(c, http.StatusBadRequest, "invalid repo name")
		return "", "", false
	}
	repoPath = filepath.Join(h.getCodeReposDir(), name)
	if !isGitCheckout(repoPath) {
		respondError(c, http.StatusNotFound, fmt.Sprintf("repo %s not found", name))
		return "", "", false
	}
	return name, repoPath, true
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
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("read repos dir: %v", err))
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
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.URL == "" {
		respondError(c, http.StatusBadRequest, "url is required")
		return
	}

	if !validRepoURL(req.URL) {
		respondError(c, http.StatusBadRequest, "invalid url")
		return
	}

	repoName := resolveRepoName(req.URL, req.Name)
	if !validRepoName(repoName) {
		respondError(c, http.StatusBadRequest, "invalid repo name; pass a name made of letters, digits, '.', '_' or '-'")
		return
	}

	reposDir := h.getCodeReposDir()
	repoPath := filepath.Join(reposDir, repoName)

	if _, err := os.Stat(repoPath); err == nil {
		respondError(c, http.StatusConflict, fmt.Sprintf("repo %s already exists", repoName))
		return
	}

	if err := os.MkdirAll(reposDir, 0755); err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("create repos dir: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", gitCloneArgs(req.URL, repoPath)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("git clone failed: %s", strings.TrimSpace(string(output))))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cloning completed",
		"name":    repoName,
	})
}

// IndexCodeRepo triggers indexing for a repository.
func (h *Handler) IndexCodeRepo(c *gin.Context) {
	name, _, ok := h.existingRepoPath(c)
	if !ok {
		return
	}

	if h.codeIndexer == nil {
		respondError(c, http.StatusServiceUnavailable, "code graph indexer not initialized")
		return
	}

	progress, err := h.codeIndexer.IndexRepo(c.Request.Context(), name)
	if err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("indexing failed: %v", err))
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
	_, repoPath, ok := h.existingRepoPath(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	// Fetch
	cmd := exec.CommandContext(ctx, "git", "fetch", "--depth=1")
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("git fetch failed: %s", strings.TrimSpace(string(out))))
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
			respondError(c, http.StatusInternalServerError, fmt.Sprintf("pull failed: %s, reset failed: %s", strings.TrimSpace(string(out)), strings.TrimSpace(string(out2))))
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
	name, repoPath, ok := h.existingRepoPath(c)
	if !ok {
		return
	}

	// Delete graph data if available
	if h.codeGraphRepo != nil {
		if err := h.codeGraphRepo.DeleteRepoGraph(c.Request.Context(), name); err != nil {
			respondError(c, http.StatusInternalServerError, fmt.Sprintf("delete graph data failed: %v", err))
			return
		}
	}

	// Delete directory
	if err := os.RemoveAll(repoPath); err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("delete repo directory failed: %v", err))
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

// extractRepoNameFromURL extracts repository name from a git URL, including the
// scp-like form git@host:owner/repo.git.
func extractRepoNameFromURL(rawURL string) string {
	rawURL = strings.TrimSuffix(rawURL, "/")
	rawURL = strings.TrimSuffix(rawURL, ".git")
	rawURL = strings.TrimSuffix(rawURL, "/")
	parts := strings.FieldsFunc(rawURL, func(r rune) bool { return r == '/' || r == ':' })
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
