// Package questcatalog — AP-096 数据驱动任务目录（首批 ≥24 条安全任务）。
// 进度仅由可信业务事件驱动；禁止 page_view / open_pokedex / safe_explore 等客户端可伪造事件。
package questcatalog

import (
	"encoding/json"
	"fmt"
	"time"
)

// ConfigVersion 当前目录版本（配置回滚对比用）。
const ConfigVersion = "quest-catalog.v1"

// Objective 复合目标中的单条目标。
type Objective struct {
	ID     string `json:"id"`
	Event  string `json:"event"`
	Target int64  `json:"target"`
	// Filter 可选过滤（如 city、species）。
	Filter map[string]string `json:"filter,omitempty"`
}

// Reward 任务奖励（写入 Wallet ledger）。
type Reward struct {
	Gold    int64             `json:"gold,omitempty"`
	Stamina int64             `json:"stamina,omitempty"`
	Items   map[string]int64  `json:"items,omitempty"`
}

// Def 静态任务定义。
type Def struct {
	QuestID       string
	Type          string // main|research|daily|weekly|city|event
	Title         string
	Description   string
	Objectives    []Objective
	Rewards       Reward
	Prerequisites []string
	ResetPolicy   string // none|daily|weekly
	Free          bool
	MinLevel      int
	SortOrder     int
	DurationHours int
	// Event window (optional).
	StartsAt *time.Time
	EndsAt   *time.Time
}

// TrustedEvents 服务端可接受的事件类型白名单。
var TrustedEvents = map[string]struct{}{
	"capture_success":   {},
	"species_new":       {},
	"battle_complete":   {},
	"dispatch_complete": {},
	"visit_city":        {},
	"season_checkin":    {},
	"research_note":     {},
	"collection_count":  {},
}

// ForbiddenEvents 客户端可伪造 / 打开页面类事件，一律拒绝。
var ForbiddenEvents = map[string]struct{}{
	"open_pokedex":  {},
	"safe_explore":  {},
	"page_view":     {},
	"open_map":      {},
	"open_page":     {},
	"ui_open":       {},
	"client_tick":   {},
	"view_goal":     {},
	"reopen_app":    {},
}

// IsTrustedEvent 是否为可信业务事件。
func IsTrustedEvent(event string) bool {
	if _, bad := ForbiddenEvents[event]; bad {
		return false
	}
	_, ok := TrustedEvents[event]
	return ok
}

