// Package repo — 剧情进度与选择（AP-132）。
package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/narrativecatalog"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNarrativeNotFound   = errors.New("narrative_not_found")
	ErrNarrativeIllegal    = errors.New("narrative_illegal")
	ErrNarrativeDuplicate  = errors.New("narrative_duplicate_choice")
	ErrNarrativeWithdrawn  = errors.New("narrative_withdrawn")
	ErrNarrativeBadVersion = errors.New("narrative_version")
)

// NarrativeRepo 剧情状态仓储。
type NarrativeRepo struct {
	db *gorm.DB
}

// NewNarrativeRepo 构造。
func NewNarrativeRepo(db *gorm.DB) *NarrativeRepo { return &NarrativeRepo{db: db} }

// SeedContent 幂等写入 authored 内容。
func (r *NarrativeRepo) SeedContent() error {
	for _, n := range narrativecatalog.SeedNodes() {
		now := time.Now().UTC()
		row := models.NarrativeNode{
			NodeID: n.NodeID, ChapterID: n.ChapterID, Title: n.Title, Body: n.Body,
			Kind: n.Kind, ContentVersion: narrativecatalog.ContentVersion, Priority: n.Priority,
			Active: true, SafeFallback: n.SafeFallback, TagsJSON: narrativecatalog.MustJSON(n.Tags),
			CreatedAt: now, UpdatedAt: now,
		}
		if err := r.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "node_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"chapter_id", "title", "body", "kind", "content_version", "priority", "safe_fallback", "tags_json", "updated_at",
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	for _, c := range narrativecatalog.SeedChoices() {
		var existing models.NarrativeChoice
		err := r.db.Where("choice_id = ?", c.ChoiceID).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row := models.NarrativeChoice{
			ChoiceID: c.ChoiceID, FromNodeID: c.FromNodeID, ToNodeID: c.ToNodeID,
			Label: c.Label, Prompt: c.Prompt, EffectsJSON: narrativecatalog.MustJSON(c.Effects),
			SortOrder: c.SortOrder, Active: true,
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}
		if err := r.db.Create(&row).Error; err != nil {
			return err
		}
	}
	for _, f := range narrativecatalog.SeedFragments() {
		var existing models.StoryFragment
		err := r.db.Where("fragment_id = ?", f.FragmentID).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row := models.StoryFragment{
			FragmentID: f.FragmentID, Title: f.Title, Body: f.Body, Priority: f.Priority,
			TriggersJSON: narrativecatalog.MustJSON(f.Triggers), MutexGroup: f.MutexGroup,
			CooldownHours: f.CooldownHours, FallbackID: f.FallbackID,
			ContentVersion: narrativecatalog.ContentVersion, Active: true,
			CreatedAt: time.Now().UTC(),
		}
		if err := r.db.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetNode 读取节点；撤回则返回安全节点摘要。
func (r *NarrativeRepo) GetNode(nodeID string) (*models.NarrativeNode, error) {
	var n models.NarrativeNode
	err := r.db.Where("node_id = ? AND active = ?", nodeID, true).First(&n).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNarrativeNotFound
	}
	if err != nil {
		return nil, err
	}
	if n.Withdrawn {
		if n.SafeFallback != "" {
			return r.GetNode(n.SafeFallback)
		}
		return nil, ErrNarrativeWithdrawn
	}
	return &n, nil
}

// ListChoices 列出节点可选边。
func (r *NarrativeRepo) ListChoices(fromNodeID string) ([]models.NarrativeChoice, error) {
	var rows []models.NarrativeChoice
	err := r.db.Where("from_node_id = ? AND active = ?", fromNodeID, true).
		Order("sort_order asc").Find(&rows).Error
	return rows, err
}

// EnsureProgress 确保章节进度存在。
func (r *NarrativeRepo) EnsureProgress(ownerKey, deviceID, accountID, chapterID string) (*models.NarrativeProgress, error) {
	var p models.NarrativeProgress
	err := r.db.Where("owner_key = ? AND chapter_id = ?", ownerKey, chapterID).First(&p).Error
	if err == nil {
		return &p, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	start := chapterStart(chapterID)
	p = models.NarrativeProgress{
		OwnerKey: ownerKey, ChapterID: chapterID, DeviceID: deviceID, AccountID: accountID,
		CurrentNodeID: start, CheckpointNode: start, ContentVersion: narrativecatalog.ContentVersion,
		FlagsJSON: "{}", RelationshipsJSON: "{}", ServerVersion: 1,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := r.db.Create(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func chapterStart(chapterID string) string {
	switch chapterID {
	case "ch3":
		return "ch3_rain_eaves"
	case "ch4":
		return "ch4_map_blank"
	case "fail_forward":
		return "ff_miss_1"
	default:
		return "ch1_intro"
	}
}

// PullProgress 拉取全部章节进度。
func (r *NarrativeRepo) PullProgress(ownerKey string) ([]models.NarrativeProgress, error) {
	var rows []models.NarrativeProgress
	err := r.db.Where("owner_key = ?", ownerKey).Order("chapter_id asc").Find(&rows).Error
	return rows, err
}

// MarkSeen 标记已读。
func (r *NarrativeRepo) MarkSeen(ownerKey, deviceID, accountID, nodeID, summary string) error {
	now := time.Now().UTC()
	var s models.NarrativeSeenState
	err := r.db.Where("owner_key = ? AND node_id = ?", ownerKey, nodeID).First(&s).Error
	if err == nil {
		s.LastSeen = now
		if summary != "" {
			s.Summary = summary
		}
		return r.db.Save(&s).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	s = models.NarrativeSeenState{
		OwnerKey: ownerKey, NodeID: nodeID, DeviceID: deviceID, AccountID: accountID,
		FirstSeen: now, LastSeen: now, Summary: summary,
	}
	return r.db.Create(&s).Error
}

// ListSeen 已读列表。
func (r *NarrativeRepo) ListSeen(ownerKey string) ([]models.NarrativeSeenState, error) {
	var rows []models.NarrativeSeenState
	err := r.db.Where("owner_key = ?", ownerKey).Order("first_seen asc").Find(&rows).Error
	return rows, err
}

// SubmitChoice 幂等提交选择。
func (r *NarrativeRepo) SubmitChoice(ownerKey, deviceID, accountID, chapterID, choiceID, operationID string) (*models.NarrativeProgress, bool, error) {
	operationID = strings.TrimSpace(operationID)
	if operationID == "" || choiceID == "" {
		return nil, false, ErrNarrativeIllegal
	}
	var out *models.NarrativeProgress
	var idempotent bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing models.NarrativeChoiceLog
		if err := tx.Where("operation_id = ?", operationID).First(&existing).Error; err == nil {
			idempotent = true
			var p models.NarrativeProgress
			if e := tx.Where("owner_key = ? AND chapter_id = ?", ownerKey, chapterID).First(&p).Error; e != nil {
				return e
			}
			out = &p
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var choice models.NarrativeChoice
		if err := tx.Where("choice_id = ? AND active = ?", choiceID, true).First(&choice).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNarrativeNotFound
			}
			return err
		}

		var p models.NarrativeProgress
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_key = ? AND chapter_id = ?", ownerKey, chapterID).First(&p).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNarrativeIllegal
			}
			return err
		}
		if p.CurrentNodeID != choice.FromNodeID {
			return ErrNarrativeIllegal
		}
		// 禁止重复提交同一 choice（除非不同 operation 且已离开节点——已由 current 校验）
		var prev models.NarrativeChoiceLog
		if err := tx.Where("owner_key = ? AND choice_id = ?", ownerKey, choiceID).First(&prev).Error; err == nil {
			return ErrNarrativeDuplicate
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// apply effects into flags/relationships
		flags := map[string]any{}
		rels := map[string]any{}
		_ = json.Unmarshal([]byte(p.FlagsJSON), &flags)
		_ = json.Unmarshal([]byte(p.RelationshipsJSON), &rels)
		var effects map[string]any
		_ = json.Unmarshal([]byte(choice.EffectsJSON), &effects)
		for k, v := range effects {
			if strings.HasPrefix(k, "flag:") {
				flags[strings.TrimPrefix(k, "flag:")] = v
			} else if strings.HasPrefix(k, "rel:") {
				key := strings.TrimPrefix(k, "rel:")
				cur, _ := rels[key].(float64)
				switch n := v.(type) {
				case float64:
					rels[key] = cur + n
				case int:
					rels[key] = cur + float64(n)
				default:
					rels[key] = v
				}
			} else if strings.HasPrefix(k, "clue:") {
				// clue updates handled by caller/handler layer via ClueState
				flags[k] = v
			}
		}
		fb, _ := json.Marshal(flags)
		rb, _ := json.Marshal(rels)

		// resolve target; withdrawn → safe
		toID := choice.ToNodeID
		var to models.NarrativeNode
		if err := tx.Where("node_id = ?", toID).First(&to).Error; err == nil && to.Withdrawn && to.SafeFallback != "" {
			toID = to.SafeFallback
		}

		p.CurrentNodeID = toID
		p.LastChoiceID = choiceID
		p.FlagsJSON = string(fb)
		p.RelationshipsJSON = string(rb)
		p.ServerVersion++
		p.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&p).Error; err != nil {
			return err
		}

		log := models.NarrativeChoiceLog{
			OperationID: operationID, OwnerKey: ownerKey, DeviceID: deviceID, AccountID: accountID,
			ChoiceID: choiceID, FromNodeID: choice.FromNodeID, ToNodeID: toID,
			EffectsJSON: choice.EffectsJSON, CreatedAt: time.Now().UTC(),
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		out = &p
		return nil
	})
	return out, idempotent, err
}

// AdvanceTo 非选择推进（阅读后进入下一默认节点或 fail-forward）。
func (r *NarrativeRepo) AdvanceTo(ownerKey, deviceID, accountID, chapterID, nodeID string) (*models.NarrativeProgress, error) {
	if _, err := r.GetNode(nodeID); err != nil {
		return nil, err
	}
	p, err := r.EnsureProgress(ownerKey, deviceID, accountID, chapterID)
	if err != nil {
		return nil, err
	}
	p.CurrentNodeID = nodeID
	p.ServerVersion++
	p.UpdatedAt = time.Now().UTC()
	if err := r.db.Save(p).Error; err != nil {
		return nil, err
	}
	_ = r.MarkSeen(ownerKey, deviceID, accountID, nodeID, "")
	return p, nil
}

// FailForward 连续失败推进。
func (r *NarrativeRepo) FailForward(ownerKey, deviceID, accountID string, missCount int, reason string) (*models.NarrativeNode, error) {
	nodeID := "ff_miss_1"
	switch {
	case reason == "no_camera" || reason == "permission":
		nodeID = "ff_no_camera"
	case reason == "weather":
		nodeID = "ff_weather"
	case missCount >= 3:
		nodeID = "ff_miss_3"
	case missCount == 2:
		nodeID = "ff_miss_2"
	}
	n, err := r.GetNode(nodeID)
	if err != nil {
		return nil, err
	}
	if _, err := r.EnsureProgress(ownerKey, deviceID, accountID, "fail_forward"); err != nil {
		return nil, err
	}
	if _, err := r.AdvanceTo(ownerKey, deviceID, accountID, "fail_forward", nodeID); err != nil {
		return nil, err
	}
	return n, nil
}

// TryUnlockFragments 根据观察上下文尝试解锁碎片（同一 operation 幂等）。
func (r *NarrativeRepo) TryUnlockFragments(ownerKey, deviceID, accountID, operationID string, ctx map[string]any) ([]models.StoryFragment, error) {
	var frags []models.StoryFragment
	if err := r.db.Where("active = ?", true).Order("priority desc").Find(&frags).Error; err != nil {
		return nil, err
	}
	unlocked := make([]models.StoryFragment, 0)
	for _, f := range frags {
		var triggers map[string]any
		_ = json.Unmarshal([]byte(f.TriggersJSON), &triggers)
		if !matchTriggers(triggers, ctx) {
			continue
		}
		// already unlocked?
		var u models.StoryFragmentUnlock
		err := r.db.Where("owner_key = ? AND fragment_id = ?", ownerKey, f.FragmentID).First(&u).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		op := fmt.Sprintf("%s:%s", operationID, f.FragmentID)
		row := models.StoryFragmentUnlock{
			OwnerKey: ownerKey, FragmentID: f.FragmentID, OperationID: op,
			DeviceID: deviceID, Reason: fragmentReason(f, ctx), UnlockedAt: time.Now().UTC(),
		}
		if err := r.db.Create(&row).Error; err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") {
				continue
			}
			return nil, err
		}
		unlocked = append(unlocked, f)
	}
	return unlocked, nil
}

func matchTriggers(triggers, ctx map[string]any) bool {
	if len(triggers) == 0 {
		return false
	}
	if v, ok := triggers["min_observations"]; ok {
		need := asFloat(v)
		have := asFloat(ctx["observation_count"])
		if have < need {
			return false
		}
	}
	if v, ok := triggers["first_species"]; ok {
		sp, _ := v.(string)
		if asString(ctx["first_species"]) != sp && asString(ctx["species"]) != sp {
			// allow species match on this event if first
			if asString(ctx["species"]) != sp || !asBool(ctx["is_first_species"]) {
				return false
			}
		}
	}
	if v, ok := triggers["weather"]; ok {
		if asString(ctx["weather"]) != asString(v) {
			return false
		}
	}
	if v, ok := triggers["species_set"]; ok {
		arr, _ := v.([]any)
		set, _ := ctx["species_seen"].(map[string]bool)
		if set == nil {
			return false
		}
		for _, x := range arr {
			if !set[asString(x)] {
				return false
			}
		}
	}
	return true
}

func fragmentReason(f models.StoryFragment, ctx map[string]any) string {
	// prefer authored hint from catalog via title
	if s := asString(ctx["reason"]); s != "" {
		return s
	}
	return "trigger_matched:" + f.FragmentID
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

// UpsertClue 更新线索状态。
func (r *NarrativeRepo) UpsertClue(ownerKey, clueID, status, source, evidence string) error {
	var c models.ClueState
	err := r.db.Where("owner_key = ? AND clue_id = ?", ownerKey, clueID).First(&c).Error
	now := time.Now().UTC()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c = models.ClueState{
			OwnerKey: ownerKey, ClueID: clueID, Status: status, Source: source, Evidence: evidence,
			CreatedAt: now, UpdatedAt: now,
		}
		return r.db.Create(&c).Error
	}
	if err != nil {
		return err
	}
	c.Status = status
	if source != "" {
		c.Source = source
	}
	if evidence != "" {
		c.Evidence = evidence
	}
	c.UpdatedAt = now
	return r.db.Save(&c).Error
}

// ListClues 线索列表。
func (r *NarrativeRepo) ListClues(ownerKey string) ([]models.ClueState, error) {
	var rows []models.ClueState
	err := r.db.Where("owner_key = ?", ownerKey).Order("clue_id asc").Find(&rows).Error
	return rows, err
}
