package handlers

import (
	"errors"
	"net/http"
	"strings"

	"animalpoke/backend/internal/battle"
	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BattleHandler serves AP-102 catalog + authoritative PvE sessions.
type BattleHandler struct {
	db      *gorm.DB
	devices *repo.DeviceRepo
	battles *repo.BattleRepo
	wallet  *repo.WalletRepo
}

func NewBattleHandler(db *gorm.DB, devices *repo.DeviceRepo) *BattleHandler {
	return &BattleHandler{
		db:      db,
		devices: devices,
		battles: repo.NewBattleRepo(db),
		wallet:  repo.NewWalletRepo(db),
	}
}

// BattleCatalogPayload returns design catalog for no-DB routes.
func BattleCatalogPayload() battle.Catalog {
	return battle.GetCatalog()
}

func (h *BattleHandler) ownerKey(c *gin.Context) (string, string, string, bool) {
	deviceID := middleware.GetDeviceID(c)
	if deviceID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "reason_code": "unauthorized", "request_id": middleware.GetRequestID(c)})
		return "", "", "", false
	}
	accountID := strings.TrimSpace(middleware.GetAccountID(c))
	if accountID == "" && h.devices != nil {
		if dev, err := h.devices.Find(deviceID); err == nil {
			accountID = strings.TrimSpace(dev.AccountID)
		}
	}
	return repo.OwnerKey(accountID, deviceID), accountID, deviceID, true
}

// Catalog GET /battle/catalog
func (h *BattleHandler) Catalog(c *gin.Context) {
	cat := battle.GetCatalog()
	c.JSON(http.StatusOK, gin.H{
		"catalog":    cat,
		"source":     "server",
		"request_id": middleware.GetRequestID(c),
	})
}

// Start POST /battle/pve/start
func (h *BattleHandler) Start(c *gin.Context) {
	if h.db == nil || h.battles == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "battle unavailable", "reason_code": "db_unavailable", "request_id": middleware.GetRequestID(c)})
		return
	}
	key, _, _, ok := h.ownerKey(c)
	if !ok {
		return
	}
	var req struct {
		Mode        string           `json:"mode"`
		ArchetypeID string           `json:"archetype_id"`
		Team        []battle.Fighter `json:"team" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "reason_code": "bad_request", "request_id": middleware.GetRequestID(c)})
		return
	}
	out, err := h.battles.Start(repo.StartRequest{
		OwnerKey: key, Mode: req.Mode, ArchetypeID: req.ArchetypeID, Team: req.Team,
	})
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrBattleUnknownArch):
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown archetype", "reason_code": "battle_unknown_archetype", "request_id": middleware.GetRequestID(c)})
		case errors.Is(err, repo.ErrBattleInvalidTeam):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "reason_code": "battle_invalid_team", "request_id": middleware.GetRequestID(c)})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "start failed", "reason_code": "battle_failed", "request_id": middleware.GetRequestID(c)})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"session_id":   out.Session.SessionID,
		"seed":         out.Session.Seed,
		"rule_version": out.Session.RuleVersion,
		"archetype_id": out.Session.ArchetypeID,
		"mode":         out.Session.Mode,
		"status":       out.Session.Status,
		"team":         out.Team,
		"enemies":      out.Enemies,
		"threats":      out.Threats,
		"request_id":   middleware.GetRequestID(c),
	})
}

// Settle POST /battle/pve/settle
func (h *BattleHandler) Settle(c *gin.Context) {
	if h.db == nil || h.battles == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "battle unavailable", "reason_code": "db_unavailable", "request_id": middleware.GetRequestID(c)})
		return
	}
	key, accountID, deviceID, ok := h.ownerKey(c)
	if !ok {
		return
	}
	var req struct {
		SessionID     string           `json:"session_id" binding:"required"`
		Commands      []battle.Command `json:"commands"`
		ClaimedWinner string           `json:"claimed_winner"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "reason_code": "bad_request", "request_id": middleware.GetRequestID(c)})
		return
	}
	// clients must not inject rating/reward deltas
	sess, res, err := h.battles.Settle(repo.SettleRequest{
		OwnerKey: key, SessionID: req.SessionID, Commands: req.Commands, ClaimedWinner: req.ClaimedWinner,
	})
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrBattleNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found", "reason_code": "battle_not_found", "request_id": middleware.GetRequestID(c)})
		case errors.Is(err, repo.ErrBattleNotOwner):
			c.JSON(http.StatusForbidden, gin.H{"error": "not owner", "reason_code": "battle_not_owner", "request_id": middleware.GetRequestID(c)})
		case errors.Is(err, repo.ErrBattleTamper):
			c.JSON(http.StatusBadRequest, gin.H{"error": "command log winner mismatch", "reason_code": "battle_command_tamper", "request_id": middleware.GetRequestID(c)})
		case errors.Is(err, repo.ErrBattleInvalidLog):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid battle log", "reason_code": "battle_invalid_log", "request_id": middleware.GetRequestID(c)})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "settle failed", "reason_code": "battle_failed", "request_id": middleware.GetRequestID(c)})
		}
		return
	}

	// win reward (idempotent)
	if res != nil && res.WinnerSide == "player" && h.wallet != nil {
		_, _ = h.wallet.Apply(repo.ApplyRequest{
			DeviceID: deviceID, AccountID: accountID,
			Kind: "currency", Currency: "gold", Delta: 25,
			OperationID: "battle-win:" + sess.SessionID,
			SourceType:  "battle_reward", SourceID: sess.SessionID,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":      sess.SessionID,
		"status":          sess.Status,
		"winner_side":     sess.WinnerSide,
		"command_hash":    sess.CommandHash,
		"result":          res,
		"failure_factors": res.FailureFactors,
		"request_id":      middleware.GetRequestID(c),
	})
}

// Simulate POST /battle/simulate — dry-run authoritative replay (no persistence).
func (h *BattleHandler) Simulate(c *gin.Context) {
	_, _, _, ok := h.ownerKey(c)
	if !ok {
		return
	}
	var req struct {
		Seed     string           `json:"seed" binding:"required"`
		Team     []battle.Fighter `json:"team" binding:"required"`
		Enemies  []battle.Fighter `json:"enemies" binding:"required"`
		Commands []battle.Command `json:"commands"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "reason_code": "bad_request", "request_id": middleware.GetRequestID(c)})
		return
	}
	team, err := battle.NormalizePlayerTeam(req.Team)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "reason_code": "battle_invalid_team", "request_id": middleware.GetRequestID(c)})
		return
	}
	// enemies: trust snapshot but validate skills
	for i := range req.Enemies {
		req.Enemies[i].Side = "enemy"
		if req.Enemies[i].MaxHP <= 0 {
			req.Enemies[i].MaxHP = req.Enemies[i].HP
		}
	}
	res, err := battle.Simulate(req.Seed, team, req.Enemies, req.Commands)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "reason_code": "battle_invalid_log", "request_id": middleware.GetRequestID(c)})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"result":     res,
		"request_id": middleware.GetRequestID(c),
	})
}
