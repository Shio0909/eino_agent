package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/approval"
	"eino_agent/internal/config"
	internalTool "eino_agent/internal/tool"
)

// deleteDocTool stands in for a high-risk tool.
type deleteDocTool struct{ lookupTool }

func (*deleteDocTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "delete_document", Desc: "delete a document"}, nil
}

// deleteDocStreamTool is the same tool for a model that streams it.
type deleteDocStreamTool struct{ deleteDocTool }

func (t *deleteDocStreamTool) StreamableRun(context.Context, string, ...einotool.Option) (*schema.StreamReader[string], error) {
	t.runs.Add(1)
	return schema.StreamReaderFromArray([]string{"deleted"}), nil
}

type highRisk struct {
	name string
	tool einotool.BaseTool
	runs func() int32
}

// highRiskKinds are the ways a high-risk tool can be built; each is run through
// both ways of running the agent. Every call returns new tools, which count
// their runs from zero.
func highRiskKinds() []highRisk {
	inv, both := &deleteDocTool{}, &deleteDocStreamTool{}
	return []highRisk{
		{"invokable only", inv, inv.runs.Load},
		{"invokable and streamable", both, both.runs.Load},
	}
}

func approvalService() *ChatService {
	svc := &ChatService{config: &config.Config{}}
	svc.config.HITL.Enabled = true
	svc.config.HITL.HighRiskActions = []string{"delete_document"}
	svc.approvalManager = approval.NewManager(time.Minute)
	return svc
}

// decideWhenAsked answers the first approval request that shows up.
func decideWhenAsked(t *testing.T, svc *ChatService, decision approval.DecisionValue) {
	t.Helper()
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if pending := svc.approvalManager.ListPending(context.Background(), 0); len(pending) > 0 {
				_ = svc.approvalManager.Decide(context.Background(), pending[0].ID, approval.Decision{Decision: decision, Reason: "because"})
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
}

func runWithApproval(t *testing.T, svc *ChatService, streamed bool, tool einotool.BaseTool) (*scriptedModel, *eventLog) {
	t.Helper()
	log := &eventLog{}
	m := &scriptedModel{turns: []turn{
		{chunks: []*schema.Message{callTool("c1", "delete_document", `{"id":"d1"}`)}},
		{chunks: []*schema.Message{text("done")}},
	}}
	run := newTestRunWith(t, m, 64, svc.toolCallMiddlewares(&ChatRequest{}, log.add), tool)
	var err error
	if streamed {
		_, err = streamAgent(context.Background(), run, userMessage(), func(string) bool { return true })
	} else {
		_, err = run.agent.Generate(context.Background(), userMessage())
	}
	if err != nil {
		t.Fatalf("streamed=%v: %v", streamed, err)
	}
	return m, log
}

// A high-risk tool does not run until a person approves the call, on either
// way of running the agent.
func TestHighRiskToolWaitsForApproval(t *testing.T) {
	for k := range highRiskKinds() {
		for _, streamed := range []bool{false, true} {
			kind := highRiskKinds()[k]
			svc := approvalService()
			decideWhenAsked(t, svc, approval.DecisionApprove)
			_, log := runWithApproval(t, svc, streamed, kind.tool)

			if kind.runs() != 1 {
				t.Errorf("%s, streamed=%v: approved tool ran %d times, want 1", kind.name, streamed, kind.runs())
			}
			want := []string{"action", "approval_required", "approval_resolved", "observation"}
			if got := log.types(); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("%s, streamed=%v: events %v, want %v", kind.name, streamed, got, want)
			}
		}
	}
}

// A rejected call is not run, and the model is told so rather than the run failing.
func TestRejectedHighRiskToolIsNotRunAndTheModelIsTold(t *testing.T) {
	for k := range highRiskKinds() {
		for _, streamed := range []bool{false, true} {
			kind := highRiskKinds()[k]
			svc := approvalService()
			decideWhenAsked(t, svc, approval.DecisionReject)
			m, log := runWithApproval(t, svc, streamed, kind.tool)

			if kind.runs() != 0 {
				t.Errorf("%s, streamed=%v: rejected tool ran %d times", kind.name, streamed, kind.runs())
			}
			told := toolMessages(m.seen[1])
			if !strings.Contains(told, "human approval rejected for tool delete_document") || !strings.Contains(told, "because") {
				t.Errorf("%s, streamed=%v: the model was told %q", kind.name, streamed, told)
			}
			want := []string{"action", "approval_required", "approval_rejected", "observation"}
			if got := log.types(); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("%s, streamed=%v: events %v, want %v", kind.name, streamed, got, want)
			}
		}
	}
}

