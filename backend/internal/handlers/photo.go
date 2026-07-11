// AP-098 photography quality skill HTTP handlers.
package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
)

// PhotoHandler serves calibration, scoring, personal best and daily themes.
type PhotoHandler struct {
	photo  *repo.PhotoRepo
	secret string
}

// NewPhotoHandler constructs the handler. secret is STATS_HMAC_KEY (signed scores).
func NewPhotoHandler(photo *repo.PhotoRepo, secret string) *PhotoHandler {
	return &PhotoHandler{photo: photo, secret: secret}
}

func (h *PhotoHandler) owner(c *gin.Context) (ownerKey, deviceID, accountID string, ok bool) {
	deviceID = middleware.GetDeviceID(c)
	if deviceID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized", "reason_code": "unauthorized",
			"request_id": middleware.GetRequestID(c),
		})
		return "", "", "", false
	}
	accountID = strings.TrimSpace(middleware.GetAccountID(c))
	ownerKey = repo.OwnerKey(accountID, deviceID)
	return ownerKey, deviceID, accountID, true
}

func (h *PhotoHandler) loadCalibration(ownerKey string) (services.PhotoCalibration, *models.PhotoDeviceCalibration, error) {
	row, err := h.photo.GetCalibrationRow(ownerKey)
	if err != nil {
		return services.DefaultPhotoCalibration(), nil, err
	}
	if row == nil {
		return services.DefaultPhotoCalibration(), nil, nil
	}
	return services.PhotoCalibration{
		BaselineStabilityRMS: row.BaselineStabilityRMS,
		LightingOffset:       row.LightingOffset,
		SampleCount:          row.SampleCount,
		Calibrated:           row.Calibrated,
		ConfigVersion:        row.ConfigVersion,
	}, row, nil
}

// GetCalibration GET /api/v1/photo/calibration
func (h *PhotoHandler) GetCalibration(c *gin.Context) {
	if h == nil || h.photo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "photo unavailable", "reason_code": "db_unavailable", "request_id": middleware.GetRequestID(c)})
		return
	}
	ownerKey, _, _, ok := h.owner(c)
	if !ok {
		return
	}
	cal, row, err := h.loadCalibration(ownerKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "calibration read failed", "reason_code": "photo_error", "request_id": middleware.GetRequestID(c)})
		return
	}
	resp := gin.H{
		"calibration":    cal,
		"config_version": services.PhotoQualityConfigVersion,
		"request_id":     middleware.GetRequestID(c),
	}
	if row != nil {
		resp["device_model"] = row.DeviceModel
		resp["updated_at"] = row.UpdatedAt
	}
	c.JSON(http.StatusOK, resp)
}

type calibrateRequest struct {
	StabilitySamples []float64 `json:"stability_samples"`
	LightingOffsets  []float64 `json:"lighting_offsets"`
	DeviceModel      string    `json:"device_model"`
}

