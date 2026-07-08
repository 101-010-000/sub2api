package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/tidwall/gjson"
)

const (
	RequestRiskControlModeOff     = "off"
	RequestRiskControlModeObserve = "observe"
	RequestRiskControlModeEnforce = "enforce"

	RequestRiskActionObserve      = "observe"
	RequestRiskActionReject       = "reject"
	RequestRiskActionBanSession   = "ban_session"
	RequestRiskActionBanUserAgent = "ban_user_agent"

	RequestRiskUABanScopeAPIKey = "api_key"

	RequestRiskRuleExistingSessionBan         = "existing_session_ban"
	RequestRiskRuleExistingUserAgentBan       = "existing_user_agent_ban"
	RequestRiskRuleWindowsInferenceGeoCN      = "windows_inference_geo_cn"
	RequestRiskRuleWindowsDeniedTimezone      = "windows_denied_timezone"
	RequestRiskRuleHighChineseIntensity       = "high_chinese_intensity"
	RequestRiskRuleWindowsChineseRegionSignal = "windows_chinese_language_region_signal"

	defaultRequestRiskChineseHighThreshold = 0.45
	defaultRequestRiskEventRetentionDays   = 7
	defaultRequestRiskTTLSeconds           = 3600
)

var (
	requestRiskTimezonePattern = regexp.MustCompile(`(?is)<environment_context\b[^>]*>.*?<timezone>\s*([^<\s]+)\s*</timezone>.*?</environment_context>`)
	requestRiskUnicodeHanRange = &unicode.RangeTable{R16: []unicode.Range16{{Lo: 0x4e00, Hi: 0x9fff, Stride: 1}}}
)

type RequestRiskControlConfig struct {
	Enabled              bool     `json:"enabled"`
	Mode                 string   `json:"mode"`
	WindowsEnhanced      bool     `json:"windows_enhanced"`
	DeniedTimezones      []string `json:"denied_timezones"`
	ChineseHighThreshold float64  `json:"chinese_high_threshold"`
	EventRetentionDays   int      `json:"event_retention_days"`
	CaptureRawHeaders    bool     `json:"capture_raw_headers"`
	UABanScope           string   `json:"ua_ban_scope"`
	SessionBanTTLSeconds int      `json:"session_ban_ttl_seconds"`
	UABanTTLSeconds      int      `json:"ua_ban_ttl_seconds"`
}

type UpdateRequestRiskControlConfigInput struct {
	Enabled              *bool     `json:"enabled"`
	Mode                 *string   `json:"mode"`
	WindowsEnhanced      *bool     `json:"windows_enhanced"`
	DeniedTimezones      *[]string `json:"denied_timezones"`
	ChineseHighThreshold *float64  `json:"chinese_high_threshold"`
	EventRetentionDays   *int      `json:"event_retention_days"`
	CaptureRawHeaders    *bool     `json:"capture_raw_headers"`
	UABanScope           *string   `json:"ua_ban_scope"`
	SessionBanTTLSeconds *int      `json:"session_ban_ttl_seconds"`
	UABanTTLSeconds      *int      `json:"ua_ban_ttl_seconds"`
}

type RequestRiskEvaluationInput struct {
	RequestID       string
	UserID          int64
	APIKeyID        int64
	AccountID       *int64
	RequestPath     string
	Model           string
	Headers         http.Header
	Body            []byte
	CyberSessionKey string
	SessionBlocked  bool
	Protocol        string
}

type RequestRiskDecision struct {
	Allowed              bool
	Blocked              bool
	Action               string
	Message              string
	ErrorCode            string
	StatusCode           int
	SessionBanKey        string
	SessionBanTTLSeconds int
	UABan                *RequestRiskUserAgentBan
	Event                *RequestRiskEvent
	MatchedRules         []string
}

