package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/config"
	"eino_agent/internal/database/repository"
)

type fakeSummaryRepo struct {
	mu    sync.Mutex
	items map[string]*repository.SessionSummary
}

func (f *fakeSummaryRepo) Get(_ context.Context, id string) (*repository.SessionSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.items[id]; ok {
		c := *s
		return &c, nil
	}
	return nil, nil
}

func (f *fakeSummaryRepo) Upsert(_ context.Context, s *repository.SessionSummary) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.items == nil {
		f.items = map[string]*repository.SessionSummary{}
	}
	c := *s
	f.items[s.SessionID] = &c
	return nil
}

// summaryModel answers every Generate call with a fixed structured summary and
// records the prompts it was given.
type summaryModel struct {
	mu      sync.Mutex
	prompts []string
	fail    bool
}

func (m *summaryModel) Generate(_ context.Context, in []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(in) > 0 {
		m.prompts = append(m.prompts, in[len(in)-1].Content)
	}
	if m.fail {
		return nil, errors.New("upstream down")
	}
	return &schema.Message{Role: schema.Assistant, Content: fmt.Sprintf("## 目标\n摘要#%d", len(m.prompts))}, nil
}

func (m *summaryModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("not used")
}

func (m *summaryModel) BindTools([]*schema.ToolInfo) error { return nil }

func (m *summaryModel) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.prompts)
}

func newCompactionTestService(t *testing.T, sm *summaryModel) (*ChatService, *fakeMessageRepo, *fakeSummaryRepo) {
	t.Helper()
	cfg := &config.Config{Memory: config.MemoryConfig{
		Enabled:         true,
		WindowSize:      2,
		MaxContextChars: 200,
		Compaction: config.CompactionConfig{
			Enabled:             true,
			MaxContextTokens:    20000,
			ReserveTokens:       512,
			HistoryBudgetTokens: 1500,
		},
	}}
	svc, err := NewChatService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	msgs, sums := newFakeMessageRepo(), &fakeSummaryRepo{}
	svc.messageRepo, svc.summaryRepo, svc.chatModel = msgs, sums, sm
	return svc, msgs, sums
}

var historyClock = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func addHistory(repo *fakeMessageRepo, session string, from, n int) {
	for i := from; i < from+n; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		_ = repo.Create(context.Background(), &repository.Message{
			ID:        fmt.Sprintf("m%03d", i),
			SessionID: session,
			Role:      role,
			Content:   fmt.Sprintf("消息%d: %s", i, strings.Repeat("关于部署流程的详细讨论。", 30)),
			CreatedAt: historyClock.Add(time.Duration(i) * time.Minute),
		})
	}
}

