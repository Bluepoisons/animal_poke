package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/safety"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSafetyTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:safety_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ModerationReport{}))

	h := NewSafetyHandler(db, true)
	r := gin.New()
	r.POST("/api/v1/safety/report", func(c *gin.Context) {
		c.Set("device_id", "dev-safety-1")
		h.Report(c)
	})
	r.GET("/api/v1/account/defaults", h.AccountDefaults)
	return r, db
}

func TestSafetyReport_AbusePath(t *testing.T) {
	r, db := setupSafetyTest(t)
	body, _ := json.Marshal(map[string]string{
		"category":     "abuse",
		"inference_id": "inf-1",
		"note":         "possible animal harm",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/safety/report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "accepted", resp["status"])
	assert.Equal(t, safety.CodeFlagAbuse, resp["decision_code"])
	assert.NotContains(t, w.Body.String(), "model")
	assert.NotContains(t, strings.ToLower(w.Body.String()), "base64")

	var count int64
	db.Model(&models.ModerationReport{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestSafetyReport_RejectsImagePayload(t *testing.T) {
	r, _ := setupSafetyTest(t)
	// long base64-looking note
	note := "data:image/jpeg;base64," + strings.Repeat("A", 300)
	body, _ := json.Marshal(map[string]string{"category": "injured", "note": note})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/safety/report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "image_not_allowed")
}

func TestAccountDefaults_StrictMinor(t *testing.T) {
	r, _ := setupSafetyTest(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/account/defaults?minor=1", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	defs := resp["defaults"].(map[string]interface{})
	assert.Equal(t, "minor", defs["audience"])
	assert.Equal(t, true, defs["strict"])
	assert.Equal(t, "none", defs["location_scope"])
	assert.Equal(t, false, defs["social_enabled"])
}

func setupVisionSafetyTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.ThirdPartyConfig{}
	svc := services.NewVisionService(cfg)
	h := NewVisionHandlerWithOptions(svc, VisionHandlerOptions{
		AllowSafetyFixture:    true,
		ProviderNoTrainPolicy: true,
	})
	r := gin.New()
	r.POST("/api/v1/vision/detect", h.Detect)
	return r
}

func multipartWithFixture(fieldName, filename string, data []byte, fixture string) (string, *bytes.Buffer) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile(fieldName, filename)
	_, _ = io.Copy(part, bytes.NewReader(data))
	_ = w.WriteField("safety_fixture", fixture)
	_ = w.Close()
	return w.FormDataContentType(), &buf
}

func TestVisionDetect_FixturePersonRejected(t *testing.T) {
	safety.ResetPolicyAudits()
	r := setupVisionSafetyTest(t)
	ct, body := multipartWithFixture("image", "portrait.jpg", tinyPNG(), "person")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/vision/detect", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, safety.CodeRejectPortrait, resp["reason_code"])
	animals, _ := resp["animals"].([]interface{})
	assert.Empty(t, animals)
	s := resp["safety"].(map[string]interface{})
	assert.Equal(t, false, s["collectable"])
	assert.Equal(t, safety.CodeRejectPortrait, s["decision_code"])
	// no original image in body
	assert.NotContains(t, w.Body.String(), "iVBORw0KGgo")
	assert.NotContains(t, strings.ToLower(w.Body.String()), "base64")
}

func TestVisionDetect_FixtureMatrixStable(t *testing.T) {
	r := setupVisionSafetyTest(t)
	cases := []struct {
		fixture string
		code    string
		coll    bool
	}{
		{"person", safety.CodeRejectPortrait, false},
		{"child", safety.CodeRejectChildFocus, false},
		{"plate", safety.CodeRejectSensitive, false},
		{"house", safety.CodeRejectSensitive, false},
		{"abuse", safety.CodeFlagAbuse, false},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			ct, body := multipartWithFixture("image", tc.fixture+".png", tinyPNG(), tc.fixture)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/vision/detect", body)
			req.Header.Set("Content-Type", ct)
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
			var resp map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			s := resp["safety"].(map[string]interface{})
			assert.Equal(t, tc.code, s["decision_code"])
			assert.Equal(t, tc.coll, s["collectable"])
			// response must not leak internals
			assert.NotContains(t, w.Body.String(), "fixture:")
			assert.NotContains(t, w.Body.String(), "InternalNotes")
		})
	}
}

func TestVisionDetect_PersonAnimalAllowsWithFlag(t *testing.T) {
	r := setupVisionSafetyTest(t)
	// person_animal is not hard-rejected pre-AI; mock returns cat then safety flags face
	ct, body := multipartWithFixture("image", "together.png", tinyPNG(), "person_animal")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/vision/detect", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	s := resp["safety"].(map[string]interface{})
	assert.Equal(t, safety.CodeFlagSensitive, s["decision_code"])
	assert.Equal(t, true, s["collectable"])
	animals, _ := resp["animals"].([]interface{})
	assert.NotEmpty(t, animals)
}

func TestVisionDetect_SafeAnimalOK(t *testing.T) {
	r := setupVisionSafetyTest(t)
	ct, body := multipartWithFixture("image", "cat.png", tinyPNG(), "safe_animal")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/vision/detect", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	s := resp["safety"].(map[string]interface{})
	assert.Equal(t, safety.CodeOK, s["decision_code"])
	assert.Equal(t, true, s["collectable"])
}

func TestProviderNoTrainLoggedOnDetect(t *testing.T) {
	safety.ResetPolicyAudits()
	r := setupVisionSafetyTest(t)
	ct, body := multipartWithFixture("image", "cat.png", tinyPNG(), "safe_animal")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/vision/detect", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	audits := safety.RecentPolicyAudits()
	require.NotEmpty(t, audits)
	last := audits[len(audits)-1]
	assert.Equal(t, safety.ProviderNoTrainPolicyID, last.PolicyID)
	assert.False(t, last.RetainImage)
	assert.False(t, last.AllowTrain)
	assert.NotEmpty(t, last.InputDigest)
	// digest is hex, not raw image
	assert.NotContains(t, last.InputDigest, string(tinyPNG()))
}
