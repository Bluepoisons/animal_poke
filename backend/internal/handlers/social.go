// Package handlers — AP-083 社交：好友状态机、屏蔽/静音/举报、安全分享。
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SocialHandler 社交图谱与安全分享。
type SocialHandler struct {
	flags       config.FeatureFlags
	social      *repo.SocialRepo
	animals     *repo.AnimalRepo
	shareCache  *services.TTLCache[shareCacheEntry]
	reportLimit int // 每小时举报上限
}

type shareCacheEntry struct {
	Denied  bool
	Reason  string
	Payload map[string]interface{}
}

// SocialOptions 依赖。
type SocialOptions struct {
	Flags   config.FeatureFlags
	Social  *repo.SocialRepo
	Animals *repo.AnimalRepo
}

// NewSocialHandler 构造。
func NewSocialHandler(opts SocialOptions) *SocialHandler {
	return &SocialHandler{
		flags:       opts.Flags,
		social:      opts.Social,
		animals:     opts.Animals,
		shareCache:  services.NewBoundedTTLCache[shareCacheEntry](time.Minute, 2048),
		reportLimit: 10,
	}
}

func (h *SocialHandler) socialEnabled() bool {
	return h != nil && h.flags.Social
}

func (h *SocialHandler) requireFeature(c *gin.Context) bool {
	if !h.socialEnabled() {
		featureUnavailable(c, "social")
		return false
	}
	if h.social == nil {
		middleware.WriteError(c, http.StatusServiceUnavailable, "db_unavailable", "social store unavailable", true, nil)
		return false
	}
	return true
}

func (h *SocialHandler) actorKey(c *gin.Context) string {
	return repo.SubjectKey(middleware.GetAccountID(c), middleware.GetDeviceID(c))
}

func (h *SocialHandler) ensureActor(c *gin.Context) (string, *models.SocialProfile, bool) {
	key := h.actorKey(c)
	if key == "" || key == "dev:" {
		middleware.WriteError(c, http.StatusUnauthorized, "missing_device", "device required", false, nil)
		return "", nil, false
	}
	// 默认展示名 / 未成年标记可由 query 或 body 在 settings 设置；此处懒创建成人资料
	p, err := h.social.EnsureProfile(key, "", false)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "social_error", "profile failed", true, nil)
		return "", nil, false
	}
	return key, p, true
}

// requireActorSocial 发起社交互动时要求自身 social_enabled。
func (h *SocialHandler) requireActorSocial(c *gin.Context, p *models.SocialProfile) bool {
	if p == nil || !p.SocialEnabled {
		middleware.WriteError(c, http.StatusForbidden, "social_disabled", "social features disabled for this account", false, nil)
		return false
	}
	return true
}

// ---------- settings ----------

// GetSettings GET /api/v1/social/settings
func (h *SocialHandler) GetSettings(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, p, ok := h.ensureActor(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_key":       key,
		"display_name":   p.DisplayName,
		"social_enabled": p.SocialEnabled,
		"is_minor":       p.IsMinor,
		"request_id":     middleware.GetRequestID(c),
	})
}

type socialSettingsPatch struct {
	DisplayName   *string `json:"display_name"`
	SocialEnabled *bool   `json:"social_enabled"`
	IsMinor       *bool   `json:"is_minor"`
}

// PatchSettings PATCH /api/v1/social/settings
func (h *SocialHandler) PatchSettings(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, _, ok := h.ensureActor(c)
	if !ok {
		return
	}
	var req socialSettingsPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "invalid body", false, nil)
		return
	}
	p, err := h.social.UpdateProfile(key, req.DisplayName, req.SocialEnabled, req.IsMinor)
	if err != nil {
		if errors.Is(err, repo.ErrInvalidDisplayName) {
			middleware.WriteError(c, http.StatusBadRequest, "invalid_display_name", "display name rejected", false, nil)
			return
		}
		middleware.WriteError(c, http.StatusInternalServerError, "social_error", "update failed", true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_key":       key,
		"display_name":   p.DisplayName,
		"social_enabled": p.SocialEnabled,
		"is_minor":       p.IsMinor,
		"request_id":     middleware.GetRequestID(c),
	})
}

