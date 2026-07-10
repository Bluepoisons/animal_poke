package migrate

import (
	"strings"
	"testing"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMigrateDB(t *testing.T) *gorm.DB {
	t.Helper()
	// 独立 memory DSN，避免并行测试互相污染
	dsn := "file:migrate_" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestApply_IdempotentTwice(t *testing.T) {
	db := setupMigrateDB(t)

	require.NoError(t, Apply(db))
	require.NoError(t, Apply(db), "second Apply must be no-op / idempotent")

	var count int64
	require.NoError(t, db.Model(&models.SchemaMigration{}).Count(&count).Error)
	assert.Equal(t, int64(len(allMigrations())), count)

	require.NoError(t, CheckVersion(db, CurrentVersion))

	// 表应已创建，可插入合法动物
	animal := &models.Animal{
		UUID:        "uuid-migrate-1",
		DeviceID:    "dev-1",
		Species:     "cat",
		Rarity:      3,
		GeneratedAt: time.Now().UTC(),
	}
	require.NoError(t, db.Create(animal).Error)
}

func TestStatus_AfterApply(t *testing.T) {
	db := setupMigrateDB(t)

	before, err := Status(db)
	require.NoError(t, err)
	assert.Equal(t, CurrentVersion, before.Target)
	assert.Empty(t, before.Applied)
	assert.Len(t, before.Pending, len(allMigrations()))

	require.NoError(t, Apply(db))

	after, err := Status(db)
	require.NoError(t, err)
	assert.Len(t, after.Applied, len(allMigrations()))
	assert.Empty(t, after.Pending)
	assert.Contains(t, after.Applied, CurrentVersion)

	out := FormatStatus(after)
	assert.Contains(t, out, "target:")
	assert.Contains(t, out, CurrentVersion)
	assert.Contains(t, out, "(none — schema is current)")
	assert.True(t, strings.Contains(out, "[x]"))
}

func TestApply_NilDB(t *testing.T) {
	assert.Error(t, Apply(nil))
	_, err := Status(nil)
	assert.Error(t, err)
}

func TestCheckVersion_Missing(t *testing.T) {
	db := setupMigrateDB(t)
	require.NoError(t, db.AutoMigrate(&models.SchemaMigration{}))
	err := CheckVersion(db, CurrentVersion)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), CurrentVersion)
}
