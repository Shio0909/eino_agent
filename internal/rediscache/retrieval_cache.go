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

const (
	retrievalCachePrefix = "retrieval"
	embeddingCachePrefix = "embedding"
)

type retrievalCacheStore struct {
	raw *redis.Client
}

// NewRetrievalCache 创建基于 Redis 的检索缓存；Redis 不可用时自动降级为 no-op。
func NewRetrievalCache(client *Client) cachepkg.RetrievalCache {
	if client == nil || !client.Enabled() || client.Raw() == nil {
		return cachepkg.NewNoopRetrievalCache()
	}
	return &retrievalCacheStore{raw: client.Raw()}
}

func (s *retrievalCacheStore) GetEmbedding(ctx context.Context, modelID, queryHash string) ([]float32, bool, error) {
	payload, err := s.raw.Get(ctx, s.embeddingKey(modelID, queryHash)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}
	var vector []float32
	if err := json.Unmarshal([]byte(payload), &vector); err != nil {
		return nil, false, err
	}
	return vector, true, nil
}

func (s *retrievalCacheStore) SetEmbedding(ctx context.Context, modelID, queryHash string, vector []float32, ttl time.Duration) error {
	payload, err := json.Marshal(vector)
	if err != nil {
		return err
	}
	return s.raw.Set(ctx, s.embeddingKey(modelID, queryHash), payload, ttl).Err()
}

func (s *retrievalCacheStore) GetRetrievalResult(ctx context.Context, cacheKey string) (*cachepkg.RetrievalResult, bool, error) {
	payload, err := s.raw.Get(ctx, s.retrievalKey(cacheKey)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}
	var result cachepkg.RetrievalResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return nil, false, err
	}
	return &result, true, nil
}

func (s *retrievalCacheStore) SetRetrievalResult(ctx context.Context, cacheKey string, result *cachepkg.RetrievalResult, ttl time.Duration) error {
	if result == nil {
		return nil
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	key := s.retrievalKey(cacheKey)
	pipe := s.raw.TxPipeline()
	pipe.Set(ctx, key, payload, ttl)
	for _, kbID := range knowledgeBaseIDsFromResult(result) {
		pipe.SAdd(ctx, s.knowledgeBaseKey(kbID), key)
		pipe.Expire(ctx, s.knowledgeBaseKey(kbID), ttl)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *retrievalCacheStore) InvalidateKnowledgeBase(ctx context.Context, knowledgeBaseID string) error {
	knowledgeBaseID = strings.TrimSpace(knowledgeBaseID)
	if knowledgeBaseID == "" {
		return nil
	}
	indexKey := s.knowledgeBaseKey(knowledgeBaseID)
	keys, err := s.raw.SMembers(ctx, indexKey).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	pipe := s.raw.TxPipeline()
	if len(keys) > 0 {
		pipe.Del(ctx, keys...)
	}
	pipe.Del(ctx, indexKey)
	_, err = pipe.Exec(ctx)
	return err
}

func knowledgeBaseIDsFromResult(result *cachepkg.RetrievalResult) []string {
	if result == nil {
		return nil
	}
	ids := make(map[string]struct{})
	for _, doc := range result.Documents {
		if doc.Metadata == nil {
			continue
		}
		kbID, _ := doc.Metadata["knowledge_base_id"].(string)
		kbID = strings.TrimSpace(kbID)
		if kbID == "" {
			continue
		}
		ids[kbID] = struct{}{}
	}
	resultIDs := make([]string, 0, len(ids))
	for kbID := range ids {
		resultIDs = append(resultIDs, kbID)
	}
	return resultIDs
}

func (s *retrievalCacheStore) embeddingKey(modelID, queryHash string) string {
	return fmt.Sprintf("%s:%s:%s", embeddingCachePrefix, strings.TrimSpace(modelID), strings.TrimSpace(queryHash))
}

func (s *retrievalCacheStore) retrievalKey(cacheKey string) string {
	return fmt.Sprintf("%s:%s", retrievalCachePrefix, strings.TrimSpace(cacheKey))
}

func (s *retrievalCacheStore) knowledgeBaseKey(knowledgeBaseID string) string {
	return fmt.Sprintf("%s:kb:%s:keys", retrievalCachePrefix, strings.TrimSpace(knowledgeBaseID))
}
