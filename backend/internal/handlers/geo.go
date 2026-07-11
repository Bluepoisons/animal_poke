// Package handlers MB2: 腾讯地图代理处理函数。
package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"animalpoke/backend/internal/middleware"
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

	result, err := h.geoService.GetCityContext(c.Request.Context(), lat, lng)
	if err != nil {
		slog.Error("地级市查询失败", "err", err)
		WriteProviderError(c, err, "geo lookup failed")
		return
	}

	c.JSON(http.StatusOK, result)
}
