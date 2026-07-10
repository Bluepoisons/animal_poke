// Package repo 账号绑定与设备迁移。
package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AccountRepo 账号/绑定/设备关联仓储。
type AccountRepo struct {
	db     *gorm.DB
	pepper string // 用于 token 哈希的 pepper（通常为 JWT secret）
}

// NewAccountRepo 构造。
func NewAccountRepo(db *gorm.DB, pepper string) *AccountRepo {
	return &AccountRepo{db: db, pepper: pepper}
}

// WithTx 事务绑定。
func (r *AccountRepo) WithTx(tx *gorm.DB) *AccountRepo {
	return &AccountRepo{db: tx, pepper: r.pepper}
}

// DB 底层 DB。
func (r *AccountRepo) DB() *gorm.DB { return r.db }

var (
	ErrAccountDisabled   = errors.New("account disabled")
	ErrBindingNotFound   = errors.New("binding not found")
	ErrInvalidCredential = errors.New("invalid credential")
	ErrDeviceRevoked     = errors.New("device revoked")
	ErrAlreadyBound      = errors.New("device already bound to another account")
	ErrBindingConflict   = errors.New("binding already linked to another account")
)

// HashCredential 对密码/token 做不可逆哈希（bcrypt 用于 password 类；token 用 sha256+pepper）。
func (r *AccountRepo) HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword 校验 bcrypt 密码。
func (r *AccountRepo) CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// HashToken 对 refresh/oauth token 做 peppered SHA-256（可索引比对，不存明文）。
func (r *AccountRepo) HashToken(token string) string {
	sum := sha256.Sum256([]byte(r.pepper + ":" + token))
	return hex.EncodeToString(sum[:])
}

// NormalizeEmail 规范化邮箱。
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// CreateAccount 创建新账号。
func (r *AccountRepo) CreateAccount(displayName string) (*models.Account, error) {
	acc := &models.Account{
		AccountID:   uuid.NewString(),
		DisplayName: displayName,
		Status:      "active",
	}
	if err := r.db.Create(acc).Error; err != nil {
		return nil, err
	}
	return acc, nil
}

