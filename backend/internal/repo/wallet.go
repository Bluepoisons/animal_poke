// Package repo — AP-082 钱包 / 库存 / 不可变流水仓储。
package repo

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 钱包领域错误。
var (
	ErrInsufficientBalance   = errors.New("insufficient_balance")
	ErrInsufficientItem      = errors.New("insufficient_item")
	ErrInvalidAmount         = errors.New("invalid_amount")
	ErrInvalidCurrency       = errors.New("invalid_currency")
	ErrInvalidItemID         = errors.New("invalid_item_id")
	ErrInvalidOperationID    = errors.New("invalid_operation_id")
	ErrInvalidOwner          = errors.New("invalid_owner")
	ErrWalletRepoUnavailable = errors.New("wallet repo unavailable")
)

// WalletRepo 余额快照 + 不可变流水 + 道具库存。
type WalletRepo struct {
	db *gorm.DB
}

// NewWalletRepo 构造。
func NewWalletRepo(db *gorm.DB) *WalletRepo {
	return &WalletRepo{db: db}
}

// WithTx 绑定事务。
func (r *WalletRepo) WithTx(tx *gorm.DB) *WalletRepo {
	return &WalletRepo{db: tx}
}

// DB 暴露底层连接。
func (r *WalletRepo) DB() *gorm.DB { return r.db }

// OwnerKey 解析归属键：绑定账号优先（跨设备），否则设备级游客。
func OwnerKey(accountID, deviceID string) string {
	if a := strings.TrimSpace(accountID); a != "" {
		return "acc:" + a
	}
	return "dev:" + strings.TrimSpace(deviceID)
}

// ApplyRequest 单笔账变请求（货币或道具）。
type ApplyRequest struct {
	DeviceID    string
	AccountID   string
	Kind        string // currency|item
	Currency    string // gold|stamina 或 item_id
	Delta       int64  // 正入账 / 负出账
	OperationID string
	SourceType  string
	SourceID    string
	Metadata    string
}

// ApplyResult 账变结果（含幂等重放）。
type ApplyResult struct {
	Entry      *models.WalletLedgerEntry `json:"entry"`
	Balance    int64                     `json:"balance"`
	Idempotent bool                      `json:"idempotent"`
}

// Apply 在单事务内串行应用账变：
//   - operation_id 幂等（已存在则原样返回）
//   - 余额/数量永不为负
//   - SELECT FOR UPDATE 串行化并发扣款
//   - 短暂 DB 锁冲突自动重试（SQLite 测试 / MySQL 死锁）
func (r *WalletRepo) Apply(req ApplyRequest) (*ApplyResult, error) {
	if r == nil || r.db == nil {
		return nil, ErrWalletRepoUnavailable
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return nil, ErrInvalidOwner
	}
	opID := strings.TrimSpace(req.OperationID)
	if opID == "" || len(opID) > 128 {
		return nil, ErrInvalidOperationID
	}
	if req.Delta == 0 {
		return nil, ErrInvalidAmount
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = models.LedgerKindCurrency
	}
	currency := strings.TrimSpace(req.Currency)
	if kind == models.LedgerKindCurrency {
		if currency != models.CurrencyGold && currency != models.CurrencyStamina {
			return nil, ErrInvalidCurrency
		}
	} else if kind == models.LedgerKindItem {
		if currency == "" || len(currency) > 64 {
			return nil, ErrInvalidItemID
		}
	} else {
		return nil, ErrInvalidCurrency
	}
	sourceType := strings.TrimSpace(req.SourceType)
	if sourceType == "" {
		sourceType = "system"
	}
	if len(sourceType) > 32 {
		sourceType = sourceType[:32]
	}
	owner := OwnerKey(req.AccountID, deviceID)
	const maxAttempts = 40
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		out, err := r.applyOnceResult(owner, deviceID, kind, currency, opID, sourceType, req)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if errors.Is(err, ErrInsufficientBalance) || errors.Is(err, ErrInsufficientItem) ||
			errors.Is(err, ErrInvalidAmount) || errors.Is(err, ErrInvalidCurrency) ||
			errors.Is(err, ErrInvalidItemID) || errors.Is(err, ErrInvalidOperationID) ||
			errors.Is(err, ErrInvalidOwner) {
			return nil, err
		}
		if !isTransientDBLock(err) {
			return nil, err
		}
		sleep := time.Duration(1+attempt*attempt) * time.Millisecond
		if sleep > 50*time.Millisecond {
			sleep = 50 * time.Millisecond
		}
		time.Sleep(sleep)
	}
	return nil, lastErr
}

