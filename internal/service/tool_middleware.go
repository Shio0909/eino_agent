package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/approval"
	internalTool "eino_agent/internal/tool"
)

// toolCallMiddlewares is what every tool call of an agent goes through. The
// first middleware is the outermost, so a rejected call has already been
// reported by the event middleware when invalidArgumentsToModel turns it into
// a result for the model.
func toolCallMiddlewares(events *toolEvents) []compose.ToolMiddleware {
	middlewares := []compose.ToolMiddleware{invalidArgumentsToModel()}
	if events != nil {
		middlewares = append(middlewares, events.middleware())
	}
	return middlewares
}

func (s *ChatService) toolCallMiddlewares(req *ChatRequest, sink func(StreamEvent)) []compose.ToolMiddleware {
	if sink == nil {
		return toolCallMiddlewares(nil)
	}
	events := newToolEvents(sink)
	events.approvals = s.approvalManager
	events.approvalFor = func(toolName string) approval.Request { return s.approvalRequestForTool(req, toolName) }
	return toolCallMiddlewares(events)
}

// invalidArgumentsToModel answers a call whose arguments failed validation with
// the reason, as the tool's result. The model made the mistake and can fix it
// on its next step; returning the error instead would end the whole run.
func invalidArgumentsToModel() compose.ToolMiddleware {
	reply := func(err error) (string, bool) {
		var invalid *internalTool.InvalidArgumentsError
		if !errors.As(err, &invalid) {
			return "", false
		}
		return invalid.Error() + "。请按工具的参数定义修正后重新调用。", true
	}
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, in *compose.ToolInput) (*compose.ToolOutput, error) {
				out, err := next(ctx, in)
				if msg, ok := reply(err); ok {
					return &compose.ToolOutput{Result: msg}, nil
				}
				return out, err
			}
		},
		Streamable: func(next compose.StreamableToolEndpoint) compose.StreamableToolEndpoint {
			return func(ctx context.Context, in *compose.ToolInput) (*compose.StreamToolOutput, error) {
				out, err := next(ctx, in)
				if msg, ok := reply(err); ok {
					return &compose.StreamToolOutput{Result: schema.StreamReaderFromArray([]string{msg})}, nil
				}
				return out, err
			}
		},
	}
}

// toolEvents reports the calls an agent makes to its tools: an action event
// when a call starts, an observation when it ends, a warning when the model
// keeps repeating a call, and a human approval first for a high-risk tool.
//
// It is a middleware instead of a wrapper around each tool. Eino calls the
// streaming entry point of a tool that has one and adapts an invokable-only
// tool by itself, and a middleware is applied to whichever of the two a tool
// gets run through, so there is nothing to keep in step with what a tool can do.
type toolEvents struct {
	sink        func(StreamEvent)
	tracker     *toolCallTracker
	approvals   *approval.Manager
	approvalFor func(toolName string) approval.Request
}

func newToolEvents(sink func(StreamEvent)) *toolEvents {
	return &toolEvents{sink: sink, tracker: newToolCallTracker()}
}

func (e *toolEvents) middleware() compose.ToolMiddleware {
	return compose.ToolMiddleware{Invokable: e.invokable, Streamable: e.streamable}
}

func (e *toolEvents) invokable(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
	return func(ctx context.Context, in *compose.ToolInput) (*compose.ToolOutput, error) {
		call := e.begin(in, false)
		proceed, observation, err := call.awaitApproval(ctx)
		if err != nil {
			call.end("error: "+err.Error(), err, map[string]any{"approval_blocked": true})
			return nil, err
		}
		if !proceed {
			call.end(observation, nil, map[string]any{"output_chars": len(observation), "approval_blocked": true})
			return &compose.ToolOutput{Result: observation}, nil
		}
		out, err := next(ctx, in)
		if err != nil {
			call.end("error: "+err.Error(), err, nil)
			return nil, err
		}
		call.end(out.Result, nil, map[string]any{"output_chars": len(out.Result)})
		return out, nil
	}
}

func (e *toolEvents) streamable(next compose.StreamableToolEndpoint) compose.StreamableToolEndpoint {
	return func(ctx context.Context, in *compose.ToolInput) (*compose.StreamToolOutput, error) {
		call := e.begin(in, true)
		proceed, observation, err := call.awaitApproval(ctx)
		if err != nil {
			call.end("error: "+err.Error(), err, map[string]any{"approval_blocked": true})
			return nil, err
		}
		if !proceed {
			call.end(observation, nil, map[string]any{"approval_blocked": true})
			return &compose.StreamToolOutput{Result: schema.StreamReaderFromArray([]string{observation})}, nil
		}
		out, err := next(ctx, in)
		if err != nil {
			call.end("error: "+err.Error(), err, nil)
			return nil, err
		}
		// What the tool writes goes straight to the model, so only the fact that
		// it started can be reported here.
		call.observation("stream started", nil, nil)
		return out, nil
	}
}

