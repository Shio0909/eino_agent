package compaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// minFreedFraction is the reciprocal of the share of the context a compaction
// must reclaim to count as having worked. Below it the summarisation call costs
// more than the room it bought (WeKnora: observe.go).
const minFreedFraction = 20 // 5%

// Event reports one compaction to the caller, for tracing and logs.
type Event struct {
	Reason         Reason
	TokensBefore   int
	TokensAfter    int
	MessagesBefore int
	MessagesAfter  int
	Degraded       bool
	SplitTurn      bool
	Duration       time.Duration
}

// UsageReport compares what the wrapper expected a request to cost with what
// the provider charged. The gap is the signal that the estimate is missing
// something: under-estimating is how a request gets sent that the provider
// then rejects for size, over-estimating is how compaction fires on a context
// that fits.
type UsageReport struct {
	// Predicted is the wrapper's prompt estimate before the call.
	Predicted int
	// FromUsage reports that Predicted started from the provider's count of the
	// previous call. Without it Predicted is a message-only estimate, which
	// leaves out tool schemas by construction.
	FromUsage                 bool
	Prompt, Completion, Total int
	// ToolSchemaTokens is the estimated cost of the tool definitions sent with
	// every request, to help read a gap on a first round.
	ToolSchemaTokens int
}

// DriftPercent is how far the prediction was from the provider's prompt count:
// negative when the estimate was low.
func (u UsageReport) DriftPercent() float64 {
	if u.Prompt <= 0 {
		return 0
	}
	return float64(u.Predicted-u.Prompt) / float64(u.Prompt) * 100
}

// Hooks are optional callbacks for tracing and logs.
type Hooks struct {
	OnCompact func(Event)
	OnUsage   func(UsageReport)
}

// state is what one agent run remembers between model calls. The ReAct loop
// hands the full message list to the model every round, so without this each
// round past the threshold would pay for another summarisation.
type state struct {
	mu sync.Mutex
	// summary replaces the first `covered` messages after the system prompt.
	summary string
	covered int
	// anchor identifies the first covered message. A list that does not start
	// with it is a different conversation, and the summary must not apply.
	anchor string

	// exhaustedAt is the input length at which compaction last had nothing to
	// do or freed too little. The same length will not be tried again; a longer
	// one has new history and is.
	exhaustedAt int

	// usageTokens and sentCount are the provider's count for the last request
	// (prompt plus the reply it produced) and how many messages that request
	// held. 0 means no baseline. A compaction or a trim changes what the next
	// request holds, so it clears the baseline.
	usageTokens int
	sentCount   int

	// limitRepaired is set once a length-stopped response has been repaired:
	// the heuristic can misfire, and each repair costs a summarisation.
	limitRepaired bool
}

func (st *state) resetBaseline() { st.usageTokens, st.sentCount = 0, 0 }

// estimate is the size of the next request. With a baseline, the provider's
// count already includes the system prompt, tool schemas and the previous reply,
// so only the messages appended since are estimated. The previous reply sits at
// sentCount and is skipped: the baseline counted it as completion tokens.
func (st *state) estimate(view []*schema.Message) (tokens int, fromUsage bool) {
	if st.usageTokens > 0 && st.sentCount > 0 && st.sentCount <= len(view) {
		start := st.sentCount
		if start < len(view) && view[start] != nil && view[start].Role == schema.Assistant {
			start++
		}
		return st.usageTokens + EstimateMessages(view[start:]), true
	}
	return EstimateMessages(view), false
}

// Model wraps a tool-calling chat model so that every request fits the context
// window: it compacts before the call when the estimate crosses the threshold,
// and repairs one provider-reported overflow with a compaction and a retry.
//
// One Model belongs to one agent run, because it remembers that run's summary.
type Model struct {
	inner      model.ToolCallingChatModel
	compactor  *Compactor
	st         *state
	hooks      Hooks
	toolTokens int
}

// WrapModel returns inner unchanged when compactor is nil.
func WrapModel(inner model.ToolCallingChatModel, c *Compactor, onCompact func(Event)) model.ToolCallingChatModel {
	return WrapModelWithHooks(inner, c, Hooks{OnCompact: onCompact})
}

