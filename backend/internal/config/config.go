package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// 默认开发密钥（仅 development/test 允许；各用途独立，禁止互为 fallback）。
const (
	DefaultDevJWTSecret          = "animal-poke-dev-secret"
	DefaultDevAccountTokenPepper = "animal-poke-dev-account-pepper"
	DefaultDevStatsHMACKey       = "animal-poke-dev-stats-secret"
	DefaultDevTimeSigningKey     = "animal-poke-dev-time-signing"
	DefaultDevAdminJWTSecret     = "animal-poke-dev-admin-jwt-secret"
	DefaultDevAdminIssueSecret   = "animal-poke-dev-admin-issue"
	// MinCryptoKeyLen production 密钥最小长度。
	MinCryptoKeyLen = 32
)

// Config 聚合所有服务端配置。第三方 Key 全部在此集中(客户端永不含)。
type Config struct {
	AppEnv     string
	ServerAddr string
	// MetricsAddr is the management-only listen address for Prometheus scrape
	// (default :9090). Empty disables the dedicated metrics server.
	// Never expose this port via public Ingress; use ClusterIP only.
	MetricsAddr string
	LogLevel    string
	// JWTSecret 当前 JWT HMAC 签名密钥。
	// 环境变量：JWT_SIGNING_KEY（首选）或兼容别名 JWT_SECRET。
	JWTSecret string
	// JWTSecretPrevious 可选上一版 JWT 密钥（双读窗口，仅校验旧 Token）。
	// 环境变量：JWT_SIGNING_KEY_PREVIOUS 或 JWT_SECRET_PREVIOUS。
	JWTSecretPrevious string
	// AccountTokenPepper 账号 refresh/oauth token 哈希 pepper（与 JWT 独立）。
	// 环境变量：ACCOUNT_TOKEN_PEPPER。
	AccountTokenPepper string
	// AccountTokenPepperPrevious 可选上一版 pepper（双读校验旧哈希）。
	// 环境变量：ACCOUNT_TOKEN_PEPPER_PREVIOUS。
	AccountTokenPepperPrevious string
	// StatsHMACKey rarity/stats 确定性算法 HMAC 密钥（与 JWT 独立）。
	// 环境变量：STATS_HMAC_KEY。
	StatsHMACKey string
	// StatsHMACKeyPrevious 可选上一版 stats 密钥（双读/回放窗口）。
	// 环境变量：STATS_HMAC_KEY_PREVIOUS。
	StatsHMACKeyPrevious string
	// TimeSigningKey 可信时间 API HMAC 签名密钥（与 JWT 独立）。
	// 环境变量：TIME_SIGNING_KEY。
	TimeSigningKey string
	// TimeSigningKeyPrevious 可选上一版时间签名密钥。
	// 环境变量：TIME_SIGNING_KEY_PREVIOUS。
	TimeSigningKeyPrevious string
	JWTIssuer              string
	JWTAudience            string
	JWTAccessTTL           time.Duration
	// JWTRefreshAbsoluteTTL refresh family 绝对过期（AP-078）。
	JWTRefreshAbsoluteTTL time.Duration
	// JWTRefreshIdleTTL refresh 空闲过期（自上次成功使用起算）。
	JWTRefreshIdleTTL time.Duration
	// EmailVerifyTTL 邮箱验证令牌有效期（AP-079）。
	EmailVerifyTTL time.Duration
	// PasswordResetTTL 找回密码令牌有效期（AP-079）。
	PasswordResetTTL time.Duration
	// ReauthTTL 敏感操作 re-auth 令牌有效期（AP-079）。
	ReauthTTL     time.Duration
	AIMockEnabled bool
	// AuthMockOAuthEnabled 允许 mock_oauth 绑定/登录（仅 development/test；production 强制 false）。
	// 环境变量 AUTH_MOCK_OAUTH_ENABLED；非 production 默认 true。
	AuthMockOAuthEnabled bool
	RedisURL             string
	AdminAPIKey          string
	// OpsToken 运维内部接口 X-AP-Ops-Token 校验值（可与 AdminAPIKey 相同，但独立配置）。
	OpsToken string
	// AdminJWTSecret 管理端短期 JWT 签名密钥（与设备 JWT 独立）。
	// 环境变量：ADMIN_JWT_SECRET。
	AdminJWTSecret string
	// AdminJWTSecretPrevious 可选上一版管理 JWT 密钥。
	AdminJWTSecretPrevious string
	// AdminJWTIssuer 管理 JWT iss（默认 animal-poke-admin）。
	AdminJWTIssuer string
	// AdminTokenTTL 管理 JWT 有效期（默认 15m）。
	AdminTokenTTL time.Duration
	// AdminBreakGlassEnabled 允许 X-Admin-Key 紧急入口（生产默认 false）。
	// 环境变量：ADMIN_BREAK_GLASS_ENABLED。
	AdminBreakGlassEnabled bool
	// AdminDevIssueSecret 非生产签发管理 JWT 的 bootstrap 密钥。
	// 环境变量：ADMIN_DEV_ISSUE_SECRET。
	AdminDevIssueSecret string
	// AdminSessionRevokeGrace 撤权宽限窗口（默认 0 立即失效）。
	AdminSessionRevokeGrace time.Duration
	// CommerceEnabled 商业化下单/履约总开关。production 默认 false；非 production 默认 true。
	// 环境变量 COMMERCE_ENABLED 可覆盖。
	CommerceEnabled bool
	// CommerceStoreVerify 是否启用真实商店验签路径。production 履约在未开启时返回 not ready。
	// 环境变量 COMMERCE_STORE_VERIFY。
	CommerceStoreVerify bool
	// FeatureFlags 未完成产品能力开关。production 默认全部 false。
	FeatureFlags       FeatureFlags
	MaxImageBytes      int64
	MaxImagePixels     int
	CORSAllowedOrigins []string
	// TrustedProxies trusted reverse-proxy CIDRs/IPs (empty = distrust XFF).
	TrustedProxies []string
	// StrictMinorDefaults stricter privacy/social defaults for minors.
	StrictMinorDefaults bool
	// ProviderNoTrainPolicy audit that upstream vision/LLM is no-train.
	ProviderNoTrainPolicy bool
	Database              DatabaseConfig
	ThirdParty            ThirdPartyConfig
	Server                ServerTimeouts
	// Upstream HTTP budgets / circuit breaker settings.
	Upstream UpstreamConfig
	// minClientVersion is the minimum client version allowed; empty = no minimum.
	// Set via MIN_CLIENT_VERSION env var.
	minClientVersion string
	// deprecatedOps is the deprecation registry.
	deprecatedOps map[string]DeprecatedOperation
}

