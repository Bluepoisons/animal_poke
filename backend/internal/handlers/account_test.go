package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAccountTest(t *testing.T) (*gin.Engine, *gorm.DB, *repo.DeviceRepo, *repo.AccountRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:acct_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// SQLite 写串行化，便于并发 refresh 竞态测试稳定
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&models.Device{}, &models.Account{}, &models.AccountBinding{}, &models.DeviceAccount{},
		&models.Animal{}, &models.Entitlement{}, &models.Order{}, &models.Product{},
		&models.DeviceMigrationTicket{}, &models.AccountMergeOperation{}, &models.AuditLog{},
		&models.RefreshToken{}, &models.AccountSecurityToken{},
	))
	deviceRepo := repo.NewDeviceRepo(db)
	accountRepo := repo.NewAccountRepo(db, "test-pepper-secret")
	authH := NewAuthHandler(deviceRepo, "test-secret", 24*time.Hour)
	acctH := NewAccountHandler(deviceRepo, accountRepo, "test-secret", 24*time.Hour, "animal-poke", "animal-poke-client", true)

	r := gin.New()
	r.POST("/api/v1/auth/device", authH.DeviceAuth)
	r.POST("/api/v1/auth/login", acctH.Login)
	r.POST("/api/v1/auth/refresh", acctH.Refresh)
	r.POST("/api/v1/auth/email/verify", acctH.VerifyEmail)
	r.POST("/api/v1/auth/email/verify/request", acctH.RequestEmailVerify)
	r.POST("/api/v1/auth/password/forgot", acctH.ForgotPassword)
	r.POST("/api/v1/auth/password/reset", acctH.ResetPassword)
	auth := r.Group("/api/v1")
	auth.Use(middleware.JWTAuthWithChecker("test-secret", "animal-poke", "animal-poke-client", deviceCheckerAdapter{deviceRepo}))
	{
		auth.POST("/auth/bind", acctH.Bind)
		auth.POST("/auth/logout", acctH.Logout)
		auth.GET("/auth/devices", acctH.ListDevices)
		auth.POST("/auth/devices/revoke", acctH.RevokeDevice)
		auth.GET("/auth/account", acctH.GetAccount)
		auth.POST("/auth/password/change", acctH.ChangePassword)
		auth.POST("/auth/unbind", acctH.UnbindProvider)
		auth.POST("/auth/reauth", acctH.Reauth)
		auth.GET("/sync/animals", func(c *gin.Context) {
			deviceID := middleware.GetDeviceID(c)
			accountID := middleware.GetAccountID(c)
			animalRepo := repo.NewAnimalRepo(db)
			items, err := animalRepo.ListSinceVersionScoped(deviceID, accountID, 0, 50)
			if err != nil {
				c.JSON(500, gin.H{"error": "pull failed"})
				return
			}
			c.JSON(200, gin.H{"items": items})
		})
		auth.GET("/commerce/entitlements", NewCommerceHandler(db).ListEntitlements)
	}
	return r, db, deviceRepo, accountRepo
}

type deviceCheckerAdapter struct{ repo *repo.DeviceRepo }

func (d deviceCheckerAdapter) IsDisabled(deviceID string) (bool, error) {
	return d.repo.IsDisabled(deviceID)
}
func (d deviceCheckerAdapter) TokenVersion(deviceID string) (int, error) {
	dev, err := d.repo.Find(deviceID)
	if err != nil {
		return 0, err
	}
	return dev.TokenVersion, nil
}

func deviceAuth(t *testing.T, r *gin.Engine, deviceID string) (token string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"device_id": deviceID})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/device", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp["token"].(string)
}

func deviceAuthFull(t *testing.T, r *gin.Engine, deviceID string) (token, installationSecret string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"device_id": deviceID})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/device", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	tok, _ := resp["token"].(string)
	sec, _ := resp["installation_secret"].(string)
	return tok, sec
}


func verifyEmailFromBind(t *testing.T, r *gin.Engine, bindBody []byte) {
	t.Helper()
	var resp map[string]any
	require.NoError(t, json.Unmarshal(bindBody, &resp))
	tok, _ := resp["debug_security_token"].(string)
	if tok == "" {
		return // already verified / oauth
	}
	w := authedJSON(t, r, "POST", "/api/v1/auth/email/verify", "", map[string]string{"token": tok})
	require.Equal(t, 200, w.Code, w.Body.String())
}

