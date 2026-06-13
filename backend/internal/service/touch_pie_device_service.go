package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	TouchPieDeviceStatusPending  = "pending"
	TouchPieDeviceStatusApproved = "approved"
	TouchPieDeviceStatusConsumed = "consumed"
	TouchPieDeviceStatusExpired  = "expired"

	touchPieDeviceCodeBytes = 32
	touchPieUserCodeLength  = 8
	touchPieDeviceTTL       = 10 * time.Minute
)

var (
	ErrTouchPieDeviceNotFound          = infraerrors.NotFound("TOUCH_PIE_DEVICE_NOT_FOUND", "Touch Pie device session not found")
	ErrTouchPieDeviceExpired           = infraerrors.BadRequest("TOUCH_PIE_DEVICE_EXPIRED", "Touch Pie device session expired")
	ErrTouchPieDeviceAuthorizationWait = infraerrors.BadRequest("TOUCH_PIE_AUTHORIZATION_PENDING", "Touch Pie authorization is pending")
	ErrTouchPieDeviceConsumed          = infraerrors.BadRequest("TOUCH_PIE_DEVICE_CONSUMED", "Touch Pie device session already consumed")
	ErrTouchPieAPIKeyForbidden         = infraerrors.Forbidden("TOUCH_PIE_API_KEY_FORBIDDEN", "API key does not belong to current user")
)

type TouchPieDeviceSession struct {
	ID             int64
	DeviceCodeHash string
	UserCodeHash   string
	Status         string
	UserID         *int64
	ExpiresAt      time.Time
	ApprovedAt     *time.Time
	ConsumedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type TouchPieDeviceRepository interface {
	CreateDeviceSession(ctx context.Context, session *TouchPieDeviceSession) error
	GetDeviceSessionByUserCodeHash(ctx context.Context, hash string) (*TouchPieDeviceSession, error)
	GetDeviceSessionByDeviceCodeHash(ctx context.Context, hash string) (*TouchPieDeviceSession, error)
	ApproveDeviceSession(ctx context.Context, id int64, userID int64, now time.Time) error
	ConsumeDeviceSession(ctx context.Context, id int64, now time.Time) error
	ExpireDeviceSession(ctx context.Context, id int64, now time.Time) error
}

type TouchPieDeviceStartResult struct {
	DeviceCode              string    `json:"device_code"`
	UserCode                string    `json:"user_code"`
	VerificationURI         string    `json:"verification_uri"`
	VerificationURIComplete string    `json:"verification_uri_complete"`
	ExpiresAt               time.Time `json:"expires_at"`
	IntervalSeconds         int       `json:"interval_seconds"`
}

type TouchPieDeviceTokenResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	UserID       int64  `json:"user_id"`
}

type TouchPieDeviceService struct {
	repo       TouchPieDeviceRepository
	authSvc    *AuthService
	apiKeyRepo APIKeyRepository
}

func NewTouchPieDeviceService(repo TouchPieDeviceRepository, authSvc *AuthService, apiKeyRepo APIKeyRepository) *TouchPieDeviceService {
	return &TouchPieDeviceService{repo: repo, authSvc: authSvc, apiKeyRepo: apiKeyRepo}
}

func (s *TouchPieDeviceService) Start(ctx context.Context, baseURL string) (*TouchPieDeviceStartResult, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("TOUCH_PIE_UNAVAILABLE", "Touch Pie device service unavailable")
	}
	deviceCode, err := randomBase64URL(touchPieDeviceCodeBytes)
	if err != nil {
		return nil, err
	}
	userCode, err := randomUserCode(touchPieUserCodeLength)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().UTC().Add(touchPieDeviceTTL)
	session := &TouchPieDeviceSession{
		DeviceCodeHash: hashTouchPieCode(deviceCode),
		UserCodeHash:   hashTouchPieCode(userCode),
		Status:         TouchPieDeviceStatusPending,
		ExpiresAt:      expiresAt,
	}
	if err := s.repo.CreateDeviceSession(ctx, session); err != nil {
		return nil, err
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	verificationURI := baseURL + "/touch-pie/authorize"
	return &TouchPieDeviceStartResult{
		DeviceCode:              deviceCode,
		UserCode:                userCode,
		VerificationURI:         verificationURI,
		VerificationURIComplete: verificationURI + "?user_code=" + userCode,
		ExpiresAt:               expiresAt,
		IntervalSeconds:         2,
	}, nil
}

