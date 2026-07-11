// Package models 共享 GORM 数据模型。
package models

import (
	"time"
)

// Device 设备注册表。每个客户端设备对应一条记录；可可选绑定账号。
type Device struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	DeviceID     string `gorm:"uniqueIndex;size:64;not null" json:"device_id"`
	AccountID    string `gorm:"index;size:36" json:"account_id,omitempty"` // 绑定账号后填充
	TokenVersion int    `gorm:"not null;default:1" json:"token_version"`
	Disabled     bool   `gorm:"not null;default:false" json:"disabled"`
	// InstallationSecretHash 安装密钥哈希（sha256 hex），明文仅首次注册返回一次。
	InstallationSecretHash string `gorm:"size:128" json:"-"`
	// InstallationSecretSalt 可选盐（hex），为空时直接 sha256(secret)。
	InstallationSecretSalt string `gorm:"size:64" json:"-"`
	// 授权
	ConsentVersion string     `gorm:"size:32" json:"consent_version"`
	ConsentAt      *time.Time `json:"consent_at,omitempty"`
	ConsentScope   string     `gorm:"size:128" json:"consent_scope"`
	ConsentRevoked *time.Time `gorm:"column:consent_revoked_at" json:"consent_revoked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (Device) TableName() string { return "devices" }

// Animal 玩家同步上传的动物元数据。
// 位置默认仅保存粗精度（城市/geohash），精确坐标可选且短期。
type Animal struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	UUID      string `gorm:"uniqueIndex;size:36;not null" json:"uuid"`
	DeviceID  string `gorm:"index;size:64;not null" json:"device_id"`
	AccountID string `gorm:"index;size:36" json:"account_id,omitempty"` // 绑定后归属账号
	Species   string `gorm:"size:32;not null" json:"species"`
	Breed     string `gorm:"size:64" json:"breed"`
	Rarity    int    `gorm:"not null" json:"rarity"` // 1-5 星级
	HP        int    `json:"hp"`
	ATK       int    `json:"atk"`
	DEF       int    `json:"def"`
	SPD       int    `json:"spd"`
	Class     string `gorm:"size:32" json:"class"`
	Element   string `gorm:"size:32" json:"element"`
	// 位置最小化：城市/geohash；精确坐标可选
	City               string     `gorm:"size:64" json:"city"`
	GeoHash            string     `gorm:"size:16;index" json:"geohash"`
	Latitude           float64    `json:"latitude,omitempty"`  // 粗精度或 0
	Longitude          float64    `json:"longitude,omitempty"` // 粗精度或 0
	PreciseLat         *float64   `json:"-"`                   // 不暴露给普通 API
	PreciseLng         *float64   `json:"-"`
	PreciseExpiresAt   *time.Time `json:"-"`
	GeneratedAt        time.Time  `gorm:"not null" json:"generated_at"`
	InferenceRequestID string     `gorm:"index;size:128" json:"inference_request_id"`
	// 收藏元数据（AP-090）：客户端可编辑；乐观锁依赖 ServerVersion
	Nickname      string     `gorm:"size:64" json:"nickname"`
	Favorite      bool       `gorm:"not null;default:false" json:"favorite"`
	Locked        bool       `gorm:"not null;default:false" json:"locked"` // 收藏锁：防误删
	ServerVersion int64      `gorm:"not null;default:1" json:"server_version"`
	DeletedAt     *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// TableName 明确表名。
func (Animal) TableName() string { return "animals" }

// AuditLog 反作弊/同步/安全审计日志。
type AuditLog struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	DeviceID           string     `gorm:"index:idx_audit_device_time,priority:1;size:64;not null" json:"device_id"`
	Type               string     `gorm:"index:idx_audit_type_time,priority:1;size:32;not null" json:"type"` // sync, anomaly, auth, security, commerce
	Message            string     `gorm:"type:text" json:"message"`
	InferenceRequestID string     `gorm:"index;size:128" json:"inference_request_id"`
	Metadata           string     `gorm:"type:text" json:"metadata"` // JSON 附加信息
	RiskScore          int        `gorm:"not null;default:0" json:"risk_score"`
	Status             string     `gorm:"size:32;not null;default:open" json:"status"` // open|ack|closed
	CreatedAt          time.Time  `gorm:"index:idx_audit_device_time,priority:2;index:idx_audit_type_time,priority:2" json:"created_at"`
	AckedAt            *time.Time `json:"acked_at,omitempty"`
	AckedBy            string     `gorm:"size:64" json:"acked_by,omitempty"`
}

// TableName 明确表名。
func (AuditLog) TableName() string { return "audit_logs" }

// Inference 服务端推理凭证与 provenance。
type Inference struct {
	ID                uint   `gorm:"primaryKey" json:"id"`
	InferenceID       string `gorm:"uniqueIndex;size:64;not null" json:"inference_id"`
	DeviceID          string `gorm:"index;size:64;not null" json:"device_id"`
	Kind              string `gorm:"size:32;not null" json:"kind"` // detect|analyze|value
	ParentInferenceID string `gorm:"index;size:64" json:"parent_inference_id,omitempty"`
	Provider          string `gorm:"size:64" json:"provider"`
	Model             string `gorm:"size:128" json:"model"`
	PromptVersion     string `gorm:"size:32" json:"prompt_version"`
	PromptHash        string `gorm:"size:64" json:"prompt_hash"`
	InputDigest       string `gorm:"size:64" json:"input_digest"`  // 图片/输入摘要，不含原图
	OutputDigest      string `gorm:"size:64" json:"output_digest"` // 输出摘要
	// ResultJSON 权威结果摘要（value: rarity/stats/species；detect/analyze: 关键字段）
	ResultJSON    string     `gorm:"type:text" json:"result_json,omitempty"`
	Species       string     `gorm:"size:32" json:"species,omitempty"`
	ConfigVersion string     `gorm:"size:32" json:"config_version,omitempty"`
	Status        string     `gorm:"size:32;not null;default:success" json:"status"` // success|failed|consumed
	DurationMs    int64      `json:"duration_ms"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	ConsumedAt    *time.Time `json:"consumed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// TableName 明确表名。
func (Inference) TableName() string { return "inferences" }

// DataRequest 数据导出/删除申请。
type DataRequest struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	RequestID   string     `gorm:"uniqueIndex;size:64;not null" json:"request_id"`
	DeviceID    string     `gorm:"index;size:64;not null" json:"device_id"`
	Type        string     `gorm:"size:16;not null" json:"type"`   // export|delete
	Status      string     `gorm:"size:32;not null" json:"status"` // pending|processing|completed|cancelled|failed
	Payload     string     `gorm:"type:longtext" json:"payload,omitempty"`
	ErrorMsg    string     `gorm:"type:text" json:"error_msg,omitempty"`
	RequestedAt time.Time  `json:"requested_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (DataRequest) TableName() string { return "data_requests" }

