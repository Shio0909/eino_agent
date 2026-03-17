package service

import (
	"context"
	"strings"
	"testing"
	"time"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/config"
	"eino_agent/internal/database/repository"
)

type fakeSessionCache struct {
	recent map[string][]cachepkg.SessionMessage
}

func newFakeSessionCache() *fakeSessionCache {
	return &fakeSessionCache{recent: make(map[string][]cachepkg.SessionMessage)}
}

func (f *fakeSessionCache) GetRecentMessages(_ context.Context, sessionID string, limit int) ([]cachepkg.SessionMessage, bool, error) {
	messages, ok := f.recent[sessionID]
	if !ok {
		return nil, false, nil
	}
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	cloned := make([]cachepkg.SessionMessage, len(messages))
	copy(cloned, messages)
	return cloned, true, nil
}

func (f *fakeSessionCache) SetRecentMessages(_ context.Context, sessionID string, messages []cachepkg.SessionMessage, _ time.Duration) error {
	cloned := make([]cachepkg.SessionMessage, len(messages))
	copy(cloned, messages)
	f.recent[sessionID] = cloned
	return nil
}

func (f *fakeSessionCache) GetSummary(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func (f *fakeSessionCache) SetSummary(context.Context, string, string, time.Duration) error {
	return nil
}

func (f *fakeSessionCache) InvalidateSession(_ context.Context, sessionID string) error {
	delete(f.recent, sessionID)
	return nil
}

type fakeMessageRepo struct {
	messages    map[string][]*repository.Message
	listCalls   int
	createCalls int
}

func newFakeMessageRepo() *fakeMessageRepo {
	return &fakeMessageRepo{messages: make(map[string][]*repository.Message)}
}

func (f *fakeMessageRepo) Create(_ context.Context, m *repository.Message) error {
	f.createCalls++
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	clone := *m
	f.messages[m.SessionID] = append(f.messages[m.SessionID], &clone)
	return nil
}

func (f *fakeMessageRepo) ListBySession(_ context.Context, sessionID string, limit int) ([]*repository.Message, error) {
	f.listCalls++
	items := f.messages[sessionID]
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}
	result := make([]*repository.Message, 0, len(items))
	for _, item := range items {
		clone := *item
		result = append(result, &clone)
	}
	return result, nil
}

func TestBuildMemoryInstructionUsesSessionCache(t *testing.T) {
	cfg := &config.Config{
		Memory: config.MemoryConfig{
			Enabled:                  true,
			WindowSize:               2,
			ShortTermCacheTTLMinutes: 60,
			MaxContextChars:          200,
		},
	}

	svc, err := NewChatService(cfg)
	if err != nil {
		t.Fatalf("NewChatService error = %v", err)
	}

	messageRepo := newFakeMessageRepo()
	cacheStore := newFakeSessionCache()
	cacheStore.recent["s1"] = []cachepkg.SessionMessage{
		{Role: "user", Content: "第一问", CreatedAt: time.Now().Add(-2 * time.Minute)},
		{Role: "assistant", Content: "第一答", CreatedAt: time.Now().Add(-1 * time.Minute)},
	}

	svc.messageRepo = messageRepo
	svc.sessionCache = cacheStore

	instruction := svc.buildMemoryInstruction(context.Background(), &ChatRequest{}, "s1")
	if !strings.Contains(instruction, "会话短期记忆") {
		t.Fatalf("instruction missing short-term memory section: %s", instruction)
	}
	if !strings.Contains(instruction, "第一问") || !strings.Contains(instruction, "第一答") {
		t.Fatalf("instruction missing cached messages: %s", instruction)
	}
	if messageRepo.listCalls != 0 {
		t.Fatalf("expected message repo not to be queried on cache hit, got %d", messageRepo.listCalls)
	}
}

func TestSaveMessagesRefreshesSessionCache(t *testing.T) {
	cfg := &config.Config{
		Memory: config.MemoryConfig{
			Enabled:                  true,
			WindowSize:               1,
			ShortTermCacheTTLMinutes: 60,
			MaxContextChars:          300,
		},
	}

	svc, err := NewChatService(cfg)
	if err != nil {
		t.Fatalf("NewChatService error = %v", err)
	}

	messageRepo := newFakeMessageRepo()
	cacheStore := newFakeSessionCache()
	svc.messageRepo = messageRepo
	svc.sessionCache = cacheStore

	ctx := context.Background()
	svc.saveUserMessage(ctx, "s2", "旧问题")
	svc.saveAssistantMessage(ctx, "s2", "旧回答", 0, 10)
	svc.saveUserMessage(ctx, "s2", "新问题")

	cachedMessages, hit, err := cacheStore.GetRecentMessages(ctx, "s2", 10)
	if err != nil {
		t.Fatalf("GetRecentMessages error = %v", err)
	}
	if !hit {
		t.Fatal("expected session cache hit after writes")
	}
	if len(cachedMessages) != 2 {
		t.Fatalf("expected cache to keep latest 2 messages, got %d", len(cachedMessages))
	}
	if cachedMessages[0].Content != "旧回答" || cachedMessages[1].Content != "新问题" {
		t.Fatalf("unexpected cached order/content: %#v", cachedMessages)
	}

	instruction := svc.buildMemoryInstruction(ctx, &ChatRequest{}, "s2")
	if strings.Contains(instruction, "旧问题") {
		t.Fatalf("instruction should not include evicted message: %s", instruction)
	}
	if !strings.Contains(instruction, "旧回答") || !strings.Contains(instruction, "新问题") {
		t.Fatalf("instruction missing refreshed cached messages: %s", instruction)
	}
	if messageRepo.createCalls != 3 {
		t.Fatalf("expected 3 persisted messages, got %d", messageRepo.createCalls)
	}
}
