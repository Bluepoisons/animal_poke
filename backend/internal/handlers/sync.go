// Package handlers MB4: 同步服务处理函数。
package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SyncHandler 动物同步处理器。
type SyncHandler struct {
	animalRepo   *repo.AnimalRepo
	auditService *services.AuditService
}

// NewSyncHandler 构造 SyncHandler。
func NewSyncHandler(animalRepo *repo.AnimalRepo, auditService *services.AuditService) *SyncHandler {
	return &SyncHandler{
		animalRepo:   animalRepo,
		auditService: auditService,
	}
}

type syncRequest struct {
	UUID               string  `json:"uuid" binding:"required"`
	Species            string  `json:"species" binding:"required"`
	Breed              string  `json:"breed"`
	Rarity             int     `json:"rarity" binding:"required"`
	HP                 int     `json:"hp"`
	ATK                int     `json:"atk"`
	DEF                int     `json:"def"`
	SPD                int     `json:"spd"`
	Class              string  `json:"class"`
	Element            string  `json:"element"`
	Latitude           float64 `json:"latitude"`
	Longitude          float64 `json:"longitude"`
	GeneratedAt        string  `json:"generated_at" binding:"required"`
	InferenceRequestID string  `json:"inference_request_id"`
}

type syncResponse struct {
	Status  string   `json:"status"`
	UUID    string   `json:"uuid"`
	Alerts  []string `json:"alerts,omitempty"`
}

// SyncAnimal POST /sync/animal 接收客户端上传的动物元数据。
// 去重校验 + 反作弊审计 + 落库。
func (h *SyncHandler) SyncAnimal(c *gin.Context) {
	var req syncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: uuid, species, rarity, generated_at are required"})
		return
	}

	deviceID := middleware.GetDeviceID(c)

	// 解析生成时间
	generatedAt, err := time.Parse(time.RFC3339, req.GeneratedAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid generated_at format, use RFC3339"})
		return
	}

	// 去重检查
	if h.animalRepo.ExistsByUUID(req.UUID) {
		slog.Info("动物已存在, 跳过同步", "uuid", req.UUID, "device_id", deviceID)
		c.JSON(http.StatusConflict, gin.H{"error": "animal already exists", "uuid": req.UUID})
		return
	}

	animal := &models.Animal{
		UUID:               req.UUID,
		DeviceID:           deviceID,
		Species:            req.Species,
		Breed:              req.Breed,
		Rarity:             req.Rarity,
		HP:                 req.HP,
		ATK:                req.ATK,
		DEF:                req.DEF,
		SPD:                req.SPD,
		Class:              req.Class,
		Element:            req.Element,
		Latitude:           req.Latitude,
		Longitude:          req.Longitude,
		GeneratedAt:        generatedAt,
		InferenceRequestID: req.InferenceRequestID,
	}

	// 反作弊检查(不阻断同步, 仅告警)
	alerts := h.auditService.CheckAnomaly(deviceID, animal)

	// 落库
	if err := h.animalRepo.Create(animal); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.JSON(http.StatusConflict, gin.H{"error": "animal already exists", "uuid": req.UUID})
			return
		}
		slog.Error("动物同步落库失败", "uuid", req.UUID, "device_id", deviceID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sync failed"})
		return
	}

	// 记录同步审计日志
	h.auditService.LogSync(deviceID, animal)

	slog.Info("动物同步成功", "uuid", req.UUID, "device_id", deviceID, "species", req.Species, "rarity", req.Rarity)
	c.JSON(http.StatusCreated, syncResponse{
		Status: "synced",
		UUID:   req.UUID,
		Alerts: alerts,
	})
}
