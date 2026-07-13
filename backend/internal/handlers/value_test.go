package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupValueTest() (*gin.Engine, *ValueHandler) {
	gin.SetMode(gin.TestMode)
	cfg := &config.ThirdPartyConfig{} // 空 Key, 使用 mock
	svc := services.NewLLMService(cfg)
	handler := NewValueHandler(svc)
	r := gin.New()
	r.POST("/api/v1/value/generate", handler.Generate)
	return r, handler
}

func TestValueGenerate_MissingSpecies(t *testing.T) {
	r, _ := setupValueTest()

	body, _ := json.Marshal(map[string]interface{}{"breed": "test"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/value/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

func TestValueGenerate_InvalidScore(t *testing.T) {
	r, _ := setupValueTest()
	body, _ := json.Marshal(map[string]interface{}{
		"species": "cat",
		"clarity": 99,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/value/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

func TestValueGenerate_Success(t *testing.T) {
	r, _ := setupValueTest()

	body, _ := json.Marshal(map[string]interface{}{
		"species":              "cat",
		"breed":                "British Shorthair",
		"color":                "blue-gray",
		"body_type":            "sturdy",
		"subject_completeness": 9,
		"clarity":              8,
		"lighting":             7,
		"composition":          8,
		"pose":                 7,
		"angle":                9,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/value/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var result services.ValueResult
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.GreaterOrEqual(t, result.Rarity, 1)
	assert.LessOrEqual(t, result.Rarity, 5)
	assert.GreaterOrEqual(t, result.HP, 10)
	assert.NotEmpty(t, result.Narrative)
}

func TestValueGenerate_PersistsAnimalOnServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:value_persist?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Inference{}, &models.Animal{}))
	handler := NewValueHandlerWithPersistence(
		services.NewLLMService(&config.ThirdPartyConfig{}),
		repo.NewInferenceRepo(db),
		repo.NewAnimalRepo(db),
	)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextDeviceID, "dev-value-persist")
		c.Next()
	})
	r.POST("/api/v1/value/generate", handler.Generate)

	captureID := "8324ee15-806f-47c5-857a-cd1e8283b8d8"
	body, _ := json.Marshal(map[string]interface{}{
		"species":              "cat",
		"breed":                "British Shorthair",
		"color":                "blue-gray",
		"body_type":            "sturdy",
		"subject_completeness": 9,
		"clarity":              8,
		"lighting":             7,
		"composition":          8,
		"pose":                 7,
		"angle":                9,
		"capture_id":           captureID,
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/value/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var result services.ValueResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, captureID, result.AnimalUUID)
	require.NotEmpty(t, result.InferenceID)

	var animal models.Animal
	require.NoError(t, db.Where("uuid = ?", captureID).First(&animal).Error)
	assert.Equal(t, "dev-value-persist", animal.DeviceID)
	assert.Equal(t, result.InferenceID, animal.InferenceRequestID)
	assert.Equal(t, result.Rarity, animal.Rarity)
}

func TestValueGeneratePersistsConcreteOtherAnimalLabelFromLineage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:value_other_label?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Inference{}, &models.Animal{}))
	inferences := repo.NewInferenceRepo(db)
	expires := time.Now().UTC().Add(time.Hour)
	require.NoError(t, inferences.Create(&models.Inference{
		InferenceID: "analyze-other-animal", DeviceID: "dev-value-other", Kind: "analyze",
		Species: "other_animal", Status: "success", ExpiresAt: &expires,
		ResultJSON: `{"species":"other_animal","species_label_zh":"赤狐","breed":"赤狐"}`,
	}))
	handler := NewValueHandlerWithPersistence(
		services.NewLLMService(&config.ThirdPartyConfig{}),
		inferences,
		repo.NewAnimalRepo(db),
	)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextDeviceID, "dev-value-other")
		c.Next()
	})
	r.POST("/api/v1/value/generate", handler.Generate)

	captureID := "8324ee15-806f-47c5-857a-cd1e8283b8d9"
	body, _ := json.Marshal(map[string]interface{}{
		"species": "other_animal", "species_label_zh": "赤狐", "breed": "赤狐",
		"color": "赤褐色", "body_type": "轻巧", "subject_completeness": 9,
		"clarity": 8, "lighting": 7, "composition": 8, "pose": 7, "angle": 9,
		"parent_inference_id": "analyze-other-animal", "capture_id": captureID,
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/value/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var result services.ValueResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "赤狐", result.SpeciesLabelZH)
	assert.Contains(t, result.Narrative, "赤狐")
	var animal models.Animal
	require.NoError(t, db.Where("uuid = ?", captureID).First(&animal).Error)
	assert.Equal(t, "other_animal", animal.Species)
	assert.Equal(t, "赤狐", animal.SpeciesLabelZH)
}

func TestValueGenerateRejectsSpeciesOutsideParentLineage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:value_lineage_species?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Inference{}))
	inferences := repo.NewInferenceRepo(db)
	require.NoError(t, inferences.Create(&models.Inference{
		InferenceID: "analyze-cat", DeviceID: "dev-value-lineage", Kind: "analyze",
		Species: "cat", Status: "success", ResultJSON: `{"species":"cat","species_label_zh":"猫"}`,
	}))
	handler := NewValueHandlerWithRepo(services.NewLLMService(&config.ThirdPartyConfig{}), inferences)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextDeviceID, "dev-value-lineage")
		c.Next()
	})
	r.POST("/api/v1/value/generate", handler.Generate)
	body, _ := json.Marshal(map[string]interface{}{
		"species": "dog", "species_label_zh": "狗", "parent_inference_id": "analyze-cat",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/value/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "lineage_invalid")
}

func TestApplyValueLineageValidatesAnimalIdentity(t *testing.T) {
	validReq := &valueRequest{Species: "other_animal", SpeciesLabelZH: "石斑鱼"}
	validLineage := &models.Inference{
		Kind: "analyze", Species: "other_animal",
		ResultJSON: `{"species":"other_animal","species_label_zh":"石斑鱼"}`,
	}
	require.NoError(t, applyValueLineage(validReq, validLineage))
	assert.Equal(t, "other_animal", validReq.Species)
	assert.Equal(t, "石斑鱼", validReq.SpeciesLabelZH)

	invalidReq := &valueRequest{Species: "other_animal", SpeciesLabelZH: "石斑鱼"}
	invalidLineage := &models.Inference{
		Kind: "analyze", Species: "other_animal",
		ResultJSON: `{"species":"other_animal","species_label_zh":"桌子猫"}`,
	}
	assert.Error(t, applyValueLineage(invalidReq, invalidLineage))
}
