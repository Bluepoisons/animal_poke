// Package migrate 版本化 SQL/Schema 迁移（替代生产启动 AutoMigrate）。
package migrate

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"animalpoke/backend/internal/models"

	"gorm.io/gorm"
)

// Version 当前 schema 版本。
const CurrentVersion = "0016_pvp_matchmaking"

// Apply 按版本顺序应用迁移。开发可用；生产建议由 Job 单独执行。
func Apply(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if err := db.AutoMigrate(&models.SchemaMigration{}); err != nil {
		return err
	}

	for _, m := range allMigrations() {
		var existing models.SchemaMigration
		err := db.Where("version = ?", m.version).First(&existing).Error
		if err == nil {
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		slog.Info("应用迁移", "version", m.version)
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := m.fn(tx); err != nil {
				return err
			}
			return tx.Create(&models.SchemaMigration{Version: m.version, AppliedAt: time.Now().UTC()}).Error
		}); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.version, err)
		}
	}
	return nil
}

// CheckVersion 校验 schema 是否至少到达指定版本（应用账号只读校验）。
func CheckVersion(db *gorm.DB, minVersion string) error {
	var count int64
	if err := db.Model(&models.SchemaMigration{}).Where("version = ?", minVersion).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("schema version %s not applied", minVersion)
	}
	return nil
}

// migrationSpec 单条迁移定义。
type migrationSpec struct {
	version string
	fn      func(*gorm.DB) error
}

func allMigrations() []migrationSpec {
	return []migrationSpec{
		{"0001_init_core", migrate0001},
		{"0002_device_token_consent", migrate0002},
		{"0003_inference_provenance", migrate0003},
		{"0004_privacy_location", migrate0004},
		{"0005_commerce_privacy_inference", migrate0005},
		{"0006_inference_lineage", migrate0006},
		{"0007_idempotency_keys", migrate0007},
		{"0008_commerce_security", migrate0008},
		{"0009_check_constraints", migrate0009},
		{"0011_account_merge_proof", migrate0011},
		{"0012_collection_item_fields", migrate0012},
		{"0013_wallet_inventory_ledger", migrate0013},
		{"0014_refresh_token_rotation", migrate0014},
		{"0015_ranking_settlement", migrate0015},
		{"0016_pvp_matchmaking", migrate0016},
	}
}

// StatusReport 迁移状态。
type StatusReport struct {
	Target  string
	Applied []string
	Pending []string
}

