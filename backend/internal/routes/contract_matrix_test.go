package routes

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// contractOp is one OpenAPI operation under the AP-091 matrix.
type contractOp struct {
	operationID    string
	method         string
	path           string
	successStatuses []int
	failureStatuses []int
	needsDB        bool
	needsAuth      bool
	needsAdmin     bool
	needsOps       bool
	successBody    string
	failureBody    string
	failureNoAuth  bool
	successPath    string
	multipart      bool
}

func contractConfig() *config.Config {
	cfg := testConfig()
	cfg.FeatureFlags = config.FeatureFlags{Ranking: true, PvP: true, Social: true, Ops: true}
	cfg.CommerceEnabled = true
	cfg.AdminAPIKey = "admin-test-key-ap091"
	cfg.OpsToken = "ops-test-token-ap091"
	cfg.JWTAccessTTL = 2 * time.Hour
	return cfg
}

func openContractDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:contract_%s?mode=memory&cache=shared", strings.ReplaceAll(uuid.NewString(), "-", ""))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Device{}, &models.Animal{}, &models.AuditLog{}, &models.Inference{},
		&models.DataRequest{}, &models.SecurityReport{}, &models.Product{}, &models.Order{},
		&models.Entitlement{}, &models.ModerationReport{}, &models.Account{},
	))
	return db
}

func mintDeviceJWT(t *testing.T, cfg *config.Config, deviceID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"device_id": deviceID, "sub": deviceID,
		"iss": cfg.JWTIssuer, "aud": cfg.JWTAudience,
		"jti": uuid.NewString(), "token_version": float64(1),
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(cfg.JWTSecret))
	require.NoError(t, err)
	return s
}

var contractPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
	0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0x3f,
	0x00, 0x05, 0xfe, 0x02, 0xfe, 0xdc, 0xcc, 0x59, 0xe7, 0x00, 0x00, 0x00,
	0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func allContractOps() []contractOp {
	pubFail := []int{404, 405}
	return []contractOp{
		{operationID: "getHealth", method: "GET", path: "/health", successStatuses: []int{200}, failureStatuses: pubFail},
		{operationID: "getLivez", method: "GET", path: "/livez", successStatuses: []int{200}, failureStatuses: pubFail},
		{operationID: "getReady", method: "GET", path: "/ready", successStatuses: []int{200, 503}, failureStatuses: pubFail},
		{operationID: "getReadyz", method: "GET", path: "/readyz", successStatuses: []int{200, 503}, failureStatuses: pubFail},
		{operationID: "getMetrics", method: "GET", path: "/metrics", successStatuses: []int{404}, failureStatuses: pubFail},
		{operationID: "ping", method: "GET", path: "/api/v1/ping", successStatuses: []int{200}, failureStatuses: pubFail},
		{operationID: "getTime", method: "GET", path: "/api/v1/time", successStatuses: []int{200}, failureStatuses: pubFail},

		{operationID: "authDevice", method: "POST", path: "/api/v1/auth/device", needsDB: true, successStatuses: []int{200}, failureStatuses: []int{400}, successBody: `{}`, failureBody: `{}`},
		{operationID: "authBind", method: "POST", path: "/api/v1/auth/bind", needsDB: true, needsAuth: true, successStatuses: []int{200, 400, 401, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"provider":"mock","id_token":"x"}`},
		{operationID: "authLogin", method: "POST", path: "/api/v1/auth/login", needsDB: true, successStatuses: []int{200, 400, 401}, failureStatuses: []int{400}, successBody: `{"provider":"mock","id_token":"x"}`, failureBody: `{}`},
		{operationID: "authLogout", method: "POST", path: "/api/v1/auth/logout", needsDB: true, needsAuth: true, successStatuses: []int{200, 204, 401, 500, 503}, failureStatuses: []int{401}, failureNoAuth: true},
		{operationID: "authAccount", method: "GET", path: "/api/v1/auth/account", needsDB: true, needsAuth: true, successStatuses: []int{200, 404, 503}, failureStatuses: []int{401}, failureNoAuth: true},
		{operationID: "authListDevices", method: "GET", path: "/api/v1/auth/devices", needsDB: true, needsAuth: true, successStatuses: []int{200, 503}, failureStatuses: []int{401}, failureNoAuth: true},
		{operationID: "authRevokeDevice", method: "POST", path: "/api/v1/auth/devices/revoke", needsDB: true, needsAuth: true, successStatuses: []int{200, 400, 403, 404, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"device_id":"other"}`},

		{operationID: "getCity", method: "GET", path: "/api/v1/geo/city", needsAuth: true, successPath: "/api/v1/geo/city?lat=39.9&lng=116.4", successStatuses: []int{200, 503}, failureStatuses: []int{401}, failureNoAuth: true},
		{operationID: "getWeatherWeek", method: "GET", path: "/api/v1/weather/week", needsAuth: true, successPath: "/api/v1/weather/week?lat=39.9&lng=116.4", successStatuses: []int{200, 503}, failureStatuses: []int{401}, failureNoAuth: true},

		{operationID: "visionDetect", method: "POST", path: "/api/v1/vision/detect", needsDB: true, needsAuth: true, multipart: true, successStatuses: []int{200, 400, 422, 503}, failureStatuses: []int{401}, failureNoAuth: true},
		{operationID: "visionAnalyze", method: "POST", path: "/api/v1/vision/analyze", needsDB: true, needsAuth: true, successStatuses: []int{200, 400, 404, 422, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"detect_id":"00000000-0000-0000-0000-000000000001"}`},
		{operationID: "valueGenerate", method: "POST", path: "/api/v1/value/generate", needsDB: true, needsAuth: true, successStatuses: []int{200, 400, 404, 422, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"species":"cat","cuteness":5,"rarity_hint":3}`},

		{operationID: "syncAnimal", method: "POST", path: "/api/v1/sync/animal", needsDB: true, needsAuth: true, successStatuses: []int{200, 201, 400, 409, 422, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"uuid":"00000000-0000-4000-8000-000000000099","species":"cat","breed":"x","rarity":1,"hp":1,"atk":1,"def":1,"spd":1,"class":"Ranger","element":"Wind","latitude":0,"longitude":0,"generated_at":"2020-01-01T00:00:00Z"}`},
		{operationID: "syncAnimalsBatch", method: "POST", path: "/api/v1/sync/animals", needsDB: true, needsAuth: true, successStatuses: []int{200, 400, 422, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"items":[]}`},
		{operationID: "pullAnimals", method: "GET", path: "/api/v1/sync/animals", needsDB: true, needsAuth: true, successStatuses: []int{200, 503}, failureStatuses: []int{401}, failureNoAuth: true},

		{operationID: "putConsent", method: "POST", path: "/api/v1/privacy/consent", needsDB: true, needsAuth: true, successStatuses: []int{200, 400, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"version":"1","scopes":{"analytics":true}}`},
		{operationID: "exportData", method: "POST", path: "/api/v1/privacy/export", needsDB: true, needsAuth: true, successStatuses: []int{200, 201, 202, 503}, failureStatuses: []int{401}, failureNoAuth: true},
		{operationID: "deleteData", method: "POST", path: "/api/v1/privacy/delete", needsDB: true, needsAuth: true, successStatuses: []int{200, 202, 503}, failureStatuses: []int{401}, failureNoAuth: true},
		{operationID: "getPrivacyRequest", method: "GET", path: "/api/v1/privacy/requests/{id}", needsDB: true, needsAuth: true, successPath: "/api/v1/privacy/requests/1", successStatuses: []int{200, 404, 503}, failureStatuses: []int{401}, failureNoAuth: true},

		{operationID: "securityReport", method: "POST", path: "/api/v1/security/report", needsDB: true, needsAuth: true, successStatuses: []int{200, 400, 409, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"nonce":"n1","risk_level":"low"}`},
		{operationID: "safetyReport", method: "POST", path: "/api/v1/safety/report", needsDB: true, needsAuth: true, successStatuses: []int{200, 202, 400, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"category":"abuse","detail":"spam"}`},
		{operationID: "accountDefaults", method: "GET", path: "/api/v1/account/defaults", needsAuth: true, successStatuses: []int{200}, failureStatuses: []int{401}, failureNoAuth: true},

		{operationID: "createOrder", method: "POST", path: "/api/v1/commerce/orders", needsDB: true, needsAuth: true, successStatuses: []int{200, 201, 400, 404, 501, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"product_id":"coins_100","platform":"apple"}`},
		{operationID: "fulfillOrder", method: "POST", path: "/api/v1/commerce/orders/fulfill", needsDB: true, needsAuth: true, successStatuses: []int{200, 400, 404, 501, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"order_id":"1","platform":"apple","receipt":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`},
		{operationID: "refundOrder", method: "POST", path: "/api/v1/commerce/orders/refund", needsDB: true, needsAuth: true, successStatuses: []int{200, 403, 404, 501, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"order_id":"1"}`},
		{operationID: "getOrder", method: "GET", path: "/api/v1/commerce/orders/{id}", needsDB: true, needsAuth: true, successPath: "/api/v1/commerce/orders/1", successStatuses: []int{200, 404, 501, 503}, failureStatuses: []int{401}, failureNoAuth: true},
		{operationID: "listEntitlements", method: "GET", path: "/api/v1/commerce/entitlements", needsDB: true, needsAuth: true, successStatuses: []int{200, 501, 503}, failureStatuses: []int{401}, failureNoAuth: true},

		{operationID: "adminRefundOrder", method: "POST", path: "/api/v1/admin/commerce/orders/refund", needsDB: true, needsAdmin: true, successStatuses: []int{200, 400, 404, 503}, failureStatuses: []int{401, 403}, successBody: `{"order_id":"1"}`, failureBody: `{"order_id":"1"}`},
		{operationID: "webhookRefundOrder", method: "POST", path: "/api/v1/admin/commerce/webhooks/refund", needsDB: true, needsAdmin: true, successStatuses: []int{200, 400, 404, 503}, failureStatuses: []int{401, 403}, successBody: `{"platform":"apple","receipt":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`},
		{operationID: "listAuditLogs", method: "GET", path: "/api/v1/admin/audit/logs", needsDB: true, needsAdmin: true, successStatuses: []int{200, 503}, failureStatuses: []int{401, 403}},
		{operationID: "ackAuditLog", method: "POST", path: "/api/v1/admin/audit/logs/{id}/ack", needsDB: true, needsAdmin: true, successPath: "/api/v1/admin/audit/logs/1/ack", successStatuses: []int{200, 404, 503}, failureStatuses: []int{401, 403}},

		{operationID: "rankingDaily", method: "GET", path: "/api/v1/ranking/daily", needsAuth: true, successStatuses: []int{200, 501, 503}, failureStatuses: []int{401}, failureNoAuth: true},
		{operationID: "pvpMatch", method: "POST", path: "/api/v1/pvp/match", needsAuth: true, successStatuses: []int{200, 501, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{}`},
		{operationID: "pvpResult", method: "POST", path: "/api/v1/pvp/result", needsAuth: true, successStatuses: []int{200, 400, 501, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"match_id":"m1"}`},
		{operationID: "socialFriends", method: "GET", path: "/api/v1/social/friends", needsAuth: true, successStatuses: []int{200, 501, 503}, failureStatuses: []int{401}, failureNoAuth: true},
		{operationID: "socialShare", method: "POST", path: "/api/v1/social/share", needsAuth: true, successStatuses: []int{200, 400, 501, 503}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"animal_id":"a1"}`},
		{operationID: "opsMetricsSummary", method: "GET", path: "/api/v1/ops/metrics-summary", needsAuth: true, needsOps: true, successStatuses: []int{200, 403, 501, 503}, failureStatuses: []int{401, 403}, failureNoAuth: true},

		{operationID: "errorsReport", method: "POST", path: "/api/v1/errors/report", needsAuth: true, successStatuses: []int{200, 202}, failureStatuses: []int{400, 401}, failureNoAuth: true, successBody: `{"message":"boom","stack":"at x","release":"test"}`, failureBody: `{}`},
		{operationID: "analyticsIngest", method: "POST", path: "/api/v1/analytics/events", needsAuth: true, successStatuses: []int{200, 202, 400}, failureStatuses: []int{401}, failureNoAuth: true, successBody: `{"schema_version":1,"events":[{"name":"app_open","ts":"2020-01-01T00:00:00Z"}]}`},
		{operationID: "getGameConfig", method: "GET", path: "/api/v1/config/game", needsAuth: true, successStatuses: []int{200}, failureStatuses: []int{401}, failureNoAuth: true},
		{operationID: "putGameConfig", method: "PUT", path: "/api/v1/ops/game-config", needsAuth: true, needsOps: true, successStatuses: []int{200, 400, 403}, failureStatuses: []int{401, 403}, failureNoAuth: true, successBody: `{"version":"game-config.v1","payload":{}}`},
		{operationID: "rollbackGameConfig", method: "POST", path: "/api/v1/ops/game-config/rollback", needsAuth: true, needsOps: true, successStatuses: []int{200, 400, 403, 404, 409}, failureStatuses: []int{401, 403}, failureNoAuth: true, successBody: `{"version":"game-config.v1"}`},
	}
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func TestContractMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ops := allContractOps()
	require.Equal(t, 49, len(ops), "matrix must list all OpenAPI operationIds")

	cfg := contractConfig()
	nilRouter := NewRouter(cfg, nil)
	db := openContractDB(t)
	deviceID := "contract-device-1"
	require.NoError(t, db.Create(&models.Device{
		DeviceID: deviceID, InstallationSecretHash: strings.Repeat("a", 64), TokenVersion: 1,
	}).Error)
	dbRouter := NewRouter(cfg, db)

	seen := map[string]bool{}
	for _, op := range ops {
		op := op
		require.False(t, seen[op.operationID], "duplicate %s", op.operationID)
		seen[op.operationID] = true

		t.Run(op.operationID, func(t *testing.T) {
			r := nilRouter
			if op.needsDB {
				r = dbRouter
			}
			require.NoError(t, db.Model(&models.Device{}).Where("device_id = ?", deviceID).Update("token_version", 1).Error)
			token := mintDeviceJWT(t, cfg, deviceID)

			t.Run("success", func(t *testing.T) {
				w := doContractRequest(t, r, cfg, token, op, true)
				if !containsInt(op.successStatuses, http.StatusNotFound) {
					assert.NotEqual(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
				}
				assert.Contains(t, op.successStatuses, w.Code, "success status=%d body=%s", w.Code, w.Body.String())
			})
			t.Run("failure", func(t *testing.T) {
				w := doContractRequest(t, r, cfg, token, op, false)
				if !containsInt(op.failureStatuses, http.StatusNotFound) {
					assert.NotEqual(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
				}
				assert.Contains(t, op.failureStatuses, w.Code, "failure status=%d body=%s", w.Code, w.Body.String())
				assert.NotContains(t, []int{http.StatusOK, http.StatusCreated, http.StatusAccepted}, w.Code)
			})
		})
	}
	require.Equal(t, 49, len(seen))
}

func doContractRequest(t *testing.T, r *gin.Engine, cfg *config.Config, token string, op contractOp, success bool) *httptest.ResponseRecorder {
	t.Helper()
	method := op.method
	path := op.path
	if success && op.successPath != "" {
		path = op.successPath
	}
	if !success && !op.failureNoAuth && !op.needsAuth && !op.needsAdmin && !op.needsOps && op.failureBody == "" && method == http.MethodGet {
		method = http.MethodPut
	}
	path = strings.ReplaceAll(path, "{id}", "1")

	var body io.Reader
	contentType := ""
	if success && op.multipart {
		var buf bytes.Buffer
		b := "contractboundary"
		contentType = "multipart/form-data; boundary=" + b
		buf.WriteString("--" + b + "\r\n")
		buf.WriteString(`Content-Disposition: form-data; name="image"; filename="t.png"` + "\r\n")
		buf.WriteString("Content-Type: image/png\r\n\r\n")
		buf.Write(contractPNG)
		buf.WriteString("\r\n--" + b + "--\r\n")
		body = &buf
	} else {
		payload := op.successBody
		if success && op.operationID == "authDevice" {
			payload = fmt.Sprintf(`{"device_id":"%s"}`, "dev-"+uuid.NewString())
		}
		if !success {
			payload = op.failureBody
			if payload == "" && (method == http.MethodPost || method == http.MethodPut) {
				payload = `{}`
			}
		}
		if payload != "" {
			body = strings.NewReader(payload)
			contentType = "application/json"
		}
	}

	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if success {
		if op.needsAuth {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if op.needsAdmin {
			req.Header.Set("X-Admin-Key", cfg.AdminAPIKey)
		}
		if op.needsOps {
			req.Header.Set("X-AP-Ops-Token", cfg.OpsToken)
		}
	} else {
		if op.needsAdmin {
			req.Header.Set("X-Admin-Key", "wrong-admin-key")
		}
		if op.needsOps {
			req.Header.Set("X-AP-Ops-Token", "wrong-ops-token")
		}
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