// Tools that are not high-risk are never held up for approval.
func TestOnlyHighRiskToolsNeedApproval(t *testing.T) {
	svc := approvalService()
	lookup := &lookupTool{}
	log := &eventLog{}
	run := newTestRunWith(t, &scriptedModel{turns: []turn{
		{chunks: []*schema.Message{callTool("c1", "lookup", `{}`)}},
		{chunks: []*schema.Message{text("done")}},
	}}, 64, svc.toolCallMiddlewares(&ChatRequest{}, log.add), lookup)
	if _, err := run.agent.Generate(context.Background(), userMessage()); err != nil {
		t.Fatal(err)
	}
	if got := log.types(); strings.Join(got, ",") != "action,observation" {
		t.Fatalf("events %v, want [action observation]", got)
	}
}

// A tool that is run after the model already started its answer goes through
// the same middleware as any other.
func TestLateToolCallIsReportedLikeAnyOther(t *testing.T) {
	lookup := &lookupTool{}
	m := &scriptedModel{turns: []turn{
		{chunks: []*schema.Message{text(strings.Repeat("thinking out loud ", 8)), callTool("c1", "lookup", `{}`)}},
		{chunks: []*schema.Message{text("done")}},
	}}
	run, log := newTestRun(t, m, 20, lookup)
	if _, err := streamAgent(context.Background(), run, userMessage(), func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if lookup.runs.Load() != 1 {
		t.Fatalf("tool ran %d times", lookup.runs.Load())
	}
	if got := log.types(); strings.Join(got, ",") != "action,observation" {
		t.Fatalf("events %v, want the late call reported as [action observation]", got)
	}
}

// The model gets the message, and the event stream still shows the rejected
// call as an error so it can be seen in the trace.
func TestRejectedArgumentsAreAnErrorInTheTrace(t *testing.T) {
	_, tool := validatedLookup()
	m := &scriptedModel{turns: []turn{
		{chunks: []*schema.Message{callTool("c1", "lookup", `not json`)}},
		{chunks: []*schema.Message{text("sorry")}},
	}}
	run, log := newTestRun(t, m, 64, tool)
	if _, err := run.agent.Generate(context.Background(), userMessage()); err != nil {
		t.Fatal(err)
	}
	log.mu.Lock()
	defer log.mu.Unlock()
	if len(log.events) != 2 || log.events[1].Type != "observation" || !strings.Contains(log.events[1].Error, "参数校验失败") {
		t.Fatalf("events %+v, want the observation to carry the validation error", log.events)
	}
}

type badArgsStream struct{}

func (badArgsStream) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "tail", Desc: "stream something"}, nil
}

func (badArgsStream) StreamableRun(context.Context, string, ...einotool.Option) (*schema.StreamReader[string], error) {
	return nil, &internalTool.InvalidArgumentsError{Reason: errors.New("缺少必填参数: topic")}
}

// Streamable tools get the same treatment as invokable ones.
func TestRejectedArgumentsOfAStreamableToolAreReportedToTheModel(t *testing.T) {
	m := &scriptedModel{turns: []turn{
		{chunks: []*schema.Message{callTool("c1", "tail", `{}`)}},
		{chunks: []*schema.Message{text("fixed")}},
	}}
	run, _ := newTestRun(t, m, 64, badArgsStream{})
	msg, err := streamAgent(context.Background(), run, userMessage(), func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "fixed" {
		t.Fatalf("answer %q", msg.Content)
	}
	var told string
	for _, msg := range m.seen[1] {
		if msg.Role == schema.Tool {
			told += msg.Content
		}
	}
	if !strings.Contains(told, "topic") {
		t.Fatalf("the model was told %q", told)
	}
}

type brokenTool struct{ lookupTool }

func (*brokenTool) InvokableRun(context.Context, string, ...einotool.Option) (string, error) {
	return "", errors.New("backend down")
}

// Only the model's own mistakes are handed back to it. A tool that fails is
// still a failed run, not something the model can be expected to fix.
func TestAToolThatFailsStillEndsTheRun(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		m := &scriptedModel{turns: []turn{
			{chunks: []*schema.Message{callTool("c1", "lookup", `{}`)}},
			{chunks: []*schema.Message{text("never reached")}},
		}}
		run, log := newTestRun(t, m, 64, &brokenTool{})
		var err error
		if streamed {
			_, err = streamAgent(context.Background(), run, userMessage(), func(string) bool { return true })
		} else {
			_, err = run.agent.Generate(context.Background(), userMessage())
		}
		if err == nil || !strings.Contains(err.Error(), "backend down") {
			t.Errorf("streamed=%v: err = %v, want the tool's failure", streamed, err)
		}
		log.mu.Lock()
		last := log.events[len(log.events)-1]
		log.mu.Unlock()
		if last.Type != "observation" || last.Error == "" {
			t.Errorf("streamed=%v: last event %+v, want an error observation", streamed, last)
		}
	}
}

