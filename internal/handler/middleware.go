// Package handler 中间件
package handler

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger 请求日志中间件，记录每个请求的方法、路径、状态码和耗时
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		log.Printf("[HTTP] %s %s → %d (%dms)",
			method, path, status, latency.Milliseconds())
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
