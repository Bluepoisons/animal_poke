package mlqa

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManifest_RequiredGroups(t *testing.T) {
	m, err := LoadManifest("")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "1.0.0", m.Version)
	assert.GreaterOrEqual(t, len(m.Fixtures), 8)

	seen := map[string]bool{}
	for _, f := range m.Fixtures {
		seen[f.SpeciesGroup] = true
		assert.NotEmpty(t, f.ID)
		assert.NotEmpty(t, f.Expected.Species)
	}
	for _, g := range []string{"cat", "dog", "goose", "duck", "swan", "bird", "person", "empty"} {
		assert.True(t, seen[g], "missing group %s", g)
	}
}

func TestSyntheticPNG_Deterministic(t *testing.T) {
	a, err := SyntheticPNG(ImageSpec{Kind: "synthetic", Seed: 42, Width: 16, Height: 16})
	require.NoError(t, err)
	b, err := SyntheticPNG(ImageSpec{Kind: "synthetic", Seed: 42, Width: 16, Height: 16})
	require.NoError(t, err)
	assert.Equal(t, a, b)
	assert.Greater(t, len(a), 32)
	// PNG magic
	assert.Equal(t, []byte{0x89, 0x50, 0x4e, 0x47}, a[:4])
}

func TestGoldenStub_SchemaAndPerfectMetrics(t *testing.T) {
	m, err := LoadManifest("")
	require.NoError(t, err)

	stub := NewGoldenStub(m, StubConfig{
		Model:         "stub-golden-v1",
		PromptVersion: "detect-stub",
		LatencyFloor:  time.Microsecond,
	})

	// Schema contract on a capturable fixture
	img, err := SyntheticPNG(m.Fixtures[0].Image)
	require.NoError(t, err)
	res, err := stub.DetectContext(context.Background(), img, m.Fixtures[0].ID+".png")
	require.NoError(t, err)
	require.NoError(t, ValidateDetectSchema(res))
	assert.Equal(t, "mock", res.Source)
	assert.Equal(t, "stub-golden-v1", res.Model)
	assert.Equal(t, "detect-stub", res.PromptVersion)
	assert.NotEmpty(t, res.Animals)
	assert.Equal(t, m.Fixtures[0].Expected.Species, res.Animals[0].Species)

	// Empty / negative fixture → no capturable animals
	var emptyFix *Fixture
	for i := range m.Fixtures {
		if m.Fixtures[i].SpeciesGroup == "empty" {
			emptyFix = &m.Fixtures[i]
			break
		}
	}
	require.NotNil(t, emptyFix)
	img2, err := SyntheticPNG(emptyFix.Image)
	require.NoError(t, err)
	res2, err := stub.DetectContext(context.Background(), img2, emptyFix.ID+".png")
	require.NoError(t, err)
	require.NoError(t, ValidateDetectSchema(res2))
	assert.Empty(t, res2.Animals)

	// Full evaluation
	eval, err := Evaluate(context.Background(), m, stub, "stub")
	require.NoError(t, err)
	require.NoError(t, ValidateMetricsReportStructure(eval.Metrics))
	assert.Equal(t, len(m.Fixtures), eval.FixtureCount)
	assert.Equal(t, 1.0, eval.Metrics.MacroPrecision)
	assert.Equal(t, 1.0, eval.Metrics.MacroRecall)
	assert.Equal(t, 1.0, eval.Metrics.UnknownRejection)
	assert.Equal(t, 1.0, eval.Metrics.MeanIoU)
	assert.Equal(t, "USD", eval.Metrics.Cost.Currency)
	assert.Greater(t, eval.Metrics.LatencyMs.Count, 0)
	assert.Equal(t, "stub-golden-v1", eval.Trace.Model)
	assert.Equal(t, "detect-stub", eval.Trace.PromptVersion)

	// JSON serializable metrics report (contract for CI artifact)
	raw, err := json.Marshal(eval.Metrics)
	require.NoError(t, err)
	var round MetricsReport
	require.NoError(t, json.Unmarshal(raw, &round))
	assert.InDelta(t, 1.0, round.MacroPrecision, 1e-9)
}

func TestBaselineDiff_PassesPerfectStub(t *testing.T) {
	m, err := LoadManifest("")
	require.NoError(t, err)
	base, err := LoadBaseline("")
	require.NoError(t, err)

	stub := NewGoldenStub(m, StubConfig{
		Model:         "stub-golden-v1",
		PromptVersion: "detect-stub",
	})
	eval, err := Evaluate(context.Background(), m, stub, "stub")
	require.NoError(t, err)

	diff := DiffAgainstBaseline(eval.Metrics, base.Metrics, m.Thresholds)
	if !diff.Passed {
		t.Fatalf("expected pass:\n%s", FormatDiff(diff))
	}
}

func TestBaselineDiff_FailsOnRegression(t *testing.T) {
	m, err := LoadManifest("")
	require.NoError(t, err)
	base, err := LoadBaseline("")
	require.NoError(t, err)

	// Inject: force cat fixtures empty → recall drop; inject dog on empty → precision/unknown drop
	injectEmpty := map[string]bool{}
	injectExtra := map[string]string{}
	for _, f := range m.Fixtures {
		if f.SpeciesGroup == "cat" {
			injectEmpty[f.ID] = true
		}
		if f.SpeciesGroup == "empty" {
			injectExtra[f.ID] = "dog"
		}
	}

	stub := NewGoldenStub(m, StubConfig{
		Model:                 "stub-regressed",
		PromptVersion:         "detect-stub-bad",
		InjectEmpty:           injectEmpty,
		InjectExtraCapturable: injectExtra,
	})
	eval, err := Evaluate(context.Background(), m, stub, "stub")
	require.NoError(t, err)

	diff := DiffAgainstBaseline(eval.Metrics, base.Metrics, m.Thresholds)
	require.False(t, diff.Passed, "expected baseline failure on injected regression")
	assert.NotEmpty(t, diff.Violations)
	t.Log(FormatDiff(diff))

	// Failed samples must retain model/prompt for backtracking
	foundFail := false
	for _, s := range eval.Samples {
		if !s.CapturableOK {
			foundFail = true
			assert.NotEmpty(t, s.Model)
			assert.NotEmpty(t, s.PromptVersion)
		}
	}
	assert.True(t, foundFail)
}

func TestMetricsReportStructure_RejectsIncomplete(t *testing.T) {
	err := ValidateMetricsReportStructure(MetricsReport{})
	require.Error(t, err)

	err = ValidateMetricsReportStructure(MetricsReport{
		PerClass: map[string]ClassMetrics{
			"cat":   {Precision: 1, Recall: 1},
			"dog":   {Precision: 1, Recall: 1},
			"goose": {Precision: 1, Recall: 1},
		},
		MacroPrecision:   1,
		MacroRecall:      1,
		UnknownRejection: 1,
		Cost:             CostMetrics{Currency: "USD"},
	})
	require.NoError(t, err)
}

func TestIoU_PerfectAndDisjoint(t *testing.T) {
	a := BBox{X: 0.1, Y: 0.1, Width: 0.4, Height: 0.4}
	assert.InDelta(t, 1.0, IoU(a, a), 1e-9)
	b := BBox{X: 0.6, Y: 0.6, Width: 0.2, Height: 0.2}
	assert.InDelta(t, 0.0, IoU(a, b), 1e-9)
}
