// Package handlers MB4: 同步服务处理函数。
package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"
	"animalpoke/backend/internal/taxonomy"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 批量上限（请求级）。
const maxBatchItems = 100

// 时间窗口：允许有限时钟漂移与离线延迟。
const (
	syncFutureSkew     = 2 * time.Minute
	syncMaxAge         = 30 * 24 * time.Hour
	maxBreedLen        = 64
	maxSpeciesLabelLen = 64
	maxCityLen         = 64
	maxClassLen        = 32
	maxElementLen      = 32
	maxInferenceIDLen  = 128
)

var (
	validSyncClasses = map[string]bool{
		"Warrior": true, "Mage": true, "Ranger": true,
		"Tank": true, "Support": true, "Assassin": true,
	}
	validSyncElements = map[string]bool{
		"Fire": true, "Water": true, "Grass": true, "Electric": true,
		"Ice": true, "Dark": true, "Light": true, "Earth": true, "Wind": true,
	}
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
	SpeciesLabelZH     string  `json:"species_label_zh"`
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

// syncOutcome 单条同步领域结果（单条/批量共用，禁止向 gin.Context 塞 item）。
type syncOutcome struct {
	UUID         string
	Status       string // synced | conflict | error
	HTTPStatus   int
	Error        string
	ReasonCode   string
	ReviewStatus string
}

func failOutcome(uuid, status string, httpStatus int, msg, code string) syncOutcome {
	return syncOutcome{
		UUID:       uuid,
		Status:     status,
		HTTPStatus: httpStatus,
		Error:      msg,
		ReasonCode: code,
	}
}

// SyncAnimal POST /sync/animal 接收客户端上传的动物元数据。
func (h *SyncHandler) SyncAnimal(c *gin.Context) {
	var req syncRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}

	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	out := h.validateAndSyncOne(deviceID, accountID, &req)
	if out.Status == "synced" {
		middleware.ObserveSyncOutcome("synced")
		middleware.ObserveFunnel("sync", "synced")
	} else if out.Status == "conflict" {
		middleware.ObserveSyncOutcome("conflict")
		middleware.ObserveFunnel("sync", "conflict")
	} else {
		middleware.ObserveSyncOutcome("error")
		middleware.ObserveFunnel("sync", "error")
	}
	writeSyncOutcome(c, out)
}

// BatchSyncRequest 批量推送（非原子：逐项独立结果）。
type BatchSyncRequest struct {
	Items []syncRequest `json:"items" binding:"required"`
}

// BatchSyncResponse 批量结果。
type BatchSyncResponse struct {
	Results []batchItemResult `json:"results"`
}

type batchItemResult struct {
	UUID       string `json:"uuid"`
	Status     string `json:"status"` // synced|conflict|error
	Error      string `json:"error,omitempty"`
	ReasonCode string `json:"reason_code,omitempty"`
}

// SyncAnimalsBatch POST /sync/animals
// 语义：非原子逐项处理；每项走与单条相同的 validateAndSyncOne；不返回原始 DB 错误。
func (h *SyncHandler) SyncAnimalsBatch(c *gin.Context) {
	var req BatchSyncRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if len(req.Items) == 0 {
		middleware.AbortBadRequest(c, "items_required", "items required", nil)
		return
	}
	if len(req.Items) > maxBatchItems {
		middleware.AbortBadRequest(c, "batch_too_large", "max 100 items per batch", nil)
		return
	}

	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	results := make([]batchItemResult, 0, len(req.Items))
	seen := make(map[string]int, len(req.Items))

	synced, conflicted, errored := 0, 0, 0
	for i := range req.Items {
		item := &req.Items[i]
		// 批内重复：不写库，返回稳定 reason_code（与已存在冲突语义一致但可区分）
		if item.UUID != "" {
			if prev, ok := seen[item.UUID]; ok {
				results = append(results, batchItemResult{
					UUID:       item.UUID,
					Status:     "conflict",
					Error:      "duplicate uuid in batch",
					ReasonCode: "batch_duplicate",
				})
				conflicted++
				_ = prev
				continue
			}
			seen[item.UUID] = i
		}

		out := h.validateAndSyncOne(deviceID, accountID, item)
		switch out.Status {
		case "synced":
			synced++
		case "conflict":
			conflicted++
		default:
			errored++
		}
		results = append(results, batchItemResult{
			UUID:       out.UUID,
			Status:     out.Status,
			Error:      out.Error,
			ReasonCode: out.ReasonCode,
		})
	}

	// 批量整体 outcome：全部成功记 accepted；否则记 mixed。
	if errored == 0 && conflicted == 0 {
		middleware.ObserveSyncOutcome("accepted")
		middleware.ObserveFunnel("sync", "accepted")
	} else if synced > 0 {
		middleware.ObserveSyncOutcome("mixed")
		middleware.ObserveFunnel("sync", "mixed")
	} else if conflicted > 0 {
		middleware.ObserveSyncOutcome("conflict")
		middleware.ObserveFunnel("sync", "conflict")
	} else {
		middleware.ObserveSyncOutcome("error")
		middleware.ObserveFunnel("sync", "error")
	}

	c.JSON(http.StatusOK, BatchSyncResponse{Results: results})
}

