package repo

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"animalpoke/backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupWalletDB(t *testing.T) *WalletRepo {
	t.Helper()
	// 文件库 + busy_timeout，便于并发扣款测试
	dsn := fmt.Sprintf("file:%s/wallet.db?_busy_timeout=5000&_journal_mode=WAL&cache=shared", t.TempDir())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(10)
	require.NoError(t, db.AutoMigrate(
		&models.WalletBalance{},
		&models.WalletLedgerEntry{},
		&models.InventoryItem{},
	))
	return NewWalletRepo(db)
}

func TestWallet_CreditIdempotent(t *testing.T) {
	r := setupWalletDB(t)
	req := ApplyRequest{
		DeviceID: "dev-1", Currency: models.CurrencyGold, Delta: 100,
		OperationID: "op-credit-1", SourceType: "checkin", SourceID: "day1",
		Kind: models.LedgerKindCurrency,
	}
	res1, err := r.Apply(req)
	require.NoError(t, err)
	require.False(t, res1.Idempotent)
	assert.Equal(t, int64(100), res1.Balance)

	res2, err := r.Apply(req)
	require.NoError(t, err)
	require.True(t, res2.Idempotent)
	assert.Equal(t, int64(100), res2.Balance)
	assert.Equal(t, res1.Entry.EntryID, res2.Entry.EntryID)

	// 余额仍为 100
	bals, err := r.GetBalances("", "dev-1")
	require.NoError(t, err)
	require.Len(t, bals, 1)
	assert.Equal(t, int64(100), bals[0].Balance)
}

func TestWallet_NoNegativeBalance(t *testing.T) {
	r := setupWalletDB(t)
	_, err := r.Apply(ApplyRequest{
		DeviceID: "dev-1", Currency: models.CurrencyGold, Delta: 30,
		OperationID: "op-c1", Kind: models.LedgerKindCurrency, SourceType: "reward",
	})
	require.NoError(t, err)

	_, err = r.Apply(ApplyRequest{
		DeviceID: "dev-1", Currency: models.CurrencyGold, Delta: -50,
		OperationID: "op-d1", Kind: models.LedgerKindCurrency, SourceType: "shop",
	})
	require.ErrorIs(t, err, ErrInsufficientBalance)

	bals, err := r.GetBalances("", "dev-1")
	require.NoError(t, err)
	assert.Equal(t, int64(30), bals[0].Balance)
}

func TestWallet_ConcurrentDebitSerial(t *testing.T) {
	r := setupWalletDB(t)
	_, err := r.Apply(ApplyRequest{
		DeviceID: "dev-conc", Currency: models.CurrencyGold, Delta: 100,
		OperationID: "seed", Kind: models.LedgerKindCurrency, SourceType: "admin",
	})
	require.NoError(t, err)

	const n = 50
	var wg sync.WaitGroup
	var success atomic.Int64
	var insufficient atomic.Int64
	wg.Add(n)
	for i := range n {
		i := i
		go func() {
			defer wg.Done()
			_, err := r.Apply(ApplyRequest{
				DeviceID: "dev-conc", Currency: models.CurrencyGold, Delta: -3,
				OperationID: fmt.Sprintf("debit-%d", i),
				Kind:        models.LedgerKindCurrency,
				SourceType:  "shop",
			})
			if err == nil {
				success.Add(1)
			} else if errors.Is(err, ErrInsufficientBalance) {
				insufficient.Add(1)
			} else {
				t.Errorf("unexpected err: %v", err)
			}
		}()
	}
	wg.Wait()

	// 100/3 = 33 次成功，余额 100-3*33=1
	assert.Equal(t, int64(33), success.Load())
	assert.Equal(t, int64(17), insufficient.Load())

	bals, err := r.GetBalances("", "dev-conc")
	require.NoError(t, err)
	require.Len(t, bals, 1)
	assert.Equal(t, int64(1), bals[0].Balance)
	assert.GreaterOrEqual(t, bals[0].Balance, int64(0))
}