type RequestRiskEvent struct {
	ID               int64               `json:"id"`
	CreatedAt        time.Time           `json:"created_at"`
	ExpiresAt        time.Time           `json:"expires_at"`
	UserID           *int64              `json:"user_id,omitempty"`
	APIKeyID         *int64              `json:"api_key_id,omitempty"`
	AccountID        *int64              `json:"account_id,omitempty"`
	RequestID        string              `json:"request_id"`
	SessionID        string              `json:"session_id"`
	SessionIDHash    string              `json:"session_id_hash"`
	UserAgent        string              `json:"user_agent"`
	UserAgentHash    string              `json:"user_agent_hash"`
	InferenceGeo     string              `json:"inference_geo"`
	Timezone         string              `json:"timezone"`
	Platform         string              `json:"platform"`
	LanguageSignals  map[string]any      `json:"language_signals"`
	ChineseIntensity float64             `json:"chinese_intensity"`
	MatchedRules     []string            `json:"matched_rules"`
	Action           string              `json:"action"`
	ReasonCode       string              `json:"reason_code"`
	RawHeaders       map[string][]string `json:"raw_headers,omitempty"`
	RawHeadersJSON   string              `json:"raw_headers_json,omitempty"`
	XFooRaw          string              `json:"x_foo_raw"`
	RequestPath      string              `json:"request_path"`
	Model            string              `json:"model"`
}

type RequestRiskEventFilter struct {
	pagination.PaginationParams
	Action   string
	Query    string
	Rule     string
	APIKeyID *int64
	UserID   *int64
	From     *time.Time
	To       *time.Time
}

type RequestRiskUserAgentBan struct {
	APIKeyID      int64
	UserID        int64
	UserAgentHash string
	UserAgent     string
	Reason        string
	TriggeredAt   time.Time
	BannedUntil   time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type RequestRiskControlRepository interface {
	CreateRequestRiskEvent(ctx context.Context, event *RequestRiskEvent) error
	ListRequestRiskEvents(ctx context.Context, filter RequestRiskEventFilter) ([]RequestRiskEvent, *pagination.PaginationResult, error)
	GetRequestRiskEvent(ctx context.Context, id int64) (*RequestRiskEvent, error)
	CleanupExpiredRequestRiskEvents(ctx context.Context, now time.Time) (int64, error)
	UpsertRequestRiskUABan(ctx context.Context, ban *RequestRiskUserAgentBan) error
	GetActiveRequestRiskUABan(ctx context.Context, apiKeyID int64, userAgentHash string, now time.Time) (*RequestRiskUserAgentBan, error)
}

func (s *ContentModerationService) GetRequestRiskControlConfig(ctx context.Context) (*RequestRiskControlConfig, error) {
	cfg := defaultRequestRiskControlConfig()
	if s == nil || s.settingRepo == nil {
		return cfg, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyRequestRiskControlEnabled,
		SettingKeyRequestRiskControlMode,
		SettingKeyRequestRiskControlWindowsEnhanced,
		SettingKeyRequestRiskControlDeniedTimezones,
		SettingKeyRequestRiskControlChineseHighThreshold,
		SettingKeyRequestRiskControlEventRetentionDays,
		SettingKeyRequestRiskControlCaptureRawHeaders,
		SettingKeyRequestRiskControlUABanScope,
		SettingKeyRequestRiskControlSessionBanTTLSeconds,
		SettingKeyRequestRiskControlUABanTTLSeconds,
	})
	if err != nil {
		return nil, fmt.Errorf("get request risk config: %w", err)
	}
	if v, ok := values[SettingKeyRequestRiskControlEnabled]; ok {
		cfg.Enabled = strings.TrimSpace(v) == "true"
	}
	if v, ok := values[SettingKeyRequestRiskControlMode]; ok {
		cfg.Mode = strings.TrimSpace(v)
	}
	if v, ok := values[SettingKeyRequestRiskControlWindowsEnhanced]; ok {
		cfg.WindowsEnhanced = strings.TrimSpace(v) == "true"
	}
	if v, ok := values[SettingKeyRequestRiskControlDeniedTimezones]; ok && strings.TrimSpace(v) != "" {
		var zones []string
		if err := json.Unmarshal([]byte(v), &zones); err == nil {
			cfg.DeniedTimezones = zones
		}
	}
	if v, ok := values[SettingKeyRequestRiskControlChineseHighThreshold]; ok {
		if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			cfg.ChineseHighThreshold = n
		}
	}
	if v, ok := values[SettingKeyRequestRiskControlEventRetentionDays]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.EventRetentionDays = n
		}
	}
	if v, ok := values[SettingKeyRequestRiskControlCaptureRawHeaders]; ok {
		cfg.CaptureRawHeaders = strings.TrimSpace(v) == "true"
	}
	if v, ok := values[SettingKeyRequestRiskControlUABanScope]; ok {
		cfg.UABanScope = strings.TrimSpace(v)
	}
	if v, ok := values[SettingKeyRequestRiskControlSessionBanTTLSeconds]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.SessionBanTTLSeconds = n
		}
	}
	if v, ok := values[SettingKeyRequestRiskControlUABanTTLSeconds]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.UABanTTLSeconds = n
		}
	}
	cfg.normalize()
	return cfg, nil
}

