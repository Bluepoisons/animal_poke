package mlqa

// Manifest is the vision golden-set description loaded from testdata.
type Manifest struct {
	Version           string     `json:"version"`
	Description       string     `json:"description"`
	ModelTrack        string     `json:"model_track"`
	Classes           []string   `json:"classes"`
	ConditionsCatalog []string   `json:"conditions_catalog"`
	Thresholds        Thresholds `json:"thresholds"`
	Fixtures          []Fixture  `json:"fixtures"`
}

// Thresholds control baseline regression gates.
type Thresholds struct {
	PrecisionDrop        float64 `json:"precision_drop"`
	RecallDrop           float64 `json:"recall_drop"`
	UnknownRejectionDrop float64 `json:"unknown_rejection_drop"`
	MeanIoUDrop          float64 `json:"mean_iou_drop"`
	P95LatencyMsIncrease float64 `json:"p95_latency_ms_increase"`
	MinUnknownRejection  float64 `json:"min_unknown_rejection"`
	MinPerClassPrecision float64 `json:"min_per_class_precision"`
	MinPerClassRecall    float64 `json:"min_per_class_recall"`
}

// Fixture is one labeled golden sample.
type Fixture struct {
	ID           string        `json:"id"`
	SpeciesGroup string        `json:"species_group"`
	Conditions   []string      `json:"conditions"`
	Expected     ExpectedLabel `json:"expected"`
	Image        ImageSpec     `json:"image"`
	Notes        string        `json:"notes,omitempty"`
}

// ExpectedLabel is ground-truth for a fixture.
type ExpectedLabel struct {
	Species       string  `json:"species"`
	Capturable    bool    `json:"capturable"`
	BBox          *BBox   `json:"bbox"`
	MinConfidence float64 `json:"min_confidence"`
}

// BBox is a normalized axis-aligned box in [0,1].
type BBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ImageSpec describes how to materialize a synthetic image at test time.
type ImageSpec struct {
	Kind   string `json:"kind"` // synthetic
	Seed   int    `json:"seed"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ClassMetrics holds precision/recall for one capturable class.
type ClassMetrics struct {
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	Support   int     `json:"support"`
	TP        int     `json:"tp"`
	FP        int     `json:"fp"`
	FN        int     `json:"fn"`
}

// LatencyMetrics summarizes inference latency.
type LatencyMetrics struct {
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
	Mean  float64 `json:"mean"`
	Count int     `json:"count"`
}

// CostMetrics is a cost placeholder for real-provider runs.
type CostMetrics struct {
	Currency string  `json:"currency"`
	Total    float64 `json:"total"`
	PerCall  float64 `json:"per_call"`
	Note     string  `json:"note,omitempty"`
}

// MetricsReport is the structured certification output.
type MetricsReport struct {
	PerClass                map[string]ClassMetrics `json:"per_class"`
	MacroPrecision          float64                 `json:"macro_precision"`
	MacroRecall             float64                 `json:"macro_recall"`
	UnknownRejection        float64                 `json:"unknown_rejection"`
	UnknownRejectionSupport int                     `json:"unknown_rejection_support"`
	MeanIoU                 float64                 `json:"mean_iou"`
	IoUSampleCount          int                     `json:"iou_sample_count"`
	CalibrationError        float64                 `json:"calibration_error"`
	LatencyMs               LatencyMetrics          `json:"latency_ms"`
	Cost                    CostMetrics             `json:"cost"`
}

// TraceInfo ties a report to model/prompt versions for failure backtracking.
type TraceInfo struct {
	Model         string `json:"model"`
	PromptVersion string `json:"prompt_version"`
	Provider      string `json:"provider"`
}

// BaselineFile is the on-disk baseline used for regression gating.
type BaselineFile struct {
	Version      string        `json:"version"`
	GeneratedBy  string        `json:"generated_by"`
	Mode         string        `json:"mode"`
	FixtureCount int           `json:"fixture_count"`
	Metrics      MetricsReport `json:"metrics"`
	Trace        TraceInfo     `json:"trace"`
}

// SampleResult is one fixture evaluation outcome (for debugging failed samples).
type SampleResult struct {
	FixtureID       string   `json:"fixture_id"`
	ExpectedSpecies string   `json:"expected_species"`
	Predicted       []string `json:"predicted_species"`
	CapturableOK    bool     `json:"capturable_ok"`
	IoU             float64  `json:"iou,omitempty"`
	LatencyMs       float64  `json:"latency_ms"`
	Error           string   `json:"error,omitempty"`
	Model           string   `json:"model,omitempty"`
	PromptVersion   string   `json:"prompt_version,omitempty"`
}

// EvaluationResult is the full run output.
type EvaluationResult struct {
	Mode         string         `json:"mode"`
	FixtureCount int            `json:"fixture_count"`
	Metrics      MetricsReport  `json:"metrics"`
	Trace        TraceInfo      `json:"trace"`
	Samples      []SampleResult `json:"samples"`
}

// DiffViolation is one metric that failed a baseline/threshold check.
type DiffViolation struct {
	Metric   string  `json:"metric"`
	Baseline float64 `json:"baseline"`
	Current  float64 `json:"current"`
	Limit    float64 `json:"limit"`
	Message  string  `json:"message"`
}

// DiffResult is the baseline comparison outcome.
type DiffResult struct {
	Passed     bool            `json:"passed"`
	Violations []DiffViolation `json:"violations,omitempty"`
}
