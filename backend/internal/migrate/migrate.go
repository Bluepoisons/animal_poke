// Package migrate 版本化 SQL/Schema 迁移（替代生产启动 AutoMigrate）。
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"animalpoke/backend/internal/models"

	"gorm.io/gorm"
)

// CurrentVersion 当前 schema 版本。
const CurrentVersion = "0007_data_constraints"

// 迁移锁（MySQL GET_LOCK，连接级）。
const (
	migrateLockName       = "animal_poke_schema_migrate"
	migrateLockTimeoutSec = 120
)

// migration 单条版本化迁移。
type migration struct {
	version string
	fn      func(*gorm.DB) error
}

// allMigrations 有序迁移清单。
func allMigrations() []migration {
	return []migration{
		{"0001_init_core", migrate0001},
		{"0002_device_token_consent", migrate0002},
		{"0003_inference_provenance", migrate0003},
		{"0004_privacy_location", migrate0004},
		{"0005_commerce_privacy_inference", migrate0005},
		{"0006_inference_lineage", migrate0006},
		{"0007_data_constraints", migrate0007},
	}
}

// StatusReport migrate status 输出。
type StatusReport struct {
	Target  string   `json:"target"`
	Applied []string `json:"applied"`
	Pending []string `json:"pending"`
}

// Apply 按版本顺序应用迁移（幂等）。生产建议由 Job 单独执行。
// MySQL 下使用 GET_LOCK 防并发；SQLite 无锁。
// 单版本失败可安全重试：已写入 schema_migrations 的版本会跳过。
func Apply(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	return withMigrateLock(db, func(conn *gorm.DB) error {
		return applyLocked(conn)
	})
}

func applyLocked(db *gorm.DB) error {
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
		// MySQL DDL 常隐式提交，不能依赖事务原子性；版本行在成功后写入。
		// 迁移函数自身必须幂等（约束/索引存在则跳过）。
		if err := m.fn(db); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.version, err)
		}
		if err := db.Create(&models.SchemaMigration{Version: m.version, AppliedAt: time.Now().UTC()}).Error; err != nil {
			return fmt.Errorf("migration %s record failed: %w", m.version, err)
		}
	}
	return nil
}

// Status 返回已应用 / 待应用版本。
func Status(db *gorm.DB) (*StatusReport, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	if err := db.AutoMigrate(&models.SchemaMigration{}); err != nil {
		return nil, err
	}
	var rows []models.SchemaMigration
	if err := db.Order("version asc").Find(&rows).Error; err != nil {
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
	return &StatusReport{
		Target:  CurrentVersion,
		Applied: applied,
		Pending: pending,
	}, nil
}

// FormatStatus 人类可读 status 输出。
func FormatStatus(r *StatusReport) string {
	if r == nil {
		return "status: <nil>\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "target:  %s\n", r.Target)
	fmt.Fprintf(&b, "applied: %d\n", len(r.Applied))
	for _, v := range r.Applied {
		fmt.Fprintf(&b, "  [x] %s\n", v)
	}
	fmt.Fprintf(&b, "pending: %d\n", len(r.Pending))
	if len(r.Pending) == 0 {
		b.WriteString("  (none — schema is current)\n")
	} else {
		for _, v := range r.Pending {
			fmt.Fprintf(&b, "  [ ] %s\n", v)
		}
	}
	return b.String()
}

// WriteStatus 将 Status 写入 w。
func WriteStatus(w io.Writer, db *gorm.DB) error {
	r, err := Status(db)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, FormatStatus(r))
	return err
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

// withMigrateLock 在 MySQL 上占用命名锁；其他方言直通。
// 锁钉在独立 database/sql.Conn 上（GET_LOCK 连接级），迁移走连接池，
// 避免 gorm.DB.Connection + AutoMigrate 的会话问题。
func withMigrateLock(db *gorm.DB, fn func(*gorm.DB) error) error {
	if dialectorName(db) != "mysql" {
		return fn(db)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("migrate lock sqlDB: %w", err)
	}
	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrate lock conn: %w", err)
	}
	defer conn.Close()

	var got int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, ?)", migrateLockName, migrateLockTimeoutSec).Scan(&got); err != nil {
		return fmt.Errorf("GET_LOCK: %w", err)
	}
	if got != 1 {
		return fmt.Errorf("could not acquire migrate lock %q within %ds", migrateLockName, migrateLockTimeoutSec)
	}
	defer func() {
		var released sql.NullInt64
		if err := conn.QueryRowContext(ctx, "SELECT RELEASE_LOCK(?)", migrateLockName).Scan(&released); err != nil {
			slog.Warn("RELEASE_LOCK failed", "lock", migrateLockName, "err", err)
		}
	}()
	return fn(db)
}

