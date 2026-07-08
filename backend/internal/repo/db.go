package repo

import (
	"log/slog"

	"animalpoke/backend/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// InitDB 连接 MySQL 并做连接池配置与连通性校验。返回 *gorm.DB。
func InitDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	// 基础连接池设置
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	slog.Debug("数据库初始化完成")
	return db, nil
}
