package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/config"
)

// turn scripts one model call as a list of chunks. A nil chunk is a gate: the
// stream stops there until release is closed, which lets a test observe what the
// consumer has already received while the model is still "generating".
type turn struct {
	chunks  []*schema.Message
	release chan struct{}
}

type scriptedModel struct {
	mu    sync.Mutex
	turns []turn
	calls int
	seen  [][]*schema.Message
}

func (m *scriptedModel) next(in []*schema.Message) turn {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen = append(m.seen, in)
	if m.calls >= len(m.turns) {
		m.calls++
		return turn{chunks: []*schema.Message{{Role: schema.Assistant, Content: "unscripted"}}}
	}
	t := m.turns[m.calls]
	m.calls++
	return t
}

func (m *scriptedModel) Stream(ctx context.Context, in []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	t := m.next(in)
	sr, sw := schema.Pipe[*schema.Message](1)
	go func() {
		defer sw.Close()
		for _, c := range t.chunks {
			if c == nil {
				select {
				case <-t.release:
				case <-ctx.Done():
					return
				}
				continue
			}
			if closed := sw.Send(c, nil); closed {
				return
			}
		}
	}()
	return sr, nil
}

func (m *scriptedModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	sr, err := m.Stream(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	var all []*schema.Message
	for {
		c, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			return schema.ConcatMessages(all)
		}
		if err != nil {
			return nil, err
		}
		all = append(all, c)
	}
}

func (m *scriptedModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func text(s string) *schema.Message { return &schema.Message{Role: schema.Assistant, Content: s} }

func callTool(id, name, args string) *schema.Message {
	return &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{
		Index: ptr(0), ID: id, Type: "function", Function: schema.FunctionCall{Name: name, Arguments: args},
	}}}
}

func ptr[T any](v T) *T { return &v }

// lookupTool implements only InvokableTool, like every tool in this project.
type lookupTool struct{ runs atomic.Int32 }

func (l *lookupTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "lookup", Desc: "look something up"}, nil
}

func (l *lookupTool) InvokableRun(context.Context, string, ...einotool.Option) (string, error) {
	l.runs.Add(1)
	return "found it", nil
}

// streamOnlyTool implements only StreamableTool.
type streamOnlyTool struct{}

func (streamOnlyTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "tail", Desc: "stream something"}, nil
}

func (streamOnlyTool) StreamableRun(context.Context, string, ...einotool.Option) (*schema.StreamReader[string], error) {
	return schema.StreamReaderFromArray([]string{"a", "b"}), nil
}

type bothTool struct{ lookupTool }

func (*bothTool) StreamableRun(context.Context, string, ...einotool.Option) (*schema.StreamReader[string], error) {
	return schema.StreamReaderFromArray([]string{"x"}), nil
}

type eventLog struct {
	mu     sync.Mutex
	events []StreamEvent
}

func (l *eventLog) add(e StreamEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, e)
}

func (l *eventLog) types() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, e := range l.events {
		out = append(out, e.Type)
	}
	return out
}

func newTestRun(t *testing.T, m model.ToolCallingChatModel, peek int, tools ...einotool.BaseTool) (*agentRun, *eventLog) {
	t.Helper()
	log := &eventLog{}
	wrapped := wrapAgentTools(tools, log.add)
	agent, err := react.NewAgent(context.Background(), &react.AgentConfig{
		ToolCallingModel:      m,
		ToolsConfig:           compose.ToolsNodeConfig{Tools: wrapped},
		MaxStep:               12,
		StreamToolCallChecker: newStreamToolCallChecker(peek),
	})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}
	return &agentRun{agent: agent, tools: wrapped}, log
}

func userMessage() []*schema.Message {
	return []*schema.Message{{Role: schema.User, Content: "question"}}
}

