import React, { createContext, useReducer, useEffect, useCallback, useMemo } from 'react'
import {
  ECONOMY_STORAGE_KEY,
  MAX_LOG_ENTRIES,
} from './constants'
import type {
  EconomyState, EconomyAction, EconomyContextValue,
  EconomyStats, BalanceCheckResult, EconomyLogEntry,
  GoldSource, GoldSink,
} from './types'
import {
  getTodayString, shouldResetDaily,
  balanceCheck, getEconomyStats,
} from './logic'

/** 默认初始状态 */
const initialState: EconomyState = {
  totalEarned: 0,
  totalSpent: 0,
  logs: [],
  nextLogId: 1,
  todayEarned: 0,
  todaySpent: 0,
  todayDate: getTodayString(),
}

/** 从 localStorage 加载状态 */
function loadInitialState(): EconomyState {
  try {
    const saved = localStorage.getItem(ECONOMY_STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as EconomyState
      // 基本字段校验
      if (
        typeof parsed.totalEarned !== 'number' ||
        typeof parsed.totalSpent !== 'number' ||
        !Array.isArray(parsed.logs) ||
        typeof parsed.nextLogId !== 'number' ||
        typeof parsed.todayDate !== 'string'
      ) {
        throw new Error('经济存档字段校验失败')
      }
      // 加载时检查每日重置
      if (shouldResetDaily(parsed.todayDate)) {
        parsed.todayEarned = 0
        parsed.todaySpent = 0
        parsed.todayDate = getTodayString()
      }
      return parsed
    }
  } catch (e) {
    console.warn('加载经济存档失败，使用默认值:', e)
  }
  return initialState
}

/** Reducer */
function economyReducer(state: EconomyState, action: EconomyAction): EconomyState {
  switch (action.type) {
    case 'TRACK_EARN': {
      const entry: EconomyLogEntry = {
        id: state.nextLogId,
        type: 'earn',
        amount: action.amount,
        category: action.source,
        timestamp: action.now,
        note: action.note,
      }
      // FIFO 淘汰：超过 MAX_LOG_ENTRIES 时移除最旧的
      const logs = [...state.logs, entry]
      if (logs.length > MAX_LOG_ENTRIES) {
        logs.splice(0, logs.length - MAX_LOG_ENTRIES)
      }
      return {
        ...state,
        totalEarned: state.totalEarned + action.amount,
        todayEarned: state.todayEarned + action.amount,
        logs,
        nextLogId: state.nextLogId + 1,
      }
    }

    case 'TRACK_SPEND': {
      const entry: EconomyLogEntry = {
        id: state.nextLogId,
        type: 'spend',
        amount: action.amount,
        category: action.sink,
        timestamp: action.now,
        note: action.note,
      }
      const logs = [...state.logs, entry]
      if (logs.length > MAX_LOG_ENTRIES) {
        logs.splice(0, logs.length - MAX_LOG_ENTRIES)
      }
      return {
        ...state,
        totalSpent: state.totalSpent + action.amount,
        todaySpent: state.todaySpent + action.amount,
        logs,
        nextLogId: state.nextLogId + 1,
      }
    }

    case 'RESET_DAILY': {
      return {
        ...state,
        todayEarned: 0,
        todaySpent: 0,
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

export const EconomyContext = createContext<EconomyContextValue | null>(null)

export const EconomyProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(economyReducer, undefined, loadInitialState)

  // localStorage 持久化
  useEffect(() => {
    localStorage.setItem(ECONOMY_STORAGE_KEY, JSON.stringify(state))
  }, [state])

  // 每日重置检查
  useEffect(() => {
    if (shouldResetDaily(state.todayDate)) {
      dispatch({ type: 'RESET_DAILY', date: getTodayString() })
    }
  }, [state.todayDate])

  // ---- 操作函数 ----

  const trackEarn = useCallback((amount: number, source: GoldSource, note?: string) => {
    if (amount <= 0) return
    dispatch({ type: 'TRACK_EARN', amount, source, note, now: Date.now() })
  }, [])

  const trackSpend = useCallback((amount: number, sink: GoldSink, note?: string) => {
    if (amount <= 0) return
    dispatch({ type: 'TRACK_SPEND', amount, sink, note, now: Date.now() })
  }, [])

  // ---- 查询函数 ----

  const getStats = useCallback((currentGold: number): EconomyStats => {
    return getEconomyStats(state, currentGold)
  }, [state])

  const getBalanceCheck = useCallback((): BalanceCheckResult => {
    return balanceCheck(state)
  }, [state])

  const getRecentLogs = useCallback((count: number): EconomyLogEntry[] => {
    return [...state.logs].reverse().slice(0, count)
  }, [state.logs])

  const value = useMemo<EconomyContextValue>(() => ({
    state,
    getStats,
    getBalanceCheck,
    trackEarn,
    trackSpend,
    getRecentLogs,
  }), [state, getStats, getBalanceCheck, trackEarn, trackSpend, getRecentLogs])

  return (
    <EconomyContext.Provider value={value}>
      {children}
    </EconomyContext.Provider>
  )
}
