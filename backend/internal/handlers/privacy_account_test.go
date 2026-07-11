package handlers

import (
	"encoding/json"
	"net/http"
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
		&models.DeviceMigrationTicket{}, &models.AccountMergeOperation{}, &models.RefreshToken{},
		&models.AccountSecurityToken{},
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

	dev, err := deviceRepo.Find("priv-dev-a")
	require.NoError(t, err)
	assert.True(t, dev.Disabled)

	// old token should fail after bump+disable — device disabled
	w = authedJSON(t, r, "POST", "/api/v1/privacy/delete", tokenA, map[string]string{"scope": "device"})
	assert.NotEqual(t, 200, w.Code)
}