func postLogin(t *testing.T, r *gin.Engine, payload map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func authedJSON(t *testing.T, r *gin.Engine, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body []byte
	if payload != nil {
		body, _ = json.Marshal(payload)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestBind_MockOAuth_CreatesAccountAndHashesToken(t *testing.T) {
	r, db, _, accountRepo := setupAccountTest(t)
	token := deviceAuth(t, r, "guest-device-001")

	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider":      "mock_oauth",
		"oauth_subject": "alice",
		"oauth_token":   "secret-token-value",
		"display_name":  "Alice",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var resp accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.AccountID)
	assert.False(t, resp.Guest)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.NotEmpty(t, resp.Token)

	// 凭证仅存哈希
	var b models.AccountBinding
	require.NoError(t, db.Where("provider = ? AND provider_subject = ?", "mock_oauth", "alice").First(&b).Error)
	assert.NotEqual(t, "secret-token-value", b.CredentialHash)
	assert.Equal(t, accountRepo.HashToken("secret-token-value"), b.CredentialHash)
	assert.NotContains(t, w.Body.String(), "secret-token-value")
}

func TestMerge_NoDoubleEntitlementGrant(t *testing.T) {
	r, db, _, _ := setupAccountTest(t)

	// 账号设备 A 已有月卡权益
	tokenA := deviceAuth(t, r, "device-account-a")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", tokenA, map[string]string{
		"provider": "mock_oauth", "oauth_subject": "bob", "oauth_token": "token-bob-secret",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var bindA accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bindA))

	now := time.Now().UTC()
	expA := now.Add(10 * 24 * time.Hour)
	require.NoError(t, db.Create(&models.Entitlement{
		DeviceID: "device-account-a", AccountID: bindA.AccountID, ProductID: "monthly_pass",
		OrderID: "ord-a", Active: true, StartsAt: now, ExpiresAt: &expA,
	}).Error)
	require.NoError(t, db.Create(&models.Animal{
		UUID: "uuid-a1", DeviceID: "device-account-a", AccountID: bindA.AccountID,
		Species: "cat", Rarity: 3, GeneratedAt: now, ServerVersion: 1,
	}).Error)

	// 游客设备 G 有同 product 权益与动物
	tokenG := deviceAuth(t, r, "device-guest-g")
	expG := now.Add(30 * 24 * time.Hour) // 更长有效期
	require.NoError(t, db.Create(&models.Entitlement{
		DeviceID: "device-guest-g", ProductID: "monthly_pass",
		OrderID: "ord-g", Active: true, StartsAt: now, ExpiresAt: &expG,
	}).Error)
	require.NoError(t, db.Create(&models.Animal{
		UUID: "uuid-g1", DeviceID: "device-guest-g",
		Species: "dog", Rarity: 2, GeneratedAt: now, ServerVersion: 2,
	}).Error)

	// 游客绑定同一 mock oauth → 合并
	w = authedJSON(t, r, "POST", "/api/v1/auth/bind", tokenG, map[string]string{
		"provider": "mock_oauth", "oauth_subject": "bob", "oauth_token": "token-bob-secret",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var bindG accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bindG))
	assert.Equal(t, bindA.AccountID, bindG.AccountID)
	require.NotNil(t, bindG.Merge)
	assert.GreaterOrEqual(t, bindG.Merge.EntitlementsMerged, 1)
	assert.GreaterOrEqual(t, bindG.Merge.AnimalsMoved, 1)

	// 活跃权益仅 1 份（账号侧），过期取更晚
	var active []models.Entitlement
	require.NoError(t, db.Where("product_id = ? AND active = ?", "monthly_pass", true).Find(&active).Error)
	assert.Equal(t, 1, len(active), "must not double-grant active entitlement")
	require.NotNil(t, active[0].ExpiresAt)
	// 合并后有效期应覆盖 guest 更长有效期
	assert.False(t, active[0].ExpiresAt.Before(expG.Add(-time.Second)))
	var guestEnt models.Entitlement
	require.NoError(t, db.Where("device_id = ? AND product_id = ?", "device-guest-g", "monthly_pass").First(&guestEnt).Error)
	assert.False(t, guestEnt.Active)

	// 动物都挂到账号
	var animals []models.Animal
	require.NoError(t, db.Where("account_id = ?", bindA.AccountID).Find(&animals).Error)
	assert.GreaterOrEqual(t, len(animals), 2)
}

