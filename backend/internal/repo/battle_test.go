package repo_test

import (
	"testing"

	"animalpoke/backend/internal/battle"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupBattleDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:battle_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.BattleSession{},
		&models.WalletBalance{},
		&models.WalletLedgerEntry{},
	))
	return db
}

func testTeam() []battle.Fighter {
	return []battle.Fighter{
		{
			ID: "p1", Name: "Dog", Species: "dog", Role: battle.RoleTank, Slot: battle.SlotFront,
			Element: battle.ElementWater, MaxHP: 130, HP: 130, ATK: 30, DEF: 34, SPD: 24,
			SkillIDs: []string{"shell_guard", "taunt", "heal_lick", "energy_burst"},
		},
		{
			ID: "p2", Name: "Cat", Species: "cat", Role: battle.RoleDPS, Slot: battle.SlotBack,
			Element: battle.ElementFire, MaxHP: 90, HP: 90, ATK: 44, DEF: 16, SPD: 42,
			SkillIDs: []string{"claw_strike", "fire_pounce", "energy_burst"},
		},
	}
}

func TestBattleRepo_StartSettleReplayAndTamper(t *testing.T) {
	db := setupBattleDB(t)
	r := repo.NewBattleRepo(db)

	start, err := r.Start(repo.StartRequest{
		OwnerKey: "acc:tester", ArchetypeID: "bruiser", Team: testTeam(),
	})
	require.NoError(t, err)
	require.NotNil(t, start.Session)
	assert.Equal(t, "open", start.Session.Status)
	assert.NotEmpty(t, start.Session.Seed)
	assert.NotEmpty(t, start.Threats)
	assert.Equal(t, battle.RuleVersion, start.Session.RuleVersion)

	cmds := []battle.Command{
		{ActorID: "p2", Kind: battle.CmdSkill, SkillID: "fire_pounce"},
		{ActorID: "p1", Kind: battle.CmdSkill, SkillID: "shell_guard"},
		{ActorID: "p2", Kind: battle.CmdBasic},
	}

	sess1, res1, err := r.Settle(repo.SettleRequest{
		OwnerKey: "acc:tester", SessionID: start.Session.SessionID, Commands: cmds,
	})
	require.NoError(t, err)
	require.NotNil(t, res1)
	assert.Equal(t, "completed", sess1.Status)
	assert.Equal(t, res1.CommandHash, sess1.CommandHash)
	assert.Equal(t, res1.WinnerSide, sess1.WinnerSide)

	// idempotent re-settle
	sess2, res2, err := r.Settle(repo.SettleRequest{
		OwnerKey: "acc:tester", SessionID: start.Session.SessionID, Commands: cmds,
	})
	require.NoError(t, err)
	assert.Equal(t, sess1.WinnerSide, sess2.WinnerSide)
	assert.Equal(t, res1.CommandHash, res2.CommandHash)

	// A different owner cannot settle a session.
	start3, err := r.Start(repo.StartRequest{
		OwnerKey: "acc:tester", ArchetypeID: "swarmer", Team: testTeam(),
	})
	require.NoError(t, err)
	_, _, err = r.Settle(repo.SettleRequest{
		OwnerKey: "acc:other", SessionID: start3.Session.SessionID, Commands: nil,
	})
	require.ErrorIs(t, err, repo.ErrBattleNotOwner)

	// owner-key wrong claim explicitly
	start4, err := r.Start(repo.StartRequest{
		OwnerKey: "acc:tester", ArchetypeID: "bruiser", Team: testTeam(),
	})
	require.NoError(t, err)
	// claim "draw" always if not draw — or claim nonsense side
	_, _, err = r.Settle(repo.SettleRequest{
		OwnerKey: "acc:tester", SessionID: start4.Session.SessionID, Commands: nil, ClaimedWinner: "cheater",
	})
	require.ErrorIs(t, err, repo.ErrBattleTamper)
}

func TestBattleRepo_UnknownArchetype(t *testing.T) {
	db := setupBattleDB(t)
	r := repo.NewBattleRepo(db)
	_, err := r.Start(repo.StartRequest{OwnerKey: "acc:x", ArchetypeID: "nope", Team: testTeam()})
	require.ErrorIs(t, err, repo.ErrBattleUnknownArch)
}

func TestPvP_FullLogAuthoritative(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:pvp_full_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PvPMatch{}, &models.PvPRating{}, &models.PvPQueue{}))
	r := repo.NewPvPRepo(db)

	_, q, err := r.EnqueueOrMatch("acc:a")
	require.NoError(t, err)
	assert.True(t, q)
	m, q2, err := r.EnqueueOrMatch("acc:b")
	require.NoError(t, err)
	assert.False(t, q2)
	require.NotNil(t, m)

	players := []battle.Fighter{{
		ID: "a1", Name: "A", Species: "dog", Role: battle.RoleTank, Slot: battle.SlotFront,
		Element: battle.ElementFire, MaxHP: 150, HP: 150, ATK: 50, DEF: 30, SPD: 40,
		SkillIDs: []string{"claw_strike", "energy_burst"}, Side: "player",
	}}
	enemies := []battle.Fighter{{
		ID: "b1", Name: "B", Species: "cat", Role: battle.RoleDPS, Slot: battle.SlotBack,
		Element: battle.ElementGrass, MaxHP: 60, HP: 60, ATK: 20, DEF: 10, SPD: 10,
		SkillIDs: []string{"claw_strike"}, Side: "enemy",
	}}
	// simulate to know true winner
	trueRes, err := battle.Simulate(m.Seed, players, enemies, nil)
	require.NoError(t, err)

	// wrong claimed winner with full log → reject
	wrong := m.PlayerA
	if trueRes.WinnerSide == "player" {
		wrong = m.PlayerB
	}
	// if draw, skip mismatch case
	if trueRes.WinnerSide != "draw" {
		_, err = r.SubmitResult("acc:a", m.MatchID, wrong, map[string]any{
			"seed": m.Seed, "players": players, "enemies": enemies, "commands": []battle.Command{},
		})
		require.ErrorIs(t, err, repo.ErrPvPInvalidLog)
	}

	// correct settlement
	// need fresh match if previous failed mid-way — status still matched
	derived := m.PlayerA
	if trueRes.WinnerSide == "enemy" {
		derived = m.PlayerB
	}
	if trueRes.WinnerSide == "draw" {
		t.Skip("draw path not supported in PvP ELO")
	}
	out, err := r.SubmitResult("acc:a", m.MatchID, derived, map[string]any{
		"seed": m.Seed, "players": players, "enemies": enemies, "commands": []battle.Command{},
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", out.Status)
	assert.Equal(t, derived, out.Winner)
}
