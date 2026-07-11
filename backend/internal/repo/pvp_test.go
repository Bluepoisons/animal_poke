package repo_test

import (
	"testing"

	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPvPDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:pvp_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PvPMatch{}, &models.PvPRating{}, &models.PvPQueue{}))
	return db
}

func TestPvP_MatchAndSettleOnce(t *testing.T) {
	db := setupPvPDB(t)
	r := repo.NewPvPRepo(db)
	m1, q1, err := r.EnqueueOrMatch("acc:a")
	require.NoError(t, err)
	assert.True(t, q1)
	assert.Nil(t, m1)
	m2, q2, err := r.EnqueueOrMatch("acc:b")
	require.NoError(t, err)
	assert.False(t, q2)
	require.NotNil(t, m2)
	assert.Equal(t, "matched", m2.Status)

	// settle
	out, err := r.SubmitResult("acc:a", m2.MatchID, "acc:a", map[string]any{"seed": m2.Seed, "cmds": []string{"atk"}})
	require.NoError(t, err)
	assert.Equal(t, "completed", out.Status)
	assert.Equal(t, "acc:a", out.Winner)

	// duplicate settle idempotent
	out2, err := r.SubmitResult("acc:b", m2.MatchID, "acc:b", map[string]any{"seed": m2.Seed})
	require.NoError(t, err)
	assert.Equal(t, "acc:a", out2.Winner) // still original winner

	// tampered seed rejected on unsettled - create new match
	_, _, _ = r.EnqueueOrMatch("acc:c")
	m3, _, err := r.EnqueueOrMatch("acc:d")
	require.NoError(t, err)
	require.NotNil(t, m3)
	_, err = r.SubmitResult("acc:c", m3.MatchID, "acc:c", map[string]any{"seed": "bad"})
	require.ErrorIs(t, err, repo.ErrPvPInvalidLog)

	ra, err := r.GetRating("acc:a")
	require.NoError(t, err)
	assert.Greater(t, ra.Rating, 1000)
}
