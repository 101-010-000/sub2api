//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type touchPieAPIKeyManagerStub struct {
	groups     []service.Group
	keys       []service.APIKey
	createReq  *service.CreateAPIKeyRequest
	createUser int64
	createdKey *service.APIKey
}

func (s *touchPieAPIKeyManagerStub) GetAvailableGroups(_ context.Context, userID int64) ([]service.Group, error) {
	return append([]service.Group(nil), s.groups...), nil
}

func (s *touchPieAPIKeyManagerStub) SearchAPIKeys(_ context.Context, userID int64, keyword string, limit int) ([]service.APIKey, error) {
	return append([]service.APIKey(nil), s.keys...), nil
}

func (s *touchPieAPIKeyManagerStub) Create(_ context.Context, userID int64, req service.CreateAPIKeyRequest) (*service.APIKey, error) {
	s.createUser = userID
	s.createReq = &req
	return s.createdKey, nil
}

type touchPieAPIKeyRepoForHandlerTest struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r *touchPieAPIKeyRepoForHandlerTest) Create(context.Context, *service.APIKey) error { return nil }
func (r *touchPieAPIKeyRepoForHandlerTest) GetByID(context.Context, int64) (*service.APIKey, error) {
	return r.key, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) GetKeyAndOwnerID(context.Context, int64) (string, int64, error) {
	return "", 0, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) GetByKey(context.Context, string) (*service.APIKey, error) {
	return nil, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) GetByKeyForAuth(context.Context, string) (*service.APIKey, error) {
	return nil, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) Update(context.Context, *service.APIKey) error { return nil }
func (r *touchPieAPIKeyRepoForHandlerTest) Delete(context.Context, int64) error           { return nil }
func (r *touchPieAPIKeyRepoForHandlerTest) DeleteWithAudit(context.Context, int64) error  { return nil }
func (r *touchPieAPIKeyRepoForHandlerTest) ListByUserID(context.Context, int64, pagination.PaginationParams, service.APIKeyListFilters) ([]service.APIKey, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) VerifyOwnership(context.Context, int64, []int64) ([]int64, error) {
	return nil, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) CountByUserID(context.Context, int64) (int64, error) {
	return 0, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) ExistsByKey(context.Context, string) (bool, error) {
	return false, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]service.APIKey, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) SearchAPIKeys(context.Context, int64, string, int) ([]service.APIKey, error) {
	return nil, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) ClearGroupIDByGroupID(context.Context, int64) (int64, error) {
	return 0, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) UpdateGroupIDByUserAndGroup(context.Context, int64, int64, int64) (int64, error) {
	return 0, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) CountByGroupID(context.Context, int64) (int64, error) {
	return 0, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) ListKeysByUserID(context.Context, int64) ([]string, error) {
	return nil, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) ListKeysByGroupID(context.Context, int64) ([]string, error) {
	return nil, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) IncrementQuotaUsed(context.Context, int64, float64) (float64, error) {
	return 0, nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) UpdateLastUsed(context.Context, int64, time.Time) error {
	return nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) IncrementRateLimitUsage(context.Context, int64, float64) error {
	return nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) ResetRateLimitWindows(context.Context, int64) error {
	return nil
}
func (r *touchPieAPIKeyRepoForHandlerTest) GetRateLimitData(context.Context, int64) (*service.APIKeyRateLimitData, error) {
	return nil, nil
}

func TestTouchPieExportAPIKeyIncludesTouchXMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewTouchPieHandler(service.NewTouchPieDeviceService(nil, nil, &touchPieAPIKeyRepoForHandlerTest{
		key: &service.APIKey{
			ID:     7,
			UserID: 42,
			Name:   "touch key",
			Key:    "sk-test",
			Status: service.StatusAPIKeyActive,
		},
	}), nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/touch-pie/api-keys/7/export", nil)
	c.Params = gin.Params{{Key: "id", Value: "7"}}
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

	h.ExportAPIKey(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, openai.TouchXProviderName, data["provider_name"])
	require.Equal(t, openai.TouchXSource, data["provider_source"])
	require.Equal(t, openai.TouchXAccentColor, data["provider_accent_color"])
	require.Equal(t, openai.DefaultLatestModel, data["default_model"])
}

func TestTouchPieBootstrapReturnsGroupsAndTouchXMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewTouchPieHandler(nil, &touchPieAPIKeyManagerStub{
		groups: []service.Group{{
			ID:             3,
			Name:           "OpenAI",
			Platform:       "openai",
			Status:         service.StatusActive,
			RateMultiplier: 1,
		}},
		keys: []service.APIKey{{
			ID:     7,
			UserID: 42,
			Name:   openai.TouchXProviderName,
			Status: service.StatusAPIKeyActive,
		}},
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/touch-pie/bootstrap", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

	h.Bootstrap(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, openai.TouchXProviderName, data["provider_name"])
	require.Equal(t, openai.DefaultLatestModel, data["default_model"])
	require.Len(t, data["groups"], 1)
	require.Len(t, data["api_keys"], 1)
}

func TestTouchPieCreateAPIKeyUsesSelectedGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(3)
	manager := &touchPieAPIKeyManagerStub{
		createdKey: &service.APIKey{
			ID:      9,
			UserID:  42,
			Name:    openai.TouchXProviderName,
			Key:     "sk-touchx",
			GroupID: &groupID,
			Status:  service.StatusAPIKeyActive,
		},
	}
	h := NewTouchPieHandler(nil, manager)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/touch-pie/api-keys", bytes.NewBufferString(`{"group_id":3}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

	h.CreateAPIKey(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), manager.createUser)
	require.NotNil(t, manager.createReq)
	require.Equal(t, openai.TouchXProviderName, manager.createReq.Name)
	require.NotNil(t, manager.createReq.GroupID)
	require.Equal(t, groupID, *manager.createReq.GroupID)

	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "sk-touchx", data["key"])
	require.Equal(t, openai.TouchXAccentColor, data["provider_accent_color"])
}
