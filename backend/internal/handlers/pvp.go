package handlers

import (
	"net/http"
	"strings"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PvPHandler 服务端匹配与段位结算（AP-115）。
type PvPHandler struct {
	enabled bool
	db      *gorm.DB
	devices *repo.DeviceRepo
	pvp     *repo.PvPRepo
	wallet  *repo.WalletRepo
}

func NewPvPHandler(db *gorm.DB, devices *repo.DeviceRepo, enabled bool) *PvPHandler {
	return &PvPHandler{
		enabled: enabled, db: db, devices: devices,
		pvp: repo.NewPvPRepo(db), wallet: repo.NewWalletRepo(db),
	}
}

func (h *PvPHandler) ownerKey(c *gin.Context) (string, string, string, bool) {
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

// Match POST /pvp/match
func (h *PvPHandler) Match(c *gin.Context) {
	if !h.enabled {
		featureUnavailable(c, "pvp")
		return
	}
	key, _, _, ok := h.ownerKey(c)
	if !ok {
		return
	}
	match, queued, err := h.pvp.EnqueueOrMatch(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "match failed", "reason_code": "pvp_failed", "request_id": middleware.GetRequestID(c)})
		return
	}
	if queued {
		c.JSON(http.StatusAccepted, gin.H{"status": "queued", "request_id": middleware.GetRequestID(c)})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"match_id": match.MatchID, "seed": match.Seed, "rule_version": match.RuleVersion,
		"player_a": match.PlayerA, "player_b": match.PlayerB, "status": match.Status,
		"request_id": middleware.GetRequestID(c),
	})
}

// Cancel POST /pvp/cancel
func (h *PvPHandler) Cancel(c *gin.Context) {
	if !h.enabled {
		featureUnavailable(c, "pvp")
		return
	}
	key, _, _, ok := h.ownerKey(c)
	if !ok {
		return
	}
	_ = h.pvp.CancelQueue(key)
	c.JSON(http.StatusOK, gin.H{"status": "cancelled", "request_id": middleware.GetRequestID(c)})
}

// Result POST /pvp/result
func (h *PvPHandler) Result(c *gin.Context) {
	if !h.enabled {
		featureUnavailable(c, "pvp")
		return
	}
	key, accountID, deviceID, ok := h.ownerKey(c)
	if !ok {
		return
	}
	var req struct {
		MatchID    string         `json:"match_id" binding:"required"`
		Winner     string         `json:"winner" binding:"required"`
		CommandLog map[string]any `json:"command_log"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "reason_code": "bad_request", "request_id": middleware.GetRequestID(c)})
		return
	}
	// 客户端不得提交 rating delta —— 仅 winner + 日志
	if req.CommandLog != nil {
		if _, has := req.CommandLog["rating_delta"]; has {
			c.JSON(http.StatusBadRequest, gin.H{"error": "rating_delta not allowed", "reason_code": "client_rating_forbidden", "request_id": middleware.GetRequestID(c)})
			return
		}
	}
	m, err := h.pvp.SubmitResult(key, req.MatchID, req.Winner, req.CommandLog)
	if err != nil {
		switch err {
		case repo.ErrPvPNotOwner:
			c.JSON(http.StatusForbidden, gin.H{"error": "not a participant", "reason_code": "pvp_not_owner", "request_id": middleware.GetRequestID(c)})
		case repo.ErrPvPInvalidLog:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid battle log", "reason_code": "pvp_invalid_log", "request_id": middleware.GetRequestID(c)})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "settle failed", "reason_code": "pvp_failed", "request_id": middleware.GetRequestID(c)})
		}
		return
	}
	// 胜者奖励（幂等 operation）
	if m.Winner == key {
		_, _ = h.wallet.Apply(repo.ApplyRequest{
			DeviceID: deviceID, AccountID: accountID,
			Kind: "currency", Currency: "gold", Delta: 20,
			OperationID: "pvp-win:" + m.MatchID + ":" + key,
			SourceType:  "pvp_reward", SourceID: m.MatchID,
		})
	}
	ra, _ := h.pvp.GetRating(m.PlayerA)
	rb, _ := h.pvp.GetRating(m.PlayerB)
	c.JSON(http.StatusOK, gin.H{
		"match_id": m.MatchID, "status": m.Status, "winner": m.Winner,
		"ratings": gin.H{
			m.PlayerA: gin.H{"rating": ra.Rating, "wins": ra.Wins, "losses": ra.Losses},
			m.PlayerB: gin.H{"rating": rb.Rating, "wins": rb.Wins, "losses": rb.Losses},
		},
		"request_id": middleware.GetRequestID(c),
	})
}
