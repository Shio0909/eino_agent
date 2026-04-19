// Package repository - LLM 调用审计日志仓储
package repository

import (
	"context"
	"time"

	"eino_agent/internal/database/postgres"
)

// LLMAuditLog LLM 调用审计记录
type LLMAuditLog struct {
	ID               int64     `json:"id"`
	TraceID          string    `json:"trace_id"`
	UserID           string    `json:"user_id"`
	SessionID        string    `json:"session_id"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	Mode             string    `json:"mode"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	LatencyMs        int       `json:"latency_ms"`
	CostEstimateUSD  float64   `json:"cost_estimate_usd"`
	CreatedAt        time.Time `json:"created_at"`
}

// LLMAuditRepository LLM 审计日志仓储接口
type LLMAuditRepository interface {
	// Create 写入一条审计日志（fire-and-forget，忽略错误不影响主流程）
	Create(ctx context.Context, log *LLMAuditLog) error
	// List 按时间倒序分页查询（用于管理后台）
	List(ctx context.Context, userID string, limit, offset int) ([]*LLMAuditLog, error)
}

type llmAuditRepo struct {
	db *postgres.DB
}

// NewLLMAuditRepository 创建 LLM 审计日志仓储
func NewLLMAuditRepository(db *postgres.DB) LLMAuditRepository {
	return &llmAuditRepo{db: db}
}

func (r *llmAuditRepo) Create(ctx context.Context, entry *LLMAuditLog) error {
	const q = `
		INSERT INTO llm_audit_logs
			(trace_id, user_id, session_id, provider, model, mode,
			 prompt_tokens, completion_tokens, total_tokens,
			 latency_ms, cost_estimate_usd)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	_, err := r.db.Pool().Exec(ctx, q,
		entry.TraceID, entry.UserID, entry.SessionID,
		entry.Provider, entry.Model, entry.Mode,
		entry.PromptTokens, entry.CompletionTokens, entry.TotalTokens,
		entry.LatencyMs, entry.CostEstimateUSD,
	)
	return err
}

func (r *llmAuditRepo) List(ctx context.Context, userID string, limit, offset int) ([]*LLMAuditLog, error) {
	var q string
	var args []interface{}
	if userID != "" {
		q = `SELECT id,trace_id,user_id,session_id,provider,model,mode,
			        prompt_tokens,completion_tokens,total_tokens,latency_ms,
			        cost_estimate_usd,created_at
			   FROM llm_audit_logs WHERE user_id=$1
			   ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		args = []interface{}{userID, limit, offset}
	} else {
		q = `SELECT id,trace_id,user_id,session_id,provider,model,mode,
			        prompt_tokens,completion_tokens,total_tokens,latency_ms,
			        cost_estimate_usd,created_at
			   FROM llm_audit_logs
			   ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		args = []interface{}{limit, offset}
	}

	rows, err := r.db.Pool().Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*LLMAuditLog
	for rows.Next() {
		l := &LLMAuditLog{}
		if err := rows.Scan(
			&l.ID, &l.TraceID, &l.UserID, &l.SessionID,
			&l.Provider, &l.Model, &l.Mode,
			&l.PromptTokens, &l.CompletionTokens, &l.TotalTokens,
			&l.LatencyMs, &l.CostEstimateUSD, &l.CreatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
