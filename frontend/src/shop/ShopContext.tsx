import React, { createContext, useReducer, useEffect, useCallback, useRef, useMemo } from 'react'
import { useStamina } from '../stamina/useStamina'
import {
  ITEM_DEFS,
  SHOP_STORAGE_KEY,
  POTION_RECOVERY,
} from './constants'
import type { ItemId } from './constants'
import type {
  ShopState,
  ShopAction,
  ShopContextValue,
  BuyResult,
  UseItemResult,
  CheckInResult,
} from './types'
import {
  buyItem as calculateBuy,
  getCheckInReward,
  calculateCaptureBoost,
  getTodayString,
  shouldResetDailyPurchases,
} from './logic'

/** 默认初始状态 */
const initialState: ShopState = {
  inventory: {},
  checkIn: {
    streak: 0,
    lastCheckInDate: '',
  },
  dailyPurchases: {},
  dailyPurchaseDate: getTodayString(),
}

/** 从 localStorage 加载状态 */
function loadInitialState(): ShopState {
  try {
    const saved = localStorage.getItem(SHOP_STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as ShopState
      // 基本字段校验
      if (
        typeof parsed.inventory !== 'object' ||
        typeof parsed.checkIn !== 'object' ||
        typeof parsed.dailyPurchases !== 'object' ||
        typeof parsed.dailyPurchaseDate !== 'string'
      ) {
        throw new Error('商店存档字段校验失败')
      }
      // 加载时检查每日限购是否需要重置
      if (shouldResetDailyPurchases(parsed.dailyPurchaseDate)) {
        parsed.dailyPurchases = {}
        parsed.dailyPurchaseDate = getTodayString()
      }
      return parsed
    }
  } catch (e) {
    console.warn('加载商店存档失败，使用默认值:', e)
  }
  return initialState
}

/** Reducer */
function shopReducer(state: ShopState, action: ShopAction): ShopState {
  switch (action.type) {
    case 'BUY_ITEM': {
      const itemId = action.itemId
      const def = ITEM_DEFS[itemId]
      const currentCount = state.inventory[itemId] ?? 0
      const newInventory = { ...state.inventory, [itemId]: currentCount + 1 }

      // 限购道具记录购买次数
      let newDailyPurchases = state.dailyPurchases
      if (def.dailyLimit > 0) {
        const currentDaily = state.dailyPurchases[itemId] ?? 0
        newDailyPurchases = { ...state.dailyPurchases, [itemId]: currentDaily + 1 }
      }

      return {
        ...state,
        inventory: newInventory,
        dailyPurchases: newDailyPurchases,
      }
    }

    case 'USE_ITEM': {
      const itemId = action.itemId
      const currentCount = state.inventory[itemId] ?? 0
      if (currentCount <= 0) return state

      const newCount = currentCount - 1
      const newInventory = { ...state.inventory }
      if (newCount <= 0) {
        delete newInventory[itemId]
      } else {
        newInventory[itemId] = newCount
      }

      return {
        ...state,
        inventory: newInventory,
      }
    }

    case 'ADD_ITEM': {
      const itemId = action.itemId
      const currentCount = state.inventory[itemId] ?? 0
      return {
        ...state,
        inventory: { ...state.inventory, [itemId]: currentCount + action.count },
      }
    }

    case 'CHECK_IN': {
      return {
        ...state,
        checkIn: {
          streak: action.newStreak,
          lastCheckInDate: getTodayString(),
        },
        // 第 7 天额外送道具
        inventory: action.rewardItem
          ? {
              ...state.inventory,
              [action.rewardItem]: (state.inventory[action.rewardItem] ?? 0) + 1,
            }
          : state.inventory,
      }
    }

    case 'RESET_DAILY_PURCHASES': {
      return {
        ...state,
        dailyPurchases: {},
        dailyPurchaseDate: action.date,
      }
    }

    case 'LOAD_STATE': {
      return action.state
    }

    default:
      return state
  }
}

export const ShopContext = createContext<ShopContextValue | null>(null)

