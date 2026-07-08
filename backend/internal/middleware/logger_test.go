package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestLogger_Passthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Logger())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusCreated) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestLogger_CapturesAttributes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	oldDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	r := gin.New()
	r.Use(Logger())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	out := buf.String()
	assert.Contains(t, out, "http")
	assert.Contains(t, out, "method=GET")
	assert.Contains(t, out, "path=/x")
	assert.Contains(t, out, "status=200")
	assert.Contains(t, out, "latency_ms=")
}
