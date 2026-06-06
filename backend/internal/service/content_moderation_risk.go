package service

import (
	"context"
	"math"
	"strings"
	"time"
)

func (s *ContentModerationService) riskSnapshotForUser(ctx context.Context, cfg *ContentModerationConfig, userID int64) *ContentModerationRiskSnapshot {
	if cfg == nil {
		return &ContentModerationRiskSnapshot{}
	}
	snapshot := &ContentModerationRiskSnapshot{
		Weight:                0,
		EffectiveSampleRate:   cfg.SampleRate,
		EffectiveBanThreshold: cfg.BanThreshold,
	}
	if !cfg.RiskWeightEnabled || userID <= 0 || s == nil || s.repo == nil {
		return snapshot
	}
	profile, err := s.repo.GetUserRiskProfile(ctx, userID)
	if err != nil {
		return snapshot
	}
	weight := effectiveContentModerationRiskWeight(profile, cfg, time.Now())
	snapshot.Weight = weight
	snapshot.EffectiveSampleRate = contentModerationEffectiveSampleRate(cfg, weight)
	snapshot.EffectiveBanThreshold = contentModerationEffectiveBanThreshold(cfg, weight)
	return snapshot
}

func effectiveContentModerationRiskWeight(profile *ContentModerationUserRiskProfile, cfg *ContentModerationConfig, now time.Time) float64 {
	if profile == nil || cfg == nil {
		return 0
	}
	weight := profile.CurrentWeight
	if weight <= 0 {
		return 0
	}
	if now.IsZero() {
		now = time.Now()
	}
	base := profile.LastEventAt
	if base == nil {
		base = profile.LastDecayAt
	}
	if base == nil || base.IsZero() || !now.After(*base) || cfg.DecayHalfLifeDays <= 0 {
		return weight
	}
	days := now.Sub(*base).Hours() / 24
	if days <= 0 {
		return weight
	}
	decayed := weight * math.Pow(0.5, days/float64(cfg.DecayHalfLifeDays))
	if decayed < 0.0001 {
		return 0
	}
	return decayed
}

func contentModerationEffectiveSampleRate(cfg *ContentModerationConfig, weight float64) int {
	if cfg == nil {
		return 0
	}
	rate := cfg.SampleRate + int(math.Floor(weight))
	maxRate := cfg.MaxSampleRate
	if maxRate <= 0 {
		maxRate = 100
	}
	if maxRate > 100 {
		maxRate = 100
	}
	if rate > maxRate {
		rate = maxRate
	}
	if rate < 0 {
		rate = 0
	}
	return rate
}

func contentModerationEffectiveBanThreshold(cfg *ContentModerationConfig, weight float64) int {
	if cfg == nil {
		return 0
	}
	step := cfg.BanThresholdWeightStep
	if step <= 0 {
		step = defaultContentModerationBanThresholdStep
	}
	threshold := cfg.BanThreshold - int(math.Floor(weight/float64(step)))
	minThreshold := cfg.MinEffectiveBanThreshold
	if minThreshold <= 0 {
		minThreshold = defaultContentModerationMinBanThreshold
	}
	if threshold < minThreshold {
		threshold = minThreshold
	}
	return threshold
}

func (s *ContentModerationService) decorateModerationLog(log *ContentModerationLog, snapshot *ContentModerationRiskSnapshot, contextID *int64, source string, stage string) {
	if log == nil {
		return
	}
	if snapshot != nil {
		log.RiskWeightSnapshot = snapshot.Weight
		log.EffectiveSampleRate = snapshot.EffectiveSampleRate
		log.EffectiveBanThreshold = snapshot.EffectiveBanThreshold
	}
	log.ContextID = cloneInt64Ptr(contextID)
	log.RiskEventSource = strings.TrimSpace(source)
	log.ReviewStage = strings.TrimSpace(stage)
}

func (s *ContentModerationService) recordUserRiskEvent(ctx context.Context, cfg *ContentModerationConfig, userID int64, eventType string, delta float64, source string, stage string, reason string, logID *int64, contextID *int64, manualSuspicious *bool) {
	if s == nil || s.repo == nil || cfg == nil || !cfg.RiskWeightEnabled || userID <= 0 {
		return
	}
	now := time.Now()
	profile, err := s.repo.GetUserRiskProfile(ctx, userID)
	if err != nil {
		return
	}
	if profile == nil {
		profile = &ContentModerationUserRiskProfile{UserID: userID}
	}
	before := effectiveContentModerationRiskWeight(profile, cfg, now)
	after := before + delta
	if after < 0 {
		after = 0
	}
	profile.CurrentWeight = after
	if manualSuspicious != nil {
		profile.ManualSuspicious = *manualSuspicious
	}
	switch eventType {
	case ContentModerationRiskEventFlagged:
		profile.CumulativeFlaggedCount++
	case ContentModerationRiskEventBan:
		profile.CumulativeBanCount++
	}
	profile.LastEventAt = &now
	profile.LastDecayAt = &now
	if err := s.repo.UpsertUserRiskProfile(ctx, profile); err != nil {
		return
	}
	_ = s.repo.CreateUserRiskEvent(ctx, &ContentModerationUserRiskEvent{
		UserID:                userID,
		EventType:             strings.TrimSpace(eventType),
		Source:                strings.TrimSpace(source),
		ReviewStage:           strings.TrimSpace(stage),
		WeightDelta:           delta,
		EffectiveWeightBefore: before,
		EffectiveWeightAfter:  after,
		Reason:                trimRunes(strings.TrimSpace(reason), 500),
		LogID:                 cloneInt64Ptr(logID),
		ContextID:             cloneInt64Ptr(contextID),
	})
}

func (s *ContentModerationService) riskProfileWithEffectiveWeight(ctx context.Context, cfg *ContentModerationConfig, userID int64) (*ContentModerationUserRiskProfile, error) {
	if userID <= 0 {
		return &ContentModerationUserRiskProfile{UserID: userID}, nil
	}
	var profile *ContentModerationUserRiskProfile
	var err error
	if s != nil && s.repo != nil {
		profile, err = s.repo.GetUserRiskProfile(ctx, userID)
		if err != nil {
			return nil, err
		}
	}
	if profile == nil {
		profile = &ContentModerationUserRiskProfile{UserID: userID}
	}
	profile.EffectiveWeight = effectiveContentModerationRiskWeight(profile, cfg, time.Now())
	return profile, nil
}
