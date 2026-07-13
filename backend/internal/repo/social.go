// Package repo — AP-083 社交图谱：好友请求、屏蔽、静音、举报、安全分享。
package repo

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"animalpoke/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 社交领域错误。
var (
	ErrSocialUnavailable   = errors.New("social_unavailable")
	ErrSocialDisabled      = errors.New("social_disabled")
	ErrSelfAction          = errors.New("self_action")
	ErrBlocked             = errors.New("blocked")
	ErrAlreadyFriends      = errors.New("already_friends")
	ErrNotFriends          = errors.New("not_friends")
	ErrRequestNotFound     = errors.New("request_not_found")
	ErrRequestNotPending   = errors.New("request_not_pending")
	ErrRequestUnauthorized = errors.New("request_unauthorized")
	ErrFriendLimit         = errors.New("friend_limit")
	ErrInvalidTarget       = errors.New("invalid_target")
	ErrInvalidDisplayName  = errors.New("invalid_display_name")
	ErrShareNotFound       = errors.New("share_not_found")
	ErrShareExpired        = errors.New("share_expired")
	ErrShareRevoked        = errors.New("share_revoked")
	ErrShareForbidden      = errors.New("share_forbidden")
	ErrInvalidACL          = errors.New("invalid_acl")
	ErrRateLimited         = errors.New("rate_limited")
	ErrReportInvalid       = errors.New("report_invalid")
)

// SocialRepo 社交仓储。
type SocialRepo struct {
	db *gorm.DB
}

// NewSocialRepo 构造。
func NewSocialRepo(db *gorm.DB) *SocialRepo {
	return &SocialRepo{db: db}
}

// WithTx 绑定事务。
func (r *SocialRepo) WithTx(tx *gorm.DB) *SocialRepo {
	return &SocialRepo{db: tx}
}

// DB 暴露底层连接。
func (r *SocialRepo) DB() *gorm.DB { return r.db }

// PairKey 规范化双方用户键（无序边）。
func PairKey(a, b string) string {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

// OrderedPair 返回 (low, high)。
func OrderedPair(a, b string) (string, string) {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a > b {
		return b, a
	}
	return a, b
}

// NormalizeUserKey 将 account_id / device_id / 已带前缀键规范为 user_key。
// 优先识别已有前缀；裸 UUID 形 account 用 acc:；否则视为 device。
func NormalizeUserKey(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "acc:") || strings.HasPrefix(s, "dev:") {
		return s
	}
	// UUID 形 → account
	if _, err := uuid.Parse(s); err == nil {
		return "acc:" + s
	}
	return "dev:" + s
}

// SubjectKey 当前主体键：绑定账号优先。
func SubjectKey(accountID, deviceID string) string {
	return OwnerKey(accountID, deviceID)
}

// ValidateDisplayName 昵称安全校验（长度 + 控制字符 + 粗略敏感词）。
func ValidateDisplayName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return ErrInvalidDisplayName
	}
	if utf8.RuneCountInString(n) > 32 {
		return ErrInvalidDisplayName
	}
	for _, r := range n {
		if unicode.IsControl(r) {
			return ErrInvalidDisplayName
		}
	}
	lower := strings.ToLower(n)
	for _, bad := range []string{"admin", "official", "官方", "管理员", "abuse", "色情", "赌博"} {
		if strings.Contains(lower, bad) {
			return ErrInvalidDisplayName
		}
	}
	return nil
}

