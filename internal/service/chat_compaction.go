package service

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/compaction"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/tracing"
)

// sessionHistoryScanLimit bounds how many recent messages one request reads to
// find what the stored summary has not covered yet. A session that goes this
// far past its checkpoint without a request is summarised from the newest
// messages; the gap is not recoverable and not worth a second query.
const sessionHistoryScanLimit = 400

// compactionSettings returns the configured in-turn settings. The zero value
// means "off": without a context window there is no budget to enforce.
func (s *ChatService) compactionSettings() compaction.Settings {
	c := s.config.Memory.Compaction
	return compaction.Settings{
		Enabled:          c.Enabled,
		MaxContextTokens: c.MaxContextTokens,
		ReserveTokens:    c.ReserveTokens,
		KeepRecentTokens: c.KeepRecentTokens,
	}
}

// historyBudgetTokens is what session history may occupy in the prompt.
func (s *ChatService) historyBudgetTokens() int {
	c := s.config.Memory.Compaction
	if c.HistoryBudgetTokens > 0 {
		return c.HistoryBudgetTokens
	}
	return c.MaxContextTokens / 3
}

func (s *ChatService) newCompactor(settings compaction.Settings) *compaction.Compactor {
	if s.chatModel == nil {
		return nil
	}
	return compaction.New(compaction.ModelSummarizer{Model: s.chatModel}, settings)
}

// wrapModelForCompaction guards one agent run's model calls against context
// overflow. It returns m unchanged when compaction is not configured.
func (s *ChatService) wrapModelForCompaction(m model.ToolCallingChatModel, sink func(StreamEvent)) model.ToolCallingChatModel {
	settings := s.compactionSettings()
	// Only a model call has to leave room for a reply; the session history
	// budget (historySettings) is sized separately and must not inherit this.
	settings.CompletionTokens = s.config.LLM.MaxTokens
	c := s.newCompactor(settings)
	if c == nil {
		return m
	}
	return compaction.WrapModelWithHooks(m, c, compaction.Hooks{OnUsage: logUsageDrift, OnCompact: func(ev compaction.Event) {
		log.Printf("[ChatService] context compacted reason=%s tokens=%d->%d messages=%d->%d degraded=%v split_turn=%v took=%s",
			ev.Reason, ev.TokensBefore, ev.TokensAfter, ev.MessagesBefore, ev.MessagesAfter, ev.Degraded, ev.SplitTurn, ev.Duration)
		if sink == nil {
			return
		}
		sink(StreamEvent{
			Type: "context_compaction",
			TraceStep: &TraceStep{
				Type:      "context",
				Stage:     "context_compaction",
				Level:     compactionLevel(ev.Degraded),
				Summary:   fmt.Sprintf("上下文压缩(%s): %d → %d tokens", ev.Reason, ev.TokensBefore, ev.TokensAfter),
				LatencyMs: ev.Duration.Milliseconds(),
				Metadata: map[string]any{
					"reason":          string(ev.Reason),
					"tokens_before":   ev.TokensBefore,
					"tokens_after":    ev.TokensAfter,
					"messages_before": ev.MessagesBefore,
					"messages_after":  ev.MessagesAfter,
					"degraded":        ev.Degraded,
					"split_turn":      ev.SplitTurn,
				},
			},
		})
	}})
}

// driftLogThreshold is the gap, in percent, between the estimated and the
// provider-counted prompt that is worth a log line. A persistent gap in either
// direction means the estimator is missing something (tool schemas, a
// tokenizer difference) and the reserve should be adjusted.
const driftLogThreshold = 20

func logUsageDrift(u compaction.UsageReport) {
	d := u.DriftPercent()
	if d > -driftLogThreshold && d < driftLogThreshold {
		return
	}
	log.Printf("[ChatService] token estimate drift predicted=%d provider_prompt=%d drift=%.0f%% from_usage=%v tool_schema_tokens=%d",
		u.Predicted, u.Prompt, d, u.FromUsage, u.ToolSchemaTokens)
}

func compactionLevel(degraded bool) string {
	if degraded {
		return "warn"
	}
	return "info"
}

// SetSessionSummaryRepository enables cross-turn compaction: older messages of
// a session are replaced by a stored rolling summary.
func (s *ChatService) SetSessionSummaryRepository(repo repository.SessionSummaryRepository) {
	s.summaryRepo = repo
}

func (s *ChatService) crossTurnCompactionEnabled() bool {
	c := s.config.Memory.Compaction
	return c.Enabled && c.MaxContextTokens > 0 && s.summaryRepo != nil && s.messageRepo != nil
}

// sessionHistory is the conversation memory of one session after compaction:
// the stored summary plus the messages it does not cover.
type sessionHistory struct {
	Summary  string
	Recent   []*repository.Message
	Compact  *compaction.Event
	Degraded bool
}

