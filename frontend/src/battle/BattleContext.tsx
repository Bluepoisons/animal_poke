import React, { createContext, useReducer, useEffect, useCallback, useMemo, useRef } from 'react'
import { useStamina } from '../stamina/useStamina'
import { useShop } from '../shop/useShop'
import { useStatus } from '../status/useStatus'
import { useWeather } from '../weather/useWeather'
import type {
  BattleState,
  BattleAction,
  BattlePhase,
  StrategyType,
  WeatherType,
  BattlePet,
  BattleLogEntry,
  BattleResult,
  BattleRewards,
  BattleContextValue,
  CardEntry,
} from './types'
import {
  BATTLE_STAMINA_COST,
  MAX_ROUNDS,
  AUTO_PLAY_INTERVAL_MS,
  MAX_ENERGY,
} from './constants'
import {
  executeRound,
  executeUltimate,
  checkBattleEnd,
  generateEnemy,
  computeRewards,
  cardEntryToBattlePet,
  computeBattleStats,
  applyWeatherModifier,
  applyStatusMultiplier,
  pickElement,
} from './logic'

/** 初始状态 */
const initialBattleState: BattleState = {
  phase: 'idle',
  playerPet: null,
  enemyPet: null,
  round: 0,
  maxRounds: MAX_ROUNDS,
  log: [],
  result: null,
  rewards: null,
  strategy: 'balanced',
  weather: 'sunny',
  isAutoPlaying: true,
}

/** Reducer */
function battleReducer(state: BattleState, action: BattleAction): BattleState {
  switch (action.type) {
    case 'ENTER_SELECT':
      return { ...state, phase: 'selecting', playerPet: null, enemyPet: null, log: [], round: 0, result: null, rewards: null }

    case 'SELECT_PET':
      return { ...state, playerPet: action.pet }

    case 'START_MATCHING':
      return { ...state, phase: 'matching' }

    case 'MATCH_COMPLETE':
      return { ...state, phase: 'battling', enemyPet: action.enemy, round: 1 }

    case 'BATTLE_START':
      return { ...state, phase: 'battling', round: 1 }

    case 'EXECUTE_ROUND':
      return {
        ...state,
        playerPet: action.playerPet,
        enemyPet: action.enemyPet,
        log: [...state.log.slice(-47), ...action.log], // 最多 50 条日志
      }

    case 'USE_ULTIMATE':
      return {
        ...state,
        playerPet: action.playerPet,
        enemyPet: action.enemyPet,
        log: [...state.log.slice(-49), action.log[0]], // 必杀日志
      }

    case 'SET_STRATEGY':
      return { ...state, strategy: action.strategy }

    case 'USE_ITEM':
      return {
        ...state,
        playerPet: action.playerPet,
        log: [...state.log.slice(-49), action.log[0]],
      }

    case 'BATTLE_END':
      return {
        ...state,
        phase: 'result',
        result: action.result,
        rewards: action.rewards,
        isAutoPlaying: false,
      }

    case 'SET_WEATHER':
      return { ...state, weather: action.weather }

    case 'SET_AUTO_PLAY':
      return { ...state, isAutoPlaying: action.playing }

    case 'RESET':
      return initialBattleState

    default:
      return state
  }
}

export const BattleContext = createContext<BattleContextValue | null>(null)