// ---------- friends ----------

// FriendsList GET /api/v1/social/friends?cursor=&limit=
func (h *SocialHandler) FriendsList(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, p, ok := h.ensureActor(c)
	if !ok {
		return
	}
	// 关闭社交时返回空列表（非假成功字段），明确 social_enabled=false
	if !p.SocialEnabled {
		c.JSON(http.StatusOK, gin.H{
			"friends":        []gin.H{},
			"next_cursor":    "",
			"social_enabled": false,
			"source":         "server",
			"request_id":     middleware.GetRequestID(c),
		})
		return
	}
	after, err := repo.ParseCursor(c.Query("cursor"))
	if err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_cursor", "invalid cursor", false, nil)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, err := h.social.ListFriends(key, after, limit)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "social_error", "list friends failed", true, nil)
		return
	}
	keys := make([]string, 0, len(rows))
	for _, f := range rows {
		keys = append(keys, repo.FriendOther(f, key))
	}
	profiles, _ := h.social.ProfilesByKeys(keys)
	items := make([]gin.H, 0, len(rows))
	var next uint
	for _, f := range rows {
		other := repo.FriendOther(f, key)
		name := other
		if pr, ok := profiles[other]; ok {
			name = pr.DisplayName
		}
		items = append(items, gin.H{
			"user_key":     other,
			"display_name": name,
			"since":        f.CreatedAt.UTC().Format(time.RFC3339),
		})
		next = f.ID
	}
	nextCursor := ""
	if len(rows) > 0 && (limit <= 0 || len(rows) >= limit || limit > 100 && len(rows) >= 50) {
		// 有可能还有下一页：当返回数达到 limit 时给 cursor
		lim := limit
		if lim <= 0 || lim > 100 {
			lim = 50
		}
		if len(rows) >= lim {
			nextCursor = repo.FormatCursor(next)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"friends":        items,
		"next_cursor":    nextCursor,
		"social_enabled": true,
		"source":         "server",
		"request_id":     middleware.GetRequestID(c),
	})
}

type targetUserBody struct {
	TargetUserID string `json:"target_user_id"`
	UserKey      string `json:"user_key"`
}

func (b targetUserBody) key() string {
	if s := strings.TrimSpace(b.TargetUserID); s != "" {
		return repo.NormalizeUserKey(s)
	}
	return repo.NormalizeUserKey(b.UserKey)
}

type requestIDBody struct {
	RequestID string `json:"request_id" binding:"required"`
}

// FriendRequestCreate POST /api/v1/social/friends/request
func (h *SocialHandler) FriendRequestCreate(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, p, ok := h.ensureActor(c)
	if !ok {
		return
	}
	if !h.requireActorSocial(c, p) {
		return
	}
	var body targetUserBody
	if err := c.ShouldBindJSON(&body); err != nil || body.key() == "" {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "target_user_id required", false, nil)
		return
	}
	target := body.key()
	// 确保对方资料存在（便于搜索/展示）
	_, _ = h.social.EnsureProfile(target, "", false)
	req, auto, err := h.social.CreateFriendRequest(key, target)
	if err != nil {
		h.writeSocialErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"request":       friendRequestJSON(req),
		"auto_accepted": auto,
		"request_id":    middleware.GetRequestID(c),
	})
}

// FriendRequestAccept POST /api/v1/social/friends/accept
func (h *SocialHandler) FriendRequestAccept(c *gin.Context) {
	h.resolveFriendRequest(c, "accept")
}

// FriendRequestReject POST /api/v1/social/friends/reject
func (h *SocialHandler) FriendRequestReject(c *gin.Context) {
	h.resolveFriendRequest(c, "reject")
}

// FriendRequestCancel POST /api/v1/social/friends/cancel
func (h *SocialHandler) FriendRequestCancel(c *gin.Context) {
	h.resolveFriendRequest(c, "cancel")
}