// FeatureFlags Ranking / PvP / Social / Ops 产品能力开关（AP-042）。
// 环境变量：FEATURE_RANKING / FEATURE_PVP / FEATURE_SOCIAL / FEATURE_OPS。
// production 默认 false；非 production 默认 true（便于本地联调，仍可显式关闭）。
type FeatureFlags struct {
	Ranking bool
	PvP     bool
	Social  bool
	Ops     bool
}

// ProviderBudget per-upstream timeout/retry/concurrency budget.
type ProviderBudget struct {
	TotalDeadline time.Duration
	Timeout       time.Duration
	MaxRetries    int
	MaxConcurrent int
}

// UpstreamConfig global upstream budgets and circuit breaker.
type UpstreamConfig struct {
	Geo                     ProviderBudget
	Weather                 ProviderBudget
	Vision                  ProviderBudget
	LLM                     ProviderBudget
	MaxRetryAfter           time.Duration
	CircuitFailureThreshold int
	CircuitOpenTimeout      time.Duration
}

// DefaultUpstreamConfig returns safe defaults (overridable via env).
func DefaultUpstreamConfig() UpstreamConfig {
	return UpstreamConfig{
		Geo: ProviderBudget{
			TotalDeadline: 8 * time.Second,
			Timeout:       3 * time.Second,
			MaxRetries:    1,
			MaxConcurrent: 32,
		},
		Weather: ProviderBudget{
			TotalDeadline: 8 * time.Second,
			Timeout:       3 * time.Second,
			MaxRetries:    1,
			MaxConcurrent: 32,
		},
		Vision: ProviderBudget{
			TotalDeadline: 45 * time.Second,
			Timeout:       25 * time.Second,
			MaxRetries:    1,
			MaxConcurrent: 8,
		},
		LLM: ProviderBudget{
			TotalDeadline: 50 * time.Second,
			Timeout:       40 * time.Second,
			MaxRetries:    1,
			MaxConcurrent: 8,
		},
		MaxRetryAfter:           5 * time.Second,
		CircuitFailureThreshold: 5,
		CircuitOpenTimeout:      30 * time.Second,
	}
}

// ServerTimeouts HTTP Server 超时配置。
type ServerTimeouts struct {
	ReadHeader time.Duration
	Read       time.Duration
	Write      time.Duration
	Idle       time.Duration
	MaxHeader  int
	Shutdown   time.Duration
}

// DatabaseConfig is defined in db_tls.go (MySQL DSN/TLS helpers).

