package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupWeatherTest() (*gin.Engine, *WeatherHandler) {
	gin.SetMode(gin.TestMode)
	cfg := &config.ThirdPartyConfig{CaiyunWeatherKey: ""}
	svc := services.NewWeatherService(cfg)
	handler := NewWeatherHandler(svc)
	r := gin.New()
	r.GET("/api/v1/weather/week", handler.GetWeek)
	return r, handler
}

func TestGetWeek_MissingParams(t *testing.T) {
	r, _ := setupWeatherTest()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/weather/week", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

func TestGetWeek_InvalidLat(t *testing.T) {
	r, _ := setupWeatherTest()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/weather/week?lat=abc&lng=116.4", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

func TestGetWeek_Success_NoKey(t *testing.T) {
	r, _ := setupWeatherTest()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/weather/week?lat=39.9&lng=116.4", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var result services.WeatherWeekResult
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Len(t, result.Days, 7) // 应返回 7 天随机天气
}
