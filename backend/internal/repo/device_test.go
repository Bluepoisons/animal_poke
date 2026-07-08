package repo

import (
	"testing"

	"animalpoke/backend/internal/models"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDeviceRepo(t *testing.T) *DeviceRepo {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	err = db.AutoMigrate(&models.Device{})
	assert.NoError(t, err)
	return NewDeviceRepo(db)
}

func TestDeviceRepo_FindOrCreate_New(t *testing.T) {
	repo := setupDeviceRepo(t)

	dev, err := repo.FindOrCreate("device-new")
	assert.NoError(t, err)
	assert.NotNil(t, dev)
	assert.Equal(t, "device-new", dev.DeviceID)
	assert.Greater(t, dev.ID, uint(0))
}

func TestDeviceRepo_FindOrCreate_Existing(t *testing.T) {
	repo := setupDeviceRepo(t)

	// 第一次创建
	dev1, _ := repo.FindOrCreate("device-exist")
	// 第二次查找
	dev2, err := repo.FindOrCreate("device-exist")
	assert.NoError(t, err)
	assert.Equal(t, dev1.ID, dev2.ID)
	assert.Equal(t, dev1.DeviceID, dev2.DeviceID)
}

func TestDeviceRepo_Exists(t *testing.T) {
	repo := setupDeviceRepo(t)

	assert.False(t, repo.Exists("no-such-device"))
	repo.FindOrCreate("device-abc")
	assert.True(t, repo.Exists("device-abc"))
}

func TestDeviceRepo_MultipleDevices(t *testing.T) {
	repo := setupDeviceRepo(t)

	ids := []string{"d1", "d2", "d3"}
	for _, id := range ids {
		dev, err := repo.FindOrCreate(id)
		assert.NoError(t, err)
		assert.Equal(t, id, dev.DeviceID)
	}

	for _, id := range ids {
		assert.True(t, repo.Exists(id))
	}
}
