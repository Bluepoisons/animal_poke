package config

import (
	"log/slog"
	"os"
	"strconv"
)

// Config 聚合所有服务端配置。第三方 Key 全部在此集中(客户端永不含)。
type Config struct {
	ServerAddr string
	LogLevel   string
	JWTSecret  string
	Database   DatabaseConfig
	ThirdParty ThirdPartyConfig
}

// DatabaseConfig MySQL 连接配置。
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

// DSN 生成 GORM MySQL 连接串。
func (d DatabaseConfig) DSN() string {
	return d.User + ":" + d.Password + "@tcp(" + d.Host + ":" + strconv.Itoa(d.Port) + ")/" +
		d.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
}

// ThirdPartyConfig 第三方 API Key(腾讯地图/彩云/VLM/LLM), 全部来自后端 .env。
type ThirdPartyConfig struct {
	TencentMapKey    string
	CaiyunWeatherKey string
	VLMEndpoint      string
	VLMKey           string
	LLMEndpoint      string
	LLMKey           string
	LLMModel         string
}

// Load 读取配置, 优先级: OS 环境变量 > .env > 默认值。
func Load() *Config {
	return &Config{
		ServerAddr: getEnv("SERVER_ADDR", ":8080"),
		LogLevel:   getEnv("LOG_LEVEL", "INFO"),
		JWTSecret:  getEnv("JWT_SECRET", "animal-poke-dev-secret"),
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "127.0.0.1"),
			Port:     getEnvInt("DB_PORT", 3306),
			User:     getEnv("DB_USER", "animal_poke"),
			Password: getEnv("DB_PASSWORD", "animal_poke"),
			DBName:   getEnv("DB_NAME", "animal_poke"),
		},
		ThirdParty: ThirdPartyConfig{
			TencentMapKey:    getEnv("TENCENT_MAP_KEY", ""),
			CaiyunWeatherKey: getEnv("CAIYUN_WEATHER_KEY", ""),
			VLMEndpoint:      getEnv("VLM_ENDPOINT", ""),
			VLMKey:           getEnv("VLM_KEY", ""),
			LLMEndpoint:      getEnv("LLM_ENDPOINT", ""),
			LLMKey:           getEnv("LLM_KEY", ""),
			LLMModel:         getEnv("LLM_MODEL", ""),
		},
	}
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
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: l})
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
