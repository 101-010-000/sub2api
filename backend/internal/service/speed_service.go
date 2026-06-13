package service

import (
	"context"
	"math/rand/v2"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrSpeedConfigForbidden = infraerrors.Forbidden("SPEED_CONFIG_FORBIDDEN", "优速通配置不可用")
	ErrSpeedConfigInvalid   = infraerrors.BadRequest("SPEED_CONFIG_INVALID", "优速通配置无效")
	ErrSpeedSlowRejected    = infraerrors.TooManyRequests("SPEED_SLOW_REJECTED", "优速通慢速请求被限流")
)

const (
	defaultFastQuotaRatio       = 0.30
	defaultMinFastQuotaRatio    = 0.10
	defaultMaxFastQuotaRatio    = 0.80
	defaultSlowDelayMinSeconds  = 1
	defaultSlowDelayMaxSeconds  = 5
	defaultMaxSlowDelaySeconds  = 60
	touchPieSlowDelayMinSeconds = 1
	touchPieSlowDelayMaxSeconds = 60
	defaultMaxSlowRejectRate    = 0.50

	speedBillingModeSubscription = "subscription"
)

type SpeedRepository interface {
	GetUserGroupConfig(ctx context.Context, userID, groupID int64) (*UserGroupSpeedConfig, error)
	UpsertUserGroupConfig(ctx context.Context, cfg *UserGroupSpeedConfig) error
	ClearUserGroupConfig(ctx context.Context, userID, groupID int64) error
	ResetUserGroupUsage(ctx context.Context, userID, groupID int64, now time.Time) error
	IncrementUserGroupUsage(ctx context.Context, userID, groupID int64, costUSD float64, now time.Time) error
	RecordSlowDecision(ctx context.Context, userID, groupID int64, rejected bool, now time.Time) error
	ListUserGroupSpeedStates(ctx context.Context, userID int64, visibleOnly bool) ([]UserGroupSpeedState, error)
	GetUserGroupSpeedState(ctx context.Context, userID, groupID int64) (*UserGroupSpeedState, error)
}

type UserGroupSpeedConfig struct {
	UserID              int64      `json:"user_id"`
	GroupID             int64      `json:"group_id"`
	FastQuotaRatio      *float64   `json:"fast_quota_ratio,omitempty"`
	SlowDelayMinSeconds *int       `json:"slow_delay_min_seconds,omitempty"`
	SlowDelayMaxSeconds *int       `json:"slow_delay_max_seconds,omitempty"`
	SlowRejectRate      *float64   `json:"slow_reject_rate,omitempty"`
	DailyWindowStart    *time.Time `json:"daily_window_start,omitempty"`
	WeeklyWindowStart   *time.Time `json:"weekly_window_start,omitempty"`
	MonthlyWindowStart  *time.Time `json:"monthly_window_start,omitempty"`
	DailyUsageUSD       float64    `json:"daily_usage_usd"`
	WeeklyUsageUSD      float64    `json:"weekly_usage_usd"`
	MonthlyUsageUSD     float64    `json:"monthly_usage_usd"`
	SlowRequestCount    int64      `json:"slow_request_count"`
	SlowRejectCount     int64      `json:"slow_reject_count"`
	LastSlowAt          *time.Time `json:"last_slow_at,omitempty"`
}

type EffectiveSpeedConfig struct {
	FastQuotaRatio      float64 `json:"fast_quota_ratio"`
	SlowDelayMinSeconds int     `json:"slow_delay_min_seconds"`
	SlowDelayMaxSeconds int     `json:"slow_delay_max_seconds"`
	SlowRejectRate      float64 `json:"slow_reject_rate"`
}

type SpeedWindowStatus struct {
	LimitUSD        float64    `json:"limit_usd"`
	FastLimitUSD    float64    `json:"fast_limit_usd"`
	FastUsedUSD     float64    `json:"fast_used_usd"`
	SlowLimitUSD    float64    `json:"slow_limit_usd"`
	SlowUsedUSD     float64    `json:"slow_used_usd"`
	TotalUsedUSD    float64    `json:"total_used_usd"`
	RemainingUSD    float64    `json:"remaining_usd"`
	WindowStart     *time.Time `json:"window_start,omitempty"`
	ResetsAt        *time.Time `json:"resets_at,omitempty"`
	ResetsInSeconds int64      `json:"resets_in_seconds"`
}