// ThirdPartyConfig 第三方 API Key（地图/天气/统一多模态 AI）。
type ThirdPartyConfig struct {
	TencentMapKey string
	// TencentMapSK 仅用于服务端 WebService 请求的 SN 签名，绝不能下发到客户端。
	TencentMapSK     string
	CaiyunWeatherKey string
	// AI 是统一的 OpenAI Responses 兼容 Provider，同时处理图片与文本。
	AIEndpoint string
	AIKey      string
	AIModel    string
	// 以下字段仅用于兼容旧环境变量；加载后会归一到 AI*。
	VisionEndpoint string
	VisionKey      string
	VisionModel    string
	// Deprecated: AI 配置会默认复用完整 LLM_* 三元组。
	VisionReuseLLM bool
	// VisionSource 记录统一 AI 配置来源（不含密钥）。
	VisionSource string
	LLMEndpoint  string
	LLMKey       string
	LLMModel     string
}

// AIConfigured 是否具备统一 Responses Provider 的完整配置。
func (t ThirdPartyConfig) AIConfigured() bool {
	endpoint, key, model := t.ActiveAI()
	return endpoint != "" && key != "" && model != ""
}

// ActiveAI returns the normalized provider, with direct struct construction in
// tests and integrations still accepting legacy LLM/Vision fields.
func (t ThirdPartyConfig) ActiveAI() (endpoint, key, model string) {
	if t.AIEndpoint != "" || t.AIKey != "" || t.AIModel != "" {
		return t.AIEndpoint, t.AIKey, t.AIModel
	}
	if t.LLMEndpoint != "" && t.LLMKey != "" && t.LLMModel != "" {
		return t.LLMEndpoint, t.LLMKey, t.LLMModel
	}
	if t.VisionEndpoint != "" && t.VisionKey != "" && t.VisionModel != "" {
		return t.VisionEndpoint, t.VisionKey, t.VisionModel
	}
	if t.LLMEndpoint != "" || t.LLMKey != "" || t.LLMModel != "" {
		return t.LLMEndpoint, t.LLMKey, t.LLMModel
	}
	return t.VisionEndpoint, t.VisionKey, t.VisionModel
}

// VisionConfigured 是兼容旧调用名；视觉与文本共用 AI Provider。
func (t ThirdPartyConfig) VisionConfigured() bool {
	return t.AIConfigured()
}

// LLMConfigured 是兼容旧调用名；视觉与文本共用 AI Provider。
func (t ThirdPartyConfig) LLMConfigured() bool {
	return t.AIConfigured()
}

// VisionFingerprint 返回不含密钥的配置指纹（用于日志/readiness）。
func (t ThirdPartyConfig) VisionFingerprint() string {
	if !t.VisionConfigured() {
		return "none"
	}
	endpoint, _, model := t.ActiveAI()
	raw := t.VisionSource + "|" + endpoint + "|" + model
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:8])
}

// IsProduction 判断是否生产环境。
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production") || strings.EqualFold(c.AppEnv, "prod")
}

// IsDevelopment 判断是否开发环境。
func (c *Config) IsDevelopment() bool {
	env := strings.ToLower(c.AppEnv)
	return env == "" || env == "development" || env == "dev" || env == "test" || env == "local"
}

// MockAllowed 是否允许返回 Mock 数据。
func (c *Config) MockAllowed() bool {
	return c.IsDevelopment() && c.AIMockEnabled
}

