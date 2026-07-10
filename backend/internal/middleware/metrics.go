package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// Bounded in-process Prometheus-style metrics.
// Label paths MUST be Gin route templates (c.FullPath()) or the fixed token "unknown".
// Maps are capped to prevent high-cardinality DoS from random URL paths.

const (
	// MaxHTTPSeriesKeys bounds distinct method|path|status request series.
	MaxHTTPSeriesKeys = 512
	// MaxLatencyKeys bounds distinct method|path latency series.
	MaxLatencyKeys = 256
	// MaxAICostKeys bounds AI call type labels.
	MaxAICostKeys = 64
	MaxSyncKeys   = 32
	MaxFunnelKeys = 128
	MaxConfKeys   = 8
	// UnknownPath is the fixed label when FullPath is empty (404 / unmatched).
	UnknownPath = "unknown"
)

var (
	metricsMu sync.RWMutex

	httpRequests   = make(map[string]*uint64) // method|path|status
	httpLatencySum = make(map[string]*uint64) // method|path -> ms sum
	httpLatencyCnt = make(map[string]*uint64) // method|path
	aiCostCalls    = make(map[string]*uint64) // type|provider|outcome
	aiConfidence   = make(map[string]*uint64) // bucket
	syncOutcomes   = make(map[string]*uint64)
	funnelStages   = make(map[string]*uint64) // stage|result
	aiEmptyDetect  atomic.Uint64

	droppedSeries atomic.Uint64
	rateLimitHits atomic.Uint64
	nonceReplays  atomic.Uint64
	cacheHits     atomic.Uint64
	cacheMisses   atomic.Uint64

	releaseSHA       atomic.Value // string
	processStartUnix = time.Now().Unix()
)

// ResetMetrics clears all series (tests only).
func ResetMetrics() {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	httpRequests = make(map[string]*uint64)
	httpLatencySum = make(map[string]*uint64)
	httpLatencyCnt = make(map[string]*uint64)
	aiCostCalls = make(map[string]*uint64)
	aiConfidence = make(map[string]*uint64)
	syncOutcomes = make(map[string]*uint64)
	funnelStages = make(map[string]*uint64)
	aiEmptyDetect.Store(0)
	droppedSeries.Store(0)
	rateLimitHits.Store(0)
	nonceReplays.Store(0)
	cacheHits.Store(0)
	cacheMisses.Store(0)
}

// HTTPSeriesCount returns the number of http_requests_total series keys.
func HTTPSeriesCount() int {
	metricsMu.RLock()
	defer metricsMu.RUnlock()
	return len(httpRequests)
}

// LatencySeriesCount returns the number of latency series keys.
func LatencySeriesCount() int {
	metricsMu.RLock()
	defer metricsMu.RUnlock()
	return len(httpLatencySum)
}

// DroppedSeries returns how many new series were rejected after the cap.
func DroppedSeries() uint64 {
	return droppedSeries.Load()
}

func incBounded(m map[string]*uint64, key string, max int) {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	if v, ok := m[key]; ok {
		atomic.AddUint64(v, 1)
		return
	}
	if len(m) >= max {
		droppedSeries.Add(1)
		return
	}
	v := new(uint64)
	*v = 1
	m[key] = v
}

func addBounded(m map[string]*uint64, key string, delta uint64, max int) {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	if v, ok := m[key]; ok {
		atomic.AddUint64(v, delta)
		return
	}
	if len(m) >= max {
		droppedSeries.Add(1)
		return
	}
	v := new(uint64)
	*v = delta
	m[key] = v
}

// RoutePathForMetrics returns a fixed-cardinality path label.
// Prefer Gin FullPath templates; empty / unmatched → "unknown".
func RoutePathForMetrics(fullPath string) string {
	p := strings.TrimSpace(fullPath)
	if p == "" {
		return UnknownPath
	}
	// Defensive: never accept extremely long labels even from misconfigured routes.
	if len(p) > 128 {
		return UnknownPath
	}
	return p
}

// ObserveHTTP records HTTP metrics. path MUST be a route template (c.FullPath())
// or empty (mapped to "unknown"). Raw request URLs must not be passed here.
func ObserveHTTP(method, path string, status int, latency time.Duration) {
	p := RoutePathForMetrics(path)
	method = normalizeMethod(method)
	statusLabel := statusClass(status)
	key := method + "|" + p + "|" + statusLabel
	incBounded(httpRequests, key, MaxHTTPSeriesKeys)
	lk := method + "|" + p
	addBounded(httpLatencySum, lk, uint64(latency.Milliseconds()), MaxLatencyKeys)
	incBounded(httpLatencyCnt, lk, MaxLatencyKeys)
}

func normalizeMethod(m string) string {
	m = strings.ToUpper(strings.TrimSpace(m))
	switch m {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return m
	default:
		return "OTHER"
	}
}

