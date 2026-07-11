// Package analytics implements AP-113 privacy-safe event store and metrics.
package analytics

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// Allowed event names (funnel).
var AllowedEvents = map[string]struct{}{
	"auth": {}, "camera_ok": {}, "scan": {}, "detect_result": {},
	"capture_attempt": {}, "generate_stage": {}, "collection_complete": {},
	"trade": {}, "battle_end": {}, "app_open": {}, "sync_fail": {},
}

// Event is a stored privacy-safe analytics event.
type Event struct {
	EventID           string
	OwnerKey          string // hashed/device-scoped key; never raw photo/token
	SessionID         string
	Name              string
	TS                time.Time
	SchemaVersion     int
	ExperimentID      string
	ExperimentVariant string
	AppVersion        string
	Region            string // coarse country/region only
	PropsJSON         string
	Deleted           bool
}

// Summary is ops-facing metrics with bounded latency metadata.
type Summary struct {
	DAU             int            `json:"dau"`
	Captures        int            `json:"captures"`
	CaptureSuccess  int            `json:"capture_success"`
	DetectOK        int            `json:"detect_ok"`
	SyncFailures    int            `json:"sync_failures"`
	Funnel          map[string]int `json:"funnel"`
	D1Retention     float64        `json:"d1_retention"`
	D7Retention     float64        `json:"d7_retention"`
	ByExperiment    map[string]int `json:"by_experiment,omitempty"`
	ByRegion        map[string]int `json:"by_region,omitempty"`
	ByVersion       map[string]int `json:"by_version,omitempty"`
	EventCount      int            `json:"event_count"`
	MaxEventAgeSec  int64          `json:"max_event_age_sec"`
	ComputedAt      time.Time      `json:"computed_at"`
	LatencyBoundSec int            `json:"latency_bound_sec"`
	DictionaryOwner string         `json:"dictionary_owner"`
	Source          string         `json:"source"`
	// Recomputable from raw events (gold tests assert).
	AsOf time.Time `json:"as_of"`
}

// Store is process-local durable-enough store (swap for DB later).
type Store struct {
	mu     sync.RWMutex
	byID   map[string]*Event
	now    func() time.Time
	retain time.Duration
}

// Default process store.
var (
	defaultStore *Store
	once         sync.Once
)

// Default returns the process-wide store.
func Default() *Store {
	once.Do(func() { defaultStore = NewStore(30 * 24 * time.Hour) })
	return defaultStore
}

// NewStore creates a store with retention window.
func NewStore(retain time.Duration) *Store {
	if retain <= 0 {
		retain = 30 * 24 * time.Hour
	}
	return &Store{
		byID:   map[string]*Event{},
		now:    func() time.Time { return time.Now().UTC() },
		retain: retain,
	}
}

// SetClock for tests.
func (s *Store) SetClock(now func() time.Time) { s.now = now }

// Ingest stores events with exactly-once event_id; late/out-of-order accepted if within retention.
func (s *Store) Ingest(evs []Event) (accepted, dropped, duplicate int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := s.now().Add(-s.retain)
	for i := range evs {
		ev := evs[i]
		if _, ok := AllowedEvents[ev.Name]; !ok || ev.EventID == "" || ev.SessionID == "" {
			dropped++
			continue
		}
		if !ev.TS.IsZero() && ev.TS.Before(cutoff) {
			dropped++ // too late beyond retention
			continue
		}
		if existing, ok := s.byID[ev.EventID]; ok {
			if existing.Deleted {
				// re-ingest after delete is ignored (privacy)
				dropped++
				continue
			}
			duplicate++
			continue
		}
		cp := ev
		if cp.TS.IsZero() {
			cp.TS = s.now()
		}
		if cp.SchemaVersion == 0 {
			cp.SchemaVersion = 1
		}
		s.byID[cp.EventID] = &cp
		accepted++
	}
	return
}

// DeleteOwner anonymizes/removes events for privacy delete (AP-113 + privacy).
func (s *Store) DeleteOwner(ownerKey string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, e := range s.byID {
		if e.OwnerKey == ownerKey && !e.Deleted {
			e.Deleted = true
			e.OwnerKey = ""
			e.SessionID = "redacted"
			e.PropsJSON = "{}"
			e.ExperimentID = ""
			e.ExperimentVariant = ""
			n++
		}
	}
	return n
}

