import {
  EXECUTABLE_GOAL_COUNT,
  FEATURE_GATES,
  FEATURE_GATE_MAP,
  GOAL_DEFS,
  GOAL_MAP,
  MS_PER_DAY,
  RETURN_THRESHOLD_DAYS,
  TAB_FEATURE_ORDER,
} from './constants'
import type {
  FeatureId,
  FeatureUnlockStatus,
  GoalDef,
  GoalHorizon,
  GoalProgress,
  ProgressSnapshot,
  ProgressionState,
  ReturnSummary,
} from './types'

/** UTC calendar day key YYYY-MM-DD */
export function toDateKey(ts: number): string {
  const d = new Date(ts)
  const y = d.getUTCFullYear()
  const m = String(d.getUTCMonth() + 1).padStart(2, '0')
  const day = String(d.getUTCDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

/** Whole days between two timestamps (floor, non-negative) */
export function daysBetween(fromTs: number, toTs: number): number {
  if (!Number.isFinite(fromTs) || !Number.isFinite(toTs)) return 0
  const delta = toTs - fromTs
  if (delta <= 0) return 0
  return Math.floor(delta / MS_PER_DAY)
}

/**
 * 0-based day index since first open.
 * D1 = 0, D7 = 6, D30 = 29.
 */
export function getDayIndex(firstOpenAt: number, now: number): number {
  return daysBetween(firstOpenAt, now)
}

/** Primary horizon focus for the current day index */
export function getPrimaryHorizon(dayIndex: number): GoalHorizon {
  if (dayIndex < 6) return 'D1'
  if (dayIndex < 29) return 'D7'
  return 'D30'
}

/** Create default persisted progression state */
export function createDefaultProgressionState(now = Date.now()): ProgressionState {
  return {
    firstOpenAt: now,
    lastActiveAt: now,
    pokedexViews: 0,
    safeExplores: 0,
    seasonCheckins: 0,
    uniqueSpeciesCount: 0,
    citiesVisited: 0,
    battlesPlayed: 0,
    dispatchesCompleted: 0,
    progress: {},
    completedGoalIds: [],
    dailyRollDate: '',
    dailyGoalIds: [],
    returnBannerDismissedAt: null,
  }
}

/** Merge partial / legacy saves safely */
export function normalizeProgressionState(
  raw: Partial<ProgressionState> | null | undefined,
  now = Date.now(),
): ProgressionState {
  const base = createDefaultProgressionState(now)
  if (!raw || typeof raw !== 'object') return base
  return {
    firstOpenAt: typeof raw.firstOpenAt === 'number' ? raw.firstOpenAt : base.firstOpenAt,
    lastActiveAt: typeof raw.lastActiveAt === 'number' ? raw.lastActiveAt : base.lastActiveAt,
    pokedexViews: Math.max(0, Number(raw.pokedexViews) || 0),
    safeExplores: Math.max(0, Number(raw.safeExplores) || 0),
    seasonCheckins: Math.max(0, Number(raw.seasonCheckins) || 0),
    uniqueSpeciesCount: Math.max(0, Number(raw.uniqueSpeciesCount) || 0),
    citiesVisited: Math.max(0, Number(raw.citiesVisited) || 0),
    battlesPlayed: Math.max(0, Number(raw.battlesPlayed) || 0),
    dispatchesCompleted: Math.max(0, Number(raw.dispatchesCompleted) || 0),
    progress:
      raw.progress && typeof raw.progress === 'object' && !Array.isArray(raw.progress)
        ? { ...raw.progress }
        : {},
    completedGoalIds: Array.isArray(raw.completedGoalIds)
      ? raw.completedGoalIds.filter((id): id is string => typeof id === 'string')
      : [],
    dailyRollDate: typeof raw.dailyRollDate === 'string' ? raw.dailyRollDate : '',
    dailyGoalIds: Array.isArray(raw.dailyGoalIds)
      ? raw.dailyGoalIds.filter((id): id is string => typeof id === 'string')
      : [],
    returnBannerDismissedAt:
      typeof raw.returnBannerDismissedAt === 'number' ? raw.returnBannerDismissedAt : null,
  }
}

/**
 * Build snapshot from progression state + live stamina/level/captures.
 */
export function buildSnapshot(
  state: ProgressionState,
  live: {
    now?: number
    level: number
    totalCaptures: number
    currentStamina: number
    uniqueSpeciesCount?: number
    citiesVisited?: number
    battlesPlayed?: number
  },
): ProgressSnapshot {
  const now = live.now ?? Date.now()
  return {
    now,
    firstOpenAt: state.firstOpenAt,
    lastActiveAt: state.lastActiveAt,
    level: Math.max(1, live.level | 0),
    totalCaptures: Math.max(0, live.totalCaptures | 0),
    uniqueSpeciesCount: Math.max(
      0,
      live.uniqueSpeciesCount ?? state.uniqueSpeciesCount,
    ),
    citiesVisited: Math.max(0, live.citiesVisited ?? state.citiesVisited),
    battlesPlayed: Math.max(0, live.battlesPlayed ?? state.battlesPlayed),
    dispatchesCompleted: Math.max(0, state.dispatchesCompleted),
    pokedexViews: Math.max(0, state.pokedexViews),
    safeExplores: Math.max(0, state.safeExplores),
    seasonCheckins: Math.max(0, state.seasonCheckins),
    currentStamina: Math.max(0, live.currentStamina | 0),
    progress: { ...state.progress },
    completedGoalIds: [...state.completedGoalIds],
  }
}

/** Current counter for a goal from snapshot (action-based fallback) */
export function getGoalCounter(def: GoalDef, snap: ProgressSnapshot): number {
  if (snap.completedGoalIds.includes(def.id)) return def.target
  const explicit = snap.progress[def.id]
  if (typeof explicit === 'number' && explicit >= 0) {
    return Math.min(def.target, explicit)
  }
  switch (def.action) {
    case 'capture':
      // cumulative capture goals use totalCaptures; daily ones rely on progress map
      if (def.horizon === 'daily') return 0
      return Math.min(def.target, snap.totalCaptures)
    case 'open_pokedex':
      return Math.min(def.target, snap.pokedexViews)
    case 'safe_explore':
      return Math.min(def.target, snap.safeExplores)
    case 'battle':
      return Math.min(def.target, snap.battlesPlayed)
    case 'dispatch':
      return Math.min(def.target, snap.dispatchesCompleted)
    case 'collect_species':
      return Math.min(def.target, snap.uniqueSpeciesCount)
    case 'visit_region':
      return Math.min(def.target, snap.citiesVisited)
    case 'season_checkin':
      return Math.min(def.target, snap.seasonCheckins)
    default:
      return 0
  }
}

export function isFeatureUnlocked(feature: FeatureId, snap: ProgressSnapshot): boolean {
  const gate = FEATURE_GATE_MAP[feature]
  if (!gate) return false
  if (gate.always) return true
  const dayIndex = getDayIndex(snap.firstOpenAt, snap.now)
  if (snap.level < gate.minLevel) return false
  if (snap.totalCaptures < gate.minCaptures) return false
  if (dayIndex < gate.minDayIndex) return false
  if (snap.battlesPlayed < gate.minBattles) return false
  if (snap.uniqueSpeciesCount < gate.minSpecies) return false
  return true
}

export function getFeatureUnlockStatus(
  feature: FeatureId,
  snap: ProgressSnapshot,
): FeatureUnlockStatus {
  const unlocked = isFeatureUnlocked(feature, snap)
  if (unlocked) return { feature, unlocked: true, lockReason: null }
  const gate = FEATURE_GATE_MAP[feature]
  if (!gate) return { feature, unlocked: false, lockReason: '未知功能' }
  const dayIndex = getDayIndex(snap.firstOpenAt, snap.now)
  if (snap.level < gate.minLevel) {
    return { feature, unlocked: false, lockReason: `需要等级 ${gate.minLevel}` }
  }
  if (snap.totalCaptures < gate.minCaptures) {
    return { feature, unlocked: false, lockReason: `需要捕获 ${gate.minCaptures} 只` }
  }
  if (dayIndex < gate.minDayIndex) {
    return { feature, unlocked: false, lockReason: `第 ${gate.minDayIndex + 1} 天开放` }
  }
  if (snap.battlesPlayed < gate.minBattles) {
    return { feature, unlocked: false, lockReason: `需要战斗 ${gate.minBattles} 场` }
  }
  if (snap.uniqueSpeciesCount < gate.minSpecies) {
    return { feature, unlocked: false, lockReason: `需要 ${gate.minSpecies} 个物种` }
  }
  return { feature, unlocked: false, lockReason: '尚未解锁' }
}

export function listFeatureUnlocks(snap: ProgressSnapshot): FeatureUnlockStatus[] {
  return FEATURE_GATES.map((g) => getFeatureUnlockStatus(g.feature, snap))
}

export function getUnlockedFeatures(snap: ProgressSnapshot): Set<FeatureId> {
  const set = new Set<FeatureId>()
  for (const g of FEATURE_GATES) {
    if (isFeatureUnlocked(g.feature, snap)) set.add(g.feature)
  }
  return set
}

/**
 * Visible bottom tabs — locked features are omitted (no toast spam).
 */
export function getVisibleTabs(
  snap: ProgressSnapshot,
): Array<'discover' | 'pokedex' | 'battle' | 'store' | 'achievement'> {
  return TAB_FEATURE_ORDER.filter((id) => isFeatureUnlocked(id, snap))
}

/** Whether goal prerequisites (level/captures/day) are met */
export function isGoalUnlocked(def: GoalDef, snap: ProgressSnapshot): boolean {
  const dayIndex = getDayIndex(snap.firstOpenAt, snap.now)
  if (dayIndex < def.minDayIndex) return false
  if (snap.level < def.minLevel) return false
  if (snap.totalCaptures < def.minCaptures) return false
  // battle/dispatch goals also need feature unlock
  if (def.action === 'battle' && !isFeatureUnlocked('battle', snap)) return false
  if (def.action === 'dispatch' && !isFeatureUnlocked('dispatch', snap)) return false
  if (def.action === 'season_checkin' && def.horizon !== 'D1' && !isFeatureUnlocked('season', snap)) {
    // D30 season goals need season feature; daily season same
    if (def.minDayIndex >= 7) return isFeatureUnlocked('season', snap)
  }
  return true
}

export function computeGoalProgress(def: GoalDef, snap: ProgressSnapshot): GoalProgress {
  const current = getGoalCounter(def, snap)
  const completed = current >= def.target || snap.completedGoalIds.includes(def.id)
  const unlocked = isGoalUnlocked(def, snap)
  const blockedByStamina = !def.free && snap.currentStamina <= 0 && !completed
  const executable = unlocked && !completed && (def.free || snap.currentStamina > 0)
  return {
    def,
    current: Math.min(current, def.target),
    target: def.target,
    completed,
    executable,
    blockedByStamina,
  }
}

export function getGoalsByHorizon(horizon: GoalHorizon, snap: ProgressSnapshot): GoalProgress[] {
  return GOAL_DEFS.filter((g) => g.horizon === horizon).map((g) => computeGoalProgress(g, snap))
}

/**
 * Fill up to `limit` executable goals.
 * Priority:
 * 1) Primary horizon incomplete executable
 * 2) Free goals when stamina exhausted (always keep free activities)
 * 3) Daily pool
 * 4) Other incomplete unlocked goals
 *
 * Day-1 guarantee: when dayIndex === 0, the D1 trio is preferred so
 * first day always surfaces 3 executable goals.
 */
export function selectExecutableGoals(
  snap: ProgressSnapshot,
  limit = EXECUTABLE_GOAL_COUNT,
): GoalProgress[] {
  const dayIndex = getDayIndex(snap.firstOpenAt, snap.now)
  const primary = getPrimaryHorizon(dayIndex)
  const staminaEmpty = snap.currentStamina <= 0

  const all = GOAL_DEFS.map((g) => computeGoalProgress(g, snap))
  const incomplete = all.filter((g) => !g.completed && isGoalUnlocked(g.def, snap))

  const pick: GoalProgress[] = []
  const seen = new Set<string>()

  const push = (g: GoalProgress) => {
    if (pick.length >= limit) return
    if (seen.has(g.def.id)) return
    // When stamina empty, only free goals are executable
    if (staminaEmpty && !g.def.free) return
    if (!g.executable && !(staminaEmpty && g.def.free && !g.completed)) return
    // recompute executable for free when stamina empty
    if (staminaEmpty && g.def.free && !g.completed) {
      pick.push({ ...g, executable: true, blockedByStamina: false })
    } else if (g.executable) {
      pick.push(g)
    } else {
      return
    }
    seen.add(g.def.id)
  }

  // D1 guarantee
  if (dayIndex === 0) {
    for (const g of incomplete.filter((x) => x.def.horizon === 'D1')) push(g)
  }

  // Primary horizon
  for (const g of incomplete.filter((x) => x.def.horizon === primary)) push(g)

  // Free first when stamina empty
  if (staminaEmpty) {
    for (const g of incomplete.filter((x) => x.def.free)) push(g)
  }

  // Daily pool
  for (const g of incomplete.filter((x) => x.def.horizon === 'daily')) push(g)

  // Any remaining executable
  for (const g of incomplete) push(g)

  // If still short and stamina empty, force free incomplete (even if blocked flags odd)
  if (pick.length < limit && staminaEmpty) {
    for (const g of all) {
      if (pick.length >= limit) break
      if (g.completed || !g.def.free) continue
      if (!isGoalUnlocked(g.def, snap)) continue
      if (seen.has(g.def.id)) continue
      pick.push({ ...g, executable: true, blockedByStamina: false })
      seen.add(g.def.id)
    }
  }

  return pick.slice(0, limit)
}

/**
 * Day-1 always returns 3 executable goals (acceptance).
 * Uses D1 defs; if some completed, fills from free/daily.
 */
export function getDayOneGoals(snap: ProgressSnapshot): GoalProgress[] {
  return selectExecutableGoals({ ...snap, firstOpenAt: snap.now, lastActiveAt: snap.now }, 3)
}

/**
 * Returning player summary — calm next step, no FOMO copy.
 */
export function buildReturnSummary(snap: ProgressSnapshot): ReturnSummary {
  const daysAway = daysBetween(snap.lastActiveAt, snap.now)
  const dayIndex = getDayIndex(snap.firstOpenAt, snap.now)
  const isReturning = daysAway >= RETURN_THRESHOLD_DAYS

  const freeActivities = selectExecutableGoals(
    { ...snap, currentStamina: 0 },
    EXECUTABLE_GOAL_COUNT,
  ).filter((g) => g.def.free)

  if (!isReturning) {
    return {
      isReturning: false,
      daysAway,
      dayIndex,
      headline: '',
      summaryLines: [],
      nextStep: selectExecutableGoals(snap, 1)[0] ?? null,
      freeActivities,
    }
  }

  const nextStep = selectExecutableGoals(snap, 1)[0] ?? null
  const summaryLines = [
    `等级 ${snap.level} · 已收藏 ${snap.totalCaptures} 只`,
    snap.uniqueSpeciesCount > 0 ? `物种 ${snap.uniqueSpeciesCount} 种` : '物种收集可继续推进',
    snap.citiesVisited > 0 ? `造访区域 ${snap.citiesVisited}` : '可从附近地图开始探索',
  ]

  return {
    isReturning: true,
    daysAway,
    dayIndex,
    headline: `欢迎回来，你离开了 ${daysAway} 天`,
    summaryLines,
    nextStep,
    freeActivities,
  }
}

/**
 * Apply action counters into progression state (pure).
 * Does not auto-complete goals; call syncGoalCompletions after.
 * `touch` is a no-op so session open does not clear return-window detection.
 */
export function applyProgressEvent(
  state: ProgressionState,
  event:
    | { type: 'capture'; speciesNew?: boolean; now: number }
    | { type: 'open_pokedex'; now: number }
    | { type: 'safe_explore'; now: number }
    | { type: 'battle'; now: number }
    | { type: 'dispatch'; now: number }
    | { type: 'visit_city'; isNew?: boolean; now: number }
    | { type: 'season_checkin'; now: number }
    | { type: 'touch'; now: number },
): ProgressionState {
  if (event.type === 'touch') {
    return state
  }

  const next: ProgressionState = {
    ...state,
    progress: { ...state.progress },
    completedGoalIds: [...state.completedGoalIds],
    lastActiveAt: event.now,
  }

  const bump = (goalId: string, by = 1) => {
    next.progress[goalId] = (next.progress[goalId] ?? 0) + by
  }

  switch (event.type) {
    case 'capture': {
      bump('daily_capture')
      bump('d1_first_capture')
      bump('d30_collection_20')
      if (event.speciesNew) {
        next.uniqueSpeciesCount = Math.max(0, next.uniqueSpeciesCount) + 1
      }
      break
    }
    case 'open_pokedex': {
      next.pokedexViews += 1
      bump('d1_open_pokedex')
      bump('daily_pokedex')
      break
    }
    case 'safe_explore': {
      next.safeExplores += 1
      bump('d1_safe_explore')
      bump('daily_safe_explore')
      break
    }
    case 'battle': {
      next.battlesPlayed += 1
      bump('d7_first_battle')
      bump('daily_battle')
      break
    }
    case 'dispatch': {
      next.dispatchesCompleted += 1
      bump('d7_first_dispatch')
      break
    }
    case 'visit_city': {
      if (event.isNew !== false) {
        next.citiesVisited += 1
        bump('d30_three_regions')
      }
      break
    }
    case 'season_checkin': {
      next.seasonCheckins += 1
      bump('d30_season_checkins')
      bump('daily_season')
      break
    }
  }

  return next
}

/**
 * Mark goals complete when counters reach targets.
 * Uses live totalCaptures for capture-based cumulative goals.
 */
export function syncGoalCompletions(
  state: ProgressionState,
  live: { totalCaptures: number; uniqueSpeciesCount?: number },
): ProgressionState {
  const snap = buildSnapshot(state, {
    level: 1,
    totalCaptures: live.totalCaptures,
    currentStamina: 1,
    uniqueSpeciesCount: live.uniqueSpeciesCount ?? state.uniqueSpeciesCount,
  })
  const completed = new Set(state.completedGoalIds)
  let changed = false
  for (const def of GOAL_DEFS) {
    if (completed.has(def.id)) continue
    const current = getGoalCounter(def, { ...snap, completedGoalIds: [...completed] })
    if (current >= def.target) {
      completed.add(def.id)
      changed = true
    }
  }
  if (!changed) return state
  return { ...state, completedGoalIds: [...completed] }
}

/** Roll daily goal ids for a date key if needed */
export function ensureDailyGoals(
  state: ProgressionState,
  snap: ProgressSnapshot,
): ProgressionState {
  const dateKey = toDateKey(snap.now)
  if (state.dailyRollDate === dateKey && state.dailyGoalIds.length > 0) {
    return state
  }
  // Pick up to 3 from daily pool that are unlocked; prefer free when no stamina
  const dailyDefs = GOAL_DEFS.filter((g) => g.horizon === 'daily')
  const ranked = dailyDefs
    .map((d) => computeGoalProgress(d, snap))
    .filter((g) => isGoalUnlocked(g.def, snap))
    .sort((a, b) => {
      if (snap.currentStamina <= 0) {
        if (a.def.free !== b.def.free) return a.def.free ? -1 : 1
      }
      return 0
    })
  const ids = ranked.slice(0, EXECUTABLE_GOAL_COUNT).map((g) => g.def.id)
  // Always ensure free fillers exist in the roll when possible
  if (snap.currentStamina <= 0) {
    for (const d of dailyDefs) {
      if (ids.length >= EXECUTABLE_GOAL_COUNT) break
      if (!d.free) continue
      if (!ids.includes(d.id) && isGoalUnlocked(d, snap)) ids.push(d.id)
    }
  }
  return { ...state, dailyRollDate: dateKey, dailyGoalIds: ids }
}

export function getGoalDef(id: string): GoalDef | undefined {
  return GOAL_MAP[id]
}

export function listAllGoalProgress(snap: ProgressSnapshot): GoalProgress[] {
  return GOAL_DEFS.map((g) => computeGoalProgress(g, snap))
}
