package cache

import (
	"context"
	"time"
)

// NoopSessionCache 是无副作用的 SessionCache 实现，用于降级场景。
type NoopSessionCache struct{}

// NoopRetrievalCache 是无副作用的 RetrievalCache 实现，用于降级场景。
type NoopRetrievalCache struct{}

// NoopImportStateStore 是无副作用的 ImportStateStore 实现，用于降级场景。
type NoopImportStateStore struct{}

// NoopLocker 是无副作用的 Locker 实现，用于降级场景。
type NoopLocker struct{}

type noopLock struct{}

func NewNoopSessionCache() SessionCache {
	return NoopSessionCache{}
}

func NewNoopRetrievalCache() RetrievalCache {
	return NoopRetrievalCache{}
}

func NewNoopImportStateStore() ImportStateStore {
	return NoopImportStateStore{}
}

func NewNoopLocker() Locker {
	return NoopLocker{}
}

func (NoopSessionCache) GetRecentMessages(context.Context, string, int) ([]SessionMessage, bool, error) {
	return nil, false, nil
}

func (NoopSessionCache) SetRecentMessages(context.Context, string, []SessionMessage, time.Duration) error {
	return nil
}

func (NoopSessionCache) GetSummary(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func (NoopSessionCache) SetSummary(context.Context, string, string, time.Duration) error {
	return nil
}

func (NoopSessionCache) InvalidateSession(context.Context, string) error {
	return nil
}

func (NoopRetrievalCache) GetEmbedding(context.Context, string, string) ([]float32, bool, error) {
	return nil, false, nil
}

func (NoopRetrievalCache) SetEmbedding(context.Context, string, string, []float32, time.Duration) error {
	return nil
}

func (NoopRetrievalCache) GetRetrievalResult(context.Context, string) (*RetrievalResult, bool, error) {
	return nil, false, nil
}

func (NoopRetrievalCache) SetRetrievalResult(context.Context, string, *RetrievalResult, time.Duration) error {
	return nil
}

func (NoopRetrievalCache) InvalidateKnowledgeBase(context.Context, string) error {
	return nil
}

func (NoopImportStateStore) GetTaskState(context.Context, string) (*ImportTaskState, bool, error) {
	return nil, false, nil
}

func (NoopImportStateStore) SetTaskState(context.Context, string, *ImportTaskState, time.Duration) error {
	return nil
}

func (NoopImportStateStore) DeleteTaskState(context.Context, string) error {
	return nil
}

func (NoopLocker) TryLock(context.Context, string, time.Duration) (Lock, bool, error) {
	return noopLock{}, true, nil
}

func (noopLock) Release(context.Context) error {
	return nil
}
