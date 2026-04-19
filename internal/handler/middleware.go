// Package handler 中间件
package handler

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"eino_agent/internal/logger"
)

const ginTraceIDKey = "trace_id"
const ginUserIDKey = "user_id" // 由 JWTMiddleware 写入

// TraceIDMiddleware 为每个请求生成唯一 trace_id，
// 注入 gin.Context、context.Context 和响应头 X-Trace-ID。
func TraceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}
		c.Set(ginTraceIDKey, traceID)
		c.Header("X-Trace-ID", traceID)

		// 把 traceID 注入底层 context.Context，方便服务层使用 logger.FromContext
		ctx := logger.WithTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// RequestLogger 请求日志中间件（结构化日志，携带 trace_id、user_id）。
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		traceID, _ := c.Get(ginTraceIDKey)
		userID, _ := c.Get(ginUserIDKey)

		slog.Info("http_request",
			"trace_id", traceID,
			"user_id", userID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}

// ErrorResponse 统一错误响应格式
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// respondError 统一错误响应辅助函数
func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, ErrorResponse{Error: message, Code: status})
}

// respondErrorWithDetails 带详情的错误响应
func respondErrorWithDetails(c *gin.Context, status int, message, details string) {
	c.JSON(status, ErrorResponse{Error: message, Code: status, Details: details})
}
