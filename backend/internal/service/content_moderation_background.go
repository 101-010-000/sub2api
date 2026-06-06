package service

import (
	"context"
	"strings"
	"time"
)

func (s *ContentModerationService) processBackgroundReviews(ctx context.Context, cfg *ContentModerationConfig) {
	if s == nil || s.repo == nil || cfg == nil || !cfg.BackgroundReviewEnabled {
		return
	}
	items, err := s.repo.ClaimPendingContexts(ctx, cfg.BackgroundReviewBatchSize)
	if err != nil || len(items) == 0 {
		return
	}
	for _, item := range items {
		s.processBackgroundReviewContext(ctx, cfg, item)
	}
}

func (s *ContentModerationService) processBackgroundReviewContext(ctx context.Context, cfg *ContentModerationConfig, item ContentModerationContext) {
	if s == nil || s.repo == nil {
		return
	}
	if s.encryptor == nil {
		s.finishBackgroundReviewWithError(ctx, cfg, item, "context decrypt unavailable")
		return
	}
	plain, err := s.encryptor.Decrypt(item.EncryptedContext)
	if err != nil {
		s.finishBackgroundReviewWithError(ctx, cfg, item, err.Error())
		return
	}
	_, reviewInput, err := parseContentModerationNormalizedContext(plain)
	if err != nil {
		s.finishBackgroundReviewWithError(ctx, cfg, item, err.Error())
		return
	}
	if reviewInput.IsEmpty() {
		now := time.Now()
		_ = s.repo.UpdateContextReview(ctx, ContentModerationContextReviewUpdate{
			ID:              item.ID,
			Status:          ContentModerationContextStatusSkipped,
			ReviewedAt:      &now,
			LastReviewError: "empty user message context",
		})
		return
	}
	checkInput := ContentModerationCheckInput{
		RequestID:  item.RequestID,
		UserEmail:  item.UserEmail,
		APIKeyName: item.APIKeyName,
		GroupID:    cloneInt64Ptr(item.GroupID),
		GroupName:  item.GroupName,
		Endpoint:   item.Endpoint,
		Provider:   item.Provider,
		Model:      item.Model,
		Protocol:   item.Protocol,
	}
	if item.UserID != nil {
		checkInput.UserID = *item.UserID
	}
	if item.APIKeyID != nil {
		checkInput.APIKeyID = *item.APIKeyID
	}
	if len(cfg.enabledAuditModels()) == 0 && len(cfg.apiKeys()) == 0 {
		s.finishBackgroundReviewWithError(ctx, cfg, item, "no background review audit backend configured")
		return
	}
	snapshot := s.riskSnapshotForUser(ctx, cfg, checkInput.UserID)
	contextID := item.ID
	var decision *ContentModerationDecision
	queueDelay := 0
	if len(cfg.enabledAuditModels()) > 0 {
		decision = s.checkModelAuditSync(ctx, checkInput, cfg, reviewInput, item.InputHash, nil, &queueDelay, false, &contextID, snapshot, ContentModerationRiskEventSourceBackgroundReview, ContentModerationReviewStageBackground)
	} else {
		decision = s.checkSync(ctx, checkInput, cfg, reviewInput, item.InputHash, &queueDelay, false, &contextID, snapshot, ContentModerationRiskEventSourceBackgroundReview, ContentModerationReviewStageBackground)
	}
	now := time.Now()
	status := ContentModerationContextStatusReviewed
	var logID *int64
	flagged := false
	if decision != nil {
		flagged = decision.Flagged
		if decision.LogID > 0 {
			logID = &decision.LogID
		}
	}
	_ = s.repo.UpdateContextReview(ctx, ContentModerationContextReviewUpdate{
		ID:                item.ID,
		Status:            status,
		ReviewedAt:        &now,
		LastReviewLogID:   logID,
		LastReviewFlagged: flagged,
		LastReviewError:   "",
	})
	s.lastBackgroundReviewUnix.Store(now.Unix())
}

func (s *ContentModerationService) finishBackgroundReviewWithError(ctx context.Context, cfg *ContentModerationConfig, item ContentModerationContext, message string) {
	if s == nil || s.repo == nil {
		return
	}
	message = trimRunes(strings.TrimSpace(message), 500)
	status := ContentModerationContextStatusPending
	var next *time.Time
	maxAttempts := item.MaxReviewAttempts
	if maxAttempts <= 0 {
		maxAttempts = cfg.BackgroundReviewMaxAttempts
	}
	if maxAttempts <= 0 {
		maxAttempts = defaultContentModerationReviewMaxAttempts
	}
	if item.ReviewAttempts >= maxAttempts {
		status = ContentModerationContextStatusFailed
	} else {
		backoff := time.Duration(cfg.BackgroundReviewRetryBackoffSeconds) * time.Second
		if backoff <= 0 {
			backoff = defaultContentModerationReviewBackoffSeconds * time.Second
		}
		t := time.Now().Add(backoff)
		next = &t
	}
	_ = s.repo.UpdateContextReview(ctx, ContentModerationContextReviewUpdate{
		ID:              item.ID,
		Status:          status,
		NextReviewAt:    next,
		LastReviewError: message,
	})
}
