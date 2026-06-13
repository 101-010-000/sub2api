package handler

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type TouchPieHandler struct {
	deviceService *service.TouchPieDeviceService
	apiKeyService touchPieAPIKeyManager
}

type touchPieAPIKeyManager interface {
	GetAvailableGroups(ctx context.Context, userID int64) ([]service.Group, error)
	SearchAPIKeys(ctx context.Context, userID int64, keyword string, limit int) ([]service.APIKey, error)
	Create(ctx context.Context, userID int64, req service.CreateAPIKeyRequest) (*service.APIKey, error)
}

func NewTouchPieHandler(deviceService *service.TouchPieDeviceService, apiKeyService touchPieAPIKeyManager) *TouchPieHandler {
	return &TouchPieHandler{deviceService: deviceService, apiKeyService: apiKeyService}
}

type touchPieStartRequest struct {
	BaseURL string `json:"base_url"`
}

type touchPieApproveRequest struct {
	UserCode string `json:"user_code" binding:"required"`
}

type touchPieTokenRequest struct {
	DeviceCode string `json:"device_code" binding:"required"`
}

type touchPieCreateAPIKeyRequest struct {
	Name    string `json:"name"`
	GroupID *int64 `json:"group_id"`
}

type touchPieAPIKeyCandidate struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	GroupID *int64 `json:"group_id"`
}

type touchPieBootstrapResponse struct {
	Groups              []dto.Group               `json:"groups"`
	APIKeys             []touchPieAPIKeyCandidate `json:"api_keys"`
	ProviderName        string                    `json:"provider_name"`
	ProviderSource      string                    `json:"provider_source"`
	ProviderAccentColor string                    `json:"provider_accent_color"`
	DefaultModel        string                    `json:"default_model"`
}

type touchPieExportKeyResponse struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	Key                 string `json:"key"`
	Status              string `json:"status"`
	ProviderName        string `json:"provider_name"`
	ProviderSource      string `json:"provider_source"`
	ProviderAccentColor string `json:"provider_accent_color"`
	DefaultModel        string `json:"default_model"`
}

func (h *TouchPieHandler) StartDevice(c *gin.Context) {
	var req touchPieStartRequest
	_ = c.ShouldBindJSON(&req)
	baseURL := strings.TrimSpace(req.BaseURL)
	if baseURL == "" {
		baseURL = requestPublicBaseURL(c)
	}
	result, err := h.deviceService.Start(c.Request.Context(), baseURL)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TouchPieHandler) ApproveDevice(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req touchPieApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.deviceService.Approve(c.Request.Context(), req.UserCode, subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"approved": true})
}

func (h *TouchPieHandler) Token(c *gin.Context) {
	var req touchPieTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.deviceService.Token(c.Request.Context(), req.DeviceCode)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TouchPieHandler) Bootstrap(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.apiKeyService == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("TOUCH_PIE_UNAVAILABLE", "Touch Pie API key service unavailable"))
		return
	}

	groups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	outGroups := make([]dto.Group, 0, len(groups))
	for i := range groups {
		outGroups = append(outGroups, *dto.GroupFromService(&groups[i]))
	}

	keys, err := h.apiKeyService.SearchAPIKeys(c.Request.Context(), subject.UserID, openai.TouchXProviderName, 10)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	outKeys := make([]touchPieAPIKeyCandidate, 0, len(keys))
	for i := range keys {
		key := keys[i]
		outKeys = append(outKeys, touchPieAPIKeyCandidate{
			ID:      key.ID,
			Name:    key.Name,
			Status:  key.Status,
			GroupID: key.GroupID,
		})
	}

	response.Success(c, touchPieBootstrapResponse{
		Groups:              outGroups,
		APIKeys:             outKeys,
		ProviderName:        openai.TouchXProviderName,
		ProviderSource:      openai.TouchXSource,
		ProviderAccentColor: openai.TouchXAccentColor,
		DefaultModel:        openai.DefaultLatestModel,
	})
}

func (h *TouchPieHandler) CreateAPIKey(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.apiKeyService == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("TOUCH_PIE_UNAVAILABLE", "Touch Pie API key service unavailable"))
		return
	}

	var req touchPieCreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = openai.TouchXProviderName
	}

	apiKey, err := h.apiKeyService.Create(c.Request.Context(), subject.UserID, service.CreateAPIKeyRequest{
		Name:    name,
		GroupID: req.GroupID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, newTouchPieExportKeyResponse(apiKey))
}

func (h *TouchPieHandler) ExportAPIKey(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid key ID")
		return
	}
	apiKey, err := h.deviceService.ExportAPIKey(c.Request.Context(), subject.UserID, keyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, newTouchPieExportKeyResponse(apiKey))
}

func newTouchPieExportKeyResponse(apiKey *service.APIKey) touchPieExportKeyResponse {
	return touchPieExportKeyResponse{
		ID:                  apiKey.ID,
		Name:                apiKey.Name,
		Key:                 apiKey.Key,
		Status:              apiKey.Status,
		ProviderName:        openai.TouchXProviderName,
		ProviderSource:      openai.TouchXSource,
		ProviderAccentColor: openai.TouchXAccentColor,
		DefaultModel:        openai.DefaultLatestModel,
	}
}

func requestPublicBaseURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = strings.TrimSpace(strings.Split(forwardedProto, ",")[0])
	}
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}
