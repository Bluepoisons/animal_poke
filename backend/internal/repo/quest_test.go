package repo

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/questcatalog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupQuestRepo(t *testing.T) (*QuestRepo, *WalletRepo) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s/quest.db?_busy_timeout=5000&_journal_mode=WAL&cache=shared", t.TempDir())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(10)
	require.NoError(t, db.AutoMigrate(
		&models.WalletBalance{}, &models.WalletLedgerEntry{}, &models.InventoryItem{},
		&models.QuestDefinition{}, &models.QuestProgress{}, &models.QuestClaim{}, &models.QuestEventLog{},
	))
	w := NewWalletRepo(db)
	q := NewQuestRepo(db, w)
	require.NoError(t, q.SeedDefinitions())
	return q, w
}

func TestQuest_SeedMin24(t *testing.T) {
	q, _ := setupQuestRepo(t)
	defs, err := q.ListDefinitions(false)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(defs), 24)
	require.NoError(t, questcatalog.ValidateGraph())
}

func TestQuest_RejectPageOpenEvents(t *testing.T) {
	q, _ := setupQuestRepo(t)
	for _, et := range []string{"open_pokedex", "safe_explore", "page_view", "open_map"} {
		_, err := q.ApplyEvent(ApplyEventRequest{
			DeviceID: "dev-1", EventID: "e-" + et, EventType: et,
		})
		assert.ErrorIs(t, err, ErrQuestEventForbidden, et)
	}
}

func TestQuest_TrustedEventProgressAndClaimIdempotent(t *testing.T) {
	q, w := setupQuestRepo(t)
	// main_first_capture needs 1 capture
	res, err := q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-a", EventID: "cap-1", EventType: models.QuestEventCaptureSuccess,
	})
	require.NoError(t, err)
	assert.False(t, res.Idempotent)
	assert.Contains(t, res.Updated, "main_first_capture")

	// 幂等 event
	res2, err := q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-a", EventID: "cap-1", EventType: models.QuestEventCaptureSuccess,
	})
	require.NoError(t, err)
	assert.True(t, res2.Idempotent)

	// claim
	c1, err := q.Claim(ClaimRequest{DeviceID: "dev-a", QuestID: "main_first_capture"})
	require.NoError(t, err)
	assert.False(t, c1.Idempotent)
	assert.Equal(t, int64(30), c1.Gold)

	// 再次 claim 幂等，金币不双发
	c2, err := q.Claim(ClaimRequest{DeviceID: "dev-a", QuestID: "main_first_capture"})
	require.NoError(t, err)
	assert.True(t, c2.Idempotent)

	bals, err := w.GetBalances("", "dev-a")
	require.NoError(t, err)
	var gold int64
	for _, b := range bals {
		if b.Currency == models.CurrencyGold {
			gold = b.Balance
		}
	}
	assert.Equal(t, int64(30), gold)
}

func TestQuest_CompoundObjectives(t *testing.T) {
	q, _ := setupQuestRepo(t)
	// research_compound_cap_note: capture 2 + research_note 1
	_, err := q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-c", EventID: "c1", EventType: models.QuestEventCaptureSuccess,
	})
	require.NoError(t, err)
	_, err = q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-c", EventID: "c2", EventType: models.QuestEventCaptureSuccess,
	})
	require.NoError(t, err)
	// 尚未完成
	_, err = q.Claim(ClaimRequest{DeviceID: "dev-c", QuestID: "research_compound_cap_note"})
	assert.ErrorIs(t, err, ErrQuestNotClaimable)

	_, err = q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-c", EventID: "n1", EventType: models.QuestEventResearchNote,
	})
	require.NoError(t, err)
	c, err := q.Claim(ClaimRequest{DeviceID: "dev-c", QuestID: "research_compound_cap_note"})
	require.NoError(t, err)
	assert.Equal(t, int64(70), c.Gold)
}

func TestQuest_ConcurrentClaimOnce(t *testing.T) {
	q, w := setupQuestRepo(t)
	_, err := q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-conc", EventID: "chk-1", EventType: models.QuestEventSeasonCheckin,
	})
	require.NoError(t, err)

	const n = 20
	var wg sync.WaitGroup
	var created atomic.Int64
	var idempotent atomic.Int64
	wg.Add(n)
	for i := range n {
		go func() {
			defer wg.Done()
			res, err := q.Claim(ClaimRequest{DeviceID: "dev-conc", QuestID: "daily_season_checkin"})
			if err != nil {
				t.Errorf("claim err: %v", err)
				return
			}
			if res.Idempotent {
				idempotent.Add(1)
			} else {
				created.Add(1)
			}
		}()
		_ = i
	}
	wg.Wait()
	// 至多 1 次真正入账
	assert.Equal(t, int64(1), created.Load())
	assert.Equal(t, int64(n-1), idempotent.Load())

	bals, err := w.GetBalances("", "dev-conc")
	require.NoError(t, err)
	var gold int64
	for _, b := range bals {
		if b.Currency == models.CurrencyGold {
			gold = b.Balance
		}
	}
	assert.Equal(t, int64(8), gold) // daily_season_checkin reward
}

