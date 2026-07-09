package routes

import (
	"time"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/handlers"
	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NewRouter 组装 Gin 引擎: 全局中间件链 + 路由分组。
// 中间件顺序: Logger -> Recovery -> CORS。
func NewRouter(cfg *config.Config, db *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS())

	// 健康检查(不依赖 DB, 始终可用)
	r.GET("/health", handlers.Health())

	// 构造服务与仓储
	thirdParty := &cfg.ThirdParty
	geoService := services.NewGeoService(thirdParty)
	weatherService := services.NewWeatherService(thirdParty)
	aiService := services.NewAIService(thirdParty)

	var deviceRepo *repo.DeviceRepo
	var animalRepo *repo.AnimalRepo
	var auditService *services.AuditService
	if db != nil {
		deviceRepo = repo.NewDeviceRepo(db)
		animalRepo = repo.NewAnimalRepo(db)
		auditRepo := repo.NewAuditLogRepo(db)
		auditService = services.NewAuditService(animalRepo, auditRepo)
	}

	// 限流器
	rateLimiter := middleware.NewRateLimiter(100.0/60.0, 10) // 100 req/min, burst 10
	costCounter := middleware.NewDailyCallCounter(middleware.DefaultDailyLimits)

	api := r.Group("/api/v1")
	{
		// 公开端点(无需鉴权)
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{"msg": "pong", "db": db != nil})
		})

		// MB1: 设备鉴权(无需鉴权)
		if deviceRepo != nil {
			authHandler := handlers.NewAuthHandler(deviceRepo, cfg.JWTSecret, 30*24*time.Hour) // 30 天过期
			api.POST("/auth/device", authHandler.DeviceAuth)
		}

		// 需鉴权的端点
		auth := api.Group("")
		auth.Use(middleware.JWTAuth(cfg.JWTSecret))
		{
			// MB2: 第三方 API 代理
			geoHandler := handlers.NewGeoHandler(geoService)
			weatherHandler := handlers.NewWeatherHandler(weatherService)
			auth.GET("/geo/city", geoHandler.GetCity)
			auth.GET("/weather/week", weatherHandler.GetWeek)

			// MB3: AI 推理编排(附每日调用上限)
			ai := auth.Group("")
			ai.Use(middleware.RateLimitByDevice(rateLimiter))
			{
				visionHandler := handlers.NewVisionHandler(aiService)
				valueHandler := handlers.NewValueHandler(aiService)
				ai.POST("/vision/detect", middleware.CostLimitByType(costCounter, "detect"), visionHandler.Detect)
				ai.POST("/vision/analyze", middleware.CostLimitByType(costCounter, "analyze"), visionHandler.Analyze)
				ai.POST("/value/generate", middleware.CostLimitByType(costCounter, "value"), valueHandler.Generate)
			}

			// MB4: 同步服务
			if animalRepo != nil && auditService != nil {
				syncHandler := handlers.NewSyncHandler(animalRepo, auditService)
				auth.POST("/sync/animal", syncHandler.SyncAnimal)
			}
		}
	}
	return r
}
