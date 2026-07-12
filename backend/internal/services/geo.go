// Package services MB2: 腾讯地图逆地理编码代理。
package services

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"time"
)

// GeoCityResult 地级市查询结果。
type GeoCityResult struct {
	City       string `json:"city"`
	District   string `json:"district"`
	Province   string `json:"province"`
	Cached     bool   `json:"cached"`
	Source     string `json:"source,omitempty"`
	Degraded   bool   `json:"degraded,omitempty"`
	ReasonCode string `json:"reason_code,omitempty"`
}

// tencentGeoResponse 腾讯地图逆地理编码响应结构。
type tencentGeoResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Result  struct {
		AddressComponent struct {
			Province string `json:"province"`
			City     string `json:"city"`
			District string `json:"district"`
		} `json:"address_component"`
	} `json:"result"`
}

// GetCity 根据 GPS 坐标获取地级市名称。
func (s *GeoService) GetCity(lat, lng float64) (*GeoCityResult, error) {
	return s.GetCityContext(context.Background(), lat, lng)
}

// GetCityContext 带 context 的城市查询。
func (s *GeoService) GetCityContext(ctx context.Context, lat, lng float64) (*GeoCityResult, error) {
	cacheKey := cityCacheKey(lat, lng)
	if result, ok := geoCache.Get(cacheKey); ok {
		result.Cached = true
		if result.Source == "" {
			result.Source = "cache"
		}
		return &result, nil
	}

	if s.cfg.TencentMapKey == "" {
		slog.Debug("腾讯地图 Key 未配置, 返回空城市")
		return &GeoCityResult{Source: "mock", Degraded: true, ReasonCode: "provider_not_configured"}, nil
	}

	result, err := s.callTencentMap(ctx, lat, lng)
	if err != nil {
		slog.Warn("腾讯地图 API 调用失败", "err", err)
		return &GeoCityResult{Source: "mock", Degraded: true, ReasonCode: "provider_error"}, nil
	}
	result.Source = "real"
	geoCache.Set(cacheKey, *result, 6*time.Hour)
	return result, nil
}

func (s *GeoService) callTencentMap(ctx context.Context, lat, lng float64) (*GeoCityResult, error) {
	const uri = "/ws/geocoder/v1/"
	params := url.Values{
		"key":      {s.cfg.TencentMapKey},
		"location": {fmt.Sprintf("%f,%f", lat, lng)},
	}
	query := params.Encode()
	endpoint := "https://apis.map.qq.com" + uri + "?" + query
	if s.cfg.TencentMapSK != "" {
		endpoint += "&sig=" + tencentMapSignature(uri, query, s.cfg.TencentMapSK)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	var (
		resp *http.Response
		body []byte
		err2 error
	)
	if s.provider != nil {
		resp, body, err2 = s.provider.Do(ctx, req, 1<<20)
	} else {
		client := s.client
		if client == nil {
			client = DefaultHTTPClient(5 * time.Second)
		}
		resp, body, err2 = DoWithRetry(ctx, client, req, 1, 1<<20)
	}
	if err2 != nil {
		return nil, err2
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tencent map returned status %d", resp.StatusCode)
	}

	var geoResp tencentGeoResponse
	if err := json.Unmarshal(body, &geoResp); err != nil {
		return nil, fmt.Errorf("json decode failed: %w", err)
	}
	if geoResp.Status != 0 {
		return nil, fmt.Errorf("tencent map error: %s", geoResp.Message)
	}

	return &GeoCityResult{
		Province: geoResp.Result.AddressComponent.Province,
		City:     geoResp.Result.AddressComponent.City,
		District: geoResp.Result.AddressComponent.District,
	}, nil
}

// tencentMapSignature 按腾讯位置服务规则生成 GET 请求的 sig 签名。
// 签名时的参数顺序必须与实际发出的请求保持一致。
func tencentMapSignature(uri, encodedQuery, secretKey string) string {
	query, err := url.QueryUnescape(encodedQuery)
	if err != nil {
		// url.Values.Encode 总会生成合法编码；这里保留安全降级，避免未来手动构造
		// 参数时因畸形输入导致请求处理 panic。
		query = encodedQuery
	}
	sum := md5.Sum([]byte(uri + "?" + query + secretKey))
	return hex.EncodeToString(sum[:])
}

func cityCacheKey(lat, lng float64) string {
	return fmt.Sprintf("city:%.2f,%.2f", math.Floor(lat*100)/100, math.Floor(lng*100)/100)
}

var geoCache = NewBoundedTTLCache[GeoCityResult](5*time.Minute, 2048)

// RoundCoord 粗化坐标到约 1.1km 精度（隐私最小化）。
func RoundCoord(v float64) float64 {
	return math.Floor(v*100) / 100
}

// EncodeGeoHash 简易 geohash 近似（两位小数网格）。
func EncodeGeoHash(lat, lng float64) string {
	return fmt.Sprintf("%.2f,%.2f", RoundCoord(lat), RoundCoord(lng))
}
