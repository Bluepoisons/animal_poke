import React, { createContext, useReducer, useEffect, useCallback, useMemo } from 'react'
import { useStamina } from '../stamina/useStamina'
import { useEconomy } from './useEconomy'
import { useShop } from '../shop/useShop'
import { useLbs } from '../lbs/useLbs'
import {
  DISPATCH_STORAGE_KEY,
  DISPATCH_CHECK_INTERVAL_MS,
  DISPATCH_MISSION_DEFS,
} from './constants'
import type {
  DispatchState, DispatchAction, DispatchContextValue,
  DispatchResult, CollectResult,
  DispatchMissionType, DispatchMission,
} from './types'
import type { RarityTier } from '../types'
import {
  getTodayString, shouldResetDaily,
  createMission, isMissionCompleted,
  getAvailableSlots, getPetMission,
  getMissionCountdown, getSpeedUpCost,
} from './logic'

/** 默认初始状态 */
const initialState: DispatchState = {
  missions: [],
  todayDispatchCount: 0,
  todayDate: getTodayString(),
}

/** 从 localStorage 加载状态 */
function loadInitialState(): DispatchState {
  try {
    const saved = localStorage.getItem(DISPATCH_STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as DispatchState
      if (
        !Array.isArray(parsed.missions) ||
        typeof parsed.todayDispatchCount !== 'number' ||
        typeof parsed.todayDate !== 'string'
      ) {
        throw new Error('派遣存档字段校验失败')
      }
      if (shouldResetDaily(parsed.todayDate)) {
        parsed.todayDispatchCount = 0
        parsed.todayDate = getTodayString()
      }
      return parsed
    }
  } catch (e) {
    console.warn('加载派遣存档失败，使用默认值:', e)
  }
  return initialState
}

/** Reducer */
function dispatchReducer(state: DispatchState, action: DispatchAction): DispatchState {
  switch (action.type) {
    case 'START_MISSION': {
      return {
        ...state,
        missions: [...state.missions, action.mission],
        todayDispatchCount: state.todayDispatchCount + 1,
      }
    }

    case 'COMPLETE_MISSION': {
      return {
        ...state,
        missions: state.missions.map(m =>
          m.id === action.missionId ? { ...m, status: 'completed' as const } : m
        ),
      }
    }

    case 'COLLECT_MISSION': {
      return {
        ...state,
        missions: state.missions.map(m =>
          m.id === action.missionId ? { ...m, status: 'collected' as const } : m
        ),
      }
    }

    case 'SPEED_UP_MISSION': {
      return {
        ...state,
        missions: state.missions.map(m =>
          m.id === action.missionId
            ? { ...m, status: 'completed' as const, endTime: action.now }
            : m
        ),
      }
    }

    case 'RESET_DAILY': {
      return {
        ...state,
        todayDispatchCount: 0,
        todayDate: action.date,
      }
    }

    case 'LOAD_STATE': {
      return action.state
    }

    default:
      return state
  }
}

export const DispatchContext = createContext<DispatchContextValue | null>(null)

