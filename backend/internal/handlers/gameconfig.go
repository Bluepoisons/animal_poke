package handlers

import (
	"net/http"
	"sync"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
)

// Shared in-process store (single-replica MVP; multi-replica should use Redis/DB later).
var (
	gameConfigOnce  sync.Once
	gameConfigStore *services.GameConfigStore
)

func getGameConfigStore() *services.GameConfigStore {
	gameConfigOnce.Do(func() {
		// Empty path: memory only (rollback within process). Ops can still PUT defaults.
		gameConfigStore = services.NewGameConfigStore("")
	})
	return gameConfigStore
}

// GameConfigGet GET /api/v1/config/game — public (auth) read of versioned game config.
func (h *ProductHandler) GameConfigGet(c *gin.Context) {
	cfg := getGameConfigStore().Get()
	c.JSON(http.StatusOK, gin.H{
		"version":    cfg.Version,
		"economy":    cfg.Economy,
		"features":   cfg.Features,
		"meta":       cfg.Meta,
		"request_id": middleware.GetRequestID(c),
	})
}

// GameConfigPut PUT /api/v1/ops/game-config — ops-only update with hard bounds.
func (h *ProductHandler) GameConfigPut(c *gin.Context) {
	if !h.flags.Ops {
		featureUnavailable(c, "ops")
		return
	}
	if !h.opsAuthorized(c) {
		middleware.WriteError(c, http.StatusForbidden, "ops_forbidden", "ops access denied", false, nil)
		return
	}
	var body services.GameConfig
	if err := middleware.BindStrictJSON(c, &body); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if err := getGameConfigStore().Put(body); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "config_invalid", err.Error(), false, nil)
		return
	}
	cfg := getGameConfigStore().Get()
	c.JSON(http.StatusOK, gin.H{
		"version":    cfg.Version,
		"economy":    cfg.Economy,
		"features":   cfg.Features,
		"request_id": middleware.GetRequestID(c),
	})
}

// GameConfigRollback POST /api/v1/ops/game-config/rollback
func (h *ProductHandler) GameConfigRollback(c *gin.Context) {
	if !h.flags.Ops {
		featureUnavailable(c, "ops")
		return
	}
	if !h.opsAuthorized(c) {
		middleware.WriteError(c, http.StatusForbidden, "ops_forbidden", "ops access denied", false, nil)
		return
	}
	cfg, err := getGameConfigStore().Rollback()
	if err != nil {
		middleware.WriteError(c, http.StatusConflict, "no_previous_config", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"version":    cfg.Version,
		"economy":    cfg.Economy,
		"features":   cfg.Features,
		"request_id": middleware.GetRequestID(c),
	})
}
