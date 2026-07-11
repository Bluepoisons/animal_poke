package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animalpoke/backend/internal/admin"
	"animalpoke/backend/internal/handlers"
	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAdminAPI(t *testing.T) (*gin.Engine, *admin.TokenService, *admin.SessionStore, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:admin_rbac_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AdminSession{}, &models.AdminActionLog{}, &models.SecurityReport{}, &models.Order{}, &models.Entitlement{}, &models.AuditLog{}, &models.Product{}))

	store := admin.NewSessionStore(db)
	tokens := admin.NewTokenService(admin.TokenConfig{
		Secret: "admin-secret-for-tests-32chars!!", Issuer: "animal-poke-admin",
		Audience: "animal-poke-admin-test", Env: "test", TTL: time.Hour,
	}, store)
	auditor := admin.NewActionAuditor(db, "admin-secret-for-tests-32chars!!")
	h := handlers.NewAdminHandler(handlers.AdminHandlerOptions{
		Tokens: tokens, Sessions: store, Auditor: auditor, DB: db,
		AdminAPIKey: "shared", BreakGlass: true, Production: false,
		DevIssueKey: "dev-issue-secret", Env: "test",
	})
	authCfg := middleware.AdminAuthConfig{
		Tokens: tokens, Sessions: store, Auditor: auditor,
		AdminAPIKey: "shared", BreakGlassEnabled: true, Production: false, Env: "test",
		RequireReasonOnWrite: true,
	}
	r := gin.New()
	r.Use(middleware.RequestID())
	g := r.Group("/api/v1/admin")
	g.POST("/auth/token", middleware.OptionalAdminAuth(authCfg), h.IssueToken)
	sec := g.Group("")
	sec.Use(middleware.AdminAuthRBAC(authCfg))
	sec.PUT("/config/game", middleware.RequireAdminPermission(admin.PermConfigWrite, auditor), h.WriteGameConfig)
	sec.GET("/security/reports/:id", middleware.RequireAdminPermission(admin.PermSecurityReportMeta, auditor), h.GetSecurityReport)
	sec.POST("/sessions/revoke", middleware.RequireAdminPermission(admin.PermSessionRevoke, auditor), h.RevokeSession)
	sec.POST("/commerce/orders/refund", middleware.RequireAdminPermission(admin.PermCommerceRefund, auditor), handlers.NewCommerceHandlerWithOptions(db, handlers.CommerceOptions{Enabled: true}).AdminRefundOrder)
	return r, tokens, store, db
}

func issueRole(t *testing.T, r *gin.Engine, actor, role string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"actor": actor, "role": role, "dev_secret": "dev-issue-secret"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp["token"].(string)
}

func TestAdminRBAC_SupportCannotConfigOrRefund(t *testing.T) {
	r, _, _, db := setupAdminAPI(t)
	tok := issueRole(t, r, "support@ex.com", admin.RoleSupport)

	// config write denied
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/config/game", bytes.NewReader([]byte(`{"x":1}`)))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Reason", "try config")
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)

	// refund denied
	require.NoError(t, db.Create(&models.Order{
		OrderID: "ord-1", DeviceID: "dev-1", ProductID: "p1", Status: "fulfilled",
		AmountCents: 100, Currency: "CNY",
	}).Error)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/commerce/orders/refund",
		bytes.NewReader([]byte(`{"order_id":"ord-1","reason":"customer"}`)))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Reason", "customer request")
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}

func TestAdminRBAC_FinanceCannotReadSecurityBody(t *testing.T) {
	r, _, _, db := setupAdminAPI(t)
	require.NoError(t, db.Create(&models.SecurityReport{
		ReportID: "rep-1", DeviceID: "dev-1", Nonce: "n1", Payload: "SECRET_BODY", RiskScore: 80,
	}).Error)
	tok := issueRole(t, r, "fin@ex.com", admin.RoleFinance)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/security/reports/rep-1", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"body_redacted":true`)
	assert.NotContains(t, w.Body.String(), "SECRET_BODY")

	// security role can read body
	secTok := issueRole(t, r, "sec@ex.com", admin.RoleSecurity)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/security/reports/rep-1", nil)
	req.Header.Set("Authorization", "Bearer "+secTok)
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "SECRET_BODY")
	assert.Contains(t, w.Body.String(), `"body_redacted":false`)
}

func TestAdminRBAC_FinanceCanRefundWithActorAudit(t *testing.T) {
	r, _, _, db := setupAdminAPI(t)
	require.NoError(t, db.Create(&models.Order{
		OrderID: "ord-2", DeviceID: "dev-2", ProductID: "p1", Status: "fulfilled",
		AmountCents: 200, Currency: "CNY",
	}).Error)
	tok := issueRole(t, r, "fin@ex.com", admin.RoleFinance)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/commerce/orders/refund",
		bytes.NewReader([]byte(`{"order_id":"ord-2","reason":"chargeback"}`)))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Reason", "chargeback")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "fin@ex.com")

	var logs []models.AuditLog
	require.NoError(t, db.Where("type = ?", "commerce").Find(&logs).Error)
	require.NotEmpty(t, logs)
	assert.Contains(t, logs[0].Metadata, "fin@ex.com")
	assert.Contains(t, logs[0].Metadata, "chargeback")
}

func TestAdminRBAC_RevokeInvalidatesSession(t *testing.T) {
	r, _, store, _ := setupAdminAPI(t)
	secTok := issueRole(t, r, "sec@ex.com", admin.RoleSecurity)
	// issue support session then revoke
	supTok := issueRole(t, r, "sup@ex.com", admin.RoleSupport)
	// parse session from support by using store revoke via security
	// first list: get session via token service parse
	// revoke all for actor
	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"actor_id": "sup@ex.com", "reason": "offboarding"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/sessions/revoke", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+secTok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Reason", "offboarding")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())

	// support token no longer works
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/security/reports/x", nil)
	req.Header.Set("Authorization", "Bearer "+supTok)
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
	_ = store
}
