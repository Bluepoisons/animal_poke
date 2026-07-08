package services

import (
	"testing"

	"animalpoke/backend/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestVisionService_Detect_Mock(t *testing.T) {
	cfg := &config.ThirdPartyConfig{} // 空 Key
	svc := NewVisionService(cfg)

	result, err := svc.Detect([]byte("fake"), "test.jpg")
	assert.NoError(t, err)
	assert.Len(t, result.Animals, 1)
	assert.Equal(t, "cat", result.Animals[0].Species)
	assert.Greater(t, result.Animals[0].Confidence, 0.9)
}

func TestVisionService_Analyze_Mock(t *testing.T) {
	cfg := &config.ThirdPartyConfig{}
	svc := NewVisionService(cfg)

	result, err := svc.Analyze([]byte("fake"), "test.jpg")
	assert.NoError(t, err)
	assert.Equal(t, "British Shorthair", result.Breed)
	assert.Equal(t, "blue-gray", result.Color)
	assert.Equal(t, "sturdy", result.BodyType)
	assert.Equal(t, 8, result.QualityScore)
}

func TestMockDetect_Structure(t *testing.T) {
	result := mockDetect()
	assert.NotNil(t, result)
	assert.Greater(t, len(result.Animals), 0)
	box := result.Animals[0].BoundingBox
	assert.Greater(t, box.Width, 0.0)
	assert.Greater(t, box.Height, 0.0)
}

func TestMockAnalyze_Structure(t *testing.T) {
	result := mockAnalyze()
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Breed)
	assert.NotEmpty(t, result.Color)
	assert.GreaterOrEqual(t, result.SubjectCompleteness, 1)
	assert.LessOrEqual(t, result.SubjectCompleteness, 10)
}
