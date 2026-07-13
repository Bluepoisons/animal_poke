package repo

import (
	"net/http"
	"testing"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestIdempotencyClaimReturnsPersistedLease(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:idem_repo_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.IdempotencyRecord{}))

	store := NewIdempotencyRepo(db)
	claim, created, err := store.BeginOrGet("dev-1", "adventure.generate", "key-1", "hash-1", time.Hour)
	require.NoError(t, err)
	require.True(t, created)
	var persisted models.IdempotencyRecord
	require.NoError(t, db.First(&persisted, claim.ID).Error)
	assert.Equal(t, persisted.UpdatedAt, claim.UpdatedAt)

	completed, err := store.CompleteClaim(claim, http.StatusGatewayTimeout, `{"error":"timeout"}`, true)
	require.NoError(t, err)
	assert.True(t, completed)
}