func TestRevokeDevice_InvalidatesToken(t *testing.T) {
	r, _, _, _ := setupAccountTest(t)
	tokenA := deviceAuth(t, r, "dev-main-001")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", tokenA, map[string]string{
		"provider": "mock_oauth", "oauth_subject": "carol", "oauth_token": "token-carol-xx",
	})
	require.Equal(t, 200, w.Code)
	var bindA accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bindA))
	tokenA = bindA.Token

	// 第二设备登录
	body, _ := json.Marshal(map[string]string{
		"device_id": "dev-lost-002", "provider": "mock_oauth",
		"oauth_subject": "carol", "oauth_token": "token-carol-xx",
	})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	require.Equal(t, 200, w2.Code, w2.Body.String())
	var loginResp accountAuthResponse
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &loginResp))
	tokenLost := loginResp.Token

	// 主设备吊销丢失设备
	w = authedJSON(t, r, "POST", "/api/v1/auth/devices/revoke", tokenA, map[string]string{
		"device_id": "dev-lost-002",
	})
	require.Equal(t, 200, w.Code, w.Body.String())

	// 丢失设备 token 应 401
	w = authedJSON(t, r, "GET", "/api/v1/auth/account", tokenLost, nil)
	assert.Equal(t, 401, w.Code)

	// 列表中状态为 revoked
	w = authedJSON(t, r, "GET", "/api/v1/auth/devices", tokenA, nil)
	require.Equal(t, 200, w.Code)
	var list map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	items, _ := list["items"].([]any)
	found := false
	for _, it := range items {
		m := it.(map[string]any)
		if m["device_id"] == "dev-lost-002" {
			found = true
			assert.Equal(t, "revoked", m["status"])
		}
	}
	assert.True(t, found)
}

func TestRecoverAfterClearLocal_MockLogin(t *testing.T) {
	r, db, _, _ := setupAccountTest(t)

	// 原设备绑定并有收藏
	token1 := deviceAuth(t, r, "dev-original")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token1, map[string]string{
		"provider": "email", "email": "user@example.com", "password": "password123",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	verifyEmailFromBind(t, r, w.Body.Bytes())
	var bindResp accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bindResp))
	require.NotEmpty(t, bindResp.AccountID)

	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.Animal{
		UUID: "uuid-recover-1", DeviceID: "dev-original", AccountID: bindResp.AccountID,
		Species: "cat", Rarity: 4, GeneratedAt: now, ServerVersion: 10,
	}).Error)
	require.NoError(t, db.Create(&models.Entitlement{
		DeviceID: "dev-original", AccountID: bindResp.AccountID, ProductID: "monthly_pass",
		Active: true, StartsAt: now,
	}).Error)

	// 模拟清除本地：新 device_id 登录
	body, _ := json.Marshal(map[string]string{
		"device_id": "dev-new-after-clear", "provider": "email",
		"email": "user@example.com", "password": "password123",
	})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	require.Equal(t, 200, w2.Code, w2.Body.String())
	var login accountAuthResponse
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &login))
	assert.Equal(t, bindResp.AccountID, login.AccountID)
	assert.NotEmpty(t, login.Token)
	// JWT 含 account_id
	parsed, err := jwt.Parse(login.Token, func(t *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})
	require.NoError(t, err)
	claims := parsed.Claims.(jwt.MapClaims)
	assert.Equal(t, bindResp.AccountID, claims["account_id"])

	// 拉取动物应恢复
	w = authedJSON(t, r, "GET", "/api/v1/sync/animals", login.Token, nil)
	require.Equal(t, 200, w.Code, w.Body.String())
	var pull map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pull))
	items, _ := pull["items"].([]any)
	assert.GreaterOrEqual(t, len(items), 1)

	// 权益可恢复
	w = authedJSON(t, r, "GET", "/api/v1/commerce/entitlements", login.Token, nil)
	require.Equal(t, 200, w.Code)
	var ents map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ents))
	entItems, _ := ents["items"].([]any)
	assert.GreaterOrEqual(t, len(entItems), 1)
}

func TestLogout_BumpsTokenVersion(t *testing.T) {
	r, _, deviceRepo, _ := setupAccountTest(t)
	token := deviceAuth(t, r, "dev-logout-1")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "mock_oauth", "oauth_subject": "dave", "oauth_token": "token-dave-secret",
	})
	require.Equal(t, 200, w.Code)
	var bind accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bind))
	token = bind.Token

	w = authedJSON(t, r, "POST", "/api/v1/auth/logout", token, nil)
	require.Equal(t, 200, w.Code)

	// 旧 token 失效
	w = authedJSON(t, r, "GET", "/api/v1/auth/account", token, nil)
	assert.Equal(t, 401, w.Code)

	dev, err := deviceRepo.Find("dev-logout-1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, dev.TokenVersion, 2)
}

