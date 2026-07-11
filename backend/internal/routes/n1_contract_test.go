package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestN1CompatibilityMatrix validates N-1 contract behaviors.
func TestN1CompatibilityMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	// Register a deprecated operation for testing
	cfg.RegisterDeprecation("GET", "/api/v1/deprecated-resource",
		"Migrate to /api/v2/resource",
		time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC))

	vc := cfg.VersionConfig()

	r := gin.New()
	r.Use(middleware.Version(vc))
	r.GET("/api/v1/deprecated-resource", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": "legacy"})
	})
	r.GET("/api/v1/current-resource", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": "current"})
	})
	r.GET("/api/v1/version", func(c *gin.Context) {
		c.JSON(200, gin.H{"api_version": "v1"})
	})

	t.Run("old client on current endpoint succeeds", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/current-resource", nil)
		req.Header.Set("X-Client-Version", "0.9.0")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("deprecated endpoint emits sunset headers to old client", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/deprecated-resource", nil)
		req.Header.Set("X-Client-Version", "1.0.0")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "true", w.Header().Get("Deprecation"))
		assert.NotEmpty(t, w.Header().Get("Sunset"))
		assert.Contains(t, w.Header().Get("Warning"), "Migrate")
	})

	t.Run("deprecated endpoint still returns correct data", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/deprecated-resource", nil)
		req.Header.Set("X-Client-Version", "1.0.0")
		r.ServeHTTP(w, req)
		var body map[string]string
		require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
		assert.Equal(t, "legacy", body["data"])
	})

	t.Run("version endpoint works for old client", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/version", nil)
		req.Header.Set("X-Client-Version", "0.9.0")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var info map[string]string
		require.NoError(t, json.NewDecoder(w.Body).Decode(&info))
		assert.Equal(t, "v1", info["api_version"])
	})

	t.Run("X-Client-Version-Received echoed for all requests", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/current-resource", nil)
		req.Header.Set("X-Client-Version", "1.5.2")
		r.ServeHTTP(w, req)
		assert.Equal(t, "1.5.2", w.Header().Get("X-Client-Version-Received"))
	})
}

// TestN1BreakingChangeDetection verifies that removed endpoints return 404.
func TestN1BreakingChangeDetection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}

	r := gin.New()
	r.Use(middleware.Version(cfg.VersionConfig()))
	r.GET("/api/v1/stable-endpoint", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	t.Run("stable endpoint works with old client", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/stable-endpoint", nil)
		req.Header.Set("X-Client-Version", "0.9.0")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("removed endpoint returns 404 with X-Client-Version", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/removed-endpoint", nil)
		req.Header.Set("X-Client-Version", "0.9.0")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
