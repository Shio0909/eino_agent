package compaction

import (
	"unicode"

	"github.com/cloudwego/eino/schema"
)

const (
	perMessageOverhead  = 4
	perToolCallOverhead = 6
	perConversationTail = 3
	imagePartTokens     = 1200

	// The rates are deliberately on the high side: over-estimating compacts a
	// little early, under-estimating overflows the window.
	cjkTokensPerRune   = 1.0
	otherTokensPerRune = 0.3
)

// EstimateString returns an approximate token count for text.
func EstimateString(s string) int {
	if s == "" {
		return 0
	}
	var cjk, other float64
	for _, r := range s {
		if isCJK(r) {
			cjk++
		} else if !unicode.IsSpace(r) || r == '\n' {
			other++
		}
	}
	return int(cjk*cjkTokensPerRune+other*otherTokensPerRune) + 1
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) ||
		(r >= 0x3000 && r <= 0x303F) || (r >= 0xFF00 && r <= 0xFFEF)
}

// EstimateMessage counts every field that goes out on the wire, including the
// easy-to-forget ones. Reasoning content matters most: thinking models need
// earlier turns' reasoning sent back, and leaving it out makes a large context
// measure small, so compaction cuts in the wrong place and runs again next round.
func EstimateMessage(m *schema.Message) int {
	if m == nil {
		return 0
	}
	n := perMessageOverhead
	n += EstimateString(string(m.Role)) + EstimateString(m.Content) + EstimateString(m.Name)
	n += EstimateString(m.ToolCallID) + EstimateString(m.ReasoningContent)
	for _, p := range m.UserInputMultiContent {
		if p.Type == schema.ChatMessagePartTypeText {
			n += EstimateString(p.Text)
		} else {
			n += imagePartTokens
		}
	}
	for _, tc := range m.ToolCalls {
		n += EstimateString(tc.Function.Name) + EstimateString(tc.Function.Arguments) + perToolCallOverhead
	}
	return n
}

// EstimateMessages returns the estimated size of a message list.
func EstimateMessages(msgs []*schema.Message) int {
	total := perConversationTail
	for _, m := range msgs {
		total += EstimateMessage(m)
	}
	return total
}
