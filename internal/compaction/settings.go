// Package compaction keeps a conversation inside a model's context window by
// replacing older messages with a structured summary.
//
// The design follows WeKnora's agent context compaction
// (github.com/Tencent/WeKnora, internal/agent/compaction, MIT License,
// Copyright (c) Tencent). It is re-implemented here on Eino's schema.Message:
//
//   - the cut point is chosen by token budget alone, never "keep the current
//     turn intact", so a long ReAct loop can still be compacted;
//   - a cut never separates a tool result from the assistant message that
//     requested it, and a turn that is split gets its own prefix summary;
//   - successive summaries are updated in place instead of summarising a
//     summary of a summary;
//   - a failed summariser degrades to a raw archive instead of stalling;
//   - a provider "context too long" error is repaired by one compaction and one
//     retry.
//
// Beyond that, the model wrapper follows WeKnora's accounting and repair rules:
//
//   - the size of the next request is the provider's own count of the previous
//     one plus an estimate for what was appended since, so tool schemas and
//     whatever else the estimate cannot see are already in the baseline;
//   - a compaction that frees under 5% of the context is discarded and not
//     tried again at that size, because each attempt costs a summarisation;
//   - when the weight sits inside the keep-recent window, which a cut cannot
//     reach, oversized tool results are shrunk to head and tail previews;
//   - the provider sees a sanitised view (no empty or adjacent same-role
//     messages, no orphaned tool results) while the wrapper keeps the
//     structured one;
//   - a response that stopped on length having emitted less than was allowed is
//     read as a full window and repaired once per run.
//
// Token counts are estimated with a character heuristic instead of a BPE table,
// which avoids a large dependency and fits CJK text better than cl100k.
package compaction

const (
	// DefaultReserveTokens is the room kept free for the next response.
	DefaultReserveTokens = 4096
	// SafetyTokens is slack between the estimate and what the provider will
	// count. Without it the request that exactly fits by our arithmetic is the
	// one that gets rejected.
	SafetyTokens = 4096
	// DefaultKeepRecentTokens is how much recent conversation survives a
	// compaction verbatim. It is what makes compaction terminate: whatever the
	// context grew to, it comes out at roughly summary + this.
	DefaultKeepRecentTokens = 6000
	minSummaryTokens        = 512
)

// Settings configures when compaction runs and how much it keeps.
type Settings struct {
	// Enabled turns automatic compaction on. A disabled compactor still
	// repairs a provider-reported overflow, which is a repair, not a policy.
	Enabled bool
	// MaxContextTokens is the model's context window. Zero disables
	// compaction: without a window there is no budget to enforce.
	MaxContextTokens int
	// ReserveTokens is the part of the window history may not use because the
	// next response has to fit there.
	ReserveTokens int
	// CompletionTokens is the max_tokens requested per model call. When set,
	// the reserve is raised to at least CompletionTokens+SafetyTokens (but never
	// above half the window): a model allowed to emit 4096 tokens needs that
	// much free, or the request is accepted and the reply is cut short.
	CompletionTokens int
	// KeepRecentTokens is the budget of recent messages kept verbatim.
	KeepRecentTokens int
	// MaxSummaryTokens caps the summarisation completion. Zero means the
	// budget derived from the reserve.
	MaxSummaryTokens int
}

// Normalize fills in defaults and reconciles budgets that cannot all hold.
func (s Settings) Normalize() Settings {
	if s.ReserveTokens <= 0 {
		s.ReserveTokens = DefaultReserveTokens
	}
	if s.KeepRecentTokens <= 0 {
		s.KeepRecentTokens = DefaultKeepRecentTokens
	}
	if s.CompletionTokens > 0 && s.MaxContextTokens > 0 {
		// Capped at half the window: past that, history has too little room
		// and it is better to risk a cut-short reply than to compact constantly.
		s.ReserveTokens = max(s.ReserveTokens, min(s.CompletionTokens+SafetyTokens, s.MaxContextTokens/2))
	}
	// What matters is how much room a compaction leaves before the next one.
	// Keeping a quarter of the usable window leaves the compacted context at
	// roughly half the threshold, which is several rounds of work.
	if s.MaxContextTokens > 0 {
		usable := s.MaxContextTokens - s.ReserveTokens
		if quarter := usable / 4; quarter > 0 && s.KeepRecentTokens > quarter {
			s.KeepRecentTokens = quarter
		}
	}
	return s
}

// Threshold is the context size above which history must be compacted.
func (s Settings) Threshold() int {
	if s.MaxContextTokens <= 0 {
		return 0
	}
	return max(s.MaxContextTokens-s.ReserveTokens, 1)
}

// ShouldCompact reports whether the context has crossed the threshold.
func (s Settings) ShouldCompact(contextTokens int) bool {
	if !s.Enabled {
		return false
	}
	t := s.Threshold()
	return t > 0 && contextTokens > t
}

// summaryBudget caps a summarisation completion. It is also capped at the
// keep-recent budget: the summary sits in the compacted context next to the
// retained tail, and without a ceiling an in-place update grows every round
// and reclaims the room compaction just freed.
func (s Settings) summaryBudget() int {
	budget := s.ReserveTokens * 4 / 5
	if s.MaxSummaryTokens > 0 && s.MaxSummaryTokens < budget {
		budget = s.MaxSummaryTokens
	}
	if s.KeepRecentTokens > 0 && s.KeepRecentTokens < budget {
		budget = s.KeepRecentTokens
	}
	return max(budget, minSummaryTokens)
}

func (s Settings) turnPrefixBudget() int {
	return max(s.summaryBudget()/2, minSummaryTokens/2)
}
