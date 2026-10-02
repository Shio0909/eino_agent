package compaction

import "github.com/cloudwego/eino/schema"

// CutPoint divides a conversation into "summarise this" and "keep this".
type CutPoint struct {
	// FirstKeptIdx is the index of the first message kept verbatim.
	FirstKeptIdx int
	// TurnStartIdx is the user message that opened the turn the cut lands in,
	// or -1 when the cut falls on a turn boundary.
	TurnStartIdx int
	// IsSplitTurn reports that the cut divides one turn, so the part before
	// it needs its own summary to explain the part after it.
	IsSplitTurn bool
}

// isCutPoint reports whether history may be cut immediately before m. A tool
// result may not: it must stay with the assistant message that requested it,
// and providers reject an orphaned tool result.
func isCutPoint(m *schema.Message) bool { return m.Role != schema.Tool }

func isTurnStart(m *schema.Message) bool { return m.Role == schema.User }

// historyStart is the first index compaction may touch. The system prompt
// carries instructions, not history.
func historyStart(msgs []*schema.Message) int {
	if len(msgs) > 0 && msgs[0].Role == schema.System {
		return 1
	}
	return 0
}

// FindCutPoint walks back from the newest message and keeps the largest suffix
// that fits keepRecentTokens. The budget is a ceiling, not a floor: stopping at
// the first message that reaches it and keeping that one too would let a single
// large tool result blow the budget on its own.
func FindCutPoint(msgs []*schema.Message, start, keepRecentTokens int) CutPoint {
	start = max(start, 0)
	var cuts []int
	for i := start; i < len(msgs); i++ {
		if isCutPoint(msgs[i]) {
			cuts = append(cuts, i)
		}
	}
	if len(cuts) == 0 {
		return CutPoint{FirstKeptIdx: start, TurnStartIdx: -1}
	}
	cut := cuts[len(cuts)-1]
	acc := 0
	for i := len(msgs) - 1; i >= start; i-- {
		acc += EstimateMessage(msgs[i])
		if acc > keepRecentTokens {
			break
		}
		if isCutPoint(msgs[i]) {
			cut = i
		}
	}
	if isTurnStart(msgs[cut]) {
		return CutPoint{FirstKeptIdx: cut, TurnStartIdx: -1}
	}
	turn := -1
	for i := cut; i >= start; i-- {
		if isTurnStart(msgs[i]) {
			turn = i
			break
		}
	}
	return CutPoint{FirstKeptIdx: cut, TurnStartIdx: turn, IsSplitTurn: turn >= 0}
}