// SecurityReport 客户端安全报告。
type SecurityReport struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ReportID  string    `gorm:"uniqueIndex;size:64;not null" json:"report_id"`
	DeviceID  string    `gorm:"index;size:64;not null" json:"device_id"`
	Nonce     string    `gorm:"uniqueIndex;size:64;not null" json:"nonce"`
	Payload   string    `gorm:"type:text" json:"payload"`
	RiskScore int       `gorm:"not null;default:0" json:"risk_score"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 明确表名。
func (SecurityReport) TableName() string { return "security_reports" }

// Product 商业化商品。
type Product struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ProductID   string    `gorm:"uniqueIndex;size:64;not null" json:"product_id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Type        string    `gorm:"size:32;not null" json:"type"` // consumable|subscription|non_consumable
	PriceCents  int       `gorm:"not null" json:"price_cents"`
	Currency    string    `gorm:"size:8;not null;default:CNY" json:"currency"`
	DurationDay int       `json:"duration_day"` // 月卡天数
	Active      bool      `gorm:"not null;default:true" json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName 明确表名。
func (Product) TableName() string { return "products" }

// Order 服务端订单。
// 幂等作用域为 (device_id, idempotency_key)；receipt_hash 为空时使用 NULL（可多单未履约）。
type Order struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	OrderID        string     `gorm:"uniqueIndex;size:64;not null" json:"order_id"`
	DeviceID       string     `gorm:"uniqueIndex:idx_order_device_idem,priority:1;size:64;not null" json:"device_id"`
	AccountID      string     `gorm:"index;size:36" json:"account_id,omitempty"`
	ProductID      string     `gorm:"index;size:64;not null" json:"product_id"`
	Status         string     `gorm:"size:32;not null" json:"status"` // created|paid|fulfilled|refunded|failed
	Platform       string     `gorm:"size:32" json:"platform"`        // apple|google|mock
	ReceiptHash    *string    `gorm:"uniqueIndex;size:128" json:"receipt_hash,omitempty"`
	AmountCents    int        `json:"amount_cents"`
	Currency       string     `gorm:"size:8" json:"currency"`
	FulfilledAt    *time.Time `json:"fulfilled_at,omitempty"`
	RefundedAt     *time.Time `json:"refunded_at,omitempty"`
	IdempotencyKey string     `gorm:"uniqueIndex:idx_order_device_idem,priority:2;size:64" json:"idempotency_key"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (Order) TableName() string { return "orders" }

// Entitlement 权益（月卡等）；绑定后可按 account_id 跨设备恢复。
type Entitlement struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	DeviceID  string     `gorm:"uniqueIndex:idx_entitlement_device_product,priority:1;size:64;not null" json:"device_id"`
	AccountID string     `gorm:"index;size:36" json:"account_id,omitempty"`
	ProductID string     `gorm:"uniqueIndex:idx_entitlement_device_product,priority:2;size:64;not null" json:"product_id"`
	OrderID   string     `gorm:"index;size:64" json:"order_id"`
	Active    bool       `gorm:"not null;default:true" json:"active"`
	StartsAt  time.Time  `json:"starts_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (Entitlement) TableName() string { return "entitlements" }

// ModerationReport 用户/系统安全举报（虐待、受伤动物等），不含原图。
type ModerationReport struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ReportID     string    `gorm:"uniqueIndex;size:64;not null" json:"report_id"`
	DeviceID     string    `gorm:"index;size:64;not null" json:"device_id"`
	Category     string    `gorm:"size:32;not null;index" json:"category"` // abuse|injured|portrait|sensitive|other
	DecisionCode string    `gorm:"size:64;not null" json:"decision_code"`
	InferenceID  string    `gorm:"index;size:64" json:"inference_id,omitempty"`
	Note         string    `gorm:"type:text" json:"note,omitempty"`
	Status       string    `gorm:"size:32;not null;default:open" json:"status"` // open|reviewing|closed
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 明确表名。
func (ModerationReport) TableName() string { return "moderation_reports" }

// IdempotencyRecord 服务端幂等键（device_id + route + key 唯一）。
type IdempotencyRecord struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	DeviceID     string     `gorm:"uniqueIndex:idx_idem_device_route_key,priority:1;size:64;not null" json:"device_id"`
	Route        string     `gorm:"uniqueIndex:idx_idem_device_route_key,priority:2;size:128;not null" json:"route"`
	Key          string     `gorm:"column:key_name;uniqueIndex:idx_idem_device_route_key,priority:3;size:128;not null" json:"key"`
	RequestHash  string     `gorm:"size:64;not null" json:"request_hash"`
	Status       string     `gorm:"size:32;not null" json:"status"` // processing|completed|failed
	HTTPStatus   int        `json:"http_status"`
	ResponseBody string     `gorm:"type:longtext" json:"response_body,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ExpiresAt    time.Time  `gorm:"index" json:"expires_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// TableName 明确表名。
func (IdempotencyRecord) TableName() string { return "idempotency_records" }

// SchemaMigration 简易版本化迁移记录。
type SchemaMigration struct {
	Version   string    `gorm:"primaryKey;size:64" json:"version"`
	AppliedAt time.Time `json:"applied_at"`
}

// TableName 明确表名。
func (SchemaMigration) TableName() string { return "schema_migrations" }

// Account 可选绑定账号（游客默认无账号）。
type Account struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AccountID   string    `gorm:"uniqueIndex;size:36;not null" json:"account_id"`
	DisplayName string    `gorm:"size:64" json:"display_name"`
	Status      string    `gorm:"size:16;not null;default:active" json:"status"` // active|disabled
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 明确表名。
func (Account) TableName() string { return "accounts" }

