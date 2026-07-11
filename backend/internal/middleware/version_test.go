package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animalpoke/backend/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestVersionMiddleware_MinClientVersionGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.VersionConfig{
		MinClientVersion: "1.2.0",
	}

	r := gin.New()
	r.Use(Version(cfg))
	r.GET("/api/v1/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	t.Run("too old client receives 426", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("X-Client-Version", "1.0.0")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUpgradeRequired, w.Code)
		assert.Contains(t, w.Body.String(), "client_too_old")
	})

	t.Run("equal version passes", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("X-Client-Version", "1.2.0")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "1.2.0", w.Header().Get("X-Min-Client-Version"))
	})

	t.Run("newer version passes", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("X-Client-Version", "2.0.0")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("unknown version passes (conservative)", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/test", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestVersionMiddleware_DeprecationHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sunset := time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC)
	cfg := config.VersionConfig{
		DeprecatedOperations: map[string]config.DeprecatedOperation{
			"GET /api/v1/old-endpoint": {
				Method:     "GET",
				Path:       "/api/v1/old-endpoint",
				SunsetDate: sunset,
				Migration:  "Migrate to /api/v2/new-endpoint",
			},
		},
	}

	r := gin.New()
	r.Use(Version(cfg))
	r.GET("/api/v1/old-endpoint", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	r.GET("/api/v1/current-endpoint", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	t.Run("deprecated endpoint emits headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/old-endpoint", nil)
		req.Header.Set("X-Client-Version", "1.0.0")
		r.ServeHTTP(w, req)
		assert.Equal(t, "true", w.Header().Get("Deprecation"))
		assert.Contains(t, w.Header().Get("Sunset"), "15 Jan 2027")
		assert.Contains(t, w.Header().Get("Warning"), "Migrate to /api/v2/new-endpoint")
	})

	t.Run("non-deprecated endpoint has no deprecation headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/current-endpoint", nil)
		r.ServeHTTP(w, req)
		assert.Empty(t, w.Header().Get("Deprecation"))
	})
}

func TestVersionMiddleware_CapabilityEcho(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.VersionConfig{}

	r := gin.New()
	r.Use(Version(cfg))
	r.GET("/api/v1/test", func(c *gin.Context) {
		caps, _ := c.Get("client_capabilities")
		c.JSON(200, gin.H{"caps": caps})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("X-Client-Capabilities", "pvp,ranking, social")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "pvp")
	assert.Contains(t, w.Body.String(), "ranking")
}

func TestVersionLess(t *testing.T) {
	assert.True(t, versionLess("1.0.0", "2.0.0"))
	assert.True(t, versionLess("1.0.0", "1.1.0"))
	assert.True(t, versionLess("1.0.0", "1.0.1"))
	assert.False(t, versionLess("2.0.0", "1.0.0"))
	assert.False(t, versionLess("1.0.0", "1.0.0"))
	assert.True(t, versionLess("0.9.0", "1.0.0"))
}
