package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SessionStore 管理端会话：签发、查询、撤销（支持并发撤权）。
type SessionStore struct {
	mu       sync.RWMutex
	mem      map[string]*models.AdminSession // session_id → session
	db       *gorm.DB
	// RevokeGrace 撤权后仍可完成的宽限窗口（默认 0：立即失效）。
	RevokeGrace time.Duration
}

// NewSessionStore 构造；db 可为 nil（纯内存，测试/无 DB）。
func NewSessionStore(db *gorm.DB) *SessionStore {
	return &SessionStore{
		mem: make(map[string]*models.AdminSession),
		db:  db,
	}
}

// Create 创建活跃会话。
func (s *SessionStore) Create(actorID, subject, role, env, authMode string, ttl time.Duration) (*models.AdminSession, error) {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	now := time.Now().UTC()
	sess := &models.AdminSession{
		SessionID: uuid.NewString(),
		ActorID:   actorID,
		Subject:   subject,
		Role:      NormalizeRole(role),
		Env:       env,
		AuthMode:  authMode,
		Status:    "active",
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.mu.Lock()
	s.mem[sess.SessionID] = cloneSession(sess)
	s.mu.Unlock()
	if s.db != nil {
		if err := s.db.Create(sess).Error; err != nil {
			return nil, err
		}
	}
	return sess, nil
}

// Get 查询会话。
func (s *SessionStore) Get(sessionID string) (*models.AdminSession, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("empty session id")
	}
	s.mu.RLock()
	if mem, ok := s.mem[sessionID]; ok {
		out := cloneSession(mem)
		s.mu.RUnlock()
		return out, nil
	}
	s.mu.RUnlock()
	if s.db == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var row models.AdminSession
	if err := s.db.Where("session_id = ?", sessionID).First(&row).Error; err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.mem[sessionID] = cloneSession(&row)
	s.mu.Unlock()
	return &row, nil
}

// IsActive 会话是否仍有效（未过期、未撤销或在宽限窗口内）。
func (s *SessionStore) IsActive(sessionID string, now time.Time) (bool, error) {
	sess, err := s.Get(sessionID)
	if err != nil {
		return false, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if now.After(sess.ExpiresAt) {
		return false, nil
	}
	if sess.Status == "active" && sess.RevokedAt == nil {
		return true, nil
	}
	// 撤权后宽限窗口
	if sess.RevokedAt != nil && s.RevokeGrace > 0 {
		deadline := sess.RevokedAt.Add(s.RevokeGrace)
		if now.Before(deadline) || now.Equal(deadline) {
			return true, nil
		}
	}
	return false, nil
}

// Revoke 撤销单会话。
func (s *SessionStore) Revoke(sessionID, by string) error {
	now := time.Now().UTC()
	s.mu.Lock()
	if mem, ok := s.mem[sessionID]; ok {
		mem.Status = "revoked"
		mem.RevokedAt = &now
		mem.RevokedBy = by
		mem.UpdatedAt = now
	}
	s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	return s.db.Model(&models.AdminSession{}).Where("session_id = ?", sessionID).Updates(map[string]interface{}{
		"status":     "revoked",
		"revoked_at": now,
		"revoked_by": by,
		"updated_at": now,
	}).Error
}

// RevokeAllForActor 撤销某 actor 的全部活跃会话。
func (s *SessionStore) RevokeAllForActor(actorID, by string) (int, error) {
	now := time.Now().UTC()
	n := 0
	s.mu.Lock()
	for _, mem := range s.mem {
		if mem.ActorID == actorID && mem.Status == "active" {
			mem.Status = "revoked"
			mem.RevokedAt = &now
			mem.RevokedBy = by
			mem.UpdatedAt = now
			n++
		}
	}
	s.mu.Unlock()
	if s.db == nil {
		return n, nil
	}
	res := s.db.Model(&models.AdminSession{}).
		Where("actor_id = ? AND status = ?", actorID, "active").
		Updates(map[string]interface{}{
			"status":     "revoked",
			"revoked_at": now,
			"revoked_by": by,
			"updated_at": now,
		})
	if res.Error != nil {
		return n, res.Error
	}
	if int(res.RowsAffected) > n {
		n = int(res.RowsAffected)
	}
	return n, nil
}

func cloneSession(in *models.AdminSession) *models.AdminSession {
	if in == nil {
		return nil
	}
	out := *in
	if in.RevokedAt != nil {
		t := *in.RevokedAt
		out.RevokedAt = &t
	}
	return &out
}

// ActionAuditor 管理动作审计（含防篡改 HMAC）。
type ActionAuditor struct {
	db     *gorm.DB
	secret []byte
}

// NewActionAuditor 构造。
func NewActionAuditor(db *gorm.DB, secret string) *ActionAuditor {
	return &ActionAuditor{db: db, secret: []byte(secret)}
}

// ActionInput 写入审计所需字段。
type ActionInput struct {
	Actor     Actor
	Action    string
	Resource  string
	Reason    string
	RequestID string
	Outcome   string // allow|deny|error|ok
	Metadata  map[string]any
}

// Record 写入不可变管理动作审计。
func (a *ActionAuditor) Record(in ActionInput) (*models.AdminActionLog, error) {
	if a == nil {
		return nil, fmt.Errorf("auditor nil")
	}
	now := time.Now().UTC()
	metaJSON := ""
	if in.Metadata != nil {
		b, err := json.Marshal(in.Metadata)
		if err == nil {
			metaJSON = string(b)
		}
	}
	if in.Outcome == "" {
		in.Outcome = "ok"
	}
	row := &models.AdminActionLog{
		EntryID:   uuid.NewString(),
		ActorID:   in.Actor.ActorID,
		Subject:   in.Actor.Subject,
		Role:      NormalizeRole(in.Actor.Role),
		SessionID: in.Actor.SessionID,
		AuthMode:  in.Actor.AuthMode,
		Action:    in.Action,
		Resource:  in.Resource,
		Reason:    in.Reason,
		RequestID: in.RequestID,
		Outcome:   in.Outcome,
		Metadata:  metaJSON,
		Env:       in.Actor.Env,
		CreatedAt: now,
	}
	row.Integrity = a.sign(row)
	if a.db != nil {
		if err := a.db.Create(row).Error; err != nil {
			return nil, err
		}
	}
	return row, nil
}

// VerifyIntegrity 校验防篡改 HMAC。
func (a *ActionAuditor) VerifyIntegrity(row *models.AdminActionLog) bool {
	if a == nil || row == nil {
		return false
	}
	return hmac.Equal([]byte(row.Integrity), []byte(a.sign(row)))
}

func (a *ActionAuditor) sign(row *models.AdminActionLog) string {
	// 字段顺序固定；不含 Integrity 自身。
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%d",
		row.EntryID, row.ActorID, row.Subject, row.Role, row.SessionID, row.AuthMode,
		row.Action, row.Resource, row.Reason, row.RequestID, row.Outcome,
		row.CreatedAt.UTC().UnixNano(),
	)
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
