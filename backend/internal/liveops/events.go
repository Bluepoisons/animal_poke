// Package liveops implements AP-081 server-authoritative season/event instances.
package liveops

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Instance states.
const (
	StateScheduled = "scheduled"
	StateOpen      = "open"
	StateSettling  = "settling"
	StateClosed    = "closed"
	StateCancelled = "cancelled"
)

// Definition is static authored event content (not time-shifted by client).
type Definition struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"` // season | event
	Title           string `json:"title"`
	Timezone        string `json:"timezone"`  // IANA, e.g. Asia/Shanghai
	StartsAt        string `json:"starts_at"` // RFC3339
	EndsAt          string `json:"ends_at"`
	EnrollRequired  bool   `json:"enroll_required"`
	RewardRef       string `json:"reward_ref"`
	CompensationRef string `json:"compensation_ref,omitempty"`
	MinClient       string `json:"min_client_version,omitempty"`
}

// Instance is a runtime event with server-driven state.
type Instance struct {
	Definition
	InstanceID   string `json:"instance_id"`
	State        string `json:"state"`
	SettleCursor string `json:"settle_cursor,omitempty"`
	UpdatedAt    string `json:"updated_at"`
}

// PlayerProgress tracks enroll/progress/claim for one player on one instance.
type PlayerProgress struct {
	InstanceID  string `json:"instance_id"`
	Enrolled    bool   `json:"enrolled"`
	Progress    int    `json:"progress"`
	Target      int    `json:"target"`
	Claimed     bool   `json:"claimed"`
	Compensated bool   `json:"compensated"`
	UpdatedAt   string `json:"updated_at"`
}

// Clock abstracts time for tests.
type Clock func() time.Time

// Store is an in-memory authoritative event store.
type Store struct {
	mu        sync.Mutex
	clock     Clock
	defs      map[string]Definition
	instances map[string]*Instance
	progress  map[string]*PlayerProgress // key: instanceID|player
	claims    map[string]struct{}        // claim keys for idempotency
}

// NewStore creates an empty store with real clock.
func NewStore() *Store {
	return &Store{
		clock:     func() time.Time { return time.Now().UTC() },
		defs:      map[string]Definition{},
		instances: map[string]*Instance{},
		progress:  map[string]*PlayerProgress{},
		claims:    map[string]struct{}{},
	}
}

// SetClock overrides the clock (tests).
func (s *Store) SetClock(c Clock) { s.clock = c }

// UpsertDefinition registers/updates a definition and ensures an instance exists.
func (s *Store) UpsertDefinition(def Definition) (*Instance, error) {
	if strings.TrimSpace(def.ID) == "" {
		return nil, errors.New("definition id required")
	}
	if def.Timezone == "" {
		def.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(def.Timezone); err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}
	if _, err := time.Parse(time.RFC3339, def.StartsAt); err != nil {
		return nil, fmt.Errorf("starts_at: %w", err)
	}
	if _, err := time.Parse(time.RFC3339, def.EndsAt); err != nil {
		return nil, fmt.Errorf("ends_at: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defs[def.ID] = def
	instID := "inst-" + def.ID
	inst, ok := s.instances[instID]
	if !ok {
		inst = &Instance{Definition: def, InstanceID: instID, State: StateScheduled}
		s.instances[instID] = inst
	} else {
		inst.Definition = def
	}
	s.refreshLocked(inst)
	return cloneInstance(inst), nil
}

// Tick refreshes all instance states from the server clock.
func (s *Store) Tick() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, inst := range s.instances {
		s.refreshLocked(inst)
	}
}

func (s *Store) refreshLocked(inst *Instance) {
	if inst.State == StateCancelled || inst.State == StateClosed {
		return
	}
	now := s.clock()
	start, _ := time.Parse(time.RFC3339, inst.StartsAt)
	end, _ := time.Parse(time.RFC3339, inst.EndsAt)
	// Interpret bounds in event timezone for display correctness; compare in UTC instants.
	loc, err := time.LoadLocation(inst.Timezone)
	if err == nil {
		// Re-parse as wall times in zone if inputs lacked offset? RFC3339 already absolute.
		_ = loc
	}
	switch {
	case now.Before(start):
		inst.State = StateScheduled
	case !now.Before(start) && now.Before(end):
		if inst.State == StateScheduled || inst.State == StateOpen {
			inst.State = StateOpen
		}
	case !now.Before(end) && inst.State == StateOpen:
		inst.State = StateSettling
		inst.SettleCursor = "0"
	case !now.Before(end) && inst.State == StateSettling:
		// remain settling until SettleBatch closes
	}
	inst.UpdatedAt = now.UTC().Format(time.RFC3339)
}

// ListInstances returns all instances after tick.
func (s *Store) ListInstances() []Instance {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Instance, 0, len(s.instances))
	for _, inst := range s.instances {
		s.refreshLocked(inst)
		out = append(out, *cloneInstance(inst))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstanceID < out[j].InstanceID })
	return out
}

