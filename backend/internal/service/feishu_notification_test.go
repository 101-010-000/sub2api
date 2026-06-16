//go:build unit

package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type feishuNotificationTestBindingRepo struct {
	binding *FeishuUserIdentityBinding
}

func (r *feishuNotificationTestBindingRepo) UpsertFeishuUserIdentityBinding(ctx context.Context, input UpsertFeishuUserIdentityBindingInput) (*FeishuUserIdentityBinding, error) {
	return nil, ErrFeishuNotificationConflict
}

func (r *feishuNotificationTestBindingRepo) GetFeishuNotificationBinding(ctx context.Context, userID int64, appID string) (*FeishuUserIdentityBinding, error) {
	if r == nil || r.binding == nil {
		return nil, ErrFeishuNotificationNotBound
	}
	return r.binding, nil
}

func (r *feishuNotificationTestBindingRepo) GetFeishuBindingByUnionID(ctx context.Context, appID, tenantKey, unionID, purpose string) (*FeishuUserIdentityBinding, error) {
	return nil, ErrFeishuNotificationNotBound
}

func (r *feishuNotificationTestBindingRepo) ListFeishuBindingsByUser(ctx context.Context, userID int64) ([]FeishuUserIdentityBinding, error) {
	return nil, nil
}

func (r *feishuNotificationTestBindingRepo) SetFeishuNotificationEnabled(ctx context.Context, userID int64, appID string, enabled bool) (*FeishuUserIdentityBinding, error) {
	return nil, ErrFeishuNotificationNotBound
}

func (r *feishuNotificationTestBindingRepo) DeleteFeishuNotificationBinding(ctx context.Context, userID int64, appID string) error {
	return nil
}

func newFeishuNotificationTestService(t *testing.T, tokenBody any, messageHandler http.HandlerFunc) (*FeishuNotificationService, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if s, ok := tokenBody.(string); ok {
				_, _ = w.Write([]byte(s))
				return
			}
			require.NoError(t, json.NewEncoder(w).Encode(tokenBody))
		case "/messages":
			messageHandler(w, r)
		default:
			t.Fatalf("unexpected feishu path: %s", r.URL.Path)
		}
	}))
	settingRepo := &contentModerationTestSettingRepo{values: map[string]string{
		SettingKeyFeishuNotifyEnabled:    "true",
		SettingKeyFeishuNotifyAppID:      "cli-test",
		SettingKeyFeishuNotifyAppSecret:  "secret",
		SettingKeyFeishuNotifyTokenURL:   server.URL + "/token",
		SettingKeyFeishuNotifyMessageURL: server.URL + "/messages",
	}}
	bindingRepo := &feishuNotificationTestBindingRepo{binding: &FeishuUserIdentityBinding{
		UserID:              1,
		AppID:               "cli-test",
		OpenID:              "ou-test",
		NotificationEnabled: true,
	}}
	return NewFeishuNotificationService(settingRepo, bindingRepo), server.Close
}

func TestFeishuNotificationSendRejectsMalformedSuccessResponses(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		errContains string
	}{
		{name: "empty", body: "", errContains: "empty response body"},
		{name: "html", body: "<html>ok</html>", errContains: "invalid json response"},
		{name: "missing_code", body: `{"data":{"message_id":"om_x"}}`, errContains: "missing code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, cleanup := newFeishuNotificationTestService(t, map[string]any{
				"code":                0,
				"tenant_access_token": "tenant-token",
			}, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			})
			defer cleanup()

			err := svc.SendBalanceLow(context.Background(), FeishuBalanceLowNotification{UserID: 1, Balance: 1, Threshold: 2})
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestFeishuNotificationSendAcceptsCodeZeroResponse(t *testing.T) {
	svc, cleanup := newFeishuNotificationTestService(t, map[string]any{
		"code":                0,
		"tenant_access_token": "tenant-token",
	}, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"message_id": "om_test"},
		})
	})
	defer cleanup()

	err := svc.SendBalanceLow(context.Background(), FeishuBalanceLowNotification{UserID: 1, Balance: 1, Threshold: 2})
	require.NoError(t, err)
}

func TestFeishuNotificationTokenRejectsMissingCodeResponse(t *testing.T) {
	svc, cleanup := newFeishuNotificationTestService(t, `{"tenant_access_token":"tenant-token"}`, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0})
	})
	defer cleanup()

	err := svc.SendBalanceLow(context.Background(), FeishuBalanceLowNotification{UserID: 1, Balance: 1, Threshold: 2})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing code")
}

func TestBalanceLowEmailFallbackLogsWhenNoRecipients(t *testing.T) {
	var output strings.Builder
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	svc := &BalanceNotifyService{notificationEmailService: NewNotificationEmailService(newNotificationEmailMemorySettingRepo(), nil)}
	svc.sendBalanceLowEmails(nil, 42, "Alice", "alice@example.com", 1, 2, "Sub2API", "")

	require.Contains(t, output.String(), "balance low email fallback skipped")
	require.Contains(t, output.String(), "user_id=42")
}
