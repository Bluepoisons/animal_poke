import { describe, it, expect } from 'vitest'
import {
  buildReturnSummary,
  buildSnapshot,
  computeGoalProgress,
  createDefaultProgressionState,
  daysBetween,
  getDayIndex,
  getFeatureUnlockStatus,
  getPrimaryHorizon,
  getUnlockedFeatures,
  getVisibleTabs,
  isFeatureUnlocked,
  isGoalUnlocked,
  normalizeProgressionState,
  selectExecutableGoals,
  applyProgressEvent,
  syncGoalCompletions,
  ensureDailyGoals,
  toDateKey,
} from './logic'
import { EXECUTABLE_GOAL_COUNT, GOAL_MAP, RETURN_THRESHOLD_DAYS } from './constants'
import type { ProgressSnapshot, ProgressionState } from './types'

const DAY = 86_400_000
const T0 = Date.UTC(2026, 6, 1, 12, 0, 0) // 2026-07-01

function snap(overrides: Partial<ProgressSnapshot> = {}): ProgressSnapshot {
  return {
    now: T0,
    firstOpenAt: T0,
    lastActiveAt: T0,
    level: 1,
    totalCaptures: 0,
    uniqueSpeciesCount: 0,
    citiesVisited: 0,
    battlesPlayed: 0,
    dispatchesCompleted: 0,
    pokedexViews: 0,
    safeExplores: 0,
    seasonCheckins: 0,
    currentStamina: 10,
    progress: {},
    completedGoalIds: [],
    ...overrides,
  }
}

describe('day index & horizon', () => {
  it('#1 D1 is dayIndex 0', () => {
    expect(getDayIndex(T0, T0)).toBe(0)
    expect(getPrimaryHorizon(0)).toBe('D1')
  })

  it('#2 D7 is dayIndex 6', () => {
    expect(getDayIndex(T0, T0 + 6 * DAY)).toBe(6)
    expect(getPrimaryHorizon(6)).toBe('D7')
  })

  it('#3 D30 is dayIndex 29', () => {
    expect(getDayIndex(T0, T0 + 29 * DAY)).toBe(29)
    expect(getPrimaryHorizon(29)).toBe('D30')
  })

  it('#4 daysBetween clamps negative', () => {
    expect(daysBetween(T0 + DAY, T0)).toBe(0)
  })
})

describe('goal computation', () => {
  it('#5 D1 always surfaces 3 executable goals for new players', () => {
    const goals = selectExecutableGoals(snap(), EXECUTABLE_GOAL_COUNT)
    expect(goals).toHaveLength(3)
    expect(goals.every((g) => g.executable)).toBe(true)
    const ids = goals.map((g) => g.def.id)
    expect(ids).toContain('d1_first_capture')
    expect(ids).toContain('d1_open_pokedex')
    expect(ids).toContain('d1_safe_explore')
  })

  it('#6 stamina exhausted still yields free executable activities', () => {
    const goals = selectExecutableGoals(snap({ currentStamina: 0 }), 3)
    expect(goals.length).toBeGreaterThanOrEqual(2)
    expect(goals.every((g) => g.def.free)).toBe(true)
    expect(goals.every((g) => g.executable)).toBe(true)
    // capture (non-free) must not appear
    expect(goals.some((g) => g.def.id === 'd1_first_capture')).toBe(false)
  })

  it('#7 capture progress completes d1_first_capture', () => {
    const g = computeGoalProgress(GOAL_MAP.d1_first_capture, snap({ totalCaptures: 1 }))
    expect(g.completed).toBe(true)
    expect(g.current).toBe(1)
  })

  it('#8 open_pokedex counter drives d1_open_pokedex', () => {
    const g = computeGoalProgress(GOAL_MAP.d1_open_pokedex, snap({ pokedexViews: 1 }))
    expect(g.completed).toBe(true)
  })

  it('#9 D7 species goal uses uniqueSpeciesCount', () => {
    const g = computeGoalProgress(
      GOAL_MAP.d7_three_species,
      snap({ uniqueSpeciesCount: 2, now: T0 + 6 * DAY }),
    )
    expect(g.current).toBe(2)
    expect(g.completed).toBe(false)
    const done = computeGoalProgress(
      GOAL_MAP.d7_three_species,
      snap({ uniqueSpeciesCount: 3, now: T0 + 6 * DAY }),
    )
    expect(done.completed).toBe(true)
  })

  it('#10 completed goals excluded from executable list', () => {
    const goals = selectExecutableGoals(
      snap({
        totalCaptures: 1,
        pokedexViews: 1,
        safeExplores: 1,
        completedGoalIds: ['d1_first_capture', 'd1_open_pokedex', 'd1_safe_explore'],
      }),
      3,
    )
    expect(goals.every((g) => !g.completed)).toBe(true)
    expect(goals.every((g) => !g.def.id.startsWith('d1_') || g.def.horizon === 'daily')).toBe(true)
  })
})

