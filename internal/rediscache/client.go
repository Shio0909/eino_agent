package rediscache

import (
	"context"
	"errors"
	"sync"
	"time"

	redis "github.com/redis/go-redis/v9"

	"eino_agent/internal/config"
)

// ErrDisabled 表示 Redis 处于未启用或降级状态。
var ErrDisabled = errors.New("redis unavailable")

// Status 描述 Redis 运行状态，供健康检查和日志使用。
type Status struct {
	Configured bool   `json:"configured"`
	Available  bool   `json:"available"`
	Mode       string `json:"mode"`
	Addr       string `json:"addr,omitempty"`
	LastError  string `json:"last_error,omitempty"`
}

// Client 是 go-redis 客户端的轻量包装，提供降级和状态探测能力。
type Client struct {
	mu      sync.RWMutex
	raw     *redis.Client
	status  Status
	timeout time.Duration
	enabled bool
}

// NewClient 初始化 Redis 客户端；若连接探测失败则返回错误，由调用方决定是否降级。
func NewClient(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	status := Status{
		Configured: cfg.Addr != "",
		Addr:       cfg.Addr,
		Mode:       "disabled",
	}
	if cfg.Addr == "" {
		return &Client{status: status, timeout: 2 * time.Second}, nil
	}

	raw := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	client := &Client{
		raw:     raw,
		timeout: 2 * time.Second,
		enabled: true,
		status: Status{
			Configured: true,
			Available:  false,
			Mode:       "degraded",
			Addr:       cfg.Addr,
		},
	}

	pingCtx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	if err := raw.Ping(pingCtx).Err(); err != nil {
		client.setStatus(func(status *Status) {
			status.LastError = err.Error()
		})
		_ = raw.Close()
		client.raw = nil
		client.enabled = false
		return client, err
	}

	client.setStatus(func(status *Status) {
		status.Available = true
		status.Mode = "active"
	})
	return client, nil
}

// NewFallbackClient 返回一个不可用但可安全查询状态的降级客户端。
func NewFallbackClient(cfg config.RedisConfig, err error) *Client {
	status := Status{
		Configured: cfg.Addr != "",
		Available:  false,
		Mode:       "degraded",
		Addr:       cfg.Addr,
	}
	if err != nil {
		status.LastError = err.Error()
	}
	return &Client{status: status, timeout: 2 * time.Second}
}

// Raw 返回底层 go-redis 客户端，供后续阶段接入具体 store 使用。
func (c *Client) Raw() *redis.Client {
	if c == nil {
		return nil
	}
	return c.raw
}

// Enabled 返回当前客户端是否处于可执行 Redis 操作的状态。
func (c *Client) Enabled() bool {
	return c != nil && c.enabled && c.raw != nil
}

// Ping 主动探测 Redis 可用性。
func (c *Client) Ping(ctx context.Context) error {
	if !c.Enabled() {
		return ErrDisabled
	}
	pingCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if err := c.raw.Ping(pingCtx).Err(); err != nil {
		c.setStatus(func(status *Status) {
			status.Available = false
			status.Mode = "degraded"
			status.LastError = err.Error()
		})
		return err
	}
	c.setStatus(func(status *Status) {
		if status.Configured {
			status.Available = true
			status.Mode = "active"
			status.LastError = ""
		}
	})
	return nil
}

// Status 返回 Redis 当前状态快照。
func (c *Client) Status(ctx context.Context) Status {
	if c == nil {
		return Status{Configured: false, Available: false, Mode: "disabled"}
	}
	status := c.getStatus()
	if c.Enabled() {
		_ = c.Ping(ctx)
		status = c.getStatus()
	}
	return status
}

// Close 关闭底层连接。
func (c *Client) Close() error {
	if c == nil || c.raw == nil {
		return nil
	}
	return c.raw.Close()
}

func (c *Client) getStatus() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Client) setStatus(update func(*Status)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	update(&c.status)
}
