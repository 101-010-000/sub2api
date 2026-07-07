-- 请求级地区/客户端信号风控：事件记录与 API Key 作用域 UA ban

CREATE TABLE IF NOT EXISTS request_risk_events (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    user_id BIGINT,
    api_key_id BIGINT,
    account_id BIGINT,
    request_id VARCHAR(255) NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    session_id_hash CHAR(64) NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    user_agent_hash CHAR(64) NOT NULL DEFAULT '',
    inference_geo VARCHAR(32) NOT NULL DEFAULT '',
    timezone VARCHAR(128) NOT NULL DEFAULT '',
    platform VARCHAR(32) NOT NULL DEFAULT '',
    language_signals_json TEXT NOT NULL DEFAULT '{}',
    chinese_intensity DOUBLE PRECISION NOT NULL DEFAULT 0,
    matched_rules_json TEXT NOT NULL DEFAULT '[]',
    action VARCHAR(64) NOT NULL DEFAULT '',
    reason_code VARCHAR(128) NOT NULL DEFAULT '',
    raw_headers_json TEXT NOT NULL DEFAULT '{}',
    x_foo_raw TEXT NOT NULL DEFAULT '',
    request_path VARCHAR(255) NOT NULL DEFAULT '',
    model VARCHAR(255) NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_request_risk_events_created_at ON request_risk_events (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_risk_events_expires_at ON request_risk_events (expires_at);
CREATE INDEX IF NOT EXISTS idx_request_risk_events_api_key_id ON request_risk_events (api_key_id);
CREATE INDEX IF NOT EXISTS idx_request_risk_events_user_id ON request_risk_events (user_id);
CREATE INDEX IF NOT EXISTS idx_request_risk_events_session_hash ON request_risk_events (session_id_hash);
CREATE INDEX IF NOT EXISTS idx_request_risk_events_ua_hash ON request_risk_events (user_agent_hash);

CREATE TABLE IF NOT EXISTS request_risk_ua_bans (
    id BIGSERIAL PRIMARY KEY,
    api_key_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL DEFAULT 0,
    user_agent_hash CHAR(64) NOT NULL,
    user_agent TEXT NOT NULL DEFAULT '',
    reason VARCHAR(128) NOT NULL DEFAULT '',
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    banned_until TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT request_risk_ua_bans_api_key_ua_unique UNIQUE (api_key_id, user_agent_hash)
);

CREATE INDEX IF NOT EXISTS idx_request_risk_ua_bans_banned_until ON request_risk_ua_bans (banned_until);
CREATE INDEX IF NOT EXISTS idx_request_risk_ua_bans_api_key_hash ON request_risk_ua_bans (api_key_id, user_agent_hash);
