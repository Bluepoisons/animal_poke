package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestPvPMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProductHandler()
	r.POST("/api/v1/pvp/match", h.PvPMatch)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pvp/match", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
