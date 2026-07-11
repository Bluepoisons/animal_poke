// Package repo MB4: 动物同步仓储操作。
package repo

import (
	"errors"
	"time"

	"animalpoke/backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// SoftDeleteByAccount 软删除账号归属的全部动物（AP-077）。
func (r *AnimalRepo) SoftDeleteByAccount(accountID string) error {
	if accountID == "" {
		return nil
	}
	now := time.Now().UTC()
	base := now.UnixNano()
	return r.db.Model(&models.Animal{}).
		Where("account_id = ? AND deleted_at IS NULL", accountID).
		Updates(map[string]interface{}{
			"deleted_at":         now,
			"server_version":     gorm.Expr("? + id", base),
			"precise_lat":        nil,
			"precise_lng":        nil,
			"precise_expires_at": nil,
		}).Error
}

// ClearExpiredPreciseLocation 物理清理已过期的精确坐标字段。
// 保留策略：精确经纬度仅短期保存（PreciseExpiresAt，默认同步时 24h）；
// 到期后置空，粗精度 city/geohash 可保留至用户发起删除；删除时一并清除精确字段。
// 可在删除事务内调用，也可由运维/定时任务周期性调用。
func (r *AnimalRepo) ClearExpiredPreciseLocation(now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return r.db.Model(&models.Animal{}).
		Where("precise_expires_at IS NOT NULL AND precise_expires_at < ?", now).
		Updates(map[string]interface{}{
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
	err := q.Order("server_version asc, id asc").Limit(limit).Find(&animals).Error
	if err != nil {
		return nil, err
	}
	for i := range animals {
		if animals[i].DeletedAt != nil {
			// tombstone：仅同步删除标记，禁止回传原内容/精确坐标
			animals[i] = models.Animal{
				UUID:          animals[i].UUID,
				DeletedAt:     animals[i].DeletedAt,
				ServerVersion: animals[i].ServerVersion,
			}
			continue
		}
		animals[i].PreciseLat = nil
		animals[i].PreciseLng = nil
		animals[i].PreciseExpiresAt = nil
	}
	return animals, nil
}

// 收藏项乐观锁 / 所有权相关哨兵错误。
var (
	ErrVersionConflict = errors.New("version_conflict")
	ErrAnimalNotFound  = errors.New("animal_not_found")
	ErrAnimalNotOwned  = errors.New("animal_not_owned")
	ErrAnimalLocked    = errors.New("animal_locked")
	ErrAlreadyDeleted  = errors.New("already_deleted")
)

// CollectionPatch 可编辑收藏元数据（部分更新）。
type CollectionPatch struct {
	Nickname *string
	Favorite *bool
	Locked   *bool
}

// FindByUUIDIncludingDeleted 按 UUID 查找（含软删 tombstone）。
func (r *AnimalRepo) FindByUUIDIncludingDeleted(uuid string) (*models.Animal, error) {
	var animal models.Animal
	err := r.db.Where("uuid = ?", uuid).First(&animal).Error
	if err != nil {
		return nil, err
	}
	return &animal, nil
}

// OwnsAnimal 设备或账号是否拥有该动物。
func OwnsAnimal(a *models.Animal, deviceID, accountID string) bool {
	if a == nil {
		return false
	}
	if deviceID != "" && a.DeviceID == deviceID {
		return true
	}
	if accountID != "" && a.AccountID != "" && a.AccountID == accountID {
		return true
	}
	return false
}

// ownershipClause 生成所有权过滤条件。
func ownershipClause(deviceID, accountID string) clause.Expr {
	if accountID != "" {
		return gorm.Expr("(device_id = ? OR account_id = ?)", deviceID, accountID)
	}
	return gorm.Expr("device_id = ?", deviceID)
}

// PatchCollection 乐观锁更新 nickname/favorite/locked。
// expectedVersion 必须等于当前 server_version；成功后提升版本。
func (r *AnimalRepo) PatchCollection(uuid, deviceID, accountID string, expectedVersion int64, patch CollectionPatch) (*models.Animal, error) {
	if uuid == "" {
		return nil, ErrAnimalNotFound
	}
	cur, err := r.FindByUUIDIncludingDeleted(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAnimalNotFound
		}
		return nil, err
	}
	if !OwnsAnimal(cur, deviceID, accountID) {
		// 不泄露存在性
		return nil, ErrAnimalNotFound
	}
	if cur.DeletedAt != nil {
		return cur, ErrAlreadyDeleted
	}
	if cur.ServerVersion != expectedVersion {
		return cur, ErrVersionConflict
	}

	updates := map[string]interface{}{
		"server_version": time.Now().UTC().UnixNano(),
	}
	if patch.Nickname != nil {
		updates["nickname"] = *patch.Nickname
	}
	if patch.Favorite != nil {
		updates["favorite"] = *patch.Favorite
	}
	if patch.Locked != nil {
		updates["locked"] = *patch.Locked
	}
	if len(updates) == 1 {
		// 仅版本字段，无实际变更
		return cur, nil
	}

	res := r.db.Model(&models.Animal{}).
		Where("uuid = ? AND deleted_at IS NULL AND server_version = ?", uuid, expectedVersion).
		Where(ownershipClause(deviceID, accountID)).
		Updates(updates)
	if res.Error != nil {
		// SQLite 并发锁：若版本已被他人推进则返回冲突，便于客户端合并
		if isBusyOrLocked(res.Error) {
			fresh, ferr := r.FindByUUIDIncludingDeleted(uuid)
			if ferr == nil && OwnsAnimal(fresh, deviceID, accountID) {
				if fresh.DeletedAt != nil {
					return fresh, ErrAlreadyDeleted
				}
				if fresh.ServerVersion != expectedVersion {
					return fresh, ErrVersionConflict
				}
			}
		}
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		// 并发写：重读当前版本
		fresh, ferr := r.FindByUUIDIncludingDeleted(uuid)
		if ferr != nil {
			if errors.Is(ferr, gorm.ErrRecordNotFound) {
				return nil, ErrAnimalNotFound
			}
			return nil, ferr
		}
		if !OwnsAnimal(fresh, deviceID, accountID) {
			return nil, ErrAnimalNotFound
		}
		if fresh.DeletedAt != nil {
			return fresh, ErrAlreadyDeleted
		}
		return fresh, ErrVersionConflict
	}
	return r.FindByUUID(uuid)
}

// SoftDeleteOne 单只软删 tombstone + 乐观锁；清除精确坐标。
// locked=true 时拒绝删除。
func (r *AnimalRepo) SoftDeleteOne(uuid, deviceID, accountID string, expectedVersion int64) (*models.Animal, error) {
	if uuid == "" {
		return nil, ErrAnimalNotFound
	}
	cur, err := r.FindByUUIDIncludingDeleted(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAnimalNotFound
		}
		return nil, err
	}
	if !OwnsAnimal(cur, deviceID, accountID) {
		return nil, ErrAnimalNotFound
	}
	if cur.DeletedAt != nil {
		return cur, ErrAlreadyDeleted
	}
	if cur.Locked {
		return cur, ErrAnimalLocked
	}
	if cur.ServerVersion != expectedVersion {
		return cur, ErrVersionConflict
	}

	now := time.Now().UTC()
	newVer := now.UnixNano()
	res := r.db.Model(&models.Animal{}).
		Where("uuid = ? AND deleted_at IS NULL AND server_version = ? AND locked = ?", uuid, expectedVersion, false).
		Where(ownershipClause(deviceID, accountID)).
		Updates(map[string]interface{}{
			"deleted_at":         now,
			"server_version":     newVer,
			"precise_lat":        nil,
			"precise_lng":        nil,
			"precise_expires_at": nil,
			// 内容脱敏：tombstone 不可恢复昵称等用户编辑字段
			"nickname": "",
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		fresh, ferr := r.FindByUUIDIncludingDeleted(uuid)
		if ferr != nil {
			if errors.Is(ferr, gorm.ErrRecordNotFound) {
				return nil, ErrAnimalNotFound
			}
			return nil, ferr
		}
		if !OwnsAnimal(fresh, deviceID, accountID) {
			return nil, ErrAnimalNotFound
		}
		if fresh.DeletedAt != nil {
			return fresh, ErrAlreadyDeleted
		}
		if fresh.Locked {
			return fresh, ErrAnimalLocked
		}
		return fresh, ErrVersionConflict
	}
	// 返回最小 tombstone
	return &models.Animal{
		UUID:          uuid,
		DeletedAt:     &now,
		ServerVersion: newVer,
	}, nil
}

func isBusyOrLocked(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return containsASCIIFold(s, "database is locked") ||
		containsASCIIFold(s, "database table is locked") ||
		containsASCIIFold(s, "busy")
}

func containsASCIIFold(s, sub string) bool {
	ls, lsub := len(s), len(sub)
	if lsub == 0 {
		return true
	}
	for i := 0; i+lsub <= ls; i++ {
		ok := true
		for j := 0; j < lsub; j++ {
			a, b := s[i+j], sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
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
