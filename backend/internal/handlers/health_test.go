package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	Health()(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
	_, err := time.Parse(time.RFC3339, body["time"])
	assert.NoError(t, err, "time 应为 RFC3339 格式")
}

func TestLivez(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/livez", nil)
	Livez()(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestReady_NoDB_Development(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)

	Ready(ReadyDeps{DB: nil, AppEnv: "development"})(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestReady_NoDB_Production(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)

	Ready(ReadyDeps{DB: nil, AppEnv: "production"})(c)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "unavailable", body["db"])
	assert.Equal(t, "unavailable", body["db_reason"])
}

func TestReady_ConfigErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)

	Ready(ReadyDeps{
		DB:          nil,
		AppEnv:      "development",
		ReadyErrors: []string{"vision provider not configured"},
	})(c)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestReadyChecker_SetReadyErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := NewReadyChecker(ReadyDeps{AppEnv: "development"})
	// 初始 ready
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	Readyz(checker)(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// 注入错误后 unready
	checker.SetReadyErrors([]string{"boom"})
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	Readyz(checker)(c2)
	assert.Equal(t, http.StatusServiceUnavailable, w2.Code)

	// 恢复
	checker.SetReadyErrors(nil)
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	Readyz(checker)(c3)
	assert.Equal(t, http.StatusOK, w3.Code)
}
