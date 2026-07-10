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
		c.JSON(http.StatusForbidden, gin.H{
			"error":       "ops access denied",
			"reason_code": "ops_forbidden",
			"request_id":  middleware.GetRequestID(c),
		})
		return
	}
	var body services.GameConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "invalid body",
			"reason_code": "invalid_request",
			"request_id":  middleware.GetRequestID(c),
		})
		return
	}
	if err := getGameConfigStore().Put(body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       err.Error(),
			"reason_code": "config_invalid",
			"request_id":  middleware.GetRequestID(c),
		})
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
		c.JSON(http.StatusForbidden, gin.H{
			"error":       "ops access denied",
			"reason_code": "ops_forbidden",
			"request_id":  middleware.GetRequestID(c),
		})
		return
	}
	cfg, err := getGameConfigStore().Rollback()
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":       err.Error(),
			"reason_code": "no_previous_config",
			"request_id":  middleware.GetRequestID(c),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"version":    cfg.Version,
		"economy":    cfg.Economy,
		"features":   cfg.Features,
		"request_id": middleware.GetRequestID(c),
	})
}
