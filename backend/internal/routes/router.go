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

// deviceChecker 适配 DeviceRepo 到 middleware.DeviceChecker。
type deviceChecker struct {
	repo *repo.DeviceRepo
}

func (d deviceChecker) IsDisabled(deviceID string) (bool, error) {
	return d.repo.IsDisabled(deviceID)
}

func (d deviceChecker) TokenVersion(deviceID string) (int, error) {
	dev, err := d.repo.Find(deviceID)
	if err != nil {
		return 0, err
	}
	return dev.TokenVersion, nil
}

// NewRouter 组装 Gin 引擎: 全局中间件链 + 路由分组。
func NewRouter(cfg *config.Config, db *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS())
	r.MaxMultipartMemory = cfg.MaxImageBytes

	// Liveness
	r.GET("/health", handlers.Health())

	mockAllowed := cfg.MockAllowed()
	sharedHTTP := services.DefaultHTTPClient(30 * time.Second)

	thirdParty := &cfg.ThirdParty
	geoService := services.NewGeoServiceWithOptions(thirdParty, mockAllowed, sharedHTTP)
	weatherService := services.NewWeatherServiceWithOptions(thirdParty, mockAllowed, sharedHTTP)
	aiService := services.NewAIServiceWithOptions(thirdParty, mockAllowed, sharedHTTP)

	var deviceRepo *repo.DeviceRepo
	var animalRepo *repo.AnimalRepo
	var auditService *services.AuditService
	var auditRepo *repo.AuditLogRepo
	var inferenceRepo *repo.InferenceRepo
	if db != nil {
		deviceRepo = repo.NewDeviceRepo(db)
		animalRepo = repo.NewAnimalRepo(db)
		auditRepo = repo.NewAuditLogRepo(db)
		auditService = services.NewAuditService(animalRepo, auditRepo)
		inferenceRepo = repo.NewInferenceRepo(db)
	}

	// Readiness
	r.GET("/ready", handlers.Ready(handlers.ReadyDeps{
		DB:          db,
		ReadyErrors: cfg.ReadyErrors(),
		AppEnv:      cfg.AppEnv,
	}))

	// 限流：优先共享存储接口（内存实现可替换 Redis）
	sharedCounter := middleware.NewMemorySharedCounter()
	rateLimiter := middleware.NewRateLimiter(100.0/60.0, 10).WithShared(sharedCounter)
	costCounter := middleware.NewDailyCallCounter(middleware.DefaultDailyLimits).WithShared(sharedCounter)
	ipLimiter := middleware.NewRateLimiter(20.0/60.0, 5)

	api := r.Group("/api/v1")
	{
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{"msg": "pong", "db": db != nil, "app_env": cfg.AppEnv})
		})

		// 可信时间（公开，带签名）
		timeHandler := handlers.NewTimeHandler(cfg.JWTSecret)
		api.GET("/time", timeHandler.GetTime)

		// 设备鉴权（IP 限流防爆破）
		if deviceRepo != nil {
			authHandler := handlers.NewAuthHandlerFull(
				deviceRepo, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTIssuer, cfg.JWTAudience,
			)
			api.POST("/auth/device", middleware.RateLimitByIP(ipLimiter), authHandler.DeviceAuth)
		}

		// JWT
		var checker middleware.DeviceChecker
		if deviceRepo != nil {
			checker = deviceChecker{repo: deviceRepo}
		}
		auth := api.Group("")
		auth.Use(middleware.JWTAuthWithChecker(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, checker))
		{
			geoHandler := handlers.NewGeoHandler(geoService)
			weatherHandler := handlers.NewWeatherHandler(weatherService)
			auth.GET("/geo/city", geoHandler.GetCity)
			auth.GET("/weather/week", weatherHandler.GetWeek)

			ai := auth.Group("")
			ai.Use(middleware.RateLimitByDevice(rateLimiter))
			{
				visionHandler := handlers.NewVisionHandlerWithOptions(aiService, handlers.VisionHandlerOptions{
					InferenceRepo:  inferenceRepo,
					DeviceRepo:     deviceRepo,
					MaxBytes:       cfg.MaxImageBytes,
					MaxPixels:      cfg.MaxImagePixels,
					RequireConsent: cfg.IsProduction(),
					ConsentVersion: "v1",
				})
				valueHandler := handlers.NewValueHandlerWithRepo(aiService, inferenceRepo)
				ai.POST("/vision/detect", middleware.CostLimitByType(costCounter, "detect"), visionHandler.Detect)
				ai.POST("/vision/analyze", middleware.CostLimitByType(costCounter, "analyze"), visionHandler.Analyze)
				ai.POST("/value/generate", middleware.CostLimitByType(costCounter, "value"), valueHandler.Generate)
			}

			if animalRepo != nil && auditService != nil {
				syncHandler := handlers.NewSyncHandlerFull(animalRepo, auditService, inferenceRepo)
				auth.POST("/sync/animal", syncHandler.SyncAnimal)
				auth.POST("/sync/animals", syncHandler.SyncAnimalsBatch)
				auth.GET("/sync/animals", syncHandler.PullAnimals)
			}

			if db != nil && deviceRepo != nil {
				privacy := handlers.NewPrivacyHandler(db, deviceRepo, animalRepo, inferenceRepo, auditRepo)
				auth.POST("/privacy/consent", privacy.PutConsent)
				auth.POST("/privacy/export", privacy.ExportData)
				auth.POST("/privacy/delete", privacy.DeleteData)
				auth.GET("/privacy/requests/:id", privacy.GetDataRequest)

				sec := handlers.NewSecurityHandler(db, auditRepo)
				auth.POST("/security/report", sec.Report)

				commerce := handlers.NewCommerceHandler(db)
				auth.POST("/commerce/orders", commerce.CreateOrder)
				auth.POST("/commerce/orders/fulfill", commerce.FulfillOrder)
				auth.POST("/commerce/orders/refund", commerce.RefundOrder)
				auth.GET("/commerce/orders/:id", commerce.GetOrder)
				auth.GET("/commerce/entitlements", commerce.ListEntitlements)
			}
		}

		// 管理员 RBAC（API Key）
		if auditRepo != nil {
			admin := api.Group("/admin")
			admin.Use(middleware.AdminAuth(cfg.AdminAPIKey))
			{
				ah := handlers.NewAuditHandler(auditRepo)
				admin.GET("/audit/logs", ah.List)
				admin.POST("/audit/logs/:id/ack", ah.Ack)
			}
		}
	}
	return r
}