// Summarize recomputes metrics from raw events (gold-testable).
func (s *Store) Summarize(asOf time.Time, window time.Duration) Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if asOf.IsZero() {
		asOf = s.now()
	}
	if window <= 0 {
		window = 24 * time.Hour
	}
	start := asOf.Add(-window)
	funnel := map[string]int{}
	ownersToday := map[string]struct{}{}
	ownersD0 := map[string]time.Time{}            // first seen day
	ownersActive := map[string]map[int]struct{}{} // owner -> day offsets
	byExp := map[string]int{}
	byReg := map[string]int{}
	byVer := map[string]int{}
	captures, captureOK, detectOK, syncFail := 0, 0, 0, 0
	count := 0
	var oldest time.Time
	for _, e := range s.byID {
		if e.Deleted {
			continue
		}
		count++
		if oldest.IsZero() || e.TS.Before(oldest) {
			oldest = e.TS
		}
		day := dayIndex(e.TS, asOf)
		if e.OwnerKey != "" {
			if _, ok := ownersActive[e.OwnerKey]; !ok {
				ownersActive[e.OwnerKey] = map[int]struct{}{}
			}
			ownersActive[e.OwnerKey][day] = struct{}{}
			if first, ok := ownersD0[e.OwnerKey]; !ok || e.TS.Before(first) {
				ownersD0[e.OwnerKey] = e.TS
			}
		}
		if e.TS.Before(start) || e.TS.After(asOf) {
			// still count retention from full history; funnel only in window
			continue
		}
		funnel[e.Name]++
		if e.OwnerKey != "" {
			ownersToday[e.OwnerKey] = struct{}{}
		}
		if e.ExperimentID != "" {
			byExp[e.ExperimentID+":"+e.ExperimentVariant]++
		}
		if e.Region != "" {
			byReg[e.Region]++
		}
		if e.AppVersion != "" {
			byVer[e.AppVersion]++
		}
		switch e.Name {
		case "capture_attempt":
			captures++
			if propOutcome(e.PropsJSON) == "success" {
				captureOK++
			}
		case "collection_complete":
			captures++
			captureOK++
		case "detect_result":
			if propOutcome(e.PropsJSON) == "success" {
				detectOK++
			}
		case "sync_fail":
			syncFail++
		}
	}
	d1, d7 := retention(ownersActive, ownersD0, asOf)
	age := int64(0)
	if !oldest.IsZero() {
		age = int64(asOf.Sub(oldest).Seconds())
	}
	return Summary{
		DAU:             len(ownersToday),
		Captures:        captures,
		CaptureSuccess:  captureOK,
		DetectOK:        detectOK,
		SyncFailures:    syncFail,
		Funnel:          funnel,
		D1Retention:     d1,
		D7Retention:     d7,
		ByExperiment:    byExp,
		ByRegion:        byReg,
		ByVersion:       byVer,
		EventCount:      count,
		MaxEventAgeSec:  age,
		ComputedAt:      s.now(),
		LatencyBoundSec: 60,
		DictionaryOwner: "product-analytics",
		Source:          "analytics_store",
		AsOf:            asOf,
	}
}

// Gold recompute helper for tests: list event names in window.
func (s *Store) EventNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.byID))
	for id, e := range s.byID {
		if !e.Deleted {
			out = append(out, id+":"+e.Name)
		}
	}
	sort.Strings(out)
	return out
}

func dayIndex(ts, asOf time.Time) int {
	// days before asOf day (0 = today)
	a := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	t := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
	return int(a.Sub(t).Hours() / 24)
}

func retention(active map[string]map[int]struct{}, first map[string]time.Time, asOf time.Time) (d1, d7 float64) {
	// D1: among owners whose first day was yesterday relative to asOf, fraction active on day 0
	// Simplified: cohort first-seen on dayOffset N, return rate active on day 0.
	var c1, r1, c7, r7 int
	for owner, firstTS := range first {
		off := dayIndex(firstTS, asOf)
		days := active[owner]
		if off == 1 {
			c1++
			if _, ok := days[0]; ok {
				r1++
			}
		}
		if off == 7 {
			c7++
			if _, ok := days[0]; ok {
				r7++
			}
		}
	}
	if c1 > 0 {
		d1 = float64(r1) / float64(c1)
	}
	if c7 > 0 {
		d7 = float64(r7) / float64(c7)
	}
	return
}

func propOutcome(propsJSON string) string {
	if propsJSON == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(propsJSON), &m); err != nil {
		return ""
	}
	if v, ok := m["outcome"].(string); ok {
		return strings.ToLower(v)
	}
	return ""
}

// MetricDictionary documents metric definitions for owners/stop rules.
type MetricDef struct {
	Name         string `json:"name"`
	Definition   string `json:"definition"`
	Owner        string `json:"owner"`
	StopRule     string `json:"stop_rule,omitempty"`
	LatencySec   int    `json:"latency_bound_sec"`
	Recomputable bool   `json:"recomputable_from_events"`
}

// Dictionary returns the metric dictionary.
func Dictionary() []MetricDef {
	return []MetricDef{
		{Name: "dau", Definition: "Distinct owner_key with ≥1 event in last 24h", Owner: "product-analytics", LatencySec: 60, Recomputable: true},
		{Name: "captures", Definition: "capture_attempt + collection_complete in window", Owner: "product-analytics", LatencySec: 60, Recomputable: true},
		{Name: "capture_success", Definition: "successful captures (outcome=success or collection_complete)", Owner: "product-analytics", LatencySec: 60, Recomputable: true},
		{Name: "detect_ok", Definition: "detect_result with outcome=success", Owner: "product-analytics", LatencySec: 60, Recomputable: true},
		{Name: "sync_failures", Definition: "count of sync_fail events", Owner: "product-analytics", LatencySec: 60, Recomputable: true},
		{Name: "d1_retention", Definition: "share of owners first seen D-1 active on D0", Owner: "product-analytics", StopRule: "pause experiment if D1 drop > 20% vs control for 3d", LatencySec: 3600, Recomputable: true},
		{Name: "d7_retention", Definition: "share of owners first seen D-7 active on D0", Owner: "product-analytics", StopRule: "halt rollout if D7 drop > 15% vs control", LatencySec: 3600, Recomputable: true},
	}
}

// ErrNotFound for query APIs.
var ErrNotFound = errors.New("not found")