// GetInstance returns one instance.
func (s *Store) GetInstance(id string) (*Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, ok := s.instances[id]
	if !ok {
		return nil, errors.New("instance not found")
	}
	s.refreshLocked(inst)
	return cloneInstance(inst), nil
}

// Cancel cancels an instance and marks it for compensation.
func (s *Store) Cancel(instanceID string) (*Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, ok := s.instances[instanceID]
	if !ok {
		return nil, errors.New("instance not found")
	}
	if inst.State == StateClosed {
		return nil, errors.New("cannot cancel closed instance")
	}
	inst.State = StateCancelled
	inst.UpdatedAt = s.clock().UTC().Format(time.RFC3339)
	return cloneInstance(inst), nil
}

// Enroll registers a player if open (server time).
func (s *Store) Enroll(instanceID, playerID string) (*PlayerProgress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, ok := s.instances[instanceID]
	if !ok {
		return nil, errors.New("instance not found")
	}
	s.refreshLocked(inst)
	if inst.State != StateOpen {
		return nil, fmt.Errorf("enroll not allowed in state %s", inst.State)
	}
	key := progressKey(instanceID, playerID)
	p, ok := s.progress[key]
	if !ok {
		p = &PlayerProgress{InstanceID: instanceID, Target: 1}
		s.progress[key] = p
	}
	p.Enrolled = true
	p.UpdatedAt = s.clock().UTC().Format(time.RFC3339)
	return cloneProgress(p), nil
}

