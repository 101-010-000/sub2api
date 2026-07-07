package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type contentModerationRepository struct {
	db *sql.DB
}

func NewContentModerationRepository(db *sql.DB) service.ContentModerationRepository {
	return &contentModerationRepository{db: db}
}

func (r *contentModerationRepository) CreateLog(ctx context.Context, log *service.ContentModerationLog) error {
	if log == nil {
		return nil
	}
	categoryScores, err := json.Marshal(log.CategoryScores)
	if err != nil {
		return fmt.Errorf("marshal moderation category scores: %w", err)
	}
	thresholdSnapshot, err := json.Marshal(log.ThresholdSnapshot)
	if err != nil {
		return fmt.Errorf("marshal moderation thresholds: %w", err)
	}
	keywordHits, err := json.Marshal(log.KeywordHits)
	if err != nil {
		return fmt.Errorf("marshal moderation keyword hits: %w", err)
	}
	auditContext, err := json.Marshal(log.AuditContext)
	if err != nil {
		return fmt.Errorf("marshal moderation audit context: %w", err)
	}
	var userID any
	if log.UserID != nil {
		userID = *log.UserID
	}
	var apiKeyID any
	if log.APIKeyID != nil {
		apiKeyID = *log.APIKeyID
	}
	var groupID any
	if log.GroupID != nil {
		groupID = *log.GroupID
	}
	var latency any
	if log.UpstreamLatencyMS != nil {
		latency = *log.UpstreamLatencyMS
	}
	err = r.db.QueryRowContext(ctx, `
INSERT INTO content_moderation_logs (
    request_id, user_id, user_email, api_key_id, api_key_name, group_id, group_name,
    endpoint, provider, model, mode, action, flagged, highest_category, highest_score,
    category_scores, threshold_snapshot, input_excerpt, keyword_hits, audit_context, upstream_latency_ms, error, matched_keyword,
    violation_count, auto_banned, email_sent, queue_delay_ms, context_id, risk_weight_snapshot,
    effective_sample_rate, effective_ban_threshold, risk_event_source, review_stage
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12, $13, $14, $15,
    $16::jsonb, $17::jsonb, $18, $19::jsonb, $20::jsonb, $21, $22, $23,
    $24, $25, $26, $27, $28, $29, $30, $31, $32, $33
) RETURNING id, created_at`,
		log.RequestID, userID, log.UserEmail, apiKeyID, log.APIKeyName, groupID, log.GroupName,
		log.Endpoint, log.Provider, log.Model, log.Mode, log.Action, log.Flagged, log.HighestCategory, log.HighestScore,
		string(categoryScores), string(thresholdSnapshot), log.InputExcerpt, string(keywordHits), string(auditContext), latency, log.Error,
		log.MatchedKeyword, log.ViolationCount, log.AutoBanned, log.EmailSent, nullableIntPtr(log.QueueDelayMS), nullableInt64Ptr(log.ContextID),
		log.RiskWeightSnapshot, log.EffectiveSampleRate, log.EffectiveBanThreshold, log.RiskEventSource, log.ReviewStage,
	).Scan(&log.ID, &log.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert content moderation log: %w", err)
	}
	return nil
}

func (r *contentModerationRepository) ListLogs(ctx context.Context, filter service.ContentModerationLogFilter) ([]service.ContentModerationLog, *pagination.PaginationResult, error) {
	where, args := buildContentModerationLogWhere(filter)
	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM content_moderation_logs l "+whereSQL, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count content moderation logs: %w", err)
	}

	params := filter.Pagination
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
SELECT
    l.id, l.request_id, l.user_id, l.user_email, l.api_key_id, l.api_key_name, l.group_id, l.group_name,
    l.endpoint, l.provider, l.model, l.mode, l.action, l.flagged, l.highest_category, l.highest_score,
    l.category_scores, l.threshold_snapshot, l.input_excerpt, COALESCE(l.keyword_hits, '[]'::jsonb), l.audit_context, l.upstream_latency_ms, l.error,
    l.violation_count, l.auto_banned, l.email_sent, l.context_id, l.risk_weight_snapshot, l.effective_sample_rate,
    l.effective_ban_threshold, l.risk_event_source, l.review_stage, COALESCE(u.status, ''), l.queue_delay_ms, l.matched_keyword, l.created_at