// All 返回首批安全审查任务目录（≥24）。
func All() []Def {
	return []Def{
		// ---------- main（主线，无重置） ----------
		{
			QuestID: "main_first_capture", Type: "main", Title: "完成首次捕获",
			Description: "用相机识别并收藏你的第一只动物",
			Objectives:  []Objective{{ID: "cap1", Event: "capture_success", Target: 1}},
			Rewards:     Reward{Gold: 30}, ResetPolicy: "none", Free: false, MinLevel: 1, SortOrder: 10,
		},
		{
			QuestID: "main_three_captures", Type: "main", Title: "捕获 3 只动物",
			Description: "累计完成 3 次可信捕获",
			Objectives:  []Objective{{ID: "cap3", Event: "capture_success", Target: 3}},
			Rewards:     Reward{Gold: 50}, Prerequisites: []string{"main_first_capture"},
			ResetPolicy: "none", Free: false, MinLevel: 1, SortOrder: 20,
		},
		{
			QuestID: "main_first_species", Type: "main", Title: "解锁新物种",
			Description: "收藏一个新物种",
			Objectives:  []Objective{{ID: "sp1", Event: "species_new", Target: 1}},
			Rewards:     Reward{Gold: 40}, Prerequisites: []string{"main_first_capture"},
			ResetPolicy: "none", Free: false, MinLevel: 1, SortOrder: 30,
		},
		{
			QuestID: "main_first_battle", Type: "main", Title: "完成一场战斗",
			Description: "完成 1 场服务端确认的对战",
			Objectives:  []Objective{{ID: "bat1", Event: "battle_complete", Target: 1}},
			Rewards:     Reward{Gold: 50}, Prerequisites: []string{"main_three_captures"},
			ResetPolicy: "none", Free: false, MinLevel: 2, SortOrder: 40,
		},
		{
			QuestID: "main_first_dispatch", Type: "main", Title: "完成一次派遣",
			Description: "派遣并领取服务端确认的结果",
			Objectives:  []Objective{{ID: "dis1", Event: "dispatch_complete", Target: 1}},
			Rewards:     Reward{Gold: 50}, Prerequisites: []string{"main_three_captures"},
			ResetPolicy: "none", Free: false, MinLevel: 3, SortOrder: 50,
		},
		{
			QuestID: "main_collection_10", Type: "main", Title: "收藏 10 只动物",
			Description: "长期收集目标",
			Objectives:  []Objective{{ID: "col10", Event: "capture_success", Target: 10}},
			Rewards:     Reward{Gold: 120}, Prerequisites: []string{"main_three_captures"},
			ResetPolicy: "none", Free: false, MinLevel: 1, SortOrder: 60,
		},

		// ---------- research（研究） ----------
		{
			QuestID: "research_three_species", Type: "research", Title: "研究 3 个物种",
			Description: "至少解锁 3 个不同物种",
			Objectives:  []Objective{{ID: "sp3", Event: "species_new", Target: 3}},
			Rewards:     Reward{Gold: 80}, ResetPolicy: "none", Free: false, MinLevel: 1, SortOrder: 110,
		},
		{
			QuestID: "research_note_3", Type: "research", Title: "提交 3 条研究笔记",
			Description: "服务端确认的研究笔记（非打开页面）",
			Objectives:  []Objective{{ID: "note3", Event: "research_note", Target: 3}},
			Rewards:     Reward{Gold: 45}, ResetPolicy: "none", Free: true, MinLevel: 1, SortOrder: 120,
		},
		{
			QuestID: "research_compound_cap_note", Type: "research", Title: "捕获并记录",
			Description: "复合目标：捕获 2 次 + 研究笔记 1 次",
			Objectives: []Objective{
				{ID: "cap2", Event: "capture_success", Target: 2},
				{ID: "note1", Event: "research_note", Target: 1},
			},
			Rewards: Reward{Gold: 70}, ResetPolicy: "none", Free: false, MinLevel: 1, SortOrder: 130,
		},
		{
			QuestID: "research_collection_count_5", Type: "research", Title: "图鉴研究进度",
			Description: "服务端 collection_count 事件推进（不可伪造打开图鉴）",
			Objectives:  []Objective{{ID: "cc5", Event: "collection_count", Target: 5}},
			Rewards:     Reward{Gold: 60}, ResetPolicy: "none", Free: true, MinLevel: 1, SortOrder: 140,
		},

		// ---------- daily（每日，零体力保底含 free） ----------
		{
			QuestID: "daily_capture", Type: "daily", Title: "今日捕获 1 次",
			Description: "完成一次可信捕获",
			Objectives:  []Objective{{ID: "dcap", Event: "capture_success", Target: 1}},
			Rewards:     Reward{Gold: 15}, ResetPolicy: "daily", Free: false, MinLevel: 1, SortOrder: 210,
		},
		{
			QuestID: "daily_capture_3", Type: "daily", Title: "今日捕获 3 次",
			Description: "完成三次可信捕获",
			Objectives:  []Objective{{ID: "dcap3", Event: "capture_success", Target: 3}},
			Rewards:     Reward{Gold: 35}, ResetPolicy: "daily", Free: false, MinLevel: 1, SortOrder: 220,
		},
		{
			QuestID: "daily_battle", Type: "daily", Title: "今日战斗 1 场",
			Description: "完成一场战斗",
			Objectives:  []Objective{{ID: "dbat", Event: "battle_complete", Target: 1}},
			Rewards:     Reward{Gold: 20}, ResetPolicy: "daily", Free: false, MinLevel: 2, SortOrder: 230,
		},
		{
			QuestID: "daily_season_checkin", Type: "daily", Title: "赛季签到",
			Description: "零体力免费任务：服务端日签（非打开页面）",
			Objectives:  []Objective{{ID: "dchk", Event: "season_checkin", Target: 1}},
			Rewards:     Reward{Gold: 8}, ResetPolicy: "daily", Free: true, MinLevel: 1, SortOrder: 240,
		},
		{
			QuestID: "daily_research_note", Type: "daily", Title: "今日研究笔记",
			Description: "零体力：提交 1 条研究笔记",
			Objectives:  []Objective{{ID: "dnote", Event: "research_note", Target: 1}},
			Rewards:     Reward{Gold: 6}, ResetPolicy: "daily", Free: true, MinLevel: 1, SortOrder: 250,
		},
		{
			QuestID: "daily_visit_city", Type: "daily", Title: "今日造访城市",
			Description: "服务端记录的城市到访（需可信 visit_city）",
			Objectives:  []Objective{{ID: "dcity", Event: "visit_city", Target: 1}},
			Rewards:     Reward{Gold: 10}, ResetPolicy: "daily", Free: true, MinLevel: 1, SortOrder: 260,
		},

		// ---------- weekly ----------
		{
			QuestID: "weekly_capture_10", Type: "weekly", Title: "本周捕获 10 次",
			Description: "累计 10 次可信捕获",
			Objectives:  []Objective{{ID: "wcap10", Event: "capture_success", Target: 10}},
			Rewards:     Reward{Gold: 100}, ResetPolicy: "weekly", Free: false, MinLevel: 1, SortOrder: 310,
		},
		{
			QuestID: "weekly_species_2", Type: "weekly", Title: "本周新物种 2 个",
			Description: "解锁 2 个新物种",
			Objectives:  []Objective{{ID: "wsp2", Event: "species_new", Target: 2}},
			Rewards:     Reward{Gold: 80}, ResetPolicy: "weekly", Free: false, MinLevel: 1, SortOrder: 320,
		},
		{
			QuestID: "weekly_battle_3", Type: "weekly", Title: "本周战斗 3 场",
			Description: "完成 3 场战斗",
			Objectives:  []Objective{{ID: "wbat3", Event: "battle_complete", Target: 3}},
			Rewards:     Reward{Gold: 90}, ResetPolicy: "weekly", Free: false, MinLevel: 2, SortOrder: 330,
		},
		{
			QuestID: "weekly_checkin_5", Type: "weekly", Title: "本周签到 5 次",
			Description: "零体力：赛季签到 5 次",
			Objectives:  []Objective{{ID: "wchk5", Event: "season_checkin", Target: 5}},
			Rewards:     Reward{Gold: 40}, ResetPolicy: "weekly", Free: true, MinLevel: 1, SortOrder: 340,
		},

		// ---------- city ----------
		{
			QuestID: "city_visit_3", Type: "city", Title: "造访 3 座城市",
			Description: "可信 visit_city 累计 3 次（跨城市去重由事件 payload 控制）",
			Objectives:  []Objective{{ID: "city3", Event: "visit_city", Target: 3}},
			Rewards:     Reward{Gold: 120}, ResetPolicy: "none", Free: true, MinLevel: 1, SortOrder: 410,
		},
		{
			QuestID: "city_capture_in_city", Type: "city", Title: "城市捕获组合",
			Description: "复合：捕获 2 + 造访城市 1",
			Objectives: []Objective{
				{ID: "ccap", Event: "capture_success", Target: 2},
				{ID: "cvis", Event: "visit_city", Target: 1},
			},
			Rewards: Reward{Gold: 55}, ResetPolicy: "none", Free: false, MinLevel: 1, SortOrder: 420,
		},

		// ---------- event（活动；默认始终开放窗口由 EndsAt 控制，nil=长期） ----------
		{
			QuestID: "event_welcome_capture", Type: "event", Title: "欢迎活动：捕获",
			Description: "活动期间完成 2 次捕获",
			Objectives:  []Objective{{ID: "ecap2", Event: "capture_success", Target: 2}},
			Rewards:     Reward{Gold: 25, Stamina: 10}, ResetPolicy: "none", Free: false, MinLevel: 1, SortOrder: 510,
			DurationHours: 72,
		},
		{
			QuestID: "event_welcome_checkin", Type: "event", Title: "欢迎活动：签到",
			Description: "活动期间完成 1 次赛季签到（免费）",
			Objectives:  []Objective{{ID: "echk", Event: "season_checkin", Target: 1}},
			Rewards:     Reward{Gold: 15}, ResetPolicy: "none", Free: true, MinLevel: 1, SortOrder: 520,
			DurationHours: 72,
		},
	}
}

