package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/config"
	"eino_agent/internal/container"
)

// Streamed ReAct runs against a real model. Skipped unless
// COMPACTION_LIVE_CONFIG points at a config file with a working llm section:
//
//	COMPACTION_LIVE_CONFIG=$PWD/configs/config.yaml go test ./internal/service -run LiveStreamAgent -v -count=1
func liveStreamRun(t *testing.T, peek int) (*agentRun, *liveTool, string) {
	t.Helper()
	path := os.Getenv("COMPACTION_LIVE_CONFIG")
	if path == "" {
		t.Skip("set COMPACTION_LIVE_CONFIG to run against a real model")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	m, _, err := container.NewLLMProvider(context.Background(), &cfg.LLM)
	if err != nil {
		t.Fatalf("build llm: %v", err)
	}
	llm, ok := m.(model.ToolCallingChatModel)
	if !ok {
		t.Fatalf("model %T does not support tool calling", m)
	}

	code := fmt.Sprintf("zx-%d", time.Now().UnixNano()%1_000_000)
	runs := &liveTool{}
	type in struct {
		Topic string `json:"topic" jsonschema:"description=the topic to look up"`
	}
	lookup, err := utils.InferTool("lookup", "Look up the access code of a topic. The answer cannot be known without this tool.",
		func(_ context.Context, _ in) (string, error) {
			runs.ran()
			return "The access code is " + code + ".", nil
		})
	if err != nil {
		t.Fatal(err)
	}
	toolsConfig := compose.ToolsNodeConfig{
		Tools:               []einotool.BaseTool{lookup},
		ToolCallMiddlewares: toolCallMiddlewares(newToolEvents(func(StreamEvent) {})),
	}
	agent, err := react.NewAgent(context.Background(), &react.AgentConfig{
		ToolCallingModel:      llm,
		ToolsConfig:           toolsConfig,
		MaxStep:               12,
		StreamToolCallChecker: newStreamToolCallChecker(peek),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &agentRun{agent: agent, toolsConfig: toolsConfig}, runs, code
}

// liveTool counts the calls of the lookup tool and remembers when the last one began.
type liveTool struct {
	n    atomic.Int32
	mu   sync.Mutex
	last time.Time
}

func (l *liveTool) ran() {
	l.n.Add(1)
	l.mu.Lock()
	l.last = time.Now()
	l.mu.Unlock()
}

func (l *liveTool) count() int32 { return l.n.Load() }

func (l *liveTool) lastRun() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.last
}

type liveStream struct {
	mu     sync.Mutex
	text   strings.Builder
	events int
	first  time.Duration
	start  time.Time
}

func (l *liveStream) emit(s string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.events == 0 {
		l.first = time.Since(l.start)
	}
	l.events++
	l.text.WriteString(s)
	return true
}

// A long answer after a tool call must arrive in pieces, well before the model
// has finished writing it.
func TestLiveStreamAgentAnswersLiveAfterATool(t *testing.T) {
	run, runs, code := liveStreamRun(t, 256)
	ls := &liveStream{start: time.Now()}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_, err := streamAgent(ctx, run, []*schema.Message{
		{Role: schema.System, Content: "Always call the lookup tool before answering."},
		{Role: schema.User, Content: "Look up the access code for topic alpha, then write at least 12 sentences about why access codes should be rotated, and state the code in the first sentence."},
	}, ls.emit)
	total := time.Since(ls.start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("tool runs=%d, first text at %v of %v, %d pieces, %d chars", runs.count(), ls.first.Round(time.Millisecond), total.Round(time.Millisecond), ls.events, ls.text.Len())
	if runs.count() < 1 {
		t.Fatal("the tool was never called")
	}
	if !strings.Contains(ls.text.String(), code) {
		t.Fatalf("the answer does not contain the tool's code %s: %q", code, ls.text.String())
	}
	if ls.events < 5 {
		t.Fatalf("answer arrived in %d pieces, want a stream", ls.events)
	}
	if ls.first > total*8/10 {
		t.Fatalf("first text at %v of %v: the answer was not streamed", ls.first, total)
	}
}

// With a window far smaller than the preamble the model writes before calling
// the tool, the turn is taken for the answer first. The tool call must still be
// run and its result used.
func TestLiveStreamAgentSurvivesToolCallAfterThePeekWindow(t *testing.T) {
	run, runs, code := liveStreamRun(t, 8)
	ls := &liveStream{start: time.Now()}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	const trials = 3
	late := 0
	for i := 0; i < trials; i++ {
		before := runs.count()
		ls = &liveStream{start: time.Now()}
		_, err := streamAgent(ctx, run, []*schema.Message{
			{Role: schema.User, Content: "First write two sentences explaining what you are about to do (at least 80 characters). Then call the lookup tool for topic beta. Then tell me the access code it returned."},
		}, ls.emit)
		if err != nil {
			t.Fatalf("trial %d: %v", i, err)
		}
		t.Logf("trial %d: tool runs=%d, first text at %v, %d pieces", i, runs.count()-before, ls.first.Round(time.Millisecond), ls.events)
		if runs.count() == before {
			t.Fatalf("trial %d: the tool was never called: %q", i, ls.text.String())
		}
		if !strings.Contains(ls.text.String(), code) {
			t.Fatalf("trial %d: the answer lacks the tool result %s: %q", i, code, ls.text.String())
		}
		if ls.start.Add(ls.first).Before(runs.lastRun()) {
			late++
		}
	}
	t.Logf("%d of %d trials wrote text before the tool call ran, which is the case this test is about", late, trials)
}
