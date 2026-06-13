//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type touchPieDeviceRepoStub struct {
	nextID   int64
	byUser  map[string]*TouchPieDeviceSession
	byDev   map[string]*TouchPieDeviceSession
	approve []int64
	consume []int64
}

func newTouchPieDeviceRepoStub() *touchPieDeviceRepoStub {
	return &touchPieDeviceRepoStub{
		nextID:  1,
		byUser: make(map[string]*TouchPieDeviceSession),
		byDev:  make(map[string]*TouchPieDeviceSession),
	}
}

func (r *touchPieDeviceRepoStub) CreateDeviceSession(_ context.Context, session *TouchPieDeviceSession) error {
	clone := *session
	clone.ID = r.nextID
	r.nextID++
	r.byUser[clone.UserCodeHash] = &clone
	r.byDev[clone.DeviceCodeHash] = &clone
	session.ID = clone.ID
	return nil
}

func (r *touchPieDeviceRepoStub) GetDeviceSessionByUserCodeHash(_ context.Context, hash string) (*TouchPieDeviceSession, error) {
	session := r.byUser[hash]
	if session == nil {
		return nil, ErrTouchPieDeviceNotFound
	}
	clone := *session
	return &clone, nil
}

func (r *touchPieDeviceRepoStub) GetDeviceSessionByDeviceCodeHash(_ context.Context, hash string) (*TouchPieDeviceSession, error) {
	session := r.byDev[hash]
	if session == nil {
		return nil, ErrTouchPieDeviceNotFound
	}
	clone := *session
	return &clone, nil
}

func (r *touchPieDeviceRepoStub) ApproveDeviceSession(_ context.Context, id int64, userID int64, now time.Time) error {
	for _, session := range r.byDev {
		if session.ID == id && session.Status != TouchPieDeviceStatusConsumed && session.ExpiresAt.After(now) {
			session.Status = TouchPieDeviceStatusApproved
			session.UserID = &userID
			session.ApprovedAt = &now
			r.approve = append(r.approve, userID)
			return nil
		}
	}
	return ErrTouchPieDeviceNotFound
}

func (r *touchPieDeviceRepoStub) ConsumeDeviceSession(_ context.Context, id int64, now time.Time) error {
	for _, session := range r.byDev {
		if session.ID == id && session.Status == TouchPieDeviceStatusApproved && session.ExpiresAt.After(now) {
			session.Status = TouchPieDeviceStatusConsumed
			session.ConsumedAt = &now
			r.consume = append(r.consume, id)
			return nil
		}
	}
	return ErrTouchPieDeviceNotFound
}

func (r *touchPieDeviceRepoStub) ExpireDeviceSession(_ context.Context, id int64, now time.Time) error {
	for _, session := range r.byDev {
		if session.ID == id {
			session.Status = TouchPieDeviceStatusExpired
			session.UpdatedAt = now
			return nil
		}
	}
	return ErrTouchPieDeviceNotFound
}

func TestTouchPieDeviceServiceFlow(t *testing.T) {
	ctx := context.Background()
	repo := newTouchPieDeviceRepoStub()
	userRepo := &userRepoStub{
		user: &User{
			ID:     42,
			Email:  "user@example.com",
			Role:   RoleUser,
			Status: StatusActive,
		},
	}
	authSvc := NewAuthService(
		nil,
		userRepo,
		nil,
		&refreshTokenCacheStub{},
		&config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 1, RefreshTokenExpireDays: 30}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	svc := NewTouchPieDeviceService(repo, authSvc, nil)

	start, err := svc.Start(ctx, "https://sub2api.test/")
	require.NoError(t, err)
	require.NotEmpty(t, start.DeviceCode)
	require.Len(t, start.UserCode, touchPieUserCodeLength)
	require.Equal(t, "https://sub2api.test/touch-pie/authorize", start.VerificationURI)
	require.Equal(t, start.VerificationURI+"?user_code="+start.UserCode, start.VerificationURIComplete)

	_, err = svc.Token(ctx, start.DeviceCode)
	require.ErrorIs(t, err, ErrTouchPieDeviceAuthorizationWait)

	require.NoError(t, svc.Approve(ctx, start.UserCode, 42))
	token, err := svc.Token(ctx, start.DeviceCode)
	require.NoError(t, err)
	require.NotEmpty(t, token.AccessToken)
	require.NotEmpty(t, token.RefreshToken)
	require.Equal(t, "Bearer", token.TokenType)
	require.Equal(t, int64(42), token.UserID)
	require.Equal(t, []int64{repo.byDev[hashTouchPieCode(start.DeviceCode)].ID}, repo.consume)

	_, err = svc.Token(ctx, start.DeviceCode)
	require.ErrorIs(t, err, ErrTouchPieDeviceConsumed)
}

func TestTouchPieApproveDoesNotOverwriteOtherUser(t *testing.T) {
	ctx := context.Background()
	repo := newTouchPieDeviceRepoStub()
	svc := NewTouchPieDeviceService(repo, nil, nil)

	start, err := svc.Start(ctx, "https://sub2api.test")
	require.NoError(t, err)
	require.NoError(t, svc.Approve(ctx, start.UserCode, 42))

	err = svc.Approve(ctx, start.UserCode, 99)
	require.ErrorIs(t, err, ErrTouchPieDeviceConsumed)
	require.Equal(t, []int64{42}, repo.approve)
}

func TestTouchPieExportAPIKeyChecksOwnership(t *testing.T) {
	ctx := context.Background()
	apiKeyRepo := &apiKeyRepoStub{
		apiKey: &APIKey{
			ID:     7,
			UserID: 42,
			Name:   "Touch Pie",
			Key:    "sk-test",
			Status: StatusAPIKeyActive,
		},
	}
	svc := NewTouchPieDeviceService(nil, nil, apiKeyRepo)

	key, err := svc.ExportAPIKey(ctx, 42, 7)
	require.NoError(t, err)
	require.Equal(t, "sk-test", key.Key)

	_, err = svc.ExportAPIKey(ctx, 99, 7)
	require.ErrorIs(t, err, ErrTouchPieAPIKeyForbidden)

	apiKeyRepo.getByIDErr = errors.New("db down")
	_, err = svc.ExportAPIKey(ctx, 42, 7)
	require.ErrorContains(t, err, "db down")
}
