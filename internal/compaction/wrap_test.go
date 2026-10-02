package compaction

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// usageModel answers from a script and reports provider usage, which
// recordingModel does not.
type usageModel struct {
	mu    sync.Mutex
	sent  [][]*schema.Message
	reply func(n int, in []*schema.Message) *schema.Message
}

func (m *usageModel) Generate(_ context.Context, in []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, in)
	return m.reply(len(m.sent)-1, in), nil
}

// Stream delivers the text first and the usage with a final, contentless chunk,
// which is how OpenAI-compatible providers report it.
func (m *usageModel) Stream(ctx context.Context, in []*schema.Message, o ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, _ := m.Generate(ctx, in, o...)
	return schema.StreamReaderFromArray([]*schema.Message{
		{Role: schema.Assistant, Content: msg.Content},
		{Role: schema.Assistant, ResponseMeta: msg.ResponseMeta},
	}), nil
}

func (m *usageModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) { return m, nil }

func (m *usageModel) requests() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

func replyWith(content string, prompt, completion int, finish string) *schema.Message {
	return &schema.Message{
		Role: schema.Assistant, Content: content,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: finish,
			Usage:        &schema.TokenUsage{PromptTokens: prompt, CompletionTokens: completion, TotalTokens: prompt + completion},
		},
	}
}

func hasSummary(msgs []*schema.Message) bool {
	for _, m := range msgs {
		if IsSummary(m) {
			return true
		}
	}
	return false
}

func TestProviderUsageTriggersCompactionWhereTheEstimateWouldNot(t *testing.T) {
	run := func(reportedPrompt int) (summaries int, compacted bool) {
		fake := &scriptedSummarizer{text: "S"}
		inner := &usageModel{reply: func(int, []*schema.Message) *schema.Message {
			return replyWith("ok", reportedPrompt, 100, "stop")
		}}
		m := WrapModel(inner, New(fake, smallSettings()), nil) // threshold 1600
		msgs := bigConversation(3)                             // about 750 estimated tokens
		if _, err := m.Generate(context.Background(), msgs); err != nil {
			t.Fatal(err)
		}
		// One more ReAct round: the assistant call and a short tool result.
		msgs = append(msgs, call("n", "search"), result("n", "short"))
		if _, err := m.Generate(context.Background(), msgs); err != nil {
			t.Fatal(err)
		}
		return fake.calls(), hasSummary(inner.sent[1])
	}

	// The provider counted tool schemas and system text the estimate cannot see.
	if n, ok := run(1550); n == 0 || !ok {
		t.Fatalf("a provider count of 1550+100 tokens must trigger compaction (summaries=%d, summary sent=%v)", n, ok)
	}
	// Without usage the same conversation looks small and passes through.
	if n, ok := run(0); n != 0 || ok {
		t.Fatalf("with no usage reported the estimate alone must not compact (summaries=%d, summary sent=%v)", n, ok)
	}
}

func TestStreamedUsageBecomesTheBaselineAndIsReportedOnce(t *testing.T) {
	inner := &usageModel{reply: func(int, []*schema.Message) *schema.Message {
		return replyWith("ok", 900, 50, "stop")
	}}
	var reports []UsageReport
	m := WrapModelWithHooks(inner, New(&scriptedSummarizer{text: "S"}, smallSettings()),
		Hooks{OnUsage: func(u UsageReport) { reports = append(reports, u) }})

	drain := func(msgs []*schema.Message) {
		t.Helper()
		sr, err := m.Stream(context.Background(), msgs)
		if err != nil {
			t.Fatal(err)
		}
		for {
			if _, err := sr.Recv(); err == io.EOF {
				return
			} else if err != nil {
				t.Fatal(err)
			}
		}
	}
	msgs := []*schema.Message{sys(), user("hi")}
	drain(msgs)
	if len(reports) != 1 || reports[0].FromUsage || reports[0].Prompt != 900 {
		t.Fatalf("first stream: want one report with no baseline, got %+v", reports)
	}
	drain(append(msgs, asst("ok"), user("and then?")))
	if len(reports) != 2 || !reports[1].FromUsage {
		t.Fatalf("second stream must start from the provider's count, got %+v", reports)
	}
	if reports[1].Predicted < 950 {
		t.Fatalf("prediction %d ignored the 950 tokens the provider already counted", reports[1].Predicted)
	}
}