// EnsureProfile 确保资料存在；未成年人默认关闭社交。
func (r *SocialRepo) EnsureProfile(userKey, displayName string, isMinor bool) (*models.SocialProfile, error) {
	if r == nil || r.db == nil {
		return nil, ErrSocialUnavailable
	}
	userKey = strings.TrimSpace(userKey)
	if userKey == "" {
		return nil, ErrInvalidTarget
	}
	var p models.SocialProfile
	err := r.db.Where("user_key = ?", userKey).First(&p).Error
	if err == nil {
		return &p, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	name := strings.TrimSpace(displayName)
	if name == "" {
		// 默认展示名：截取 user_key 后缀
		name = defaultDisplayName(userKey)
	}
	if err := ValidateDisplayName(name); err != nil {
		name = defaultDisplayName(userKey)
	}
	p = models.SocialProfile{
		UserKey:       userKey,
		DisplayName:   name,
		SocialEnabled: !isMinor, // 未成年人默认关闭
		IsMinor:       isMinor,
	}
	if err := r.db.Create(&p).Error; err != nil {
		// 并发创建：再读一次
		var existing models.SocialProfile
		if e2 := r.db.Where("user_key = ?", userKey).First(&existing).Error; e2 == nil {
			return &existing, nil
		}
		return nil, err
	}
	return &p, nil
}

func defaultDisplayName(userKey string) string {
	s := userKey
	if i := strings.IndexByte(s, ':'); i >= 0 && i+1 < len(s) {
		s = s[i+1:]
	}
	if len(s) > 8 {
		s = s[:8]
	}
	if s == "" {
		s = "player"
	}
	return "player-" + s
}

// GetProfile 读取资料。
func (r *SocialRepo) GetProfile(userKey string) (*models.SocialProfile, error) {
	if r == nil || r.db == nil {
		return nil, ErrSocialUnavailable
	}
	var p models.SocialProfile
	if err := r.db.Where("user_key = ?", userKey).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateProfile 更新展示名 / 社交开关 / 未成年标记。
func (r *SocialRepo) UpdateProfile(userKey string, displayName *string, socialEnabled *bool, isMinor *bool) (*models.SocialProfile, error) {
	p, err := r.EnsureProfile(userKey, "", false)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if displayName != nil {
		if err := ValidateDisplayName(*displayName); err != nil {
			return nil, err
		}
		updates["display_name"] = strings.TrimSpace(*displayName)
	}
	if socialEnabled != nil {
		// 未成年强制关闭社交（不可自行开启）
		if p.IsMinor || (isMinor != nil && *isMinor) {
			updates["social_enabled"] = false
		} else {
			updates["social_enabled"] = *socialEnabled
		}
	}
	if isMinor != nil {
		updates["is_minor"] = *isMinor
		if *isMinor {
			updates["social_enabled"] = false
		}
	}
	if len(updates) == 0 {
		return p, nil
	}
	if err := r.db.Model(&models.SocialProfile{}).Where("user_key = ?", userKey).Updates(updates).Error; err != nil {
		return nil, err
	}
	return r.GetProfile(userKey)
}

// IsBlockedEither 任一方屏蔽则 true。
func (r *SocialRepo) IsBlockedEither(a, b string) (bool, error) {
	if r == nil || r.db == nil {
		return false, ErrSocialUnavailable
	}
	var n int64
	err := r.db.Model(&models.SocialBlock{}).
		Where("(blocker_user_key = ? AND blocked_user_key = ?) OR (blocker_user_key = ? AND blocked_user_key = ?)",
			a, b, b, a).
		Count(&n).Error
	return n > 0, err
}

// IsBlocked 明确方向：blocker 是否屏蔽了 blocked。
func (r *SocialRepo) IsBlocked(blocker, blocked string) (bool, error) {
	if r == nil || r.db == nil {
		return false, ErrSocialUnavailable
	}
	var n int64
	err := r.db.Model(&models.SocialBlock{}).
		Where("blocker_user_key = ? AND blocked_user_key = ?", blocker, blocked).
		Count(&n).Error
	return n > 0, err
}

// requireSocialActive 双方社交开启且未屏蔽。
func (r *SocialRepo) requireSocialActive(actor, target string) error {
	if actor == "" || target == "" {
		return ErrInvalidTarget
	}
	if actor == target {
		return ErrSelfAction
	}
	blocked, err := r.IsBlockedEither(actor, target)
	if err != nil {
		return err
	}
	if blocked {
		return ErrBlocked
	}
	ap, err := r.EnsureProfile(actor, "", false)
	if err != nil {
		return err
	}
	if !ap.SocialEnabled {
		return ErrSocialDisabled
	}
	tp, err := r.EnsureProfile(target, "", false)
	if err != nil {
		return err
	}
	if !tp.SocialEnabled {
		return ErrSocialDisabled
	}
	return nil
}

// CountFriends 活跃好友数。
func (r *SocialRepo) CountFriends(userKey string) (int64, error) {
	var n int64
	err := r.db.Model(&models.Friendship{}).
		Where("status = ? AND (user_low = ? OR user_high = ?)", models.FriendshipActive, userKey, userKey).
		Count(&n).Error
	return n, err
}

// AreFriends 是否为活跃好友。
func (r *SocialRepo) AreFriends(a, b string) (bool, error) {
	low, high := OrderedPair(a, b)
	var n int64
	err := r.db.Model(&models.Friendship{}).
		Where("user_low = ? AND user_high = ? AND status = ?", low, high, models.FriendshipActive).
		Count(&n).Error
	return n > 0, err
}

// ListFriends cursor 分页（按 friendship.id 升序）。
func (r *SocialRepo) ListFriends(userKey string, afterID uint, limit int) ([]models.Friendship, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows []models.Friendship
	q := r.db.Where("status = ? AND (user_low = ? OR user_high = ?)", models.FriendshipActive, userKey, userKey)
	if afterID > 0 {
		q = q.Where("id > ?", afterID)
	}
	err := q.Order("id asc").Limit(limit).Find(&rows).Error
	return rows, err
}

// FriendOther 返回好友边上的另一方。
func FriendOther(f models.Friendship, me string) string {
	if f.UserLow == me {
		return f.UserHigh
	}
	return f.UserLow
}

// CreateFriendRequest 发起请求；交叉邀请自动合并为好友；幂等。
func (r *SocialRepo) CreateFriendRequest(from, to string) (*models.FriendRequest, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, ErrSocialUnavailable
	}
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if err := r.requireSocialActive(from, to); err != nil {
		return nil, false, err
	}
	// 已是好友 → 幂等
	friends, err := r.AreFriends(from, to)
	if err != nil {
		return nil, false, err
	}
	if friends {
		return nil, true, ErrAlreadyFriends
	}
	// 好友上限
	nFrom, err := r.CountFriends(from)
	if err != nil {
		return nil, false, err
	}
	if nFrom >= models.MaxFriendsPerUser {
		return nil, false, ErrFriendLimit
	}
	nTo, err := r.CountFriends(to)
	if err != nil {
		return nil, false, err
	}
	if nTo >= models.MaxFriendsPerUser {
		return nil, false, ErrFriendLimit
	}

	var out *models.FriendRequest
	var autoAccepted bool
	err = r.db.Transaction(func(tx *gorm.DB) error {
		sr := r.WithTx(tx)
		// 事务内再查屏蔽（block race）
		blocked, e := sr.IsBlockedEither(from, to)
		if e != nil {
			return e
		}
		if blocked {
			return ErrBlocked
		}
		// 反向 pending：交叉邀请 → 自动接受
		var reverse models.FriendRequest
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("from_user_key = ? AND to_user_key = ? AND status = ?", to, from, models.FriendRequestPending).
			First(&reverse).Error
		if e == nil {
			now := time.Now().UTC()
			if err := tx.Model(&reverse).Updates(map[string]interface{}{
				"status":      models.FriendRequestAccepted,
				"resolved_at": now,
			}).Error; err != nil {
				return err
			}
			if err := sr.upsertFriendship(from, to); err != nil {
				return err
			}
			// 取消同 pair 其他 pending
			_ = tx.Model(&models.FriendRequest{}).
				Where("pair_key = ? AND status = ? AND request_id <> ?", reverse.PairKey, models.FriendRequestPending, reverse.RequestID).
				Updates(map[string]interface{}{
					"status":      models.FriendRequestCancelled,
					"resolved_at": now,
				}).Error
			reverse.Status = models.FriendRequestAccepted
			reverse.ResolvedAt = &now
			out = &reverse
			autoAccepted = true
			return nil
		}
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		// 已有同向 pending → 幂等返回
		var existing models.FriendRequest
		e = tx.Where("from_user_key = ? AND to_user_key = ? AND status = ?", from, to, models.FriendRequestPending).
			First(&existing).Error
		if e == nil {
			out = &existing
			return nil
		}
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		req := models.FriendRequest{
			RequestID:   uuid.NewString(),
			FromUserKey: from,
			ToUserKey:   to,
			PairKey:     PairKey(from, to),
			Status:      models.FriendRequestPending,
		}
		if err := tx.Create(&req).Error; err != nil {
			return err
		}
		out = &req
		return nil
	})
	return out, autoAccepted, err
}

func (r *SocialRepo) upsertFriendship(a, b string) error {
	low, high := OrderedPair(a, b)
	var f models.Friendship
	err := r.db.Where("user_low = ? AND user_high = ?", low, high).First(&f).Error
	if err == nil {
		if f.Status == models.FriendshipActive {
			return nil
		}
		return r.db.Model(&f).Updates(map[string]interface{}{
			"status":     models.FriendshipActive,
			"removed_at": nil,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(&models.Friendship{
		UserLow:  low,
		UserHigh: high,
		Status:   models.FriendshipActive,
	}).Error
}

// AcceptFriendRequest 接受请求（幂等：已接受则返回）。
func (r *SocialRepo) AcceptFriendRequest(actor, requestID string) (*models.FriendRequest, error) {
	if r == nil || r.db == nil {
		return nil, ErrSocialUnavailable
	}
	var out *models.FriendRequest
	err := r.db.Transaction(func(tx *gorm.DB) error {
		sr := r.WithTx(tx)
		var req models.FriendRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", requestID).First(&req).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRequestNotFound
			}
			return err
		}
		if req.ToUserKey != actor {
			return ErrRequestUnauthorized
		}
		if req.Status == models.FriendRequestAccepted {
			out = &req
			return nil // 幂等
		}
		if req.Status != models.FriendRequestPending {
			return ErrRequestNotPending
		}
		// block 优先
		blocked, err := sr.IsBlockedEither(req.FromUserKey, req.ToUserKey)
		if err != nil {
			return err
		}
		if blocked {
			return ErrBlocked
		}
		// 社交开关
		if err := sr.requireSocialActive(req.FromUserKey, req.ToUserKey); err != nil {
			return err
		}
		nFrom, err := sr.CountFriends(req.FromUserKey)
		if err != nil {
			return err
		}
		nTo, err := sr.CountFriends(req.ToUserKey)
		if err != nil {
			return err
		}
		if nFrom >= models.MaxFriendsPerUser || nTo >= models.MaxFriendsPerUser {
			return ErrFriendLimit
		}
		now := time.Now().UTC()
		if err := tx.Model(&req).Updates(map[string]interface{}{
			"status":      models.FriendRequestAccepted,
			"resolved_at": now,
		}).Error; err != nil {
			return err
		}
		if err := sr.upsertFriendship(req.FromUserKey, req.ToUserKey); err != nil {
			return err
		}
		req.Status = models.FriendRequestAccepted
		req.ResolvedAt = &now
		out = &req
		return nil
	})
	return out, err
}

// RejectFriendRequest 拒绝。
func (r *SocialRepo) RejectFriendRequest(actor, requestID string) (*models.FriendRequest, error) {
	return r.resolveRequest(actor, requestID, true, models.FriendRequestRejected)
}

// CancelFriendRequest 发起方取消。
func (r *SocialRepo) CancelFriendRequest(actor, requestID string) (*models.FriendRequest, error) {
	return r.resolveRequest(actor, requestID, false, models.FriendRequestCancelled)
}

func (r *SocialRepo) resolveRequest(actor, requestID string, asReceiver bool, status string) (*models.FriendRequest, error) {
	if r == nil || r.db == nil {
		return nil, ErrSocialUnavailable
	}
	var out *models.FriendRequest
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var req models.FriendRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", requestID).First(&req).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRequestNotFound
			}
			return err
		}
		if asReceiver {
			if req.ToUserKey != actor {
				return ErrRequestUnauthorized
			}
		} else {
			if req.FromUserKey != actor {
				return ErrRequestUnauthorized
			}
		}
		if req.Status == status {
			out = &req
			return nil
		}
		if req.Status != models.FriendRequestPending {
			// 已接受后再 reject/cancel 不允许；但重复 reject 已在上面处理
			if req.Status == models.FriendRequestAccepted && status == models.FriendRequestRejected {
				return ErrRequestNotPending
			}
			if req.Status != models.FriendRequestPending {
				return ErrRequestNotPending
			}
		}
		now := time.Now().UTC()
		if err := tx.Model(&req).Updates(map[string]interface{}{
			"status":      status,
			"resolved_at": now,
		}).Error; err != nil {
			return err
		}
		req.Status = status
		req.ResolvedAt = &now
		out = &req
		return nil
	})
	return out, err
}