export const DispatchProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const stamina = useStamina()
  const economy = useEconomy()
  const shop = useShop()
  const lbs = useLbs()

  const [state, dispatch] = useReducer(dispatchReducer, undefined, loadInitialState)

  // localStorage 持久化
  useEffect(() => {
    localStorage.setItem(DISPATCH_STORAGE_KEY, JSON.stringify(state))
  }, [state])

  // 每日重置检查
  useEffect(() => {
    if (shouldResetDaily(state.todayDate)) {
      dispatch({ type: 'RESET_DAILY', date: getTodayString() })
    }
  }, [state.todayDate])

  // 定时检查已完成任务
  useEffect(() => {
    const interval = setInterval(() => {
      const now = Date.now()
      for (const mission of state.missions) {
        if (isMissionCompleted(mission, now)) {
          dispatch({ type: 'COMPLETE_MISSION', missionId: mission.id, now })
        }
      }
    }, DISPATCH_CHECK_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [state.missions])

  // 页面可见性恢复时检查
  useEffect(() => {
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        const now = Date.now()
        for (const mission of state.missions) {
          if (isMissionCompleted(mission, now)) {
            dispatch({ type: 'COMPLETE_MISSION', missionId: mission.id, now })
          }
        }
      }
    }
    document.addEventListener('visibilitychange', handleVisibility)
    return () => document.removeEventListener('visibilitychange', handleVisibility)
  }, [state.missions])

  // 首次加载时检查（处理离线期间完成的任务）
  useEffect(() => {
    const now = Date.now()
    for (const mission of state.missions) {
      if (isMissionCompleted(mission, now)) {
        dispatch({ type: 'COMPLETE_MISSION', missionId: mission.id, now })
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []) // 仅首次加载

  // ---- 派生值 ----

  const availableSlots = useMemo(
    () => getAvailableSlots(state, stamina.state.level),
    [state.missions, stamina.state.level]
  )

  // ---- 操作函数 ----

  const startMission = useCallback(
    (missionType: DispatchMissionType, petId: string, petRarity: RarityTier): DispatchResult => {
      // 检查槽位
      if (availableSlots <= 0) {
        return { success: false, reason: 'no_slots' }
      }

      // 检查宠物是否正在派遣
      const existing = getPetMission(state, petId)
      if (existing) {
        return { success: false, reason: 'pet_busy' }
      }

      // 检查体力
      const def = DISPATCH_MISSION_DEFS.find(d => d.type === missionType)!
      if (stamina.state.currentStamina < def.staminaCost) {
        return { success: false, reason: 'insufficient_stamina' }
      }

      // 扣体力
      stamina.consumeStamina(def.staminaCost)

      // 创建任务
      const now = Date.now()
      const city = lbs.state.cityName || '未知'
      const mission = createMission(missionType, petId, petRarity, city, now)

      dispatch({ type: 'START_MISSION', mission, now })

      return { success: true, mission }
    },
    [availableSlots, state, stamina, lbs.state.cityName]
  )

  const collectMission = useCallback((missionId: string): CollectResult => {
    const mission = state.missions.find(m => m.id === missionId)
    if (!mission) {
      return { success: false, reason: 'not_found' }
    }
    if (mission.status !== 'completed') {
      return { success: false, reason: 'not_completed' }
    }

    // 发放奖励
    stamina.addGold(mission.rewards.gold)
    economy.trackEarn(mission.rewards.gold, 'dispatch', `派遣: ${mission.type}`)

    // 道具掉落
    if (mission.rewards.droppedItem) {
      shop.addItem(mission.rewards.droppedItem as any)
    }

    dispatch({ type: 'COLLECT_MISSION', missionId })

    return { success: true, rewards: mission.rewards }
  }, [state.missions, stamina, economy, shop])

  const speedUpMission = useCallback((missionId: string): {
    success: boolean
    reason?: 'insufficient_gold' | 'not_found' | 'already_completed'
  } => {
    const mission = state.missions.find(m => m.id === missionId)
    if (!mission) {
      return { success: false, reason: 'not_found' }
    }
    if (mission.status !== 'active') {
      return { success: false, reason: 'already_completed' }
    }

    const cost = getSpeedUpCost(mission)
    if (stamina.state.gold < cost) {
      return { success: false, reason: 'insufficient_gold' }
    }

    // 扣金币
    stamina.addGold(-cost)
    economy.trackSpend(cost, 'dispatch_speedup', `加速: ${mission.type}`)

    // 立即完成
    dispatch({ type: 'SPEED_UP_MISSION', missionId, now: Date.now() })

    return { success: true }
  }, [state.missions, stamina, economy])

  const checkCompleted = useCallback(() => {
    const now = Date.now()
    for (const mission of state.missions) {
      if (isMissionCompleted(mission, now)) {
        dispatch({ type: 'COMPLETE_MISSION', missionId: mission.id, now })
      }
    }
  }, [state.missions])

  const getPetMissionFn = useCallback((petId: string): DispatchMission | null => {
    return getPetMission(state, petId)
  }, [state])

  const getMissionCountdownFn = useCallback((missionId: string): number => {
    const mission = state.missions.find(m => m.id === missionId)
    if (!mission) return 0
    return getMissionCountdown(mission)
  }, [state.missions])

  const value = useMemo<DispatchContextValue>(() => ({
    state,
    availableSlots,
    missionDefs: DISPATCH_MISSION_DEFS,
    startMission,
    collectMission,
    speedUpMission,
    checkCompleted,
    getPetMission: getPetMissionFn,
    getMissionCountdown: getMissionCountdownFn,
  }), [
    state, availableSlots, startMission, collectMission, speedUpMission,
    checkCompleted, getPetMissionFn, getMissionCountdownFn,
  ])

  return (
    <DispatchContext.Provider value={value}>
      {children}
    </DispatchContext.Provider>
  )
}