// applyOnceResult 执行一次事务并返回结果。
func (r *WalletRepo) applyOnceResult(owner, deviceID, kind, currency, opID, sourceType string, req ApplyRequest) (*ApplyResult, error) {
	var out *ApplyResult
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing models.WalletLedgerEntry
		err := tx.Where("operation_id = ?", opID).First(&existing).Error
		if err == nil {
			out = &ApplyResult{Entry: &existing, Balance: existing.BalanceAfter, Idempotent: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var balanceAfter int64
		if kind == models.LedgerKindCurrency {
			bal, err := lockOrCreateBalance(tx, owner, currency, deviceID, req.AccountID)
			if err != nil {
				return err
			}
			next := bal.Balance + req.Delta
			if next < 0 {
				return ErrInsufficientBalance
			}
			if err := tx.Model(&models.WalletBalance{}).
				Where("id = ?", bal.ID).
				Updates(map[string]interface{}{
					"balance":    next,
					"device_id":  deviceID,
					"account_id": strings.TrimSpace(req.AccountID),
					"updated_at": time.Now().UTC(),
				}).Error; err != nil {
				return err
			}
			balanceAfter = next
		} else {
			item, err := lockOrCreateInventory(tx, owner, currency, deviceID, req.AccountID)
			if err != nil {
				return err
			}
			next := item.Quantity + req.Delta
			if next < 0 {
				return ErrInsufficientItem
			}
			if err := tx.Model(&models.InventoryItem{}).
				Where("id = ?", item.ID).
				Updates(map[string]interface{}{
					"quantity":   next,
					"device_id":  deviceID,
					"account_id": strings.TrimSpace(req.AccountID),
					"updated_at": time.Now().UTC(),
				}).Error; err != nil {
				return err
			}
			balanceAfter = next
		}

		amount := req.Delta
		if amount < 0 {
			amount = -amount
		}
		entry := models.WalletLedgerEntry{
			EntryID:      uuid.NewString(),
			OperationID:  opID,
			OwnerKey:     owner,
			DeviceID:     deviceID,
			AccountID:    strings.TrimSpace(req.AccountID),
			Kind:         kind,
			Currency:     currency,
			Delta:        req.Delta,
			Amount:       amount,
			BalanceAfter: balanceAfter,
			SourceType:   sourceType,
			SourceID:     strings.TrimSpace(req.SourceID),
			Metadata:     req.Metadata,
			CreatedAt:    time.Now().UTC(),
		}
		if err := tx.Create(&entry).Error; err != nil {
			if isUniqueViolation(err) {
				var again models.WalletLedgerEntry
				if err2 := tx.Where("operation_id = ?", opID).First(&again).Error; err2 == nil {
					out = &ApplyResult{Entry: &again, Balance: again.BalanceAfter, Idempotent: true}
					return nil
				}
			}
			return err
		}
		out = &ApplyResult{Entry: &entry, Balance: balanceAfter, Idempotent: false}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func lockOrCreateBalance(tx *gorm.DB, owner, currency, deviceID, accountID string) (*models.WalletBalance, error) {
	var bal models.WalletBalance
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("owner_key = ? AND currency = ?", owner, currency).
		First(&bal).Error
	if err == nil {
		return &bal, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	bal = models.WalletBalance{
		OwnerKey:  owner,
		Currency:  currency,
		DeviceID:  deviceID,
		AccountID: strings.TrimSpace(accountID),
		Balance:   0,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := tx.Create(&bal).Error; err != nil {
		if isUniqueViolation(err) {
			if err2 := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("owner_key = ? AND currency = ?", owner, currency).
				First(&bal).Error; err2 != nil {
				return nil, err2
			}
			return &bal, nil
		}
		return nil, err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", bal.ID).First(&bal).Error; err != nil {
		return nil, err
	}
	return &bal, nil
}

func lockOrCreateInventory(tx *gorm.DB, owner, itemID, deviceID, accountID string) (*models.InventoryItem, error) {
	var item models.InventoryItem
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("owner_key = ? AND item_id = ?", owner, itemID).
		First(&item).Error
	if err == nil {
		return &item, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	item = models.InventoryItem{
		OwnerKey:  owner,
		ItemID:    itemID,
		DeviceID:  deviceID,
		AccountID: strings.TrimSpace(accountID),
		Quantity:  0,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := tx.Create(&item).Error; err != nil {
		if isUniqueViolation(err) {
			if err2 := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("owner_key = ? AND item_id = ?", owner, itemID).
				First(&item).Error; err2 != nil {
				return nil, err2
			}
			return &item, nil
		}
		return nil, err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", item.ID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// GetBalances 返回 owner 的货币余额快照。
func (r *WalletRepo) GetBalances(accountID, deviceID string) ([]models.WalletBalance, error) {
	if r == nil || r.db == nil {
		return nil, ErrWalletRepoUnavailable
	}
	owner := OwnerKey(accountID, deviceID)
	var rows []models.WalletBalance
	err := r.db.Where("owner_key = ?", owner).Order("currency asc").Find(&rows).Error
	return rows, err
}

// GetInventory 返回 owner 的道具列表。
func (r *WalletRepo) GetInventory(accountID, deviceID string) ([]models.InventoryItem, error) {
	if r == nil || r.db == nil {
		return nil, ErrWalletRepoUnavailable
	}
	owner := OwnerKey(accountID, deviceID)
	var rows []models.InventoryItem
	err := r.db.Where("owner_key = ? AND quantity > 0", owner).Order("item_id asc").Find(&rows).Error
	return rows, err
}

// ListLedger 分页列出流水（按 id 降序）。
func (r *WalletRepo) ListLedger(accountID, deviceID, currency string, afterID uint, limit int) ([]models.WalletLedgerEntry, error) {
	if r == nil || r.db == nil {
		return nil, ErrWalletRepoUnavailable
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	owner := OwnerKey(accountID, deviceID)
	q := r.db.Where("owner_key = ?", owner)
	if currency != "" {
		q = q.Where("currency = ?", currency)
	}
	if afterID > 0 {
		q = q.Where("id < ?", afterID)
	}
	var rows []models.WalletLedgerEntry
	err := q.Order("id desc").Limit(limit).Find(&rows).Error
	return rows, err
}

// RebuildBalance 从不可变流水重算余额并写回快照（对账）。
func (r *WalletRepo) RebuildBalance(accountID, deviceID, currency string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, ErrWalletRepoUnavailable
	}
	if currency != models.CurrencyGold && currency != models.CurrencyStamina {
		return 0, ErrInvalidCurrency
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return 0, ErrInvalidOwner
	}
	owner := OwnerKey(accountID, deviceID)
	var sum int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.WalletLedgerEntry{}).
			Where("owner_key = ? AND kind = ? AND currency = ?", owner, models.LedgerKindCurrency, currency).
			Select("COALESCE(SUM(delta), 0)").
			Scan(&sum).Error; err != nil {
			return err
		}
		if sum < 0 {
			return fmt.Errorf("ledger sum negative: %d", sum)
		}
		bal, err := lockOrCreateBalance(tx, owner, currency, deviceID, accountID)
		if err != nil {
			return err
		}
		return tx.Model(&models.WalletBalance{}).
			Where("id = ?", bal.ID).
			Updates(map[string]interface{}{
				"balance":    sum,
				"updated_at": time.Now().UTC(),
			}).Error
	})
	return sum, err
}

// RebuildInventory 从流水重算道具数量并写回快照。
func (r *WalletRepo) RebuildInventory(accountID, deviceID, itemID string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, ErrWalletRepoUnavailable
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return 0, ErrInvalidItemID
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return 0, ErrInvalidOwner
	}
	owner := OwnerKey(accountID, deviceID)
	var sum int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.WalletLedgerEntry{}).
			Where("owner_key = ? AND kind = ? AND currency = ?", owner, models.LedgerKindItem, itemID).
			Select("COALESCE(SUM(delta), 0)").
			Scan(&sum).Error; err != nil {
			return err
		}
		if sum < 0 {
			return fmt.Errorf("inventory ledger sum negative: %d", sum)
		}
		item, err := lockOrCreateInventory(tx, owner, itemID, deviceID, accountID)
		if err != nil {
			return err
		}
		return tx.Model(&models.InventoryItem{}).
			Where("id = ?", item.ID).
			Updates(map[string]interface{}{
				"quantity":   sum,
				"updated_at": time.Now().UTC(),
			}).Error
	})
	return sum, err
}

// FindByOperationID 按 operation_id 查流水。
func (r *WalletRepo) FindByOperationID(operationID string) (*models.WalletLedgerEntry, error) {
	if r == nil || r.db == nil {
		return nil, ErrWalletRepoUnavailable
	}
	var e models.WalletLedgerEntry
	err := r.db.Where("operation_id = ?", operationID).First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique") || strings.Contains(s, "duplicate")
}

func isTransientDBLock(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "database is locked") ||
		strings.Contains(s, "database table is locked") ||
		strings.Contains(s, "table is locked") ||
		strings.Contains(s, "busy") ||
		strings.Contains(s, "deadlock") ||
		strings.Contains(s, "lock wait timeout")
}
