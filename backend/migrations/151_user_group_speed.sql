-- User-visible fast/slow quota ("优速通") settings and per-user overrides.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS speed_config_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS user_speed_config_allowed BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS default_fast_quota_ratio DECIMAL(6, 4) NOT NULL DEFAULT 0.3000,
    ADD COLUMN IF NOT EXISTS min_fast_quota_ratio DECIMAL(6, 4) NOT NULL DEFAULT 0.1000,
    ADD COLUMN IF NOT EXISTS max_fast_quota_ratio DECIMAL(6, 4) NOT NULL DEFAULT 0.8000,
    ADD COLUMN IF NOT EXISTS default_slow_delay_min_seconds INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS default_slow_delay_max_seconds INT NOT NULL DEFAULT 5,
    ADD COLUMN IF NOT EXISTS max_slow_delay_seconds INT NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS default_slow_reject_rate DECIMAL(6, 4) NOT NULL DEFAULT 0.0000,
    ADD COLUMN IF NOT EXISTS max_slow_reject_rate DECIMAL(6, 4) NOT NULL DEFAULT 0.5000;

CREATE TABLE IF NOT EXISTS user_group_speed_configs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,

    fast_quota_ratio DECIMAL(6, 4),
    slow_delay_min_seconds INT,
    slow_delay_max_seconds INT,
    slow_reject_rate DECIMAL(6, 4),

    daily_window_start TIMESTAMPTZ,
    weekly_window_start TIMESTAMPTZ,
    monthly_window_start TIMESTAMPTZ,
    daily_usage_usd DECIMAL(20, 10) NOT NULL DEFAULT 0,
    weekly_usage_usd DECIMAL(20, 10) NOT NULL DEFAULT 0,
    monthly_usage_usd DECIMAL(20, 10) NOT NULL DEFAULT 0,

    slow_request_count BIGINT NOT NULL DEFAULT 0,
    slow_reject_count BIGINT NOT NULL DEFAULT 0,
    last_slow_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(user_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_user_group_speed_user ON user_group_speed_configs(user_id);
CREATE INDEX IF NOT EXISTS idx_user_group_speed_group ON user_group_speed_configs(group_id);
