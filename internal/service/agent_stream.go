package service

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// maxLateToolRounds bounds how many times a streamed run may be continued
// after a model called a tool past the checker's peek window. A model that does
// that every round is not going to settle, and each round is a model call.
const maxLateToolRounds = 4

// errStreamConsumerGone is returned when emit reports that nobody is reading the
// answer any more, so the run stops instead of paying for the rest of it.
var errStreamConsumerGone = errors.New("stream consumer gone")

// streamAgent runs the ReAct agent in streaming mode and passes the text of the
// final answer to emit as it is produced. It returns the final message.
//
// Eino decides whether a model turn is an answer or tool calls from the start
// of its stream (see newStreamToolCallChecker). When a model writes more text
// than the checker looks at and only then calls a tool, the turn is taken for
// the answer and the agent ends with a message that still has tool calls in it.
// Those calls are run here and the agent is started again on the longer
// history, so the call is not lost; the text that came first stays in the
// answer, as it would in any chat transcript.
func streamAgent(ctx context.Context, run *agentRun, messages []*schema.Message, emit func(string) bool) (*schema.Message, error) {
	history := append([]*schema.Message(nil), messages...)
	for round := 0; ; round++ {
		sr, err := run.agent.Stream(ctx, history)
		if err != nil {
			return nil, err
		}
		msg, err := drainAnswer(sr, emit)
		if err != nil {
			return msg, err
		}
		if msg == nil || len(msg.ToolCalls) == 0 {
			return msg, nil
		}
		if round >= maxLateToolRounds {
			return msg, fmt.Errorf("the model kept calling tools after writing text, giving up after %d continuations", maxLateToolRounds)
		}
		toolNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{Tools: run.tools})
		if err != nil {
			return msg, fmt.Errorf("continue after a late tool call: %w", err)
		}
		results, err := toolNode.Invoke(ctx, msg)
		if err != nil {
			return msg, fmt.Errorf("run a late tool call: %w", err)
		}
		history = append(history, msg)
		history = append(history, results...)
	}
}

// drainAnswer reads a stream to its end, passing each text chunk to emit, and
// returns the chunks joined into one message.
func drainAnswer(sr *schema.StreamReader[*schema.Message], emit func(string) bool) (*schema.Message, error) {
	defer sr.Close()
	var chunks []*schema.Message
	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return joinChunks(chunks), err
		}
		if chunk == nil {
			continue
		}
		chunks = append(chunks, chunk)
		if chunk.Content != "" && !emit(chunk.Content) {
			return joinChunks(chunks), errStreamConsumerGone
		}
	}
	return joinChunks(chunks), nil
}

func joinChunks(chunks []*schema.Message) *schema.Message {
	if len(chunks) == 0 {
		return nil
	}
	msg, err := schema.ConcatMessages(chunks)
	if err != nil {
		// Chunks that cannot be merged still carry the text that was streamed.
		var text string
		for _, c := range chunks {
			text += c.Content
		}
		return &schema.Message{Role: schema.Assistant, Content: text}
	}
	return msg
}