// validateAndSyncOne 统一校验 + 落库，单条与批量唯一路径。
func (h *SyncHandler) validateAndSyncOne(deviceID, accountID string, req *syncRequest) syncOutcome {
	if req == nil {
		return failOutcome("", "error", http.StatusBadRequest, "invalid request", "invalid_request")
	}

	if errOut := validateSyncFields(req); errOut != nil {
		return *errOut
	}

	generatedAt, err := time.Parse(time.RFC3339, req.GeneratedAt)
	if err != nil {
		// 兼容 RFC3339Nano
		generatedAt, err = time.Parse(time.RFC3339Nano, req.GeneratedAt)
		if err != nil {
			return failOutcome(req.UUID, "error", http.StatusBadRequest, "invalid generated_at format, use RFC3339", "invalid_generated_at")
		}
	}
	if errOut := validateGeneratedAt(req.UUID, generatedAt); errOut != nil {
		return *errOut
	}

	// 有 InferenceRepo 时强制 value inference
	if h.inferenceRepo != nil && strings.TrimSpace(req.InferenceRequestID) == "" {
		return failOutcome(req.UUID, "error", http.StatusBadRequest, "inference_request_id required", "inference_required")
	}

	exists, err := h.animalRepo.ExistsByUUID(req.UUID)
	if err != nil {
		slog.Error("去重查询失败", "err", err)
		return failOutcome(req.UUID, "error", http.StatusInternalServerError, "sync failed", "sync_failed")
	}
	if exists {
		return failOutcome(req.UUID, "conflict", http.StatusConflict, "animal already exists", "duplicate_animal")
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
		SpeciesLabelZH:     req.SpeciesLabelZH,
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
					Species        string `json:"species"`
					SpeciesLabelZH string `json:"species_label_zh"`
					Breed          string `json:"breed"`
					Rarity         int    `json:"rarity"`
					HP             int    `json:"hp"`
					ATK            int    `json:"atk"`
					DEF            int    `json:"def"`
					SPD            int    `json:"spd"`
					Class          string `json:"class"`
					Element        string `json:"element"`
				}
				if err := json.Unmarshal([]byte(inf.ResultJSON), &auth); err == nil {
					if auth.Species != "" {
						animal.Species = auth.Species
					}
					if auth.SpeciesLabelZH != "" {
						animal.SpeciesLabelZH = auth.SpeciesLabelZH
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
			normalizedSpecies, normalizedLabel, identityErr := services.NormalizeAnimalIdentity(animal.Species, animal.SpeciesLabelZH)
			if identityErr != nil {
				return repo.ErrInferenceTampered
			}
			if inf.Species != "" {
				inferenceSpecies, _ := taxonomy.Normalize(inf.Species)
				if inferenceSpecies != normalizedSpecies {
					return repo.ErrInferenceTampered
				}
			}
			animal.Species = normalizedSpecies
			animal.SpeciesLabelZH = normalizedLabel

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
		return mapSyncPersistError(req.UUID, err, h.inferenceRepo != nil || req.InferenceRequestID != "")
	}

	slog.Info("动物同步成功", "uuid", req.UUID, "device_id", deviceID, "species", req.Species, "rarity", animal.Rarity)
	return syncOutcome{
		UUID:         req.UUID,
		Status:       "synced",
		HTTPStatus:   http.StatusCreated,
		ReviewStatus: review,
	}
}

// validateSyncFields 严格字段校验（单条/批量共用；会规范化 species）。
func validateSyncFields(req *syncRequest) *syncOutcome {
	uuidStr := strings.TrimSpace(req.UUID)
	speciesRaw := strings.TrimSpace(req.Species)
	generatedAt := strings.TrimSpace(req.GeneratedAt)

	if uuidStr == "" || speciesRaw == "" || req.Rarity == 0 || generatedAt == "" {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest,
			"invalid request: uuid, species, rarity, generated_at are required", "invalid_request")
		return &o
	}
	req.UUID = uuidStr
	req.GeneratedAt = generatedAt

	if _, err := uuid.Parse(uuidStr); err != nil {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "invalid uuid", "invalid_uuid")
		return &o
	}

	normSpecies, _ := taxonomy.Normalize(speciesRaw)
	if !taxonomy.Capturable(normSpecies) {
		req.Species = normSpecies
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "species not capturable", "species_unsupported")
		return &o
	}
	req.Species = normSpecies
	if utf8.RuneCountInString(req.SpeciesLabelZH) > maxSpeciesLabelLen {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "species_label_zh too long", "invalid_string_length")
		return &o
	}
	validatedSpecies, validatedLabel, err := services.NormalizeAnimalIdentity(normSpecies, req.SpeciesLabelZH)
	if err != nil {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "species_label_zh does not match species", "invalid_species_label")
		return &o
	}
	req.Species = validatedSpecies
	req.SpeciesLabelZH = validatedLabel

	if req.Rarity < 1 || req.Rarity > 5 {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "rarity must be 1-5", "invalid_rarity")
		return &o
	}

	// 可选数值：为 0 表示未提供（可由服务端 inference 覆盖）；非 0 则校验范围。
	if req.HP != 0 && (req.HP < 10 || req.HP > 100) {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "hp out of range", "invalid_stats")
		return &o
	}
	if req.ATK != 0 && (req.ATK < 5 || req.ATK > 50) {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "atk out of range", "invalid_stats")
		return &o
	}
	if req.DEF != 0 && (req.DEF < 5 || req.DEF > 50) {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "def out of range", "invalid_stats")
		return &o
	}
	if req.SPD != 0 && (req.SPD < 5 || req.SPD > 50) {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "spd out of range", "invalid_stats")
		return &o
	}

	if req.Class != "" {
		if utf8.RuneCountInString(req.Class) > maxClassLen || !validSyncClasses[req.Class] {
			o := failOutcome(uuidStr, "error", http.StatusBadRequest, "invalid class", "invalid_class")
			return &o
		}
	}
	if req.Element != "" {
		if utf8.RuneCountInString(req.Element) > maxElementLen || !validSyncElements[req.Element] {
			o := failOutcome(uuidStr, "error", http.StatusBadRequest, "invalid element", "invalid_element")
			return &o
		}
	}

	if req.Latitude < -90 || req.Latitude > 90 || req.Longitude < -180 || req.Longitude > 180 {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "coordinates out of range", "invalid_coords")
		return &o
	}

	if utf8.RuneCountInString(req.Breed) > maxBreedLen || utf8.RuneCountInString(req.City) > maxCityLen {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "string field too long", "invalid_string_length")
		return &o
	}
	if utf8.RuneCountInString(req.InferenceRequestID) > maxInferenceIDLen {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "inference_request_id too long", "invalid_string_length")
		return &o
	}

	return nil
}