FROM content_moderation_logs l
LEFT JOIN users u ON u.id = l.user_id `+whereSQL+`
ORDER BY l.created_at DESC, l.id DESC
LIMIT $`+fmt.Sprint(len(queryArgs)-1)+` OFFSET $`+fmt.Sprint(len(queryArgs)),
		queryArgs...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list content moderation logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.ContentModerationLog, 0)
	for rows.Next() {
		var item service.ContentModerationLog
		var userID, apiKeyID, groupID, latency, contextID, queueDelay sql.NullInt64
		var scoresRaw, thresholdsRaw, keywordHitsRaw []byte
		var auditContextRaw sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.RequestID,
			&userID,
			&item.UserEmail,
			&apiKeyID,
			&item.APIKeyName,
			&groupID,
			&item.GroupName,
			&item.Endpoint,
			&item.Provider,
			&item.Model,
			&item.Mode,
			&item.Action,
			&item.Flagged,
			&item.HighestCategory,
			&item.HighestScore,
			&scoresRaw,
			&thresholdsRaw,
			&item.InputExcerpt,
			&keywordHitsRaw,
			&auditContextRaw,
			&latency,
			&item.Error,
			&item.ViolationCount,
			&item.AutoBanned,
			&item.EmailSent,
			&contextID,
			&item.RiskWeightSnapshot,
			&item.EffectiveSampleRate,
			&item.EffectiveBanThreshold,
			&item.RiskEventSource,
			&item.ReviewStage,
			&item.UserStatus,
			&queueDelay,
			&item.MatchedKeyword,
			&item.CreatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan content moderation log: %w", err)
		}
		if userID.Valid {
			v := userID.Int64
			item.UserID = &v
		}
		if apiKeyID.Valid {
			v := apiKeyID.Int64
			item.APIKeyID = &v
		}
		if groupID.Valid {
			v := groupID.Int64
			item.GroupID = &v
		}
		if latency.Valid {
			v := int(latency.Int64)
			item.UpstreamLatencyMS = &v
		}
		if contextID.Valid {
			v := contextID.Int64
			item.ContextID = &v
		}
		if queueDelay.Valid {
			v := int(queueDelay.Int64)
			item.QueueDelayMS = &v
		}
		item.CategoryScores = map[string]float64{}
		_ = json.Unmarshal(scoresRaw, &item.CategoryScores)
		item.ThresholdSnapshot = map[string]float64{}
		_ = json.Unmarshal(thresholdsRaw, &item.ThresholdSnapshot)
		_ = json.Unmarshal(keywordHitsRaw, &item.KeywordHits)
		if auditContextRaw.Valid && strings.TrimSpace(auditContextRaw.String) != "" {
			var auditCtx service.ContentModerationAuditContext
			if err := json.Unmarshal([]byte(auditContextRaw.String), &auditCtx); err == nil {
				item.AuditContext = &auditCtx
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate content moderation logs: %w", err)
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *contentModerationRepository) CountFlaggedByUserSince(ctx context.Context, userID int64, since time.Time, excludeCyberPolicy bool) (int, error) {
	if userID <= 0 {
		return 0, nil
	}
	// SQL 中的 'cyber_policy' 字面量须与 service.ContentModerationActionCyberPolicy 保持一致。
	var count int
	err := r.db.QueryRowContext(ctx, `
WITH last_auto_ban AS (
    SELECT MAX(created_at) AS at
    FROM content_moderation_logs
    WHERE user_id = $1 AND auto_banned = TRUE
)
SELECT COUNT(*)
FROM content_moderation_logs
WHERE user_id = $1
  AND flagged = TRUE
  AND action <> 'hash_block'
  AND ($3::bool IS FALSE OR action <> 'cyber_policy')
  AND created_at >= $2
  AND created_at > COALESCE((SELECT at FROM last_auto_ban), '-infinity'::timestamptz)
`, userID, since, excludeCyberPolicy).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count user content moderation flagged logs: %w", err)
	}
	return count, nil
}

func (r *contentModerationRepository) UpdateLogEmailSent(ctx context.Context, id int64, sent bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE content_moderation_logs SET email_sent = $1 WHERE id = $2`, sent, id)
	if err != nil {
		return fmt.Errorf("update content moderation log email_sent: %w", err)
	}
	return nil
}

func (r *contentModerationRepository) CleanupExpiredLogs(ctx context.Context, hitBefore time.Time, nonHitBefore time.Time, contextBefore time.Time) (*service.ContentModerationCleanupResult, error) {
	result := &service.ContentModerationCleanupResult{FinishedAt: time.Now()}
	if r == nil || r.db == nil {
		return result, nil
	}
	hitExec, err := r.db.ExecContext(ctx, `
DELETE FROM content_moderation_logs
WHERE flagged = TRUE AND created_at < $1
`, hitBefore)
	if err != nil {
		return nil, fmt.Errorf("delete expired hit content moderation logs: %w", err)
	}
	result.DeletedHit, _ = hitExec.RowsAffected()

	nonHitExec, err := r.db.ExecContext(ctx, `
DELETE FROM content_moderation_logs
WHERE flagged = FALSE AND created_at < $1
`, nonHitBefore)
	if err != nil {
		return nil, fmt.Errorf("delete expired non-hit content moderation logs: %w", err)
	}
	result.DeletedNonHit, _ = nonHitExec.RowsAffected()

	if !contextBefore.IsZero() {
		contextExec, err := r.db.ExecContext(ctx, `
DELETE FROM content_moderation_contexts
WHERE created_at < $1
`, contextBefore)
		if err != nil {
			return nil, fmt.Errorf("delete expired content moderation contexts: %w", err)
		}
		result.DeletedContext, _ = contextExec.RowsAffected()
	}

	result.FinishedAt = time.Now()
	return result, nil
}

