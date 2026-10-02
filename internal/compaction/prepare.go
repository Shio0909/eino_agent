package compaction

import (
	"strings"

	"github.com/cloudwego/eino/schema"
)

// extraKey marks a message as the output of a compaction, so the next one
// recognises it and updates it in place.
const extraKey = "eino_agent_compaction_summary"

// The summary goes in as a user message, not a second system message: a system
// message is an instruction, this is conversation history.
const (
	summaryPrefix = "以下是之前对话的摘要:\n\n<summary>\n"
	summarySuffix = "\n</summary>"
)

// Preparation is everything the summarisation step needs, computed before any
// LLM call so the caller can decide not to make one.
type Preparation struct {
	FirstKeptIdx        int
	MessagesToSummarize []*schema.Message
	TurnPrefixMessages  []*schema.Message
	IsSplitTurn         bool
	PreviousSummary     string
	TokensBefore        int
}

// IsSummary reports whether m is a compaction summary message.
func IsSummary(m *schema.Message) bool {
	if m == nil || m.Extra == nil {
		return false
	}
	v, _ := m.Extra[extraKey].(bool)
	return v
}

// SummaryMessage wraps summary text in the envelope the model sees.
func SummaryMessage(summary string) *schema.Message {
	return &schema.Message{
		Role:    schema.User,
		Content: summaryPrefix + summary + summarySuffix,
		Extra:   map[string]any{extraKey: true},
	}
}

func unwrapSummary(content string) string {
	start := strings.Index(content, "<summary>")
	if start < 0 {
		return strings.TrimSpace(content)
	}
	start += len("<summary>")
	end := strings.LastIndex(content, "</summary>")
	if end < start {
		return strings.TrimSpace(content[start:])
	}
	return strings.TrimSpace(content[start:end])
}

// Prepare picks the cut point and collects the ranges around it. It returns
// nil when compaction cannot help: nothing falls outside the keep-recent
// budget. That nil is the guard against spending a summarisation call on a
// context that cannot shrink.
func Prepare(msgs []*schema.Message, s Settings) *Preparation {
	if len(msgs) == 0 {
		return nil
	}
	s = s.Normalize()
	boundary := historyStart(msgs)
	previous := ""
	// Summarising starts after the previous summary, which is excluded from
	// the input; feeding it back is what turns successive compactions into a
	// summary of a summary.
	for i := len(msgs) - 1; i >= boundary; i-- {
		if IsSummary(msgs[i]) {
			previous = unwrapSummary(msgs[i].Content)
			boundary = i + 1
			break
		}
	}
	if boundary >= len(msgs) {
		return nil
	}
	cut := FindCutPoint(msgs, boundary, s.KeepRecentTokens)
	historyEnd := cut.FirstKeptIdx
	if cut.IsSplitTurn {
		historyEnd = cut.TurnStartIdx
	}
	toSummarize := cloneRange(msgs, boundary, historyEnd)
	var prefix []*schema.Message
	if cut.IsSplitTurn {
		prefix = cloneRange(msgs, cut.TurnStartIdx, cut.FirstKeptIdx)
	}
	if len(toSummarize) == 0 && len(prefix) == 0 {
		return nil
	}
	return &Preparation{
		FirstKeptIdx:        cut.FirstKeptIdx,
		MessagesToSummarize: toSummarize,
		TurnPrefixMessages:  prefix,
		IsSplitTurn:         cut.IsSplitTurn,
		PreviousSummary:     previous,
		TokensBefore:        EstimateMessages(msgs),
	}
}

// Apply rebuilds the list as system prompt, summary, and the verbatim tail.
func Apply(msgs []*schema.Message, p *Preparation, summary string) []*schema.Message {
	tail := min(max(p.FirstKeptIdx, 0), len(msgs))
	out := make([]*schema.Message, 0, 2+len(msgs)-tail)
	if historyStart(msgs) == 1 {
		out = append(out, msgs[0])
	}
	out = append(out, SummaryMessage(summary))
	return append(out, msgs[tail:]...)
}

func cloneRange(msgs []*schema.Message, start, end int) []*schema.Message {
	start = max(start, 0)
	end = min(end, len(msgs))
	if start >= end {
		return nil
	}
	out := make([]*schema.Message, end-start)
	copy(out, msgs[start:end])
	return out
}
