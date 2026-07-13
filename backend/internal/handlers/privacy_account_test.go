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

func setupPrivacyAccount(t *testing.T) (*gin.Engine, *gorm.DB, *repo.AccountRepo, *repo.DeviceRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:privacc_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Device{}, &models.Account{}, &models.AccountBinding{}, &models.DeviceAccount{},
		&models.Animal{}, &models.Entitlement{}, &models.Order{}, &models.Product{},
		&models.DataRequest{}, &models.SecurityReport{}, &models.Inference{},
		&models.AuditLog{},
		&models.DeviceMigrationTicket{}, &models.AccountMergeOperation{}, &models.RefreshToken{},
		&models.AccountSecurityToken{},
		&models.AdventureRun{},
		&models.IdempotencyRecord{},
		&models.GrowthEvent{},
		&models.ResearcherTrack{},
		&models.CompanionProfile{},
		&models.CompanionMemoryNode{},
		&models.GrowthResetAudit{},
	))
	deviceRepo := repo.NewDeviceRepo(db)
	accountRepo := repo.NewAccountRepo(db, "test-pepper-secret")
	animalRepo := repo.NewAnimalRepo(db)
	infRepo := repo.NewInferenceRepo(db)
	authH := NewAuthHandler(deviceRepo, "test-secret", 24*time.Hour)
	acctH := NewAccountHandler(deviceRepo, accountRepo, "test-secret", 24*time.Hour, "animal-poke", "animal-poke-client", true)
	priv := NewPrivacyHandlerFull(db, deviceRepo, animalRepo, infRepo, nil, accountRepo)

	r := gin.New()
	r.POST("/api/v1/auth/device", authH.DeviceAuth)
	r.POST("/api/v1/auth/login", acctH.Login)
	r.POST("/api/v1/auth/email/verify", acctH.VerifyEmail)
	auth := r.Group("/api/v1")
	auth.Use(middleware.JWTAuthWithChecker("test-secret", "animal-poke", "animal-poke-client", deviceCheckerAdapter{deviceRepo}))
	{
		auth.POST("/auth/bind", acctH.Bind)
		auth.POST("/privacy/export", priv.ExportData)
		auth.POST("/privacy/delete", priv.DeleteData)
		auth.GET("/sync/animals", func(c *gin.Context) {
			deviceID := middleware.GetDeviceID(c)
			accountID := middleware.GetAccountID(c)
			items, err := animalRepo.ListSinceVersionScoped(deviceID, accountID, 0, 50)
			if err != nil {
				c.JSON(500, gin.H{"error": "pull failed"})
				return
			}
			c.JSON(200, gin.H{"items": items})
		})
	}
	return r, db, accountRepo, deviceRepo
}

func TestPrivacy_AccountDelete_RequiresReauth(t *testing.T) {
	r, _, _, _ := setupPrivacyAccount(t)
	token := deviceAuth(t, r, "priv-dev-1")
	// bind email
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "email", "email": "user@example.com", "password": "password12",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	verifyEmailFromBind(t, r, w.Body.Bytes())
	var bind map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bind))
	token = bind["token"].(string)

	// missing reauth
	w = authedJSON(t, r, "POST", "/api/v1/privacy/delete", token, map[string]string{
		"scope": "account", "confirm": "DELETE",
	})
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