export const BattleProvider: React.FC<{ children: React.ReactNode; weather?: WeatherType }> = ({ children, weather = 'sunny' }) => {
  const stamina = useStamina()
  const shop = useShop()
  const status = useStatus()
  const weatherCtx = useWeather()

  const [state, dispatch] = useReducer(battleReducer, undefined, () => ({
    ...initialBattleState,
    weather,
  }))

  // 自动回合计时器
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    if (state.phase === 'battling' && state.isAutoPlaying) {
      // 能量满时暂停自动播放，等待玩家决定是否释放必杀
      if (state.playerPet && state.playerPet.energy >= MAX_ENERGY) {
        // 暂停，等玩家操作
        if (timerRef.current) {
          clearInterval(timerRef.current)
          timerRef.current = null
        }
        return
      }

      timerRef.current = setInterval(() => {
        if (!state.playerPet || !state.enemyPet) return

        // 检查战斗是否已结束
        const endResult = checkBattleEnd(state.playerPet, state.enemyPet, state.round)
        if (endResult) {
          const rewards = computeRewards(endResult, state.enemyPet.rarity, 1.0)
          dispatch({ type: 'BATTLE_END', result: endResult, rewards })
          return
        }

        // 执行回合
        const { player, enemy, logs } = executeRound(
          state.playerPet,
          state.enemyPet,
          state.strategy,
          state.weather,
          state.round
        )

        dispatch({ type: 'EXECUTE_ROUND', log: logs, playerPet: player, enemyPet: enemy })

        // 回合数 +1
        const nextRound = state.round + 1
        const nextEndResult = checkBattleEnd(player, enemy, nextRound)
        if (nextEndResult) {
          const rewards = computeRewards(nextEndResult, enemy.rarity, 1.0)
          dispatch({ type: 'BATTLE_END', result: nextEndResult, rewards })
        }
      }, AUTO_PLAY_INTERVAL_MS)

      return () => {
        if (timerRef.current) {
          clearInterval(timerRef.current)
          timerRef.current = null
        }
      }
    } else {
      if (timerRef.current) {
        clearInterval(timerRef.current)
        timerRef.current = null
      }
    }
  }, [state.phase, state.isAutoPlaying, state.playerPet?.energy])

  // 进入选宠
  const enterSelect = useCallback(() => {
    dispatch({ type: 'ENTER_SELECT' })
  }, [])

  // 选择宠物（消耗 20 体力）
  const selectPet = useCallback((entry: CardEntry): boolean => {
    // 检查体力
    if (stamina.state.currentStamina < BATTLE_STAMINA_COST) {
      return false
    }
    // 扣体力
    stamina.consumeStamina(BATTLE_STAMINA_COST)

    const pet = cardEntryToBattlePet(entry, state.weather)
    // 应用状态修正（感冒 -35% 等）
    const statusMultiplier = status.getStatModifier(entry.id)
    if (statusMultiplier !== 1.0) {
      pet.stats = applyStatusMultiplier(pet.stats, statusMultiplier)
      pet.currentHp = pet.stats.hp
    }
    dispatch({ type: 'SELECT_PET', pet })
    return true
  }, [stamina, state.weather, status])

  // 开始匹配
  const startMatching = useCallback(() => {
    dispatch({ type: 'START_MATCHING' })
    // 模拟匹配延迟
    setTimeout(() => {
      const enemy = generateEnemy(stamina.state.level)
      dispatch({ type: 'MATCH_COMPLETE', enemy })
    }, 1500)
  }, [stamina.state.level])

  // 执行下一回合（手动模式）
  const executeNextRound = useCallback(() => {
    if (!state.playerPet || !state.enemyPet) return

    // 检查战斗是否已结束
    const endResult = checkBattleEnd(state.playerPet, state.enemyPet, state.round)
    if (endResult) {
      const rewards = computeRewards(endResult, state.enemyPet.rarity, 1.0)
      dispatch({ type: 'BATTLE_END', result: endResult, rewards })
      return
    }

    const { player, enemy, logs } = executeRound(
      state.playerPet,
      state.enemyPet,
      state.strategy,
      state.weather,
      state.round
    )

    dispatch({ type: 'EXECUTE_ROUND', log: logs, playerPet: player, enemyPet: enemy })

    const nextRound = state.round + 1
    const nextEndResult = checkBattleEnd(player, enemy, nextRound)
    if (nextEndResult) {
      const rewards = computeRewards(nextEndResult, enemy.rarity, 1.0)
      dispatch({ type: 'BATTLE_END', result: nextEndResult, rewards })
    }
  }, [state.playerPet, state.enemyPet, state.strategy, state.weather, state.round])

  // 释放必杀
  const useUltimate = useCallback((): boolean => {
    if (!state.playerPet || !state.enemyPet) return false
    if (state.playerPet.energy < MAX_ENERGY) return false

    const { attacker, defender, log } = executeUltimate(
      state.playerPet,
      state.enemyPet,
      state.strategy,
      state.weather,
      state.round,
      '我方'
    )

    dispatch({ type: 'USE_ULTIMATE', log: [log], playerPet: attacker, enemyPet: defender })

    // 检查是否击杀
    const endResult = checkBattleEnd(attacker, defender, state.round)
    if (endResult) {
      const rewards = computeRewards(endResult, defender.rarity, 1.0)
      dispatch({ type: 'BATTLE_END', result: endResult, rewards })
    }

    return true
  }, [state.playerPet, state.enemyPet, state.strategy, state.weather, state.round])

  // 切换策略
  const setStrategy = useCallback((strategy: StrategyType) => {
    dispatch({ type: 'SET_STRATEGY', strategy })
  }, [])

  // 使用道具
  const useBattleItem = useCallback((itemId: string): boolean => {
    if (!state.playerPet) return false

    const result = shop.useItem(itemId as any)
    if (!result.success) return false

    // food_pack: 恢复 30% HP
    const pet = { ...state.playerPet, stats: { ...state.playerPet.stats } }
    if (itemId === 'food_pack') {
      const healAmount = Math.round(pet.stats.hp * 0.3)
      pet.currentHp = Math.min(pet.stats.hp, pet.currentHp + healAmount)
    }

    const log: BattleLogEntry = {
      round: state.round,
      text: itemId === 'food_pack' ? `使用食物包，恢复 ${Math.round(pet.stats.hp * 0.3)} HP` : `使用道具`,
      type: 'item',
    }

    dispatch({ type: 'USE_ITEM', log: [log], playerPet: pet })
    return true
  }, [state.playerPet, state.round, shop])

  // 切换自动/手动
  const toggleAutoPlay = useCallback(() => {
    dispatch({ type: 'SET_AUTO_PLAY', playing: !state.isAutoPlaying })
  }, [state.isAutoPlaying])

  // 确认结算
  const finishBattle = useCallback(() => {
    if (state.rewards) {
      stamina.addGold(state.rewards.gold)
      // 道具掉落处理
      if (state.rewards.droppedItem) {
        shop.addItem(state.rewards.droppedItem as any)
      }
    }

    // 感冒判定：雨雪天出战宠物有概率感冒（需在 RESET 前读取 playerPet.id）
    if (state.playerPet) {
      const coldRisk = weatherCtx.getColdRisk()
      if (coldRisk.isRisky && Math.random() < coldRisk.probability) {
        status.applyCold(state.playerPet.id, 'battle')
      }
    }

    dispatch({ type: 'RESET' })
  }, [state.rewards, state.playerPet, stamina, shop, weatherCtx, status])

  // 重置
  const reset = useCallback(() => {
    dispatch({ type: 'RESET' })
  }, [])

  const value = useMemo<BattleContextValue>(() => ({
    state,
    enterSelect,
    selectPet,
    startMatching,
    executeNextRound,
    useUltimate,
    setStrategy,
    useBattleItem,
    toggleAutoPlay,
    finishBattle,
    reset,
  }), [state, enterSelect, selectPet, startMatching, executeNextRound, useUltimate, setStrategy, useBattleItem, toggleAutoPlay, finishBattle, reset])

  return (
    <BattleContext.Provider value={value}>
      {children}
    </BattleContext.Provider>
  )
}
