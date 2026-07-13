// Package handlers MB3: AI 推理处理函数(LLM 数值生成)。
package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"
	"animalpoke/backend/internal/taxonomy"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ValueHandler LLM 数值生成处理器。
type ValueHandler struct {
	aiService     *services.AIService
	inferenceRepo *repo.InferenceRepo
	animalRepo    *repo.AnimalRepo
}

// NewValueHandler 构造 ValueHandler。
func NewValueHandler(aiService *services.AIService) *ValueHandler {
	return &ValueHandler{aiService: aiService}
}

// NewValueHandlerWithRepo 带 provenance。
func NewValueHandlerWithRepo(aiService *services.AIService, inf *repo.InferenceRepo) *ValueHandler {
	return &ValueHandler{aiService: aiService, inferenceRepo: inf}
}

// NewValueHandlerWithPersistence 在 value 推理完成后直接创建服务端动物记录。
func NewValueHandlerWithPersistence(aiService *services.AIService, inf *repo.InferenceRepo, animals *repo.AnimalRepo) *ValueHandler {
	return &ValueHandler{aiService: aiService, inferenceRepo: inf, animalRepo: animals}
}

type valueRequest struct {
	Species             string `json:"species" binding:"required"`
	SpeciesLabelZH      string `json:"species_label_zh"`
	Breed               string `json:"breed"`
	Color               string `json:"color"`
	BodyType            string `json:"body_type"`
	SubjectCompleteness int    `json:"subject_completeness"`
	Clarity             int    `json:"clarity"`
	Lighting            int    `json:"lighting"`
	Composition         int    `json:"composition"`
	Pose                int    `json:"pose"`
	Angle               int    `json:"angle"`
	// ParentInferenceID 应为 analyze 推理 ID（生产链路）；可选兼容旧客户端
	ParentInferenceID  string `json:"parent_inference_id"`
	AnalyzeInferenceID string `json:"analyze_inference_id"`
	// InferenceRequestID 兼容旧客户端字段名
	InferenceRequestID string `json:"inference_request_id"`
	// CaptureID 客户端 capture/session id，作稳定 seed 候选
	CaptureID string `json:"capture_id"`
}

// Generate POST /value/generate
func (h *ValueHandler) Generate(c *gin.Context) {
	var req valueRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}

	deviceID := middleware.GetDeviceID(c)
	parent := firstNonEmpty(req.ParentInferenceID, req.AnalyzeInferenceID)
	if h.inferenceRepo != nil && parent != "" {
		lineage, err := h.inferenceRepo.FindForDevice(parent, deviceID)
		if err != nil || (lineage.Kind != "analyze" && lineage.Kind != "detect") || (lineage.Status != "success" && lineage.Status != "consumed") {
			middleware.WriteError(c, http.StatusConflict, "lineage_invalid", "invalid parent inference", false, nil)
			return
		}
		if err := applyValueLineage(&req, lineage); err != nil {
			middleware.WriteError(c, http.StatusConflict, "lineage_invalid", err.Error(), false, nil)
			return
		}
	}

	// 稳定 seed：优先 parent/analyze/inference_request_id，其次 capture_id
	seedID := firstNonEmpty(
		req.ParentInferenceID,
		req.AnalyzeInferenceID,
		req.InferenceRequestID,
		req.CaptureID,
	)

	input := services.ValueInput{
		Species:             req.Species,
		SpeciesLabelZH:      req.SpeciesLabelZH,
		Breed:               req.Breed,
		Color:               req.Color,
		BodyType:            req.BodyType,
		SubjectCompleteness: req.SubjectCompleteness,
		Clarity:             req.Clarity,
		Lighting:            req.Lighting,
		Composition:         req.Composition,
		Pose:                req.Pose,
		Angle:               req.Angle,
		SeedID:              seedID,
	}
	if err := input.Validate(); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_value_input", err.Error(), false, nil)
		return
	}

	slog.Info("AI 数值生成请求", "device_id", deviceID, "species", req.Species, "seed_id", seedID)
	start := time.Now()

	result, err := h.aiService.GenerateValueContext(c.Request.Context(), input)
	if err != nil {
		slog.Error("AI 数值生成失败", "device_id", deviceID, "err", err)
		WriteProviderError(c, err, "value generation failed")
		return
	}
	result.SpeciesLabelZH = services.ChineseSpeciesLabel(req.Species, req.SpeciesLabelZH)

	if h.inferenceRepo != nil {
		id := uuid.NewString()
		cfgVer := result.ConfigVersion
		if cfgVer == "" {
			cfgVer = services.StatsConfigVersion
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"species":          req.Species,
			"species_label_zh": result.SpeciesLabelZH,
			"breed":            req.Breed,
			"rarity":           result.Rarity,
			"hp":               result.HP,
			"atk":              result.ATK,
			"def":              result.DEF,
			"spd":              result.SPD,
			"class":            result.Class,
			"element":          result.Element,
			"config_version":   cfgVer,
			"seed_id":          result.SeedID,
			"factors":          result.Factors,
		})
		exp := time.Now().UTC().Add(24 * time.Hour)
		inferenceStatus := "success"
		var consumedAt *time.Time
		if h.animalRepo != nil {
			// 动物与 value inference 同事务创建，推理在创建后已被消费，旧同步端点
			// 不能再凭同一 inference 重复创建动物。
			consumed := time.Now().UTC()
			inferenceStatus = "consumed"
			consumedAt = &consumed
		}
		inference := &models.Inference{
			InferenceID:       id,
			DeviceID:          deviceID,
			Kind:              "value",
			ParentInferenceID: parent,
			Provider:          "algo",
			Model:             result.Model,
			PromptVersion:     result.PromptVersion,
			InputDigest:       sha256Hex([]byte(fmt.Sprintf("%s|%s|%s|%s|%s", req.Species, result.SpeciesLabelZH, req.Breed, req.Color, seedID))),
			OutputDigest:      sha256Hex([]byte(fmt.Sprintf("%d|%d|%s|%s", result.Rarity, result.HP, result.Class, cfgVer))),
			ResultJSON:        string(payload),
			Species:           req.Species,
			ConfigVersion:     cfgVer,
			Status:            inferenceStatus,
			ConsumedAt:        consumedAt,
			DurationMs:        time.Since(start).Milliseconds(),
			ExpiresAt:         &exp,
		}
		if h.animalRepo == nil {
			if err := h.inferenceRepo.Create(inference); err != nil {
				slog.Error("value inference 保存失败", "device_id", deviceID, "err", err)
				middleware.WriteError(c, http.StatusInternalServerError, "inference_persist_failed", "value result save failed", true, nil)
				return
			}
		} else {
			animalUUID := req.CaptureID
			if _, err := uuid.Parse(animalUUID); err != nil {
				animalUUID = uuid.NewString()
			}
			normalizedSpecies, _ := taxonomy.Normalize(req.Species)
			animal := &models.Animal{
				UUID:               animalUUID,
				DeviceID:           deviceID,
				AccountID:          middleware.GetAccountID(c),
				Species:            normalizedSpecies,
				SpeciesLabelZH:     result.SpeciesLabelZH,
				Breed:              req.Breed,
				Rarity:             result.Rarity,
				HP:                 result.HP,
				ATK:                result.ATK,
				DEF:                result.DEF,
				SPD:                result.SPD,
				Class:              result.Class,
				Element:            result.Element,
				GeneratedAt:        time.Now().UTC(),
				InferenceRequestID: id,
				ServerVersion:      time.Now().UTC().UnixNano(),
			}
			if err := h.animalRepo.DB().Transaction(func(tx *gorm.DB) error {
				if err := h.inferenceRepo.WithTx(tx).Create(inference); err != nil {
					return err
				}
				return h.animalRepo.WithTx(tx).Create(animal)
			}); err != nil {
				slog.Error("value 结果落库失败", "device_id", deviceID, "err", err)
				middleware.WriteError(c, http.StatusInternalServerError, "capture_persist_failed", "capture save failed", true, nil)
				return
			}
			result.AnimalUUID = animalUUID
		}
		result.InferenceID = id
	}

	c.JSON(http.StatusOK, result)
}

