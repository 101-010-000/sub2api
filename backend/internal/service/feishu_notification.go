package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/imroc/req/v3"
	"github.com/tidwall/gjson"
)

const (
	FeishuIdentityPurposeNotify = "notify"
	FeishuIdentityPurposePanel  = "panel"

	defaultFeishuNotifyTokenURL   = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	defaultFeishuNotifyMessageURL = "https://open.feishu.cn/open-apis/im/v1/messages"
	defaultFeishuPanelPath        = "/feishu/panel"
)

var (
	ErrFeishuNotificationNotBound = infraerrors.NotFound("FEISHU_NOTIFICATION_NOT_BOUND", "feishu notification is not bound")
	ErrFeishuNotificationConflict = infraerrors.Conflict("FEISHU_NOTIFICATION_CONFLICT", "feishu identity is already bound to another user")
	ErrFeishuNotificationDisabled = infraerrors.Forbidden("FEISHU_NOTIFICATION_DISABLED", "feishu notification is disabled")
)

type FeishuNotificationConfig struct {
	Enabled    bool
	AppID      string
	AppSecret  string
	TokenURL   string
	MessageURL string
	PanelURL   string
}

type FeishuUserIdentityBinding struct {
	UserID              int64
	AppID               string
	TenantKey           string
	OpenID              string
	UnionID             string
	Purpose             string
	NotificationEnabled bool
	Metadata            map[string]any
	BoundAt             time.Time
	LastSeenAt          time.Time
}

type UpsertFeishuUserIdentityBindingInput struct {
	UserID              int64
	AppID               string
	TenantKey           string
	OpenID              string
	UnionID             string
	Purpose             string
	NotificationEnabled bool
	Metadata            map[string]any
}

type FeishuUserIdentityRepository interface {
	UpsertFeishuUserIdentityBinding(ctx context.Context, input UpsertFeishuUserIdentityBindingInput) (*FeishuUserIdentityBinding, error)
	GetFeishuNotificationBinding(ctx context.Context, userID int64, appID string) (*FeishuUserIdentityBinding, error)
	GetFeishuBindingByUnionID(ctx context.Context, appID, tenantKey, unionID, purpose string) (*FeishuUserIdentityBinding, error)
	ListFeishuBindingsByUser(ctx context.Context, userID int64) ([]FeishuUserIdentityBinding, error)
	SetFeishuNotificationEnabled(ctx context.Context, userID int64, appID string, enabled bool) (*FeishuUserIdentityBinding, error)
	DeleteFeishuNotificationBinding(ctx context.Context, userID int64, appID string) error
}

type FeishuNotificationStatus struct {
	Bound               bool   `json:"bound"`
	Enabled             bool   `json:"enabled"`
	AppID               string `json:"app_id,omitempty"`
	TenantKey           string `json:"tenant_key,omitempty"`
	UnionIDHint         string `json:"union_id_hint,omitempty"`
	OpenIDHint          string `json:"open_id_hint,omitempty"`
	BindStartPath       string `json:"bind_start_path,omitempty"`
	PanelURL            string `json:"panel_url,omitempty"`
	CanOpenPanel        bool   `json:"can_open_panel"`
	NotificationEnabled bool   `json:"notification_enabled"`
}

type FeishuBalanceLowNotification struct {
	UserID      int64
	UserName    string
	UserEmail   string
	Balance     float64
	Threshold   float64
	SiteName    string
	RechargeURL string
}

type FeishuSubscriptionExpiryNotification struct {
	UserID            int64
	SubscriptionID    int64
	RecipientName     string
	GroupName         string
	ExpiresAt         time.Time
	DaysRemaining     int
	SourceReminderKey string
}

type FeishuContentModerationBanNotification struct {
	UserID         int64
	UserName       string
	UserEmail      string
	GroupName      string
	Category       string
	Score          float64
	ViolationCount int
	BanThreshold   int
	BanDurationMin int
}

type FeishuContentModerationViolationNotification struct {
	UserID         int64
	UserName       string
	UserEmail      string
	GroupName      string
	Category       string
	Score          float64
	ViolationCount int
	BanThreshold   int
}

type FeishuNotificationService struct {
	settingRepo SettingRepository
	bindingRepo FeishuUserIdentityRepository
}

