package compaction_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/compaction"
	"eino_agent/internal/config"
	"eino_agent/internal/container"
)

// These tests call a real model with a deliberately tiny context window, so
// compaction fires after a handful of turns. They are skipped unless
// COMPACTION_LIVE_CONFIG points at a config file with a working llm section:
//
//	COMPACTION_LIVE_CONFIG=configs/config.yaml go test ./internal/compaction -run Live -v -count=1
func liveModel(t *testing.T) model.ToolCallingChatModel {
	t.Helper()
	path := os.Getenv("COMPACTION_LIVE_CONFIG")
	if path == "" {
		t.Skip("set COMPACTION_LIVE_CONFIG to run against a real model")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	m, _, err := container.NewLLMProvider(context.Background(), &cfg.LLM)
	if err != nil {
		t.Fatalf("build llm: %v", err)
	}
	tc, ok := m.(model.ToolCallingChatModel)
	if !ok {
		t.Fatalf("model %T does not support tool calling", m)
	}
	return tc
}

type eventLog struct {
	mu     sync.Mutex
	events []compaction.Event
}

func (l *eventLog) add(e compaction.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, e)
}

func (l *eventLog) all() []compaction.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]compaction.Event(nil), l.events...)
}

func liveSettings() compaction.Settings {
	return compaction.Settings{Enabled: true, MaxContextTokens: 1800, ReserveTokens: 400, KeepRecentTokens: 350}
}

func filler(topic string, n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "Background note %d about %s: this paragraph is routine detail with no decision in it. ", i, topic)
	}
	return sb.String()
}

// Facts planted in the first turns must still be answerable after the turns
// that held them were replaced by a summary.
func TestLiveFactsSurviveCompaction(t *testing.T) {
	llm := liveModel(t)
	var log eventLog
	c := compaction.New(compaction.ModelSummarizer{Model: llm}, liveSettings())
	wrapped := compaction.WrapModel(llm, c, log.add)

	msgs := []*schema.Message{
		{Role: schema.System, Content: "You are a concise project assistant. Answer from the conversation."},
		{Role: schema.User, Content: "Remember this: the project codename is Aurora-7 and the release deadline is 14 November. We must not use Redis."},
		{Role: schema.Assistant, Content: "Noted: codename Aurora-7, deadline 14 November, no Redis."},
	}
	for i := 0; i < 6; i++ {
		msgs = append(msgs,
			&schema.Message{Role: schema.User, Content: fmt.Sprintf("Here is background %d. %s", i, filler("logistics", 14))},
			&schema.Message{Role: schema.Assistant, Content: fmt.Sprintf("Understood, background %d recorded. %s", i, filler("logistics", 6))},
		)
	}
	msgs = append(msgs, &schema.Message{Role: schema.User, Content: "What is the project codename, the release deadline, and which datastore are we forbidden to use? Answer in one line."})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	out, err := wrapped.Generate(ctx, msgs)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	events := log.all()
	if len(events) == 0 {
		t.Fatal("no compaction happened; the window is too large for this conversation")
	}
	for _, e := range events {
		t.Logf("compaction: reason=%s tokens %d->%d messages %d->%d degraded=%v took=%s",
			e.Reason, e.TokensBefore, e.TokensAfter, e.MessagesBefore, e.MessagesAfter, e.Degraded, e.Duration)
		if e.Degraded {
			t.Errorf("the real summariser failed and compaction degraded to the raw archive")
		}
		if e.TokensAfter >= e.TokensBefore {
			t.Errorf("compaction did not shrink the context: %d -> %d", e.TokensBefore, e.TokensAfter)
		}
	}
	t.Logf("answer: %s", out.Content)
	for _, want := range []string{"Aurora-7", "14 November", "Redis"} {
		if !strings.Contains(out.Content, want) {
			t.Errorf("answer lost %q after compaction", want)
		}
	}
}

var reactFacts = map[string]string{
	"alpha": "ZX-4401",
	"beta":  "ZX-7712",
	"gamma": "ZX-9035",
	"delta": "ZX-1268",
}

type reactRun struct {
	answer string
	calls  []string
	err    error
}

