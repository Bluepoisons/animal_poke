import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  type ReactNode,
} from 'react'
import { useStamina } from '../stamina/useStamina'
import { safeGetItem, safeSetItem } from '../utils/safeStorage'
import { PROGRESSION_STORAGE_KEY } from './constants'
import {
  applyProgressEvent,
  buildReturnSummary,
  buildSnapshot,
  createDefaultProgressionState,
  ensureDailyGoals,
  getUnlockedFeatures,
  isFeatureUnlocked,
  normalizeProgressionState,
  selectExecutableGoals,
  syncGoalCompletions,
  getGoalsByHorizon,
  getPrimaryHorizon,
  getDayIndex,
} from './logic'
import type {
  FeatureId,
  ProgressionAction,
  ProgressionContextValue,
  ProgressionState,
} from './types'

function loadInitialState(): ProgressionState {
  const now = Date.now()
  const saved = safeGetItem<Partial<ProgressionState> | null>(PROGRESSION_STORAGE_KEY, null)
  return normalizeProgressionState(saved, now)
}

function progressionReducer(state: ProgressionState, action: ProgressionAction): ProgressionState {
  switch (action.type) {
    case 'TOUCH_ACTIVE':
      return applyProgressEvent(state, { type: 'touch', now: action.now })
    case 'RECORD_CAPTURE':
      return applyProgressEvent(state, {
        type: 'capture',
        speciesNew: action.speciesNew,
        now: action.now,
      })
    case 'OPEN_POKEDEX':
      return applyProgressEvent(state, { type: 'open_pokedex', now: action.now })
    case 'SAFE_EXPLORE':
      return applyProgressEvent(state, { type: 'safe_explore', now: action.now })
    case 'RECORD_BATTLE':
      return applyProgressEvent(state, { type: 'battle', now: action.now })
    case 'RECORD_DISPATCH':
      return applyProgressEvent(state, { type: 'dispatch', now: action.now })
    case 'VISIT_CITY':
      return applyProgressEvent(state, {
        type: 'visit_city',
        isNew: action.isNew,
        now: action.now,
      })
    case 'SEASON_CHECKIN':
      return applyProgressEvent(state, { type: 'season_checkin', now: action.now })
    case 'COMPLETE_GOAL': {
      if (state.completedGoalIds.includes(action.goalId)) return state
      return {
        ...state,
        completedGoalIds: [...state.completedGoalIds, action.goalId],
        lastActiveAt: action.now,
      }
    }
    case 'SET_DAILY_GOALS':
      return { ...state, dailyRollDate: action.date, dailyGoalIds: action.goalIds }
    case 'DISMISS_RETURN_BANNER':
      return { ...state, returnBannerDismissedAt: action.now, lastActiveAt: action.now }
    case 'LOAD_STATE':
      return action.state
    default:
      return state
  }
}

export const ProgressionContext = createContext<ProgressionContextValue | null>(null)

