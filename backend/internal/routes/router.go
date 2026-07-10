package routes

import (
	"log/slog"
	"net/http"

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
		middleware.AbortUnavailable(c, reason, "service unavailable", 30)
	}
}

// NewRouter 组装 Gin 引擎: 全局中间件链 + 路由分组。
// 所有业务路由始终注册；依赖缺失时返回 503 而非 404。
func NewRouter(cfg *config.Config, db *gorm.DB) *gin.Engine {
	r := gin.New()
	// 可信代理：仅信任配置的上游，防止伪造 X-Forwarded-For 绕过 IP 限流
	if len(cfg.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
			// 配置错误不应静默吞掉；开发可继续，生产启动前应校验
			_ = r.SetTrustedProxies(nil)
		}
	} else {
		// 未配置时不信任任何代理头，ClientIP 使用直连地址
		_ = r.SetTrustedProxies(nil)
	}
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowedOrigins: cfg.CORSAllowedOrigins,
		DevOpen:        cfg.IsDevelopment(),
	}))
	// 全局 body 硬上限（可选兜底，略大于最大图片上传）。
	r.Use(middleware.GlobalBodyLimit(middleware.MaxBodyGlobal))
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
	// AP-036: /metrics is NOT on the public Ingress-facing engine.
	// Scrape the dedicated metrics server on METRICS_ADDR (default :9090).
	// Explicit 404 keeps probes/scanners from learning a 200 endpoint.
	r.GET("/metrics", func(c *gin.Context) {
		c.Status(http.StatusNotFound)
	})

	mockAllowed := cfg.MockAllowed()
	geoProvider, weatherProvider, visionProvider, llmProvider := services.NewProvidersFromConfig(cfg.Upstream)

	thirdParty := &cfg.ThirdParty
	geoService := services.NewGeoServiceWithProvider(thirdParty, mockAllowed, geoProvider)
	weatherService := services.NewWeatherServiceWithProvider(thirdParty, mockAllowed, weatherProvider)
	aiService := services.NewAIServiceWithProviders(thirdParty, mockAllowed, visionProvider, llmProvider)

	var deviceRepo *repo.DeviceRepo
	var animalRepo *repo.AnimalRepo
	var auditService *services.AuditService
	var auditRepo *repo.AuditLogRepo
	var inferenceRepo *repo.InferenceRepo
	var accountRepo *repo.AccountRepo
	if db != nil {
		deviceRepo = repo.NewDeviceRepo(db)
		animalRepo = repo.NewAnimalRepo(db)
		auditRepo = repo.NewAuditLogRepo(db)
		auditService = services.NewAuditService(animalRepo, auditRepo)
		inferenceRepo = repo.NewInferenceRepo(db)
		accountRepo = repo.NewAccountRepo(db, cfg.JWTSecret)
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
		var authHandler *handlers.AuthHandler
		var accountHandler *handlers.AccountHandler
		if deviceRepo != nil {
			authHandler = handlers.NewAuthHandlerFull(
				deviceRepo, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTIssuer, cfg.JWTAudience,
			)
			api.POST("/auth/device",
				middleware.RateLimitByIP(ipLimiter),
				middleware.BodyLimit(middleware.MaxBodyDefault),
				authHandler.DeviceAuth,
			)
		} else {
			api.POST("/auth/device", unavailable("db_unavailable"))
		}
		if deviceRepo != nil && accountRepo != nil {
			accountHandler = handlers.NewAccountHandler(
				deviceRepo, accountRepo, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTIssuer, cfg.JWTAudience,
			)
			api.POST("/auth/login",
				middleware.RateLimitByIP(ipLimiter),
				middleware.BodyLimit(middleware.MaxBodyDefault),
				accountHandler.Login,
			)
		} else {
			api.POST("/auth/login", unavailable("db_unavailable"))
		}

		// JWT
		var checker middleware.DeviceChecker
		if deviceRepo != nil {
			checker = deviceChecker{repo: deviceRepo}
		}
		auth := api.Group("")
		auth.Use(middleware.JWTAuthWithConfig(middleware.JWTAuthConfig{
			Secret:         cfg.JWTSecret,
			PreviousSecret: cfg.JWTSecretPrevious,
			Issuer:         cfg.JWTIssuer,
			Audience:       cfg.JWTAudience,
			Checker:        checker,
		}))
		{
			geoHandler := handlers.NewGeoHandler(geoService)
			weatherHandler := handlers.NewWeatherHandler(weatherService)
			auth.GET("/geo/city", geoHandler.GetCity)
			auth.GET("/weather/week", weatherHandler.GetWeek)

			errHandler := handlers.NewErrorReportHandler()
			auth.POST("/errors/report", middleware.BodyLimit(middleware.MaxBodyErrorReport), errHandler.Report)

			// Analytics funnel ingest (privacy-safe; no photos/tokens/precise coords)
			analyticsHandler := handlers.NewAnalyticsHandler()
			auth.POST("/analytics/events", middleware.BodyLimit(middleware.MaxBodyDefault), analyticsHandler.Ingest)

			// 账号绑定 / 设备管理
			if accountHandler != nil {
				auth.POST("/auth/bind", accountHandler.Bind)
				auth.POST("/auth/logout", accountHandler.Logout)
				auth.GET("/auth/devices", accountHandler.ListDevices)
				auth.POST("/auth/devices/revoke", accountHandler.RevokeDevice)
				auth.GET("/auth/account", accountHandler.GetAccount)
			} else {
				auth.POST("/auth/bind", unavailable("db_unavailable"))
				auth.POST("/auth/logout", unavailable("db_unavailable"))
				auth.GET("/auth/devices", unavailable("db_unavailable"))
				auth.POST("/auth/devices/revoke", unavailable("db_unavailable"))
				auth.GET("/auth/account", unavailable("db_unavailable"))
			}

			product := handlers.NewProductHandlerWithOptions(handlers.ProductOptions{
				Flags:    cfg.FeatureFlags,
				OpsToken: cfg.OpsToken,
			})
			auth.GET("/ranking/daily", product.RankingDaily)
			auth.POST("/pvp/match", middleware.BodyLimit(middleware.MaxBodyDefault), product.PvPMatch)
			auth.POST("/pvp/result", middleware.BodyLimit(middleware.MaxBodyDefault), product.PvPReport)
			auth.GET("/social/friends", product.FriendsList)
			auth.POST("/social/share", middleware.BodyLimit(middleware.MaxBodyDefault), product.ShareCreate)
			auth.GET("/ops/metrics-summary", product.OpsMetrics)

			ai := auth.Group("")
			// device + digest 多维限流（account 维度在有 account_id 时由 RateLimitByAccount 扩展）
			ai.Use(middleware.RateLimitByDevice(rateLimiter))
			ai.Use(middleware.RateLimitByDigest(digestLimiter))
			{
				visionHandler := handlers.NewVisionHandlerWithOptions(aiService, handlers.VisionHandlerOptions{
					InferenceRepo:         inferenceRepo,
					DeviceRepo:            deviceRepo,
					MaxBytes:              cfg.MaxImageBytes,
					MaxPixels:             cfg.MaxImagePixels,
					RequireConsent:        cfg.IsProduction(),
					ConsentVersion:        "v1",
					ProviderNoTrainPolicy: cfg.ProviderNoTrainPolicy,
					AllowSafetyFixture:    cfg.IsDevelopment() || cfg.MockAllowed(),
				})
				valueHandler := handlers.NewValueHandlerWithRepo(aiService, inferenceRepo)
				ai.POST("/vision/detect", middleware.CostLimitByType(costCounter, "detect"), visionHandler.Detect)
				ai.POST("/vision/analyze", middleware.CostLimitByType(costCounter, "analyze"), visionHandler.Analyze)
				ai.POST("/value/generate",
					middleware.BodyLimit(middleware.MaxBodyDefault),
					middleware.CostLimitByType(costCounter, "value"),
					valueHandler.Generate,
				)
			}

			// 同步：始终注册
			if animalRepo != nil && auditService != nil {
				syncHandler := handlers.NewSyncHandlerFull(animalRepo, auditService, inferenceRepo)
				auth.POST("/sync/animal", middleware.BodyLimit(middleware.MaxBodyDefault), syncHandler.SyncAnimal)
				auth.POST("/sync/animals", middleware.BodyLimit(middleware.MaxBodySyncBatch), syncHandler.SyncAnimalsBatch)
				auth.GET("/sync/animals", syncHandler.PullAnimals)
			} else {
				auth.POST("/sync/animal", unavailable("db_unavailable"))
				auth.POST("/sync/animals", unavailable("db_unavailable"))
				auth.GET("/sync/animals", unavailable("db_unavailable"))
			}

			// 隐私 / 安全 / 商业化 / 内容审核
			safetyH := handlers.NewSafetyHandler(db, cfg.StrictMinorDefaults)
			auth.GET("/account/defaults", safetyH.AccountDefaults)
			// safety report only needs structured metadata; always registered
			auth.POST("/safety/report", middleware.BodyLimit(middleware.MaxBodyDefault), safetyH.Report)
			if db != nil && deviceRepo != nil {
				privacy := handlers.NewPrivacyHandler(db, deviceRepo, animalRepo, inferenceRepo, auditRepo)
				auth.POST("/privacy/consent", middleware.BodyLimit(middleware.MaxBodyDefault), privacy.PutConsent)
				auth.POST("/privacy/export", middleware.BodyLimit(middleware.MaxBodyDefault), privacy.ExportData)
				auth.POST("/privacy/delete", middleware.BodyLimit(middleware.MaxBodyDefault), privacy.DeleteData)
				auth.GET("/privacy/requests/:id", privacy.GetDataRequest)

				sec := handlers.NewSecurityHandler(db, auditRepo, sharedCounter)
				auth.POST("/security/report", middleware.BodyLimit(middleware.MaxBodyDefault), sec.Report)

				commerce := handlers.NewCommerceHandler(db)
				auth.POST("/commerce/orders", middleware.BodyLimit(middleware.MaxBodyDefault), commerce.CreateOrder)
				auth.POST("/commerce/orders/fulfill", middleware.BodyLimit(middleware.MaxBodyReceipt), commerce.FulfillOrder)
				auth.POST("/commerce/orders/refund", middleware.BodyLimit(middleware.MaxBodyDefault), commerce.RefundOrder)
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

				if db != nil {
					commerceAdmin := handlers.NewCommerceHandlerWithOptions(db, handlers.CommerceOptions{
						Production:  cfg.IsProduction(),
						Enabled:     cfg.CommerceEnabled,
						StoreVerify: cfg.CommerceStoreVerify,
					})
					admin.POST("/commerce/orders/refund", commerceAdmin.AdminRefundOrder)
					admin.POST("/commerce/webhooks/refund", commerceAdmin.WebhookRefundOrder)
				} else {
					admin.POST("/commerce/orders/refund", unavailable("db_unavailable"))
					admin.POST("/commerce/webhooks/refund", unavailable("db_unavailable"))
				}
			}
		} else {
			admin := api.Group("/admin")
			admin.Use(middleware.AdminAuth(cfg.AdminAPIKey))
			admin.GET("/audit/logs", unavailable("db_unavailable"))
			admin.POST("/audit/logs/:id/ack", unavailable("db_unavailable"))
			admin.POST("/commerce/orders/refund", unavailable("db_unavailable"))
			admin.POST("/commerce/webhooks/refund", unavailable("db_unavailable"))
		}
	}
	return r
}
