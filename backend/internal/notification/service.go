// Package notification implements AP-084 inbox, outbox and push preferences.
package notification

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"animalpoke/backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PushProvider delivers push notifications (mockable).
type PushProvider interface {
	Send(token, title, body string) error
}

// MockProvider records sends; RateLimitN triggers 429-like errors for first N calls.
type MockProvider struct {
	mu        sync.Mutex
	Sends     []string
	FailTimes int
	calls     int
}

func (m *MockProvider) Send(token, title, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.calls <= m.FailTimes {
		return errProvider429
	}
	m.Sends = append(m.Sends, token+"|"+title)
	return nil
}

var errProvider429 = errors.New("provider 429")

// Service owns inbox/outbox operations.
type Service struct {
	db       *gorm.DB
	provider PushProvider
	now      func() time.Time
}

// NewService creates a notification service.
func NewService(db *gorm.DB, provider PushProvider) *Service {
	if provider == nil {
		provider = &MockProvider{}
	}
	return &Service{db: db, provider: provider, now: func() time.Time { return time.Now().UTC() }}
}

// SetClock overrides time (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Enqueue creates inbox + outbox atomically with dedupe.
func (s *Service) Enqueue(ownerKey, category, title, body, dedupeKey string, sensitive bool) (*models.InboxMessage, error) {
	if ownerKey == "" || category == "" || title == "" || dedupeKey == "" {
		return nil, errors.New("owner/category/title/dedupe required")
	}
	if !validCategory(category) {
		return nil, fmt.Errorf("invalid category %q", category)
	}
	var msg models.InboxMessage
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Exactly-once inbox display: unique dedupe_key.
		var existing models.InboxMessage
		if err := tx.Where("dedupe_key = ?", dedupeKey).First(&existing).Error; err == nil {
			msg = existing
			return nil
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		msg = models.InboxMessage{
			OwnerKey:  ownerKey,
			Category:  category,
			Title:     title,
			Body:      body,
			DedupeKey: dedupeKey,
			Sensitive: sensitive,
			CreatedAt: s.now(),
		}
		if err := tx.Create(&msg).Error; err != nil {
			// concurrent insert race → load winner
			if err2 := tx.Where("dedupe_key = ?", dedupeKey).First(&msg).Error; err2 == nil {
				return nil
			}
			return err
		}
		out := models.NotificationOutbox{
			OwnerKey:      ownerKey,
			InboxID:       msg.ID,
			Category:      category,
			DedupeKey:     "outbox:" + dedupeKey,
			State:         models.OutboxPending,
			NextAttemptAt: s.now(),
			CreatedAt:     s.now(),
			UpdatedAt:     s.now(),
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&out).Error
	})
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// ListInbox returns messages after cursor (created_at,id), limit.
func (s *Service) ListInbox(ownerKey string, afterID uint, limit int) ([]models.InboxMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := s.db.Where("owner_key = ?", ownerKey).Order("id asc").Limit(limit)
	if afterID > 0 {
		q = q.Where("id > ?", afterID)
	}
	var rows []models.InboxMessage
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// MarkRead sets read_at once.
func (s *Service) MarkRead(ownerKey string, id uint) error {
	now := s.now()
	res := s.db.Model(&models.InboxMessage{}).
		Where("id = ? AND owner_key = ? AND read_at IS NULL", id, ownerKey).
		Update("read_at", now)
	return res.Error
}

// Ack sets acked_at once.
func (s *Service) Ack(ownerKey string, id uint) error {
	now := s.now()
	res := s.db.Model(&models.InboxMessage{}).
		Where("id = ? AND owner_key = ? AND acked_at IS NULL", id, ownerKey).
		Update("acked_at", now)
	return res.Error
}

// UpsertPushToken registers/refreshes a device push token.
func (s *Service) UpsertPushToken(ownerKey, token, platform string) error {
	if ownerKey == "" || token == "" {
		return errors.New("owner and token required")
	}
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		platform = "web"
	}
	now := s.now()
	var row models.PushDeviceToken
	err := s.db.Where("owner_key = ? AND token = ?", ownerKey, token).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return s.db.Create(&models.PushDeviceToken{
			OwnerKey: ownerKey, Token: token, Platform: platform, Active: true, LastSeenAt: now, CreatedAt: now,
		}).Error
	}
	if err != nil {
		return err
	}
	return s.db.Model(&row).Updates(map[string]interface{}{
		"active": true, "platform": platform, "last_seen_at": now, "disabled_at": nil,
	}).Error
}

// DisablePushToken marks token inactive (invalid provider response).
func (s *Service) DisablePushToken(token string) error {
	now := s.now()
	return s.db.Model(&models.PushDeviceToken{}).Where("token = ?", token).
		Updates(map[string]interface{}{"active": false, "disabled_at": now}).Error
}