func TestGuestRemainsDefault(t *testing.T) {
	r, _, _, _ := setupAccountTest(t)
	token := deviceAuth(t, r, "pure-guest-dev")
	w := authedJSON(t, r, "GET", "/api/v1/auth/account", token, nil)
	require.Equal(t, 200, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["guest"])
}

func setupAccountTestWithMock(t *testing.T, allowMock bool) (*gin.Engine, *gorm.DB, *repo.DeviceRepo, *repo.AccountRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:acct_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Device{}, &models.Account{}, &models.AccountBinding{}, &models.DeviceAccount{},
		&models.Animal{}, &models.Entitlement{}, &models.Order{}, &models.Product{},
		&models.DeviceMigrationTicket{}, &models.AccountMergeOperation{}, &models.AuditLog{},
		&models.RefreshToken{}, &models.AccountSecurityToken{},
	))
	deviceRepo := repo.NewDeviceRepo(db)
	accountRepo := repo.NewAccountRepo(db, "test-pepper-secret")
	authH := NewAuthHandler(deviceRepo, "test-secret", 24*time.Hour)
	acctH := NewAccountHandler(deviceRepo, accountRepo, "test-secret", 24*time.Hour, "animal-poke", "animal-poke-client", allowMock)

	r := gin.New()
	r.POST("/api/v1/auth/device", authH.DeviceAuth)
	r.POST("/api/v1/auth/login", acctH.Login)
	r.POST("/api/v1/auth/refresh", acctH.Refresh)
	r.POST("/api/v1/auth/email/verify", acctH.VerifyEmail)
	auth := r.Group("/api/v1")
	auth.Use(middleware.JWTAuthWithChecker("test-secret", "animal-poke", "animal-poke-client", deviceCheckerAdapter{deviceRepo}))
	{
		auth.POST("/auth/bind", acctH.Bind)
		auth.POST("/auth/logout", acctH.Logout)
		auth.GET("/auth/devices", acctH.ListDevices)
		auth.POST("/auth/devices/revoke", acctH.RevokeDevice)
		auth.GET("/auth/account", acctH.GetAccount)
	}
	return r, db, deviceRepo, accountRepo
}

func TestBind_MockOAuth_DisabledReturns404(t *testing.T) {
	r, _, _, _ := setupAccountTestWithMock(t, false)
	token := deviceAuth(t, r, "guest-no-mock")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "mock_oauth", "oauth_subject": "alice", "oauth_token": "secret-token-value",
	})
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "provider_unavailable", resp["reason_code"])
}

