import React, { createContext, useReducer, useEffect, useCallback, useMemo } from 'react'
import { useWeather } from '../weather/useWeather'
import { useShop } from '../shop/useShop'
import {
  STATUS_STORAGE_KEY,
  RECOVERY_CHECK_INTERVAL_MS,
} from './constants'
import type {
  StatusState,
  StatusAction,
  StatusContextValue,
  StatusType,
  CureColdResult,
  StatusDisplay,
  StatusEffect,
  StatusSource,
  PetStatusRecord,
} from './types'
import {
  applyColdToRecord,
  cureColdFromRecord,
  checkRecovery,
  clearExpiredEffects,
  getStatMultiplier,
  getColdRemainingDays,
  getStatusDisplay,
  isExpired,
} from './logic'

/** 默认初始状态 */
const initialState: StatusState = {
  records: {},
}

/** 从 localStorage 加载状态 */
function loadInitialState(): StatusState {
  try {
    const saved = localStorage.getItem(STATUS_STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as StatusState
      if (typeof parsed.records !== 'object' || parsed.records === null) {
        throw new Error('状态存档字段校验失败')
      }
      return parsed
    }
  } catch (e) {
    console.warn('加载状态存档失败，使用默认值:', e)
  }
  return initialState
}

/** Reducer */
function statusReducer(state: StatusState, action: StatusAction): StatusState {
  switch (action.type) {
    case 'APPLY_COLD': {
      const existing = state.records[action.petId] ?? null
      const { record, added } = applyColdToRecord(existing, action.source, action.now)
      if (!added) return state
      return {
        ...state,
        records: {
          ...state.records,
          [action.petId]: { ...record, petId: action.petId },
        },
      }
    }

    case 'CURE_COLD': {
      const existing = state.records[action.petId]
      if (!existing) return state
      const { record, cured } = cureColdFromRecord(existing)
      if (!cured || !record) return state
      return {
        ...state,
        records: {
          ...state.records,
          [action.petId]: record,
        },
      }
    }

    case 'APPLY_PLEASURE': {
      // 愉悦不持久化，仅作为 UI 派生状态
      // 此 action 为 no-op，愉悦由 getStatusDisplay 从天气派生
      return state
    }

    case 'REMOVE_PLEASURE': {
      return state
    }

    case 'CHECK_RECOVERY': {
      const newRecords: Record<string, PetStatusRecord> = {}
      let changed = false

      for (const [petId, record] of Object.entries(state.records)) {
        // 先清理非感冒过期效果
        const cleaned = clearExpiredEffects(record, action.now)
        // 再检查感冒恢复
        const result = checkRecovery(cleaned, action.now)
        if (result.expired || cleaned !== record) {
          changed = true
        }
        // 仅保留有活跃状态或有永久损伤的记录
        if (result.record.effects.length > 0 || result.record.permanentDamageMultiplier < 1.0) {
          newRecords[petId] = result.record
        }
      }

      return changed ? { records: newRecords } : state
    }

    case 'CLEAR_EXPIRED': {
      const newRecords: Record<string, PetStatusRecord> = {}
      let changed = false

      for (const [petId, record] of Object.entries(state.records)) {
        const cleaned = clearExpiredEffects(record, action.now)
        if (cleaned.effects.length !== record.effects.length) {
          changed = true
        }
        if (cleaned.effects.length > 0 || cleaned.permanentDamageMultiplier < 1.0) {
          newRecords[petId] = cleaned
        }
      }

      return changed ? { records: newRecords } : state
    }

    case 'CLEAR_PET': {
      if (!state.records[action.petId]) return state
      const newRecords = { ...state.records }
      delete newRecords[action.petId]
      return { records: newRecords }
    }

    case 'LOAD_STATE': {
      return action.state
    }

    default:
      return state
  }
}

export const StatusContext = createContext<StatusContextValue | null>(null)

