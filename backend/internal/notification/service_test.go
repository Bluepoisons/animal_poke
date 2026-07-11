package notification

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:notif_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.InboxMessage{}, &models.NotificationOutbox{},
		&models.PushDeviceToken{}, &models.NotificationPreference{},
	))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// SQLite memory DBs are single-writer; serialize connections for concurrent tests.
	sqlDB.SetMaxOpenConns(1)
	return db
}

func TestEnqueue_DedupeExactlyOnce(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, &MockProvider{})
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.Enqueue("dev:1", models.NotifCategorySecurity, "t", "b", "dedupe-1", true)
		}()
	}
	wg.Wait()
	var count int64
	require.NoError(t, db.Model(&models.InboxMessage{}).Where("dedupe_key = ?", "dedupe-1").Count(&count).Error)
	assert.Equal(t, int64(1), count)
	require.NoError(t, db.Model(&models.NotificationOutbox{}).Where("dedupe_key = ?", "outbox:dedupe-1").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestProcessOutbox_RetryOn429ThenDeliver(t *testing.T) {
	db := testDB(t)
	prov := &MockProvider{FailTimes: 1}
	svc := NewService(db, prov)
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })

	require.NoError(t, svc.UpsertPushToken("dev:1", "tok-a", "ios"))
	msg, err := svc.Enqueue("dev:1", models.NotifCategoryRefund, "refund", "ok", "ref-1", true)
	require.NoError(t, err)
	require.NotZero(t, msg.ID)

	n, err := svc.ProcessOutbox(10)
	require.NoError(t, err)
	assert.Equal(t, 0, n) // failed

	var out models.NotificationOutbox
	require.NoError(t, db.Where("inbox_id = ?", msg.ID).First(&out).Error)
	assert.Equal(t, models.OutboxFailed, out.State)

	// advance past backoff
	now = now.Add(5 * time.Second)
	n, err = svc.ProcessOutbox(10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.NoError(t, db.Where("inbox_id = ?", msg.ID).First(&out).Error)
	assert.Equal(t, models.OutboxDelivered, out.State)
	assert.Len(t, prov.Sends, 1)
}

func TestMarketingOptOutStopsImmediately(t *testing.T) {
	db := testDB(t)
	prov := &MockProvider{}
	svc := NewService(db, prov)
	require.NoError(t, svc.UpsertPushToken("dev:2", "tok-b", "android"))
	consent := false
	_, err := svc.UpdatePrefs("dev:2", &consent, nil, nil, nil, nil)
	require.NoError(t, err)

	_, err = svc.Enqueue("dev:2", models.NotifCategoryMarketing, "sale", "buy", "mkt-1", false)
	require.NoError(t, err)
	n, err := svc.ProcessOutbox(10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Empty(t, prov.Sends)
	var out models.NotificationOutbox
	require.NoError(t, db.Where("dedupe_key = ?", "outbox:mkt-1").First(&out).Error)
	assert.Equal(t, models.OutboxSkipped, out.State)
}

func TestQuietHoursDefersMarketing(t *testing.T) {
	db := testDB(t)
	prov := &MockProvider{}
	svc := NewService(db, prov)
	// 23:00 Shanghai is quiet
	now := time.Date(2026, 7, 11, 15, 0, 0, 0, time.UTC) // 23:00 CST
	svc.SetClock(func() time.Time { return now })
	consent := true
	_, err := svc.UpdatePrefs("dev:3", &consent, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, svc.UpsertPushToken("dev:3", "tok-c", "web"))
	_, err = svc.Enqueue("dev:3", models.NotifCategoryMarketing, "m", "b", "mkt-2", false)
	require.NoError(t, err)
	_, err = svc.ProcessOutbox(10)
	require.NoError(t, err)
	assert.Empty(t, prov.Sends)
	var out models.NotificationOutbox
	require.NoError(t, db.Where("dedupe_key = ?", "outbox:mkt-2").First(&out).Error)
	assert.Equal(t, models.OutboxPending, out.State)
	assert.True(t, out.NextAttemptAt.After(now))
}

func TestSecurityBypassesQuietHours(t *testing.T) {
	db := testDB(t)
	prov := &MockProvider{}
	svc := NewService(db, prov)
	now := time.Date(2026, 7, 11, 15, 0, 0, 0, time.UTC) // quiet for marketing
	svc.SetClock(func() time.Time { return now })
	require.NoError(t, svc.UpsertPushToken("dev:4", "tok-d", "ios"))
	_, err := svc.Enqueue("dev:4", models.NotifCategorySecurity, "login", "new device", "sec-1", true)
	require.NoError(t, err)
	n, err := svc.ProcessOutbox(10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Len(t, prov.Sends, 1)
}

func TestInboxReadAckAndCursor(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, &MockProvider{})
	for i := 1; i <= 3; i++ {
		_, err := svc.Enqueue("dev:5", models.NotifCategoryEvent, "t", "b", fmt.Sprintf("e-%d", i), false)
		require.NoError(t, err)
	}
	rows, err := svc.ListInbox("dev:5", 0, 2)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.NoError(t, svc.MarkRead("dev:5", rows[0].ID))
	require.NoError(t, svc.Ack("dev:5", rows[0].ID))
	more, err := svc.ListInbox("dev:5", rows[1].ID, 10)
	require.NoError(t, err)
	require.Len(t, more, 1)
}
