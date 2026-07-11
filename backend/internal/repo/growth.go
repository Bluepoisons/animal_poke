// Package repo — AP-099 研究员成长与纯虚拟伙伴关系。
package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 成长领域错误。
var (
	ErrGrowthRepoUnavailable = errors.New("growth_repo_unavailable")
	ErrGrowthInvalidEvent    = errors.New("growth_invalid_event")
	ErrGrowthForbiddenKind   = errors.New("growth_forbidden_kind")
	ErrGrowthPaidPower       = errors.New("growth_paid_power_forbidden")
	ErrGrowthDecayForbidden  = errors.New("growth_decay_forbidden")
	ErrGrowthCapReached      = errors.New("growth_cap_reached")
	ErrGrowthNotFound        = errors.New("growth_not_found")
	ErrGrowthAnimalNotOwned  = errors.New("growth_animal_not_owned")
)

// GrowthConfigVersion 当前配置版本。
const GrowthConfigVersion = models.GrowthConfigVersion

// 研究员轨道 XP 上限（防刷，不绑定付费）。
const researcherTrackXPCap int64 = 10_000

// 伙伴羁绊 XP 上限。
const companionBondXPCap int64 = 5_000

// 研究员等级阈值（累计 XP → level）。内容/便利解锁，不改识别/战力。
var researcherLevelThresholds = []int64{0, 20, 50, 100, 180, 300, 500, 800, 1200, 1800, 2500}

// 伙伴羁绊等级阈值。
var companionLevelThresholds = []int64{0, 10, 25, 50, 80, 120, 180, 260, 360, 500}
// CompanionNodeDef 可见成长节点定义（每收藏至少 3 个）。
type CompanionNodeDef struct {
	NodeID     string `json:"node_id"`
	Title      string `json:"title"`
	Kind       string `json:"kind"` // memory|decor|journal
	UnlockAtXP int64  `json:"unlock_at_xp"`
}

// CompanionNodeCatalog 默认 5 个可见节点（验收：≥3）。
var CompanionNodeCatalog = []CompanionNodeDef{
	{NodeID: "first_meeting", Title: "初次相遇", Kind: "memory", UnlockAtXP: 0},
	{NodeID: "shared_journal", Title: "共同观察日记", Kind: "journal", UnlockAtXP: 10},
	{NodeID: "decorative_bond", Title: "装饰羁绊", Kind: "decor", UnlockAtXP: 25},
	{NodeID: "photo_album", Title: "合影相册", Kind: "memory", UnlockAtXP: 50},
	{NodeID: "habitat_note", Title: "栖息地札记", Kind: "journal", UnlockAtXP: 80},
}

// GrowthEventSpec 允许的事件及其效果。
type GrowthEventSpec struct {
	Kind     string
	Track    string // photography|ecology|safe_observation|companion
	DeltaXP  int64
	DailyCap int // 0 = 无日 cap（事件本身仍幂等）
}

// AllowedGrowthEvents 白名单（无付费战力、无衰减、无现实投喂）。
var AllowedGrowthEvents = map[string]GrowthEventSpec{
	models.GrowthEventPhotoCapture:      {Kind: models.GrowthEventPhotoCapture, Track: models.GrowthTrackPhotography, DeltaXP: 5},
	models.GrowthEventPhotoQuality:      {Kind: models.GrowthEventPhotoQuality, Track: models.GrowthTrackPhotography, DeltaXP: 8},
	models.GrowthEventSpeciesFirst:      {Kind: models.GrowthEventSpeciesFirst, Track: models.GrowthTrackEcology, DeltaXP: 15},
	models.GrowthEventSpeciesResearch:   {Kind: models.GrowthEventSpeciesResearch, Track: models.GrowthTrackEcology, DeltaXP: 6},
	models.GrowthEventSafeExplore:       {Kind: models.GrowthEventSafeExplore, Track: models.GrowthTrackSafeObservation, DeltaXP: 5},
	models.GrowthEventDistanceRespect:   {Kind: models.GrowthEventDistanceRespect, Track: models.GrowthTrackSafeObservation, DeltaXP: 7},
	models.GrowthEventCompanionInteract: {Kind: models.GrowthEventCompanionInteract, Track: "companion", DeltaXP: 4},
	models.GrowthEventCompanionMemory:   {Kind: models.GrowthEventCompanionMemory, Track: "companion", DeltaXP: 6},
	models.GrowthEventCompanionDecor:    {Kind: models.GrowthEventCompanionDecor, Track: "companion", DeltaXP: 5},
}

