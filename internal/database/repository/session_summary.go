package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"eino_agent/internal/database/postgres"
)

// SessionSummary replaces a session's messages up to and including
// CoveredUntilMessageID when conversation memory is built.
type SessionSummary struct {
	SessionID             string    `json:"session_id"`
	Summary               string    `json:"summary"`
	CoveredUntilMessageID string    `json:"covered_until_message_id"`
	CoveredUntilAt        time.Time `json:"covered_until_at"`
	CoveredCount          int       `json:"covered_count"`
	Degraded              bool      `json:"degraded"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// SessionSummaryRepository stores one rolling summary per session.
type SessionSummaryRepository interface {
	// Get returns nil, nil when the session has no summary yet.
	Get(ctx context.Context, sessionID string) (*SessionSummary, error)
	Upsert(ctx context.Context, s *SessionSummary) error
}

type pgSessionSummaryRepo struct {
	db *postgres.DB
}

func NewSessionSummaryRepository(db *postgres.DB) SessionSummaryRepository {
	return &pgSessionSummaryRepo{db: db}
}

func (r *pgSessionSummaryRepo) Get(ctx context.Context, sessionID string) (*SessionSummary, error) {
	s := &SessionSummary{}
	err := r.db.QueryRow(ctx, `
		SELECT session_id, summary, covered_until_message_id, covered_until_at, covered_count, degraded, updated_at
		FROM session_summaries WHERE session_id = $1
	`, sessionID).Scan(&s.SessionID, &s.Summary, &s.CoveredUntilMessageID, &s.CoveredUntilAt, &s.CoveredCount, &s.Degraded, &s.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *pgSessionSummaryRepo) Upsert(ctx context.Context, s *SessionSummary) error {
	return r.db.Exec(ctx, `
		INSERT INTO session_summaries (session_id, summary, covered_until_message_id, covered_until_at, covered_count, degraded, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
		ON CONFLICT (session_id) DO UPDATE SET
			summary = EXCLUDED.summary,
			covered_until_message_id = EXCLUDED.covered_until_message_id,
			covered_until_at = EXCLUDED.covered_until_at,
			covered_count = EXCLUDED.covered_count,
			degraded = EXCLUDED.degraded,
			updated_at = CURRENT_TIMESTAMP
	`, s.SessionID, s.Summary, s.CoveredUntilMessageID, s.CoveredUntilAt, s.CoveredCount, s.Degraded)
}