type UserGroupSpeedStatus struct {
	UserID           int64                `json:"user_id"`
	GroupID          int64                `json:"group_id"`
	GroupName        string               `json:"group_name"`
	VisibleToUser    bool                 `json:"visible_to_user"`
	Enabled          bool                 `json:"enabled"`
	BillingMode      string               `json:"billing_mode"`
	State            string               `json:"state"` // fast / slow / exhausted / disabled
	Config           EffectiveSpeedConfig `json:"config"`
	Limits           EffectiveSpeedLimits `json:"limits"`
	Daily            *SpeedWindowStatus   `json:"daily,omitempty"`
	Weekly           *SpeedWindowStatus   `json:"weekly,omitempty"`
	Monthly          *SpeedWindowStatus   `json:"monthly,omitempty"`
	SlowRequestCount int64                `json:"slow_request_count"`
	SlowRejectCount  int64                `json:"slow_reject_count"`
	LastSlowAt       *time.Time           `json:"last_slow_at,omitempty"`
}

type EffectiveSpeedLimits struct {
	MinFastQuotaRatio   float64 `json:"min_fast_quota_ratio"`
	MaxFastQuotaRatio   float64 `json:"max_fast_quota_ratio"`
	MaxSlowDelaySeconds int     `json:"max_slow_delay_seconds"`
	MaxSlowRejectRate   float64 `json:"max_slow_reject_rate"`
}

type UserGroupSpeedState struct {
	Group        *Group
	Config       *UserGroupSpeedConfig
	Subscription *UserSubscription
}

type SpeedDecision struct {
	Enabled  bool
	State    string
	Delay    time.Duration
	Rejected bool
	Status   *UserGroupSpeedStatus
}

type SpeedService struct {
	repo SpeedRepository
}

func NewSpeedService(repo SpeedRepository) *SpeedService {
	return &SpeedService{repo: repo}
}

func (s *SpeedService) ListUserStatuses(ctx context.Context, userID int64, visibleOnly bool) ([]UserGroupSpeedStatus, error) {
	if s == nil || s.repo == nil {
		return []UserGroupSpeedStatus{}, nil
	}
	states, err := s.repo.ListUserGroupSpeedStates(ctx, userID, visibleOnly)
	if err != nil {
		return nil, err
	}
	out := make([]UserGroupSpeedStatus, 0, len(states))
	now := time.Now().UTC()
	for i := range states {
		out = append(out, *s.statusFromState(&states[i], now))
	}
	return out, nil
}

func (s *SpeedService) GetUserStatus(ctx context.Context, userID, groupID int64, requireUserVisible bool) (*UserGroupSpeedStatus, error) {
	if s == nil || s.repo == nil {
		return nil, ErrSpeedConfigForbidden
	}
	state, err := s.repo.GetUserGroupSpeedState(ctx, userID, groupID)
	if err != nil {
		return nil, err
	}
	if !speedSupportedGroup(state.Group) {
		return nil, ErrSpeedConfigForbidden
	}
	if state.Subscription == nil {
		return nil, ErrSpeedConfigForbidden
	}
	if requireUserVisible && !state.Group.UserSpeedConfigAllowed {
		return nil, ErrSpeedConfigForbidden
	}
	return s.statusFromState(state, time.Now().UTC()), nil
}

func (s *SpeedService) GetSubscriptionStatus(ctx context.Context, sub *UserSubscription) (*UserGroupSpeedStatus, error) {
	if s == nil || s.repo == nil || sub == nil || sub.Group == nil || sub.User == nil {
		return nil, nil
	}
	cfg, err := s.repo.GetUserGroupConfig(ctx, sub.User.ID, sub.Group.ID)
	if err != nil {
		return nil, err
	}
	return s.statusFromState(&UserGroupSpeedState{
		Group:        sub.Group,
		Config:       cfg,
		Subscription: sub,
	}, time.Now().UTC()), nil
}