func (r *contentModerationRepository) UpsertUserBan(ctx context.Context, ban *service.ContentModerationUserBan) error {
	if ban == nil || ban.UserID <= 0 {
		return nil
	}
	triggeredAt := ban.TriggeredAt
	if triggeredAt.IsZero() {
		triggeredAt = time.Now()
	}
	bannedUntil := ban.BannedUntil
	if bannedUntil.IsZero() {
		bannedUntil = triggeredAt.Add(time.Hour)
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO content_moderation_user_bans (user_id, reason, triggered_at, banned_until, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
ON CONFLICT (user_id) DO UPDATE SET
    reason = EXCLUDED.reason,
    triggered_at = EXCLUDED.triggered_at,
    banned_until = EXCLUDED.banned_until,
    updated_at = NOW()
`, ban.UserID, ban.Reason, triggeredAt, bannedUntil)
	if err != nil {
		return fmt.Errorf("upsert content moderation user ban: %w", err)
	}
	return nil
}

func (r *contentModerationRepository) GetActiveUserBan(ctx context.Context, userID int64, now time.Time) (*service.ContentModerationUserBan, error) {
	if userID <= 0 {
		return nil, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	var ban service.ContentModerationUserBan
	err := r.db.QueryRowContext(ctx, `
SELECT user_id, reason, triggered_at, banned_until, created_at, updated_at
FROM content_moderation_user_bans
WHERE user_id = $1 AND banned_until > $2
`, userID, now).Scan(&ban.UserID, &ban.Reason, &ban.TriggeredAt, &ban.BannedUntil, &ban.CreatedAt, &ban.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get active content moderation user ban: %w", err)
	}
	return &ban, nil
}

func (r *contentModerationRepository) ClearUserBan(ctx context.Context, userID int64, now time.Time) error {
	if userID <= 0 {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE content_moderation_user_bans
SET banned_until = $2, updated_at = NOW()
WHERE user_id = $1 AND banned_until > $2
`, userID, now)
	if err != nil {
		return fmt.Errorf("clear content moderation user ban: %w", err)
	}
	return nil
}

func (r *contentModerationRepository) CountSelfUnbanAttempts(ctx context.Context, userID int64, since time.Time) (int, error) {
	if userID <= 0 {
		return 0, nil
	}
	var count int
	err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM content_moderation_self_unban_records
WHERE user_id = $1 AND allowed = TRUE AND created_at >= $2
`, userID, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count content moderation self unban attempts: %w", err)
	}
	return count, nil
}

func (r *contentModerationRepository) CreateSelfUnbanRecord(ctx context.Context, record *service.ContentModerationSelfUnbanRecord) error {
	if record == nil || record.UserID <= 0 {
		return nil
	}
	banTriggeredAt := record.BanTriggeredAt
	if banTriggeredAt.IsZero() {
		banTriggeredAt = time.Now()
	}
	err := r.db.QueryRowContext(ctx, `
INSERT INTO content_moderation_self_unban_records (user_id, ban_triggered_at, attempt_no, allowed, reason, created_at)
VALUES ($1, $2, $3, $4, $5, NOW())
RETURNING id, created_at
`, record.UserID, banTriggeredAt, record.AttemptNo, record.Allowed, record.Reason).Scan(&record.ID, &record.CreatedAt)
	if err != nil {
		return fmt.Errorf("create content moderation self unban record: %w", err)
	}
	return nil
}

func (r *contentModerationRepository) GetUserRiskProfile(ctx context.Context, userID int64) (*service.ContentModerationUserRiskProfile, error) {
	if userID <= 0 {
		return nil, nil
	}
	var profile service.ContentModerationUserRiskProfile
	var lastEventAt, lastDecayAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
SELECT user_id, current_weight, manual_suspicious, cumulative_flagged_count, cumulative_ban_count,
       last_event_at, last_decay_at, created_at, updated_at
FROM content_moderation_user_risk_profiles
WHERE user_id = $1
`, userID).Scan(
		&profile.UserID,
		&profile.CurrentWeight,
		&profile.ManualSuspicious,
		&profile.CumulativeFlaggedCount,
		&profile.CumulativeBanCount,
		&lastEventAt,
		&lastDecayAt,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get content moderation risk profile: %w", err)
	}
	if lastEventAt.Valid {
		t := lastEventAt.Time
		profile.LastEventAt = &t
	}
	if lastDecayAt.Valid {
		t := lastDecayAt.Time
		profile.LastDecayAt = &t
	}
	return &profile, nil
}

