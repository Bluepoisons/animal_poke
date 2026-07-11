package research

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamLifecycleTwoUsersClaimOnce(t *testing.T) {
	s := NewService()
	team, err := s.Create("u1", "Alice", "河岸研究", false)
	require.NoError(t, err)
	_, err = s.Join(team.Invite, "u2", "Bob")
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err = s.Contribute(team.ID, "u1", "滨水区", "灯光记录")
		require.NoError(t, err)
		_, err = s.Contribute(team.ID, "u2", "滨水区", "声景记录")
		require.NoError(t, err)
	}
	// daily cap
	_, err = s.Contribute(team.ID, "u1", "滨水区", "extra")
	assert.ErrorIs(t, err, ErrCap)

	require.NoError(t, s.ClaimReward(team.ID, "u1"))
	require.NoError(t, s.ClaimReward(team.ID, "u2"))
	assert.ErrorIs(t, s.ClaimReward(team.ID, "u1"), ErrDuplicate)
}

func TestBlockAndNoPreciseLocation(t *testing.T) {
	s := NewService()
	team, err := s.Create("u1", "A", "t", false)
	require.NoError(t, err)
	_, err = s.Join(team.Invite, "u2", "B")
	require.NoError(t, err)
	require.NoError(t, s.Block(team.ID, "u1", "u2"))
	_, err = s.Contribute(team.ID, "u2", "x", "y")
	assert.ErrorIs(t, err, ErrForbidden)

	_, err = s.Contribute(team.ID, "u1", "31.2,121.4", "note")
	require.NoError(t, err)
	got, err := s.Get(team.ID, "u1")
	require.NoError(t, err)
	require.NotEmpty(t, got.Board)
	assert.Equal(t, "redacted", got.Board[len(got.Board)-1].Region)
}

func TestSoloEquivalentPath(t *testing.T) {
	s := NewService()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return now })
	team, err := s.Create("solo", "Me", "solo", true)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, err = s.SoloContribute(team.ID, "solo")
		require.NoError(t, err)
	}
	require.NoError(t, s.ClaimReward(team.ID, "solo"))
}

func TestNickSanitize(t *testing.T) {
	s := NewService()
	team, err := s.Create("u1", "admin_hack", "t", false)
	require.NoError(t, err)
	assert.Equal(t, "researcher", team.Members["u1"].Nickname)
}
