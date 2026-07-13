package services

import (
	"encoding/json"
	"testing"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/taxonomy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVisionService_Detect_Mock(t *testing.T) {
	cfg := &config.ThirdPartyConfig{} // 空 Key
	svc := NewVisionService(cfg)

	result, err := svc.Detect([]byte("fake"), "test.jpg")
	assert.NoError(t, err)
	assert.Len(t, result.Animals, 1)
	assert.Equal(t, "cat", result.Animals[0].Species)
	assert.Equal(t, "猫", result.Animals[0].Label)
	assert.Greater(t, result.Animals[0].Confidence, 0.9)
	assert.Equal(t, "mock", result.Source)
}

func TestVisionService_Analyze_Mock(t *testing.T) {
	cfg := &config.ThirdPartyConfig{}
	svc := NewVisionService(cfg)

	result, err := svc.Analyze([]byte("fake"), "test.jpg")
	assert.NoError(t, err)
	assert.Equal(t, "英国短毛猫", result.Breed)
	assert.Equal(t, "蓝灰色", result.Color)
	assert.Equal(t, "敦实", result.BodyType)
	assert.Equal(t, 8, result.QualityScore)
	assert.Equal(t, "mock", result.Source)
}

func TestGeneratedChineseIsNormalizedToSimplified(t *testing.T) {
	normalized, err := simplifyGeneratedChinese("英國短毛貓在雲海探險")
	assert.NoError(t, err)
	assert.Equal(t, "英国短毛猫在云海探险", normalized)
	assert.True(t, isChineseGeneratedText(normalized))
	assert.False(t, isChineseGeneratedText("英國短毛貓"))

	analysis := &AnalysisResult{Breed: "英國短毛貓", Color: "藍灰色", BodyType: "勻稱"}
	assert.NoError(t, simplifyAnalysisDescriptions(analysis))
	assert.Equal(t, "英国短毛猫", analysis.Breed)
	assert.Equal(t, "蓝灰色", analysis.Color)
	assert.Equal(t, "匀称", analysis.BodyType)
}

func TestChineseSpeciesUsesContentRegistry(t *testing.T) {
	assert.Equal(t, "鸟", chineseSpecies("bird"))
	assert.Equal(t, "青蛙", chineseSpecies("frog"))
	assert.Equal(t, "其他动物", chineseSpecies("other_animal"))
	assert.Equal(t, "动物伙伴", chineseSpecies("not-registered"))
}

func TestMockDetect_Structure(t *testing.T) {
	result := mockDetect()
	assert.NotNil(t, result)
	assert.Greater(t, len(result.Animals), 0)
	assert.True(t, taxonomy.Capturable(result.Animals[0].Species))
	assert.Equal(t, BoundingBox{}, result.Animals[0].BoundingBox)
}

func TestDetectBoxJSON_OmitsAbsentBoundingBox(t *testing.T) {
	payload, err := json.Marshal(DetectBox{Species: "cat", TargetID: "0", Confidence: 0.95})
	assert.NoError(t, err)
	assert.NotContains(t, string(payload), "bounding_box")
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
	assert.Len(t, filtered.Animals, 1)
	assert.Equal(t, "bird", filtered.Animals[0].Species)
	assert.Equal(t, "鸟", filtered.Animals[0].Label)

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
	assert.Contains(t, p, "FICTIONAL")
	assert.Contains(t, p, "cat")
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
	assert.Len(t, r.Animals, 3)
	assert.Equal(t, "duck", r.Animals[0].Species)
	assert.Equal(t, "bird", r.Animals[1].Species)
	assert.Equal(t, "cat", r.Animals[2].Species)
	for _, animal := range r.Animals {
		assert.NotEqual(t, "goose", animal.Species, "no bird category may silently become goose")
	}
}

func TestValidateDetectResult_OtherAnimalContract(t *testing.T) {
	for _, label := range []string{
		"赤狐", "仓鼠", "长颈鹿", "斑马",
		"蚯蚓", "海绵", "蚊子", "石斑鱼", "木虱",
		"宽吻海豚", "白头海雕", "中华蜜蜂", "丹顶鹤", "大白鲨", "鸭嘴兽",
	} {
		t.Run(label, func(t *testing.T) {
			r := &DetectResult{Animals: []DetectBox{{Species: "other_animal", Label: label, Confidence: 0.91}}}
			assert.NoError(t, validateDetectResult(r))
			require.Len(t, r.Animals, 1)
			assert.Equal(t, "other_animal", r.Animals[0].Species)
			assert.Equal(t, label, r.Animals[0].Label)
		})
	}

	registered := &DetectResult{Animals: []DetectBox{{Species: "other_animal", Label: "猫", Confidence: 0.91}}}
	assert.NoError(t, validateDetectResult(registered))
	require.Len(t, registered.Animals, 1)
	assert.Equal(t, "cat", registered.Animals[0].Species)
	assert.Equal(t, "猫", registered.Animals[0].Label)

	for _, label := range []string{
		"其他动物", "未知动物", "不明动物", "动物", "生物",
		"桌子", "石头", "蘑菇", "植物", "车辆", "人", "玩具", "屏幕", "plant", "car",
		"赤狐玩具", "桌子赤狐", "长颈鹿模型", "桌子猫", "机器人狗", "木马", "木鱼",
	} {
		t.Run(label, func(t *testing.T) {
			bad := &DetectResult{Animals: []DetectBox{{Species: "other_animal", Label: label, Confidence: 0.91}}}
			assert.Error(t, validateDetectResult(bad))
		})
	}
}

func TestNormalizeConcreteAnimalLabel(t *testing.T) {
	tests := []struct {
		input   string
		label   string
		species string
	}{
		{input: "長頸鹿", label: "长颈鹿", species: "other_animal"},
		{input: "猫", label: "猫", species: "cat"},
		{input: "蚯蚓", label: "蚯蚓", species: "other_animal"},
		{input: "海绵", label: "海绵", species: "other_animal"},
		{input: "蚊子", label: "蚊子", species: "other_animal"},
		{input: "石斑鱼", label: "石斑鱼", species: "other_animal"},
		{input: "木虱", label: "木虱", species: "other_animal"},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			label, species, err := NormalizeConcreteAnimalLabel(test.input)
			require.NoError(t, err)
			assert.Equal(t, test.label, label)
			assert.Equal(t, test.species, species)
		})
	}

	for _, input := range []string{
		"桌子", "石头", "蘑菇", "植物", "车辆", "人物", "人", "玩具", "屏幕",
		"赤狐玩具", "桌子猫", "机器人狗", "木马", "木鱼", "海绵模型", "怪兽", "神兽", "魔兽",
	} {
		t.Run(input, func(t *testing.T) {
			_, _, err := NormalizeConcreteAnimalLabel(input)
			assert.Error(t, err)
		})
	}
}