func TestSessionHistoryStaysUnderBudgetAndReusesSummary(t *testing.T) {
	sm := &summaryModel{}
	svc, msgs, sums := newCompactionTestService(t, sm)
	ctx := context.Background()

	addHistory(msgs, "s1", 0, 40)
	h, err := svc.loadSessionHistory(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if h.Summary == "" || h.Compact == nil {
		t.Fatalf("expected a compaction, got %+v", h)
	}
	if len(h.Recent) == 0 || len(h.Recent) >= 40 {
		t.Fatalf("recent = %d messages, want a verbatim tail smaller than the history", len(h.Recent))
	}
	if h.Recent[len(h.Recent)-1].ID != "m039" {
		t.Fatalf("tail must end at the newest message, got %s", h.Recent[len(h.Recent)-1].ID)
	}
	stored, _ := sums.Get(ctx, "s1")
	if stored == nil || stored.CoveredUntilMessageID == "" {
		t.Fatal("summary checkpoint was not stored")
	}
	if h.Recent[0].ID <= stored.CoveredUntilMessageID {
		t.Fatalf("tail starts at %s, inside covered range up to %s", h.Recent[0].ID, stored.CoveredUntilMessageID)
	}

	first := sm.calls()
	if _, err := svc.loadSessionHistory(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	if sm.calls() != first {
		t.Fatalf("an unchanged session re-summarised: %d -> %d calls", first, sm.calls())
	}

	// Growing past the budget again updates the stored summary instead of
	// summarising a summary of a summary.
	addHistory(msgs, "s1", 40, 20)
	h2, err := svc.loadSessionHistory(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if sm.calls() == first {
		t.Fatal("expected a second summarisation after the session grew")
	}
	carried := false
	for _, p := range sm.prompts[first:] {
		carried = carried || strings.Contains(p, "<previous-summary>") && strings.Contains(p, "摘要#1")
	}
	if !carried {
		t.Fatal("an update must carry the previous summary into the prompt")
	}
	stored2, _ := sums.Get(ctx, "s1")
	if stored2.CoveredCount <= stored.CoveredCount {
		t.Fatalf("covered count did not grow: %d -> %d", stored.CoveredCount, stored2.CoveredCount)
	}
	if h2.Recent[len(h2.Recent)-1].ID != "m059" {
		t.Fatalf("tail must end at the newest message, got %s", h2.Recent[len(h2.Recent)-1].ID)
	}
}

func TestSessionHistoryShortSessionIsNotSummarised(t *testing.T) {
	sm := &summaryModel{}
	svc, msgs, sums := newCompactionTestService(t, sm)
	msgs.messages["s1"] = nil
	_ = msgs.Create(context.Background(), &repository.Message{ID: "a", SessionID: "s1", Role: "user", Content: "你好"})
	_ = msgs.Create(context.Background(), &repository.Message{ID: "b", SessionID: "s1", Role: "assistant", Content: "你好，有什么可以帮你？"})

	h, err := svc.loadSessionHistory(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if sm.calls() != 0 || h.Summary != "" || len(h.Recent) != 2 {
		t.Fatalf("short session: calls=%d summary=%q recent=%d", sm.calls(), h.Summary, len(h.Recent))
	}
	if s, _ := sums.Get(context.Background(), "s1"); s != nil {
		t.Fatal("no summary should be stored for a short session")
	}
}

func TestSessionHistorySummariserFailureDegradesToArchive(t *testing.T) {
	sm := &summaryModel{fail: true}
	svc, msgs, sums := newCompactionTestService(t, sm)
	addHistory(msgs, "s1", 0, 40)

	h, err := svc.loadSessionHistory(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if !h.Degraded || h.Summary == "" {
		t.Fatalf("expected a degraded archive summary, got degraded=%v summary=%q", h.Degraded, h.Summary)
	}
	if s, _ := sums.Get(context.Background(), "s1"); s == nil || !s.Degraded {
		t.Fatal("degraded flag should be persisted")
	}
	if got := formatSessionHistory(h); !strings.Contains(got, "降级") {
		t.Fatalf("prompt should say the summary is degraded: %.120s", got)
	}
}

func TestUncoveredMessagesFallsBackToClock(t *testing.T) {
	msgs := []*repository.Message{
		{ID: "x1", CreatedAt: historyClock},
		{ID: "x2", CreatedAt: historyClock.Add(time.Minute)},
		{ID: "x3", CreatedAt: historyClock.Add(2 * time.Minute)},
	}
	got := uncoveredMessages(msgs, &repository.SessionSummary{CoveredUntilMessageID: "gone", CoveredUntilAt: historyClock.Add(time.Minute)})
	if len(got) != 1 || got[0].ID != "x3" {
		t.Fatalf("got %v", got)
	}
	if got := uncoveredMessages(msgs, nil); len(got) != 3 {
		t.Fatalf("no summary should cover nothing, got %d", len(got))
	}
}

func TestBuildMemoryInstructionUsesCompactedHistory(t *testing.T) {
	sm := &summaryModel{}
	svc, msgs, _ := newCompactionTestService(t, sm)
	addHistory(msgs, "s1", 0, 40)

	inst := svc.buildMemoryInstruction(context.Background(), &ChatRequest{}, "s1")
	if !strings.Contains(inst, "【会话摘要") || !strings.Contains(inst, "【会话近期记录】") {
		t.Fatalf("memory should carry summary and recent tail: %.200s", inst)
	}
	if !strings.Contains(inst, "消息39") {
		t.Fatal("newest message must be present verbatim")
	}
}

func TestBuildMemoryInstructionWithoutSummaryRepoKeepsWindowBehaviour(t *testing.T) {
	sm := &summaryModel{}
	svc, msgs, _ := newCompactionTestService(t, sm)
	svc.summaryRepo = nil
	addHistory(msgs, "s1", 0, 4)

	inst := svc.buildMemoryInstruction(context.Background(), &ChatRequest{}, "s1")
	if !strings.Contains(inst, "【会话短期记忆】") {
		t.Fatalf("expected legacy window memory, got %.120s", inst)
	}
	if sm.calls() != 0 {
		t.Fatal("legacy path must not call the model")
	}
}

func TestFormatMemoryMessagesDropsTheOldestTurnsFirst(t *testing.T) {
	var msgs []*repository.Message
	for i := 1; i <= 8; i++ {
		msgs = append(msgs,
			&repository.Message{Role: "user", Content: fmt.Sprintf("问题%d %s", i, strings.Repeat("知识库", 30))},
			&repository.Message{Role: "assistant", Content: fmt.Sprintf("回答%d %s", i, strings.Repeat("检索", 30))})
	}
	// Room for roughly three of the ~100-character lines.
	out := formatMemoryMessages(msgs, 330)
	if !strings.Contains(out, "回答8") || !strings.Contains(out, "问题8") {
		t.Fatalf("the newest turn must survive a tight budget:\n%s", out)
	}
	if strings.Contains(out, "问题1 ") || strings.Contains(out, "问题2 ") {
		t.Fatalf("the oldest turns should be the ones dropped:\n%s", out)
	}
	if n := len([]rune(out)); n > 330 {
		t.Fatalf("output is %d characters, over the 330 budget", n)
	}
	if strings.Index(out, "问题8") > strings.Index(out, "回答8") {
		t.Fatalf("turns must read in chronological order:\n%s", out)
	}
}

func TestFormatMemoryMessagesKeepsTheStartOfAnOversizedNewestMessage(t *testing.T) {
	out := formatMemoryMessages([]*repository.Message{
		{Role: "user", Content: "older"},
		{Role: "user", Content: strings.Repeat("长", 900)},
	}, 50)
	if out == "" || strings.Contains(out, "older") {
		t.Fatalf("want only a truncated newest message, got %q", out)
	}
}
