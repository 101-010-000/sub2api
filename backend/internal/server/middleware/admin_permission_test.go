//go:build unit

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequireAdminPermission(t *testing.T) {
	tests := []struct {
		name        string
		superAdmin  bool
		permissions []string
		wantStatus  int
	}{
		{
			name:       "super_admin_bypasses_permission_check",
			superAdmin: true,
			wantStatus: http.StatusOK,
		},
		{
			name:        "delegated_user_with_permission_allowed",
			permissions: []string{service.AdminPermissionUsersRead},
			wantStatus:  http.StatusOK,
		},
		{
			name:        "delegated_user_without_permission_forbidden",
			permissions: []string{service.AdminPermissionDashboardRead},
			wantStatus:  http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := serveAdminPermissionRequest(func(c *gin.Context) {
				c.Set(string(ContextKeyAdminSuper), tc.superAdmin)
				c.Set(string(ContextKeyAdminPermissions), tc.permissions)
			}, RequireAdminPermission(service.AdminPermissionUsersRead))
			require.Equal(t, tc.wantStatus, w.Code)
		})
	}
}
func TestRequireAnyAdminPermission(t *testing.T) {
	w := serveAdminPermissionRequest(func(c *gin.Context) {
		c.Set(string(ContextKeyAdminPermissions), []string{service.AdminPermissionSettingsRead})
	}, RequireAnyAdminPermission(service.AdminPermissionUsersRead, service.AdminPermissionSettingsRead))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRequireSuperAdmin(t *testing.T) {
	tests := []struct {
		name       string
		superAdmin bool
		wantStatus int
	}{
		{name: "super_admin_allowed", superAdmin: true, wantStatus: http.StatusOK},
		{name: "delegated_user_forbidden", wantStatus: http.StatusForbidden},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := serveAdminPermissionRequest(func(c *gin.Context) {
				c.Set(string(ContextKeyAdminSuper), tc.superAdmin)
				c.Set(string(ContextKeyAdminPermissions), []string{service.AdminPermissionUsersWrite})
			}, RequireSuperAdmin())
			require.Equal(t, tc.wantStatus, w.Code)
		})
	}
}

func TestAdminPermissionGuard(t *testing.T) {
	t.Run("delegated_user_allowed_by_resolved_permission", func(t *testing.T) {
		w := serveAdminPermissionRequest(func(c *gin.Context) {
			c.Set(string(ContextKeyAdminPermissions), []string{service.AdminPermissionUsersWrite})
		}, AdminPermissionGuard(func(method, fullPath string) AdminAccessRule {
			require.Equal(t, http.MethodPut, method)
			require.Equal(t, "/t", fullPath)
			return AdminAccessRule{Permission: service.AdminPermissionUsersWrite}
		}))
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("super_admin_bypasses_resolver", func(t *testing.T) {
		w := serveAdminPermissionRequest(func(c *gin.Context) {
			c.Set(string(ContextKeyAdminSuper), true)
		}, AdminPermissionGuard(nil))
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("super_only_rule_forbids_delegated_user", func(t *testing.T) {
		w := serveAdminPermissionRequest(func(c *gin.Context) {
			c.Set(string(ContextKeyAdminPermissions), []string{service.AdminPermissionUsersWrite})
		}, AdminPermissionGuard(func(string, string) AdminAccessRule {
			return AdminAccessRule{SuperOnly: true}
		}))
		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("delegated_admin_rule_allows_any_delegated_permission", func(t *testing.T) {
		w := serveAdminPermissionRequest(func(c *gin.Context) {
			c.Set(string(ContextKeyAdminPermissions), []string{service.AdminPermissionUsersRead})
		}, AdminPermissionGuard(func(string, string) AdminAccessRule {
			return AdminAccessRule{AllowDelegated: true}
		}))
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("delegated_admin_rule_forbids_user_without_permissions", func(t *testing.T) {
		w := serveAdminPermissionRequest(func(c *gin.Context) {}, AdminPermissionGuard(func(string, string) AdminAccessRule {
			return AdminAccessRule{AllowDelegated: true}
		}))
		require.Equal(t, http.StatusForbidden, w.Code)
	})
}

func serveAdminPermissionRequest(seed gin.HandlerFunc, guard gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(seed)
	r.Use(guard)
	r.PUT("/t", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/t", nil)
	r.ServeHTTP(w, req)
	return w
}