func (s *SpeedService) UpdateUserConfig(ctx context.Context, actorIsAdmin bool, userID, groupID int64, input UserGroupSpeedConfig) (*UserGroupSpeedStatus, error) {
	if s == nil || s.repo == nil {
		return nil, ErrSpeedConfigForbidden
	}
	state, err := s.repo.GetUserGroupSpeedState(ctx, userID, groupID)
	if err != nil {
		return nil, err
	}
	if !speedSupportedGroup(state.Group) || state.Subscription == nil {
		return nil, ErrSpeedConfigForbidden
	}
	if !actorIsAdmin && !state.Group.UserSpeedConfigAllowed {
		return nil, ErrSpeedConfigForbidden
	}
	cfg := &UserGroupSpeedConfig{
		UserID:              userID,
		GroupID:             groupID,
		FastQuotaRatio:      input.FastQuotaRatio,
		SlowDelayMinSeconds: input.SlowDelayMinSeconds,
		SlowDelayMaxSeconds: input.SlowDelayMaxSeconds,
		SlowRejectRate:      input.SlowRejectRate,
	}
	if err := validateSpeedConfig(state.Group, cfg); err != nil {
		return nil, err
	}
	if err := s.repo.UpsertUserGroupConfig(ctx, cfg); err != nil {
		return nil, err
	}
	return s.GetUserStatus(ctx, userID, groupID, !actorIsAdmin)
}

func (s *SpeedService) ResetUsage(ctx context.Context, userID, groupID int64) error {
	if s == nil || s.repo == nil {
		return nil
	}
	state, err := s.repo.GetUserGroupSpeedState(ctx, userID, groupID)
	if err != nil {
		return err
	}
	if !speedSupportedGroup(state.Group) || state.Subscription == nil {
		return ErrSpeedConfigForbidden
	}
	return s.repo.ResetUserGroupUsage(ctx, userID, groupID, time.Now().UTC())
}

func (s *SpeedService) ClearUserConfig(ctx context.Context, userID, groupID int64) (*UserGroupSpeedStatus, error) {
	if s == nil || s.repo == nil {
		return nil, ErrSpeedConfigForbidden
	}
	state, err := s.repo.GetUserGroupSpeedState(ctx, userID, groupID)
	if err != nil {
		return nil, err
	}
	if !speedSupportedGroup(state.Group) || state.Subscription == nil {
		return nil, ErrSpeedConfigForbidden
	}
	if err := s.repo.ClearUserGroupConfig(ctx, userID, groupID); err != nil {
		return nil, err
	}
	return s.GetUserStatus(ctx, userID, groupID, false)
}

func (s *SpeedService) RecordUsage(ctx context.Context, userID, groupID int64, costUSD float64) {
	if s == nil || s.repo == nil || userID <= 0 || groupID <= 0 || costUSD <= 0 {
		return
	}
	_ = s.repo.IncrementUserGroupUsage(ctx, userID, groupID, costUSD, time.Now().UTC())
}

func (s *SpeedService) Decide(ctx context.Context, user *User, group *Group, subscription *UserSubscription) (*SpeedDecision, error) {
	if s == nil || s.repo == nil || user == nil || !speedSupportedGroup(group) || subscription == nil {
		return &SpeedDecision{Enabled: false, State: "disabled"}, nil
	}
	state := &UserGroupSpeedState{Group: group, Subscription: subscription}
	cfg, _ := s.repo.GetUserGroupConfig(ctx, user.ID, group.ID)
	state.Config = cfg
	status := s.statusFromState(state, time.Now().UTC())
	if status.State == "disabled" || status.State == "fast" {
		return &SpeedDecision{Enabled: status.Enabled, State: status.State, Status: status}, nil
	}
	if status.State == "exhausted" {
		return &SpeedDecision{Enabled: true, State: "exhausted", Status: status}, ErrSubscriptionInvalid
	}
	if rand.Float64() < status.Config.SlowRejectRate {
		_ = s.repo.RecordSlowDecision(ctx, user.ID, group.ID, true, time.Now().UTC())
		return &SpeedDecision{Enabled: true, State: "slow", Rejected: true, Status: status}, ErrSpeedSlowRejected
	}
	delay := randomSlowDelay(status.Config.SlowDelayMinSeconds, status.Config.SlowDelayMaxSeconds)
	_ = s.repo.RecordSlowDecision(ctx, user.ID, group.ID, false, time.Now().UTC())
	return &SpeedDecision{Enabled: true, State: "slow", Delay: delay, Status: status}, nil
}

