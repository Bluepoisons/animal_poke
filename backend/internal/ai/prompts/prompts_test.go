package prompts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectPrompt_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, DetectPrompt)
	assert.Contains(t, DetectPrompt, "species")
	assert.Contains(t, DetectPrompt, "bounding_box")
	assert.Contains(t, DetectPrompt, "JSON")
}

func TestAnalyzePrompt_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, AnalyzePrompt)
	assert.Contains(t, AnalyzePrompt, "breed")
	assert.Contains(t, AnalyzePrompt, "quality_score")
	assert.Contains(t, AnalyzePrompt, "subject_completeness")
	assert.Contains(t, AnalyzePrompt, "clarity")
	assert.Contains(t, AnalyzePrompt, "lighting")
	assert.Contains(t, AnalyzePrompt, "composition")
	assert.Contains(t, AnalyzePrompt, "pose")
	assert.Contains(t, AnalyzePrompt, "angle")
}

func TestValuePrompt_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, ValuePrompt)
	assert.Contains(t, ValuePrompt, "{{.Species}}")
	assert.Contains(t, ValuePrompt, "{{.Breed}}")
	assert.Contains(t, ValuePrompt, "{{.Color}}")
	assert.Contains(t, ValuePrompt, "rarity")
	assert.Contains(t, ValuePrompt, "hp")
	assert.Contains(t, ValuePrompt, "atk")
	assert.Contains(t, ValuePrompt, "def")
	assert.Contains(t, ValuePrompt, "spd")
	assert.Contains(t, ValuePrompt, "class")
	assert.Contains(t, ValuePrompt, "element")
	assert.Contains(t, ValuePrompt, "narrative")
}

func TestValuePrompt_Render(t *testing.T) {
	p := ValuePrompt
	p = strings.ReplaceAll(p, "{{.Species}}", "cat")
	p = strings.ReplaceAll(p, "{{.Breed}}", "British Shorthair")
	p = strings.ReplaceAll(p, "{{.Color}}", "blue-gray")
	p = strings.ReplaceAll(p, "{{.BodyType}}", "sturdy")

	assert.Contains(t, p, "cat")
	assert.Contains(t, p, "British Shorthair")
	assert.Contains(t, p, "blue-gray")
	assert.Contains(t, p, "sturdy")
	assert.NotContains(t, p, "{{.Species}}")
}