// ForbiddenGrowthKinds 明确拒绝的路径（衰减/现实投喂/付费战力）。
var ForbiddenGrowthKinds = map[string]error{
	"feed":            ErrGrowthDecayForbidden, // 现实投喂
	"real_feed":       ErrGrowthDecayForbidden,
	"decay":           ErrGrowthDecayForbidden,
	"affinity_decay":  ErrGrowthDecayForbidden,
	"paid_power":      ErrGrowthPaidPower,
	"iap_power":       ErrGrowthPaidPower,
	"combat_boost":    ErrGrowthPaidPower,
	"stat_boost":      ErrGrowthPaidPower,
	"battle_power":    ErrGrowthPaidPower,
}

// GrowthRepo 成长仓储。
type GrowthRepo struct {
	db *gorm.DB
}

// NewGrowthRepo 构造。
func NewGrowthRepo(db *gorm.DB) *GrowthRepo {
	return &GrowthRepo{db: db}
}

// WithTx 绑定事务。
func (r *GrowthRepo) WithTx(tx *gorm.DB) *GrowthRepo {
	return &GrowthRepo{db: tx}
}

// DB 暴露底层连接。
func (r *GrowthRepo) DB() *gorm.DB { return r.db }

// LevelFromXP 根据阈值计算等级。
func LevelFromXP(xp int64, thresholds []int64) int {
	if xp < 0 {
		xp = 0
	}
	lvl := 0
	for i, t := range thresholds {
		if xp >= t {
			lvl = i
		}
	}
	return lvl
}

// ResearcherLevel 研究员等级。
func ResearcherLevel(xp int64) int {
	return LevelFromXP(xp, researcherLevelThresholds)
}

// CompanionLevel 伙伴羁绊等级。
func CompanionLevel(xp int64) int {
	return LevelFromXP(xp, companionLevelThresholds)
}

// AllResearcherTracks 三轨道常量。
func AllResearcherTracks() []string {
	return []string{
		models.GrowthTrackPhotography,
		models.GrowthTrackEcology,
		models.GrowthTrackSafeObservation,
	}
}

// ApplyGrowthRequest 记录成长事件请求。
type ApplyGrowthRequest struct {
	DeviceID    string
	AccountID   string
	EventID     string
	Kind        string
	AnimalUUID  string
	SourceType  string
	SourceID    string
	Metadata    string
	// OverrideDelta 可选：服务端可按质量覆盖默认 XP（仍受 cap 与白名单约束）。
	OverrideDelta *int64
}

// ApplyGrowthResult 成长事件结果。
type ApplyGrowthResult struct {
	Event          *models.GrowthEvent       `json:"event"`
	Idempotent     bool                      `json:"idempotent"`
	Researcher     []models.ResearcherTrack  `json:"researcher,omitempty"`
	Companion      *models.CompanionProfile  `json:"companion,omitempty"`
	Nodes          []models.CompanionMemoryNode `json:"nodes,omitempty"`
	UnlockedNodes  []string                  `json:"unlocked_nodes,omitempty"`
	CombatUnchanged bool                     `json:"combat_unchanged"`
}