func TestPrivacy_AccountDelete_WipesAccountData(t *testing.T) {
	r, db, _, deviceRepo := setupPrivacyAccount(t)
	tokenA := deviceAuth(t, r, "priv-dev-a")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", tokenA, map[string]string{
		"provider": "email", "email": "wipe@example.com", "password": "password12",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	verifyEmailFromBind(t, r, w.Body.Bytes())
	var bind map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bind))
	tokenA = bind["token"].(string)
	accountID := bind["account_id"].(string)

	require.NoError(t, db.Create(&models.Animal{
		UUID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", DeviceID: "priv-dev-a", AccountID: accountID,
		Species: "cat", Rarity: 1, GeneratedAt: time.Now().UTC(), ServerVersion: 1,
	}).Error)
	require.NoError(t, db.Create(&models.AdventureRun{
		RunID:       "account-adventure-delete-1",
		OwnerKey:    "account:" + accountID,
		OperationID: "account-privacy-delete-operation-1",
		DeviceID:    "priv-dev-a",
		AccountID:   accountID,
		AnimalUUID:  "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Theme:       "mistwood",
		Title:       "账号下的探险",
		Status:      "generated",
		ResultJSON:  `{"intro":"这段探险应随账号删除一并清除。"}`,
	}).Error)
	const foreignAccountID = "foreign-account-that-must-survive"
	require.NoError(t, db.Create(&models.Animal{
		UUID: "ffffffff-ffff-ffff-ffff-ffffffffffff", DeviceID: "priv-dev-a", AccountID: foreignAccountID,
		Species: "dog", Rarity: 1, GeneratedAt: time.Now().UTC(), ServerVersion: 1,
	}).Error)
	require.NoError(t, db.Create(&models.AdventureRun{
		RunID:       "foreign-account-adventure",
		OwnerKey:    repo.OwnerKey(foreignAccountID, "priv-dev-a"),
		OperationID: "foreign-account-operation",
		DeviceID:    "priv-dev-a",
		AccountID:   foreignAccountID,
		AnimalUUID:  "ffffffff-ffff-ffff-ffff-ffffffffffff",
		Theme:       "mistwood",
		Title:       "其他账号的探险",
		Status:      "generated",
		ResultJSON:  `{"title":"其他账号的探险"}`,
	}).Error)
	require.NoError(t, db.Create(&models.Device{
		DeviceID: "rebound-history-device", AccountID: foreignAccountID, TokenVersion: 1,
	}).Error)
	require.NoError(t, db.Create(&models.DeviceAccount{
		DeviceID: "rebound-history-device", AccountID: accountID, Status: "revoked", LinkedAt: time.Now().UTC(),
	}).Error)

	w = authedJSON(t, r, "POST", "/api/v1/privacy/delete", tokenA, map[string]string{
		"scope": "account", "confirm": "DELETE", "reauth_password": "password12",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "completed", resp["status"])
	assert.Equal(t, "account", resp["scope"])

	var a models.Animal
	require.NoError(t, db.Where("uuid = ?", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa").First(&a).Error)
	assert.NotNil(t, a.DeletedAt)

	var acc models.Account
	require.NoError(t, db.Where("account_id = ?", accountID).First(&acc).Error)
	assert.Equal(t, "deleted", acc.Status)

	var adventureCount int64
	require.NoError(t, db.Model(&models.AdventureRun{}).Where("account_id = ?", accountID).Count(&adventureCount).Error)
	assert.EqualValues(t, 0, adventureCount)
	var foreignAnimal models.Animal
	require.NoError(t, db.Where("uuid = ?", "ffffffff-ffff-ffff-ffff-ffffffffffff").First(&foreignAnimal).Error)
	assert.Nil(t, foreignAnimal.DeletedAt, "account deletion must not scan another account by device_id")
	var foreignAdventureCount int64
	require.NoError(t, db.Model(&models.AdventureRun{}).Where("run_id = ?", "foreign-account-adventure").Count(&foreignAdventureCount).Error)
	assert.EqualValues(t, 1, foreignAdventureCount)
	rebound, err := deviceRepo.Find("rebound-history-device")
	require.NoError(t, err)
	assert.False(t, rebound.Disabled, "revoked or rebound historical devices must not be disabled")

	dev, err := deviceRepo.Find("priv-dev-a")
	require.NoError(t, err)
	assert.True(t, dev.Disabled)

	// old token should fail after bump+disable — device disabled
	w = authedJSON(t, r, "POST", "/api/v1/privacy/delete", tokenA, map[string]string{"scope": "device"})
	assert.NotEqual(t, 200, w.Code)
}

func decodePrivacyExportData(t *testing.T, response *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope["data"], &data))
	return data
}

