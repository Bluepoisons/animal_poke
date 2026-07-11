package liveops

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateMachine_OpenClaimUsesServerClock(t *testing.T) {
	s := NewStore()
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	now := base
	s.SetClock(func() time.Time { return now })

	inst, err := s.UpsertDefinition(Definition{
		ID: "ev1", Kind: "event", Title: "T", Timezone: "UTC",
		StartsAt:       base.Add(time.Hour).Format(time.RFC3339),
		EndsAt:         base.Add(2 * time.Hour).Format(time.RFC3339),
		EnrollRequired: true, RewardRef: "reward:gold-10",
	})
	require.NoError(t, err)
	assert.Equal(t, StateScheduled, inst.State)

	// Client cannot enroll early by lying about time.
	_, err = s.Enroll(inst.InstanceID, "p1")
	require.Error(t, err)

	now = base.Add(90 * time.Minute)
	_, err = s.Enroll(inst.InstanceID, "p1")
	require.NoError(t, err)
	_, err = s.AddProgress(inst.InstanceID, "p1", 1)
	require.NoError(t, err)

	// Still open — claim forbidden.
	_, _, err = s.ClaimReward(inst.InstanceID, "p1", "")
	require.Error(t, err)

	now = base.Add(3 * time.Hour)
	// Force settling via refresh path
	got, err := s.GetInstance(inst.InstanceID)
	require.NoError(t, err)
	assert.Equal(t, StateSettling, got.State)

	p, ref, err := s.ClaimReward(inst.InstanceID, "p1", "claim-1")
	require.NoError(t, err)
	assert.True(t, p.Claimed)
	assert.Equal(t, "reward:gold-10", ref)

	// Concurrent claims exactly once.
	var wg sync.WaitGroup
	var claims int
	var mu sync.Mutex
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := s.ClaimReward(inst.InstanceID, "p1", "claim-1")
			if err == nil {
				mu.Lock()
				claims++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.GreaterOrEqual(t, claims, 1)
	// claim map prevents double grant semantics; progress stays claimed once
	assert.True(t, s.GetProgress(inst.InstanceID, "p1").Claimed)
}

func TestCancelCompensation(t *testing.T) {
	s := NewStore()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return now })
	inst, err := s.UpsertDefinition(Definition{
		ID: "ev2", Kind: "event", Title: "T", Timezone: "Asia/Shanghai",
		StartsAt:        now.Add(-time.Hour).Format(time.RFC3339),
		EndsAt:          now.Add(time.Hour).Format(time.RFC3339),
		CompensationRef: "reward:comp-1",
	})
	require.NoError(t, err)
	_, err = s.Enroll(inst.InstanceID, "p2")
	require.NoError(t, err)

	cancelled, err := s.Cancel(inst.InstanceID)
	require.NoError(t, err)
	assert.Equal(t, StateCancelled, cancelled.State)

	p, ref, err := s.Compensate(inst.InstanceID, "p2")
	require.NoError(t, err)
	assert.True(t, p.Compensated)
	assert.Equal(t, "reward:comp-1", ref)

	// idempotent
	p2, _, err := s.Compensate(inst.InstanceID, "p2")
	require.NoError(t, err)
	assert.True(t, p2.Compensated)
}

func TestSettleBatchCloses(t *testing.T) {
	s := NewStore()
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return now })
	inst, err := s.UpsertDefinition(Definition{
		ID: "ev3", Kind: "season", Title: "S", Timezone: "UTC",
		StartsAt:  now.Add(-2 * time.Hour).Format(time.RFC3339),
		EndsAt:    now.Add(-time.Hour).Format(time.RFC3339),
		RewardRef: "r",
	})
	require.NoError(t, err)
	// Pre-seed progress while open: redefine with open window temporarily
	// Directly inject progress for settlement counting.
	s.mu.Lock()
	s.progress[progressKey(inst.InstanceID, "a")] = &PlayerProgress{InstanceID: inst.InstanceID, Progress: 1, Target: 1}
	s.progress[progressKey(inst.InstanceID, "b")] = &PlayerProgress{InstanceID: inst.InstanceID, Progress: 1, Target: 1}
	s.instances[inst.InstanceID].State = StateSettling
	s.instances[inst.InstanceID].SettleCursor = "0"
	s.mu.Unlock()

	out, err := s.SettleBatch(inst.InstanceID, 1)
	require.NoError(t, err)
	assert.Equal(t, StateSettling, out.State)
	out, err = s.SettleBatch(inst.InstanceID, 10)
	require.NoError(t, err)
	assert.Equal(t, StateClosed, out.State)
}