func applyValueLineage(req *valueRequest, lineage *models.Inference) error {
	if req == nil || lineage == nil {
		return fmt.Errorf("invalid parent inference")
	}
	authoritativeSpecies := strings.TrimSpace(lineage.Species)
	authoritativeLabel := ""
	if lineage.Kind == "detect" {
		targets, err := parseDetectTargets(lineage.ResultJSON)
		if err != nil || len(targets) == 0 {
			return fmt.Errorf("parent inference has no target")
		}
		authoritativeSpecies = targets[0].Species
		authoritativeLabel = targets[0].Label
	} else if lineage.ResultJSON != "" {
		var result struct {
			Species        string `json:"species"`
			SpeciesLabelZH string `json:"species_label_zh"`
		}
		if err := json.Unmarshal([]byte(lineage.ResultJSON), &result); err != nil {
			return fmt.Errorf("parent inference result invalid")
		}
		if strings.TrimSpace(result.Species) != "" {
			authoritativeSpecies = result.Species
		}
		authoritativeLabel = result.SpeciesLabelZH
	}
	lineageSpecies, _ := taxonomy.Normalize(strings.TrimSpace(lineage.Species))
	authoritativeSpecies, authoritativeLabel, err := services.NormalizeAnimalIdentity(authoritativeSpecies, authoritativeLabel)
	if err != nil {
		return fmt.Errorf("parent inference has invalid animal identity")
	}
	if strings.TrimSpace(lineage.Species) != "" && lineageSpecies != authoritativeSpecies {
		return fmt.Errorf("parent inference species does not match result")
	}
	requestedSpecies, _ := taxonomy.Normalize(req.Species)
	if !taxonomy.Capturable(authoritativeSpecies) || requestedSpecies != authoritativeSpecies {
		return fmt.Errorf("species does not match parent inference")
	}
	if clientLabel := strings.TrimSpace(req.SpeciesLabelZH); clientLabel != "" {
		clientSpecies, clientLabel, err := services.NormalizeAnimalIdentity(requestedSpecies, clientLabel)
		if err != nil || clientSpecies != authoritativeSpecies || clientLabel != authoritativeLabel {
			return fmt.Errorf("species label does not match parent inference")
		}
	}
	req.Species = authoritativeSpecies
	req.SpeciesLabelZH = authoritativeLabel
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
