package middleware

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type AdminAccessRule struct {
	Permission     string
	SuperOnly      bool
	AllowDelegated bool
}

func RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsSuperAdminContext(c) {
			c.Next()
			return
		}
		AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Super admin access required")
	}
}

func RequireAdminPermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsSuperAdminContext(c) || service.HasAdminPermission(GetAdminPermissionsFromContext(c), permission) {
			c.Next()
			return
		}
		AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Admin permission required")
	}
}

func RequireAnyAdminPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsSuperAdminContext(c) {
			c.Next()
			return
		}
		current := GetAdminPermissionsFromContext(c)
		for _, permission := range permissions {
			if service.HasAdminPermission(current, permission) {
				c.Next()
				return
			}
		}
		AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Admin permission required")
	}
}

func AdminPermissionGuard(resolve func(method, fullPath string) AdminAccessRule) gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsSuperAdminContext(c) {
			c.Next()
			return
		}
		if resolve == nil {
			AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Super admin access required")
			return
		}
		rule := resolve(c.Request.Method, c.FullPath())
		if rule.SuperOnly || strings.TrimSpace(rule.Permission) == "" {
			if rule.AllowDelegated && len(GetAdminPermissionsFromContext(c)) > 0 {
				c.Next()
				return
			}
			AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Super admin access required")
			return
		}
		if !service.HasAdminPermission(GetAdminPermissionsFromContext(c), rule.Permission) {
			AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Admin permission required")
			return
		}
		c.Next()
	}
}