// AccountBinding 账号外部身份绑定（email / mock_oauth 等）。
// CredentialHash 仅存哈希，永不存明文 token/password。
// AP-079：email 新建默认可为 pending（Verified=false）；OAuth 绑定视为已验证。
// 未验证邮箱不得作为恢复凭证（login / 找回密码）。
type AccountBinding struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	AccountID       string     `gorm:"index;size:36;not null" json:"account_id"`
	Provider        string     `gorm:"uniqueIndex:idx_binding_provider_subject,priority:1;size:32;not null" json:"provider"` // email|mock_oauth|apple|google
	ProviderSubject string     `gorm:"uniqueIndex:idx_binding_provider_subject,priority:2;size:191;not null" json:"provider_subject"`
	CredentialHash  string     `gorm:"size:128;not null" json:"-"`
	Verified        bool       `gorm:"not null;default:false" json:"verified"`
	VerifiedAt      *time.Time `json:"verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (AccountBinding) TableName() string { return "account_bindings" }

// 安全令牌用途（AP-079）。
const (
	SecurityPurposeEmailVerify   = "email_verify"
	SecurityPurposePasswordReset = "password_reset"
	SecurityPurposeReauth        = "reauth"
)

// AccountSecurityToken 邮箱验证 / 找回密码 / 近期 re-auth 一次性令牌（AP-079）。
// 明文仅签发时返回或投递邮件；库中仅存 peppered SHA-256 哈希。
type AccountSecurityToken struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	TokenID   string     `gorm:"uniqueIndex;size:36;not null" json:"token_id"`
	TokenHash string     `gorm:"uniqueIndex;size:128;not null" json:"-"`
	Purpose   string     `gorm:"index:idx_sec_token_purpose_account,priority:1;size:32;not null" json:"purpose"` // email_verify|password_reset|reauth
	AccountID string     `gorm:"index:idx_sec_token_purpose_account,priority:2;size:36;not null" json:"account_id"`
	Subject   string     `gorm:"index;size:191" json:"subject,omitempty"` // 规范化邮箱等
	BindingID uint       `gorm:"index" json:"binding_id,omitempty"`
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// TableName 明确表名。
func (AccountSecurityToken) TableName() string { return "account_security_tokens" }

// DeviceAccount 设备与账号的关联（支持多设备、撤销）。
type DeviceAccount struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	DeviceID         string     `gorm:"uniqueIndex;size:64;not null" json:"device_id"`
	AccountID        string     `gorm:"index;size:36;not null" json:"account_id"`
	Status           string     `gorm:"size:16;not null;default:active" json:"status"` // active|revoked
	RefreshTokenHash string     `gorm:"size:128" json:"-"`                             // 刷新令牌哈希
	RefreshExpiresAt *time.Time `json:"refresh_expires_at,omitempty"`
	LinkedAt         time.Time  `json:"linked_at"`
	LastSeenAt       *time.Time `json:"last_seen_at,omitempty"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (DeviceAccount) TableName() string { return "device_accounts" }

// RefreshToken 刷新令牌族（AP-078）：rotate-on-use + 重用检测。
// 明文仅签发时返回一次；库中仅存 peppered SHA-256 哈希。
// 同一 family 内轮换；若已 rotated 的令牌被再次使用，则撤销整族。
type RefreshToken struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	TokenID           string     `gorm:"uniqueIndex;size:36;not null" json:"token_id"`
	FamilyID          string     `gorm:"index;size:36;not null" json:"family_id"`
	AccountID         string     `gorm:"index;size:36;not null" json:"account_id"`
	DeviceID          string     `gorm:"index;size:64;not null" json:"device_id"`
	TokenHash         string     `gorm:"uniqueIndex;size:128;not null" json:"-"`
	ParentTokenID     string     `gorm:"index;size:36" json:"parent_token_id,omitempty"`
	Status            string     `gorm:"size:16;not null;default:active;index" json:"status"` // active|rotated|revoked
	AbsoluteExpiresAt time.Time  `gorm:"not null;index" json:"absolute_expires_at"`
	IdleExpiresAt     time.Time  `gorm:"not null;index" json:"idle_expires_at"`
	RotatedAt         *time.Time `json:"rotated_at,omitempty"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt        *time.Time `json:"last_used_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (RefreshToken) TableName() string { return "refresh_tokens" }

// DeviceMigrationTicket 一次性设备资产迁移票据（AP-076）。
// 明文仅签发时返回；库中仅存 peppered/sha256 哈希，防止仅凭 device_id 认领游客资产。
type DeviceMigrationTicket struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	TicketID       string     `gorm:"uniqueIndex;size:36;not null" json:"ticket_id"`
	TicketHash     string     `gorm:"uniqueIndex;size:128;not null" json:"-"`
	SourceDeviceID string     `gorm:"index;size:64;not null" json:"source_device_id"`
	AccountID      string     `gorm:"index;size:36" json:"account_id,omitempty"` // 可选：限制仅指定账号可消费
	ExpiresAt      time.Time  `gorm:"not null;index" json:"expires_at"`
	UsedAt         *time.Time `json:"used_at,omitempty"`
	OperationID    string     `gorm:"index;size:36" json:"operation_id,omitempty"` // 消费时写入
	CreatedAt      time.Time  `json:"created_at"`
}

// TableName 明确表名。
func (DeviceMigrationTicket) TableName() string { return "device_migration_tickets" }

// AccountMergeOperation 登录合并/链接操作审计（唯一 operation_id）。
type AccountMergeOperation struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	OperationID    string    `gorm:"uniqueIndex;size:36;not null" json:"operation_id"`
	AccountID      string    `gorm:"index;size:36;not null" json:"account_id"`
	DeviceID       string    `gorm:"index;size:64;not null" json:"device_id"`
	ActorAccountID string    `gorm:"size:36;not null" json:"actor_account_id"` // 真实审计 actor
	ProofType      string    `gorm:"size:32;not null" json:"proof_type"`       // none|installation_secret|migration_ticket
	Merged         bool      `gorm:"not null;default:false" json:"merged"`
	AnimalsMoved   int       `json:"animals_moved"`
	EntitlementsMv int       `json:"entitlements_moved"`
	OrdersMoved    int       `json:"orders_moved"`
	CreatedAt      time.Time `json:"created_at"`
}

// TableName 明确表名。
func (AccountMergeOperation) TableName() string { return "account_merge_operations" }

// ---------- AP-082 Wallet / Inventory / Reward Ledger ----------

// 支持的货币类型。
const (
	CurrencyGold    = "gold"
	CurrencyStamina = "stamina"
)

// 流水资产类型。
const (
	LedgerKindCurrency = "currency"
	LedgerKindItem     = "item"
)

