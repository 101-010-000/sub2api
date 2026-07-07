package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type settingHandlerFeishuBindingRepo struct {
	binding *service.FeishuUserIdentityBinding
}

func (r *settingHandlerFeishuBindingRepo) UpsertFeishuUserIdentityBinding(ctx context.Context, input service.UpsertFeishuUserIdentityBindingInput) (*service.FeishuUserIdentityBinding, error) {
	return nil, service.ErrFeishuNotificationConflict
}

func (r *settingHandlerFeishuBindingRepo) GetFeishuNotificationBinding(ctx context.Context, userID int64, appID string) (*service.FeishuUserIdentityBinding, error) {
	if r == nil || r.binding == nil || r.binding.UserID != userID || r.binding.AppID != appID {
		return nil, service.ErrFeishuNotificationNotBound
	}
	return r.binding, nil
}

func (r *settingHandlerFeishuBindingRepo) GetFeishuBindingByUnionID(ctx context.Context, appID, tenantKey, unionID, purpose string) (*service.FeishuUserIdentityBinding, error) {
	return nil, service.ErrFeishuNotificationNotBound
}

func (r *settingHandlerFeishuBindingRepo) ListFeishuBindingsByUser(ctx context.Context, userID int64) ([]service.FeishuUserIdentityBinding, error) {
	return nil, nil
}

func (r *settingHandlerFeishuBindingRepo) SetFeishuNotificationEnabled(ctx context.Context, userID int64, appID string, enabled bool) (*service.FeishuUserIdentityBinding, error) {
	return nil, service.ErrFeishuNotificationNotBound
}

func (r *settingHandlerFeishuBindingRepo) DeleteFeishuNotificationBinding(ctx context.Context, userID int64, appID string) error {
	return nil
}

func TestSettingHandler_UpdateSettings_PreservesFeishuNotifySecretWhenOmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyFeishuNotifyEnabled:   "true",
		service.SettingKeyFeishuNotifyAppID:     "cli-old",
		service.SettingKeyFeishuNotifyAppSecret: "old-secret",
	}}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(map[string]any{
		"feishu_notify_enabled": false,
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "old-secret", repo.values[service.SettingKeyFeishuNotifyAppSecret])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, data["feishu_notify_app_secret_configured"])
}

func TestSettingHandler_TestFeishuNotification_SendsBoundUserCard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var messageCalled atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"tenant_access_token": "tenant-token",
			}))
		case "/messages":
			messageCalled.Store(true)
			require.Equal(t, "open_id", r.URL.Query().Get("receive_id_type"))
			require.Equal(t, "Bearer tenant-token", r.Header.Get("Authorization"))

			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "ou-test", body["receive_id"])
			require.Equal(t, "interactive", body["msg_type"])
			require.Contains(t, body["content"], "飞书通知链路测试")
			require.Contains(t, body["content"], "/feishu/panel")

			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"code": 0}))
		default:
			t.Fatalf("unexpected feishu path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	settingRepo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyFeishuNotifyEnabled:    "true",
		service.SettingKeyFeishuNotifyAppID:      "cli-test",
		service.SettingKeyFeishuNotifyAppSecret:  "secret",
		service.SettingKeyFeishuNotifyTokenURL:   server.URL + "/token",
		service.SettingKeyFeishuNotifyMessageURL: server.URL + "/messages",
		service.SettingKeyFeishuNotifyPanelURL:   "/feishu/panel",
	}}
	bindingRepo := &settingHandlerFeishuBindingRepo{binding: &service.FeishuUserIdentityBinding{
		UserID:              42,
		AppID:               "cli-test",
		OpenID:              "ou-test",
		NotificationEnabled: true,
	}}
	handler := NewSettingHandler(nil, nil, nil, nil, nil, nil, nil)
	handler.SetFeishuNotificationService(service.NewFeishuNotificationService(settingRepo, bindingRepo))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/feishu-notification/test", strings.NewReader(`{"user_id":42}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.TestFeishuNotification(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, messageCalled.Load())

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, data["sent"])
}
