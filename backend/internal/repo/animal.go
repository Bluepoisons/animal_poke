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

// SoftDeleteByDevice 软删除设备全部动物，并提升 server_version 以便 pull 下发 tombstone。
// 同时清除精确坐标（隐私：删除后不得保留可恢复精确定位）。
func (r *AnimalRepo) SoftDeleteByDevice(deviceID string) error {
	now := time.Now().UTC()
	// base+id 保证同批软删仍有单调且互异的 server_version，避免游标卡死。
	base := now.UnixNano()
	return r.db.Model(&models.Animal{}).
		Where("device_id = ? AND deleted_at IS NULL", deviceID).
		Updates(map[string]interface{}{
			"deleted_at":         now,
			"server_version":     gorm.Expr("? + id", base),
			"precise_lat":        nil,
			"precise_lng":        nil,
			"precise_expires_at": nil,
		}).Error
}

// ListSinceVersion 按 server_version 游标拉取（设备作用域）。
func (r *AnimalRepo) ListSinceVersion(deviceID string, sinceVersion int64, limit int) ([]models.Animal, error) {
	return r.ListSinceVersionScoped(deviceID, "", sinceVersion, limit)
}

// ListSinceVersionScoped 按 device 或 account 拉取（绑定后跨设备恢复）。
func (r *AnimalRepo) ListSinceVersionScoped(deviceID, accountID string, sinceVersion int64, limit int) ([]models.Animal, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var animals []models.Animal
	q := r.db.Where("server_version > ?", sinceVersion)
	if accountID != "" {
		q = q.Where("(account_id = ? OR device_id = ?)", accountID, deviceID)
	} else {
		q = q.Where("device_id = ?", deviceID)
	}
	err := q.Order("server_version asc").Limit(limit).Find(&animals).Error
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

// Sentinel errors for inference consume / lineage.
var (
	ErrInferenceNotFound        = errors.New("inference not found")
	ErrInferenceAlreadyUsed     = errors.New("inference already consumed")
	ErrInferenceNotSuccess      = errors.New("inference not successful")
	ErrInferenceWrongKind       = errors.New("inference kind not allowed")
	ErrInferenceExpired         = errors.New("inference expired")
	ErrInferenceDeviceMismatch  = errors.New("inference device mismatch")
	ErrInferenceSpeciesMismatch = errors.New("inference species mismatch")
	ErrInferenceTampered        = errors.New("inference result mismatch")
)

// Consume 原子消费成功推理：条件 UPDATE status=success → consumed，要求 RowsAffected==1。
// 仅允许 kind=value 用于创建动物（detect/analyze 不可消费落库）。
func (r *InferenceRepo) Consume(tx *gorm.DB, inferenceID, deviceID string) (*models.Inference, error) {
	return r.ConsumeValue(tx, inferenceID, deviceID, "")
}

// ConsumeValue 原子消费 value 推理，可选校验 species。
func (r *InferenceRepo) ConsumeValue(tx *gorm.DB, inferenceID, deviceID, expectedSpecies string) (*models.Inference, error) {
	db := r.db
	if tx != nil {
		db = tx
	}
	if inferenceID == "" {
		return nil, ErrInferenceNotFound
	}
	now := time.Now().UTC()

	// 先读出权威结果（同事务内），再条件更新
	var inf models.Inference
	err := db.Where("inference_id = ? AND device_id = ?", inferenceID, deviceID).First(&inf).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInferenceNotFound
		}
		return nil, err
	}
	if inf.DeviceID != deviceID {
		return nil, ErrInferenceDeviceMismatch
	}
	if inf.Kind != "value" {
		return nil, ErrInferenceWrongKind
	}
	if inf.Status == "consumed" {
		return nil, ErrInferenceAlreadyUsed
	}
	if inf.Status != "success" {
		return nil, ErrInferenceNotSuccess
	}
	if inf.ExpiresAt != nil && !inf.ExpiresAt.IsZero() && now.After(*inf.ExpiresAt) {
		return nil, ErrInferenceExpired
	}
	if expectedSpecies != "" && inf.Species != "" && inf.Species != expectedSpecies {
		return nil, ErrInferenceSpeciesMismatch
	}

	res := db.Model(&models.Inference{}).
		Where("inference_id = ? AND device_id = ? AND status = ? AND kind = ?", inferenceID, deviceID, "success", "value").
		Updates(map[string]interface{}{
			"status":      "consumed",
			"consumed_at": now,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected != 1 {
		// 并发下另一请求已消费
		return nil, ErrInferenceAlreadyUsed
	}
	inf.Status = "consumed"
	inf.ConsumedAt = &now
	return &inf, nil
}

// FindForDevice 按设备作用域查找。
func (r *InferenceRepo) FindForDevice(inferenceID, deviceID string) (*models.Inference, error) {
	var inf models.Inference
	err := r.db.Where("inference_id = ? AND device_id = ?", inferenceID, deviceID).First(&inf).Error
	if err != nil {
		return nil, err
	}
	return &inf, nil
}

// SoftDeleteByDevice 删除设备推理（隐私删除）。
func (r *InferenceRepo) SoftDeleteByDevice(deviceID string) error {
	return r.db.Where("device_id = ?", deviceID).Delete(&models.Inference{}).Error
}
