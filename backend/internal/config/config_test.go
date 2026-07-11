package config

import (
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDSN(t *testing.T) {
	tests := []struct {
		name string
		cfg  DatabaseConfig
		want string
	}{
		{
			name: "standard",
			cfg:  DatabaseConfig{Host: "127.0.0.1", Port: 3306, User: "u", Password: "p", DBName: "db", TLSMode: "false"},
			want: "u:p@tcp(127.0.0.1:3306)/db?charset=utf8mb4&parseTime=True&loc=UTC&tls=false&timeout=5s&readTimeout=10s&writeTimeout=10s",
		},
		{
			name: "special_password",
			cfg:  DatabaseConfig{Host: "db.host", Port: 3307, User: "root", Password: "p@ss:w/rd", DBName: "prod", TLSMode: "require"},
			want: "root:" + url.QueryEscape("p@ss:w/rd") + "@tcp(db.host:3307)/prod?charset=utf8mb4&parseTime=True&loc=UTC&tls=require&timeout=5s&readTimeout=10s&writeTimeout=10s",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cfg.DSN())
		})
	}
}

// clearProviderEnv 清空 Vision/VLM/LLM 相关环境，避免宿主环境干扰。
func clearProviderEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"APP_ENV", "SERVER_ADDR", "METRICS_ADDR", "LOG_LEVEL", "AI_MOCK_ENABLED",
		"JWT_SECRET", "JWT_SECRET_PREVIOUS", "JWT_SIGNING_KEY", "JWT_SIGNING_KEY_PREVIOUS",
		"ACCOUNT_TOKEN_PEPPER", "ACCOUNT_TOKEN_PEPPER_PREVIOUS",
		"STATS_HMAC_KEY", "STATS_HMAC_KEY_PREVIOUS",
		"TIME_SIGNING_KEY", "TIME_SIGNING_KEY_PREVIOUS",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_TLS",
		"TENCENT_MAP_KEY", "CAIYUN_WEATHER_KEY",
		"VISION_ENDPOINT", "VISION_KEY", "VISION_MODEL",
		"VLM_ENDPOINT", "VLM_KEY", "VLM_MODEL",
		"LLM_ENDPOINT", "LLM_KEY", "LLM_MODEL",
		"VISION_REUSE_LLM",
		"CORS_ALLOWED_ORIGINS", "ADMIN_API_KEY", "OPS_TOKEN",
		"FEATURE_RANKING", "FEATURE_PVP", "FEATURE_SOCIAL", "FEATURE_OPS",
		"COMMERCE_ENABLED", "COMMERCE_STORE_VERIFY",
	}
	saved := map[string]string{}
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			saved[k] = v
		}
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for _, k := range keys {
			os.Unsetenv(k)
		}
		for k, v := range saved {
			os.Setenv(k, v)
		}
	})
}

// 用例需要环境变量为空以验证默认值; t.Setenv 无法 unset, 故手动保存/清除/恢复。
func TestLoad_Defaults(t *testing.T) {
	clearProviderEnv(t)

	cfg := Load()
	assert.Equal(t, "development", cfg.AppEnv)
	assert.Equal(t, ":8080", cfg.ServerAddr)
	assert.Equal(t, ":9090", cfg.MetricsAddr)
	assert.Equal(t, "INFO", cfg.LogLevel)
	assert.Equal(t, "127.0.0.1", cfg.Database.Host)
	assert.Equal(t, 3306, cfg.Database.Port)
	assert.Equal(t, "animal_poke", cfg.Database.User)
	assert.Equal(t, "animal_poke", cfg.Database.Password)
	assert.Equal(t, "animal_poke", cfg.Database.DBName)
	assert.Equal(t, "", cfg.ThirdParty.TencentMapKey)
	assert.Equal(t, "", cfg.ThirdParty.VisionKey)
	assert.Equal(t, "", cfg.ThirdParty.LLMModel)
	assert.False(t, cfg.ThirdParty.VisionConfigured())
	assert.Equal(t, "none", cfg.ThirdParty.VisionSource)
	assert.True(t, cfg.AIMockEnabled)
	assert.True(t, cfg.MockAllowed())
	// 非 production 默认开启 product feature flags
	assert.True(t, cfg.FeatureFlags.Ranking)
	assert.True(t, cfg.FeatureFlags.PvP)
	assert.True(t, cfg.FeatureFlags.Social)
	assert.True(t, cfg.FeatureFlags.Ops)
}

