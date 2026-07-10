// Package handlers MB2: 腾讯地图代理处理函数。
package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
)

// GeoHandler 地理位置处理器。
type GeoHandler struct {
	geoService *services.GeoService
}

// NewGeoHandler 构造 GeoHandler。
func NewGeoHandler(geoService *services.GeoService) *GeoHandler {
	return &GeoHandler{geoService: geoService}
}

// GetCity GET /geo/city?lat=xxx&lng=xxx 返回地级市。
func (h *GeoHandler) GetCity(c *gin.Context) {
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

	result, err := h.geoService.GetCityContext(c.Request.Context(), lat, lng)
	if err != nil {
		slog.Error("地级市查询失败", "err", err)
		WriteProviderError(c, err, "geo lookup failed")
		return
	}

	c.JSON(http.StatusOK, result)
}
