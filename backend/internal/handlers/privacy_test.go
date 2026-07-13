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
		&models.DeviceAccount{},
		&models.RefreshToken{},
		&models.AdventureRun{},
		&models.IdempotencyRecord{},
		&models.GrowthEvent{},
		&models.ResearcherTrack{},
		&models.CompanionProfile{},
		&models.CompanionMemoryNode{},
		&models.GrowthResetAudit{},
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

func seedPrivacyGrowthData(t *testing.T, db *gorm.DB, deviceID, animalUUID string) {
	t.Helper()
	ownerKey := repo.OwnerKey("", deviceID)
	require.NoError(t, db.Create(&models.GrowthEvent{
		EventID: "privacy-growth-event", OwnerKey: ownerKey, DeviceID: deviceID,
		Kind: models.GrowthEventCompanionMemory, Track: "companion", AnimalUUID: animalUUID,
		DeltaXP: 6, XPAfter: 6, SourceType: "adventure", SourceID: "adventure-export-1",
		ConfigVersion: models.GrowthConfigVersion,
	}).Error)
	require.NoError(t, db.Create(&models.ResearcherTrack{
		OwnerKey: ownerKey, Track: models.GrowthTrackEcology, DeviceID: deviceID,
		XP: 15, Level: 1, ConfigVersion: models.GrowthConfigVersion,
	}).Error)
	require.NoError(t, db.Create(&models.CompanionProfile{
		AnimalUUID: animalUUID, OwnerKey: ownerKey, DeviceID: deviceID,
		BondXP: 6, BondLevel: 0, ConfigVersion: models.GrowthConfigVersion,
	}).Error)
	require.NoError(t, db.Create(&models.CompanionMemoryNode{
		AnimalUUID: animalUUID, NodeID: "first_expedition", OwnerKey: ownerKey,
		Title: "第一次远征", Kind: "memory", Visible: true, Unlocked: true,
	}).Error)
	require.NoError(t, db.Create(&models.GrowthResetAudit{
		AuditID: "privacy-growth-audit", OwnerKey: ownerKey, DeviceID: deviceID,
		Scope: "companion", AnimalUUID: animalUUID, Reason: "test", FromVersion: models.GrowthConfigVersion,
		ToVersion: models.GrowthConfigVersion, SnapshotJSON: `{"bond_xp":6}`,
	}).Error)
	require.NoError(t, db.Create(&models.IdempotencyRecord{
		DeviceID: deviceID, Route: "adventure.generate", Key: "privacy-idempotency",
		RequestHash: "request-hash", Status: "completed", HTTPStatus: http.StatusCreated,
		ResponseBody: `{"title":"应被删除的缓存剧情"}`, ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}).Error)
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
	require.NoError(t, db.Create(&models.AdventureRun{
		RunID:            "adventure-export-1",
		OwnerKey:         "device:dev-privacy-1",
		OperationID:      "privacy-export-operation-1",
		DeviceID:         "dev-privacy-1",
		AnimalUUID:       "uuid-0000",
		Theme:            "mistwood",
		Title:            "雾灯森径的约定",
		Status:           "completed",
		ResultJSON:       `{"intro":"伙伴循着微光走进森林。"}`,
		SelectedChoiceID: "kindness",
		Outcome:          "伙伴守护了迷路的小动物。",
		SouvenirName:     "萤火叶笺",
		BondDelta:        6,
		PromptVersion:    "companion-adventure-zh-v2",
		Source:           "ai",
	}).Error)
	additionalRuns := make([]models.AdventureRun, 0, 500)
	for i := 2; i <= 501; i++ {
		additionalRuns = append(additionalRuns, models.AdventureRun{
			RunID: fmt.Sprintf("adventure-export-%d", i), OwnerKey: repo.OwnerKey("", "dev-privacy-1"),
			OperationID: fmt.Sprintf("privacy-export-operation-%d", i), DeviceID: "dev-privacy-1",
			AnimalUUID: "uuid-0000", Theme: "mistwood", Title: "雾灯森径的约定", Status: "generated",
			ResultJSON: `{"title":"雾灯森径的约定"}`, PromptVersion: "companion-adventure-zh-v2", Source: "ai",
		})
	}
	require.NoError(t, db.CreateInBatches(&additionalRuns, 100).Error)
	seedPrivacyGrowthData(t, db, "dev-privacy-1", "uuid-0000")

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

	var adventures []map[string]interface{}
	require.NoError(t, json.Unmarshal(data["adventures"], &adventures))
	require.Len(t, adventures, 501, "adventure export must not stop at the former 500-row limit")
	assert.Equal(t, "adventure-export-1", adventures[0]["run_id"])
	assert.Equal(t, "雾灯森径的约定", adventures[0]["title"])
	assert.Equal(t, "萤火叶笺", adventures[0]["souvenir_name"])
	assert.EqualValues(t, 6, adventures[0]["bond_delta"])
	assert.NotContains(t, adventures[0], "operation_id")
	assert.NotContains(t, adventures[0], "prompt_version")
	assert.NotContains(t, adventures[0], "source")

	var growth map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data["growth"], &growth))
	for _, key := range []string{"events", "researcher_tracks", "companions", "companion_nodes", "reset_audits"} {
		var rows []map[string]any
		require.NoError(t, json.Unmarshal(growth[key], &rows), key)
		require.Len(t, rows, 1, key)
	}
}