func (h *SocialHandler) resolveFriendRequest(c *gin.Context, action string) {
	if !h.requireFeature(c) {
		return
	}
	key, p, ok := h.ensureActor(c)
	if !ok {
		return
	}
	if action != "cancel" && !h.requireActorSocial(c, p) {
		return
	}
	var body requestIDBody
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.RequestID) == "" {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "request_id required", false, nil)
		return
	}
	var (
		req *models.FriendRequest
		err error
	)
	switch action {
	case "accept":
		req, err = h.social.AcceptFriendRequest(key, body.RequestID)
	case "reject":
		req, err = h.social.RejectFriendRequest(key, body.RequestID)
	case "cancel":
		req, err = h.social.CancelFriendRequest(key, body.RequestID)
	default:
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "unknown action", false, nil)
		return
	}
	if err != nil {
		h.writeSocialErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"request":    friendRequestJSON(req),
		"request_id": middleware.GetRequestID(c),
	})
}

// FriendRemove POST /api/v1/social/friends/remove
func (h *SocialHandler) FriendRemove(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, _, ok := h.ensureActor(c)
	if !ok {
		return
	}
	var body targetUserBody
	if err := c.ShouldBindJSON(&body); err != nil || body.key() == "" {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "target_user_id required", false, nil)
		return
	}
	if err := h.social.RemoveFriend(key, body.key()); err != nil {
		h.writeSocialErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":     "removed",
		"user_key":   body.key(),
		"request_id": middleware.GetRequestID(c),
	})
}

// FriendRequestsList GET /api/v1/social/friends/requests?direction=incoming|outgoing
func (h *SocialHandler) FriendRequestsList(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, _, ok := h.ensureActor(c)
	if !ok {
		return
	}
	dir := c.DefaultQuery("direction", "incoming")
	after, err := repo.ParseCursor(c.Query("cursor"))
	if err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_cursor", "invalid cursor", false, nil)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, err := h.social.ListRequests(key, dir, after, limit)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "social_error", "list requests failed", true, nil)
		return
	}
	items := make([]gin.H, 0, len(rows))
	var next uint
	lim := limit
	if lim <= 0 || lim > 100 {
		lim = 50
	}
	for _, r := range rows {
		items = append(items, friendRequestJSON(&r))
		next = r.ID
	}
	nextCursor := ""
	if len(rows) >= lim {
		nextCursor = repo.FormatCursor(next)
	}
	c.JSON(http.StatusOK, gin.H{
		"requests":    items,
		"direction":   dir,
		"next_cursor": nextCursor,
		"request_id":  middleware.GetRequestID(c),
	})
}

func friendRequestJSON(r *models.FriendRequest) gin.H {
	if r == nil {
		return gin.H{}
	}
	return gin.H{
		"request_id":    r.RequestID,
		"from_user_key": r.FromUserKey,
		"to_user_key":   r.ToUserKey,
		"status":        r.Status,
		"created_at":    r.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// ---------- block / mute / report ----------

// BlockUser POST /api/v1/social/block
func (h *SocialHandler) BlockUser(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, _, ok := h.ensureActor(c)
	if !ok {
		return
	}
	var body targetUserBody
	if err := c.ShouldBindJSON(&body); err != nil || body.key() == "" {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "target_user_id required", false, nil)
		return
	}
	target := body.key()
	if err := h.social.BlockUser(key, target); err != nil {
		h.writeSocialErr(c, err)
		return
	}
	// 撤销缓存中与双方相关的 share 读缓存（保守：整表清理代价高，按 token 未知时跳过）
	c.JSON(http.StatusOK, gin.H{
		"status":     "blocked",
		"user_key":   target,
		"request_id": middleware.GetRequestID(c),
	})
}

// UnblockUser POST /api/v1/social/unblock
func (h *SocialHandler) UnblockUser(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, _, ok := h.ensureActor(c)
	if !ok {
		return
	}
	var body targetUserBody
	if err := c.ShouldBindJSON(&body); err != nil || body.key() == "" {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "target_user_id required", false, nil)
		return
	}
	target := body.key()
	if err := h.social.UnblockUser(key, target); err != nil {
		h.writeSocialErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":     "unblocked",
		"user_key":   target,
		"request_id": middleware.GetRequestID(c),
	})
}

