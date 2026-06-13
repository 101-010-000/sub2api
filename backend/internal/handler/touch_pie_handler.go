package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type TouchPieHandler struct {
	service *service.TouchPieDeviceService
}

func NewTouchPieHandler(service *service.TouchPieDeviceService) *TouchPieHandler {
	return &TouchPieHandler{service: service}
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

type touchPieExportKeyResponse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Key    string `json:"key"`
	Status string `json:"status"`
}

func (h *TouchPieHandler) StartDevice(c *gin.Context) {
	var req touchPieStartRequest
	_ = c.ShouldBindJSON(&req)
	baseURL := strings.TrimSpace(req.BaseURL)
	if baseURL == "" {
		baseURL = requestPublicBaseURL(c)
	}
	result, err := h.service.Start(c.Request.Context(), baseURL)
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
	if err := h.service.Approve(c.Request.Context(), req.UserCode, subject.UserID); err != nil {
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
	result, err := h.service.Token(c.Request.Context(), req.DeviceCode)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
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
	apiKey, err := h.service.ExportAPIKey(c.Request.Context(), subject.UserID, keyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, touchPieExportKeyResponse{
		ID:     apiKey.ID,
		Name:   apiKey.Name,
		Key:    apiKey.Key,
		Status: apiKey.Status,
	})
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