func (s *ContentModerationService) UpdateRequestRiskControlConfig(ctx context.Context, input UpdateRequestRiskControlConfigInput) (*RequestRiskControlConfig, error) {
	cfg, err := s.GetRequestRiskControlConfig(ctx)
	if err != nil {
		return nil, err
	}
	if input.Enabled != nil {
		cfg.Enabled = *input.Enabled
	}
	if input.Mode != nil {
		cfg.Mode = *input.Mode
	}
	if input.WindowsEnhanced != nil {
		cfg.WindowsEnhanced = *input.WindowsEnhanced
	}
	if input.DeniedTimezones != nil {
		cfg.DeniedTimezones = append([]string(nil), (*input.DeniedTimezones)...)
	}
	if input.ChineseHighThreshold != nil {
		cfg.ChineseHighThreshold = *input.ChineseHighThreshold
	}
	if input.EventRetentionDays != nil {
		cfg.EventRetentionDays = *input.EventRetentionDays
	}
	if input.CaptureRawHeaders != nil {
		cfg.CaptureRawHeaders = *input.CaptureRawHeaders
	}
	if input.UABanScope != nil {
		cfg.UABanScope = *input.UABanScope
	}
	if input.SessionBanTTLSeconds != nil {
		cfg.SessionBanTTLSeconds = *input.SessionBanTTLSeconds
	}
	if input.UABanTTLSeconds != nil {
		cfg.UABanTTLSeconds = *input.UABanTTLSeconds
	}
	if err := validateRequestRiskControlConfig(cfg); err != nil {
		return nil, err
	}
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("setting repository not initialized")
	}
	deniedZones, err := json.Marshal(cfg.DeniedTimezones)
	if err != nil {
		return nil, err
	}
	updates := map[string]string{
		SettingKeyRequestRiskControlEnabled:              strconv.FormatBool(cfg.Enabled),
		SettingKeyRequestRiskControlMode:                 cfg.Mode,
		SettingKeyRequestRiskControlWindowsEnhanced:      strconv.FormatBool(cfg.WindowsEnhanced),
		SettingKeyRequestRiskControlDeniedTimezones:      string(deniedZones),
		SettingKeyRequestRiskControlChineseHighThreshold: strconv.FormatFloat(cfg.ChineseHighThreshold, 'f', -1, 64),
		SettingKeyRequestRiskControlEventRetentionDays:   strconv.Itoa(cfg.EventRetentionDays),
		SettingKeyRequestRiskControlCaptureRawHeaders:    strconv.FormatBool(cfg.CaptureRawHeaders),
		SettingKeyRequestRiskControlUABanScope:           cfg.UABanScope,
		SettingKeyRequestRiskControlSessionBanTTLSeconds: strconv.Itoa(cfg.SessionBanTTLSeconds),
		SettingKeyRequestRiskControlUABanTTLSeconds:      strconv.Itoa(cfg.UABanTTLSeconds),
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *ContentModerationService) EvaluateRequestRisk(ctx context.Context, input RequestRiskEvaluationInput) (*RequestRiskDecision, error) {
	allow := &RequestRiskDecision{Allowed: true, Action: RequestRiskActionObserve}
	if s == nil || s.settingRepo == nil {
		return allow, nil
	}
	cfg, err := s.GetRequestRiskControlConfig(ctx)
	if err != nil {
		return allow, err
	}
	if !cfg.Enabled || cfg.Mode == RequestRiskControlModeOff {
		return allow, nil
	}

	repo := s.requestRiskRepo()
	now := time.Now().UTC()
	signals := buildRequestRiskSignals(input, cfg)
	decision := &RequestRiskDecision{Allowed: true, Action: RequestRiskActionObserve}
	event := signals.toEvent(input, cfg, now)

	if repo != nil && input.APIKeyID > 0 && event.UserAgentHash != "" {
		ban, err := repo.GetActiveRequestRiskUABan(ctx, input.APIKeyID, event.UserAgentHash, now)
		if err != nil {
			return allow, err
		}
		if ban != nil {
			addRequestRiskMatch(event, RequestRiskRuleExistingUserAgentBan)
			event.Action = RequestRiskActionReject
			event.ReasonCode = RequestRiskRuleExistingUserAgentBan
		}
	}
	if input.SessionBlocked {
		addRequestRiskMatch(event, RequestRiskRuleExistingSessionBan)
		event.Action = strongestRequestRiskAction(event.Action, RequestRiskActionReject)
		event.ReasonCode = RequestRiskRuleExistingSessionBan
	}

	applyRequestRiskRules(cfg, signals, event)
	action := strongestRequestRiskAction(event.Action, decision.Action)
	event.Action = action
	if event.ReasonCode == "" && len(event.MatchedRules) > 0 {
		event.ReasonCode = event.MatchedRules[0]
	}

	if repo != nil && event.Action != "" && len(event.MatchedRules) > 0 {
		if err := repo.CreateRequestRiskEvent(ctx, event); err != nil {
			return allow, err
		}
	}

	if cfg.Mode != RequestRiskControlModeEnforce || len(event.MatchedRules) == 0 {
		decision.Event = event
		decision.MatchedRules = append([]string(nil), event.MatchedRules...)
		return decision, nil
	}

	switch event.Action {
	case RequestRiskActionBanSession:
		decision.Action = RequestRiskActionBanSession
		decision.Blocked = true
		decision.Allowed = false
		decision.SessionBanKey = input.CyberSessionKey
		decision.SessionBanTTLSeconds = cfg.SessionBanTTLSeconds
	case RequestRiskActionBanUserAgent:
		decision.Action = RequestRiskActionBanUserAgent
		decision.Blocked = true
		decision.Allowed = false
		decision.UABan = requestRiskUABanFromEvent(input, event, cfg, now)
		if repo != nil && decision.UABan != nil {
			if err := repo.UpsertRequestRiskUABan(ctx, decision.UABan); err != nil {
				return allow, err
			}
		}
	case RequestRiskActionReject:
		decision.Action = RequestRiskActionReject
		decision.Blocked = true
		decision.Allowed = false
	default:
		decision.Action = RequestRiskActionObserve
	}
	if decision.Blocked {
		code, message := s.requestRiskCyberuseError(ctx, input.RequestID, input.UserID)
		decision.ErrorCode = code
		decision.Message = message
		decision.StatusCode = http.StatusForbidden
	}
	decision.Event = event
	decision.MatchedRules = append([]string(nil), event.MatchedRules...)
	return decision, nil
}

