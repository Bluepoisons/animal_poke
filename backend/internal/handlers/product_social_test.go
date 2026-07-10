package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestFriendsList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProductHandler()
	r.GET("/api/v1/social/friends", h.FriendsList)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/social/friends", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
