/**
 * AP-050: D1/D7/D30 progression goals, unlock gates, return path.
 * Client-side pure types for the local progression engine.
 */

/** Goal time horizon */
export type GoalHorizon = 'D1' | 'D7' | 'D30' | 'daily'

/** Player action that advances a goal */
export type GoalAction =
  | 'capture'
  | 'open_pokedex'
  | 'safe_explore'
  | 'battle'
  | 'dispatch'
  | 'collect_species'
  | 'visit_region'
  | 'season_checkin'

/** Feature ids that can be gated by level / behavior / day */
export type FeatureId =
  | 'discover'
  | 'map'
  | 'pokedex'
  | 'battle'
  | 'store'
  | 'achievement'
  | 'dispatch'
  | 'season'
  | 'pvp'

/** Navigation target used by UI (aligned with ScreenId + soft links) */
export type GoalNavigateTo =
  | 'discover'
  | 'map'
  | 'capture'
  | 'pokedex'
  | 'battle'
  | 'store'
  | 'dispatch'
  | 'none'

/** Static goal definition */
export interface GoalDef {
  id: string
  horizon: GoalHorizon
  title: string
  description: string
  action: GoalAction
  target: number
  /** true = does not require stamina (free activity) */
  free: boolean
  navigateTo: GoalNavigateTo
  rewardGold: number
  /** Minimum player level to show as executable (0 = always) */
  minLevel: number
  /** Minimum total captures to show as executable */
  minCaptures: number
  /** Minimum calendar days since first open (0-based day index: D1 = 0) */
  minDayIndex: number
}

/** Feature unlock gate */
export interface FeatureUnlockGate {
  feature: FeatureId
  minLevel: number
  minCaptures: number
  /** 0-based day index since first open */
  minDayIndex: number
  minBattles: number
  minSpecies: number
  /** Always unlocked when true */
  always?: boolean
}

/**
 * Snapshot of player progression inputs.
 * All counters are non-negative integers; timestamps are Unix ms.
 */
export interface ProgressSnapshot {
  now: number
  firstOpenAt: number
  lastActiveAt: number
  level: number
  totalCaptures: number
  uniqueSpeciesCount: number
  citiesVisited: number
  battlesPlayed: number
  dispatchesCompleted: number
  pokedexViews: number
  safeExplores: number
  seasonCheckins: number
  currentStamina: number
  /** goalId -> current progress value */
  progress: Record<string, number>
  completedGoalIds: string[]
}

/** Runtime progress for one goal */
export interface GoalProgress {
  def: GoalDef
  current: number
  target: number
  completed: boolean
  /** Can be pursued right now (unlock + free if stamina exhausted) */
  executable: boolean
  /** Blocked only because stamina is 0 and goal is not free */
  blockedByStamina: boolean
}

export interface FeatureUnlockStatus {
  feature: FeatureId
  unlocked: boolean
  /** Short reason when locked (for debug / future UI, not toast spam) */
  lockReason: string | null
}

/** Returning player summary — no FOMO pressure */
export interface ReturnSummary {
  isReturning: boolean
  daysAway: number
  dayIndex: number
  headline: string
  summaryLines: string[]
  nextStep: GoalProgress | null
  freeActivities: GoalProgress[]
}

/** Persisted progression state (localStorage) */
export interface ProgressionState {
  firstOpenAt: number
  lastActiveAt: number
  pokedexViews: number
  safeExplores: number
  seasonCheckins: number
  uniqueSpeciesCount: number
  citiesVisited: number
  battlesPlayed: number
  dispatchesCompleted: number
  progress: Record<string, number>
  completedGoalIds: string[]
  /** Last date (YYYY-MM-DD) daily goals were rolled */
  dailyRollDate: string
  /** Goal ids selected for today (up to 3) */
  dailyGoalIds: string[]
  /** Whether return banner was dismissed for this return session */
  returnBannerDismissedAt: number | null
}

export type ProgressionAction =
  | { type: 'TOUCH_ACTIVE'; now: number }
  | { type: 'RECORD_CAPTURE'; speciesNew: boolean; now: number }
  | { type: 'OPEN_POKEDEX'; now: number }
  | { type: 'SAFE_EXPLORE'; now: number }
  | { type: 'RECORD_BATTLE'; now: number }
  | { type: 'RECORD_DISPATCH'; now: number }
  | { type: 'VISIT_CITY'; isNew: boolean; now: number }
  | { type: 'SEASON_CHECKIN'; now: number }
  | { type: 'COMPLETE_GOAL'; goalId: string; now: number }
  | { type: 'SET_DAILY_GOALS'; date: string; goalIds: string[] }
  | { type: 'DISMISS_RETURN_BANNER'; now: number }
  | { type: 'LOAD_STATE'; state: ProgressionState }

export interface ProgressionContextValue {
  state: ProgressionState
  snapshot: ProgressSnapshot
  dailyGoals: GoalProgress[]
  horizonGoals: GoalProgress[]
  unlockedFeatures: Set<FeatureId>
  isFeatureUnlocked: (feature: FeatureId) => boolean
  returnSummary: ReturnSummary
  recordCapture: (speciesNew?: boolean) => void
  openPokedex: () => void
  safeExplore: () => void
  recordBattle: () => void
  recordDispatch: () => void
  visitCity: (isNew?: boolean) => void
  seasonCheckin: () => void
  dismissReturnBanner: () => void
  completeGoal: (goalId: string) => void
}
