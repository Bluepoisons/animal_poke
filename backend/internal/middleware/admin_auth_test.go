package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animalpoke/backend/internal/admin"
	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdminRouter(t *testing.T, cfg middleware.AdminAuthConfig, perm string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	g := r.Group("/admin")
	g.Use(middleware.AdminAuthRBAC(cfg))
	if perm != "" {
		g.Use(middleware.RequireAdminPermission(perm, cfg.Auditor))
	}
	g.GET("/ping", func(c *gin.Context) {
		a := middleware.GetAdminActor(c)
		c.JSON(200, gin.H{"actor": a.ActorID, "role": a.Role, "mode": a.AuthMode})
	})
	g.POST("/write", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true, "reason": middleware.GetAdminReason(c)})
	})
	return r
}

func TestAdminAuth_JWTHappyPath(t *testing.T) {
	store := admin.NewSessionStore(nil)
	tokens := admin.NewTokenService(admin.TokenConfig{
		Secret: "admin-secret-for-tests-32chars!!", Issuer: "animal-poke-admin",
		Audience: "animal-poke-admin-test", Env: "test", TTL: time.Hour,
	}, store)
	res, err := tokens.Issue("alice", "alice", admin.RoleFinance, "jwt")
	require.NoError(t, err)

	r := setupAdminRouter(t, middleware.AdminAuthConfig{
		Tokens: tokens, Sessions: store, Env: "test",
	}, admin.PermCommerceRefund)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/ping", nil)
	req.Header.Set("Authorization", "Bearer "+res.Token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "alice", body["actor"])
	assert.Equal(t, admin.RoleFinance, body["role"])
}

func TestAdminAuth_SupportDeniedRefund(t *testing.T) {
	store := admin.NewSessionStore(nil)
	tokens := admin.NewTokenService(admin.TokenConfig{
		Secret: "admin-secret-for-tests-32chars!!", Issuer: "animal-poke-admin",
		Audience: "animal-poke-admin-test", Env: "test", TTL: time.Hour,
	}, store)
	res, err := tokens.Issue("support1", "support1", admin.RoleSupport, "jwt")
	require.NoError(t, err)

	r := setupAdminRouter(t, middleware.AdminAuthConfig{
		Tokens: tokens, Sessions: store, Env: "test",
	}, admin.PermCommerceRefund)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/ping", nil)
	req.Header.Set("Authorization", "Bearer "+res.Token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
	assert.Contains(t, w.Body.String(), "admin_permission_denied")
}

func TestAdminAuth_ProductionSharedKeyForbidden(t *testing.T) {
	r := setupAdminRouter(t, middleware.AdminAuthConfig{
		AdminAPIKey: "shared-key", BreakGlassEnabled: false, Production: true, Env: "production",
	}, "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/ping", nil)
	req.Header.Set("X-Admin-Key", "shared-key")
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
	assert.Contains(t, w.Body.String(), "shared_key_forbidden")
}

func TestAdminAuth_BreakGlassWithActorReason(t *testing.T) {
	r := setupAdminRouter(t, middleware.AdminAuthConfig{
		AdminAPIKey: "shared-key", BreakGlassEnabled: true, Production: true, Env: "production",
	}, "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/ping", nil)
	req.Header.Set("X-Admin-Key", "shared-key")
	req.Header.Set("X-Admin-Actor", "oncall@example.com")
	req.Header.Set("X-Admin-Reason", "sev1 outage")
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "break_glass")
	assert.Contains(t, w.Body.String(), "oncall@example.com")
}

func TestAdminAuth_SessionRevoked(t *testing.T) {
	store := admin.NewSessionStore(nil)
	tokens := admin.NewTokenService(admin.TokenConfig{
		Secret: "admin-secret-for-tests-32chars!!", Issuer: "animal-poke-admin",
		Audience: "animal-poke-admin-test", Env: "test", TTL: time.Hour,
	}, store)
	res, err := tokens.Issue("sec", "sec", admin.RoleSecurity, "jwt")
	require.NoError(t, err)
	require.NoError(t, store.Revoke(res.SessionID, "super"))

	r := setupAdminRouter(t, middleware.AdminAuthConfig{
		Tokens: tokens, Sessions: store, Env: "test",
	}, "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/ping", nil)
	req.Header.Set("Authorization", "Bearer "+res.Token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "admin_session_revoked")
}

func TestAdminAuth_WriteRequiresReason(t *testing.T) {
	store := admin.NewSessionStore(nil)
	tokens := admin.NewTokenService(admin.TokenConfig{
		Secret: "admin-secret-for-tests-32chars!!", Issuer: "animal-poke-admin",
		Audience: "animal-poke-admin-test", Env: "test", TTL: time.Hour,
	}, store)
	res, err := tokens.Issue("ops1", "ops1", admin.RoleOps, "jwt")
	require.NoError(t, err)

	r := setupAdminRouter(t, middleware.AdminAuthConfig{
		Tokens: tokens, Sessions: store, Env: "test", RequireReasonOnWrite: true,
	}, "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/write", nil)
	req.Header.Set("Authorization", "Bearer "+res.Token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "admin_reason_required")
}