func TestLoad_Overrides(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("APP_ENV", "development")
	t.Setenv("SERVER_ADDR", ":9999")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "13306")
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASSWORD", "pw")
	t.Setenv("DB_NAME", "prod")
	t.Setenv("TENCENT_MAP_KEY", "tk")
	t.Setenv("LLM_MODEL", "qwen3.6-flash")
	t.Setenv("VISION_ENDPOINT", "https://vision.example")
	t.Setenv("VISION_KEY", "vk")
	t.Setenv("VISION_MODEL", "vision-model")

	cfg := Load()
	assert.Equal(t, ":9999", cfg.ServerAddr)
	assert.Equal(t, "DEBUG", cfg.LogLevel)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 13306, cfg.Database.Port)
	assert.Equal(t, "root", cfg.Database.User)
	assert.Equal(t, "pw", cfg.Database.Password)
	assert.Equal(t, "prod", cfg.Database.DBName)
	assert.Equal(t, "tk", cfg.ThirdParty.TencentMapKey)
	assert.Equal(t, "qwen3.6-flash", cfg.ThirdParty.LLMModel)
	assert.Equal(t, "https://vision.example", cfg.ThirdParty.VisionEndpoint)
	assert.Equal(t, "vk", cfg.ThirdParty.VisionKey)
	assert.Equal(t, "vision-model", cfg.ThirdParty.VisionModel)
	assert.True(t, cfg.ThirdParty.VisionConfigured())
	assert.Equal(t, "vision", cfg.ThirdParty.VisionSource)
}

func TestLoad_VisionAtomicTriplet(t *testing.T) {
	tests := []struct {
		name            string
		env             map[string]string
		wantConfigured  bool
		wantSource      string
		wantEndpoint    string
		wantModel       string
		wantValidateErr string
	}{
		{
			name: "complete_vision",
			env: map[string]string{
				"VISION_ENDPOINT": "https://vision.example/v1",
				"VISION_KEY":      "vk",
				"VISION_MODEL":    "vision-model",
			},
			wantConfigured: true,
			wantSource:     "vision",
			wantEndpoint:   "https://vision.example/v1",
			wantModel:      "vision-model",
		},
		{
			name: "complete_vlm_compat",
			env: map[string]string{
				"VLM_ENDPOINT": "https://vlm.example/v1",
				"VLM_KEY":      "vlmk",
				"VLM_MODEL":    "vlm-model",
			},
			wantConfigured: true,
			wantSource:     "vlm",
			wantEndpoint:   "https://vlm.example/v1",
			wantModel:      "vlm-model",
		},
		{
			name: "llm_only_no_reuse_vision_not_configured",
			env: map[string]string{
				"LLM_ENDPOINT": "https://llm.example/v1",
				"LLM_KEY":      "lk",
				"LLM_MODEL":    "text-model",
			},
			wantConfigured: false,
			wantSource:     "none",
		},
		{
			name: "explicit_reuse_llm",
			env: map[string]string{
				"LLM_ENDPOINT":     "https://llm.example/v1",
				"LLM_KEY":          "lk",
				"LLM_MODEL":        "text-model",
				"VISION_REUSE_LLM": "true",
			},
			wantConfigured: true,
			wantSource:     "llm_reuse",
			wantEndpoint:   "https://llm.example/v1",
			wantModel:      "text-model",
		},
		{
			name: "partial_vision_missing_key",
			env: map[string]string{
				"VISION_ENDPOINT": "https://vision.example/v1",
				"VISION_MODEL":    "vision-model",
			},
			wantConfigured:  false,
			wantSource:      "vision_partial",
			wantValidateErr: "complete triplet",
		},
		{
			name: "mixed_vision_endpoint_vlm_key",
			env: map[string]string{
				"VISION_ENDPOINT": "https://vision.example/v1",
				"VLM_KEY":         "vlmk",
				"LLM_MODEL":       "text-model",
			},
			wantConfigured:  false,
			wantSource:      "vision_partial",
			wantValidateErr: "complete triplet",
		},
		{
			name: "vision_wins_over_vlm",
			env: map[string]string{
				"VISION_ENDPOINT": "https://vision.example/v1",
				"VISION_KEY":      "vk",
				"VISION_MODEL":    "vision-model",
				"VLM_ENDPOINT":    "https://vlm.example/v1",
				"VLM_KEY":         "vlmk",
				"VLM_MODEL":       "vlm-model",
			},
			wantConfigured: true,
			wantSource:     "vision",
			wantEndpoint:   "https://vision.example/v1",
			wantModel:      "vision-model",
		},
		{
			name: "reuse_false_even_if_llm_complete",
			env: map[string]string{
				"LLM_ENDPOINT":     "https://llm.example/v1",
				"LLM_KEY":          "lk",
				"LLM_MODEL":        "text-model",
				"VISION_REUSE_LLM": "false",
			},
			wantConfigured: false,
			wantSource:     "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearProviderEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			cfg := Load()
			assert.Equal(t, tt.wantConfigured, cfg.ThirdParty.VisionConfigured(), "VisionConfigured")
			assert.Equal(t, tt.wantSource, cfg.ThirdParty.VisionSource, "VisionSource")
			if tt.wantEndpoint != "" {
				assert.Equal(t, tt.wantEndpoint, cfg.ThirdParty.VisionEndpoint)
			}
			if tt.wantModel != "" {
				assert.Equal(t, tt.wantModel, cfg.ThirdParty.VisionModel)
			}
			if tt.wantSource == "llm_reuse" {
				assert.True(t, cfg.ThirdParty.VisionReuseLLM)
				assert.NotEqual(t, "none", cfg.ThirdParty.VisionFingerprint())
			}
			err := cfg.Validate()
			if tt.wantValidateErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantValidateErr)
			} else if !cfg.IsProduction() {
				// 开发环境仅在残缺时失败；完整/未配置均可通过（mock 可补）
				if tt.wantSource == "vision_partial" || tt.wantSource == "vlm_partial" {
					require.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			}
		})
	}
}

