package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultGameConfig_Valid(t *testing.T) {
	cfg := DefaultGameConfig()
	require.NoError(t, ValidateGameConfig(cfg))
	assert.Equal(t, 20.0, cfg.Economy["captureStaminaCost"])
	assert.Equal(t, 50.0, cfg.Economy["toyBallPrice"])
}

func TestValidateGameConfig_RejectsIllegal(t *testing.T) {
	cfg := DefaultGameConfig()
	cfg.Economy["captureStaminaCost"] = 0
	assert.Error(t, ValidateGameConfig(cfg))
	cfg = DefaultGameConfig()
	cfg.Economy["toyBallPrice"] = 999999
	assert.Error(t, ValidateGameConfig(cfg))
}

func TestGameConfigStore_PutAndRollback(t *testing.T) {
	s := NewGameConfigStore("")
	next := DefaultGameConfig()
	next.Version = "game-config.v1.1"
	next.Economy["captureStaminaCost"] = 25
	require.NoError(t, s.Put(next))
	assert.Equal(t, 25.0, s.Get().Economy["captureStaminaCost"])
	rolled, err := s.Rollback()
	require.NoError(t, err)
	assert.Equal(t, 20.0, rolled.Economy["captureStaminaCost"])
}
