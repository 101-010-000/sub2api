package admin

import (
	"net"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AdminAPIKeyHandler handles admin API key management
type AdminAPIKeyHandler struct {
	adminService         service.AdminService
	apiKeyService        *service.APIKeyService
	apiKeyRuntimeService *service.APIKeyRuntimeService
}

// NewAdminAPIKeyHandler creates a new admin API key handler
func NewAdminAPIKeyHandler(adminService service.AdminService) *AdminAPIKeyHandler {
	return &AdminAPIKeyHandler{
		adminService: adminService,
	}
}

func ProvideAdminAPIKeyHandler(adminService service.AdminService, apiKeyService *service.APIKeyService, apiKeyRuntimeService *service.APIKeyRuntimeService) *AdminAPIKeyHandler {
	h := NewAdminAPIKeyHandler(adminService)
	h.apiKeyService = apiKeyService
	h.apiKeyRuntimeService = apiKeyRuntimeService
	return h
}

// AdminUpdateAPIKeyGroupRequest represents the request to update an API key.
type AdminUpdateAPIKeyGroupRequest struct {
	GroupID             *int64 `json:"group_id"`               // nil=不修改, 0=解绑, >0=绑定到目标分组
	ResetRateLimitUsage *bool  `json:"reset_rate_limit_usage"` // true=重置 5h/1d/7d 限速用量
	MaxActiveIPs         *int   `json:"max_active_ips"`          // nil=不修改, 0=不限制
	IPIdleTimeoutSeconds *int   `json:"ip_idle_timeout_seconds"` // nil=不修改, 0=默认
	MaxConcurrency       *int   `json:"max_concurrency"`         // nil=不修改, 0=不限制
}

type adminRemoveActiveIPRequest struct {
	IP string `json:"ip" binding:"required"`
}

// UpdateGroup handles updating an API key's admin-managed fields.
// PUT /api/v1/admin/api-keys/:id
func (h *AdminAPIKeyHandler) UpdateGroup(c *gin.Context) {
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	var req AdminUpdateAPIKeyGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	var resetKey *service.APIKey
	if req.ResetRateLimitUsage != nil && *req.ResetRateLimitUsage {
		resetKey, err = h.adminService.AdminResetAPIKeyRateLimitUsage(c.Request.Context(), keyID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	var runtimeKey *service.APIKey
	if req.MaxActiveIPs != nil || req.IPIdleTimeoutSeconds != nil || req.MaxConcurrency != nil {
		runtimeKey, err = h.adminService.AdminUpdateAPIKeyRuntimeLimits(c.Request.Context(), keyID, service.AdminUpdateAPIKeyRuntimeLimitsInput{
			MaxActiveIPs:         req.MaxActiveIPs,
			IPIdleTimeoutSeconds: req.IPIdleTimeoutSeconds,
			MaxConcurrency:       req.MaxConcurrency,
		})
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	result, err := h.adminService.AdminUpdateAPIKeyGroupID(c.Request.Context(), keyID, req.GroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if resetKey != nil && req.GroupID == nil {
		result.APIKey = resetKey
	}
	if runtimeKey != nil && req.GroupID == nil {
		result.APIKey = runtimeKey
	}

	resp := struct {
		APIKey                 *dto.APIKey `json:"api_key"`
		AutoGrantedGroupAccess bool        `json:"auto_granted_group_access"`
		GrantedGroupID         *int64      `json:"granted_group_id,omitempty"`
		GrantedGroupName       string      `json:"granted_group_name,omitempty"`
	}{
		APIKey:                 dto.APIKeyFromService(result.APIKey),
		AutoGrantedGroupAccess: result.AutoGrantedGroupAccess,
		GrantedGroupID:         result.GrantedGroupID,
		GrantedGroupName:       result.GrantedGroupName,
	}
	response.Success(c, resp)
}

func (h *AdminAPIKeyHandler) GetRuntime(c *gin.Context) {
	if h.apiKeyRuntimeService == nil {
		response.InternalError(c, "API key runtime service is not configured")
		return
	}
	key, ok := h.getAPIKey(c)
	if !ok {
		return
	}
	status, err := h.apiKeyRuntimeService.GetStatus(c.Request.Context(), key)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *AdminAPIKeyHandler) RemoveRuntimeIP(c *gin.Context) {
	if h.apiKeyRuntimeService == nil {
		response.InternalError(c, "API key runtime service is not configured")
		return
	}
	key, ok := h.getAPIKey(c)
	if !ok {
		return
	}
	var req adminRemoveActiveIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if net.ParseIP(strings.TrimSpace(req.IP)) == nil {
		response.BadRequest(c, "Invalid ip")
		return
	}
	if err := h.apiKeyRuntimeService.RemoveActiveIP(c.Request.Context(), key, strings.TrimSpace(req.IP)); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "active IP removed"})
}

func (h *AdminAPIKeyHandler) ClearRuntimeIPs(c *gin.Context) {
	if h.apiKeyRuntimeService == nil {
		response.InternalError(c, "API key runtime service is not configured")
		return
	}
	key, ok := h.getAPIKey(c)
	if !ok {
		return
	}
	if err := h.apiKeyRuntimeService.ClearActiveIPs(c.Request.Context(), key); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "active IPs cleared"})
}

func (h *AdminAPIKeyHandler) getAPIKey(c *gin.Context) (*service.APIKey, bool) {
	if h.apiKeyService == nil {
		response.InternalError(c, "API key service is not configured")
		return nil, false
	}
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return nil, false
	}
	key, err := h.apiKeyService.GetByID(c.Request.Context(), keyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	return key, true
}