describe('feature unlock', () => {
  it('#11 discover/map/pokedex/store always unlocked', () => {
    const s = snap()
    expect(isFeatureUnlocked('discover', s)).toBe(true)
    expect(isFeatureUnlocked('map', s)).toBe(true)
    expect(isFeatureUnlocked('pokedex', s)).toBe(true)
    expect(isFeatureUnlocked('store', s)).toBe(true)
  })

  it('#12 battle locked for brand-new player; unlocked after capture+level', () => {
    expect(isFeatureUnlocked('battle', snap())).toBe(false)
    expect(
      isFeatureUnlocked(
        'battle',
        snap({ level: 2, totalCaptures: 1 }),
      ),
    ).toBe(true)
  })

  it('#13 dispatch unlocks around D7 gates', () => {
    expect(isFeatureUnlocked('dispatch', snap({ level: 3, totalCaptures: 3 }))).toBe(false)
    expect(
      isFeatureUnlocked(
        'dispatch',
        snap({
          level: 3,
          totalCaptures: 3,
          firstOpenAt: T0,
          now: T0 + 3 * DAY,
        }),
      ),
    ).toBe(true)
  })

  it('#14 locked features hidden from tabs (no toast spam path)', () => {
    const tabsNew = getVisibleTabs(snap())
    expect(tabsNew).toEqual(['discover', 'pokedex', 'store'])
    expect(tabsNew).not.toContain('battle')
    expect(tabsNew).not.toContain('achievement')

    const tabsReady = getVisibleTabs(snap({ level: 2, totalCaptures: 1 }))
    expect(tabsReady).toContain('battle')
    expect(tabsReady).toContain('achievement')
  })

  it('#15 lock reason is descriptive for UI debug, not toast', () => {
    const st = getFeatureUnlockStatus('pvp', snap())
    expect(st.unlocked).toBe(false)
    expect(st.lockReason).toMatch(/等级|捕获|天|战斗/)
  })

  it('#16 battle goal not unlocked until feature gate met', () => {
    expect(isGoalUnlocked(GOAL_MAP.d7_first_battle, snap({ now: T0 + DAY }))).toBe(false)
    expect(
      isGoalUnlocked(
        GOAL_MAP.d7_first_battle,
        snap({ level: 2, totalCaptures: 1, now: T0 + DAY }),
      ),
    ).toBe(true)
  })
})

