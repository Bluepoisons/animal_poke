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
	animalRepo    *repo.AnimalRepo
	auditService  *services.AuditService
	inferenceRepo *repo.InferenceRepo
	db            *gorm.DB
}

// NewSyncHandler 构造 SyncHandler。
func NewSyncHandler(animalRepo *repo.AnimalRepo, auditService *services.AuditService) *SyncHandler {
	return &SyncHandler{
		animalRepo:   animalRepo,
		auditService: auditService,
		db:           animalRepo.DB(),
	}
}

// NewSyncHandlerFull 完整构造。
func NewSyncHandlerFull(animalRepo *repo.AnimalRepo, auditService *services.AuditService, inf *repo.InferenceRepo) *SyncHandler {
	h := NewSyncHandler(animalRepo, auditService)
	h.inferenceRepo = inf
	return h
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
	City               string  `json:"city"`
	GeneratedAt        string  `json:"generated_at" binding:"required"`
	InferenceRequestID string  `json:"inference_request_id"`
	KeepPrecise        bool    `json:"keep_precise_location"`
}

type syncResponse struct {
	Status       string `json:"status"`
	UUID         string `json:"uuid"`
	ReviewStatus string `json:"review_status,omitempty"`
}

// SyncAnimal POST /sync/animal 接收客户端上传的动物元数据。
func (h *SyncHandler) SyncAnimal(c *gin.Context) {
	var req syncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: uuid, species, rarity, generated_at are required"})
		return
	}

	deviceID := middleware.GetDeviceID(c)

	generatedAt, err := time.Parse(time.RFC3339, req.GeneratedAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid generated_at format, use RFC3339"})
		return
	}

	exists, err := h.animalRepo.ExistsByUUID(req.UUID)
	if err != nil {
		slog.Error("去重查询失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sync failed"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{
			"error": "animal already exists",
			"uuid": req.UUID,
			"reason_code": "duplicate_animal",
		})
		return
	}

	// 位置最小化
	lat := services.RoundCoord(req.Latitude)
	lng := services.RoundCoord(req.Longitude)
	geohash := ""
	if req.Latitude != 0 || req.Longitude != 0 {
		geohash = services.EncodeGeoHash(req.Latitude, req.Longitude)
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
		City:               req.City,
		GeoHash:            geohash,
		Latitude:           lat,
		Longitude:          lng,
		GeneratedAt:        generatedAt,
		InferenceRequestID: req.InferenceRequestID,
		ServerVersion:      time.Now().UTC().UnixNano(),
	}
	if req.KeepPrecise && (req.Latitude != 0 || req.Longitude != 0) {
		pl, pg := req.Latitude, req.Longitude
		exp := time.Now().UTC().Add(24 * time.Hour)
		animal.PreciseLat = &pl
		animal.PreciseLng = &pg
		animal.PreciseExpiresAt = &exp
	}

	// 事务：消费 inference + 落库 + 审计
	var review string
	err = h.db.Transaction(func(tx *gorm.DB) error {
		animalRepo := h.animalRepo.WithTx(tx)
		auditRepo := repo.NewAuditLogRepo(tx)
		audit := h.auditService.WithTx(animalRepo, auditRepo)

		if req.InferenceRequestID != "" && h.inferenceRepo != nil {
			if _, err := h.inferenceRepo.WithTx(tx).Consume(tx, req.InferenceRequestID, deviceID); err != nil {
				// 允许 value 凭证缺失时仅告警路径；严格模式拒绝
				return err
			}
		}

		check := audit.CheckAnomaly(deviceID, animal)
		review = check.ReviewStatus

		if err := animalRepo.Create(animal); err != nil {
			return err
		}
		audit.LogSync(deviceID, animal)
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || isDuplicate(err) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "animal already exists",
				"uuid": req.UUID,
				"reason_code": "duplicate_animal",
			})
			return
		}
		if req.InferenceRequestID != "" {
			msg := err.Error()
			switch {
			case msg == "inference already consumed" || contains(msg, "already consumed"):
				c.JSON(http.StatusConflict, gin.H{"error": "inference already consumed", "reason_code": "inference_consumed"})
				return
			case contains(msg, "expired"):
				c.JSON(http.StatusConflict, gin.H{"error": "inference expired", "reason_code": "inference_expired"})
				return
			case msg == "inference not successful" || errors.Is(err, gorm.ErrRecordNotFound) || contains(msg, "inference"):
				c.JSON(http.StatusConflict, gin.H{"error": "invalid or reused inference", "reason_code": "inference_invalid"})
				return
			}
		}
		slog.Error("动物同步失败", "uuid", req.UUID, "device_id", deviceID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sync failed"})
		return
	}

	slog.Info("动物同步成功", "uuid", req.UUID, "device_id", deviceID, "species", req.Species, "rarity", req.Rarity)
	c.JSON(http.StatusCreated, syncResponse{
		Status:       "synced",
		UUID:         req.UUID,
		ReviewStatus: review,
	})
}

