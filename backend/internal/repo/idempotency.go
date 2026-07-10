package repo

import (
	"errors"
	"time"

	"animalpoke/backend/internal/models"

	"gorm.io/gorm"
)

// IdempotencyRepo 幂等键仓储。
type IdempotencyRepo struct {
	db *gorm.DB
}

// NewIdempotencyRepo 构造。
func NewIdempotencyRepo(db *gorm.DB) *IdempotencyRepo {
	return &IdempotencyRepo{db: db}
}

// WithTx 事务绑定。
func (r *IdempotencyRepo) WithTx(tx *gorm.DB) *IdempotencyRepo {
	return &IdempotencyRepo{db: tx}
}

var (
	ErrIdempotencyConflict   = errors.New("idempotency_conflict")
	ErrIdempotencyInProgress = errors.New("idempotency_in_progress")
	ErrIdempotencyNotFound   = errors.New("idempotency not found")
)

// BeginOrGet 尝试创建 processing 记录；若已存在则返回现有记录。
// 返回 (record, created, error)
func (r *IdempotencyRepo) BeginOrGet(deviceID, route, key, requestHash string, ttl time.Duration) (*models.IdempotencyRecord, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, errors.New("idempotency repo unavailable")
	}
	now := time.Now().UTC()

	var existing models.IdempotencyRecord
	err := r.db.Where("device_id = ? AND route = ? AND key_name = ?", deviceID, route, key).First(&existing).Error
	if err == nil {
		if !existing.ExpiresAt.IsZero() && now.After(existing.ExpiresAt) {
			_ = r.db.Delete(&existing).Error
		} else {
			return &existing, false, nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	rec := &models.IdempotencyRecord{
		DeviceID:    deviceID,
		Route:       route,
		Key:         key,
		RequestHash: requestHash,
		Status:      "processing",
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(ttl),
	}
	if err := r.db.Create(rec).Error; err != nil {
		// unique conflict — re-read
		var again models.IdempotencyRecord
		if e2 := r.db.Where("device_id = ? AND route = ? AND key_name = ?", deviceID, route, key).First(&again).Error; e2 == nil {
			return &again, false, nil
		}
		return nil, false, err
	}
	return rec, true, nil
}

// Complete 写入最终响应。
func (r *IdempotencyRepo) Complete(deviceID, route, key string, httpStatus int, body string, cacheable bool) error {
	now := time.Now().UTC()
	status := "completed"
	if httpStatus >= 500 {
		status = "failed"
	}
	if !cacheable {
		return r.db.Where("device_id = ? AND route = ? AND key_name = ?", deviceID, route, key).Delete(&models.IdempotencyRecord{}).Error
	}
	return r.db.Model(&models.IdempotencyRecord{}).
		Where("device_id = ? AND route = ? AND key_name = ?", deviceID, route, key).
		Updates(map[string]interface{}{
			"status":        status,
			"http_status":   httpStatus,
			"response_body": body,
			"completed_at":  now,
			"updated_at":    now,
		}).Error
}

// Delete 删除记录。
func (r *IdempotencyRepo) Delete(deviceID, route, key string) error {
	return r.db.Where("device_id = ? AND route = ? AND key_name = ?", deviceID, route, key).
		Delete(&models.IdempotencyRecord{}).Error
}
