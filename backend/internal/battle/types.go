// Package battle implements AP-102 battle design: teams, skills, statuses,
// enemy archetypes, deterministic seed+command settlement, threats and failure explain.
package battle

// RuleVersion is the authoritative simulation version embedded in sessions and logs.
const RuleVersion = "battle.v1"

// Hard rules preventing soft-locks and client abuse.
const (
	MaxTeamSize            = 3
	MaxRounds              = 40
	MaxEnergy              = 100
	EnergyPerBasic         = 18
	EnergyPerHit           = 10
	EnergySkillCost        = 100
	MinDamageFloorRatio    = 0.08 // at least 8% of ATK as floor damage
	MaxControlStreak       = 2    // consecutive controlled turns before forced break
	ControlImmunityTurns   = 1
	MaxStatusStacks        = 2
	ZeroDamageBreakAfter   = 6 // consecutive zero-damage actions → true damage break
	TrueDamageBreak        = 5
	MaxCommandsPerBattle   = 256
	MaxSkillLevel          = 3
	DefaultSkillLevel      = 1
	PositionFrontDamageIn  = 1.0
	PositionMidDamageIn    = 0.85
	PositionBackDamageIn   = 0.70
	PositionFrontDamageOut = 0.90
	PositionMidDamageOut   = 1.0
	PositionBackDamageOut  = 1.10
)

// RoleID is a team positioning / class identity.
type RoleID string

const (
	RoleTank    RoleID = "tank"
	RoleDPS     RoleID = "dps"
	RoleSupport RoleID = "support"
	RoleControl RoleID = "control"
)

// SlotID is a team formation slot.
type SlotID string

const (
	SlotFront SlotID = "front"
	SlotMid   SlotID = "mid"
	SlotBack  SlotID = "back"
)

// ElementID mirrors battle elements used for multipliers.
type ElementID string

const (
	ElementFire  ElementID = "fire"
	ElementWater ElementID = "water"
	ElementGrass ElementID = "grass"
	ElementLight ElementID = "light"
	ElementDark  ElementID = "dark"
)

// StatusID is a runtime status effect key.
type StatusID string

const (
	StatusBurn   StatusID = "burn"
	StatusBleed  StatusID = "bleed"
	StatusPoison StatusID = "poison"
	StatusStun   StatusID = "stun"
	StatusRoot   StatusID = "root"
	StatusSlow   StatusID = "slow"
	StatusAtkUp  StatusID = "atk_up"
	StatusDefUp  StatusID = "def_up"
	StatusShield StatusID = "shield"
	StatusRegen  StatusID = "regen"
	StatusImmune StatusID = "control_immune"
)

// SkillKind classifies skill energy/usage.
type SkillKind string

const (
	SkillKindActive  SkillKind = "active"
	SkillKindEnergy  SkillKind = "energy" // requires full energy bar
	SkillKindPassive SkillKind = "passive"
)

// EffectKind is a skill payload kind.
type EffectKind string

const (
	EffectDamage  EffectKind = "damage"
	EffectHeal    EffectKind = "heal"
	EffectStatus  EffectKind = "status"
	EffectShield  EffectKind = "shield"
	EffectCleanse EffectKind = "cleanse"
)

// CommandKind is a player/AI action recorded in the command log.
type CommandKind string

const (
	CmdBasic    CommandKind = "basic"
	CmdSkill    CommandKind = "skill"
	CmdUltimate CommandKind = "ultimate"
	CmdPass     CommandKind = "pass"
	CmdStrategy CommandKind = "strategy"
)

// Strategy mirrors frontend aggressive/balanced/defensive.
type Strategy string

const (
	StrategyAggressive Strategy = "aggressive"
	StrategyBalanced   Strategy = "balanced"
	StrategyDefensive  Strategy = "defensive"
)

// RoleDef describes a team role.
type RoleDef struct {
	ID            RoleID  `json:"id"`
	NameZH        string  `json:"name_zh"`
	NameEN        string  `json:"name_en"`
	PreferredSlot SlotID  `json:"preferred_slot"`
	HPMod         float64 `json:"hp_mod"`
	ATKMod        float64 `json:"atk_mod"`
	DEFMod        float64 `json:"def_mod"`
	SPDMod        float64 `json:"spd_mod"`
	Description   string  `json:"description"`
}

