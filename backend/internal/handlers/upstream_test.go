package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteProviderError_MapsStatusAndFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(middleware.ContextRequestID, "rid-1")

	err := services.NewUpstreamError("vision", services.ReasonUpstreamRateLimited, http.StatusTooManyRequests, true, 2*time.Second, errors.New("429"))
	WriteProviderError(c, err, "detection failed")

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "2", w.Header().Get("Retry-After"))
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "detection failed", body["error"])
	assert.Equal(t, services.ReasonUpstreamRateLimited, body["reason_code"])
	assert.Equal(t, true, body["retryable"])
	assert.Equal(t, "rid-1", body["request_id"])
}

func TestWriteProviderError_Timeout504(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	WriteProviderError(c, context.DeadlineExceeded, "weather lookup failed")
	assert.Equal(t, http.StatusGatewayTimeout, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, services.ReasonUpstreamTimeout, body["reason_code"])
	assert.Equal(t, true, body["retryable"])
}

func TestWriteProviderError_CanceledSilent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	WriteProviderError(c, context.Canceled, "detection failed")
	assert.Equal(t, 200, w.Code) // gin default, no write
	assert.Equal(t, 0, w.Body.Len())
}
