// Package models 共享 GORM 数据模型。Device / Animal / AuditLog 供 repo 与 handler 使用。
package models

import (
	"time"
)

// Device 设备注册表。每个客户端设备对应一条记录。
type Device struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  string    `gorm:"uniqueIndex;size:64;not null" json:"device_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 明确表名。
func (Device) TableName() string { return "devices" }

// Animal 玩家同步上传的动物元数据。
type Animal struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	UUID               string    `gorm:"uniqueIndex;size:36;not null" json:"uuid"`
	DeviceID           string    `gorm:"index;size:64;not null" json:"device_id"`
	Species            string    `gorm:"size:32;not null" json:"species"`
	Breed              string    `gorm:"size:64" json:"breed"`
	Rarity             int       `gorm:"not null" json:"rarity"` // 1-5 星级
	HP                 int       `json:"hp"`
	ATK                int       `json:"atk"`
	DEF                int       `json:"def"`
	SPD                int       `json:"spd"`
	Class              string    `gorm:"size:32" json:"class"`
	Element            string    `gorm:"size:32" json:"element"`
	Latitude           float64   `json:"latitude"`
	Longitude          float64   `json:"longitude"`
	GeneratedAt        time.Time `gorm:"not null" json:"generated_at"`
	InferenceRequestID string    `gorm:"index;size:128" json:"inference_request_id"`
	CreatedAt          time.Time `json:"created_at"`
}

// TableName 明确表名。
func (Animal) TableName() string { return "animals" }

// AuditLog 反作弊审计日志。
type AuditLog struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	DeviceID           string    `gorm:"index;size:64;not null" json:"device_id"`
	Type               string    `gorm:"size:32;not null" json:"type"` // sync, anomaly, auth
	Message            string    `gorm:"type:text" json:"message"`
	InferenceRequestID string    `gorm:"index;size:128" json:"inference_request_id"`
	Metadata           string    `gorm:"type:text" json:"metadata"` // JSON 附加信息
	CreatedAt          time.Time `json:"created_at"`
}

// TableName 明确表名。
func (AuditLog) TableName() string { return "audit_logs" }
