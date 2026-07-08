// Package services MB2: 腾讯地图逆地理编码代理。
package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"
)

// GeoCityResult 地级市查询结果。
type GeoCityResult struct {
	City     string `json:"city"`
	District string `json:"district"`
	Province string `json:"province"`
	Cached   bool   `json:"cached"`
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
// 先查内存缓存(地级市精度), 未命中再调腾讯地图 API。
func (s *GeoService) GetCity(lat, lng float64) (*GeoCityResult, error) {
	cacheKey := cityCacheKey(lat, lng)
	if result, ok := geoCache.Get(cacheKey); ok {
		result.Cached = true
		return &result, nil
	}

	if s.cfg.TencentMapKey == "" {
		slog.Debug("腾讯地图 Key 未配置, 返回空城市")
		return &GeoCityResult{}, nil
	}

	result, err := s.callTencentMap(lat, lng)
	if err != nil {
		slog.Warn("腾讯地图 API 调用失败", "err", err)
		// 降级: 返回空城市
		return &GeoCityResult{}, nil
	}

	// 缓存以城市级别精度(6 小时 TTL)
	geoCache.Set(cacheKey, *result, 6*time.Hour)
	return result, nil
}

func (s *GeoService) callTencentMap(lat, lng float64) (*GeoCityResult, error) {
	url := fmt.Sprintf("https://apis.map.qq.com/ws/geocoder/v1/?location=%f,%f&key=%s",
		lat, lng, s.cfg.TencentMapKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tencent map returned status %d", resp.StatusCode)
	}

	var geoResp tencentGeoResponse
	if err := json.NewDecoder(resp.Body).Decode(&geoResp); err != nil {
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

// cityCacheKey 按城市级精度(保留 2 位小数)生成缓存 key。
func cityCacheKey(lat, lng float64) string {
	return fmt.Sprintf("city:%.2f,%.2f", math.Floor(lat*100)/100, math.Floor(lng*100)/100)
}

// geoCache 地级市缓存(单例, 6h TTL)。
var geoCache = NewTTLCache[GeoCityResult](5 * time.Minute)
