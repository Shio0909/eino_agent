package handler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type componentHealth struct {
	Status    string `json:"status"` // ok | degraded | down | disabled
	Required  bool   `json:"required"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

type healthProbe struct {
	name     string
	required bool
	enabled  bool
	check    func(ctx context.Context) error
}

// LivenessCheck 只表示进程存活，不访问任何依赖。
func (h *Handler) LivenessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now().UTC().Format(time.RFC3339)})
}

// HealthCheck 并发探测各依赖的真实连通性；必需依赖不可用时返回 503。
func (h *Handler) HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	probes := []healthProbe{
		{name: "database", required: h.cfg.Database.Host != "", enabled: h.db != nil, check: func(ctx context.Context) error { return h.db.Ping(ctx) }},
		{name: "vector_store", required: true, enabled: h.vectorDB != nil, check: func(ctx context.Context) error { return h.vectorDB.Ping(ctx) }},
		{name: "redis", enabled: h.redisClient != nil && h.redisClient.Enabled(), check: func(ctx context.Context) error { return h.redisClient.Ping(ctx) }},
	}

	results := make(map[string]componentHealth, len(probes)+4)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, p := range probes {
		if !p.enabled {
			status := "disabled"
			if p.required {
				status = "down"
			}
			results[p.name] = componentHealth{Status: status, Required: p.required}
			continue
		}
		wg.Add(1)
		go func(p healthProbe) {
			defer wg.Done()
			started := time.Now()
			err := p.check(ctx)
			res := componentHealth{Status: "ok", Required: p.required, LatencyMs: time.Since(started).Milliseconds()}
			if err != nil {
				res.Status = "down"
				res.Error = err.Error()
			}
			mu.Lock()
			results[p.name] = res
			mu.Unlock()
		}(p)
	}
	wg.Wait()

	results["llm"] = presence(h.chatService != nil, true)
	results["embedding"] = presence(h.currentEmbedder() != nil, true)
	results["docreader"] = presence(h.docReaderCli != nil, false)
	results["import_queue"] = presence(h.importQueue != nil, false)
	results["graphrag"] = presence(h.graphRAGService != nil, false)

	overall, code := "ok", http.StatusOK
	for _, r := range results {
		if r.Status != "down" {
			continue
		}
		if r.Required {
			overall, code = "down", http.StatusServiceUnavailable
			break
		}
		overall = "degraded"
	}

	c.JSON(code, gin.H{
		"status":     overall,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"components": results,
	})
}

func presence(ok, required bool) componentHealth {
	if ok {
		return componentHealth{Status: "ok", Required: required}
	}
	if required {
		return componentHealth{Status: "down", Required: required}
	}
	return componentHealth{Status: "disabled", Required: required}
}
