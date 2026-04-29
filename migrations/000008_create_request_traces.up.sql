CREATE TABLE IF NOT EXISTS request_traces (
    id          BIGSERIAL PRIMARY KEY,
    trace_id    VARCHAR(36)  NOT NULL UNIQUE,
    tenant_id   INTEGER      NOT NULL DEFAULT 1,
    user_id     VARCHAR(100) NOT NULL DEFAULT '',
    session_id  VARCHAR(100) NOT NULL DEFAULT '',
    message_id  VARCHAR(36)  NOT NULL DEFAULT '',
    mode        VARCHAR(50)  NOT NULL DEFAULT '',
    status      VARCHAR(50)  NOT NULL DEFAULT 'completed',
    latency_ms  INTEGER      NOT NULL DEFAULT 0,
    steps       JSONB        NOT NULL DEFAULT '[]'::jsonb,
    summary     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    error       TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_request_traces_session_created ON request_traces(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_traces_user_created ON request_traces(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_traces_created_at ON request_traces(created_at DESC);