// legacyChatModel is what ChatService holds: the older interface, which the
// runtime agent builder narrows to a tool-calling model.
type legacyChatModel struct{ *scriptedModel }

func (legacyChatModel) BindTools([]*schema.ToolInfo) error { return nil }

func runtimeAgentService(m *scriptedModel) *ChatService {
	svc := approvalService()
	svc.config.Agent.EnableKnowledgeTool = true
	svc.config.Agent.MaxSteps = 8
	svc.chatModel = legacyChatModel{m}
	return svc
}

func toolMessages(msgs []*schema.Message) string {
	var out string
	for _, msg := range msgs {
		if msg.Role == schema.Tool {
			out += msg.Content
		}
	}
	return out
}

// The same behaviour has to hold in the agent the server builds for a request,
// where the middlewares are installed by buildRuntimeAgentForRequest and the
// tools are the real, validated ones.
func TestRuntimeAgentGivesBadKnowledgeSearchArgumentsBackToTheModel(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		m := &scriptedModel{turns: []turn{
			{chunks: []*schema.Message{callTool("c1", "knowledge_search", `{}`)}},
			{chunks: []*schema.Message{callTool("c2", "knowledge_search", `{"query":"x"}`)}},
			{chunks: []*schema.Message{text("answered")}},
		}}
		svc := runtimeAgentService(m)
		log := &eventLog{}
		run, _, err := svc.buildRuntimeAgentForRequest(context.Background(),
			fakeRetriever{docs: []*schema.Document{{ID: "d", Content: "hit"}}}, &ChatRequest{}, log.add)
		if err != nil {
			t.Fatal(err)
		}
		var answer string
		if streamed {
			msg, e := streamAgent(context.Background(), run, userMessage(), func(string) bool { return true })
			err = e
			if msg != nil {
				answer = msg.Content
			}
		} else {
			msg, e := run.agent.Generate(context.Background(), userMessage())
			err = e
			if msg != nil {
				answer = msg.Content
			}
		}
		if err != nil || answer != "answered" {
			t.Fatalf("streamed=%v: answer %q, err %v", streamed, answer, err)
		}
		if told := toolMessages(m.seen[1]); !strings.Contains(told, "query") || !strings.Contains(told, "参数校验失败") {
			t.Errorf("streamed=%v: the model was told %q", streamed, told)
		}
	}
}

func TestRuntimeAgentHoldsAHighRiskToolForApproval(t *testing.T) {
	m := &scriptedModel{turns: []turn{
		{chunks: []*schema.Message{callTool("c1", "delete_document", `{}`)}},
		{chunks: []*schema.Message{text("understood")}},
	}}
	svc := runtimeAgentService(m)
	tool := &deleteDocTool{}
	svc.mcpTools = append(svc.mcpTools, tool)
	decideWhenAsked(t, svc, approval.DecisionReject)

	log := &eventLog{}
	run, _, err := svc.buildRuntimeAgentForRequest(context.Background(), nil, &ChatRequest{}, log.add)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.agent.Generate(context.Background(), userMessage()); err != nil {
		t.Fatal(err)
	}
	if tool.runs.Load() != 0 {
		t.Fatalf("a rejected tool ran %d times", tool.runs.Load())
	}
	if !strings.Contains(toolMessages(m.seen[1]), "human approval rejected") {
		t.Fatalf("the model was told %q", toolMessages(m.seen[1]))
	}
}

// Without an event sink (nothing to report to) the model must still be told
// about a bad call.
func TestRuntimeAgentWithoutAnEventSinkStillRecoversFromBadArguments(t *testing.T) {
	m := &scriptedModel{turns: []turn{
		{chunks: []*schema.Message{callTool("c1", "knowledge_search", `{}`)}},
		{chunks: []*schema.Message{text("answered")}},
	}}
	svc := runtimeAgentService(m)
	run, _, err := svc.buildRuntimeAgentForRequest(context.Background(),
		fakeRetriever{}, &ChatRequest{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := run.agent.Generate(context.Background(), userMessage())
	if err != nil || msg.Content != "answered" {
		t.Fatalf("answer %v, err %v", msg, err)
	}
}
