package compaction

import (
	"fmt"

	"github.com/cloudwego/eino/schema"
)

// Trimming is the last resort, after compaction. A compaction cuts between
// messages, so when the context is still over budget the weight is inside the
// keep-recent window: one tool result large enough to matter on its own. What
// trimming removes is gone without a summary standing in for it, so it stays a
// fallback. The rules are WeKnora's (internal/agent/observe.go).
const (
	minToolResultTokens     = 8 * 1024
	maxToolResultTokens     = 32 * 1024
	toolResultTokenFraction = 5 // a fifth of the window
)

// toolResultBudget is how many tokens all tool results together may occupy.
func toolResultBudget(maxContextTokens int) int {
	if maxContextTokens <= 0 {
		return maxToolResultTokens
	}
	return min(max(maxContextTokens/toolResultTokenFraction, minToolResultTokens), maxToolResultTokens)
}

// trimToolResults returns a copy of msgs whose tool results fit budget tokens
// in total, and whether anything changed. Assistant messages are untouched, so
// every trimmed result keeps the call/result pairing providers require, and the
// input messages are never mutated.
//
// Every result starts as a short marker. The remaining budget is then spent
// newest first: a result that fits comes back whole, and the first that does
// not gets the largest head-and-tail preview that does.
func trimToolResults(msgs []*schema.Message, budget int) ([]*schema.Message, bool) {
	if budget <= 0 || len(msgs) == 0 {
		return msgs, false
	}
	var idx []int
	total := 0
	for i, m := range msgs {
		if m != nil && m.Role == schema.Tool {
			idx = append(idx, i)
			total += EstimateMessage(m)
		}
	}
	if len(idx) == 0 || total <= budget {
		return msgs, false
	}

	out := append([]*schema.Message(nil), msgs...)
	base := make(map[int]int, len(idx))
	remaining := budget
	for _, i := range idx {
		marked := withContent(msgs[i], compactedMarker(msgs[i].Content))
		out[i] = marked
		base[i] = EstimateMessage(marked)
		remaining -= base[i]
	}
	remaining = max(remaining, 0)

	for k := len(idx) - 1; k >= 0; k-- {
		i := idx[k]
		extra := EstimateMessage(msgs[i]) - base[i]
		if extra <= remaining {
			out[i] = msgs[i]
			remaining -= extra
			continue
		}
		out[i] = previewToolMessage(msgs[i], base[i]+remaining)
		remaining = 0
	}
	return out, true
}

func compactedMarker(content string) string {
	return fmt.Sprintf("[Tool result compacted: original_bytes=%d. Re-run the tool with narrower filters or a smaller range if more detail is needed.]", len(content))
}

func withContent(m *schema.Message, content string) *schema.Message {
	cp := *m
	cp.Content = content
	return &cp
}

// previewToolMessage fits the largest head-and-tail excerpt of m into
// maxTokens, by bisection on the number of runes kept. A quarter goes to the
// head and the rest to the tail, because a tool result's conclusion is usually
// at its end.
func previewToolMessage(m *schema.Message, maxTokens int) *schema.Message {
	runes := []rune(m.Content)
	base := withContent(m, compactedMarker(m.Content))
	if len(runes) == 0 || EstimateMessage(base) >= maxTokens {
		return base
	}
	best := base
	low, high := 1, len(runes)
	for low <= high {
		keep := low + (high-low)/2
		head := keep / 4
		tail := keep - head
		candidate := withContent(m, fmt.Sprintf("%s\n\n%s\n...[tool result preview omitted]...\n%s",
			base.Content, string(runes[:head]), string(runes[len(runes)-tail:])))
		if EstimateMessage(candidate) <= maxTokens {
			best = candidate
			low = keep + 1
		} else {
			high = keep - 1
		}
	}
	return best
}