// WalletBalance 余额快照（按 owner_key + currency 唯一）。
// owner_key = "acc:<account_id>" 或 "dev:<device_id>"，跨设备账号共享余额。
type WalletBalance struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OwnerKey  string    `gorm:"uniqueIndex:idx_wallet_owner_currency,priority:1;size:68;not null" json:"owner_key"`
	Currency  string    `gorm:"uniqueIndex:idx_wallet_owner_currency,priority:2;size:32;not null" json:"currency"` // gold|stamina
	DeviceID  string    `gorm:"index;size:64;not null" json:"device_id"`
	AccountID string    `gorm:"index;size:36" json:"account_id,omitempty"`
	Balance   int64     `gorm:"not null;default:0" json:"balance"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 明确表名。
func (WalletBalance) TableName() string { return "wallet_balances" }

// WalletLedgerEntry 不可变奖励/消费流水。
// operation_id 全局唯一，保证同一奖励来源只入账一次；可从流水重建余额。
type WalletLedgerEntry struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	EntryID      string    `gorm:"uniqueIndex;size:36;not null" json:"entry_id"`
	OperationID  string    `gorm:"uniqueIndex;size:128;not null" json:"operation_id"`
	OwnerKey     string    `gorm:"index:idx_ledger_owner_time,priority:1;size:68;not null" json:"owner_key"`
	DeviceID     string    `gorm:"index;size:64;not null" json:"device_id"`
	AccountID    string    `gorm:"index;size:36" json:"account_id,omitempty"`
	Kind         string    `gorm:"size:16;not null" json:"kind"`     // currency|item
	Currency     string    `gorm:"size:64;not null" json:"currency"` // gold|stamina 或 item_id
	Delta        int64     `gorm:"not null" json:"delta"`            // 正入账 / 负出账
	Amount       int64     `gorm:"not null" json:"amount"`           // 绝对值
	BalanceAfter int64     `gorm:"not null" json:"balance_after"`
	SourceType   string    `gorm:"size:32;not null" json:"source_type"` // checkin|task|battle|capture|shop|admin|compensate|reward|system|dispatch|level_up
	SourceID     string    `gorm:"size:128" json:"source_id,omitempty"`
	Metadata     string    `gorm:"type:text" json:"metadata,omitempty"`
	CreatedAt    time.Time `gorm:"index:idx_ledger_owner_time,priority:2" json:"created_at"`
}

// TableName 明确表名。
func (WalletLedgerEntry) TableName() string { return "wallet_ledger_entries" }

// InventoryItem 道具库存快照（按 owner_key + item_id 唯一）。
type InventoryItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OwnerKey  string    `gorm:"uniqueIndex:idx_inv_owner_item,priority:1;size:68;not null" json:"owner_key"`
	ItemID    string    `gorm:"uniqueIndex:idx_inv_owner_item,priority:2;size:64;not null" json:"item_id"`
	DeviceID  string    `gorm:"index;size:64;not null" json:"device_id"`
	AccountID string    `gorm:"index;size:36" json:"account_id,omitempty"`
	Quantity  int64     `gorm:"not null;default:0" json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 明确表名。
func (InventoryItem) TableName() string { return "inventory_items" }

// RankingDailyScore 日榜累计分（AP-114）。
type RankingDailyScore struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Date      string    `gorm:"uniqueIndex:idx_rank_score,priority:1;size:10;not null" json:"date"` // YYYY-MM-DD UTC
	City      string    `gorm:"uniqueIndex:idx_rank_score,priority:2;size:64;not null" json:"city"`
	OwnerType string    `gorm:"uniqueIndex:idx_rank_score,priority:3;size:16;not null" json:"owner_type"`
	OwnerID   string    `gorm:"uniqueIndex:idx_rank_score,priority:4;size:64;not null" json:"owner_id"`
	Score     int64     `gorm:"not null;default:0;index" json:"score"`
	Eligible  bool      `gorm:"not null;default:true" json:"eligible"`
	Display   string    `gorm:"size:64" json:"display_name"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (RankingDailyScore) TableName() string { return "ranking_daily_scores" }

// RankingSnapshot 日结算快照（不可变）。
type RankingSnapshot struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SnapshotID  string    `gorm:"uniqueIndex;size:36;not null" json:"snapshot_id"`
	Date        string    `gorm:"uniqueIndex:idx_rank_snap,priority:1;size:10;not null" json:"date"`
	City        string    `gorm:"uniqueIndex:idx_rank_snap,priority:2;size:64;not null" json:"city"`
	EntriesJSON string    `gorm:"type:longtext;not null" json:"entries_json"`
	SettledAt   time.Time `gorm:"not null" json:"settled_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (RankingSnapshot) TableName() string { return "ranking_snapshots" }

// RankingRewardGrant 结算奖励发放记录（幂等）。
type RankingRewardGrant struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SnapshotID  string    `gorm:"uniqueIndex:idx_rank_reward,priority:1;size:36;not null" json:"snapshot_id"`
	OwnerType   string    `gorm:"uniqueIndex:idx_rank_reward,priority:2;size:16;not null" json:"owner_type"`
	OwnerID     string    `gorm:"uniqueIndex:idx_rank_reward,priority:3;size:64;not null" json:"owner_id"`
	Rank        int       `gorm:"not null" json:"rank"`
	Gold        int64     `gorm:"not null" json:"gold"`
	OperationID string    `gorm:"size:128;not null" json:"operation_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (RankingRewardGrant) TableName() string { return "ranking_reward_grants" }

// PvPMatch 服务端匹配会话（AP-115）。
type PvPMatch struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	MatchID     string     `gorm:"uniqueIndex;size:36;not null" json:"match_id"`
	PlayerA     string     `gorm:"index;size:80;not null" json:"player_a"`
	PlayerB     string     `gorm:"index;size:80;not null" json:"player_b"`
	Seed        string     `gorm:"size:64;not null" json:"seed"`
	RuleVersion string     `gorm:"size:32;not null" json:"rule_version"`
	Status      string     `gorm:"size:16;not null;index" json:"status"` // matched|completed|cancelled|disputed
	Winner      string     `gorm:"size:80" json:"winner,omitempty"`
	ResultJSON  string     `gorm:"type:text" json:"result_json,omitempty"`
	SettledAt   *time.Time `json:"settled_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (PvPMatch) TableName() string { return "pvp_matches" }

