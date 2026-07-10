package handlers

import (
	"net/http"
	"strings"
	"time"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// ProductHandler 区域排行 / PvP / 社交 / Ops 服务端权威接口（MVP 骨架）。
// 未完成能力由 FeatureFlags 控制：关闭时返回 501 feature_unavailable，禁止假成功。
type ProductHandler struct {
	flags    config.FeatureFlags
	opsToken string
}

// ProductOptions 运行时开关。
type ProductOptions struct {
	Flags    config.FeatureFlags
	OpsToken string
}

func NewProductHandler() *ProductHandler {
	// 默认关闭全部未完成能力（安全默认）。
	return NewProductHandlerWithOptions(ProductOptions{})
}

func NewProductHandlerWithOptions(opts ProductOptions) *ProductHandler {
	return &ProductHandler{flags: opts.Flags, opsToken: opts.OpsToken}
}

// featureUnavailable 返回结构化 501，reason_code=feature_unavailable。
func featureUnavailable(c *gin.Context, feature string) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":       "feature unavailable",
		"reason_code": "feature_unavailable",
		"feature":     feature,
		"request_id":  middleware.GetRequestID(c),
	})
}

// RankingDaily GET /api/v1/ranking/daily?city=
func (h *ProductHandler) RankingDaily(c *gin.Context) {
	if !h.flags.Ranking {
		featureUnavailable(c, "ranking")
		return
	}
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
	if !h.flags.PvP {
		featureUnavailable(c, "pvp")
		return
	}
	// 骨架阶段：匹配尚未实现。flag 开启时仍不得返回空 match_id 的 2xx 假成功。
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":       "pvp matchmaking not ready",
		"reason_code": "feature_unavailable",
		"feature":     "pvp",
		"request_id":  middleware.GetRequestID(c),
	})
}

// PvPReport POST /api/v1/pvp/result — server-authoritative ELO placeholder
func (h *ProductHandler) PvPReport(c *gin.Context) {
	if !h.flags.PvP {
		featureUnavailable(c, "pvp")
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":       "pvp result reporting not ready",
		"reason_code": "feature_unavailable",
		"feature":     "pvp",
		"request_id":  middleware.GetRequestID(c),
	})
}

// FriendsList GET /api/v1/social/friends
func (h *ProductHandler) FriendsList(c *gin.Context) {
	if !h.flags.Social {
		featureUnavailable(c, "social")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"friends":    []gin.H{},
		"source":     "server",
		"request_id": middleware.GetRequestID(c),
	})
}

// ShareCreate POST /api/v1/social/share
func (h *ProductHandler) ShareCreate(c *gin.Context) {
	if !h.flags.Social {
		featureUnavailable(c, "social")
		return
	}
	// 骨架阶段：share 未实现不可猜 ID / 过期 / ACL，禁止返回 pending 假成功。
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":       "social share not ready",
		"reason_code": "feature_unavailable",
		"feature":     "social",
		"request_id":  middleware.GetRequestID(c),
	})
}

// OpsMetrics GET /api/v1/ops/metrics-summary
// 仅当 FEATURE_OPS=true 且通过内部/管理员校验时可用。
func (h *ProductHandler) OpsMetrics(c *gin.Context) {
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
	c.JSON(http.StatusOK, gin.H{
		"dau":        0,
		"captures":   0,
		"source":     "server",
		"request_id": middleware.GetRequestID(c),
	})
}

// opsAuthorized 校验 X-AP-Ops-Token 或 JWT role claim（admin/ops/internal）。
func (h *ProductHandler) opsAuthorized(c *gin.Context) bool {
	if h.opsToken != "" {
		tok := strings.TrimSpace(c.GetHeader("X-AP-Ops-Token"))
		if tok != "" && tok == h.opsToken {
			return true
		}
	}
	role := strings.ToLower(strings.TrimSpace(middleware.GetRole(c)))
	switch role {
	case "admin", "ops", "internal":
		return true
	default:
		return false
	}
}