func dialectorName(db *gorm.DB) string {
	if db == nil || db.Dialector == nil {
		return ""
	}
	return strings.ToLower(db.Dialector.Name())
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

// migrate0007 数据完整性约束（MySQL CHECK/FK；SQLite 记录版本即可）。
// Expand/contract 见 docs/runbooks/schema-migrate.md。
func migrate0007(db *gorm.DB) error {
	switch dialectorName(db) {
	case "mysql":
		return migrate0007MySQL(db)
	default:
		// SQLite 等：单元测试路径不强制 CHECK（方言差异）；版本行仍标记已应用。
		slog.Info("skip MySQL-only constraints on dialect", "dialect", dialectorName(db))
		return nil
	}
}

func migrate0007MySQL(db *gorm.DB) error {
	// --- animals: species / rarity / stats ---
	checks := []struct {
		table string
		name  string
		expr  string
	}{
		{"animals", "chk_animals_species", "species IN ('cat','dog','goose')"},
		{"animals", "chk_animals_rarity", "rarity BETWEEN 1 AND 5"},
		{"animals", "chk_animals_hp", "hp >= 0 AND hp <= 100"},
		{"animals", "chk_animals_atk", "atk >= 0 AND atk <= 50"},
		{"animals", "chk_animals_def", "def >= 0 AND def <= 50"},
		{"animals", "chk_animals_spd", "spd >= 0 AND spd <= 50"},
		// inferences status / species（空串表示未填）
		{"inferences", "chk_inferences_status", "status IN ('success','failed','consumed')"},
		{"inferences", "chk_inferences_kind", "kind IN ('detect','analyze','value')"},
		{"inferences", "chk_inferences_species", "species = '' OR species IN ('cat','dog','goose','unknown','unsupported')"},
		// commerce amounts / status
		{"products", "chk_products_price", "price_cents >= 0"},
		{"products", "chk_products_type", "type IN ('consumable','subscription','non_consumable')"},
		{"orders", "chk_orders_amount", "amount_cents >= 0"},
		{"orders", "chk_orders_status", "status IN ('created','paid','fulfilled','refunded','failed')"},
		// privacy / audit status
		{"data_requests", "chk_data_requests_type", "type IN ('export','delete')"},
		{"data_requests", "chk_data_requests_status", "status IN ('pending','processing','completed','cancelled','failed')"},
		{"audit_logs", "chk_audit_logs_status", "status IN ('open','ack','closed')"},
		{"audit_logs", "chk_audit_logs_risk", "risk_score >= 0 AND risk_score <= 100"},
	}
	for _, c := range checks {
		if err := addCheckIfMissing(db, c.table, c.name, c.expr); err != nil {
			return err
		}
	}

	// FK-ish：device / product 引用（幂等；失败时给出可操作错误）
	fks := []struct {
		table    string
		name     string
		cols     string
		refTable string
		refCols  string
	}{
		{"animals", "fk_animals_device", "device_id", "devices", "device_id"},
		{"orders", "fk_orders_product", "product_id", "products", "product_id"},
	}
	for _, fk := range fks {
		if err := addForeignKeyIfMissing(db, fk.table, fk.name, fk.cols, fk.refTable, fk.refCols); err != nil {
			return err
		}
	}

	// 复合唯一：同设备同 product 权益已由 GORM uniqueIndex 覆盖；
	// 推理凭证 inference_id / 订单 idempotency_key / receipt_hash / security nonce 已 unique。
	// 补充：同一 device 下 security report 的业务幂等已由 nonce 全局唯一保证。
	return nil
}

func addCheckIfMissing(db *gorm.DB, table, name, expr string) error {
	exists, err := constraintExists(db, table, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	sql := fmt.Sprintf("ALTER TABLE `%s` ADD CONSTRAINT `%s` CHECK (%s)", table, name, expr)
	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("add check %s.%s: %w", table, name, err)
	}
	slog.Info("added CHECK constraint", "table", table, "name", name)
	return nil
}

func addForeignKeyIfMissing(db *gorm.DB, table, name, cols, refTable, refCols string) error {
	exists, err := constraintExists(db, table, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	// 先确保引用列有索引（MySQL FK 要求）
	sql := fmt.Sprintf(
		"ALTER TABLE `%s` ADD CONSTRAINT `%s` FOREIGN KEY (%s) REFERENCES `%s` (%s)",
		table, name, cols, refTable, refCols,
	)
	if err := db.Exec(sql).Error; err != nil {
		// Expand 阶段：若存在孤儿行导致 FK 失败，记录并返回错误（需先清洗）
		return fmt.Errorf("add FK %s.%s (expand: clean orphans first): %w", table, name, err)
	}
	slog.Info("added FOREIGN KEY", "table", table, "name", name)
	return nil
}

func constraintExists(db *gorm.DB, table, name string) (bool, error) {
	var n int64
	err := db.Raw(`
		SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
		WHERE CONSTRAINT_SCHEMA = DATABASE()
		  AND TABLE_NAME = ?
		  AND CONSTRAINT_NAME = ?
	`, table, name).Scan(&n).Error
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// DefaultStatusWriter 便于 CLI 注入；测试可替换。
var DefaultStatusWriter io.Writer = os.Stdout
