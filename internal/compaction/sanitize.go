package compaction

import (
	"html"

	"github.com/cloudwego/eino/schema"
)

// Sanitize returns the view of msgs that is safe to send to a provider:
//
//   - empty messages are dropped (some providers reject them);
//   - adjacent same-role messages are merged (some providers reject those too).
//     A compaction summary is a user message, so it can end up next to the real
//     user message that follows it;
//   - a tool result whose call is not in the list becomes an escaped user
//     message, which keeps the data without letting external output pass as
//     an instruction.
//
// The wrapper keeps the unsanitised list. Merging folds live conversation into
// the summary envelope, where the next compaction could not read it back out.
//
// Unlike WeKnora's tools.SanitizeMessages, messages that carry tool calls or
// multimodal parts are never merged: folding only the text of a second
// assistant message would silently drop its calls.
func Sanitize(msgs []*schema.Message) []*schema.Message {
	if len(msgs) == 0 {
		return msgs
	}
	out := make([]*schema.Message, 0, len(msgs))
	for i, m := range msgs {
		if m == nil {
			continue
		}
		if m.Content == "" && m.Role != schema.System && m.Role != schema.Tool &&
			len(m.ToolCalls) == 0 && !hasParts(m) {
			continue
		}
		if len(out) > 0 && m.Role != schema.Tool {
			prev := out[len(out)-1]
			if prev.Role == m.Role && mergeable(prev) && mergeable(m) {
				merged := *prev
				merged.Content = prev.Content + "\n\n" + m.Content
				out[len(out)-1] = &merged
				continue
			}
		}
		if m.Role == schema.Tool && m.ToolCallID != "" && !hasMatchingCall(msgs[:i], m.ToolCallID) {
			m = &schema.Message{
				Role: schema.User,
				Content: "<untrusted_tool_result name=\"" + html.EscapeString(m.ToolName) +
					"\">\n" + html.EscapeString(m.Content) + "\n</untrusted_tool_result>",
			}
		}
		out = append(out, m)
	}
	return out
}

func hasParts(m *schema.Message) bool {
	return len(m.MultiContent) > 0 || len(m.UserInputMultiContent) > 0 || len(m.AssistantGenMultiContent) > 0
}

func mergeable(m *schema.Message) bool {
	return m.Role != schema.Tool && len(m.ToolCalls) == 0 && !hasParts(m)
}

func hasMatchingCall(before []*schema.Message, id string) bool {
	for i := len(before) - 1; i >= 0; i-- {
		if m := before[i]; m != nil && m.Role == schema.Assistant {
			for _, tc := range m.ToolCalls {
				if tc.ID == id {
					return true
				}
			}
		}
	}
	return false
}
