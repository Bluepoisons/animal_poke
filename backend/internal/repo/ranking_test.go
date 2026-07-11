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

func setupRankDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:rank_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RankingDailyScore{}, &models.RankingSnapshot{}, &models.RankingRewardGrant{},
		&models.WalletBalance{}, &models.WalletLedgerEntry{}, &models.InventoryItem{}))
	return db
}

func TestRanking_OrderAndSnapshotImmutable(t *testing.T) {
	db := setupRankDB(t)
	r := repo.NewRankingRepo(db)
	require.NoError(t, r.AddScore("2026-07-11", "Shanghai", repo.RankingOwner{Type: "account", ID: "a1"}, 10, "A1", true))
	require.NoError(t, r.AddScore("2026-07-11", "Shanghai", repo.RankingOwner{Type: "account", ID: "a2"}, 30, "A2", true))
	require.NoError(t, r.AddScore("2026-07-11", "Shanghai", repo.RankingOwner{Type: "account", ID: "a3"}, 20, "A3", false))
	rows, total, err := r.ListBoard("2026-07-11", "Shanghai", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, "a2", rows[0].OwnerID)
	rank, score, err := r.MyRank("2026-07-11", "Shanghai", repo.RankingOwner{Type: "account", ID: "a1"})
	require.NoError(t, err)
	assert.Equal(t, 2, rank)
	assert.Equal(t, int64(10), score)

	s1, err := r.SettleCity("2026-07-11", "Shanghai")
	require.NoError(t, err)
	s2, err := r.SettleCity("2026-07-11", "Shanghai")
	require.NoError(t, err)
	assert.Equal(t, s1.SnapshotID, s2.SnapshotID)

	w := repo.NewWalletRepo(db)
	require.NoError(t, r.GrantTopRewards(s1, w, 3, func(rank int) int64 {
		if rank == 1 {
			return 100
		}
		return 10
	}))
	require.NoError(t, r.GrantTopRewards(s1, w, 3, func(rank int) int64 {
		if rank == 1 {
			return 100
		}
		return 10
	}))
	bals, err := w.GetBalances("a2", "rank-system")
	require.NoError(t, err)
	require.NotEmpty(t, bals)
	assert.Equal(t, int64(100), bals[0].Balance)
}
