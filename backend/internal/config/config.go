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
	AIMockEnabled          bool
	// AuthMockOAuthEnabled 允许 mock_oauth 绑定/登录（仅 development/test；production 强制 false）。
	// 环境变量 AUTH_MOCK_OAUTH_ENABLED；非 production 默认 true。
	AuthMockOAuthEnabled bool
	RedisURL             string
	AdminAPIKey          string
	// OpsToken 运维内部接口 X-AP-Ops-Token 校验值（可与 AdminAPIKey 相同，但独立配置）。
	OpsToken string
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
			TotalDeadline: 45 * time.Second,
			Timeout:       25 * time.Second,
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

// DatabaseConfig MySQL 连接配置。
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	TLSMode         string // prefer / require / skip-verify / false
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DSN 使用 mysql.Config 兼容特殊字符密码，统一 UTC。
func (d DatabaseConfig) DSN() string {
	// 手动拼装但正确 URL 编码用户名/密码，避免特殊字符破坏 DSN。
	user := url.QueryEscape(d.User)
	pass := url.QueryEscape(d.Password)
	tls := d.TLSMode
	if tls == "" {
		tls = "false"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=UTC&tls=%s&timeout=5s&readTimeout=10s&writeTimeout=10s",
		user, pass, d.Host, d.Port, d.DBName, url.QueryEscape(tls))
}

// ThirdPartyConfig 第三方 API Key(腾讯地图/彩云/Vision/LLM)。
type ThirdPartyConfig struct {
	TencentMapKey    string
	CaiyunWeatherKey string
	// Vision 图片检测/分析（原子三元组）
	VisionEndpoint string
	VisionKey      string
	VisionModel    string
	// VisionReuseLLM 为 true 时允许整组复用 LLM 配置（默认禁止隐式回退）
	VisionReuseLLM bool
	// VisionSource 记录配置来源：vision|vlm|llm_reuse|none（不含密钥）
	VisionSource string
	// Text/LLM 数值生成
	LLMEndpoint string
	LLMKey      string
	LLMModel    string
}

// VisionConfigured 是否具备 Vision 调用条件（完整三元组）。
func (t ThirdPartyConfig) VisionConfigured() bool {
	return t.VisionEndpoint != "" && t.VisionKey != "" && t.VisionModel != ""
}

// LLMConfigured 是否具备 Text 调用条件。
func (t ThirdPartyConfig) LLMConfigured() bool {
	return t.LLMEndpoint != "" && t.LLMKey != "" && t.LLMModel != ""
}

