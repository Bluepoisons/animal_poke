// Package repo MB4: 动物同步仓储操作。
package repo

import (
	"errors"
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

// WithTx 返回绑定事务的 repo。
func (r *AnimalRepo) WithTx(tx *gorm.DB) *AnimalRepo {
	return &AnimalRepo{db: tx}
}

// DB 暴露底层 DB（事务用）。
func (r *AnimalRepo) DB() *gorm.DB { return r.db }

// Create 插入动物记录。
func (r *AnimalRepo) Create(animal *models.Animal) error {
	return r.db.Create(animal).Error
}

// FindByUUID 按 UUID 查找动物。
func (r *AnimalRepo) FindByUUID(uuid string) (*models.Animal, error) {
	var animal models.Animal
	err := r.db.Where("uuid = ? AND deleted_at IS NULL", uuid).First(&animal).Error
	if err != nil {
		return nil, err
	}
	return &animal, nil
}

// ExistsByUUID 检查 UUID 是否已存在；返回 (exists, error)。
func (r *AnimalRepo) ExistsByUUID(uuid string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Animal{}).Where("uuid = ?", uuid).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CountRecentHighRarity 统计某设备最近 since 之后的高稀有度动物数量（使用服务端 CreatedAt）。
func (r *AnimalRepo) CountRecentHighRarity(deviceID string, minRarity int, since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.Animal{}).
		Where("device_id = ? AND rarity >= ? AND created_at >= ? AND deleted_at IS NULL", deviceID, minRarity, since).
		Count(&count).Error
	return count, err
}

// FindByInferenceRequestID 按推理请求 ID 查找动物（设备作用域）。
func (r *AnimalRepo) FindByInferenceRequestID(deviceID, requestID string) ([]models.Animal, error) {
	var animals []models.Animal
	q := r.db.Where("inference_request_id = ?", requestID)
	if deviceID != "" {
		q = q.Where("device_id = ?", deviceID)
	}
	err := q.Find(&animals).Error
	return animals, err
}

// ListByDevice 分页列出设备动物。
func (r *AnimalRepo) ListByDevice(deviceID string, afterID uint, limit int) ([]models.Animal, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var animals []models.Animal
	q := r.db.Where("device_id = ? AND deleted_at IS NULL", deviceID)
	if afterID > 0 {
		q = q.Where("id > ?", afterID)
	}
	err := q.Order("id asc").Limit(limit).Find(&animals).Error
	return animals, err
}

// SoftDeleteByDevice 软删除设备全部动物。
func (r *AnimalRepo) SoftDeleteByDevice(deviceID string) error {
	now := time.Now().UTC()
	return r.db.Model(&models.Animal{}).Where("device_id = ? AND deleted_at IS NULL", deviceID).
		Update("deleted_at", now).Error
}

// ListSinceVersion 按 server_version 游标拉取。
func (r *AnimalRepo) ListSinceVersion(deviceID string, sinceVersion int64, limit int) ([]models.Animal, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var animals []models.Animal
	err := r.db.Where("device_id = ? AND server_version > ?", deviceID, sinceVersion).
		Order("server_version asc").Limit(limit).Find(&animals).Error
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

// WithTx 事务绑定。
func (r *AuditLogRepo) WithTx(tx *gorm.DB) *AuditLogRepo {
	return &AuditLogRepo{db: tx}
}

// Create 写入审计日志。
func (r *AuditLogRepo) Create(log *models.AuditLog) error {
	return r.db.Create(log).Error
}

// Query 分页检索。
func (r *AuditLogRepo) Query(deviceID, logType, status string, since, until *time.Time, offset, limit int) ([]models.AuditLog, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := r.db.Model(&models.AuditLog{})
	if deviceID != "" {
		q = q.Where("device_id = ?", deviceID)
	}
	if logType != "" {
		q = q.Where("type = ?", logType)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if since != nil {
		q = q.Where("created_at >= ?", *since)
	}
	if until != nil {
		q = q.Where("created_at <= ?", *until)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []models.AuditLog
	err := q.Order("created_at desc").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}

// Ack 处置告警。
func (r *AuditLogRepo) Ack(id uint, by string) error {
	now := time.Now().UTC()
	return r.db.Model(&models.AuditLog{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":   "ack",
		"acked_at": now,
		"acked_by": by,
	}).Error
}

// InferenceRepo 推理凭证仓储。
type InferenceRepo struct {
	db *gorm.DB
}

// NewInferenceRepo 构造。
func NewInferenceRepo(db *gorm.DB) *InferenceRepo {
	return &InferenceRepo{db: db}
}

// WithTx 事务绑定。
func (r *InferenceRepo) WithTx(tx *gorm.DB) *InferenceRepo {
	return &InferenceRepo{db: tx}
}

// Create 写入推理记录。
func (r *InferenceRepo) Create(inf *models.Inference) error {
	return r.db.Create(inf).Error
}

// Find 查找。
func (r *InferenceRepo) Find(inferenceID string) (*models.Inference, error) {
	var inf models.Inference
	err := r.db.Where("inference_id = ?", inferenceID).First(&inf).Error
	if err != nil {
		return nil, err
	}
	return &inf, nil
}

// Consume 一次性消费成功推理（幂等：已 consumed 返回错误）。
func (r *InferenceRepo) Consume(tx *gorm.DB, inferenceID, deviceID string) (*models.Inference, error) {
	db := r.db
	if tx != nil {
		db = tx
	}
	var inf models.Inference
	err := db.Where("inference_id = ? AND device_id = ?", inferenceID, deviceID).First(&inf).Error
	if err != nil {
		return nil, err
	}
	if inf.Status == "consumed" {
		return nil, errors.New("inference already consumed")
	}
	if inf.Status != "success" {
		return nil, errors.New("inference not successful")
	}
	now := time.Now().UTC()
	if err := db.Model(&inf).Updates(map[string]interface{}{
		"status":      "consumed",
		"consumed_at": now,
	}).Error; err != nil {
		return nil, err
	}
	inf.Status = "consumed"
	inf.ConsumedAt = &now
	return &inf, nil
}

// SoftDeleteByDevice 删除设备推理（隐私删除）。
func (r *InferenceRepo) SoftDeleteByDevice(deviceID string) error {
	return r.db.Where("device_id = ?", deviceID).Delete(&models.Inference{}).Error
}