func TestCompactionThatFreesTooLittleIsDiscardedAndNotRepeated(t *testing.T) {
	// A summary larger than what it replaces frees nothing.
	fake := &scriptedSummarizer{text: words(600)}
	inner := &recordingModel{}
	var events int
	m := WrapModel(inner, New(fake, smallSettings()), func(Event) { events++ })

	msgs := bigConversation(10)
	for range 2 {
		if _, err := m.Generate(context.Background(), msgs); err != nil {
			t.Fatal(err)
		}
	}
	if events != 0 || hasSummary(inner.sent[0]) {
		t.Fatal("a compaction that did not shrink the context must not be applied")
	}
	if fake.calls() != 1 {
		t.Fatalf("the same context was summarised %d times; the second attempt buys nothing", fake.calls())
	}

	// New history is a new chance.
	msgs = append(msgs, call("n", "search"), result("n", words(40)))
	if _, err := m.Generate(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	if fake.calls() != 2 {
		t.Fatalf("a longer context must be tried again, summariser calls=%d", fake.calls())
	}
}

func TestOversizedToolResultIsTrimmedWhenCompactionCannotReachIt(t *testing.T) {
	s := Settings{Enabled: true, MaxContextTokens: 20000, ReserveTokens: 1000, KeepRecentTokens: 300}
	fake := &scriptedSummarizer{text: "S"}
	inner := &recordingModel{}
	m := WrapModel(inner, New(fake, s), nil)

	huge := words(8000) // far beyond the window, and inside the kept tail
	msgs := []*schema.Message{sys(), user("read it"), call("a", "search"), result("a", huge)}
	if _, err := m.Generate(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	sent := inner.sent[0]
	var got *schema.Message
	for _, msg := range sent {
		if msg.Role == schema.Tool {
			got = msg
		}
	}
	if got == nil || got.ToolCallID != "a" {
		t.Fatal("the tool result and its call id must survive trimming")
	}
	if got.Content == huge || !strings.Contains(got.Content, "Tool result compacted") {
		t.Fatal("the oversized tool result was sent whole")
	}
	if EstimateMessages(sent) > s.MaxContextTokens-s.ReserveTokens {
		t.Fatalf("request is still %d tokens, over the %d threshold", EstimateMessages(sent), s.MaxContextTokens-s.ReserveTokens)
	}
	if msgs[3].Content != huge {
		t.Fatal("trimming must not mutate the caller's messages")
	}
}

func TestTrimToolResultsSpendsTheBudgetNewestFirst(t *testing.T) {
	a, b, c := words(200), words(200), words(200)
	msgs := []*schema.Message{
		user("q"), call("1", "search"), result("1", a), call("2", "search"), result("2", b), call("3", "search"), result("3", c),
	}
	one := EstimateMessage(result("1", a))

	if out, ok := trimToolResults(msgs, one*10); ok || &out[0] != &msgs[0] {
		t.Fatal("results within budget must come back untouched")
	}

	out, ok := trimToolResults(msgs, one+one/2)
	if !ok || len(out) != len(msgs) {
		t.Fatalf("trim = %v with %d messages, want true and %d", ok, len(out), len(msgs))
	}
	if out[6].Content != c {
		t.Fatal("the newest result must be kept whole")
	}
	if out[2].Content == a || !strings.Contains(out[2].Content, "Tool result compacted") {
		t.Fatal("the oldest result must be reduced to a marker")
	}
	for i, msg := range out {
		if msg.Role != msgs[i].Role || msg.ToolCallID != msgs[i].ToolCallID {
			t.Fatalf("message %d changed role or call id", i)
		}
	}
	if msgs[2].Content != a {
		t.Fatal("the input was mutated")
	}
	total := 0
	for _, i := range []int{2, 4, 6} {
		total += EstimateMessage(out[i])
	}
	if total > one+one/2 {
		t.Fatalf("tool results total %d tokens, over the %d budget", total, one+one/2)
	}
}

func TestToolResultPreviewKeepsHeadAndTail(t *testing.T) {
	long := "HEAD-MARK " + strings.Repeat("filler text ", 3000) + " TAIL-MARK"
	out := previewToolMessage(result("a", long), 600)
	if !strings.Contains(out.Content, "HEAD-MARK") || !strings.Contains(out.Content, "TAIL-MARK") {
		t.Fatal("a preview must keep both ends")
	}
	if EstimateMessage(out) > 600 {
		t.Fatalf("preview is %d tokens, over its 600 budget", EstimateMessage(out))
	}
}

func TestSanitizeFixesWhatProvidersReject(t *testing.T) {
	summary := SummaryMessage("earlier")
	in := []*schema.Message{
		sys(), summary, user("next question"), user(""), asst("a"),
		call("1", "search"), result("1", "found"),
		result("ghost", "<b>data</b>"),
	}
	in[7].ToolName = "search"
	out := Sanitize(in)

	if out[1].Role != schema.User || !strings.Contains(out[1].Content, "earlier") || !strings.Contains(out[1].Content, "next question") {
		t.Fatalf("the summary and the user message after it must be merged: %+v", out[1])
	}
	for i := 1; i < len(out); i++ {
		carriesCalls := len(out[i].ToolCalls) > 0 || len(out[i-1].ToolCalls) > 0
		if out[i].Role == out[i-1].Role && out[i].Role != schema.Tool && !carriesCalls {
			t.Fatalf("adjacent %s messages at %d", out[i].Role, i)
		}
		if out[i].Content == "" && len(out[i].ToolCalls) == 0 {
			t.Fatalf("empty message at %d", i)
		}
	}
	var calls int
	for _, m := range out {
		calls += len(m.ToolCalls)
	}
	if calls != 1 {
		t.Fatalf("tool calls = %d, want the one call preserved", calls)
	}
	last := out[len(out)-1]
	if last.Role != schema.User || !strings.Contains(last.Content, "untrusted_tool_result") || strings.Contains(last.Content, "<b>") {
		t.Fatalf("an orphaned tool result must become escaped user data: %+v", last)
	}
	if !IsSummary(in[1]) || in[1].Content != summary.Content {
		t.Fatal("sanitising must not mutate its input")
	}
}

func TestSanitizeNeverMergesAMessageThatCarriesToolCalls(t *testing.T) {
	in := []*schema.Message{user("q"), asst("thinking aloud"), call("1", "search"), result("1", "r")}
	out := Sanitize(in)
	if len(out) != 4 || len(out[2].ToolCalls) != 1 {
		t.Fatalf("an assistant message with a tool call was merged away: %d messages", len(out))
	}
}

func TestResponseHitContextLimit(t *testing.T) {
	cases := []struct {
		name   string
		resp   *schema.Message
		window int
		budget int
		want   bool
	}{
		{"nil", nil, 8000, 1000, false},
		{"no metadata", &schema.Message{}, 8000, 1000, false},
		{"normal stop", replyWith("x", 3000, 200, "stop"), 8000, 1000, false},
		{"prompt above the window", replyWith("x", 9000, 200, "stop"), 8000, 1000, true},
		{"length with no room to generate", replyWith("", 7950, 0, "length"), 8000, 1000, true},
		{"length below the requested cap", replyWith("x", 3000, 300, "length"), 8000, 1000, true},
		{"length at the requested cap is an ordinary truncation", replyWith("x", 3000, 1000, "length"), 8000, 1000, false},
		{"length without a requested cap to compare with", replyWith("x", 3000, 300, "length"), 8000, 0, false},
		{"max_tokens spelling", replyWith("x", 3000, 300, "max_tokens"), 8000, 1000, true},
	}
	for _, c := range cases {
		if got := ResponseHitContextLimit(c.resp, c.window, c.budget); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLengthStopIsRepairedOncePerRun(t *testing.T) {
	s := Settings{Enabled: true, MaxContextTokens: 6000, ReserveTokens: 400, KeepRecentTokens: 300, CompletionTokens: 1000}
	fake := &scriptedSummarizer{text: "S"}
	inner := &usageModel{reply: func(n int, in []*schema.Message) *schema.Message {
		if !hasSummary(in) {
			return replyWith("cut off", 1500, 100, "length") // stopped short of the 1000 allowed
		}
		return replyWith("complete answer", 600, 100, "stop")
	}}
	m := WrapModel(inner, New(fake, s), nil)

	msgs := bigConversation(8) // under the threshold, so only the repair can compact
	out, err := m.Generate(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if out.Content != "complete answer" || inner.requests() != 2 {
		t.Fatalf("got %q after %d requests, want the repaired answer after 2", out.Content, inner.requests())
	}

	// A second length stop in the same run is returned as it is.
	inner2 := &usageModel{reply: func(int, []*schema.Message) *schema.Message {
		return replyWith("cut off", 1500, 100, "length")
	}}
	m2 := WrapModel(inner2, New(&scriptedSummarizer{text: "S"}, s), nil)
	for range 3 {
		if _, err := m2.Generate(context.Background(), bigConversation(8)); err != nil {
			t.Fatal(err)
		}
	}
	if inner2.requests() > 4 {
		t.Fatalf("%d requests for 3 rounds: a repair ran more than once", inner2.requests())
	}
}

func TestReserveCoversTheCompletionBudget(t *testing.T) {
	got := Settings{Enabled: true, MaxContextTokens: 32000, CompletionTokens: 4096}.Normalize()
	if got.ReserveTokens != 4096+SafetyTokens {
		t.Fatalf("reserve = %d, want the completion budget plus the safety margin (%d)", got.ReserveTokens, 4096+SafetyTokens)
	}
	small := Settings{Enabled: true, MaxContextTokens: 6000, ReserveTokens: 1000, CompletionTokens: 4096}.Normalize()
	if small.ReserveTokens != 3000 || small.Threshold() != 3000 {
		t.Fatalf("a small window must keep a usable threshold, got reserve=%d threshold=%d", small.ReserveTokens, small.Threshold())
	}
	if plain := (Settings{Enabled: true, MaxContextTokens: 32000}).Normalize(); plain.ReserveTokens != DefaultReserveTokens {
		t.Fatalf("without a completion budget the reserve stays at the default, got %d", plain.ReserveTokens)
	}
}

func TestUsageReportDriftPercent(t *testing.T) {
	if got := (UsageReport{Predicted: 750, Prompt: 1000}).DriftPercent(); got != -25 {
		t.Fatalf("drift = %v, want -25", got)
	}
	if (UsageReport{Predicted: 750}).DriftPercent() != 0 {
		t.Fatal("no provider count means no drift to report")
	}
}

func TestBaselineSkipsTheReplyTheProviderAlreadyCounted(t *testing.T) {
	inner := &usageModel{reply: func(int, []*schema.Message) *schema.Message {
		return replyWith("a long and chatty reply", 900, 50, "stop")
	}}
	var reports []UsageReport
	m := WrapModelWithHooks(inner, New(&scriptedSummarizer{text: "S"}, smallSettings()),
		Hooks{OnUsage: func(u UsageReport) { reports = append(reports, u) }})

	first := []*schema.Message{sys(), user("hi")}
	if _, err := m.Generate(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	next := user("and then?")
	second := append(append([]*schema.Message{}, first...), asst("a long and chatty reply"), next)
	if _, err := m.Generate(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	// 900 prompt + 50 completion covers the reply; only the new user message is added.
	want := 950 + EstimateMessages([]*schema.Message{next})
	if len(reports) != 2 || reports[1].Predicted != want {
		t.Fatalf("predicted %+v, want exactly %d: the reply must not be counted twice", reports, want)
	}
}

func TestPredictionAfterACompactionDoesNotReuseTheOldBaseline(t *testing.T) {
	inner := &usageModel{reply: func(int, []*schema.Message) *schema.Message {
		return replyWith("ok", 1700, 50, "stop") // over the 1600 threshold once added to anything
	}}
	var reports []UsageReport
	m := WrapModelWithHooks(inner, New(&scriptedSummarizer{text: "S"}, smallSettings()),
		Hooks{OnUsage: func(u UsageReport) { reports = append(reports, u) }})

	msgs := []*schema.Message{sys(), user("research"), call("0", "search"), result("0", "r")}
	if _, err := m.Generate(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	msgs = append(msgs, bigConversation(6)[2:]...)
	if _, err := m.Generate(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	sent := inner.sent[1]
	if !hasSummary(sent) {
		t.Fatal("the provider's count must have forced a compaction")
	}
	r := reports[1]
	if r.FromUsage || r.Predicted != EstimateMessages(sent) {
		t.Fatalf("after a compaction the prediction must be a fresh estimate of what was sent (%d), got %+v", EstimateMessages(sent), r)
	}
}

func TestEveryProviderCallIsSanitised(t *testing.T) {
	bad := []*schema.Message{sys(), user(""), user("question"), result("ghost", "<b>x</b>")}
	check := func(label string, sent []*schema.Message) {
		t.Helper()
		for i, msg := range sent {
			if msg.Role == schema.Tool {
				t.Fatalf("%s: orphaned tool result reached the provider at %d", label, i)
			}
			if msg.Content == "" && len(msg.ToolCalls) == 0 {
				t.Fatalf("%s: empty message reached the provider at %d", label, i)
			}
		}
	}

	inner := &usageModel{reply: func(int, []*schema.Message) *schema.Message { return replyWith("ok", 10, 1, "stop") }}
	m := WrapModel(inner, New(&scriptedSummarizer{text: "S"}, smallSettings()), nil)
	if _, err := m.Generate(context.Background(), bad); err != nil {
		t.Fatal(err)
	}
	check("generate", inner.sent[0])

	sr, err := m.Stream(context.Background(), bad)
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := sr.Recv(); err != nil {
			break
		}
	}
	check("stream", inner.sent[1])
}

func TestOnlyOneLengthRepairPerRun(t *testing.T) {
	m := WrapModel(&usageModel{reply: func(int, []*schema.Message) *schema.Message { return nil }},
		New(&scriptedSummarizer{text: "S"}, Settings{Enabled: true, MaxContextTokens: 6000, ReserveTokens: 400, KeepRecentTokens: 300, CompletionTokens: 1000}), nil).(*Model)
	cut := replyWith("cut off", 1500, 100, "length")
	if !m.responseHitLimit(cut) {
		t.Fatal("the first length stop must be repaired")
	}
	if m.responseHitLimit(cut) {
		t.Fatal("a second repair in one run means a misfiring heuristic is paying for summaries every round")
	}
}