func TestExportData_AdventureQueryFailureMarksRequestFailed(t *testing.T) {
	r, db, _, deviceRepo := setupPrivacyLifecycle(t)
	_, err := deviceRepo.FindOrCreate("dev-privacy-1")
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&models.AdventureRun{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/privacy/export", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "export_failed")

	var request models.DataRequest
	require.NoError(t, db.Where("device_id = ? AND type = ?", "dev-privacy-1", "export").Order("id desc").First(&request).Error)
	assert.Equal(t, "failed", request.Status)
	assert.NotEmpty(t, request.ErrorMsg)
	assert.NotNil(t, request.CompletedAt)
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
	require.NoError(t, db.Create(&models.AdventureRun{
		RunID:       "adventure-delete-1",
		OwnerKey:    "device:dev-privacy-1",
		OperationID: "privacy-delete-operation-1",
		DeviceID:    "dev-privacy-1",
		AnimalUUID:  "uuid-0000",
		Theme:       "mistwood",
		Title:       "待删除的探险",
		Status:      "generated",
		ResultJSON:  `{"intro":"这段探险应随隐私删除一并清除。"}`,
	}).Error)
	seedPrivacyGrowthData(t, db, "dev-privacy-1", "uuid-0000")

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

	// 探险剧情已删
	var adventureCount int64
	require.NoError(t, db.Model(&models.AdventureRun{}).Where("device_id = ?", "dev-privacy-1").Count(&adventureCount).Error)
	assert.EqualValues(t, 0, adventureCount)
	for _, model := range []any{
		&models.GrowthEvent{}, &models.ResearcherTrack{}, &models.CompanionProfile{},
		&models.CompanionMemoryNode{}, &models.GrowthResetAudit{}, &models.IdempotencyRecord{},
	} {
		var count int64
		require.NoError(t, db.Model(model).Count(&count).Error)
		assert.Zero(t, count)
	}

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
	var adventures2 []interface{}
	require.NoError(t, json.Unmarshal(data2["adventures"], &adventures2))
	assert.Len(t, adventures2, 0)
}

func TestDeleteData_AdventureDeleteFailureRollsBack(t *testing.T) {
	r, db, _, deviceRepo := setupPrivacyLifecycle(t)
	dev, err := deviceRepo.FindOrCreate("dev-privacy-1")
	require.NoError(t, err)
	seedAnimals(t, db, "dev-privacy-1", 1)
	require.NoError(t, db.Create(&models.AdventureRun{
		RunID:       "adventure-delete-failure",
		OwnerKey:    repo.OwnerKey("", "dev-privacy-1"),
		OperationID: "adventure-delete-failure-operation",
		DeviceID:    "dev-privacy-1",
		AnimalUUID:  "uuid-0000",
		Theme:       "mistwood",
		Title:       "不可静默失败的探险",
		Status:      "generated",
		ResultJSON:  `{"title":"不可静默失败的探险"}`,
	}).Error)
	require.NoError(t, db.Exec(`
		CREATE TRIGGER fail_adventure_delete
		BEFORE DELETE ON adventure_runs
		BEGIN
			SELECT RAISE(ABORT, 'forced adventure delete failure');
		END;
	`).Error)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/privacy/delete", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "delete_failed")

	var animal models.Animal
	require.NoError(t, db.Where("uuid = ?", "uuid-0000").First(&animal).Error)
	assert.Nil(t, animal.DeletedAt, "the surrounding transaction must roll back")
	var adventureCount int64
	require.NoError(t, db.Model(&models.AdventureRun{}).Where("run_id = ?", "adventure-delete-failure").Count(&adventureCount).Error)
	assert.EqualValues(t, 1, adventureCount)
	refreshed, err := deviceRepo.Find("dev-privacy-1")
	require.NoError(t, err)
	assert.Equal(t, dev.TokenVersion, refreshed.TokenVersion)
}

func TestPullAnimals_TombstoneOnlyAfterDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:pull_tomb_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Animal{}, &models.Device{}, &models.DataRequest{}, &models.Inference{}, &models.SecurityReport{},
		&models.Entitlement{}, &models.Order{}, &models.AuditLog{}, &models.DeviceAccount{}, &models.RefreshToken{},
		&models.AdventureRun{}, &models.IdempotencyRecord{}, &models.GrowthEvent{}, &models.ResearcherTrack{},
		&models.CompanionProfile{}, &models.CompanionMemoryNode{}, &models.GrowthResetAudit{},
	))

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
