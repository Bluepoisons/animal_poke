package mlqa

import (
	"fmt"
	"strings"
)

// DiffAgainstBaseline compares current metrics to a stored baseline using
// thresholds from the manifest. Fails closed on regression beyond limits.
func DiffAgainstBaseline(current MetricsReport, baseline MetricsReport, th Thresholds) DiffResult {
	var violations []DiffViolation

	// Absolute floors (manifest mins)
	if th.MinUnknownRejection > 0 && current.UnknownRejection+1e-9 < th.MinUnknownRejection {
		violations = append(violations, DiffViolation{
			Metric:   "unknown_rejection",
			Baseline: baseline.UnknownRejection,
			Current:  current.UnknownRejection,
			Limit:    th.MinUnknownRejection,
			Message:  fmt.Sprintf("unknown_rejection %.4f below min %.4f", current.UnknownRejection, th.MinUnknownRejection),
		})
	}

	// Macro drops vs baseline
	if drop := baseline.MacroPrecision - current.MacroPrecision; drop > th.PrecisionDrop+1e-9 {
		violations = append(violations, DiffViolation{
			Metric:   "macro_precision",
			Baseline: baseline.MacroPrecision,
			Current:  current.MacroPrecision,
			Limit:    th.PrecisionDrop,
			Message:  fmt.Sprintf("macro_precision dropped by %.4f (limit %.4f)", drop, th.PrecisionDrop),
		})
	}
	if drop := baseline.MacroRecall - current.MacroRecall; drop > th.RecallDrop+1e-9 {
		violations = append(violations, DiffViolation{
			Metric:   "macro_recall",
			Baseline: baseline.MacroRecall,
			Current:  current.MacroRecall,
			Limit:    th.RecallDrop,
			Message:  fmt.Sprintf("macro_recall dropped by %.4f (limit %.4f)", drop, th.RecallDrop),
		})
	}
	if drop := baseline.UnknownRejection - current.UnknownRejection; drop > th.UnknownRejectionDrop+1e-9 {
		violations = append(violations, DiffViolation{
			Metric:   "unknown_rejection",
			Baseline: baseline.UnknownRejection,
			Current:  current.UnknownRejection,
			Limit:    th.UnknownRejectionDrop,
			Message:  fmt.Sprintf("unknown_rejection dropped by %.4f (limit %.4f)", drop, th.UnknownRejectionDrop),
		})
	}
	if drop := baseline.MeanIoU - current.MeanIoU; drop > th.MeanIoUDrop+1e-9 {
		violations = append(violations, DiffViolation{
			Metric:   "mean_iou",
			Baseline: baseline.MeanIoU,
			Current:  current.MeanIoU,
			Limit:    th.MeanIoUDrop,
			Message:  fmt.Sprintf("mean_iou dropped by %.4f (limit %.4f)", drop, th.MeanIoUDrop),
		})
	}

	// Latency regression: only flag large increases (stub latencies are tiny).
	if th.P95LatencyMsIncrease > 0 {
		inc := current.LatencyMs.P95 - baseline.LatencyMs.P95
		if inc > th.P95LatencyMsIncrease+1e-9 {
			violations = append(violations, DiffViolation{
				Metric:   "latency_p95_ms",
				Baseline: baseline.LatencyMs.P95,
				Current:  current.LatencyMs.P95,
				Limit:    th.P95LatencyMsIncrease,
				Message:  fmt.Sprintf("p95 latency increased by %.2fms (limit %.2fms)", inc, th.P95LatencyMsIncrease),
			})
		}
	}

	// Per-class floors and drops
	for _, c := range CapturableClasses {
		cur, okC := current.PerClass[c]
		base, okB := baseline.PerClass[c]
		if !okC {
			violations = append(violations, DiffViolation{
				Metric:  "per_class." + c,
				Message: "missing per_class metrics for " + c,
			})
			continue
		}
		if th.MinPerClassPrecision > 0 && cur.Precision+1e-9 < th.MinPerClassPrecision {
			violations = append(violations, DiffViolation{
				Metric:  "per_class." + c + ".precision",
				Current: cur.Precision,
				Limit:   th.MinPerClassPrecision,
				Message: fmt.Sprintf("%s precision %.4f below min %.4f", c, cur.Precision, th.MinPerClassPrecision),
			})
		}
		if th.MinPerClassRecall > 0 && cur.Recall+1e-9 < th.MinPerClassRecall {
			violations = append(violations, DiffViolation{
				Metric:  "per_class." + c + ".recall",
				Current: cur.Recall,
				Limit:   th.MinPerClassRecall,
				Message: fmt.Sprintf("%s recall %.4f below min %.4f", c, cur.Recall, th.MinPerClassRecall),
			})
		}
		if okB {
			if drop := base.Precision - cur.Precision; drop > th.PrecisionDrop+1e-9 {
				violations = append(violations, DiffViolation{
					Metric:   "per_class." + c + ".precision",
					Baseline: base.Precision,
					Current:  cur.Precision,
					Limit:    th.PrecisionDrop,
					Message:  fmt.Sprintf("%s precision dropped by %.4f", c, drop),
				})
			}
			if drop := base.Recall - cur.Recall; drop > th.RecallDrop+1e-9 {
				violations = append(violations, DiffViolation{
					Metric:   "per_class." + c + ".recall",
					Baseline: base.Recall,
					Current:  cur.Recall,
					Limit:    th.RecallDrop,
					Message:  fmt.Sprintf("%s recall dropped by %.4f", c, drop),
				})
			}
		}
	}

	return DiffResult{
		Passed:     len(violations) == 0,
		Violations: violations,
	}
}

// FormatDiff returns a human-readable multi-line summary.
func FormatDiff(d DiffResult) string {
	if d.Passed {
		return "baseline diff: PASS (no regressions beyond thresholds)"
	}
	var b strings.Builder
	b.WriteString("baseline diff: FAIL\n")
	for _, v := range d.Violations {
		b.WriteString("  - ")
		if v.Message != "" {
			b.WriteString(v.Message)
		} else {
			fmt.Fprintf(&b, "%s baseline=%.4f current=%.4f limit=%.4f", v.Metric, v.Baseline, v.Current, v.Limit)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
