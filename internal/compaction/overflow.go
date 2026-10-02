package compaction

import (
	"regexp"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// Overflow is worth detecting precisely because the recovery is specific:
// compact and retry once. Misread as a generic failure it ends the turn;
// misread as a transient error it retries the same oversized request. The
// patterns come from real provider responses (collected by WeKnora).
var overflowPatterns = compile(
	`prompt is too long`,
	`request_too_large`,
	`input is too long for requested model`,
	`exceeds the context window`,
	`exceeds (the )?(model'?s )?maximum context length`,
	`input token count.*exceeds the maximum`,
	`maximum prompt length is \d+`,
	`reduce the length of the messages`,
	`maximum context length is \d+ tokens`,
	`exceeds (the )?maximum allowed input length`,
	`is longer than the model'?s context length`,
	`exceeds the limit of \d+`,
	`exceeds the available context size`,
	`greater than the context length`,
	`context window exceeds limit`,
	`exceeded model token limit`,
	`too large for model with \d+ maximum context length`,
	`model_context_window_exceeded`,
	`prompt too long; exceeded (max )?context length`,
	`range of input length should be`,
	`context[_ ]length[_ ]exceeded`,
	`too many tokens`,
	`token limit exceeded`,
	`input length.*exceed`,
	`超过.*(最大)?(上下文|长度|token)`,
)

// Errors that mention token counts while meaning something else, for example
// "ThrottlingException: Too many tokens, please wait".
var notOverflowPatterns = compile(`throttling`, `service unavailable`, `rate limit`, `too many requests`)

func compile(patterns ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		out[i] = regexp.MustCompile(`(?i)` + p)
	}
	return out
}

// IsOverflowError reports whether a failed LLM call failed because the request
// did not fit in the context window.
func IsOverflowError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, p := range notOverflowPatterns {
		if p.MatchString(msg) {
			return false
		}
	}
	for _, p := range overflowPatterns {
		if p.MatchString(msg) {
			return true
		}
	}
	return false
}

// ResponseHitContextLimit reports whether a successful response was shaped by a
// full context window rather than by the completion budget that was asked for
// (WeKnora's rule, internal/agent/compaction/overflow.go). Three shapes:
//
//   - the provider accepted an oversized request silently and reported a prompt
//     larger than the window;
//   - the provider truncated the input to fill the window exactly, leaving no
//     room to generate, and stopped on length with no output;
//   - the provider stopped on length having emitted less than was allowed, so
//     the requested cap cannot be what stopped it. A model whose own output
//     ceiling is below the requested cap also looks like this, which is why
//     the caller repairs at most once per run.
//
// completionBudget is the max_tokens that was requested; zero disables the
// third shape, since without it there is nothing to compare against.
func ResponseHitContextLimit(resp *schema.Message, contextWindow, completionBudget int) bool {
	if resp == nil || resp.ResponseMeta == nil {
		return false
	}
	var prompt, completion int
	if u := resp.ResponseMeta.Usage; u != nil {
		prompt, completion = u.PromptTokens, u.CompletionTokens
	}
	lengthStop := isLengthStop(resp.ResponseMeta.FinishReason)

	if contextWindow > 0 && prompt > contextWindow {
		return true
	}
	if contextWindow > 0 && lengthStop && completion == 0 && prompt > 0 && prompt >= contextWindow*99/100 {
		return true
	}
	return lengthStop && completionBudget > 0 && completion > 0 && completion < completionBudget
}

func isLengthStop(reason string) bool {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "length", "max_tokens", "max_output_tokens":
		return true
	}
	return false
}
