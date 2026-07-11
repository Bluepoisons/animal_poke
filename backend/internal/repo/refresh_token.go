// Package repo 刷新令牌族（AP-078）：签发、rotate-on-use、重用检测、绝对/空闲过期。
package repo

import (
	"errors"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DefaultRefreshAbsoluteTTL = 30 * 24 * time.Hour
	DefaultRefreshIdleTTL     = 7 * 24 * time.Hour
	// RefreshReuseGrace 并发刷新/响应丢失宽限：此窗口内已 rotated 令牌再次出现视为竞态，不整族吊销。
	RefreshReuseGrace = 15 * time.Second
)

// 刷新相关错误（可判定 reason_code）。
var (
	ErrRefreshInvalid   = errors.New("refresh token invalid")
	ErrRefreshExpired   = errors.New("refresh token expired")
	ErrRefreshRevoked   = errors.New("refresh token revoked")
	ErrRefreshReused    = errors.New("refresh token reused")
	ErrRefreshConflict  = errors.New("refresh token conflict")
	ErrRefreshNoAccount = errors.New("refresh requires bound account")
)

// RefreshPolicy 绝对/空闲过期策略。
type RefreshPolicy struct {
	AbsoluteTTL time.Duration
	IdleTTL     time.Duration
}

// Normalize 填充默认 TTL。
func (p RefreshPolicy) Normalize() RefreshPolicy {
	if p.AbsoluteTTL <= 0 {
		p.AbsoluteTTL = DefaultRefreshAbsoluteTTL
	}
	if p.IdleTTL <= 0 {
		p.IdleTTL = DefaultRefreshIdleTTL
	}
	return p
}

// IssueRefreshResult 新签发的 refresh 明文 + 元数据。
type IssueRefreshResult struct {
	Plain     string
	TokenID   string
	FamilyID  string
	ExpiresAt time.Time // 取 absolute 与 idle 中较早者，便于客户端展示
}

// RotateRefreshResult 轮换结果。
type RotateRefreshResult struct {
	Plain     string
	TokenID   string
	FamilyID  string
	AccountID string
	DeviceID  string
	ExpiresAt time.Time
}

// IssueRefreshFamily 为设备签发新的 refresh family（登录/绑定）。
// 会撤销该设备上既有 active family，并同步 DeviceAccount.refresh_token_hash。
func (r *AccountRepo) IssueRefreshFamily(deviceID, accountID string, policy RefreshPolicy) (*IssueRefreshResult, error) {
	return r.issueRefreshFamilyTx(r.db, deviceID, accountID, policy)
}

func (r *AccountRepo) issueRefreshFamilyTx(tx *gorm.DB, deviceID, accountID string, policy RefreshPolicy) (*IssueRefreshResult, error) {
	if deviceID == "" || accountID == "" {
		return nil, ErrRefreshNoAccount
	}
	policy = policy.Normalize()
	now := time.Now().UTC()

	// 撤销该设备既有未吊销令牌（换绑/重登开启新 family）
	if err := tx.Model(&models.RefreshToken{}).
		Where("device_id = ? AND status IN ?", deviceID, []string{"active", "rotated"}).
		Updates(map[string]interface{}{
			"status":     "revoked",
			"revoked_at": now,
		}).Error; err != nil {
		return nil, err
	}

	plain := newRefreshPlain()
	hash := r.HashToken(plain)
	familyID := uuid.NewString()
	tokenID := uuid.NewString()
	absExp := now.Add(policy.AbsoluteTTL)
	idleExp := now.Add(policy.IdleTTL)

	row := &models.RefreshToken{
		TokenID:           tokenID,
		FamilyID:          familyID,
		AccountID:         accountID,
		DeviceID:          deviceID,
		TokenHash:         hash,
		Status:            "active",
		AbsoluteExpiresAt: absExp,
		IdleExpiresAt:     idleExp,
		LastUsedAt:        &now,
	}
	if err := tx.Create(row).Error; err != nil {
		return nil, err
	}

	// 同步 DeviceAccount 当前哈希（兼容旧读取路径）
	if err := tx.Model(&models.DeviceAccount{}).
		Where("device_id = ?", deviceID).
		Updates(map[string]interface{}{
			"refresh_token_hash": hash,
			"refresh_expires_at": absExp,
			"last_seen_at":       now,
		}).Error; err != nil {
		return nil, err
	}

	exp := absExp
	if idleExp.Before(exp) {
		exp = idleExp
	}
	return &IssueRefreshResult{
		Plain:     plain,
		TokenID:   tokenID,
		FamilyID:  familyID,
		ExpiresAt: exp,
	}, nil
}