func TestPrivacy_ExportScopesSeparateAccountDevices(t *testing.T) {
	r, db, _, _ := setupPrivacyAccount(t)
	tokenA := deviceAuth(t, r, "privacy-export-device-a")
	bind := authedJSON(t, r, http.MethodPost, "/api/v1/auth/bind", tokenA, map[string]string{
		"provider": "email", "email": "scope@example.com", "password": "password12",
	})
	require.Equal(t, http.StatusOK, bind.Code, bind.Body.String())
	verifyEmailFromBind(t, r, bind.Body.Bytes())
	var bound map[string]any
	require.NoError(t, json.Unmarshal(bind.Body.Bytes(), &bound))
	tokenA = bound["token"].(string)
	accountID := bound["account_id"].(string)

	loginB := postLogin(t, r, map[string]string{
		"device_id": "privacy-export-device-b", "provider": "email",
		"email": "scope@example.com", "password": "password12",
	})
	require.Equal(t, http.StatusOK, loginB.Code, loginB.Body.String())

	for _, animal := range []models.Animal{
		{UUID: "10000000-0000-0000-0000-000000000001", DeviceID: "privacy-export-device-a", AccountID: accountID, Species: "cat", Rarity: 1, GeneratedAt: time.Now().UTC(), ServerVersion: 1},
		{UUID: "10000000-0000-0000-0000-000000000002", DeviceID: "privacy-export-device-b", AccountID: accountID, Species: "dog", Rarity: 1, GeneratedAt: time.Now().UTC(), ServerVersion: 2},
	} {
		require.NoError(t, db.Create(&animal).Error)
	}
	for i, deviceID := range []string{"privacy-export-device-a", "privacy-export-device-b"} {
		run := models.AdventureRun{
			RunID: fmt.Sprintf("scope-account-run-%d", i), OwnerKey: repo.OwnerKey(accountID, deviceID),
			OperationID: fmt.Sprintf("scope-account-operation-%d", i), DeviceID: deviceID, AccountID: accountID,
			AnimalUUID: fmt.Sprintf("10000000-0000-0000-0000-00000000000%d", i+1), Theme: "mistwood",
			Title: "账号探险", Status: "generated", ResultJSON: `{"title":"账号探险"}`,
		}
		require.NoError(t, db.Create(&run).Error)
		event := models.GrowthEvent{
			EventID: fmt.Sprintf("scope-account-event-%d", i), OwnerKey: repo.OwnerKey(accountID, deviceID),
			DeviceID: deviceID, AccountID: accountID, Kind: models.GrowthEventCompanionMemory,
			Track: "companion", ConfigVersion: models.GrowthConfigVersion,
		}
		require.NoError(t, db.Create(&event).Error)
	}
	require.NoError(t, db.Create(&models.ResearcherTrack{
		OwnerKey: repo.OwnerKey(accountID, ""), Track: models.GrowthTrackEcology,
		DeviceID: "privacy-export-device-a", AccountID: accountID, XP: 21, Level: 1,
		ConfigVersion: models.GrowthConfigVersion,
	}).Error)
	require.NoError(t, db.Create(&models.ResearcherTrack{
		OwnerKey: repo.OwnerKey("", "privacy-export-device-a"), Track: models.GrowthTrackPhotography,
		DeviceID: "privacy-export-device-a", XP: 5, ConfigVersion: models.GrowthConfigVersion,
	}).Error)

	deviceData := decodePrivacyExportData(t, authedJSON(t, r, http.MethodPost, "/api/v1/privacy/export", tokenA, map[string]string{"scope": "device"}))
	var deviceAnimals, deviceAdventures, deviceEvents, deviceTracks []map[string]any
	require.NoError(t, json.Unmarshal(deviceData["animals"], &deviceAnimals))
	require.NoError(t, json.Unmarshal(deviceData["adventures"], &deviceAdventures))
	var deviceGrowth map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(deviceData["growth"], &deviceGrowth))
	require.NoError(t, json.Unmarshal(deviceGrowth["events"], &deviceEvents))
	require.NoError(t, json.Unmarshal(deviceGrowth["researcher_tracks"], &deviceTracks))
	assert.Len(t, deviceAnimals, 1)
	assert.Len(t, deviceAdventures, 1)
	assert.Len(t, deviceEvents, 1)
	require.Len(t, deviceTracks, 1)
	assert.Equal(t, repo.OwnerKey("", "privacy-export-device-a"), deviceTracks[0]["owner_key"])

	accountData := decodePrivacyExportData(t, authedJSON(t, r, http.MethodPost, "/api/v1/privacy/export", tokenA, map[string]string{"scope": "account"}))
	var accountAnimals, accountAdventures, accountEvents, accountTracks, devices []map[string]any
	require.NoError(t, json.Unmarshal(accountData["animals"], &accountAnimals))
	require.NoError(t, json.Unmarshal(accountData["adventures"], &accountAdventures))
	var accountGrowth map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(accountData["growth"], &accountGrowth))
	require.NoError(t, json.Unmarshal(accountGrowth["events"], &accountEvents))
	require.NoError(t, json.Unmarshal(accountGrowth["researcher_tracks"], &accountTracks))
	require.NoError(t, json.Unmarshal(accountData["devices"], &devices))
	assert.Len(t, accountAnimals, 2)
	assert.Len(t, accountAdventures, 2)
	assert.Len(t, accountEvents, 2)
	require.Len(t, accountTracks, 1)
	assert.Equal(t, repo.OwnerKey(accountID, ""), accountTracks[0]["owner_key"])
	assert.Len(t, devices, 2)
}

