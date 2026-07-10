// Package handlers MB4: 同步服务处理函数。
package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"
	"animalpoke/backend/internal/taxonomy"

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

	normSpecies, _ := taxonomy.Normalize(req.Species)
	if !taxonomy.Capturable(normSpecies) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "species not capturable", "reason_code": "species_unsupported", "species": normSpecies})
		return
	}
	req.Species = normSpecies

	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)

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
			"error":       "animal already exists",
			"uuid":        req.UUID,
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
		AccountID:          accountID,
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

	// 服务端权威：有 InferenceRepo 时强制 value inference
	if h.inferenceRepo != nil && req.InferenceRequestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "inference_request_id required", "reason_code": "inference_required"})
		return
	}

	// 事务：原子消费 value inference + 用服务端结果覆盖关键字段 + 落库 + 审计
	var review string
	err = h.db.Transaction(func(tx *gorm.DB) error {
		animalRepo := h.animalRepo.WithTx(tx)
		auditRepo := repo.NewAuditLogRepo(tx)
		audit := h.auditService.WithTx(animalRepo, auditRepo)

		if h.inferenceRepo != nil {
			inf, err := h.inferenceRepo.WithTx(tx).ConsumeValue(tx, req.InferenceRequestID, deviceID, req.Species)
			if err != nil {
				return err
			}
			// 从服务端 ResultJSON 构造权威字段，拒绝客户端重抽稀有度
			if inf.ResultJSON != "" {
				var auth struct {
					Species string `json:"species"`
					Breed   string `json:"breed"`
					Rarity  int    `json:"rarity"`
					HP      int    `json:"hp"`
					ATK     int    `json:"atk"`
					DEF     int    `json:"def"`
					SPD     int    `json:"spd"`
					Class   string `json:"class"`
					Element string `json:"element"`
				}
				if err := json.Unmarshal([]byte(inf.ResultJSON), &auth); err == nil {
					// 严格比对或覆盖：客户端伪造 legendary/超范围属性将被覆盖为服务端值
					if auth.Species != "" {
						animal.Species = auth.Species
					}
					if auth.Breed != "" {
						animal.Breed = auth.Breed
					}
					if auth.Rarity > 0 {
						animal.Rarity = auth.Rarity
					}
					if auth.HP > 0 {
						animal.HP = auth.HP
					}
					if auth.ATK > 0 {
						animal.ATK = auth.ATK
					}
					if auth.DEF > 0 {
						animal.DEF = auth.DEF
					}
					if auth.SPD > 0 {
						animal.SPD = auth.SPD
					}
					if auth.Class != "" {
						animal.Class = auth.Class
					}
					if auth.Element != "" {
						animal.Element = auth.Element
					}
				}
			} else if inf.Species != "" {
				animal.Species = inf.Species
			}
			// 范围校验（服务端权威后仍保证边界）
			if animal.Rarity < 1 || animal.Rarity > 5 {
				return repo.ErrInferenceTampered
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
				"error":       "animal already exists",
				"uuid":        req.UUID,
				"reason_code": "duplicate_animal",
			})
			return
		}
		if req.InferenceRequestID != "" || h.inferenceRepo != nil {
			switch {
			case errors.Is(err, repo.ErrInferenceAlreadyUsed):
				c.JSON(http.StatusConflict, gin.H{"error": "inference already consumed", "reason_code": "inference_consumed"})
				return
			case errors.Is(err, repo.ErrInferenceExpired):
				c.JSON(http.StatusConflict, gin.H{"error": "inference expired", "reason_code": "inference_expired"})
				return
			case errors.Is(err, repo.ErrInferenceWrongKind):
				c.JSON(http.StatusConflict, gin.H{"error": "detect/analyze inference cannot create animals", "reason_code": "inference_wrong_kind"})
				return
			case errors.Is(err, repo.ErrInferenceSpeciesMismatch), errors.Is(err, repo.ErrInferenceTampered):
				c.JSON(http.StatusConflict, gin.H{"error": "forged or mismatched stats rejected", "reason_code": "inference_tampered"})
				return
			case errors.Is(err, repo.ErrInferenceNotFound), errors.Is(err, repo.ErrInferenceNotSuccess), errors.Is(err, gorm.ErrRecordNotFound):
				c.JSON(http.StatusConflict, gin.H{"error": "invalid or reused inference", "reason_code": "inference_invalid"})
				return
			case contains(err.Error(), "inference"):
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
		status, errMsg := h.syncOneScoped(middleware.GetDeviceID(c), middleware.GetAccountID(c), item)
		results = append(results, batchItemResult{UUID: item.UUID, Status: status, Error: errMsg})
	}
	c.JSON(http.StatusOK, BatchSyncResponse{Results: results})
}

func (h *SyncHandler) syncOne(deviceID string, req syncRequest) (string, string) {
	return h.syncOneScoped(deviceID, "", req)
}

func (h *SyncHandler) syncOneScoped(deviceID, accountID string, req syncRequest) (string, string) {
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
		UUID: req.UUID, DeviceID: deviceID, AccountID: accountID, Species: req.Species, Breed: req.Breed,
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
		if h.inferenceRepo != nil {
			if req.InferenceRequestID == "" {
				return errors.New("inference_request_id required")
			}
			inf, err := h.inferenceRepo.WithTx(tx).ConsumeValue(tx, req.InferenceRequestID, deviceID, req.Species)
			if err != nil {
				return err
			}
			if inf.ResultJSON != "" {
				var auth struct {
					Species string `json:"species"`
					Rarity  int    `json:"rarity"`
					HP      int    `json:"hp"`
					ATK     int    `json:"atk"`
					DEF     int    `json:"def"`
					SPD     int    `json:"spd"`
					Class   string `json:"class"`
					Element string `json:"element"`
				}
				if json.Unmarshal([]byte(inf.ResultJSON), &auth) == nil {
					if auth.Species != "" {
						animal.Species = auth.Species
					}
					if auth.Rarity > 0 {
						animal.Rarity = auth.Rarity
					}
					if auth.HP > 0 {
						animal.HP = auth.HP
					}
					if auth.ATK > 0 {
						animal.ATK = auth.ATK
					}
					if auth.DEF > 0 {
						animal.DEF = auth.DEF
					}
					if auth.SPD > 0 {
						animal.SPD = auth.SPD
					}
					if auth.Class != "" {
						animal.Class = auth.Class
					}
					if auth.Element != "" {
						animal.Element = auth.Element
					}
				}
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
	accountID := middleware.GetAccountID(c)
	var since int64
	if v := c.Query("since_version"); v != "" {
		if _, err := parseInt64(v, &since); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid since_version"})
			return
		}
	}
	limit := 50
	if v := c.Query("limit"); v != "" {
		var n int64
		if _, err := parseInt64(v, &n); err == nil && n > 0 {
			limit = int(n)
			if limit > 200 {
				limit = 200
			}
		}
	}
	items, err := h.animalRepo.ListSinceVersionScoped(deviceID, accountID, since, limit)
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
	var next int64 = since
	if len(items) > 0 {
		next = items[len(items)-1].ServerVersion
	}
	// 空页保持当前 since，禁止 next_version 回到 0 导致游标回退
	hasMore := len(items) >= limit
	c.JSON(http.StatusOK, gin.H{
		"items":        items,
		"next_version": next,
		"next_cursor":  next,
		"has_more":     hasMore,
		"limit":        limit,
	})
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