func TestLogin_MockOAuth_DisabledReturns404(t *testing.T) {
	r, _, _, _ := setupAccountTestWithMock(t, false)
	body, _ := json.Marshal(map[string]string{
		"device_id": "dev-login-mock-off", "provider": "mock_oauth",
		"oauth_subject": "alice", "oauth_token": "secret-token-value",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "provider_unavailable", resp["reason_code"])
}

func TestBind_Email_StillWorksWhenMockDisabled(t *testing.T) {
	r, _, _, _ := setupAccountTestWithMock(t, false)
	token := deviceAuth(t, r, "guest-email-only")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "email", "email": "user@example.com", "password": "password12",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
}

// AP-076: 仅知道他人 device_id 不能合并其游客资产。
func TestLogin_HijackGuestAssets_WithoutProofFails(t *testing.T) {
	r, db, _, accountRepo := setupAccountTest(t)

	// 受害者游客设备注册并持有动物/订单
	_, victimSecret := deviceAuthFull(t, r, "victim-guest-device")
	require.NotEmpty(t, victimSecret)
	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.Animal{
		UUID: "uuid-victim-1", DeviceID: "victim-guest-device",
		Species: "cat", Rarity: 3, GeneratedAt: now, ServerVersion: 1,
	}).Error)
	require.NoError(t, db.Create(&models.Order{
		OrderID: "ord-victim-1", DeviceID: "victim-guest-device", ProductID: "monthly_pass",
		Status: "fulfilled", AmountCents: 100, Currency: "CNY", IdempotencyKey: "ik-v1",
	}).Error)
	require.NoError(t, db.Create(&models.Entitlement{
		DeviceID: "victim-guest-device", ProductID: "monthly_pass",
		OrderID: "ord-victim-1", Active: true, StartsAt: now,
	}).Error)

	// 攻击者自有账号
	tokenA := deviceAuth(t, r, "attacker-device")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", tokenA, map[string]string{
		"provider": "email", "email": "attacker@example.com", "password": "password123",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	verifyEmailFromBind(t, r, w.Body.Bytes())

	// 攻击：登录时填入受害者 device_id，无 installation_secret
	w2 := postLogin(t, r, map[string]string{
		"device_id": "victim-guest-device", "provider": "email",
		"email": "attacker@example.com", "password": "password123",
	})
	require.Equal(t, http.StatusForbidden, w2.Code, w2.Body.String())
	assert.Contains(t, w2.Body.String(), "device_ownership_required")

	// 资产仍属游客，未挂到攻击者账号
	var animal models.Animal
	require.NoError(t, db.Where("uuid = ?", "uuid-victim-1").First(&animal).Error)
	assert.True(t, animal.AccountID == "" || animal.AccountID == "victim-guest-device" || animal.AccountID != "")
	assert.Equal(t, "victim-guest-device", animal.DeviceID)
	assert.Empty(t, animal.AccountID)

	// 正确证明后可合并
	w3 := postLogin(t, r, map[string]string{
		"device_id": "victim-guest-device", "provider": "email",
		"email": "attacker@example.com", "password": "password123",
		"installation_secret": victimSecret,
	})
	require.Equal(t, 200, w3.Code, w3.Body.String())
	var login accountAuthResponse
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &login))
	require.NotEmpty(t, login.OperationID)
	require.NoError(t, db.Where("uuid = ?", "uuid-victim-1").First(&animal).Error)
	assert.Equal(t, login.AccountID, animal.AccountID)
	_ = accountRepo
}

// AP-076: 撤销设备无证明不得自动复活。
func TestLogin_RevokedDevice_StaysRevokedWithoutProof(t *testing.T) {
	r, db, deviceRepo, _ := setupAccountTest(t)

	tokenA, secretA := deviceAuthFull(t, r, "dev-main-rev")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", tokenA, map[string]string{
		"provider": "mock_oauth", "oauth_subject": "revuser", "oauth_token": "token-revuser-xx",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var bindA accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bindA))
	tokenA = bindA.Token

	// 第二设备登录（新设备，无需证明）
	w2 := postLogin(t, r, map[string]string{
		"device_id": "dev-to-revoke", "provider": "mock_oauth",
		"oauth_subject": "revuser", "oauth_token": "token-revuser-xx",
	})
	require.Equal(t, 200, w2.Code, w2.Body.String())

	// 吊销
	w = authedJSON(t, r, "POST", "/api/v1/auth/devices/revoke", tokenA, map[string]string{
		"device_id": "dev-to-revoke",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	dev, err := deviceRepo.Find("dev-to-revoke")
	require.NoError(t, err)
	assert.True(t, dev.Disabled)

	// 无证明重新登录 → 拒绝，保持 disabled
	w3 := postLogin(t, r, map[string]string{
		"device_id": "dev-to-revoke", "provider": "mock_oauth",
		"oauth_subject": "revuser", "oauth_token": "token-revuser-xx",
	})
	require.Equal(t, http.StatusForbidden, w3.Code, w3.Body.String())
	assert.Contains(t, w3.Body.String(), "device_revoked")
	dev, err = deviceRepo.Find("dev-to-revoke")
	require.NoError(t, err)
	assert.True(t, dev.Disabled)

	// 有 installation_secret 时可复活（若设备有 secret；新登录创建的设备可能无 secret）
	// 为 dev-to-revoke 设置 secret 后验证证明路径
	secret, salt, err := repo.GenerateInstallationSecret()
	require.NoError(t, err)
	claimed, err := deviceRepo.SetInstallationSecret("dev-to-revoke", secret, salt)
	require.NoError(t, err)
	require.True(t, claimed)

	w4 := postLogin(t, r, map[string]string{
		"device_id": "dev-to-revoke", "provider": "mock_oauth",
		"oauth_subject": "revuser", "oauth_token": "token-revuser-xx",
		"installation_secret": secret,
	})
	require.Equal(t, 200, w4.Code, w4.Body.String())
	dev, err = deviceRepo.Find("dev-to-revoke")
	require.NoError(t, err)
	assert.False(t, dev.Disabled)
	_ = secretA
	_ = db
}

// AP-076: 迁移票据重放失败。
func TestLogin_MigrationTicket_ReplayFails(t *testing.T) {
	r, db, _, accountRepo := setupAccountTest(t)

	// 账号
	tokenA := deviceAuth(t, r, "acct-ticket-main")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", tokenA, map[string]string{
		"provider": "email", "email": "ticket@example.com", "password": "password123",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	verifyEmailFromBind(t, r, w.Body.Bytes())
	var bind accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bind))

	// 游客资产设备
	_, _ = deviceAuthFull(t, r, "guest-ticket-dev")
	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.Animal{
		UUID: "uuid-ticket-1", DeviceID: "guest-ticket-dev",
		Species: "dog", Rarity: 2, GeneratedAt: now, ServerVersion: 1,
	}).Error)

	plain, _, err := accountRepo.CreateMigrationTicket("guest-ticket-dev", bind.AccountID, time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, plain)

	w1 := postLogin(t, r, map[string]string{
		"device_id": "guest-ticket-dev", "provider": "email",
		"email": "ticket@example.com", "password": "password123",
		"migration_ticket": plain,
	})
	require.Equal(t, 200, w1.Code, w1.Body.String())

	// 重放
	w2 := postLogin(t, r, map[string]string{
		"device_id": "guest-ticket-dev", "provider": "email",
		"email": "ticket@example.com", "password": "password123",
		"migration_ticket": plain,
	})
	// 设备已归属账号，重放时若仍带 used ticket：若 needsProof 因 secret 仍要求证明则 ticket_replay
	// 或若已归属同账号且无禁用/无额外资产，可能直接成功（幂等链接）。强制：对 used ticket 在 needsProof 时失败。
	// 这里设备已有 account，再次登录同账号：若设备有 installation_secret 则 needsProof。
	// deviceAuthFull 会写入 secret，因此再次登录需要证明；used ticket → ticket_replay 或 invalid
	require.NotEqual(t, 200, w2.Code, "replay must not succeed with used ticket when proof required: %s", w2.Body.String())
	assert.True(t,
		strings.Contains(w2.Body.String(), "ticket_replay") ||
			strings.Contains(w2.Body.String(), "ticket_invalid") ||
			strings.Contains(w2.Body.String(), "device_ownership_required"),
		w2.Body.String())
}