func NewFeishuNotificationService(settingRepo SettingRepository, bindingRepo FeishuUserIdentityRepository) *FeishuNotificationService {
	return &FeishuNotificationService{settingRepo: settingRepo, bindingRepo: bindingRepo}
}

func (s *FeishuNotificationService) GetConfig(ctx context.Context) (FeishuNotificationConfig, error) {
	cfg := FeishuNotificationConfig{
		TokenURL:   defaultFeishuNotifyTokenURL,
		MessageURL: defaultFeishuNotifyMessageURL,
		PanelURL:   defaultFeishuPanelPath,
	}
	if s == nil || s.settingRepo == nil {
		return cfg, nil
	}
	settings, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyFeishuNotifyEnabled,
		SettingKeyFeishuNotifyAppID,
		SettingKeyFeishuNotifyAppSecret,
		SettingKeyFeishuNotifyTokenURL,
		SettingKeyFeishuNotifyMessageURL,
		SettingKeyFeishuNotifyPanelURL,
	})
	if err != nil {
		return cfg, err
	}
	cfg.Enabled = strings.TrimSpace(settings[SettingKeyFeishuNotifyEnabled]) == "true"
	cfg.AppID = strings.TrimSpace(settings[SettingKeyFeishuNotifyAppID])
	cfg.AppSecret = strings.TrimSpace(settings[SettingKeyFeishuNotifyAppSecret])
	cfg.TokenURL = firstNonEmpty(settings[SettingKeyFeishuNotifyTokenURL], defaultFeishuNotifyTokenURL)
	cfg.MessageURL = firstNonEmpty(settings[SettingKeyFeishuNotifyMessageURL], defaultFeishuNotifyMessageURL)
	cfg.PanelURL = firstNonEmpty(settings[SettingKeyFeishuNotifyPanelURL], defaultFeishuPanelPath)
	return cfg, nil
}

func (s *FeishuNotificationService) GetStatus(ctx context.Context, userID int64) (FeishuNotificationStatus, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return FeishuNotificationStatus{}, err
	}
	status := FeishuNotificationStatus{
		AppID:         cfg.AppID,
		PanelURL:      cfg.PanelURL,
		CanOpenPanel:  cfg.Enabled && cfg.AppID != "" && cfg.PanelURL != "",
		BindStartPath: "/api/v1/auth/oauth/feishu/notify/bind/start",
	}
	if s == nil || s.bindingRepo == nil || userID <= 0 || cfg.AppID == "" {
		return status, nil
	}
	binding, err := s.bindingRepo.GetFeishuNotificationBinding(ctx, userID, cfg.AppID)
	if err != nil {
		if infraerrors.Code(err) == infraerrors.Code(ErrFeishuNotificationNotBound) {
			return status, nil
		}
		return status, err
	}
	status.Bound = true
	status.Enabled = binding.NotificationEnabled
	status.NotificationEnabled = binding.NotificationEnabled
	status.TenantKey = binding.TenantKey
	status.UnionIDHint = maskOpaqueIdentity(binding.UnionID)
	status.OpenIDHint = maskOpaqueIdentity(binding.OpenID)
	return status, nil
}

func (s *FeishuNotificationService) SetEnabled(ctx context.Context, userID int64, enabled bool) (FeishuNotificationStatus, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return FeishuNotificationStatus{}, err
	}
	if cfg.AppID == "" || s == nil || s.bindingRepo == nil {
		return FeishuNotificationStatus{}, ErrFeishuNotificationNotBound
	}
	if _, err := s.bindingRepo.SetFeishuNotificationEnabled(ctx, userID, cfg.AppID, enabled); err != nil {
		return FeishuNotificationStatus{}, err
	}
	return s.GetStatus(ctx, userID)
}

func (s *FeishuNotificationService) UpsertNotifyBinding(ctx context.Context, input UpsertFeishuUserIdentityBindingInput) (*FeishuUserIdentityBinding, error) {
	if s == nil || s.bindingRepo == nil {
		return nil, ErrFeishuNotificationDisabled
	}
	input.Purpose = FeishuIdentityPurposeNotify
	input.NotificationEnabled = true
	return s.bindingRepo.UpsertFeishuUserIdentityBinding(ctx, input)
}