func (r *contentModerationRepository) UpsertUserRiskProfile(ctx context.Context, profile *service.ContentModerationUserRiskProfile) error {
	if profile == nil || profile.UserID <= 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO content_moderation_user_risk_profiles (
    user_id, current_weight, manual_suspicious, cumulative_flagged_count, cumulative_ban_count,
    last_event_at, last_decay_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
ON CONFLICT (user_id) DO UPDATE SET
    current_weight = EXCLUDED.current_weight,
    manual_suspicious = EXCLUDED.manual_suspicious,
    cumulative_flagged_count = EXCLUDED.cumulative_flagged_count,
    cumulative_ban_count = EXCLUDED.cumulative_ban_count,
    last_event_at = EXCLUDED.last_event_at,
    last_decay_at = EXCLUDED.last_decay_at,
    updated_at = NOW()
`, profile.UserID, profile.CurrentWeight, profile.ManualSuspicious, profile.CumulativeFlaggedCount, profile.CumulativeBanCount, profile.LastEventAt, profile.LastDecayAt)
	if err != nil {
		return fmt.Errorf("upsert content moderation risk profile: %w", err)
	}
	return nil
}

func (r *contentModerationRepository) CreateUserRiskEvent(ctx context.Context, event *service.ContentModerationUserRiskEvent) error {
	if event == nil || event.UserID <= 0 {
		return nil
	}
	err := r.db.QueryRowContext(ctx, `
INSERT INTO content_moderation_user_risk_events (
    user_id, event_type, source, review_stage, weight_delta, effective_weight_before,
    effective_weight_after, reason, log_id, context_id, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
RETURNING id, created_at
`, event.UserID, event.EventType, event.Source, event.ReviewStage, event.WeightDelta, event.EffectiveWeightBefore, event.EffectiveWeightAfter, event.Reason, nullableInt64Ptr(event.LogID), nullableInt64Ptr(event.ContextID)).Scan(&event.ID, &event.CreatedAt)
	if err != nil {
		return fmt.Errorf("create content moderation risk event: %w", err)
	}
	return nil
}

func (r *contentModerationRepository) ListUserRiskEvents(ctx context.Context, userID int64, limit int) ([]service.ContentModerationUserRiskEvent, error) {
	if userID <= 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, event_type, source, review_stage, weight_delta, effective_weight_before,
       effective_weight_after, reason, log_id, context_id, created_at
FROM content_moderation_user_risk_events
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2
`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list content moderation risk events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.ContentModerationUserRiskEvent, 0)
	for rows.Next() {
		var item service.ContentModerationUserRiskEvent
		var logID, contextID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.UserID, &item.EventType, &item.Source, &item.ReviewStage, &item.WeightDelta, &item.EffectiveWeightBefore, &item.EffectiveWeightAfter, &item.Reason, &logID, &contextID, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan content moderation risk event: %w", err)
		}
		if logID.Valid {
			v := logID.Int64
			item.LogID = &v
		}
		if contextID.Valid {
			v := contextID.Int64
			item.ContextID = &v
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content moderation risk events: %w", err)
	}
	return out, nil
}

func (r *contentModerationRepository) CreateContext(ctx context.Context, item *service.ContentModerationContext) error {
	if item == nil {
		return nil
	}
	var userID, apiKeyID, groupID any
	if item.UserID != nil {
		userID = *item.UserID
	}
	if item.APIKeyID != nil {
		apiKeyID = *item.APIKeyID
	}
	if item.GroupID != nil {
		groupID = *item.GroupID
	}
	err := r.db.QueryRowContext(ctx, `
INSERT INTO content_moderation_contexts (
    request_id, user_id, user_email, api_key_id, api_key_name, group_id, group_name,
    endpoint, provider, model, protocol, input_hash, context_hash, encrypted_context,
    context_summary, context_bytes, status, review_stage, max_review_attempts, next_review_at,
    last_capture_error, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12, $13, $14,
    $15, $16, $17, $18, $19, $20,
    $21, NOW(), NOW()
)
ON CONFLICT (request_id, input_hash, review_stage)
DO UPDATE SET
    user_email = EXCLUDED.user_email,
    api_key_name = EXCLUDED.api_key_name,
    group_name = EXCLUDED.group_name,
    endpoint = EXCLUDED.endpoint,
    provider = EXCLUDED.provider,
    model = EXCLUDED.model,
    protocol = EXCLUDED.protocol,
    context_hash = EXCLUDED.context_hash,
    encrypted_context = EXCLUDED.encrypted_context,
    context_summary = EXCLUDED.context_summary,
    context_bytes = EXCLUDED.context_bytes,
    last_capture_error = EXCLUDED.last_capture_error,
    updated_at = NOW()
RETURNING id, created_at, updated_at
`, item.RequestID, userID, item.UserEmail, apiKeyID, item.APIKeyName, groupID, item.GroupName,
		item.Endpoint, item.Provider, item.Model, item.Protocol, item.InputHash, item.ContextHash, item.EncryptedContext,
		item.ContextSummary, item.ContextBytes, item.Status, item.ReviewStage, item.MaxReviewAttempts, item.NextReviewAt, item.LastCaptureError,
	).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create content moderation context: %w", err)
	}
	return nil
}

