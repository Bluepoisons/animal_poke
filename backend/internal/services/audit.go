// Package services MB4: 反作弊审计服务。
package services

import (
	"log/slog"
	"time"

	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
)

// AuditService 反作弊审计服务。
type AuditService struct {
	animalRepo  *repo.AnimalRepo
	auditRepo   *repo.AuditLogRepo
}

// NewAuditService 构造 AuditService。
func NewAuditService(animalRepo *repo.AnimalRepo, auditRepo *repo.AuditLogRepo) *AuditService {
	return &AuditService{
		animalRepo: animalRepo,
		auditRepo:  auditRepo,
	}
}

// CheckAnomaly 检查设备是否存在异常行为, 发现异常时记录审计日志并返回告警列表。
// 规则:
// 1. 同设备 10 分钟内 >3 只传说级(5 星) → 告警
// 2. 推理请求 ID 被复用(同一 ID 对应不同动物 UUID) → 告警
func (s *AuditService) CheckAnomaly(deviceID string, animal *models.Animal) []string {
	var alerts []string

	// 规则 1: 高稀有度频次异常
	if animal.Rarity >= 5 {
		since := time.Now().Add(-10 * time.Minute)
		count, err := s.animalRepo.CountRecentHighRarity(deviceID, 5, since)
		if err != nil {
			slog.Error("反作弊统计失败", "err", err)
		} else if count >= 2 { // 已有 2 只 + 当前 = 3 只 → 告警
			msg := "高稀有度频次异常: 10 分钟内 >=3 只传说级(5星)动物"
			alerts = append(alerts, msg)
			s.saveAlert(deviceID, "anomaly", msg, animal.InferenceRequestID, "")
			slog.Warn("反作弊告警", "device_id", deviceID, "message", msg)
		}
	}

	// 规则 2: 推理请求 ID 复用
	if animal.InferenceRequestID != "" {
		existing, err := s.animalRepo.FindByInferenceRequestID(animal.InferenceRequestID)
		if err != nil {
			slog.Error("推理请求 ID 查询失败", "err", err)
		} else if len(existing) > 0 {
			msg := "推理请求 ID 复用: 同一 inference_request_id 对应多只动物"
			alerts = append(alerts, msg)
			s.saveAlert(deviceID, "anomaly", msg, animal.InferenceRequestID, "")
			slog.Warn("反作弊告警", "device_id", deviceID, "message", msg, "inference_request_id", animal.InferenceRequestID)
		}
	}

	return alerts
}

// LogSync 记录正常同步审计日志。
func (s *AuditService) LogSync(deviceID string, animal *models.Animal) {
	s.saveAlert(deviceID, "sync", "动物同步",
		animal.InferenceRequestID, animal.UUID)
}

func (s *AuditService) saveAlert(deviceID, logType, msg, requestID, metadata string) {
	log := &models.AuditLog{
		DeviceID:           deviceID,
		Type:               logType,
		Message:            msg,
		InferenceRequestID: requestID,
		Metadata:           metadata,
	}
	if err := s.auditRepo.Create(log); err != nil {
		slog.Error("审计日志写入失败", "err", err)
	}
}