func TestNormalizeAnimalIdentity(t *testing.T) {
	tests := []struct {
		name        string
		species     string
		label       string
		wantSpecies string
		wantLabel   string
		wantErr     bool
	}{
		{name: "registered default", species: "cat", wantSpecies: "cat", wantLabel: "猫"},
		{name: "broad canonical", species: "other_animal", label: "石斑鱼", wantSpecies: "other_animal", wantLabel: "石斑鱼"},
		{name: "broad alias", species: "其他动物", label: "木虱", wantSpecies: "other_animal", wantLabel: "木虱"},
		{name: "object compound", species: "other_animal", label: "桌子猫", wantErr: true},
		{name: "registered mismatch", species: "cat", label: "狗", wantErr: true},
		{name: "broad missing label", species: "other_animal", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			species, label, err := NormalizeAnimalIdentity(test.species, test.label)
			if test.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.wantSpecies, species)
			assert.Equal(t, test.wantLabel, label)
		})
	}
}

func TestValidateDetectResult_RejectsConcreteSpeciesLabelContradictions(t *testing.T) {
	unsupported := &DetectResult{Animals: []DetectBox{{Species: "cat", Label: "玩具", Confidence: 0.9}}}
	assert.Error(t, validateDetectResult(unsupported))

	mismatch := &DetectResult{Animals: []DetectBox{{Species: "cat", Label: "狗", Confidence: 0.9}}}
	assert.Error(t, validateDetectResult(mismatch))
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

	english := *ok
	english.Breed = "英国短毛猫 British Shorthair"
	assert.Error(t, validateAnalysisResult(&english))
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
