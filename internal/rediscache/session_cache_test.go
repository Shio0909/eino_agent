package rediscache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/config"
)

func TestSessionCacheRoundTripAndInvalidate(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run error = %v", err)
	}
	defer mr.Close()

	client, err := NewClient(context.Background(), config.RedisConfig{Addr: mr.Addr()})
	if err != nil {
		t.Fatalf("NewClient error = %v", err)
	}
	defer client.Close()

	store := NewSessionCache(client)
	ctx := context.Background()
	messages := []cachepkg.SessionMessage{
		{Role: "user", Content: "hello", CreatedAt: time.Now().Add(-time.Minute)},
		{Role: "assistant", Content: "world", CreatedAt: time.Now()},
	}

	if err := store.SetRecentMessages(ctx, "session-1", messages, 5*time.Minute); err != nil {
		t.Fatalf("SetRecentMessages error = %v", err)
	}
	if err := store.SetSummary(ctx, "session-1", "summary", 5*time.Minute); err != nil {
		t.Fatalf("SetSummary error = %v", err)
	}

	gotMessages, hit, err := store.GetRecentMessages(ctx, "session-1", 10)
	if err != nil {
		t.Fatalf("GetRecentMessages error = %v", err)
	}
	if !hit {
		t.Fatal("expected cache hit for recent messages")
	}
	if len(gotMessages) != 2 || gotMessages[0].Content != "hello" || gotMessages[1].Content != "world" {
		t.Fatalf("unexpected recent messages: %#v", gotMessages)
	}

	summary, hit, err := store.GetSummary(ctx, "session-1")
	if err != nil {
		t.Fatalf("GetSummary error = %v", err)
	}
	if !hit || summary != "summary" {
		t.Fatalf("unexpected summary result: hit=%v summary=%q", hit, summary)
	}

	if err := store.InvalidateSession(ctx, "session-1"); err != nil {
		t.Fatalf("InvalidateSession error = %v", err)
	}

	_, hit, err = store.GetRecentMessages(ctx, "session-1", 10)
	if err != nil {
		t.Fatalf("GetRecentMessages after invalidate error = %v", err)
	}
	if hit {
		t.Fatal("expected recent message cache miss after invalidation")
	}

	_, hit, err = store.GetSummary(ctx, "session-1")
	if err != nil {
		t.Fatalf("GetSummary after invalidate error = %v", err)
	}
	if hit {
		t.Fatal("expected summary cache miss after invalidation")
	}
}
