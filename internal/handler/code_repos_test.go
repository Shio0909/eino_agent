package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/config"
)

// codeRepoEnv lays out
//
//	<root>/data/            <- a sibling of the repos dir that must never be touched
//	<root>/data/repos/real/.git
//	<root>/data/repos/scratch/   (a directory that is not a git checkout)
type codeRepoEnv struct {
	h        *Handler
	root     string
	reposDir string
}

func newCodeRepoEnv(t *testing.T) *codeRepoEnv {
	t.Helper()
	root := t.TempDir()
	reposDir := filepath.Join(root, "data", "repos")
	for _, dir := range []string{filepath.Join(reposDir, "real", ".git"), filepath.Join(reposDir, "scratch")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "data", "app.db"), []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Agent.CodeSearchReposDir = reposDir
	return &codeRepoEnv{h: &Handler{cfg: cfg}, root: root, reposDir: reposDir}
}

func (e *codeRepoEnv) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(e.root, rel))
	return err == nil
}

func (e *codeRepoEnv) call(handle gin.HandlerFunc, method, body string, params gin.Params) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/code-repos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	handle(c)
	return w
}

func nameParam(name string) gin.Params { return gin.Params{{Key: "name", Value: name}} }

// A repo name is used as a path segment, so a name that climbs out of the repos
// directory would let one request delete the database next to it.
func TestCodeRepoMutationsRejectNamesOutsideTheReposDir(t *testing.T) {
	for _, name := range []string{"..", ".", "../real", "a/b", ".hidden", "bad name", ""} {
		for label, run := range map[string]func(*codeRepoEnv) *httptest.ResponseRecorder{
			"delete": func(e *codeRepoEnv) *httptest.ResponseRecorder {
				return e.call(e.h.DeleteCodeRepo, http.MethodDelete, "", nameParam(name))
			},
			"pull": func(e *codeRepoEnv) *httptest.ResponseRecorder {
				return e.call(e.h.PullCodeRepo, http.MethodPost, "", nameParam(name))
			},
			"index": func(e *codeRepoEnv) *httptest.ResponseRecorder {
				return e.call(e.h.IndexCodeRepo, http.MethodPost, "", nameParam(name))
			},
		} {
			env := newCodeRepoEnv(t)
			w := run(env)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s %q: status = %d, want 400 (body=%s)", label, name, w.Code, w.Body.String())
			}
			if !env.exists("data/app.db") || !env.exists("data/repos/real/.git") || !env.exists("data/repos/scratch") {
				t.Errorf("%s %q: something outside the request was removed", label, name)
			}
		}
	}
}

