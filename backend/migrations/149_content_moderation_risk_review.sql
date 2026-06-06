-- 风控中心：用户风险画像、加密上下文、后台复检与访问审计

CREATE TABLE IF NOT EXISTS content_moderation_contexts (
    id                    BIGSERIAL PRIMARY KEY,
    request_id            VARCHAR(128) NOT NULL DEFAULT '',
    user_id               BIGINT REFERENCES users(id) ON DELETE SET NULL,
    user_email            VARCHAR(255) NOT NULL DEFAULT '',
    api_key_id            BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
    api_key_name          VARCHAR(100) NOT NULL DEFAULT '',
    group_id              BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    group_name            VARCHAR(255) NOT NULL DEFAULT '',
    endpoint              VARCHAR(128) NOT NULL DEFAULT '',
    provider              VARCHAR(64) NOT NULL DEFAULT '',
    model                 VARCHAR(255) NOT NULL DEFAULT '',
    protocol              VARCHAR(64) NOT NULL DEFAULT '',
    input_hash            VARCHAR(64) NOT NULL DEFAULT '',
    context_hash          VARCHAR(64) NOT NULL DEFAULT '',
    encrypted_context     TEXT NOT NULL DEFAULT '',
    context_summary       TEXT NOT NULL DEFAULT '',
    context_bytes         INT NOT NULL DEFAULT 0,
    status                VARCHAR(32) NOT NULL DEFAULT 'pending',
    review_stage          VARCHAR(32) NOT NULL DEFAULT 'background_review',
    review_attempts       INT NOT NULL DEFAULT 0,
    max_review_attempts   INT NOT NULL DEFAULT 3,
    next_review_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processing_started_at TIMESTAMPTZ,
    reviewed_at           TIMESTAMPTZ,
    last_review_log_id    BIGINT REFERENCES content_moderation_logs(id) ON DELETE SET NULL,
    last_review_flagged   BOOLEAN NOT NULL DEFAULT FALSE,
    last_review_error     TEXT NOT NULL DEFAULT '',
    last_capture_error    TEXT NOT NULL DEFAULT '',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_content_moderation_contexts_request_hash_stage
    ON content_moderation_contexts(request_id, input_hash, review_stage);
CREATE INDEX IF NOT EXISTS idx_content_moderation_contexts_status_next
    ON content_moderation_contexts(status, next_review_at, id);
CREATE INDEX IF NOT EXISTS idx_content_moderation_contexts_user_created
    ON content_moderation_contexts(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_moderation_contexts_created
    ON content_moderation_contexts(created_at DESC);

CREATE TABLE IF NOT EXISTS content_moderation_user_risk_profiles (
    user_id                  BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current_weight           DECIMAL(12, 4) NOT NULL DEFAULT 0,
    manual_suspicious        BOOLEAN NOT NULL DEFAULT FALSE,
    cumulative_flagged_count INT NOT NULL DEFAULT 0,
    cumulative_ban_count     INT NOT NULL DEFAULT 0,
    last_event_at            TIMESTAMPTZ,
    last_decay_at            TIMESTAMPTZ,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_content_moderation_risk_profiles_weight
    ON content_moderation_user_risk_profiles(current_weight DESC);
CREATE INDEX IF NOT EXISTS idx_content_moderation_risk_profiles_manual
    ON content_moderation_user_risk_profiles(manual_suspicious, updated_at DESC);

CREATE TABLE IF NOT EXISTS content_moderation_user_risk_events (
    id                      BIGSERIAL PRIMARY KEY,
    user_id                 BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type              VARCHAR(32) NOT NULL,
    source                  VARCHAR(32) NOT NULL DEFAULT '',
    review_stage            VARCHAR(32) NOT NULL DEFAULT '',
    weight_delta            DECIMAL(12, 4) NOT NULL DEFAULT 0,
    effective_weight_before DECIMAL(12, 4) NOT NULL DEFAULT 0,
    effective_weight_after  DECIMAL(12, 4) NOT NULL DEFAULT 0,
    reason                  TEXT NOT NULL DEFAULT '',
    log_id                  BIGINT REFERENCES content_moderation_logs(id) ON DELETE SET NULL,
    context_id              BIGINT REFERENCES content_moderation_contexts(id) ON DELETE SET NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_content_moderation_risk_events_user_created
    ON content_moderation_user_risk_events(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_moderation_risk_events_type_created
    ON content_moderation_user_risk_events(event_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_moderation_risk_events_context
    ON content_moderation_user_risk_events(context_id);

CREATE TABLE IF NOT EXISTS content_moderation_context_access_logs (
    id            BIGSERIAL PRIMARY KEY,
    context_id    BIGINT NOT NULL REFERENCES content_moderation_contexts(id) ON DELETE CASCADE,
    admin_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action        VARCHAR(32) NOT NULL DEFAULT 'view',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_content_moderation_context_access_context_created
    ON content_moderation_context_access_logs(context_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_moderation_context_access_admin_created
    ON content_moderation_context_access_logs(admin_user_id, created_at DESC);

ALTER TABLE content_moderation_logs
    ADD COLUMN IF NOT EXISTS context_id BIGINT REFERENCES content_moderation_contexts(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS risk_weight_snapshot DECIMAL(12, 4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS effective_sample_rate INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS effective_ban_threshold INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS risk_event_source VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS review_stage VARCHAR(32) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_content_moderation_logs_context_id
    ON content_moderation_logs(context_id);
CREATE INDEX IF NOT EXISTS idx_content_moderation_logs_review_stage_created
    ON content_moderation_logs(review_stage, created_at DESC);
