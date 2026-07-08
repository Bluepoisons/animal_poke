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

func setupGeoTest() (*gin.Engine, *GeoHandler) {
	gin.SetMode(gin.TestMode)
	cfg := &config.ThirdPartyConfig{TencentMapKey: ""} // 无 Key, 返回空
	svc := services.NewGeoService(cfg)
	handler := NewGeoHandler(svc)
	r := gin.New()
	r.GET("/api/v1/geo/city", handler.GetCity)
	return r, handler
}

func TestGetCity_MissingParams(t *testing.T) {
	r, _ := setupGeoTest()

	tests := []struct {
		name string
		url  string
	}{
		{"missing both", "/api/v1/geo/city"},
		{"missing lng", "/api/v1/geo/city?lat=39.9"},
		{"missing lat", "/api/v1/geo/city?lng=116.4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.url, nil)
			r.ServeHTTP(w, req)
			assert.Equal(t, 400, w.Code)
		})
	}
}

func TestGetCity_InvalidParams(t *testing.T) {
	r, _ := setupGeoTest()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/geo/city?lat=abc&lng=xyz", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

func TestGetCity_OutOfRange(t *testing.T) {
	r, _ := setupGeoTest()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/geo/city?lat=100&lng=200", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

func TestGetCity_Success_NoKey(t *testing.T) {
	r, _ := setupGeoTest()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/geo/city?lat=39.9042&lng=116.4074", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var result services.GeoCityResult
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Empty(t, result.City) // 无 Key 时返回空
}
