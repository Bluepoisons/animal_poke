package battle

// Catalog snapshot helpers — pure data, no I/O.

// Roles returns tank/dps/support/control definitions.
func Roles() []RoleDef {
	return []RoleDef{
		{ID: RoleTank, NameZH: "坦克", NameEN: "Tank", PreferredSlot: SlotFront, HPMod: 1.25, ATKMod: 0.85, DEFMod: 1.30, SPDMod: 0.85, Description: "前排承伤，嘲讽与护盾"},
		{ID: RoleDPS, NameZH: "输出", NameEN: "DPS", PreferredSlot: SlotBack, HPMod: 0.90, ATKMod: 1.25, DEFMod: 0.85, SPDMod: 1.10, Description: "后排爆发，依赖保护"},
		{ID: RoleSupport, NameZH: "辅助", NameEN: "Support", PreferredSlot: SlotMid, HPMod: 1.00, ATKMod: 0.80, DEFMod: 1.00, SPDMod: 1.00, Description: "治疗、增益与净化"},
		{ID: RoleControl, NameZH: "控制", NameEN: "Control", PreferredSlot: SlotMid, HPMod: 0.95, ATKMod: 0.95, DEFMod: 1.05, SPDMod: 1.15, Description: "减速、禁锢与打断，受硬上限约束"},
	}
}

// Statuses returns the status catalog with anti-softlock caps.
func Statuses() []StatusDef {
	return []StatusDef{
		{ID: StatusBurn, NameZH: "灼烧", NameEN: "Burn", MaxDuration: 3, MaxStacks: 2, TickDamagePct: 0.04, Description: "回合结束按最大生命百分比掉血"},
		{ID: StatusBleed, NameZH: "流血", NameEN: "Bleed", MaxDuration: 3, MaxStacks: 2, TickDamagePct: 0.03, Description: "持续物理创伤"},
		{ID: StatusPoison, NameZH: "中毒", NameEN: "Poison", MaxDuration: 4, MaxStacks: 2, TickDamagePct: 0.03, Description: "持续毒素伤害"},
		{ID: StatusStun, NameZH: "眩晕", NameEN: "Stun", IsControl: true, MaxDuration: 1, MaxStacks: 1, Description: "跳过行动；连控最多 2 回合后强制免疫"},
		{ID: StatusRoot, NameZH: "禁锢", NameEN: "Root", IsControl: true, MaxDuration: 2, MaxStacks: 1, Description: "无法使用技能，仍可普攻"},
		{ID: StatusSlow, NameZH: "减速", NameEN: "Slow", IsControl: true, MaxDuration: 2, MaxStacks: 1, SPDMod: -0.30, Description: "速度下降"},
		{ID: StatusAtkUp, NameZH: "攻击提升", NameEN: "ATK Up", MaxDuration: 3, MaxStacks: 2, ATKMod: 0.20, Description: "攻击提升"},
		{ID: StatusDefUp, NameZH: "防御提升", NameEN: "DEF Up", MaxDuration: 3, MaxStacks: 2, DEFMod: 0.25, Description: "防御提升"},
		{ID: StatusShield, NameZH: "护盾", NameEN: "Shield", MaxDuration: 2, MaxStacks: 1, ShieldPct: 0.15, Description: "吸收伤害"},
		{ID: StatusRegen, NameZH: "再生", NameEN: "Regen", MaxDuration: 3, MaxStacks: 2, TickDamagePct: -0.04, Description: "回合结束回复生命"},
		{ID: StatusImmune, NameZH: "控免", NameEN: "Control Immune", MaxDuration: 1, MaxStacks: 1, Description: "短暂免疫控制（连控破局）"},
	}
}