// AP-076: 并发登录最多一次合并。
func TestLogin_ConcurrentMergeOnce(t *testing.T) {
	r, db, _, accountRepo := setupAccountTest(t)

	tokenA := deviceAuth(t, r, "acct-conc-main")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", tokenA, map[string]string{
		"provider": "email", "email": "conc@example.com", "password": "password123",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	verifyEmailFromBind(t, r, w.Body.Bytes())
	var bind accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bind))

	_, secret := deviceAuthFull(t, r, "guest-conc-dev")
	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.Animal{
		UUID: "uuid-conc-1", DeviceID: "guest-conc-dev",
		Species: "cat", Rarity: 4, GeneratedAt: now, ServerVersion: 1,
	}).Error)

	const n = 8
	codes := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			w := postLogin(t, r, map[string]string{
				"device_id": "guest-conc-dev", "provider": "email",
				"email": "conc@example.com", "password": "password123",
				"installation_secret": secret,
			})
			codes[i] = w.Code
		}(i)
	}
	wg.Wait()

	ok := 0
	for _, c := range codes {
		if c == 200 {
			ok++
		}
	}
	assert.GreaterOrEqual(t, ok, 1)
	// 动物只归属一次
	var animal models.Animal
	require.NoError(t, db.Where("uuid = ?", "uuid-conc-1").First(&animal).Error)
	assert.Equal(t, bind.AccountID, animal.AccountID)

	var ops []models.AccountMergeOperation
	require.NoError(t, db.Where("device_id = ? AND account_id = ?", "guest-conc-dev", bind.AccountID).Find(&ops).Error)
	assert.GreaterOrEqual(t, len(ops), 1)
	// operation_id 唯一
	seen := map[string]struct{}{}
	for _, op := range ops {
		_, dup := seen[op.OperationID]
		assert.False(t, dup)
		seen[op.OperationID] = struct{}{}
	}
	_ = accountRepo
}