func TestCodeRepoMutationsOnlyTouchGitCheckouts(t *testing.T) {
	env := newCodeRepoEnv(t)
	w := env.call(env.h.DeleteCodeRepo, http.MethodDelete, "", nameParam("scratch"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("deleting a plain directory: status = %d, want 404 (body=%s)", w.Code, w.Body.String())
	}
	if !env.exists("data/repos/scratch") {
		t.Fatal("a directory that is not a repo was deleted")
	}
}

func TestDeleteCodeRepoRemovesTheCheckout(t *testing.T) {
	env := newCodeRepoEnv(t)
	w := env.call(env.h.DeleteCodeRepo, http.MethodDelete, "", nameParam("real"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if env.exists("data/repos/real") {
		t.Fatal("repo directory still there")
	}
	if !env.exists("data/repos/scratch") || !env.exists("data/app.db") {
		t.Fatal("deleted more than the named repo")
	}
}

func TestCloneCodeRepoRejectsUnsafeNamesAndUrls(t *testing.T) {
	cases := []struct{ label, body string }{
		{"name climbing out", `{"url":"https://example.com/x/y.git","name":"../escaped"}`},
		{"name with separator", `{"url":"https://example.com/x/y.git","name":"a/b"}`},
		{"name taken from a url ending in ..", `{"url":"https://example.com/x/.."}`},
		{"url that is an option", `{"url":"--upload-pack=touch /tmp/eino_pwned","name":"ok"}`},
		{"short option url", `{"url":"-c core.sshCommand=touch","name":"ok2"}`},
	}
	for _, tc := range cases {
		env := newCodeRepoEnv(t)
		w := env.call(env.h.CloneCodeRepo, http.MethodPost, tc.body, nil)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (body=%s)", tc.label, w.Code, w.Body.String())
		}
		if env.exists("data/escaped") || env.exists("data/repos/ok") || env.exists("data/repos/ok2") {
			t.Errorf("%s: a directory was created", tc.label)
		}
	}
}

func TestMCPCloneCodeRepoRejectsUnsafeNamesAndUrls(t *testing.T) {
	env := newCodeRepoEnv(t)
	for _, tc := range []struct{ url, name string }{
		{"https://example.com/x/y.git", "../escaped"},
		{"https://example.com/x/..", ""},
		{"--upload-pack=touch /tmp/eino_pwned", "ok"},
	} {
		if _, err := env.h.MCPCloneCodeRepo(t.Context(), tc.url, tc.name); err == nil {
			t.Errorf("clone(%q, %q) succeeded", tc.url, tc.name)
		}
	}
	if env.exists("data/escaped") || env.exists("data/repos/ok") {
		t.Fatal("a directory was created")
	}
}

func TestMCPIndexCodeRepoRejectsNamesOutsideTheReposDir(t *testing.T) {
	env := newCodeRepoEnv(t)
	_, err := env.h.MCPIndexCodeRepo(t.Context(), "..")
	if err == nil || !strings.Contains(err.Error(), "不合法") {
		t.Fatalf("index of '..': err = %v, want a name-validation error", err)
	}
}

func TestValidRepoName(t *testing.T) {
	for name, want := range map[string]bool{
		"repo": true, "my-repo_1.2": true, "A9": true, "a..b": true,
		"": false, ".": false, "..": false, ".git": false, "-x": false, "a/b": false, `a\b`: false, "a b": false,
		strings.Repeat("a", 101): false,
	} {
		if got := validRepoName(name); got != want {
			t.Errorf("validRepoName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestExtractRepoNameFromURL(t *testing.T) {
	for url, want := range map[string]string{
		"https://github.com/owner/repo.git":  "repo",
		"https://github.com/owner/repo/":     "repo",
		"https://github.com/owner/repo.git/": "repo",
		"https://github.com/owner/repo":      "repo",
		"git@github.com:owner/repo.git":      "repo",
		"git@host:repo.git":                  "repo",
		"ssh://git@host:2222/owner/repo":     "repo",
	} {
		if got := extractRepoNameFromURL(url); got != want {
			t.Errorf("extractRepoNameFromURL(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestCloneCodeRepoClonesIntoTheReposDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	env := newCodeRepoEnv(t)
	src := filepath.Join(t.TempDir(), "source-repo")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}

	w := env.call(env.h.CloneCodeRepo, http.MethodPost, `{"url":"file://`+src+`","name":"copy"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !env.exists("data/repos/copy/.git") {
		t.Fatal("checkout missing")
	}
	again := env.call(env.h.CloneCodeRepo, http.MethodPost, `{"url":"file://`+src+`","name":"copy"}`, nil)
	if again.Code != http.StatusConflict {
		t.Fatalf("second clone: status = %d, want 409", again.Code)
	}
}

func TestValidRepoURL(t *testing.T) {
	for url, want := range map[string]bool{
		"https://github.com/o/r.git": true, "git@github.com:o/r.git": true, "/srv/git/r": true,
		"": false, "-u": false, "--upload-pack=x": false, "https://x/r\n--upload-pack=x": false, "https://x/\x00r": false, "a\x7fb": false,
	} {
		if got := validRepoURL(url); got != want {
			t.Errorf("validRepoURL(%q) = %v, want %v", url, got, want)
		}
	}
}

func TestGitCloneArgsEndOptionsBeforeTheLocation(t *testing.T) {
	args := gitCloneArgs("https://x/r.git", "/tmp/r")
	sep := -1
	for i, a := range args {
		if a == "--" {
			sep = i
		}
	}
	if sep < 0 || args[sep+1] != "https://x/r.git" || args[sep+2] != "/tmp/r" || len(args) != sep+3 {
		t.Fatalf("args = %v", args)
	}
}