// applyStrongCryptoKeys 为 production Validate 测试填充四类互不相同的强密钥。
func applyStrongCryptoKeys(cfg *Config) {
	cfg.JWTSecret = "prod-jwt-signing-key-32chars-min!"
	cfg.AccountTokenPepper = "prod-account-pepper-32chars-min!!"
	cfg.StatsHMACKey = "prod-stats-hmac-key-32chars-min!"
	cfg.TimeSigningKey = "prod-time-signing-key-32chars-min!"
	cfg.AuthMockOAuthEnabled = false
}

func TestValidate_ProductionHTTPEndpoint(t *testing.T) {
	clearProviderEnv(t)
	cfg := Load()
	cfg.AppEnv = "production"
	applyStrongCryptoKeys(cfg)
	cfg.Database.Password = "complex-pass"
	cfg.AIMockEnabled = false
	cfg.AdminAPIKey = "admin-secret"
	cfg.CORSAllowedOrigins = []string{"https://app.example.com"}
	cfg.ThirdParty = ThirdPartyConfig{
		VisionEndpoint: "http://vision.example/v1",
		VisionKey:      "k",
		VisionModel:    "m",
		VisionSource:   "vision",
		LLMEndpoint:    "https://llm.example/v1",
		LLMKey:         "k",
		LLMModel:       "m",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}

func TestValidate_ProductionLocalhostRejected(t *testing.T) {
	clearProviderEnv(t)
	cfg := Load()
	cfg.AppEnv = "production"
	applyStrongCryptoKeys(cfg)
	cfg.Database.Password = "complex-pass"
	cfg.AIMockEnabled = false
	cfg.AdminAPIKey = "admin-secret"
	cfg.CORSAllowedOrigins = []string{"https://app.example.com"}
	cfg.ThirdParty = ThirdPartyConfig{
		VisionEndpoint: "https://localhost:8080/v1",
		VisionKey:      "k",
		VisionModel:    "m",
		VisionSource:   "vision",
		LLMEndpoint:    "https://llm.example/v1",
		LLMKey:         "k",
		LLMModel:       "m",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "localhost")
}

func TestValidate_Production(t *testing.T) {
	clearProviderEnv(t)
	cfg := Load()
	cfg.AppEnv = "production"
	cfg.JWTSecret = DefaultDevJWTSecret
	cfg.AIMockEnabled = true
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")

	applyStrongCryptoKeys(cfg)
	cfg.Database.Password = "complex-pass"
	cfg.AIMockEnabled = false
	cfg.AuthMockOAuthEnabled = false
	cfg.ThirdParty = ThirdPartyConfig{
		VisionEndpoint: "https://v.example", VisionKey: "k", VisionModel: "m", VisionSource: "vision",
		LLMEndpoint: "https://l.example", LLMKey: "k", LLMModel: "m",
	}
	cfg.AdminAPIKey = "admin-secret"
	// 缺 CORS 白名单应失败
	err = cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CORS_ALLOWED_ORIGINS")

	cfg.CORSAllowedOrigins = []string{"https://app.example.com"}
	assert.NoError(t, cfg.Validate())
}

func TestLoad_CryptoKeys_DevDefaultsDistinct(t *testing.T) {
	clearProviderEnv(t)
	cfg := Load()
	assert.Equal(t, DefaultDevJWTSecret, cfg.JWTSecret)
	assert.Equal(t, DefaultDevAccountTokenPepper, cfg.AccountTokenPepper)
	assert.Equal(t, DefaultDevStatsHMACKey, cfg.StatsHMACKey)
	assert.Equal(t, DefaultDevTimeSigningKey, cfg.TimeSigningKey)
	// 开发默认也按用途分离，避免误用同一串
	assert.NotEqual(t, cfg.JWTSecret, cfg.AccountTokenPepper)
	assert.NotEqual(t, cfg.JWTSecret, cfg.StatsHMACKey)
	assert.NotEqual(t, cfg.JWTSecret, cfg.TimeSigningKey)
}

func TestLoad_JWTSigningKeyPreferredOverAlias(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("JWT_SIGNING_KEY", "signing-key-from-preferred-name-xx")
	t.Setenv("JWT_SECRET", "signing-key-from-alias-should-lose")
	t.Setenv("JWT_SIGNING_KEY_PREVIOUS", "prev-from-preferred")
	t.Setenv("JWT_SECRET_PREVIOUS", "prev-from-alias")
	cfg := Load()
	assert.Equal(t, "signing-key-from-preferred-name-xx", cfg.JWTSecret)
	assert.Equal(t, "prev-from-preferred", cfg.JWTSecretPrevious)
}

func TestLoad_JWTSecretAliasStillWorks(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("JWT_SECRET", "legacy-jwt-secret-alias-value")
	t.Setenv("JWT_SECRET_PREVIOUS", "legacy-prev")
	cfg := Load()
	assert.Equal(t, "legacy-jwt-secret-alias-value", cfg.JWTSecret)
	assert.Equal(t, "legacy-prev", cfg.JWTSecretPrevious)
}

func TestLoad_PurposeKeysAndPrevious(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("ACCOUNT_TOKEN_PEPPER", "acct-pepper-current")
	t.Setenv("ACCOUNT_TOKEN_PEPPER_PREVIOUS", "acct-pepper-prev")
	t.Setenv("STATS_HMAC_KEY", "stats-hmac-current")
	t.Setenv("STATS_HMAC_KEY_PREVIOUS", "stats-hmac-prev")
	t.Setenv("TIME_SIGNING_KEY", "time-sign-current")
	t.Setenv("TIME_SIGNING_KEY_PREVIOUS", "time-sign-prev")
	cfg := Load()
	assert.Equal(t, "acct-pepper-current", cfg.AccountTokenPepper)
	assert.Equal(t, "acct-pepper-prev", cfg.AccountTokenPepperPrevious)
	assert.Equal(t, "stats-hmac-current", cfg.StatsHMACKey)
	assert.Equal(t, "stats-hmac-prev", cfg.StatsHMACKeyPrevious)
	assert.Equal(t, "time-sign-current", cfg.TimeSigningKey)
	assert.Equal(t, "time-sign-prev", cfg.TimeSigningKeyPrevious)
}

func TestValidate_ProductionMissingPurposeKey(t *testing.T) {
	clearProviderEnv(t)
	cfg := Load()
	cfg.AppEnv = "production"
	applyStrongCryptoKeys(cfg)
	cfg.AccountTokenPepper = "" // 缺 pepper
	cfg.Database.Password = "complex-pass"
	cfg.AIMockEnabled = false
	cfg.AdminAPIKey = "admin-secret"
	cfg.CORSAllowedOrigins = []string{"https://app.example.com"}
	cfg.ThirdParty = ThirdPartyConfig{
		VisionEndpoint: "https://v.example", VisionKey: "k", VisionModel: "m", VisionSource: "vision",
		LLMEndpoint: "https://l.example", LLMKey: "k", LLMModel: "m",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ACCOUNT_TOKEN_PEPPER")
}

func TestValidate_ProductionSharedKeysRejected(t *testing.T) {
	clearProviderEnv(t)
	cfg := Load()
	cfg.AppEnv = "production"
	shared := "shared-secret-must-not-cross-purpose!!"
	cfg.JWTSecret = shared
	cfg.AccountTokenPepper = shared
	cfg.StatsHMACKey = "prod-stats-hmac-key-32chars-min!"
	cfg.TimeSigningKey = "prod-time-signing-key-32chars-min!"
	cfg.Database.Password = "complex-pass"
	cfg.AIMockEnabled = false
	cfg.AdminAPIKey = "admin-secret"
	cfg.CORSAllowedOrigins = []string{"https://app.example.com"}
	cfg.ThirdParty = ThirdPartyConfig{
		VisionEndpoint: "https://v.example", VisionKey: "k", VisionModel: "m", VisionSource: "vision",
		LLMEndpoint: "https://l.example", LLMKey: "k", LLMModel: "m",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ACCOUNT_TOKEN_PEPPER")
}

func TestValidate_ProductionNoCrossPurposeFallback(t *testing.T) {
	// production Load 不得用 JWT 填补其他用途
	clearProviderEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SIGNING_KEY", "prod-jwt-signing-key-32chars-min!")
	// 故意不设 ACCOUNT/STATS/TIME
	cfg := Load()
	assert.Equal(t, "prod-jwt-signing-key-32chars-min!", cfg.JWTSecret)
	assert.Empty(t, cfg.AccountTokenPepper)
	assert.Empty(t, cfg.StatsHMACKey)
	assert.Empty(t, cfg.TimeSigningKey)
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ACCOUNT_TOKEN_PEPPER")
	assert.Contains(t, err.Error(), "STATS_HMAC_KEY")
	assert.Contains(t, err.Error(), "TIME_SIGNING_KEY")
}

func TestCapabilityStatus_NoSecrets(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("VISION_ENDPOINT", "https://vision.example/v1")
	t.Setenv("VISION_KEY", "super-secret-key")
	t.Setenv("VISION_MODEL", "vision-model")
	cfg := Load()
	status := cfg.CapabilityStatus()
	assert.Equal(t, true, status["vision_configured"])
	assert.Equal(t, "vision", status["vision_source"])
	// 不得包含 key / endpoint 原文
	raw := strings.ToLower(strings.Join(mapValues(status), " "))
	assert.NotContains(t, raw, "super-secret-key")
	assert.NotContains(t, raw, "vision.example")
	// feature flags 出现在 capability 中
	assert.Contains(t, status, "feature_ranking")
	assert.Contains(t, status, "feature_pvp")
	assert.Contains(t, status, "feature_social")
	assert.Contains(t, status, "feature_ops")
}

func mapValues(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, strings.TrimSpace(strings.ToLower(toString(v))))
	}
	return out
}

func toString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func TestMockAllowed(t *testing.T) {
	cfg := &Config{AppEnv: "development", AIMockEnabled: true}
	assert.True(t, cfg.MockAllowed())
	cfg.AIMockEnabled = false
	assert.False(t, cfg.MockAllowed())
	cfg.AppEnv = "production"
	cfg.AIMockEnabled = true
	assert.False(t, cfg.MockAllowed())
}

func TestGetEnv_Default(t *testing.T) {
	os.Unsetenv("MY_MISSING_KEY")
	assert.Equal(t, "fallback", getEnv("MY_MISSING_KEY", "fallback"))

	t.Setenv("MY_PRESENT_KEY", "val")
	assert.Equal(t, "val", getEnv("MY_PRESENT_KEY", "fallback"))

	t.Setenv("MY_EMPTY_KEY", "")
	assert.Equal(t, "fallback", getEnv("MY_EMPTY_KEY", "fallback"))
}

func TestGetEnvInt_Fallback(t *testing.T) {
	t.Setenv("MY_INT_BAD", "not-a-number")
	assert.Equal(t, 42, getEnvInt("MY_INT_BAD", 42))

	t.Setenv("MY_INT_GOOD", "7")
	assert.Equal(t, 7, getEnvInt("MY_INT_GOOD", 42))

	os.Unsetenv("MY_INT_MISSING")
	assert.Equal(t, 42, getEnvInt("MY_INT_MISSING", 42))

	t.Setenv("MY_INT_EMPTY", "")
	assert.Equal(t, 42, getEnvInt("MY_INT_EMPTY", 42))
}

func TestGetEnvDuration(t *testing.T) {
	t.Setenv("MY_DUR", "2h")
	assert.Equal(t, 2*time.Hour, getEnvDuration("MY_DUR", time.Second))
	t.Setenv("MY_DUR_SEC", "30")
	assert.Equal(t, 30*time.Second, getEnvDuration("MY_DUR_SEC", time.Second))
}

func TestSetupLogger_NoPanic(t *testing.T) {
	for _, lvl := range []string{"DEBUG", "INFO", "WARN", "WARNING", "ERROR", "UNKNOWN"} {
		assert.NotPanics(t, func() { SetupLogger(lvl) }, "level=%s", lvl)
	}
}

func TestSetupLogger_LevelFilter(t *testing.T) {
	oldDefault := slog.Default()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		slog.SetDefault(oldDefault)
	})

	SetupLogger("ERROR")
	slog.Info("info_should_be_filtered")
	slog.Error("error_should_appear")

	_ = w.Close()
	out, _ := io.ReadAll(r)
	s := string(out)

	assert.Contains(t, s, "error_should_appear")
	assert.False(t, strings.Contains(s, "info_should_be_filtered"),
		"INFO 级别在 ERROR 阈值下应被过滤")
}