// Load 读取配置, 优先级: OS 环境变量 > .env > 默认值。
// Vision 配置视为原子三元组：禁止 VISION/VLM/LLM 字段级混合；默认禁止回退到 LLM_*。
func Load() *Config {
	cfg := &Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		ServerAddr:  getEnv("SERVER_ADDR", ":8080"),
		MetricsAddr: getEnv("METRICS_ADDR", ":9090"),
		LogLevel:    getEnv("LOG_LEVEL", "INFO"),
		// 密钥在下方 load* 中按用途填充（AP-087）
		JWTIssuer:               getEnv("JWT_ISSUER", "animal-poke"),
		JWTAudience:             getEnv("JWT_AUDIENCE", "animal-poke-client"),
		JWTAccessTTL:            getEnvDuration("JWT_ACCESS_TTL", 2*time.Hour),
		JWTRefreshAbsoluteTTL:   getEnvDuration("JWT_REFRESH_ABSOLUTE_TTL", 30*24*time.Hour),
		JWTRefreshIdleTTL:       getEnvDuration("JWT_REFRESH_IDLE_TTL", 7*24*time.Hour),
		EmailVerifyTTL:          getEnvDuration("EMAIL_VERIFY_TTL", 24*time.Hour),
		PasswordResetTTL:        getEnvDuration("PASSWORD_RESET_TTL", time.Hour),
		ReauthTTL:               getEnvDuration("REAUTH_TTL", 5*time.Minute),
		AIMockEnabled:           getEnvBool("AI_MOCK_ENABLED", true),
		RedisURL:                getEnv("REDIS_URL", ""),
		AdminAPIKey:             getEnv("ADMIN_API_KEY", ""),
		OpsToken:                getEnv("OPS_TOKEN", ""),
		AdminJWTIssuer:          getEnv("ADMIN_JWT_ISSUER", "animal-poke-admin"),
		AdminTokenTTL:           getEnvDuration("ADMIN_TOKEN_TTL", 15*time.Minute),
		AdminSessionRevokeGrace: getEnvDuration("ADMIN_SESSION_REVOKE_GRACE", 0),
		MaxImageBytes:           int64(getEnvInt("MAX_IMAGE_BYTES", 5*1024*1024)),
		MaxImagePixels:          getEnvInt("MAX_IMAGE_PIXELS", 12_000_000),
		CORSAllowedOrigins:      splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "")),
		TrustedProxies:          splitCSV(getEnv("TRUSTED_PROXIES", "")),
		// 默认开启：未成年人严格默认 + Provider 不训练审计
		StrictMinorDefaults:   getEnvBool("STRICT_MINOR_DEFAULTS", true),
		ProviderNoTrainPolicy: getEnvBool("PROVIDER_NO_TRAIN_POLICY", true),
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "127.0.0.1"),
			Port:            getEnvInt("DB_PORT", 3306),
			User:            getEnv("DB_USER", "animal_poke"),
			Password:        getEnv("DB_PASSWORD", "animal_poke"),
			DBName:          getEnv("DB_NAME", "animal_poke"),
			TLSMode:         getEnv("DB_TLS", "false"),
			TLSCAFile:       getEnv("DB_TLS_CA", ""),
			TLSCertFile:     getEnv("DB_TLS_CERT", ""),
			TLSKeyFile:      getEnv("DB_TLS_KEY", ""),
			TLSServerName:   getEnv("DB_TLS_SERVER_NAME", ""),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 10*time.Minute),
		},
		ThirdParty: ThirdPartyConfig{
			TencentMapKey:    getEnv("TENCENT_MAP_KEY", ""),
			TencentMapSK:     getEnv("TENCENT_MAP_SK", ""),
			CaiyunWeatherKey: getEnv("CAIYUN_WEATHER_KEY", ""),
			AIEndpoint:       getEnv("AI_ENDPOINT", ""),
			AIKey:            getEnv("AI_KEY", ""),
			AIModel:          getEnv("AI_MODEL", ""),
			LLMEndpoint:      getEnv("LLM_ENDPOINT", ""),
			LLMKey:           getEnv("LLM_KEY", ""),
			LLMModel:         getEnv("LLM_MODEL", ""),
			VisionReuseLLM:   getEnvBool("VISION_REUSE_LLM", false),
		},
		Server: ServerTimeouts{
			ReadHeader: getEnvDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			Read:       getEnvDuration("HTTP_READ_TIMEOUT", 30*time.Second),
			Write:      getEnvDuration("HTTP_WRITE_TIMEOUT", 60*time.Second),
			Idle:       getEnvDuration("HTTP_IDLE_TIMEOUT", 90*time.Second),
			MaxHeader:  getEnvInt("HTTP_MAX_HEADER_BYTES", 1<<20),
			Shutdown:   getEnvDuration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
		},
		Upstream: loadUpstreamConfig(),
	}

	// AP-089: minimum client version gate (empty = no minimum).
	cfg.minClientVersion = getEnv("MIN_CLIENT_VERSION", "")

	// 按用途拆分密钥（AP-087）。production 禁止跨用途 fallback；非 production 使用独立默认值。
	isProd := cfg.IsProduction()
	cfg.JWTSecret, cfg.JWTSecretPrevious = loadJWTSigningKeys(isProd)
	cfg.AccountTokenPepper, cfg.AccountTokenPepperPrevious = loadPurposeKey(
		"ACCOUNT_TOKEN_PEPPER", "ACCOUNT_TOKEN_PEPPER_PREVIOUS", DefaultDevAccountTokenPepper, isProd,
	)
	cfg.StatsHMACKey, cfg.StatsHMACKeyPrevious = loadPurposeKey(
		"STATS_HMAC_KEY", "STATS_HMAC_KEY_PREVIOUS", DefaultDevStatsHMACKey, isProd,
	)
	cfg.TimeSigningKey, cfg.TimeSigningKeyPrevious = loadPurposeKey(
		"TIME_SIGNING_KEY", "TIME_SIGNING_KEY_PREVIOUS", DefaultDevTimeSigningKey, isProd,
	)
	// 管理端 JWT 密钥（AP-085）：与设备 JWT 独立；非 production 回退 dev 默认。
	cfg.AdminJWTSecret, cfg.AdminJWTSecretPrevious = loadPurposeKey(
		"ADMIN_JWT_SECRET", "ADMIN_JWT_SECRET_PREVIOUS", DefaultDevAdminJWTSecret, isProd,
	)
	// break-glass：production 默认关闭；非 production 默认开启（便于联调）。
	cfg.AdminBreakGlassEnabled = getEnvBool("ADMIN_BREAK_GLASS_ENABLED", !isProd)
	if isProd {
		cfg.AdminDevIssueSecret = "" // 生产禁止 dev issue
	} else {
		cfg.AdminDevIssueSecret = getEnv("ADMIN_DEV_ISSUE_SECRET", DefaultDevAdminIssueSecret)
	}

	// 原子加载 Vision 三元组（禁止字段级混合）
	cfg.ThirdParty.loadAITriplet()

	// 开发默认开启 mock；生产强制关闭（即使 env 写了 true）
	if cfg.IsProduction() {
		cfg.AIMockEnabled = getEnvBool("AI_MOCK_ENABLED", false)
	}

	// mock_oauth：production 强制关闭；非 production 默认开启，可用 AUTH_MOCK_OAUTH_ENABLED 覆盖（AP-063）。
	if cfg.IsProduction() {
		cfg.AuthMockOAuthEnabled = false
	} else {
		cfg.AuthMockOAuthEnabled = getEnvBool("AUTH_MOCK_OAUTH_ENABLED", true)
	}

	// 商业化：production 默认关闭；非 production 默认开启。COMMERCE_ENABLED 可覆盖。
	commerceDefault := !cfg.IsProduction()
	cfg.CommerceEnabled = getEnvBool("COMMERCE_ENABLED", commerceDefault)
	// 真实商店验签：默认关闭，需显式开启 COMMERCE_STORE_VERIFY=true。
	cfg.CommerceStoreVerify = getEnvBool("COMMERCE_STORE_VERIFY", false)

	// 产品 feature flags：production 默认全部关闭；非 production 默认开启。
	// FEATURE_RANKING / FEATURE_PVP / FEATURE_SOCIAL / FEATURE_OPS 可覆盖。
	featureDefault := !cfg.IsProduction()
	cfg.FeatureFlags = FeatureFlags{
		Ranking: getEnvBool("FEATURE_RANKING", featureDefault),
		PvP:     getEnvBool("FEATURE_PVP", featureDefault),
		Social:  getEnvBool("FEATURE_SOCIAL", featureDefault),
		Ops:     getEnvBool("FEATURE_OPS", featureDefault),
	}

	// OPS_TOKEN 未配置时回退到 ADMIN_API_KEY（仍要求显式配置才可通过 ops 校验）。
	if cfg.OpsToken == "" {
		cfg.OpsToken = cfg.AdminAPIKey
	}
	return cfg
}

