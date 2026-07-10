package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPrivacyLifecycle(t *testing.T) (*gin.Engine, *gorm.DB, *repo.AnimalRepo, *repo.DeviceRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:privacy_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Device{},
		&models.Animal{},
		&models.Inference{},
		&models.DataRequest{},
		&models.SecurityReport{},
		&models.Order{},
		&models.Entitlement{},
		&models.AuditLog{},
	))

	deviceRepo := repo.NewDeviceRepo(db)
	animalRepo := repo.NewAnimalRepo(db)
	infRepo := repo.NewInferenceRepo(db)
	auditRepo := repo.NewAuditLogRepo(db)
	privacy := NewPrivacyHandler(db, deviceRepo, animalRepo, infRepo, auditRepo)

	r := gin.New()
	inject := func(c *gin.Context) {
		c.Set(middleware.ContextKeyDeviceID, "dev-privacy-1")
		c.Next()
	}
	r.POST("/privacy/export", inject, privacy.ExportData)
	r.POST("/privacy/delete", inject, privacy.DeleteData)
	return r, db, animalRepo, deviceRepo
}

func seedAnimals(t *testing.T, db *gorm.DB, deviceID string, n int) {
	t.Helper()
	now := time.Now().UTC()
	for i := range n {
		pl := 31.2 + float64(i)*0.0001
		pg := 121.5 + float64(i)*0.0001
		exp := now.Add(time.Hour)
		a := models.Animal{
			UUID:             fmt.Sprintf("uuid-%04d", i),
			DeviceID:         deviceID,
			Species:          "cat",
			Rarity:           2,
			GeneratedAt:      now,
			ServerVersion:    int64(i + 1),
			PreciseLat:       &pl,
			PreciseLng:       &pg,
			PreciseExpiresAt: &exp,
		}
		require.NoError(t, db.Create(&a).Error)
	}
}

func TestExportData_Complete201Animals(t *testing.T) {
	r, db, _, deviceRepo := setupPrivacyLifecycle(t)
	_, err := deviceRepo.FindOrCreate("dev-privacy-1")
	require.NoError(t, err)
	require.NoError(t, deviceRepo.UpdateConsent("dev-privacy-1", "v1", "photo,location", false))
	seedAnimals(t, db, "dev-privacy-1", 201)

	// 安全报告与历史请求
	require.NoError(t, db.Create(&models.SecurityReport{
		ReportID: "r1", DeviceID: "dev-privacy-1", Nonce: "n1", Payload: "secret-should-not-export", RiskScore: 3,
	}).Error)
	require.NoError(t, db.Create(&models.Order{
		OrderID: "o1", DeviceID: "dev-privacy-1", ProductID: "month", Status: "fulfilled",
		IdempotencyKey: "ik1", ReceiptHash: strPtr("rh1"),
	}).Error)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/privacy/export", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	var animals []map[string]interface{}
	require.NoError(t, json.Unmarshal(data["animals"], &animals))
	assert.Len(t, animals, 201, "export must paginate beyond first 200")

	// 精确坐标不得出现在导出 JSON
	body := w.Body.String()
	assert.NotContains(t, body, "precise_lat")
	assert.NotContains(t, body, "secret-should-not-export")

	var consent map[string]interface{}
	require.NoError(t, json.Unmarshal(data["consent"], &consent))
	assert.Equal(t, "v1", consent["version"])

	var sec map[string]interface{}
	require.NoError(t, json.Unmarshal(data["security_reports"], &sec))
	assert.EqualValues(t, 1, sec["count"])
}

