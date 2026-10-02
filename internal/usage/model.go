package usage

import (
	"context"
	"errors"
	"io"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// WrapChatModel counts the tokens of every call to inner. The wrapper offers
// exactly the capabilities inner has: callers probe for ToolCallingChatModel
// by type assertion, and a wrapper that claimed it unconditionally would turn
// that probe into a later error.
func WrapChatModel(inner model.ChatModel, m *Meter) model.ChatModel {
	base := &metered{inner: inner, meter: m}
	if tc, ok := inner.(model.ToolCallingChatModel); ok {
		return &meteredToolCalling{metered: base, tc: tc}
	}
	return base
}

type metered struct {
	inner model.ChatModel
	meter *Meter
}

func (w *metered) BindTools(tools []*schema.ToolInfo) error { return w.inner.BindTools(tools) }

type meteredToolCalling struct {
	*metered
	tc model.ToolCallingChatModel
}

func (w *meteredToolCalling) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound, err := w.tc.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &meteredToolCalling{metered: &metered{inner: bindless{bound}, meter: w.meter}, tc: bound}, nil
}

// bindless adapts a ToolCallingChatModel to the ChatModel interface that
// metered holds. BindTools is the deprecated, unsafe way to bind, and a model
// returned from WithTools is already bound; refuse rather than mutate it.
type bindless struct{ model.ToolCallingChatModel }

func (bindless) BindTools([]*schema.ToolInfo) error {
	return errors.New("usage: tools are already bound; use WithTools")
}

func (w *metered) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	out, err := w.inner.Generate(ctx, in, opts...)
	if err == nil {
		w.record(out)
	}
	return out, err
}

// Stream passes chunks through and counts the usage the provider attaches to
// one of them. A consumer that stops reading early does not get its call
// counted, because the provider sends usage last.
func (w *metered) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	src, err := w.inner.Stream(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	out, sink := schema.Pipe[*schema.Message](1)
	go func() {
		defer sink.Close()
		defer src.Close()
		var last *schema.Message
		for {
			chunk, err := src.Recv()
			if err == io.EOF {
				w.record(last)
				return
			}
			if err != nil {
				sink.Send(nil, err)
				return
			}
			if chunk != nil && chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
				last = chunk
			}
			if closed := sink.Send(chunk, nil); closed {
				return
			}
		}
	}()
	return out, nil
}

func (w *metered) record(msg *schema.Message) {
	if msg == nil || msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil {
		return
	}
	u := msg.ResponseMeta.Usage
	w.meter.Record(int64(u.PromptTokens), int64(u.CompletionTokens), int64(u.TotalTokens))
}