func TestLoad_FeatureFlags_ProductionDefaultsFalse(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("APP_ENV", "production")
	cfg := Load()
	assert.False(t, cfg.FeatureFlags.Ranking)
	assert.False(t, cfg.FeatureFlags.PvP)
	assert.False(t, cfg.FeatureFlags.Social)
	assert.False(t, cfg.FeatureFlags.Ops)
}

func TestLoad_FeatureFlags_Overrides(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("FEATURE_RANKING", "true")
	t.Setenv("FEATURE_PVP", "0")
	t.Setenv("FEATURE_SOCIAL", "yes")
	t.Setenv("FEATURE_OPS", "false")
	cfg := Load()
	assert.True(t, cfg.FeatureFlags.Ranking)
	assert.False(t, cfg.FeatureFlags.PvP)
	assert.True(t, cfg.FeatureFlags.Social)
	assert.False(t, cfg.FeatureFlags.Ops)
}

func TestLoad_OpsTokenFallsBackToAdmin(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("ADMIN_API_KEY", "admin-only")
	cfg := Load()
	assert.Equal(t, "admin-only", cfg.OpsToken)

	clearProviderEnv(t)
	t.Setenv("ADMIN_API_KEY", "admin-only")
	t.Setenv("OPS_TOKEN", "ops-specific")
	cfg = Load()
	assert.Equal(t, "ops-specific", cfg.OpsToken)
}

func TestAuthMockOAuth_ProductionForcedOff(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("AUTH_MOCK_OAUTH_ENABLED", "true")
	cfg := Load()
	assert.False(t, cfg.AuthMockOAuthEnabled)
}

func TestAuthMockOAuth_DevDefaultOn(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("APP_ENV", "development")
	cfg := Load()
	assert.True(t, cfg.AuthMockOAuthEnabled)

	t.Setenv("AUTH_MOCK_OAUTH_ENABLED", "false")
	cfg = Load()
	assert.False(t, cfg.AuthMockOAuthEnabled)
}
