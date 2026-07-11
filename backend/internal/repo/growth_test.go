package repo

import (
	"fmt"
	"testing"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupGrowthRepo(t *testing.T) (*GrowthRepo, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s/growth.db?_busy_timeout=5000&_journal_mode=WAL&cache=shared", t.TempDir())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Animal{},
		&models.ResearcherTrack{},
		&models.GrowthEvent{},
		&models.CompanionProfile{},
		&models.CompanionMemoryNode{},
		&models.GrowthResetAudit{},
	))
	return NewGrowthRepo(db), db
}

func seedAnimal(t *testing.T, db *gorm.DB, deviceID, accountID string) string {
	t.Helper()
	id := uuid.NewString()
	a := &models.Animal{
		UUID:      id,
		DeviceID:  deviceID,
		AccountID: accountID,
		Species:   "cat",
		Breed:     "tabby",
		Rarity:    2,
		HP:        100,
		ATK:       20,
		DEF:       15,
		SPD:       10,
		GeneratedAt: time.Now().UTC(),
		ServerVersion: 1,
	}
	require.NoError(t, db.Create(a).Error)
	return id
}

func TestGrowth_ResearcherTracksAndIdempotent(t *testing.T) {
	r, _ := setupGrowthRepo(t)
	res1, err := r.ApplyEvent(ApplyGrowthRequest{
		DeviceID: "dev-1", EventID: "e-photo-1", Kind: models.GrowthEventPhotoCapture,
	})
	require.NoError(t, err)
	require.False(t, res1.Idempotent)
	require.True(t, res1.CombatUnchanged)
	require.Len(t, res1.Researcher, 3)

	var photoXP int64
	for _, tr := range res1.Researcher {
		if tr.Track == models.GrowthTrackPhotography {
			photoXP = tr.XP
		}
	}
	assert.Equal(t, int64(5), photoXP)

	res2, err := r.ApplyEvent(ApplyGrowthRequest{
		DeviceID: "dev-1", EventID: "e-photo-1", Kind: models.GrowthEventPhotoCapture,
	})
	require.NoError(t, err)
	assert.True(t, res2.Idempotent)
	for _, tr := range res2.Researcher {
		if tr.Track == models.GrowthTrackPhotography {
			assert.Equal(t, photoXP, tr.XP)
		}
	}
}

func TestGrowth_CrossDeviceAccountShare(t *testing.T) {
	r, _ := setupGrowthRepo(t)
	_, err := r.ApplyEvent(ApplyGrowthRequest{
		DeviceID: "dev-a", AccountID: "acc-1", EventID: "e-eco-1", Kind: models.GrowthEventSpeciesFirst,
	})
	require.NoError(t, err)

	tracks, err := r.GetResearcher("acc-1", "dev-b")
	require.NoError(t, err)
	var ecoXP int64
	for _, tr := range tracks {
		if tr.Track == models.GrowthTrackEcology {
			ecoXP = tr.XP
		}
	}
	assert.Equal(t, int64(15), ecoXP, "bound account shares growth across devices")
}

func TestGrowth_CompanionThreeVisibleNodes(t *testing.T) {
	r, db := setupGrowthRepo(t)
	animalID := seedAnimal(t, db, "dev-1", "")
	// preserve combat stats
	var before models.Animal
	require.NoError(t, db.Where("uuid = ?", animalID).First(&before).Error)

	res, err := r.ApplyEvent(ApplyGrowthRequest{
		DeviceID: "dev-1", EventID: "e-c1", Kind: models.GrowthEventCompanionInteract, AnimalUUID: animalID,
	})
	require.NoError(t, err)
	require.NotNil(t, res.Companion)
	require.GreaterOrEqual(t, len(res.Nodes), 3)

	visible := 0
	for _, n := range res.Nodes {
		if n.Visible {
			visible++
		}
	}
	assert.GreaterOrEqual(t, visible, 3)
	// first_meeting should unlock at 0 XP; interact adds XP so at least first_meeting
	unlocked := 0
	for _, n := range res.Nodes {
		if n.Unlocked {
			unlocked++
		}
	}
	assert.GreaterOrEqual(t, unlocked, 1)

	var after models.Animal
	require.NoError(t, db.Where("uuid = ?", animalID).First(&after).Error)
	assert.Equal(t, before.HP, after.HP)
	assert.Equal(t, before.ATK, after.ATK)
	assert.Equal(t, before.DEF, after.DEF)
	assert.Equal(t, before.SPD, after.SPD)
}

