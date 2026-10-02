package compaction

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// scriptedSummarizer answers every call with a fixed text, or fails.
type scriptedSummarizer struct {
	mu      sync.Mutex
	prompts []string
	text    string
	err     error
	cut     bool
}

func (s *scriptedSummarizer) Summarize(_ context.Context, _, prompt string, _ int) (Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts = append(s.prompts, prompt)
	if s.err != nil {
		return Summary{}, s.err
	}
	return Summary{Text: s.text, Truncated: s.cut}, nil
}

func (s *scriptedSummarizer) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.prompts)
}

func user(text string) *schema.Message { return &schema.Message{Role: schema.User, Content: text} }
func asst(text string) *schema.Message {
	return &schema.Message{Role: schema.Assistant, Content: text}
}
func call(id, name string) *schema.Message {
	return &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: id, Function: schema.FunctionCall{Name: name, Arguments: `{"q":"x"}`}}}}
}
func result(id, text string) *schema.Message {
	return &schema.Message{Role: schema.Tool, ToolCallID: id, ToolName: "search", Content: text}
}
func sys() *schema.Message { return &schema.Message{Role: schema.System, Content: "be helpful"} }

func words(n int) string { return strings.Repeat("alpha beta gamma delta ", n) }

func smallSettings() Settings {
	return Settings{Enabled: true, MaxContextTokens: 2000, ReserveTokens: 400, KeepRecentTokens: 300}
}

func TestEstimateTreatsCJKAsDenserThanLatin(t *testing.T) {
	zh := EstimateString(strings.Repeat("知识库问答", 20))
	en := EstimateString(strings.Repeat("hello", 20))
	if zh <= en {
		t.Fatalf("100 CJK characters (%d) should cost more than 100 Latin characters (%d)", zh, en)
	}
	if EstimateString("") != 0 {
		t.Fatal("empty string must cost nothing")
	}
}

func TestEstimateCountsReasoningAndToolCalls(t *testing.T) {
	plain := EstimateMessage(asst("ok"))
	withReasoning := EstimateMessage(&schema.Message{Role: schema.Assistant, Content: "ok", ReasoningContent: words(50)})
	if withReasoning < plain+100 {
		t.Fatalf("reasoning content was not counted: %d vs %d", withReasoning, plain)
	}
	if EstimateMessage(call("1", "search")) <= EstimateMessage(asst("")) {
		t.Fatal("tool call was not counted")
	}
}

func TestCutPointNeverSeparatesToolResultFromItsCall(t *testing.T) {
	msgs := []*schema.Message{
		sys(), user("q1"), asst(words(40)), user("q2"),
		call("a", "search"), result("a", words(60)), call("b", "search"), result("b", words(60)), asst("done"),
	}
	for _, keep := range []int{50, 120, 250, 400, 800} {
		cut := FindCutPoint(msgs, 1, keep)
		if msgs[cut.FirstKeptIdx].Role == schema.Tool {
			t.Fatalf("keep=%d: cut lands on a tool result at %d", keep, cut.FirstKeptIdx)
		}
	}
}

func TestCutPointBudgetIsACeilingNotAFloor(t *testing.T) {
	msgs := []*schema.Message{sys(), user("q"), call("a", "search"), result("a", words(400)), asst("short")}
	cut := FindCutPoint(msgs, 1, 100)
	var kept int
	for _, m := range msgs[cut.FirstKeptIdx:] {
		kept += EstimateMessage(m)
	}
	if kept > 100 {
		t.Fatalf("kept %d tokens with a budget of 100: the huge tool result was kept as well", kept)
	}
}