// Calibrate POST /api/v1/photo/calibrate
func (h *PhotoHandler) Calibrate(c *gin.Context) {
	if h == nil || h.photo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "photo unavailable", "reason_code": "db_unavailable", "request_id": middleware.GetRequestID(c)})
		return
	}
	ownerKey, deviceID, accountID, ok := h.owner(c)
	if !ok {
		return
	}
	var req calibrateRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.StabilitySamples) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stability_samples required", "reason_code": "bad_request", "request_id": middleware.GetRequestID(c)})
		return
	}
	if len(req.StabilitySamples) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many samples", "reason_code": "bad_request", "request_id": middleware.GetRequestID(c)})
		return
	}

	// Load existing and blend with new samples.
	existing, _, err := h.loadCalibration(ownerKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "calibration read failed", "reason_code": "photo_error", "request_id": middleware.GetRequestID(c)})
		return
	}
	built := services.BuildCalibration(req.StabilitySamples, req.LightingOffsets)
	merged := existing
	if built.SampleCount > 0 {
		total := existing.SampleCount + built.SampleCount
		if total <= 0 {
			total = built.SampleCount
		}
		wNew := float64(built.SampleCount) / float64(total)
		if wNew > 0.6 {
			wNew = 0.6
		}
		if existing.SampleCount == 0 {
			merged = built
		} else {
			merged.BaselineStabilityRMS = existing.BaselineStabilityRMS*(1-wNew) + built.BaselineStabilityRMS*wNew
			merged.LightingOffset = existing.LightingOffset*(1-wNew) + built.LightingOffset*wNew
			merged.SampleCount = total
			merged.Calibrated = total >= services.PhotoMinSensorSamples
		}
	}
	merged.ConfigVersion = services.PhotoQualityConfigVersion

	row, err := h.photo.UpsertCalibrationRow(&models.PhotoDeviceCalibration{
		OwnerKey:             ownerKey,
		DeviceID:             deviceID,
		AccountID:            accountID,
		BaselineStabilityRMS: merged.BaselineStabilityRMS,
		LightingOffset:       merged.LightingOffset,
		SampleCount:          merged.SampleCount,
		Calibrated:           merged.Calibrated,
		DeviceModel:          req.DeviceModel,
		ConfigVersion:        services.PhotoQualityConfigVersion,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "calibrate failed", "reason_code": "photo_error", "request_id": middleware.GetRequestID(c)})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"calibration": services.PhotoCalibration{
			BaselineStabilityRMS: row.BaselineStabilityRMS,
			LightingOffset:       row.LightingOffset,
			SampleCount:          row.SampleCount,
			Calibrated:           row.Calibrated,
			ConfigVersion:        row.ConfigVersion,
		},
		"config_version": services.PhotoQualityConfigVersion,
		"request_id":     middleware.GetRequestID(c),
	})
}

type scoreRequest struct {
	Metrics       services.PhotoMetrics `json:"metrics" binding:"required"`
	ThemeID       string                `json:"theme_id"`
	A11yCompleted bool                  `json:"a11y_completed"`
	// Persist false = dry-run score only (no daily cap consume) for preview.
	Persist *bool `json:"persist"`
}

// Score POST /api/v1/photo/score — server-authoritative scoring + optional persist.
func (h *PhotoHandler) Score(c *gin.Context) {
	if h == nil || h.photo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "photo unavailable", "reason_code": "db_unavailable", "request_id": middleware.GetRequestID(c)})
		return
	}
	ownerKey, deviceID, accountID, ok := h.owner(c)
	if !ok {
		return
	}
	var req scoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metrics required", "reason_code": "bad_request", "request_id": middleware.GetRequestID(c)})
		return
	}
	cal, _, err := h.loadCalibration(ownerKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "calibration read failed", "reason_code": "photo_error", "request_id": middleware.GetRequestID(c)})
		return
	}
	result, err := services.ScorePhotoQuality(req.Metrics, cal, h.secret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(), "reason_code": "bad_sensor",
			"request_id": middleware.GetRequestID(c),
		})
		return
	}

	persist := true
	if req.Persist != nil {
		persist = *req.Persist
	}

	theme := services.DailyPhotoTheme(time.Now().UTC())
	themeID := req.ThemeID
	if themeID == "" {
		themeID = theme.ThemeID
	}
	themeMet := services.ThemeMet(theme, result, req.A11yCompleted)
	dimScore := result.Dimensions.DimensionValue(theme.TargetDimension)

	resp := gin.H{
		"score":                 result,
		"rarity_quality_factor": services.PhotoQualityForRarity(result),
		"theme":                 theme,
		"theme_met":             themeMet,
		"config_version":        services.PhotoQualityConfigVersion,
		"request_id":            middleware.GetRequestID(c),
		"persisted":             false,
	}

	if !persist {
		c.JSON(http.StatusOK, resp)
		return
	}

	record, best, err := h.photo.SaveScoreResult(repo.SaveScoreInput{
		OwnerKey:       ownerKey,
		DeviceID:       deviceID,
		AccountID:      accountID,
		Overall:        result.Overall,
		Band:           result.Band,
		Stability:      result.Dimensions.Stability,
		Completeness:   result.Dimensions.SubjectCompleteness,
		Lighting:       result.Dimensions.Lighting,
		Occlusion:      result.Dimensions.Occlusion,
		Composition:    result.Dimensions.Composition,
		SafeDistance:   result.Dimensions.SafeDistance,
		ChasePenalty:   result.ChasePenalty,
		RarityEligible: result.RarityEligible,
		MetricsDigest:  result.MetricsDigest,
		Signature:      result.Signature,
		ConfigVersion:  result.ConfigVersion,
		ThemeID:        themeID,
		ThemeDimScore:  dimScore,
		ThemeMet:       themeMet,
		A11yCompleted:  req.A11yCompleted,
		DailyCap:       services.PhotoScoreDailyCap,
	})
	if err != nil {
		if errors.Is(err, repo.ErrPhotoDailyCap) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "daily photo score cap reached", "reason_code": "photo_daily_cap",
				"cap": services.PhotoScoreDailyCap, "request_id": middleware.GetRequestID(c),
				"score": result,
			})
			return
		}
		if errors.Is(err, repo.ErrPhotoDuplicate) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "duplicate metrics fingerprint today", "reason_code": "photo_duplicate",
				"request_id": middleware.GetRequestID(c),
				"score":      result,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save score failed", "reason_code": "photo_error", "request_id": middleware.GetRequestID(c)})
		return
	}
	resp["persisted"] = true
	resp["score_id"] = record.ScoreID
	resp["personal_best"] = best
	c.JSON(http.StatusOK, resp)
}

