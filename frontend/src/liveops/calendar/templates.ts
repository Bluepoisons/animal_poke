/** AP-105: safe live-event templates + 90-day calendar (config-driven). */

export type EventTemplateKind =
  | 'observation_week'
  | 'city_research'
  | 'knowledge_challenge'
  | 'photo_theme'
  | 'welfare_day'

export interface EventTemplate {
  id: string
  kind: EventTemplateKind
  title: string
  durationDays: number
  /** Soft reward budget units (not FOMO hard gates) */
  rewardBudget: number
  metrics: string[]
  /** Safety: never require night outdoor / cross-city chase */
  safety: {
    nightOutdoorRequired: false
    extremeWeatherRequired: false
    crossCityChase: false
    rareAnimalGather: false
  }
  missPolicy: 'compensate_soft' | 'rerun_window' | 'catalog_unlock'
  rollback: string
  welfareReview: string
}

export interface CalendarSlot {
  dayOffset: number // 0..89 from season start
  templateId: string
  targetPlayers: 'all' | 'returning' | 'new'
  notes?: string
}

export const TEMPLATES: EventTemplate[] = [
  {
    id: 'tpl.observation_week',
    kind: 'observation_week',
    title: '观察周',
    durationDays: 7,
    rewardBudget: 100,
    metrics: ['optional_find_rate', 'home_mode_completion'],
    safety: {
      nightOutdoorRequired: false,
      extremeWeatherRequired: false,
      crossCityChase: false,
      rareAnimalGather: false,
    },
    missPolicy: 'rerun_window',
    rollback: 'disable instance; keep catalog clues',
    welfareReview: 'no baiting wild animals; photo optional',
  },
  {
    id: 'tpl.city_research',
    kind: 'city_research',
    title: '城市研究',
    durationDays: 5,
    rewardBudget: 80,
    metrics: ['clue_discovery', 'branch_diversity'],
    safety: {
      nightOutdoorRequired: false,
      extremeWeatherRequired: false,
      crossCityChase: false,
      rareAnimalGather: false,
    },
    missPolicy: 'catalog_unlock',
    rollback: 'publish prior definition version',
    welfareReview: 'coarse places only',
  },
  {
    id: 'tpl.knowledge',
    kind: 'knowledge_challenge',
    title: '知识挑战',
    durationDays: 3,
    rewardBudget: 40,
    metrics: ['quiz_accuracy', 'retry_without_punish'],
    safety: {
      nightOutdoorRequired: false,
      extremeWeatherRequired: false,
      crossCityChase: false,
      rareAnimalGather: false,
    },
    missPolicy: 'compensate_soft',
    rollback: 'remove quiz set',
    welfareReview: 'education first',
  },
  {
    id: 'tpl.photo',
    kind: 'photo_theme',
    title: '摄影主题',
    durationDays: 4,
    rewardBudget: 60,
    metrics: ['theme_complete', 'skip_to_text_rate'],
    safety: {
      nightOutdoorRequired: false,
      extremeWeatherRequired: false,
      crossCityChase: false,
      rareAnimalGather: false,
    },
    missPolicy: 'rerun_window',
    rollback: 'theme off',
    welfareReview: 'no rare species aggregation',
  },
  {
    id: 'tpl.welfare',
    kind: 'welfare_day',
    title: '福利日',
    durationDays: 1,
    rewardBudget: 30,
    metrics: ['claim_once', 'opt_in_rate'],
    safety: {
      nightOutdoorRequired: false,
      extremeWeatherRequired: false,
      crossCityChase: false,
      rareAnimalGather: false,
    },
    missPolicy: 'compensate_soft',
    rollback: 'stop grants',
    welfareReview: 'no pay-to-skip anxiety',
  },
]

/** Build a 90-day cadence rotating templates without unsafe stacking. */
export function buildNinetyDayCalendar(): CalendarSlot[] {
  const order = [
    'tpl.observation_week',
    'tpl.knowledge',
    'tpl.city_research',
    'tpl.photo',
    'tpl.welfare',
  ]
  const slots: CalendarSlot[] = []
  let day = 0
  let i = 0
  while (day < 90) {
    const templateId = order[i % order.length]
    const tpl = TEMPLATES.find((t) => t.id === templateId)
    if (!tpl) break
    const dur = Math.min(tpl.durationDays, 90 - day)
    slots.push({
      dayOffset: day,
      templateId,
      targetPlayers: i % 3 === 0 ? 'new' : i % 3 === 1 ? 'returning' : 'all',
      notes: `${tpl.title} window ${dur}d`,
    })
    day += dur
    // breathing room day between major blocks
    if (day < 90 && tpl.durationDays >= 5) {
      day += 1
    }
    i++
  }
  return slots
}

export function templateById(id: string): EventTemplate | undefined {
  return TEMPLATES.find((t) => t.id === id)
}

/** Overlap rule: max one primary event; welfare may soft-stack. */
export function canStack(a: EventTemplate, b: EventTemplate): boolean {
  if (a.id === b.id) return false
  return a.kind === 'welfare_day' || b.kind === 'welfare_day'
}

export function missDoesNotLockCore(t: EventTemplate): boolean {
  return t.missPolicy === 'compensate_soft' || t.missPolicy === 'rerun_window' || t.missPolicy === 'catalog_unlock'
}