// statusClass collapses status codes to fixed buckets to bound cardinality.
func statusClass(status int) string {
	switch {
	case status >= 100 && status < 200:
		return "1xx"
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "unknown"
	}
}

// ObserveAICost records AI provider call counts by type (bounded labels).
func ObserveAICost(callType string) {
	ObserveAI(callType, "unknown", "ok")
}

// SetReleaseSHA sets process release identity for metrics labels.
func SetReleaseSHA(sha string) {
	sha = strings.TrimSpace(sha)
	if sha == "" {
		sha = "unknown"
	}
	if len(sha) > 40 {
		sha = sha[:40]
	}
	releaseSHA.Store(sha)
}

// GetReleaseSHA returns release identity.
func GetReleaseSHA() string {
	if v := releaseSHA.Load(); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "unknown"
}

func sanitizeMetricLabel(s, fallback string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return fallback
	}
	if len(s) > 32 {
		s = s[:32]
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return fallback
	}
	return out
}

// ObserveAI records AI call with low-cardinality provider/outcome labels.
func ObserveAI(callType, provider, outcome string) {
	ct := sanitizeMetricLabel(callType, "unknown")
	pr := sanitizeMetricLabel(provider, "unknown")
	oc := sanitizeMetricLabel(outcome, "ok")
	incBounded(aiCostCalls, ct+"|"+pr+"|"+oc, MaxAICostKeys)
}

// ObserveDetectEmpty records empty vision detections.
func ObserveDetectEmpty() { aiEmptyDetect.Add(1) }

// ObserveConfidence records coarse confidence buckets.
func ObserveConfidence(maxConfidence float64) {
	bucket := "lt50"
	switch {
	case maxConfidence >= 0.8:
		bucket = "ge80"
	case maxConfidence >= 0.5:
		bucket = "50_80"
	}
	incBounded(aiConfidence, bucket, MaxConfKeys)
}

// ObserveSyncOutcome records sync results.
func ObserveSyncOutcome(outcome string) {
	incBounded(syncOutcomes, sanitizeMetricLabel(outcome, "unknown"), MaxSyncKeys)
}

// ObserveFunnel records core-loop funnel stages.
func ObserveFunnel(stage, result string) {
	key := sanitizeMetricLabel(stage, "unknown") + "|" + sanitizeMetricLabel(result, "unknown")
	incBounded(funnelStages, key, MaxFunnelKeys)
}

// ObserveRateLimit records rate-limit hits.
func ObserveRateLimit() {
	rateLimitHits.Add(1)
}

// ObserveNonceReplay records nonce replay rejects.
func ObserveNonceReplay() {
	nonceReplays.Add(1)
}

// ObserveCache records cache hit/miss.
func ObserveCache(hit bool) {
	if hit {
		cacheHits.Add(1)
	} else {
		cacheMisses.Add(1)
	}
}