// StatusDef is a catalog status definition.
type StatusDef struct {
	ID            StatusID `json:"id"`
	NameZH        string   `json:"name_zh"`
	NameEN        string   `json:"name_en"`
	IsControl     bool     `json:"is_control"`
	MaxDuration   int      `json:"max_duration"`
	MaxStacks     int      `json:"max_stacks"`
	TickDamagePct float64  `json:"tick_damage_pct,omitempty"` // of max HP
	ATKMod        float64  `json:"atk_mod,omitempty"`
	DEFMod        float64  `json:"def_mod,omitempty"`
	SPDMod        float64  `json:"spd_mod,omitempty"`
	ShieldPct     float64  `json:"shield_pct,omitempty"`
	Description   string   `json:"description"`
}

// SkillEffect is one payload applied by a skill.
type SkillEffect struct {
	Kind       EffectKind `json:"kind"`
	Power      float64    `json:"power,omitempty"` // damage/heal multiplier of ATK or maxHP
	Status     StatusID   `json:"status,omitempty"`
	Duration   int        `json:"duration,omitempty"`
	TargetSelf bool       `json:"target_self,omitempty"`
}

// SkillDef is a catalog skill.
type SkillDef struct {
	ID          string        `json:"id"`
	NameZH      string        `json:"name_zh"`
	NameEN      string        `json:"name_en"`
	Kind        SkillKind     `json:"kind"`
	Element     ElementID     `json:"element,omitempty"`
	Roles       []RoleID      `json:"roles"`
	Cooldown    int           `json:"cooldown"`
	EnergyCost  int           `json:"energy_cost"`
	BasePower   float64       `json:"base_power"`
	Effects     []SkillEffect `json:"effects"`
	UpgradeNote string        `json:"upgrade_note"`
	Description string        `json:"description"`
	// Level scaling: power *= 1 + 0.12*(level-1)
}

// UpgradeDef documents skill upgrade tiers.
type UpgradeDef struct {
	Level       int     `json:"level"`
	PowerBonus  float64 `json:"power_bonus"`
	CooldownCut int     `json:"cooldown_cut"`
	Description string  `json:"description"`
}

// ArchetypeDef is an enemy team template.
type ArchetypeDef struct {
	ID          string       `json:"id"`
	NameZH      string       `json:"name_zh"`
	NameEN      string       `json:"name_en"`
	ThreatTags  []string     `json:"threat_tags"`
	Members     []ArchMember `json:"members"`
	CounterHint string       `json:"counter_hint"`
	Difficulty  int          `json:"difficulty"` // 1-5
}

// ArchMember is one enemy fighter template.
type ArchMember struct {
	Name     string    `json:"name"`
	Species  string    `json:"species"`
	Role     RoleID    `json:"role"`
	Slot     SlotID    `json:"slot"`
	Element  ElementID `json:"element"`
	HP       int       `json:"hp"`
	ATK      int       `json:"atk"`
	DEF      int       `json:"def"`
	SPD      int       `json:"spd"`
	SkillIDs []string  `json:"skill_ids"`
}

// TeamBuild is a recommended formation.
type TeamBuild struct {
	ID          string   `json:"id"`
	NameZH      string   `json:"name_zh"`
	NameEN      string   `json:"name_en"`
	Roles       []RoleID `json:"roles"`
	SkillIDs    []string `json:"skill_ids"`
	Counters    []string `json:"counters"` // archetype ids
	RarityHint  string   `json:"rarity_hint"`
	Description string   `json:"description"`
}

// Fighter is a runtime combatant snapshot (server authoritative).
type Fighter struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Species  string    `json:"species"`
	Role     RoleID    `json:"role"`
	Slot     SlotID    `json:"slot"`
	Element  ElementID `json:"element"`
	MaxHP    int       `json:"max_hp"`
	HP       int       `json:"hp"`
	ATK      int       `json:"atk"`
	DEF      int       `json:"def"`
	SPD      int       `json:"spd"`
	CRIT     int       `json:"crit"` // percent 0-100
	EVA      int       `json:"eva"`  // percent 0-100
	Energy   int       `json:"energy"`
	SkillIDs []string  `json:"skill_ids"`
	// SkillLevels maps skill id → 1..3
	SkillLevels map[string]int `json:"skill_levels,omitempty"`
	Side        string         `json:"side"` // "player" | "enemy"
}

