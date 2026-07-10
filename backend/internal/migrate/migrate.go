// Package migrate 版本化 SQL/Schema 迁移（替代生产启动 AutoMigrate）。
package migrate

import (
	"fmt"
	"log/slog"
	"time"

	"animalpoke/backend/internal/models"

	"gorm.io/gorm"
)

// Version 当前 schema 版本。
const CurrentVersion = "0007_auth_device_secret"

// Apply 按版本顺序应用迁移。开发可用；生产建议由 Job 单独执行。
func Apply(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if err := db.AutoMigrate(&models.SchemaMigration{}); err != nil {
		return err
	}

	migrations := []struct {
		version string
		fn      func(*gorm.DB) error
	}{
		{"0001_init_core", migrate0001},
		{"0002_device_token_consent", migrate0002},
		{"0003_inference_provenance", migrate0003},
		{"0004_privacy_location", migrate0004},
		{"0005_commerce_privacy_inference", migrate0005},
		{"0006_inference_lineage", migrate0006},
		{"0007_auth_device_secret", migrate0007},
	}

	for _, m := range migrations {
		var existing models.SchemaMigration
		err := db.Where("version = ?", m.version).First(&existing).Error
		if err == nil {
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		slog.Info("应用迁移", "version", m.version)
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := m.fn(tx); err != nil {
				return err
			}
			return tx.Create(&models.SchemaMigration{Version: m.version, AppliedAt: time.Now().UTC()}).Error
		}); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.version, err)
		}
	}
	return nil
}

// CheckVersion 校验 schema 是否至少到达指定版本（应用账号只读校验）。
func CheckVersion(db *gorm.DB, minVersion string) error {
	var count int64
	if err := db.Model(&models.SchemaMigration{}).Where("version = ?", minVersion).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("schema version %s not applied", minVersion)
	}
	return nil
}

func migrate0001(db *gorm.DB) error {
	return db.AutoMigrate(&models.Device{}, &models.Animal{}, &models.AuditLog{})
}

func migrate0002(db *gorm.DB) error {
	return db.AutoMigrate(&models.Device{})
}

func migrate0003(db *gorm.DB) error {
	return db.AutoMigrate(&models.Inference{})
}

func migrate0004(db *gorm.DB) error {
	return db.AutoMigrate(&models.Animal{}, &models.DataRequest{}, &models.SecurityReport{})
}

func migrate0005(db *gorm.DB) error {
	return db.AutoMigrate(&models.Product{}, &models.Order{}, &models.Entitlement{}, &models.AuditLog{})
}

func migrate0006(db *gorm.DB) error {
	return db.AutoMigrate(&models.Inference{})
}

func migrate0007(db *gorm.DB) error {
	return db.AutoMigrate(&models.Device{})
}
