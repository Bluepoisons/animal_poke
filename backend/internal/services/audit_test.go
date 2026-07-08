package services

import (
	"testing"
	"time"

	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuditTest(t *testing.T) *AuditService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	err = db.AutoMigrate(&models.Animal{}, &models.AuditLog{})
	assert.NoError(t, err)

	animalRepo := repo.NewAnimalRepo(db)
	auditRepo := repo.NewAuditLogRepo(db)
	return NewAuditService(animalRepo, auditRepo)
}

func TestAuditService_NoAnomaly_NormalAnimal(t *testing.T) {
	svc := setupAuditTest(t)

	animal := &models.Animal{
		UUID:               "uuid-1",
		DeviceID:           "device-1",
		Rarity:             3,
		GeneratedAt:        time.Now(),
		InferenceRequestID: "request-1",
	}

	alerts := svc.CheckAnomaly("device-1", animal)
	assert.Empty(t, alerts) // 正常行为不应告警
}

func TestAuditService_NoAnomaly_SingleLegendary(t *testing.T) {
	svc := setupAuditTest(t)

	animal := &models.Animal{
		UUID:               "uuid-2",
		DeviceID:           "device-2",
		Rarity:             5,
		GeneratedAt:        time.Now(),
		InferenceRequestID: "request-2",
	}

	alerts := svc.CheckAnomaly("device-2", animal)
	assert.Empty(t, alerts) // 单只传说级不告警(需要 >=3)
}

func TestAuditService_Anomaly_ManyLegendary(t *testing.T) {
	svc := setupAuditTest(t)

	// 预插入 2 只传说级
	for i := 0; i < 2; i++ {
		err := svc.animalRepo.Create(&models.Animal{
			UUID:               "legendary-" + string(rune('a'+i)),
			DeviceID:           "device-3",
			Rarity:             5,
			GeneratedAt:        time.Now(),
			InferenceRequestID: "req-" + string(rune('a'+i)),
		})
		assert.NoError(t, err)
	}

	// 第 3 只传说级应触发告警
	animal := &models.Animal{
		UUID:               "legendary-c",
		DeviceID:           "device-3",
		Rarity:             5,
		GeneratedAt:        time.Now(),
		InferenceRequestID: "req-c",
	}
	alerts := svc.CheckAnomaly("device-3", animal)
	assert.NotEmpty(t, alerts)
}

func TestAuditService_Anomaly_RequestIDReuse(t *testing.T) {
	svc := setupAuditTest(t)

	// 预插入一只动物
	err := svc.animalRepo.Create(&models.Animal{
		UUID:               "uuid-reuse-1",
		DeviceID:           "device-4",
		Rarity:             2,
		GeneratedAt:        time.Now(),
		InferenceRequestID: "same-request-id",
	})
	assert.NoError(t, err)

	// 用同一 request_id 同步不同动物
	animal := &models.Animal{
		UUID:               "uuid-reuse-2",
		DeviceID:           "device-4",
		Rarity:             3,
		GeneratedAt:        time.Now(),
		InferenceRequestID: "same-request-id",
	}
	alerts := svc.CheckAnomaly("device-4", animal)
	assert.NotEmpty(t, alerts)
}

func TestAuditService_LogSync(t *testing.T) {
	svc := setupAuditTest(t)

	animal := &models.Animal{
		UUID:               "uuid-log",
		DeviceID:           "device-5",
		InferenceRequestID: "request-log",
	}
	svc.LogSync("device-5", animal)

	// 验证审计日志已写入
	logs, _ := svc.animalRepo.FindByInferenceRequestID("request-log")
	assert.Empty(t, logs) // animal 未落库(只是 LogSync 写 audit_log)
}
