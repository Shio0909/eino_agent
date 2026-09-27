// Package neo4jutil 汇集 graphrag 与 codegraph 共用的 Neo4j 连接、会话与记录解析辅助函数。
package neo4jutil

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// DefaultUsername 未配置用户名时使用的 Neo4j 默认用户名。
const DefaultUsername = "neo4j"

// ConnectConfig Neo4j 连接参数，只来自调用方传入的配置。
type ConnectConfig struct {
	URI      string
	Username string
	Password string

	// MaxRetries 连接重试次数，<=0 时为 5。
	MaxRetries int
	// RetryInterval 重试间隔，<=0 时为 2s。
	RetryInterval time.Duration
	// LogPrefix 日志前缀，如 "[GraphRAG]"。
	LogPrefix string
}

// ErrMissingURI 表示未配置 Neo4j URI。
var ErrMissingURI = errors.New("neo4j: uri is required")

// ErrMissingPassword 表示未配置 Neo4j 密码。
var ErrMissingPassword = errors.New("neo4j: password is required")

// Validate 校验连接参数并填充可安全默认的字段（仅用户名）。
func (c *ConnectConfig) Validate() error {
	c.URI = strings.TrimSpace(c.URI)
	if c.URI == "" {
		return ErrMissingURI
	}
	if strings.TrimSpace(c.Username) == "" {
		c.Username = DefaultUsername
	}
	if c.Password == "" {
		return ErrMissingPassword
	}
	return nil
}

// Connect 创建驱动并校验连通性（带重试）。调用方拥有返回的驱动，负责 Close。
func Connect(ctx context.Context, cfg ConnectConfig) (neo4j.DriverWithContext, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 5
	}
	interval := cfg.RetryInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	prefix := cfg.LogPrefix
	if prefix == "" {
		prefix = "[Neo4j]"
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		driver, err := neo4j.NewDriverWithContext(cfg.URI, neo4j.BasicAuth(cfg.Username, cfg.Password, ""))
		if err != nil {
			lastErr = err
			log.Printf("%s 创建 Neo4j 驱动失败 (尝试 %d/%d): %v", prefix, attempt, maxRetries, err)
		} else if err = driver.VerifyConnectivity(ctx); err == nil {
			log.Printf("%s Neo4j 连接成功: %s (尝试 %d 次)", prefix, cfg.URI, attempt)
			return driver, nil
		} else {
			lastErr = err
			log.Printf("%s Neo4j 连接验证失败 (尝试 %d/%d): %v", prefix, attempt, maxRetries, err)
			_ = driver.Close(ctx)
		}

		if attempt == maxRetries {
			break
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("neo4j: connect canceled after %d attempts: %w", attempt, ctx.Err())
		case <-time.After(interval):
		}
	}
	return nil, fmt.Errorf("neo4j: connect failed after %d attempts: %w", maxRetries, lastErr)
}

// ReadSession 打开只读会话，调用方负责 Close。
func ReadSession(ctx context.Context, driver neo4j.DriverWithContext) neo4j.SessionWithContext {
	return driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
}

// WriteSession 打开写会话，调用方负责 Close。
func WriteSession(ctx context.Context, driver neo4j.DriverWithContext) neo4j.SessionWithContext {
	return driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
}

// Exec 在自动提交事务中执行写语句并消费结果。
// 自动提交模式下服务端错误可能要到消费结果时才返回，只检查 Run 的返回值会漏掉错误。
func Exec(ctx context.Context, session neo4j.SessionWithContext, cypher string, params map[string]any) error {
	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return err
	}
	_, err = result.Consume(ctx)
	return err
}

// String 读取记录字段为字符串，缺失或 nil 返回空串。
func String(record *neo4j.Record, key string) string {
	if record == nil {
		return ""
	}
	val, ok := record.Get(key)
	if !ok || val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

// Int 读取记录字段为 int，缺失或类型不符返回 0。
func Int(record *neo4j.Record, key string) int {
	if record == nil {
		return 0
	}
	val, ok := record.Get(key)
	if !ok {
		return 0
	}
	return ToInt(val)
}

// ToInt 将 Neo4j 返回的数值转为 int。
func ToInt(val any) int {
	switch v := val.(type) {
	case int64:
		return int(v)
	case int:
		return v
	case int32:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// ToFloat 将 Neo4j 返回的数值转为 float64。
func ToFloat(val any) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int64:
		return float64(v)
	case int:
		return float64(v)
	default:
		return 0
	}
}

// ToStrings 将 Neo4j 返回的列表转为 []string，非列表返回 nil。
func ToStrings(v any) []string {
	switch list := v.(type) {
	case []string:
		return append([]string(nil), list...)
	case []any:
		result := make([]string, 0, len(list))
		for _, item := range list {
			if item == nil {
				continue
			}
			if s, ok := item.(string); ok {
				result = append(result, s)
				continue
			}
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	default:
		return nil
	}
}

// QuoteIdentifier 以反引号转义标签/关系类型等标识符，防止拼接进 Cypher 时被注入。
func QuoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
