// Package repo 账号安全令牌（邮箱验证 / 找回密码 / re-auth，AP-079）。
package repo

import (
	"errors"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 安全令牌默认 TTL。
const (
	DefaultEmailVerifyTTL   = 24 * time.Hour
	DefaultPasswordResetTTL = time.Hour
	DefaultReauthTTL        = 5 * time.Minute
)

var (
	ErrSecurityTokenNotFound = errors.New("security token not found")
	ErrSecurityTokenUsed     = errors.New("security token already used")
	ErrSecurityTokenExpired  = errors.New("security token expired")
	ErrSecurityTokenPurpose  = errors.New("security token purpose mismatch")
	ErrLastRecoveryMethod    = errors.New("cannot remove last recovery method")
	ErrBindingUnverified     = errors.New("binding not verified")
	ErrEmailNotVerified      = errors.New("email not verified")
)

// CreateSecurityToken 签发一次性安全令牌（明文仅返回一次）。
func (r *AccountRepo) CreateSecurityToken(purpose, accountID, subject string, bindingID uint, ttl time.Duration) (plain string, row *models.AccountSecurityToken, err error) {
	if purpose == "" || accountID == "" {
		return "", nil, errors.New("purpose and account_id required")
	}
	if ttl <= 0 {
		switch purpose {
		case models.SecurityPurposeEmailVerify:
			ttl = DefaultEmailVerifyTTL
		case models.SecurityPurposePasswordReset:
			ttl = DefaultPasswordResetTTL
		case models.SecurityPurposeReauth:
			ttl = DefaultReauthTTL
		default:
			ttl = time.Hour
		}
	}
	plain = uuid.NewString() + uuid.NewString()
	now := time.Now().UTC()
	row = &models.AccountSecurityToken{
		TokenID:   uuid.NewString(),
		TokenHash: r.HashToken(plain),
		Purpose:   purpose,
		AccountID: accountID,
		Subject:   subject,
		BindingID: bindingID,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}
	if err := r.db.Create(row).Error; err != nil {
		return "", nil, err
	}
	return plain, row, nil
}

// InvalidateSecurityTokens 使某账号某用途未使用令牌全部失效（改密/重发时轮换）。
func (r *AccountRepo) InvalidateSecurityTokens(accountID, purpose string) error {
	now := time.Now().UTC()
	q := r.db.Model(&models.AccountSecurityToken{}).
		Where("account_id = ? AND used_at IS NULL", accountID)
	if purpose != "" {
		q = q.Where("purpose = ?", purpose)
	}
	return q.Update("used_at", now).Error
}

// ConsumeSecurityToken 原子消费安全令牌；校验 purpose 与可选 accountID。
func (r *AccountRepo) ConsumeSecurityToken(plain, purpose, accountID string) (*models.AccountSecurityToken, error) {
	if plain == "" {
		return nil, ErrSecurityTokenNotFound
	}
	var out *models.AccountSecurityToken
	err := r.db.Transaction(func(tx *gorm.DB) error {
		hashes := []string{r.HashToken(plain)}
		if r.pepperPrevious != "" {
			prev := r.hashTokenWith(r.pepperPrevious, plain)
			if prev != hashes[0] {
				hashes = append(hashes, prev)
			}
		}
		var t models.AccountSecurityToken
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash IN ?", hashes)
		if purpose != "" {
			q = q.Where("purpose = ?", purpose)
		}
		err := q.First(&t).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSecurityTokenNotFound
		}
		if err != nil {
			return err
		}
		if purpose != "" && t.Purpose != purpose {
			return ErrSecurityTokenPurpose
		}
		if accountID != "" && t.AccountID != accountID {
			return ErrSecurityTokenNotFound
		}
		if t.UsedAt != nil {
			return ErrSecurityTokenUsed
		}
		now := time.Now().UTC()
		if now.After(t.ExpiresAt) {
			return ErrSecurityTokenExpired
		}
		res := tx.Model(&models.AccountSecurityToken{}).
			Where("id = ? AND used_at IS NULL", t.ID).
			Update("used_at", now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrSecurityTokenUsed
		}
		t.UsedAt = &now
		out = &t
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PeekSecurityToken 只读校验（不消费），用于 reauth 中间件多次敏感操作窗口内检查。
func (r *AccountRepo) PeekSecurityToken(plain, purpose, accountID string) (*models.AccountSecurityToken, error) {
	if plain == "" {
		return nil, ErrSecurityTokenNotFound
	}
	hashes := []string{r.HashToken(plain)}
	if r.pepperPrevious != "" {
		prev := r.hashTokenWith(r.pepperPrevious, plain)
		if prev != hashes[0] {
			hashes = append(hashes, prev)
		}
	}
	var t models.AccountSecurityToken
	q := r.db.Where("token_hash IN ?", hashes)
	if purpose != "" {
		q = q.Where("purpose = ?", purpose)
	}
	if err := q.First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSecurityTokenNotFound
		}
		return nil, err
	}
	if accountID != "" && t.AccountID != accountID {
		return nil, ErrSecurityTokenNotFound
	}
	if t.UsedAt != nil {
		return nil, ErrSecurityTokenUsed
	}
	if time.Now().UTC().After(t.ExpiresAt) {
		return nil, ErrSecurityTokenExpired
	}
	return &t, nil
}

// MarkBindingVerified 标记绑定已验证。
func (r *AccountRepo) MarkBindingVerified(bindingID uint) error {
	now := time.Now().UTC()
	res := r.db.Model(&models.AccountBinding{}).
		Where("id = ?", bindingID).
		Updates(map[string]interface{}{
			"verified":    true,
			"verified_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrBindingNotFound
	}
	return nil
}

// FindBindingByID 按主键查找绑定。
func (r *AccountRepo) FindBindingByID(id uint) (*models.AccountBinding, error) {
	var b models.AccountBinding
	if err := r.db.Where("id = ?", id).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// UpdateBindingPassword 更新邮箱绑定密码哈希。
func (r *AccountRepo) UpdateBindingPassword(bindingID uint, passwordHash string) error {
	return r.db.Model(&models.AccountBinding{}).
		Where("id = ? AND provider = ?", bindingID, "email").
		Update("credential_hash", passwordHash).Error
}

// CountRecoveryMethods 统计可用于恢复的绑定数（已验证 email 或 OAuth）。
func (r *AccountRepo) CountRecoveryMethods(accountID string) (int, error) {
	var n int64
	err := r.db.Model(&models.AccountBinding{}).
		Where("account_id = ?", accountID).
		Where("(provider = ? AND verified = ?) OR provider <> ?", "email", true, "email").
		Count(&n).Error
	return int(n), err
}

// DeleteBinding 删除绑定；禁止移除最后一种恢复方式。
func (r *AccountRepo) DeleteBinding(accountID, provider, subject string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var b models.AccountBinding
		err := tx.Where("account_id = ? AND provider = ? AND provider_subject = ?", accountID, provider, subject).
			First(&b).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrBindingNotFound
		}
		if err != nil {
			return err
		}
		// 未验证 email 不是恢复方式，可直接删
		isRecovery := b.Provider != "email" || b.Verified
		if isRecovery {
			var n int64
			if err := tx.Model(&models.AccountBinding{}).
				Where("account_id = ?", accountID).
				Where("(provider = ? AND verified = ?) OR provider <> ?", "email", true, "email").
				Count(&n).Error; err != nil {
				return err
			}
			if n <= 1 {
				return ErrLastRecoveryMethod
			}
		}
		return tx.Delete(&b).Error
	})
}

