package cases

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateGetTransitionAnd404(t *testing.T) {
	s := NewService()
	c, err := s.Create(ResourceModeration, "rep-1", "user-1", "a@example.com", 24)
	require.NoError(t, err)
	assert.Equal(t, StateOpen, c.State)

	got, err := s.Get(c.ID, "admin-1", RoleAdmin)
	require.NoError(t, err)
	assert.Equal(t, "a@example.com", got.ReporterEmail)

	// support sees masked email
	sup, err := s.Get(c.ID, "sup-1", RoleSupport)
	require.NoError(t, err)
	assert.Equal(t, "a***@example.com", sup.ReporterEmail)
	assert.Empty(t, sup.InternalNotes) // none yet

	_, err = s.Get("case-missing", "admin", RoleAdmin)
	require.Error(t, err)
	assert.True(t, IsNotFound(err))

	// illegal jump open -> resolved
	_, err = s.Transition(c.ID, "admin", StateResolved, "done")
	require.Error(t, err)
	assert.True(t, IsConflict(err))

	_, err = s.Transition(c.ID, "admin", StateTriaged, "")
	require.NoError(t, err)
	_, err = s.Transition(c.ID, "admin", StateInProgress, "")
	require.NoError(t, err)
	_, err = s.Transition(c.ID, "admin", StateResolved, "fixed")
	require.NoError(t, err)

	userView, err := s.UserStatus(c.ID, "user-1")
	require.NoError(t, err)
	assert.Equal(t, "resolved", userView.UserVisible)
	assert.Nil(t, userView.InternalNotes)
	assert.Equal(t, "reviewed", userView.Reason)

	audits := s.Audits(c.ID)
	require.NotEmpty(t, audits)
	actions := map[string]int{}
	for _, a := range audits {
		actions[a.Action]++
	}
	assert.GreaterOrEqual(t, actions["view"], 1)
	assert.GreaterOrEqual(t, actions["transition"], 1)
}

func TestConcurrentClaim(t *testing.T) {
	s := NewService()
	c, err := s.Create(ResourceOrder, "ord-9", "user-2", "b@x.com", 1)
	require.NoError(t, err)

	var wg sync.WaitGroup
	var wins int
	var mu sync.Mutex
	for _, agent := range []string{"a1", "a2", "a3", "a4", "a5"} {
		wg.Add(1)
		go func(agent string) {
			defer wg.Done()
			_, err := s.Assign(c.ID, agent, agent)
			if err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}(agent)
	}
	wg.Wait()
	assert.Equal(t, 1, wins)
	got, err := s.Get(c.ID, "admin", RoleAdmin)
	require.NoError(t, err)
	assert.NotEmpty(t, got.Assignee)
}

func TestSLABreach(t *testing.T) {
	s := NewService()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return now })
	c, err := s.Create(ResourcePrivacy, "pr-1", "u", "c@x.com", 1)
	require.NoError(t, err)
	assert.False(t, c.SLABreached)
	now = now.Add(2 * time.Hour)
	got, err := s.Get(c.ID, "admin", RoleAdmin)
	require.NoError(t, err)
	assert.True(t, got.SLABreached)
}

func TestAddNoteImmutableAudit(t *testing.T) {
	s := NewService()
	c, err := s.Create(ResourceSecurity, "sec-1", "u", "d@x.com", 24)
	require.NoError(t, err)
	_, err = s.AddNote(c.ID, "admin", "looks phishing")
	require.NoError(t, err)
	got, err := s.Get(c.ID, "admin", RoleAdmin)
	require.NoError(t, err)
	require.Len(t, got.InternalNotes, 1)
	userView, err := s.UserStatus(c.ID, "u")
	require.NoError(t, err)
	assert.Nil(t, userView.InternalNotes)
}
