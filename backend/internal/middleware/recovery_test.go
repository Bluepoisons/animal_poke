package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRecovery_PanicReturns500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), Recovery())
	r.GET("/boom", func(c *gin.Context) { panic("oops") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var body ErrorResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "internal server error", body.Error)
	assert.Equal(t, "internal_error", body.ReasonCode)
	assert.True(t, body.Retryable)
	assert.NotEmpty(t, body.RequestID)
}

func TestRecovery_NormalPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusTeapot) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTeapot, w.Code)
}
