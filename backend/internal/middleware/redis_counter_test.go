//go:build integration

package middleware

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 可选 Redis 集成测试：REDIS_URL=redis://localhost:6379 go test -tags=integration ./internal/middleware/ -run Redis
func TestRedisSharedCounter_Integration(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set")
	}
	rc, err := NewRedisSharedCounter(url)
	require.NoError(t, err)
	defer rc.Close()

	ctx := context.Background()
	key := "test:ap023:incr:" + time.Now().Format("150405.000")

	n, err := rc.Incr(ctx, key, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	n, err = rc.Incr(ctx, key, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)

	tbKey := "test:ap023:tb:" + time.Now().Format("150405.000")
	ok, err := rc.AllowToken(ctx, tbKey, 100.0/60.0, 2)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = rc.AllowToken(ctx, tbKey, 100.0/60.0, 2)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = rc.AllowToken(ctx, tbKey, 100.0/60.0, 2)
	require.NoError(t, err)
	assert.False(t, ok)

	nonceKey := "test:ap023:nonce:" + time.Now().Format("150405.000")
	ok, err = rc.SetNX(ctx, nonceKey, 5*time.Second)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = rc.SetNX(ctx, nonceKey, 5*time.Second)
	require.NoError(t, err)
	assert.False(t, ok)
}
