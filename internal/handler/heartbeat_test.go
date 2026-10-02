package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/usage"
)

type heartbeatBody struct {
	Busy    bool         `json:"busy"`
	Version string       `json:"version"`
	Usage   usage.Totals `json:"usage"`
}

func pollHeartbeat(t *testing.T, r *gin.Engine) heartbeatBody {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/heartbeat", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var b heartbeatBody
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode %q: %v", w.Body, err)
	}
	return b
}

func TestHeartbeatReportsBusyVersionAndCumulativeUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	started, release := make(chan struct{}), make(chan struct{})
	r := gin.New()
	r.GET("/heartbeat", h.Heartbeat)
	r.POST("/chat", h.TrackInFlight(), func(c *gin.Context) {
		close(started)
		<-release
		c.Status(http.StatusOK)
	})

	t.Setenv("AGENT_VERSION", "v9.9")
	before := usage.Default.Totals()
	idle := pollHeartbeat(t, r)
	if idle.Busy || idle.Version != "v9.9" {
		t.Fatalf("idle heartbeat = %+v, want not busy with the image version", idle)
	}

	usage.Default.Record(120, 30, 150)
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/chat", nil))
	}()
	<-started
	busy := pollHeartbeat(t, r)
	if !busy.Busy {
		t.Fatal("a chat turn in flight must show as busy")
	}
	if busy.Usage.TotalTokens-before.TotalTokens != 150 || busy.Usage.PromptTokens-before.PromptTokens != 120 {
		t.Fatalf("usage %+v did not grow by the recorded 120/30/150 from %+v", busy.Usage, before)
	}

	close(release)
	<-done
	deadline := time.Now().Add(time.Second)
	for pollHeartbeat(t, r).Busy {
		if time.Now().After(deadline) {
			t.Fatal("still busy after the request finished")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHeartbeatFallsBackToTheBuildVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("AGENT_VERSION", "")
	r := gin.New()
	r.GET("/heartbeat", (&Handler{}).Heartbeat)
	if got := pollHeartbeat(t, r).Version; got != Version {
		t.Fatalf("version = %q, want the build version %q", got, Version)
	}
}

func TestHeartbeatIsRegisteredOutsideAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&Handler{}).RegisterRoutes(r)
	for _, route := range r.Routes() {
		if route.Method == http.MethodGet && route.Path == "/heartbeat" {
			return
		}
	}
	t.Fatal("GET /heartbeat is not registered")
}
