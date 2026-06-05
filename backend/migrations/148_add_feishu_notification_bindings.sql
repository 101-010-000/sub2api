CREATE TABLE IF NOT EXISTS user_feishu_identity_bindings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    app_id TEXT NOT NULL,
    tenant_key TEXT NOT NULL DEFAULT '',
    open_id TEXT NOT NULL,
    union_id TEXT NOT NULL,
    purpose TEXT NOT NULL DEFAULT 'notify',
    notification_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    bound_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_feishu_identity_bindings_purpose_check
        CHECK (purpose IN ('notify', 'panel')),
    CONSTRAINT user_feishu_identity_bindings_app_id_check
        CHECK (BTRIM(app_id) <> ''),
    CONSTRAINT user_feishu_identity_bindings_open_id_check
        CHECK (BTRIM(open_id) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS user_feishu_identity_bindings_app_open_purpose_uq
    ON user_feishu_identity_bindings (app_id, open_id, purpose);

CREATE INDEX IF NOT EXISTS user_feishu_identity_bindings_union_purpose_idx
    ON user_feishu_identity_bindings (tenant_key, union_id, purpose)
    WHERE union_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS user_feishu_identity_bindings_user_app_purpose_uq
    ON user_feishu_identity_bindings (user_id, app_id, purpose);

CREATE INDEX IF NOT EXISTS user_feishu_identity_bindings_user_purpose_idx
    ON user_feishu_identity_bindings (user_id, purpose);

INSERT INTO settings (key, value)
VALUES
    ('feishu_notify_enabled', 'false'),
    ('feishu_notify_app_id', ''),
    ('feishu_notify_app_secret', ''),
    ('feishu_notify_token_url', 'https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal'),
    ('feishu_notify_message_url', 'https://open.feishu.cn/open-apis/im/v1/messages'),
    ('feishu_notify_panel_url', '/feishu/panel')
ON CONFLICT (key) DO NOTHING;
