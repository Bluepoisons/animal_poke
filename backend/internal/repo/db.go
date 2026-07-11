package repo

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"animalpoke/backend/internal/config"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDB 使用驱动原生 mysql.Config + NewConnector 连接 MySQL（特殊字符密码安全），
// 并配置连接池与连通性校验。
func InitDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	mysqlCfg, err := cfg.MySQLConfig()
	if err != nil {
		return nil, fmt.Errorf("mysql config: %w", err)
	}
	connector, err := mysqldriver.NewConnector(mysqlCfg)
	if err != nil {
		return nil, fmt.Errorf("mysql connector: %w", err)
	}
	sqlDB := sql.OpenDB(connector)

	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 25
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 10
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	} else {
		sqlDB.SetConnMaxLifetime(30 * time.Minute)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	} else {
		sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("mysql ping (%s): %w", config.ClassifyDBError(err), err)
	}

	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("gorm open (%s): %w", config.ClassifyDBError(err), err)
	}

	slog.Debug("数据库初始化完成",
		"max_open", maxOpen,
		"max_idle", maxIdle,
		"conn_max_lifetime", cfg.ConnMaxLifetime.String(),
		"tls_mode", config.NormalizeTLSMode(cfg.TLSMode),
	)
	return db, nil
}