// A wrapper must not claim a way of running that its base lacks. Eino picks the
// streaming entry point of a tool that has one, and adapts a tool that has only
// an invokable one; a wrapper that declares both turns that adaptation off.
func TestEventToolDeclaresOnlyWhatTheBaseCanDo(t *testing.T) {
	wrapped := wrapAgentTools([]einotool.BaseTool{&lookupTool{}, streamOnlyTool{}, &bothTool{}}, func(StreamEvent) {})
	type caps struct{ invokable, streamable bool }
	want := []caps{{true, false}, {false, true}, {true, true}}
	for i, w := range wrapped {
		_, inv := w.(einotool.InvokableTool)
		_, str := w.(einotool.StreamableTool)
		if got := (caps{inv, str}); got != want[i] {
			info, _ := w.Info(context.Background())
			t.Errorf("tool %s: invokable=%v streamable=%v, want %+v", info.Name, inv, str, want[i])
		}
	}
}

func TestReActStreamRunsInvokableOnlyTools(t *testing.T) {
	lookup := &lookupTool{}
	m := &scriptedModel{turns: []turn{
		{chunks: []*schema.Message{callTool("c1", "lookup", `{}`)}},
		{chunks: []*schema.Message{text("the "), text("answer")}},
	}}
	run, log := newTestRun(t, m, 64, lookup)

	var got strings.Builder
	msg, err := streamAgent(context.Background(), run, userMessage(), func(s string) bool { got.WriteString(s); return true })
	if err != nil {
		t.Fatalf("streamAgent: %v", err)
	}
	if got.String() != "the answer" || msg.Content != "the answer" {
		t.Fatalf("streamed %q, message %q", got.String(), msg.Content)
	}
	if lookup.runs.Load() != 1 {
		t.Fatalf("tool ran %d times, want 1", lookup.runs.Load())
	}
	types := log.types()
	if len(types) != 2 || types[0] != "action" || types[1] != "observation" {
		t.Fatalf("tool events = %v, want [action observation]", types)
	}
}

