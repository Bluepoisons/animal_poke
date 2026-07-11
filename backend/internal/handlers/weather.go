// Package handlers MB2: 天气代理处理函数。
package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"animalpoke/backend/internal/middleware"
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
		middleware.WriteError(c, http.StatusBadRequest, "missing_params", "lat and lng are required", false, nil)
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_lat", "invalid lat", false, nil)
		return
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_lng", "invalid lng", false, nil)
		return
	}

	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		middleware.WriteError(c, http.StatusBadRequest, "coords_out_of_range", "coordinates out of range", false, nil)
		return
	}

	result, err := h.weatherService.GetWeekWeatherContext(c.Request.Context(), lat, lng)
	if err != nil {
		slog.Error("天气查询失败", "err", err)
		WriteProviderError(c, err, "weather lookup failed")
		return
	}

	c.JSON(http.StatusOK, result)
}