func TestGrowth_ForbiddenPaidPowerAndDecay(t *testing.T) {
	r, _ := setupGrowthRepo(t)
	_, err := r.ApplyEvent(ApplyGrowthRequest{DeviceID: "dev-1", EventID: "bad1", Kind: "paid_power"})
	assert.ErrorIs(t, err, ErrGrowthPaidPower)
	_, err = r.ApplyEvent(ApplyGrowthRequest{DeviceID: "dev-1", EventID: "bad2", Kind: "decay"})
	assert.ErrorIs(t, err, ErrGrowthDecayForbidden)
	_, err = r.ApplyEvent(ApplyGrowthRequest{DeviceID: "dev-1", EventID: "bad3", Kind: "feed"})
	assert.ErrorIs(t, err, ErrGrowthDecayForbidden)
	_, err = r.ApplyEvent(ApplyGrowthRequest{DeviceID: "dev-1", EventID: "bad4", Kind: "unknown_kind"})
	assert.ErrorIs(t, err, ErrGrowthForbiddenKind)
}

func TestGrowth_UnlockMultipleNodes(t *testing.T) {
	r, db := setupGrowthRepo(t)
	animalID := seedAnimal(t, db, "dev-1", "")
	// pump bond XP via multiple interact events
	for i := range 10 {
		_, err := r.ApplyEvent(ApplyGrowthRequest{
			DeviceID:   "dev-1",
			EventID:    fmt.Sprintf("bond-%d", i),
			Kind:       models.GrowthEventCompanionInteract,
			AnimalUUID: animalID,
		})
		require.NoError(t, err)
	}
	comp, nodes, err := r.GetCompanion("", "dev-1", animalID)
	require.NoError(t, err)
	require.NotNil(t, comp)
	assert.GreaterOrEqual(t, comp.BondXP, int64(40))
	unlocked := 0
	for _, n := range nodes {
		if n.Unlocked {
			unlocked++
		}
		assert.True(t, n.Visible)
	}
	assert.GreaterOrEqual(t, unlocked, 3)
	assert.GreaterOrEqual(t, len(nodes), 3)
}

func TestGrowth_ResetAuditable(t *testing.T) {
	r, _ := setupGrowthRepo(t)
	_, err := r.ApplyEvent(ApplyGrowthRequest{
		DeviceID: "dev-1", EventID: "r1", Kind: models.GrowthEventSafeExplore,
	})
	require.NoError(t, err)
	audit, err := r.Reset(ResetRequest{
		DeviceID: "dev-1", Scope: ResetScopeResearcher, Reason: "config migration test",
	})
	require.NoError(t, err)
	require.NotNil(t, audit)
	assert.Equal(t, "researcher", audit.Scope)
	assert.NotEmpty(t, audit.SnapshotJSON)
	assert.Contains(t, audit.SnapshotJSON, models.GrowthTrackSafeObservation)

	tracks, err := r.GetResearcher("", "dev-1")
	require.NoError(t, err)
	for _, tr := range tracks {
		assert.Equal(t, int64(0), tr.XP)
	}
	// events remain for audit
	events, err := r.ListEvents("", "dev-1", 0, 10)
	require.NoError(t, err)
	assert.NotEmpty(t, events)
}

func TestGrowth_CatalogRules(t *testing.T) {
	cat := Catalog()
	assert.Equal(t, GrowthConfigVersion, cat.ConfigVersion)
	assert.GreaterOrEqual(t, cat.Rules.MinVisibleNodesPerCompanion, 3)
	assert.True(t, cat.Rules.NoDecay)
	assert.True(t, cat.Rules.NoPaidPower)
	assert.True(t, cat.Rules.NoRealWorldFeeding)
	assert.True(t, cat.Rules.CombatStatsUnchanged)
	assert.GreaterOrEqual(t, len(cat.CompanionNodes), 3)
	assert.Len(t, cat.Tracks, 3)
}

func TestLevelFromXP(t *testing.T) {
	assert.Equal(t, 0, ResearcherLevel(0))
	assert.Equal(t, 1, ResearcherLevel(20))
	assert.Equal(t, 2, ResearcherLevel(50))
	assert.Equal(t, 0, CompanionLevel(0))
	assert.Equal(t, 1, CompanionLevel(10))
}
