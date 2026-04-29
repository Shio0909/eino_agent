package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"eino_agent/internal/database/postgres"
)

type RequestTrace struct {
	ID        int64     `json:"id"`
	TraceID   string    `json:"trace_id"`
	TenantID  int       `json:"tenant_id"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	MessageID string    `json:"message_id"`
	Mode      string    `json:"mode"`
	Status    string    `json:"status"`
	LatencyMs int       `json:"latency_ms"`
	Steps     any       `json:"steps"`
	Summary   JSON      `json:"summary"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RequestTraceFilter struct {
	TenantID  int
	UserID    string
	SessionID string
	Limit     int
	Offset    int
}

type RequestTraceRepository interface {
	Create(ctx context.Context, trace *RequestTrace) error
	UpdateMessageID(ctx context.Context, traceID, messageID string) error
	GetByTraceID(ctx context.Context, traceID string) (*RequestTrace, error)
	List(ctx context.Context, filter RequestTraceFilter) ([]*RequestTrace, int, error)
}

type requestTraceRepo struct {
	db *postgres.DB
}

func NewRequestTraceRepository(db *postgres.DB) RequestTraceRepository {
	return &requestTraceRepo{db: db}
}

func (r *requestTraceRepo) Create(ctx context.Context, trace *RequestTrace) error {
	if trace == nil {
		return nil
	}
	steps, _ := json.Marshal(trace.Steps)
	summary, _ := json.Marshal(trace.Summary)
	return r.db.QueryRow(ctx, `
		INSERT INTO request_traces
			(trace_id, tenant_id, user_id, session_id, message_id, mode, status, latency_ms, steps, summary, error)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (trace_id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			user_id = EXCLUDED.user_id,
			session_id = EXCLUDED.session_id,
			message_id = EXCLUDED.message_id,
			mode = EXCLUDED.mode,
			status = EXCLUDED.status,
			latency_ms = EXCLUDED.latency_ms,
			steps = EXCLUDED.steps,
			summary = EXCLUDED.summary,
			error = EXCLUDED.error,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`, trace.TraceID, trace.TenantID, trace.UserID, trace.SessionID, trace.MessageID, trace.Mode, trace.Status, trace.LatencyMs, steps, summary, trace.Error).
		Scan(&trace.ID, &trace.CreatedAt, &trace.UpdatedAt)
}

func (r *requestTraceRepo) UpdateMessageID(ctx context.Context, traceID, messageID string) error {
	if strings.TrimSpace(traceID) == "" || strings.TrimSpace(messageID) == "" {
		return nil
	}
	err := r.db.Exec(ctx, `
		UPDATE request_traces
		SET message_id = $2, updated_at = NOW()
		WHERE trace_id = $1
	`, traceID, messageID)
	return err
}

func (r *requestTraceRepo) GetByTraceID(ctx context.Context, traceID string) (*RequestTrace, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return nil, nil
	}
	trace := &RequestTrace{}
	var steps, summary []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, trace_id, tenant_id, user_id, session_id, message_id, mode, status, latency_ms, steps, summary, error, created_at, updated_at
		FROM request_traces
		WHERE trace_id = $1
	`, traceID).Scan(&trace.ID, &trace.TraceID, &trace.TenantID, &trace.UserID, &trace.SessionID, &trace.MessageID, &trace.Mode, &trace.Status, &trace.LatencyMs, &steps, &summary, &trace.Error, &trace.CreatedAt, &trace.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	_ = json.Unmarshal(steps, &trace.Steps)
	_ = json.Unmarshal(summary, &trace.Summary)
	return trace, nil
}

func (r *requestTraceRepo) List(ctx context.Context, filter RequestTraceFilter) ([]*RequestTrace, int, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	where := []string{"tenant_id = $1"}
	args := []any{filter.TenantID}
	if strings.TrimSpace(filter.UserID) != "" {
		args = append(args, filter.UserID)
		where = append(where, fmt.Sprintf("user_id = $%d", len(args)))
	}
	if strings.TrimSpace(filter.SessionID) != "" {
		args = append(args, filter.SessionID)
		where = append(where, fmt.Sprintf("session_id = $%d", len(args)))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM request_traces WHERE %s", whereSQL)
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, trace_id, tenant_id, user_id, session_id, message_id, mode, status, latency_ms, summary, error, created_at, updated_at
		FROM request_traces
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []*RequestTrace{}
	for rows.Next() {
		trace := &RequestTrace{}
		var summary []byte
		if err := rows.Scan(&trace.ID, &trace.TraceID, &trace.TenantID, &trace.UserID, &trace.SessionID, &trace.MessageID, &trace.Mode, &trace.Status, &trace.LatencyMs, &summary, &trace.Error, &trace.CreatedAt, &trace.UpdatedAt); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(summary, &trace.Summary)
		items = append(items, trace)
	}
	return items, total, rows.Err()
}