func TestPrivacy_DeviceScopeExcludesPreviousAccountAfterRebind(t *testing.T) {
	r, db, _, _ := setupPrivacyAccount(t)
	token := deviceAuth(t, r, "privacy-rebound-device")
	bind := authedJSON(t, r, http.MethodPost, "/api/v1/auth/bind", token, map[string]string{
		"provider": "email", "email": "current@example.com", "password": "password12",
	})
	require.Equal(t, http.StatusOK, bind.Code, bind.Body.String())
	verifyEmailFromBind(t, r, bind.Body.Bytes())
	var bound map[string]any
	require.NoError(t, json.Unmarshal(bind.Body.Bytes(), &bound))
	token = bound["token"].(string)
	currentAccountID := bound["account_id"].(string)
	const previousAccountID = "previous-account-id"

	for _, animal := range []models.Animal{
		{UUID: "20000000-0000-0000-0000-000000000001", DeviceID: "privacy-rebound-device", AccountID: previousAccountID, Species: "cat", Rarity: 1, GeneratedAt: time.Now().UTC(), ServerVersion: 1},
		{UUID: "20000000-0000-0000-0000-000000000002", DeviceID: "privacy-rebound-device", AccountID: currentAccountID, Species: "dog", Rarity: 1, GeneratedAt: time.Now().UTC(), ServerVersion: 2},
	} {
		require.NoError(t, db.Create(&animal).Error)
	}
	for i, accountID := range []string{previousAccountID, currentAccountID} {
		run := models.AdventureRun{
			RunID: fmt.Sprintf("rebound-run-%d", i), OwnerKey: repo.OwnerKey(accountID, "privacy-rebound-device"),
			OperationID: fmt.Sprintf("rebound-operation-%d", i), DeviceID: "privacy-rebound-device", AccountID: accountID,
			AnimalUUID: fmt.Sprintf("20000000-0000-0000-0000-00000000000%d", i+1), Theme: "mistwood",
			Title: "重绑设备探险", Status: "generated", ResultJSON: `{"title":"重绑设备探险"}`,
		}
		require.NoError(t, db.Create(&run).Error)
		event := models.GrowthEvent{
			EventID: fmt.Sprintf("rebound-event-%d", i), OwnerKey: repo.OwnerKey(accountID, "privacy-rebound-device"),
			DeviceID: "privacy-rebound-device", AccountID: accountID, Kind: models.GrowthEventCompanionMemory,
			Track: "companion", ConfigVersion: models.GrowthConfigVersion,
		}
		require.NoError(t, db.Create(&event).Error)
		require.NoError(t, db.Create(&models.ResearcherTrack{
			OwnerKey: repo.OwnerKey(accountID, ""), Track: models.GrowthTrackEcology,
			DeviceID: "privacy-rebound-device", AccountID: accountID, XP: int64(10 + i),
			ConfigVersion: models.GrowthConfigVersion,
		}).Error)
	}

	deviceData := decodePrivacyExportData(t, authedJSON(t, r, http.MethodPost, "/api/v1/privacy/export", token, map[string]string{"scope": "device"}))
	assert.NotContains(t, string(deviceData["animals"]), previousAccountID)
	var exportedAnimals, exportedAdventures []map[string]any
	require.NoError(t, json.Unmarshal(deviceData["animals"], &exportedAnimals))
	require.NoError(t, json.Unmarshal(deviceData["adventures"], &exportedAdventures))
	require.Len(t, exportedAnimals, 1)
	require.Len(t, exportedAdventures, 1)
	assert.Equal(t, currentAccountID, exportedAnimals[0]["account_id"])
	assert.Equal(t, "rebound-run-1", exportedAdventures[0]["run_id"])

	deleted := authedJSON(t, r, http.MethodPost, "/api/v1/privacy/delete", token, map[string]string{"scope": "device"})
	require.Equal(t, http.StatusOK, deleted.Code, deleted.Body.String())
	var previousAnimal, currentAnimal models.Animal
	require.NoError(t, db.Where("uuid = ?", "20000000-0000-0000-0000-000000000001").First(&previousAnimal).Error)
	require.NoError(t, db.Where("uuid = ?", "20000000-0000-0000-0000-000000000002").First(&currentAnimal).Error)
	assert.Nil(t, previousAnimal.DeletedAt)
	assert.NotNil(t, currentAnimal.DeletedAt)
	var previousRuns, currentRuns, previousEvents, currentEvents int64
	require.NoError(t, db.Model(&models.AdventureRun{}).Where("account_id = ?", previousAccountID).Count(&previousRuns).Error)
	require.NoError(t, db.Model(&models.AdventureRun{}).Where("account_id = ?", currentAccountID).Count(&currentRuns).Error)
	require.NoError(t, db.Model(&models.GrowthEvent{}).Where("account_id = ?", previousAccountID).Count(&previousEvents).Error)
	require.NoError(t, db.Model(&models.GrowthEvent{}).Where("account_id = ?", currentAccountID).Count(&currentEvents).Error)
	assert.EqualValues(t, 1, previousRuns)
	assert.Zero(t, currentRuns)
	assert.EqualValues(t, 1, previousEvents)
	assert.Zero(t, currentEvents)
	var tracks int64
	require.NoError(t, db.Model(&models.ResearcherTrack{}).Count(&tracks).Error)
	assert.EqualValues(t, 2, tracks, "account-owned aggregate snapshots must survive device deletion")
}

