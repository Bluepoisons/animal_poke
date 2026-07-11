package narrativecatalog

// QAAnnotation supplies authored QA metadata for one narrative node. It is
// intentionally separate from runtime state: lint-only fields must not change
// a player's progress or choice resolution.
type QAAnnotation struct {
	Locale            string
	Summary           string
	EthicsLabels      []string
	AssetLicense      string
	Subtitle          string
	VoiceAsset        string
	Timeline          int
	KnowledgeProvides []string
	KnowledgeRequires []string
	Terminal          bool
}

// QAEntryNodes lists every supported authored entry route. Fail-forward routes
// have separate entries because they are selected from observation conditions,
// rather than from a preceding choice edge.
func QAEntryNodes() []string {
	return []string{
		"ch1_intro",
		"ch3_rain_eaves",
		"ch4_map_blank",
		"ff_miss_1",
		"ff_miss_2",
		"ff_miss_3",
		"ff_no_camera",
		"ff_weather",
	}
}

func qa(summary string, timeline int, terminal bool) QAAnnotation {
	return QAAnnotation{
		Locale:       "zh-CN",
		Summary:      summary,
		EthicsLabels: []string{"fictional", "animal_welfare"},
		AssetLicense: "internal-authored",
		Timeline:     timeline,
		Terminal:     terminal,
	}
}

// QAAnnotations is the auditable metadata manifest for every authored node.
// All current material is text-only; Subtitle and VoiceAsset stay empty until
// a voiced asset is introduced, when the QA gate requires them as a pair.
func QAAnnotations() map[string]QAAnnotation {
	return map[string]QAAnnotation{
		"ch1_intro":            qa("玩家开始记录城市观察。", 100, false),
		"ch1_choice_pace":      qa("玩家选择快速记录或继续观察。", 101, false),
		"ch1_after_quick":      qa("快速记录后留下下一次相遇的空间。", 102, false),
		"ch1_after_slow":       qa("耐心观察让问题先于答案出现。", 102, false),
		"ch1_checkpoint":       qa("第一章以问题和手账检查点收束。", 103, true),
		"ff_miss_1":            qa("第一次未观察到动物仍形成记录。", 200, true),
		"ff_miss_2":            qa("连续落空仍可通过旧笔记推进。", 200, true),
		"ff_miss_3":            qa("动物缺席被解释为季节线索。", 200, true),
		"ff_no_camera":         qa("没有相机时可通过访谈和声音记录推进。", 200, true),
		"ff_weather":           qa("恶劣天气时可安全地整理档案。", 200, true),
		"ch3_rain_eaves":       qa("相互矛盾的线索引出不确定性。", 300, false),
		"ch3_choice_judge":     qa("玩家选择暂缓或暂定判断。", 301, false),
		"ch3_hold_judgment":    qa("玩家将线索保持为待定。", 302, false),
		"ch3_quick_judge":      qa("玩家记录带来源标注的暂定结论。", 302, true),
		"ch3_evidence_view":    qa("不同证据来源被分栏展示。", 303, false),
		"ch3_npc_testimony":    qa("角色证词与模型推测并列且不互相覆盖。", 304, true),
		"ch4_map_blank":        qa("地图空白成为城市变化的线索。", 400, false),
		"ch4_zero_capture":     qa("零捕获路线仍可构成完整篇章。", 401, false),
		"ch4_ending_materials": qa("玩家为展览收集非稀有素材。", 402, false),
		"ch4_season_archive":   qa("季节档案为缺席提供多种解释。", 403, false),
		"ch4_city_change":      qa("城市改造改变地图与角色态度。", 404, false),
		"ch4_pace_echo":        qa("第一章的节奏选择在后续产生回响。", 405, false),
		"ch4_exhibit_three":    qa("三类平等可得的材料构成展览结局。", 406, true),
	}
}
