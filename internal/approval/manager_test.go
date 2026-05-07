package approval

import (
	"context"
	"testing"
	"time"
)

func TestManagerApproveUnblocksWaiter(t *testing.T) {
	manager := NewManager(1 * time.Minute)
	req := Request{
		TenantID:  1,
		UserID:    "user-1",
		Source:    "chat_stream",
		Action:    "tool:import_url",
		ToolName:  "import_url",
		ToolInput: `{"url":"https://example.com"}`,
		Reason:    "external write action",
	}
	approval, err := manager.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	waitCh := make(chan Decision, 1)
	errCh := make(chan error, 1)
	go func() {
		decision, err := manager.Wait(context.Background(), approval.ID)
		if err != nil {
			errCh <- err
			return
		}
		waitCh <- decision
	}()

	if err := manager.Decide(context.Background(), approval.ID, Decision{Decision: DecisionApprove, DeciderUserID: "user-1", Reason: "looks safe"}); err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("Wait returned error: %v", err)
	case decision := <-waitCh:
		if decision.Decision != DecisionApprove || decision.Reason != "looks safe" {
			t.Fatalf("unexpected decision: %#v", decision)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not unblock")
	}
}

func TestManagerRejectSkipsTool(t *testing.T) {
	manager := NewManager(1 * time.Minute)
	approval, err := manager.Create(context.Background(), Request{TenantID: 1, UserID: "user-1", Action: "delete_document", ToolName: "delete_document", ToolInput: `{"id":"doc-1"}`})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := manager.Decide(context.Background(), approval.ID, Decision{Decision: DecisionReject, DeciderUserID: "user-1", Reason: "wrong doc"}); err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	decision, err := manager.Wait(context.Background(), approval.ID)
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if decision.Decision != DecisionReject || decision.Reason != "wrong doc" {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestManagerRejectsDuplicateDecision(t *testing.T) {
	manager := NewManager(1 * time.Minute)
	approval, err := manager.Create(context.Background(), Request{TenantID: 1, Action: "delete_document", ToolName: "delete_document", ToolInput: `{"id":"doc-1"}`})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := manager.Decide(context.Background(), approval.ID, Decision{Decision: DecisionApprove}); err != nil {
		t.Fatalf("first Decide returned error: %v", err)
	}
	if err := manager.Decide(context.Background(), approval.ID, Decision{Decision: DecisionReject}); err == nil {
		t.Fatal("second Decide succeeded, want error")
	}
}

func TestManagerExpiresPendingApproval(t *testing.T) {
	manager := NewManager(10 * time.Millisecond)
	approval, err := manager.Create(context.Background(), Request{TenantID: 1, Action: "import_url", ToolName: "import_url", ToolInput: `{"url":"https://example.com"}`})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	decision, err := manager.Wait(context.Background(), approval.ID)
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if decision.Decision != DecisionExpire {
		t.Fatalf("unexpected decision: %#v", decision)
	}
	stored, ok := manager.Get(context.Background(), approval.ID)
	if !ok {
		t.Fatal("approval not found")
	}
	if stored.Status != StatusExpired {
		t.Fatalf("status = %s, want %s", stored.Status, StatusExpired)
	}
}

func TestManagerValidateApprovedActionHash(t *testing.T) {
	manager := NewManager(1 * time.Minute)
	approval, err := manager.Create(context.Background(), Request{TenantID: 1, Action: "import_url", ToolName: "import_url", ToolInput: `{"url":"https://example.com/a"}`})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := manager.Decide(context.Background(), approval.ID, Decision{Decision: DecisionApprove}); err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if err := manager.ValidateApproved(context.Background(), approval.ID, Request{TenantID: 1, Action: "import_url", ToolName: "import_url", ToolInput: `{"url":"https://example.com/a"}`}); err != nil {
		t.Fatalf("ValidateApproved returned error: %v", err)
	}
	if err := manager.ValidateApproved(context.Background(), approval.ID, Request{TenantID: 1, Action: "import_url", ToolName: "import_url", ToolInput: `{"url":"https://example.com/b"}`}); err == nil {
		t.Fatal("ValidateApproved accepted mismatched action hash")
	}
}