func (r *contentModerationRepository) ClaimPendingContexts(ctx context.Context, batchSize int) ([]service.ContentModerationContext, error) {
	if batchSize <= 0 || batchSize > 100 {
		batchSize = 5
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin content moderation context claim: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	rows, queryErr := tx.QueryContext(ctx, `
WITH picked AS (
    SELECT id
    FROM content_moderation_contexts
    WHERE status = 'pending'
      AND next_review_at <= NOW()
      AND review_attempts < max_review_attempts
    ORDER BY next_review_at ASC, id ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE content_moderation_contexts c
SET status = 'processing',
    processing_started_at = NOW(),
    review_attempts = review_attempts + 1,
    updated_at = NOW()
FROM picked
WHERE c.id = picked.id
RETURNING c.id, c.request_id, c.user_id, c.user_email, c.api_key_id, c.api_key_name, c.group_id, c.group_name,
          c.endpoint, c.provider, c.model, c.protocol, c.input_hash, c.context_hash, c.encrypted_context,
          c.context_summary, c.context_bytes, c.status, c.review_stage, c.review_attempts, c.max_review_attempts,
          c.next_review_at, c.processing_started_at, c.reviewed_at, c.last_review_log_id, c.last_review_flagged,
          c.last_review_error, c.last_capture_error, c.created_at, c.updated_at
`, batchSize)
	if queryErr != nil {
		err = queryErr
		return nil, fmt.Errorf("claim content moderation contexts: %w", err)
	}
	items, scanErr := scanContentModerationContexts(rows)
	_ = rows.Close()
	if scanErr != nil {
		err = scanErr
		return nil, scanErr
	}
	if commitErr := tx.Commit(); commitErr != nil {
		err = commitErr
		return nil, fmt.Errorf("commit content moderation context claim: %w", commitErr)
	}
	return items, nil
}

func (r *contentModerationRepository) UpdateContextReview(ctx context.Context, update service.ContentModerationContextReviewUpdate) error {
	if update.ID <= 0 {
		return nil
	}
	status := strings.TrimSpace(update.Status)
	if status == "" {
		status = service.ContentModerationContextStatusReviewed
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE content_moderation_contexts
SET status = $2,
    next_review_at = COALESCE($3, next_review_at),
    reviewed_at = $4,
    last_review_log_id = $5,
    last_review_flagged = $6,
    last_review_error = $7,
    updated_at = NOW()
WHERE id = $1
`, update.ID, status, update.NextReviewAt, update.ReviewedAt, nullableInt64Ptr(update.LastReviewLogID), update.LastReviewFlagged, update.LastReviewError)
	if err != nil {
		return fmt.Errorf("update content moderation context review: %w", err)
	}
	return nil
}

func (r *contentModerationRepository) CountContextsByStatus(ctx context.Context) (*service.ContentModerationContextStatusCounts, error) {
	counts := &service.ContentModerationContextStatusCounts{}
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)::BIGINT, COALESCE(SUM(context_bytes), 0)::BIGINT, COALESCE(AVG(context_bytes), 0)::BIGINT
FROM content_moderation_contexts
`).Scan(&counts.Total, &counts.TotalBytes, &counts.AvgBytes); err != nil {
		return nil, fmt.Errorf("summarize content moderation contexts: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT status, COUNT(*), MAX(reviewed_at)
FROM content_moderation_contexts
WHERE status IN ('pending', 'processing', 'failed', 'reviewed')
GROUP BY status
`)
	if err != nil {
		return nil, fmt.Errorf("count content moderation contexts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var status string
		var count int64
		var lastReviewed sql.NullTime
		if err := rows.Scan(&status, &count, &lastReviewed); err != nil {
			return nil, fmt.Errorf("scan content moderation context counts: %w", err)
		}
		switch status {
		case service.ContentModerationContextStatusPending:
			counts.Pending = count
		case service.ContentModerationContextStatusProcessing:
			counts.Processing = count
		case service.ContentModerationContextStatusFailed:
			counts.Failed = count
		}
		if lastReviewed.Valid {
			t := lastReviewed.Time
			if counts.LastReviewedAt == nil || t.After(*counts.LastReviewedAt) {
				counts.LastReviewedAt = &t
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content moderation context counts: %w", err)
	}
	return counts, nil
}

func (r *contentModerationRepository) ListUserContexts(ctx context.Context, userID int64, limit int) ([]service.ContentModerationContext, error) {
	if userID <= 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, request_id, user_id, user_email, api_key_id, api_key_name, group_id, group_name,
       endpoint, provider, model, protocol, input_hash, context_hash, encrypted_context,
       context_summary, context_bytes, status, review_stage, review_attempts, max_review_attempts,
       next_review_at, processing_started_at, reviewed_at, last_review_log_id, last_review_flagged,
       last_review_error, last_capture_error, created_at, updated_at
FROM content_moderation_contexts
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2
`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list content moderation user contexts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanContentModerationContexts(rows)
}

func (r *contentModerationRepository) GetContextByID(ctx context.Context, contextID int64) (*service.ContentModerationContext, error) {
	if contextID <= 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, request_id, user_id, user_email, api_key_id, api_key_name, group_id, group_name,
       endpoint, provider, model, protocol, input_hash, context_hash, encrypted_context,
       context_summary, context_bytes, status, review_stage, review_attempts, max_review_attempts,
       next_review_at, processing_started_at, reviewed_at, last_review_log_id, last_review_flagged,
       last_review_error, last_capture_error, created_at, updated_at
FROM content_moderation_contexts
WHERE id = $1
`, contextID)
	if err != nil {
		return nil, fmt.Errorf("get content moderation context: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items, err := scanContentModerationContexts(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (r *contentModerationRepository) CreateContextAccessLog(ctx context.Context, contextID int64, adminUserID int64, action string) error {
	if contextID <= 0 {
		return nil
	}
	var admin any
	if adminUserID > 0 {
		admin = adminUserID
	}
	action = strings.TrimSpace(action)
	if action == "" {
		action = "view"
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO content_moderation_context_access_logs (context_id, admin_user_id, action, created_at)
VALUES ($1, $2, $3, NOW())
`, contextID, admin, action)
	if err != nil {
		return fmt.Errorf("create content moderation context access log: %w", err)
	}
	return nil
}

func scanContentModerationContexts(rows *sql.Rows) ([]service.ContentModerationContext, error) {
	items := make([]service.ContentModerationContext, 0)
	for rows.Next() {
		var item service.ContentModerationContext
		var userID, apiKeyID, groupID, lastLogID sql.NullInt64
		var processingStartedAt, reviewedAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.RequestID,
			&userID,
			&item.UserEmail,
			&apiKeyID,
			&item.APIKeyName,
			&groupID,
			&item.GroupName,
			&item.Endpoint,
			&item.Provider,
			&item.Model,
			&item.Protocol,
			&item.InputHash,
			&item.ContextHash,
			&item.EncryptedContext,
			&item.ContextSummary,
			&item.ContextBytes,
			&item.Status,
			&item.ReviewStage,
			&item.ReviewAttempts,
			&item.MaxReviewAttempts,
			&item.NextReviewAt,
			&processingStartedAt,
			&reviewedAt,
			&lastLogID,
			&item.LastReviewFlagged,
			&item.LastReviewError,
			&item.LastCaptureError,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan content moderation context: %w", err)
		}
		if userID.Valid {
			v := userID.Int64
			item.UserID = &v
		}
		if apiKeyID.Valid {
			v := apiKeyID.Int64
			item.APIKeyID = &v
		}
		if groupID.Valid {
			v := groupID.Int64
			item.GroupID = &v
		}
		if processingStartedAt.Valid {
			t := processingStartedAt.Time
			item.ProcessingStartedAt = &t
		}
		if reviewedAt.Valid {
			t := reviewedAt.Time
			item.ReviewedAt = &t
		}
		if lastLogID.Valid {
			v := lastLogID.Int64
			item.LastReviewLogID = &v
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content moderation contexts: %w", err)
	}
	return items, nil
}

func nullableIntPtr(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt64Ptr(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func buildContentModerationLogWhere(filter service.ContentModerationLogFilter) ([]string, []any) {
	where := []string{"l.id IS NOT NULL"}
	args := make([]any, 0)
	add := func(expr string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(expr, len(args)))
	}
	switch strings.ToLower(strings.TrimSpace(filter.Result)) {
	case "hit", "flagged":
		where = append(where, "l.flagged = TRUE")
	case "blocked", "block":
		where = append(where, "l.action IN ('block', 'keyword_block', 'hash_block')")
	case "pass", "allow":
		where = append(where, "l.flagged = FALSE AND l.error = ''")
	case "error":
		where = append(where, "l.error <> ''")
	}
	if filter.GroupID != nil {
		add("l.group_id = $%d", *filter.GroupID)
	}
	if endpoint := strings.TrimSpace(filter.Endpoint); endpoint != "" {
		add("l.endpoint = $%d", endpoint)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + search + "%"
		args = append(args, like, like, like, like, like)
		idx := len(args) - 4
		where = append(where, fmt.Sprintf("(l.request_id ILIKE $%d OR l.user_email ILIKE $%d OR l.api_key_name ILIKE $%d OR l.model ILIKE $%d OR l.input_excerpt ILIKE $%d)", idx, idx+1, idx+2, idx+3, idx+4))
	}
	if filter.From != nil && !filter.From.IsZero() {
		add("l.created_at >= $%d", *filter.From)
	}
	if filter.To != nil && !filter.To.IsZero() {
		add("l.created_at <= $%d", *filter.To)
	}
	return where, args
}

func (r *contentModerationRepository) CreateRequestRiskEvent(ctx context.Context, event *service.RequestRiskEvent) error {
	if event == nil {
		return nil
	}
	matchedRules, err := json.Marshal(event.MatchedRules)
	if err != nil {
		return fmt.Errorf("marshal request risk matched rules: %w", err)
	}
	languageSignals, err := json.Marshal(event.LanguageSignals)
	if err != nil {
		return fmt.Errorf("marshal request risk language signals: %w", err)
	}
	rawHeadersJSON := strings.TrimSpace(event.RawHeadersJSON)
	if rawHeadersJSON == "" && event.RawHeaders != nil {
		raw, err := json.Marshal(event.RawHeaders)
		if err != nil {
			return fmt.Errorf("marshal request risk raw headers: %w", err)
		}
		rawHeadersJSON = string(raw)
	}
	err = r.db.QueryRowContext(ctx, `
INSERT INTO request_risk_events (
    created_at, expires_at, user_id, api_key_id, account_id, request_id,
    session_id, session_id_hash, user_agent, user_agent_hash,
    inference_geo, timezone, platform, language_signals_json, chinese_intensity,
    matched_rules_json, action, reason_code, raw_headers_json, x_foo_raw,
    request_path, model
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14, $15,
    $16, $17, $18, $19, $20,
    $21, $22
) RETURNING id, created_at`,
		event.CreatedAt, event.ExpiresAt, nullableInt64Ptr(event.UserID), nullableInt64Ptr(event.APIKeyID), nullableInt64Ptr(event.AccountID), event.RequestID,
		event.SessionID, event.SessionIDHash, event.UserAgent, event.UserAgentHash,
		event.InferenceGeo, event.Timezone, event.Platform, string(languageSignals), event.ChineseIntensity,
		string(matchedRules), event.Action, event.ReasonCode, rawHeadersJSON, event.XFooRaw,
		event.RequestPath, event.Model,
	).Scan(&event.ID, &event.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert request risk event: %w", err)
	}
	event.RawHeadersJSON = rawHeadersJSON
	return nil
}

func (r *contentModerationRepository) ListRequestRiskEvents(ctx context.Context, filter service.RequestRiskEventFilter) ([]service.RequestRiskEvent, *pagination.PaginationResult, error) {
	where, args := buildRequestRiskEventWhere(filter)
	whereSQL := "WHERE " + strings.Join(where, " AND ")
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM request_risk_events "+whereSQL, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count request risk events: %w", err)
	}
	params := filter.PaginationParams
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
SELECT id, created_at, expires_at, user_id, api_key_id, account_id, request_id,
       session_id, session_id_hash, user_agent, user_agent_hash,
       inference_geo, timezone, platform, language_signals_json, chinese_intensity,
       matched_rules_json, action, reason_code, raw_headers_json, x_foo_raw,
       request_path, model
FROM request_risk_events `+whereSQL+`
ORDER BY created_at DESC, id DESC
LIMIT $`+fmt.Sprint(len(queryArgs)-1)+` OFFSET $`+fmt.Sprint(len(queryArgs)),
		queryArgs...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list request risk events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.RequestRiskEvent, 0)
	for rows.Next() {
		item, err := scanRequestRiskEvent(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate request risk events: %w", err)
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *contentModerationRepository) GetRequestRiskEvent(ctx context.Context, id int64) (*service.RequestRiskEvent, error) {
	if id <= 0 {
		return nil, sql.ErrNoRows
	}
	row := r.db.QueryRowContext(ctx, `
SELECT id, created_at, expires_at, user_id, api_key_id, account_id, request_id,
       session_id, session_id_hash, user_agent, user_agent_hash,
       inference_geo, timezone, platform, language_signals_json, chinese_intensity,
       matched_rules_json, action, reason_code, raw_headers_json, x_foo_raw,
       request_path, model
FROM request_risk_events
WHERE id = $1`, id)
	item, err := scanRequestRiskEvent(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infraerrors.NotFound("REQUEST_RISK_EVENT_NOT_FOUND", "request risk event not found")
		}
		return nil, err
	}
	return &item, nil
}

func (r *contentModerationRepository) CleanupExpiredRequestRiskEvents(ctx context.Context, now time.Time) (int64, error) {
	if now.IsZero() {
		now = time.Now()
	}
	events, err := r.db.ExecContext(ctx, `DELETE FROM request_risk_events WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired request risk events: %w", err)
	}
	_, _ = r.db.ExecContext(ctx, `DELETE FROM request_risk_ua_bans WHERE banned_until <= $1`, now)
	deleted, _ := events.RowsAffected()
	return deleted, nil
}