// WrapModelWithHooks is WrapModel with every callback available.
func WrapModelWithHooks(inner model.ToolCallingChatModel, c *Compactor, h Hooks) model.ToolCallingChatModel {
	if c == nil {
		return inner
	}
	return &Model{inner: inner, compactor: c, st: &state{}, hooks: h}
}

func (m *Model) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	cp := *m
	cp.inner = bound
	cp.toolTokens = estimateToolSchemas(tools)
	return &cp, nil
}

func estimateToolSchemas(tools []*schema.ToolInfo) int {
	n := 0
	for _, t := range tools {
		if t == nil {
			continue
		}
		n += EstimateString(t.Name) + EstimateString(t.Desc) + perToolCallOverhead
		if js, err := t.ParamsOneOf.ToJSONSchema(); err == nil && js != nil {
			if b, err := js.MarshalJSON(); err == nil {
				n += EstimateString(string(b))
			}
		}
	}
	return n
}

// request is one fitted model call: what to send, and what it was expected to cost.
type request struct {
	msgs      []*schema.Message
	predicted int
	fromUsage bool
}

func (m *Model) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	req := m.fit(ctx, input, false)
	out, err := m.inner.Generate(ctx, Sanitize(req.msgs), opts...)
	if err != nil {
		if !IsOverflowError(err) {
			return out, err
		}
		retry := m.fit(ctx, input, true)
		if !smaller(retry.msgs, req.msgs) {
			return out, err
		}
		out, err = m.inner.Generate(ctx, Sanitize(retry.msgs), opts...)
		req = retry
	}
	if err != nil {
		return out, err
	}
	m.observe(req, out.ResponseMeta)

	if m.responseHitLimit(out) {
		retry := m.fit(ctx, input, true)
		if smaller(retry.msgs, req.msgs) {
			if repaired, rerr := m.inner.Generate(ctx, Sanitize(retry.msgs), opts...); rerr == nil {
				m.observe(retry, repaired.ResponseMeta)
				return repaired, nil
			}
		}
	}
	return out, nil
}

func (m *Model) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	req := m.fit(ctx, input, false)
	out, err := m.inner.Stream(ctx, Sanitize(req.msgs), opts...)
	if err != nil {
		if !IsOverflowError(err) {
			return nil, err
		}
		retry := m.fit(ctx, input, true)
		if !smaller(retry.msgs, req.msgs) {
			return nil, err
		}
		if out, err = m.inner.Stream(ctx, Sanitize(retry.msgs), opts...); err != nil {
			return nil, err
		}
		req = retry
	}
	return m.observeStream(out, req), nil
}

// responseHitLimit reports whether a finished response says the window was
// full, and claims the one repair a run is allowed. A stream cannot be repaired
// this way: its chunks are already with the consumer by the time the reason is
// known, so only Generate takes this path.
func (m *Model) responseHitLimit(out *schema.Message) bool {
	s := m.compactor.settings
	if !ResponseHitContextLimit(out, s.MaxContextTokens, s.CompletionTokens) {
		return false
	}
	m.st.mu.Lock()
	defer m.st.mu.Unlock()
	if m.st.limitRepaired {
		return false
	}
	m.st.limitRepaired = true
	return true
}

// observe records the provider's count for req as the next baseline and
// reports how the prediction compared.
func (m *Model) observe(req request, meta *schema.ResponseMeta) {
	if meta == nil || meta.Usage == nil {
		return
	}
	u := meta.Usage
	total := u.TotalTokens
	if total <= 0 {
		total = u.PromptTokens + u.CompletionTokens
	}
	if total <= 0 {
		return
	}
	m.st.mu.Lock()
	m.st.usageTokens, m.st.sentCount = total, len(req.msgs)
	m.st.mu.Unlock()

	if m.hooks.OnUsage != nil && u.PromptTokens > 0 {
		m.hooks.OnUsage(UsageReport{
			Predicted: req.predicted, FromUsage: req.fromUsage,
			Prompt: u.PromptTokens, Completion: u.CompletionTokens, Total: total,
			ToolSchemaTokens: m.toolTokens,
		})
	}
}

