package routes

import (
	"log/slog"
	"net/http"
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

// unavailable 依赖不可用时返回结构化 503（避免 404）。
func unavailable(reason string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Retry-After", "30")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":       "service unavailable",
			"reason_code": reason,
			"request_id":  middleware.GetRequestID(c),
		})
	}
}

// NewRouter 组装 Gin 引擎: 全局中间件链 + 路由分组。
// 所有业务路由始终注册；依赖缺失时返回 503 而非 404。
func NewRouter(cfg *config.Config, db *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowedOrigins: cfg.CORSAllowedOrigins,
		DevOpen:        cfg.IsDevelopment(),
	}))
	r.MaxMultipartMemory = cfg.MaxImageBytes

	// Liveness / Readiness
	r.GET("/health", handlers.Health())
	r.GET("/livez", handlers.Livez())
	readyChecker := handlers.NewReadyChecker(handlers.ReadyDeps{
		DB:          db,
		ReadyErrors: cfg.ReadyErrors(),
		AppEnv:      cfg.AppEnv,
	})
	r.GET("/ready", handlers.Readyz(readyChecker))
	r.GET("/readyz", handlers.Readyz(readyChecker))
	r.GET("/metrics", middleware.MetricsHandler())

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

	// 限流 / 配额 / nonce：REDIS_URL 存在则用 Redis 共享，否则内存实现。
	// Fail 策略见 middleware 包注释（限流/配额 fail-open，nonce fail-closed）。
	sharedCounter := middleware.SharedCounter(middleware.NewMemorySharedCounter())
	if cfg.RedisURL != "" {
		rc, err := middleware.NewRedisSharedCounter(cfg.RedisURL)
		if err != nil {
			slog.Warn("REDIS_URL 不可用，降级内存 SharedCounter", "err", err)
		} else {
			sharedCounter = rc
			slog.Info("已启用 Redis SharedCounter")
		}
	}
	// AI：device 维度 100/min burst 10；附带 digest 维度防同图刷
	rateLimiter := middleware.NewRateLimiter(100.0/60.0, 10).WithShared(sharedCounter)
	// 每日配额（detect/analyze/value）跨 Pod 一致
	costCounter := middleware.NewDailyCallCounter(middleware.DefaultDailyLimits).WithShared(sharedCounter)
	// 鉴权：IP 维度 20/min burst 5
	ipLimiter := middleware.NewRateLimiter(20.0/60.0, 5).WithShared(sharedCounter)
	// digest 维度独立桶（同图短时重复）
	digestLimiter := middleware.NewRateLimiter(10.0/60.0, 3).WithShared(sharedCounter)

	api := r.Group("/api/v1")
	{
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{"msg": "pong", "db": db != nil, "app_env": cfg.AppEnv, "request_id": middleware.GetRequestID(c)})
		})

		// 可信时间（公开，带签名）
		timeHandler := handlers.NewTimeHandler(cfg.JWTSecret)
		api.GET("/time", timeHandler.GetTime)

		// 设备鉴权：始终注册；无 DB 时 503
		if deviceRepo != nil {
			authHandler := handlers.NewAuthHandlerFull(
				deviceRepo, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTIssuer, cfg.JWTAudience,
			)
			api.POST("/auth/device", middleware.RateLimitByIP(ipLimiter), authHandler.DeviceAuth)
		} else {
			api.POST("/auth/device", unavailable("db_unavailable"))
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

			errHandler := handlers.NewErrorReportHandler()
			auth.POST("/errors/report", errHandler.Report)

			product := handlers.NewProductHandler()
			auth.GET("/ranking/daily", product.RankingDaily)
			auth.POST("/pvp/match", product.PvPMatch)
			auth.POST("/pvp/result", product.PvPReport)
			auth.GET("/social/friends", product.FriendsList)
			auth.POST("/social/share", product.ShareCreate)
			auth.GET("/ops/metrics-summary", product.OpsMetrics)

			ai := auth.Group("")
			// device + digest 多维限流（account 维度在有 account_id 时由 RateLimitByAccount 扩展）
			ai.Use(middleware.RateLimitByDevice(rateLimiter))
			ai.Use(middleware.RateLimitByDigest(digestLimiter))
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

			// 同步：始终注册
			if animalRepo != nil && auditService != nil {
				syncHandler := handlers.NewSyncHandlerFull(animalRepo, auditService, inferenceRepo)
				auth.POST("/sync/animal", syncHandler.SyncAnimal)
				auth.POST("/sync/animals", syncHandler.SyncAnimalsBatch)
				auth.GET("/sync/animals", syncHandler.PullAnimals)
			} else {
				auth.POST("/sync/animal", unavailable("db_unavailable"))
				auth.POST("/sync/animals", unavailable("db_unavailable"))
				auth.GET("/sync/animals", unavailable("db_unavailable"))
			}

			// 隐私 / 安全 / 商业化
			if db != nil && deviceRepo != nil {
				privacy := handlers.NewPrivacyHandler(db, deviceRepo, animalRepo, inferenceRepo, auditRepo)
				auth.POST("/privacy/consent", privacy.PutConsent)
				auth.POST("/privacy/export", privacy.ExportData)
				auth.POST("/privacy/delete", privacy.DeleteData)
				auth.GET("/privacy/requests/:id", privacy.GetDataRequest)

				sec := handlers.NewSecurityHandler(db, auditRepo, sharedCounter)
				auth.POST("/security/report", sec.Report)

				commerce := handlers.NewCommerceHandler(db)
				auth.POST("/commerce/orders", commerce.CreateOrder)
				auth.POST("/commerce/orders/fulfill", commerce.FulfillOrder)
				auth.POST("/commerce/orders/refund", commerce.RefundOrder)
				auth.GET("/commerce/orders/:id", commerce.GetOrder)
				auth.GET("/commerce/entitlements", commerce.ListEntitlements)
			} else {
				auth.POST("/privacy/consent", unavailable("db_unavailable"))
				auth.POST("/privacy/export", unavailable("db_unavailable"))
				auth.POST("/privacy/delete", unavailable("db_unavailable"))
				auth.GET("/privacy/requests/:id", unavailable("db_unavailable"))
				auth.POST("/security/report", unavailable("db_unavailable"))
				auth.POST("/commerce/orders", unavailable("db_unavailable"))
				auth.POST("/commerce/orders/fulfill", unavailable("db_unavailable"))
				auth.POST("/commerce/orders/refund", unavailable("db_unavailable"))
				auth.GET("/commerce/orders/:id", unavailable("db_unavailable"))
				auth.GET("/commerce/entitlements", unavailable("db_unavailable"))
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
		} else {
			admin := api.Group("/admin")
			admin.Use(middleware.AdminAuth(cfg.AdminAPIKey))
			admin.GET("/audit/logs", unavailable("db_unavailable"))
			admin.POST("/audit/logs/:id/ack", unavailable("db_unavailable"))
		}
	}
	return r
}
