package repo

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupInferenceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:inf_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Inference{}, &models.Animal{}))
	return db
}

func TestConsumeValue_AtomicSingleWinner(t *testing.T) {
	db := setupInferenceDB(t)
	repo := NewInferenceRepo(db)
	payload, _ := json.Marshal(map[string]interface{}{
		"species": "cat", "rarity": 3, "hp": 50, "atk": 20, "def": 20, "spd": 20, "class": "Ranger", "element": "Wind",
	})
	exp := time.Now().UTC().Add(time.Hour)
	require.NoError(t, repo.Create(&models.Inference{
		InferenceID: "inf-1", DeviceID: "dev-1", Kind: "value", Status: "success",
		Species: "cat", ResultJSON: string(payload), ExpiresAt: &exp,
	}))

	var okCount int64
	var wg sync.WaitGroup
	n := 50
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			err := db.Transaction(func(tx *gorm.DB) error {
				_, err := repo.WithTx(tx).ConsumeValue(tx, "inf-1", "dev-1", "cat")
				return err
			})
			if err == nil {
				atomic.AddInt64(&okCount, 1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(1), okCount)
}

func TestConsumeValue_RejectDetectKind(t *testing.T) {
	db := setupInferenceDB(t)
	repo := NewInferenceRepo(db)
	require.NoError(t, repo.Create(&models.Inference{
		InferenceID: "det-1", DeviceID: "dev-1", Kind: "detect", Status: "success",
	}))
	_, err := repo.ConsumeValue(nil, "det-1", "dev-1", "")
	assert.ErrorIs(t, err, ErrInferenceWrongKind)
}

func TestConsumeValue_Expired(t *testing.T) {
	db := setupInferenceDB(t)
	repo := NewInferenceRepo(db)
	past := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, repo.Create(&models.Inference{
		InferenceID: "exp-1", DeviceID: "dev-1", Kind: "value", Status: "success", ExpiresAt: &past,
	}))
	_, err := repo.ConsumeValue(nil, "exp-1", "dev-1", "")
	assert.ErrorIs(t, err, ErrInferenceExpired)
}

func TestConsumeValue_WrongDevice(t *testing.T) {
	db := setupInferenceDB(t)
	repo := NewInferenceRepo(db)
	require.NoError(t, repo.Create(&models.Inference{
		InferenceID: "inf-2", DeviceID: "dev-a", Kind: "value", Status: "success",
	}))
	_, err := repo.ConsumeValue(nil, "inf-2", "dev-b", "")
	assert.ErrorIs(t, err, ErrInferenceNotFound)
}

func TestConsumeValue_SpeciesMismatch(t *testing.T) {
	db := setupInferenceDB(t)
	repo := NewInferenceRepo(db)
	require.NoError(t, repo.Create(&models.Inference{
		InferenceID: "inf-3", DeviceID: "dev-1", Kind: "value", Status: "success", Species: "cat",
	}))
	_, err := repo.ConsumeValue(nil, "inf-3", "dev-1", "dog")
	assert.ErrorIs(t, err, ErrInferenceSpeciesMismatch)
}
