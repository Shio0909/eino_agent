package compaction

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const (
	// Each attempt is a full LLM round trip, and a compaction that keeps
	// failing is better served by the raw archive than by stalling the turn.
	maxSummarizationAttempts = 2
	summarizationTimeout     = 60 * time.Second
)

// Reason records what asked for a compaction.
type Reason string

const (
	ReasonThreshold Reason = "threshold"
	ReasonOverflow  Reason = "overflow"
)

// ErrNothingToCompact means the context cannot be made smaller: everything
// outside the keep-recent budget is already gone. Callers treat it as a stop
// signal, not something to retry.
var ErrNothingToCompact = errors.New("compaction: nothing outside the keep-recent budget")

// Summarizer produces text from a system prompt and a user prompt.
type Summarizer interface {
	Summarize(ctx context.Context, system, prompt string, maxTokens int) (Summary, error)
}

// Summary is one summariser response.
type Summary struct {
	Text string
	// Truncated reports that generation stopped at the token cap, so the text
	// ends mid-section.
	Truncated bool
}

// ModelSummarizer summarises with an Eino chat model.
type ModelSummarizer struct{ Model model.BaseChatModel }

func (m ModelSummarizer) Summarize(ctx context.Context, system, prompt string, maxTokens int) (Summary, error) {
	resp, err := m.Model.Generate(ctx, []*schema.Message{
		{Role: schema.System, Content: system},
		{Role: schema.User, Content: prompt},
	}, model.WithTemperature(0.3), model.WithMaxTokens(maxTokens))
	if err != nil {
		return Summary{}, err
	}
	if resp == nil {
		return Summary{}, errors.New("empty response from LLM")
	}
	out := Summary{Text: resp.Content}
	if resp.ResponseMeta != nil {
		switch strings.ToLower(strings.TrimSpace(resp.ResponseMeta.FinishReason)) {
		case "length", "max_tokens", "max_output_tokens":
			out.Truncated = true
		}
	}
	return out, nil
}

// Result describes what one compaction did.
type Result struct {
	Messages []*schema.Message
	Summary  string
	Reason   Reason
	// FirstKeptIdx is where the verbatim tail began in the input.
	FirstKeptIdx   int
	TokensBefore   int
	TokensAfter    int
	MessagesBefore int
	MessagesAfter  int
	// Degraded reports that a summary came from the raw archive because the
	// summariser failed. The context still shrank; its memory is coarser.
	Degraded  bool
	SplitTurn bool
}

// Freed is how much room the compaction recovered.
func (r *Result) Freed() int { return r.TokensBefore - r.TokensAfter }

// Compactor summarises conversation history into a checkpoint.
type Compactor struct {
	summarizer Summarizer
	settings   Settings
}

// New returns nil when compaction cannot run, so a nil compactor means
// "feature off" without a second flag.
func New(s Summarizer, settings Settings) *Compactor {
	if s == nil || settings.MaxContextTokens <= 0 {
		return nil
	}
	return &Compactor{summarizer: s, settings: settings.Normalize()}
}

// Settings returns the normalised settings.
func (c *Compactor) Settings() Settings {
	if c == nil {
		return Settings{}
	}
	return c.settings
}

// Compact replaces history older than the keep-recent budget with a summary.
func (c *Compactor) Compact(ctx context.Context, msgs []*schema.Message, reason Reason) (*Result, error) {
	if c == nil {
		return nil, ErrNothingToCompact
	}
	prep := Prepare(msgs, c.settings)
	if prep == nil {
		return nil, ErrNothingToCompact
	}
	summary, degraded := c.buildSummary(ctx, prep)
	out := Apply(msgs, prep, summary)
	return &Result{
		Messages:       out,
		Summary:        summary,
		Reason:         reason,
		FirstKeptIdx:   prep.FirstKeptIdx,
		TokensBefore:   prep.TokensBefore,
		TokensAfter:    EstimateMessages(out),
		MessagesBefore: len(msgs),
		MessagesAfter:  len(out),
		Degraded:       degraded,
		SplitTurn:      prep.IsSplitTurn,
	}, nil
}

func (c *Compactor) buildSummary(ctx context.Context, p *Preparation) (string, bool) {
	degraded := false
	history := p.PreviousSummary
	if len(p.MessagesToSummarize) > 0 {
		instructions := initialInstructions
		if p.PreviousSummary != "" {
			instructions = updateInstructions
		}
		text, err := c.summarize(ctx, p.MessagesToSummarize, p.PreviousSummary, instructions, c.settings.summaryBudget())
		if err != nil {
			degraded = true
			// The previous summary is still the best record of everything
			// before this span, so the archive is appended to it.
			history = joinNonEmpty(p.PreviousSummary, fitArchive(rawArchive(p.MessagesToSummarize), c.settings.summaryBudget()))
		} else {
			history = text
		}
	}
	if p.IsSplitTurn && len(p.TurnPrefixMessages) > 0 {
		prefix, err := c.summarize(ctx, p.TurnPrefixMessages, "", turnPrefixInstructions, c.settings.turnPrefixBudget())
		if err != nil {
			degraded = true
			prefix = fitArchive(rawArchive(p.TurnPrefixMessages), c.settings.turnPrefixBudget())
		}
		if history == "" {
			history = "No prior history."
		}
		history += splitTurnSeparator + prefix
	}
	return history, degraded
}

func (c *Compactor) summarize(ctx context.Context, msgs []*schema.Message, previous, instructions string, maxTokens int) (string, error) {
	prompt := buildPrompt(msgs, previous, instructions)
	var last error
	for attempt := 1; attempt <= maxSummarizationAttempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, summarizationTimeout)
		resp, err := c.summarizer.Summarize(callCtx, summarizationSystemPrompt, prompt, maxTokens)
		cancel()
		if err != nil {
			last = err
			continue
		}
		if strings.TrimSpace(resp.Text) == "" {
			last = errors.New("empty response from LLM")
			continue
		}
		// A summary cut off by the token cap reads like a valid one but ends
		// mid-section, and every later round inherits that as its only memory.
		if resp.Truncated {
			last = errors.New("generation hit the token cap and the summary is incomplete")
			continue
		}
		return strings.TrimSpace(resp.Text), nil
	}
	return "", fmt.Errorf("summarization failed after %d attempts: %w", maxSummarizationAttempts, last)
}

// buildPrompt wraps the transcript in a tag and puts the instructions last, so
// the summariser cannot mistake conversation text for its own instructions.
func buildPrompt(msgs []*schema.Message, previous, instructions string) string {
	var sb strings.Builder
	sb.WriteString("<conversation>\n")
	sb.WriteString(serialize(msgs))
	sb.WriteString("\n</conversation>\n\n")
	if previous != "" {
		sb.WriteString("<previous-summary>\n")
		sb.WriteString(previous)
		sb.WriteString("\n</previous-summary>\n\n")
	}
	sb.WriteString(instructions)
	return sb.String()
}

func joinNonEmpty(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "\n\n")
}
