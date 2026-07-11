// Package repo — AP-096 任务进度 / 领取 / 事件 / 过期补偿。
package repo

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/questcatalog"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 任务领域错误。
var (
	ErrQuestRepoUnavailable = errors.New("quest repo unavailable")
	ErrQuestNotFound        = errors.New("quest_not_found")
	ErrQuestNotCompletable  = errors.New("quest_not_completable")
	ErrQuestNotClaimable    = errors.New("quest_not_claimable")
	ErrQuestExpired         = errors.New("quest_expired")
	ErrQuestPrereq          = errors.New("quest_prereq_unmet")
	ErrQuestEventForbidden  = errors.New("quest_event_forbidden")
	ErrQuestEventUntrusted  = errors.New("quest_event_untrusted")
	ErrQuestEventInvalid    = errors.New("quest_event_invalid")
	ErrQuestDisabled        = errors.New("quest_disabled")
)

// errEventIdempotent 事务内标记：event_id 已处理。
var errEventIdempotent = errors.New("quest_event_idempotent")

// QuestRepo 数据驱动任务仓储。
type QuestRepo struct {
	db     *gorm.DB
	wallet *WalletRepo
	// nowFn 可注入时钟（时区重置 / 过期测试）。
	nowFn func() time.Time
	// loc 重置时区，默认 Asia/Shanghai。
	loc *time.Location
}

// NewQuestRepo 构造。
func NewQuestRepo(db *gorm.DB, wallet *WalletRepo) *QuestRepo {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	return &QuestRepo{
		db:     db,
		wallet: wallet,
		nowFn:  func() time.Time { return time.Now().UTC() },
		loc:    loc,
	}
}

// SetNowFunc 测试注入时钟。
func (r *QuestRepo) SetNowFunc(fn func() time.Time) {
	if fn != nil {
		r.nowFn = fn
	}
}

// SetLocation 设置重置时区。
func (r *QuestRepo) SetLocation(loc *time.Location) {
	if loc != nil {
		r.loc = loc
	}
}

// DB 暴露底层连接。
func (r *QuestRepo) DB() *gorm.DB { return r.db }