// ListBlocks GET /api/v1/social/blocks
func (h *SocialHandler) ListBlocks(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, _, ok := h.ensureActor(c)
	if !ok {
		return
	}
	after, err := repo.ParseCursor(c.Query("cursor"))
	if err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_cursor", "invalid cursor", false, nil)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, err := h.social.ListBlocks(key, after, limit)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "social_error", "list blocks failed", true, nil)
		return
	}
	items := make([]gin.H, 0, len(rows))
	var next uint
	lim := limit
	if lim <= 0 || lim > 100 {
		lim = 50
	}
	for _, b := range rows {
		items = append(items, gin.H{
			"user_key":   b.BlockedUserKey,
			"blocked_at": b.CreatedAt.UTC().Format(time.RFC3339),
		})
		next = b.ID
	}
	nextCursor := ""
	if len(rows) >= lim {
		nextCursor = repo.FormatCursor(next)
	}
	c.JSON(http.StatusOK, gin.H{
		"blocks":      items,
		"next_cursor": nextCursor,
		"request_id":  middleware.GetRequestID(c),
	})
}

// MuteUser POST /api/v1/social/mute
func (h *SocialHandler) MuteUser(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, _, ok := h.ensureActor(c)
	if !ok {
		return
	}
	var body targetUserBody
	if err := c.ShouldBindJSON(&body); err != nil || body.key() == "" {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "target_user_id required", false, nil)
		return
	}
	if err := h.social.MuteUser(key, body.key()); err != nil {
		h.writeSocialErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "muted", "user_key": body.key(), "request_id": middleware.GetRequestID(c)})
}

// UnmuteUser POST /api/v1/social/unmute
func (h *SocialHandler) UnmuteUser(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, _, ok := h.ensureActor(c)
	if !ok {
		return
	}
	var body targetUserBody
	if err := c.ShouldBindJSON(&body); err != nil || body.key() == "" {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "target_user_id required", false, nil)
		return
	}
	if err := h.social.UnmuteUser(key, body.key()); err != nil {
		h.writeSocialErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "unmuted", "user_key": body.key(), "request_id": middleware.GetRequestID(c)})
}

type socialReportBody struct {
	TargetUserID string `json:"target_user_id"`
	UserKey      string `json:"user_key"`
	Category     string `json:"category" binding:"required"`
	Note         string `json:"note"`
}

func (b socialReportBody) key() string {
	if s := strings.TrimSpace(b.TargetUserID); s != "" {
		return repo.NormalizeUserKey(s)
	}
	return repo.NormalizeUserKey(b.UserKey)
}

// ReportUser POST /api/v1/social/report
func (h *SocialHandler) ReportUser(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, _, ok := h.ensureActor(c)
	if !ok {
		return
	}
	var body socialReportBody
	if err := c.ShouldBindJSON(&body); err != nil || body.key() == "" {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "target_user_id and category required", false, nil)
		return
	}
	// 滥用限流：每小时 reportLimit 次
	n, err := h.social.CountRecentReports(key, time.Now().UTC().Add(-time.Hour))
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "social_error", "report failed", true, nil)
		return
	}
	if int(n) >= h.reportLimit {
		middleware.AbortTooMany(c, "report_rate_limited", "too many reports", 3600, nil)
		return
	}
	rep, err := h.social.CreateUserReport(key, body.key(), body.Category, body.Note)
	if err != nil {
		h.writeSocialErr(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"status":     "accepted",
		"report_id":  rep.ReportID,
		"request_id": middleware.GetRequestID(c),
	})
}

// SearchUsers GET /api/v1/social/search?q=
func (h *SocialHandler) SearchUsers(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, p, ok := h.ensureActor(c)
	if !ok {
		return
	}
	if !h.requireActorSocial(c, p) {
		return
	}
	q := c.Query("q")
	rows, err := h.social.SearchProfiles(key, q, 20)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "social_error", "search failed", true, nil)
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		items = append(items, gin.H{
			"user_key":     r.UserKey,
			"display_name": r.DisplayName,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"users":      items,
		"request_id": middleware.GetRequestID(c),
	})
}

// ---------- share ----------

