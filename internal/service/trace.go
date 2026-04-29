package service

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/database/repository"
	"eino_agent/internal/tracing"
)

type traceCollector struct {
	traceID string
	started time.Time
	steps   []TraceStep
}

func newTraceCollector(traceID string) *traceCollector {
	return &traceCollector{traceID: traceID, started: time.Now()}
}

func (t *traceCollector) add(step TraceStep) {
	if t == nil {
		return
	}
	if step.TraceID == "" {
		step.TraceID = t.traceID
	}
	if step.Seq == 0 {
		step.Seq = len(t.steps) + 1
	}
	if step.Type == "" {
		step.Type = "status"
	}
	if step.Level == "" {
		step.Level = "info"
	}
	if step.Error != "" && step.Level == "info" {
		step.Level = "error"
	}
	if step.LatencyMs == 0 {
		step.LatencyMs = time.Since(t.started).Milliseconds()
	}
	t.steps = append(t.steps, step)
}

func (t *traceCollector) addStage(stage string, started time.Time, metadata map[string]any) {
	t.add(TraceStep{Type: "status", Stage: stage, LatencyMs: time.Since(started).Milliseconds(), Metadata: metadata})
}

func (t *traceCollector) addEvent(ev StreamEvent) {
	if ev.TraceStep != nil {
		t.add(*ev.TraceStep)
		return
	}
	t.add(TraceStep{
		Type:      ev.Type,
		Content:   ev.Content,
		ToolName:  ev.ToolName,
		ToolInput: ev.ToolInput,
		DocID:     ev.DocID,
		LatencyMs: ev.LatencyMs,
		Error:     ev.Error,
	})
}

func (t *traceCollector) addTraceEvent(ev tracing.Event) {
	if t == nil {
		return
	}
	t.add(TraceStep{
		Type:      ev.Type,
		Stage:     ev.Stage,
		Level:     ev.Level,
		Summary:   ev.Summary,
		Content:   ev.Content,
		ToolName:  ev.ToolName,
		ToolInput: ev.ToolInput,
		DocID:     ev.DocID,
		LatencyMs: ev.LatencyMs,
		Error:     ev.Error,
		Metadata:  ev.Metadata,
	})
}

func (t *traceCollector) context(ctx context.Context) context.Context {
	if t == nil {
		return ctx
	}
	return tracing.WithSink(ctx, t.addTraceEvent)
}

func (t *traceCollector) addError(stage string, err error, metadata map[string]any) {
	if err == nil {
		return
	}
	t.add(TraceStep{Type: "error", Stage: stage, Level: "error", Error: err.Error(), Summary: err.Error(), Metadata: metadata})
}

func (t *traceCollector) summary(mode, status string, latencyMs int64, sourceCount int, errText string) repository.JSON {
	if t == nil {
		return repository.JSON{}
	}
	summary := repository.JSON{
		"mode":         mode,
		"status":       status,
		"latency_ms":   latencyMs,
		"source_count": sourceCount,
		"step_count":   len(t.steps),
	}
	if errText != "" {
		summary["error"] = errText
	}
	for _, step := range t.steps {
		if step.Stage == "request" && step.Metadata != nil {
			if query, ok := step.Metadata["query"].(string); ok && query != "" {
				summary["query"] = query
			}
		}
		if step.Stage == "rewrite" && step.Content != "" {
			summary["rewrite_query"] = step.Content
		}
	}
	return summary
}

func (t *traceCollector) snapshot() []TraceStep {
	if t == nil || len(t.steps) == 0 {
		return nil
	}
	out := make([]TraceStep, len(t.steps))
	copy(out, t.steps)
	return out
}

type tracedRetriever struct {
	base  retriever.Retriever
	trace *traceCollector
}

func newTracedRetriever(base retriever.Retriever, trace *traceCollector) retriever.Retriever {
	if base == nil || trace == nil {
		return base
	}
	return &tracedRetriever{base: base, trace: trace}
}

func (r *tracedRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	started := time.Now()
	docs, err := r.base.Retrieve(ctx, query, opts...)
	metadata := map[string]any{"query": query, "doc_count": len(docs)}
	if err != nil {
		metadata["error"] = err.Error()
	}
	if len(docs) > 0 {
		ids := make([]string, 0, len(docs))
		for _, doc := range docs {
			if doc != nil && doc.ID != "" {
				ids = append(ids, doc.ID)
			}
			if len(ids) >= 10 {
				break
			}
		}
		metadata["doc_ids"] = ids
	}
	r.trace.addStage("retrieve", started, metadata)
	return docs, err
}

func (r *tracedRetriever) RetrieveWithMode(ctx context.Context, query string, mode string) ([]*schema.Document, error) {
	mr, ok := r.base.(interface {
		RetrieveWithMode(context.Context, string, string) ([]*schema.Document, error)
	})
	if !ok {
		return r.Retrieve(ctx, query)
	}
	started := time.Now()
	docs, err := mr.RetrieveWithMode(ctx, query, mode)
	metadata := map[string]any{"query": query, "mode": mode, "doc_count": len(docs)}
	if err != nil {
		metadata["error"] = err.Error()
	}
	r.trace.addStage("retrieve", started, metadata)
	return docs, err
}