// PvPRating 段位分。
type PvPRating struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OwnerKey  string    `gorm:"uniqueIndex;size:80;not null" json:"owner_key"`
	Rating    int       `gorm:"not null;default:1000" json:"rating"`
	Wins      int       `gorm:"not null;default:0" json:"wins"`
	Losses    int       `gorm:"not null;default:0" json:"losses"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (PvPRating) TableName() string { return "pvp_ratings" }

// PvPQueue 匹配队列。
type PvPQueue struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	OwnerKey   string    `gorm:"uniqueIndex;size:80;not null" json:"owner_key"`
	Rating     int       `gorm:"not null;default:1000;index" json:"rating"`
	EnqueuedAt time.Time `gorm:"not null;index" json:"enqueued_at"`
}

func (PvPQueue) TableName() string { return "pvp_queue" }

// ---------- AP-083 Social: friends / block / mute / report / share ----------

// Social 关系与请求状态常量。
const (
	FriendRequestPending   = "pending"
	FriendRequestAccepted  = "accepted"
	FriendRequestRejected  = "rejected"
	FriendRequestCancelled = "cancelled"

	FriendshipActive  = "active"
	FriendshipRemoved = "removed"

	ShareACLLink    = "link"    // 持有 capability token 即可（仍受 block 约束）
	ShareACLFriends = "friends" // 仅双向好友
	ShareACLPublic  = "public"  // 任意已鉴权用户（仍受 block 约束）

	MaxFriendsPerUser = 200
)

// SocialProfile 用户社交偏好与可搜索资料。
// UserKey = "acc:<account_id>" 或 "dev:<device_id>"。
type SocialProfile struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserKey       string    `gorm:"uniqueIndex;size:68;not null" json:"user_key"`
	DisplayName   string    `gorm:"index;size:64;not null" json:"display_name"`
	SocialEnabled bool      `gorm:"not null;default:true" json:"social_enabled"`
	IsMinor       bool      `gorm:"not null;default:false" json:"is_minor"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName 明确表名。
func (SocialProfile) TableName() string { return "social_profiles" }

// FriendRequest 好友请求状态机：pending → accepted|rejected|cancelled。
// PairKey 为规范化双方键（min|max），用于交叉邀请合并与唯一约束。
type FriendRequest struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	RequestID   string     `gorm:"uniqueIndex;size:36;not null" json:"request_id"`
	FromUserKey string     `gorm:"index:idx_fr_from_status,priority:1;size:68;not null" json:"from_user_key"`
	ToUserKey   string     `gorm:"index:idx_fr_to_status,priority:1;size:68;not null" json:"to_user_key"`
	PairKey     string     `gorm:"index;size:140;not null" json:"pair_key"`
	Status      string     `gorm:"index:idx_fr_from_status,priority:2;index:idx_fr_to_status,priority:2;size:16;not null" json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// TableName 明确表名。
func (FriendRequest) TableName() string { return "friend_requests" }

// Friendship 已接受的好友边（无向，UserLow/UserHigh 规范化）。
type Friendship struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserLow   string     `gorm:"uniqueIndex:idx_friendship_pair,priority:1;size:68;not null" json:"user_low"`
	UserHigh  string     `gorm:"uniqueIndex:idx_friendship_pair,priority:2;size:68;not null" json:"user_high"`
	Status    string     `gorm:"size:16;not null;default:active;index" json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	RemovedAt *time.Time `json:"removed_at,omitempty"`
}

// TableName 明确表名。
func (Friendship) TableName() string { return "friendships" }

// SocialBlock 屏蔽关系：Blocker 屏蔽 Blocked；屏蔽优先于任何关系与分享。
type SocialBlock struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	BlockerUserKey string    `gorm:"uniqueIndex:idx_social_block,priority:1;size:68;not null" json:"blocker_user_key"`
	BlockedUserKey string    `gorm:"uniqueIndex:idx_social_block,priority:2;index;size:68;not null" json:"blocked_user_key"`
	CreatedAt      time.Time `json:"created_at"`
}

// TableName 明确表名。
func (SocialBlock) TableName() string { return "social_blocks" }

// SocialMute 静音：不打断关系，仅抑制通知/动态（本阶段仅持久化状态）。
type SocialMute struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	MuterUserKey string    `gorm:"uniqueIndex:idx_social_mute,priority:1;size:68;not null" json:"muter_user_key"`
	MutedUserKey string    `gorm:"uniqueIndex:idx_social_mute,priority:2;index;size:68;not null" json:"muted_user_key"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName 明确表名。
func (SocialMute) TableName() string { return "social_mutes" }

// SocialUserReport 用户间举报（不含图片/精确坐标）。
type SocialUserReport struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	ReportID        string    `gorm:"uniqueIndex;size:36;not null" json:"report_id"`
	ReporterUserKey string    `gorm:"index:idx_social_report_reporter_time,priority:1;size:68;not null" json:"reporter_user_key"`
	TargetUserKey   string    `gorm:"index;size:68;not null" json:"target_user_key"`
	Category        string    `gorm:"size:32;not null" json:"category"` // harassment|spam|inappropriate|other
	Note            string    `gorm:"type:text" json:"note,omitempty"`
	Status          string    `gorm:"size:16;not null;default:open" json:"status"` // open|reviewing|closed
	CreatedAt       time.Time `gorm:"index:idx_social_report_reporter_time,priority:2" json:"created_at"`
}

// TableName 明确表名。
func (SocialUserReport) TableName() string { return "social_user_reports" }

// SocialShare 安全分享：不可猜 capability token + ACL + 过期 + 撤销。
// 快照仅含粗粒度字段，禁止精确坐标、设备信息、未授权图片。
type SocialShare struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	ShareToken   string     `gorm:"uniqueIndex;size:64;not null" json:"-"` // capability token（响应一次性返回）
	OwnerUserKey string     `gorm:"index;size:68;not null" json:"owner_user_key"`
	AnimalUUID   string     `gorm:"index;size:36;not null" json:"animal_uuid"`
	ACL          string     `gorm:"size:16;not null;default:link" json:"acl"` // link|friends|public
	Species      string     `gorm:"size:32" json:"species"`
	Breed        string     `gorm:"size:64" json:"breed"`
	Rarity       int        `json:"rarity"`
	Nickname     string     `gorm:"size:64" json:"nickname"`
	City         string     `gorm:"size:64" json:"city"` // 仅粗粒度城市
	ExpiresAt    time.Time  `gorm:"index;not null" json:"expires_at"`
	RevokedAt    *time.Time `gorm:"index" json:"revoked_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (SocialShare) TableName() string { return "social_shares" }

// ---------- AP-085 Admin RBAC ----------

