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

func setupNarr(t *testing.T) *repo.NarrativeRepo {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:frag_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.NarrativeNode{}, &models.NarrativeChoice{}, &models.NarrativeProgress{},
		&models.NarrativeSeenState{}, &models.NarrativeChoiceLog{},
		&models.StoryFragment{}, &models.StoryFragmentUnlock{}, &models.ClueState{},
	))
	r := repo.NewNarrativeRepo(db)
	require.NoError(t, r.SeedContent())
	return r
}

func TestFragments_FirstAndMilestoneDistinct(t *testing.T) {
	r := setupNarr(t)
	u1, err := r.TryUnlockFragments("dev:d1", "d1", "", "op-a", map[string]any{
		"species": "cat", "is_first_species": true, "observation_count": 1,
	})
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, f := range u1 {
		ids[f.FragmentID] = true
	}
	assert.True(t, ids["frag_first_any"] || ids["frag_first_cat"])

	// idempotent same operation
	u3, err := r.TryUnlockFragments("dev:d1", "d1", "", "op-a", map[string]any{
		"species": "cat", "is_first_species": true, "observation_count": 1,
	})
	require.NoError(t, err)
	assert.Empty(t, u3)

	// later milestone
	u2, err := r.TryUnlockFragments("dev:d1", "d1", "", "op-b", map[string]any{
		"species": "dog", "observation_count": 6, "species_seen": map[string]bool{"cat": true, "dog": true},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, u2)
}