func (s *ContentModerationService) ListRequestRiskEvents(ctx context.Context, filter RequestRiskEventFilter) ([]RequestRiskEvent, *pagination.PaginationResult, error) {
	repo := s.requestRiskRepo()
	if repo == nil {
		return []RequestRiskEvent{}, paginationResultFromEmpty(filter.PaginationParams), nil
	}
	return repo.ListRequestRiskEvents(ctx, filter)
}

func (s *ContentModerationService) GetRequestRiskEvent(ctx context.Context, id int64) (*RequestRiskEvent, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_REQUEST_RISK_EVENT_ID", "invalid request risk event id")
	}
	repo := s.requestRiskRepo()
	if repo == nil {
		return nil, infraerrors.NotFound("REQUEST_RISK_EVENT_NOT_FOUND", "request risk event not found")
	}
	return repo.GetRequestRiskEvent(ctx, id)
}

func (s *ContentModerationService) cleanupRequestRiskEvents(ctx context.Context, now time.Time) int64 {
	repo := s.requestRiskRepo()
	if repo == nil {
		return 0
	}
	deleted, err := repo.CleanupExpiredRequestRiskEvents(ctx, now)
	if err != nil {
		return 0
	}
	return deleted
}

func (s *ContentModerationService) requestRiskRepo() RequestRiskControlRepository {
	if s == nil || s.repo == nil {
		return nil
	}
	repo, ok := s.repo.(RequestRiskControlRepository)
	if !ok {
		return nil
	}
	return repo
}

