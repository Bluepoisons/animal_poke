package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	require.NoError(t, db.AutoMigrate(
		&models.Device{}, &models.Account{}, &models.AccountBinding{}, &models.DeviceAccount{},
		&models.Animal{}, &models.Entitlement{}, &models.Order{}, &models.Product{},
	))
	deviceRepo := repo.NewDeviceRepo(db)
	accountRepo := repo.NewAccountRepo(db, "test-pepper-secret")
	authH := NewAuthHandler(deviceRepo, "test-secret", 24*time.Hour)
	acctH := NewAccountHandler(deviceRepo, accountRepo, "test-secret", 24*time.Hour, "animal-poke", "animal-poke-client", true)

	r := gin.New()
	r.POST("/api/v1/auth/device", authH.DeviceAuth)
	r.POST("/api/v1/auth/login", acctH.Login)
	auth := r.Group("/api/v1")
	auth.Use(middleware.JWTAuthWithChecker("test-secret", "animal-poke", "animal-poke-client", deviceCheckerAdapter{deviceRepo}))
	{
		auth.POST("/auth/bind", acctH.Bind)
		auth.POST("/auth/logout", acctH.Logout)
		auth.GET("/auth/devices", acctH.ListDevices)
		auth.POST("/auth/devices/revoke", acctH.RevokeDevice)
		auth.GET("/auth/account", acctH.GetAccount)
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
	))
	deviceRepo := repo.NewDeviceRepo(db)
	accountRepo := repo.NewAccountRepo(db, "test-pepper-secret")
	authH := NewAuthHandler(deviceRepo, "test-secret", 24*time.Hour)
	acctH := NewAccountHandler(deviceRepo, accountRepo, "test-secret", 24*time.Hour, "animal-poke", "animal-poke-client", allowMock)

	r := gin.New()
	r.POST("/api/v1/auth/device", authH.DeviceAuth)
	r.POST("/api/v1/auth/login", acctH.Login)
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
