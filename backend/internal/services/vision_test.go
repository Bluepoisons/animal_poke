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
	// parse only; taxonomy filter applied in validateDetectResult
	assert.Equal(t, "bird", arr.Animals[0].Species)

	filtered := &DetectResult{Animals: arr.Animals}
	assert.NoError(t, validateDetectResult(filtered))
	assert.Empty(t, filtered.Animals, "bird must not become goose or capturable")

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


func TestValidateDetectResult_NoSilentGoose(t *testing.T) {
	mk := func(species string, conf float64) DetectBox {
		var b DetectBox
		b.Species = species
		b.Confidence = conf
		b.BoundingBox.X, b.BoundingBox.Y = 0.1, 0.1
		b.BoundingBox.Width, b.BoundingBox.Height = 0.2, 0.2
		return b
	}
	r := &DetectResult{Animals: []DetectBox{
		mk("duck", 0.9),
		mk("bird", 0.8),
		mk("cat", 0.7),
	}}
	assert.NoError(t, validateDetectResult(r))
	assert.Len(t, r.Animals, 1)
	assert.Equal(t, "cat", r.Animals[0].Species)
}

func TestValidateDetectResult_EmptyAndIllegal(t *testing.T) {
	mk := func(species string, conf float64) DetectBox {
		var b DetectBox
		b.Species = species
		b.Confidence = conf
		b.BoundingBox.X, b.BoundingBox.Y = 0, 0
		b.BoundingBox.Width, b.BoundingBox.Height = 0.5, 0.5
		return b
	}
	r := &DetectResult{Animals: []DetectBox{mk("", 0.5)}}
	assert.NoError(t, validateDetectResult(r))
	assert.Empty(t, r.Animals)

	bad := &DetectResult{Animals: []DetectBox{mk("cat", 1.5)}}
	assert.Error(t, validateDetectResult(bad))
}


func TestValidateDetectResult_InvalidBoxArea(t *testing.T) {
	var b DetectBox
	b.Species = "cat"
	b.Confidence = 0.9
	b.BoundingBox = BoundingBox{X: 0.1, Y: 0.1, Width: 0.001, Height: 0.001} // area 1e-6 < min
	r := &DetectResult{Animals: []DetectBox{b}}
	assert.Error(t, validateDetectResult(r))
}

func TestValidateDetectResult_MultiTargetIDs(t *testing.T) {
	mk := func(species, tid string, conf float64) DetectBox {
		return DetectBox{
			Species: species, TargetID: tid, Confidence: conf,
			BoundingBox: BoundingBox{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2},
		}
	}
	r := &DetectResult{Animals: []DetectBox{
		mk("dog", "dog-1", 0.8),
		mk("cat", "cat-1", 0.95),
	}}
	assert.NoError(t, validateDetectResult(r))
	assert.Len(t, r.Animals, 2)
	assert.Len(t, r.Targets, 2)
	assert.Equal(t, r.Animals, r.Targets)
	// sorted by confidence desc → cat first
	assert.Equal(t, "cat", r.Targets[0].Species)
	assert.Equal(t, "cat-1", r.Targets[0].TargetID)
	assert.Equal(t, "dog-1", r.Targets[1].TargetID)
}

func TestValidateAnalysisResult_StrictScores(t *testing.T) {
	ok := mockAnalyze()
	assert.NoError(t, validateAnalysisResult(ok))

	bad := *ok
	bad.QualityScore = 11
	assert.Error(t, validateAnalysisResult(&bad))

	missing := *ok
	missing.Breed = ""
	assert.Error(t, validateAnalysisResult(&missing))
}

func TestParseAnalysisJSON_RejectMultiAndMarkdown(t *testing.T) {
	var r AnalysisResult
	assert.Error(t, parseAnalysisJSON("```json\n{\"breed\":\"x\"}\n```", &r))
	assert.Error(t, parseAnalysisJSON(`{"breed":"a","color":"b","body_type":"c","quality_score":5,"subject_completeness":5,"clarity":5,"lighting":5,"composition":5,"pose":5,"angle":5}{"extra":1}`, &r))
}

func TestFindTarget_ByIDAndBox(t *testing.T) {
	targets := []DetectBox{
		{Species: "cat", TargetID: "0", Confidence: 0.9, BoundingBox: BoundingBox{X: 0.1, Y: 0.1, Width: 0.3, Height: 0.4}},
		{Species: "dog", TargetID: "1", Confidence: 0.85, BoundingBox: BoundingBox{X: 0.5, Y: 0.2, Width: 0.3, Height: 0.4}},
	}
	t0, err := FindTarget(targets, "0", nil)
	assert.NoError(t, err)
	assert.Equal(t, "cat", t0.Species)

	box := BoundingBox{X: 0.52, Y: 0.22, Width: 0.28, Height: 0.38}
	t1, err := FindTarget(targets, "", &box)
	assert.NoError(t, err)
	assert.Equal(t, "dog", t1.Species)

	_, err = FindTarget(targets, "missing", nil)
	assert.Error(t, err)

	bad := BoundingBox{X: 0.9, Y: 0.9, Width: 0.2, Height: 0.2} // out of range
	_, err = FindTarget(targets, "", &bad)
	assert.Error(t, err)
}

func TestMockDetect_HasTargets(t *testing.T) {
	r := mockDetect()
	assert.NotEmpty(t, r.Targets)
	assert.Equal(t, r.Animals[0].TargetID, r.Targets[0].TargetID)
	assert.NotEmpty(t, r.Targets[0].TargetID)
}
