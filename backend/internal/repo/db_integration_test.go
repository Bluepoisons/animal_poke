//go:build integration

// 集成测试: 需要 MySQL。先 `make db-up` 起 docker-compose MySQL, 再 `make test-integration`。
// 无法连接时自动 t.Skip, 不阻塞 CI。

package repo

import (
	"os"
	"strconv"
	"testing"

	"animalpoke/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// mysqlReachable 探测 MySQL 是否可连(InitDB 内部会 Ping)。关闭探测连接。
func mysqlReachable(cfg config.DatabaseConfig) bool {
	db, err := InitDB(cfg)
	if err != nil {
		return false
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	return true
}

func TestInitDB_ConnectAndPoolConfig(t *testing.T) {
	cfg := defaultDBConfig()

	if !mysqlReachable(cfg) {
		t.Skipf("MySQL 不可达(host=%s:%d), 跳过集成测试。请先 `make db-up`。", cfg.Host, cfg.Port)
	}

	db, err := InitDB(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	sqlDB, err := db.DB()
	require.NoError(t, err)

	// 连通性
	assert.NoError(t, sqlDB.Ping())

	// 连接池配置生效
	stats := sqlDB.Stats()
	assert.Equal(t, 25, stats.MaxOpenConnections, "MaxOpenConns 应为 25")
}

func TestInitDB_BadCredentials(t *testing.T) {
	probe := defaultDBConfig()
	if !mysqlReachable(probe) {
		t.Skipf("MySQL 不可达, 跳过")
	}

	cfg := defaultDBConfig()
	cfg.User = "definitely_nonexistent_user"
	cfg.Password = "wrong"

	_, err := InitDB(cfg)
	assert.Error(t, err, "错误凭据应返回 error")
}