func TestWallet_RebuildBalance(t *testing.T) {
	r := setupWalletDB(t)
	_, err := r.Apply(ApplyRequest{
		DeviceID: "dev-rb", Currency: models.CurrencyGold, Delta: 200,
		OperationID: "rb-1", Kind: models.LedgerKindCurrency, SourceType: "reward",
	})
	require.NoError(t, err)
	_, err = r.Apply(ApplyRequest{
		DeviceID: "dev-rb", Currency: models.CurrencyGold, Delta: -70,
		OperationID: "rb-2", Kind: models.LedgerKindCurrency, SourceType: "shop",
	})
	require.NoError(t, err)

	// 人为破坏快照
	require.NoError(t, r.db.Model(&models.WalletBalance{}).
		Where("owner_key = ?", OwnerKey("", "dev-rb")).
		Update("balance", 9999).Error)

	sum, err := r.RebuildBalance("", "dev-rb", models.CurrencyGold)
	require.NoError(t, err)
	assert.Equal(t, int64(130), sum)

	bals, err := r.GetBalances("", "dev-rb")
	require.NoError(t, err)
	assert.Equal(t, int64(130), bals[0].Balance)
}

func TestWallet_InventoryGrantConsume(t *testing.T) {
	r := setupWalletDB(t)
	res, err := r.Apply(ApplyRequest{
		DeviceID: "dev-inv", Kind: models.LedgerKindItem, Currency: "toy-ball",
		Delta: 2, OperationID: "inv-g1", SourceType: "checkin",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), res.Balance)

	// 幂等
	res2, err := r.Apply(ApplyRequest{
		DeviceID: "dev-inv", Kind: models.LedgerKindItem, Currency: "toy-ball",
		Delta: 2, OperationID: "inv-g1", SourceType: "checkin",
	})
	require.NoError(t, err)
	assert.True(t, res2.Idempotent)

	_, err = r.Apply(ApplyRequest{
		DeviceID: "dev-inv", Kind: models.LedgerKindItem, Currency: "toy-ball",
		Delta: -1, OperationID: "inv-c1", SourceType: "capture",
	})
	require.NoError(t, err)

	items, err := r.GetInventory("", "dev-inv")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(1), items[0].Quantity)

	_, err = r.Apply(ApplyRequest{
		DeviceID: "dev-inv", Kind: models.LedgerKindItem, Currency: "toy-ball",
		Delta: -5, OperationID: "inv-c2", SourceType: "capture",
	})
	require.ErrorIs(t, err, ErrInsufficientItem)
}

func TestWallet_AccountOwnerShared(t *testing.T) {
	r := setupWalletDB(t)
	_, err := r.Apply(ApplyRequest{
		DeviceID: "dev-a", AccountID: "acc-1", Currency: models.CurrencyGold,
		Delta: 50, OperationID: "acc-c1", Kind: models.LedgerKindCurrency, SourceType: "reward",
	})
	require.NoError(t, err)

	// 同账号另一设备可见余额
	bals, err := r.GetBalances("acc-1", "dev-b")
	require.NoError(t, err)
	require.Len(t, bals, 1)
	assert.Equal(t, int64(50), bals[0].Balance)

	// 从 dev-b 扣款
	res, err := r.Apply(ApplyRequest{
		DeviceID: "dev-b", AccountID: "acc-1", Currency: models.CurrencyGold,
		Delta: -20, OperationID: "acc-d1", Kind: models.LedgerKindCurrency, SourceType: "shop",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(30), res.Balance)
}

func TestWallet_ListLedger(t *testing.T) {
	r := setupWalletDB(t)
	for i := range 5 {
		_, err := r.Apply(ApplyRequest{
			DeviceID: "dev-led", Currency: models.CurrencyGold, Delta: int64(i + 1),
			OperationID: fmt.Sprintf("led-%d", i), Kind: models.LedgerKindCurrency, SourceType: "reward",
		})
		require.NoError(t, err)
	}
	rows, err := r.ListLedger("", "dev-led", models.CurrencyGold, 0, 3)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	// 降序
	assert.Greater(t, rows[0].ID, rows[1].ID)
}
