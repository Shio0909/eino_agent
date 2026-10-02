package compaction

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
)

const (
	// Tool output is the largest contributor to context size and the summary
	// needs its gist, not its bytes.
	toolResultMaxChars = 2000
	textMaxChars       = 4000
	toolArgsMaxChars   = 400
)

// serialize renders messages as a transcript rather than as a conversation. A
// model handed real messages tries to continue them; handed a transcript, it
// summarises them.
func serialize(msgs []*schema.Message) string {
	var parts []string
	for _, m := range msgs {
		if m == nil {
			continue
		}
		switch m.Role {
		case schema.User:
			if c := truncate(m.Content, textMaxChars); c != "" {
				parts = append(parts, "[User]: "+c)
			}
		case schema.Assistant:
			if m.ReasoningContent != "" {
				parts = append(parts, "[Assistant thinking]: "+truncate(m.ReasoningContent, textMaxChars))
			}
			if m.Content != "" {
				parts = append(parts, "[Assistant]: "+truncate(m.Content, textMaxChars))
			}
			if calls := serializeToolCalls(m.ToolCalls); calls != "" {
				parts = append(parts, "[Assistant tool calls]: "+calls)
			}
		case schema.Tool:
			if c := truncateEnds(m.Content, toolResultMaxChars); c != "" {
				parts = append(parts, fmt.Sprintf("[Tool result %s]: %s", m.ToolName, c))
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

func serializeToolCalls(calls []schema.ToolCall) string {
	if len(calls) == 0 {
		return ""
	}
	out := make([]string, 0, len(calls))
	for _, tc := range calls {
		out = append(out, fmt.Sprintf("%s(%s)", tc.Function.Name, renderArgs(tc.Function.Arguments)))
	}
	return strings.Join(out, "; ")
}

// renderArgs turns an arguments blob into sorted key=value pairs and drops
// oversized values, so the same call always renders the same way.
func renderArgs(arguments string) string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(arguments), &parsed); err != nil {
		return truncate(arguments, toolArgsMaxChars)
	}
	keys := make([]string, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		enc, err := json.Marshal(parsed[k])
		if err != nil {
			continue
		}
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, truncate(string(enc), toolArgsMaxChars)))
	}
	return strings.Join(pairs, ", ")
}

func truncate(s string, maxChars int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	return fmt.Sprintf("%s\n\n[... %d more characters truncated]", string(r[:maxChars]), len(r)-maxChars)
}

// truncateEnds shortens tool output by dropping its middle. Tool results put
// their conclusion at the end (an error, a total, a status line) as often as at
// the start, and a summariser that only sees the head reports the wrong thing
// or loses the fact the next round needs.
func truncateEnds(s string, maxChars int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	head := maxChars * 2 / 3
	tail := maxChars - head
	return fmt.Sprintf("%s\n\n[... %d characters omitted ...]\n\n%s",
		string(r[:head]), len(r)-maxChars, string(r[len(r)-tail:]))
}

// rawArchive is the fallback when the summariser is unavailable. It is lossy,
// but it keeps tool names and arguments so the next round does not redo work.
func rawArchive(msgs []*schema.Message) string {
	var sb strings.Builder
	sb.WriteString("Raw conversation archive (LLM summarization unavailable):\n\n")
	for _, m := range msgs {
		if m == nil {
			continue
		}
		switch m.Role {
		case schema.User:
			fmt.Fprintf(&sb, "- User: %s\n", truncate(m.Content, 500))
		case schema.Assistant:
			if calls := serializeToolCalls(m.ToolCalls); calls != "" {
				fmt.Fprintf(&sb, "- Assistant [%s]: %s\n", calls, truncate(m.Content, 500))
				continue
			}
			fmt.Fprintf(&sb, "- Assistant: %s\n", truncate(m.Content, 500))
		case schema.Tool:
			fmt.Fprintf(&sb, "- Tool[%s]: %s\n", m.ToolName, truncateEnds(m.Content, 500))
		}
	}
	return sb.String()
}

// fitArchive keeps the newest part of an archive that exceeds the token
// budget. Without the cap a degraded compaction of many short messages would
// not shrink the context, and the next round would overflow anyway.
func fitArchive(archive string, budgetTokens int) string {
	if budgetTokens <= 0 || EstimateString(archive) <= budgetTokens {
		return archive
	}
	runes := []rune(archive)
	// Start from the Latin rate and tighten until the estimate fits, so CJK
	// text, which costs a token per character, ends up within budget too.
	keep := min(len(runes)-1, int(float64(budgetTokens)/otherTokensPerRune))
	for keep > 1 && EstimateString(string(runes[len(runes)-keep:])) > budgetTokens {
		keep = keep * 3 / 4
	}
	tail := string(runes[len(runes)-keep:])
	if i := strings.Index(tail, "\n- "); i >= 0 {
		tail = tail[i+1:]
	}
	return "Raw conversation archive (LLM summarization unavailable; older entries dropped):\n\n" + tail
}
