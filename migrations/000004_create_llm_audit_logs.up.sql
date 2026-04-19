-- LLM 调用审计日志表
-- 记录每次 LLM API 调用的模型、Token 用量、耗时和预估费用，
-- 用于成本分析、性能优化和安全审计。
CREATE TABLE IF NOT EXISTS llm_audit_logs (
    id               BIGSERIAL PRIMARY KEY,
    trace_id         VARCHAR(36)    NOT NULL DEFAULT '',
    user_id          VARCHAR(100)   NOT NULL DEFAULT '',
    session_id       VARCHAR(100)   NOT NULL DEFAULT '',
    provider         VARCHAR(50)    NOT NULL,
    model            VARCHAR(100)   NOT NULL,
    mode             VARCHAR(50)    NOT NULL DEFAULT 'chat', -- chat | pipeline | embedding
    prompt_tokens    INTEGER        NOT NULL DEFAULT 0,
    completion_tokens INTEGER       NOT NULL DEFAULT 0,
    total_tokens     INTEGER        NOT NULL DEFAULT 0,
    latency_ms       INTEGER        NOT NULL DEFAULT 0,
    -- 预估费用（USD），基于公开定价估算，非实际账单
    cost_estimate_usd DECIMAL(10,6) NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_llm_audit_trace_id   ON llm_audit_logs(trace_id);
CREATE INDEX IF NOT EXISTS idx_llm_audit_created_at ON llm_audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_llm_audit_user_id    ON llm_audit_logs(user_id);