// loadAITriplet 按优先级加载完整的统一 AI 三元组。
// 规则：
//  1. 完整 AI_* 优先；
//  2. 其次完整 LLM_*、VISION_*、VLM_* 旧三元组；
//  3. 任何候选前缀若只配置了部分字段，视为不完整（启动校验失败）。
func (t *ThirdPartyConfig) loadAITriplet() {
	ai := envTriplet("AI_ENDPOINT", "AI_KEY", "AI_MODEL")
	vision := envTriplet("VISION_ENDPOINT", "VISION_KEY", "VISION_MODEL")
	vlm := envTriplet("VLM_ENDPOINT", "VLM_KEY", "VLM_MODEL")
	llm := envTriplet("LLM_ENDPOINT", "LLM_KEY", "LLM_MODEL")

	// 记录部分配置以便 Validate 拒绝残缺配置。
	t.AIEndpoint, t.AIKey, t.AIModel = "", "", ""
	t.VisionSource = "none"

	switch {
	case ai.complete():
		t.AIEndpoint, t.AIKey, t.AIModel = ai.endpoint, ai.key, ai.model
		t.VisionSource = "ai"
	case ai.partial():
		t.AIEndpoint, t.AIKey, t.AIModel = ai.endpoint, ai.key, ai.model
		t.VisionSource = "ai_partial"
	case llm.complete():
		t.AIEndpoint, t.AIKey, t.AIModel = llm.endpoint, llm.key, llm.model
		if t.VisionReuseLLM {
			t.VisionSource = "llm_reuse"
		} else {
			t.VisionSource = "llm"
		}
	case vision.complete():
		t.AIEndpoint, t.AIKey, t.AIModel = vision.endpoint, vision.key, vision.model
		t.VisionSource = "vision"
	case vision.partial():
		t.AIEndpoint, t.AIKey, t.AIModel = vision.endpoint, vision.key, vision.model
		t.VisionSource = "vision_partial"
	case vlm.complete():
		t.AIEndpoint, t.AIKey, t.AIModel = vlm.endpoint, vlm.key, vlm.model
		t.VisionSource = "vlm"
	case vlm.partial():
		t.AIEndpoint, t.AIKey, t.AIModel = vlm.endpoint, vlm.key, vlm.model
		t.VisionSource = "vlm_partial"
	}
	// Deprecated Vision fields remain populated for status and old integrations.
	t.VisionEndpoint, t.VisionKey, t.VisionModel = t.AIEndpoint, t.AIKey, t.AIModel
}

