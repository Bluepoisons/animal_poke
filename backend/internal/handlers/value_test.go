package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
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