// Status 返回已应用/待应用迁移。
func Status(db *gorm.DB) (*StatusReport, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	if err := db.AutoMigrate(&models.SchemaMigration{}); err != nil {
		return nil, err
	}
	var rows []models.SchemaMigration
	if err := db.Order("applied_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	appliedSet := make(map[string]struct{}, len(rows))
	applied := make([]string, 0, len(rows))
	for _, r := range rows {
		applied = append(applied, r.Version)
		appliedSet[r.Version] = struct{}{}
	}
	pending := make([]string, 0)
	for _, m := range allMigrations() {
		if _, ok := appliedSet[m.version]; !ok {
			pending = append(pending, m.version)
		}
	}
	return &StatusReport{Target: CurrentVersion, Applied: applied, Pending: pending}, nil
}

// FormatStatus 人类可读迁移状态。
func FormatStatus(s *StatusReport) string {
	if s == nil {
		return "status: <nil>"
	}
	var b strings.Builder
	b.WriteString("target: ")
	b.WriteString(s.Target)
	b.WriteString("\n")
	b.WriteString("applied:\n")
	if len(s.Applied) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, v := range s.Applied {
			b.WriteString("  [x] ")
			b.WriteString(v)
			b.WriteString("\n")
		}
	}
	b.WriteString("pending:\n")
	if len(s.Pending) == 0 {
		b.WriteString("  (none — schema is current)\n")
	} else {
		for _, v := range s.Pending {
			b.WriteString("  [ ] ")
			b.WriteString(v)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func migrate0001(db *gorm.DB) error {
	return db.AutoMigrate(&models.Device{}, &models.Animal{}, &models.AuditLog{})
}

func migrate0002(db *gorm.DB) error {
	return db.AutoMigrate(&models.Device{})
}

func migrate0003(db *gorm.DB) error {
	return db.AutoMigrate(&models.Inference{})
}

func migrate0004(db *gorm.DB) error {
	return db.AutoMigrate(&models.Animal{}, &models.DataRequest{}, &models.SecurityReport{})
}

func migrate0005(db *gorm.DB) error {
	return db.AutoMigrate(&models.Product{}, &models.Order{}, &models.Entitlement{}, &models.AuditLog{})
}

func migrate0006(db *gorm.DB) error {
	return db.AutoMigrate(&models.Inference{})
}

func migrate0007(db *gorm.DB) error {
	return db.AutoMigrate(&models.IdempotencyRecord{})
}

// migrate0008 商业化安全：设备级幂等、回执哈希、商品种子。
func migrate0008(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.Order{}, &models.Product{}); err != nil {
		return err
	}
	var count int64
	if err := db.Model(&models.Product{}).Where("product_id = ?", "month_card").Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		if err := db.Create(&models.Product{
			ProductID: "month_card", Name: "月卡", Type: "subscription",
			PriceCents: 1800, Currency: "CNY", DurationDay: 30, Active: true,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// WriteStatus 查询并输出迁移状态（CLI 用）。
func WriteStatus(w io.Writer, db *gorm.DB) error {
	if w == nil {
		return fmt.Errorf("writer is nil")
	}
	s, err := Status(db)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, FormatStatus(s))
	return err
}

// migrate0009 业务 CHECK 约束（MySQL 8.0.16+ 强制执行；SQLite 测试忽略失败）。
func migrate0009(db *gorm.DB) error {
	// 用原生 SQL 加约束；重复执行时忽略已存在错误。
	stmts := []string{
		`ALTER TABLE animals ADD CONSTRAINT chk_animals_rarity CHECK (rarity >= 1 AND rarity <= 5)`,
		`ALTER TABLE animals ADD CONSTRAINT chk_animals_species CHECK (species IN ('cat','dog','goose'))`,
		`ALTER TABLE products ADD CONSTRAINT chk_products_price CHECK (price_cents >= 0)`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			msg := strings.ToLower(err.Error())
			// MySQL: Duplicate check constraint name / already exists
			if strings.Contains(msg, "duplicate") || strings.Contains(msg, "already exists") || strings.Contains(msg, "check constraint") && strings.Contains(msg, "exists") {
				continue
			}
			// SQLite AutoMigrate path may not support ALTER CHECK the same way — soft skip in non-mysql
			if strings.Contains(msg, "syntax") || strings.Contains(msg, "near") {
				slog.Warn("skip CHECK constraint on non-MySQL dialect", "err", err)
				continue
			}
			return err
		}
	}
	return nil
}

// migrate0011 登录合并设备持有证明：迁移票据 + 合并操作审计。
func migrate0011(db *gorm.DB) error {
	return db.AutoMigrate(&models.DeviceMigrationTicket{}, &models.AccountMergeOperation{})
}

// migrate0012 收藏项可编辑元数据：nickname / favorite / locked。
func migrate0012(db *gorm.DB) error {
	return db.AutoMigrate(&models.Animal{})
}

// migrate0013 钱包余额快照、不可变流水、道具库存（AP-082）。
func migrate0013(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.WalletBalance{},
		&models.WalletLedgerEntry{},
		&models.InventoryItem{},
	); err != nil {
		return err
	}
	// 余额/数量非负约束（MySQL 强制；SQLite 软跳过）。
	stmts := []string{
		`ALTER TABLE wallet_balances ADD CONSTRAINT chk_wallet_balance_nonneg CHECK (balance >= 0)`,
		`ALTER TABLE inventory_items ADD CONSTRAINT chk_inventory_qty_nonneg CHECK (quantity >= 0)`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "duplicate") || strings.Contains(msg, "already exists") {
				continue
			}
			if strings.Contains(msg, "syntax") || strings.Contains(msg, "near") {
				slog.Warn("skip CHECK constraint on non-MySQL dialect", "err", err)
				continue
			}
			// SQLite may reject ALTER ADD CONSTRAINT differently
			if strings.Contains(msg, "constraint") && strings.Contains(msg, "exists") {
				continue
			}
			return err
		}
	}
	return nil
}

// migrate0014 刷新令牌族：rotate-on-use、绝对/空闲过期、重用检测（AP-078）。
// 同时确保 accounts / account_bindings / device_accounts 表存在（历史路径可能仅 AutoMigrate）。
func migrate0014(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Account{},
		&models.AccountBinding{},
		&models.DeviceAccount{},
		&models.RefreshToken{},
	)
}

// migrate0015 区域日榜分数、结算快照与奖励发放（AP-114）。
func migrate0015(db *gorm.DB) error {
	return db.AutoMigrate(&models.RankingDailyScore{}, &models.RankingSnapshot{}, &models.RankingRewardGrant{})
}

// migrate0016 PvP 匹配队列、对局与段位（AP-115）。
func migrate0016(db *gorm.DB) error {
	return db.AutoMigrate(&models.PvPMatch{}, &models.PvPRating{}, &models.PvPQueue{})
}