// Skills returns the first-batch skill library (≥12).
func Skills() []SkillDef {
	return []SkillDef{
		{
			ID: "claw_strike", NameZH: "利爪突袭", NameEN: "Claw Strike", Kind: SkillKindActive,
			Element: ElementDark, Roles: []RoleID{RoleDPS, RoleControl}, Cooldown: 0, BasePower: 1.0,
			Effects:     []SkillEffect{{Kind: EffectDamage, Power: 1.0}},
			UpgradeNote: "每级伤害 +12%", Description: "基础物理输出技能",
		},
		{
			ID: "bite", NameZH: "撕咬", NameEN: "Bite", Kind: SkillKindActive,
			Element: ElementDark, Roles: []RoleID{RoleDPS, RoleTank}, Cooldown: 2, BasePower: 1.15,
			Effects: []SkillEffect{
				{Kind: EffectDamage, Power: 1.15},
				{Kind: EffectStatus, Status: StatusBleed, Duration: 2},
			},
			UpgradeNote: "流血持续 +1", Description: "造成伤害并施加流血",
		},
		{
			ID: "shell_guard", NameZH: "甲壳守护", NameEN: "Shell Guard", Kind: SkillKindActive,
			Roles: []RoleID{RoleTank}, Cooldown: 3, BasePower: 0,
			Effects: []SkillEffect{
				{Kind: EffectStatus, Status: StatusDefUp, Duration: 2, TargetSelf: true},
				{Kind: EffectShield, Power: 0.15, TargetSelf: true},
			},
			UpgradeNote: "护盾比例 +5%", Description: "提升防御并获得护盾",
		},
		{
			ID: "taunt", NameZH: "嘲讽", NameEN: "Taunt", Kind: SkillKindActive,
			Roles: []RoleID{RoleTank}, Cooldown: 3, BasePower: 0.6,
			Effects: []SkillEffect{
				{Kind: EffectDamage, Power: 0.6},
				{Kind: EffectStatus, Status: StatusAtkUp, Duration: 2, TargetSelf: true},
			},
			UpgradeNote: "冷却 -1", Description: "吸引仇恨并强化自身攻击",
		},
		{
			ID: "heal_lick", NameZH: "舔舐愈合", NameEN: "Heal Lick", Kind: SkillKindActive,
			Roles: []RoleID{RoleSupport}, Cooldown: 2, BasePower: 0.22,
			Effects:     []SkillEffect{{Kind: EffectHeal, Power: 0.22, TargetSelf: true}},
			UpgradeNote: "治疗量 +4% 最大生命", Description: "回复自身生命",
		},
		{
			ID: "howl", NameZH: "战吼", NameEN: "Howl", Kind: SkillKindActive,
			Roles: []RoleID{RoleSupport, RoleTank}, Cooldown: 3, BasePower: 0,
			Effects:     []SkillEffect{{Kind: EffectStatus, Status: StatusAtkUp, Duration: 2, TargetSelf: true}},
			UpgradeNote: "攻击增益叠加上限 +1", Description: "提升攻击，适合低稀有度开团",
		},
		{
			ID: "wing_gust", NameZH: "振翅狂风", NameEN: "Wing Gust", Kind: SkillKindActive,
			Element: ElementLight, Roles: []RoleID{RoleControl, RoleDPS}, Cooldown: 2, BasePower: 0.85,
			Effects: []SkillEffect{
				{Kind: EffectDamage, Power: 0.85},
				{Kind: EffectStatus, Status: StatusSlow, Duration: 2},
			},
			UpgradeNote: "减速持续 +1", Description: "伤害并减速",
		},
		{
			ID: "mud_trap", NameZH: "泥沼陷阱", NameEN: "Mud Trap", Kind: SkillKindActive,
			Element: ElementGrass, Roles: []RoleID{RoleControl}, Cooldown: 3, BasePower: 0.55,
			Effects: []SkillEffect{
				{Kind: EffectDamage, Power: 0.55},
				{Kind: EffectStatus, Status: StatusRoot, Duration: 2},
			},
			UpgradeNote: "冷却 -1", Description: "禁锢目标；连控受全局上限约束",
		},
		{
			ID: "fire_pounce", NameZH: "烈焰扑击", NameEN: "Fire Pounce", Kind: SkillKindActive,
			Element: ElementFire, Roles: []RoleID{RoleDPS}, Cooldown: 2, BasePower: 1.25,
			Effects: []SkillEffect{
				{Kind: EffectDamage, Power: 1.25},
				{Kind: EffectStatus, Status: StatusBurn, Duration: 2},
			},
			UpgradeNote: "灼烧叠加上限生效", Description: "火系爆发并灼烧",
		},
		{
			ID: "water_splash", NameZH: "水花溅射", NameEN: "Water Splash", Kind: SkillKindActive,
			Element: ElementWater, Roles: []RoleID{RoleDPS, RoleSupport}, Cooldown: 2, BasePower: 1.05,
			Effects: []SkillEffect{
				{Kind: EffectDamage, Power: 1.05},
				{Kind: EffectCleanse, TargetSelf: true},
			},
			UpgradeNote: "伤害 +12%", Description: "水系伤害并净化自身负面",
		},
		{
			ID: "leaf_bind", NameZH: "藤蔓束缚", NameEN: "Leaf Bind", Kind: SkillKindActive,
			Element: ElementGrass, Roles: []RoleID{RoleControl, RoleSupport}, Cooldown: 3, BasePower: 0.7,
			Effects: []SkillEffect{
				{Kind: EffectDamage, Power: 0.7},
				{Kind: EffectStatus, Status: StatusPoison, Duration: 3},
			},
			UpgradeNote: "中毒持续 +1", Description: "草系控制与中毒",
		},
		{
			ID: "light_flare", NameZH: "闪光爆裂", NameEN: "Light Flare", Kind: SkillKindActive,
			Element: ElementLight, Roles: []RoleID{RoleDPS, RoleSupport}, Cooldown: 3, BasePower: 1.2,
			Effects:     []SkillEffect{{Kind: EffectDamage, Power: 1.2}},
			UpgradeNote: "对暗系额外 +10%", Description: "光系高伤",
		},
		{
			ID: "dark_fang", NameZH: "暗影之牙", NameEN: "Dark Fang", Kind: SkillKindActive,
			Element: ElementDark, Roles: []RoleID{RoleDPS, RoleControl}, Cooldown: 2, BasePower: 1.1,
			Effects: []SkillEffect{
				{Kind: EffectDamage, Power: 1.1},
				{Kind: EffectStatus, Status: StatusStun, Duration: 1},
			},
			UpgradeNote: "眩晕命中更稳", Description: "暗系伤害并短眩晕",
		},
		{
			ID: "energy_burst", NameZH: "能量爆发", NameEN: "Energy Burst", Kind: SkillKindEnergy,
			Roles: []RoleID{RoleDPS, RoleTank, RoleSupport, RoleControl}, Cooldown: 0, EnergyCost: EnergySkillCost, BasePower: 1.8,
			Effects:     []SkillEffect{{Kind: EffectDamage, Power: 1.8}},
			UpgradeNote: "能量技伤害 +12%/级", Description: "能量满时可释放大招，忽略闪避",
		},
		{
			ID: "pack_regen", NameZH: "群体再生", NameEN: "Pack Regen", Kind: SkillKindActive,
			Roles: []RoleID{RoleSupport}, Cooldown: 4, BasePower: 0,
			Effects:     []SkillEffect{{Kind: EffectStatus, Status: StatusRegen, Duration: 3, TargetSelf: true}},
			UpgradeNote: "再生强度提升", Description: "持续回复，低稀有度续航核心",
		},
	}
}

