package rediscache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/config"
)

func TestImportStateStoreRoundTripAndDelete(t *testing.T) {
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

	store := NewImportStateStore(client)
	ctx := context.Background()
	state := &cachepkg.ImportTaskState{
		Status:     "processing",
		Stage:      "vectorizing",
		ChunkCount: 7,
		StartedAt:  time.Now().Add(-time.Minute),
		UpdatedAt:  time.Now(),
	}

	if err := store.SetTaskState(ctx, "task-1", state, 5*time.Minute); err != nil {
		t.Fatalf("SetTaskState error = %v", err)
	}

	got, hit, err := store.GetTaskState(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetTaskState error = %v", err)
	}
	if !hit || got == nil {
		t.Fatalf("expected task state hit, got hit=%v state=%#v", hit, got)
	}
	if got.Status != "processing" || got.Stage != "vectorizing" || got.ChunkCount != 7 {
		t.Fatalf("unexpected task state: %#v", got)
	}

	if err := store.DeleteTaskState(ctx, "task-1"); err != nil {
		t.Fatalf("DeleteTaskState error = %v", err)
	}

	_, hit, err = store.GetTaskState(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetTaskState after delete error = %v", err)
	}
	if hit {
		t.Fatal("expected task state miss after delete")
	}
}