// AdminSession 管理端会话（短期 JWT 绑定 sid，支持撤权）。
type AdminSession struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	SessionID string     `gorm:"uniqueIndex;size:36;not null" json:"session_id"`
	ActorID   string     `gorm:"index;size:128;not null" json:"actor_id"`
	Subject   string     `gorm:"index;size:128;not null" json:"subject"`
	Role      string     `gorm:"size:32;not null" json:"role"`
	Env       string     `gorm:"size:32;not null" json:"env"`
	AuthMode  string     `gorm:"size:32;not null" json:"auth_mode"`                   // jwt|break_glass
	Status    string     `gorm:"size:16;not null;default:active;index" json:"status"` // active|revoked
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	RevokedBy string     `gorm:"size:128" json:"revoked_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (AdminSession) TableName() string { return "admin_sessions" }

// AdminActionLog 管理动作审计（真实 actor/session/reason/request_id + 防篡改 HMAC）。
type AdminActionLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	EntryID   string    `gorm:"uniqueIndex;size:36;not null" json:"entry_id"`
	ActorID   string    `gorm:"index:idx_admin_action_actor_time,priority:1;size:128;not null" json:"actor_id"`
	Subject   string    `gorm:"size:128;not null" json:"subject"`
	Role      string    `gorm:"size:32;not null" json:"role"`
	SessionID string    `gorm:"index;size:36" json:"session_id"`
	AuthMode  string    `gorm:"size:32;not null" json:"auth_mode"`
	Action    string    `gorm:"index:idx_admin_action_name_time,priority:1;size:64;not null" json:"action"`
	Resource  string    `gorm:"size:128" json:"resource"`
	Reason    string    `gorm:"size:512" json:"reason"`
	RequestID string    `gorm:"index;size:64" json:"request_id"`
	Outcome   string    `gorm:"size:16;not null" json:"outcome"` // allow|deny|error|ok
	Metadata  string    `gorm:"type:text" json:"metadata,omitempty"`
	Env       string    `gorm:"size:32" json:"env"`
	Integrity string    `gorm:"size:128;not null" json:"integrity"` // HMAC-SHA256 hex
	CreatedAt time.Time `gorm:"index:idx_admin_action_actor_time,priority:2;index:idx_admin_action_name_time,priority:2" json:"created_at"`
}

// TableName 明确表名。
func (AdminActionLog) TableName() string { return "admin_action_logs" }

// ---------- AP-096 Quest / Objectives / Progress / Claim ----------

// 任务类型。
const (
	QuestTypeMain     = "main"
	QuestTypeResearch = "research"
	QuestTypeDaily    = "daily"
	QuestTypeWeekly   = "weekly"
	QuestTypeCity     = "city"
	QuestTypeEvent    = "event"
)

// 重置策略。
const (
	QuestResetNone   = "none"
	QuestResetDaily  = "daily"
	QuestResetWeekly = "weekly"
)

// 任务进度状态。
const (
	QuestStatusActive      = "active"
	QuestStatusCompleted   = "completed"
	QuestStatusClaimed     = "claimed"
	QuestStatusExpired     = "expired"
	QuestStatusCompensated = "compensated"
)

// 可信业务事件（禁止 page_view / open_pokedex / safe_explore 等客户端可伪造事件）。
const (
	QuestEventCaptureSuccess  = "capture_success"
	QuestEventSpeciesNew      = "species_new"
	QuestEventBattleComplete  = "battle_complete"
	QuestEventDispatchDone    = "dispatch_complete"
	QuestEventVisitCity       = "visit_city"
	QuestEventSeasonCheckin   = "season_checkin"
	QuestEventResearchNote    = "research_note"
	QuestEventCollectionCount = "collection_count"
)

// QuestDefinition 数据驱动任务定义（配置种子 + 可选热更版本）。
// ObjectivesJSON / RewardsJSON / PrerequisitesJSON 存 JSON 文本。
type QuestDefinition struct {
	ID                uint   `gorm:"primaryKey" json:"id"`
	QuestID           string `gorm:"uniqueIndex;size:64;not null" json:"quest_id"`
	Type              string `gorm:"index;size:16;not null" json:"type"` // main|research|daily|weekly|city|event
	Title             string `gorm:"size:128;not null" json:"title"`
	Description       string `gorm:"size:512" json:"description"`
	ObjectivesJSON    string `gorm:"type:text;not null" json:"objectives_json"`
	RewardsJSON       string `gorm:"type:text;not null" json:"rewards_json"`
	PrerequisitesJSON string `gorm:"type:text" json:"prerequisites_json,omitempty"`
	ResetPolicy       string `gorm:"size:16;not null;default:none" json:"reset_policy"` // none|daily|weekly
	Free              bool   `gorm:"not null;default:false" json:"free"`                // 零体力可执行
	Enabled           bool   `gorm:"not null;default:true;index" json:"enabled"`
	ConfigVersion     string `gorm:"size:32;not null;default:v1" json:"config_version"`
	MinLevel          int    `gorm:"not null;default:1" json:"min_level"`
	SortOrder         int    `gorm:"not null;default:0" json:"sort_order"`
	// DurationHours >0 时，进度从首次激活起算过期；0 表示仅按 period 边界。
	DurationHours int `gorm:"not null;default:0" json:"duration_hours"`
	// StartsAt / EndsAt 活动窗口（event 类型常用）。
	StartsAt  *time.Time `json:"starts_at,omitempty"`
	EndsAt    *time.Time `json:"ends_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (QuestDefinition) TableName() string { return "quest_definitions" }

// QuestProgress 玩家任务进度（owner + quest + period 唯一）。
// ProgressJSON 形如 {"obj_id": currentCount}；复合目标全部达标后 status=completed。
type QuestProgress struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	OwnerKey      string     `gorm:"uniqueIndex:idx_quest_prog_owner_period,priority:1;size:68;not null" json:"owner_key"`
	QuestID       string     `gorm:"uniqueIndex:idx_quest_prog_owner_period,priority:2;size:64;not null" json:"quest_id"`
	PeriodKey     string     `gorm:"uniqueIndex:idx_quest_prog_owner_period,priority:3;size:32;not null" json:"period_key"`
	DeviceID      string     `gorm:"index;size:64;not null" json:"device_id"`
	AccountID     string     `gorm:"index;size:36" json:"account_id,omitempty"`
	ProgressJSON  string     `gorm:"type:text;not null" json:"progress_json"`
	Status        string     `gorm:"index;size:16;not null;default:active" json:"status"`
	ConfigVersion string     `gorm:"size:32;not null;default:v1" json:"config_version"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	ClaimedAt     *time.Time `json:"claimed_at,omitempty"`
	ExpiresAt     *time.Time `gorm:"index" json:"expires_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (QuestProgress) TableName() string { return "quest_progress" }

// ---------- AP-098 Photography quality skill ----------

