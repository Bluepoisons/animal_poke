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

func TestMigrate0028_ReplacesJournalNodesWithExpeditionMemories(t *testing.T) {
	db := setupMigrateDB(t)
	require.NoError(t, migrate0021(db))
	require.NoError(t, db.Create(&models.CompanionMemoryNode{
		AnimalUUID: "animal-1", NodeID: "shared_journal", OwnerKey: "dev:device-1",
		Title: "共同观察日记", Kind: "journal", Visible: true, UnlockAtXP: 10,
	}).Error)
	require.NoError(t, db.Create(&models.GrowthEvent{
		EventID: "event-1", OwnerKey: "dev:device-1", DeviceID: "device-1",
		Kind: models.GrowthEventCompanionMemory, NodeID: "shared_journal",
		ConfigVersion: models.GrowthConfigVersion,
	}).Error)

	require.NoError(t, migrate0028(db))

	var node models.CompanionMemoryNode
	require.NoError(t, db.Where("animal_uuid = ?", "animal-1").First(&node).Error)
	assert.Equal(t, "first_expedition", node.NodeID)
	assert.Equal(t, "共同远征印记", node.Title)
	assert.Equal(t, "memory", node.Kind)
	var event models.GrowthEvent
	require.NoError(t, db.Where("event_id = ?", "event-1").First(&event).Error)
	assert.Equal(t, "first_expedition", event.NodeID)
}

func TestMigrate0028_MergesWhenExpeditionNodeAlreadyExists(t *testing.T) {
	db := setupMigrateDB(t)
	require.NoError(t, migrate0021(db))
	unlockedAt := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Create(&models.CompanionMemoryNode{
		AnimalUUID: "animal-existing", NodeID: "shared_journal", OwnerKey: "dev:device-1",
		Title: "旧手账节点", Kind: "journal", Visible: true, Unlocked: true,
		UnlockedAt: &unlockedAt, UnlockAtXP: 8,
	}).Error)
	require.NoError(t, db.Create(&models.CompanionMemoryNode{
		AnimalUUID: "animal-existing", NodeID: "first_expedition", OwnerKey: "dev:device-1",
		Title: "已经存在的远征印记", Kind: "memory", Visible: false, Unlocked: false, UnlockAtXP: 10,
	}).Error)
	require.NoError(t, db.Create(&models.GrowthEvent{
		EventID: "event-existing", OwnerKey: "dev:device-1", DeviceID: "device-1",
		Kind: models.GrowthEventCompanionMemory, NodeID: "shared_journal",
		ConfigVersion: models.GrowthConfigVersion,
	}).Error)

	require.NoError(t, migrate0028(db))

	var nodes []models.CompanionMemoryNode
	require.NoError(t, db.Where("animal_uuid = ?", "animal-existing").Find(&nodes).Error)
	require.Len(t, nodes, 1)
	assert.Equal(t, "first_expedition", nodes[0].NodeID)
	assert.Equal(t, "已经存在的远征印记", nodes[0].Title)
	assert.True(t, nodes[0].Visible)
	assert.True(t, nodes[0].Unlocked)
	require.NotNil(t, nodes[0].UnlockedAt)
	assert.WithinDuration(t, unlockedAt, *nodes[0].UnlockedAt, time.Second)
	assert.EqualValues(t, 8, nodes[0].UnlockAtXP)
	var event models.GrowthEvent
	require.NoError(t, db.Where("event_id = ?", "event-existing").First(&event).Error)
	assert.Equal(t, "first_expedition", event.NodeID)
}

func TestMigrate0029_DropsLegacySpeciesCheck(t *testing.T) {
	db := setupMigrateDB(t)
	require.NoError(t, db.Exec(`
		CREATE TABLE animals (
			id integer PRIMARY KEY AUTOINCREMENT,
			uuid text NOT NULL,
			device_id text NOT NULL,
			species text NOT NULL,
			rarity integer NOT NULL,
			generated_at datetime NOT NULL,
			CONSTRAINT chk_animals_species CHECK (species IN ('cat','dog','goose'))
		)
	`).Error)
	require.True(t, db.Migrator().HasConstraint(&models.Animal{}, "chk_animals_species"))
	require.Error(t, db.Exec(`INSERT INTO animals (uuid, device_id, species, rarity, generated_at) VALUES ('before', 'dev', 'horse', 1, CURRENT_TIMESTAMP)`).Error)

	require.NoError(t, migrate0029(db))
	assert.False(t, db.Migrator().HasConstraint(&models.Animal{}, "chk_animals_species"))
	require.NoError(t, db.Exec(`INSERT INTO animals (uuid, device_id, species, rarity, generated_at) VALUES ('after', 'dev', 'horse', 1, CURRENT_TIMESTAMP)`).Error)
}

func TestMigrate0030_AddsConcreteChineseSpeciesLabel(t *testing.T) {
	db := setupMigrateDB(t)
	require.NoError(t, Apply(db))
	assert.True(t, db.Migrator().HasColumn(&models.Animal{}, "species_label_zh"))

	animal := models.Animal{
		UUID: "550e8400-e29b-41d4-a716-446655440030", DeviceID: "dev-species-label",
		Species: "other_animal", SpeciesLabelZH: "赤狐", Breed: "赤狐",
		Rarity: 1, GeneratedAt: time.Now().UTC(), ServerVersion: 1,
	}
	require.NoError(t, db.Create(&animal).Error)

	var stored models.Animal
	require.NoError(t, db.Where("uuid = ?", animal.UUID).First(&stored).Error)
	assert.Equal(t, "赤狐", stored.SpeciesLabelZH)
}