func (s *ContentModerationService) requestRiskCyberuseError(ctx context.Context, requestID string, userID int64) (string, string) {
	cfg, err := s.loadConfig(ctx)
	if err != nil || cfg == nil || !cfg.cyberuseResponseApplies(userID) || !cfg.CyberuseResponse.EmitToClient {
		return defaultContentModerationCyberuseErrorCode, defaultContentModerationCyberuseMessage
	}
	code := strings.TrimSpace(cfg.CyberuseResponse.ErrorCode)
	if code == "" {
		code = defaultContentModerationCyberuseErrorCode
	}
	message := strings.TrimSpace(cfg.CyberuseResponse.Message)
	if message == "" {
		message = defaultContentModerationCyberuseMessage
	}
	if cfg.CyberuseResponse.IncludeRequestID && strings.TrimSpace(requestID) != "" {
		message = fmt.Sprintf("%s (request_id: %s)", message, strings.TrimSpace(requestID))
	}
	return code, message
}

func defaultRequestRiskControlConfig() *RequestRiskControlConfig {
	cfg := &RequestRiskControlConfig{
		Enabled:              false,
		Mode:                 RequestRiskControlModeOff,
		WindowsEnhanced:      true,
		DeniedTimezones:      []string{"Asia/Shanghai", "Asia/Urumqi"},
		ChineseHighThreshold: defaultRequestRiskChineseHighThreshold,
		EventRetentionDays:   defaultRequestRiskEventRetentionDays,
		CaptureRawHeaders:    true,
		UABanScope:           RequestRiskUABanScopeAPIKey,
		SessionBanTTLSeconds: defaultRequestRiskTTLSeconds,
		UABanTTLSeconds:      defaultRequestRiskTTLSeconds,
	}
	cfg.normalize()
	return cfg
}

func validateRequestRiskControlConfig(cfg *RequestRiskControlConfig) error {
	if cfg == nil {
		return infraerrors.BadRequest("INVALID_REQUEST_RISK_CONFIG", "request risk config is required")
	}
	cfg.normalize()
	switch cfg.Mode {
	case RequestRiskControlModeOff, RequestRiskControlModeObserve, RequestRiskControlModeEnforce:
	default:
		return infraerrors.BadRequest("INVALID_REQUEST_RISK_MODE", "request risk mode is invalid")
	}
	if cfg.ChineseHighThreshold <= 0 || cfg.ChineseHighThreshold > 1 {
		return infraerrors.BadRequest("INVALID_REQUEST_RISK_CHINESE_THRESHOLD", "chinese high threshold must be in (0, 1]")
	}
	if cfg.EventRetentionDays <= 0 || cfg.EventRetentionDays > 30 {
		return infraerrors.BadRequest("INVALID_REQUEST_RISK_RETENTION", "event retention days must be 1-30")
	}
	if cfg.SessionBanTTLSeconds <= 0 || cfg.UABanTTLSeconds <= 0 {
		return infraerrors.BadRequest("INVALID_REQUEST_RISK_TTL", "ban ttl seconds must be positive")
	}
	return nil
}

func (cfg *RequestRiskControlConfig) normalize() {
	if cfg == nil {
		return
	}
	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	if cfg.Mode == "" {
		cfg.Mode = RequestRiskControlModeOff
	}
	cleanZones := make([]string, 0, len(cfg.DeniedTimezones))
	seen := make(map[string]struct{}, len(cfg.DeniedTimezones))
	for _, zone := range cfg.DeniedTimezones {
		zone = strings.TrimSpace(zone)
		if zone == "" {
			continue
		}
		if _, ok := seen[zone]; ok {
			continue
		}
		seen[zone] = struct{}{}
		cleanZones = append(cleanZones, zone)
	}
	if len(cleanZones) == 0 {
		cleanZones = []string{"Asia/Shanghai", "Asia/Urumqi"}
	}
	cfg.DeniedTimezones = cleanZones
	if cfg.ChineseHighThreshold <= 0 || cfg.ChineseHighThreshold > 1 {
		cfg.ChineseHighThreshold = defaultRequestRiskChineseHighThreshold
	}
	if cfg.EventRetentionDays <= 0 {
		cfg.EventRetentionDays = defaultRequestRiskEventRetentionDays
	}
	if cfg.SessionBanTTLSeconds <= 0 {
		cfg.SessionBanTTLSeconds = defaultRequestRiskTTLSeconds
	}
	if cfg.UABanTTLSeconds <= 0 {
		cfg.UABanTTLSeconds = defaultRequestRiskTTLSeconds
	}
	cfg.UABanScope = strings.ToLower(strings.TrimSpace(cfg.UABanScope))
	if cfg.UABanScope == "" {
		cfg.UABanScope = RequestRiskUABanScopeAPIKey
	}
}