// InvalidateAccountSessions 改密后吊销全部 access/refresh（AP-079）。
// 吊销 refresh family + 清空 device_account refresh +  bump 全部设备 token_version。
func (r *AccountRepo) InvalidateAccountSessions(accountID string) error {
	if accountID == "" {
		return errors.New("account_id required")
	}
	now := time.Now().UTC()
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.RefreshToken{}).
			Where("account_id = ? AND status IN ?", accountID, []string{"active", "rotated"}).
			Updates(map[string]interface{}{
				"status":     "revoked",
				"revoked_at": now,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.DeviceAccount{}).
			Where("account_id = ?", accountID).
			Updates(map[string]interface{}{
				"refresh_token_hash": "",
				"refresh_expires_at": nil,
			}).Error; err != nil {
			return err
		}
		// bump 所有关联设备 token_version
		return tx.Model(&models.Device{}).
			Where("account_id = ?", accountID).
			Update("token_version", gorm.Expr("token_version + 1")).Error
	})
}

// FindVerifiedEmailBinding 查找账号下已验证邮箱绑定（改密/reauth）。
func (r *AccountRepo) FindVerifiedEmailBinding(accountID string) (*models.AccountBinding, error) {
	var b models.AccountBinding
	err := r.db.Where("account_id = ? AND provider = ? AND verified = ?", accountID, "email", true).
		First(&b).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// FindEmailBinding 按规范化邮箱查找绑定。
func (r *AccountRepo) FindEmailBinding(email string) (*models.AccountBinding, error) {
	return r.FindBinding("email", NormalizeEmail(email))
}