// The point of streaming: the consumer sees the start of the answer while the
// model is still producing the rest of it.
func TestAnswerReachesTheConsumerWhileTheModelIsStillWriting(t *testing.T) {
	release := make(chan struct{})
	m := &scriptedModel{turns: []turn{
		{chunks: []*schema.Message{text("first part of the answer, "), nil, text("second part")}, release: release},
	}}
	run, _ := newTestRun(t, m, 5)

	first := make(chan string, 8)
	done := make(chan error, 1)
	go func() {
		_, err := streamAgent(context.Background(), run, userMessage(), func(s string) bool { first <- s; return true })
		done <- err
	}()
	select {
	case s := <-first:
		if !strings.HasPrefix(s, "first part") {
			t.Fatalf("first delta = %q", s)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("nothing reached the consumer while the model was blocked: the answer is buffered until the model finishes")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// Text before a tool call is a turn of tool calls, as long as the call arrives
// inside the peek window.
func TestTextBeforeAToolCallInsideThePeekWindowIsNotTheAnswer(t *testing.T) {
	lookup := &lookupTool{}
	m := &scriptedModel{turns: []turn{
		{chunks: []*schema.Message{text("let me check. "), callTool("c1", "lookup", `{}`)}},
		{chunks: []*schema.Message{text("done")}},
	}}
	run, _ := newTestRun(t, m, 200, lookup)

	var got strings.Builder
	if _, err := streamAgent(context.Background(), run, userMessage(), func(s string) bool { got.WriteString(s); return true }); err != nil {
		t.Fatal(err)
	}
	if got.String() != "done" {
		t.Fatalf("streamed %q, want only the final answer", got.String())
	}
	if lookup.runs.Load() != 1 {
		t.Fatalf("tool ran %d times, want 1", lookup.runs.Load())
	}
}

// A model that writes more than the peek window before it calls a tool has
// already been taken for answering. The tool call must not be dropped: it is
// executed and the run continues.
func TestToolCallAfterThePeekWindowIsStillExecuted(t *testing.T) {
	lookup := &lookupTool{}
	preamble := strings.Repeat("thinking out loud ", 8)
	m := &scriptedModel{turns: []turn{
		{chunks: []*schema.Message{text(preamble), callTool("c1", "lookup", `{}`)}},
		{chunks: []*schema.Message{text("the real answer")}},
	}}
	run, _ := newTestRun(t, m, 20, lookup)

	var got strings.Builder
	msg, err := streamAgent(context.Background(), run, userMessage(), func(s string) bool { got.WriteString(s); return true })
	if err != nil {
		t.Fatal(err)
	}
	if lookup.runs.Load() != 1 {
		t.Fatalf("tool ran %d times, want 1", lookup.runs.Load())
	}
	if msg.Content != "the real answer" {
		t.Fatalf("final message = %q", msg.Content)
	}
	if !strings.HasSuffix(got.String(), "the real answer") {
		t.Fatalf("streamed %q, want it to end with the answer", got.String())
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	last := m.seen[len(m.seen)-1]
	if len(last) < 3 || last[len(last)-1].Role != schema.Tool || last[len(last)-1].Content != "found it" {
		t.Fatalf("the continuation did not carry the tool result: %+v", last)
	}
}

func TestLateToolCallsDoNotLoopForever(t *testing.T) {
	lookup := &lookupTool{}
	preamble := strings.Repeat("blah ", 20)
	var turns []turn
	for i := 0; i < 20; i++ {
		turns = append(turns, turn{chunks: []*schema.Message{text(preamble), callTool("c", "lookup", `{}`)}})
	}
	run, _ := newTestRun(t, &scriptedModel{turns: turns}, 10, lookup)
	_, err := streamAgent(context.Background(), run, userMessage(), func(string) bool { return true })
	if err == nil {
		t.Fatal("expected an error once the continuation limit is reached")
	}
	if int(lookup.runs.Load()) > maxLateToolRounds+1 {
		t.Fatalf("tool ran %d times", lookup.runs.Load())
	}
}

func TestStreamAgentStopsWhenTheConsumerIsGone(t *testing.T) {
	m := &scriptedModel{turns: []turn{{chunks: []*schema.Message{text("a"), text("b"), text("c")}}}}
	run, _ := newTestRun(t, m, 1)
	n := 0
	_, err := streamAgent(context.Background(), run, userMessage(), func(string) bool { n++; return false })
	if !errors.Is(err, errStreamConsumerGone) {
		t.Fatalf("err = %v, want errStreamConsumerGone", err)
	}
	if n != 1 {
		t.Fatalf("emit called %d times after it returned false", n)
	}
}

func TestStreamToolCallCheckerPeekWindow(t *testing.T) {
	check := func(peek int, chunks ...*schema.Message) bool {
		got, err := newStreamToolCallChecker(peek)(context.Background(), schema.StreamReaderFromArray(chunks))
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	long := text(strings.Repeat("x", 50))
	call := callTool("c", "lookup", `{}`)
	cases := []struct {
		name string
		peek int
		in   []*schema.Message
		want bool
	}{
		{"tool call first", 10, []*schema.Message{call}, true},
		{"empty chunks then tool call", 10, []*schema.Message{text(""), text(""), call}, true},
		{"short text then tool call", 100, []*schema.Message{text("hi"), call}, true},
		{"answer longer than the window", 10, []*schema.Message{long, call}, false},
		{"plain answer", 100, []*schema.Message{text("a"), text("b")}, false},
		{"zero window reads everything", 0, []*schema.Message{long, long, call}, true},
	}
	for _, c := range cases {
		if got := check(c.peek, c.in...); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// Repeat-call tracking used to live in a second wrapper around eventTool; it
// must keep working now that the wrappers are chosen by capability.
func TestWrappedToolsStillWarnAboutRepeatedCalls(t *testing.T) {
	log := &eventLog{}
	svc := &ChatService{config: &config.Config{}}
	wrapped := svc.wrapAgentToolsForRequest([]einotool.BaseTool{&lookupTool{}}, &ChatRequest{}, log.add)
	inv := wrapped[0].(einotool.InvokableTool)
	for i := 0; i < 3; i++ {
		if _, err := inv.InvokableRun(context.Background(), `{"q":"same"}`); err != nil {
			t.Fatal(err)
		}
	}
	warnings := 0
	for _, ty := range log.types() {
		if ty == "warning" {
			warnings++
		}
	}
	if warnings != 1 {
		t.Fatalf("got %d warnings in %v, want exactly 1", warnings, log.types())
	}
}