func TestDeleteData_TombstonePullAndTokenBump(t *testing.T) {
	r, db, animalRepo, deviceRepo := setupPrivacyLifecycle(t)
	dev, err := deviceRepo.FindOrCreate("dev-privacy-1")
	require.NoError(t, err)
	require.NoError(t, deviceRepo.UpdateConsent("dev-privacy-1", "v1", "photo", false))
	beforeTV := dev.TokenVersion

	// 3 只动物 + 推理 + 安全报告 + 导出 payload + 权益
	seedAnimals(t, db, "dev-privacy-1", 3)
	require.NoError(t, db.Create(&models.Inference{
		InferenceID: "inf-1", DeviceID: "dev-privacy-1", Kind: "value", Status: "success",
	}).Error)
	require.NoError(t, db.Create(&models.SecurityReport{
		ReportID: "sr1", DeviceID: "dev-privacy-1", Nonce: "nx", Payload: "x",
	}).Error)
	require.NoError(t, db.Create(&models.DataRequest{
		RequestID: "exp-old", DeviceID: "dev-privacy-1", Type: "export", Status: "completed",
		Payload: `{"animals":[{"uuid":"leak"}]}`, RequestedAt: time.Now().UTC(),
	}).Error)
	require.NoError(t, db.Create(&models.Entitlement{
		DeviceID: "dev-privacy-1", ProductID: "month", Active: true, StartsAt: time.Now().UTC(),
	}).Error)
	require.NoError(t, db.Create(&models.Order{
		OrderID: "keep-order", DeviceID: "dev-privacy-1", ProductID: "month", Status: "fulfilled",
		IdempotencyKey: "ik-keep", ReceiptHash: strPtr("rh-keep"),
	}).Error)

	// delete
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/privacy/delete", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var delResp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &delResp))
	assert.Equal(t, "completed", delResp["status"])

	// token version bumped
	dev2, err := deviceRepo.Find("dev-privacy-1")
	require.NoError(t, err)
	assert.Equal(t, beforeTV+1, dev2.TokenVersion)
	assert.NotNil(t, dev2.ConsentRevoked)

	// pull / ListSinceVersion → tombstones only
	items, err := animalRepo.ListSinceVersion("dev-privacy-1", 0, 50)
	require.NoError(t, err)
	require.Len(t, items, 3)
	for _, it := range items {
		assert.NotNil(t, it.DeletedAt)
		assert.Empty(t, it.Species, "tombstone must not carry full content")
		assert.Empty(t, it.DeviceID)
		assert.Nil(t, it.PreciseLat)
		assert.NotEmpty(t, it.UUID)
		assert.Greater(t, it.ServerVersion, int64(0))
	}

	// 历史 export payload 已清空
	var oldExp models.DataRequest
	require.NoError(t, db.Where("request_id = ?", "exp-old").First(&oldExp).Error)
	assert.Empty(t, oldExp.Payload)

	// 安全报告已删
	var secCount int64
	require.NoError(t, db.Model(&models.SecurityReport{}).Where("device_id = ?", "dev-privacy-1").Count(&secCount).Error)
	assert.EqualValues(t, 0, secCount)

	// 推理已删
	var infCount int64
	require.NoError(t, db.Model(&models.Inference{}).Where("device_id = ?", "dev-privacy-1").Count(&infCount).Error)
	assert.EqualValues(t, 0, infCount)

	// 权益失效；订单保留
	var ent models.Entitlement
	require.NoError(t, db.Where("device_id = ?", "dev-privacy-1").First(&ent).Error)
	assert.False(t, ent.Active)
	var ord models.Order
	require.NoError(t, db.Where("order_id = ?", "keep-order").First(&ord).Error)
	assert.Equal(t, "fulfilled", ord.Status)

	// re-export：无动物内容
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodPost, "/privacy/export", nil)
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)
	var resp2 map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	var data2 map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp2["data"], &data2))
	var animals2 []interface{}
	require.NoError(t, json.Unmarshal(data2["animals"], &animals2))
	assert.Len(t, animals2, 0)
}

func TestPullAnimals_TombstoneOnlyAfterDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:pull_tomb_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Animal{}, &models.Device{}, &models.DataRequest{}, &models.Inference{}, &models.SecurityReport{}, &models.Entitlement{}, &models.Order{}, &models.AuditLog{}))

	deviceRepo := repo.NewDeviceRepo(db)
	animalRepo := repo.NewAnimalRepo(db)
	_, _ = deviceRepo.FindOrCreate("dev-privacy-1")
	seedAnimals(t, db, "dev-privacy-1", 2)

	// soft delete via privacy handler path
	privacy := NewPrivacyHandler(db, deviceRepo, animalRepo, repo.NewInferenceRepo(db), repo.NewAuditLogRepo(db))
	pr := gin.New()
	pr.POST("/privacy/delete", func(c *gin.Context) {
		c.Set(middleware.ContextKeyDeviceID, "dev-privacy-1")
		privacy.DeleteData(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/privacy/delete", nil)
	pr.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	sync := NewSyncHandler(animalRepo, nil)
	sr := gin.New()
	sr.GET("/sync/animals", func(c *gin.Context) {
		c.Set(middleware.ContextKeyDeviceID, "dev-privacy-1")
		sync.PullAnimals(c)
	})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/sync/animals?since_version=0", nil)
	sr.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &body))
	items, ok := body["items"].([]interface{})
	require.True(t, ok)
	require.Len(t, items, 2)
	for _, raw := range items {
		m := raw.(map[string]interface{})
		assert.NotEmpty(t, m["uuid"])
		assert.NotNil(t, m["deleted_at"])
		assert.NotNil(t, m["server_version"])
		_, hasSpecies := m["species"]
		assert.False(t, hasSpecies, "tombstone must not include species")
	}
}

func strPtr(s string) *string { return &s }