// Upgrades documents skill upgrade tiers 1-3.
func Upgrades() []UpgradeDef {
	return []UpgradeDef{
		{Level: 1, PowerBonus: 0, CooldownCut: 0, Description: "基础形态"},
		{Level: 2, PowerBonus: 0.12, CooldownCut: 0, Description: "伤害/治疗 +12%"},
		{Level: 3, PowerBonus: 0.24, CooldownCut: 1, Description: "伤害/治疗 +24%，冷却最多 -1"},
	}
}

// Archetypes returns ≥6 enemy templates.
func Archetypes() []ArchetypeDef {
	return []ArchetypeDef{
		{
			ID: "bruiser", NameZH: "莽撞重击手", NameEN: "Bruiser", Difficulty: 2,
			ThreatTags:  []string{"high_hp", "high_atk"},
			CounterHint: "用控制打断输出窗口，或高防坦克硬吃",
			Members: []ArchMember{
				{Name: "狂犬", Species: "dog", Role: RoleTank, Slot: SlotFront, Element: ElementFire, HP: 160, ATK: 42, DEF: 28, SPD: 22, SkillIDs: []string{"bite", "taunt", "energy_burst"}},
				{Name: "凶猫", Species: "cat", Role: RoleDPS, Slot: SlotBack, Element: ElementFire, HP: 100, ATK: 48, DEF: 18, SPD: 40, SkillIDs: []string{"claw_strike", "fire_pounce", "energy_burst"}},
			},
		},
		{
			ID: "glass_cannon", NameZH: "玻璃大炮", NameEN: "Glass Cannon", Difficulty: 2,
			ThreatTags:  []string{"burst", "low_hp"},
			CounterHint: "先手秒杀或护盾硬抗一波",
			Members: []ArchMember{
				{Name: "影猫", Species: "cat", Role: RoleDPS, Slot: SlotBack, Element: ElementDark, HP: 80, ATK: 58, DEF: 12, SPD: 48, SkillIDs: []string{"dark_fang", "claw_strike", "energy_burst"}},
				{Name: "疾兔", Species: "rabbit", Role: RoleDPS, Slot: SlotMid, Element: ElementLight, HP: 75, ATK: 50, DEF: 14, SPD: 52, SkillIDs: []string{"light_flare", "wing_gust", "energy_burst"}},
			},
		},
		{
			ID: "iron_wall", NameZH: "铁壁防线", NameEN: "Iron Wall", Difficulty: 3,
			ThreatTags:  []string{"high_def", "stall"},
			CounterHint: "百分比灼烧/中毒与破防增益",
			Members: []ArchMember{
				{Name: "甲鹅", Species: "goose", Role: RoleTank, Slot: SlotFront, Element: ElementWater, HP: 180, ATK: 28, DEF: 48, SPD: 18, SkillIDs: []string{"shell_guard", "taunt", "water_splash"}},
				{Name: "护犬", Species: "dog", Role: RoleSupport, Slot: SlotMid, Element: ElementWater, HP: 130, ATK: 30, DEF: 36, SPD: 24, SkillIDs: []string{"howl", "heal_lick", "pack_regen"}},
			},
		},
		{
			ID: "controller", NameZH: "控场大师", NameEN: "Controller", Difficulty: 3,
			ThreatTags:  []string{"stun", "root", "slow"},
			CounterHint: "带净化/高 SPD，利用连控免疫窗口输出",
			Members: []ArchMember{
				{Name: "沼鹅", Species: "goose", Role: RoleControl, Slot: SlotMid, Element: ElementGrass, HP: 120, ATK: 34, DEF: 30, SPD: 36, SkillIDs: []string{"mud_trap", "leaf_bind", "wing_gust"}},
				{Name: "影控", Species: "cat", Role: RoleControl, Slot: SlotBack, Element: ElementDark, HP: 95, ATK: 38, DEF: 20, SPD: 42, SkillIDs: []string{"dark_fang", "claw_strike", "energy_burst"}},
			},
		},
		{
			ID: "swarmer", NameZH: "群聚骚扰", NameEN: "Swarmer", Difficulty: 2,
			ThreatTags:  []string{"multi_hit", "attrition"},
			CounterHint: "AOE 或优先击杀高速单位",
			Members: []ArchMember{
				{Name: "兔A", Species: "rabbit", Role: RoleDPS, Slot: SlotFront, Element: ElementGrass, HP: 70, ATK: 32, DEF: 14, SPD: 46, SkillIDs: []string{"claw_strike", "leaf_bind"}},
				{Name: "兔B", Species: "rabbit", Role: RoleDPS, Slot: SlotMid, Element: ElementGrass, HP: 70, ATK: 32, DEF: 14, SPD: 44, SkillIDs: []string{"claw_strike", "wing_gust"}},
				{Name: "兔C", Species: "rabbit", Role: RoleSupport, Slot: SlotBack, Element: ElementLight, HP: 65, ATK: 28, DEF: 12, SPD: 40, SkillIDs: []string{"heal_lick", "howl", "energy_burst"}},
			},
		},
		{
			ID: "healer_boss", NameZH: "再生首领", NameEN: "Healer Boss", Difficulty: 4,
			ThreatTags:  []string{"regen", "sustain"},
			CounterHint: "爆发窗口集火，或持续压制治疗",
			Members: []ArchMember{
				{Name: "首领犬", Species: "dog", Role: RoleTank, Slot: SlotFront, Element: ElementLight, HP: 200, ATK: 36, DEF: 34, SPD: 20, SkillIDs: []string{"shell_guard", "taunt", "energy_burst"}},
				{Name: "祭司鹅", Species: "goose", Role: RoleSupport, Slot: SlotBack, Element: ElementWater, HP: 140, ATK: 26, DEF: 30, SPD: 28, SkillIDs: []string{"pack_regen", "heal_lick", "water_splash", "howl"}},
			},
		},
	}
}

