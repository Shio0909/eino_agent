package service

import (
	"context"
	"strings"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	internalTool "eino_agent/internal/tool"
)

// requiresQuery is a tool whose schema makes "query" mandatory, run behind the
// same argument validation the server puts in front of its built-in tools.
type requiresQuery struct{ runs int }

func (*requiresQuery) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "lookup", Desc: "look something up", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"query": {Type: schema.String, Desc: "what to look up", Required: true},
	})}, nil
}

func (t *requiresQuery) InvokableRun(context.Context, string, ...einotool.Option) (string, error) {
	t.runs++
	return "found it", nil
}

func validatedLookup() (*requiresQuery, einotool.BaseTool) {
	inner := &requiresQuery{}
	return inner, internalTool.WrapWithValidation(inner)
}

// Models do call tools with missing or malformed arguments. The validation
// message exists so the model can correct itself; if the call instead fails the
// whole run, the user gets an error for something the model would have fixed.
func TestInvalidToolArgumentsAreReportedToTheModelNotTheCaller(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		inner, tool := validatedLookup()
		m := &scriptedModel{turns: []turn{
			{chunks: []*schema.Message{callTool("c1", "lookup", `{}`)}},
			{chunks: []*schema.Message{callTool("c2", "lookup", `{"query":"x"}`)}},
			{chunks: []*schema.Message{text("recovered")}},
		}}
		run, log := newTestRun(t, m, 64, tool)

		var answer string
		var err error
		if streamed {
			var b strings.Builder
			msg, e := streamAgent(context.Background(), run, userMessage(), func(s string) bool { b.WriteString(s); return true })
			err, answer = e, b.String()
			_ = msg
		} else {
			msg, e := run.agent.Generate(context.Background(), userMessage())
			err = e
			if msg != nil {
				answer = msg.Content
			}
		}
		if err != nil {
			t.Fatalf("streamed=%v: the run failed instead of telling the model: %v", streamed, err)
		}
		if answer != "recovered" {
			t.Errorf("streamed=%v: answer %q, want the model's answer after it corrected the call", streamed, answer)
		}
		if inner.runs != 1 {
			t.Errorf("streamed=%v: tool ran %d times, want only the corrected call", streamed, inner.runs)
		}
		if len(m.seen) < 2 {
			t.Fatalf("streamed=%v: the model was called %d times", streamed, len(m.seen))
		}
		var told string
		for _, msg := range m.seen[1] {
			if msg.Role == schema.Tool {
				told += msg.Content
			}
		}
		if !strings.Contains(told, "query") || !strings.Contains(told, "参数校验失败") {
			t.Errorf("streamed=%v: the model was told %q, want the validation message naming the missing field", streamed, told)
		}
		if types := log.types(); len(types) < 2 || types[0] != "action" || types[1] != "observation" {
			t.Errorf("streamed=%v: events %v, want the rejected call reported as action then observation", streamed, types)
		}
	}
}