// GetPrefs returns preferences with defaults.
func (s *Service) GetPrefs(ownerKey string) (*models.NotificationPreference, error) {
	var p models.NotificationPreference
	err := s.db.Where("owner_key = ?", ownerKey).First(&p).Error
	if err == gorm.ErrRecordNotFound {
		p = models.NotificationPreference{
			OwnerKey: ownerKey, QuietStartHour: 22, QuietEndHour: 8, Timezone: "Asia/Shanghai",
			CreatedAt: s.now(), UpdatedAt: s.now(),
		}
		if err := s.db.Create(&p).Error; err != nil {
			return nil, err
		}
		return &p, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdatePrefs updates quiet hours / marketing / minor flags.
func (s *Service) UpdatePrefs(ownerKey string, marketing *bool, minor *bool, quietStart, quietEnd *int, tz *string) (*models.NotificationPreference, error) {
	p, err := s.GetPrefs(ownerKey)
	if err != nil {
		return nil, err
	}
	if marketing != nil {
		p.MarketingConsent = *marketing
	}
	if minor != nil {
		p.Minor = *minor
	}
	if quietStart != nil {
		p.QuietStartHour = *quietStart
	}
	if quietEnd != nil {
		p.QuietEndHour = *quietEnd
	}
	if tz != nil && strings.TrimSpace(*tz) != "" {
		if _, err := time.LoadLocation(*tz); err != nil {
			return nil, fmt.Errorf("invalid timezone")
		}
		p.Timezone = *tz
	}
	// Minors get stricter defaults.
	if p.Minor {
		if p.QuietStartHour > 21 {
			// keep
		} else {
			p.QuietStartHour = 21
		}
		if p.QuietEndHour < 8 {
			p.QuietEndHour = 8
		}
	}
	p.UpdatedAt = s.now()
	if err := s.db.Save(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

// ProcessOutbox delivers pending outbox rows (at-least-once push; inbox already durable).
func (s *Service) ProcessOutbox(limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []models.NotificationOutbox
	now := s.now()
	if err := s.db.Where("state IN ? AND next_attempt_at <= ?", []string{models.OutboxPending, models.OutboxFailed}, now).
		Order("id asc").Limit(limit).Find(&rows).Error; err != nil {
		return 0, err
	}
	done := 0
	for i := range rows {
		if err := s.deliverOne(&rows[i]); err == nil {
			done++
		}
	}
	return done, nil
}

func (s *Service) deliverOne(row *models.NotificationOutbox) error {
	prefs, _ := s.GetPrefs(row.OwnerKey)
	// Marketing must stop immediately when consent withdrawn.
	if row.Category == models.NotifCategoryMarketing && (prefs == nil || !prefs.MarketingConsent) {
		return s.db.Model(row).Updates(map[string]interface{}{
			"state": models.OutboxSkipped, "updated_at": s.now(), "last_error": "marketing_opt_out",
		}).Error
	}
	// Quiet hours: transactional/security still deliver; marketing deferred.
	if row.Category == models.NotifCategoryMarketing && prefs != nil && inQuietHours(s.now(), prefs) {
		next := nextQuietEnd(s.now(), prefs)
		return s.db.Model(row).Updates(map[string]interface{}{
			"state": models.OutboxPending, "next_attempt_at": next, "updated_at": s.now(), "last_error": "quiet_hours",
		}).Error
	}

	var inbox models.InboxMessage
	if err := s.db.First(&inbox, row.InboxID).Error; err != nil {
		return err
	}
	var tokens []models.PushDeviceToken
	if err := s.db.Where("owner_key = ? AND active = ?", row.OwnerKey, true).Find(&tokens).Error; err != nil {
		return err
	}
	if len(tokens) == 0 {
		// No tokens: mark delivered for inbox-only path (at-least-once satisfied by inbox).
		now := s.now()
		return s.db.Model(row).Updates(map[string]interface{}{
			"state": models.OutboxDelivered, "delivered_at": now, "updated_at": now,
		}).Error
	}

	_ = s.db.Model(row).Updates(map[string]interface{}{"state": models.OutboxSending, "updated_at": s.now()}).Error
	var lastErr error
	for _, tok := range tokens {
		if err := s.provider.Send(tok.Token, inbox.Title, inbox.Body); err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "gone") {
				_ = s.DisablePushToken(tok.Token)
			}
			continue
		}
	}
	if lastErr != nil {
		attempts := row.Attempts + 1
		backoff := time.Duration(attempts*attempts) * time.Second
		if backoff > 5*time.Minute {
			backoff = 5 * time.Minute
		}
		_ = s.db.Model(row).Updates(map[string]interface{}{
			"state": models.OutboxFailed, "attempts": attempts, "next_attempt_at": s.now().Add(backoff),
			"last_error": truncate(lastErr.Error(), 250), "updated_at": s.now(),
		}).Error
		return lastErr
	}
	now := s.now()
	return s.db.Model(row).Updates(map[string]interface{}{
		"state": models.OutboxDelivered, "delivered_at": now, "updated_at": now, "last_error": "",
	}).Error
}

func validCategory(c string) bool {
	switch c {
	case models.NotifCategorySecurity, models.NotifCategoryPrivacy, models.NotifCategoryRefund,
		models.NotifCategoryReport, models.NotifCategoryEvent, models.NotifCategoryMarketing:
		return true
	default:
		return false
	}
}

func inQuietHours(now time.Time, p *models.NotificationPreference) bool {
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	h := local.Hour()
	start, end := p.QuietStartHour, p.QuietEndHour
	if p.Minor {
		// Stricter: 21:00-08:00 minimum window for minors.
		if start > 21 {
			start = start
		} else {
			start = 21
		}
		if end < 8 {
			end = 8
		}
	}
	if start == end {
		return false
	}
	if start < end {
		return h >= start && h < end
	}
	// wraps midnight
	return h >= start || h < end
}

func nextQuietEnd(now time.Time, p *models.NotificationPreference) time.Time {
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	end := p.QuietEndHour
	if p.Minor && end < 8 {
		end = 8
	}
	next := time.Date(local.Year(), local.Month(), local.Day(), end, 0, 0, 0, loc)
	if !next.After(local) {
		next = next.Add(24 * time.Hour)
	}
	return next.UTC()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
