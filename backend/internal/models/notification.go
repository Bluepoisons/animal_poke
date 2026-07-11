package models

import "time"

// Inbox categories.
const (
	NotifCategorySecurity  = "security"
	NotifCategoryPrivacy   = "privacy"
	NotifCategoryRefund    = "refund"
	NotifCategoryReport    = "report"
	NotifCategoryEvent     = "event"
	NotifCategoryMarketing = "marketing"
)

// Outbox delivery states.
const (
	OutboxPending   = "pending"
	OutboxSending   = "sending"
	OutboxDelivered = "delivered"
	OutboxFailed    = "failed"
	OutboxSkipped   = "skipped" // quiet hours / marketing opt-out
)

// InboxMessage is the user-visible notification (exactly-once display via id).
type InboxMessage struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	OwnerKey    string     `gorm:"index:idx_inbox_owner_cursor,priority:1;size:80;not null" json:"owner_key"`
	Category    string     `gorm:"size:32;not null;index" json:"category"`
	Title       string     `gorm:"size:160;not null" json:"title"`
	Body        string     `gorm:"type:text;not null" json:"body"`
	DedupeKey   string     `gorm:"uniqueIndex;size:128;not null" json:"dedupe_key"`
	Sensitive   bool       `gorm:"not null;default:false" json:"sensitive"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	AckedAt     *time.Time `json:"acked_at,omitempty"`
	CreatedAt   time.Time  `gorm:"index:idx_inbox_owner_cursor,priority:2" json:"created_at"`
	PayloadJSON string     `gorm:"type:text" json:"payload_json,omitempty"`
}

func (InboxMessage) TableName() string { return "inbox_messages" }

// NotificationOutbox is the transactional outbox for async push delivery.
type NotificationOutbox struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	OwnerKey      string     `gorm:"index;size:80;not null" json:"owner_key"`
	InboxID       uint       `gorm:"index;not null" json:"inbox_id"`
	Category      string     `gorm:"size:32;not null" json:"category"`
	DedupeKey     string     `gorm:"uniqueIndex;size:128;not null" json:"dedupe_key"`
	State         string     `gorm:"size:16;not null;default:pending;index" json:"state"`
	Attempts      int        `gorm:"not null;default:0" json:"attempts"`
	NextAttemptAt time.Time  `gorm:"column:next_attempt_at;index" json:"next_attempt_at"`
	LastError     string     `gorm:"size:256" json:"last_error,omitempty"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (NotificationOutbox) TableName() string { return "notification_outbox" }

// PushDeviceToken stores provider tokens with lifecycle.
type PushDeviceToken struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	OwnerKey   string     `gorm:"uniqueIndex:idx_push_owner_token,priority:1;size:80;not null" json:"owner_key"`
	Token      string     `gorm:"uniqueIndex:idx_push_owner_token,priority:2;size:256;not null" json:"token"`
	Platform   string     `gorm:"size:16;not null" json:"platform"` // ios|android|web
	Active     bool       `gorm:"not null;default:true;index" json:"active"`
	LastSeenAt time.Time  `json:"last_seen_at"`
	DisabledAt *time.Time `json:"disabled_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (PushDeviceToken) TableName() string { return "push_device_tokens" }

// NotificationPreference stores quiet hours and marketing consent.
type NotificationPreference struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	OwnerKey         string    `gorm:"uniqueIndex;size:80;not null" json:"owner_key"`
	MarketingConsent bool      `gorm:"not null;default:false" json:"marketing_consent"`
	Minor            bool      `gorm:"not null;default:false" json:"minor"`
	QuietStartHour   int       `gorm:"not null;default:22" json:"quiet_start_hour"` // local 0-23
	QuietEndHour     int       `gorm:"not null;default:8" json:"quiet_end_hour"`
	Timezone         string    `gorm:"size:64;not null;default:Asia/Shanghai" json:"timezone"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedAt        time.Time `json:"created_at"`
}

func (NotificationPreference) TableName() string { return "notification_preferences" }