// RemoveFriend 解除好友（幂等）。
func (r *SocialRepo) RemoveFriend(actor, other string) error {
	if r == nil || r.db == nil {
		return ErrSocialUnavailable
	}
	if actor == other || other == "" {
		return ErrInvalidTarget
	}
	low, high := OrderedPair(actor, other)
	now := time.Now().UTC()
	res := r.db.Model(&models.Friendship{}).
		Where("user_low = ? AND user_high = ? AND status = ?", low, high, models.FriendshipActive).
		Updates(map[string]interface{}{
			"status":     models.FriendshipRemoved,
			"removed_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	return nil
}

// ListRequests 列表（incoming|outgoing），仅 pending，cursor=id。
func (r *SocialRepo) ListRequests(userKey, direction string, afterID uint, limit int) ([]models.FriendRequest, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows []models.FriendRequest
	q := r.db.Where("status = ?", models.FriendRequestPending)
	switch direction {
	case "outgoing":
		q = q.Where("from_user_key = ?", userKey)
	default:
		q = q.Where("to_user_key = ?", userKey)
	}
	if afterID > 0 {
		q = q.Where("id > ?", afterID)
	}
	err := q.Order("id asc").Limit(limit).Find(&rows).Error
	return rows, err
}

// BlockUser 屏蔽：写入 block、解除好友、取消双向 pending 请求。幂等。
func (r *SocialRepo) BlockUser(blocker, blocked string) error {
	if r == nil || r.db == nil {
		return ErrSocialUnavailable
	}
	blocker, blocked = strings.TrimSpace(blocker), strings.TrimSpace(blocked)
	if blocker == "" || blocked == "" {
		return ErrInvalidTarget
	}
	if blocker == blocked {
		return ErrSelfAction
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 幂等 upsert block
		var existing models.SocialBlock
		err := tx.Where("blocker_user_key = ? AND blocked_user_key = ?", blocker, blocked).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&models.SocialBlock{
				BlockerUserKey: blocker,
				BlockedUserKey: blocked,
			}).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		// 解除好友
		low, high := OrderedPair(blocker, blocked)
		now := time.Now().UTC()
		_ = tx.Model(&models.Friendship{}).
			Where("user_low = ? AND user_high = ? AND status = ?", low, high, models.FriendshipActive).
			Updates(map[string]interface{}{
				"status":     models.FriendshipRemoved,
				"removed_at": now,
			}).Error
		// 取消双向 pending
		_ = tx.Model(&models.FriendRequest{}).
			Where("status = ? AND ((from_user_key = ? AND to_user_key = ?) OR (from_user_key = ? AND to_user_key = ?))",
				models.FriendRequestPending, blocker, blocked, blocked, blocker).
			Updates(map[string]interface{}{
				"status":      models.FriendRequestCancelled,
				"resolved_at": now,
			}).Error
		return nil
	})
}

