package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestErrorReport_Accepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewErrorReportHandler()
	r.POST("/api/v1/errors/report", h.Report)
	w := httptest.NewRecorder()
	body := `{"message":"boom","stack":"at x","route":"/discover","release":"1.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/errors/report", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestErrorReport_RequiresMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewErrorReportHandler()
	r.POST("/api/v1/errors/report", h.Report)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/errors/report", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
