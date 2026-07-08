import React, { createContext, useReducer, useEffect, useCallback, useMemo } from 'react'
import {
  RECOVERY_TICK_MS,
  STORAGE_KEY,
  POTION_PRICE,
  POTION_RECOVERY,
  CAPTURE_XP,
} from './constants'
import type { StaminaState, StaminaAction, StaminaContextValue, LevelUpResult, BuyPotionResult } from './types'
import {
  getMaxStamina,
  calculateRecovery,
  tryLevelUp,
  canConsume,
  calculateBuyPotion,
  getTodayString,
  shouldResetDailyPurchases,
  getExpProgress,
  getCaptureXp,
  migrateState,
} from './logic'

/** 从 localStorage 加载初始状态（含数据迁移 + 离线恢复 + 每日限购重置） */
function loadInitialState(): StaminaState {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as Partial<StaminaState>
      const migrated = migrateState(parsed)
      // 加载时检查每日限购是否需要重置
      if (shouldResetDailyPurchases(migrated.potionPurchaseDate)) {
        migrated.potionPurchasesToday = 0
        migrated.potionPurchaseDate = getTodayString()
      }
      // 加载时立即计算离线恢复
      const maxStamina = getMaxStamina(migrated.level)
      const oldStamina = migrated.currentStamina
      const { current } = calculateRecovery(
        migrated.lastRecoverTime,
        oldStamina,
        maxStamina
      )
      if (current > oldStamina) {
        const recoveredPoints = current - oldStamina
        const consumedTime = recoveredPoints * 360_000
        migrated.lastRecoverTime = current >= maxStamina
          ? Date.now()
          : migrated.lastRecoverTime + consumedTime
      }
      migrated.currentStamina = current
      return migrated
    }
  } catch (e) {
    console.warn('加载体力存档失败，使用默认值:', e)
  }

  // 默认初始状态
  return {
    level: 1,
    exp: 0,
    currentStamina: 120,
    totalCaptures: 0,
    lastRecoverTime: Date.now(),
    gold: 0,
    potionPurchasesToday: 0,
    potionPurchaseDate: getTodayString(),
    totalBattlesWon: 0,
    totalBattles: 0,
    currentWinStreak: 0,
    maxWinStreak: 0,
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
      if (current === state.currentStamina) {
        return state
      }
      const recoveredPoints = current - state.currentStamina
      const consumedTime = recoveredPoints * 360_000
      const newLastRecoverTime = current >= maxStamina
        ? action.now
        : state.lastRecoverTime + consumedTime

      return {
        ...state,
        currentStamina: current,
        lastRecoverTime: newLastRecoverTime,
      }
    }

    case 'CONSUME': {
      if (!canConsume(state.currentStamina, action.amount)) {
        return state
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

    case 'ADD_EXP': {
      const newExp = state.exp + action.amount
      const levelUpResult = tryLevelUp(state.level, newExp)
      if (levelUpResult.leveledUp) {
        const newMaxStamina = getMaxStamina(levelUpResult.newLevel)
        return {
          ...state,
          level: levelUpResult.newLevel,
          exp: newExp,
          currentStamina: newMaxStamina,
          gold: state.gold + levelUpResult.rewardGold,
        }
      }
      return { ...state, exp: newExp }
    }

    case 'ADD_CAPTURE': {
      const xp = action.rarity ? getCaptureXp(action.rarity) : CAPTURE_XP.common
      const newExp = state.exp + xp
      const newTotalCaptures = state.totalCaptures + action.count
      const levelUpResult = tryLevelUp(state.level, newExp)
      if (levelUpResult.leveledUp) {
        const newMaxStamina = getMaxStamina(levelUpResult.newLevel)
        return {
          ...state,
          level: levelUpResult.newLevel,
          exp: newExp,
          totalCaptures: newTotalCaptures,
          currentStamina: newMaxStamina,
          gold: state.gold + levelUpResult.rewardGold,
        }
      }
      return {
        ...state,
        exp: newExp,
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

    case 'RECORD_BATTLE': {
      const won = action.result === 'win'
      const newWinStreak = won ? state.currentWinStreak + 1 : 0
      return {
        ...state,
        totalBattles: state.totalBattles + 1,
        totalBattlesWon: state.totalBattlesWon + (won ? 1 : 0),
        currentWinStreak: newWinStreak,
        maxWinStreak: Math.max(state.maxWinStreak, newWinStreak),
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

  // 每日限购重置检查
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

  const { currentLevelExp, nextLevelExp, progress: expProgress } = useMemo(
    () => getExpProgress(state.level, state.exp),
    [state.level, state.exp]
  )

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

  const addCapture = useCallback((count: number, rarity?: import('../types').RarityTier): LevelUpResult => {
    const xp = rarity ? getCaptureXp(rarity) : CAPTURE_XP.common
    const result = tryLevelUp(state.level, state.exp + xp)
    dispatch({ type: 'ADD_CAPTURE', count, rarity })
    return result
  }, [state.level, state.exp])

  const addExp = useCallback((amount: number): LevelUpResult => {
    const result = tryLevelUp(state.level, state.exp + amount)
    dispatch({ type: 'ADD_EXP', amount })
    return result
  }, [state.level, state.exp])

  const recordBattle = useCallback((result: 'win' | 'lose' | 'draw') => {
    dispatch({ type: 'RECORD_BATTLE', result })
  }, [])

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
    nextLevelExp,
    currentLevelExp,
    expProgress,
    nextRecoverIn,
    consumeStamina,
    addStamina,
    addCapture,
    addExp,
    recordBattle,
    addGold,
    buyStaminaPotion,
  }), [state, maxStamina, nextLevelExp, currentLevelExp, expProgress, nextRecoverIn, consumeStamina, addStamina, addCapture, addExp, recordBattle, addGold, buyStaminaPotion])

  return (
    <StaminaContext.Provider value={value}>
      {children}
    </StaminaContext.Provider>
  )
}
