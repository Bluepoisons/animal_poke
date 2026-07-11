package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupProduct(t *testing.T, opts ProductOptions) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewProductHandlerWithOptions(opts)
	r := gin.New()
	// 可选注入 role（模拟 JWT claim）
	r.Use(func(c *gin.Context) {
		if role := c.GetHeader("X-Test-Role"); role != "" {
			c.Set(middleware.ContextKeyRole, role)
		}
		if dev := c.GetHeader("X-Test-Device"); dev != "" {
			c.Set(middleware.ContextKeyDeviceID, dev)
		}
		c.Next()
	})
	r.GET("/api/v1/ranking/daily", h.RankingDaily)
	r.POST("/api/v1/pvp/match", h.PvPMatch)
	r.POST("/api/v1/pvp/result", h.PvPReport)
	r.GET("/api/v1/social/friends", h.FriendsList)
	r.POST("/api/v1/social/share", h.ShareCreate)
	r.GET("/api/v1/ops/metrics-summary", h.OpsMetrics)
	return r
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func TestProduct_FlagsOff_Return501FeatureUnavailable(t *testing.T) {
	r := setupProduct(t, ProductOptions{Flags: config.FeatureFlags{}}) // all false

	cases := []struct {
		method, path, feature string
	}{
		{http.MethodGet, "/api/v1/ranking/daily", "ranking"},
		{http.MethodPost, "/api/v1/pvp/match", "pvp"},
		{http.MethodPost, "/api/v1/pvp/result", "pvp"},
		{http.MethodGet, "/api/v1/social/friends", "social"},
		{http.MethodPost, "/api/v1/social/share", "social"},
		{http.MethodGet, "/api/v1/ops/metrics-summary", "ops"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotImplemented, w.Code)
			body := decodeJSON(t, w)
			assert.Equal(t, "feature_unavailable", body["reason_code"])
			assert.Equal(t, tc.feature, body["feature"])
			// 不得出现假成功字段
			assert.NotContains(t, body, "match_id")
			assert.NotContains(t, body, "share_id")
			assert.NotContains(t, body, "entries")
			assert.NotContains(t, body, "dau")
		})
	}
}

func TestPvPMatch(t *testing.T) {
	// 默认 handler 全关：501
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProductHandler()
	r.POST("/api/v1/pvp/match", h.PvPMatch)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pvp/match", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, "feature_unavailable", body["reason_code"])
}

func TestPvPMatch_FlagOn_NoEmptyMatchID(t *testing.T) {
	r := setupProduct(t, ProductOptions{Flags: config.FeatureFlags{PvP: true}})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pvp/match", nil)
	r.ServeHTTP(w, req)
	// flag 开但实现未就绪：503，不得 2xx + 空 match_id
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, "feature_unavailable", body["reason_code"])
	assert.NotContains(t, body, "match_id")
}

func TestFriendsList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProductHandler()
	r.GET("/api/v1/social/friends", h.FriendsList)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/social/friends", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, "feature_unavailable", body["reason_code"])
}

func TestShareCreate_FlagOn_NoPendingShare(t *testing.T) {
	r := setupProduct(t, ProductOptions{Flags: config.FeatureFlags{Social: true}})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/social/share", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, "feature_unavailable", body["reason_code"])
	assert.NotContains(t, body, "share_id")
}

func TestRankingDaily_FlagOn(t *testing.T) {
	r := setupProduct(t, ProductOptions{Flags: config.FeatureFlags{Ranking: true}})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ranking/daily?city=Shanghai", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, "Shanghai", body["city"])
	assert.Equal(t, "server", body["source"])
}

func TestOpsMetrics_FlagOff(t *testing.T) {
	r := setupProduct(t, ProductOptions{Flags: config.FeatureFlags{Ops: false}, OpsToken: "secret"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ops/metrics-summary", nil)
	req.Header.Set("X-AP-Ops-Token", "secret")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, "feature_unavailable", body["reason_code"])
}

func TestOpsMetrics_FlagOn_NoToken_Forbidden(t *testing.T) {
	r := setupProduct(t, ProductOptions{Flags: config.FeatureFlags{Ops: true}, OpsToken: "secret"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ops/metrics-summary", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, "ops_forbidden", body["reason_code"])
	assert.NotContains(t, body, "dau")
}

func TestOpsMetrics_FlagOn_WrongToken_Forbidden(t *testing.T) {
	r := setupProduct(t, ProductOptions{Flags: config.FeatureFlags{Ops: true}, OpsToken: "secret"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ops/metrics-summary", nil)
	req.Header.Set("X-AP-Ops-Token", "wrong")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestOpsMetrics_FlagOn_ValidToken(t *testing.T) {
	r := setupProduct(t, ProductOptions{Flags: config.FeatureFlags{Ops: true}, OpsToken: "secret"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ops/metrics-summary", nil)
	req.Header.Set("X-AP-Ops-Token", "secret")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, "analytics_store", body["source"])
}

func TestOpsMetrics_FlagOn_RoleClaim(t *testing.T) {
	r := setupProduct(t, ProductOptions{Flags: config.FeatureFlags{Ops: true}, OpsToken: "secret"})
	for _, role := range []string{"admin", "ops", "internal"} {
		t.Run(role, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/ops/metrics-summary", nil)
			req.Header.Set("X-Test-Role", role)
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
	// ordinary device role rejected
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ops/metrics-summary", nil)
	req.Header.Set("X-Test-Role", "device")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
