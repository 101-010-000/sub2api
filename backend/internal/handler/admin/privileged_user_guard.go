package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

func requireManageableUser(c *gin.Context, adminService service.AdminService, userID int64) (*service.User, bool) {
	if adminService == nil {
		response.InternalError(c, "Admin service is not configured")
		return nil, false
	}
	user, err := adminService.GetUser(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	if !middleware.IsSuperAdminContext(c) && user.CanAccessAdmin() {
		response.Forbidden(c, "Only super admin can manage admin users")
		return nil, false
	}
	return user, true
}

func requireManageableUsers(c *gin.Context, adminService service.AdminService, userIDs []int64) bool {
	if middleware.IsSuperAdminContext(c) {
		return true
	}
	checkedUserIDs := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if _, checked := checkedUserIDs[userID]; checked {
			continue
		}
		if _, ok := requireManageableUser(c, adminService, userID); !ok {
			return false
		}
		checkedUserIDs[userID] = struct{}{}
	}
	return true
}

func requireManageableAPIKey(c *gin.Context, adminService service.AdminService, keyID int64) (*service.APIKey, bool) {
	if adminService == nil {
		response.InternalError(c, "Admin service is not configured")
		return nil, false
	}
	key, err := adminService.GetAPIKey(c.Request.Context(), keyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	if key == nil {
		response.InternalError(c, "API key service returned no key")
		return nil, false
	}
	if _, ok := requireManageableUser(c, adminService, key.UserID); !ok {
		return nil, false
	}
	return key, true
}