// RotateRefresh rotate-on-use：消费当前 active refresh，签发同族新令牌。
// 并发：仅一行能将 status active→rotated；失败者得 ErrRefreshConflict。
// 重用：宽限外再次使用已 rotated 令牌 → 整族吊销 + ErrRefreshReused。
func (r *AccountRepo) RotateRefresh(plain string, policy RefreshPolicy) (*RotateRefreshResult, error) {
	if plain == "" {
		return nil, ErrRefreshInvalid
	}
	policy = policy.Normalize()

	var out *RotateRefreshResult
	// 语义错误在事务提交后再返回，避免 revoke 被 rollback（重用/过期吊销必须落库）
	var resultErr error
	err := r.db.Transaction(func(tx *gorm.DB) error {
		ar := r.WithTx(tx)
		now := time.Now().UTC()

		row, err := ar.findRefreshByPlainTx(tx, plain)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				resultErr = ErrRefreshInvalid
				return nil
			}
			return err
		}

		switch row.Status {
		case "revoked":
			resultErr = ErrRefreshRevoked
			return nil
		case "rotated":
			// 宽限内视为并发竞态；宽限外视为重用攻击
			if row.RotatedAt != nil && now.Sub(*row.RotatedAt) <= RefreshReuseGrace {
				resultErr = ErrRefreshConflict
				return nil
			}
			if err := ar.revokeFamilyTx(tx, row.FamilyID, row.DeviceID, now); err != nil {
				return err
			}
			resultErr = ErrRefreshReused
			return nil
		case "active":
			// continue
		default:
			resultErr = ErrRefreshInvalid
			return nil
		}

		// 过期检查
		if now.After(row.AbsoluteExpiresAt) || now.After(row.IdleExpiresAt) {
			if err := ar.revokeFamilyTx(tx, row.FamilyID, row.DeviceID, now); err != nil {
				return err
			}
			resultErr = ErrRefreshExpired
			return nil
		}

		// 设备/账号状态
		var da models.DeviceAccount
		if err := tx.Where("device_id = ? AND account_id = ?", row.DeviceID, row.AccountID).First(&da).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if rerr := ar.revokeFamilyTx(tx, row.FamilyID, row.DeviceID, now); rerr != nil {
					return rerr
				}
				resultErr = ErrRefreshInvalid
				return nil
			}
			return err
		}
		if da.Status != "active" {
			if rerr := ar.revokeFamilyTx(tx, row.FamilyID, row.DeviceID, now); rerr != nil {
				return rerr
			}
			resultErr = ErrDeviceRevoked
			return nil
		}
		var dev models.Device
		if err := tx.Where("device_id = ?", row.DeviceID).First(&dev).Error; err != nil {
			return err
		}
		if dev.Disabled {
			if rerr := ar.revokeFamilyTx(tx, row.FamilyID, row.DeviceID, now); rerr != nil {
				return rerr
			}
			resultErr = ErrDeviceDisabled
			return nil
		}
		if _, err := ar.EnsureAccountActive(row.AccountID); err != nil {
			if rerr := ar.revokeFamilyTx(tx, row.FamilyID, row.DeviceID, now); rerr != nil {
				return rerr
			}
			resultErr = err
			return nil
		}

		// 原子占位：仅 active 可转 rotated
		res := tx.Model(&models.RefreshToken{}).
			Where("id = ? AND status = ?", row.ID, "active").
			Updates(map[string]interface{}{
				"status":       "rotated",
				"rotated_at":   now,
				"last_used_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 并发抢占失败
			resultErr = ErrRefreshConflict
			return nil
		}

		newPlain := newRefreshPlain()
		newHash := ar.HashToken(newPlain)
		newID := uuid.NewString()
		idleExp := now.Add(policy.IdleTTL)
		// 绝对过期继承 family 起点（父令牌上的 AbsoluteExpiresAt 已固定）
		absExp := row.AbsoluteExpiresAt
		if idleExp.After(absExp) {
			// idle 不得越过 absolute
			idleExp = absExp
		}
		child := &models.RefreshToken{
			TokenID:           newID,
			FamilyID:          row.FamilyID,
			AccountID:         row.AccountID,
			DeviceID:          row.DeviceID,
			TokenHash:         newHash,
			ParentTokenID:     row.TokenID,
			Status:            "active",
			AbsoluteExpiresAt: absExp,
			IdleExpiresAt:     idleExp,
			LastUsedAt:        &now,
		}
		if err := tx.Create(child).Error; err != nil {
			return err
		}

		if err := tx.Model(&models.DeviceAccount{}).
			Where("device_id = ?", row.DeviceID).
			Updates(map[string]interface{}{
				"refresh_token_hash": newHash,
				"refresh_expires_at": absExp,
				"last_seen_at":       now,
			}).Error; err != nil {
			return err
		}

		exp := absExp
		if idleExp.Before(exp) {
			exp = idleExp
		}
		out = &RotateRefreshResult{
			Plain:     newPlain,
			TokenID:   newID,
			FamilyID:  row.FamilyID,
			AccountID: row.AccountID,
			DeviceID:  row.DeviceID,
			ExpiresAt: exp,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if resultErr != nil {
		return nil, resultErr
	}
	return out, nil
}

// RevokeRefreshFamiliesForDevice 吊销设备全部 refresh family（登出/撤销/删除）。
func (r *AccountRepo) RevokeRefreshFamiliesForDevice(deviceID string) error {
	now := time.Now().UTC()
	return r.db.Transaction(func(tx *gorm.DB) error {
		return r.WithTx(tx).revokeDeviceRefreshTx(tx, deviceID, now)
	})
}

// RevokeRefreshFamiliesForAccount 吊销账号全部 refresh（改密/账号禁用预留）。
func (r *AccountRepo) RevokeRefreshFamiliesForAccount(accountID string) error {
	now := time.Now().UTC()
	return r.db.Model(&models.RefreshToken{}).
		Where("account_id = ? AND status IN ?", accountID, []string{"active", "rotated"}).
		Updates(map[string]interface{}{
			"status":     "revoked",
			"revoked_at": now,
		}).Error
}

func (r *AccountRepo) revokeDeviceRefreshTx(tx *gorm.DB, deviceID string, now time.Time) error {
	if err := tx.Model(&models.RefreshToken{}).
		Where("device_id = ? AND status IN ?", deviceID, []string{"active", "rotated"}).
		Updates(map[string]interface{}{
			"status":     "revoked",
			"revoked_at": now,
		}).Error; err != nil {
		return err
	}
	return tx.Model(&models.DeviceAccount{}).
		Where("device_id = ?", deviceID).
		Updates(map[string]interface{}{
			"refresh_token_hash": "",
			"refresh_expires_at": nil,
		}).Error
}

func (r *AccountRepo) revokeFamilyTx(tx *gorm.DB, familyID, deviceID string, now time.Time) error {
	if err := tx.Model(&models.RefreshToken{}).
		Where("family_id = ? AND status IN ?", familyID, []string{"active", "rotated"}).
		Updates(map[string]interface{}{
			"status":     "revoked",
			"revoked_at": now,
		}).Error; err != nil {
		return err
	}
	// 吊销 access：bump token_version
	if deviceID != "" {
		if err := tx.Model(&models.Device{}).Where("device_id = ?", deviceID).
			Update("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.DeviceAccount{}).
			Where("device_id = ?", deviceID).
			Updates(map[string]interface{}{
				"refresh_token_hash": "",
				"refresh_expires_at": nil,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *AccountRepo) findRefreshByPlainTx(tx *gorm.DB, plain string) (*models.RefreshToken, error) {
	// 当前 pepper（并发安全依赖后续 active→rotated 原子更新，不依赖 FOR UPDATE 以便 SQLite 测试）
	hash := r.hashTokenWith(r.pepper, plain)
	var row models.RefreshToken
	err := tx.Where("token_hash = ?", hash).First(&row).Error
	if err == nil {
		return &row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// previous pepper 双读
	if r.pepperPrevious != "" {
		prev := r.hashTokenWith(r.pepperPrevious, plain)
		err = tx.Where("token_hash = ?", prev).First(&row).Error
		if err == nil {
			return &row, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func newRefreshPlain() string {
	// 64 hex chars entropic refresh
	return uuid.NewString() + uuid.NewString()
}