// ActiveStatus is a runtime stack on a fighter.
type ActiveStatus struct {
	ID       StatusID `json:"id"`
	Stacks   int      `json:"stacks"`
	Duration int      `json:"duration"`
	Source   string   `json:"source,omitempty"`
}

// Command is one authoritative action.
type Command struct {
	ActorID  string      `json:"actor_id"`
	Kind     CommandKind `json:"kind"`
	SkillID  string      `json:"skill_id,omitempty"`
	TargetID string      `json:"target_id,omitempty"`
	Strategy Strategy    `json:"strategy,omitempty"`
	// Round is optional client annotation; server re-derives order.
	Round int `json:"round,omitempty"`
}

// CommandLog is the seed-bound action log for settlement.
type CommandLog struct {
	Seed        string    `json:"seed"`
	RuleVersion string    `json:"rule_version"`
	Commands    []Command `json:"commands"`
	// ClaimedWinner is ignored when full simulation runs; kept for legacy PvP.
	ClaimedWinner string `json:"claimed_winner,omitempty"`
}

// Event is a single simulation log line for explain/UI.
type Event struct {
	Round   int    `json:"round"`
	ActorID string `json:"actor_id,omitempty"`
	Kind    string `json:"kind"`
	Text    string `json:"text"`
	Damage  int    `json:"damage,omitempty"`
	Heal    int    `json:"heal,omitempty"`
	Status  string `json:"status,omitempty"`
}

// Threat is a pre-battle readable warning.
type Threat struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // low|medium|high
	TextZH   string `json:"text_zh"`
	TextEN   string `json:"text_en"`
	Counter  string `json:"counter,omitempty"`
}

// FailureFactor explains a loss.
type FailureFactor struct {
	Code   string `json:"code"`
	Weight int    `json:"weight"`
	TextZH string `json:"text_zh"`
	TextEN string `json:"text_en"`
}

// Result is the authoritative settlement output.
type Result struct {
	WinnerSide     string          `json:"winner_side"` // player|enemy|draw
	Rounds         int             `json:"rounds"`
	Events         []Event         `json:"events"`
	PlayerAlive    int             `json:"player_alive"`
	EnemyAlive     int             `json:"enemy_alive"`
	FailureFactors []FailureFactor `json:"failure_factors,omitempty"`
	RuleVersion    string          `json:"rule_version"`
	Seed           string          `json:"seed"`
	CommandHash    string          `json:"command_hash"`
	EndedBy        string          `json:"ended_by"` // ko|timeout|softlock_break
	Metrics        ResultMetrics   `json:"metrics"`
}

// ResultMetrics supports balance tests and post-battle UI.
type ResultMetrics struct {
	PlayerDamageDealt  int `json:"player_damage_dealt"`
	EnemyDamageDealt   int `json:"enemy_damage_dealt"`
	ControlTurnsEnemy  int `json:"control_turns_enemy"`
	ControlTurnsPlayer int `json:"control_turns_player"`
	HealingDone        int `json:"healing_done"`
	ZeroDamageBreaks   int `json:"zero_damage_breaks"`
	ElementDisadvHits  int `json:"element_disadv_hits"`
}

// Catalog is the full design payload for clients.
type Catalog struct {
	RuleVersion      string         `json:"rule_version"`
	Roles            []RoleDef      `json:"roles"`
	Slots            []SlotID       `json:"slots"`
	Statuses         []StatusDef    `json:"statuses"`
	Skills           []SkillDef     `json:"skills"`
	Upgrades         []UpgradeDef   `json:"upgrades"`
	Archetypes       []ArchetypeDef `json:"archetypes"`
	RecommendedTeams []TeamBuild    `json:"recommended_teams"`
	Limits           map[string]int `json:"limits"`
}
