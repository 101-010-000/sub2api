-- 优速通策略审计字段：拒绝提示、状态、等待耗时与随速通路由。

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS speed_slow_reject_message TEXT NOT NULL DEFAULT 'You''ve sent too many requests.';

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS speed_state VARCHAR(16),
    ADD COLUMN IF NOT EXISTS speed_wait_ms INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS speed_route VARCHAR(24);

ALTER TABLE usage_logs
    ALTER COLUMN account_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_usage_logs_speed_state_created_at
    ON usage_logs(speed_state, created_at)
    WHERE speed_state IS NOT NULL;