type requestRiskSignals struct {
	SessionID        string
	UserAgent        string
	InferenceGeo     string
	Timezone         string
	Platform         string
	AcceptLanguage   string
	XFooRaw          string
	ChineseIntensity float64
	LanguageSignals  map[string]any
	RawHeaders       map[string][]string
}

func buildRequestRiskSignals(input RequestRiskEvaluationInput, cfg *RequestRiskControlConfig) requestRiskSignals {
	headers := cloneRequestRiskHeaders(input.Headers)
	xFooRaw := firstHeaderValue(headers, "X-Foo")
	xFooValues := ParseRequestRiskXFoo(xFooRaw)
	ua := firstNonEmpty(
		firstHeaderValue(headers, "User-Agent"),
		xFooValues["user-agent"],
		xFooValues["user_agent"],
		xFooValues["ua"],
	)
	acceptLanguage := firstNonEmpty(
		firstHeaderValue(headers, "Accept-Language"),
		xFooValues["accept-language"],
		xFooValues["accept_language"],
	)
	inferenceGeo := firstNonEmpty(
		firstHeaderValue(headers, "inference_geo"),
		firstHeaderValue(headers, "Inference-Geo"),
		firstHeaderValue(headers, "X-Inference-Geo"),
		xFooValues["inference_geo"],
		xFooValues["geo"],
	)
	timezone := firstNonEmpty(
		firstHeaderValue(headers, "timezone"),
		firstHeaderValue(headers, "Time-Zone"),
		firstHeaderValue(headers, "X-Timezone"),
		xFooValues["timezone"],
		ExtractRequestRiskTimezoneFromResponsesBody(input.Body),
	)
	text := ExtractRequestRiskTextFromResponsesBody(input.Body)
	intensity := RequestRiskChineseIntensity(requestRiskLanguageText(text))
	languageSignals := map[string]any{
		"accept_language":       acceptLanguage,
		"accept_language_zh":    requestRiskAcceptLanguageHasChinese(acceptLanguage),
		"accept_language_zh_cn": requestRiskAcceptLanguageHasMainlandChinese(acceptLanguage),
		"x_foo":                 xFooValues,
	}
	return requestRiskSignals{
		SessionID:        ExtractRequestRiskSessionID(headers, input.Body),
		UserAgent:        ua,
		InferenceGeo:     strings.ToUpper(strings.TrimSpace(inferenceGeo)),
		Timezone:         strings.TrimSpace(timezone),
		Platform:         RequestRiskClientPlatform(ua),
		AcceptLanguage:   acceptLanguage,
		XFooRaw:          xFooRaw,
		ChineseIntensity: intensity,
		LanguageSignals:  languageSignals,
		RawHeaders:       headers,
	}
}