// MustJSON helpers for seeding.
func ObjectivesJSON(objs []Objective) string {
	b, err := json.Marshal(objs)
	if err != nil {
		panic(fmt.Sprintf("objectives json: %v", err))
	}
	return string(b)
}

func RewardsJSON(r Reward) string {
	b, err := json.Marshal(r)
	if err != nil {
		panic(fmt.Sprintf("rewards json: %v", err))
	}
	return string(b)
}

func PrerequisitesJSON(p []string) string {
	if len(p) == 0 {
		return "[]"
	}
	b, err := json.Marshal(p)
	if err != nil {
		panic(fmt.Sprintf("prereq json: %v", err))
	}
	return string(b)
}

// Count returns catalog size.
func Count() int { return len(All()) }

// ByID looks up a definition.
func ByID(id string) (Def, bool) {
	for _, d := range All() {
		if d.QuestID == id {
			return d, true
		}
	}
	return Def{}, false
}

// FreeExecutable returns free quests (zero-stamina path).
func FreeExecutable() []Def {
	out := make([]Def, 0)
	for _, d := range All() {
		if d.Free {
			out = append(out, d)
		}
	}
	return out
}

// ValidateGraph 检查前置可达性（简单：前置必须存在且无自环）。
func ValidateGraph() error {
	ids := map[string]struct{}{}
	for _, d := range All() {
		if _, dup := ids[d.QuestID]; dup {
			return fmt.Errorf("duplicate quest_id %s", d.QuestID)
		}
		ids[d.QuestID] = struct{}{}
		if len(d.Objectives) == 0 {
			return fmt.Errorf("quest %s has no objectives", d.QuestID)
		}
		for _, o := range d.Objectives {
			if !IsTrustedEvent(o.Event) {
				return fmt.Errorf("quest %s objective %s uses untrusted event %s", d.QuestID, o.ID, o.Event)
			}
			if o.Target <= 0 {
				return fmt.Errorf("quest %s objective %s target must be >0", d.QuestID, o.ID)
			}
		}
		if d.Rewards.Gold < 0 || d.Rewards.Stamina < 0 {
			return fmt.Errorf("quest %s negative reward", d.QuestID)
		}
	}
	for _, d := range All() {
		for _, p := range d.Prerequisites {
			if _, ok := ids[p]; !ok {
				return fmt.Errorf("quest %s missing prerequisite %s", d.QuestID, p)
			}
			if p == d.QuestID {
				return fmt.Errorf("quest %s self prerequisite", d.QuestID)
			}
		}
	}
	if Count() < 24 {
		return fmt.Errorf("catalog size %d < 24", Count())
	}
	if len(FreeExecutable()) == 0 {
		return fmt.Errorf("no free quests for zero-stamina path")
	}
	return nil
}
