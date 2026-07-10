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
	ServerVersion      int64      `gorm:"not null;default:1" json:"server_version"`
	DeletedAt          *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
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
	DeviceID       string     `gorm:"index;size:64;not null" json:"device_id"`
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
type AccountBinding struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	AccountID       string    `gorm:"index;size:36;not null" json:"account_id"`
	Provider        string    `gorm:"uniqueIndex:idx_binding_provider_subject,priority:1;size:32;not null" json:"provider"` // email|mock_oauth|apple|google
	ProviderSubject string    `gorm:"uniqueIndex:idx_binding_provider_subject,priority:2;size:191;not null" json:"provider_subject"`
	CredentialHash  string    `gorm:"size:128;not null" json:"-"`
	Verified        bool      `gorm:"not null;default:true" json:"verified"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName 明确表名。
func (AccountBinding) TableName() string { return "account_bindings" }

// DeviceAccount 设备与账号的关联（支持多设备、撤销）。
type DeviceAccount struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	DeviceID         string     `gorm:"uniqueIndex;size:64;not null" json:"device_id"`
	AccountID        string     `gorm:"index;size:36;not null" json:"account_id"`
	Status           string     `gorm:"size:16;not null;default:active" json:"status"` // active|revoked
	RefreshTokenHash string     `gorm:"size:128" json:"-"` // 刷新令牌哈希
	RefreshExpiresAt *time.Time `json:"refresh_expires_at,omitempty"`
	LinkedAt         time.Time  `json:"linked_at"`
	LastSeenAt       *time.Time `json:"last_seen_at,omitempty"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// TableName 明确表名。
func (DeviceAccount) TableName() string { return "device_accounts" }