// toolCall is one call of a tool, as the events about it describe it.
type toolCall struct {
	*toolEvents
	name, args string
	streamed   bool
	started    time.Time
}

func (e *toolEvents) begin(in *compose.ToolInput, streamed bool) *toolCall {
	c := &toolCall{toolEvents: e, name: in.Name, args: in.Arguments, streamed: streamed, started: time.Now()}
	if e.tracker != nil {
		if warning := e.tracker.record(c.name, c.args); warning != "" {
			e.sink(StreamEvent{Type: "warning", Content: warning})
		}
	}
	c.event("action", "", 0, nil, c.metadata(nil))
	return c
}

func (c *toolCall) metadata(extra map[string]any) map[string]any {
	m := map[string]any{"args_chars": len(c.args)}
	if c.streamed {
		m["streamable"] = true
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func (c *toolCall) elapsedMs() int64 { return time.Since(c.started).Milliseconds() }

// end reports the result of the call, shortened for the event.
func (c *toolCall) end(result string, err error, extra map[string]any) {
	c.observation(summarizeToolOutput(result), err, extra)
}

func (c *toolCall) observation(content string, err error, extra map[string]any) {
	c.event("observation", content, c.elapsedMs(), err, c.metadata(extra))
}

func (c *toolCall) event(eventType, content string, latencyMs int64, err error, metadata map[string]any) {
	step := &TraceStep{
		Type:      eventType,
		Stage:     eventType,
		Content:   content,
		ToolName:  c.name,
		ToolInput: c.args,
		LatencyMs: latencyMs,
		Metadata:  metadata,
	}
	if err != nil {
		step.Level = "error"
		step.Error = err.Error()
	}
	c.sink(StreamEvent{
		Type:      eventType,
		Content:   content,
		ToolName:  c.name,
		ToolInput: c.args,
		Metadata:  metadata,
		LatencyMs: latencyMs,
		Error:     step.Error,
		TraceStep: step,
	})
}

// awaitApproval holds a high-risk tool call until a person has decided on it.
// It reports whether the call may go ahead; when it may not, the string is what
// the model is told instead.
func (c *toolCall) awaitApproval(ctx context.Context) (bool, string, error) {
	if c.approvals == nil || c.approvalFor == nil {
		return true, "", nil
	}
	req := c.approvalFor(c.name)
	if req.Action == "" {
		return true, "", nil
	}
	req.ToolInput = c.args
	item, err := c.approvals.Create(ctx, req)
	if err != nil {
		return false, "", fmt.Errorf("create approval request: %w", err)
	}
	metadata := map[string]any{
		"args_chars":  len(c.args),
		"action_hash": item.ActionHash,
		"source":      item.Source,
	}
	c.approvalEvent("approval_required", item, metadata, "")
	decision, err := c.approvals.Wait(ctx, item.ID)
	if err != nil {
		return false, "", err
	}
	current, _ := c.approvals.Get(ctx, item.ID)
	if current.ID == "" {
		current = item
	}
	switch decision.Decision {
	case approval.DecisionApprove:
		c.approvalEvent("approval_resolved", current, metadata, decision.Reason)
		return true, "", nil
	case approval.DecisionReject:
		c.approvalEvent("approval_rejected", current, metadata, decision.Reason)
		return false, humanDecisionObservation("rejected", c.name, decision.Reason), nil
	case approval.DecisionExpire:
		c.approvalEvent("approval_expired", current, metadata, decision.Reason)
		return false, humanDecisionObservation("expired", c.name, decision.Reason), nil
	default:
		return false, humanDecisionObservation("rejected", c.name, decision.Reason), nil
	}
}

func (c *toolCall) approvalEvent(eventType string, item approval.Approval, metadata map[string]any, decisionReason string) {
	content := item.Reason
	if decisionReason != "" {
		content = decisionReason
	}
	latencyMs := c.elapsedMs()
	c.sink(StreamEvent{
		Type:           eventType,
		Content:        content,
		ToolName:       c.name,
		ToolInput:      c.args,
		ApprovalID:     item.ID,
		ApprovalStatus: string(item.Status),
		Action:         item.Action,
		RiskLevel:      item.RiskLevel,
		Reason:         item.Reason,
		ExpiresAt:      &item.ExpiresAt,
		Metadata:       metadata,
		LatencyMs:      latencyMs,
		TraceStep: &TraceStep{
			Type:      eventType,
			Stage:     eventType,
			Content:   content,
			ToolName:  c.name,
			ToolInput: c.args,
			LatencyMs: latencyMs,
			Metadata:  metadata,
		},
	})
}
