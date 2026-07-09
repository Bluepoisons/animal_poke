// Package services 联网服务实现层(MB2 腾讯地图/彩云天气, MB3 AI 编排)。
// F6 仅建立目录与结构, 具体实现见对应后端任务。
//
// 第三方 Key 已集中在 config.ThirdPartyConfig, 经后端代理调用, 客户端永不含第三方 Key。
// 下面给出各服务的占位骨架, 后续任务填充真实调用逻辑。
package services

import "animalpoke/backend/internal/config"

// GeoService 腾讯地图代理(逆地理编码 / POI)。MB2 实现。
type GeoService struct {
	cfg *config.ThirdPartyConfig
}

// NewGeoService 构造 GeoService。
func NewGeoService(cfg *config.ThirdPartyConfig) *GeoService {
	return &GeoService{cfg: cfg}
}

// WeatherService 彩云天气代理。MB2 实现。
type WeatherService struct {
	cfg *config.ThirdPartyConfig
}

// NewWeatherService 构造 WeatherService。
func NewWeatherService(cfg *config.ThirdPartyConfig) *WeatherService {
	return &WeatherService{cfg: cfg}
}

// AIService 统一 AI 编排(视觉检测/深度分析/数值生成), 共用同一 LLM 端点与模型。
type AIService struct {
	cfg *config.ThirdPartyConfig
}

// NewAIService 构造 AIService。
func NewAIService(cfg *config.ThirdPartyConfig) *AIService {
	return &AIService{cfg: cfg}
}
