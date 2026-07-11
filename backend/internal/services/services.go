// Package services 联网服务实现层(MB2 腾讯地图/彩云天气, MB3 AI 编排)。
//
// 第三方 Key 已集中在 config.ThirdPartyConfig, 经后端代理调用, 客户端永不含第三方 Key。
package services

import (
	"animalpoke/backend/internal/config"
	"net/http"
)

// GeoService 腾讯地图代理(逆地理编码 / POI)。
type GeoService struct {
	cfg      *config.ThirdPartyConfig
	client   *http.Client
	mock     bool
	provider *Provider
}

// NewGeoService 构造 GeoService。
func NewGeoService(cfg *config.ThirdPartyConfig) *GeoService {
	return NewGeoServiceWithOptions(cfg, true, nil)
}

// NewGeoServiceWithOptions 带 mock 控制的构造。
func NewGeoServiceWithOptions(cfg *config.ThirdPartyConfig, mockAllowed bool, client *http.Client) *GeoService {
	budget := config.DefaultUpstreamConfig().Geo
	if client == nil {
		client = DefaultHTTPClient(budget.Timeout)
	}
	p := NewProvider(ProviderOptions{
		Name:   "geo",
		Budget: budget,
		Client: client,
	})
	return &GeoService{cfg: cfg, client: client, mock: mockAllowed, provider: p}
}

// NewGeoServiceWithProvider 完整构造（含自定义 Provider）。
func NewGeoServiceWithProvider(cfg *config.ThirdPartyConfig, mockAllowed bool, provider *Provider) *GeoService {
	client := (*http.Client)(nil)
	if provider != nil {
		client = provider.Client
	}
	if client == nil {
		client = DefaultHTTPClient(config.DefaultUpstreamConfig().Geo.Timeout)
	}
	if provider == nil {
		provider = NewProvider(ProviderOptions{Name: "geo", Budget: config.DefaultUpstreamConfig().Geo, Client: client})
	}
	return &GeoService{cfg: cfg, client: client, mock: mockAllowed, provider: provider}
}

// WeatherService 彩云天气代理。
type WeatherService struct {
	cfg      *config.ThirdPartyConfig
	client   *http.Client
	mock     bool
	provider *Provider
}

// NewWeatherService 构造 WeatherService。
func NewWeatherService(cfg *config.ThirdPartyConfig) *WeatherService {
	return NewWeatherServiceWithOptions(cfg, true, nil)
}

// NewWeatherServiceWithOptions 带 mock 控制的构造。
func NewWeatherServiceWithOptions(cfg *config.ThirdPartyConfig, mockAllowed bool, client *http.Client) *WeatherService {
	budget := config.DefaultUpstreamConfig().Weather
	if client == nil {
		client = DefaultHTTPClient(budget.Timeout)
	}
	p := NewProvider(ProviderOptions{
		Name:   "weather",
		Budget: budget,
		Client: client,
	})
	return &WeatherService{cfg: cfg, client: client, mock: mockAllowed, provider: p}
}

// NewWeatherServiceWithProvider 完整构造。
func NewWeatherServiceWithProvider(cfg *config.ThirdPartyConfig, mockAllowed bool, provider *Provider) *WeatherService {
	client := (*http.Client)(nil)
	if provider != nil {
		client = provider.Client
	}
	if client == nil {
		client = DefaultHTTPClient(config.DefaultUpstreamConfig().Weather.Timeout)
	}
	if provider == nil {
		provider = NewProvider(ProviderOptions{Name: "weather", Budget: config.DefaultUpstreamConfig().Weather, Client: client})
	}
	return &WeatherService{cfg: cfg, client: client, mock: mockAllowed, provider: provider}
}

// AIService 统一 AI 编排(视觉检测/深度分析/数值生成)。
// Vision 与 Text 使用独立 Endpoint/Key/Model，可指向同一供应商。
type AIService struct {
	cfg                 *config.ThirdPartyConfig
	client              *http.Client
	mock                bool
	visionProvider      *Provider
	llmProvider         *Provider
	statsSecret         string // STATS_HMAC_KEY（与 JWT 独立）
	statsSecretPrevious string // 可选上一版，双读/回放
}

// NewAIService 构造 AIService（开发默认允许 mock）。
func NewAIService(cfg *config.ThirdPartyConfig) *AIService {
	return NewAIServiceWithOptions(cfg, true, nil)
}

// NewAIServiceWithOptions 带 mock 与共享 client 的构造。
func NewAIServiceWithOptions(cfg *config.ThirdPartyConfig, mockAllowed bool, client *http.Client) *AIService {
	u := config.DefaultUpstreamConfig()
	if client == nil {
		client = DefaultHTTPClient(u.Vision.Timeout)
	}
	vision := NewProvider(ProviderOptions{Name: "vision", Budget: u.Vision, Client: DefaultHTTPClient(u.Vision.Timeout)})
	llm := NewProvider(ProviderOptions{Name: "llm", Budget: u.LLM, Client: DefaultHTTPClient(u.LLM.Timeout)})
	// 若调用方传入共享 client，作为 fallback 保留
	_ = client
	return &AIService{cfg: cfg, client: client, mock: mockAllowed, visionProvider: vision, llmProvider: llm}
}

// NewAIServiceWithProviders 完整构造。
func NewAIServiceWithProviders(cfg *config.ThirdPartyConfig, mockAllowed bool, vision, llm *Provider) *AIService {
	u := config.DefaultUpstreamConfig()
	if vision == nil {
		vision = NewProvider(ProviderOptions{Name: "vision", Budget: u.Vision})
	}
	if llm == nil {
		llm = NewProvider(ProviderOptions{Name: "llm", Budget: u.LLM})
	}
	return &AIService{
		cfg:            cfg,
		client:         vision.Client,
		mock:           mockAllowed,
		visionProvider: vision,
		llmProvider:    llm,
	}
}

// WithStatsSecret 注入 rarity/stats HMAC 密钥（STATS_HMAC_KEY，与 JWT 独立）。
func (s *AIService) WithStatsSecret(secret string) *AIService {
	return s.WithStatsSecrets(secret, "")
}

// WithStatsSecrets 注入当前与可选 previous stats HMAC 密钥（单写双读）。
func (s *AIService) WithStatsSecrets(secret, previous string) *AIService {
	if s != nil {
		s.statsSecret = secret
		s.statsSecretPrevious = previous
	}
	return s
}

// NewVisionService 兼容旧测试命名，实际返回 AIService。
func NewVisionService(cfg *config.ThirdPartyConfig) *AIService {
	return NewAIService(cfg)
}

// NewLLMService 兼容旧测试命名，实际返回 AIService。
func NewLLMService(cfg *config.ThirdPartyConfig) *AIService {
	return NewAIService(cfg)
}
