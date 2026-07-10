package mlqa

import (
	"context"
	"fmt"
	"time"

	"animalpoke/backend/internal/services"
	"animalpoke/backend/internal/taxonomy"
)

// CapturableClasses are the classes scored for precision/recall.
var CapturableClasses = []string{
	taxonomy.SpeciesCat,
	taxonomy.SpeciesDog,
	taxonomy.SpeciesGoose,
}

// Evaluate runs the golden set against detector and produces a metrics report.
func Evaluate(ctx context.Context, m *Manifest, detector Detector, mode string) (*EvaluationResult, error) {
	if m == nil {
		return nil, fmt.Errorf("nil manifest")
	}
	if detector == nil {
		return nil, fmt.Errorf("nil detector")
	}
	if mode == "" {
		mode = "stub"
	}

	// Counters for capturable classes
	tp := map[string]int{}
	fp := map[string]int{}
	fn := map[string]int{}
	for _, c := range CapturableClasses {
		tp[c], fp[c], fn[c] = 0, 0, 0
	}

	unknownTotal := 0
	unknownRejected := 0
	var ious []float64
	var latencies []float64
	var confAbsErr []float64
	samples := make([]SampleResult, 0, len(m.Fixtures))

	var lastModel, lastPV string

	for _, fix := range m.Fixtures {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		img, err := SyntheticPNG(fix.Image)
		if err != nil {
			return nil, fmt.Errorf("synthetic png %s: %w", fix.ID, err)
		}
		filename := fix.ID + ".png"

		start := time.Now()
		res, err := detector.DetectContext(ctx, img, filename)
		elapsed := float64(time.Since(start).Microseconds()) / 1000.0 // ms
		latencies = append(latencies, elapsed)

		sr := SampleResult{
			FixtureID:       fix.ID,
			ExpectedSpecies: fix.Expected.Species,
			LatencyMs:       elapsed,
		}

		if err != nil {
			sr.Error = err.Error()
			samples = append(samples, sr)
			// count as miss for capturable expected
			if fix.Expected.Capturable {
				if _, ok := fn[fix.Expected.Species]; ok {
					fn[fix.Expected.Species]++
				}
			} else {
				unknownTotal++
				// error is not a clean rejection
			}
			continue
		}

		if res.Model != "" {
			lastModel = res.Model
			sr.Model = res.Model
		}
		if res.PromptVersion != "" {
			lastPV = res.PromptVersion
			sr.PromptVersion = res.PromptVersion
		}

		// Predicted capturable species (production already filters; be defensive).
		preds := make([]string, 0, len(res.Animals))
		var bestBox *BBox
		var bestConf float64
		for _, a := range res.Animals {
			norm, _ := taxonomy.Normalize(a.Species)
			if !taxonomy.Capturable(norm) {
				continue
			}
			preds = append(preds, norm)
			if bestBox == nil || a.Confidence > bestConf {
				bestConf = a.Confidence
				bb := BBox{X: a.BoundingBox.X, Y: a.BoundingBox.Y, Width: a.BoundingBox.Width, Height: a.BoundingBox.Height}
				bestBox = &bb
			}
			// calibration placeholder: |conf - 1| when correct species later
			_ = a
		}
		sr.Predicted = preds

		if fix.Expected.Capturable {
			exp := fix.Expected.Species
			// primary prediction: first / highest-conf capturable
			pred := ""
			if len(preds) > 0 {
				pred = preds[0]
			}
			hit := pred == exp
			if hit {
				tp[exp]++
				sr.CapturableOK = true
				if fix.Expected.BBox != nil && bestBox != nil {
					iou := IoU(*fix.Expected.BBox, *bestBox)
					ious = append(ious, iou)
					sr.IoU = iou
				}
				// calibration: target conf ~1 for correct
				if bestConf > 0 {
					confAbsErr = append(confAbsErr, mathAbs(bestConf-1.0))
				}
			} else {
				fn[exp]++
				if pred != "" {
					if _, ok := fp[pred]; ok {
						fp[pred]++
					}
				}
				sr.CapturableOK = false
			}
			// extra preds beyond first count as FP for those classes
			for i := 1; i < len(preds); i++ {
				if _, ok := fp[preds[i]]; ok {
					fp[preds[i]]++
				}
			}
		} else {
			unknownTotal++
			if len(preds) == 0 {
				unknownRejected++
				sr.CapturableOK = true
			} else {
				// false positive capturable on negative fixture
				for _, p := range preds {
					if _, ok := fp[p]; ok {
						fp[p]++
					}
				}
				sr.CapturableOK = false
			}
		}

		samples = append(samples, sr)
	}

	perClass := map[string]ClassMetrics{}
	var precSum, recSum float64
	nClass := 0
	for _, c := range CapturableClasses {
		cm := buildClassMetrics(tp[c], fp[c], fn[c])
		perClass[c] = cm
		precSum += cm.Precision
		recSum += cm.Recall
		nClass++
	}
	macroP, macroR := 1.0, 1.0
	if nClass > 0 {
		macroP = precSum / float64(nClass)
		macroR = recSum / float64(nClass)
	}

	unkRej := 1.0
	if unknownTotal > 0 {
		unkRej = float64(unknownRejected) / float64(unknownTotal)
	}

	meanIoU := 1.0
	if len(ious) > 0 {
		meanIoU = mean(ious)
	}

	calErr := 0.0
	if len(confAbsErr) > 0 {
		calErr = mean(confAbsErr)
	}

	provider := "mock"
	if mode == "real" {
		provider = "real"
	}

	out := &EvaluationResult{
		Mode:         mode,
		FixtureCount: len(m.Fixtures),
		Metrics: MetricsReport{
			PerClass:                perClass,
			MacroPrecision:          macroP,
			MacroRecall:             macroR,
			UnknownRejection:        unkRej,
			UnknownRejectionSupport: unknownTotal,
			MeanIoU:                 meanIoU,
			IoUSampleCount:          len(ious),
			CalibrationError:        calErr,
			LatencyMs:               summarizeLatency(latencies),
			Cost: CostMetrics{
				Currency: "USD",
				Total:    0,
				PerCall:  0,
				Note:     costNote(mode),
			},
		},
		Trace: TraceInfo{
			Model:         firstNonEmpty(lastModel, "unknown"),
			PromptVersion: firstNonEmpty(lastPV, "unknown"),
			Provider:      provider,
		},
		Samples: samples,
	}
	return out, nil
}

