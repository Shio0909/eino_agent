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
	now         func() time.Time
}

func newFakeMessageRepo() *fakeMessageRepo {
	return &fakeMessageRepo{
		messages: make(map[string][]*repository.Message),
		now:      time.Now,
	}
}

func (f *fakeMessageRepo) Create(_ context.Context, m *repository.Message) error {
	f.createCalls++
	if m.CreatedAt.IsZero() {
		m.CreatedAt = f.now()
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
	baseTime := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
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
	messageRepo.now = func() time.Time { return baseTime }
	cacheStore := newFakeSessionCache()
	cacheStore.recent["s1"] = []cachepkg.SessionMessage{
		{Role: "user", Content: "first question", CreatedAt: baseTime.Add(-2 * time.Minute)},
		{Role: "assistant", Content: "first answer", CreatedAt: baseTime.Add(-1 * time.Minute)},
	}

	svc.messageRepo = messageRepo
	svc.sessionCache = cacheStore

	instruction := svc.buildMemoryInstruction(context.Background(), &ChatRequest{}, "s1")
	if !strings.Contains(instruction, "first question") || !strings.Contains(instruction, "first answer") {
		t.Fatalf("instruction missing cached messages: %s", instruction)
	}
	if messageRepo.listCalls != 0 {
		t.Fatalf("expected message repo not to be queried on cache hit, got %d", messageRepo.listCalls)
	}
}

func TestFollowUpEvidenceInstructionUsesLastAssistantSources(t *testing.T) {
	cfg := &config.Config{Memory: config.MemoryConfig{Enabled: true, MaxContextChars: 1000}}
	svc, err := NewChatService(cfg)
	if err != nil {
		t.Fatalf("NewChatService error = %v", err)
	}
	messageRepo := newFakeMessageRepo()
	svc.messageRepo = messageRepo

	ctx := context.Background()
	svc.saveAssistantMessageWithTrace(ctx, "s1", "上一轮答案", 0, 10, nil, []Source{
		{DocID: "doc-1", Content: "Eino Agent 支持 Pipeline RAG 和 Agentic ReAct 两条路径。", Metadata: map[string]any{"source": "README.md"}},
	})

	instruction, sources := svc.buildFollowUpEvidenceInstruction(ctx, &ChatRequest{Message: "这个有什么风险？"}, "s1")
	if len(sources) != 1 {
		t.Fatalf("sources len = %d, want 1", len(sources))
	}
	if !strings.Contains(instruction, "上一轮检索证据") || !strings.Contains(instruction, "Pipeline RAG") || !strings.Contains(instruction, "README.md") {
		t.Fatalf("instruction missing last evidence: %s", instruction)
	}
}

func TestFollowUpEvidenceInstructionReadsJSONDecodedSources(t *testing.T) {
	cfg := &config.Config{Memory: config.MemoryConfig{Enabled: true, MaxContextChars: 1000}}
	svc, err := NewChatService(cfg)
	if err != nil {
		t.Fatalf("NewChatService error = %v", err)
	}
	messageRepo := newFakeMessageRepo()
	messageRepo.messages["s1"] = []*repository.Message{{
		SessionID: "s1",
		Role:      "assistant",
		Content:   "上一轮答案",
		AgentSteps: repository.JSON{"sources": []any{map[string]any{
			"content":  "GraphRAG 用图谱关系补充向量检索。",
			"doc_id":   "doc-graph",
			"metadata": map[string]any{"source": "graphrag.md"},
		}}},
	}}
	svc.messageRepo = messageRepo

	instruction, sources := svc.buildFollowUpEvidenceInstruction(context.Background(), &ChatRequest{Message: "继续展开这个"}, "s1")
	if len(sources) != 1 || sources[0].DocID != "doc-graph" {
		t.Fatalf("unexpected decoded sources: %#v", sources)
	}
	if !strings.Contains(instruction, "GraphRAG") || !strings.Contains(instruction, "graphrag.md") {
		t.Fatalf("instruction missing decoded evidence: %s", instruction)
	}
}

func TestFollowUpEvidenceInstructionSkipsNewTopic(t *testing.T) {
	cfg := &config.Config{Memory: config.MemoryConfig{Enabled: true, MaxContextChars: 1000}}
	svc, err := NewChatService(cfg)
	if err != nil {
		t.Fatalf("NewChatService error = %v", err)
	}
	messageRepo := newFakeMessageRepo()
	svc.messageRepo = messageRepo

	ctx := context.Background()
	svc.saveAssistantMessageWithTrace(ctx, "s1", "上一轮答案", 0, 10, nil, []Source{
		{DocID: "doc-1", Content: "旧证据", Metadata: map[string]any{"source": "old.md"}},
	})

	instruction, sources := svc.buildFollowUpEvidenceInstruction(ctx, &ChatRequest{Message: "请介绍 Redis 的持久化机制"}, "s1")
	if instruction != "" || len(sources) != 0 {
		t.Fatalf("expected no reused evidence for new topic, got instruction=%q sources=%#v", instruction, sources)
	}
}

func TestFollowUpEvidenceInstructionSkipsStandaloneRiskQuestion(t *testing.T) {
	cfg := &config.Config{Memory: config.MemoryConfig{Enabled: true, MaxContextChars: 1000}}
	svc, err := NewChatService(cfg)
	if err != nil {
		t.Fatalf("NewChatService error = %v", err)
	}
	messageRepo := newFakeMessageRepo()
	svc.messageRepo = messageRepo

	ctx := context.Background()
	svc.saveAssistantMessageWithTrace(ctx, "s1", "上一轮答案", 0, 10, nil, []Source{
		{DocID: "doc-1", Content: "Eino Agent 的旧证据", Metadata: map[string]any{"source": "old.md"}},
	})

	instruction, sources := svc.buildFollowUpEvidenceInstruction(ctx, &ChatRequest{Message: "Redis 持久化有什么风险？"}, "s1")
	if instruction != "" || len(sources) != 0 {
		t.Fatalf("expected standalone topic risk question not to reuse evidence, got instruction=%q sources=%#v", instruction, sources)
	}
}

func TestPrepareChatContextTracksFollowUpEvidenceCount(t *testing.T) {
	cfg := &config.Config{Memory: config.MemoryConfig{Enabled: true, MaxContextChars: 1000}}
	svc, err := NewChatService(cfg)
	if err != nil {
		t.Fatalf("NewChatService error = %v", err)
	}
	messageRepo := newFakeMessageRepo()
	svc.messageRepo = messageRepo

	ctx := context.Background()
	svc.saveAssistantMessageWithTrace(ctx, "s1", "上一轮答案", 0, 10, nil, []Source{
		{DocID: "doc-1", Content: "上一轮证据", Metadata: map[string]any{"source": "evidence.md"}},
	})

	cc := svc.prepareChatContext(ctx, &ChatRequest{SessionID: "s1", Message: "继续展开这个"})
	if cc.followUpEvidenceCount != 1 {
		t.Fatalf("followUpEvidenceCount = %d, want 1", cc.followUpEvidenceCount)
	}
	if !strings.Contains(cc.runtimeInstruction, "上一轮检索证据") {
		t.Fatalf("runtimeInstruction missing evidence: %s", cc.runtimeInstruction)
	}
}

func TestSaveMessagesRefreshesSessionCache(t *testing.T) {
	baseTime := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
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
	messageRepo.now = func() time.Time { return baseTime }
	cacheStore := newFakeSessionCache()
	svc.messageRepo = messageRepo
	svc.sessionCache = cacheStore

	ctx := context.Background()
	svc.saveUserMessage(ctx, "s2", "old question")
	svc.saveAssistantMessage(ctx, "s2", "assistant answer", 0, 10)
	svc.saveUserMessage(ctx, "s2", "latest question")

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
	if cachedMessages[0].Content != "assistant answer" || cachedMessages[1].Content != "latest question" {
		t.Fatalf("unexpected cached order/content: %#v", cachedMessages)
	}

	instruction := svc.buildMemoryInstruction(ctx, &ChatRequest{}, "s2")
	if strings.Contains(instruction, "old question") {
		t.Fatalf("instruction should not include evicted message: %s", instruction)
	}
	if !strings.Contains(instruction, "assistant answer") || !strings.Contains(instruction, "latest question") {
		t.Fatalf("instruction missing refreshed cached messages: %s", instruction)
	}
	if messageRepo.createCalls != 3 {
		t.Fatalf("expected 3 persisted messages, got %d", messageRepo.createCalls)
	}
}