// UnblockUser 取消屏蔽（幂等）。
func (r *SocialRepo) UnblockUser(blocker, blocked string) error {
	if r == nil || r.db == nil {
		return ErrSocialUnavailable
	}
	return r.db.Where("blocker_user_key = ? AND blocked_user_key = ?", blocker, blocked).
		Delete(&models.SocialBlock{}).Error
}

// ListBlocks 列出我屏蔽的人。
func (r *SocialRepo) ListBlocks(blocker string, afterID uint, limit int) ([]models.SocialBlock, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows []models.SocialBlock
	q := r.db.Where("blocker_user_key = ?", blocker)
	if afterID > 0 {
		q = q.Where("id > ?", afterID)
	}
	err := q.Order("id asc").Limit(limit).Find(&rows).Error
	return rows, err
}

// MuteUser 静音（幂等）。
func (r *SocialRepo) MuteUser(muter, muted string) error {
	if r == nil || r.db == nil {
		return ErrSocialUnavailable
	}
	if muter == muted || muted == "" {
		return ErrSelfAction
	}
	var existing models.SocialMute
	err := r.db.Where("muter_user_key = ? AND muted_user_key = ?", muter, muted).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(&models.SocialMute{MuterUserKey: muter, MutedUserKey: muted}).Error
}

