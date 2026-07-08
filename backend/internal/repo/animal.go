// Package repo MB4: 动物同步仓储操作。
package repo

import (
	"time"

	"animalpoke/backend/internal/models"

	"gorm.io/gorm"
)

// AnimalRepo 动物仓储。
type AnimalRepo struct {
	db *gorm.DB
}

// NewAnimalRepo 构造 AnimalRepo。
func NewAnimalRepo(db *gorm.DB) *AnimalRepo {
	return &AnimalRepo{db: db}
}

// Create 插入动物记录。UUID 唯一约束违反时返回 gorm.ErrDuplicatedKey。
func (r *AnimalRepo) Create(animal *models.Animal) error {
	return r.db.Create(animal).Error
}

// FindByUUID 按 UUID 查找动物。
func (r *AnimalRepo) FindByUUID(uuid string) (*models.Animal, error) {
	var animal models.Animal
	err := r.db.Where("uuid = ?", uuid).First(&animal).Error
	if err != nil {
		return nil, err
	}
	return &animal, nil
}

// ExistsByUUID 检查 UUID 是否已存在(去重)。
func (r *AnimalRepo) ExistsByUUID(uuid string) bool {
	var count int64
	r.db.Model(&models.Animal{}).Where("uuid = ?", uuid).Count(&count)
	return count > 0
}

// CountRecentHighRarity 统计某设备最近 duration 内的高稀有度动物数量(rarity >= minRarity)。
func (r *AnimalRepo) CountRecentHighRarity(deviceID string, minRarity int, since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.Animal{}).
		Where("device_id = ? AND rarity >= ? AND generated_at >= ?", deviceID, minRarity, since).
		Count(&count).Error
	return count, err
}

// FindByInferenceRequestID 按推理请求 ID 查找动物(用于反作弊校验)。
func (r *AnimalRepo) FindByInferenceRequestID(requestID string) ([]models.Animal, error) {
	var animals []models.Animal
	err := r.db.Where("inference_request_id = ?", requestID).Find(&animals).Error
	return animals, err
}

// AuditLogRepo 审计日志仓储。
type AuditLogRepo struct {
	db *gorm.DB
}

// NewAuditLogRepo 构造 AuditLogRepo。
func NewAuditLogRepo(db *gorm.DB) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

// Create 写入审计日志。
func (r *AuditLogRepo) Create(log *models.AuditLog) error {
	return r.db.Create(log).Error
}
