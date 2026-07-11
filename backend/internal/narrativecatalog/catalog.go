// Package narrativecatalog 数据驱动剧情内容（AP-132/118/119/120/127/128）。
// 主线 canon 仅 authored；禁止页面硬编码章节条件。
package narrativecatalog

import "encoding/json"

// ContentVersion 当前内容包版本。
const ContentVersion = "narrative-v1"

// NodeDef 节点定义。
type NodeDef struct {
	NodeID       string
	ChapterID    string
	Title        string
	Body         string
	Kind         string // story|choice|fragment|fail_forward|ending
	SafeFallback string
	Tags         []string
	Priority     int
}

// ChoiceDef 选择边。
type ChoiceDef struct {
	ChoiceID   string
	FromNodeID string
	ToNodeID   string
	Label      string
	Prompt     string
	Effects    map[string]any // flags / relationships deltas
	SortOrder  int
}

// FragmentDef 观察碎片。
type FragmentDef struct {
	FragmentID    string
	Title         string
	Body          string
	Priority      int
	Triggers      map[string]any // observation_count, first_species, weather, etc.
	MutexGroup    string
	CooldownHours int
	FallbackID    string
	ReasonHint    string // 为何解锁
}

// SeedNodes 首批节点（含 ch1 入口、fail-forward、ch3/ch4 骨架）。
func SeedNodes() []NodeDef {
	return []NodeDef{
		{NodeID: "ch1_intro", ChapterID: "ch1", Title: "开篇：手账的第一页", Body: "你打开新的观察手账。城市里的猫狗鹅不需要英雄——它们需要被温柔地看见。", Kind: "story", Priority: 100},
		{NodeID: "ch1_choice_pace", ChapterID: "ch1", Title: "今天的节奏", Body: "傍晚光线很好。你是想尽快记一条，还是多看一会儿？", Kind: "choice", Priority: 90},
		{NodeID: "ch1_after_quick", ChapterID: "ch1", Title: "快速记录", Body: "你写下简短笔记。手账边缘留白，像是在等下一次相遇。", Kind: "story", Priority: 80},
		{NodeID: "ch1_after_slow", ChapterID: "ch1", Title: "继续观察", Body: "你没有急着按下快门。远处屋檐下有影子移动，问题比答案先到。", Kind: "story", Priority: 80},
		{NodeID: "ch1_checkpoint", ChapterID: "ch1", Title: "第一章检查点", Body: "手账首页已写。无论拍到与否，你都带着问题离开。", Kind: "story", SafeFallback: "ch1_intro", Priority: 70},

		// fail-forward
		{NodeID: "ff_miss_1", ChapterID: "fail_forward", Title: "第一次落空", Body: "镜头里没有目标。空白本身也是记录：这一刻，这座城市没有把动物交给你。", Kind: "fail_forward", Priority: 50},
		{NodeID: "ff_miss_2", ChapterID: "fail_forward", Title: "连续落空", Body: "你翻出旧手账边缘的铅笔痕。有人在同一条路上走过，留下的是问题而不是答案。", Kind: "fail_forward", Priority: 51},
		{NodeID: "ff_miss_3", ChapterID: "fail_forward", Title: "缺席也是线索", Body: "第三次无有效观察。NPC 林阿姨说：雨前的猫会换地方。缺席，也在讲述季节。", Kind: "fail_forward", Priority: 52},
		{NodeID: "ff_no_camera", ChapterID: "fail_forward", Title: "没有相机时", Body: "没有相机也能做研究员：整理旧照片、访谈邻居、在窗边记录声音。", Kind: "fail_forward", Priority: 53},
		{NodeID: "ff_weather", ChapterID: "fail_forward", Title: "恶劣天气", Body: "风雨暂停了外出。你打开档案柜，发现去年同一天的空白页——季节在重复提问。", Kind: "fail_forward", Priority: 54},

		// chapter 3 unreliable narration
		{NodeID: "ch3_rain_eaves", ChapterID: "ch3", Title: "第三章：雨落屋檐", Body: "雨落屋檐。模型说「猫」，邻居说「只是塑料袋」，你自己的记录写着「不确定」。", Kind: "story", Priority: 60},
		{NodeID: "ch3_choice_judge", ChapterID: "ch3", Title: "如何处理矛盾证词", Body: "你要现在下结论，还是保留判断、等待更多证据？", Kind: "choice", Priority: 59},
		{NodeID: "ch3_hold_judgment", ChapterID: "ch3", Title: "保留判断", Body: "你把线索标为 pending。手账页脚写：未知比错误更诚实。", Kind: "story", Priority: 58},
		{NodeID: "ch3_quick_judge", ChapterID: "ch3", Title: "暂定结论", Body: "你写下暂定结论，并标注来源为「推测」。后续章节会回响这个决定。", Kind: "story", Priority: 58},
		{NodeID: "ch3_evidence_view", ChapterID: "ch3", Title: "证据来源视图", Body: "线索状态：unknown / pending / confirmed / disputed。NPC 证词、模型推测与玩家观察分栏显示，互不覆盖。", Kind: "story", Priority: 57},
		{NodeID: "ch3_npc_testimony", ChapterID: "ch3", Title: "林阿姨的说法", Body: "「那不是猫，是风吹的布。」她很肯定。模型输出仍写着 cat（低置信，已标推测）。", Kind: "story", Priority: 56},

		// chapter 4 absence
		{NodeID: "ch4_map_blank", ChapterID: "ch4", Title: "第四章：地图上的空白", Body: "地图上有一格空白。不是 bug，是城市改造与季节迁徙留下的空位。", Kind: "story", Priority: 40},
		{NodeID: "ch4_zero_capture", ChapterID: "ch4", Title: "零捕获路线", Body: "这一章你没有新增捕获。旧照片、空白记录和角色态度构成完整篇章。", Kind: "story", Priority: 39},
		{NodeID: "ch4_ending_materials", ChapterID: "ch4", Title: "为展览准备材料", Body: "你收集到至少三类材料：空白页注释、角色回响、季节对照。终章展览可选它们。", Kind: "ending", Priority: 38},
		{NodeID: "ch4_pace_echo", ChapterID: "ch4", Title: "跨章回响：节奏", Body: "若你在第一章选择了缓慢观察，角色会记得你的耐心；若选择效率，手账语气更短促。这是延迟后果，不是对错分。", Kind: "story", Priority: 37},
		{NodeID: "ch4_season_archive", ChapterID: "ch4", Title: "季节档案", Body: "旧照片显示去年此时也有空白。空白有解释：迁徙、施工、或你改了出门时间。", Kind: "story", Priority: 36},
		{NodeID: "ch4_city_change", ChapterID: "ch4", Title: "城市改造", Body: "地图上的缺口对应新围挡。角色态度随前三章选择变化，但主线仍汇合于此。", Kind: "story", Priority: 35},
		{NodeID: "ch4_exhibit_three", ChapterID: "ch4", Title: "三类展览素材", Body: "可选材料：1) 空白页注释 2) 角色回响 3) 季节对照。无稀有动物门槛。", Kind: "ending", Priority: 34},
	}
}

