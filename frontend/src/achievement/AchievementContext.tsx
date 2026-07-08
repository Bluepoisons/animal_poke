import React, { createContext, useReducer, useEffect, useCallback, useMemo } from 'react'
import { useStamina } from '../stamina/useStamina'
import { ACHIEVEMENT_DEFS, ACHIEVEMENT_MAP, ACHIEVEMENT_STORAGE_KEY } from './constants'
import type {
  AchievementState,
  AchievementAction,
  AchievementContextValue,
  AchievementStats,
  AchievementProgress,
  AchievementCheckResult,
  AchievementCategory,
} from './types'
import {
  checkAchievements as checkAchievementsLogic,
  getAchievementProgress,
  getAllAchievementProgress,
} from './logic'

/** 从 localStorage 加载初始状态 */
function loadInitialState(): AchievementState {
  try {
    const saved = localStorage.getItem(ACHIEVEMENT_STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as AchievementState
      if (
        !Array.isArray(parsed.unlocked) ||
        !Array.isArray(parsed.unlockedTitles) ||
        !Array.isArray(parsed.pendingNotifications)
      ) {
        throw new Error('成就存档字段校验失败')
      }
      return parsed
    }
  } catch (e) {
    console.warn('加载成就存档失败，使用默认值:', e)
  }
  return {
    unlocked: [],
    unlockedTitles: [],
    activeTitle: null,
    pendingNotifications: [],
  }
}

/** Reducer */
function achievementReducer(state: AchievementState, action: AchievementAction): AchievementState {
  switch (action.type) {
    case 'UNLOCK_ACHIEVEMENTS': {
      const newUnlocked = action.ids.map((id) => ({
        id,
        unlockedAt: action.unlockedAt,
      }))
      const newTitles = action.ids
        .map((id) => ACHIEVEMENT_MAP[id]?.reward.title)
        .filter((t): t is string => !!t)

      return {
        ...state,
        unlocked: [...state.unlocked, ...newUnlocked],
        unlockedTitles: [...state.unlockedTitles, ...newTitles],
        pendingNotifications: [...state.pendingNotifications, ...action.ids],
      }
    }

    case 'SET_ACTIVE_TITLE': {
      return { ...state, activeTitle: action.title || null }
    }

    case 'ENQUEUE_NOTIFICATION': {
      return {
        ...state,
        pendingNotifications: [...state.pendingNotifications, action.id],
      }
    }

    case 'DEQUEUE_NOTIFICATION': {
      const [, ...rest] = state.pendingNotifications
      return { ...state, pendingNotifications: rest }
    }

    case 'LOAD_STATE': {
      return action.state
    }

    default:
      return state
  }
}

export const AchievementContext = createContext<AchievementContextValue | null>(null)

export const AchievementProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const stamina = useStamina()
  const [state, dispatch] = useReducer(achievementReducer, undefined, loadInitialState)

  // localStorage 持久化
  useEffect(() => {
    localStorage.setItem(ACHIEVEMENT_STORAGE_KEY, JSON.stringify(state))
  }, [state])

  const checkAchievements = useCallback((stats: AchievementStats): AchievementCheckResult => {
    const alreadyUnlocked = new Set(state.unlocked.map((u) => u.id))
    const result = checkAchievementsLogic(stats, alreadyUnlocked)

    if (result.changed) {
      dispatch({
        type: 'UNLOCK_ACHIEVEMENTS',
        ids: result.newlyUnlocked,
        unlockedAt: Date.now(),
      })
      // 发放金币奖励
      let totalGold = 0
      for (const id of result.newlyUnlocked) {
        const def = ACHIEVEMENT_MAP[id]
        if (def) totalGold += def.reward.gold
      }
      if (totalGold > 0) {
        stamina.addGold(totalGold)
      }
    }

    return result
  }, [state.unlocked, stamina])

  const getProgress = useCallback((id: string, stats: AchievementStats): AchievementProgress => {
    const def = ACHIEVEMENT_MAP[id]
    if (!def) throw new Error(`成就 ${id} 不存在`)
    const unlocked = state.unlocked.some((u) => u.id === id)
    return getAchievementProgress(def, stats, unlocked)
  }, [state.unlocked])

  const getAllProgress = useCallback((stats: AchievementStats): AchievementProgress[] => {
    const unlockedIds = new Set(state.unlocked.map((u) => u.id))
    return getAllAchievementProgress(stats, unlockedIds)
  }, [state.unlocked])

  const getByCategory = useCallback((
    category: AchievementCategory,
    stats: AchievementStats
  ): AchievementProgress[] => {
    const unlockedIds = new Set(state.unlocked.map((u) => u.id))
    return ACHIEVEMENT_DEFS
      .filter((def) => def.category === category)
      .map((def) => getAchievementProgress(def, stats, unlockedIds.has(def.id)))
  }, [state.unlocked])

  const getUnlockedCount = useCallback((): number => state.unlocked.length, [state.unlocked])
  const getTotalCount = useCallback((): number => ACHIEVEMENT_DEFS.length, [])

  const setActiveTitle = useCallback((title: string | null) => {
    dispatch({ type: 'SET_ACTIVE_TITLE', title: title ?? '' })
  }, [])

  const consumeNotification = useCallback((): string | null => {
    if (state.pendingNotifications.length === 0) return null
    const next = state.pendingNotifications[0]
    dispatch({ type: 'DEQUEUE_NOTIFICATION' })
    return next
  }, [state.pendingNotifications])

  const value = useMemo<AchievementContextValue>(() => ({
    state,
    definitions: ACHIEVEMENT_DEFS,
    checkAchievements,
    getProgress,
    getAllProgress,
    getByCategory,
    getUnlockedCount,
    getTotalCount,
    setActiveTitle,
    consumeNotification,
  }), [state, checkAchievements, getProgress, getAllProgress, getByCategory, getUnlockedCount, getTotalCount, setActiveTitle, consumeNotification])

  return (
    <AchievementContext.Provider value={value}>
      {children}
    </AchievementContext.Provider>
  )
}
