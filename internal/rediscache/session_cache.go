package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"

	cachepkg "eino_agent/internal/cache"
)

const sessionCachePrefix = "session"

type sessionCacheStore struct {
	raw *redis.Client
}

// NewSessionCache 创建基于 Redis 的会话缓存；Redis 不可用时自动降级为 no-op。
func NewSessionCache(client *Client) cachepkg.SessionCache {
	if client == nil || !client.Enabled() || client.Raw() == nil {
		return cachepkg.NewNoopSessionCache()
	}
	return &sessionCacheStore{raw: client.Raw()}
}

func (s *sessionCacheStore) GetRecentMessages(ctx context.Context, sessionID string, limit int) ([]cachepkg.SessionMessage, bool, error) {
	key := s.recentMessagesKey(sessionID)
	payload, err := s.raw.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}

	var messages []cachepkg.SessionMessage
	if err := json.Unmarshal([]byte(payload), &messages); err != nil {
		return nil, false, err
	}
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	return messages, true, nil
}

func (s *sessionCacheStore) SetRecentMessages(ctx context.Context, sessionID string, messages []cachepkg.SessionMessage, ttl time.Duration) error {
	key := s.recentMessagesKey(sessionID)
	payload, err := json.Marshal(messages)
	if err != nil {
		return err
	}
	return s.raw.Set(ctx, key, payload, ttl).Err()
}

func (s *sessionCacheStore) GetSummary(ctx context.Context, sessionID string) (string, bool, error) {
	key := s.summaryKey(sessionID)
	summary, err := s.raw.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", false, nil
		}
		return "", false, err
	}
	return summary, true, nil
}

func (s *sessionCacheStore) SetSummary(ctx context.Context, sessionID, summary string, ttl time.Duration) error {
	key := s.summaryKey(sessionID)
	return s.raw.Set(ctx, key, summary, ttl).Err()
}

func (s *sessionCacheStore) InvalidateSession(ctx context.Context, sessionID string) error {
	keys := []string{s.recentMessagesKey(sessionID), s.summaryKey(sessionID)}
	return s.raw.Del(ctx, keys...).Err()
}

func (s *sessionCacheStore) recentMessagesKey(sessionID string) string {
	return s.key(sessionID, "recent_messages")
}

func (s *sessionCacheStore) summaryKey(sessionID string) string {
	return s.key(sessionID, "summary")
}

func (s *sessionCacheStore) key(sessionID, suffix string) string {
	return fmt.Sprintf("%s:%s:%s", sessionCachePrefix, strings.TrimSpace(sessionID), suffix)
}
