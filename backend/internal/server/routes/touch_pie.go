package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterTouchPieRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	settingService *service.SettingService,
) {
	touchPie := v1.Group("/touch-pie")
	{
		touchPie.POST("/device/start", h.TouchPie.StartDevice)
		touchPie.POST("/device/token", h.TouchPie.Token)
	}

	authenticated := touchPie.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))
	{
		authenticated.GET("/bootstrap", h.TouchPie.Bootstrap)
		authenticated.POST("/device/approve", h.TouchPie.ApproveDevice)
		authenticated.POST("/api-keys", h.TouchPie.CreateAPIKey)
		authenticated.POST("/api-keys/:id/export", h.TouchPie.ExportAPIKey)
	}
}