// RecommendedTeams returns ≥3 viable low-rarity-friendly builds.
func RecommendedTeams() []TeamBuild {
	return []TeamBuild{
		{
			ID: "budget_sustain", NameZH: "低配续航流", NameEN: "Budget Sustain",
			Roles:       []RoleID{RoleTank, RoleSupport, RoleDPS},
			SkillIDs:    []string{"shell_guard", "heal_lick", "pack_regen", "claw_strike", "howl"},
			Counters:    []string{"bruiser", "swarmer", "glass_cannon"},
			RarityHint:  "common/uncommon 即可通关多数 2 星",
			Description: "坦克承伤 + 治疗续航，靠策略与节奏磨死，不依赖高稀有度",
		},
		{
			ID: "control_burst", NameZH: "控场爆发流", NameEN: "Control Burst",
			Roles:       []RoleID{RoleControl, RoleDPS, RoleSupport},
			SkillIDs:    []string{"mud_trap", "dark_fang", "fire_pounce", "water_splash", "energy_burst"},
			Counters:    []string{"glass_cannon", "healer_boss", "swarmer"},
			RarityHint:  "uncommon 起，强调先手与能量大招",
			Description: "短控创造输出窗口，大招斩杀；连控不会无限延长",
		},
		{
			ID: "element_counter", NameZH: "元素克制流", NameEN: "Element Counter",
			Roles:       []RoleID{RoleDPS, RoleDPS, RoleTank},
			SkillIDs:    []string{"fire_pounce", "water_splash", "leaf_bind", "light_flare", "shell_guard"},
			Counters:    []string{"iron_wall", "bruiser", "controller"},
			RarityHint:  "按敌方元素换技能即可，低稀有度可打过克制关",
			Description: "围绕元素表换装，三套技能覆盖常见威胁",
		},
	}
}

