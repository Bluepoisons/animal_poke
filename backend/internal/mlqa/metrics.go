package mlqa

import (
	"math"
	"sort"
)

// IoU computes intersection-over-union for two normalized boxes.
func IoU(a, b BBox) float64 {
	ax2, ay2 := a.X+a.Width, a.Y+a.Height
	bx2, by2 := b.X+b.Width, b.Y+b.Height
	ix1 := math.Max(a.X, b.X)
	iy1 := math.Max(a.Y, b.Y)
	ix2 := math.Min(ax2, bx2)
	iy2 := math.Min(ay2, by2)
	iw := math.Max(0, ix2-ix1)
	ih := math.Max(0, iy2-iy1)
	inter := iw * ih
	if inter <= 0 {
		return 0
	}
	union := a.Width*a.Height + b.Width*b.Height - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

// percentile returns the p-th percentile (0-100) of a sorted copy of values.
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	if p <= 0 {
		return cp[0]
	}
	if p >= 100 {
		return cp[len(cp)-1]
	}
	// nearest-rank
	rank := int(math.Ceil(p/100*float64(len(cp)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(cp) {
		rank = len(cp) - 1
	}
	return cp[rank]
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var s float64
	for _, v := range values {
		s += v
	}
	return s / float64(len(values))
}

// buildClassMetrics computes precision/recall from TP/FP/FN.
func buildClassMetrics(tp, fp, fn int) ClassMetrics {
	cm := ClassMetrics{TP: tp, FP: fp, FN: fn, Support: tp + fn}
	if tp+fp > 0 {
		cm.Precision = float64(tp) / float64(tp+fp)
	} else if tp+fn == 0 {
		// no support and no false positives → treat as perfect placeholder
		cm.Precision = 1
	}
	if tp+fn > 0 {
		cm.Recall = float64(tp) / float64(tp+fn)
	} else {
		cm.Recall = 1
	}
	return cm
}

// summarizeLatency builds LatencyMetrics from raw ms samples.
func summarizeLatency(ms []float64) LatencyMetrics {
	return LatencyMetrics{
		P50:   percentile(ms, 50),
		P95:   percentile(ms, 95),
		P99:   percentile(ms, 99),
		Mean:  mean(ms),
		Count: len(ms),
	}
}