// SeedChoices 选择边。
func SeedChoices() []ChoiceDef {
	return []ChoiceDef{
		{ChoiceID: "ch1_pace_quick", FromNodeID: "ch1_intro", ToNodeID: "ch1_after_quick", Label: "尽快记一条", Prompt: "效率优先", Effects: map[string]any{"flag:pace": "quick", "rel:self_trust": 0}, SortOrder: 1},
		// fix: choices from choice node
		{ChoiceID: "ch1_c_quick", FromNodeID: "ch1_choice_pace", ToNodeID: "ch1_after_quick", Label: "尽快记一条", Effects: map[string]any{"flag:pace": "quick"}, SortOrder: 1},
		{ChoiceID: "ch1_c_slow", FromNodeID: "ch1_choice_pace", ToNodeID: "ch1_after_slow", Label: "再观察一会儿", Effects: map[string]any{"flag:pace": "slow", "rel:patience": 1}, SortOrder: 2},
		{ChoiceID: "ch3_hold", FromNodeID: "ch3_choice_judge", ToNodeID: "ch3_hold_judgment", Label: "保留判断", Effects: map[string]any{"flag:judgment": "hold", "clue:rain_shadow": "pending"}, SortOrder: 1},
		{ChoiceID: "ch3_decide", FromNodeID: "ch3_choice_judge", ToNodeID: "ch3_quick_judge", Label: "作暂定结论", Effects: map[string]any{"flag:judgment": "tentative", "clue:rain_shadow": "disputed"}, SortOrder: 2},
		{ChoiceID: "ch3_to_evidence", FromNodeID: "ch3_hold_judgment", ToNodeID: "ch3_evidence_view", Label: "查看证据来源", Effects: map[string]any{"flag:opened_evidence": true}, SortOrder: 1},
		{ChoiceID: "ch3_to_npc", FromNodeID: "ch3_evidence_view", ToNodeID: "ch3_npc_testimony", Label: "对照邻居证词", Effects: map[string]any{}, SortOrder: 1},
		// auto edges as single-option choices for linear beats
		{ChoiceID: "ch1_intro_next", FromNodeID: "ch1_intro", ToNodeID: "ch1_choice_pace", Label: "继续", Effects: map[string]any{}, SortOrder: 1},
		{ChoiceID: "ch1_quick_next", FromNodeID: "ch1_after_quick", ToNodeID: "ch1_checkpoint", Label: "合上这一页", Effects: map[string]any{}, SortOrder: 1},
		{ChoiceID: "ch1_slow_next", FromNodeID: "ch1_after_slow", ToNodeID: "ch1_checkpoint", Label: "合上这一页", Effects: map[string]any{}, SortOrder: 1},
		{ChoiceID: "ch3_open_choice", FromNodeID: "ch3_rain_eaves", ToNodeID: "ch3_choice_judge", Label: "面对矛盾证词", Effects: map[string]any{}, SortOrder: 1},
		{ChoiceID: "ch4_to_zero", FromNodeID: "ch4_map_blank", ToNodeID: "ch4_zero_capture", Label: "记录空白", Effects: map[string]any{"flag:zero_capture_route": true}, SortOrder: 1},
		{ChoiceID: "ch4_to_end", FromNodeID: "ch4_zero_capture", ToNodeID: "ch4_ending_materials", Label: "整理展览材料", Effects: map[string]any{}, SortOrder: 1},
		{ChoiceID: "ch4_to_season", FromNodeID: "ch4_ending_materials", ToNodeID: "ch4_season_archive", Label: "打开季节档案", Effects: map[string]any{}, SortOrder: 1},
		{ChoiceID: "ch4_to_city", FromNodeID: "ch4_season_archive", ToNodeID: "ch4_city_change", Label: "对照城市改造", Effects: map[string]any{}, SortOrder: 1},
		{ChoiceID: "ch4_to_exhibit", FromNodeID: "ch4_city_change", ToNodeID: "ch4_exhibit_three", Label: "选定展览素材", Effects: map[string]any{"flag:exhibit_ready": true}, SortOrder: 1},
	}
}

