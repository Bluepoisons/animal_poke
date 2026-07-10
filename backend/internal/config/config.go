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

// 默认开发 JWT Secret（仅 development/test 允许）。
const DefaultDevJWTSecret = "animal-poke-dev-secret"

// Config 聚合所有服务端配置。第三方 Key 全部在此集中(客户端永不含)。
type Config struct {
	AppEnv             string
	ServerAddr         string
	LogLevel           string
	JWTSecret          string
	JWTIssuer          string
	JWTAudience        string
	JWTAccessTTL       time.Duration
	AIMockEnabled      bool
	RedisURL           string
	AdminAPIKey        string
	MaxImageBytes      int64
	MaxImagePixels     int
	CORSAllowedOrigins []string
	Database           DatabaseConfig
	ThirdParty         ThirdPartyConfig
	Server             ServerTimeouts
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
		AppEnv:             getEnv("APP_ENV", "development"),
		ServerAddr:         getEnv("SERVER_ADDR", ":8080"),
		LogLevel:           getEnv("LOG_LEVEL", "INFO"),
		JWTSecret:          getEnv("JWT_SECRET", DefaultDevJWTSecret),
		JWTIssuer:          getEnv("JWT_ISSUER", "animal-poke"),
		JWTAudience:        getEnv("JWT_AUDIENCE", "animal-poke-client"),
		JWTAccessTTL:       getEnvDuration("JWT_ACCESS_TTL", 2*time.Hour),
		AIMockEnabled:      getEnvBool("AI_MOCK_ENABLED", true),
		RedisURL:           getEnv("REDIS_URL", ""),
		AdminAPIKey:        getEnv("ADMIN_API_KEY", ""),
		MaxImageBytes:      int64(getEnvInt("MAX_IMAGE_BYTES", 5*1024*1024)),
		MaxImagePixels:     getEnvInt("MAX_IMAGE_PIXELS", 12_000_000),
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "")),
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
	}

	// 原子加载 Vision 三元组（禁止字段级混合）
	cfg.ThirdParty.loadVisionTriplet()

	// 开发默认开启 mock；生产强制关闭（即使 env 写了 true）
	if cfg.IsProduction() {
		cfg.AIMockEnabled = getEnvBool("AI_MOCK_ENABLED", false)
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
		if c.JWTSecret == "" || c.JWTSecret == DefaultDevJWTSecret || len(c.JWTSecret) < 32 {
			errs = append(errs, "production requires strong JWT_SECRET (>=32 chars, not default)")
		}
		if c.Database.Password == "" || c.Database.Password == "animal_poke" {
			errs = append(errs, "production forbids default DB_PASSWORD")
		}
		if c.AIMockEnabled {
			errs = append(errs, "production forbids AI_MOCK_ENABLED=true")
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