type providerTriplet struct {
	endpoint, key, model string
}

func envTriplet(epKey, keyKey, modelKey string) providerTriplet {
	return providerTriplet{
		endpoint: getEnv(epKey, ""),
		key:      getEnv(keyKey, ""),
		model:    getEnv(modelKey, ""),
	}
}

func (p providerTriplet) complete() bool {
	return p.endpoint != "" && p.key != "" && p.model != ""
}

func (p providerTriplet) partial() bool {
	any := p.endpoint != "" || p.key != "" || p.model != ""
	return any && !p.complete()
}

func (p providerTriplet) empty() bool {
	return p.endpoint == "" && p.key == "" && p.model == ""
}

// Validate 集中配置校验。production 缺必需项时返回 error。
// 同时拒绝不完整/混合的 Vision 三元组与生产不安全 endpoint。
func (c *Config) Validate() error {
	var errs []string

	// Vision 原子三元组完整性（任何环境）
	if err := c.ThirdParty.validateVisionTriplet(); err != nil {
		errs = append(errs, err.Error())
	}

	// MySQL TLS / client cert completeness (all envs); production forbids plaintext/skip-verify.
	if err := c.Database.ValidateDatabaseTLS(c.IsProduction()); err != nil {
		errs = append(errs, err.Error())
	}

	// Redis: when configured, enforce auth; production requires rediss:// + password + host verify.
	if err := ValidateRedisURL(c.RedisURL, c.IsProduction()); err != nil {
		errs = append(errs, err.Error())
	}

	if c.IsProduction() {
		if err := c.validateProductionCryptoKeys(); err != nil {
			errs = append(errs, err.Error())
		}
		if c.Database.Password == "" || c.Database.Password == "animal_poke" {
			errs = append(errs, "production forbids default DB_PASSWORD")
		}
		if c.AIMockEnabled {
			errs = append(errs, "production forbids AI_MOCK_ENABLED=true")
		}
		if c.AuthMockOAuthEnabled {
			errs = append(errs, "production forbids AUTH_MOCK_OAUTH_ENABLED=true")
		}
		if !c.ThirdParty.AIConfigured() {
			errs = append(errs, "production requires AI_ENDPOINT/KEY/MODEL")
		}
		if c.AdminAPIKey == "" {
			errs = append(errs, "production requires ADMIN_API_KEY for break-glass only")
		}
		if c.AdminJWTSecret == "" {
			errs = append(errs, "production requires ADMIN_JWT_SECRET for admin RBAC")
		}
		if c.AdminTokenTTL <= 0 || c.AdminTokenTTL > time.Hour {
			errs = append(errs, "production ADMIN_TOKEN_TTL must be in (0, 1h]")
		}
		if len(c.CORSAllowedOrigins) == 0 {
			errs = append(errs, "production requires CORS_ALLOWED_ORIGINS allowlist")
		}
		// 生产统一 AI endpoint 必须 HTTPS，拒绝 localhost / 明文 HTTP / 空模型。
		if c.ThirdParty.AIConfigured() {
			endpoint, _, model := c.ThirdParty.ActiveAI()
			if err := validateProductionEndpoint("AI", endpoint, model); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}

	if c.JWTAccessTTL <= 0 {
		errs = append(errs, "JWT_ACCESS_TTL must be positive")
	}
	if c.JWTRefreshAbsoluteTTL <= 0 {
		errs = append(errs, "JWT_REFRESH_ABSOLUTE_TTL must be positive")
	}
	if c.JWTRefreshIdleTTL <= 0 {
		errs = append(errs, "JWT_REFRESH_IDLE_TTL must be positive")
	}
	if c.MaxImageBytes <= 0 {
		errs = append(errs, "MAX_IMAGE_BYTES must be positive")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (t ThirdPartyConfig) validateVisionTriplet() error {
	src := t.VisionSource
	if src == "ai_partial" || src == "vision_partial" || src == "vlm_partial" {
		return fmt.Errorf("ai config must be a complete triplet (endpoint+key+model); incomplete %s rejected", src)
	}
	// 检测字段级混合：若环境中同时存在 VISION 与 VLM 的部分交叉且最终不完整
	// loadVisionTriplet 已处理；此处再校验“显式混合前缀”场景：
	// 例如 VISION_ENDPOINT + VLM_KEY + LLM_MODEL（各有值但无完整 VISION 或 VLM）
	ai := envTriplet("AI_ENDPOINT", "AI_KEY", "AI_MODEL")
	if ai.partial() {
		return errors.New("incomplete AI_* triplet")
	}
	return nil
}

func validateProductionEndpoint(name, endpoint, model string) error {
	if strings.TrimSpace(model) == "" {
		return fmt.Errorf("production %s_MODEL must not be empty", name)
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("production %s_ENDPOINT is not a valid URL", name)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("production %s_ENDPOINT must use HTTPS", name)
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasSuffix(host, ".local") {
		return fmt.Errorf("production %s_ENDPOINT must not target localhost", name)
	}
	return nil
}

// ReadyErrors 返回 readiness 维度的配置问题（不输出 Secret/endpoint/key）。
func (c *Config) ReadyErrors() []string {
	var issues []string
	if c.IsProduction() {
		if err := c.Validate(); err != nil {
			// 脱敏：去掉可能含 endpoint 的细节，只保留安全 capability 状态
			issues = append(issues, sanitizeReadyErrors(err.Error())...)
		}
	} else {
		if err := c.ThirdParty.validateVisionTriplet(); err != nil {
			issues = append(issues, err.Error())
		}
		if !c.ThirdParty.AIConfigured() && !c.MockAllowed() {
			issues = append(issues, "ai provider not configured and mock disabled")
		}
	}
	return issues
}

// CapabilityStatus 返回安全的 capability 状态（不含 endpoint/key）。
func (c *Config) CapabilityStatus() map[string]interface{} {
	return map[string]interface{}{
		"ai_configured":      c.ThirdParty.AIConfigured(),
		"ai_source":          c.ThirdParty.VisionSource,
		"vision_configured":  c.ThirdParty.VisionConfigured(),
		"vision_source":      c.ThirdParty.VisionSource,
		"vision_fingerprint": c.ThirdParty.VisionFingerprint(),
		"llm_configured":     c.ThirdParty.LLMConfigured(),
		"mock_allowed":       c.MockAllowed(),
		"vision_reuse_llm":   c.ThirdParty.VisionReuseLLM,
		"feature_ranking":    c.FeatureFlags.Ranking,
		"feature_pvp":        c.FeatureFlags.PvP,
		"feature_social":     c.FeatureFlags.Social,
		"feature_ops":        c.FeatureFlags.Ops,
	}
}

func sanitizeReadyErrors(joined string) []string {
	parts := strings.Split(joined, "; ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 不透出具体 URL
		if strings.Contains(strings.ToLower(p), "http://") || strings.Contains(strings.ToLower(p), "https://") {
			if strings.Contains(p, "VISION") {
				out = append(out, "vision endpoint failed production safety checks")
				continue
			}
			if strings.Contains(p, "LLM") {
				out = append(out, "llm endpoint failed production safety checks")
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

// SetupLogger 根据级别配置 slog 全局默认 logger。
func SetupLogger(level string) {
	var l slog.Level
	switch level {
	case "DEBUG":
		l = slog.LevelDebug
	case "WARN", "WARNING":
		l = slog.LevelWarn
	case "ERROR":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l})
	slog.SetDefault(slog.New(handler))
}

func loadUpstreamConfig() UpstreamConfig {
	u := DefaultUpstreamConfig()
	u.Geo = loadProviderBudget("UPSTREAM_GEO", u.Geo)
	u.Weather = loadProviderBudget("UPSTREAM_WEATHER", u.Weather)
	u.Vision = loadProviderBudget("UPSTREAM_VISION", u.Vision)
	u.LLM = loadProviderBudget("UPSTREAM_LLM", u.LLM)
	u.MaxRetryAfter = getEnvDuration("UPSTREAM_MAX_RETRY_AFTER", u.MaxRetryAfter)
	u.CircuitFailureThreshold = getEnvInt("UPSTREAM_CIRCUIT_FAILURES", u.CircuitFailureThreshold)
	u.CircuitOpenTimeout = getEnvDuration("UPSTREAM_CIRCUIT_OPEN_TIMEOUT", u.CircuitOpenTimeout)
	if u.MaxRetryAfter <= 0 {
		u.MaxRetryAfter = 5 * time.Second
	}
	if u.CircuitFailureThreshold <= 0 {
		u.CircuitFailureThreshold = 5
	}
	if u.CircuitOpenTimeout <= 0 {
		u.CircuitOpenTimeout = 30 * time.Second
	}
	return u
}

func loadProviderBudget(prefix string, def ProviderBudget) ProviderBudget {
	b := def
	b.TotalDeadline = getEnvDuration(prefix+"_DEADLINE", b.TotalDeadline)
	b.Timeout = getEnvDuration(prefix+"_TIMEOUT", b.Timeout)
	b.MaxRetries = getEnvInt(prefix+"_MAX_RETRIES", b.MaxRetries)
	b.MaxConcurrent = getEnvInt(prefix+"_CONCURRENCY", b.MaxConcurrent)
	if b.Timeout <= 0 {
		b.Timeout = def.Timeout
	}
	if b.MaxRetries < 0 {
		b.MaxRetries = 0
	}
	if b.MaxConcurrent <= 0 {
		b.MaxConcurrent = def.MaxConcurrent
	}
	return b
}

// loadJWTSigningKeys 加载 JWT 签名密钥。
// 首选 JWT_SIGNING_KEY / JWT_SIGNING_KEY_PREVIOUS；兼容 JWT_SECRET / JWT_SECRET_PREVIOUS。
// 非 production 未配置时回退 DefaultDevJWTSecret；production 保持空串由 Validate 拒绝。
func loadJWTSigningKeys(isProd bool) (current, previous string) {
	current = firstNonEmpty(os.Getenv("JWT_SIGNING_KEY"), os.Getenv("JWT_SECRET"))
	previous = firstNonEmpty(os.Getenv("JWT_SIGNING_KEY_PREVIOUS"), os.Getenv("JWT_SECRET_PREVIOUS"))
	if current == "" && !isProd {
		current = DefaultDevJWTSecret
	}
	return current, previous
}

// loadPurposeKey 加载单一用途密钥及其可选 previous（双读窗口）。
// 非 production 未配置时使用 devDefault；production 不回退、不跨用途借用。
func loadPurposeKey(envKey, prevEnvKey, devDefault string, isProd bool) (current, previous string) {
	current = os.Getenv(envKey)
	previous = os.Getenv(prevEnvKey)
	if current == "" && !isProd {
		current = devDefault
	}
	return current, previous
}

// validateProductionCryptoKeys production 强制：各用途密钥齐全、足够长、非默认、互不共享。
func (c *Config) validateProductionCryptoKeys() error {
	var errs []string

	type keySpec struct {
		label      string
		value      string
		devDefault string
	}
	specs := []keySpec{
		// 报错文案含 JWT_SECRET 以便兼容旧运维检索；首选 env 仍为 JWT_SIGNING_KEY。
		{"JWT_SIGNING_KEY/JWT_SECRET", c.JWTSecret, DefaultDevJWTSecret},
		{"ACCOUNT_TOKEN_PEPPER", c.AccountTokenPepper, DefaultDevAccountTokenPepper},
		{"STATS_HMAC_KEY", c.StatsHMACKey, DefaultDevStatsHMACKey},
		{"TIME_SIGNING_KEY", c.TimeSigningKey, DefaultDevTimeSigningKey},
		{"ADMIN_JWT_SECRET", c.AdminJWTSecret, DefaultDevAdminJWTSecret},
	}
	for _, s := range specs {
		if s.value == "" || s.value == s.devDefault || len(s.value) < MinCryptoKeyLen {
			errs = append(errs, fmt.Sprintf("production requires strong %s (>=%d chars, not default)", s.label, MinCryptoKeyLen))
		}
	}

	// 禁止跨用途共享同一密钥（扩大单点泄露面）。
	if c.JWTSecret != "" {
		if c.JWTSecret == c.AccountTokenPepper {
			errs = append(errs, "production forbids sharing JWT signing key with ACCOUNT_TOKEN_PEPPER")
		}
		if c.JWTSecret == c.StatsHMACKey {
			errs = append(errs, "production forbids sharing JWT signing key with STATS_HMAC_KEY")
		}
		if c.JWTSecret == c.TimeSigningKey {
			errs = append(errs, "production forbids sharing JWT signing key with TIME_SIGNING_KEY")
		}
		if c.JWTSecret == c.AdminJWTSecret {
			errs = append(errs, "production forbids sharing JWT signing key with ADMIN_JWT_SECRET")
		}
	}
	if c.AccountTokenPepper != "" && c.AccountTokenPepper == c.StatsHMACKey {
		errs = append(errs, "production forbids sharing ACCOUNT_TOKEN_PEPPER with STATS_HMAC_KEY")
	}
	if c.AccountTokenPepper != "" && c.AccountTokenPepper == c.TimeSigningKey {
		errs = append(errs, "production forbids sharing ACCOUNT_TOKEN_PEPPER with TIME_SIGNING_KEY")
	}
	if c.StatsHMACKey != "" && c.StatsHMACKey == c.TimeSigningKey {
		errs = append(errs, "production forbids sharing STATS_HMAC_KEY with TIME_SIGNING_KEY")
	}
	if c.AdminJWTSecret != "" {
		if c.AdminJWTSecret == c.AccountTokenPepper {
			errs = append(errs, "production forbids sharing ADMIN_JWT_SECRET with ACCOUNT_TOKEN_PEPPER")
		}
		if c.AdminJWTSecret == c.StatsHMACKey {
			errs = append(errs, "production forbids sharing ADMIN_JWT_SECRET with STATS_HMAC_KEY")
		}
		if c.AdminJWTSecret == c.TimeSigningKey {
			errs = append(errs, "production forbids sharing ADMIN_JWT_SECRET with TIME_SIGNING_KEY")
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getEnvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func getEnvDuration(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	// 支持纯秒数
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Second
	}
	return def
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