// runReAct drives one ReAct loop whose tool results are far larger than the
// window used by the compacting run.
func runReAct(t *testing.T, llm model.ToolCallingChatModel) reactRun {
	t.Helper()
	type lookupIn struct {
		Topic string `json:"topic" jsonschema:"description=one of alpha, beta, gamma, delta"`
	}
	var (
		mu    sync.Mutex
		calls []string
	)
	lookup, err := utils.InferTool("lookup", "Look up the access code for a topic. Returns a long report; the code is stated in the last sentence.",
		func(_ context.Context, in lookupIn) (string, error) {
			mu.Lock()
			calls = append(calls, in.Topic)
			mu.Unlock()
			code, ok := reactFacts[strings.ToLower(strings.TrimSpace(in.Topic))]
			if !ok {
				return "unknown topic", nil
			}
			return filler(in.Topic, 40) + fmt.Sprintf("The access code for %s is %s.", in.Topic, code), nil
		})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := react.NewAgent(context.Background(), &react.AgentConfig{
		ToolCallingModel: llm,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: []tool.BaseTool{lookup}},
		MaxStep:          24,
		MessageModifier: func(_ context.Context, in []*schema.Message) []*schema.Message {
			sys := &schema.Message{Role: schema.System, Content: "You must call the lookup tool for each of the four topics alpha, beta, gamma and delta, one call per round. " +
				"Answer only after all four codes are known. Never guess a code."}
			return append([]*schema.Message{sys}, in...)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	out, err := agent.Generate(ctx, []*schema.Message{{
		Role:    schema.User,
		Content: "Look up the access codes for alpha, beta, gamma and delta, then list all four codes in one line.",
	}})
	run := reactRun{err: err}
	if out != nil {
		run.answer = out.Content
	}
	mu.Lock()
	run.calls = append(run.calls, calls...)
	mu.Unlock()
	return run
}

func (r reactRun) missing() []string {
	var out []string
	for topic, code := range reactFacts {
		if !strings.Contains(r.answer, code) {
			out = append(out, topic)
		}
	}
	return out
}

// A ReAct loop is one turn with many rounds. Large tool results must be
// compacted mid-turn and the loop must still finish with every fact it
// collected. The control runs the same loop without compaction, so a failure
// of the model itself is not blamed on the compactor.
//
// "realistic" sizes the window so one tool result (~700 tokens) is a fraction
// of it, and is asserted. "stress" makes one tool result half the usable
// window: compaction then fires almost every round because even the minimum
// context sits above the threshold. It is only logged, as a record of that
// limit.
func TestLiveReActLoopSurvivesLargeToolResults(t *testing.T) {
	llm := liveModel(t)
	const trials = 3

	controlOK := 0
	for i := 0; i < trials; i++ {
		r := runReAct(t, llm)
		ok := r.err == nil && len(r.missing()) == 0
		if ok {
			controlOK++
		}
		t.Logf("control   trial %d: ok=%v err=%v calls=%v missing=%v", i, ok, r.err, r.calls, r.missing())
	}

	windows := []struct {
		name     string
		settings compaction.Settings
		assert   bool
	}{
		{"realistic", compaction.Settings{Enabled: true, MaxContextTokens: 3200, ReserveTokens: 500, KeepRecentTokens: 700}, true},
		{"stress", liveSettings(), false},
	}
	for _, w := range windows {
		ok, compacted, total := 0, 0, 0
		for i := 0; i < trials; i++ {
			var log eventLog
			c := compaction.New(compaction.ModelSummarizer{Model: llm}, w.settings)
			r := runReAct(t, compaction.WrapModel(llm, c, log.add))
			events := log.all()
			for _, e := range events {
				t.Logf("  [%s] compaction: reason=%s tokens %d->%d messages %d->%d degraded=%v split_turn=%v took=%s",
					w.name, e.Reason, e.TokensBefore, e.TokensAfter, e.MessagesBefore, e.MessagesAfter, e.Degraded, e.SplitTurn, e.Duration)
			}
			done := r.err == nil && len(r.missing()) == 0
			t.Logf("%s trial %d: ok=%v err=%v compactions=%d calls=%v missing=%v answer=%q",
				w.name, i, done, r.err, len(events), r.calls, r.missing(), r.answer)
			total++
			if len(events) > 0 {
				compacted++
				if done {
					ok++
				}
			}
		}
		t.Logf("%s: control %d/%d; %d/%d trials compacted, of which %d completed with every code", w.name, controlOK, trials, compacted, total, ok)
		if !w.assert {
			continue
		}
		if compacted == 0 {
			t.Fatalf("%s: no trial reached the window; the scenario does not exercise compaction", w.name)
		}
		if ok < compacted {
			t.Errorf("%s: %d of %d compacted trials failed to finish with every code", w.name, compacted-ok, compacted)
		}
	}
}

// The baseline mechanism depends on the provider reporting usage on both
// Generate and Stream. This checks that it does, and logs how far the
// character-based estimate was from the provider's count.
func TestLiveProviderReportsUsageForBothCallStyles(t *testing.T) {
	llm := liveModel(t)
	var mu sync.Mutex
	var reports []compaction.UsageReport
	s := compaction.Settings{Enabled: true, MaxContextTokens: 100000, ReserveTokens: 4096, CompletionTokens: 256}
	wrapped := compaction.WrapModelWithHooks(llm, compaction.New(compaction.ModelSummarizer{Model: llm}, s),
		compaction.Hooks{OnUsage: func(u compaction.UsageReport) { mu.Lock(); reports = append(reports, u); mu.Unlock() }})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	msgs := []*schema.Message{
		{Role: schema.System, Content: "You are a concise assistant."},
		{Role: schema.User, Content: "用一句话说明什么是上下文窗口。" + filler("memory", 20)},
	}
	out, err := wrapped.Generate(ctx, msgs)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	msgs = append(msgs, out, &schema.Message{Role: schema.User, Content: "再用一句话举个例子。"})
	sr, err := wrapped.Stream(ctx, msgs)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	for {
		if _, err := sr.Recv(); err != nil {
			break
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for i, r := range reports {
		t.Logf("call %d: predicted=%d provider_prompt=%d completion=%d drift=%.1f%% from_usage=%v",
			i, r.Predicted, r.Prompt, r.Completion, r.DriftPercent(), r.FromUsage)
	}
	if len(reports) != 2 {
		t.Fatalf("got %d usage reports, want one for Generate and one for Stream (does the provider send usage when streaming?)", len(reports))
	}
	if !reports[1].FromUsage {
		t.Error("the streamed call should have started from the Generate call's usage")
	}
}
