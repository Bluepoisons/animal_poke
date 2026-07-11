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
	db             *gorm.DB
	pepper         string // ACCOUNT_TOKEN_PEPPER（与 JWT 独立）
	pepperPrevious string // 可选上一版 pepper，仅双读校验
}

// NewAccountRepo 构造（单 pepper）。
func NewAccountRepo(db *gorm.DB, pepper string) *AccountRepo {
	return NewAccountRepoWithPeppers(db, pepper, "")
}

// NewAccountRepoWithPeppers 构造（当前 + 可选 previous，单写双读）。
func NewAccountRepoWithPeppers(db *gorm.DB, pepper, previous string) *AccountRepo {
	return &AccountRepo{db: db, pepper: pepper, pepperPrevious: previous}
}

// WithTx 事务绑定。
func (r *AccountRepo) WithTx(tx *gorm.DB) *AccountRepo {
	return &AccountRepo{db: tx, pepper: r.pepper, pepperPrevious: r.pepperPrevious}
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
	ErrDeviceOwnership   = errors.New("device ownership proof required")
	ErrInvalidMergeProof = errors.New("invalid device ownership proof")
	ErrDeviceDisabled    = errors.New("device revoked or disabled")
	ErrTicketReplay      = errors.New("migration ticket already used")
	ErrTicketExpired     = errors.New("migration ticket expired")
	ErrTicketNotFound    = errors.New("migration ticket not found")
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

// HashToken 对 refresh/oauth token 做 peppered SHA-256（单写：始终用当前 pepper）。
func (r *AccountRepo) HashToken(token string) string {
	return r.hashTokenWith(r.pepper, token)
}

func (r *AccountRepo) hashTokenWith(pepper, token string) string {
	sum := sha256.Sum256([]byte(pepper + ":" + token))
	return hex.EncodeToString(sum[:])
}

// matchTokenHash 双读：当前 pepper 命中或 previous pepper 命中即通过。
func (r *AccountRepo) matchTokenHash(stored, token string) bool {
	if stored == "" || token == "" {
		return false
	}
	if stored == r.hashTokenWith(r.pepper, token) {
		return true
	}
	if r.pepperPrevious != "" && stored == r.hashTokenWith(r.pepperPrevious, token) {
		return true
	}
	return false
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
	AnimalsMoved       int    `json:"animals_moved"`
	AnimalsSkipped     int    `json:"animals_skipped"`
	EntitlementsMoved  int    `json:"entitlements_moved"`
	EntitlementsMerged int    `json:"entitlements_merged"`
	OrdersMoved        int    `json:"orders_moved"`
	OperationID        string `json:"operation_id,omitempty"`
	ProofType          string `json:"proof_type,omitempty"`
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
		return r.matchTokenHash(b.CredentialHash, passwordOrToken)
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

// LoginMergeProof 登录合并持有证明。
type LoginMergeProof struct {
	InstallationSecret string
	MigrationTicket    string
}

// LoginMergeResult 登录合并事务结果。
type LoginMergeResult struct {
	Device      *models.Device
	Merge       *MergeStats
	Refresh     string
	OperationID string
	ProofType   string
	Created     bool // 本请求新建设备
}

// DeviceHasGuestAssets 设备是否仍有可合并游客资产（动物/权益/订单）。
func (r *AccountRepo) DeviceHasGuestAssets(deviceID string) (bool, error) {
	var n int64
	if err := r.db.Model(&models.Animal{}).
		Where("device_id = ? AND deleted_at IS NULL AND (account_id = '' OR account_id IS NULL)", deviceID).
		Count(&n).Error; err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	if err := r.db.Model(&models.Entitlement{}).
		Where("device_id = ? AND active = ? AND (account_id = '' OR account_id IS NULL)", deviceID, true).
		Count(&n).Error; err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	if err := r.db.Model(&models.Order{}).
		Where("device_id = ? AND (account_id = '' OR account_id IS NULL)", deviceID).
		Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// CreateMigrationTicket 签发一次性迁移票据（明文仅返回一次）。
// accountID 非空时限制仅该账号可消费。
func (r *AccountRepo) CreateMigrationTicket(sourceDeviceID, accountID string, ttl time.Duration) (plain string, ticket *models.DeviceMigrationTicket, err error) {
	if sourceDeviceID == "" {
		return "", nil, errors.New("source device required")
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	raw := uuid.NewString() + uuid.NewString()
	plain = raw
	now := time.Now().UTC()
	ticket = &models.DeviceMigrationTicket{
		TicketID:       uuid.NewString(),
		TicketHash:     r.HashToken(plain),
		SourceDeviceID: sourceDeviceID,
		AccountID:      accountID,
		ExpiresAt:      now.Add(ttl),
		CreatedAt:      now,
	}
	if err := r.db.Create(ticket).Error; err != nil {
		return "", nil, err
	}
	return plain, ticket, nil
}

// consumeMigrationTicketTx 在事务内原子消费票据（防重放）。
func (r *AccountRepo) consumeMigrationTicketTx(tx *gorm.DB, plain, sourceDeviceID, accountID, operationID string) error {
	if plain == "" {
		return ErrTicketNotFound
	}
	hashes := []string{r.HashToken(plain)}
	if r.pepperPrevious != "" {
		prev := r.hashTokenWith(r.pepperPrevious, plain)
		if prev != hashes[0] {
			hashes = append(hashes, prev)
		}
	}
	var t models.DeviceMigrationTicket
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("ticket_hash IN ?", hashes).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrTicketNotFound
	}
	if err != nil {
		return err
	}
	if t.UsedAt != nil {
		return ErrTicketReplay
	}
	now := time.Now().UTC()
	if now.After(t.ExpiresAt) {
		return ErrTicketExpired
	}
	if t.SourceDeviceID != sourceDeviceID {
		return ErrInvalidMergeProof
	}
	if t.AccountID != "" && t.AccountID != accountID {
		return ErrInvalidMergeProof
	}
	res := tx.Model(&models.DeviceMigrationTicket{}).
		Where("id = ? AND used_at IS NULL", t.ID).
		Updates(map[string]interface{}{
			"used_at":      now,
			"operation_id": operationID,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrTicketReplay
	}
	return nil
}

// mergeGuestIntoAccountTx 事务内合并（与 MergeGuestIntoAccount 语义一致）。
func (r *AccountRepo) mergeGuestIntoAccountTx(tx *gorm.DB, guestDeviceID, accountID string) (*MergeStats, error) {
	stats := &MergeStats{}
	var guestAnimals []models.Animal
	if err := tx.Where("device_id = ? AND deleted_at IS NULL", guestDeviceID).Find(&guestAnimals).Error; err != nil {
		return nil, err
	}
	for _, a := range guestAnimals {
		if a.AccountID == accountID {
			stats.AnimalsSkipped++
			continue
		}
		res := tx.Model(&models.Animal{}).Where("id = ?", a.ID).
			Updates(map[string]interface{}{
				"account_id":     accountID,
				"server_version": time.Now().UTC().UnixNano(),
			})
		if res.Error != nil {
			return nil, res.Error
		}
		stats.AnimalsMoved++
	}

	var guestEnts []models.Entitlement
	if err := tx.Where("device_id = ?", guestDeviceID).Find(&guestEnts).Error; err != nil {
		return nil, err
	}
	for _, ge := range guestEnts {
		var accountEnts []models.Entitlement
		if err := tx.Where("account_id = ? AND product_id = ? AND active = ?", accountID, ge.ProductID, true).
			Find(&accountEnts).Error; err != nil {
			return nil, err
		}
		var peers []models.Entitlement
		for _, ae := range accountEnts {
			if ae.ID != ge.ID {
				peers = append(peers, ae)
			}
		}
		if len(peers) == 0 {
			if err := tx.Model(&models.Entitlement{}).Where("id = ?", ge.ID).
				Update("account_id", accountID).Error; err != nil {
				return nil, err
			}
			stats.EntitlementsMoved++
			continue
		}
		target := peers[0]
		updates := map[string]interface{}{"active": true}
		if ge.ExpiresAt != nil {
			if target.ExpiresAt == nil || ge.ExpiresAt.After(*target.ExpiresAt) {
				updates["expires_at"] = ge.ExpiresAt
			}
		}
		if err := tx.Model(&models.Entitlement{}).Where("id = ?", target.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		if err := tx.Model(&models.Entitlement{}).Where("id = ?", ge.ID).
			Updates(map[string]interface{}{"active": false, "account_id": accountID}).Error; err != nil {
			return nil, err
		}
		stats.EntitlementsMerged++
	}

	res := tx.Model(&models.Order{}).Where("device_id = ? AND (account_id = '' OR account_id IS NULL)", guestDeviceID).
		Update("account_id", accountID)
	if res.Error != nil {
		return nil, res.Error
	}
	stats.OrdersMoved = int(res.RowsAffected)
	return stats, nil
}

// linkDeviceTx 事务内链接设备（不自动启用已禁用 Device.Disabled——由调用方显式 Enable）。
func (r *AccountRepo) linkDeviceTx(tx *gorm.DB, deviceID, accountID, refreshPlain string, refreshTTL time.Duration) (*models.DeviceAccount, string, error) {
	now := time.Now().UTC()
	var refresh string
	if refreshPlain != "" {
		refresh = refreshPlain
	} else {
		refresh = uuid.NewString() + uuid.NewString()
	}
	refreshHash := r.HashToken(refresh)
	var exp *time.Time
	if refreshTTL > 0 {
		e := now.Add(refreshTTL)
		exp = &e
	}

	var da models.DeviceAccount
	err := tx.Where("device_id = ?", deviceID).First(&da).Error
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
		if err := tx.Save(&da).Error; err != nil {
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
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "device_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"account_id", "status", "refresh_token_hash", "refresh_expires_at", "linked_at", "last_seen_at", "revoked_at", "updated_at"}),
		}).Create(&da).Error; err != nil {
			return nil, "", err
		}
	} else {
		return nil, "", err
	}

	if err := tx.Model(&models.Device{}).Where("device_id = ?", deviceID).
		Update("account_id", accountID).Error; err != nil {
		return nil, "", err
	}
	return &da, refresh, nil
}

// LoginLinkAndMerge 登录：校验持有证明、合并游客资产、链接设备、写审计，单事务。
// 规则：
//   - 新设备（不存在）：创建并链接，无需证明；不合并他人资产。
//   - 已有设备且需合并游客资产 / 设备 disabled：必须 installation_secret 或 migration_ticket。
//   - 已撤销/disabled 无证明：拒绝自动 Enable。
//   - 仅知道 device_id 不能合并他人动物/订单/权益。
func (r *AccountRepo) LoginLinkAndMerge(deviceID, accountID string, proof LoginMergeProof, refreshTTL time.Duration) (*LoginMergeResult, error) {
	if deviceID == "" || accountID == "" {
		return nil, errors.New("device_id and account_id required")
	}
	operationID := uuid.NewString()
	result := &LoginMergeResult{OperationID: operationID}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		ar := r.WithTx(tx)
		dr := NewDeviceRepo(tx)

		// 锁定设备行（若存在）
		var dev models.Device
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("device_id = ?", deviceID).First(&dev).Error
		created := false
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			// 新设备：安全路径，不认领任何既有游客资产
			dev = models.Device{DeviceID: deviceID, TokenVersion: 1}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "device_id"}},
				DoNothing: true,
			}).Create(&dev).Error; err != nil {
				return err
			}
			if dev.ID == 0 {
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("device_id = ?", deviceID).First(&dev).Error; err != nil {
					return err
				}
			} else {
				created = true
			}
			result.Created = created
		} else if findErr != nil {
			return findErr
		}

		// 已绑定其他账号
		if dev.AccountID != "" && dev.AccountID != accountID {
			return ErrAlreadyBound
		}

		hasAssets := false
		if !created {
			var n int64
			if err := tx.Model(&models.Animal{}).
				Where("device_id = ? AND deleted_at IS NULL AND (account_id = '' OR account_id IS NULL OR account_id != ?)", deviceID, accountID).
				Count(&n).Error; err != nil {
				return err
			}
			if n > 0 {
				hasAssets = true
			}
			if !hasAssets {
				if err := tx.Model(&models.Entitlement{}).
					Where("device_id = ? AND active = ? AND (account_id = '' OR account_id IS NULL OR account_id != ?)", deviceID, true, accountID).
					Count(&n).Error; err != nil {
					return err
				}
				if n > 0 {
					hasAssets = true
				}
			}
			if !hasAssets {
				if err := tx.Model(&models.Order{}).
					Where("device_id = ? AND (account_id = '' OR account_id IS NULL)", deviceID).
					Count(&n).Error; err != nil {
					return err
				}
				if n > 0 {
					hasAssets = true
				}
			}
		}

		needsMerge := !created && dev.AccountID != accountID && hasAssets
		needsEnable := dev.Disabled
		// 已有 installation secret 的设备：合并或启用都需要证明
		// 无 secret 但有资产：也需要票据证明（无法用 secret）
		needsProof := needsMerge || needsEnable
		// 设备已存在且非本账号归属（含空 account 的游客）时，只要不是“本账号已绑定且无资产无禁用”，合并链接前对“有 secret 的设备”也要求证明
		if !created && dev.AccountID != accountID && dev.InstallationSecretHash != "" {
			needsProof = true
		}

		proofType := "none"
		proved := false
		if proof.InstallationSecret != "" {
			ok, vErr := dr.VerifyInstallationSecret(deviceID, proof.InstallationSecret)
			if vErr != nil {
				return vErr
			}
			if ok {
				proved = true
				proofType = "installation_secret"
			} else if proof.MigrationTicket == "" {
				return ErrInvalidMergeProof
			}
		}
		if !proved && proof.MigrationTicket != "" {
			if err := ar.consumeMigrationTicketTx(tx, proof.MigrationTicket, deviceID, accountID, operationID); err != nil {
				return err
			}
			proved = true
			proofType = "migration_ticket"
		}

		if needsProof && !proved {
			if needsEnable {
				return ErrDeviceDisabled
			}
			return ErrDeviceOwnership
		}

		// 无证明且设备 disabled：绝不自动 Enable
		if needsEnable {
			if !proved {
				return ErrDeviceDisabled
			}
			if err := tx.Model(&models.Device{}).Where("device_id = ?", deviceID).
				Update("disabled", false).Error; err != nil {
				return err
			}
			dev.Disabled = false
		}

		var mergeStats *MergeStats
		if needsMerge || (!created && dev.AccountID != accountID) {
			// 仅在有证明或新设备无资产时允许合并；新设备 created 路径跳过
			if !created {
				if hasAssets && !proved {
					return ErrDeviceOwnership
				}
				// 无资产时也可链接；有资产必须 proved（上面已保证）
				ms, err := ar.mergeGuestIntoAccountTx(tx, deviceID, accountID)
				if err != nil {
					return err
				}
				mergeStats = ms
			}
		}

		_, refresh, err := ar.linkDeviceTx(tx, deviceID, accountID, "", refreshTTL)
		if err != nil {
			return err
		}
		result.Refresh = refresh

		if mergeStats == nil {
			mergeStats = &MergeStats{}
		}
		mergeStats.OperationID = operationID
		mergeStats.ProofType = proofType
		result.Merge = mergeStats
		result.ProofType = proofType

		// 写唯一 operation + 审计
		animals := 0
		ents := 0
		orders := 0
		merged := false
		if mergeStats != nil {
			animals = mergeStats.AnimalsMoved
			ents = mergeStats.EntitlementsMoved + mergeStats.EntitlementsMerged
			orders = mergeStats.OrdersMoved
			merged = animals+ents+orders > 0
		}
		op := &models.AccountMergeOperation{
			OperationID:    operationID,
			AccountID:      accountID,
			DeviceID:       deviceID,
			ActorAccountID: accountID,
			ProofType:      proofType,
			Merged:         merged,
			AnimalsMoved:   animals,
			EntitlementsMv: ents,
			OrdersMoved:    orders,
		}
		if err := tx.Create(op).Error; err != nil {
			return err
		}
		meta := fmt.Sprintf(`{"operation_id":%q,"account_id":%q,"device_id":%q,"actor":%q,"proof_type":%q,"animals_moved":%d,"entitlements":%d,"orders_moved":%d}`,
			operationID, accountID, deviceID, accountID, proofType, animals, ents, orders)
		if err := tx.Create(&models.AuditLog{
			DeviceID:  deviceID,
			Type:      "auth",
			Message:   "account_login_merge",
			Metadata:  meta,
			RiskScore: 0,
			Status:    "closed",
		}).Error; err != nil {
			return err
		}

		// 重新读取设备
		if err := tx.Where("device_id = ?", deviceID).First(&dev).Error; err != nil {
			return err
		}
		result.Device = &dev
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
