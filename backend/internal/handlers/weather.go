// Package handlers MB2: 天气代理处理函数。
package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
)

// WeatherHandler 天气处理器。
type WeatherHandler struct {
	weatherService *services.WeatherService
}

// NewWeatherHandler 构造 WeatherHandler。
func NewWeatherHandler(weatherService *services.WeatherService) *WeatherHandler {
	return &WeatherHandler{weatherService: weatherService}
}

// GetWeek GET /weather/week?lat=xxx&lng=xxx 返回本周天气。
func (h *WeatherHandler) GetWeek(c *gin.Context) {
	latStr := c.Query("lat")
	lngStr := c.Query("lng")
	if latStr == "" || lngStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat and lng are required"})
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lat"})
		return
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lng"})
		return
	}

	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "coordinates out of range"})
		return
	}

	result, err := h.weatherService.GetWeekWeather(lat, lng)
	if err != nil {
		slog.Error("天气查询失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "weather lookup failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}