func (s *FeishuNotificationService) SendBalanceLow(ctx context.Context, input FeishuBalanceLowNotification) error {
	displayName := firstNonEmpty(input.UserName, input.UserEmail, "用户")
	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title":    map[string]any{"tag": "plain_text", "content": "余额不足提醒"},
			"template": "orange",
		},
		"elements": []any{
			map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**%s**，您的账户余额已低于提醒阈值。", displayName)}},
			map[string]any{"tag": "div", "fields": []any{
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**当前余额**\n$%.2f", input.Balance)}},
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**提醒阈值**\n$%.2f", input.Threshold)}},
			}},
			s.feishuPanelActionElement(ctx, "打开面板", input.RechargeURL),
		},
	}
	return s.sendInteractiveCard(ctx, input.UserID, card)
}

func (s *FeishuNotificationService) SendSubscriptionExpiryReminder(ctx context.Context, input FeishuSubscriptionExpiryNotification) error {
	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title":    map[string]any{"tag": "plain_text", "content": "订阅到期提醒"},
			"template": "yellow",
		},
		"elements": []any{
			map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("您的订阅 **%s** 将在 **%d 天后** 到期。", input.GroupName, input.DaysRemaining)}},
			map[string]any{"tag": "div", "fields": []any{
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": "**订阅**\n" + input.GroupName}},
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": "**到期时间**\n" + input.ExpiresAt.Format("2006-01-02 15:04")}},
			}},
			s.feishuPanelActionElement(ctx, "查看面板", ""),
		},
	}
	return s.sendInteractiveCard(ctx, input.UserID, card)
}

func (s *FeishuNotificationService) SendContentModerationViolation(ctx context.Context, input FeishuContentModerationViolationNotification) error {
	displayName := firstNonEmpty(input.UserName, input.UserEmail, "用户")
	groupName := firstNonEmpty(input.GroupName, "-")
	category := firstNonEmpty(input.Category, "-")
	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title":    map[string]any{"tag": "plain_text", "content": "账户风控提醒"},
			"template": "orange",
		},
		"elements": []any{
			map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**%s**，您的 API 请求触发了内容风控规则。", displayName)}},
			map[string]any{"tag": "div", "fields": []any{
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**命中分组**\n%s", groupName)}},
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**风险类别**\n%s", category)}},
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**最高分数**\n%.3f", input.Score)}},
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**触发次数**\n%d / %d", input.ViolationCount, input.BanThreshold)}},
			}},
			s.feishuPanelActionElement(ctx, "查看账户", ""),
		},
	}
	return s.sendInteractiveCard(ctx, input.UserID, card)
}

func (s *FeishuNotificationService) SendContentModerationBan(ctx context.Context, input FeishuContentModerationBanNotification) error {
	displayName := firstNonEmpty(input.UserName, input.UserEmail, "用户")
	groupName := firstNonEmpty(input.GroupName, "-")
	category := firstNonEmpty(input.Category, "-")
	banDuration := "-"
	if input.BanDurationMin > 0 {
		banDuration = fmt.Sprintf("%d 分钟", input.BanDurationMin)
	}
	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title":    map[string]any{"tag": "plain_text", "content": "账户风控封禁通知"},
			"template": "red",
		},
		"elements": []any{
			map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**%s**，您的账户已因触发内容风控规则被自动封禁。", displayName)}},
			map[string]any{"tag": "div", "fields": []any{
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**命中分组**\n%s", groupName)}},
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**风险类别**\n%s", category)}},
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**最高分数**\n%.3f", input.Score)}},
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**触发次数**\n%d / %d", input.ViolationCount, input.BanThreshold)}},
				map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**封禁时长**\n%s", banDuration)}},
			}},
			s.feishuPanelActionElement(ctx, "查看账户", ""),
		},
	}
	return s.sendInteractiveCard(ctx, input.UserID, card)
}

func (s *FeishuNotificationService) sendInteractiveCard(ctx context.Context, userID int64, card map[string]any) error {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return err
	}
	if !cfg.Enabled || cfg.AppID == "" || cfg.AppSecret == "" {
		return ErrFeishuNotificationDisabled
	}
	if s == nil || s.bindingRepo == nil {
		return ErrFeishuNotificationNotBound
	}
	binding, err := s.bindingRepo.GetFeishuNotificationBinding(ctx, userID, cfg.AppID)
	if err != nil {
		return err
	}
	if !binding.NotificationEnabled {
		return ErrFeishuNotificationDisabled
	}
	token, err := s.fetchTenantAccessToken(ctx, cfg)
	if err != nil {
		return err
	}
	cardJSON, err := json.Marshal(card)
	if err != nil {
		return err
	}
	messageURL, err := buildFeishuMessageURL(cfg.MessageURL)
	if err != nil {
		return err
	}
	body := map[string]any{
		"receive_id": binding.OpenID,
		"msg_type":   "interactive",
		"content":    string(cardJSON),
	}
	resp, err := req.C().SetTimeout(30*time.Second).R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+token).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(body).
		Post(messageURL)
	if err != nil {
		return fmt.Errorf("send feishu message: %w", err)
	}
	if err := validateFeishuNotifyAPIResponse("send feishu message", resp); err != nil {
		return err
	}
	slog.Info("feishu notification sent", "user_id", userID, "app_id", cfg.AppID)
	return nil
}

