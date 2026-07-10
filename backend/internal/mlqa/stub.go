package mlqa

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"animalpoke/backend/internal/services"
	"animalpoke/backend/internal/taxonomy"
)

// Detector is the minimal interface the golden runner needs.
// *services.AIService satisfies this in real-provider mode.
type Detector interface {
	DetectContext(ctx context.Context, imageData []byte, filename string) (*services.DetectResult, error)
}

// StubConfig tunes the perfect/near-perfect golden stub.
type StubConfig struct {
	// Model / PromptVersion written into DetectResult for traceability.
	Model         string
	PromptVersion string
	// LatencyFloor simulates a non-zero but tiny latency floor.
	LatencyFloor time.Duration
	// InjectSpecies overrides the predicted species for a fixture id (regression tests).
	InjectSpecies map[string]string
	// InjectEmpty forces empty animals for a fixture id.
	InjectEmpty map[string]bool
	// InjectExtraCapturable forces an extra wrong capturable detection.
	InjectExtraCapturable map[string]string
}

// GoldenStub is a fixture-aware mock vision provider for PR contract tests.
// It returns perfect predictions derived from the golden manifest expected labels
// (not image pixels), so schema/metrics/baseline gates run without API keys.
type GoldenStub struct {
	byID map[string]Fixture
	cfg  StubConfig
	// lastTrace is updated on each Detect for report assembly.
	lastModel string
	lastPV    string
}

// NewGoldenStub builds a stub from a loaded manifest.
func NewGoldenStub(m *Manifest, cfg StubConfig) *GoldenStub {
	if cfg.Model == "" {
		cfg.Model = "stub-golden-v1"
	}
	if cfg.PromptVersion == "" {
		cfg.PromptVersion = "detect-stub"
	}
	if cfg.LatencyFloor <= 0 {
		cfg.LatencyFloor = time.Microsecond
	}
	byID := make(map[string]Fixture, len(m.Fixtures))
	for _, f := range m.Fixtures {
		byID[f.ID] = f
	}
	return &GoldenStub{byID: byID, cfg: cfg}
}

// DetectContext implements Detector.
// Filename should be "<fixture_id>.png" (as produced by the evaluator).
func (s *GoldenStub) DetectContext(ctx context.Context, imageData []byte, filename string) (*services.DetectResult, error) {
	_ = imageData
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if s.cfg.LatencyFloor > 0 {
		time.Sleep(s.cfg.LatencyFloor)
	}

	id := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	fix, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("golden stub: unknown fixture id %q from filename %q", id, filename)
	}

	s.lastModel = s.cfg.Model
	s.lastPV = s.cfg.PromptVersion

	if s.cfg.InjectEmpty != nil && s.cfg.InjectEmpty[id] {
		return &services.DetectResult{
			Animals:       []services.DetectBox{},
			Source:        "mock",
			Model:         s.cfg.Model,
			PromptVersion: s.cfg.PromptVersion,
		}, nil
	}

	result := &services.DetectResult{
		Animals:       []services.DetectBox{},
		Source:        "mock",
		Model:         s.cfg.Model,
		PromptVersion: s.cfg.PromptVersion,
	}

	species := fix.Expected.Species
	if s.cfg.InjectSpecies != nil {
		if inj, ok := s.cfg.InjectSpecies[id]; ok {
			species = inj
		}
	}

	// Perfect stub: capturable positives return expected box; negatives return empty
	// capturable set (taxonomy would strip unsupported labels).
	if fix.Expected.Capturable {
		box := services.DetectBox{
			Species:    species,
			Confidence: maxFloat(fix.Expected.MinConfidence, 0.92),
			Label:      species,
			TargetID:   "0",
		}
		if fix.Expected.BBox != nil {
			box.BoundingBox.X = fix.Expected.BBox.X
			box.BoundingBox.Y = fix.Expected.BBox.Y
			box.BoundingBox.Width = fix.Expected.BBox.Width
			box.BoundingBox.Height = fix.Expected.BBox.Height
		}
		// Normalize via taxonomy so contract matches production path.
		norm, orig := taxonomy.Normalize(box.Species)
		box.Species = norm
		if box.Label == "" {
			box.Label = orig
		}
		if taxonomy.Capturable(norm) {
			result.Animals = append(result.Animals, box)
		}
	}
	// else: empty animals — correct unknown rejection

	if s.cfg.InjectExtraCapturable != nil {
		if extra, ok := s.cfg.InjectExtraCapturable[id]; ok && extra != "" {
			norm, orig := taxonomy.Normalize(extra)
			if taxonomy.Capturable(norm) {
				result.Animals = append(result.Animals, services.DetectBox{
					Species:    norm,
					Label:      orig,
					Confidence: 0.8,
					TargetID:   "inject",
					BoundingBox: struct {
						X      float64 `json:"x"`
						Y      float64 `json:"y"`
						Width  float64 `json:"width"`
						Height float64 `json:"height"`
					}{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2},
				})
			}
		}
	}

	return result, nil
}

// Trace returns the last model/prompt version used by the stub.
func (s *GoldenStub) Trace() TraceInfo {
	return TraceInfo{
		Model:         firstNonEmpty(s.lastModel, s.cfg.Model),
		PromptVersion: firstNonEmpty(s.lastPV, s.cfg.PromptVersion),
		Provider:      "mock",
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