// UnmuteUser 取消静音。
func (r *SocialRepo) UnmuteUser(muter, muted string) error {
	if r == nil || r.db == nil {
		return ErrSocialUnavailable
	}
	return r.db.Where("muter_user_key = ? AND muted_user_key = ?", muter, muted).
		Delete(&models.SocialMute{}).Error
}

// CreateUserReport 用户举报；短时限流由 handler 控制。
func (r *SocialRepo) CreateUserReport(reporter, target, category, note string) (*models.SocialUserReport, error) {
	if r == nil || r.db == nil {
		return nil, ErrSocialUnavailable
	}
	if reporter == target {
		return nil, ErrSelfAction
	}
	cat := strings.ToLower(strings.TrimSpace(category))
	switch cat {
	case "harassment", "spam", "inappropriate", "other":
	default:
		return nil, ErrReportInvalid
	}
	if utf8.RuneCountInString(note) > 500 {
		note = string([]rune(note)[:500])
	}
	// 拒绝疑似图片 payload
	if looksLikeBinaryNote(note) {
		return nil, ErrReportInvalid
	}
	rep := &models.SocialUserReport{
		ReportID:        uuid.NewString(),
		ReporterUserKey: reporter,
		TargetUserKey:   target,
		Category:        cat,
		Note:            strings.TrimSpace(note),
		Status:          "open",
	}
	if err := r.db.Create(rep).Error; err != nil {
		return nil, err
	}
	return rep, nil
}