func TestQuest_DailyResetTimezone(t *testing.T) {
	q, _ := setupQuestRepo(t)
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	q.SetLocation(loc)

	// Day 1
	day1 := time.Date(2026, 3, 10, 10, 0, 0, 0, loc).UTC()
	q.SetNowFunc(func() time.Time { return day1 })
	_, err = q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-tz", EventID: "d1-chk", EventType: models.QuestEventSeasonCheckin,
	})
	require.NoError(t, err)
	_, err = q.Claim(ClaimRequest{DeviceID: "dev-tz", QuestID: "daily_season_checkin"})
	require.NoError(t, err)

	// Same calendar day later — period same, already claimed
	day1b := time.Date(2026, 3, 10, 22, 0, 0, 0, loc).UTC()
	q.SetNowFunc(func() time.Time { return day1b })
	_, err = q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-tz", EventID: "d1b-chk", EventType: models.QuestEventSeasonCheckin,
	})
	require.NoError(t, err)
	// progress for same period is claimed — claim again idempotent
	c, err := q.Claim(ClaimRequest{DeviceID: "dev-tz", QuestID: "daily_season_checkin"})
	require.NoError(t, err)
	assert.True(t, c.Idempotent)

	// Next day new period
	day2 := time.Date(2026, 3, 11, 1, 0, 0, 0, loc).UTC()
	q.SetNowFunc(func() time.Time { return day2 })
	_, err = q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-tz", EventID: "d2-chk", EventType: models.QuestEventSeasonCheckin,
	})
	require.NoError(t, err)
	c2, err := q.Claim(ClaimRequest{DeviceID: "dev-tz", QuestID: "daily_season_checkin"})
	require.NoError(t, err)
	assert.False(t, c2.Idempotent)
	assert.Equal(t, int64(8), c2.Gold)
}

func TestQuest_ExpireAndCompensate(t *testing.T) {
	q, w := setupQuestRepo(t)
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	q.SetNowFunc(func() time.Time { return now })

	// complete daily capture
	_, err := q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-exp", EventID: "cap-e1", EventType: models.QuestEventCaptureSuccess,
	})
	require.NoError(t, err)

	// 强制过期
	owner := OwnerKey("", "dev-exp")
	period := q.PeriodKey(models.QuestResetDaily, now)
	past := now.Add(-time.Hour)
	require.NoError(t, q.DB().Model(&models.QuestProgress{}).
		Where("owner_key = ? AND quest_id = ? AND period_key = ?", owner, "daily_capture", period).
		Update("expires_at", past).Error)

	// 普通 claim 应拒绝
	_, err = q.Claim(ClaimRequest{DeviceID: "dev-exp", QuestID: "daily_capture"})
	assert.ErrorIs(t, err, ErrQuestExpired)

	// 补偿
	q.SetNowFunc(func() time.Time { return now.Add(time.Minute) })
	comp, err := q.CompensateExpired("", "dev-exp")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, comp.Compensated, 1)

	bals, err := w.GetBalances("", "dev-exp")
	require.NoError(t, err)
	var gold int64
	for _, b := range bals {
		if b.Currency == models.CurrencyGold {
			gold = b.Balance
		}
	}
	// daily_capture gold=15 → compensate 7
	assert.Equal(t, int64(7), gold)

	// 再次补偿不双发
	comp2, err := q.CompensateExpired("", "dev-exp")
	require.NoError(t, err)
	assert.Equal(t, 0, comp2.Compensated)
}

func TestQuest_ConfigRollbackKeepsProgress(t *testing.T) {
	q, _ := setupQuestRepo(t)
	_, err := q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-cfg", EventID: "cap-cfg", EventType: models.QuestEventCaptureSuccess,
	})
	require.NoError(t, err)

	// 重新 seed 同版本应 no-op
	require.NoError(t, q.SeedDefinitions())
	views, err := q.ListForOwner("", "dev-cfg", false)
	require.NoError(t, err)
	found := false
	for _, v := range views {
		if v.Definition.QuestID == "main_first_capture" && v.Progress != nil {
			found = true
			assert.Equal(t, models.QuestStatusCompleted, v.Progress.Status)
		}
	}
	assert.True(t, found)
}

func TestQuest_FreeQuestsWhenStaminaZero(t *testing.T) {
	q, _ := setupQuestRepo(t)
	views, err := q.ListForOwner("", "dev-free", true)
	require.NoError(t, err)
	assert.NotEmpty(t, views)
	for _, v := range views {
		assert.True(t, v.Free)
	}
}

func TestQuest_Simulate30Days(t *testing.T) {
	q, _ := setupQuestRepo(t)
	stats, err := q.SimulateDays("dev-sim", 30)
	require.NoError(t, err)
	assert.Equal(t, 60, stats["events"])
	assert.Greater(t, stats["free_ok"], 0)
	assert.Greater(t, stats["claims"], 0)
}

func TestQuest_PrereqBlocksProgress(t *testing.T) {
	q, _ := setupQuestRepo(t)
	// main_three_captures 需要 main_first_capture 先完成
	// 直接 3 次 capture 但未 claim/complete first? first 也会被同一 capture 推进
	// 使用 species_new 推进 main_first_species（前置 main_first_capture）
	_, err := q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-pre", EventID: "sp-only", EventType: models.QuestEventSpeciesNew,
	})
	require.NoError(t, err)
	// main_first_species 前置未满足 → 不应完成
	_, err = q.Claim(ClaimRequest{DeviceID: "dev-pre", QuestID: "main_first_species"})
	assert.ErrorIs(t, err, ErrQuestNotClaimable)

	// 完成前置
	_, err = q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-pre", EventID: "cap-pre", EventType: models.QuestEventCaptureSuccess,
	})
	require.NoError(t, err)
	_, err = q.Claim(ClaimRequest{DeviceID: "dev-pre", QuestID: "main_first_capture"})
	require.NoError(t, err)

	// 再推 species
	_, err = q.ApplyEvent(ApplyEventRequest{
		DeviceID: "dev-pre", EventID: "sp-2", EventType: models.QuestEventSpeciesNew,
	})
	require.NoError(t, err)
	_, err = q.Claim(ClaimRequest{DeviceID: "dev-pre", QuestID: "main_first_species"})
	require.NoError(t, err)
}
