package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/migrate"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func photoTestRouter(t *testing.T) (*gin.Engine, *PhotoHandler) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := "file:photo_" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, migrate.Apply(db))
	h := NewPhotoHandler(repo.NewPhotoRepo(db), "test-photo-hmac-secret-32chars!!")
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyDeviceID, "dev-photo-1")
		c.Set(middleware.ContextRequestID, "req-photo-test")
		c.Next()
	})
	r.GET("/photo/calibration", h.GetCalibration)
	r.POST("/photo/calibrate", h.Calibrate)
	r.POST("/photo/score", h.Score)
	r.GET("/photo/personal-best", h.PersonalBest)
	r.GET("/photo/theme/daily", h.DailyTheme)
	r.POST("/photo/theme/progress", h.ThemeProgress)
	return r, h
}

func TestPhotoHandler_CalibrateAndScore(t *testing.T) {
	r, _ := photoTestRouter(t)

	// calibrate
	body := map[string]any{
		"stability_samples": []float64{0.08, 0.09, 0.07, 0.08},
		"lighting_offsets":  []float64{0.01, 0.0},
		"device_model":      "unit-test",
	}
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/photo/calibrate", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// score
	metrics := services.PhotoMetrics{
		StabilityRMS: 0.07, SubjectFillRatio: 0.3, SubjectCenterOffset: 0.1,
		LightingScore: 0.7, OcclusionRatio: 0.1, SubjectCompleteness: 0.8,
		EstimatedDistanceM: 5, SensorSamples: 10,
	}
	sb, _ := json.Marshal(map[string]any{"metrics": metrics})
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/photo/score", bytes.NewReader(sb))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["persisted"])
	score, ok := resp["score"].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, score["signature"])
	assert.NotEmpty(t, score["band"])

	// personal best
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/photo/personal-best", nil)
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)

	// daily theme has a11y
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodGet, "/photo/theme/daily", nil)
	r.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusOK, w4.Code)
	var themeResp map[string]any
	require.NoError(t, json.Unmarshal(w4.Body.Bytes(), &themeResp))
	theme, ok := themeResp["theme"].(map[string]any)
	require.True(t, ok)
	a11y, ok := theme["accessibility_alternative"].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, a11y["goal_id"])
}

func TestPhotoHandler_BadSensorAndDuplicate(t *testing.T) {
	r, _ := photoTestRouter(t)

	bad := map[string]any{
		"metrics": map[string]any{
			"stability_rms": "nope",
		},
	}
	// invalid JSON shape still binds but zeros — use NaN via explicit invalid range
	metrics := services.PhotoMetrics{
		StabilityRMS: 99, SubjectFillRatio: 0.3, SubjectCenterOffset: 0.1,
		LightingScore: 0.5, OcclusionRatio: 0.1, SubjectCompleteness: 0.5, SensorSamples: 5,
	}
	sb, _ := json.Marshal(map[string]any{"metrics": metrics})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/photo/score", bytes.NewReader(sb))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	_ = bad

	// good then duplicate
	good := services.PhotoMetrics{
		StabilityRMS: 0.07, SubjectFillRatio: 0.3, SubjectCenterOffset: 0.1,
		LightingScore: 0.7, OcclusionRatio: 0.1, SubjectCompleteness: 0.8,
		EstimatedDistanceM: 5, SensorSamples: 10,
	}
	gb, _ := json.Marshal(map[string]any{"metrics": good})
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/photo/score", bytes.NewReader(gb))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/photo/score", bytes.NewReader(gb))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusConflict, w2.Code)
}

func TestPhotoHandler_A11yThemeProgress(t *testing.T) {
	r, _ := photoTestRouter(t)
	// get theme
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/photo/theme/daily", nil)
	r.ServeHTTP(w, req)
	var themeResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &themeResp))
	theme := themeResp["theme"].(map[string]any)
	themeID := theme["theme_id"].(string)

	body, _ := json.Marshal(map[string]any{"theme_id": themeID, "a11y_completed": true})
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/photo/theme/progress", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	var prog map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &prog))
	p := prog["progress"].(map[string]any)
	assert.Equal(t, true, p["a11y_completed"])
	assert.Equal(t, true, p["completed"])
}