func validateGeneratedAt(uuidStr string, generatedAt time.Time) *syncOutcome {
	now := time.Now().UTC()
	ga := generatedAt.UTC()
	if ga.After(now.Add(syncFutureSkew)) {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "generated_at in future", "invalid_time")
		return &o
	}
	if ga.Before(now.Add(-syncMaxAge)) {
		o := failOutcome(uuidStr, "error", http.StatusBadRequest, "generated_at too old", "invalid_time")
		return &o
	}
	return nil
}

// mapSyncPersistError 将持久化/推理错误映射为稳定 reason_code，禁止泄露原始 DB 错误。
func mapSyncPersistError(uuidStr string, err error, inferenceAware bool) syncOutcome {
	if errors.Is(err, gorm.ErrDuplicatedKey) || isDuplicate(err) {
		return failOutcome(uuidStr, "conflict", http.StatusConflict, "animal already exists", "duplicate_animal")
	}
	if inferenceAware {
		switch {
		case errors.Is(err, repo.ErrInferenceAlreadyUsed):
			return failOutcome(uuidStr, "conflict", http.StatusConflict, "inference already consumed", "inference_consumed")
		case errors.Is(err, repo.ErrInferenceExpired):
			return failOutcome(uuidStr, "conflict", http.StatusConflict, "inference expired", "inference_expired")
		case errors.Is(err, repo.ErrInferenceWrongKind):
			return failOutcome(uuidStr, "conflict", http.StatusConflict, "detect/analyze inference cannot create animals", "inference_wrong_kind")
		case errors.Is(err, repo.ErrInferenceSpeciesMismatch), errors.Is(err, repo.ErrInferenceTampered):
			return failOutcome(uuidStr, "conflict", http.StatusConflict, "forged or mismatched stats rejected", "inference_tampered")
		case errors.Is(err, repo.ErrInferenceNotFound), errors.Is(err, repo.ErrInferenceNotSuccess), errors.Is(err, gorm.ErrRecordNotFound):
			return failOutcome(uuidStr, "conflict", http.StatusConflict, "invalid or reused inference", "inference_invalid")
		case contains(err.Error(), "inference"):
			return failOutcome(uuidStr, "conflict", http.StatusConflict, "invalid or reused inference", "inference_invalid")
		}
	}
	slog.Error("动物同步失败", "uuid", uuidStr, "err", err)
	return failOutcome(uuidStr, "error", http.StatusInternalServerError, "sync failed", "sync_failed")
}