// BatchSyncRequest 批量推送。
type BatchSyncRequest struct {
	Items []syncRequest `json:"items" binding:"required"`
}

// BatchSyncResponse 批量结果。
type BatchSyncResponse struct {
	Results []batchItemResult `json:"results"`
}

type batchItemResult struct {
	UUID   string `json:"uuid"`
	Status string `json:"status"` // synced|conflict|error
	Error  string `json:"error,omitempty"`
}

// SyncAnimalsBatch POST /sync/animals
func (h *SyncHandler) SyncAnimalsBatch(c *gin.Context) {
	var req BatchSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "items required"})
		return
	}
	if len(req.Items) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max 100 items per batch"})
		return
	}
	results := make([]batchItemResult, 0, len(req.Items))
	for _, item := range req.Items {
		// 复用单条逻辑：构造临时 context 调用较重，这里内联简化
		c.Set("batch_item", item)
		// 直接调用内部
		status, errMsg := h.syncOne(middleware.GetDeviceID(c), item)
		results = append(results, batchItemResult{UUID: item.UUID, Status: status, Error: errMsg})
	}
	c.JSON(http.StatusOK, BatchSyncResponse{Results: results})
}

func (h *SyncHandler) syncOne(deviceID string, req syncRequest) (string, string) {
	generatedAt, err := time.Parse(time.RFC3339, req.GeneratedAt)
	if err != nil {
		return "error", "invalid generated_at"
	}
	exists, err := h.animalRepo.ExistsByUUID(req.UUID)
	if err != nil {
		return "error", "db error"
	}
	if exists {
		return "conflict", "already exists"
	}
	animal := &models.Animal{
		UUID: req.UUID, DeviceID: deviceID, Species: req.Species, Breed: req.Breed,
		Rarity: req.Rarity, HP: req.HP, ATK: req.ATK, DEF: req.DEF, SPD: req.SPD,
		Class: req.Class, Element: req.Element, City: req.City,
		Latitude: services.RoundCoord(req.Latitude), Longitude: services.RoundCoord(req.Longitude),
		GeoHash:     services.EncodeGeoHash(req.Latitude, req.Longitude),
		GeneratedAt: generatedAt, InferenceRequestID: req.InferenceRequestID,
		ServerVersion: time.Now().UTC().UnixNano(),
	}
	err = h.db.Transaction(func(tx *gorm.DB) error {
		ar := h.animalRepo.WithTx(tx)
		auditRepo := repo.NewAuditLogRepo(tx)
		audit := h.auditService.WithTx(ar, auditRepo)
		if req.InferenceRequestID != "" && h.inferenceRepo != nil {
			if _, err := h.inferenceRepo.WithTx(tx).Consume(tx, req.InferenceRequestID, deviceID); err != nil {
				return err
			}
		}
		_ = audit.CheckAnomaly(deviceID, animal)
		if err := ar.Create(animal); err != nil {
			return err
		}
		audit.LogSync(deviceID, animal)
		return nil
	})
	if err != nil {
		if isDuplicate(err) {
			return "conflict", "already exists"
		}
		return "error", err.Error()
	}
	return "synced", ""
}

// PullAnimals GET /sync/animals?since_version=
func (h *SyncHandler) PullAnimals(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	var since int64
	if v := c.Query("since_version"); v != "" {
		if _, err := parseInt64(v, &since); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid since_version"})
			return
		}
	}
	limit := 50
	items, err := h.animalRepo.ListSinceVersion(deviceID, since, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pull failed"})
		return
	}
	// 脱敏：不返回精确坐标
	for i := range items {
		items[i].PreciseLat = nil
		items[i].PreciseLng = nil
		items[i].PreciseExpiresAt = nil
	}
	var next int64
	if len(items) > 0 {
		next = items[len(items)-1].ServerVersion
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "next_version": next})
}

func isDuplicate(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return errors.Is(err, gorm.ErrDuplicatedKey) ||
		contains(s, "UNIQUE") || contains(s, "unique") || contains(s, "Duplicate")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && (stringIndex(s, sub) >= 0)))
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func parseInt64(s string, out *int64) (int64, error) {
	var n int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			if ch == '-' && n == 0 {
				continue
			}
			return 0, errors.New("not int")
		}
		n = n*10 + int64(ch-'0')
	}
	*out = n
	return n, nil
}
