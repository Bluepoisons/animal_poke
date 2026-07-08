package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHealthRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestPingRoute_DBNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "pong", body["msg"])
	assert.Equal(t, false, body["db"])
}

func TestCORSHeadersPresentThroughChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
}

func TestRecoveryInChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(nil, nil)
	r.GET("/boom", func(c *gin.Context) { panic("x") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
