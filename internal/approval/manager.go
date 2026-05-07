package approval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	StatusExpired  Status = "expired"
)

type DecisionValue string

const (
	DecisionApprove DecisionValue = "approve"
	DecisionReject  DecisionValue = "reject"
	DecisionExpire  DecisionValue = "expire"
)

type Request struct {
	TenantID  int            `json:"tenant_id"`
	UserID    string         `json:"user_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	TraceID   string         `json:"trace_id,omitempty"`
	Source    string         `json:"source"`
	Action    string         `json:"action"`
	ToolName  string         `json:"tool_name"`
	ToolInput string         `json:"tool_input"`
	Reason    string         `json:"reason"`
	RiskLevel string         `json:"risk_level"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type Decision struct {
	Decision      DecisionValue `json:"decision"`
	DeciderUserID string        `json:"decider_user_id,omitempty"`
	Reason        string        `json:"reason,omitempty"`
	DecidedAt     time.Time     `json:"decided_at,omitempty"`
}

type Approval struct {
	ID             string         `json:"approval_id"`
	TenantID       int            `json:"tenant_id"`
	UserID         string         `json:"user_id,omitempty"`
	SessionID      string         `json:"session_id,omitempty"`
	TraceID        string         `json:"trace_id,omitempty"`
	Source         string         `json:"source"`
	Action         string         `json:"action"`
	ToolName       string         `json:"tool_name"`
	ToolInput      string         `json:"tool_input"`
	Reason         string         `json:"reason"`
	RiskLevel      string         `json:"risk_level"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Status         Status         `json:"status"`
	ActionHash     string         `json:"action_hash"`
	CreatedAt      time.Time      `json:"created_at"`
	ExpiresAt      time.Time      `json:"expires_at"`
	DecidedAt      *time.Time     `json:"decided_at,omitempty"`
	DecisionReason string         `json:"decision_reason,omitempty"`
	DeciderUserID  string         `json:"decider_user_id,omitempty"`
}

type Manager struct {
	mu    sync.RWMutex
	ttl   time.Duration
	items map[string]*approvalState
	now   func() time.Time
}

type approvalState struct {
	approval Approval
	decision chan Decision
}

func NewManager(ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Manager{
		ttl:   ttl,
		items: make(map[string]*approvalState),
		now:   time.Now,
	}
}

func (m *Manager) Create(_ context.Context, req Request) (Approval, error) {
	if req.Action == "" {
		return Approval{}, fmt.Errorf("approval action is required")
	}
	now := m.now().UTC()
	approval := Approval{
		ID:         uuid.NewString(),
		TenantID:   req.TenantID,
		UserID:     req.UserID,
		SessionID:  req.SessionID,
		TraceID:    req.TraceID,
		Source:     req.Source,
		Action:     req.Action,
		ToolName:   req.ToolName,
		ToolInput:  req.ToolInput,
		Reason:     req.Reason,
		RiskLevel:  req.RiskLevel,
		Metadata:   copyMetadata(req.Metadata),
		Status:     StatusPending,
		ActionHash: ActionHash(req),
		CreatedAt:  now,
		ExpiresAt:  now.Add(m.ttl),
	}

	m.mu.Lock()
	m.items[approval.ID] = &approvalState{approval: approval, decision: make(chan Decision, 1)}
	m.mu.Unlock()
	return approval, nil
}

func (m *Manager) Wait(ctx context.Context, id string) (Decision, error) {
	state, ok := m.state(id)
	if !ok {
		return Decision{}, fmt.Errorf("approval not found")
	}

	m.mu.RLock()
	approval := state.approval
	m.mu.RUnlock()
	if approval.Status != StatusPending {
		return decisionFromApproval(approval), nil
	}

	timer := time.NewTimer(time.Until(approval.ExpiresAt))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return Decision{}, ctx.Err()
	case decision := <-state.decision:
		return decision, nil
	case <-timer.C:
		decision := Decision{Decision: DecisionExpire, DecidedAt: m.now().UTC()}
		m.setDecision(id, decision, StatusExpired)
		return decision, nil
	}
}

func (m *Manager) Decide(_ context.Context, id string, decision Decision) error {
	if decision.Decision != DecisionApprove && decision.Decision != DecisionReject {
		return fmt.Errorf("invalid approval decision: %s", decision.Decision)
	}
	if decision.DecidedAt.IsZero() {
		decision.DecidedAt = m.now().UTC()
	}
	status := StatusApproved
	if decision.Decision == DecisionReject {
		status = StatusRejected
	}
	return m.setDecision(id, decision, status)
}

func (m *Manager) Get(_ context.Context, id string) (Approval, bool) {
	state, ok := m.state(id)
	if !ok {
		return Approval{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return state.approval, true
}

func (m *Manager) ListPending(_ context.Context, tenantID int) []Approval {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Approval, 0)
	for _, state := range m.items {
		approval := state.approval
		if approval.Status != StatusPending {
			continue
		}
		if tenantID > 0 && approval.TenantID != tenantID {
			continue
		}
		out = append(out, approval)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (m *Manager) ValidateApproved(ctx context.Context, id string, req Request) error {
	approval, ok := m.Get(ctx, id)
	if !ok {
		return fmt.Errorf("approval not found")
	}
	if approval.Status != StatusApproved {
		return fmt.Errorf("approval status is %s", approval.Status)
	}
	if m.now().UTC().After(approval.ExpiresAt) {
		return fmt.Errorf("approval expired")
	}
	if approval.ActionHash != ActionHash(req) {
		return fmt.Errorf("approval action does not match request")
	}
	return nil
}

func ActionHash(req Request) string {
	payload := struct {
		TenantID  int    `json:"tenant_id"`
		UserID    string `json:"user_id,omitempty"`
		Source    string `json:"source"`
		Action    string `json:"action"`
		ToolName  string `json:"tool_name"`
		ToolInput string `json:"tool_input"`
	}{
		TenantID:  req.TenantID,
		UserID:    req.UserID,
		Source:    req.Source,
		Action:    req.Action,
		ToolName:  req.ToolName,
		ToolInput: req.ToolInput,
	}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (m *Manager) state(id string) (*approvalState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.items[id]
	return state, ok
}

func (m *Manager) setDecision(id string, decision Decision, status Status) error {
	m.mu.Lock()
	state, ok := m.items[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("approval not found")
	}
	if state.approval.Status != StatusPending {
		m.mu.Unlock()
		return fmt.Errorf("approval already decided")
	}
	decidedAt := decision.DecidedAt
	state.approval.Status = status
	state.approval.DecidedAt = &decidedAt
	state.approval.DecisionReason = decision.Reason
	state.approval.DeciderUserID = decision.DeciderUserID
	m.mu.Unlock()

	select {
	case state.decision <- decision:
	default:
	}
	return nil
}

func decisionFromApproval(approval Approval) Decision {
	decision := Decision{Reason: approval.DecisionReason, DeciderUserID: approval.DeciderUserID}
	if approval.DecidedAt != nil {
		decision.DecidedAt = *approval.DecidedAt
	}
	switch approval.Status {
	case StatusApproved:
		decision.Decision = DecisionApprove
	case StatusRejected:
		decision.Decision = DecisionReject
	case StatusExpired:
		decision.Decision = DecisionExpire
	}
	return decision
}

func copyMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
