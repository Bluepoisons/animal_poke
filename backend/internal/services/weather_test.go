package services

import (
	"testing"
	"time"

	"animalpoke/backend/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestSkyconToGameWeather(t *testing.T) {
	tests := []struct {
		skycon string
		want   string
	}{
		{"CLEAR_DAY", "CLEAR"},
		{"CLEAR_NIGHT", "CLEAR"},
		{"PARTLY_CLOUDY_DAY", "CLOUDY"},
		{"PARTLY_CLOUDY_NIGHT", "CLOUDY"},
		{"CLOUDY", "CLOUDY"},
		{"LIGHT_RAIN", "RAIN"},
		{"MODERATE_RAIN", "RAIN"},
		{"HEAVY_RAIN", "RAIN"},
		{"STORM_RAIN", "RAIN"},
		{"LIGHT_SNOW", "SNOW"},
		{"MODERATE_SNOW", "SNOW"},
		{"HEAVY_SNOW", "SNOW"},
		{"FOG", "HAZE"},
		{"HAZE", "HAZE"},
		{"SAND", "HAZE"},
		{"WIND", "WIND"},
		{"UNKNOWN_TYPE", "CLEAR"},
	}
	for _, tt := range tests {
		t.Run(tt.skycon, func(t *testing.T) {
			assert.Equal(t, tt.want, skyconToGameWeather(tt.skycon))
		})
	}
}

func TestWeatherService_GetWeek_NoKey(t *testing.T) {
	cfg := &config.ThirdPartyConfig{CaiyunWeatherKey: ""}
	svc := NewWeatherService(cfg)

	result, err := svc.GetWeekWeather(39.9, 116.4)
	assert.NoError(t, err)
	assert.Len(t, result.Days, 7)
}

func TestWeatherService_GetWeek_Cached(t *testing.T) {
	cfg := &config.ThirdPartyConfig{CaiyunWeatherKey: ""}
	svc := NewWeatherService(cfg)

	key := weatherCacheKey(39.9, 116.4)
	weatherCache.Set(key, WeatherWeekResult{Days: []WeatherDay{{Date: "2025-01-01", Skycon: "CLEAR"}}}, time.Hour)
	defer weatherCache.Delete(key)

	result, err := svc.GetWeekWeather(39.9, 116.4)
	assert.NoError(t, err)
	assert.True(t, result.Cached)
	assert.Equal(t, "CLEAR", result.Days[0].Skycon)
}

func TestRandomWeather(t *testing.T) {
	result := randomWeather()
	assert.Len(t, result.Days, 7)
	valid := map[string]bool{"CLEAR": true, "CLOUDY": true, "RAIN": true, "SNOW": true, "HAZE": true, "WIND": true}
	for _, day := range result.Days {
		assert.True(t, valid[day.Skycon], "unexpected weather type: %s", day.Skycon)
	}
}

func TestWeatherCacheKey(t *testing.T) {
	key := weatherCacheKey(39.9042, 116.4074)
	assert.Contains(t, key, "weather:")
}

func TestWeatherService_GracefulDegradation(t *testing.T) {
	// 即使无 Key 也不应报错
	cfg := &config.ThirdPartyConfig{CaiyunWeatherKey: ""}
	svc := NewWeatherService(cfg)

	result, err := svc.GetWeekWeather(25.0, 121.5)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.Days)
}