// AddProgress increments progress while open; ignores client-provided timestamps.
func (s *Store) AddProgress(instanceID, playerID string, delta int) (*PlayerProgress, error) {
	if delta <= 0 {
		return nil, errors.New("delta must be positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, ok := s.instances[instanceID]
	if !ok {
		return nil, errors.New("instance not found")
	}
	s.refreshLocked(inst)
	if inst.State != StateOpen {
		return nil, fmt.Errorf("progress not allowed in state %s", inst.State)
	}
	key := progressKey(instanceID, playerID)
	p, ok := s.progress[key]
	if !ok {
		p = &PlayerProgress{InstanceID: instanceID, Target: 1}
		s.progress[key] = p
	}
	if inst.EnrollRequired && !p.Enrolled {
		return nil, errors.New("not enrolled")
	}
	p.Progress += delta
	if p.Progress > p.Target {
		p.Progress = p.Target
	}
	p.UpdatedAt = s.clock().UTC().Format(time.RFC3339)
	return cloneProgress(p), nil
}

// ClaimReward grants reward once after end (settling/closed), idempotent on claimKey.
func (s *Store) ClaimReward(instanceID, playerID, claimKey string) (*PlayerProgress, string, error) {
	if claimKey == "" {
		claimKey = instanceID + "|" + playerID + "|reward"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, ok := s.instances[instanceID]
	if !ok {
		return nil, "", errors.New("instance not found")
	}
	s.refreshLocked(inst)
	// Server clock: cannot claim while still open (client clock skew irrelevant).
	if inst.State != StateSettling && inst.State != StateClosed {
		return nil, "", fmt.Errorf("claim not allowed in state %s", inst.State)
	}
	key := progressKey(instanceID, playerID)
	p, ok := s.progress[key]
	if !ok || p.Progress < p.Target {
		return nil, "", errors.New("reward not eligible")
	}
	if _, dup := s.claims[claimKey]; dup || p.Claimed {
		// Exactly-once: return prior claim without double grant.
		p.Claimed = true
		return cloneProgress(p), inst.RewardRef, nil
	}
	s.claims[claimKey] = struct{}{}
	p.Claimed = true
	p.UpdatedAt = s.clock().UTC().Format(time.RFC3339)
	return cloneProgress(p), inst.RewardRef, nil
}

// Compensate grants cancellation compensation once.
func (s *Store) Compensate(instanceID, playerID string) (*PlayerProgress, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, ok := s.instances[instanceID]
	if !ok {
		return nil, "", errors.New("instance not found")
	}
	if inst.State != StateCancelled {
		return nil, "", errors.New("compensation only for cancelled instances")
	}
	key := progressKey(instanceID, playerID)
	p, ok := s.progress[key]
	if !ok {
		p = &PlayerProgress{InstanceID: instanceID, Target: 1, Enrolled: true}
		s.progress[key] = p
	}
	if p.Compensated {
		return cloneProgress(p), inst.CompensationRef, nil
	}
	p.Compensated = true
	p.UpdatedAt = s.clock().UTC().Format(time.RFC3339)
	return cloneProgress(p), inst.CompensationRef, nil
}

// SettleBatch advances settling cursor and closes when done.
func (s *Store) SettleBatch(instanceID string, batchSize int) (*Instance, error) {
	if batchSize <= 0 {
		batchSize = 100
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, ok := s.instances[instanceID]
	if !ok {
		return nil, errors.New("instance not found")
	}
	s.refreshLocked(inst)
	if inst.State != StateSettling {
		return nil, fmt.Errorf("settle not allowed in state %s", inst.State)
	}
	// Simulate sharded settlement by counting players and advancing cursor.
	n := 0
	for k := range s.progress {
		if strings.HasPrefix(k, instanceID+"|") {
			n++
		}
	}
	cur := 0
	fmt.Sscanf(inst.SettleCursor, "%d", &cur)
	cur += batchSize
	inst.SettleCursor = fmt.Sprintf("%d", cur)
	if cur >= n {
		inst.State = StateClosed
	}
	inst.UpdatedAt = s.clock().UTC().Format(time.RFC3339)
	return cloneInstance(inst), nil
}

// GetProgress returns player progress.
func (s *Store) GetProgress(instanceID, playerID string) *PlayerProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.progress[progressKey(instanceID, playerID)]
	if p == nil {
		return &PlayerProgress{InstanceID: instanceID, Target: 1}
	}
	return cloneProgress(p)
}

func progressKey(instanceID, playerID string) string {
	return instanceID + "|" + playerID
}

func cloneInstance(i *Instance) *Instance {
	cp := *i
	return &cp
}

func cloneProgress(p *PlayerProgress) *PlayerProgress {
	cp := *p
	return &cp
}

var (
	defaultStore *Store
	defaultOnce  sync.Once
)

// Default process-wide store.
func Default() *Store {
	defaultOnce.Do(func() {
		defaultStore = NewStore()
		// Seed a demo season open window around "now" for empty envs.
		now := time.Now().UTC()
		_, _ = defaultStore.UpsertDefinition(Definition{
			ID: "season-demo", Kind: "season", Title: "Demo Season",
			Timezone:       "Asia/Shanghai",
			StartsAt:       now.Add(-time.Hour).Format(time.RFC3339),
			EndsAt:         now.Add(24 * time.Hour).Format(time.RFC3339),
			EnrollRequired: true, RewardRef: "reward:season-demo", CompensationRef: "reward:season-demo-comp",
		})
	})
	return defaultStore
}
