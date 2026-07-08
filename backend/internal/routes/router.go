package routes

import (
	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/handlers"
	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NewRouter 组装 Gin 引擎: 全局中间件链 + 路由。
// 中间件顺序: Logger -> Recovery -> CORS。
func NewRouter(cfg *config.Config, db *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS())

	// 健康检查(不依赖 DB, 始终可用)
	r.GET("/health", handlers.Health())

	// API v1 分组(后续 MB1-MB5 在此挂载: /auth/device, /geo/city, /vision/detect ...)
	api := r.Group("/api/v1")
	{
		// 占位探活, 验证路由分组可用
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{"msg": "pong", "db": db != nil})
		})
		_ = cfg
	}
	return r
}