// ApplyEvent 幂等记录成长事件并更新快照。
// 不修改 Animal 战斗属性；拒绝衰减与付费战力路径。
func (r *GrowthRepo) ApplyEvent(req ApplyGrowthRequest) (*ApplyGrowthResult, error) {
	if r == nil || r.db == nil {
		return nil, ErrGrowthRepoUnavailable
	}
	eventID := strings.TrimSpace(req.EventID)
	kind := strings.TrimSpace(strings.ToLower(req.Kind))
	if eventID == "" || kind == "" {
		return nil, ErrGrowthInvalidEvent
	}
	if err, ok := ForbiddenGrowthKinds[kind]; ok {
		return nil, err
	}
	spec, ok := AllowedGrowthEvents[kind]
	if !ok {
		return nil, ErrGrowthForbiddenKind
	}
	delta := spec.DeltaXP
	if req.OverrideDelta != nil {
		if *req.OverrideDelta <= 0 || *req.OverrideDelta > 50 {
			return nil, ErrGrowthInvalidEvent
		}
		delta = *req.OverrideDelta
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	accountID := strings.TrimSpace(req.AccountID)
	if deviceID == "" && accountID == "" {
		return nil, ErrGrowthInvalidEvent
	}
	owner := OwnerKey(accountID, deviceID)
	animalUUID := strings.TrimSpace(req.AnimalUUID)

	// 伙伴类事件必须绑定收藏
	if spec.Track == "companion" {
		if animalUUID == "" {
			return nil, ErrGrowthInvalidEvent
		}
	}

	var out *ApplyGrowthResult
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// 幂等：已存在直接返回
		var existing models.GrowthEvent
		err := tx.Where("event_id = ?", eventID).First(&existing).Error
		if err == nil {
			res, buildErr := r.buildResult(tx, owner, accountID, deviceID, &existing, true, animalUUID)
			if buildErr != nil {
				return buildErr
			}
			out = res
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 伙伴事件：校验所有权并确保档案
		var companion *models.CompanionProfile
		var nodes []models.CompanionMemoryNode
		var unlocked []string
		xpAfter := int64(0)
		levelAfter := 0
		track := spec.Track

		if spec.Track == "companion" {
			animal, aerr := findOwnedAnimal(tx, animalUUID, deviceID, accountID)
			if aerr != nil {
				return aerr
			}
			_ = animal
			comp, _, uerr := ensureCompanion(tx, owner, deviceID, accountID, animalUUID)
			if uerr != nil {
				return uerr
			}
			// 应用 bond XP（cap）
			newXP := comp.BondXP + delta
			if newXP > companionBondXPCap {
				delta = companionBondXPCap - comp.BondXP
				if delta < 0 {
					delta = 0
				}
				newXP = companionBondXPCap
			}
			comp.BondXP = newXP
			comp.BondLevel = CompanionLevel(newXP)
			// 装饰阶段随 bond level 提升，不改战力
			comp.DecorStage = comp.BondLevel / 2
			comp.ConfigVersion = GrowthConfigVersion
			if err := tx.Save(comp).Error; err != nil {
				return err
			}
			// 解锁节点
			unlocked, nodes, err = unlockCompanionNodes(tx, comp)
			if err != nil {
				return err
			}
			companion = comp
			xpAfter = comp.BondXP
			levelAfter = comp.BondLevel
		} else {
			// 研究员轨道
			tr, terr := lockOrCreateTrack(tx, owner, track, deviceID, accountID)
			if terr != nil {
				return terr
			}
			newXP := tr.XP + delta
			if newXP > researcherTrackXPCap {
				delta = researcherTrackXPCap - tr.XP
				if delta < 0 {
					delta = 0
				}
				newXP = researcherTrackXPCap
			}
			tr.XP = newXP
			tr.Level = ResearcherLevel(newXP)
			tr.ConfigVersion = GrowthConfigVersion
			if err := tx.Save(tr).Error; err != nil {
				return err
			}
			xpAfter = tr.XP
			levelAfter = tr.Level

			// 可选：关联收藏时仍确保 companion 存在但不强制加 bond
			if animalUUID != "" {
				if _, aerr := findOwnedAnimal(tx, animalUUID, deviceID, accountID); aerr == nil {
					comp, ns, uerr := ensureCompanion(tx, owner, deviceID, accountID, animalUUID)
					if uerr == nil {
						companion = comp
						nodes = ns
					}
				}
			}
		}

		ev := &models.GrowthEvent{
			EventID:       eventID,
			OwnerKey:      owner,
			DeviceID:      deviceID,
			AccountID:     accountID,
			Kind:          kind,
			Track:         track,
			AnimalUUID:    animalUUID,
			DeltaXP:       delta,
			XPAfter:       xpAfter,
			LevelAfter:    levelAfter,
			SourceType:    strings.TrimSpace(req.SourceType),
			SourceID:      strings.TrimSpace(req.SourceID),
			Metadata:      req.Metadata,
			ConfigVersion: GrowthConfigVersion,
			CreatedAt:     time.Now().UTC(),
		}
		if len(unlocked) > 0 {
			ev.NodeID = unlocked[0]
		}
		if err := tx.Create(ev).Error; err != nil {
			if isUniqueViolation(err) {
				var again models.GrowthEvent
				if ferr := tx.Where("event_id = ?", eventID).First(&again).Error; ferr == nil {
					res, buildErr := r.buildResult(tx, owner, accountID, deviceID, &again, true, animalUUID)
					if buildErr != nil {
						return buildErr
					}
					out = res
					return nil
				}
			}
			return err
		}

		tracks, _ := listTracksTx(tx, owner)
		out = &ApplyGrowthResult{
			Event:           ev,
			Idempotent:      false,
			Researcher:      tracks,
			Companion:       companion,
			Nodes:           nodes,
			UnlockedNodes:   unlocked,
			CombatUnchanged: true,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *GrowthRepo) buildResult(tx *gorm.DB, owner, accountID, deviceID string, ev *models.GrowthEvent, idempotent bool, animalUUID string) (*ApplyGrowthResult, error) {
	tracks, err := listTracksTx(tx, owner)
	if err != nil {
		return nil, err
	}
	res := &ApplyGrowthResult{
		Event:           ev,
		Idempotent:      idempotent,
		Researcher:      tracks,
		CombatUnchanged: true,
	}
	au := animalUUID
	if au == "" {
		au = ev.AnimalUUID
	}
	if au != "" {
		var comp models.CompanionProfile
		if err := tx.Where("animal_uuid = ? AND owner_key = ?", au, owner).First(&comp).Error; err == nil {
			res.Companion = &comp
			var nodes []models.CompanionMemoryNode
			_ = tx.Where("animal_uuid = ?", au).Order("unlock_at_xp asc").Find(&nodes).Error
			res.Nodes = nodes
		}
	}
	return res, nil
}

func findOwnedAnimal(tx *gorm.DB, animalUUID, deviceID, accountID string) (*models.Animal, error) {
	var a models.Animal
	err := tx.Where("uuid = ? AND deleted_at IS NULL", animalUUID).First(&a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGrowthNotFound
		}
		return nil, err
	}
	if !OwnsAnimal(&a, deviceID, accountID) {
		return nil, ErrGrowthAnimalNotOwned
	}
	return &a, nil
}

func lockOrCreateTrack(tx *gorm.DB, owner, track, deviceID, accountID string) (*models.ResearcherTrack, error) {
	var tr models.ResearcherTrack
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("owner_key = ? AND track = ?", owner, track).
		First(&tr).Error
	if err == nil {
		return &tr, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	tr = models.ResearcherTrack{
		OwnerKey:      owner,
		Track:         track,
		DeviceID:      deviceID,
		AccountID:     accountID,
		XP:            0,
		Level:         0,
		ConfigVersion: GrowthConfigVersion,
	}
	if err := tx.Create(&tr).Error; err != nil {
		if isUniqueViolation(err) {
			if ferr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("owner_key = ? AND track = ?", owner, track).
				First(&tr).Error; ferr == nil {
				return &tr, nil
			}
		}
		return nil, err
	}
	return &tr, nil
}

func listTracksTx(tx *gorm.DB, owner string) ([]models.ResearcherTrack, error) {
	var rows []models.ResearcherTrack
	if err := tx.Where("owner_key = ?", owner).Find(&rows).Error; err != nil {
		return nil, err
	}
	by := map[string]models.ResearcherTrack{}
	for _, r := range rows {
		by[r.Track] = r
	}
	out := make([]models.ResearcherTrack, 0, 3)
	for _, t := range AllResearcherTracks() {
		if v, ok := by[t]; ok {
			out = append(out, v)
		} else {
			out = append(out, models.ResearcherTrack{
				OwnerKey:      owner,
				Track:         t,
				XP:            0,
				Level:         0,
				ConfigVersion: GrowthConfigVersion,
			})
		}
	}
	return out, nil
}

// ensureCompanion 创建伙伴档案并保证 ≥3 可见节点。
func ensureCompanion(tx *gorm.DB, owner, deviceID, accountID, animalUUID string) (*models.CompanionProfile, []models.CompanionMemoryNode, error) {
	var comp models.CompanionProfile
	err := tx.Where("animal_uuid = ?", animalUUID).First(&comp).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, err
		}
		comp = models.CompanionProfile{
			AnimalUUID:    animalUUID,
			OwnerKey:      owner,
			DeviceID:      deviceID,
			AccountID:     accountID,
			BondXP:        0,
			BondLevel:     0,
			DecorStage:    0,
			Title:         "新朋友",
			ConfigVersion: GrowthConfigVersion,
		}
		if err := tx.Create(&comp).Error; err != nil {
			if isUniqueViolation(err) {
				if ferr := tx.Where("animal_uuid = ?", animalUUID).First(&comp).Error; ferr != nil {
					return nil, nil, ferr
				}
			} else {
				return nil, nil, err
			}
		}
	}
	// 确保节点
	nodes, err := ensureCompanionNodes(tx, &comp)
	if err != nil {
		return nil, nil, err
	}
	// 0 XP 时解锁 first_meeting
	unlocked, nodes, err := unlockCompanionNodes(tx, &comp)
	if err != nil {
		return nil, nil, err
	}
	_ = unlocked
	return &comp, nodes, nil
}

func ensureCompanionNodes(tx *gorm.DB, comp *models.CompanionProfile) ([]models.CompanionMemoryNode, error) {
	var existing []models.CompanionMemoryNode
	if err := tx.Where("animal_uuid = ?", comp.AnimalUUID).Find(&existing).Error; err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, n := range existing {
		have[n.NodeID] = true
	}
	for _, def := range CompanionNodeCatalog {
		if have[def.NodeID] {
			continue
		}
		n := models.CompanionMemoryNode{
			AnimalUUID: comp.AnimalUUID,
			NodeID:     def.NodeID,
			OwnerKey:   comp.OwnerKey,
			Title:      def.Title,
			Kind:       def.Kind,
			Visible:    true, // 全部可见（验收：≥3 可见）
			Unlocked:   false,
			UnlockAtXP: def.UnlockAtXP,
		}
		if err := tx.Create(&n).Error; err != nil && !isUniqueViolation(err) {
			return nil, err
		}
	}
	var nodes []models.CompanionMemoryNode
	if err := tx.Where("animal_uuid = ?", comp.AnimalUUID).Order("unlock_at_xp asc").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

func unlockCompanionNodes(tx *gorm.DB, comp *models.CompanionProfile) ([]string, []models.CompanionMemoryNode, error) {
	var nodes []models.CompanionMemoryNode
	if err := tx.Where("animal_uuid = ?", comp.AnimalUUID).Order("unlock_at_xp asc").Find(&nodes).Error; err != nil {
		return nil, nil, err
	}
	var unlocked []string
	now := time.Now().UTC()
	for i := range nodes {
		n := &nodes[i]
		if !n.Unlocked && comp.BondXP >= n.UnlockAtXP {
			n.Unlocked = true
			t := now
			n.UnlockedAt = &t
			if err := tx.Save(n).Error; err != nil {
				return nil, nil, err
			}
			unlocked = append(unlocked, n.NodeID)
		}
	}
	// 刷新
	if err := tx.Where("animal_uuid = ?", comp.AnimalUUID).Order("unlock_at_xp asc").Find(&nodes).Error; err != nil {
		return unlocked, nil, err
	}
	return unlocked, nodes, nil
}

// GetResearcher 返回三轨道快照。
func (r *GrowthRepo) GetResearcher(accountID, deviceID string) ([]models.ResearcherTrack, error) {
	if r == nil || r.db == nil {
		return nil, ErrGrowthRepoUnavailable
	}
	owner := OwnerKey(accountID, deviceID)
	return listTracksTx(r.db, owner)
}

// ListEvents 分页列出成长事件（新→旧）。
func (r *GrowthRepo) ListEvents(accountID, deviceID string, afterID uint, limit int) ([]models.GrowthEvent, error) {
	if r == nil || r.db == nil {
		return nil, ErrGrowthRepoUnavailable
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	owner := OwnerKey(accountID, deviceID)
	var rows []models.GrowthEvent
	q := r.db.Where("owner_key = ?", owner)
	if afterID > 0 {
		q = q.Where("id < ?", afterID)
	}
	err := q.Order("id desc").Limit(limit).Find(&rows).Error
	return rows, err
}

// GetCompanion 获取伙伴档案 + 节点；不存在则按所有权创建。
func (r *GrowthRepo) GetCompanion(accountID, deviceID, animalUUID string) (*models.CompanionProfile, []models.CompanionMemoryNode, error) {
	if r == nil || r.db == nil {
		return nil, nil, ErrGrowthRepoUnavailable
	}
	animalUUID = strings.TrimSpace(animalUUID)
	if animalUUID == "" {
		return nil, nil, ErrGrowthInvalidEvent
	}
	owner := OwnerKey(accountID, deviceID)
	var comp *models.CompanionProfile
	var nodes []models.CompanionMemoryNode
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if _, err := findOwnedAnimal(tx, animalUUID, deviceID, accountID); err != nil {
			return err
		}
		c, ns, err := ensureCompanion(tx, owner, deviceID, accountID, animalUUID)
		if err != nil {
			return err
		}
		comp = c
		nodes = ns
		return nil
	})
	return comp, nodes, err
}

// ListCompanions 列出 owner 的伙伴摘要。
func (r *GrowthRepo) ListCompanions(accountID, deviceID string, limit int) ([]models.CompanionProfile, error) {
	if r == nil || r.db == nil {
		return nil, ErrGrowthRepoUnavailable
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	owner := OwnerKey(accountID, deviceID)
	var rows []models.CompanionProfile
	err := r.db.Where("owner_key = ?", owner).Order("updated_at desc").Limit(limit).Find(&rows).Error
	return rows, err
}

// CatalogResponse 配置目录（客户端渲染节点树）。
type GrowthCatalog struct {
	ConfigVersion string              `json:"config_version"`
	Tracks        []CatalogTrack      `json:"tracks"`
	Events        []CatalogEvent      `json:"events"`
	CompanionNodes []CompanionNodeDef `json:"companion_nodes"`
	Rules         CatalogRules        `json:"rules"`
	LevelThresholds CatalogThresholds `json:"level_thresholds"`
}

type CatalogTrack struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type CatalogEvent struct {
	Kind    string `json:"kind"`
	Track   string `json:"track"`
	DeltaXP int64  `json:"delta_xp"`
}

type CatalogRules struct {
	MinVisibleNodesPerCompanion int  `json:"min_visible_nodes_per_companion"`
	NoDecay                     bool `json:"no_decay"`
	NoRealWorldFeeding          bool `json:"no_real_world_feeding"`
	NoPaidPower                 bool `json:"no_paid_power"`
	CombatStatsUnchanged        bool `json:"combat_stats_unchanged"`
	RecognitionUnchanged        bool `json:"recognition_unchanged"`
	CrossDeviceAuthoritative    bool `json:"cross_device_authoritative"`
}

type CatalogThresholds struct {
	Researcher []int64 `json:"researcher"`
	Companion  []int64 `json:"companion"`
}

// Catalog 返回版本化成长目录。
func Catalog() GrowthCatalog {
	events := make([]CatalogEvent, 0, len(AllowedGrowthEvents))
	for _, s := range AllowedGrowthEvents {
		events = append(events, CatalogEvent{Kind: s.Kind, Track: s.Track, DeltaXP: s.DeltaXP})
	}
	return GrowthCatalog{
		ConfigVersion: GrowthConfigVersion,
		Tracks: []CatalogTrack{
			{ID: models.GrowthTrackPhotography, Title: "摄影"},
			{ID: models.GrowthTrackEcology, Title: "生态知识"},
			{ID: models.GrowthTrackSafeObservation, Title: "安全观察"},
		},
		Events:         events,
		CompanionNodes: CompanionNodeCatalog,
		Rules: CatalogRules{
			MinVisibleNodesPerCompanion: 3,
			NoDecay:                     true,
			NoRealWorldFeeding:          true,
			NoPaidPower:                 true,
			CombatStatsUnchanged:        true,
			RecognitionUnchanged:        true,
			CrossDeviceAuthoritative:    true,
		},
		LevelThresholds: CatalogThresholds{
			Researcher: append([]int64(nil), researcherLevelThresholds...),
			Companion:  append([]int64(nil), companionLevelThresholds...),
		},
	}
}

// ResetScope 重置范围。
type ResetScope string

const (
	ResetScopeResearcher ResetScope = "researcher"
	ResetScopeCompanion  ResetScope = "companion"
	ResetScopeAll        ResetScope = "all"
)

// ResetRequest 可审计重置。
type ResetRequest struct {
	DeviceID   string
	AccountID  string
	Scope      ResetScope
	AnimalUUID string
	Reason     string
	ToVersion  string
}

// Reset 重置成长并写审计快照（不删事件流水，保证可审计）。
func (r *GrowthRepo) Reset(req ResetRequest) (*models.GrowthResetAudit, error) {
	if r == nil || r.db == nil {
		return nil, ErrGrowthRepoUnavailable
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: reason required", ErrGrowthInvalidEvent)
	}
	scope := req.Scope
	if scope == "" {
		scope = ResetScopeAll
	}
	if scope != ResetScopeResearcher && scope != ResetScopeCompanion && scope != ResetScopeAll {
		return nil, fmt.Errorf("%w: invalid scope", ErrGrowthInvalidEvent)
	}
	owner := OwnerKey(req.AccountID, req.DeviceID)
	toVer := strings.TrimSpace(req.ToVersion)
	if toVer == "" {
		toVer = GrowthConfigVersion
	}

	var audit *models.GrowthResetAudit
	err := r.db.Transaction(func(tx *gorm.DB) error {
		snapshot := map[string]any{}
		if scope == ResetScopeResearcher || scope == ResetScopeAll {
			var tracks []models.ResearcherTrack
			_ = tx.Where("owner_key = ?", owner).Find(&tracks).Error
			snapshot["researcher"] = tracks
			if err := tx.Where("owner_key = ?", owner).Delete(&models.ResearcherTrack{}).Error; err != nil {
				return err
			}
		}
		if scope == ResetScopeCompanion || scope == ResetScopeAll {
			q := tx.Where("owner_key = ?", owner)
			nq := tx.Where("owner_key = ?", owner)
			if scope == ResetScopeCompanion && strings.TrimSpace(req.AnimalUUID) != "" {
				q = q.Where("animal_uuid = ?", req.AnimalUUID)
				nq = nq.Where("animal_uuid = ?", req.AnimalUUID)
			}
			var comps []models.CompanionProfile
			_ = q.Find(&comps).Error
			var nodes []models.CompanionMemoryNode
			_ = nq.Find(&nodes).Error
			snapshot["companions"] = comps
			snapshot["nodes"] = nodes
			if err := q.Delete(&models.CompanionProfile{}).Error; err != nil {
				return err
			}
			if err := nq.Delete(&models.CompanionMemoryNode{}).Error; err != nil {
				return err
			}
		}
		// 序列化快照
		snapJSON := ""
		if b, err := json.Marshal(snapshot); err == nil {
			snapJSON = string(b)
		}
		a := &models.GrowthResetAudit{
			AuditID:      uuid.NewString(),
			OwnerKey:     owner,
			DeviceID:     req.DeviceID,
			AccountID:    req.AccountID,
			Scope:        string(scope),
			AnimalUUID:   strings.TrimSpace(req.AnimalUUID),
			Reason:       reason,
			FromVersion:  GrowthConfigVersion,
			ToVersion:    toVer,
			SnapshotJSON: snapJSON,
			CreatedAt:    time.Now().UTC(),
		}
		if err := tx.Create(a).Error; err != nil {
			return err
		}
		audit = a
		return nil
	})
	return audit, err
}