export const StatusProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(statusReducer, undefined, loadInitialState)
  const weather = useWeather()
  const shop = useShop()

  // 当前天气（用于派生愉悦标签）
  const currentWeather = weather.getBattleModifier()

  // localStorage 持久化
  useEffect(() => {
    localStorage.setItem(STATUS_STORAGE_KEY, JSON.stringify(state))
  }, [state])

  // 首次加载时检查恢复（处理离线期间过期的感冒）
  useEffect(() => {
    dispatch({ type: 'CHECK_RECOVERY', now: Date.now() })
  }, [])

  // 定时恢复检查（每小时）
  useEffect(() => {
    const interval = setInterval(() => {
      dispatch({ type: 'CHECK_RECOVERY', now: Date.now() })
    }, RECOVERY_CHECK_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [])

  // 页面可见性恢复时检查
  useEffect(() => {
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        dispatch({ type: 'CHECK_RECOVERY', now: Date.now() })
      }
    }
    document.addEventListener('visibilitychange', handleVisibility)
    return () => document.removeEventListener('visibilitychange', handleVisibility)
  }, [])

  // ---- 查询函数 ----

  const getPetEffects = useCallback((petId: string): StatusEffect[] => {
    const record = state.records[petId]
    if (!record) return []
    return record.effects.filter(e => !isExpired(e))
  }, [state.records])

  const getPetStatusDisplay = useCallback((petId: string): StatusDisplay[] => {
    const record = state.records[petId] ?? null
    return getStatusDisplay(record, currentWeather)
  }, [state.records, currentWeather])

  const hasStatus = useCallback((petId: string, type: StatusType): boolean => {
    const effects = getPetEffects(petId)
    if (effects.some(e => e.type === type)) return true
    // 愉悦从天气派生
    if (type === 'pleasure' && currentWeather === 'sunny') return true
    return false
  }, [getPetEffects, currentWeather])

  const getStatModifier = useCallback((petId: string): number => {
    return getStatMultiplier(state.records[petId])
  }, [state.records])

  const getPermanentDamage = useCallback((petId: string): number => {
    return state.records[petId]?.permanentDamageMultiplier ?? 1.0
  }, [state.records])

  const getColdRemainingDaysFn = useCallback((petId: string): number | null => {
    return getColdRemainingDays(state.records[petId])
  }, [state.records])

  // ---- 操作函数 ----

  const applyCold = useCallback((petId: string, source: StatusSource) => {
    dispatch({ type: 'APPLY_COLD', petId, source, now: Date.now() })
  }, [])

  const cureCold = useCallback((petId: string): CureColdResult => {
    // 检查是否有感冒
    const record = state.records[petId]
    const hasCold = record?.effects.some(e => e.type === 'cold' && !isExpired(e))
    if (!hasCold) {
      return { success: false, reason: 'no_cold' }
    }

    // 检查是否有感冒药
    const medicineCount = shop.getItemCount('cold_medicine')
    if (medicineCount <= 0) {
      return { success: false, reason: 'no_medicine' }
    }

    // 使用感冒药
    const useResult = shop.useItem('cold_medicine')
    if (!useResult.success) {
      return { success: false, reason: 'no_medicine' }
    }

    // 治愈感冒
    dispatch({ type: 'CURE_COLD', petId })
    return { success: true }
  }, [state.records, shop])

  const applyPleasure = useCallback((_petId: string) => {
    // 愉悦不持久化，no-op
  }, [])

  const removePleasure = useCallback((_petId: string) => {
    // 愉悦不持久化，no-op
  }, [])

  const checkRecoveryFn = useCallback(() => {
    dispatch({ type: 'CHECK_RECOVERY', now: Date.now() })
  }, [])

  const value = useMemo<StatusContextValue>(() => ({
    state,
    getPetEffects,
    getPetStatusDisplay,
    hasStatus,
    getStatModifier,
    getPermanentDamage,
    getColdRemainingDays: getColdRemainingDaysFn,
    applyCold,
    cureCold,
    applyPleasure,
    removePleasure,
    checkRecovery: checkRecoveryFn,
  }), [
    state, getPetEffects, getPetStatusDisplay, hasStatus,
    getStatModifier, getPermanentDamage, getColdRemainingDaysFn,
    applyCold, cureCold, applyPleasure, removePleasure, checkRecoveryFn,
  ])

  return (
    <StatusContext.Provider value={value}>
      {children}
    </StatusContext.Provider>
  )
}