// PhotoDeviceCalibration stores per-owner device sensor baseline (AP-098).
type PhotoDeviceCalibration struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	OwnerKey             string    `gorm:"uniqueIndex;size:68;not null" json:"owner_key"`
	DeviceID             string    `gorm:"index;size:64;not null" json:"device_id"`
	AccountID            string    `gorm:"index;size:36" json:"account_id,omitempty"`
	BaselineStabilityRMS float64   `gorm:"not null;default:0.08" json:"baseline_stability_rms"`
	LightingOffset       float64   `gorm:"not null;default:0" json:"lighting_offset"`
	SampleCount          int       `gorm:"not null;default:0" json:"sample_count"`
	Calibrated           bool      `gorm:"not null;default:false" json:"calibrated"`
	DeviceModel          string    `gorm:"size:64" json:"device_model,omitempty"`
	ConfigVersion        string    `gorm:"size:32;not null;default:photo-quality-v1" json:"config_version"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// TableName 明确表名。
func (PhotoDeviceCalibration) TableName() string { return "photo_device_calibrations" }

// PhotoScoreRecord persists a scored observation attempt (anti-farm + personal best).
type PhotoScoreRecord struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ScoreID        string    `gorm:"uniqueIndex;size:36;not null" json:"score_id"`
	OwnerKey       string    `gorm:"index:idx_photo_score_owner_day,priority:1;size:68;not null" json:"owner_key"`
	DeviceID       string    `gorm:"index;size:64;not null" json:"device_id"`
	AccountID      string    `gorm:"index;size:36" json:"account_id,omitempty"`
	DayKey         string    `gorm:"index:idx_photo_score_owner_day,priority:2;size:10;not null" json:"day_key"` // YYYY-MM-DD UTC
	Overall        float64   `gorm:"not null" json:"overall"`
	Band           string    `gorm:"size:16;not null" json:"band"`
	Stability      float64   `gorm:"not null" json:"stability"`
	Completeness   float64   `gorm:"not null" json:"subject_completeness"`
	Lighting       float64   `gorm:"not null" json:"lighting"`
	Occlusion      float64   `gorm:"not null" json:"occlusion"`
	Composition    float64   `gorm:"not null" json:"composition"`
	SafeDistance   float64   `gorm:"not null" json:"safe_distance"`
	ChasePenalty   bool      `gorm:"not null;default:false" json:"chase_penalty"`
	RarityEligible bool      `gorm:"not null;default:true" json:"rarity_eligible"`
	MetricsDigest  string    `gorm:"index;size:32;not null" json:"metrics_digest"`
	Signature      string    `gorm:"size:128;not null" json:"signature"`
	ConfigVersion  string    `gorm:"size:32;not null" json:"config_version"`
	ThemeID        string    `gorm:"size:32" json:"theme_id,omitempty"`
	A11yCompleted  bool      `gorm:"not null;default:false" json:"a11y_completed"`
	CreatedAt      time.Time `gorm:"index" json:"created_at"`
}

// TableName 明确表名。
func (PhotoScoreRecord) TableName() string { return "photo_score_records" }

// PhotoPersonalBest tracks best overall and per-dimension scores.
type PhotoPersonalBest struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	OwnerKey     string    `gorm:"uniqueIndex;size:68;not null" json:"owner_key"`
	DeviceID     string    `gorm:"index;size:64;not null" json:"device_id"`
	AccountID    string    `gorm:"index;size:36" json:"account_id,omitempty"`
	Overall      float64   `gorm:"not null;default:0" json:"overall"`
	Stability    float64   `gorm:"not null;default:0" json:"stability"`
	Completeness float64   `gorm:"not null;default:0" json:"subject_completeness"`
	Lighting     float64   `gorm:"not null;default:0" json:"lighting"`
	Occlusion    float64   `gorm:"not null;default:0" json:"occlusion"`
	Composition  float64   `gorm:"not null;default:0" json:"composition"`
	SafeDistance float64   `gorm:"not null;default:0" json:"safe_distance"`
	BestScoreID  string    `gorm:"size:36" json:"best_score_id,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName 明确表名。
func (PhotoPersonalBest) TableName() string { return "photo_personal_bests" }

// PhotoThemeProgress daily photography theme / a11y alternative progress.
type PhotoThemeProgress struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	OwnerKey      string     `gorm:"uniqueIndex:idx_photo_theme,priority:1;size:68;not null" json:"owner_key"`
	DayKey        string     `gorm:"uniqueIndex:idx_photo_theme,priority:2;size:10;not null" json:"day_key"`
	ThemeID       string     `gorm:"size:32;not null" json:"theme_id"`
	DeviceID      string     `gorm:"index;size:64;not null" json:"device_id"`
	AccountID     string     `gorm:"index;size:36" json:"account_id,omitempty"`
	Completed     bool       `gorm:"not null;default:false" json:"completed"`
	A11yCompleted bool       `gorm:"not null;default:false" json:"a11y_completed"`
	BestDimScore  float64    `gorm:"not null;default:0" json:"best_dim_score"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (PhotoThemeProgress) TableName() string { return "photo_theme_progress" }

// QuestClaim 领取记录；operation_id 与钱包流水一致，保证奖励恰好一次。
type QuestClaim struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ClaimID     string    `gorm:"uniqueIndex;size:36;not null" json:"claim_id"`
	OperationID string    `gorm:"uniqueIndex;size:128;not null" json:"operation_id"`
	OwnerKey    string    `gorm:"index:idx_quest_claim_owner,priority:1;size:68;not null" json:"owner_key"`
	QuestID     string    `gorm:"index:idx_quest_claim_owner,priority:2;size:64;not null" json:"quest_id"`
	PeriodKey   string    `gorm:"size:32;not null" json:"period_key"`
	DeviceID    string    `gorm:"index;size:64;not null" json:"device_id"`
	AccountID   string    `gorm:"index;size:36" json:"account_id,omitempty"`
	Status      string    `gorm:"size:16;not null" json:"status"` // claimed|compensated
	RewardsJSON string    `gorm:"type:text" json:"rewards_json,omitempty"`
	GoldGranted int64     `gorm:"not null;default:0" json:"gold_granted"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName 明确表名。
func (QuestClaim) TableName() string { return "quest_claims" }

// QuestEventLog 可信事件幂等日志（event_id 全局唯一）。
type QuestEventLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	EventID   string    `gorm:"uniqueIndex;size:128;not null" json:"event_id"`
	OwnerKey  string    `gorm:"index:idx_quest_event_owner_time,priority:1;size:68;not null" json:"owner_key"`
	DeviceID  string    `gorm:"index;size:64;not null" json:"device_id"`
	AccountID string    `gorm:"index;size:36" json:"account_id,omitempty"`
	EventType string    `gorm:"size:32;not null" json:"event_type"`
	Payload   string    `gorm:"type:text" json:"payload,omitempty"`
	AppliedAt time.Time `gorm:"index:idx_quest_event_owner_time,priority:2" json:"applied_at"`
}

// TableName 明确表名。
func (QuestEventLog) TableName() string { return "quest_event_logs" }

// ---------- AP-099 Researcher Growth + Virtual Companion ----------