func (s *SpeedService) statusFromState(state *UserGroupSpeedState, now time.Time) *UserGroupSpeedStatus {
	group := state.Group
	if group == nil {
		return &UserGroupSpeedStatus{State: "disabled"}
	}
	cfg := mergeSpeedConfig(group, state.Config)
	status := &UserGroupSpeedStatus{
		GroupID:          group.ID,
		GroupName:        group.Name,
		VisibleToUser:    group.UserSpeedConfigAllowed && group.SpeedConfigEnabled,
		Enabled:          group.SpeedConfigEnabled,
		State:            "disabled",
		Config:           cfg,
		Limits:           effectiveSpeedLimits(group),
		SlowRequestCount: slowRequestCount(state.Config),
		SlowRejectCount:  slowRejectCount(state.Config),
		LastSlowAt:       lastSlowAt(state.Config),
	}
	if state.Config != nil {
		status.UserID = state.Config.UserID
	} else if state.Subscription != nil {
		status.UserID = state.Subscription.UserID
	}
	if !speedSupportedGroup(group) {
		status.Enabled = false
		status.VisibleToUser = false
		return status
	}
	if state.Subscription == nil {
		return status
	}
	status.BillingMode = speedBillingModeSubscription
	status.Daily = speedWindow(group.DailyLimitUSD, state.Subscription.DailyUsageUSD, state.Subscription.DailyWindowStart, 24*time.Hour, cfg.FastQuotaRatio, now)
	status.Weekly = speedWindow(group.WeeklyLimitUSD, state.Subscription.WeeklyUsageUSD, state.Subscription.WeeklyWindowStart, 7*24*time.Hour, cfg.FastQuotaRatio, now)
	status.Monthly = speedWindow(group.MonthlyLimitUSD, state.Subscription.MonthlyUsageUSD, state.Subscription.MonthlyWindowStart, 30*24*time.Hour, cfg.FastQuotaRatio, now)
	status.State = resolveSpeedState(status.Daily, status.Weekly, status.Monthly)
	return status
}

func speedSupportedGroup(group *Group) bool {
	return group != nil && group.SpeedConfigEnabled && group.IsSubscriptionType()
}

func validateSpeedConfig(group *Group, cfg *UserGroupSpeedConfig) error {
	limits := effectiveSpeedLimits(group)
	if cfg.FastQuotaRatio != nil && (*cfg.FastQuotaRatio < limits.MinFastQuotaRatio || *cfg.FastQuotaRatio > limits.MaxFastQuotaRatio) {
		return ErrSpeedConfigInvalid
	}
	if cfg.SlowRejectRate != nil && (*cfg.SlowRejectRate < 0 || *cfg.SlowRejectRate > limits.MaxSlowRejectRate) {
		return ErrSpeedConfigInvalid
	}
	if cfg.SlowDelayMinSeconds != nil && (*cfg.SlowDelayMinSeconds < 0 || *cfg.SlowDelayMinSeconds > limits.MaxSlowDelaySeconds) {
		return ErrSpeedConfigInvalid
	}
	if cfg.SlowDelayMaxSeconds != nil && (*cfg.SlowDelayMaxSeconds < 0 || *cfg.SlowDelayMaxSeconds > limits.MaxSlowDelaySeconds) {
		return ErrSpeedConfigInvalid
	}
	if cfg.SlowDelayMinSeconds != nil && cfg.SlowDelayMaxSeconds != nil && *cfg.SlowDelayMinSeconds > *cfg.SlowDelayMaxSeconds {
		return ErrSpeedConfigInvalid
	}
	return nil
}

func mergeSpeedConfig(group *Group, cfg *UserGroupSpeedConfig) EffectiveSpeedConfig {
	fastRatio := defaultedRatio(group.DefaultFastQuotaRatio, defaultFastQuotaRatio)
	minDelay := defaultedInt(group.DefaultSlowDelayMinSeconds, defaultSlowDelayMinSeconds)
	maxDelay := defaultedInt(group.DefaultSlowDelayMaxSeconds, defaultSlowDelayMaxSeconds)
	rejectRate := clampSpeedRatio(group.DefaultSlowRejectRate)
	if cfg != nil {
		if cfg.FastQuotaRatio != nil {
			fastRatio = *cfg.FastQuotaRatio
		}
		if cfg.SlowDelayMinSeconds != nil {
			minDelay = *cfg.SlowDelayMinSeconds
		}
		if cfg.SlowDelayMaxSeconds != nil {
			maxDelay = *cfg.SlowDelayMaxSeconds
		}
		if cfg.SlowRejectRate != nil {
			rejectRate = *cfg.SlowRejectRate
		}
	}
	limits := effectiveSpeedLimits(group)
	if fastRatio < limits.MinFastQuotaRatio {
		fastRatio = limits.MinFastQuotaRatio
	}
	if fastRatio > limits.MaxFastQuotaRatio {
		fastRatio = limits.MaxFastQuotaRatio
	}
	if maxDelay > limits.MaxSlowDelaySeconds {
		maxDelay = limits.MaxSlowDelaySeconds
	}
	if minDelay > maxDelay {
		minDelay = maxDelay
	}
	if rejectRate > limits.MaxSlowRejectRate {
		rejectRate = limits.MaxSlowRejectRate
	}
	return EffectiveSpeedConfig{
		FastQuotaRatio:      fastRatio,
		SlowDelayMinSeconds: minDelay,
		SlowDelayMaxSeconds: maxDelay,
		SlowRejectRate:      rejectRate,
	}
}

