CREATE TABLE IF NOT EXISTS touch_pie_device_sessions (
    id BIGSERIAL PRIMARY KEY,
    device_code_hash VARCHAR(64) NOT NULL UNIQUE,
    user_code_hash VARCHAR(64) NOT NULL UNIQUE,
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    approved_at TIMESTAMPTZ NULL,
    consumed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_touch_pie_device_sessions_user_code_status
    ON touch_pie_device_sessions(user_code_hash, status);

CREATE INDEX IF NOT EXISTS idx_touch_pie_device_sessions_expires_at
    ON touch_pie_device_sessions(expires_at);