// SeedDefinitions 将目录写入 quest_definitions（幂等 upsert by quest_id）。
func (r *QuestRepo) SeedDefinitions() error {
	if r == nil || r.db == nil {
		return ErrQuestRepoUnavailable
	}
	if err := questcatalog.ValidateGraph(); err != nil {
		return err
	}
	for _, d := range questcatalog.All() {
		row := models.QuestDefinition{
			QuestID:           d.QuestID,
			Type:              d.Type,
			Title:             d.Title,
			Description:       d.Description,
			ObjectivesJSON:    questcatalog.ObjectivesJSON(d.Objectives),
			RewardsJSON:       questcatalog.RewardsJSON(d.Rewards),
			PrerequisitesJSON: questcatalog.PrerequisitesJSON(d.Prerequisites),
			ResetPolicy:       d.ResetPolicy,
			Free:              d.Free,
			Enabled:           true,
			ConfigVersion:     questcatalog.ConfigVersion,
			MinLevel:          d.MinLevel,
			SortOrder:         d.SortOrder,
			DurationHours:     d.DurationHours,
			StartsAt:          d.StartsAt,
			EndsAt:            d.EndsAt,
		}
		var existing models.QuestDefinition
		err := r.db.Where("quest_id = ?", d.QuestID).First(&existing).Error
		if err == nil {
			// 配置回滚：仅当版本不同时更新内容，保留主键。
			if existing.ConfigVersion == questcatalog.ConfigVersion &&
				existing.ObjectivesJSON == row.ObjectivesJSON &&
				existing.RewardsJSON == row.RewardsJSON {
				continue
			}
			row.ID = existing.ID
			row.CreatedAt = existing.CreatedAt
			if err := r.db.Save(&row).Error; err != nil {
				return err
			}
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := r.db.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// ListDefinitions 列出启用定义。
func (r *QuestRepo) ListDefinitions(includeDisabled bool) ([]models.QuestDefinition, error) {
	if r == nil || r.db == nil {
		return nil, ErrQuestRepoUnavailable
	}
	var rows []models.QuestDefinition
	q := r.db.Order("sort_order asc, quest_id asc")
	if !includeDisabled {
		q = q.Where("enabled = ?", true)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetDefinition 按 quest_id 取定义。
func (r *QuestRepo) GetDefinition(questID string) (*models.QuestDefinition, error) {
	if r == nil || r.db == nil {
		return nil, ErrQuestRepoUnavailable
	}
	var row models.QuestDefinition
	err := r.db.Where("quest_id = ?", questID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrQuestNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// PeriodKey 按重置策略与时区计算 period。
func (r *QuestRepo) PeriodKey(resetPolicy string, now time.Time) string {
	loc := r.loc
	if loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)
	switch resetPolicy {
	case models.QuestResetDaily:
		return local.Format("2006-01-02")
	case models.QuestResetWeekly:
		y, w := local.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w)
	default:
		return "once"
	}
}

// periodEnd 返回 period 结束时刻（UTC）。
func (r *QuestRepo) periodEnd(resetPolicy, periodKey string) *time.Time {
	loc := r.loc
	if loc == nil {
		loc = time.UTC
	}
	switch resetPolicy {
	case models.QuestResetDaily:
		t, err := time.ParseInLocation("2006-01-02", periodKey, loc)
		if err != nil {
			return nil
		}
		end := t.Add(24 * time.Hour).UTC()
		return &end
	case models.QuestResetWeekly:
		// periodKey = YYYY-Www
		var y, w int
		if _, err := fmt.Sscanf(periodKey, "%d-W%d", &y, &w); err != nil {
			return nil
		}
		// ISO week: Monday of week
		jan4 := time.Date(y, 1, 4, 0, 0, 0, 0, loc)
		// weekday Mon=1
		wd := int(jan4.Weekday())
		if wd == 0 {
			wd = 7
		}
		week1Mon := jan4.AddDate(0, 0, -(wd - 1))
		start := week1Mon.AddDate(0, 0, (w-1)*7)
		end := start.AddDate(0, 0, 7).UTC()
		return &end
	default:
		return nil
	}
}

// QuestView 列表/详情视图。
type QuestView struct {
	Definition    models.QuestDefinition     `json:"definition"`
	Progress      *models.QuestProgress      `json:"progress,omitempty"`
	Objectives    []questcatalog.Objective   `json:"objectives"`
	Rewards       questcatalog.Reward        `json:"rewards"`
	Prerequisites []string                   `json:"prerequisites,omitempty"`
	Counters      map[string]int64           `json:"counters"`
	Claimable     bool                       `json:"claimable"`
	Free          bool                       `json:"free"`
}

// ListForOwner 列出任务及当前 period 进度。
func (r *QuestRepo) ListForOwner(accountID, deviceID string, freeOnly bool) ([]QuestView, error) {
	if r == nil || r.db == nil {
		return nil, ErrQuestRepoUnavailable
	}
	defs, err := r.ListDefinitions(false)
	if err != nil {
		return nil, err
	}
	owner := OwnerKey(accountID, deviceID)
	now := r.nowFn()
	out := make([]QuestView, 0, len(defs))
	for _, def := range defs {
		if freeOnly && !def.Free {
			continue
		}
		if def.StartsAt != nil && now.Before(def.StartsAt.UTC()) {
			continue
		}
		if def.EndsAt != nil && !now.Before(def.EndsAt.UTC()) {
			continue
		}
		period := r.PeriodKey(def.ResetPolicy, now)
		prog, err := r.getOrNilProgress(owner, def.QuestID, period)
		if err != nil {
			return nil, err
		}
		view, err := r.buildView(def, prog)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (r *QuestRepo) getOrNilProgress(owner, questID, period string) (*models.QuestProgress, error) {
	var p models.QuestProgress
	err := r.db.Where("owner_key = ? AND quest_id = ? AND period_key = ?", owner, questID, period).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *QuestRepo) buildView(def models.QuestDefinition, prog *models.QuestProgress) (QuestView, error) {
	var objs []questcatalog.Objective
	if err := json.Unmarshal([]byte(def.ObjectivesJSON), &objs); err != nil {
		return QuestView{}, err
	}
	var reward questcatalog.Reward
	if err := json.Unmarshal([]byte(def.RewardsJSON), &reward); err != nil {
		return QuestView{}, err
	}
	var prereq []string
	if def.PrerequisitesJSON != "" {
		_ = json.Unmarshal([]byte(def.PrerequisitesJSON), &prereq)
	}
	counters := map[string]int64{}
	if prog != nil && prog.ProgressJSON != "" {
		_ = json.Unmarshal([]byte(prog.ProgressJSON), &counters)
	}
	claimable := prog != nil && prog.Status == models.QuestStatusCompleted
	return QuestView{
		Definition:    def,
		Progress:      prog,
		Objectives:    objs,
		Rewards:       reward,
		Prerequisites: prereq,
		Counters:      counters,
		Claimable:     claimable,
		Free:          def.Free,
	}, nil
}

// ApplyEventRequest 可信事件请求。
type ApplyEventRequest struct {
	DeviceID  string
	AccountID string
	EventID   string
	EventType string
	Delta     int64 // 默认 1
	// Payload 可选 JSON 字符串（city/species 等）。
	Payload string
}

// ApplyEventResult 事件应用结果。
type ApplyEventResult struct {
	Idempotent bool     `json:"idempotent"`
	EventType  string   `json:"event_type"`
	Updated    []string `json:"updated_quest_ids"`
}

// ApplyEvent 幂等应用可信业务事件并推进进度。
// 拒绝 open_pokedex / safe_explore / page_view 等客户端可伪造事件。
func (r *QuestRepo) ApplyEvent(req ApplyEventRequest) (*ApplyEventResult, error) {
	if r == nil || r.db == nil {
		return nil, ErrQuestRepoUnavailable
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return nil, ErrInvalidOwner
	}
	eventID := strings.TrimSpace(req.EventID)
	if eventID == "" || len(eventID) > 128 {
		return nil, ErrQuestEventInvalid
	}
	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" {
		return nil, ErrQuestEventInvalid
	}
	if _, bad := questcatalog.ForbiddenEvents[eventType]; bad {
		return nil, ErrQuestEventForbidden
	}
	if !questcatalog.IsTrustedEvent(eventType) {
		return nil, ErrQuestEventUntrusted
	}
	delta := req.Delta
	if delta == 0 {
		delta = 1
	}
	if delta < 0 || delta > 1000 {
		return nil, ErrQuestEventInvalid
	}
	owner := OwnerKey(req.AccountID, deviceID)
	now := r.nowFn()

	// 幂等：event_id 已存在则直接返回
	var existing models.QuestEventLog
	err := r.db.Where("event_id = ?", eventID).First(&existing).Error
	if err == nil {
		return &ApplyEventResult{Idempotent: true, EventType: existing.EventType, Updated: nil}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	defs, err := r.ListDefinitions(false)
	if err != nil {
		return nil, err
	}

	updated := make([]string, 0)
	inserted := false
	err = r.db.Transaction(func(tx *gorm.DB) error {
		// 再次检查幂等（并发）
		var again models.QuestEventLog
		if err := tx.Where("event_id = ?", eventID).First(&again).Error; err == nil {
			return errEventIdempotent
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		log := models.QuestEventLog{
			EventID:   eventID,
			OwnerKey:  owner,
			DeviceID:  deviceID,
			AccountID: strings.TrimSpace(req.AccountID),
			EventType: eventType,
			Payload:   req.Payload,
			AppliedAt: now,
		}
		if err := tx.Create(&log).Error; err != nil {
			if isUniqueViolation(err) {
				return errEventIdempotent
			}
			return err
		}
		inserted = true

		for _, def := range defs {
			if def.StartsAt != nil && now.Before(def.StartsAt.UTC()) {
				continue
			}
			if def.EndsAt != nil && !now.Before(def.EndsAt.UTC()) {
				continue
			}
			var objs []questcatalog.Objective
			if err := json.Unmarshal([]byte(def.ObjectivesJSON), &objs); err != nil {
				return err
			}
			matches := false
			for _, o := range objs {
				if o.Event == eventType {
					matches = true
					break
				}
			}
			if !matches {
				continue
			}
			// 前置检查
			ok, err := r.prereqsMetTx(tx, owner, def)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			period := r.PeriodKey(def.ResetPolicy, now)
			prog, err := r.lockOrCreateProgress(tx, owner, deviceID, req.AccountID, def, period, now)
			if err != nil {
				return err
			}
			if prog.Status == models.QuestStatusClaimed ||
				prog.Status == models.QuestStatusCompensated ||
				prog.Status == models.QuestStatusExpired {
				continue
			}
			// 过期检查
			if prog.ExpiresAt != nil && !now.Before(prog.ExpiresAt.UTC()) {
				if prog.Status != models.QuestStatusExpired {
					prog.Status = models.QuestStatusExpired
					if err := tx.Save(prog).Error; err != nil {
						return err
					}
				}
				continue
			}

			counters := map[string]int64{}
			if prog.ProgressJSON != "" {
				_ = json.Unmarshal([]byte(prog.ProgressJSON), &counters)
			}
			changed := false
			for _, o := range objs {
				if o.Event != eventType {
					continue
				}
				// filter: 若目标带 filter.city 等，payload 需匹配
				if !filterMatch(o.Filter, req.Payload) {
					continue
				}
				cur := counters[o.ID]
				if cur >= o.Target {
					continue
				}
				next := cur + delta
				if next > o.Target {
					next = o.Target
				}
				counters[o.ID] = next
				changed = true
			}
			if !changed {
				continue
			}
			b, _ := json.Marshal(counters)
			prog.ProgressJSON = string(b)
			// 复合目标全部达标？
			allDone := true
			for _, o := range objs {
				if counters[o.ID] < o.Target {
					allDone = false
					break
				}
			}
			if allDone && prog.Status == models.QuestStatusActive {
				prog.Status = models.QuestStatusCompleted
				t := now
				prog.CompletedAt = &t
			}
			prog.UpdatedAt = now
			if err := tx.Save(prog).Error; err != nil {
				return err
			}
			updated = append(updated, def.QuestID)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errEventIdempotent) {
			return &ApplyEventResult{Idempotent: true, EventType: eventType, Updated: nil}, nil
		}
		return nil, err
	}
	_ = inserted
	return &ApplyEventResult{Idempotent: false, EventType: eventType, Updated: updated}, nil
}

func filterMatch(filter map[string]string, payload string) bool {
	if len(filter) == 0 {
		return true
	}
	if payload == "" {
		return false
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		// try generic map
		var any map[string]interface{}
		if err2 := json.Unmarshal([]byte(payload), &any); err2 != nil {
			return false
		}
		m = map[string]string{}
		for k, v := range any {
			m[k] = fmt.Sprint(v)
		}
	}
	for k, want := range filter {
		if m[k] != want {
			return false
		}
	}
	return true
}

func (r *QuestRepo) prereqsMetTx(tx *gorm.DB, owner string, def models.QuestDefinition) (bool, error) {
	if def.PrerequisitesJSON == "" || def.PrerequisitesJSON == "[]" {
		return true, nil
	}
	var prereq []string
	if err := json.Unmarshal([]byte(def.PrerequisitesJSON), &prereq); err != nil {
		return false, err
	}
	if len(prereq) == 0 {
		return true, nil
	}
	for _, p := range prereq {
		// 前置任务一旦 claimed 或 completed 即满足（跨 period 取最近成功）
		var cnt int64
		err := tx.Model(&models.QuestProgress{}).
			Where("owner_key = ? AND quest_id = ? AND status IN ?", owner, p,
				[]string{models.QuestStatusCompleted, models.QuestStatusClaimed, models.QuestStatusCompensated}).
			Count(&cnt).Error
		if err != nil {
			return false, err
		}
		if cnt == 0 {
			return false, nil
		}
	}
	return true, nil
}

func (r *QuestRepo) lockOrCreateProgress(tx *gorm.DB, owner, deviceID, accountID string, def models.QuestDefinition, period string, now time.Time) (*models.QuestProgress, error) {
	var p models.QuestProgress
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("owner_key = ? AND quest_id = ? AND period_key = ?", owner, def.QuestID, period).
		First(&p).Error
	if err == nil {
		return &p, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// 初始 counters
	var objs []questcatalog.Objective
	_ = json.Unmarshal([]byte(def.ObjectivesJSON), &objs)
	counters := map[string]int64{}
	for _, o := range objs {
		counters[o.ID] = 0
	}
	b, _ := json.Marshal(counters)
	p = models.QuestProgress{
		OwnerKey:      owner,
		QuestID:       def.QuestID,
		PeriodKey:     period,
		DeviceID:      deviceID,
		AccountID:     strings.TrimSpace(accountID),
		ProgressJSON:  string(b),
		Status:        models.QuestStatusActive,
		ConfigVersion: def.ConfigVersion,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	// 过期：period 结束 or duration
	if pe := r.periodEnd(def.ResetPolicy, period); pe != nil {
		p.ExpiresAt = pe
	}
	if def.DurationHours > 0 {
		exp := now.Add(time.Duration(def.DurationHours) * time.Hour)
		if p.ExpiresAt == nil || exp.Before(*p.ExpiresAt) {
			p.ExpiresAt = &exp
		}
	}
	if def.EndsAt != nil {
		end := def.EndsAt.UTC()
		if p.ExpiresAt == nil || end.Before(*p.ExpiresAt) {
			p.ExpiresAt = &end
		}
	}
	if err := tx.Create(&p).Error; err != nil {
		if isUniqueViolation(err) {
			var again models.QuestProgress
			if err2 := tx.Where("owner_key = ? AND quest_id = ? AND period_key = ?", owner, def.QuestID, period).First(&again).Error; err2 == nil {
				return &again, nil
			}
		}
		return nil, err
	}
	return &p, nil
}

// ClaimRequest 领取请求。
type ClaimRequest struct {
	DeviceID  string
	AccountID string
	QuestID   string
}

// ClaimResult 领取结果。
type ClaimResult struct {
	Claim      *models.QuestClaim `json:"claim"`
	Idempotent bool               `json:"idempotent"`
	Gold       int64              `json:"gold"`
	Stamina    int64              `json:"stamina"`
	Balances   map[string]int64   `json:"balances,omitempty"`
}

// Claim 幂等领取：operation_id 稳定，钱包入账恰好一次。
func (r *QuestRepo) Claim(req ClaimRequest) (*ClaimResult, error) {
	const maxAttempts = 40
	var lastErr error
	for attempt := range maxAttempts {
		out, err := r.claimOnce(req)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if errors.Is(err, ErrQuestNotFound) || errors.Is(err, ErrQuestExpired) ||
			errors.Is(err, ErrQuestDisabled) || errors.Is(err, ErrInvalidOwner) ||
			errors.Is(err, ErrWalletRepoUnavailable) {
			return nil, err
		}
		if errors.Is(err, ErrQuestNotClaimable) {
			// 并发窗口：别人可能刚领完
			if res := r.tryIdempotentClaim(req); res != nil {
				return res, nil
			}
			// 短暂等待 claim 行落库
			if attempt < maxAttempts-1 {
				time.Sleep(time.Duration(1+attempt*attempt) * time.Millisecond)
				if res := r.tryIdempotentClaim(req); res != nil {
					return res, nil
				}
				// 若进度仍是 completed 则继续重试 claimOnce
				continue
			}
			return nil, err
		}
		if !isTransientDBLock(err) {
			return nil, err
		}
		sleep := time.Duration(1+attempt*attempt) * time.Millisecond
		if sleep > 50*time.Millisecond {
			sleep = 50 * time.Millisecond
		}
		time.Sleep(sleep)
	}
	if res := r.tryIdempotentClaim(req); res != nil {
		return res, nil
	}
	return nil, lastErr
}

func (r *QuestRepo) tryIdempotentClaim(req ClaimRequest) *ClaimResult {
	deviceID := strings.TrimSpace(req.DeviceID)
	questID := strings.TrimSpace(req.QuestID)
	if deviceID == "" || questID == "" || r == nil || r.db == nil {
		return nil
	}
	def, err := r.GetDefinition(questID)
	if err != nil {
		return nil
	}
	owner := OwnerKey(req.AccountID, deviceID)
	period := r.PeriodKey(def.ResetPolicy, r.nowFn())
	opID := claimOperationID(owner, questID, period)
	var existing models.QuestClaim
	if err := r.db.Where("operation_id = ?", opID).First(&existing).Error; err == nil {
		return &ClaimResult{Claim: &existing, Idempotent: true, Gold: existing.GoldGranted}
	}
	// 钱包已入账但 claim 行尚未写入
	if r.wallet != nil {
		if e, err := r.wallet.FindByOperationID(opID); err == nil && e != nil {
			// 补 claim 行
			now := r.nowFn()
			claim := models.QuestClaim{
				ClaimID:     uuid.NewString(),
				OperationID: opID,
				OwnerKey:    owner,
				QuestID:     questID,
				PeriodKey:   period,
				DeviceID:    deviceID,
				AccountID:   strings.TrimSpace(req.AccountID),
				Status:      models.QuestStatusClaimed,
				RewardsJSON: def.RewardsJSON,
				GoldGranted: e.Amount,
				CreatedAt:   now,
			}
			if err := r.db.Create(&claim).Error; err == nil {
				_ = r.db.Model(&models.QuestProgress{}).
					Where("owner_key = ? AND quest_id = ? AND period_key = ?", owner, questID, period).
					Updates(map[string]interface{}{
						"status": models.QuestStatusClaimed, "claimed_at": now, "updated_at": now,
					}).Error
				return &ClaimResult{Claim: &claim, Idempotent: true, Gold: claim.GoldGranted}
			}
			var again models.QuestClaim
			if r.db.Where("operation_id = ?", opID).First(&again).Error == nil {
				return &ClaimResult{Claim: &again, Idempotent: true, Gold: again.GoldGranted}
			}
		}
	}
	return nil
}

func (r *QuestRepo) claimOnce(req ClaimRequest) (*ClaimResult, error) {
	if r == nil || r.db == nil {
		return nil, ErrQuestRepoUnavailable
	}
	if r.wallet == nil {
		return nil, ErrWalletRepoUnavailable
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return nil, ErrInvalidOwner
	}
	questID := strings.TrimSpace(req.QuestID)
	if questID == "" {
		return nil, ErrQuestNotFound
	}
	def, err := r.GetDefinition(questID)
	if err != nil {
		return nil, err
	}
	if !def.Enabled {
		return nil, ErrQuestDisabled
	}
	now := r.nowFn()
	owner := OwnerKey(req.AccountID, deviceID)
	period := r.PeriodKey(def.ResetPolicy, now)
	opID := claimOperationID(owner, questID, period)

	// 已有 claim → 幂等
	var existing models.QuestClaim
	err = r.db.Where("operation_id = ?", opID).First(&existing).Error
	if err == nil {
		return &ClaimResult{Claim: &existing, Idempotent: true, Gold: existing.GoldGranted}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 校验进度可领（不在此处改状态，避免并发误伤）
	var prog models.QuestProgress
	err = r.db.Where("owner_key = ? AND quest_id = ? AND period_key = ?", owner, questID, period).First(&prog).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrQuestNotClaimable
	}
	if err != nil {
		return nil, err
	}
	if prog.Status == models.QuestStatusClaimed || prog.Status == models.QuestStatusCompensated {
		if res := r.tryIdempotentClaim(req); res != nil {
			return res, nil
		}
		return nil, ErrQuestNotClaimable
	}
	if prog.Status != models.QuestStatusCompleted {
		return nil, ErrQuestNotClaimable
	}
	if prog.ExpiresAt != nil && !now.Before(prog.ExpiresAt.UTC()) {
		return nil, ErrQuestExpired
	}

	var reward questcatalog.Reward
	if err := json.Unmarshal([]byte(def.RewardsJSON), &reward); err != nil {
		return nil, err
	}

	// 钱包先入账（operation_id 全局唯一 → 真正的并发闸门）
	balances := map[string]int64{}
	walletIdempotent := false
	if reward.Gold > 0 {
		res, err := r.wallet.Apply(ApplyRequest{
			DeviceID:    deviceID,
			AccountID:   req.AccountID,
			Kind:        models.LedgerKindCurrency,
			Currency:    models.CurrencyGold,
			Delta:       reward.Gold,
			OperationID: opID,
			SourceType:  "task",
			SourceID:    questID,
			Metadata:    fmt.Sprintf(`{"period":"%s","config":"%s"}`, period, def.ConfigVersion),
		})
		if err != nil {
			return nil, err
		}
		balances[models.CurrencyGold] = res.Balance
		walletIdempotent = res.Idempotent
	}
	if reward.Stamina > 0 {
		stOp := truncateOp(opID + ":stamina")
		res, err := r.wallet.Apply(ApplyRequest{
			DeviceID:    deviceID,
			AccountID:   req.AccountID,
			Kind:        models.LedgerKindCurrency,
			Currency:    models.CurrencyStamina,
			Delta:       reward.Stamina,
			OperationID: stOp,
			SourceType:  "task",
			SourceID:    questID,
			Metadata:    fmt.Sprintf(`{"period":"%s"}`, period),
		})
		if err != nil {
			return nil, err
		}
		balances[models.CurrencyStamina] = res.Balance
	}
	for itemID, qty := range reward.Items {
		if qty <= 0 {
			continue
		}
		itemOp := truncateOp(opID + ":item:" + itemID)
		if _, err := r.wallet.Apply(ApplyRequest{
			DeviceID:    deviceID,
			AccountID:   req.AccountID,
			Kind:        models.LedgerKindItem,
			Currency:    itemID,
			Delta:       qty,
			OperationID: itemOp,
			SourceType:  "task",
			SourceID:    questID,
		}); err != nil {
			return nil, err
		}
	}

	// gold=0 时用 claim 行唯一约束做闸门
	claim := models.QuestClaim{
		ClaimID:     uuid.NewString(),
		OperationID: opID,
		OwnerKey:    owner,
		QuestID:     questID,
		PeriodKey:   period,
		DeviceID:    deviceID,
		AccountID:   strings.TrimSpace(req.AccountID),
		Status:      models.QuestStatusClaimed,
		RewardsJSON: def.RewardsJSON,
		GoldGranted: reward.Gold,
		CreatedAt:   now,
	}
	if err := r.db.Create(&claim).Error; err != nil {
		if isUniqueViolation(err) {
			var c models.QuestClaim
			if e2 := r.db.Where("operation_id = ?", opID).First(&c).Error; e2 == nil {
				return &ClaimResult{Claim: &c, Idempotent: true, Gold: c.GoldGranted, Balances: balances}, nil
			}
			return &ClaimResult{Claim: &claim, Idempotent: true, Gold: reward.Gold, Balances: balances}, nil
		}
		return nil, err
	}
	_ = r.db.Model(&models.QuestProgress{}).
		Where("owner_key = ? AND quest_id = ? AND period_key = ?", owner, questID, period).
		Updates(map[string]interface{}{
			"status":     models.QuestStatusClaimed,
			"claimed_at": now,
			"updated_at": now,
		}).Error

	return &ClaimResult{
		Claim:      &claim,
		Idempotent: walletIdempotent,
		Gold:       reward.Gold,
		Stamina:    reward.Stamina,
		Balances:   balances,
	}, nil
}

func claimOperationID(owner, questID, period string) string {
	// 稳定且 ≤128
	raw := fmt.Sprintf("quest:%s:%s:%s", questID, period, owner)
	if len(raw) <= 128 {
		return raw
	}
	sum := sha1.Sum([]byte(raw))
	return "quest:" + hex.EncodeToString(sum[:]) // 46 chars
}

func truncateOp(s string) string {
	if len(s) <= 128 {
		return s
	}
	sum := sha1.Sum([]byte(s))
	return "q:" + hex.EncodeToString(sum[:])
}

// CompensateResult 过期补偿结果。
type CompensateResult struct {
	Compensated int `json:"compensated"`
	Expired     int `json:"expired"`
}

// CompensateExpired 将已完成但过期未领的任务补偿入账；active 过期标 expired。
// 补偿金为原 gold 的 50%（至少 1，若原 >0）。
func (r *QuestRepo) CompensateExpired(accountID, deviceID string) (*CompensateResult, error) {
	if r == nil || r.db == nil {
		return nil, ErrQuestRepoUnavailable
	}
	owner := OwnerKey(accountID, deviceID)
	now := r.nowFn()
	var rows []models.QuestProgress
	if err := r.db.Where("owner_key = ? AND status IN ? AND expires_at IS NOT NULL AND expires_at <= ?",
		owner,
		[]string{models.QuestStatusActive, models.QuestStatusCompleted},
		now,
	).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := &CompensateResult{}
	for _, prog := range rows {
		if prog.Status == models.QuestStatusActive {
			_ = r.db.Model(&models.QuestProgress{}).Where("id = ?", prog.ID).
				Updates(map[string]interface{}{"status": models.QuestStatusExpired, "updated_at": now}).Error
			out.Expired++
			continue
		}
		// completed + expired → compensate
		def, err := r.GetDefinition(prog.QuestID)
		if err != nil {
			continue
		}
		var reward questcatalog.Reward
		_ = json.Unmarshal([]byte(def.RewardsJSON), &reward)
		gold := reward.Gold / 2
		if reward.Gold > 0 && gold < 1 {
			gold = 1
		}
		opID := "quest-comp:" + claimOperationID(owner, prog.QuestID, prog.PeriodKey)
		if len(opID) > 128 {
			opID = truncateOp(opID)
		}
		// 已补偿？
		var existing models.QuestClaim
		if err := r.db.Where("operation_id = ?", opID).First(&existing).Error; err == nil {
			continue
		}
		if gold > 0 && r.wallet != nil {
			if _, err := r.wallet.Apply(ApplyRequest{
				DeviceID:    deviceID,
				AccountID:   accountID,
				Kind:        models.LedgerKindCurrency,
				Currency:    models.CurrencyGold,
				Delta:       gold,
				OperationID: opID,
				SourceType:  "compensate",
				SourceID:    prog.QuestID,
				Metadata:    fmt.Sprintf(`{"period":"%s","reason":"expired_unclaimed"}`, prog.PeriodKey),
			}); err != nil {
				return nil, err
			}
		}
		claim := models.QuestClaim{
			ClaimID:     uuid.NewString(),
			OperationID: opID,
			OwnerKey:    owner,
			QuestID:     prog.QuestID,
			PeriodKey:   prog.PeriodKey,
			DeviceID:    deviceID,
			AccountID:   strings.TrimSpace(accountID),
			Status:      models.QuestStatusCompensated,
			RewardsJSON: fmt.Sprintf(`{"gold":%d}`, gold),
			GoldGranted: gold,
			CreatedAt:   now,
		}
		if err := r.db.Create(&claim).Error; err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return nil, err
		}
		_ = r.db.Model(&models.QuestProgress{}).Where("id = ?", prog.ID).
			Updates(map[string]interface{}{
				"status":     models.QuestStatusCompensated,
				"claimed_at": now,
				"updated_at": now,
			}).Error
		out.Compensated++
	}
	return out, nil
}

// SimulateDays 简单 30 天可达性模拟：每日 season_checkin + capture，检查 free 任务与主线是否可推进。
func (r *QuestRepo) SimulateDays(deviceID string, days int) (map[string]int, error) {
	if days <= 0 {
		days = 30
	}
	stats := map[string]int{
		"events":  0,
		"claims":  0,
		"free_ok": 0,
	}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for d := 0; d < days; d++ {
		day := base.AddDate(0, 0, d)
		r.SetNowFunc(func() time.Time { return day })
		// free path
		_, err := r.ApplyEvent(ApplyEventRequest{
			DeviceID: deviceID, EventID: fmt.Sprintf("sim-chk-%d", d),
			EventType: models.QuestEventSeasonCheckin, Delta: 1,
		})
		if err != nil {
			return stats, err
		}
		stats["events"]++
		_, err = r.ApplyEvent(ApplyEventRequest{
			DeviceID: deviceID, EventID: fmt.Sprintf("sim-cap-%d", d),
			EventType: models.QuestEventCaptureSuccess, Delta: 1,
		})
		if err != nil {
			return stats, err
		}
		stats["events"]++
		// claim free daily if ready
		views, err := r.ListForOwner("", deviceID, true)
		if err != nil {
			return stats, err
		}
		if len(views) > 0 {
			stats["free_ok"]++
		}
		for _, v := range views {
			if v.Claimable {
				if _, err := r.Claim(ClaimRequest{DeviceID: deviceID, QuestID: v.Definition.QuestID}); err == nil {
					stats["claims"]++
				}
			}
		}
	}
	return stats, nil
}