describe('return summary', () => {
  it('#17 not returning when daysAway < threshold', () => {
    const r = buildReturnSummary(
      snap({ lastActiveAt: T0 - (RETURN_THRESHOLD_DAYS - 1) * DAY, now: T0 }),
    )
    expect(r.isReturning).toBe(false)
    expect(r.headline).toBe('')
  })

  it('#18 returning player gets calm summary + one next step', () => {
    const r = buildReturnSummary(
      snap({
        lastActiveAt: T0 - 5 * DAY,
        now: T0,
        totalCaptures: 4,
        level: 3,
        uniqueSpeciesCount: 2,
      }),
    )
    expect(r.isReturning).toBe(true)
    expect(r.daysAway).toBe(5)
    expect(r.headline).toMatch(/欢迎回来/)
    expect(r.headline).not.toMatch(/错过|限时|再不/)
    expect(r.summaryLines.length).toBeGreaterThan(0)
    expect(r.nextStep).not.toBeNull()
    expect(r.nextStep?.executable).toBe(true)
    expect(r.freeActivities.every((g) => g.def.free)).toBe(true)
  })

  it('#19 return free activities available even with 0 stamina', () => {
    const r = buildReturnSummary(
      snap({
        lastActiveAt: T0 - 10 * DAY,
        now: T0,
        currentStamina: 0,
      }),
    )
    expect(r.isReturning).toBe(true)
    expect(r.freeActivities.length).toBeGreaterThan(0)
  })
})

describe('state transitions', () => {
  it('#20 applyProgressEvent bumps counters and lastActiveAt', () => {
    let state = createDefaultProgressionState(T0)
    state = applyProgressEvent(state, { type: 'open_pokedex', now: T0 + 1000 })
    expect(state.pokedexViews).toBe(1)
    expect(state.lastActiveAt).toBe(T0 + 1000)
    expect(state.progress.d1_open_pokedex).toBe(1)

    state = applyProgressEvent(state, { type: 'safe_explore', now: T0 + 2000 })
    expect(state.safeExplores).toBe(1)

    state = applyProgressEvent(state, { type: 'capture', speciesNew: true, now: T0 + 3000 })
    expect(state.uniqueSpeciesCount).toBe(1)
  })

  it('#21 syncGoalCompletions marks finished goals', () => {
    let state = createDefaultProgressionState(T0)
    state = applyProgressEvent(state, { type: 'open_pokedex', now: T0 })
    state = applyProgressEvent(state, { type: 'safe_explore', now: T0 })
    state = syncGoalCompletions(state, { totalCaptures: 1 })
    expect(state.completedGoalIds).toContain('d1_open_pokedex')
    expect(state.completedGoalIds).toContain('d1_safe_explore')
    expect(state.completedGoalIds).toContain('d1_first_capture')
  })

  it('#22 ensureDailyGoals rolls once per date', () => {
    const state = createDefaultProgressionState(T0)
    const s = snap({ now: T0 })
    const rolled = ensureDailyGoals(state, s)
    expect(rolled.dailyRollDate).toBe(toDateKey(T0))
    expect(rolled.dailyGoalIds.length).toBeGreaterThan(0)
    const again = ensureDailyGoals(rolled, s)
    expect(again.dailyGoalIds).toEqual(rolled.dailyGoalIds)
  })

  it('#23 normalizeProgressionState repairs corrupt saves', () => {
    const n = normalizeProgressionState({ pokedexViews: -3, completedGoalIds: 'bad' as unknown as string[] }, T0)
    expect(n.pokedexViews).toBe(0)
    expect(Array.isArray(n.completedGoalIds)).toBe(true)
  })

  it('#24 buildSnapshot merges live stamina fields', () => {
    const state: ProgressionState = createDefaultProgressionState(T0)
    const out = buildSnapshot(state, {
      now: T0,
      level: 4,
      totalCaptures: 9,
      currentStamina: 0,
    })
    expect(out.level).toBe(4)
    expect(out.totalCaptures).toBe(9)
    expect(out.currentStamina).toBe(0)
  })

  it('#25 getUnlockedFeatures returns Set membership', () => {
    const set = getUnlockedFeatures(snap({ level: 5, totalCaptures: 5, battlesPlayed: 3, now: T0 + 20 * DAY }))
    expect(set.has('battle')).toBe(true)
    expect(set.has('pvp')).toBe(true)
  })
})