func (s requestRiskSignals) toEvent(input RequestRiskEvaluationInput, cfg *RequestRiskControlConfig, now time.Time) *RequestRiskEvent {
	expiresAt := now.AddDate(0, 0, defaultRequestRiskEventRetentionDays)
	if cfg != nil && cfg.EventRetentionDays > 0 {
		expiresAt = now.AddDate(0, 0, cfg.EventRetentionDays)
	}
	var rawHeaders map[string][]string
	var rawHeadersJSON string
	if cfg == nil || cfg.CaptureRawHeaders {
		rawHeaders = s.RawHeaders
		if b, err := json.Marshal(rawHeaders); err == nil {
			rawHeadersJSON = string(b)
		}
	}
	var userID *int64
	if input.UserID > 0 {
		v := input.UserID
		userID = &v
	}
	var apiKeyID *int64
	if input.APIKeyID > 0 {
		v := input.APIKeyID
		apiKeyID = &v
	}
	return &RequestRiskEvent{
		CreatedAt:        now,
		ExpiresAt:        expiresAt,
		UserID:           userID,
		APIKeyID:         apiKeyID,
		AccountID:        input.AccountID,
		RequestID:        strings.TrimSpace(input.RequestID),
		SessionID:        s.SessionID,
		SessionIDHash:    requestRiskHash(s.SessionID),
		UserAgent:        s.UserAgent,
		UserAgentHash:    requestRiskHash(s.UserAgent),
		InferenceGeo:     s.InferenceGeo,
		Timezone:         s.Timezone,
		Platform:         s.Platform,
		LanguageSignals:  s.LanguageSignals,
		ChineseIntensity: s.ChineseIntensity,
		MatchedRules:     []string{},
		Action:           RequestRiskActionObserve,
		RawHeaders:       rawHeaders,
		RawHeadersJSON:   rawHeadersJSON,
		XFooRaw:          s.XFooRaw,
		RequestPath:      strings.TrimSpace(input.RequestPath),
		Model:            strings.TrimSpace(input.Model),
	}
}

func applyRequestRiskRules(cfg *RequestRiskControlConfig, signals requestRiskSignals, event *RequestRiskEvent) {
	if cfg == nil || event == nil {
		return
	}
	windows := signals.Platform == "windows"
	deniedTimezone := requestRiskTimezoneDenied(signals.Timezone, cfg.DeniedTimezones)
	strongRegion := strings.EqualFold(signals.InferenceGeo, "CN") || deniedTimezone
	if cfg.WindowsEnhanced && windows && strings.EqualFold(signals.InferenceGeo, "CN") {
		addRequestRiskMatch(event, RequestRiskRuleWindowsInferenceGeoCN)
		event.Action = strongestRequestRiskAction(event.Action, RequestRiskActionBanUserAgent)
	}
	if cfg.WindowsEnhanced && windows && deniedTimezone {
		addRequestRiskMatch(event, RequestRiskRuleWindowsDeniedTimezone)
		event.Action = strongestRequestRiskAction(event.Action, RequestRiskActionReject)
	}
	if signals.ChineseIntensity >= cfg.ChineseHighThreshold && signals.ChineseIntensity > 0 {
		addRequestRiskMatch(event, RequestRiskRuleHighChineseIntensity)
		event.Action = strongestRequestRiskAction(event.Action, RequestRiskActionBanSession)
	}
	if cfg.WindowsEnhanced && windows && requestRiskAcceptLanguageHasMainlandChinese(signals.AcceptLanguage) && strongRegion {
		addRequestRiskMatch(event, RequestRiskRuleWindowsChineseRegionSignal)
		event.Action = strongestRequestRiskAction(event.Action, RequestRiskActionReject)
	}
}

func addRequestRiskMatch(event *RequestRiskEvent, rule string) {
	if event == nil || strings.TrimSpace(rule) == "" {
		return
	}
	for _, existing := range event.MatchedRules {
		if existing == rule {
			return
		}
	}
	event.MatchedRules = append(event.MatchedRules, rule)
}

func strongestRequestRiskAction(a, b string) string {
	rank := map[string]int{
		"":                            0,
		RequestRiskActionObserve:      1,
		RequestRiskActionReject:       2,
		RequestRiskActionBanUserAgent: 3,
		RequestRiskActionBanSession:   4,
	}
	if rank[b] > rank[a] {
		return b
	}
	if a == "" {
		return RequestRiskActionObserve
	}
	return a
}

func requestRiskUABanFromEvent(input RequestRiskEvaluationInput, event *RequestRiskEvent, cfg *RequestRiskControlConfig, now time.Time) *RequestRiskUserAgentBan {
	if event == nil || input.APIKeyID <= 0 || event.UserAgentHash == "" {
		return nil
	}
	ttl := defaultRequestRiskTTLSeconds
	if cfg != nil && cfg.UABanTTLSeconds > 0 {
		ttl = cfg.UABanTTLSeconds
	}
	return &RequestRiskUserAgentBan{
		APIKeyID:      input.APIKeyID,
		UserID:        input.UserID,
		UserAgentHash: event.UserAgentHash,
		UserAgent:     event.UserAgent,
		Reason:        event.ReasonCode,
		TriggeredAt:   now,
		BannedUntil:   now.Add(time.Duration(ttl) * time.Second),
	}
}

