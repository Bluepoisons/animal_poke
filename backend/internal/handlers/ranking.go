package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RankingHandler 区域日榜与结算（AP-114）。
type RankingHandler struct {
	flags   bool
	db      *gorm.DB
	devices *repo.DeviceRepo
	ranking *repo.RankingRepo
	wallet  *repo.WalletRepo
}

func NewRankingHandler(db *gorm.DB, devices *repo.DeviceRepo, enabled bool) *RankingHandler {
	return &RankingHandler{
		flags: enabled, db: db, devices: devices,
		ranking: repo.NewRankingRepo(db),
		wallet:  repo.NewWalletRepo(db),
	}
}

func (h *RankingHandler) owner(c *gin.Context) (repo.RankingOwner, bool) {
	deviceID := middleware.GetDeviceID(c)
	if deviceID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "reason_code": "unauthorized", "request_id": middleware.GetRequestID(c)})
		return repo.RankingOwner{}, false
	}
	if acc := strings.TrimSpace(middleware.GetAccountID(c)); acc != "" {
		return repo.RankingOwner{Type: "account", ID: acc}, true
	}
	if h.devices != nil {
		if dev, err := h.devices.Find(deviceID); err == nil && strings.TrimSpace(dev.AccountID) != "" {
			return repo.RankingOwner{Type: "account", ID: dev.AccountID}, true
		}
	}
	return repo.RankingOwner{Type: "device", ID: deviceID}, true
}

// Daily GET /ranking/daily
func (h *RankingHandler) Daily(c *gin.Context) {
	if !h.flags {
		featureUnavailable(c, "ranking")
		return
	}
	city := c.DefaultQuery("city", "unknown")
	date := c.DefaultQuery("date", time.Now().UTC().Format("2006-01-02"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	// prefer immutable snapshot if settled
	if snap, err := h.ranking.GetSnapshot(date, city); err == nil && snap != nil {
		c.JSON(http.StatusOK, gin.H{
			"city": city, "date": date, "snapshot_id": snap.SnapshotID,
			"settled_at": snap.SettledAt, "entries_json": snap.EntriesJSON,
			"immutable": true, "source": "server", "request_id": middleware.GetRequestID(c),
		})
		return
	}

	rows, total, err := h.ranking.ListBoard(date, city, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ranking failed", "reason_code": "ranking_failed", "request_id": middleware.GetRequestID(c)})
		return
	}
	entries := make([]gin.H, 0, len(rows))
	for i, r := range rows {
		entries = append(entries, gin.H{
			"rank": offset + i + 1, "owner_type": r.OwnerType, "owner_id": r.OwnerID,
			"score": r.Score, "display_name": r.Display,
		})
	}
	me, ok := h.owner(c)
	var my gin.H
	if ok {
		rank, score, _ := h.ranking.MyRank(date, city, me)
		my = gin.H{"rank": rank, "score": score, "owner_type": me.Type, "owner_id": me.ID}
	}
	c.JSON(http.StatusOK, gin.H{
		"city": city, "date": date, "entries": entries, "total": total, "me": my,
		"immutable": false, "source": "server", "request_id": middleware.GetRequestID(c),
	})
}

// ReportScore POST /ranking/score — 可信业务事件入分（MVP：鉴权客户端上报，后续可改为服务端事件）。
func (h *RankingHandler) ReportScore(c *gin.Context) {
	if !h.flags {
		featureUnavailable(c, "ranking")
		return
	}
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	var req struct {
		City     string `json:"city"`
		Delta    int64  `json:"delta" binding:"required"`
		Date     string `json:"date"`
		Display  string `json:"display_name"`
		Eligible *bool  `json:"eligible"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Delta == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "reason_code": "bad_request", "request_id": middleware.GetRequestID(c)})
		return
	}
	// anti-cheat eligibility: require location consent if device available
	eligible := true
	if req.Eligible != nil {
		eligible = *req.Eligible
	}
	if h.devices != nil {
		if dev, err := h.devices.Find(middleware.GetDeviceID(c)); err == nil {
			scope := strings.ToLower(dev.ConsentScope)
			if !strings.Contains(scope, "location") || dev.ConsentRevoked != nil {
				eligible = false
			}
		}
	}
	if err := h.ranking.AddScore(req.Date, req.City, owner, req.Delta, req.Display, eligible); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "score failed", "reason_code": "ranking_failed", "request_id": middleware.GetRequestID(c)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "eligible": eligible, "request_id": middleware.GetRequestID(c)})
}

// Settle POST /ranking/settle — 运维/定时任务结算某城某日（幂等快照 + top 奖励）。
func (h *RankingHandler) Settle(c *gin.Context) {
	if !h.flags {
		featureUnavailable(c, "ranking")
		return
	}
	var req struct {
		City string `json:"city" binding:"required"`
		Date string `json:"date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "city required", "reason_code": "bad_request", "request_id": middleware.GetRequestID(c)})
		return
	}
	snap, err := h.ranking.SettleCity(req.Date, req.City)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settle failed", "reason_code": "settle_failed", "request_id": middleware.GetRequestID(c)})
		return
	}
	_ = h.ranking.GrantTopRewards(snap, h.wallet, 3, func(rank int) int64 {
		switch rank {
		case 1:
			return 100
		case 2:
			return 60
		case 3:
			return 30
		default:
			return 0
		}
	})
	c.JSON(http.StatusOK, gin.H{
		"snapshot_id": snap.SnapshotID, "date": snap.Date, "city": snap.City,
		"settled_at": snap.SettledAt, "request_id": middleware.GetRequestID(c),
	})
}
