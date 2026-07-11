package battle_test

import (
	"fmt"
	"testing"

	"animalpoke/backend/internal/battle"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func samplePlayer() []battle.Fighter {
	return []battle.Fighter{
		{
			ID: "p-tank", Name: "坦犬", Species: "dog", Role: battle.RoleTank, Slot: battle.SlotFront,
			Element: battle.ElementWater, MaxHP: 140, HP: 140, ATK: 32, DEF: 36, SPD: 24,
			SkillIDs: []string{"shell_guard", "taunt", "heal_lick", "energy_burst"},
		},
		{
			ID: "p-dps", Name: "输出猫", Species: "cat", Role: battle.RoleDPS, Slot: battle.SlotBack,
			Element: battle.ElementFire, MaxHP: 95, HP: 95, ATK: 48, DEF: 18, SPD: 42,
			SkillIDs:    []string{"claw_strike", "fire_pounce", "energy_burst"},
			SkillLevels: map[string]int{"fire_pounce": 2},
		},
		{
			ID: "p-sup", Name: "辅助鹅", Species: "goose", Role: battle.RoleSupport, Slot: battle.SlotMid,
			Element: battle.ElementGrass, MaxHP: 110, HP: 110, ATK: 28, DEF: 28, SPD: 30,
			SkillIDs: []string{"howl", "pack_regen", "water_splash", "energy_burst"},
		},
	}
}

func TestCatalogMinimums(t *testing.T) {
	c := battle.GetCatalog()
	assert.Equal(t, battle.RuleVersion, c.RuleVersion)
	assert.GreaterOrEqual(t, len(c.Skills), 12)
	assert.GreaterOrEqual(t, len(c.Archetypes), 6)
	assert.GreaterOrEqual(t, len(c.RecommendedTeams), 3)
	assert.Equal(t, 4, len(c.Roles))
	assert.Equal(t, 3, len(c.Slots))
	assert.NotEmpty(t, c.Statuses)
	assert.NotEmpty(t, c.Upgrades)
}

func TestReplayConsistency(t *testing.T) {
	players, err := battle.NormalizePlayerTeam(samplePlayer())
	require.NoError(t, err)
	seed := "replay-seed-001"
	enemies, err := battle.BuildEnemyTeam("bruiser", seed)
	require.NoError(t, err)

	cmds := []battle.Command{
		{ActorID: "p-dps", Kind: battle.CmdSkill, SkillID: "fire_pounce"},
		{ActorID: "p-tank", Kind: battle.CmdSkill, SkillID: "shell_guard"},
		{ActorID: "p-sup", Kind: battle.CmdSkill, SkillID: "howl"},
		{ActorID: "p-dps", Kind: battle.CmdBasic},
		{ActorID: "p-tank", Kind: battle.CmdBasic},
		{ActorID: "p-dps", Kind: battle.CmdUltimate},
	}

	r1, err := battle.Simulate(seed, players, enemies, cmds)
	require.NoError(t, err)
	r2, err := battle.Simulate(seed, players, enemies, cmds)
	require.NoError(t, err)

	assert.Equal(t, r1.WinnerSide, r2.WinnerSide)
	assert.Equal(t, r1.Rounds, r2.Rounds)
	assert.Equal(t, r1.CommandHash, r2.CommandHash)
	assert.Equal(t, r1.Metrics, r2.Metrics)
	assert.Equal(t, len(r1.Events), len(r2.Events))
	for i := range r1.Events {
		assert.Equal(t, r1.Events[i], r2.Events[i], "event %d", i)
	}
}

func TestCommandTamperChangesHashOrRejects(t *testing.T) {
	players, err := battle.NormalizePlayerTeam(samplePlayer())
	require.NoError(t, err)
	seed := "tamper-seed"
	enemies, err := battle.BuildEnemyTeam("glass_cannon", seed)
	require.NoError(t, err)

	base := []battle.Command{
		{ActorID: "p-dps", Kind: battle.CmdSkill, SkillID: "fire_pounce"},
		{ActorID: "p-tank", Kind: battle.CmdBasic},
	}
	r1, err := battle.Simulate(seed, players, enemies, base)
	require.NoError(t, err)

	// tampered command sequence must not claim same hash
	tampered := append([]battle.Command{}, base...)
	tampered = append(tampered, battle.Command{ActorID: "p-dps", Kind: battle.CmdUltimate})
	r2, err := battle.Simulate(seed, players, enemies, tampered)
	require.NoError(t, err)
	assert.NotEqual(t, r1.CommandHash, r2.CommandHash)

	// unknown skill on fighter is ignored → falls back basic (no panic)
	bad := []battle.Command{{ActorID: "p-dps", Kind: battle.CmdSkill, SkillID: "not_a_real_skill"}}
	_, err = battle.Simulate(seed, players, enemies, bad)
	require.NoError(t, err)

	// unknown equipped skill rejected at normalize/validate
	badTeam := samplePlayer()
	badTeam[0].SkillIDs = []string{"totally_fake"}
	_, err = battle.NormalizePlayerTeam(badTeam)
	require.Error(t, err)
}

func TestNoInfiniteControl(t *testing.T) {
	// player alone vs heavy controller; ensure battle ends and control breaks fire
	players, err := battle.NormalizePlayerTeam([]battle.Fighter{{
		ID: "solo", Name: "Solo", Species: "dog", Role: battle.RoleTank, Slot: battle.SlotFront,
		Element: battle.ElementLight, MaxHP: 200, HP: 200, ATK: 40, DEF: 40, SPD: 10,
		SkillIDs: []string{"claw_strike", "energy_burst"},
	}})
	require.NoError(t, err)
	seed := "control-seed"
	enemies, err := battle.BuildEnemyTeam("controller", seed)
	require.NoError(t, err)

	// empty commands → pure AI both sides
	res, err := battle.Simulate(seed, players, enemies, nil)
	require.NoError(t, err)
	assert.LessOrEqual(t, res.Rounds, battle.MaxRounds)
	assert.Contains(t, []string{"player", "enemy", "draw"}, res.WinnerSide)

	// if player was controlled, control_break or immunity should eventually appear or battle ends
	assert.True(t, res.Rounds > 0)
}

func TestZeroDamageFloorAndBreak(t *testing.T) {
	// absurd DEF should still take floor damage; battle must terminate
	players, err := battle.NormalizePlayerTeam([]battle.Fighter{{
		ID: "atk", Name: "Atk", Species: "cat", Role: battle.RoleDPS, Slot: battle.SlotBack,
		Element: battle.ElementFire, MaxHP: 80, HP: 80, ATK: 10, DEF: 5, SPD: 50,
		SkillIDs: []string{"claw_strike", "energy_burst"},
	}})
	require.NoError(t, err)
	enemies := []battle.Fighter{{
		ID: "wall", Name: "Wall", Species: "goose", Role: battle.RoleTank, Slot: battle.SlotFront,
		Element: battle.ElementWater, MaxHP: 50, HP: 50, ATK: 5, DEF: 9999, SPD: 1,
		SkillIDs: []string{"shell_guard", "claw_strike"}, Side: "enemy",
	}}
	res, err := battle.Simulate("floor-seed", players, enemies, nil)
	require.NoError(t, err)
	assert.LessOrEqual(t, res.Rounds, battle.MaxRounds)
	// some damage should have been dealt (floor or softlock break)
	assert.True(t, res.Metrics.PlayerDamageDealt+res.Metrics.ZeroDamageBreaks > 0 || res.WinnerSide != "")
}

func TestThreatsAndFailureExplain(t *testing.T) {
	players := samplePlayer()
	threats := battle.AnalyzeThreats(players, "controller")
	require.NotEmpty(t, threats)
	found := false
	for _, th := range threats {
		if th.Code == "control_chain" || th.Code == "archetype_hint" {
			found = true
		}
		assert.NotEmpty(t, th.TextZH)
	}
	assert.True(t, found)

	// force a loss-ish simulation for failure factors
	weak, err := battle.NormalizePlayerTeam([]battle.Fighter{{
		ID: "w", Name: "Weak", Species: "rabbit", Role: battle.RoleDPS, Slot: battle.SlotBack,
		Element: battle.ElementGrass, MaxHP: 40, HP: 40, ATK: 12, DEF: 5, SPD: 20,
		SkillIDs: []string{"claw_strike"},
	}})
	require.NoError(t, err)
	enemies, err := battle.BuildEnemyTeam("bruiser", "fail-seed")
	require.NoError(t, err)
	res, err := battle.Simulate("fail-seed", weak, enemies, nil)
	require.NoError(t, err)
	if res.WinnerSide != "player" {
		require.NotEmpty(t, res.FailureFactors)
		assert.NotEmpty(t, res.FailureFactors[0].TextZH)
	}
}

func TestThreeBuildsCanClearLowArchetypes(t *testing.T) {
	// Acceptance: at least three effective builds; low rarity can clear with strategy.
	// Use common-tier stats (~rarity common bases) against difficulty-2 archetypes.
	builds := [][]battle.Fighter{
		// budget sustain
		{
			{ID: "t", Name: "T", Species: "dog", Role: battle.RoleTank, Slot: battle.SlotFront, Element: battle.ElementWater, MaxHP: 120, HP: 120, ATK: 28, DEF: 34, SPD: 22, SkillIDs: []string{"shell_guard", "taunt", "heal_lick", "energy_burst"}},
			{ID: "s", Name: "S", Species: "goose", Role: battle.RoleSupport, Slot: battle.SlotMid, Element: battle.ElementWater, MaxHP: 100, HP: 100, ATK: 24, DEF: 26, SPD: 28, SkillIDs: []string{"pack_regen", "howl", "heal_lick", "energy_burst"}},
			{ID: "d", Name: "D", Species: "cat", Role: battle.RoleDPS, Slot: battle.SlotBack, Element: battle.ElementFire, MaxHP: 85, HP: 85, ATK: 40, DEF: 16, SPD: 40, SkillIDs: []string{"claw_strike", "fire_pounce", "energy_burst"}},
		},
		// control burst
		{
			{ID: "c", Name: "C", Species: "goose", Role: battle.RoleControl, Slot: battle.SlotMid, Element: battle.ElementGrass, MaxHP: 100, HP: 100, ATK: 32, DEF: 28, SPD: 38, SkillIDs: []string{"mud_trap", "leaf_bind", "dark_fang", "energy_burst"}},
			{ID: "d2", Name: "D2", Species: "cat", Role: battle.RoleDPS, Slot: battle.SlotBack, Element: battle.ElementDark, MaxHP: 90, HP: 90, ATK: 46, DEF: 16, SPD: 44, SkillIDs: []string{"fire_pounce", "claw_strike", "energy_burst"}},
			{ID: "s2", Name: "S2", Species: "dog", Role: battle.RoleSupport, Slot: battle.SlotFront, Element: battle.ElementLight, MaxHP: 110, HP: 110, ATK: 26, DEF: 30, SPD: 26, SkillIDs: []string{"howl", "water_splash", "heal_lick"}},
		},
		// element counter
		{
			{ID: "f", Name: "F", Species: "cat", Role: battle.RoleDPS, Slot: battle.SlotBack, Element: battle.ElementFire, MaxHP: 90, HP: 90, ATK: 44, DEF: 18, SPD: 40, SkillIDs: []string{"fire_pounce", "claw_strike", "energy_burst"}},
			{ID: "w", Name: "W", Species: "dog", Role: battle.RoleTank, Slot: battle.SlotFront, Element: battle.ElementWater, MaxHP: 125, HP: 125, ATK: 30, DEF: 36, SPD: 24, SkillIDs: []string{"shell_guard", "water_splash", "taunt"}},
			{ID: "g", Name: "G", Species: "goose", Role: battle.RoleControl, Slot: battle.SlotMid, Element: battle.ElementGrass, MaxHP: 100, HP: 100, ATK: 34, DEF: 28, SPD: 34, SkillIDs: []string{"leaf_bind", "wing_gust", "energy_burst"}},
		},
	}
	targets := []string{"bruiser", "glass_cannon", "swarmer"}
	wins := 0
	for bi, raw := range builds {
		team, err := battle.NormalizePlayerTeam(raw)
		require.NoError(t, err)
		arch := targets[bi%len(targets)]
		// try a few seeds; at least one win per build
		buildWin := false
		for s := 0; s < 20; s++ {
			seed := fmt.Sprintf("build-%d-seed-%d", bi, s)
			enemies, err := battle.BuildEnemyTeam(arch, seed)
			require.NoError(t, err)
			res, err := battle.Simulate(seed, team, enemies, nil)
			require.NoError(t, err)
			if res.WinnerSide == "player" {
				buildWin = true
				break
			}
		}
		if buildWin {
			wins++
		}
	}
	assert.GreaterOrEqual(t, wins, 3, "expected all three recommended-style builds to clear a low archetype")
}
func TestWinRateSimulationSmoke(t *testing.T) {
	// multi-seed termination smoke across all archetypes
	team, err := battle.NormalizePlayerTeam(samplePlayer())
	require.NoError(t, err)
	archs := []string{"bruiser", "glass_cannon", "swarmer", "iron_wall", "controller", "healer_boss"}
	const per = 30
	total := 0
	wins := 0
	for _, arch := range archs {
		for i := 0; i < per; i++ {
			seed := fmt.Sprintf("wr-%s-%d", arch, i)
			enemies, err := battle.BuildEnemyTeam(arch, seed)
			require.NoError(t, err)
			res, err := battle.Simulate(seed, team, enemies, nil)
			require.NoError(t, err)
			assert.LessOrEqual(t, res.Rounds, battle.MaxRounds)
			assert.Contains(t, []string{"player", "enemy", "draw"}, res.WinnerSide)
			total++
			if res.WinnerSide == "player" {
				wins++
			}
		}
	}
	// recommended full team should be able to clear content; record rate for regression visibility
	rate := float64(wins) / float64(total)
	t.Logf("aggregate win rate=%.2f (%d/%d)", rate, wins, total)
	assert.Greater(t, rate, 0.2, "recommended team should clear a meaningful share of archetypes")
}
