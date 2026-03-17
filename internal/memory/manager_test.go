package memory

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestManagerGetMessages_NewSession(t *testing.T) {
	mgr := NewManager(nil, DefaultConfig())
	msgs := mgr.GetMessages("new-session", "你是助手")

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != schema.System {
		t.Errorf("expected system role, got %s", msgs[0].Role)
	}
	if msgs[0].Content != "你是助手" {
		t.Errorf("expected system prompt, got %s", msgs[0].Content)
	}
}

func TestManagerAddAndGetMessages(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig()
	cfg.EnableAutoSummary = false // 禁用自动摘要，简化测试
	mgr := NewManager(nil, cfg)

	sessionID := "test-session"

	// 添加对话
	err := mgr.AddUserAssistantPair(ctx, sessionID, "你好", "你好！有什么可以帮你的？")
	if err != nil {
		t.Fatalf("AddUserAssistantPair failed: %v", err)
	}

	err = mgr.AddUserAssistantPair(ctx, sessionID, "今天天气如何？", "今天天气晴朗。")
	if err != nil {
		t.Fatalf("AddUserAssistantPair failed: %v", err)
	}

	msgs := mgr.GetMessages(sessionID, "你是助手")

	// system_prompt + 4 条消息 = 5
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs))
	}

	// 验证消息顺序
	if msgs[1].Role != schema.User || msgs[1].Content != "你好" {
		t.Errorf("unexpected message[1]: %+v", msgs[1])
	}
	if msgs[2].Role != schema.Assistant || msgs[2].Content != "你好！有什么可以帮你的？" {
		t.Errorf("unexpected message[2]: %+v", msgs[2])
	}
}

func TestManagerSlidingWindow(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig()
	cfg.WindowSize = 2 // 只保留最近 2 轮（4 条消息）
	cfg.EnableAutoSummary = false
	mgr := NewManager(nil, cfg)

	sessionID := "window-test"

	// 添加 5 轮对话（10 条消息）
	for i := 0; i < 5; i++ {
		err := mgr.AddUserAssistantPair(ctx, sessionID, "问题", "回答")
		if err != nil {
			t.Fatal(err)
		}
	}

	msgs := mgr.GetMessages(sessionID, "prompt")

	// system_prompt + 最近 4 条 = 5
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages (1 system + 4 window), got %d", len(msgs))
	}
}

func TestManagerClearSession(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig()
	cfg.EnableAutoSummary = false
	mgr := NewManager(nil, cfg)

	sessionID := "clear-test"
	_ = mgr.AddMessage(ctx, sessionID, &schema.Message{Role: schema.User, Content: "test"})

	if mgr.GetSessionCount() != 1 {
		t.Fatalf("expected 1 session")
	}

	mgr.ClearSession(sessionID)

	if mgr.GetSessionCount() != 0 {
		t.Fatalf("expected 0 sessions after clear")
	}
}

func TestManagerLongTermContext_Empty(t *testing.T) {
	mgr := NewManager(nil, DefaultConfig())
	ctx := mgr.GetLongTermContext("unknown-user")
	if ctx != "" {
		t.Errorf("expected empty long term context, got %q", ctx)
	}
}
