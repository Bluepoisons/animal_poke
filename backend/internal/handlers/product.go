package handlers

import (
	"net/http"
	"time"

	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// ProductHandler 区域排行 / PvP / 社交 服务端权威接口（MVP 骨架）。
type ProductHandler struct{}

func NewProductHandler() *ProductHandler { return &ProductHandler{} }

// RankingDaily GET /api/v1/ranking/daily?city=
func (h *ProductHandler) RankingDaily(c *gin.Context) {
	city := c.Query("city")
	if city == "" {
		city = "unknown"
	}
	c.JSON(http.StatusOK, gin.H{
		"city":       city,
		"date":       time.Now().UTC().Format("2006-01-02"),
		"entries":    []gin.H{},
		"source":     "server",
		"request_id": middleware.GetRequestID(c),
		"note":       "daily settlement job pending; empty board is authoritative",
	})
}

// PvPMatch POST /api/v1/pvp/match
func (h *ProductHandler) PvPMatch(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"match_id":   "",
		"status":     "queued",
		"elo":        1000,
		"source":     "server",
		"request_id": middleware.GetRequestID(c),
	})
}

// PvPReport POST /api/v1/pvp/result — server-authoritative ELO placeholder
func (h *ProductHandler) PvPReport(c *gin.Context) {
	c.JSON(http.StatusAccepted, gin.H{
		"accepted":   true,
		"elo_delta":  0,
		"source":     "server",
		"request_id": middleware.GetRequestID(c),
	})
}

// FriendsList GET /api/v1/social/friends
func (h *ProductHandler) FriendsList(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"friends":    []gin.H{},
		"source":     "server",
		"request_id": middleware.GetRequestID(c),
	})
}

// ShareCreate POST /api/v1/social/share
func (h *ProductHandler) ShareCreate(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{
		"share_id":   "pending",
		"source":     "server",
		"request_id": middleware.GetRequestID(c),
	})
}

// OpsMetrics GET /api/v1/ops/metrics-summary
func (h *ProductHandler) OpsMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"dau":        0,
		"captures":   0,
		"source":     "server",
		"request_id": middleware.GetRequestID(c),
	})
}