// RenderMetrics returns Prometheus text exposition.
func RenderMetrics() string {
	var b strings.Builder

	metricsMu.RLock()
	reqSnap := make(map[string]uint64, len(httpRequests))
	for k, v := range httpRequests {
		reqSnap[k] = atomic.LoadUint64(v)
	}
	sumSnap := make(map[string]uint64, len(httpLatencySum))
	for k, v := range httpLatencySum {
		sumSnap[k] = atomic.LoadUint64(v)
	}
	cntSnap := make(map[string]uint64, len(httpLatencyCnt))
	for k, v := range httpLatencyCnt {
		cntSnap[k] = atomic.LoadUint64(v)
	}
	aiSnap := make(map[string]uint64, len(aiCostCalls))
	for k, v := range aiCostCalls {
		aiSnap[k] = atomic.LoadUint64(v)
	}
	confSnap := make(map[string]uint64, len(aiConfidence))
	for k, v := range aiConfidence {
		confSnap[k] = atomic.LoadUint64(v)
	}
	syncSnap := make(map[string]uint64, len(syncOutcomes))
	for k, v := range syncOutcomes {
		syncSnap[k] = atomic.LoadUint64(v)
	}
	funnelSnap := make(map[string]uint64, len(funnelStages))
	for k, v := range funnelStages {
		funnelSnap[k] = atomic.LoadUint64(v)
	}
	metricsMu.RUnlock()

	b.WriteString("# HELP http_requests_total Total HTTP requests\n")
	b.WriteString("# TYPE http_requests_total counter\n")
	for k, v := range reqSnap {
		parts := strings.SplitN(k, "|", 3)
		if len(parts) != 3 {
			continue
		}
		fmt.Fprintf(&b, "http_requests_total{method=%q,path=%q,status=%q} %d\n",
			parts[0], parts[1], parts[2], v)
	}

	b.WriteString("# HELP http_request_duration_ms_sum Request latency sum in ms\n")
	b.WriteString("# TYPE http_request_duration_ms_sum counter\n")
	for k, v := range sumSnap {
		parts := strings.SplitN(k, "|", 2)
		if len(parts) != 2 {
			continue
		}
		fmt.Fprintf(&b, "http_request_duration_ms_sum{method=%q,path=%q} %d\n",
			parts[0], parts[1], v)
	}

	b.WriteString("# HELP http_request_duration_ms_count Request latency count\n")
	b.WriteString("# TYPE http_request_duration_ms_count counter\n")
	for k, v := range cntSnap {
		parts := strings.SplitN(k, "|", 2)
		if len(parts) != 2 {
			continue
		}
		fmt.Fprintf(&b, "http_request_duration_ms_count{method=%q,path=%q} %d\n",
			parts[0], parts[1], v)
	}

	b.WriteString("# HELP ai_calls_total AI provider calls by type/provider/outcome\n")
	b.WriteString("# TYPE ai_calls_total counter\n")
	for k, v := range aiSnap {
		parts := strings.SplitN(k, "|", 3)
		if len(parts) != 3 {
			fmt.Fprintf(&b, "ai_calls_total{type=%q,provider=%q,outcome=%q} %d\n", k, "unknown", "unknown", v)
			continue
		}
		fmt.Fprintf(&b, "ai_calls_total{type=%q,provider=%q,outcome=%q} %d\n", parts[0], parts[1], parts[2], v)
	}

	b.WriteString("# HELP ai_detect_empty_total Empty vision detect results\n")
	b.WriteString("# TYPE ai_detect_empty_total counter\n")
	fmt.Fprintf(&b, "ai_detect_empty_total %d\n", aiEmptyDetect.Load())

	b.WriteString("# HELP ai_confidence_bucket_total Detection confidence buckets\n")
	b.WriteString("# TYPE ai_confidence_bucket_total counter\n")
	for k, v := range confSnap {
		fmt.Fprintf(&b, "ai_confidence_bucket_total{bucket=%q} %d\n", k, v)
	}

	b.WriteString("# HELP sync_outcomes_total Sync queue outcomes\n")
	b.WriteString("# TYPE sync_outcomes_total counter\n")
	for k, v := range syncSnap {
		fmt.Fprintf(&b, "sync_outcomes_total{outcome=%q} %d\n", k, v)
	}

	b.WriteString("# HELP game_funnel_total Core game funnel stages\n")
	b.WriteString("# TYPE game_funnel_total counter\n")
	for k, v := range funnelSnap {
		parts := strings.SplitN(k, "|", 2)
		if len(parts) != 2 {
			continue
		}
		fmt.Fprintf(&b, "game_funnel_total{stage=%q,result=%q} %d\n", parts[0], parts[1], v)
	}

	fmt.Fprintf(&b, "# HELP rate_limit_hits_total Rate limit hits\n# TYPE rate_limit_hits_total counter\nrate_limit_hits_total %d\n", rateLimitHits.Load())
	fmt.Fprintf(&b, "# HELP nonce_replays_total Nonce replay rejects\n# TYPE nonce_replays_total counter\nnonce_replays_total %d\n", nonceReplays.Load())
	fmt.Fprintf(&b, "# HELP cache_hits_total Cache hits\n# TYPE cache_hits_total counter\ncache_hits_total %d\n", cacheHits.Load())
	fmt.Fprintf(&b, "# HELP cache_misses_total Cache misses\n# TYPE cache_misses_total counter\ncache_misses_total %d\n", cacheMisses.Load())
	fmt.Fprintf(&b, "# HELP metrics_series_dropped_total Series dropped after cardinality cap\n# TYPE metrics_series_dropped_total counter\nmetrics_series_dropped_total %d\n", droppedSeries.Load())
	fmt.Fprintf(&b, "# HELP process_start_time_seconds Process start unix time\n# TYPE process_start_time_seconds gauge\nprocess_start_time_seconds %d\n", processStartUnix)
	fmt.Fprintf(&b, "# HELP app_info App release identity\n# TYPE app_info gauge\napp_info{release=%q} 1\n", GetReleaseSHA())

	return b.String()
}

// MetricsHandler is a Gin handler for /metrics (management server only).
func MetricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(RenderMetrics()))
	}
}

// MetricsHTTPHandler serves Prometheus text on net/http.
func MetricsHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body := RenderMetrics()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte(body))
		}
	})
}

// NewMetricsServer builds a dedicated management HTTP server for Prometheus scrape.
// Bind via METRICS_ADDR (default :9090). Do not expose this Service on public Ingress.
func NewMetricsServer(addr string) *http.Server {
	if strings.TrimSpace(addr) == "" {
		addr = ":9090"
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", MetricsHTTPHandler())
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
}
