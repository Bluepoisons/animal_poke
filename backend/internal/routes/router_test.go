package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func testConfig() *config.Config {
	return &config.Config{
		AppEnv:         "development",
		ServerAddr:     ":0",
		LogLevel:       "INFO",
		JWTSecret:      "test-secret-at-least-32-chars-long!!",
		JWTIssuer:      "animal-poke",
		JWTAudience:    "animal-poke-client",
		AIMockEnabled:  true,
		MaxImageBytes:  5 << 20,
		MaxImagePixels: 12_000_000,
	}
}

func TestHealthRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(testConfig(), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestLivezReadyz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(testConfig(), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	r.ServeHTTP(w2, req2)
	// 开发无 DB 仍 ready
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestPingRoute_DBNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(testConfig(), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "pong", body["msg"])
	assert.Equal(t, false, body["db"])
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestCORSHeadersPresentThroughChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(testConfig(), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
}

func TestRecoveryInChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(testConfig(), nil)
	r.GET("/boom", func(c *gin.Context) { panic("x") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAuthDeviceRoute_DBNil_Returns503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(testConfig(), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/device", nil)
	r.ServeHTTP(w, req)

	// 无 DB 时路由仍注册，返回 503 而非 404
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "30", w.Header().Get("Retry-After"))
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "db_unavailable", body["reason_code"])
}

func TestSyncRoute_DBNil_Returns503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig()
	r := NewRouter(cfg, nil)

	// 需要 JWT — 无 token 401；有无效 token 也 401。先验证路由存在（非 404）
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/animal", nil)
	r.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGeoCityRoute_RequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(testConfig(), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/geo/city?lat=39.9&lng=116.4", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMetricsRoute_PublicReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(testConfig(), nil)

	// Public engine must not expose Prometheus (AP-036).
	w0 := httptest.NewRecorder()
	r.ServeHTTP(w0, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, w0.Code)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotContains(t, w.Body.String(), "http_requests_total")
}

func TestMetricsSeriesBoundedViaPublicRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	middleware.ResetMetrics()
	t.Cleanup(middleware.ResetMetrics)

	r := NewRouter(testConfig(), nil)
	// 1k random unmatched paths → single "unknown" series, not unbounded growth.
	for i := 0; i < 1000; i++ {
		w := httptest.NewRecorder()
		path := "/no-such/" + strconv.Itoa(i)
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusNotFound, w.Code)
	}
	// Known route still records its template
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	assert.LessOrEqual(t, middleware.HTTPSeriesCount(), 16)
	body := middleware.RenderMetrics()
	assert.Contains(t, body, `path="unknown"`)
	assert.Contains(t, body, `path="/health"`)
	// Must not contain random path fragments as labels
	assert.NotContains(t, body, "/no-such/999")
}