// PersonalBest GET /api/v1/photo/personal-best
func (h *PhotoHandler) PersonalBest(c *gin.Context) {
	if h == nil || h.photo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "photo unavailable", "reason_code": "db_unavailable", "request_id": middleware.GetRequestID(c)})
		return
	}
	ownerKey, _, _, ok := h.owner(c)
	if !ok {
		return
	}
	best, err := h.photo.GetPersonalBest(ownerKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "personal best failed", "reason_code": "photo_error", "request_id": middleware.GetRequestID(c)})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"personal_best":  best,
		"config_version": services.PhotoQualityConfigVersion,
		"request_id":     middleware.GetRequestID(c),
	})
}

// DailyTheme GET /api/v1/photo/theme/daily
func (h *PhotoHandler) DailyTheme(c *gin.Context) {
	theme := services.DailyPhotoTheme(time.Now().UTC())
	if h == nil || h.photo == nil {
		c.JSON(http.StatusOK, gin.H{
			"theme":          theme,
			"progress":       nil,
			"config_version": services.PhotoQualityConfigVersion,
			"request_id":     middleware.GetRequestID(c),
		})
		return
	}
	ownerKey, _, _, ok := h.owner(c)
	if !ok {
		return
	}
	prog, err := h.photo.GetThemeProgress(ownerKey, theme.Date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "theme progress failed", "reason_code": "photo_error", "request_id": middleware.GetRequestID(c)})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"theme":          theme,
		"progress":       prog,
		"config_version": services.PhotoQualityConfigVersion,
		"request_id":     middleware.GetRequestID(c),
	})
}

type themeProgressRequest struct {
	ThemeID       string `json:"theme_id"`
	A11yCompleted bool   `json:"a11y_completed"`
}

// ThemeProgress POST /api/v1/photo/theme/progress — mark a11y alternative complete.
func (h *PhotoHandler) ThemeProgress(c *gin.Context) {
	if h == nil || h.photo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "photo unavailable", "reason_code": "db_unavailable", "request_id": middleware.GetRequestID(c)})
		return
	}
	ownerKey, deviceID, accountID, ok := h.owner(c)
	if !ok {
		return
	}
	var req themeProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body", "reason_code": "bad_request", "request_id": middleware.GetRequestID(c)})
		return
	}
	theme := services.DailyPhotoTheme(time.Now().UTC())
	themeID := req.ThemeID
	if themeID == "" {
		themeID = theme.ThemeID
	}
	if themeID != theme.ThemeID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "theme_id must match today", "reason_code": "bad_request", "request_id": middleware.GetRequestID(c)})
		return
	}
	if !req.A11yCompleted {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "use POST /photo/score for skill progress; this endpoint is for a11y alternative",
			"reason_code": "bad_request", "request_id": middleware.GetRequestID(c),
		})
		return
	}
	prog, err := h.photo.MarkA11yThemeComplete(ownerKey, deviceID, accountID, themeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "theme progress failed", "reason_code": "photo_error", "request_id": middleware.GetRequestID(c)})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"theme":          theme,
		"progress":       prog,
		"config_version": services.PhotoQualityConfigVersion,
		"request_id":     middleware.GetRequestID(c),
	})
}