func effectiveSpeedLimits(group *Group) EffectiveSpeedLimits {
	return EffectiveSpeedLimits{
		MinFastQuotaRatio:   defaultedRatio(group.MinFastQuotaRatio, defaultMinFastQuotaRatio),
		MaxFastQuotaRatio:   defaultedRatio(group.MaxFastQuotaRatio, defaultMaxFastQuotaRatio),
		MaxSlowDelaySeconds: defaultedInt(group.MaxSlowDelaySeconds, defaultMaxSlowDelaySeconds),
		MaxSlowRejectRate:   defaultedRatio(group.MaxSlowRejectRate, defaultMaxSlowRejectRate),
	}
}

func speedWindow(limit *float64, used float64, start *time.Time, dur time.Duration, ratio float64, now time.Time) *SpeedWindowStatus {
	if limit == nil || *limit <= 0 {
		return nil
	}
	if start != nil && now.After(start.Add(dur)) {
		used = 0
	}
	fastLimit := *limit * ratio
	fastUsed := used
	if fastUsed > fastLimit {
		fastUsed = fastLimit
	}
	slowUsed := used - fastLimit
	if slowUsed < 0 {
		slowUsed = 0
	}
	res := &SpeedWindowStatus{
		LimitUSD:     *limit,
		FastLimitUSD: fastLimit,
		FastUsedUSD:  fastUsed,
		SlowLimitUSD: *limit - fastLimit,
		SlowUsedUSD:  slowUsed,
		TotalUsedUSD: used,
		RemainingUSD: *limit - used,
		WindowStart:  start,
	}
	if res.RemainingUSD < 0 {
		res.RemainingUSD = 0
	}
	if start != nil {
		reset := start.Add(dur)
		res.ResetsAt = &reset
		res.ResetsInSeconds = int64(time.Until(reset).Seconds())
		if res.ResetsInSeconds < 0 {
			res.ResetsInSeconds = 0
		}
	}
	return res
}

func resolveSpeedState(windows ...*SpeedWindowStatus) string {
	hasWindow := false
	slow := false
	for _, w := range windows {
		if w == nil {
			continue
		}
		hasWindow = true
		if w.TotalUsedUSD >= w.LimitUSD {
			return "exhausted"
		}
		if w.TotalUsedUSD >= w.FastLimitUSD {
			slow = true
		}
	}
	if !hasWindow {
		return "disabled"
	}
	if slow {
		return "slow"
	}
	return "fast"
}

func randomSlowDelay(minSeconds, maxSeconds int) time.Duration {
	if minSeconds < touchPieSlowDelayMinSeconds {
		minSeconds = touchPieSlowDelayMinSeconds
	}
	if minSeconds > touchPieSlowDelayMaxSeconds {
		minSeconds = touchPieSlowDelayMaxSeconds
	}
	if maxSeconds < minSeconds {
		maxSeconds = minSeconds
	}
	if maxSeconds > touchPieSlowDelayMaxSeconds {
		maxSeconds = touchPieSlowDelayMaxSeconds
	}
	if maxSeconds == minSeconds {
		return time.Duration(minSeconds) * time.Second
	}
	return time.Duration(minSeconds+rand.IntN(maxSeconds-minSeconds+1)) * time.Second
}

func defaultedRatio(v, fallback float64) float64 {
	if v <= 0 {
		return fallback
	}
	return clampSpeedRatio(v)
}

func defaultedInt(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}

func clampSpeedRatio(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func slowRequestCount(cfg *UserGroupSpeedConfig) int64 {
	if cfg == nil {
		return 0
	}
	return cfg.SlowRequestCount
}

func slowRejectCount(cfg *UserGroupSpeedConfig) int64 {
	if cfg == nil {
		return 0
	}
	return cfg.SlowRejectCount
}

func lastSlowAt(cfg *UserGroupSpeedConfig) *time.Time {
	if cfg == nil {
		return nil
	}
	return cfg.LastSlowAt
}