export const ShopProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const stamina = useStamina()
  const [state, dispatch] = useReducer(shopReducer, undefined, loadInitialState)

  // 玩具球激活状态（useRef 管理，不持久化，刷新后失效）
  const activeCaptureBoostRef = useRef<ItemId | null>(null)

  // localStorage 持久化
  useEffect(() => {
    localStorage.setItem(SHOP_STORAGE_KEY, JSON.stringify(state))
  }, [state])

  // 每日限购重置检查
  useEffect(() => {
    if (shouldResetDailyPurchases(state.dailyPurchaseDate)) {
      dispatch({ type: 'RESET_DAILY_PURCHASES', date: getTodayString() })
    }
  }, [state.dailyPurchaseDate])

  const buyItem = useCallback((itemId: ItemId): BuyResult => {
    const def = ITEM_DEFS[itemId]

    // 体力药剂：委托给 StaminaContext.buyStaminaPotion（已有完整购买逻辑）
    if (itemId === 'stamina_potion') {
      const result = stamina.buyStaminaPotion()
      if (result.success) {
        // 购买成功，加到背包（虽然效果是即时的，但保留记录用于背包展示）
        dispatch({ type: 'BUY_ITEM', itemId })
        return {
          success: true,
          remainingGold: stamina.state.gold - def.price,
          remainingDailyPurchases: result.remainingPurchases,
        }
      }
      return {
        success: false,
        reason: result.reason,
        remainingGold: stamina.state.gold,
        remainingDailyPurchases: result.remainingPurchases,
      }
    }

    // 其他道具：走 ShopContext 统一购买流程
    const result = calculateBuy(
      stamina.state.gold,
      def.price,
      state.dailyPurchases[itemId] ?? 0,
      def.dailyLimit
    )

    if (result.success) {
      // 扣金币（走 StaminaContext）
      stamina.addGold(-def.price)
      // 加道具到背包 + 记录限购
      dispatch({ type: 'BUY_ITEM', itemId })
    }

    return result
  }, [stamina, state.dailyPurchases])

  const useItem = useCallback((itemId: ItemId): UseItemResult => {
    const count = state.inventory[itemId] ?? 0
    if (count <= 0) {
      return { success: false, reason: 'not_in_inventory' }
    }

    dispatch({ type: 'USE_ITEM', itemId })

    // 玩具球类道具：激活捕获增益
    if (itemId === 'toy_ball' || itemId === 'premium_toy_ball') {
      activeCaptureBoostRef.current = itemId
    }

    // 体力药剂：使用时恢复体力
    if (itemId === 'stamina_potion') {
      stamina.addStamina(POTION_RECOVERY)
    }

    return { success: true }
  }, [state.inventory, stamina])

  const checkIn = useCallback((): CheckInResult => {
    const result = getCheckInReward(
      state.checkIn.streak,
      state.checkIn.lastCheckInDate
    )

    if (result.success) {
      // 发放金币奖励
      stamina.addGold(result.reward)
      // 更新签到状态（含第 7 天额外道具）
      dispatch({
        type: 'CHECK_IN',
        reward: result.reward,
        newStreak: result.newStreak,
        rewardItem: result.rewardItem,
      })
    }

    return result
  }, [stamina, state.checkIn])

  const getItemCount = useCallback((itemId: ItemId): number => {
    return state.inventory[itemId] ?? 0
  }, [state.inventory])

  const getDailyPurchaseCount = useCallback((itemId: ItemId): number => {
    return state.dailyPurchases[itemId] ?? 0
  }, [state.dailyPurchases])

  const getCaptureBoost = useCallback((): number => {
    return calculateCaptureBoost(activeCaptureBoostRef.current)
  }, [])

  const consumeCaptureBoost = useCallback(() => {
    activeCaptureBoostRef.current = null
  }, [])

  const isCaptureBoostActive = useCallback((): boolean => {
    return activeCaptureBoostRef.current !== null
  }, [])

  const value = useMemo<ShopContextValue>(() => ({
    state,
    buyItem,
    useItem,
    checkIn,
    getItemCount,
    getDailyPurchaseCount,
    getCaptureBoost,
    consumeCaptureBoost,
    isCaptureBoostActive,
  }), [state, buyItem, useItem, checkIn, getItemCount, getDailyPurchaseCount, getCaptureBoost, consumeCaptureBoost, isCaptureBoostActive])

  return (
    <ShopContext.Provider value={value}>
      {children}
    </ShopContext.Provider>
  )
}
