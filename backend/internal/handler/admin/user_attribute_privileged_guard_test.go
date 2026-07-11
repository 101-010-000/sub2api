//go:build unit

package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserAttributeHandlerDelegatedAdminCannotUpdatePrivilegedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	adminSvc.users[0].Role = service.RoleAdmin
	handler := NewUserAttributeHandler(nil, adminSvc)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAdminPermissions), []string{service.AdminPermissionUsersWrite})
		c.Next()
	})
	router.PUT("/api/v1/admin/users/:id/attributes", handler.UpdateUserAttributes)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/1/attributes", bytes.NewBufferString(`{"values":{}}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}
