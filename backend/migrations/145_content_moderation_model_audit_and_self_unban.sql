-- 风控中心：模型审计上下文、用户级限时封禁、自助解封

ALTER TABLE content_moderation_logs
    ADD COLUMN IF NOT EXISTS keyword_hits JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS audit_context JSONB;

CREATE TABLE IF NOT EXISTS content_moderation_user_bans (
    user_id      BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    reason       TEXT NOT NULL DEFAULT '',
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    banned_until TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_content_moderation_user_bans_until
    ON content_moderation_user_bans(banned_until DESC);

CREATE TABLE IF NOT EXISTS content_moderation_self_unban_records (
    id               BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ban_triggered_at  TIMESTAMPTZ NOT NULL,
    attempt_no        INT NOT NULL,
    allowed           BOOLEAN NOT NULL DEFAULT TRUE,
    reason            TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_content_moderation_self_unban_user_created
    ON content_moderation_self_unban_records(user_id, created_at DESC);
