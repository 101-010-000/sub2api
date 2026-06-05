-- API Key runtime limits: dynamic active IP binding and per-key concurrency.

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS max_active_ips INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS ip_idle_timeout_seconds INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_concurrency INT NOT NULL DEFAULT 0;
