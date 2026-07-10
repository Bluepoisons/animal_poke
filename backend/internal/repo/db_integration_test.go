//go:build integration

// 集成测试: 需要 MySQL。
//   make db-up && make test-integration
// 当 RUN_MYSQL_TESTS=1（CI）时，MySQL 不可达必须 fail，不得 skip。

package repo

import (
	"os"
	"strconv"
	"testing"
	"time"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/migrate"
	"animalpoke/backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func defaultDBConfig() config.DatabaseConfig {
	return config.DatabaseConfig{
		Host:     envStr("DB_HOST", "127.0.0.1"),
		Port:     envInt("DB_PORT", 3306),
		User:     envStr("DB_USER", "animal_poke"),
		Password: envStr("DB_PASSWORD", "animal_poke"),
		DBName:   envStr("DB_NAME", "animal_poke"),
	}
}

func requireMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	cfg := defaultDBConfig()
	db, err := InitDB(cfg)
	if err != nil {
		if os.Getenv("RUN_MYSQL_TESTS") == "1" {
			t.Fatalf("MySQL 不可达(host=%s:%d) 且 RUN_MYSQL_TESTS=1: %v", cfg.Host, cfg.Port, err)
		}
		t.Skipf("MySQL 不可达(host=%s:%d), 跳过集成测试。请先 `make db-up` 或设置 RUN_MYSQL_TESTS=1。", cfg.Host, cfg.Port)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestInitDB_ConnectAndPoolConfig(t *testing.T) {
	db := requireMySQL(t)

	sqlDB, err := db.DB()
	require.NoError(t, err)

	assert.NoError(t, sqlDB.Ping())

	stats := sqlDB.Stats()
	assert.Equal(t, 25, stats.MaxOpenConnections, "MaxOpenConns 应为 25")
}

func TestInitDB_BadCredentials(t *testing.T) {
	// 先确认基线可达；不可达时按 RUN_MYSQL_TESTS 策略 fail/skip
	_ = requireMySQL(t)

	cfg := defaultDBConfig()
	cfg.User = "definitely_nonexistent_user"
	cfg.Password = "wrong"

	_, err := InitDB(cfg)
	assert.Error(t, err, "错误凭据应返回 error")
}

func TestMigrate_ApplyIdempotentAndConstraints(t *testing.T) {
	db := requireMySQL(t)

	require.NoError(t, migrate.Apply(db))
	require.NoError(t, migrate.Apply(db), "second Apply must succeed idempotently")
	require.NoError(t, migrate.CheckVersion(db, migrate.CurrentVersion))

	st, err := migrate.Status(db)
	require.NoError(t, err)
	assert.Empty(t, st.Pending)
	assert.Contains(t, st.Applied, migrate.CurrentVersion)

	// 合法 device + animal
	devID := "itest-dev-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	require.NoError(t, db.Create(&models.Device{DeviceID: devID}).Error)

	okAnimal := &models.Animal{
		UUID:        "itest-animal-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		DeviceID:    devID,
		Species:     "cat",
		Rarity:      3,
		HP:          50,
		ATK:         20,
		DEF:         20,
		SPD:         20,
		GeneratedAt: time.Now().UTC(),
	}
	require.NoError(t, db.Create(okAnimal).Error)

	// 非法 rarity=99 → DB 拒绝
	badRarity := &models.Animal{
		UUID:        "itest-bad-rarity-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		DeviceID:    devID,
		Species:     "cat",
		Rarity:      99,
		GeneratedAt: time.Now().UTC(),
	}
	err = db.Create(badRarity).Error
	require.Error(t, err, "rarity=99 must be rejected by CHECK")

	// 非法 species → DB 拒绝
	badSpecies := &models.Animal{
		UUID:        "itest-bad-species-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		DeviceID:    devID,
		Species:     "dragon",
		Rarity:      3,
		GeneratedAt: time.Now().UTC(),
	}
	err = db.Create(badSpecies).Error
	require.Error(t, err, "illegal species must be rejected by CHECK")

	// 负金额 product → DB 拒绝
	badProduct := &models.Product{
		ProductID:  "itest-prod-neg-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Name:       "bad",
		Type:       "consumable",
		PriceCents: -1,
		Currency:   "CNY",
	}
	err = db.Create(badProduct).Error
	require.Error(t, err, "negative price_cents must be rejected")

	// 合法 product + 重复 inference 凭证
	prodID := "itest-prod-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	require.NoError(t, db.Create(&models.Product{
		ProductID:  prodID,
		Name:       "ok",
		Type:       "consumable",
		PriceCents: 100,
		Currency:   "CNY",
	}).Error)

	infID := "itest-inf-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	require.NoError(t, db.Create(&models.Inference{
		InferenceID: infID,
		DeviceID:    devID,
		Kind:        "value",
		Status:      "success",
		Species:     "cat",
	}).Error)
	// 重复 inference_id
	err = db.Create(&models.Inference{
		InferenceID: infID,
		DeviceID:    devID,
		Kind:        "value",
		Status:      "success",
		Species:     "cat",
	}).Error
	require.Error(t, err, "duplicate inference credential must be rejected")
}
