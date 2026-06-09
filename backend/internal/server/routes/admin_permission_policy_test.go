//go:build unit

package routes

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResolveAdminAccessRule(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		permission string
		superOnly  bool
	}{
		{
			name:       "dashboard_get_requires_read",
			method:     http.MethodGet,
			path:       "/api/v1/admin/dashboard/stats",
			permission: service.AdminPermissionDashboardRead,
		},
		{
			name:       "safe_query_post_requires_read",
			method:     http.MethodPost,
			path:       "/api/v1/admin/dashboard/users-usage",
			permission: service.AdminPermissionDashboardRead,
		},
		{
			name:       "user_update_requires_write",
			method:     http.MethodPut,
			path:       "/api/v1/admin/users/:id",
			permission: service.AdminPermissionUsersWrite,
		},
		{
			name:      "delete_is_super_only",
			method:    http.MethodDelete,
			path:      "/api/v1/admin/users/:id",
			superOnly: true,
		},
		{
			name:      "permission_registry_is_super_only",
			method:    http.MethodGet,
			path:      "/api/v1/admin/permissions",
			superOnly: true,
		},
		{
			name:      "admin_api_key_read_is_super_only",
			method:    http.MethodGet,
			path:      "/api/v1/admin/settings/admin-api-key",
			superOnly: true,
		},
		{
			name:      "backup_download_is_super_only",
			method:    http.MethodGet,
			path:      "/api/v1/admin/backups/:id/download-url",
			superOnly: true,
		},
		{
			name:      "system_restart_is_super_only",
			method:    http.MethodPost,
			path:      "/api/v1/admin/system/restart",
			superOnly: true,
		},
		{
			name:      "account_credential_apply_is_super_only",
			method:    http.MethodPost,
			path:      "/api/v1/admin/accounts/:id/apply-oauth-credentials",
			superOnly: true,
		},
		{
			name:      "account_data_export_is_super_only",
			method:    http.MethodGet,
			path:      "/api/v1/admin/accounts/data",
			superOnly: true,
		},
		{
			name:       "payment_provider_list_requires_read",
			method:     http.MethodGet,
			path:       "/api/v1/admin/payment/providers",
			permission: service.AdminPermissionPaymentRead,
		},
		{
			name:      "payment_provider_write_is_super_only",
			method:    http.MethodPost,
			path:      "/api/v1/admin/payment/providers",
			superOnly: true,
		},
		{
			name:      "payment_refund_is_super_only",
			method:    http.MethodPost,
			path:      "/api/v1/admin/payment/orders/:id/refund",
			superOnly: true,
		},
		{
			name:       "nested_scheduled_tests_route_requires_scheduled_tests_read",
			method:     http.MethodGet,
			path:       "/api/v1/admin/accounts/:id/scheduled-test-plans",
			permission: service.AdminPermissionScheduledTestsRead,
		},
		{
			name:       "nested_user_subscriptions_route_requires_subscriptions_read",
			method:     http.MethodGet,
			path:       "/api/v1/admin/users/:id/subscriptions",
			permission: service.AdminPermissionSubscriptionsRead,
		},
		{
			name:      "unknown_admin_route_is_super_only",
			method:    http.MethodGet,
			path:      "/api/v1/admin/unknown",
			superOnly: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rule := resolveAdminAccessRule(tc.method, tc.path)
			require.Equal(t, tc.superOnly, rule.SuperOnly)
			require.Equal(t, tc.permission, rule.Permission)
		})
	}
}