func (s *TouchPieDeviceService) Approve(ctx context.Context, userCode string, userID int64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("TOUCH_PIE_UNAVAILABLE", "Touch Pie device service unavailable")
	}
	session, err := s.repo.GetDeviceSessionByUserCodeHash(ctx, hashTouchPieCode(userCode))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if session.ExpiresAt.Before(now) {
		_ = s.repo.ExpireDeviceSession(ctx, session.ID, now)
		return ErrTouchPieDeviceExpired
	}
	if session.Status == TouchPieDeviceStatusConsumed {
		return ErrTouchPieDeviceConsumed
	}
	if session.Status != TouchPieDeviceStatusPending && session.Status != TouchPieDeviceStatusApproved {
		return ErrTouchPieDeviceNotFound
	}
	return s.repo.ApproveDeviceSession(ctx, session.ID, userID, now)
}

func (s *TouchPieDeviceService) Token(ctx context.Context, deviceCode string) (*TouchPieDeviceTokenResult, error) {
	if s == nil || s.repo == nil || s.authSvc == nil {
		return nil, infraerrors.ServiceUnavailable("TOUCH_PIE_UNAVAILABLE", "Touch Pie device service unavailable")
	}
	session, err := s.repo.GetDeviceSessionByDeviceCodeHash(ctx, hashTouchPieCode(deviceCode))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if session.ExpiresAt.Before(now) {
		_ = s.repo.ExpireDeviceSession(ctx, session.ID, now)
		return nil, ErrTouchPieDeviceExpired
	}
	switch session.Status {
	case TouchPieDeviceStatusPending:
		return nil, ErrTouchPieDeviceAuthorizationWait
	case TouchPieDeviceStatusConsumed:
		return nil, ErrTouchPieDeviceConsumed
	case TouchPieDeviceStatusApproved:
	default:
		return nil, ErrTouchPieDeviceNotFound
	}
	if session.UserID == nil || *session.UserID <= 0 {
		return nil, ErrTouchPieDeviceNotFound
	}
	user, err := s.authSvc.userRepo.GetByID(ctx, *session.UserID)
	if err != nil {
		return nil, err
	}
	tokenPair, err := s.authSvc.GenerateTokenPair(ctx, user, "touch-pie")
	if err != nil {
		return nil, err
	}
	if err := s.repo.ConsumeDeviceSession(ctx, session.ID, now); err != nil {
		return nil, err
	}
	return &TouchPieDeviceTokenResult{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
		TokenType:    "Bearer",
		UserID:       user.ID,
	}, nil
}

func (s *TouchPieDeviceService) ExportAPIKey(ctx context.Context, userID, keyID int64) (*APIKey, error) {
	if s == nil || s.apiKeyRepo == nil {
		return nil, infraerrors.ServiceUnavailable("TOUCH_PIE_UNAVAILABLE", "Touch Pie device service unavailable")
	}
	apiKey, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return nil, err
	}
	if apiKey == nil || apiKey.UserID != userID {
		return nil, ErrTouchPieAPIKeyForbidden
	}
	return apiKey, nil
}

func hashTouchPieCode(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func randomBase64URL(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate touch pie code: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func randomUserCode(length int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	if length <= 0 {
		return "", errors.New("invalid user code length")
	}
	out := make([]byte, length)
	max := big.NewInt(int64(len(alphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate touch pie user code: %w", err)
		}
		out[i] = alphabet[n.Int64()]
	}
	return string(out), nil
}
