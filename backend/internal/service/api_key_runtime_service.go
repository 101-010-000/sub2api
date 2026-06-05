package service

import (
	"context"
	"fmt"
	"time"
)

type APIKeyRuntimeCache interface {
	BindActiveIP(ctx context.Context, keyID int64, clientIP string, maxActiveIPs int, idleTimeout time.Duration) (bool, error)
	RemoveActiveIP(ctx context.Context, keyID int64, clientIP string) error
	ClearActiveIPs(ctx context.Context, keyID int64) error
	GetActiveIPs(ctx context.Context, keyID int64, idleTimeout time.Duration) ([]APIKeyActiveIP, error)
	AcquireAPIKeySlot(ctx context.Context, keyID int64, maxConcurrency int, requestID string) (bool, error)
	ReleaseAPIKeySlot(ctx context.Context, keyID int64, requestID string) error
	GetAPIKeyConcurrency(ctx context.Context, keyID int64) (int, error)
}

type APIKeyActiveIP struct {
	IP                   string    `json:"ip"`
	LastSeenAt           time.Time `json:"last_seen_at"`
	ExpiresAt            time.Time `json:"expires_at"`
	RemainingIdleSeconds int       `json:"remaining_idle_seconds"`
}

type APIKeyRuntimeStatus struct {
	MaxActiveIPs         int              `json:"max_active_ips"`
	IPIdleTimeoutSeconds int              `json:"ip_idle_timeout_seconds"`
	MaxConcurrency       int              `json:"max_concurrency"`
	CurrentConcurrency   int              `json:"current_concurrency"`
	ActiveIPCount        int              `json:"active_ip_count"`
	ActiveIPs            []APIKeyActiveIP `json:"active_ips"`
}

type APIKeySlot struct {
	acquired  bool
	keyID     int64
	requestID string
	cache     APIKeyRuntimeCache
}

func (s *APIKeySlot) Acquired() bool {
	return s != nil && s.acquired
}

func (s *APIKeySlot) Release(ctx context.Context) error {
	if s == nil || !s.acquired || s.cache == nil {
		return nil
	}
	s.acquired = false
	return s.cache.ReleaseAPIKeySlot(ctx, s.keyID, s.requestID)
}

type APIKeyRuntimeService struct {
	cache APIKeyRuntimeCache
}

func NewAPIKeyRuntimeService(cache APIKeyRuntimeCache) *APIKeyRuntimeService {
	return &APIKeyRuntimeService{cache: cache}
}

func (s *APIKeyRuntimeService) EnforceActiveIP(ctx context.Context, apiKey *APIKey, clientIP string) error {
	if apiKey == nil || apiKey.MaxActiveIPs <= 0 {
		return nil
	}
	if s == nil || s.cache == nil {
		return ErrAPIKeyRuntimeUnavailable
	}
	allowed, err := s.cache.BindActiveIP(
		ctx,
		apiKey.ID,
		clientIP,
		apiKey.MaxActiveIPs,
		time.Duration(EffectiveAPIKeyIPIdleTimeoutSeconds(apiKey.IPIdleTimeoutSeconds))*time.Second,
	)
	if err != nil {
		return ErrAPIKeyRuntimeUnavailable.WithCause(fmt.Errorf("bind active ip: %w", err))
	}
	if !allowed {
		return ErrAPIKeyActiveIPLimitExceeded
	}
	return nil
}

func (s *APIKeyRuntimeService) AcquireConcurrency(ctx context.Context, apiKey *APIKey) (*APIKeySlot, error) {
	if apiKey == nil || apiKey.MaxConcurrency <= 0 {
		return &APIKeySlot{}, nil
	}
	if s == nil || s.cache == nil {
		return nil, ErrAPIKeyRuntimeUnavailable
	}
	requestID := generateRequestID()
	acquired, err := s.cache.AcquireAPIKeySlot(ctx, apiKey.ID, apiKey.MaxConcurrency, requestID)
	if err != nil {
		return nil, ErrAPIKeyRuntimeUnavailable.WithCause(fmt.Errorf("acquire api key slot: %w", err))
	}
	if !acquired {
		return nil, ErrAPIKeyConcurrencyExceeded
	}
	return &APIKeySlot{
		acquired:  true,
		keyID:     apiKey.ID,
		requestID: requestID,
		cache:     s.cache,
	}, nil
}

func (s *APIKeyRuntimeService) GetStatus(ctx context.Context, apiKey *APIKey) (*APIKeyRuntimeStatus, error) {
	if apiKey == nil {
		return nil, ErrAPIKeyNotFound
	}
	status := &APIKeyRuntimeStatus{
		MaxActiveIPs:         apiKey.MaxActiveIPs,
		IPIdleTimeoutSeconds: apiKey.IPIdleTimeoutSeconds,
		MaxConcurrency:       apiKey.MaxConcurrency,
		ActiveIPs:            []APIKeyActiveIP{},
	}
	if s == nil || s.cache == nil {
		if apiKey.MaxActiveIPs > 0 || apiKey.MaxConcurrency > 0 {
			return nil, ErrAPIKeyRuntimeUnavailable
		}
		return status, nil
	}
	if apiKey.MaxConcurrency > 0 {
		count, err := s.cache.GetAPIKeyConcurrency(ctx, apiKey.ID)
		if err != nil {
			return nil, ErrAPIKeyRuntimeUnavailable.WithCause(fmt.Errorf("get api key concurrency: %w", err))
		}
		status.CurrentConcurrency = count
	}
	if apiKey.MaxActiveIPs > 0 {
		idleTimeout := time.Duration(EffectiveAPIKeyIPIdleTimeoutSeconds(apiKey.IPIdleTimeoutSeconds)) * time.Second
		ips, err := s.cache.GetActiveIPs(ctx, apiKey.ID, idleTimeout)
		if err != nil {
			return nil, ErrAPIKeyRuntimeUnavailable.WithCause(fmt.Errorf("get active ips: %w", err))
		}
		status.ActiveIPs = ips
		status.ActiveIPCount = len(ips)
	}
	return status, nil
}

func (s *APIKeyRuntimeService) RemoveActiveIP(ctx context.Context, apiKey *APIKey, clientIP string) error {
	if apiKey == nil {
		return ErrAPIKeyNotFound
	}
	if s == nil || s.cache == nil {
		return ErrAPIKeyRuntimeUnavailable
	}
	return s.cache.RemoveActiveIP(ctx, apiKey.ID, clientIP)
}

func (s *APIKeyRuntimeService) ClearActiveIPs(ctx context.Context, apiKey *APIKey) error {
	if apiKey == nil {
		return ErrAPIKeyNotFound
	}
	if s == nil || s.cache == nil {
		return ErrAPIKeyRuntimeUnavailable
	}
	return s.cache.ClearActiveIPs(ctx, apiKey.ID)
}
