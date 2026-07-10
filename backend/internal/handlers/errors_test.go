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

func TestRedact_StripsSecretsCoordsPhotos(t *testing.T) {
	in := "Bearer abc.def.ghi token lat:31.2304,lng:121.4737 data:image/jpeg;base64,/9j/4AAQ password=super"
	out := redact(in)
	assert.NotContains(t, out, "abc.def.ghi")
	assert.NotContains(t, out, "31.2304")
	assert.NotContains(t, out, "base64,/9j")
	assert.Contains(t, out, "[redacted")
}

func TestErrorReport_AcceptsRequestIDAndExtra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewErrorReportHandler()
	r.POST("/api/v1/errors/report", h.Report)
	w := httptest.NewRecorder()
	body := `{"message":"boom","component":"CaptureScreen","route":"/capture","release":"deadbeef","level":"error","request_id":"client-rid","extra":{"phase":"detect","token":"should-redact"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/errors/report", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)
}