func costNote(mode string) string {
	if mode == "real" {
		return "populate from provider billing when running real certification"
	}
	return "stub has zero provider cost"
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// ValidateDetectSchema checks DetectResult contract fields used by certification.
func ValidateDetectSchema(r *services.DetectResult) error {
	if r == nil {
		return fmt.Errorf("nil detect result")
	}
	if r.Animals == nil {
		return fmt.Errorf("animals must be non-nil slice")
	}
	for i, a := range r.Animals {
		if a.Confidence < 0 || a.Confidence > 1 {
			return fmt.Errorf("animal[%d] confidence out of range", i)
		}
		bb := a.BoundingBox
		if bb.X < 0 || bb.Y < 0 || bb.Width < 0 || bb.Height < 0 ||
			bb.X > 1 || bb.Y > 1 || bb.Width > 1 || bb.Height > 1 ||
			bb.X+bb.Width > 1.0001 || bb.Y+bb.Height > 1.0001 {
			return fmt.Errorf("animal[%d] bounding_box out of range", i)
		}
		if a.Species == "" {
			return fmt.Errorf("animal[%d] species empty", i)
		}
	}
	return nil
}

// ValidateMetricsReportStructure ensures required metrics fields are present and finite.
func ValidateMetricsReportStructure(m MetricsReport) error {
	if m.PerClass == nil {
		return fmt.Errorf("per_class required")
	}
	for _, c := range CapturableClasses {
		cm, ok := m.PerClass[c]
		if !ok {
			return fmt.Errorf("per_class missing %s", c)
		}
		if cm.Precision < 0 || cm.Precision > 1 || cm.Recall < 0 || cm.Recall > 1 {
			return fmt.Errorf("per_class %s precision/recall out of range", c)
		}
	}
	if m.MacroPrecision < 0 || m.MacroPrecision > 1 {
		return fmt.Errorf("macro_precision out of range")
	}
	if m.MacroRecall < 0 || m.MacroRecall > 1 {
		return fmt.Errorf("macro_recall out of range")
	}
	if m.UnknownRejection < 0 || m.UnknownRejection > 1 {
		return fmt.Errorf("unknown_rejection out of range")
	}
	if m.LatencyMs.Count < 0 {
		return fmt.Errorf("latency count invalid")
	}
	if m.Cost.Currency == "" {
		return fmt.Errorf("cost.currency required")
	}
	return nil
}
