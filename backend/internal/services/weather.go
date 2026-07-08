// Package services MB2: 彩云天气代理。
package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// WeatherDay 单日天气数据。
type WeatherDay struct {
	Date    string `json:"date"`
	Skycon  string `json:"skycon"`
	TempMax float64 `json:"temp_max"`
	TempMin float64 `json:"temp_min"`
}

// WeatherWeekResult 一周天气结果。
type WeatherWeekResult struct {
	Days   []WeatherDay `json:"days"`
	Cached bool         `json:"cached"`
}

// caiyunDailyResponse 彩云天气天级预报响应。
type caiyunDailyResponse struct {
	Status  string `json:"status"`
	APIVersion string `json:"api_version"`
	Result struct {
		Daily struct {
			Temperature []struct {
				Max float64 `json:"max"`
				Min float64 `json:"min"`
			} `json:"temperature"`
			Skycon []struct {
				Value string `json:"value"`
				Date  string `json:"date"`
			} `json:"skycon"`
			Status string `json:"status"`
		} `json:"daily"`
	} `json:"result"`
}

// GetWeekWeather 获取未来一周天气。
// 先查内存缓存, 未命中调彩云 API; API 不可用时生成随机天气兜底。
func (s *WeatherService) GetWeekWeather(lat, lng float64) (*WeatherWeekResult, error) {
	cacheKey := weatherCacheKey(lat, lng)
	if result, ok := weatherCache.Get(cacheKey); ok {
		result.Cached = true
		return &result, nil
	}

	if s.cfg.CaiyunWeatherKey == "" {
		slog.Debug("彩云天气 Key 未配置, 使用随机天气")
		result := randomWeather()
		weatherCache.Set(cacheKey, result, time.Hour) // 随机天气缓存 1 小时
		return &result, nil
	}

	result, err := s.callCaiyun(lat, lng)
	if err != nil {
		slog.Warn("彩云天气 API 调用失败, 降级为随机天气", "err", err)
		result = randomWeather()
	}

	weatherCache.Set(cacheKey, result, 3*time.Hour)
	return &result, nil
}

func (s *WeatherService) callCaiyun(lat, lng float64) (WeatherWeekResult, error) {
	url := fmt.Sprintf("https://api.caiyunapp.com/v2.5/%s/%f,%f/daily.json",
		s.cfg.CaiyunWeatherKey, lng, lat)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return WeatherWeekResult{}, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WeatherWeekResult{}, fmt.Errorf("caiyun returned status %d", resp.StatusCode)
	}

	var cr caiyunDailyResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return WeatherWeekResult{}, fmt.Errorf("json decode failed: %w", err)
	}
	if cr.Status != "ok" {
		return WeatherWeekResult{}, fmt.Errorf("caiyun status: %s", cr.Status)
	}

	// skycon 映射到游戏天气: CLEAR_DAY/CLOUDY/RAIN/SNOW/WIND/HAZE 等
	days := make([]WeatherDay, 0, len(cr.Result.Daily.Skycon))
	for i, sk := range cr.Result.Daily.Skycon {
		day := WeatherDay{
			Date:   sk.Date,
			Skycon: skyconToGameWeather(sk.Value),
		}
		if i < len(cr.Result.Daily.Temperature) {
			day.TempMax = cr.Result.Daily.Temperature[i].Max
			day.TempMin = cr.Result.Daily.Temperature[i].Min
		}
		days = append(days, day)
	}
	return WeatherWeekResult{Days: days}, nil
}

// skyconToGameWeather 将彩云 skycon 值映射到游戏内天气类型。
// 参考 3.5 天气概率表:
// 晴天类: CLEAR_DAY/CLEAR_NIGHT → CLEAR
// 多云类: PARTLY_CLOUDY_DAY/PARTLY_CLOUDY_NIGHT/CLOUDY → CLOUDY
// 雨天类: LIGHT_RAIN/MODERATE_RAIN/HEAVY_RAIN/STORM_RAIN → RAIN
// 雪天类: LIGHT_SNOW/MODERATE_SNOW/HEAVY_SNOW → SNOW
// 雾霾类: FOG/HAZE/SAND → HAZE
// 风天类: WIND → WIND
func skyconToGameWeather(skycon string) string {
	switch skycon {
	case "CLEAR_DAY", "CLEAR_NIGHT":
		return "CLEAR"
	case "PARTLY_CLOUDY_DAY", "PARTLY_CLOUDY_NIGHT", "CLOUDY":
		return "CLOUDY"
	case "LIGHT_RAIN", "MODERATE_RAIN", "HEAVY_RAIN", "STORM_RAIN":
		return "RAIN"
	case "LIGHT_SNOW", "MODERATE_SNOW", "HEAVY_SNOW":
		return "SNOW"
	case "FOG", "HAZE", "SAND":
		return "HAZE"
	case "WIND":
		return "WIND"
	default:
		return "CLEAR"
	}
}

// randomWeather 生成一周随机天气(兜底)。
func randomWeather() WeatherWeekResult {
	weathers := []string{"CLEAR", "CLOUDY", "RAIN", "SNOW", "HAZE", "WIND"}
	weights := []int{35, 25, 15, 5, 10, 10} // 加权随机
	total := 0
	for _, w := range weights {
		total += w
	}

	days := make([]WeatherDay, 7)
	now := time.Now()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < 7; i++ {
		r := rng.Intn(total)
		idx := 0
		cum := weights[0]
		for r >= cum {
			idx++
			cum += weights[idx]
		}
		days[i] = WeatherDay{
			Date:    now.AddDate(0, 0, i).Format("2006-01-02"),
			Skycon:  weathers[idx],
			TempMax: 20 + float64(rng.Intn(15)),
			TempMin: 10 + float64(rng.Intn(10)),
		}
	}
	return WeatherWeekResult{Days: days}
}

func weatherCacheKey(lat, lng float64) string {
	return fmt.Sprintf("weather:%.2f,%.2f", math.Floor(lat*100)/100, math.Floor(lng*100)/100)
}

var weatherCache = NewTTLCache[WeatherWeekResult](5 * time.Minute)
