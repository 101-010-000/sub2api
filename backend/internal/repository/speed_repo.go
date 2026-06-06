package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type speedRepository struct {
	db *sql.DB
}

func NewSpeedRepository(db *sql.DB) service.SpeedRepository {
	return &speedRepository{db: db}
}

func (r *speedRepository) GetUserGroupConfig(ctx context.Context, userID, groupID int64) (*service.UserGroupSpeedConfig, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			user_id, group_id, fast_quota_ratio, slow_delay_min_seconds, slow_delay_max_seconds, slow_reject_rate,
			daily_window_start, weekly_window_start, monthly_window_start,
			daily_usage_usd, weekly_usage_usd, monthly_usage_usd,
			slow_request_count, slow_reject_count, last_slow_at
		FROM user_group_speed_configs
		WHERE user_id = $1 AND group_id = $2
	`, userID, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	cfg, err := scanSpeedConfig(rows)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		return nil, errors.New("multiple user group speed configs found")
	}
	return cfg, rows.Err()
}

func (r *speedRepository) UpsertUserGroupConfig(ctx context.Context, cfg *service.UserGroupSpeedConfig) error {
	if r == nil || r.db == nil || cfg == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_group_speed_configs (
			user_id, group_id, fast_quota_ratio, slow_delay_min_seconds, slow_delay_max_seconds, slow_reject_rate
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, group_id) DO UPDATE SET
			fast_quota_ratio = EXCLUDED.fast_quota_ratio,
			slow_delay_min_seconds = EXCLUDED.slow_delay_min_seconds,
			slow_delay_max_seconds = EXCLUDED.slow_delay_max_seconds,
			slow_reject_rate = EXCLUDED.slow_reject_rate,
			updated_at = NOW()
	`, cfg.UserID, cfg.GroupID, speedNullFloat64(cfg.FastQuotaRatio), nullInt(cfg.SlowDelayMinSeconds), nullInt(cfg.SlowDelayMaxSeconds), speedNullFloat64(cfg.SlowRejectRate))
	return err
}

func (r *speedRepository) ClearUserGroupConfig(ctx context.Context, userID, groupID int64) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_group_speed_configs SET
			fast_quota_ratio = NULL,
			slow_delay_min_seconds = NULL,
			slow_delay_max_seconds = NULL,
			slow_reject_rate = NULL,
			updated_at = NOW()
		WHERE user_id = $1 AND group_id = $2
	`, userID, groupID)
	return err
}

func (r *speedRepository) ResetUserGroupUsage(ctx context.Context, userID, groupID int64, now time.Time) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_group_speed_configs (
			user_id, group_id, daily_window_start, weekly_window_start, monthly_window_start,
			daily_usage_usd, weekly_usage_usd, monthly_usage_usd,
			slow_request_count, slow_reject_count, last_slow_at
		) VALUES ($1, $2, $3, $3, $3, 0, 0, 0, 0, 0, NULL)
		ON CONFLICT (user_id, group_id) DO UPDATE SET
			daily_window_start = EXCLUDED.daily_window_start,
			weekly_window_start = EXCLUDED.weekly_window_start,
			monthly_window_start = EXCLUDED.monthly_window_start,
			daily_usage_usd = 0,
			weekly_usage_usd = 0,
			monthly_usage_usd = 0,
			slow_request_count = 0,
			slow_reject_count = 0,
			last_slow_at = NULL,
			updated_at = NOW()
	`, userID, groupID, now)
	return err
}

func (r *speedRepository) IncrementUserGroupUsage(ctx context.Context, userID, groupID int64, costUSD float64, now time.Time) error {
	if r == nil || r.db == nil || costUSD <= 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_group_speed_configs (
			user_id, group_id, daily_window_start, weekly_window_start, monthly_window_start,
			daily_usage_usd, weekly_usage_usd, monthly_usage_usd
		) VALUES ($1, $2, $4, $4, $4, $3, $3, $3)
		ON CONFLICT (user_id, group_id) DO UPDATE SET
			daily_window_start = CASE
				WHEN user_group_speed_configs.daily_window_start IS NULL OR user_group_speed_configs.daily_window_start <= $4 - INTERVAL '24 hours' THEN $4
				ELSE user_group_speed_configs.daily_window_start
			END,
			weekly_window_start = CASE
				WHEN user_group_speed_configs.weekly_window_start IS NULL OR user_group_speed_configs.weekly_window_start <= $4 - INTERVAL '7 days' THEN $4
				ELSE user_group_speed_configs.weekly_window_start
			END,
			monthly_window_start = CASE
				WHEN user_group_speed_configs.monthly_window_start IS NULL OR user_group_speed_configs.monthly_window_start <= $4 - INTERVAL '30 days' THEN $4
				ELSE user_group_speed_configs.monthly_window_start
			END,
			daily_usage_usd = CASE
				WHEN user_group_speed_configs.daily_window_start IS NULL OR user_group_speed_configs.daily_window_start <= $4 - INTERVAL '24 hours' THEN $3
				ELSE user_group_speed_configs.daily_usage_usd + $3
			END,
			weekly_usage_usd = CASE
				WHEN user_group_speed_configs.weekly_window_start IS NULL OR user_group_speed_configs.weekly_window_start <= $4 - INTERVAL '7 days' THEN $3
				ELSE user_group_speed_configs.weekly_usage_usd + $3
			END,
			monthly_usage_usd = CASE
				WHEN user_group_speed_configs.monthly_window_start IS NULL OR user_group_speed_configs.monthly_window_start <= $4 - INTERVAL '30 days' THEN $3
				ELSE user_group_speed_configs.monthly_usage_usd + $3
			END,
			updated_at = NOW()
	`, userID, groupID, costUSD, now)
	return err
}

func (r *speedRepository) RecordSlowDecision(ctx context.Context, userID, groupID int64, rejected bool, now time.Time) error {
	if r == nil || r.db == nil {
		return nil
	}
	rejectInc := int64(0)
	if rejected {
		rejectInc = 1
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_group_speed_configs (
			user_id, group_id, slow_request_count, slow_reject_count, last_slow_at
		) VALUES ($1, $2, 1, $3, $4)
		ON CONFLICT (user_id, group_id) DO UPDATE SET
			slow_request_count = user_group_speed_configs.slow_request_count + 1,
			slow_reject_count = user_group_speed_configs.slow_reject_count + $3,
			last_slow_at = $4,
			updated_at = NOW()
	`, userID, groupID, rejectInc, now)
	return err
}