func writeSyncOutcome(c *gin.Context, out syncOutcome) {
	switch out.Status {
	case "synced":
		c.JSON(out.HTTPStatus, syncResponse{
			Status:       "synced",
			UUID:         out.UUID,
			ReviewStatus: out.ReviewStatus,
		})
	case "conflict":
		body := gin.H{
			"error":       out.Error,
			"uuid":        out.UUID,
			"reason_code": out.ReasonCode,
			"request_id":  middleware.GetRequestID(c),
			"retryable":   false,
		}
		c.JSON(out.HTTPStatus, body)
	default:
		body := gin.H{
			"error":       out.Error,
			"reason_code": out.ReasonCode,
			"request_id":  middleware.GetRequestID(c),
			"retryable":   out.HTTPStatus >= 500,
		}
		if out.UUID != "" {
			body["uuid"] = out.UUID
		}
		// 物种不支持时附带规范化 species 便于客户端处理
		if out.ReasonCode == "species_unsupported" {
			// no extra fields required
		}
		c.JSON(out.HTTPStatus, body)
	}
}

// PullAnimals GET /sync/animals?since_version=
func (h *SyncHandler) PullAnimals(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	var since int64
	if v := c.Query("since_version"); v != "" {
		if _, err := parseInt64(v, &since); err != nil {
			middleware.WriteError(c, http.StatusBadRequest, "invalid_request", "invalid since_version", false, nil)
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
		middleware.WriteError(c, http.StatusInternalServerError, "sync_failed", "pull failed", true, nil)
		return
	}
	// 二次脱敏：活跃行精确坐标；软删已在 repo 裁剪为 tombstone。
	out := make([]gin.H, 0, len(items))
	for i := range items {
		if items[i].DeletedAt != nil {
			out = append(out, gin.H{
				"uuid":           items[i].UUID,
				"deleted_at":     items[i].DeletedAt,
				"server_version": items[i].ServerVersion,
			})
			continue
		}
		items[i].PreciseLat = nil
		items[i].PreciseLng = nil
		items[i].PreciseExpiresAt = nil
		out = append(out, gin.H{
			"uuid":                 items[i].UUID,
			"device_id":            items[i].DeviceID,
			"species":              items[i].Species,
			"species_label_zh":     items[i].SpeciesLabelZH,
			"breed":                items[i].Breed,
			"rarity":               items[i].Rarity,
			"hp":                   items[i].HP,
			"atk":                  items[i].ATK,
			"def":                  items[i].DEF,
			"spd":                  items[i].SPD,
			"class":                items[i].Class,
			"element":              items[i].Element,
			"city":                 items[i].City,
			"geohash":              items[i].GeoHash,
			"latitude":             items[i].Latitude,
			"longitude":            items[i].Longitude,
			"generated_at":         items[i].GeneratedAt,
			"inference_request_id": items[i].InferenceRequestID,
			"nickname":             items[i].Nickname,
			"favorite":             items[i].Favorite,
			"locked":               items[i].Locked,
			"server_version":       items[i].ServerVersion,
			"created_at":           items[i].CreatedAt,
		})
	}
	// 空页保持当前 since，禁止 next_version 回到 0 导致游标回退
	var next int64 = since
	if len(items) > 0 {
		next = items[len(items)-1].ServerVersion
	}
	hasMore := len(items) >= limit
	c.JSON(http.StatusOK, gin.H{
		"items":        out,
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
