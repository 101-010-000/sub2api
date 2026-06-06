-- 随速通：后台隐藏备用分组路由配置。
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS suisu_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS suisu_fallback_group_id BIGINT,
    ADD COLUMN IF NOT EXISTS suisu_slow_route_ratio DECIMAL(6, 4) NOT NULL DEFAULT 0.0000,
    ADD COLUMN IF NOT EXISTS suisu_busy_route_ratio DECIMAL(6, 4) NOT NULL DEFAULT 0.0000;

CREATE INDEX IF NOT EXISTS idx_groups_suisu_fallback_group_id
    ON groups(suisu_fallback_group_id)
    WHERE suisu_fallback_group_id IS NOT NULL;
