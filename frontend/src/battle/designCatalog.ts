/**
 * AP-102 battle design catalog (client mirror of server `/battle/catalog`).
 * Authoritative settlement remains server-side; this powers team builder UI.
 */

export type BattleRoleId = 'tank' | 'dps' | 'support' | 'control'
export type BattleSlotId = 'front' | 'mid' | 'back'
export type BattleElementId = 'fire' | 'water' | 'grass' | 'light' | 'dark'

export interface SkillDef {
  id: string
  nameZh: string
  kind: 'active' | 'energy' | 'passive'
  roles: BattleRoleId[]
  cooldown: number
  energyCost: number
  description: string
}

export interface ArchetypeDef {
  id: string
  nameZh: string
  threatTags: string[]
  counterHint: string
  difficulty: number
}

export interface TeamBuild {
  id: string
  nameZh: string
  roles: BattleRoleId[]
  skillIds: string[]
  counters: string[]
  rarityHint: string
  description: string
}

export const BATTLE_RULE_VERSION = 'battle.v1'

export const BATTLE_SKILLS: SkillDef[] = [
  { id: 'claw_strike', nameZh: '利爪突袭', kind: 'active', roles: ['dps', 'control'], cooldown: 0, energyCost: 0, description: '基础物理输出' },
  { id: 'bite', nameZh: '撕咬', kind: 'active', roles: ['dps', 'tank'], cooldown: 2, energyCost: 0, description: '伤害并流血' },
  { id: 'shell_guard', nameZh: '甲壳守护', kind: 'active', roles: ['tank'], cooldown: 3, energyCost: 0, description: '防御提升+护盾' },
  { id: 'taunt', nameZh: '嘲讽', kind: 'active', roles: ['tank'], cooldown: 3, energyCost: 0, description: '吸引仇恨' },
  { id: 'heal_lick', nameZh: '舔舐愈合', kind: 'active', roles: ['support'], cooldown: 2, energyCost: 0, description: '回复生命' },
  { id: 'howl', nameZh: '战吼', kind: 'active', roles: ['support', 'tank'], cooldown: 3, energyCost: 0, description: '攻击提升' },
  { id: 'wing_gust', nameZh: '振翅狂风', kind: 'active', roles: ['control', 'dps'], cooldown: 2, energyCost: 0, description: '伤害并减速' },
  { id: 'mud_trap', nameZh: '泥沼陷阱', kind: 'active', roles: ['control'], cooldown: 3, energyCost: 0, description: '禁锢（受连控上限）' },
  { id: 'fire_pounce', nameZh: '烈焰扑击', kind: 'active', roles: ['dps'], cooldown: 2, energyCost: 0, description: '火系爆发+灼烧' },
  { id: 'water_splash', nameZh: '水花溅射', kind: 'active', roles: ['dps', 'support'], cooldown: 2, energyCost: 0, description: '水系伤害+净化' },
  { id: 'leaf_bind', nameZh: '藤蔓束缚', kind: 'active', roles: ['control', 'support'], cooldown: 3, energyCost: 0, description: '草系中毒' },
  { id: 'light_flare', nameZh: '闪光爆裂', kind: 'active', roles: ['dps', 'support'], cooldown: 3, energyCost: 0, description: '光系高伤' },
  { id: 'dark_fang', nameZh: '暗影之牙', kind: 'active', roles: ['dps', 'control'], cooldown: 2, energyCost: 0, description: '暗系+短眩晕' },
  { id: 'energy_burst', nameZh: '能量爆发', kind: 'energy', roles: ['dps', 'tank', 'support', 'control'], cooldown: 0, energyCost: 100, description: '能量大招' },
  { id: 'pack_regen', nameZh: '群体再生', kind: 'active', roles: ['support'], cooldown: 4, energyCost: 0, description: '持续回复' },
]

export const BATTLE_ARCHETYPES: ArchetypeDef[] = [
  { id: 'bruiser', nameZh: '莽撞重击手', threatTags: ['high_hp', 'high_atk'], counterHint: '控制打断或高防硬吃', difficulty: 2 },
  { id: 'glass_cannon', nameZh: '玻璃大炮', threatTags: ['burst', 'low_hp'], counterHint: '先手秒杀或护盾硬抗', difficulty: 2 },
  { id: 'iron_wall', nameZh: '铁壁防线', threatTags: ['high_def', 'stall'], counterHint: '灼烧/中毒与破防', difficulty: 3 },
  { id: 'controller', nameZh: '控场大师', threatTags: ['stun', 'root', 'slow'], counterHint: '净化与连控免疫窗口', difficulty: 3 },
  { id: 'swarmer', nameZh: '群聚骚扰', threatTags: ['multi_hit', 'attrition'], counterHint: '优先击杀高速单位', difficulty: 2 },
  { id: 'healer_boss', nameZh: '再生首领', threatTags: ['regen', 'sustain'], counterHint: '爆发集火治疗位', difficulty: 4 },
]

export const RECOMMENDED_TEAMS: TeamBuild[] = [
  {
    id: 'budget_sustain',
    nameZh: '低配续航流',
    roles: ['tank', 'support', 'dps'],
    skillIds: ['shell_guard', 'heal_lick', 'pack_regen', 'claw_strike', 'howl'],
    counters: ['bruiser', 'swarmer', 'glass_cannon'],
    rarityHint: 'common/uncommon 即可',
    description: '坦克+治疗磨死，不依赖高稀有度',
  },
  {
    id: 'control_burst',
    nameZh: '控场爆发流',
    roles: ['control', 'dps', 'support'],
    skillIds: ['mud_trap', 'dark_fang', 'fire_pounce', 'water_splash', 'energy_burst'],
    counters: ['glass_cannon', 'healer_boss', 'swarmer'],
    rarityHint: 'uncommon 起',
    description: '短控创造输出窗口，大招斩杀',
  },
  {
    id: 'element_counter',
    nameZh: '元素克制流',
    roles: ['dps', 'dps', 'tank'],
    skillIds: ['fire_pounce', 'water_splash', 'leaf_bind', 'light_flare', 'shell_guard'],
    counters: ['iron_wall', 'bruiser', 'controller'],
    rarityHint: '按敌方元素换技能',
    description: '围绕元素表换装覆盖威胁',
  },
]

export function assertCatalogMinimums(): { skills: number; archetypes: number; teams: number } {
  return {
    skills: BATTLE_SKILLS.length,
    archetypes: BATTLE_ARCHETYPES.length,
    teams: RECOMMENDED_TEAMS.length,
  }
}