func looksLikeBinaryNote(s string) bool {
	if strings.Contains(s, "base64,") || strings.Contains(s, "data:image") {
		return true
	}
	// 高比例非打印字符
	if len(s) > 32 {
		nonPrint := 0
		for _, r := range s {
			if r < 32 && r != '\n' && r != '\t' {
				nonPrint++
			}
		}
		if nonPrint > len(s)/4 {
			return true
		}
	}
	return false
}

// SearchProfiles 按展示名前缀搜索；排除自己与双向屏蔽。
func (r *SocialRepo) SearchProfiles(actor, q string, limit int) ([]models.SocialProfile, error) {
	if r == nil || r.db == nil {
		return nil, ErrSocialUnavailable
	}
	q = strings.TrimSpace(q)
	if q == "" || utf8.RuneCountInString(q) < 1 {
		return nil, nil
	}
	if utf8.RuneCountInString(q) > 32 {
		q = string([]rune(q)[:32])
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var rows []models.SocialProfile
	err := r.db.Where("social_enabled = ? AND user_key <> ? AND display_name LIKE ?", true, actor, q+"%").
		Order("id asc").Limit(limit * 2).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]models.SocialProfile, 0, limit)
	for _, p := range rows {
		blocked, err := r.IsBlockedEither(actor, p.UserKey)
		if err != nil {
			return nil, err
		}
		if blocked {
			continue
		}
		out = append(out, p)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// CreateShare 创建安全分享；返回明文 token（仅此一次在响应中暴露）。
func (r *SocialRepo) CreateShare(ownerKey string, animal *models.Animal, acl string, ttl time.Duration) (*models.SocialShare, string, error) {
	if r == nil || r.db == nil {
		return nil, "", ErrSocialUnavailable
	}
	if animal == nil {
		return nil, "", ErrAnimalNotFound
	}
	// 归属：account 优先，否则 device
	ownerOK := false
	if animal.AccountID != "" && strings.HasPrefix(ownerKey, "acc:") {
		ownerOK = ownerKey == "acc:"+animal.AccountID
	}
	if !ownerOK && animal.DeviceID != "" {
		ownerOK = ownerKey == "dev:"+animal.DeviceID || ownerKey == OwnerKey(animal.AccountID, animal.DeviceID)
	}
	// 也允许 ownerKey 与 SubjectKey(animal.AccountID, animal.DeviceID) 一致
	if !ownerOK {
		ownerOK = ownerKey == OwnerKey(animal.AccountID, animal.DeviceID)
	}
	if !ownerOK {
		return nil, "", ErrAnimalNotOwned
	}
	acl = strings.ToLower(strings.TrimSpace(acl))
	if acl == "" {
		acl = models.ShareACLLink
	}
	switch acl {
	case models.ShareACLLink, models.ShareACLFriends, models.ShareACLPublic:
	default:
		return nil, "", ErrInvalidACL
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	if ttl > 30*24*time.Hour {
		ttl = 30 * 24 * time.Hour
	}
	token, err := newShareToken()
	if err != nil {
		return nil, "", err
	}
	// 快照：无精确坐标、无 device_id
	share := &models.SocialShare{
		ShareToken:     token,
		OwnerUserKey:   ownerKey,
		AnimalUUID:     animal.UUID,
		ACL:            acl,
		Species:        animal.Species,
		SpeciesLabelZH: animal.SpeciesLabelZH,
		Breed:          animal.Breed,
		Rarity:         animal.Rarity,
		Nickname:       animal.Nickname,
		City:           animal.City, // 粗粒度
		ExpiresAt:      time.Now().UTC().Add(ttl),
	}
	if err := r.db.Create(share).Error; err != nil {
		return nil, "", err
	}
	return share, token, nil
}

func newShareToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// 43 字符 URL-safe，不可枚举
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GetShareByToken 按 capability token 查找。
func (r *SocialRepo) GetShareByToken(token string) (*models.SocialShare, error) {
	if r == nil || r.db == nil {
		return nil, ErrSocialUnavailable
	}
	token = strings.TrimSpace(token)
	if token == "" || len(token) < 16 {
		return nil, ErrShareNotFound
	}
	var s models.SocialShare
	if err := r.db.Where("share_token = ?", token).First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrShareNotFound
		}
		return nil, err
	}
	return &s, nil
}