// 研究员成长轨道。
const (
	GrowthTrackPhotography     = "photography"
	GrowthTrackEcology         = "ecology"
	GrowthTrackSafeObservation = "safe_observation"
)

// 成长事件类型（服务端权威，幂等 event_id）。
const (
	GrowthEventPhotoCapture      = "photo_capture"
	GrowthEventPhotoQuality      = "photo_quality"
	GrowthEventSpeciesFirst      = "species_first"
	GrowthEventSpeciesResearch   = "species_research"
	GrowthEventSafeExplore       = "safe_explore"
	GrowthEventDistanceRespect   = "distance_respect"
	GrowthEventCompanionInteract = "companion_interact"
	GrowthEventCompanionMemory   = "companion_memory"
	GrowthEventCompanionDecor    = "companion_decor"
)

// 成长配置版本（迁移/重置可审计）。
const GrowthConfigVersion = "growth.v1"

// ResearcherTrack 玩家研究员成长轨道快照（摄影 / 生态知识 / 安全观察）。
// owner_key = "acc:<id>" | "dev:<id>"，绑定账号后跨设备共享。
type ResearcherTrack struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	OwnerKey      string    `gorm:"uniqueIndex:idx_research_owner_track,priority:1;size:68;not null" json:"owner_key"`
	Track         string    `gorm:"uniqueIndex:idx_research_owner_track,priority:2;size:32;not null" json:"track"`
	DeviceID      string    `gorm:"index;size:64;not null" json:"device_id"`
	AccountID     string    `gorm:"index;size:36" json:"account_id,omitempty"`
	XP            int64     `gorm:"not null;default:0" json:"xp"`
	Level         int       `gorm:"not null;default:0" json:"level"`
	ConfigVersion string    `gorm:"size:32;not null;default:growth.v1" json:"config_version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (ResearcherTrack) TableName() string { return "researcher_tracks" }

// GrowthEvent 不可变成长事件流水（跨设备恢复权威源）。
type GrowthEvent struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	EventID       string    `gorm:"uniqueIndex;size:128;not null" json:"event_id"`
	OwnerKey      string    `gorm:"index:idx_growth_event_owner_time,priority:1;size:68;not null" json:"owner_key"`
	DeviceID      string    `gorm:"index;size:64;not null" json:"device_id"`
	AccountID     string    `gorm:"index;size:36" json:"account_id,omitempty"`
	Kind          string    `gorm:"size:32;not null;index" json:"kind"`
	Track         string    `gorm:"size:32" json:"track,omitempty"` // photography|ecology|safe_observation|companion
	AnimalUUID    string    `gorm:"index;size:36" json:"animal_uuid,omitempty"`
	DeltaXP       int64     `gorm:"not null;default:0" json:"delta_xp"`
	XPAfter       int64     `gorm:"not null;default:0" json:"xp_after"`
	LevelAfter    int       `gorm:"not null;default:0" json:"level_after"`
	NodeID        string    `gorm:"size:64" json:"node_id,omitempty"`
	SourceType    string    `gorm:"size:32" json:"source_type,omitempty"`
	SourceID      string    `gorm:"size:128" json:"source_id,omitempty"`
	Metadata      string    `gorm:"type:text" json:"metadata,omitempty"`
	ConfigVersion string    `gorm:"size:32;not null" json:"config_version"`
	CreatedAt     time.Time `gorm:"index:idx_growth_event_owner_time,priority:2" json:"created_at"`
}

func (GrowthEvent) TableName() string { return "growth_events" }

// CompanionProfile 收藏对象纯虚拟伙伴档案（装饰成长，不改战斗力）。
type CompanionProfile struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	AnimalUUID    string    `gorm:"uniqueIndex;size:36;not null" json:"animal_uuid"`
	OwnerKey      string    `gorm:"index:idx_companion_owner,priority:1;size:68;not null" json:"owner_key"`
	DeviceID      string    `gorm:"index;size:64;not null" json:"device_id"`
	AccountID     string    `gorm:"index;size:36" json:"account_id,omitempty"`
	BondXP        int64     `gorm:"not null;default:0" json:"bond_xp"`
	BondLevel     int       `gorm:"not null;default:0" json:"bond_level"`
	DecorStage    int       `gorm:"not null;default:0" json:"decor_stage"` // 装饰阶段，非战力
	Title         string    `gorm:"size:64" json:"title,omitempty"`
	ConfigVersion string    `gorm:"size:32;not null;default:growth.v1" json:"config_version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (CompanionProfile) TableName() string { return "companion_profiles" }

// CompanionMemoryNode 伙伴可见成长节点（每收藏至少 3 个可见节点）。
type CompanionMemoryNode struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	AnimalUUID string     `gorm:"uniqueIndex:idx_companion_node,priority:1;size:36;not null" json:"animal_uuid"`
	NodeID     string     `gorm:"uniqueIndex:idx_companion_node,priority:2;size:64;not null" json:"node_id"`
	OwnerKey   string     `gorm:"index;size:68;not null" json:"owner_key"`
	Title      string     `gorm:"size:64;not null" json:"title"`
	Kind       string     `gorm:"size:32;not null" json:"kind"` // memory|decor|journal
	Visible    bool       `gorm:"not null;default:true" json:"visible"`
	Unlocked   bool       `gorm:"not null;default:false" json:"unlocked"`
	UnlockAtXP int64      `gorm:"not null;default:0" json:"unlock_at_xp"`
	UnlockedAt *time.Time `json:"unlocked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (CompanionMemoryNode) TableName() string { return "companion_memory_nodes" }

// GrowthResetAudit 成长重置/迁移审计（可审计、可回放）。
type GrowthResetAudit struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	AuditID      string    `gorm:"uniqueIndex;size:36;not null" json:"audit_id"`
	OwnerKey     string    `gorm:"index;size:68;not null" json:"owner_key"`
	DeviceID     string    `gorm:"index;size:64;not null" json:"device_id"`
	AccountID    string    `gorm:"index;size:36" json:"account_id,omitempty"`
	Scope        string    `gorm:"size:32;not null" json:"scope"` // researcher|companion|all
	AnimalUUID   string    `gorm:"size:36" json:"animal_uuid,omitempty"`
	Reason       string    `gorm:"size:256;not null" json:"reason"`
	FromVersion  string    `gorm:"size:32" json:"from_version,omitempty"`
	ToVersion    string    `gorm:"size:32" json:"to_version,omitempty"`
	SnapshotJSON string    `gorm:"type:longtext" json:"snapshot_json,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func (GrowthResetAudit) TableName() string   { return "growth_reset_audits" }
func (PhotoThemeProgress) TableName() string { return "photo_theme_progress" }