// GetCatalog assembles the client-facing design catalog.
func GetCatalog() Catalog {
	return Catalog{
		RuleVersion:      RuleVersion,
		Roles:            Roles(),
		Slots:            []SlotID{SlotFront, SlotMid, SlotBack},
		Statuses:         Statuses(),
		Skills:           Skills(),
		Upgrades:         Upgrades(),
		Archetypes:       Archetypes(),
		RecommendedTeams: RecommendedTeams(),
		Limits: map[string]int{
			"max_team_size":           MaxTeamSize,
			"max_rounds":              MaxRounds,
			"max_energy":              MaxEnergy,
			"max_control_streak":      MaxControlStreak,
			"max_status_stacks":       MaxStatusStacks,
			"max_skill_level":         MaxSkillLevel,
			"zero_damage_break_after": ZeroDamageBreakAfter,
		},
	}
}

// SkillByID looks up a skill definition.
func SkillByID(id string) (SkillDef, bool) {
	for _, s := range Skills() {
		if s.ID == id {
			return s, true
		}
	}
	return SkillDef{}, false
}

// StatusByID looks up a status definition.
func StatusByID(id StatusID) (StatusDef, bool) {
	for _, s := range Statuses() {
		if s.ID == id {
			return s, true
		}
	}
	return StatusDef{}, false
}

// ArchetypeByID looks up an enemy archetype.
func ArchetypeByID(id string) (ArchetypeDef, bool) {
	for _, a := range Archetypes() {
		if a.ID == id {
			return a, true
		}
	}
	return ArchetypeDef{}, false
}

// RoleByID looks up a role.
func RoleByID(id RoleID) (RoleDef, bool) {
	for _, r := range Roles() {
		if r.ID == id {
			return r, true
		}
	}
	return RoleDef{}, false
}