func TestPrivacy_AccountDelete_AdventureFailureRollsBack(t *testing.T) {
	r, db, _, deviceRepo := setupPrivacyAccount(t)
	token := deviceAuth(t, r, "priv-dev-failure")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "email", "email": "failure@example.com", "password": "password12",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	verifyEmailFromBind(t, r, w.Body.Bytes())
	var bind map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bind))
	token = bind["token"].(string)
	accountID := bind["account_id"].(string)

	require.NoError(t, db.Create(&models.Animal{
		UUID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", DeviceID: "priv-dev-failure", AccountID: accountID,
		Species: "dog", Rarity: 2, GeneratedAt: time.Now().UTC(), ServerVersion: 1,
	}).Error)
	require.NoError(t, db.Create(&models.AdventureRun{
		RunID:       "account-adventure-delete-failure",
		OwnerKey:    repo.OwnerKey(accountID, "priv-dev-failure"),
		OperationID: "account-adventure-delete-failure-operation",
		DeviceID:    "priv-dev-failure",
		AccountID:   accountID,
		AnimalUUID:  "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		Theme:       "mistwood",
		Title:       "账号删除失败时的探险",
		Status:      "generated",
		ResultJSON:  `{"title":"账号删除失败时的探险"}`,
	}).Error)
	require.NoError(t, db.Exec(`
		CREATE TRIGGER fail_account_adventure_delete
		BEFORE DELETE ON adventure_runs
		BEGIN
			SELECT RAISE(ABORT, 'forced account adventure delete failure');
		END;
	`).Error)

	w = authedJSON(t, r, "POST", "/api/v1/privacy/delete", token, map[string]string{
		"scope": "account", "confirm": "DELETE", "reauth_password": "password12",
	})
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "delete_failed")

	var animal models.Animal
	require.NoError(t, db.Where("uuid = ?", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb").First(&animal).Error)
	assert.Nil(t, animal.DeletedAt, "account deletion transaction must roll back")
	var adventureCount int64
	require.NoError(t, db.Model(&models.AdventureRun{}).Where("account_id = ?", accountID).Count(&adventureCount).Error)
	assert.EqualValues(t, 1, adventureCount)
	var account models.Account
	require.NoError(t, db.Where("account_id = ?", accountID).First(&account).Error)
	assert.Equal(t, "active", account.Status)
	dev, err := deviceRepo.Find("priv-dev-failure")
	require.NoError(t, err)
	assert.False(t, dev.Disabled)
}
