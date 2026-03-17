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

const importStatePrefix = "import:state"

type importStateStore struct {
	raw *redis.Client
}

// NewImportStateStore 创建基于 Redis 的导入状态存储；Redis 不可用时自动降级为 no-op。
func NewImportStateStore(client *Client) cachepkg.ImportStateStore {
	if client == nil || !client.Enabled() || client.Raw() == nil {
		return cachepkg.NewNoopImportStateStore()
	}
	return &importStateStore{raw: client.Raw()}
}

func (s *importStateStore) GetTaskState(ctx context.Context, taskID string) (*cachepkg.ImportTaskState, bool, error) {
	payload, err := s.raw.Get(ctx, s.stateKey(taskID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}

	var state cachepkg.ImportTaskState
	if err := json.Unmarshal([]byte(payload), &state); err != nil {
		return nil, false, err
	}
	return &state, true, nil
}

func (s *importStateStore) SetTaskState(ctx context.Context, taskID string, state *cachepkg.ImportTaskState, ttl time.Duration) error {
	if state == nil {
		return nil
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.raw.Set(ctx, s.stateKey(taskID), payload, ttl).Err()
}

func (s *importStateStore) DeleteTaskState(ctx context.Context, taskID string) error {
	return s.raw.Del(ctx, s.stateKey(taskID)).Err()
}

func (s *importStateStore) stateKey(taskID string) string {
	return fmt.Sprintf("%s:%s", importStatePrefix, strings.TrimSpace(taskID))
}
