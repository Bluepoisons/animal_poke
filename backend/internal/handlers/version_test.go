package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"animalpoke/backend/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestVersionHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	r := gin.New()
	r.GET("/api/v1/version", Version(cfg))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/version", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"api_version":"v1"`)
}
