package prompts

import (
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectPrompt_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, DetectPrompt)
	assert.Contains(t, DetectPrompt, "species")
	assert.Contains(t, DetectPrompt, "bounding_box")
	assert.Contains(t, DetectPrompt, "JSON")
	assert.Contains(t, DetectPrompt, `"animals"`)
	assert.NotContains(t, DetectPrompt, "Return ONLY a JSON array")
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
	assert.Contains(t, ValuePrompt, "{{.SubjectCompleteness}}")
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
	tmpl, err := template.New("value").Parse(ValuePrompt)
	require.NoError(t, err)
	var buf strings.Builder
	err = tmpl.Execute(&buf, map[string]interface{}{
		"Species":             "cat",
		"Breed":               "British Shorthair",
		"Color":               "blue-gray",
		"BodyType":            "sturdy",
		"SubjectCompleteness": 9,
		"Clarity":             8,
		"Lighting":            7,
		"Composition":         8,
		"Pose":                7,
		"Angle":               9,
	})
	require.NoError(t, err)
	p := buf.String()
	assert.Contains(t, p, "cat")
	assert.Contains(t, p, "British Shorthair")
	assert.Contains(t, p, "blue-gray")
	assert.Contains(t, p, "sturdy")
	assert.Contains(t, p, "completeness=9")
	assert.NotContains(t, p, "{{.")
}
