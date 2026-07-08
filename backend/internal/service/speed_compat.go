package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrSpeedConfigForbidden = infraerrors.Forbidden("SPEED_CONFIG_FORBIDDEN", "优速通配置不可用")

const (
	SpeedStateRefused = "refused"
	SpeedRouteDirect  = "direct"
)

type UserGroupSpeedConfig struct {
	FastQuotaRatio      *float64 `json:"fast_quota_ratio,omitempty"`
	SlowDelayMinSeconds *int     `json:"slow_delay_min_seconds,omitempty"`
	SlowDelayMaxSeconds *int     `json:"slow_delay_max_seconds,omitempty"`
	SlowRejectRate      *float64 `json:"slow_reject_rate,omitempty"`
}

type EffectiveSpeedConfig struct {
	FastQuotaRatio      float64 `json:"fast_quota_ratio"`
	SlowDelayMinSeconds int     `json:"slow_delay_min_seconds"`
	SlowDelayMaxSeconds int     `json:"slow_delay_max_seconds"`
	SlowRejectRate      float64 `json:"slow_reject_rate"`
}

type EffectiveSpeedLimits struct {
	MinFastQuotaRatio   float64 `json:"min_fast_quota_ratio"`
	MaxFastQuotaRatio   float64 `json:"max_fast_quota_ratio"`
	MaxSlowDelaySeconds int     `json:"max_slow_delay_seconds"`
	MaxSlowRejectRate   float64 `json:"max_slow_reject_rate"`
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
	State            string               `json:"state"`
	Config           EffectiveSpeedConfig `json:"config"`
	Limits           EffectiveSpeedLimits `json:"limits"`
	Daily            *SpeedWindowStatus   `json:"daily,omitempty"`
	Weekly           *SpeedWindowStatus   `json:"weekly,omitempty"`
	Monthly          *SpeedWindowStatus   `json:"monthly,omitempty"`
	SlowRequestCount int64                `json:"slow_request_count"`
	SlowRejectCount  int64                `json:"slow_reject_count"`
	LastSlowAt       *time.Time           `json:"last_slow_at,omitempty"`
}

type SpeedService struct{}

func NewSpeedService() *SpeedService {
	return &SpeedService{}
}

func (s *SpeedService) ListUserStatuses(ctx context.Context, userID int64, visibleOnly bool) ([]UserGroupSpeedStatus, error) {
	return []UserGroupSpeedStatus{}, nil
}

func (s *SpeedService) GetUserStatus(ctx context.Context, userID, groupID int64, requireUserVisible bool) (*UserGroupSpeedStatus, error) {
	return nil, ErrSpeedConfigForbidden
}

func (s *SpeedService) GetSubscriptionStatus(ctx context.Context, sub *UserSubscription) (*UserGroupSpeedStatus, error) {
	return nil, nil
}

func (s *SpeedService) UpdateUserConfig(ctx context.Context, actorIsAdmin bool, userID, groupID int64, input UserGroupSpeedConfig) (*UserGroupSpeedStatus, error) {
	return nil, ErrSpeedConfigForbidden
}

func (s *SpeedService) ResetUsage(ctx context.Context, userID, groupID int64) error {
	return ErrSpeedConfigForbidden
}

func (s *SpeedService) ClearUserConfig(ctx context.Context, userID, groupID int64) (*UserGroupSpeedStatus, error) {
	return nil, ErrSpeedConfigForbidden
}

func (s *SpeedService) RecordUsage(ctx context.Context, userID, groupID int64, costUSD float64) {}
