/**
 * AP-117 · Resident cast, relationship network, cross-chapter arcs.
 * Animals are clues only — never speaking educational NPCs or villains.
 */
import type { ChapterId } from '../season1/architecture'

export type CharacterId =
  | 'archivist'
  | 'street_photographer'
  | 'urban_planner'
  | 'journal_aide'

export type VoiceTrait = {
  /** Short sample line for blind voice tests */
  sampleLine: string
  markers: string[] // lexical / rhythm markers
}

export type ArcStage = {
  stage: 1 | 2 | 3
  chapterIds: ChapterId[]
  desire: string
  blindSpot: string
  visibleChange: string
}

export type CharacterDef = {
  id: CharacterId
  displayName: string
  role: string
  /** Explicitly fictional aide vs human roles */
  fictional: boolean
  desire: string
  blindSpot: string
  secret: string
  valueConflict: string
  voice: VoiceTrait
  arcs: ArcStage[]
  /** Knowledge keys they may learn only after chapter id */
  knowledgeGates: { key: string; afterChapter: ChapterId }[]
}

export type RelationEdge = {
  from: CharacterId
  to: CharacterId
  /** Initial stance */
  initial: string
  /** How it can change (driven by player choices later) */
  changeAxes: string[]
  /** Chapters where relation must visibly shift */
  shiftChapters: ChapterId[]
}

export type CastPack = {
  id: 'cast.season1'
  version: string
  characters: CharacterDef[]
  relations: RelationEdge[]
}