// AccessShare 校验 ACL / 过期 / 撤销 / 屏蔽后返回分享。
func (r *SocialRepo) AccessShare(viewerKey, token string) (*models.SocialShare, error) {
	s, err := r.GetShareByToken(token)
	if err != nil {
		return nil, err
	}
	if s.RevokedAt != nil {
		return nil, ErrShareRevoked
	}
	if time.Now().UTC().After(s.ExpiresAt) {
		return nil, ErrShareExpired
	}
	// 屏蔽优先
	if viewerKey != "" && viewerKey != s.OwnerUserKey {
		blocked, err := r.IsBlockedEither(viewerKey, s.OwnerUserKey)
		if err != nil {
			return nil, err
		}
		if blocked {
			return nil, ErrShareForbidden
		}
	}
	switch s.ACL {
	case models.ShareACLLink:
		// token 即授权
		return s, nil
	case models.ShareACLPublic:
		if viewerKey == "" {
			return nil, ErrShareForbidden
		}
		return s, nil
	case models.ShareACLFriends:
		if viewerKey == "" {
			return nil, ErrShareForbidden
		}
		if viewerKey == s.OwnerUserKey {
			return s, nil
		}
		ok, err := r.AreFriends(viewerKey, s.OwnerUserKey)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrShareForbidden
		}
		return s, nil
	default:
		return nil, ErrShareForbidden
	}
}

