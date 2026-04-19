// Package handler 提供 rate limiter 中间件
package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiterConfig 限流器配置
type RateLimiterConfig struct {
	RequestsPerMinute int           // 每分钟请求数上限
	BurstSize         int           // 突发容量
	CleanupInterval   time.Duration // 过期桶清理间隔
}

// DefaultRateLimiterConfig 默认配置：每分钟 60 次，突发 10
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerMinute: 60,
		BurstSize:         10,
		CleanupInterval:   5 * time.Minute,
	}
}

type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func (b *tokenBucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	config  RateLimiterConfig
}

func newRateLimiter(cfg RateLimiterConfig) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		config:  cfg,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[key]
	if !exists {
		bucket = &tokenBucket{
			tokens:     float64(rl.config.BurstSize),
			maxTokens:  float64(rl.config.BurstSize),
			refillRate: float64(rl.config.RequestsPerMinute) / 60.0,
			lastRefill: time.Now(),
		}
		rl.buckets[key] = bucket
	}
	return bucket.allow()
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		threshold := time.Now().Add(-rl.config.CleanupInterval)
		for key, bucket := range rl.buckets {
			if bucket.lastRefill.Before(threshold) {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitMiddleware 基于客户端 IP 的令牌桶限流中间件
func RateLimitMiddleware(cfg RateLimiterConfig) gin.HandlerFunc {
	limiter := newRateLimiter(cfg)

	return func(c *gin.Context) {
		key := c.ClientIP()
		if !limiter.allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "请求过于频繁，请稍后再试",
				"code":  http.StatusTooManyRequests,
			})
			return
		}
		c.Next()
	}
}

// AuthRateLimitMiddleware 认证端点专用限流（更严格：每分钟 10 次）
func AuthRateLimitMiddleware() gin.HandlerFunc {
	limiter := newRateLimiter(RateLimiterConfig{
		RequestsPerMinute: 10,
		BurstSize:         3,
		CleanupInterval:   5 * time.Minute,
	})

	return func(c *gin.Context) {
		key := c.ClientIP()
		if !limiter.allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "登录尝试过于频繁，请稍后再试",
				"code":  http.StatusTooManyRequests,
			})
			return
		}
		c.Next()
	}
}