// loadSessionHistory returns the summary and the uncovered messages, and
// compacts first when they no longer fit the history budget. Any failure
// falls back to what is already stored: memory must never block a chat turn.
func (s *ChatService) loadSessionHistory(ctx context.Context, sessionID string) (*sessionHistory, error) {
	stored, err := s.summaryRepo.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load session summary: %w", err)
	}
	msgs, err := s.messageRepo.ListBySession(ctx, sessionID, sessionHistoryScanLimit)
	if err != nil {
		return nil, fmt.Errorf("list session messages: %w", err)
	}
	uncovered := uncoveredMessages(msgs, stored)

	h := &sessionHistory{Recent: uncovered}
	if stored != nil {
		h.Summary, h.Degraded = stored.Summary, stored.Degraded
	}

	settings := s.historySettings()
	input := historyToSchema(h.Summary, uncovered)
	if !settings.ShouldCompact(compaction.EstimateMessages(input)) {
		return h, nil
	}
	c := s.newCompactor(settings)
	if c == nil {
		return h, nil
	}
	res, err := c.Compact(ctx, input, compaction.ReasonThreshold)
	if err != nil {
		return h, nil
	}
	offset := 0
	if h.Summary != "" {
		offset = 1
	}
	covered := res.FirstKeptIdx - offset
	if covered <= 0 || covered > len(uncovered) {
		return h, nil
	}
	last := uncovered[covered-1]
	next := &repository.SessionSummary{
		SessionID:             sessionID,
		Summary:               res.Summary,
		CoveredUntilMessageID: last.ID,
		CoveredUntilAt:        last.CreatedAt,
		CoveredCount:          covered,
		Degraded:              res.Degraded,
	}
	if stored != nil {
		next.CoveredCount += stored.CoveredCount
	}
	if err := s.summaryRepo.Upsert(ctx, next); err != nil {
		// The summary is still good for this turn; it is just not persisted.
		log.Printf("[ChatService] save session summary failed (session=%s): %v", sessionID, err)
	}
	log.Printf("[ChatService] session history compacted session=%s tokens=%d->%d covered=%d degraded=%v",
		sessionID, res.TokensBefore, res.TokensAfter, next.CoveredCount, res.Degraded)
	return &sessionHistory{
		Summary:  res.Summary,
		Recent:   uncovered[covered:],
		Degraded: res.Degraded,
		Compact: &compaction.Event{
			Reason: compaction.ReasonThreshold, TokensBefore: res.TokensBefore, TokensAfter: res.TokensAfter,
			MessagesBefore: res.MessagesBefore, MessagesAfter: res.MessagesAfter,
			Degraded: res.Degraded, SplitTurn: res.SplitTurn,
		},
	}, nil
}

// historySettings sizes the compactor for session history rather than the
// whole prompt: the window handed to it is the history budget, so the
// reserve is not subtracted a second time.
func (s *ChatService) historySettings() compaction.Settings {
	base := s.compactionSettings()
	budget := s.historyBudgetTokens()
	reserve := base.ReserveTokens
	if reserve <= 0 {
		reserve = compaction.DefaultReserveTokens
	}
	// Threshold() is MaxContextTokens - ReserveTokens; make it the budget.
	base.MaxContextTokens = budget + reserve
	if base.KeepRecentTokens <= 0 || base.KeepRecentTokens > budget/2 {
		base.KeepRecentTokens = budget / 2
	}
	return base
}

// uncoveredMessages drops the messages a stored summary already stands for.
func uncoveredMessages(msgs []*repository.Message, stored *repository.SessionSummary) []*repository.Message {
	if stored == nil {
		return msgs
	}
	for i, m := range msgs {
		if m != nil && m.ID == stored.CoveredUntilMessageID {
			return msgs[i+1:]
		}
	}
	// The checkpoint fell out of the scan window; fall back to the clock.
	for i, m := range msgs {
		if m != nil && m.CreatedAt.After(stored.CoveredUntilAt) {
			return msgs[i:]
		}
	}
	return nil
}

func historyToSchema(summary string, msgs []*repository.Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(msgs)+1)
	if summary != "" {
		out = append(out, compaction.SummaryMessage(summary))
	}
	for _, m := range msgs {
		if m == nil || strings.TrimSpace(m.Content) == "" {
			continue
		}
		role := schema.User
		if strings.EqualFold(m.Role, "assistant") {
			role = schema.Assistant
		}
		out = append(out, &schema.Message{Role: role, Content: m.Content})
	}
	return out
}

// formatSessionHistory renders the compacted history for the prompt. Recent
// messages are not truncated per message: compaction already bounds them.
func formatSessionHistory(h *sessionHistory) string {
	var parts []string
	if strings.TrimSpace(h.Summary) != "" {
		title := "【会话摘要（较早对话的压缩记录）】"
		if h.Degraded {
			title = "【会话摘要（较早对话的原文存档，摘要失败降级）】"
		}
		parts = append(parts, title+"\n"+strings.TrimSpace(h.Summary))
	}
	var sb strings.Builder
	for _, m := range h.Recent {
		if m == nil || strings.TrimSpace(m.Content) == "" {
			continue
		}
		role := "用户"
		if strings.EqualFold(m.Role, "assistant") {
			role = "助手"
		}
		sb.WriteString(role)
		sb.WriteString(": ")
		sb.WriteString(strings.TrimSpace(m.Content))
		sb.WriteString("\n")
	}
	if recent := strings.TrimSpace(sb.String()); recent != "" {
		parts = append(parts, "【会话近期记录】\n"+recent)
	}
	return strings.Join(parts, "\n\n")
}

func emitHistoryCompaction(ctx context.Context, sessionID string, ev *compaction.Event) {
	if ev == nil {
		return
	}
	tracing.Emit(ctx, tracing.Event{
		Type:    "context",
		Stage:   "session_history_compaction",
		Level:   compactionLevel(ev.Degraded),
		Summary: fmt.Sprintf("会话历史压缩: %d → %d tokens", ev.TokensBefore, ev.TokensAfter),
		Metadata: map[string]any{
			"session_id":      sessionID,
			"tokens_before":   ev.TokensBefore,
			"tokens_after":    ev.TokensAfter,
			"messages_before": ev.MessagesBefore,
			"messages_after":  ev.MessagesAfter,
			"degraded":        ev.Degraded,
		},
	})
}
