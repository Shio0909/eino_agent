// Package metrics 定义全局 Prometheus 指标并提供 Gin 中间件。
package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP 请求耗时（按方法、路径、状态码分组）
	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP 请求处理耗时（秒）",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
	}, []string{"method", "path", "status"})

	// LLM 调用耗时（按 provider、model 分组）
	LLMDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "llm_call_duration_seconds",
		Help:    "LLM API 调用耗时（秒）",
		Buckets: []float64{.1, .25, .5, 1, 2, 5, 10, 30, 60, 120},
	}, []string{"provider", "model", "mode"})

	// LLM Token 用量（按 provider、model、token 类型分组）
	LLMTokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_tokens_total",
		Help: "LLM API 累计 Token 用量",
	}, []string{"provider", "model", "type"}) // type: prompt | completion

	// 检索耗时（按 mode 分组）
	RetrievalDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "retrieval_duration_seconds",
		Help:    "知识库检索耗时（秒）",
		Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"mode"})

	// 文档入库计数（按状态分组）
	DocumentIngestionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "document_ingestion_total",
		Help: "文档入库次数",
	}, []string{"status"}) // status: success | failure
)

// PrometheusMiddleware 记录每个 HTTP 请求的耗时和状态码。
// 对于高基数路径（含动态参数如 /:id），使用 FullPath() 作为 label，
// 避免 label 爆炸。
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		HTTPDuration.WithLabelValues(
			c.Request.Method,
			path,
			strconv.Itoa(c.Writer.Status()),
		).Observe(time.Since(start).Seconds())
	}
}

// Handler 返回 Prometheus /metrics 端点处理器。
func Handler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// RecordLLMCall 记录一次 LLM 调用的耗时与 token 用量。
// 在 ChatService 的 Chat/ChatStream 完成后调用。
func RecordLLMCall(provider, model, mode string, duration time.Duration, promptTokens, completionTokens int) {
	LLMDuration.WithLabelValues(provider, model, mode).Observe(duration.Seconds())
	if promptTokens > 0 {
		LLMTokensTotal.WithLabelValues(provider, model, "prompt").Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		LLMTokensTotal.WithLabelValues(provider, model, "completion").Add(float64(completionTokens))
	}
}
