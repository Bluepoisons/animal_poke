// Package services MB4: 反作弊审计服务。
package services

import (
	"log/slog"
	"time"

	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
)

// AuditRuleVersion 规则版本，写入审计可追溯。
const AuditRuleVersion = "anticheat-v2"

// AuditService 反作弊审计服务。
type AuditService struct {
	animalRepo *repo.AnimalRepo
	auditRepo  *repo.AuditLogRepo
}

// NewAuditService 构造 AuditService。
func NewAuditService(animalRepo *repo.AnimalRepo, auditRepo *repo.AuditLogRepo) *AuditService {
	return &AuditService{
		animalRepo: animalRepo,
		auditRepo:  auditRepo,
	}
}

// WithTx 事务绑定。
func (s *AuditService) WithTx(animal *repo.AnimalRepo, audit *repo.AuditLogRepo) *AuditService {
	return &AuditService{animalRepo: animal, auditRepo: audit}
}

// CheckResult 服务端风控结果（不向客户端泄露具体规则）。
type CheckResult struct {
	ReviewStatus string // ok|review
	RiskScore    int
	// InternalAlerts 仅服务端日志/审计
	InternalAlerts []string
}

// CheckAnomaly 基于服务端可信数据检查异常。
// 规则:
// 1. 同设备 10 分钟内 >=3 只传说级(5 星)（使用 CreatedAt）→ review
// 2. 同设备同 inference ID 复用 → review
func (s *AuditService) CheckAnomaly(deviceID string, animal *models.Animal) CheckResult {
	var alerts []string
	risk := 0

	if animal.Rarity >= 5 {
		since := time.Now().UTC().Add(-10 * time.Minute)
		count, err := s.animalRepo.CountRecentHighRarity(deviceID, 5, since)
		if err != nil {
			slog.Error("反作弊统计失败", "err", err)
		} else if count >= 2 {
			msg := "high_rarity_burst"
			alerts = append(alerts, msg)
			risk += 50
			s.saveAlert(deviceID, "anomaly", msg, animal.InferenceRequestID, AuditRuleVersion, risk)
			slog.Warn("反作弊告警", "device_id", deviceID, "rule", msg, "version", AuditRuleVersion)
		}
	}

	if animal.InferenceRequestID != "" {
		existing, err := s.animalRepo.FindByInferenceRequestID(deviceID, animal.InferenceRequestID)
		if err != nil {
			slog.Error("推理请求 ID 查询失败", "err", err)
		} else if len(existing) > 0 {
			msg := "inference_id_reuse"
			alerts = append(alerts, msg)
			risk += 80
			s.saveAlert(deviceID, "anomaly", msg, animal.InferenceRequestID, AuditRuleVersion, risk)
			slog.Warn("反作弊告警", "device_id", deviceID, "rule", msg, "version", AuditRuleVersion)
		}
	}

	status := "ok"
	if risk >= 50 {
		status = "review"
	}
	return CheckResult{ReviewStatus: status, RiskScore: risk, InternalAlerts: alerts}
}

// LogSync 记录正常同步审计日志。
func (s *AuditService) LogSync(deviceID string, animal *models.Animal) {
	s.saveAlert(deviceID, "sync", "animal_sync", animal.InferenceRequestID, animal.UUID, 0)
}

// LogCollection 记录收藏详情编辑/删除审计（AP-090）。
// action: get|patch|delete；metadata 为 uuid 或 JSON 摘要。
func (s *AuditService) LogCollection(deviceID, action, animalUUID, metadata string) {
	msg := "collection_" + action
	s.saveAlert(deviceID, "sync", msg, "", metadata, 0)
	_ = animalUUID
}

func (s *AuditService) saveAlert(deviceID, logType, msg, requestID, metadata string, risk int) {
	status := "open"
	if logType == "sync" {
		status = "closed"
	}
	log := &models.AuditLog{
		DeviceID:           deviceID,
		Type:               logType,
		Message:            msg,
		InferenceRequestID: requestID,
		Metadata:           metadata,
		RiskScore:          risk,
		Status:             status,
	}
	if err := s.auditRepo.Create(log); err != nil {
		slog.Error("审计日志写入失败", "err", err)
	}
}