// observeStream passes chunks through and records the usage the provider sends
// with the last of them once the stream ends. The next request is only built
// after the consumer has drained the stream, so the baseline is in place by then.
func (m *Model) observeStream(in *schema.StreamReader[*schema.Message], req request) *schema.StreamReader[*schema.Message] {
	out, w := schema.Pipe[*schema.Message](1)
	go func() {
		defer w.Close()
		defer in.Close()
		var meta *schema.ResponseMeta
		for {
			chunk, err := in.Recv()
			if err == io.EOF {
				m.observe(req, meta)
				return
			}
			if err != nil {
				w.Send(nil, err)
				return
			}
			if chunk != nil && chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
				meta = chunk.ResponseMeta
			}
			if closed := w.Send(chunk, nil); closed {
				return
			}
		}
	}()
	return out
}

// fit returns the request to make. force compacts regardless of the estimate,
// which is the repair path after a provider rejected the request.
func (m *Model) fit(ctx context.Context, input []*schema.Message, force bool) request {
	sys := historyStart(input)
	rest := input[sys:]
	if len(rest) == 0 {
		return request{msgs: input, predicted: EstimateMessages(input)}
	}

	m.st.mu.Lock()
	defer m.st.mu.Unlock()

	view, hadSummary := input, 0
	if m.st.summary != "" {
		if len(rest) >= m.st.covered && fingerprint(rest[0]) == m.st.anchor {
			view = make([]*schema.Message, 0, len(input)-m.st.covered+1)
			view = append(view, input[:sys]...)
			view = append(view, SummaryMessage(m.st.summary))
			view = append(view, rest[m.st.covered:]...)
			hadSummary = 1
		} else {
			m.st.summary, m.st.covered, m.st.anchor = "", 0, ""
			m.st.exhaustedAt = 0
			m.st.resetBaseline()
		}
	}

	settings := m.compactor.settings
	predicted, fromUsage := m.st.estimate(view)
	reason := ReasonThreshold
	if force {
		reason = ReasonOverflow
	} else if !settings.ShouldCompact(predicted) {
		return request{msgs: view, predicted: predicted, fromUsage: fromUsage}
	}

	compacted := false
	if m.st.exhaustedAt == 0 || len(input) > m.st.exhaustedAt {
		started := time.Now()
		res, err := m.compactor.Compact(ctx, view, reason)
		switch {
		case err != nil:
			m.st.exhaustedAt = len(input)
		case res.Freed() < res.TokensBefore/minFreedFraction:
			m.st.exhaustedAt = len(input)
		default:
			if m.st.summary == "" {
				m.st.anchor = fingerprint(rest[0])
			}
			m.st.covered += res.FirstKeptIdx - sys - hadSummary
			m.st.summary = res.Summary
			m.st.resetBaseline()
			view, compacted = res.Messages, true
			if m.hooks.OnCompact != nil {
				m.hooks.OnCompact(Event{
					Reason: reason, TokensBefore: res.TokensBefore, TokensAfter: res.TokensAfter,
					MessagesBefore: res.MessagesBefore, MessagesAfter: res.MessagesAfter,
					Degraded: res.Degraded, SplitTurn: res.SplitTurn, Duration: time.Since(started),
				})
			}
		}
	}

	trimmedAny := false
	if !compacted || settings.ShouldCompact(EstimateMessages(view)) {
		if trimmed, ok := trimToolResults(view, toolResultBudget(settings.MaxContextTokens)); ok {
			view, trimmedAny = trimmed, true
			m.st.resetBaseline()
		}
	}
	if compacted || trimmedAny {
		// The baseline described a request that no longer exists.
		predicted, fromUsage = m.st.estimate(view)
	}
	return request{msgs: view, predicted: predicted, fromUsage: fromUsage}
}

func fingerprint(msg *schema.Message) string {
	h := sha256.New()
	h.Write([]byte(msg.Role))
	h.Write([]byte{0})
	h.Write([]byte(msg.Content))
	return hex.EncodeToString(h.Sum(nil)[:8])
}

// smaller reports whether a is worth sending instead of b. Repeating an
// identical request would only fail the same way.
func smaller(a, b []*schema.Message) bool { return EstimateMessages(a) < EstimateMessages(b) }
