// Package logger 提供全局结构化日志（基于 Go 标准库 log/slog）。
//
// 每个请求通过中间件注入 trace_id，所有日志行都携带该 ID，
// 便于在日志聚合系统（Loki、CloudWatch 等）中按请求过滤。
package logger

import (
	"context"
	"log/slog"
	"os"
)

type contextKey int

const traceIDKey contextKey = 1

// Init 根据运行模式初始化全局 slog logger。
// 在 main 函数最早处调用一次。
func Init(mode string) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if mode == "release" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// WithTraceID 把 traceID 存入 context，供后续 FromContext 使用。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// TraceIDFrom 从 context 中取 traceID，不存在时返回空字符串。
func TraceIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// FromContext 返回携带 trace_id 属性的 slog.Logger，
// 调用时只需 logger.FromContext(ctx).Info("msg", "key", val)。
func FromContext(ctx context.Context) *slog.Logger {
	tid := TraceIDFrom(ctx)
	if tid == "" {
		return slog.Default()
	}
	return slog.Default().With("trace_id", tid)
}