// RevokeShare 所有者撤销；返回是否已撤销。
func (r *SocialRepo) RevokeShare(ownerKey, token string) (*models.SocialShare, error) {
	s, err := r.GetShareByToken(token)
	if err != nil {
		return nil, err
	}
	if s.OwnerUserKey != ownerKey {
		return nil, ErrShareForbidden
	}
	if s.RevokedAt != nil {
		return s, nil // 幂等
	}
	now := time.Now().UTC()
	if err := r.db.Model(s).Updates(map[string]interface{}{
		"revoked_at": now,
	}).Error; err != nil {
		return nil, err
	}
	s.RevokedAt = &now
	return s, nil
}

// ProfilesByKeys 批量取资料。
func (r *SocialRepo) ProfilesByKeys(keys []string) (map[string]models.SocialProfile, error) {
	out := make(map[string]models.SocialProfile, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	// 去重
	uniq := make([]string, 0, len(keys))
	seen := map[string]struct{}{}
	for _, k := range keys {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		uniq = append(uniq, k)
	}
	sort.Strings(uniq)
	var rows []models.SocialProfile
	if err := r.db.Where("user_key IN ?", uniq).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, p := range rows {
		out[p.UserKey] = p
	}
	return out, nil
}

// CountRecentReports 限流：统计窗口内举报数。
func (r *SocialRepo) CountRecentReports(reporter string, since time.Time) (int64, error) {
	var n int64
	err := r.db.Model(&models.SocialUserReport{}).
		Where("reporter_user_key = ? AND created_at >= ?", reporter, since).
		Count(&n).Error
	return n, err
}

// SharePublicView 客户端安全视图（无 token 明文、无设备/坐标）。
func SharePublicView(s *models.SocialShare) map[string]interface{} {
	if s == nil {
		return map[string]interface{}{}
	}
	m := map[string]interface{}{
		"animal_uuid":      s.AnimalUUID,
		"owner_user_key":   s.OwnerUserKey,
		"acl":              s.ACL,
		"species":          s.Species,
		"species_label_zh": s.SpeciesLabelZH,
		"breed":            s.Breed,
		"rarity":           s.Rarity,
		"nickname":         s.Nickname,
		"city":             s.City,
		"expires_at":       s.ExpiresAt.UTC().Format(time.RFC3339),
		"created_at":       s.CreatedAt.UTC().Format(time.RFC3339),
		"revoked":          s.RevokedAt != nil,
	}
	// 明确不包含：latitude/longitude/device_id/share_token/precise
	return m
}

// FormatCursor 简单数字游标。
func FormatCursor(id uint) string {
	if id == 0 {
		return ""
	}
	return fmt.Sprintf("%d", id)
}

// ParseCursor 解析游标。
func ParseCursor(raw string) (uint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	var n uint64
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid cursor")
		}
		n = n*10 + uint64(c-'0')
		if n > 1<<32 {
			return 0, fmt.Errorf("invalid cursor")
		}
	}
	return uint(n), nil
}