func (r *contentModerationRepository) UpsertRequestRiskUABan(ctx context.Context, ban *service.RequestRiskUserAgentBan) error {
	if ban == nil || ban.APIKeyID <= 0 || strings.TrimSpace(ban.UserAgentHash) == "" {
		return nil
	}
	triggeredAt := ban.TriggeredAt
	if triggeredAt.IsZero() {
		triggeredAt = time.Now()
	}
	bannedUntil := ban.BannedUntil
	if bannedUntil.IsZero() {
		bannedUntil = triggeredAt.Add(time.Hour)
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO request_risk_ua_bans (
    api_key_id, user_id, user_agent_hash, user_agent, reason,
    triggered_at, banned_until, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, NOW(), NOW()
)
ON CONFLICT (api_key_id, user_agent_hash) DO UPDATE SET
    user_id = EXCLUDED.user_id,
    user_agent = EXCLUDED.user_agent,
    reason = EXCLUDED.reason,
    triggered_at = EXCLUDED.triggered_at,
    banned_until = EXCLUDED.banned_until,
    updated_at = NOW()`,
		ban.APIKeyID, ban.UserID, ban.UserAgentHash, ban.UserAgent, ban.Reason,
		triggeredAt, bannedUntil,
	)
	if err != nil {
		return fmt.Errorf("upsert request risk ua ban: %w", err)
	}
	return nil
}

func (r *contentModerationRepository) GetActiveRequestRiskUABan(ctx context.Context, apiKeyID int64, userAgentHash string, now time.Time) (*service.RequestRiskUserAgentBan, error) {
	if apiKeyID <= 0 || strings.TrimSpace(userAgentHash) == "" {
		return nil, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	var ban service.RequestRiskUserAgentBan
	err := r.db.QueryRowContext(ctx, `
SELECT api_key_id, user_id, user_agent_hash, user_agent, reason, triggered_at, banned_until, created_at, updated_at
FROM request_risk_ua_bans
WHERE api_key_id = $1 AND user_agent_hash = $2 AND banned_until > $3`,
		apiKeyID, userAgentHash, now,
	).Scan(&ban.APIKeyID, &ban.UserID, &ban.UserAgentHash, &ban.UserAgent, &ban.Reason, &ban.TriggeredAt, &ban.BannedUntil, &ban.CreatedAt, &ban.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active request risk ua ban: %w", err)
	}
	return &ban, nil
}

type requestRiskEventScanner interface {
	Scan(dest ...any) error
}

func scanRequestRiskEvent(scanner requestRiskEventScanner) (service.RequestRiskEvent, error) {
	var item service.RequestRiskEvent
	var userID, apiKeyID, accountID sql.NullInt64
	var languageRaw, matchedRulesRaw, rawHeadersRaw sql.NullString
	if err := scanner.Scan(
		&item.ID,
		&item.CreatedAt,
		&item.ExpiresAt,
		&userID,
		&apiKeyID,
		&accountID,
		&item.RequestID,
		&item.SessionID,
		&item.SessionIDHash,
		&item.UserAgent,
		&item.UserAgentHash,
		&item.InferenceGeo,
		&item.Timezone,
		&item.Platform,
		&languageRaw,
		&item.ChineseIntensity,
		&matchedRulesRaw,
		&item.Action,
		&item.ReasonCode,
		&rawHeadersRaw,
		&item.XFooRaw,
		&item.RequestPath,
		&item.Model,
	); err != nil {
		return item, fmt.Errorf("scan request risk event: %w", err)
	}
	if userID.Valid {
		v := userID.Int64
		item.UserID = &v
	}
	if apiKeyID.Valid {
		v := apiKeyID.Int64
		item.APIKeyID = &v
	}
	if accountID.Valid {
		v := accountID.Int64
		item.AccountID = &v
	}
	item.LanguageSignals = map[string]any{}
	if languageRaw.Valid && strings.TrimSpace(languageRaw.String) != "" {
		_ = json.Unmarshal([]byte(languageRaw.String), &item.LanguageSignals)
	}
	item.MatchedRules = []string{}
	if matchedRulesRaw.Valid && strings.TrimSpace(matchedRulesRaw.String) != "" {
		_ = json.Unmarshal([]byte(matchedRulesRaw.String), &item.MatchedRules)
	}
	if rawHeadersRaw.Valid {
		item.RawHeadersJSON = rawHeadersRaw.String
		item.RawHeaders = map[string][]string{}
		_ = json.Unmarshal([]byte(rawHeadersRaw.String), &item.RawHeaders)
	}
	return item, nil
}

func buildRequestRiskEventWhere(filter service.RequestRiskEventFilter) ([]string, []any) {
	where := []string{"id IS NOT NULL"}
	args := make([]any, 0)
	add := func(expr string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(expr, len(args)))
	}
	if action := strings.TrimSpace(filter.Action); action != "" {
		add("action = $%d", action)
	}
	if rule := strings.TrimSpace(filter.Rule); rule != "" {
		add("matched_rules_json ILIKE $%d", "%"+rule+"%")
	}
	if filter.APIKeyID != nil {
		add("api_key_id = $%d", *filter.APIKeyID)
	}
	if filter.UserID != nil {
		add("user_id = $%d", *filter.UserID)
	}
	if filter.From != nil && !filter.From.IsZero() {
		add("created_at >= $%d", *filter.From)
	}
	if filter.To != nil && !filter.To.IsZero() {
		add("created_at <= $%d", *filter.To)
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		like := "%" + q + "%"
		args = append(args, like, like, like, like, like, like)
		idx := len(args) - 5
		where = append(where, fmt.Sprintf("(request_id ILIKE $%d OR session_id ILIKE $%d OR user_agent ILIKE $%d OR inference_geo ILIKE $%d OR timezone ILIKE $%d OR model ILIKE $%d)", idx, idx+1, idx+2, idx+3, idx+4, idx+5))
	}
	return where, args
}
