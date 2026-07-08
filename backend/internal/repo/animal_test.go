package repo

import (
	"testing"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAnimalRepo(t *testing.T) (*AnimalRepo, *AuditLogRepo) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	err = db.AutoMigrate(&models.Animal{}, &models.AuditLog{})
	assert.NoError(t, err)
	return NewAnimalRepo(db), NewAuditLogRepo(db)
}

func TestAnimalRepo_CreateAndFind(t *testing.T) {
	repo, _ := setupAnimalRepo(t)

	animal := &models.Animal{
		UUID:        "uuid-create-1",
		DeviceID:    "device-1",
		Species:     "cat",
		Rarity:      3,
		GeneratedAt: time.Now(),
	}
	err := repo.Create(animal)
	assert.NoError(t, err)
	assert.Greater(t, animal.ID, uint(0))

	found, err := repo.FindByUUID("uuid-create-1")
	assert.NoError(t, err)
	assert.Equal(t, "cat", found.Species)
	assert.Equal(t, 3, found.Rarity)
}

func TestAnimalRepo_NotExists(t *testing.T) {
	repo, _ := setupAnimalRepo(t)

	assert.False(t, repo.ExistsByUUID("no-such-uuid"))

	_, err := repo.FindByUUID("no-such-uuid")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestAnimalRepo_DuplicateUUID(t *testing.T) {
	repo, _ := setupAnimalRepo(t)

	animal := &models.Animal{
		UUID:        "uuid-dup",
		DeviceID:    "device-1",
		Species:     "dog",
		Rarity:      2,
		GeneratedAt: time.Now(),
	}
	assert.NoError(t, repo.Create(animal))
	assert.True(t, repo.ExistsByUUID("uuid-dup"))

	// 重复创建应报错
	err := repo.Create(animal)
	assert.Error(t, err)
}

func TestAnimalRepo_CountRecentHighRarity(t *testing.T) {
	repo, _ := setupAnimalRepo(t)

	now := time.Now()
	// 插入 2 只高稀有度
	for i := 0; i < 2; i++ {
		repo.Create(&models.Animal{
			UUID:        "high-" + string(rune('a'+i)),
			DeviceID:    "device-hr",
			Rarity:      5,
			GeneratedAt: now,
		})
	}

	count, err := repo.CountRecentHighRarity("device-hr", 5, now.Add(-time.Hour))
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestAnimalRepo_CountRecentHighRarity_OutOfRange(t *testing.T) {
	repo, _ := setupAnimalRepo(t)

	repo.Create(&models.Animal{
		UUID:        "old-one",
		DeviceID:    "device-old",
		Rarity:      5,
		GeneratedAt: time.Now().Add(-2 * time.Hour),
	})

	// 只查最近 5 分钟
	count, err := repo.CountRecentHighRarity("device-old", 5, time.Now().Add(-5*time.Minute))
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestAnimalRepo_FindByInferenceRequestID(t *testing.T) {
	repo, _ := setupAnimalRepo(t)

	repo.Create(&models.Animal{
		UUID:               "uuid-req",
		DeviceID:           "device-1",
		InferenceRequestID: "req-123",
		GeneratedAt:        time.Now(),
	})

	animals, err := repo.FindByInferenceRequestID("req-123")
	assert.NoError(t, err)
	assert.Len(t, animals, 1)
	assert.Equal(t, "uuid-req", animals[0].UUID)

	// 不存在的 ID 返回空
	animals, err = repo.FindByInferenceRequestID("nonexistent")
	assert.NoError(t, err)
	assert.Len(t, animals, 0)
}

func TestAuditLogRepo_Create(t *testing.T) {
	_, auditRepo := setupAnimalRepo(t)

	log := &models.AuditLog{
		DeviceID:           "device-1",
		Type:               "sync",
		Message:            "测试审计",
		InferenceRequestID: "req-audit",
	}
	err := auditRepo.Create(log)
	assert.NoError(t, err)
	assert.Greater(t, log.ID, uint(0))
}
