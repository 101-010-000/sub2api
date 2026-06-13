package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type speedRepoStub struct {
	state *UserGroupSpeedState
	cfg   *UserGroupSpeedConfig
}

func (r *speedRepoStub) GetUserGroupConfig(context.Context, int64, int64) (*UserGroupSpeedConfig, error) {
	return r.cfg, nil
}

func (r *speedRepoStub) UpsertUserGroupConfig(_ context.Context, cfg *UserGroupSpeedConfig) error {
	r.cfg = cfg
	if r.state != nil {
		r.state.Config = cfg
	}
	return nil
}

func (r *speedRepoStub) ClearUserGroupConfig(context.Context, int64, int64) error {
	r.cfg = nil
	if r.state != nil {
		r.state.Config = nil
	}
	return nil
}

func (r *speedRepoStub) ResetUserGroupUsage(context.Context, int64, int64, time.Time) error {
	return nil
}

func (r *speedRepoStub) IncrementUserGroupUsage(context.Context, int64, int64, float64, time.Time) error {
	return nil
}

func (r *speedRepoStub) RecordSlowDecision(context.Context, int64, int64, bool, time.Time) error {
	return nil
}

func (r *speedRepoStub) ListUserGroupSpeedStates(context.Context, int64, bool) ([]UserGroupSpeedState, error) {
	if r.state == nil {
		return nil, nil
	}
	return []UserGroupSpeedState{*r.state}, nil
}

func (r *speedRepoStub) GetUserGroupSpeedState(context.Context, int64, int64) (*UserGroupSpeedState, error) {
	return r.state, nil
}

func TestSpeedServiceDefaultFastRatioAndSlowState(t *testing.T) {
	limit := 10.0
	now := time.Now().Add(-time.Hour)
	repo := &speedRepoStub{
		state: &UserGroupSpeedState{
			Group: &Group{
				ID:                    7,
				Name:                  "pro",
				SpeedConfigEnabled:    true,
				SubscriptionType:      SubscriptionTypeSubscription,
				DefaultFastQuotaRatio: 0.3,
				DailyLimitUSD:         &limit,
			},
			Subscription: &UserSubscription{
				UserID:           3,
				GroupID:          7,
				Status:           SubscriptionStatusActive,
				DailyWindowStart: &now,
				DailyUsageUSD:    4,
			},
		},
	}
	svc := NewSpeedService(repo)

	status, err := svc.GetUserStatus(context.Background(), 3, 7, false)
	require.NoError(t, err)
	require.Equal(t, "slow", status.State)
	require.Equal(t, 0.3, status.Config.FastQuotaRatio)
	require.Equal(t, 3.0, status.Daily.FastLimitUSD)
	require.Equal(t, 3.0, status.Daily.FastUsedUSD)
	require.Equal(t, 1.0, status.Daily.SlowUsedUSD)
}

func TestSpeedServiceRejectsOutOfRangeUserConfig(t *testing.T) {
	minRatio := 0.2
	repo := &speedRepoStub{
		state: &UserGroupSpeedState{
			Group: &Group{
				ID:                         7,
				SpeedConfigEnabled:         true,
				SubscriptionType:           SubscriptionTypeSubscription,
				UserSpeedConfigAllowed:     true,
				MinFastQuotaRatio:          minRatio,
				MaxFastQuotaRatio:          0.8,
				MaxSlowDelaySeconds:        10,
				DefaultSlowDelayMaxSeconds: 5,
				MaxSlowRejectRate:          0.2,
			},
			Subscription: &UserSubscription{
				UserID:  3,
				GroupID: 7,
				Status:  SubscriptionStatusActive,
			},
		},
	}
	svc := NewSpeedService(repo)
	fastRatio := 0.9

	_, err := svc.UpdateUserConfig(context.Background(), false, 3, 7, UserGroupSpeedConfig{
		FastQuotaRatio: &fastRatio,
	})
	require.ErrorIs(t, err, ErrSpeedConfigInvalid)
}

func TestSpeedServiceHidesUserConfigWhenNotAllowed(t *testing.T) {
	repo := &speedRepoStub{
		state: &UserGroupSpeedState{
			Group: &Group{
				ID:                 7,
				SpeedConfigEnabled: true,
			},
		},
	}
	svc := NewSpeedService(repo)

	_, err := svc.GetUserStatus(context.Background(), 3, 7, true)
	require.ErrorIs(t, err, ErrSpeedConfigForbidden)
}