func (r *speedRepository) ListUserGroupSpeedStates(ctx context.Context, userID int64, visibleOnly bool) ([]service.UserGroupSpeedState, error) {
	if r == nil || r.db == nil {
		return []service.UserGroupSpeedState{}, nil
	}
	filter := "g.speed_config_enabled = TRUE AND g.subscription_type = 'subscription' AND us.id IS NOT NULL"
	if visibleOnly {
		filter += " AND g.user_speed_config_allowed = TRUE"
	}
	rows, err := r.db.QueryContext(ctx, speedStateSelectSQL+filter+` ORDER BY g.sort_order ASC, g.id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.UserGroupSpeedState{}
	for rows.Next() {
		state, err := scanSpeedState(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *state)
	}
	return out, rows.Err()
}

func (r *speedRepository) GetUserGroupSpeedState(ctx context.Context, userID, groupID int64) (*service.UserGroupSpeedState, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrSpeedConfigForbidden
	}
	rows, err := r.db.QueryContext(ctx, speedStateSelectSQL+`g.id = $2`, userID, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrGroupNotFound
	}
	state, err := scanSpeedState(rows)
	if err != nil {
		return nil, err
	}
	return state, rows.Err()
}

const speedStateSelectSQL = `
	SELECT
		g.id, g.name, g.subscription_type,
		g.daily_limit_usd, g.weekly_limit_usd, g.monthly_limit_usd,
		g.speed_config_enabled, g.user_speed_config_allowed,
		g.default_fast_quota_ratio, g.min_fast_quota_ratio, g.max_fast_quota_ratio,
		g.default_slow_delay_min_seconds, g.default_slow_delay_max_seconds, g.max_slow_delay_seconds,
		g.default_slow_reject_rate, g.max_slow_reject_rate,
		ugsc.user_id, ugsc.group_id, ugsc.fast_quota_ratio, ugsc.slow_delay_min_seconds, ugsc.slow_delay_max_seconds, ugsc.slow_reject_rate,
		ugsc.daily_window_start, ugsc.weekly_window_start, ugsc.monthly_window_start,
		COALESCE(ugsc.daily_usage_usd, 0), COALESCE(ugsc.weekly_usage_usd, 0), COALESCE(ugsc.monthly_usage_usd, 0),
		COALESCE(ugsc.slow_request_count, 0), COALESCE(ugsc.slow_reject_count, 0), ugsc.last_slow_at,
		us.id, us.user_id, us.group_id, us.daily_window_start, us.weekly_window_start, us.monthly_window_start,
		COALESCE(us.daily_usage_usd, 0), COALESCE(us.weekly_usage_usd, 0), COALESCE(us.monthly_usage_usd, 0), us.expires_at, us.status
	FROM groups g
	LEFT JOIN user_group_speed_configs ugsc ON ugsc.group_id = g.id AND ugsc.user_id = $1
	LEFT JOIN user_subscriptions us ON us.group_id = g.id
		AND us.user_id = $1
		AND us.status = 'active'
		AND us.expires_at > NOW()
		AND us.deleted_at IS NULL
	WHERE g.deleted_at IS NULL AND `

type speedConfigScanner interface {
	Scan(dest ...any) error
}

func scanSpeedConfig(rows speedConfigScanner) (*service.UserGroupSpeedConfig, error) {
	var fast, reject sql.NullFloat64
	var minDelay, maxDelay sql.NullInt64
	var dailyStart, weeklyStart, monthlyStart, lastSlowAt sql.NullTime
	cfg := &service.UserGroupSpeedConfig{}
	if err := rows.Scan(
		&cfg.UserID, &cfg.GroupID, &fast, &minDelay, &maxDelay, &reject,
		&dailyStart, &weeklyStart, &monthlyStart,
		&cfg.DailyUsageUSD, &cfg.WeeklyUsageUSD, &cfg.MonthlyUsageUSD,
		&cfg.SlowRequestCount, &cfg.SlowRejectCount, &lastSlowAt,
	); err != nil {
		return nil, err
	}
	cfg.FastQuotaRatio = float64PtrFromNull(fast)
	cfg.SlowDelayMinSeconds = intPtrFromNull(minDelay)
	cfg.SlowDelayMaxSeconds = intPtrFromNull(maxDelay)
	cfg.SlowRejectRate = float64PtrFromNull(reject)
	cfg.DailyWindowStart = timePtrFromNull(dailyStart)
	cfg.WeeklyWindowStart = timePtrFromNull(weeklyStart)
	cfg.MonthlyWindowStart = timePtrFromNull(monthlyStart)
	cfg.LastSlowAt = timePtrFromNull(lastSlowAt)
	return cfg, nil
}

func scanSpeedState(rows speedConfigScanner) (*service.UserGroupSpeedState, error) {
	var dailyLimit, weeklyLimit, monthlyLimit sql.NullFloat64
	var cfgUserID, cfgGroupID sql.NullInt64
	var fast, reject sql.NullFloat64
	var minDelay, maxDelay sql.NullInt64
	var cfgDailyStart, cfgWeeklyStart, cfgMonthlyStart, lastSlowAt sql.NullTime
	var subID, subUserID, subGroupID sql.NullInt64
	var subDailyStart, subWeeklyStart, subMonthlyStart sql.NullTime
	var subExpiresAt sql.NullTime
	var subStatus sql.NullString

	state := &service.UserGroupSpeedState{Group: &service.Group{Hydrated: true}}
	cfg := &service.UserGroupSpeedConfig{}
	sub := &service.UserSubscription{}
	if err := rows.Scan(
		&state.Group.ID, &state.Group.Name, &state.Group.SubscriptionType,
		&dailyLimit, &weeklyLimit, &monthlyLimit,
		&state.Group.SpeedConfigEnabled, &state.Group.UserSpeedConfigAllowed,
		&state.Group.DefaultFastQuotaRatio, &state.Group.MinFastQuotaRatio, &state.Group.MaxFastQuotaRatio,
		&state.Group.DefaultSlowDelayMinSeconds, &state.Group.DefaultSlowDelayMaxSeconds, &state.Group.MaxSlowDelaySeconds,
		&state.Group.DefaultSlowRejectRate, &state.Group.MaxSlowRejectRate,
		&cfgUserID, &cfgGroupID, &fast, &minDelay, &maxDelay, &reject,
		&cfgDailyStart, &cfgWeeklyStart, &cfgMonthlyStart,
		&cfg.DailyUsageUSD, &cfg.WeeklyUsageUSD, &cfg.MonthlyUsageUSD,
		&cfg.SlowRequestCount, &cfg.SlowRejectCount, &lastSlowAt,
		&subID, &subUserID, &subGroupID, &subDailyStart, &subWeeklyStart, &subMonthlyStart,
		&sub.DailyUsageUSD, &sub.WeeklyUsageUSD, &sub.MonthlyUsageUSD, &subExpiresAt, &subStatus,
	); err != nil {
		return nil, err
	}
	state.Group.DailyLimitUSD = float64PtrFromNull(dailyLimit)
	state.Group.WeeklyLimitUSD = float64PtrFromNull(weeklyLimit)
	state.Group.MonthlyLimitUSD = float64PtrFromNull(monthlyLimit)
	if cfgUserID.Valid && cfgGroupID.Valid {
		cfg.UserID = cfgUserID.Int64
		cfg.GroupID = cfgGroupID.Int64
		cfg.FastQuotaRatio = float64PtrFromNull(fast)
		cfg.SlowDelayMinSeconds = intPtrFromNull(minDelay)
		cfg.SlowDelayMaxSeconds = intPtrFromNull(maxDelay)
		cfg.SlowRejectRate = float64PtrFromNull(reject)
		cfg.DailyWindowStart = timePtrFromNull(cfgDailyStart)
		cfg.WeeklyWindowStart = timePtrFromNull(cfgWeeklyStart)
		cfg.MonthlyWindowStart = timePtrFromNull(cfgMonthlyStart)
		cfg.LastSlowAt = timePtrFromNull(lastSlowAt)
		state.Config = cfg
	}
	if subID.Valid && subUserID.Valid && subGroupID.Valid {
		sub.ID = subID.Int64
		sub.UserID = subUserID.Int64
		sub.GroupID = subGroupID.Int64
		sub.DailyWindowStart = timePtrFromNull(subDailyStart)
		sub.WeeklyWindowStart = timePtrFromNull(subWeeklyStart)
		sub.MonthlyWindowStart = timePtrFromNull(subMonthlyStart)
		sub.ExpiresAt = subExpiresAt.Time
		sub.Status = subStatus.String
		state.Subscription = sub
	}
	return state, nil
}

func float64PtrFromNull(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func intPtrFromNull(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func timePtrFromNull(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func speedNullFloat64(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}
