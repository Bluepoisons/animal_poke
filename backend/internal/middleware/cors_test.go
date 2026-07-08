package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORS_NonOptionsAddsHeadersAndPassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	r := gin.New()
	r.Use(CORS())
	r.GET("/x", func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	assert.True(t, called, "下游 handler 应被执行")
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Origin, Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))
}

func TestCORS_OptionsShortCircuits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	r := gin.New()
	r.Use(CORS())
	// 显式注册 OPTIONS 路由, 否则 gin 不会进入中间件链
	r.OPTIONS("/x", func(c *gin.Context) { called = true })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	r.ServeHTTP(w, req)

	assert.False(t, called, "OPTIONS 时 CORS 应 Abort, 下游 handler 不应执行")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}