// VisionFingerprint 返回不含密钥的配置指纹（用于日志/readiness）。
func (t ThirdPartyConfig) VisionFingerprint() string {
	if !t.VisionConfigured() {
		return "none"
	}
	raw := t.VisionSource + "|" + t.VisionEndpoint + "|" + t.VisionModel
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
		JWTIssuer:          getEnv("JWT_ISSUER", "animal-poke"),
		JWTAudience:        getEnv("JWT_AUDIENCE", "animal-poke-client"),
		JWTAccessTTL:       getEnvDuration("JWT_ACCESS_TTL", 2*time.Hour),
		AIMockEnabled:      getEnvBool("AI_MOCK_ENABLED", true),
		RedisURL:           getEnv("REDIS_URL", ""),
		AdminAPIKey:        getEnv("ADMIN_API_KEY", ""),
		OpsToken:           getEnv("OPS_TOKEN", ""),
		MaxImageBytes:      int64(getEnvInt("MAX_IMAGE_BYTES", 5*1024*1024)),
		MaxImagePixels:     getEnvInt("MAX_IMAGE_PIXELS", 12_000_000),
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "")),
		TrustedProxies:     splitCSV(getEnv("TRUSTED_PROXIES", "")),
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
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 10*time.Minute),
		},
		ThirdParty: ThirdPartyConfig{
			TencentMapKey:    getEnv("TENCENT_MAP_KEY", ""),
			CaiyunWeatherKey: getEnv("CAIYUN_WEATHER_KEY", ""),
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

	// 原子加载 Vision 三元组（禁止字段级混合）
	cfg.ThirdParty.loadVisionTriplet()

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

// loadVisionTriplet 按优先级加载完整 Vision 三元组。
// 规则：
//  1. 完整 VISION_* 优先；
//  2. 否则完整 VLM_*（整组兼容，禁止与 VISION 字段混用）；
//  3. 否则仅当 VISION_REUSE_LLM=true 且 LLM 三元组完整时复用 LLM；
//  4. 任何前缀若只配置了部分字段，视为不完整（启动校验失败）。
func (t *ThirdPartyConfig) loadVisionTriplet() {
	vision := envTriplet("VISION_ENDPOINT", "VISION_KEY", "VISION_MODEL")
	vlm := envTriplet("VLM_ENDPOINT", "VLM_KEY", "VLM_MODEL")
	llm := envTriplet("LLM_ENDPOINT", "LLM_KEY", "LLM_MODEL")

	// 记录部分配置以便 Validate 拒绝混合/残缺
	t.VisionEndpoint, t.VisionKey, t.VisionModel = "", "", ""
	t.VisionSource = "none"

	switch {
	case vision.complete():
		t.VisionEndpoint, t.VisionKey, t.VisionModel = vision.endpoint, vision.key, vision.model
		t.VisionSource = "vision"
	case vision.partial():
		// 残缺 VISION_*：保留残缺值供 Validate 报错（禁止静默回退到 VLM/LLM）
		t.VisionEndpoint, t.VisionKey, t.VisionModel = vision.endpoint, vision.key, vision.model
		t.VisionSource = "vision_partial"
	case vlm.complete():
		t.VisionEndpoint, t.VisionKey, t.VisionModel = vlm.endpoint, vlm.key, vlm.model
		t.VisionSource = "vlm"
	case vlm.partial():
		t.VisionEndpoint, t.VisionKey, t.VisionModel = vlm.endpoint, vlm.key, vlm.model
		t.VisionSource = "vlm_partial"
	case t.VisionReuseLLM && llm.complete():
		t.VisionEndpoint, t.VisionKey, t.VisionModel = llm.endpoint, llm.key, llm.model
		t.VisionSource = "llm_reuse"
		slog.Info("vision 显式复用 LLM 完整配置",
			"source", t.VisionSource,
			"fingerprint", t.VisionFingerprint(),
			"model", t.VisionModel,
		)
	default:
		// 仅配置 LLM 且未显式复用 → Vision 未配置
		t.VisionSource = "none"
	}
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
		if !c.ThirdParty.VisionConfigured() {
			errs = append(errs, "production requires VISION_ENDPOINT/KEY/MODEL")
		}
		if !c.ThirdParty.LLMConfigured() {
			errs = append(errs, "production requires LLM_ENDPOINT/KEY/MODEL")
		}
		if c.AdminAPIKey == "" {
			errs = append(errs, "production requires ADMIN_API_KEY for audit RBAC")
		}
		if len(c.CORSAllowedOrigins) == 0 {
			errs = append(errs, "production requires CORS_ALLOWED_ORIGINS allowlist")
		}
		// 生产 Vision/LLM endpoint 必须 HTTPS，拒绝 localhost / 明文 HTTP / 空模型
		if c.ThirdParty.VisionConfigured() {
			if err := validateProductionEndpoint("VISION", c.ThirdParty.VisionEndpoint, c.ThirdParty.VisionModel); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if c.ThirdParty.LLMConfigured() {
			if err := validateProductionEndpoint("LLM", c.ThirdParty.LLMEndpoint, c.ThirdParty.LLMModel); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}

	if c.JWTAccessTTL <= 0 {
		errs = append(errs, "JWT_ACCESS_TTL must be positive")
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
	if src == "vision_partial" || src == "vlm_partial" {
		return fmt.Errorf("vision config must be a complete triplet (endpoint+key+model); incomplete/mixed %s rejected", src)
	}
	// 检测字段级混合：若环境中同时存在 VISION 与 VLM 的部分交叉且最终不完整
	// loadVisionTriplet 已处理；此处再校验“显式混合前缀”场景：
	// 例如 VISION_ENDPOINT + VLM_KEY + LLM_MODEL（各有值但无完整 VISION 或 VLM）
	vision := envTriplet("VISION_ENDPOINT", "VISION_KEY", "VISION_MODEL")
	vlm := envTriplet("VLM_ENDPOINT", "VLM_KEY", "VLM_MODEL")
	if !vision.empty() && !vlm.empty() && !vision.complete() && !vlm.complete() {
		return errors.New("mixed VISION_* and VLM_* fields are forbidden; use one complete triplet")
	}
	// 混合 VISION + VLM + LLM 字段
	if (vision.partial() || vlm.partial()) && !t.VisionConfigured() {
		return errors.New("incomplete vision/vlm triplet (field-level mixing with LLM is forbidden)")
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
		if !c.ThirdParty.VisionConfigured() && !c.MockAllowed() {
			issues = append(issues, "vision provider not configured and mock disabled")
		}
		if !c.ThirdParty.LLMConfigured() && !c.MockAllowed() {
			issues = append(issues, "llm provider not configured and mock disabled")
		}
	}
	return issues
}

// CapabilityStatus 返回安全的 capability 状态（不含 endpoint/key）。
func (c *Config) CapabilityStatus() map[string]interface{} {
	return map[string]interface{}{
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