// SeedFragments 观察碎片。
func SeedFragments() []FragmentDef {
	return []FragmentDef{
		{FragmentID: "frag_first_any", Title: "第一次看见", Body: "第一只被记入手账的动物。故事从这里分叉，而不是从稀有度。", Priority: 100, Triggers: map[string]any{"min_observations": 1}, ReasonHint: "完成首次有效观察"},
		{FragmentID: "frag_obs_3", Title: "三页笔记", Body: "三次观察后，你开始比较天气与出现规律。", Priority: 90, Triggers: map[string]any{"min_observations": 3}, ReasonHint: "累计观察达到 3"},
		{FragmentID: "frag_obs_6", Title: "重复的身影", Body: "重复观察同一物种，手账语气从兴奋变成询问。", Priority: 80, Triggers: map[string]any{"min_observations": 6}, ReasonHint: "累计观察达到 6"},
		{FragmentID: "frag_first_cat", Title: "猫的页边", Body: "首次猫科观察：邻居说它「总在雨前消失」。", Priority: 85, Triggers: map[string]any{"first_species": "cat"}, ReasonHint: "首次观察猫", FallbackID: "frag_first_any"},
		{FragmentID: "frag_first_dog", Title: "狗的页边", Body: "首次犬科观察：晨间与黄昏的路线不同。", Priority: 85, Triggers: map[string]any{"first_species": "dog"}, ReasonHint: "首次观察狗", FallbackID: "frag_first_any"},
		{FragmentID: "frag_weather_rain", Title: "雨天旁证", Body: "雨天观察：缺席本身成为下一章的引子。", Priority: 70, Triggers: map[string]any{"weather": "rain"}, ReasonHint: "雨天触发", CooldownHours: 24},
		{FragmentID: "frag_combo_urban", Title: "城市生态拼图", Body: "猫与狗的组合笔记让你看见同一条街的不同时间层。", Priority: 75, Triggers: map[string]any{"species_set": []string{"cat", "dog"}}, ReasonHint: "跨物种组合"},
		{FragmentID: "frag_obs_10", Title: "十次观察", Body: "里程碑：十次观察后，手账开始比较季节而不是只记物种。", Priority: 65, Triggers: map[string]any{"min_observations": 10}, ReasonHint: "累计观察达到 10"},
		{FragmentID: "frag_obs_15", Title: "十五页之后", Body: "重复与缺席同样重要——你开始为空白留出注释栏。", Priority: 60, Triggers: map[string]any{"min_observations": 15}, ReasonHint: "累计观察达到 15"},
		{FragmentID: "frag_first_goose", Title: "鹅的页边", Body: "首次鹅科观察：水边的时间感与街巷不同。", Priority: 85, Triggers: map[string]any{"first_species": "goose"}, ReasonHint: "首次观察鹅", FallbackID: "frag_first_any"},
	}
}

// MustJSON 序列化辅助。
func MustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
