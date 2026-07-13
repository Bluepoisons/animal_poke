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
			return r.TryTakeover(&existing, requestHash, ttl)
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

// TryTakeover atomically replaces one observed record with a new processing
// lease. The observed status/timestamp/hash form a compare-and-swap token, so
// concurrent stale retries cannot both obtain execution rights.
func (r *IdempotencyRepo) TryTakeover(observed *models.IdempotencyRecord, requestHash string, ttl time.Duration) (*models.IdempotencyRecord, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, errors.New("idempotency repo unavailable")
	}
	if observed == nil || observed.ID == 0 {
		return nil, false, ErrIdempotencyNotFound
	}
	now := time.Now().UTC()
	res := r.db.Model(&models.IdempotencyRecord{}).
		Where("id = ? AND status = ? AND request_hash = ? AND updated_at = ?", observed.ID, observed.Status, observed.RequestHash, observed.UpdatedAt).
		Updates(map[string]interface{}{
			"request_hash":  requestHash,
			"status":        "processing",
			"http_status":   0,
			"response_body": "",
			"completed_at":  nil,
			"updated_at":    now,
			"expires_at":    now.Add(ttl),
		})
	if res.Error != nil {
		return nil, false, res.Error
	}
	var current models.IdempotencyRecord
	if err := r.db.Where("id = ?", observed.ID).First(&current).Error; err != nil {
		return nil, false, err
	}
	return &current, res.RowsAffected == 1, nil
}

// CompleteClaim writes the final response only while the caller still owns
// the processing lease. A timed-out worker cannot overwrite a newer takeover.
func (r *IdempotencyRepo) CompleteClaim(claim *models.IdempotencyRecord, httpStatus int, body string, cacheable bool) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("idempotency repo unavailable")
	}
	if claim == nil || claim.ID == 0 {
		return false, ErrIdempotencyNotFound
	}
	now := time.Now().UTC()
	status := "completed"
	if httpStatus >= 500 {
		status = "failed"
	}
	query := r.db.Where(
		"id = ? AND status = ? AND request_hash = ? AND updated_at = ?",
		claim.ID,
		"processing",
		claim.RequestHash,
		claim.UpdatedAt,
	)
	if !cacheable {
		res := query.Delete(&models.IdempotencyRecord{})
		return res.RowsAffected == 1, res.Error
	}
	res := query.Model(&models.IdempotencyRecord{}).
		Updates(map[string]interface{}{
			"status":        status,
			"http_status":   httpStatus,
			"response_body": body,
			"completed_at":  now,
			"updated_at":    now,
		})
	return res.RowsAffected == 1, res.Error
}

// Delete 删除记录。
func (r *IdempotencyRepo) Delete(deviceID, route, key string) error {
	return r.db.Where("device_id = ? AND route = ? AND key_name = ?", deviceID, route, key).
		Delete(&models.IdempotencyRecord{}).Error
}
