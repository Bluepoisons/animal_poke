// Package repo 设备相关数据库操作。
package repo

import (
	"animalpoke/backend/internal/models"

	"gorm.io/gorm"
)

// DeviceRepo 设备仓储。
type DeviceRepo struct {
	db *gorm.DB
}

// NewDeviceRepo 构造 DeviceRepo。
func NewDeviceRepo(db *gorm.DB) *DeviceRepo {
	return &DeviceRepo{db: db}
}

// FindOrCreate 按 device_id 查找设备, 不存在则创建。
func (r *DeviceRepo) FindOrCreate(deviceID string) (*models.Device, error) {
	var dev models.Device
	err := r.db.Where("device_id = ?", deviceID).First(&dev).Error
	if err == nil {
		return &dev, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	dev = models.Device{DeviceID: deviceID}
	if err := r.db.Create(&dev).Error; err != nil {
		return nil, err
	}
	return &dev, nil
}

// Exists 检查 device_id 是否已注册。
func (r *DeviceRepo) Exists(deviceID string) bool {
	var count int64
	r.db.Model(&models.Device{}).Where("device_id = ?", deviceID).Count(&count)
	return count > 0
}
