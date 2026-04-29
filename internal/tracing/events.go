package tracing

import "context"

type Event struct {
	Type      string
	Stage     string
	Level     string
	Summary   string
	Content   string
	ToolName  string
	ToolInput string
	DocID     string
	LatencyMs int64
	Error     string
	Metadata  map[string]any
}

type Sink func(Event)

type sinkKey struct{}

func WithSink(ctx context.Context, sink Sink) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, sinkKey{}, sink)
}

func Emit(ctx context.Context, event Event) {
	if ctx == nil {
		return
	}
	sink, ok := ctx.Value(sinkKey{}).(Sink)
	if !ok || sink == nil {
		return
	}
	sink(event)
}