func (s *FeishuNotificationService) feishuPanelActionElement(ctx context.Context, label string, fallbackURL string) map[string]any {
	panelURL := strings.TrimSpace(fallbackURL)
	if cfg, err := s.GetConfig(ctx); err == nil {
		panelURL = firstNonEmpty(cfg.PanelURL, panelURL, defaultFeishuPanelPath)
	}
	if label == "" {
		label = "打开面板"
	}
	return map[string]any{
		"tag": "action",
		"actions": []any{
			map[string]any{
				"tag":  "button",
				"text": map[string]any{"tag": "plain_text", "content": label},
				"type": "primary",
				"url":  panelURL,
			},
		},
	}
}

func (s *FeishuNotificationService) fetchTenantAccessToken(ctx context.Context, cfg FeishuNotificationConfig) (string, error) {
	resp, err := req.C().SetTimeout(30*time.Second).R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]string{
			"app_id":     strings.TrimSpace(cfg.AppID),
			"app_secret": strings.TrimSpace(cfg.AppSecret),
		}).
		Post(strings.TrimSpace(cfg.TokenURL))
	if err != nil {
		return "", fmt.Errorf("request feishu tenant token: %w", err)
	}
	body := resp.String()
	if err := validateFeishuNotifyAPIResponse("feishu tenant token", resp); err != nil {
		return "", err
	}
	token := firstNonEmpty(getFeishuNotifyJSON(body, "tenant_access_token"), getFeishuNotifyJSON(body, "data.tenant_access_token"))
	if token == "" {
		return "", fmt.Errorf("feishu tenant token response missing tenant_access_token")
	}
	return token, nil
}

func buildFeishuMessageURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	q := u.Query()
	if q.Get("receive_id_type") == "" {
		q.Set("receive_id_type", "open_id")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func feishuNotifyAPIErrorCode(body string) int64 {
	body = strings.TrimSpace(body)
	if body == "" {
		return -1
	}
	code := gjson.Get(body, "code")
	if !code.Exists() {
		return -1
	}
	return code.Int()
}

func validateFeishuNotifyAPIResponse(operation string, resp *req.Response) error {
	if resp == nil {
		return fmt.Errorf("%s response is nil", operation)
	}
	body := strings.TrimSpace(resp.String())
	if !resp.IsSuccessState() {
		return fmt.Errorf("%s status=%d code=%s msg=%s", operation, resp.StatusCode, getFeishuNotifyJSON(body, "code"), firstNonEmpty(getFeishuNotifyJSON(body, "msg"), getFeishuNotifyJSON(body, "message")))
	}
	if body == "" {
		return fmt.Errorf("%s status=%d empty response body", operation, resp.StatusCode)
	}
	if !gjson.Valid(body) {
		return fmt.Errorf("%s status=%d invalid json response", operation, resp.StatusCode)
	}
	if !gjson.Get(body, "code").Exists() {
		return fmt.Errorf("%s status=%d missing code in response", operation, resp.StatusCode)
	}
	if code := feishuNotifyAPIErrorCode(body); code != 0 {
		return fmt.Errorf("%s status=%d code=%s msg=%s", operation, resp.StatusCode, getFeishuNotifyJSON(body, "code"), firstNonEmpty(getFeishuNotifyJSON(body, "msg"), getFeishuNotifyJSON(body, "message")))
	}
	return nil
}

func getFeishuNotifyJSON(body string, path string) string {
	value := gjson.Get(body, path)
	if !value.Exists() {
		return ""
	}
	if value.Type == gjson.Number {
		return strconv.FormatInt(value.Int(), 10)
	}
	return strings.TrimSpace(value.String())
}