export function ProgressionProvider({ children }: { children: ReactNode }) {
  const stamina = useStamina()
  const [state, dispatch] = useReducer(progressionReducer, undefined, loadInitialState)

  // Persist
  useEffect(() => {
    safeSetItem(PROGRESSION_STORAGE_KEY, state)
  }, [state])

  // Sync completions when captures / counters change
  useEffect(() => {
    const synced = syncGoalCompletions(state, {
      totalCaptures: stamina.state.totalCaptures,
      uniqueSpeciesCount: state.uniqueSpeciesCount,
    })
    if (synced.completedGoalIds.length !== state.completedGoalIds.length) {
      dispatch({ type: 'LOAD_STATE', state: synced })
    }
  }, [stamina.state.totalCaptures, state])

  const snapshot = useMemo(
    () =>
      buildSnapshot(state, {
        level: stamina.state.level,
        totalCaptures: stamina.state.totalCaptures,
        currentStamina: stamina.state.currentStamina,
        battlesPlayed: Math.max(state.battlesPlayed, stamina.state.totalBattles),
      }),
    [state, stamina.state.level, stamina.state.totalCaptures, stamina.state.currentStamina, stamina.state.totalBattles],
  )

  // Ensure daily roll
  useEffect(() => {
    const rolled = ensureDailyGoals(state, snapshot)
    if (
      rolled.dailyRollDate !== state.dailyRollDate ||
      rolled.dailyGoalIds.join() !== state.dailyGoalIds.join()
    ) {
      dispatch({
        type: 'SET_DAILY_GOALS',
        date: rolled.dailyRollDate,
        goalIds: rolled.dailyGoalIds,
      })
    }
  }, [state, snapshot])

  const dailyGoals = useMemo(() => selectExecutableGoals(snapshot, 3), [snapshot])

  const horizonGoals = useMemo(() => {
    const dayIndex = getDayIndex(snapshot.firstOpenAt, snapshot.now)
    const primary = getPrimaryHorizon(dayIndex)
    return getGoalsByHorizon(primary, snapshot)
  }, [snapshot])

  const unlockedFeatures = useMemo(() => getUnlockedFeatures(snapshot), [snapshot])

  const returnSummary = useMemo(() => {
    const summary = buildReturnSummary(snapshot)
    // If user dismissed banner for this return window, hide isReturning for UI
    if (
      summary.isReturning &&
      state.returnBannerDismissedAt != null &&
      state.returnBannerDismissedAt >= state.lastActiveAt - 60_000
    ) {
      // Keep data but flag for UI via dismissed — we expose raw summary;
      // AnimalPokeApp checks returnBannerDismissedAt separately if needed.
    }
    return summary
  }, [snapshot, state.returnBannerDismissedAt, state.lastActiveAt])

  const isFeatureUnlockedFn = useCallback(
    (feature: FeatureId) => isFeatureUnlocked(feature, snapshot),
    [snapshot],
  )

  const recordCapture = useCallback((speciesNew = false) => {
    dispatch({ type: 'RECORD_CAPTURE', speciesNew, now: Date.now() })
  }, [])

  const openPokedex = useCallback(() => {
    dispatch({ type: 'OPEN_POKEDEX', now: Date.now() })
  }, [])

  const safeExplore = useCallback(() => {
    dispatch({ type: 'SAFE_EXPLORE', now: Date.now() })
  }, [])

  const recordBattle = useCallback(() => {
    dispatch({ type: 'RECORD_BATTLE', now: Date.now() })
  }, [])

  const recordDispatch = useCallback(() => {
    dispatch({ type: 'RECORD_DISPATCH', now: Date.now() })
  }, [])

  const visitCity = useCallback((isNew = true) => {
    dispatch({ type: 'VISIT_CITY', isNew, now: Date.now() })
  }, [])

  const seasonCheckin = useCallback(() => {
    dispatch({ type: 'SEASON_CHECKIN', now: Date.now() })
  }, [])

  const dismissReturnBanner = useCallback(() => {
    dispatch({ type: 'DISMISS_RETURN_BANNER', now: Date.now() })
  }, [])

  const completeGoal = useCallback((goalId: string) => {
    dispatch({ type: 'COMPLETE_GOAL', goalId, now: Date.now() })
  }, [])

  const value = useMemo<ProgressionContextValue>(
    () => ({
      state,
      snapshot,
      dailyGoals,
      horizonGoals,
      unlockedFeatures,
      isFeatureUnlocked: isFeatureUnlockedFn,
      returnSummary,
      recordCapture,
      openPokedex,
      safeExplore,
      recordBattle,
      recordDispatch,
      visitCity,
      seasonCheckin,
      dismissReturnBanner,
      completeGoal,
    }),
    [
      state,
      snapshot,
      dailyGoals,
      horizonGoals,
      unlockedFeatures,
      isFeatureUnlockedFn,
      returnSummary,
      recordCapture,
      openPokedex,
      safeExplore,
      recordBattle,
      recordDispatch,
      visitCity,
      seasonCheckin,
      dismissReturnBanner,
      completeGoal,
    ],
  )

  return (
    <ProgressionContext.Provider value={value}>{children}</ProgressionContext.Provider>
  )
}

export function useProgression(): ProgressionContextValue {
  const ctx = useContext(ProgressionContext)
  if (!ctx) {
    throw new Error('useProgression 必须在 ProgressionProvider 内使用')
  }
  return ctx
}
