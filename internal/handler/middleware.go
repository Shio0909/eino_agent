// Package handler 中间件
package handler

import (
	"log/slog"
	"net/http"
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

// APIError is the body of every error response:
//
//	{"error": {"code": "not_found", "message": "...", "trace_id": "..."}}
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	TraceID string `json:"trace_id,omitempty"`
	Details any    `json:"details,omitempty"`
}

// ErrorResponse wraps APIError.
type ErrorResponse struct {
	Error APIError `json:"error"`
}

func errorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_argument"
	case http.StatusUnauthorized:
		return "unauthenticated"
	case http.StatusForbidden:
		return "permission_denied"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case http.StatusUnprocessableEntity:
		return "unprocessable"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusBadGateway:
		return "upstream_error"
	case http.StatusServiceUnavailable:
		return "unavailable"
	case http.StatusGatewayTimeout:
		return "upstream_timeout"
	default:
		if status >= 500 {
			return "internal"
		}
		return "error"
	}
}

func newErrorResponse(c *gin.Context, status int, message string, details any) ErrorResponse {
	traceID, _ := c.Get(ginTraceIDKey)
	id, _ := traceID.(string)
	return ErrorResponse{Error: APIError{Code: errorCodeForStatus(status), Message: message, TraceID: id, Details: details}}
}

func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, newErrorResponse(c, status, message, nil))
}

func respondErrorWithDetails(c *gin.Context, status int, message string, details any) {
	c.JSON(status, newErrorResponse(c, status, message, details))
}

func abortWithError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, newErrorResponse(c, status, message, nil))
}
