ALTER TABLE touch_pie_device_sessions
    ADD COLUMN IF NOT EXISTS api_key_id BIGINT NULL REFERENCES api_keys(id) ON DELETE SET NULL;
