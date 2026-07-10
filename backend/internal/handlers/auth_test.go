package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthHandlerTest(t *testing.T) (*gin.Engine, *AuthHandler, *repo.DeviceRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	// 迁移完整 Device 模型
	err = db.AutoMigrate(&models.Device{})
	require.NoError(t, err)

	deviceRepo := repo.NewDeviceRepo(db)
	handler := NewAuthHandler(deviceRepo, "test-secret", 24*time.Hour)

	r := gin.New()
	r.POST("/api/v1/auth/device", handler.DeviceAuth)
	return r, handler, deviceRepo
}

func postAuth(r *gin.Engine, body map[string]string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/device", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestDeviceAuth_Success(t *testing.T) {
	r, _, _ := setupAuthHandlerTest(t)

	w := postAuth(r, map[string]string{"device_id": "test-device-001"})
	assert.Equal(t, 200, w.Code)

	var resp authResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Token)
	assert.NotEmpty(t, resp.ExpiresAt)
	assert.NotEmpty(t, resp.InstallationSecret, "首次注册应返回 installation_secret")
	assert.GreaterOrEqual(t, len(resp.InstallationSecret), 64)
}

func TestDeviceAuth_MissingDeviceID(t *testing.T) {
	r, _, _ := setupAuthHandlerTest(t)

	w := postAuth(r, map[string]string{})
	assert.Equal(t, 400, w.Code)
}

func TestDeviceAuth_EmptyDeviceID(t *testing.T) {
	r, _, _ := setupAuthHandlerTest(t)

	w := postAuth(r, map[string]string{"device_id": ""})
	assert.Equal(t, 400, w.Code)
}

func TestDeviceAuth_SecondCallRequiresSecret(t *testing.T) {
	r, _, _ := setupAuthHandlerTest(t)

	// 首次注册
	w1 := postAuth(r, map[string]string{"device_id": "same-device-01"})
	require.Equal(t, 200, w1.Code)
	var first authResponse
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &first))
	require.NotEmpty(t, first.InstallationSecret)

	// 仅凭 device_id 冒领 → 401
	w2 := postAuth(r, map[string]string{"device_id": "same-device-01"})
	assert.Equal(t, 401, w2.Code)

	// 错误 secret → 401
	w3 := postAuth(r, map[string]string{
		"device_id":           "same-device-01",
		"installation_secret": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	})
	assert.Equal(t, 401, w3.Code)

	// 正确 secret → 200，且不再返回明文 secret
	w4 := postAuth(r, map[string]string{
		"device_id":           "same-device-01",
		"installation_secret": first.InstallationSecret,
	})
	assert.Equal(t, 200, w4.Code)
	var second authResponse
	require.NoError(t, json.Unmarshal(w4.Body.Bytes(), &second))
	assert.NotEmpty(t, second.Token)
	assert.Empty(t, second.InstallationSecret)
}

func TestDeviceAuth_ImpersonationWithOnlyDeviceIDFails(t *testing.T) {
	r, _, _ := setupAuthHandlerTest(t)

	// 受害者先注册
	w1 := postAuth(r, map[string]string{"device_id": "victim-device-xyz"})
	require.Equal(t, 200, w1.Code)

	// 攻击者仅用 device_id 尝试换 Token
	w2 := postAuth(r, map[string]string{"device_id": "victim-device-xyz"})
	assert.Equal(t, 401, w2.Code)
	assert.Contains(t, w2.Body.String(), "installation_secret")
}

func TestDeviceAuth_ConcurrentRegisterOnceSecret(t *testing.T) {
	r, _, deviceRepo := setupAuthHandlerTest(t)
	deviceID := "concurrent-device-99"

	const n = 16
	var (
		wg      sync.WaitGroup
		success int32
		secrets = make(chan string, n)
	)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			w := postAuth(r, map[string]string{"device_id": deviceID})
			if w.Code == 200 {
				var resp authResponse
				if json.Unmarshal(w.Body.Bytes(), &resp) == nil && resp.InstallationSecret != "" {
					atomic.AddInt32(&success, 1)
					secrets <- resp.InstallationSecret
				}
			}
		}()
	}
	wg.Wait()
	close(secrets)

	// 至多一个请求拿到明文 secret（并发占用）
	var got []string
	for s := range secrets {
		got = append(got, s)
	}
	assert.LessOrEqual(t, len(got), 1, "最多一个并发请求应拿到 installation_secret")

	// 最终设备应有 hash
	dev, err := deviceRepo.Find(deviceID)
	require.NoError(t, err)
	assert.NotEmpty(t, dev.InstallationSecretHash)

	// 若拿到 secret，用其换 Token 应成功；否则设备已有 secret，仅 device_id 失败
	if len(got) == 1 {
		w := postAuth(r, map[string]string{
			"device_id":           deviceID,
			"installation_secret": got[0],
		})
		assert.Equal(t, 200, w.Code)
	} else {
		// 极罕见：全部 401（均未 claimed 但竞态未拿到明文）— 仍应拒绝无 secret
		w := postAuth(r, map[string]string{"device_id": deviceID})
		assert.Equal(t, 401, w.Code)
	}
	_ = success
}