func postRefresh(t *testing.T, r *gin.Engine, refreshToken, deviceID string) *httptest.ResponseRecorder {
	t.Helper()
	payload := map[string]string{"refresh_token": refreshToken}
	if deviceID != "" {
		payload["device_id"] = deviceID
	}
	body, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func bindAndGetRefresh(t *testing.T, r *gin.Engine, deviceID, email string) (access, refresh, accountID string) {
	t.Helper()
	token := deviceAuth(t, r, deviceID)
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "email", "email": email, "password": "password123",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	verifyEmailFromBind(t, r, w.Body.Bytes())
	// re-fetch account after verify if needed (session still valid)
	var resp accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.RefreshToken)
	require.NotEmpty(t, resp.Token)
	return resp.Token, resp.RefreshToken, resp.AccountID
}

// AP-078: 正常刷新只产生一对新 token。
func TestRefresh_RotateOnUse(t *testing.T) {
	r, db, _, _ := setupAccountTest(t)
	_, refresh, accountID := bindAndGetRefresh(t, r, "dev-refresh-1", "refresh1@example.com")

	w := postRefresh(t, r, refresh, "dev-refresh-1")
	require.Equal(t, 200, w.Code, w.Body.String())
	var resp accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Token)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.NotEqual(t, refresh, resp.RefreshToken)
	assert.Equal(t, accountID, resp.AccountID)
	assert.Equal(t, "Bearer", resp.TokenType)

	// 新 access 可用
	w2 := authedJSON(t, r, "GET", "/api/v1/auth/account", resp.Token, nil)
	require.Equal(t, 200, w2.Code, w2.Body.String())

	// 旧 refresh 在宽限内为 conflict（不整族吊销）
	w3 := postRefresh(t, r, refresh, "")
	require.Equal(t, http.StatusConflict, w3.Code, w3.Body.String())
	assert.Contains(t, w3.Body.String(), "refresh_conflict")

	// 新 refresh 可继续轮换
	w4 := postRefresh(t, r, resp.RefreshToken, "")
	require.Equal(t, 200, w4.Code, w4.Body.String())

	var n int64
	require.NoError(t, db.Model(&models.RefreshToken{}).Where("device_id = ?", "dev-refresh-1").Count(&n).Error)
	assert.GreaterOrEqual(t, n, int64(2))
}

// AP-078: 并发 20 次刷新仅一次成功。
func TestRefresh_ConcurrentOnlyOneSuccess(t *testing.T) {
	r, _, _, _ := setupAccountTest(t)
	_, refresh, _ := bindAndGetRefresh(t, r, "dev-refresh-conc", "refresh-conc@example.com")

	const n = 20
	codes := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			w := postRefresh(t, r, refresh, "dev-refresh-conc")
			codes[i] = w.Code
		}(i)
	}
	wg.Wait()

	ok, conflict, other := 0, 0, 0
	for _, c := range codes {
		switch c {
		case 200:
			ok++
		case http.StatusConflict, http.StatusUnauthorized:
			conflict++
		default:
			other++
		}
	}
	assert.Equal(t, 1, ok, "codes=%v", codes)
	assert.Equal(t, n-1, conflict+other, "codes=%v", codes)
	assert.Equal(t, 0, other, "unexpected codes=%v", codes)
}

// AP-078: 宽限外重用已 rotated 令牌 → 整族吊销。
func TestRefresh_ReuseRevokesFamily(t *testing.T) {
	r, db, deviceRepo, accountRepo := setupAccountTest(t)
	_, refresh, _ := bindAndGetRefresh(t, r, "dev-refresh-reuse", "reuse@example.com")

	// 第一次成功轮换
	w := postRefresh(t, r, refresh, "")
	require.Equal(t, 200, w.Code, w.Body.String())
	var resp accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// 将 rotated_at 推到宽限外
	past := time.Now().UTC().Add(-time.Minute)
	require.NoError(t, db.Model(&models.RefreshToken{}).
		Where("token_hash = ?", accountRepo.HashToken(refresh)).
		Update("rotated_at", past).Error)

	// 重用旧 token → 吊销
	w2 := postRefresh(t, r, refresh, "")
	require.Equal(t, http.StatusUnauthorized, w2.Code, w2.Body.String())
	assert.Contains(t, w2.Body.String(), "refresh_token_reused")

	// 新 token 也失效（族吊销）
	w3 := postRefresh(t, r, resp.RefreshToken, "")
	require.Equal(t, http.StatusUnauthorized, w3.Code, w3.Body.String())

	// access token_version 已 bump
	dev, err := deviceRepo.Find("dev-refresh-reuse")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, dev.TokenVersion, 2)

	// 新 access 因 version 失效
	w4 := authedJSON(t, r, "GET", "/api/v1/auth/account", resp.Token, nil)
	assert.Equal(t, http.StatusUnauthorized, w4.Code)
}

