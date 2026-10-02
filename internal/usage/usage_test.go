package usage

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// memStore behaves like the SQL store: Add is atomic and returns the new total.
type memStore struct {
	mu    sync.Mutex
	total Totals
	fail  error
	adds  int
}

func (s *memStore) Add(_ context.Context, d Totals) (Totals, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return Totals{}, s.fail
	}
	s.adds++
	s.total = s.total.plus(d)
	return s.total, nil
}

func (s *memStore) get() Totals { s.mu.Lock(); defer s.mu.Unlock(); return s.total }

func TestRecordKeepsTotalAtLeastTheSumOfItsParts(t *testing.T) {
	var m Meter
	m.Record(100, 20, 0) // provider reported no total
	m.Record(10, 5, 100) // provider total larger than the parts: kept
	m.Record(-5, -5, -5) // nonsense is ignored
	m.Record(0, 0, 0)
	want := Totals{PromptTokens: 110, CompletionTokens: 25, TotalTokens: 120 + 100}
	if got := m.Totals(); got != want {
		t.Fatalf("totals = %+v, want %+v", got, want)
	}
}

func TestFlushPersistsOnlyTheDeltaAndSurvivesStoreFailure(t *testing.T) {
	s := &memStore{total: Totals{TotalTokens: 1000}}
	var m Meter
	if err := m.Flush(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if m.Totals().TotalTokens != 1000 {
		t.Fatalf("history from earlier runs not loaded: %+v", m.Totals())
	}

	m.Record(30, 10, 40)
	s.fail = errors.New("db down")
	if err := m.Flush(context.Background(), s); err == nil {
		t.Fatal("flush must surface the store error")
	}
	if got := m.Totals().TotalTokens; got != 1040 {
		t.Fatalf("a failed flush changed the totals: %d", got)
	}
	m.Record(5, 5, 10)
	s.fail = nil
	if err := m.Flush(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.get().TotalTokens; got != 1050 {
		t.Fatalf("stored %d, want 1050: the delta of the failed flush must not be lost or counted twice", got)
	}
	if m.Totals() != s.get() {
		t.Fatalf("meter %+v and store %+v disagree after a clean flush", m.Totals(), s.get())
	}
}

func TestTwoProcessesSharingAStoreDoNotOverwriteEachOther(t *testing.T) {
	s := &memStore{}
	var a, b Meter
	a.Record(1, 1, 2)
	b.Record(10, 10, 20)
	for _, m := range []*Meter{&a, &b} {
		if err := m.Flush(context.Background(), s); err != nil {
			t.Fatal(err)
		}
	}
	if s.get().TotalTokens != 22 {
		t.Fatalf("stored %d, want 22", s.get().TotalTokens)
	}
	if b.Totals().TotalTokens != 22 {
		t.Fatalf("a flush should refresh the view with the other writer's tokens, got %d", b.Totals().TotalTokens)
	}
}

func TestAttachLoadsHistoryAndFlushesOnShutdown(t *testing.T) {
	s := &memStore{total: Totals{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}}
	var m Meter
	ctx, cancel := context.WithCancel(context.Background())
	stopped := m.Attach(ctx, s, time.Hour)
	if m.Totals().TotalTokens != 10 {
		t.Fatalf("Attach must load history before returning, got %+v", m.Totals())
	}
	m.Record(1, 1, 2)
	cancel()
	<-stopped
	if s.get().TotalTokens != 12 {
		t.Fatalf("shutdown must flush the pending delta, stored %d", s.get().TotalTokens)
	}
}

func TestBeginEndCountsInFlightWorkAndEndIsIdempotent(t *testing.T) {
	var m Meter
	end1, end2 := m.Begin(), m.Begin()
	if m.InFlight() != 2 {
		t.Fatalf("in flight = %d, want 2", m.InFlight())
	}
	end1()
	end1()
	if m.InFlight() != 1 {
		t.Fatalf("a repeated end must not release someone else's slot, in flight = %d", m.InFlight())
	}
	end2()
	if m.InFlight() != 0 {
		t.Fatalf("in flight = %d, want 0", m.InFlight())
	}
}

func TestConcurrentRecordingIsExact(t *testing.T) {
	var m Meter
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				m.Record(2, 1, 3)
			}
		}()
	}
	wg.Wait()
	if got := m.Totals(); got != (Totals{10000, 5000, 15000}) {
		t.Fatalf("lost updates: %+v", got)
	}
}

type stubModel struct {
	reply  *schema.Message
	err    error
	chunks []*schema.Message
	bound  int
}

func (s *stubModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return s.reply, s.err
}

func (s *stubModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if s.err != nil {
		return nil, s.err
	}
	return schema.StreamReaderFromArray(s.chunks), nil
}

func (s *stubModel) BindTools([]*schema.ToolInfo) error { return nil }

// toolStub is a stubModel that also supports WithTools.
type toolStub struct{ *stubModel }

func (t toolStub) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	t.bound++
	return t, nil
}

func withUsage(content string, p, c, total int) *schema.Message {
	return &schema.Message{Role: schema.Assistant, Content: content, ResponseMeta: &schema.ResponseMeta{
		Usage: &schema.TokenUsage{PromptTokens: p, CompletionTokens: c, TotalTokens: total},
	}}
}

func TestGenerateAndStreamAreBothCounted(t *testing.T) {
	var m Meter
	stub := &stubModel{
		reply:  withUsage("hi", 100, 10, 110),
		chunks: []*schema.Message{{Role: schema.Assistant, Content: "he"}, {Role: schema.Assistant, Content: "llo"}, withUsage("", 200, 20, 220)},
	}
	w := WrapChatModel(stub, &m)

	if _, err := w.Generate(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	sr, err := w.Stream(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for {
		c, err := sr.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		text += c.Content
	}
	if text != "hello" {
		t.Fatalf("stream content changed: %q", text)
	}
	if got := m.Totals(); got != (Totals{300, 30, 330}) {
		t.Fatalf("totals = %+v, want {300 30 330}", got)
	}
}

func TestFailedAndUsageLessCallsAreNotCounted(t *testing.T) {
	var m Meter
	w := WrapChatModel(&stubModel{err: errors.New("boom")}, &m)
	if _, err := w.Generate(context.Background(), nil); err == nil {
		t.Fatal("error swallowed")
	}
	if _, err := w.Stream(context.Background(), nil); err == nil {
		t.Fatal("error swallowed")
	}
	w = WrapChatModel(&stubModel{reply: &schema.Message{Content: "no usage"}}, &m)
	if _, err := w.Generate(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !m.Totals().zero() {
		t.Fatalf("counted something: %+v", m.Totals())
	}
}

func TestWrapperOffersOnlyTheCapabilitiesOfTheModel(t *testing.T) {
	var m Meter
	if _, ok := WrapChatModel(&stubModel{}, &m).(model.ToolCallingChatModel); ok {
		t.Fatal("a model without WithTools must not look like a ToolCallingChatModel")
	}

	stub := &stubModel{reply: withUsage("x", 5, 5, 10)}
	tc, ok := WrapChatModel(toolStub{stub}, &m).(model.ToolCallingChatModel)
	if !ok {
		t.Fatal("a tool-calling model must stay one")
	}
	bound, err := tc.WithTools(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bound.Generate(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if m.Totals().TotalTokens != 10 {
		t.Fatalf("a model bound with tools must still be counted, got %+v", m.Totals())
	}
}