type shareCreateBody struct {
	AnimalUUID string `json:"animal_uuid" binding:"required"`
	ACL        string `json:"acl"`
	TTLHours   int    `json:"ttl_hours"`
}

// ShareCreate POST /api/v1/social/share
func (h *SocialHandler) ShareCreate(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, p, ok := h.ensureActor(c)
	if !ok {
		return
	}
	if !h.requireActorSocial(c, p) {
		return
	}
	if h.animals == nil {
		middleware.WriteError(c, http.StatusServiceUnavailable, "db_unavailable", "animals unavailable", true, nil)
		return
	}
	var body shareCreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "animal_uuid required", false, nil)
		return
	}
	if _, err := uuid.Parse(strings.TrimSpace(body.AnimalUUID)); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_animal_uuid", "animal_uuid must be uuid", false, nil)
		return
	}
	animal, err := h.animals.FindByUUID(strings.TrimSpace(body.AnimalUUID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.WriteError(c, http.StatusNotFound, "animal_not_found", "animal not found", false, nil)
			return
		}
		middleware.WriteError(c, http.StatusInternalServerError, "social_error", "lookup failed", true, nil)
		return
	}
	ttl := time.Duration(body.TTLHours) * time.Hour
	share, token, err := h.social.CreateShare(key, animal, body.ACL, ttl)
	if err != nil {
		h.writeSocialErr(c, err)
		return
	}
	// 响应含 capability token；不回写精确位置/设备
	view := repo.SharePublicView(share)
	view["share_token"] = token
	view["share_id"] = token
	view["request_id"] = middleware.GetRequestID(c)
	c.JSON(http.StatusCreated, view)
}

// ShareGet GET /api/v1/social/share/:token
func (h *SocialHandler) ShareGet(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	if h.social == nil {
		middleware.WriteError(c, http.StatusServiceUnavailable, "db_unavailable", "social store unavailable", true, nil)
		return
	}
	token := strings.TrimSpace(c.Param("token"))
	viewer := h.actorKey(c)
	cacheKey := "share:" + token + ":" + viewer

	// 仅否定缓存（防枚举）；肯定路径始终重校验 block/revoke/ACL
	if ent, ok := h.shareCache.Get(cacheKey); ok && ent.Denied {
		// revoked/expired 用 410；其余 403/404 由 reason 区分不够精确时统一 forbidden
		status := http.StatusForbidden
		if ent.Reason == "share_not_found" {
			status = http.StatusNotFound
		} else if ent.Reason == "share_expired" || ent.Reason == "share_revoked" {
			status = http.StatusGone
		}
		middleware.WriteError(c, status, ent.Reason, "share not accessible", false, nil)
		return
	}

	share, err := h.social.AccessShare(viewer, token)
	if err != nil {
		reason := mapShareErr(err)
		negTTL := 30 * time.Second
		if errors.Is(err, repo.ErrShareRevoked) || errors.Is(err, repo.ErrShareExpired) {
			negTTL = 5 * time.Minute
		}
		h.shareCache.Set(cacheKey, shareCacheEntry{Denied: true, Reason: reason}, negTTL)
		h.writeSocialErr(c, err)
		return
	}
	view := repo.SharePublicView(share)
	view["request_id"] = middleware.GetRequestID(c)
	c.JSON(http.StatusOK, view)
}

// ShareRevoke POST /api/v1/social/share/:token/revoke
func (h *SocialHandler) ShareRevoke(c *gin.Context) {
	if !h.requireFeature(c) {
		return
	}
	key, _, ok := h.ensureActor(c)
	if !ok {
		return
	}
	token := strings.TrimSpace(c.Param("token"))
	share, err := h.social.RevokeShare(key, token)
	if err != nil {
		h.writeSocialErr(c, err)
		return
	}
	// 失效缓存：删除 viewer 无关的宽匹配困难，写入 denied 标记用 owner 键
	h.shareCache.Set("share:"+token+":"+key, shareCacheEntry{Denied: true, Reason: "share_revoked"}, time.Hour)
	// 也写一个空 viewer 键
	h.shareCache.Set("share:"+token+":", shareCacheEntry{Denied: true, Reason: "share_revoked"}, time.Hour)
	c.JSON(http.StatusOK, gin.H{
		"status":     "revoked",
		"revoked":    true,
		"expires_at": share.ExpiresAt.UTC().Format(time.RFC3339),
		"request_id": middleware.GetRequestID(c),
	})
}

