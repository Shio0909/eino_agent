package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/database/repository"
)

// Cross-turn compaction against a real model. Skipped unless
// COMPACTION_LIVE_CONFIG points at a config file with a working llm section:
//
//	COMPACTION_LIVE_CONFIG=$PWD/configs/config.yaml go test ./internal/service -run LiveSessionHistory -v -count=1
func TestLiveSessionHistoryKeepsEarlyFactsAcrossTwoCompactions(t *testing.T) {
	path := os.Getenv("COMPACTION_LIVE_CONFIG")
	if path == "" {
		t.Skip("set COMPACTION_LIVE_CONFIG to run against a real model")
	}
	live, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	llm, _, err := container.NewLLMProvider(context.Background(), &live.LLM)
	if err != nil {
		t.Fatalf("build llm: %v", err)
	}

	svc, msgs, sums := newCompactionTestService(t, &summaryModel{})
	svc.chatModel = llm
	ctx := context.Background()

	plant := []struct{ role, text string }{
		{"user", "Please remember: our staging cluster is called orca-staging and listens on port 8443. The on-call engineer this month is Priya. We never deploy on Fridays."},
		{"assistant", "Noted: orca-staging on port 8443, on-call is Priya, no Friday deploys."},
	}
	for i, p := range plant {
		_ = msgs.Create(ctx, &repository.Message{
			ID: fmt.Sprintf("m%03d", i), SessionID: "s1", Role: p.role, Content: p.text,
			CreatedAt: historyClock.Add(time.Duration(i) * time.Minute),
		})
	}
	addHistory(msgs, "s1", 2, 38)

	ask := func(label string) string {
		h, err := svc.loadSessionHistory(ctx, "s1")
		if err != nil {
			t.Fatalf("%s: load history: %v", label, err)
		}
		memory := formatSessionHistory(h)
		stored, _ := sums.Get(ctx, "s1")
		if stored == nil || h.Summary == "" {
			t.Fatalf("%s: no summary was stored", label)
		}
		t.Logf("%s: summary degraded=%v covered=%d recent=%d summary_chars=%d",
			label, h.Degraded, stored.CoveredCount, len(h.Recent), len([]rune(h.Summary)))
		if h.Degraded {
			t.Errorf("%s: the real summariser failed and history fell back to the raw archive", label)
		}
		out, err := llm.Generate(ctx, []*schema.Message{
			{Role: schema.System, Content: "Answer using only the conversation memory below.\n\n" + memory},
			{Role: schema.User, Content: "What is the staging cluster called, which port does it listen on, who is on call, and on which weekday do we not deploy? One line."},
		})
		if err != nil {
			t.Fatalf("%s: ask: %v", label, err)
		}
		t.Logf("%s: answer: %s", label, out.Content)
		return out.Content
	}
	check := func(label, answer string) {
		for _, want := range []string{"orca-staging", "8443", "Priya", "Friday"} {
			if !strings.Contains(answer, want) {
				t.Errorf("%s: answer lost %q", label, want)
			}
		}
	}

	check("first compaction", ask("first compaction"))

	addHistory(msgs, "s1", 40, 24)
	check("second compaction (summary updated in place)", ask("second compaction"))
}
