// Package repo 设备相关数据库操作。
package repo

import (
	"errors"
	"strings"
	"time"

	"animalpoke/backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DeviceRepo 设备仓储。
type DeviceRepo struct {
	db *gorm.DB
}

// NewDeviceRepo 构造 DeviceRepo。
func NewDeviceRepo(db *gorm.DB) *DeviceRepo {
	return &DeviceRepo{db: db}
}

// WithTx 返回绑定事务的 DeviceRepo。
func (r *DeviceRepo) WithTx(tx *gorm.DB) *DeviceRepo {
	return &DeviceRepo{db: tx}
}

// FindOrCreate 按 device_id 查找设备, 不存在则创建（唯一约束并发安全）。
func (r *DeviceRepo) FindOrCreate(deviceID string) (*models.Device, error) {
	var dev models.Device
	err := r.db.Where("device_id = ?", deviceID).First(&dev).Error
	if err == nil {
		return &dev, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	dev = models.Device{DeviceID: deviceID, TokenVersion: 1}
	err = r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}},
		DoNothing: true,
	}).Create(&dev).Error
	if err != nil {
		return nil, err
	}
	// 冲突后重新读取
	if dev.ID == 0 {
		if err := r.db.Where("device_id = ?", deviceID).First(&dev).Error; err != nil {
			return nil, err
		}
	}
	return &dev, nil
}

// Find 按 device_id 查找。
func (r *DeviceRepo) Find(deviceID string) (*models.Device, error) {
	var dev models.Device
	err := r.db.Where("device_id = ?", deviceID).First(&dev).Error
	if err != nil {
		return nil, err
	}
	return &dev, nil
}

// Exists 检查 device_id 是否已注册。
func (r *DeviceRepo) Exists(deviceID string) bool {
	var count int64
	if err := r.db.Model(&models.Device{}).Where("device_id = ?", deviceID).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// IsDisabled 设备是否禁用。
func (r *DeviceRepo) IsDisabled(deviceID string) (bool, error) {
	var dev models.Device
	err := r.db.Select("disabled").Where("device_id = ?", deviceID).First(&dev).Error
	if err != nil {
		return false, err
	}
	return dev.Disabled, nil
}

// Disable 禁用设备并提升 token_version。
func (r *DeviceRepo) Disable(deviceID string) error {
	return r.db.Model(&models.Device{}).Where("device_id = ?", deviceID).
		Updates(map[string]interface{}{"disabled": true, "token_version": gorm.Expr("token_version + 1")}).Error
}

// BumpTokenVersion 吊销已有 Token。
func (r *DeviceRepo) BumpTokenVersion(deviceID string) error {
	return r.db.Model(&models.Device{}).Where("device_id = ?", deviceID).
		Update("token_version", gorm.Expr("token_version + 1")).Error
}

// UpdateConsent 更新授权。
func (r *DeviceRepo) UpdateConsent(deviceID, version, scope string, revoked bool) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"consent_version": version,
		"consent_scope":   scope,
	}
	if revoked {
		updates["consent_revoked_at"] = now
	} else {
		updates["consent_at"] = now
		updates["consent_revoked_at"] = nil
	}
	return r.db.Model(&models.Device{}).Where("device_id = ?", deviceID).Updates(updates).Error
}

// ConsentHasScope 检查 scope 列表是否包含目标（逗号分隔）。
func ConsentHasScope(scopeCSV, want string) bool {
	for _, p := range strings.Split(scopeCSV, ",") {
		if strings.TrimSpace(p) == want {
			return true
		}
	}
	return false
}

// HasValidConsent 是否具备有效授权（版本匹配 + 未撤销 + 含 photo）。
func (r *DeviceRepo) HasValidConsent(deviceID, requiredVersion string) (bool, error) {
	dev, err := r.Find(deviceID)
	if err != nil {
		return false, err
	}
	if dev.ConsentRevoked != nil {
		return false, nil
	}
	if dev.ConsentVersion == "" {
		return false, nil
	}
	if requiredVersion != "" && dev.ConsentVersion != requiredVersion {
		return false, nil
	}
	// Vision 等敏感能力要求 photo scope；空 scope 视为旧数据不合格
	if !ConsentHasScope(dev.ConsentScope, "photo") {
		return false, nil
	}
	return true, nil
}

// HasConsentScope 任意 scope 查询。
func (r *DeviceRepo) HasConsentScope(deviceID, requiredVersion, scope string) (bool, error) {
	ok, err := r.HasValidConsent(deviceID, requiredVersion)
	if err != nil || !ok {
		// photo 失败时仍可能有 location；单独查
		dev, err2 := r.Find(deviceID)
		if err2 != nil {
			return false, err2
		}
		if dev.ConsentRevoked != nil {
			return false, nil
		}
		if requiredVersion != "" && dev.ConsentVersion != requiredVersion {
			return false, nil
		}
		return ConsentHasScope(dev.ConsentScope, scope), nil
	}
	if scope == "photo" {
		return true, nil
	}
	dev, err := r.Find(deviceID)
	if err != nil {
		return false, err
	}
	return ConsentHasScope(dev.ConsentScope, scope), nil
}