func TestSplitTurnGetsAPrefixSummary(t *testing.T) {
	msgs := []*schema.Message{sys(), user("research the topic")}
	for i := 0; i < 6; i++ {
		id := fmt.Sprint(i)
		msgs = append(msgs, call(id, "search"), result(id, words(40)))
	}
	msgs = append(msgs, asst("partial"))
	fake := &scriptedSummarizer{text: "SUMMARY"}
	c := New(fake, smallSettings())
	res, err := c.Compact(context.Background(), msgs, ReasonThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if !res.SplitTurn || !strings.Contains(res.Summary, "Turn Context (split turn)") {
		t.Fatalf("a turn larger than the budget must be split and explained: %+v", res)
	}
	if res.Messages[0].Role != schema.System || !IsSummary(res.Messages[1]) {
		t.Fatal("result must be system prompt, then the summary")
	}
	if res.Freed() <= 0 {
		t.Fatalf("compaction freed nothing: %+v", res)
	}
	if fake.calls() != 1 {
		t.Fatalf("a split turn with no earlier history needs only the prefix summary, got %d calls", fake.calls())
	}
}

func TestNothingToCompactIsReportedNotRetried(t *testing.T) {
	c := New(&scriptedSummarizer{text: "S"}, smallSettings())
	_, err := c.Compact(context.Background(), []*schema.Message{sys(), user("hi"), asst("hello")}, ReasonThreshold)
	if !errors.Is(err, ErrNothingToCompact) {
		t.Fatalf("got %v", err)
	}
}

func TestSecondCompactionUpdatesThePreviousSummary(t *testing.T) {
	fake := &scriptedSummarizer{text: "FIRST"}
	c := New(fake, smallSettings())
	var msgs []*schema.Message
	msgs = append(msgs, sys())
	for i := 0; i < 8; i++ {
		msgs = append(msgs, user(fmt.Sprintf("question %d %s", i, words(20))), asst(words(20)))
	}
	first, err := c.Compact(context.Background(), msgs, ReasonThreshold)
	if err != nil {
		t.Fatal(err)
	}
	fake.text = "SECOND"
	next := append([]*schema.Message{}, first.Messages...)
	for i := 8; i < 16; i++ {
		next = append(next, user(fmt.Sprintf("question %d %s", i, words(20))), asst(words(20)))
	}
	second, err := c.Compact(context.Background(), next, ReasonThreshold)
	if err != nil {
		t.Fatal(err)
	}
	last := fake.prompts[len(fake.prompts)-1]
	if !strings.Contains(last, "<previous-summary>\nFIRST") || !strings.Contains(last, "NEW conversation messages") {
		t.Fatal("the second compaction must update the first summary, not start over")
	}
	if strings.Contains(strings.SplitN(last, "</conversation>", 2)[0], "compacted into the following summary") {
		t.Fatal("the previous summary envelope was fed back in as conversation")
	}
	if !strings.Contains(second.Summary, "SECOND") {
		t.Fatal("new summary missing")
	}
}

func TestSummariserFailureDegradesToRawArchive(t *testing.T) {
	fake := &scriptedSummarizer{err: errors.New("boom")}
	c := New(fake, smallSettings())
	var msgs []*schema.Message
	msgs = append(msgs, sys())
	for i := 0; i < 8; i++ {
		msgs = append(msgs, user(fmt.Sprintf("question %d %s", i, words(20))), asst(words(20)))
	}
	res, err := c.Compact(context.Background(), msgs, ReasonThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Degraded || !strings.Contains(res.Summary, "Raw conversation archive") {
		t.Fatalf("a failed summariser must degrade to the raw archive: %+v", res)
	}
	// The archive is capped to the summary budget and keeps the newest
	// entries, so the oldest question is the one that gets dropped.
	if !strings.Contains(res.Summary, "question 6") {
		t.Fatalf("the newest summarised entry must survive the cap: %s", res.Summary)
	}
	if strings.Contains(res.Summary, "question 0") {
		t.Fatalf("the oldest entry should have been dropped to fit the budget: %s", res.Summary)
	}
	if got, budget := EstimateString(res.Summary), c.settings.summaryBudget(); got > budget {
		t.Fatalf("degraded summary is %d tokens, over the %d budget", got, budget)
	}
	if fake.calls() != maxSummarizationAttempts {
		t.Fatalf("expected %d attempts, got %d", maxSummarizationAttempts, fake.calls())
	}
}

func TestTruncatedSummaryIsTreatedAsFailure(t *testing.T) {
	fake := &scriptedSummarizer{text: "cut off mid", cut: true}
	c := New(fake, smallSettings())
	var msgs []*schema.Message
	msgs = append(msgs, sys())
	for i := 0; i < 8; i++ {
		msgs = append(msgs, user(words(20)), asst(words(20)))
	}
	res, err := c.Compact(context.Background(), msgs, ReasonThreshold)
	if err != nil || !res.Degraded {
		t.Fatalf("a summary that hit the token cap must not become the checkpoint: %+v %v", res, err)
	}
}

func TestOverflowDetection(t *testing.T) {
	yes := []string{
		"This model's maximum context length is 8192 tokens",
		"Error: prompt is too long: 250000 tokens",
		"range of input length should be [1, 30720]",
		"context_length_exceeded",
	}
	no := []string{
		"ThrottlingException: Too many tokens, please wait before trying again",
		"429 rate limit reached",
		"invalid api key",
		"",
	}
	for _, s := range yes {
		if !IsOverflowError(errors.New(s)) {
			t.Errorf("should be overflow: %q", s)
		}
	}
	for _, s := range no {
		if IsOverflowError(errors.New(s)) {
			t.Errorf("should not be overflow: %q", s)
		}
	}
	if IsOverflowError(nil) {
		t.Error("nil is not an overflow")
	}
}

// recordingModel is a ToolCallingChatModel that records what it was sent.
type recordingModel struct {
	mu      sync.Mutex
	sent    [][]*schema.Message
	failBig int // fail with an overflow error while the request is larger than this
}

func (m *recordingModel) Generate(_ context.Context, in []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, in)
	if m.failBig > 0 && EstimateMessages(in) > m.failBig {
		return nil, errors.New("maximum context length is 4096 tokens")
	}
	return asst("ok"), nil
}

func (m *recordingModel) Stream(ctx context.Context, in []*schema.Message, o ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, in, o...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *recordingModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func bigConversation(rounds int) []*schema.Message {
	msgs := []*schema.Message{sys(), user("research the topic")}
	for i := 0; i < rounds; i++ {
		id := fmt.Sprint(i)
		msgs = append(msgs, call(id, "search"), result(id, words(40)))
	}
	return msgs
}

func TestModelCompactsOnceAndReusesTheSummaryOnLaterRounds(t *testing.T) {
	fake := &scriptedSummarizer{text: "SUMMARY"}
	inner := &recordingModel{}
	var events []Event
	m := WrapModel(inner, New(fake, smallSettings()), func(e Event) { events = append(events, e) })

	msgs := bigConversation(10)
	if _, err := m.Generate(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || fake.calls() == 0 {
		t.Fatalf("expected one compaction, got %d events", len(events))
	}
	sent := inner.sent[0]
	if EstimateMessages(sent) >= EstimateMessages(msgs) {
		t.Fatal("the request was not made smaller")
	}
	callsAfterFirst := fake.calls()

	// The ReAct loop appends one round and calls again with the full list.
	msgs = append(msgs, call("n", "search"), result("n", "short"))
	if _, err := m.Generate(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	if fake.calls() != callsAfterFirst {
		t.Fatalf("the second round summarised again (%d -> %d calls) although it fit", callsAfterFirst, fake.calls())
	}
	again := inner.sent[1]
	if !IsSummary(again[1]) || !strings.Contains(again[1].Content, "SUMMARY") {
		t.Fatal("the earlier summary was not reused")
	}
	if again[len(again)-1].Content != "short" {
		t.Fatal("the newest message must be sent verbatim")
	}
}

func TestModelLeavesSmallConversationsAlone(t *testing.T) {
	fake := &scriptedSummarizer{text: "S"}
	inner := &recordingModel{}
	m := WrapModel(inner, New(fake, smallSettings()), nil)
	msgs := []*schema.Message{sys(), user("hi")}
	if _, err := m.Generate(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	if fake.calls() != 0 || len(inner.sent[0]) != 2 {
		t.Fatal("a small conversation must pass through untouched")
	}
}

func TestModelRepairsAProviderOverflowWithOneCompactionAndRetry(t *testing.T) {
	fake := &scriptedSummarizer{text: "S"}
	// Compaction is switched off by threshold, so only the repair path can fire.
	s := smallSettings()
	s.MaxContextTokens = 1_000_000
	inner := &recordingModel{failBig: 1500}
	var reasons []Reason
	m := WrapModel(inner, New(fake, s), func(e Event) { reasons = append(reasons, e.Reason) })
	if _, err := m.Generate(context.Background(), bigConversation(12)); err != nil {
		t.Fatalf("overflow was not repaired: %v", err)
	}
	if len(inner.sent) != 2 {
		t.Fatalf("expected the failing request and one retry, got %d requests", len(inner.sent))
	}
	if len(reasons) != 1 || reasons[0] != ReasonOverflow {
		t.Fatalf("expected one overflow compaction, got %v", reasons)
	}
}

func TestModelDoesNotRetryNonOverflowErrors(t *testing.T) {
	inner := &failingModel{err: errors.New("401 invalid api key")}
	m := WrapModel(inner, New(&scriptedSummarizer{text: "S"}, smallSettings()), nil)
	if _, err := m.Generate(context.Background(), bigConversation(12)); err == nil {
		t.Fatal("expected the error")
	}
	if inner.calls != 1 {
		t.Fatalf("a non-overflow error was retried %d times", inner.calls)
	}
}

type failingModel struct {
	err   error
	calls int
}

func (f *failingModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	f.calls++
	return nil, f.err
}
func (f *failingModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	f.calls++
	return nil, f.err
}
func (f *failingModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return f, nil
}

func TestModelStreamAlsoCompacts(t *testing.T) {
	inner := &recordingModel{}
	m := WrapModel(inner, New(&scriptedSummarizer{text: "S"}, smallSettings()), nil)
	msgs := bigConversation(10)
	sr, err := m.Stream(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	sr.Close()
	if EstimateMessages(inner.sent[0]) >= EstimateMessages(msgs) {
		t.Fatal("the streamed request was not made smaller")
	}
}

func TestModelForgetsASummaryThatBelongsToAnotherConversation(t *testing.T) {
	fake := &scriptedSummarizer{text: "OLD"}
	inner := &recordingModel{}
	m := WrapModel(inner, New(fake, smallSettings()), nil)
	if _, err := m.Generate(context.Background(), bigConversation(10)); err != nil {
		t.Fatal(err)
	}
	other := []*schema.Message{sys(), user("a completely different question")}
	if _, err := m.Generate(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	for _, msg := range inner.sent[1] {
		if IsSummary(msg) {
			t.Fatal("a stale summary leaked into an unrelated conversation")
		}
	}
}

func TestWrapModelWithoutCompactorIsTransparent(t *testing.T) {
	inner := &recordingModel{}
	if WrapModel(inner, nil, nil) != model.ToolCallingChatModel(inner) {
		t.Fatal("a nil compactor must return the model unchanged")
	}
}

func TestSettingsNormalize(t *testing.T) {
	s := Settings{Enabled: true, MaxContextTokens: 8000}.Normalize()
	if s.ReserveTokens != DefaultReserveTokens {
		t.Fatalf("reserve default not applied: %+v", s)
	}
	if s.KeepRecentTokens > (8000-s.ReserveTokens)/4 {
		t.Fatalf("keep-recent must be capped at a quarter of the usable window: %+v", s)
	}
	if !s.ShouldCompact(s.Threshold()+1) || s.ShouldCompact(s.Threshold()) {
		t.Fatal("threshold boundary wrong")
	}
	if (Settings{Enabled: true}).ShouldCompact(1_000_000) {
		t.Fatal("no window means no compaction")
	}
	if New(&scriptedSummarizer{}, Settings{Enabled: true}) != nil {
		t.Fatal("a compactor without a window must be nil")
	}
}

func TestFitArchiveStaysWithinBudgetAndKeepsNewest(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "- User: 第%d条消息，讨论部署流程\n", i)
	}
	got := fitArchive(sb.String(), 800)
	if EstimateString(got) > 900 {
		t.Fatalf("archive is %d tokens, want about 800", EstimateString(got))
	}
	if !strings.Contains(got, "第399条") || strings.Contains(got, "第0条") {
		t.Fatal("the newest entries must survive and the oldest must go")
	}
	if small := "- User: hi\n"; fitArchive(small, 800) != small {
		t.Fatal("a small archive must be left alone")
	}
}

func TestToolResultKeepsItsConclusionForTheSummariser(t *testing.T) {
	long := strings.Repeat("routine detail. ", 400) + "FINAL: the access code is ZX-4401."
	transcript := serialize([]*schema.Message{user("look it up"), call("a", "lookup"), result("a", long)})
	if !strings.Contains(transcript, "ZX-4401") {
		t.Fatal("the end of a long tool result never reached the summariser")
	}
	if !strings.Contains(transcript, "characters omitted") {
		t.Fatal("the omission must be visible to the summariser")
	}
	if got := len([]rune(truncateEnds(long, toolResultMaxChars))); got > toolResultMaxChars+80 {
		t.Fatalf("truncated tool result is %d runes, over the %d cap", got, toolResultMaxChars)
	}
	if short := "ok: done"; truncateEnds(short, toolResultMaxChars) != short {
		t.Fatal("a result under the cap must pass through unchanged")
	}
	if !strings.Contains(rawArchive([]*schema.Message{result("a", long)}), "ZX-4401") {
		t.Fatal("the raw archive dropped the end of the tool result")
	}
}
