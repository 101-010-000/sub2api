-- 用户级 API Key 动态活跃 IP 上限。
-- 0 表示不限制；visible 控制普通用户接口是否展示该上限。

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS api_key_max_active_ips INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS api_key_max_active_ips_visible BOOLEAN NOT NULL DEFAULT FALSE;
