import React, { createContext, useReducer, useEffect, useCallback, useMemo } from 'react'
import {
  RECOVERY_TICK_MS,
  STORAGE_KEY,
  POTION_PRICE,
  POTION_RECOVERY,
} from './constants'
import type { StaminaState, StaminaAction, StaminaContextValue, LevelUpResult, BuyPotionResult } from './types'
import { getMaxStamina, calculateRecovery, tryLevelUp, canConsume, calculateBuyPotion, getTodayString, shouldResetDailyPurchases } from './logic'

/** 从 localStorage 加载初始状态（含离线恢复 + 每日限购重置） */
function loadInitialState(): StaminaState {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as StaminaState
      // 基本字段校验
      if (
        typeof parsed.level !== 'number' ||
        typeof parsed.currentStamina !== 'number' ||
        typeof parsed.totalCaptures !== 'number' ||
        typeof parsed.lastRecoverTime !== 'number' ||
        typeof parsed.gold !== 'number' ||
        typeof parsed.potionPurchasesToday !== 'number' ||
        typeof parsed.potionPurchaseDate !== 'string'
      ) {
        throw new Error('存档字段校验失败')
      }
      // 加载时检查每日限购是否需要重置
      if (shouldResetDailyPurchases(parsed.potionPurchaseDate)) {
        parsed.potionPurchasesToday = 0
        parsed.potionPurchaseDate = getTodayString()
      }
      // 加载时立即计算离线恢复
      const maxStamina = getMaxStamina(parsed.level)
      const oldStamina = parsed.currentStamina
      const { current } = calculateRecovery(
        parsed.lastRecoverTime,
        oldStamina,
        maxStamina
      )
      // 离线恢复后必须更新 lastRecoverTime，否则下次 TICK 会重复恢复
      if (current > oldStamina) {
        const recoveredPoints = current - oldStamina
        const consumedTime = recoveredPoints * 360_000
        parsed.lastRecoverTime = current >= maxStamina
          ? Date.now()
          : parsed.lastRecoverTime + consumedTime
      }
      parsed.currentStamina = current
      return parsed
    }
  } catch (e) {
    console.warn('加载体力存档失败，使用默认值:', e)
  }

  // 默认初始状态：Lv.1，满体力，0 捕获，0 金币
  return {
    level: 1,
    currentStamina: 120,
    totalCaptures: 0,
    lastRecoverTime: Date.now(),
    gold: 0,
    potionPurchasesToday: 0,
    potionPurchaseDate: getTodayString(),
  }
}

/** Reducer */
function staminaReducer(state: StaminaState, action: StaminaAction): StaminaState {
  switch (action.type) {
    case 'TICK_RECOVERY': {
      const maxStamina = getMaxStamina(state.level)
      const { current } = calculateRecovery(
        state.lastRecoverTime,
        state.currentStamina,
        maxStamina,
        action.now
      )
      // 体力没变化时不产生新状态
      if (current === state.currentStamina) {
        return state
      }
      // 计算新的 lastRecoverTime
      const recoveredPoints = current - state.currentStamina
      const consumedTime = recoveredPoints * 360_000
      const newLastRecoverTime = current >= maxStamina
        ? action.now // 满了，重置为当前时间
        : state.lastRecoverTime + consumedTime

      return {
        ...state,
        currentStamina: current,
        lastRecoverTime: newLastRecoverTime,
      }
    }

    case 'CONSUME': {
      if (!canConsume(state.currentStamina, action.amount)) {
        return state // 不扣
      }
      return {
        ...state,
        currentStamina: state.currentStamina - action.amount,
      }
    }

    case 'ADD_STAMINA': {
      const maxStamina = getMaxStamina(state.level)
      return {
        ...state,
        currentStamina: Math.min(state.currentStamina + action.amount, maxStamina),
      }
    }

    case 'ADD_CAPTURE': {
      const newTotalCaptures = state.totalCaptures + action.count
      const levelUpResult = tryLevelUp(state.level, newTotalCaptures)
      if (levelUpResult.leveledUp) {
        // 升级：恢复满体力 + 增加金币奖励
        const newMaxStamina = getMaxStamina(levelUpResult.newLevel)
        return {
          ...state,
          level: levelUpResult.newLevel,
          totalCaptures: newTotalCaptures,
          currentStamina: newMaxStamina,
          gold: state.gold + levelUpResult.rewardGold,
        }
      }
      return {
        ...state,
        totalCaptures: newTotalCaptures,
      }
    }

    case 'ADD_GOLD': {
      return {
        ...state,
        gold: state.gold + action.amount,
      }
    }

    case 'BUY_POTION': {
      const result = calculateBuyPotion(state.gold, state.potionPurchasesToday)
      if (!result.success) {
        return state
      }
      const maxStamina = getMaxStamina(state.level)
      return {
        ...state,
        gold: state.gold - POTION_PRICE,
        currentStamina: Math.min(state.currentStamina + POTION_RECOVERY, maxStamina),
        potionPurchasesToday: state.potionPurchasesToday + 1,
      }
    }

    case 'RESET_DAILY_PURCHASES': {
      return {
        ...state,
        potionPurchasesToday: 0,
        potionPurchaseDate: action.date,
      }
    }

    case 'LOAD_STATE': {
      return action.state
    }

    default:
      return state
  }
}

