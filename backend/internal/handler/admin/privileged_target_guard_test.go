package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type privilegedTargetSubscriptionRepo struct {
	service.UserSubscriptionRepository
	active              *service.UserSubscription
	deleted             *service.UserSubscription
	activeCalls         int
	includeDeletedCalls int
}

func (r *privilegedTargetSubscriptionRepo) GetByID(context.Context, int64) (*service.UserSubscription, error) {
	r.activeCalls++
	return r.active, nil
}

func (r *privilegedTargetSubscriptionRepo) GetByIDIncludeDeleted(context.Context, int64) (*service.UserSubscription, error) {
	r.includeDeletedCalls++
	return r.deleted, nil
}

func privilegedTargetAdminService() *stubAdminService {
	adminService := newStubAdminService()
	adminService.users[0].Role = service.RoleAdmin
	return adminService
}

func useDelegatedAdminContext(router *gin.Engine) {
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAdminPermissions), []string{
			service.AdminPermissionRedeemCodesWrite,
			service.AdminPermissionSubscriptionsWrite,
			service.AdminPermissionRiskControlWrite,
			service.AdminPermissionAffiliatesWrite,
		})
		c.Next()
	})
}

func TestRedeemHandlerDelegatedAdminCannotRedeemForPrivilegedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewRedeemHandler(privilegedTargetAdminService(), &service.RedeemService{})
	router := gin.New()
	useDelegatedAdminContext(router)
	router.POST("/redeem", handler.CreateAndRedeem)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/redeem", bytes.NewBufferString(`{
		"code":"privileged-target",
		"type":"balance",
		"value":-1,
		"user_id":1
	}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSubscriptionHandlerDelegatedAdminCannotMutatePrivilegedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	deletedAt := time.Now()
	repo := &privilegedTargetSubscriptionRepo{
		active:  &service.UserSubscription{ID: 10, UserID: 1, GroupID: 2},
		deleted: &service.UserSubscription{ID: 10, UserID: 1, GroupID: 2, DeletedAt: &deletedAt},
	}
	subscriptionService := service.NewSubscriptionService(nil, repo, nil, nil, nil)
	handler := NewSubscriptionHandler(subscriptionService, privilegedTargetAdminService())
	router := gin.New()
	useDelegatedAdminContext(router)
	router.POST("/assign", handler.Assign)
	router.POST("/bulk-assign", handler.BulkAssign)
	router.POST("/:id/extend", handler.Extend)
	router.POST("/:id/reset-quota", handler.ResetQuota)
	router.POST("/:id/revoke", handler.Revoke)
	router.POST("/:id/restore", handler.Restore)

	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "assign", path: "/assign", body: `{"user_id":1,"group_id":2}`},
		{name: "bulk assign", path: "/bulk-assign", body: `{"user_ids":[1],"group_id":2}`},
		{name: "extend", path: "/10/extend", body: `{"days":1}`},
		{name: "reset quota", path: "/10/reset-quota", body: `{"daily":true}`},
		{name: "revoke", path: "/10/revoke"},
		{name: "restore", path: "/10/restore"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusForbidden, rec.Code)
		})
	}
	require.Equal(t, 3, repo.activeCalls)
	require.Equal(t, 1, repo.includeDeletedCalls)
}

func TestContentModerationHandlerDelegatedAdminCannotMutatePrivilegedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewContentModerationHandler(nil, privilegedTargetAdminService())
	router := gin.New()
	useDelegatedAdminContext(router)
	router.POST("/:user_id/suspicion", handler.SetUserSuspicion)
	router.POST("/:user_id/self-unban", handler.SelfUnban)
	router.POST("/:user_id/unban", handler.UnbanUser)

	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "set suspicion", path: "/1/suspicion", body: `{"suspicious":true,"reason":"review"}`},
		{name: "self unban", path: "/1/self-unban"},
		{name: "unban", path: "/1/unban"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusForbidden, rec.Code)
		})
	}
}

func TestAffiliateHandlerDelegatedAdminCannotMutatePrivilegedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAffiliateHandler(nil, privilegedTargetAdminService())
	router := gin.New()
	useDelegatedAdminContext(router)
	router.PUT("/:user_id", handler.UpdateUserSettings)
	router.DELETE("/:user_id", handler.ClearUserSettings)
	router.POST("/batch-rate", handler.BatchSetRate)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "update", method: http.MethodPut, path: "/1", body: `{"aff_code":"blocked"}`},
		{name: "clear", method: http.MethodDelete, path: "/1"},
		{name: "batch", method: http.MethodPost, path: "/batch-rate", body: `{"user_ids":[1],"clear":true}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusForbidden, rec.Code)
		})
	}
}