func mapShareErr(err error) string {
	switch {
	case errors.Is(err, repo.ErrShareNotFound):
		return "share_not_found"
	case errors.Is(err, repo.ErrShareExpired):
		return "share_expired"
	case errors.Is(err, repo.ErrShareRevoked):
		return "share_revoked"
	case errors.Is(err, repo.ErrShareForbidden):
		return "share_forbidden"
	case errors.Is(err, repo.ErrBlocked):
		return "blocked"
	default:
		return "share_error"
	}
}

func (h *SocialHandler) writeSocialErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repo.ErrSocialDisabled):
		middleware.WriteError(c, http.StatusForbidden, "social_disabled", "social features disabled", false, nil)
	case errors.Is(err, repo.ErrBlocked):
		// 不暴露屏蔽细节：统一 forbidden
		middleware.WriteError(c, http.StatusForbidden, "blocked", "action not allowed", false, nil)
	case errors.Is(err, repo.ErrSelfAction):
		middleware.WriteError(c, http.StatusBadRequest, "self_action", "cannot target self", false, nil)
	case errors.Is(err, repo.ErrAlreadyFriends):
		middleware.WriteError(c, http.StatusConflict, "already_friends", "already friends", false, nil)
	case errors.Is(err, repo.ErrFriendLimit):
		middleware.WriteError(c, http.StatusConflict, "friend_limit", "friend list full", false, nil)
	case errors.Is(err, repo.ErrRequestNotFound):
		middleware.WriteError(c, http.StatusNotFound, "request_not_found", "friend request not found", false, nil)
	case errors.Is(err, repo.ErrRequestNotPending):
		middleware.WriteError(c, http.StatusConflict, "request_not_pending", "request is not pending", false, nil)
	case errors.Is(err, repo.ErrRequestUnauthorized):
		middleware.WriteError(c, http.StatusForbidden, "request_unauthorized", "not allowed for this request", false, nil)
	case errors.Is(err, repo.ErrInvalidTarget):
		middleware.WriteError(c, http.StatusBadRequest, "invalid_target", "invalid target", false, nil)
	case errors.Is(err, repo.ErrInvalidDisplayName):
		middleware.WriteError(c, http.StatusBadRequest, "invalid_display_name", "display name rejected", false, nil)
	case errors.Is(err, repo.ErrShareNotFound):
		// 枚举防护：不区分不存在与无权限时可用 404
		middleware.WriteError(c, http.StatusNotFound, "share_not_found", "share not found", false, nil)
	case errors.Is(err, repo.ErrShareExpired):
		middleware.WriteError(c, http.StatusGone, "share_expired", "share expired", false, nil)
	case errors.Is(err, repo.ErrShareRevoked):
		middleware.WriteError(c, http.StatusGone, "share_revoked", "share revoked", false, nil)
	case errors.Is(err, repo.ErrShareForbidden):
		middleware.WriteError(c, http.StatusForbidden, "share_forbidden", "share not accessible", false, nil)
	case errors.Is(err, repo.ErrAnimalNotOwned):
		middleware.WriteError(c, http.StatusForbidden, "animal_not_owned", "animal not owned", false, nil)
	case errors.Is(err, repo.ErrAnimalNotFound):
		middleware.WriteError(c, http.StatusNotFound, "animal_not_found", "animal not found", false, nil)
	case errors.Is(err, repo.ErrInvalidACL):
		middleware.WriteError(c, http.StatusBadRequest, "invalid_acl", "invalid acl", false, nil)
	case errors.Is(err, repo.ErrReportInvalid):
		middleware.WriteError(c, http.StatusBadRequest, "report_invalid", "invalid report", false, nil)
	case errors.Is(err, repo.ErrRateLimited):
		middleware.AbortTooMany(c, "rate_limited", "rate limited", 60, nil)
	default:
		middleware.WriteError(c, http.StatusInternalServerError, "social_error", "social operation failed", true, nil)
	}
}