export const StaminaContext = createContext<StaminaContextValue | null>(null)

export const StaminaProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(staminaReducer, undefined, loadInitialState)

  // 每分钟触发 TICK_RECOVERY
  useEffect(() => {
    const interval = setInterval(() => {
      dispatch({ type: 'TICK_RECOVERY', now: Date.now() })
    }, RECOVERY_TICK_MS)
    return () => clearInterval(interval)
  }, [])

  // 页面从后台切回前台时立即触发一次恢复检查
  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        dispatch({ type: 'TICK_RECOVERY', now: Date.now() })
      }
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange)
  }, [])

  // 每日限购重置检查：每次 state 变化时检查日期
  useEffect(() => {
    if (shouldResetDailyPurchases(state.potionPurchaseDate)) {
      dispatch({ type: 'RESET_DAILY_PURCHASES', date: getTodayString() })
    }
  }, [state.potionPurchaseDate])

  // localStorage 持久化
  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  }, [state])

  const maxStamina = useMemo(() => getMaxStamina(state.level), [state.level])

  // 计算 nextRecoverIn
  const nextRecoverIn = useMemo(() => {
    if (state.currentStamina >= maxStamina) return 0
    const { recoverTime } = calculateRecovery(
      state.lastRecoverTime,
      state.currentStamina,
      maxStamina,
      Date.now()
    )
    return recoverTime
  }, [state.lastRecoverTime, state.currentStamina, maxStamina])

  const consumeStamina = useCallback((amount: number): boolean => {
    if (!canConsume(state.currentStamina, amount)) {
      return false
    }
    dispatch({ type: 'CONSUME', amount })
    return true
  }, [state.currentStamina])

  const addStamina = useCallback((amount: number) => {
    dispatch({ type: 'ADD_STAMINA', amount })
  }, [])

  const addCapture = useCallback((count: number): LevelUpResult => {
    const result = tryLevelUp(state.level, state.totalCaptures + count)
    dispatch({ type: 'ADD_CAPTURE', count })
    return result
  }, [state.level, state.totalCaptures])

  const addGold = useCallback((amount: number) => {
    dispatch({ type: 'ADD_GOLD', amount })
  }, [])

  const buyStaminaPotion = useCallback((): BuyPotionResult => {
    const result = calculateBuyPotion(state.gold, state.potionPurchasesToday)
    if (result.success) {
      dispatch({ type: 'BUY_POTION' })
    }
    return result
  }, [state.gold, state.potionPurchasesToday])

  const value = useMemo<StaminaContextValue>(() => ({
    state,
    maxStamina,
    nextRecoverIn,
    consumeStamina,
    addStamina,
    addCapture,
    addGold,
    buyStaminaPotion,
  }), [state, maxStamina, nextRecoverIn, consumeStamina, addStamina, addCapture, addGold, buyStaminaPotion])

  return (
    <StaminaContext.Provider value={value}>
      {children}
    </StaminaContext.Provider>
  )
}
