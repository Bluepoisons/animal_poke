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
	assert.Equal(t, "mock", result.Source)
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
	assert.Equal(t, "mock", result.Source)
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

func TestParseDetectJSON_EnvelopeAndArray(t *testing.T) {
	env, err := parseDetectJSON(`{"animals":[{"species":"dog","confidence":0.8,"bounding_box":{"x":0.1,"y":0.1,"width":0.2,"height":0.2}}]}`)
	assert.NoError(t, err)
	assert.Equal(t, "dog", env.Animals[0].Species)

	arr, err := parseDetectJSON(`[{"species":"bird","confidence":0.7,"bounding_box":{"x":0,"y":0,"width":0.5,"height":0.5}}]`)
	assert.NoError(t, err)
	assert.Equal(t, "bird", arr.Animals[0].Species)

	empty, err := parseDetectJSON(`{"animals":[]}`)
	assert.NoError(t, err)
	assert.Empty(t, empty.Animals)
}

func TestRenderValuePrompt_Complete(t *testing.T) {
	p, err := renderValuePrompt(ValueInput{
		Species: "cat", Breed: "Tabby", Color: "orange", BodyType: "lean",
		SubjectCompleteness: 8, Clarity: 7, Lighting: 6, Composition: 5, Pose: 4, Angle: 3,
	})
	assert.NoError(t, err)
	assert.Contains(t, p, "cat")
	assert.Contains(t, p, "completeness=8")
	assert.NotContains(t, p, "{{")
}
