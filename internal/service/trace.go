package service

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type traceCollector struct {
	started time.Time
	steps   []TraceStep
}

func newTraceCollector() *traceCollector {
	return &traceCollector{started: time.Now()}
}

func (t *traceCollector) add(step TraceStep) {
	if t == nil {
		return
	}
	if step.Type == "" {
		step.Type = "status"
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
	})
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
