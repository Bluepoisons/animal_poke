package routes

import (
	"log/slog"
	"net/http"
	"strings"

	"animalpoke/backend/internal/admin"
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
	aiService = aiService.WithStatsSecrets(cfg.StatsHMACKey, cfg.StatsHMACKeyPrevious)

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
		accountRepo = repo.NewAccountRepoWithPeppers(db, cfg.AccountTokenPepper, cfg.AccountTokenPepperPrevious)
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
		timeHandler := handlers.NewTimeHandler(cfg.TimeSigningKey)
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
			// mock_oauth 仅 development/test 且 AUTH_MOCK_OAUTH_ENABLED 默认开启时可用（AP-063）
			accountHandler = handlers.NewAccountHandler(
				deviceRepo, accountRepo, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTIssuer, cfg.JWTAudience,
				cfg.AuthMockOAuthEnabled,
			)
			accountHandler.SetRefreshPolicy(cfg.JWTRefreshAbsoluteTTL, cfg.JWTRefreshIdleTTL)
			api.POST("/auth/login",
				middleware.RateLimitByIP(ipLimiter),
				middleware.BodyLimit(middleware.MaxBodyDefault),
				accountHandler.Login,
			)
			// AP-078: refresh 无需 access JWT；独立限流
			api.POST("/auth/refresh",
				middleware.RateLimitByIP(ipLimiter),
				middleware.BodyLimit(middleware.MaxBodyDefault),
				accountHandler.Refresh,
			)
		} else {
			api.POST("/auth/login", unavailable("db_unavailable"))
			api.POST("/auth/refresh", unavailable("db_unavailable"))
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
			if db != nil {
				rankH := handlers.NewRankingHandler(db, deviceRepo, cfg.FeatureFlags.Ranking)
				auth.GET("/ranking/daily", rankH.Daily)
				auth.POST("/ranking/score", middleware.BodyLimit(middleware.MaxBodyDefault), rankH.ReportScore)
				auth.POST("/ranking/settle", middleware.BodyLimit(middleware.MaxBodyDefault), rankH.Settle)
			} else {
				auth.GET("/ranking/daily", product.RankingDaily)
			}
			if db != nil {
				pvpH := handlers.NewPvPHandler(db, deviceRepo, cfg.FeatureFlags.PvP)
				auth.POST("/pvp/match", middleware.BodyLimit(middleware.MaxBodyDefault), pvpH.Match)
				auth.POST("/pvp/result", middleware.BodyLimit(middleware.MaxBodyDefault), pvpH.Result)
				auth.POST("/pvp/cancel", middleware.BodyLimit(middleware.MaxBodyDefault), pvpH.Cancel)
			} else {
				auth.POST("/pvp/match", middleware.BodyLimit(middleware.MaxBodyDefault), product.PvPMatch)
				auth.POST("/pvp/result", middleware.BodyLimit(middleware.MaxBodyDefault), product.PvPReport)
			}
			// AP-083 社交：有 DB 时挂载完整图谱；否则保留 product 骨架（flag 关 501 / 无库 503）
			if db != nil && animalRepo != nil {
				socialH := handlers.NewSocialHandler(handlers.SocialOptions{
					Flags:   cfg.FeatureFlags,
					Social:  repo.NewSocialRepo(db),
					Animals: animalRepo,
				})
				socialG := auth.Group("/social")
				// 社交写路径限流（防刷好友/举报）
				socialG.Use(middleware.RateLimitByDevice(rateLimiter))
				{
					socialG.GET("/friends", socialH.FriendsList)
					socialG.GET("/friends/requests", socialH.FriendRequestsList)
					socialG.POST("/friends/request", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.FriendRequestCreate)
					socialG.POST("/friends/accept", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.FriendRequestAccept)
					socialG.POST("/friends/reject", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.FriendRequestReject)
					socialG.POST("/friends/cancel", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.FriendRequestCancel)
					socialG.POST("/friends/remove", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.FriendRemove)
					socialG.POST("/block", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.BlockUser)
					socialG.POST("/unblock", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.UnblockUser)
					socialG.GET("/blocks", socialH.ListBlocks)
					socialG.POST("/mute", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.MuteUser)
					socialG.POST("/unmute", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.UnmuteUser)
					socialG.POST("/report", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.ReportUser)
					socialG.GET("/search", socialH.SearchUsers)
					socialG.GET("/settings", socialH.GetSettings)
					socialG.PATCH("/settings", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.PatchSettings)
					socialG.POST("/share", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.ShareCreate)
					socialG.GET("/share/:token", socialH.ShareGet)
					socialG.POST("/share/:token/revoke", middleware.BodyLimit(middleware.MaxBodyDefault), socialH.ShareRevoke)
				}
			} else {
				auth.GET("/social/friends", product.FriendsList)
				auth.POST("/social/share", middleware.BodyLimit(middleware.MaxBodyDefault), product.ShareCreate)
			}
			auth.GET("/ops/metrics-summary", product.OpsMetrics)
			// AP-059 versioned game config (read for all auth; write/rollback ops-gated)
			auth.GET("/config/game", product.GameConfigGet)
			auth.PUT("/ops/game-config", middleware.BodyLimit(middleware.MaxBodyDefault), product.GameConfigPut)
			auth.POST("/ops/game-config/rollback", middleware.BodyLimit(middleware.MaxBodyDefault), product.GameConfigRollback)

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
				// AP-090: 单只收藏详情 / 编辑 / 删除（乐观锁）
				auth.GET("/sync/animals/:uuid", syncHandler.GetAnimalDetail)
				auth.PATCH("/sync/animals/:uuid", middleware.BodyLimit(middleware.MaxBodyDefault), syncHandler.PatchAnimal)
				auth.DELETE("/sync/animals/:uuid", syncHandler.DeleteAnimal)
				// 别名：/collection/:uuid
				auth.GET("/collection/:uuid", syncHandler.GetAnimalDetail)
				auth.PATCH("/collection/:uuid", middleware.BodyLimit(middleware.MaxBodyDefault), syncHandler.PatchAnimal)
				auth.DELETE("/collection/:uuid", syncHandler.DeleteAnimal)
			} else {
				auth.POST("/sync/animal", unavailable("db_unavailable"))
				auth.POST("/sync/animals", unavailable("db_unavailable"))
				auth.GET("/sync/animals", unavailable("db_unavailable"))
				auth.GET("/sync/animals/:uuid", unavailable("db_unavailable"))
				auth.PATCH("/sync/animals/:uuid", unavailable("db_unavailable"))
				auth.DELETE("/sync/animals/:uuid", unavailable("db_unavailable"))
				auth.GET("/collection/:uuid", unavailable("db_unavailable"))
				auth.PATCH("/collection/:uuid", unavailable("db_unavailable"))
				auth.DELETE("/collection/:uuid", unavailable("db_unavailable"))
			}

			// 隐私 / 安全 / 商业化 / 内容审核
			safetyH := handlers.NewSafetyHandler(db, cfg.StrictMinorDefaults)
			auth.GET("/account/defaults", safetyH.AccountDefaults)
			// safety report only needs structured metadata; always registered
			auth.POST("/safety/report", middleware.BodyLimit(middleware.MaxBodyDefault), safetyH.Report)
			if db != nil && deviceRepo != nil {
				privacy := handlers.NewPrivacyHandlerFull(db, deviceRepo, animalRepo, inferenceRepo, auditRepo, accountRepo)
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

				// AP-082 钱包 / 库存 / 不可变流水
				walletRepo := repo.NewWalletRepo(db)
				walletH := handlers.NewWalletHandler(walletRepo)
				auth.GET("/wallet", walletH.GetWallet)
				auth.POST("/wallet/credit", middleware.BodyLimit(middleware.MaxBodyDefault), walletH.Credit)
				auth.POST("/wallet/debit", middleware.BodyLimit(middleware.MaxBodyDefault), walletH.Debit)
				auth.GET("/wallet/ledger", walletH.ListLedger)
				auth.POST("/wallet/reconcile", middleware.BodyLimit(middleware.MaxBodyDefault), walletH.Reconcile)
				auth.GET("/inventory", walletH.GetInventory)
				auth.POST("/inventory/grant", middleware.BodyLimit(middleware.MaxBodyDefault), walletH.GrantInventory)
				auth.POST("/inventory/consume", middleware.BodyLimit(middleware.MaxBodyDefault), walletH.ConsumeInventory)
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
				auth.GET("/wallet", unavailable("db_unavailable"))
				auth.POST("/wallet/credit", unavailable("db_unavailable"))
				auth.POST("/wallet/debit", unavailable("db_unavailable"))
				auth.GET("/wallet/ledger", unavailable("db_unavailable"))
				auth.POST("/wallet/reconcile", unavailable("db_unavailable"))
				auth.GET("/inventory", unavailable("db_unavailable"))
				auth.POST("/inventory/grant", unavailable("db_unavailable"))
				auth.POST("/inventory/consume", unavailable("db_unavailable"))
			}
		}

		// 管理员 RBAC（AP-085）：短期 Admin JWT + 可选 break-glass；独立限流
		adminEnv := strings.ToLower(cfg.AppEnv)
		if adminEnv == "" {
			adminEnv = "development"
		}
		adminSessions := admin.NewSessionStore(db)
		adminSessions.RevokeGrace = cfg.AdminSessionRevokeGrace
		adminTokens := admin.NewTokenService(admin.TokenConfig{
			Secret:         cfg.AdminJWTSecret,
			PreviousSecret: cfg.AdminJWTSecretPrevious,
			Issuer:         cfg.AdminJWTIssuer,
			Audience:       "animal-poke-admin-" + adminEnv,
			Env:            adminEnv,
			TTL:            cfg.AdminTokenTTL,
		}, adminSessions)
		adminAuditor := admin.NewActionAuditor(db, cfg.AdminJWTSecret)
		adminAuthCfg := middleware.AdminAuthConfig{
			Tokens:               adminTokens,
			Sessions:             adminSessions,
			Auditor:              adminAuditor,
			AdminAPIKey:          cfg.AdminAPIKey,
			BreakGlassEnabled:    cfg.AdminBreakGlassEnabled,
			Production:           cfg.IsProduction(),
			Env:                  adminEnv,
			RequireReasonOnWrite: true,
		}
		// 管理端独立限流：IP 30/min + actor 60/min（fail-closed 倾向：burst 小）
		adminIPLimiter := middleware.NewRateLimiter(30.0/60.0, 10).WithShared(sharedCounter)
		adminActorLimiter := middleware.NewRateLimiter(60.0/60.0, 20).WithShared(sharedCounter)

		adminH := handlers.NewAdminHandler(handlers.AdminHandlerOptions{
			Tokens:      adminTokens,
			Sessions:    adminSessions,
			Auditor:     adminAuditor,
			DB:          db,
			AdminAPIKey: cfg.AdminAPIKey,
			BreakGlass:  cfg.AdminBreakGlassEnabled,
			Production:  cfg.IsProduction(),
			DevIssueKey: cfg.AdminDevIssueSecret,
			Env:         adminEnv,
		})

		adminGroup := api.Group("/admin")
		adminGroup.Use(middleware.RateLimitByIP(adminIPLimiter))
		adminGroup.Use(middleware.RateLimitMulti(adminActorLimiter, func(c *gin.Context) []string {
			if a := middleware.GetAdminActor(c); a != nil && a.ActorID != "" {
				return []string{"admin-actor:" + a.ActorID}
			}
			return nil
		}))
		{
			// token 签发：可选鉴权（dev_secret / break-glass / super）
			adminGroup.POST("/auth/token", middleware.BodyLimit(middleware.MaxBodyDefault),
				middleware.OptionalAdminAuth(adminAuthCfg), adminH.IssueToken)

			secured := adminGroup.Group("")
			secured.Use(middleware.AdminAuthRBAC(adminAuthCfg))
			{
				secured.POST("/sessions/revoke", middleware.BodyLimit(middleware.MaxBodyDefault),
					middleware.RequireAdminPermission(admin.PermSessionRevoke, adminAuditor),
					middleware.AdminActionAudit(admin.PermSessionRevoke, adminAuditor),
					adminH.RevokeSession)

				secured.PUT("/config/game", middleware.BodyLimit(middleware.MaxBodyDefault),
					middleware.RequireAdminPermission(admin.PermConfigWrite, adminAuditor),
					adminH.WriteGameConfig)

				secured.GET("/security/reports/:id",
					middleware.RequireAdminPermission(admin.PermSecurityReportMeta, adminAuditor),
					adminH.GetSecurityReport)

				if auditRepo != nil {
					ah := handlers.NewAuditHandler(auditRepo)
					secured.GET("/audit/logs",
						middleware.RequireAdminPermission(admin.PermAuditLogsRead, adminAuditor),
						middleware.AdminActionAudit(admin.PermAuditLogsRead, adminAuditor),
						ah.List)
					secured.POST("/audit/logs/:id/ack",
						middleware.RequireAdminPermission(admin.PermAuditLogsAck, adminAuditor),
						middleware.AdminActionAudit(admin.PermAuditLogsAck, adminAuditor),
						ah.Ack)
				} else {
					secured.GET("/audit/logs", unavailable("db_unavailable"))
					secured.POST("/audit/logs/:id/ack", unavailable("db_unavailable"))
				}

				if db != nil {
					commerceAdmin := handlers.NewCommerceHandlerWithOptions(db, handlers.CommerceOptions{
						Production:  cfg.IsProduction(),
						Enabled:     cfg.CommerceEnabled,
						StoreVerify: cfg.CommerceStoreVerify,
					})
					secured.POST("/commerce/orders/refund",
						middleware.BodyLimit(middleware.MaxBodyDefault),
						middleware.RequireAdminPermission(admin.PermCommerceRefund, adminAuditor),
						middleware.AdminActionAudit(admin.PermCommerceRefund, adminAuditor),
						commerceAdmin.AdminRefundOrder)
					// webhook 仍走 break-glass/密钥或 JWT；平台签名接入前保持与 refund 相同网关
					secured.POST("/commerce/webhooks/refund",
						middleware.BodyLimit(middleware.MaxBodyDefault),
						middleware.RequireAdminPermission(admin.PermCommerceRefund, adminAuditor),
						commerceAdmin.WebhookRefundOrder)
				} else {
					secured.POST("/commerce/orders/refund", unavailable("db_unavailable"))
					secured.POST("/commerce/webhooks/refund", unavailable("db_unavailable"))
				}
			}
		}
	}
	return r
}