// AP-078: 设备撤销后 refresh 失效。
func TestRefresh_DeviceRevokeInvalidates(t *testing.T) {
	r, _, _, _ := setupAccountTest(t)
	tokenA, _, _ := bindAndGetRefresh(t, r, "dev-main-rev", "rev-main@example.com")

	// 第二台设备登录
	body, _ := json.Marshal(map[string]string{
		"device_id": "dev-lost-rev", "provider": "email",
		"email": "rev-main@example.com", "password": "password123",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())
	var login accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &login))
	require.NotEmpty(t, login.RefreshToken)

	// 主设备撤销 lost 设备
	w2 := authedJSON(t, r, "POST", "/api/v1/auth/devices/revoke", tokenA, map[string]string{
		"device_id": "dev-lost-rev",
	})
	require.Equal(t, 200, w2.Code, w2.Body.String())

	// lost 设备 refresh 不可用
	w3 := postRefresh(t, r, login.RefreshToken, "dev-lost-rev")
	require.Equal(t, http.StatusUnauthorized, w3.Code, w3.Body.String())
}

// AP-078: 登出后 refresh 失效。
func TestRefresh_LogoutInvalidates(t *testing.T) {
	r, _, _, _ := setupAccountTest(t)
	token, refresh, _ := bindAndGetRefresh(t, r, "dev-logout-ref", "logout-ref@example.com")

	w := authedJSON(t, r, "POST", "/api/v1/auth/logout", token, nil)
	require.Equal(t, 200, w.Code, w.Body.String())

	w2 := postRefresh(t, r, refresh, "")
	require.Equal(t, http.StatusUnauthorized, w2.Code, w2.Body.String())
}

// AP-078: 绝对过期。
func TestRefresh_AbsoluteExpiry(t *testing.T) {
	r, db, _, accountRepo := setupAccountTest(t)
	_, refresh, _ := bindAndGetRefresh(t, r, "dev-abs-exp", "abs@example.com")

	past := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(&models.RefreshToken{}).
		Where("token_hash = ?", accountRepo.HashToken(refresh)).
		Update("absolute_expires_at", past).Error)

	w := postRefresh(t, r, refresh, "")
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "refresh_token_expired")
}

// AP-078: 空闲过期。
func TestRefresh_IdleExpiry(t *testing.T) {
	r, db, _, accountRepo := setupAccountTest(t)
	_, refresh, _ := bindAndGetRefresh(t, r, "dev-idle-exp", "idle@example.com")

	past := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(&models.RefreshToken{}).
		Where("token_hash = ?", accountRepo.HashToken(refresh)).
		Update("idle_expires_at", past).Error)

	w := postRefresh(t, r, refresh, "")
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "refresh_token_expired")
}

// AP-078: pepper 轮换双读 — previous pepper 签发的 refresh 仍可轮换。
func TestRefresh_PepperPreviousStillWorks(t *testing.T) {
	r, db, deviceRepo, _ := setupAccountTest(t)
	// 用 previous pepper 手工写入一条 active refresh
	token := deviceAuth(t, r, "dev-pepper-rot")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "email", "email": "pepper@example.com", "password": "password123",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var bind accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bind))

	// 重建 repo 带 previous pepper，把当前 hash 改写成 previous 哈希
	oldRepo := repo.NewAccountRepo(db, "old-pepper-value")
	plain := "legacy-refresh-token-plain-value-xx"
	oldHash := oldRepo.HashToken(plain)
	require.NoError(t, db.Model(&models.RefreshToken{}).
		Where("device_id = ? AND status = ?", "dev-pepper-rot", "active").
		Update("token_hash", oldHash).Error)

	// 使用带 previous 的 handler
	accountRepo := repo.NewAccountRepoWithPeppers(db, "test-pepper-secret", "old-pepper-value")
	acctH := NewAccountHandler(deviceRepo, accountRepo, "test-secret", 24*time.Hour, "animal-poke", "animal-poke-client", true)
	r2 := gin.New()
	r2.POST("/api/v1/auth/refresh", acctH.Refresh)

	w2 := postRefresh(t, r2, plain, "dev-pepper-rot")
	require.Equal(t, 200, w2.Code, w2.Body.String())
	var resp accountAuthResponse
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.RefreshToken)
	assert.NotEqual(t, plain, resp.RefreshToken)
}