func RequestRiskClientPlatform(userAgent string) string {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	switch {
	case strings.Contains(ua, "windows nt") || strings.Contains(ua, "win64") || strings.Contains(ua, "wow64"):
		return "windows"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os x") || strings.Contains(ua, "darwin"):
		return "macos"
	case strings.Contains(ua, "linux") || strings.Contains(ua, "x11"):
		return "linux"
	default:
		return "unknown"
	}
}

func ExtractRequestRiskSessionID(headers http.Header, body []byte) string {
	if v := firstHeaderValue(headers, "session_id"); v != "" {
		return v
	}
	if v := firstHeaderValue(headers, "conversation_id"); v != "" {
		return v
	}
	if gjson.ValidBytes(body) {
		return strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String())
	}
	return ""
}

func ExtractRequestRiskTimezoneFromResponsesBody(body []byte) string {
	text := ExtractRequestRiskTextFromResponsesBody(body)
	if text == "" {
		return ""
	}
	matches := requestRiskTimezonePattern.FindStringSubmatch(text)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func ExtractRequestRiskTextFromResponsesBody(body []byte) string {
	if !gjson.ValidBytes(body) {
		return ""
	}
	var parts []string
	gjson.GetBytes(body, "input").ForEach(func(_, item gjson.Result) bool {
		if item.Get("content").IsArray() {
			item.Get("content").ForEach(func(_, content gjson.Result) bool {
				if v := strings.TrimSpace(content.Get("text").String()); v != "" {
					parts = append(parts, v)
				}
				return true
			})
			return true
		}
		if v := strings.TrimSpace(item.Get("text").String()); v != "" {
			parts = append(parts, v)
		}
		return true
	})
	if v := strings.TrimSpace(gjson.GetBytes(body, "instructions").String()); v != "" {
		parts = append(parts, v)
	}
	return strings.Join(parts, "\n")
}

func ParseRequestRiskXFoo(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}
	}
	out := make(map[string]string)
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		for k, v := range obj {
			out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(fmt.Sprint(v))
		}
		return out
	}
	if values, err := url.ParseQuery(raw); err == nil && len(values) > 0 {
		for k, vals := range values {
			out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(strings.Join(vals, ","))
		}
		return out
	}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ';' || r == ',' || r == '\n' }) {
		if k, v, ok := strings.Cut(part, "="); ok {
			out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
	}
	return out
}

func RequestRiskChineseIntensity(text string) float64 {
	var chinese, letters int
	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.IsDigit(r) {
			continue
		}
		if unicode.Is(requestRiskUnicodeHanRange, r) {
			chinese++
			letters++
			continue
		}
		if unicode.IsLetter(r) {
			letters++
		}
	}
	if letters == 0 {
		return 0
	}
	return float64(chinese) / float64(letters)
}

func requestRiskLanguageText(text string) string {
	return strings.TrimSpace(requestRiskTimezonePattern.ReplaceAllString(text, "\n"))
}

func requestRiskAcceptLanguageHasChinese(value string) bool {
	return strings.Contains(strings.ToLower(value), "zh")
}

func requestRiskAcceptLanguageHasMainlandChinese(value string) bool {
	lang := strings.ToLower(value)
	return strings.Contains(lang, "zh-cn") || strings.Contains(lang, "zh-hans")
}

func requestRiskTimezoneDenied(zone string, denied []string) bool {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return false
	}
	for _, item := range denied {
		if strings.EqualFold(strings.TrimSpace(item), zone) {
			return true
		}
	}
	return false
}

func requestRiskHash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func cloneRequestRiskHeaders(headers http.Header) map[string][]string {
	out := make(map[string][]string, len(headers))
	for k, values := range headers {
		copied := append([]string(nil), values...)
		out[http.CanonicalHeaderKey(k)] = copied
	}
	return out
}

func firstHeaderValue(headers map[string][]string, key string) string {
	if len(headers) == 0 {
		return ""
	}
	values := headers[http.CanonicalHeaderKey(key)]
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func paginationResultFromEmpty(params pagination.PaginationParams) *pagination.PaginationResult {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	return &pagination.PaginationResult{Total: 0, Page: params.Page, PageSize: params.Limit(), Pages: 0}
}