// FindAccount 按 account_id 查找。
func (r *AccountRepo) FindAccount(accountID string) (*models.Account, error) {
	var acc models.Account
	if err := r.db.Where("account_id = ?", accountID).First(&acc).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

// FindBinding 按 provider + subject 查找绑定。
func (r *AccountRepo) FindBinding(provider, subject string) (*models.AccountBinding, error) {
	var b models.AccountBinding
	err := r.db.Where("provider = ? AND provider_subject = ?", provider, subject).First(&b).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// UpsertBinding 创建或更新绑定（同一 account）。
func (r *AccountRepo) UpsertBinding(accountID, provider, subject, credentialHash string) (*models.AccountBinding, error) {
	var existing models.AccountBinding
	err := r.db.Where("provider = ? AND provider_subject = ?", provider, subject).First(&existing).Error
	if err == nil {
		if existing.AccountID != accountID {
			return nil, ErrBindingConflict
		}
		existing.CredentialHash = credentialHash
		existing.Verified = true
		if err := r.db.Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	b := &models.AccountBinding{
		AccountID:       accountID,
		Provider:        provider,
		ProviderSubject: subject,
		CredentialHash:  credentialHash,
		Verified:        true,
	}
	if err := r.db.Create(b).Error; err != nil {
		return nil, err
	}
	return b, nil
}

// LinkDevice 将设备关联到账号（创建/复活 DeviceAccount，更新 Device.AccountID）。
func (r *AccountRepo) LinkDevice(deviceID, accountID, refreshPlain string, refreshTTL time.Duration) (*models.DeviceAccount, string, error) {
	now := time.Now().UTC()
	var refresh string
	var refreshHash string
	var exp *time.Time
	if refreshPlain != "" {
		refresh = refreshPlain
	} else {
		refresh = uuid.NewString() + uuid.NewString()
	}
	refreshHash = r.HashToken(refresh)
	if refreshTTL > 0 {
		e := now.Add(refreshTTL)
		exp = &e
	}

	var da models.DeviceAccount
	err := r.db.Where("device_id = ?", deviceID).First(&da).Error
	if err == nil {
		if da.AccountID != accountID && da.Status == "active" {
			return nil, "", ErrAlreadyBound
		}
		da.AccountID = accountID
		da.Status = "active"
		da.RefreshTokenHash = refreshHash
		da.RefreshExpiresAt = exp
		da.LinkedAt = now
		da.LastSeenAt = &now
		da.RevokedAt = nil
		if err := r.db.Save(&da).Error; err != nil {
			return nil, "", err
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		da = models.DeviceAccount{
			DeviceID:         deviceID,
			AccountID:        accountID,
			Status:           "active",
			RefreshTokenHash: refreshHash,
			RefreshExpiresAt: exp,
			LinkedAt:         now,
			LastSeenAt:       &now,
		}
		if err := r.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "device_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"account_id", "status", "refresh_token_hash", "refresh_expires_at", "linked_at", "last_seen_at", "revoked_at", "updated_at"}),
		}).Create(&da).Error; err != nil {
			return nil, "", err
		}
	} else {
		return nil, "", err
	}

	if err := r.db.Model(&models.Device{}).Where("device_id = ?", deviceID).
		Update("account_id", accountID).Error; err != nil {
		return nil, "", err
	}
	return &da, refresh, nil
}

// UnlinkDevice 退出登录：清空 refresh，保留 account 关联（或按策略保留）。
func (r *AccountRepo) LogoutDevice(deviceID string) error {
	now := time.Now().UTC()
	// 吊销本设备 refresh + bump token_version
	if err := r.db.Model(&models.DeviceAccount{}).Where("device_id = ? AND status = ?", deviceID, "active").
		Updates(map[string]interface{}{
			"refresh_token_hash": "",
			"refresh_expires_at": nil,
			"last_seen_at":       now,
		}).Error; err != nil {
		return err
	}
	return r.db.Model(&models.Device{}).Where("device_id = ?", deviceID).
		Update("token_version", gorm.Expr("token_version + 1")).Error
}

// RevokeDevice 吊销设备（丢失设备场景）。
func (r *AccountRepo) RevokeDevice(accountID, targetDeviceID string) error {
	now := time.Now().UTC()
	res := r.db.Model(&models.DeviceAccount{}).
		Where("device_id = ? AND account_id = ? AND status = ?", targetDeviceID, accountID, "active").
		Updates(map[string]interface{}{
			"status":             "revoked",
			"revoked_at":         now,
			"refresh_token_hash": "",
			"refresh_expires_at": nil,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	// 禁用设备并 bump token
	return r.db.Model(&models.Device{}).Where("device_id = ?", targetDeviceID).
		Updates(map[string]interface{}{
			"disabled":      true,
			"token_version": gorm.Expr("token_version + 1"),
			"account_id":    "",
		}).Error
}

// ListDevices 列出账号下设备。
func (r *AccountRepo) ListDevices(accountID string) ([]models.DeviceAccount, error) {
	var list []models.DeviceAccount
	err := r.db.Where("account_id = ?", accountID).Order("linked_at desc").Find(&list).Error
	return list, err
}

// FindActiveDeviceAccount 查找活跃设备关联。
func (r *AccountRepo) FindActiveDeviceAccount(deviceID string) (*models.DeviceAccount, error) {
	var da models.DeviceAccount
	err := r.db.Where("device_id = ? AND status = ?", deviceID, "active").First(&da).Error
	if err != nil {
		return nil, err
	}
	return &da, nil
}

// MergeGuestIntoAccount 游客合并：动物/权益/订单归属账号，权益不重复发放。
// 返回合并统计。
type MergeStats struct {
	AnimalsMoved       int `json:"animals_moved"`
	AnimalsSkipped     int `json:"animals_skipped"`
	EntitlementsMoved  int `json:"entitlements_moved"`
	EntitlementsMerged int `json:"entitlements_merged"`
	OrdersMoved        int `json:"orders_moved"`
}

// MergeGuestIntoAccount 将 guestDevice 上的资产合并到 accountID。
func (r *AccountRepo) MergeGuestIntoAccount(guestDeviceID, accountID string) (*MergeStats, error) {
	stats := &MergeStats{}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// 1) Animals：无冲突则挂 account_id；UUID 已在账号下则跳过（不重复）
		var guestAnimals []models.Animal
		if err := tx.Where("device_id = ? AND deleted_at IS NULL", guestDeviceID).Find(&guestAnimals).Error; err != nil {
			return err
		}
		for _, a := range guestAnimals {
			if a.AccountID == accountID {
				continue
			}
			// 若同 UUID 已存在于其他记录（全局 unique），只更新 account_id
			var conflict models.Animal
			// UUID 全局唯一：只需设置 account_id
			// 若账号下已有同 species 奖励类资产，按 UUID 唯一保证不双写
			res := tx.Model(&models.Animal{}).Where("id = ?", a.ID).
				Updates(map[string]interface{}{
					"account_id":     accountID,
					"server_version": time.Now().UTC().UnixNano(),
				})
			if res.Error != nil {
				return res.Error
			}
			_ = conflict
			stats.AnimalsMoved++
		}

		// 2) Entitlements：同 product 在账号下已有 → 合并有效期，停用游客行；否则挂 account_id
		var guestEnts []models.Entitlement
		if err := tx.Where("device_id = ?", guestDeviceID).Find(&guestEnts).Error; err != nil {
			return err
		}
		for _, ge := range guestEnts {
			var accountEnts []models.Entitlement
			if err := tx.Where("account_id = ? AND product_id = ? AND active = ?", accountID, ge.ProductID, true).
				Find(&accountEnts).Error; err != nil {
				return err
			}
			// 排除自身
			var peers []models.Entitlement
			for _, ae := range accountEnts {
				if ae.ID != ge.ID {
					peers = append(peers, ae)
				}
			}
			if len(peers) == 0 {
				if err := tx.Model(&models.Entitlement{}).Where("id = ?", ge.ID).
					Update("account_id", accountID).Error; err != nil {
					return err
				}
				stats.EntitlementsMoved++
				continue
			}
			// 合并到第一份账号权益：取更晚过期；停用 guest
			target := peers[0]
			updates := map[string]interface{}{"active": true}
			if ge.ExpiresAt != nil {
				if target.ExpiresAt == nil || ge.ExpiresAt.After(*target.ExpiresAt) {
					updates["expires_at"] = ge.ExpiresAt
				}
			}
			if err := tx.Model(&models.Entitlement{}).Where("id = ?", target.ID).Updates(updates).Error; err != nil {
				return err
			}
			// 停用游客权益，避免双份
			if err := tx.Model(&models.Entitlement{}).Where("id = ?", ge.ID).
				Updates(map[string]interface{}{"active": false, "account_id": accountID}).Error; err != nil {
				return err
			}
			stats.EntitlementsMerged++
		}

		// 3) Orders
		res := tx.Model(&models.Order{}).Where("device_id = ? AND (account_id = '' OR account_id IS NULL)", guestDeviceID).
			Update("account_id", accountID)
		if res.Error != nil {
			return res.Error
		}
		stats.OrdersMoved = int(res.RowsAffected)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// VerifyBindingCredential 校验绑定凭证。
func (r *AccountRepo) VerifyBindingCredential(b *models.AccountBinding, passwordOrToken string) bool {
	if b == nil {
		return false
	}
	switch b.Provider {
	case "email":
		return r.CheckPassword(b.CredentialHash, passwordOrToken)
	default:
		// mock_oauth / apple / google: peppered sha256
		return b.CredentialHash != "" && b.CredentialHash == r.HashToken(passwordOrToken)
	}
}

// EnsureAccountActive 校验账号状态。
func (r *AccountRepo) EnsureAccountActive(accountID string) (*models.Account, error) {
	acc, err := r.FindAccount(accountID)
	if err != nil {
		return nil, err
	}
	if acc.Status != "active" {
		return nil, ErrAccountDisabled
	}
	return acc, nil
}

// TouchDevice 更新 last_seen。
func (r *AccountRepo) TouchDevice(deviceID string) error {
	now := time.Now().UTC()
	return r.db.Model(&models.DeviceAccount{}).Where("device_id = ? AND status = ?", deviceID, "active").
		Update("last_seen_at", now).Error
}

// FormatDeviceLabel 脱敏展示。
func FormatDeviceLabel(deviceID string) string {
	if len(deviceID) <= 8 {
		return deviceID
	}
	return fmt.Sprintf("%s…%s", deviceID[:4], deviceID[len(deviceID)-4:])
}
