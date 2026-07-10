// Package services 联网服务实现层(MB2 腾讯地图/彩云天气, MB3 AI 编排)。
//
// 第三方 Key 已集中在 config.ThirdPartyConfig, 经后端代理调用, 客户端永不含第三方 Key。
package services

import (
	"animalpoke/backend/internal/config"
	"net/http"
	"time"
)

// GeoService 腾讯地图代理(逆地理编码 / POI)。
type GeoService struct {
	cfg    *config.ThirdPartyConfig
	client *http.Client
	mock   bool
}

// NewGeoService 构造 GeoService。
func NewGeoService(cfg *config.ThirdPartyConfig) *GeoService {
	return &GeoService{cfg: cfg, client: DefaultHTTPClient(5 * time.Second), mock: true}
}

// NewGeoServiceWithOptions 带 mock 控制的构造。
func NewGeoServiceWithOptions(cfg *config.ThirdPartyConfig, mockAllowed bool, client *http.Client) *GeoService {
	if client == nil {
		client = DefaultHTTPClient(5 * time.Second)
	}
	return &GeoService{cfg: cfg, client: client, mock: mockAllowed}
}

// WeatherService 彩云天气代理。
type WeatherService struct {
	cfg    *config.ThirdPartyConfig
	client *http.Client
	mock   bool
}

// NewWeatherService 构造 WeatherService。
func NewWeatherService(cfg *config.ThirdPartyConfig) *WeatherService {
	return &WeatherService{cfg: cfg, client: DefaultHTTPClient(5 * time.Second), mock: true}
}

// NewWeatherServiceWithOptions 带 mock 控制的构造。
func NewWeatherServiceWithOptions(cfg *config.ThirdPartyConfig, mockAllowed bool, client *http.Client) *WeatherService {
	if client == nil {
		client = DefaultHTTPClient(5 * time.Second)
	}
	return &WeatherService{cfg: cfg, client: client, mock: mockAllowed}
}

// AIService 统一 AI 编排(视觉检测/深度分析/数值生成)。
// Vision 与 Text 使用独立 Endpoint/Key/Model，可指向同一供应商。
type AIService struct {
	cfg    *config.ThirdPartyConfig
	client *http.Client
	mock   bool
}

// NewAIService 构造 AIService（开发默认允许 mock）。
func NewAIService(cfg *config.ThirdPartyConfig) *AIService {
	return &AIService{cfg: cfg, client: DefaultHTTPClient(30 * time.Second), mock: true}
}

// NewAIServiceWithOptions 带 mock 与共享 client 的构造。
func NewAIServiceWithOptions(cfg *config.ThirdPartyConfig, mockAllowed bool, client *http.Client) *AIService {
	if client == nil {
		client = DefaultHTTPClient(30 * time.Second)
	}
	return &AIService{cfg: cfg, client: client, mock: mockAllowed}
}

// NewVisionService 兼容旧测试命名，实际返回 AIService。
func NewVisionService(cfg *config.ThirdPartyConfig) *AIService {
	return NewAIService(cfg)
}

// NewLLMService 兼容旧测试命名，实际返回 AIService。
func NewLLMService(cfg *config.ThirdPartyConfig) *AIService {
	return NewAIService(cfg)
}