export const season1Cast: CastPack = {
  id: 'cast.season1',
  version: '1',
  characters: [
    {
      id: 'archivist',
      displayName: '林溯',
      role: '社区档案员',
      fictional: false,
      desire: '把城市记忆做成可核验的公共档案',
      blindSpot: '把情绪化叙述当成噪声',
      secret: '曾删过一份无法溯源但很动人的口述',
      valueConflict: '完整 vs 可核验',
      voice: {
        sampleLine: '这条记录缺来源。我们可以先标「未核验」，而不是先下结论。',
        markers: ['来源', '核验', '标注', '版本', '索引'],
      },
      arcs: [
        {
          stage: 1,
          chapterIds: ['prologue.blank_page', 'ch01.alley_echo'],
          desire: '建立干净目录',
          blindSpot: '排斥现场语气',
          visibleChange: '开始收集居民口述',
        },
        {
          stage: 2,
          chapterIds: ['ch02.along_river_sleepless', 'ch03.rain_on_eaves'],
          desire: '在正文里写不确定',
          blindSpot: '仍怕「空白」显得不专业',
          visibleChange: '公开标注不确定与样本不足',
        },
        {
          stage: 3,
          chapterIds: ['ch04.map_blank', 'finale.who_tells_the_city'],
          desire: '让空白可触摸',
          blindSpot: '仍想给展览一个权威结语',
          visibleChange: '接受多声部可能更诚实',
        },
      ],
      knowledgeGates: [
        { key: 'finale_exhibition', afterChapter: 'ch04.map_blank' },
        { key: 'photographer_secret_crop', afterChapter: 'ch01.alley_echo' },
      ],
    },
    {
      id: 'street_photographer',
      displayName: '阿柯',
      role: '街拍者',
      fictional: false,
      desire: '用取景证明「我当时在场」',
      blindSpot: '低估取景对他人的冒犯',
      secret: '有一张被要求删除的巷口照片仍留着底',
      valueConflict: '现场感 vs 被摄者尊严',
      voice: {
        sampleLine: '你看光——不是数据，是那一下刚好有人经过。',
        markers: ['光', '取景', '现场', '快门', '裁切'],
      },
      arcs: [
        {
          stage: 1,
          chapterIds: ['prologue.blank_page', 'ch01.alley_echo'],
          desire: '让现场语气进档案',
          blindSpot: '把「真实」等同于自己的构图',
          visibleChange: '承认取景可能冒犯店主',
        },
        {
          stage: 2,
          chapterIds: ['ch02.along_river_sleepless', 'ch03.rain_on_eaves'],
          desire: '拒绝伤害性拍摄',
          blindSpot: '仍想用照片赢过口述',
          visibleChange: '夜拍改拍灯带与围挡，承认照片也会说谎',
        },
        {
          stage: 3,
          chapterIds: ['ch04.map_blank', 'finale.who_tells_the_city'],
          desire: '分享署名权',
          blindSpot: '害怕失去作者身份',
          visibleChange: '提交「故意不拍主体」的作品并让出部分署名',
        },
      ],
      knowledgeGates: [
        { key: 'planner_model_failure', afterChapter: 'ch02.along_river_sleepless' },
        { key: 'finale_exhibition', afterChapter: 'ch04.map_blank' },
      ],
    },
    {
      id: 'urban_planner',
      displayName: '周衡',
      role: '城市规划研究者',
      fictional: false,
      desire: '用可达性与安全数据改进公共空间',
      blindSpot: '模型边界外的生活经验',
      secret: '一份夜间灯光模型在雨季失效过',
      valueConflict: '可量化指标 vs 不可量化的地方感',
      voice: {
        sampleLine: '模型给的是区间，不是答案。区间外的人怎么走，还得另算。',
        markers: ['模型', '区间', '可达', '指标', '假设'],
      },
      arcs: [
        {
          stage: 1,
          chapterIds: ['ch01.alley_echo'],
          desire: '把安全数据带进巷口讨论',
          blindSpot: '默认数据比口述优先',
          visibleChange: '首次出场用可达性说话',
        },
        {
          stage: 2,
          chapterIds: ['ch02.along_river_sleepless', 'ch03.rain_on_eaves'],
          desire: '公开假设边界',
          blindSpot: '仍希望找到「最优解」',
          visibleChange: '公开模型失败案例',
        },
        {
          stage: 3,
          chapterIds: ['ch04.map_blank', 'finale.who_tells_the_city'],
          desire: '承认规划图也会制造空白',
          blindSpot: '害怕空白被解读为失职',
          visibleChange: '把模型失败写进展板',
        },
      ],
      knowledgeGates: [
        { key: 'archivist_deleted_oral', afterChapter: 'ch03.rain_on_eaves' },
        { key: 'finale_exhibition', afterChapter: 'ch04.map_blank' },
      ],
    },
    {
      id: 'journal_aide',
      displayName: '页页',
      role: '手账助手（明确虚构）',
      fictional: true,
      desire: '帮玩家整理线索，但不替玩家下结论',
      blindSpot: '有时过于「系统口吻」',
      secret: '没有秘密——其设定就是透明工具人，避免成为作者嘴替',
      valueConflict: '效率引导 vs 玩家自主',
      voice: {
        sampleLine: '我可以列选项，但选哪条是你的。需要我把空白也记下来吗？',
        markers: ['选项', '记录', '空白', 'Home Mode', '不必出门'],
      },
      arcs: [
        {
          stage: 1,
          chapterIds: ['prologue.blank_page', 'ch01.alley_echo'],
          desire: '教会手账基本操作',
          blindSpot: '话有点多',
          visibleChange: '提示 Home Mode 与权限',
        },
        {
          stage: 2,
          chapterIds: ['ch02.along_river_sleepless', 'ch03.rain_on_eaves'],
          desire: '提供缺席证据模板',
          blindSpot: '仍像教程',
          visibleChange: '强制提示夜间外出非必须',
        },
        {
          stage: 3,
          chapterIds: ['ch04.map_blank', 'finale.who_tells_the_city'],
          desire: '退到旁白',
          blindSpot: '——',
          visibleChange: '把话筒交给玩家',
        },
      ],
      knowledgeGates: [
        // aide may surface UI hints anytime but never spoil finale content before ch04
        { key: 'finale_exhibition', afterChapter: 'ch04.map_blank' },
      ],
    },
  ],
  relations: [
    {
      from: 'archivist',
      to: 'street_photographer',
      initial: '互相怀疑对方「不专业 / 太情绪」',
      changeAxes: ['trust_method', 'credit_sharing'],
      shiftChapters: ['ch01.alley_echo', 'ch03.rain_on_eaves', 'finale.who_tells_the_city'],
    },
    {
      from: 'archivist',
      to: 'urban_planner',
      initial: '尊重数据但怕被指标绑架',
      changeAxes: ['evidence_standard', 'public_accountability'],
      shiftChapters: ['ch02.along_river_sleepless', 'ch04.map_blank', 'finale.who_tells_the_city'],
    },
    {
      from: 'street_photographer',
      to: 'urban_planner',
      initial: '觉得模型看不见现场光',
      changeAxes: ['what_counts_as_evidence', 'night_safety'],
      shiftChapters: ['ch02.along_river_sleepless', 'ch03.rain_on_eaves', 'finale.who_tells_the_city'],
    },
    {
      from: 'journal_aide',
      to: 'archivist',
      initial: '工具协作：提醒标注与版本',
      changeAxes: ['player_agency'],
      shiftChapters: ['prologue.blank_page', 'ch03.rain_on_eaves', 'finale.who_tells_the_city'],
    },
    {
      from: 'journal_aide',
      to: 'street_photographer',
      initial: '提醒伦理与 Home Mode 替代',
      changeAxes: ['harm_reduction'],
      shiftChapters: ['ch01.alley_echo', 'ch02.along_river_sleepless'],
    },
    {
      from: 'journal_aide',
      to: 'urban_planner',
      initial: '翻译模型语言给玩家',
      changeAxes: ['explainability'],
      shiftChapters: ['ch02.along_river_sleepless', 'ch04.map_blank'],
    },
  ],
}

export function listCharacters(): CharacterDef[] {
  return season1Cast.characters
}

export function getCharacter(id: CharacterId): CharacterDef | undefined {
  return season1Cast.characters.find((c) => c.id === id)
}

/** Knowledge allowed only if current chapter order >= gate */
export function canKnow(
  characterId: CharacterId,
  knowledgeKey: string,
  currentChapter: ChapterId,
  chapterOrder: Record<ChapterId, number>,
): boolean {
  const ch = getCharacter(characterId)
  if (!ch) return false
  const gate = ch.knowledgeGates.find((g) => g.key === knowledgeKey)
  if (!gate) return true
  return (chapterOrder[currentChapter] ?? -1) >= (chapterOrder[gate.afterChapter] ?? 999)
}

export function relationsFor(id: CharacterId): RelationEdge[] {
  return season1Cast.relations.filter((r) => r.from === id || r.to === id)
}

export function validateCast(pack: CastPack = season1Cast): string[] {
  const errors: string[] = []
  if (pack.characters.length < 4) errors.push('need ≥4 resident characters')
  const ids = new Set(pack.characters.map((c) => c.id))
  if (!ids.has('journal_aide')) errors.push('need fictional journal aide')
  const aide = pack.characters.find((c) => c.id === 'journal_aide')
  if (aide && !aide.fictional) errors.push('journal_aide must be fictional')

  for (const c of pack.characters) {
    if (c.arcs.length < 3) errors.push(`${c.id}: need 3-stage arc`)
    const chaptersTouched = new Set(c.arcs.flatMap((a) => a.chapterIds))
    if (chaptersTouched.size < 3) errors.push(`${c.id}: must appear across ≥3 chapters`)
    if (!c.voice.sampleLine || c.voice.markers.length < 3) {
      errors.push(`${c.id}: weak voice profile`)
    }
    // no pure quest dispenser / absolute villain flags in text
    const bad = /唯一正确|绝对邪恶|作者认为/i
    if (bad.test(c.desire + c.secret + c.valueConflict)) {
      errors.push(`${c.id}: author-mouthpiece / absolute evil language`)
    }
  }

  // each character relates to ≥2 others
  for (const c of pack.characters) {
    const others = new Set<string>()
    for (const r of pack.relations) {
      if (r.from === c.id) others.add(r.to)
      if (r.to === c.id) others.add(r.from)
    }
    if (others.size < 2) errors.push(`${c.id}: need relations with ≥2 others`)
  }

  for (const r of pack.relations) {
    if (!ids.has(r.from) || !ids.has(r.to)) errors.push(`bad edge ${r.from}->${r.to}`)
    if (r.shiftChapters.length < 2) errors.push(`${r.from}-${r.to}: need ≥2 shift chapters`)
  }

  return errors
}

/** Voice blind-test helper: strip names, keep sample lines */
export function voiceBlindCards(): { id: CharacterId; line: string; markers: string[] }[] {
  return season1Cast.characters.map((c) => ({
    id: c.id,
    line: c.voice.sampleLine,
    markers: c.voice.markers,
  }))
}
