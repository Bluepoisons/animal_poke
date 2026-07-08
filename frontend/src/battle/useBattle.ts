import { useContext } from 'react'
import { BattleContext } from './BattleContext'
import type { BattleContextValue } from './types'

/** 战斗系统 Hook：消费 BattleContext */
export function useBattle(): BattleContextValue {
  const ctx = useContext(BattleContext)
  if (!ctx) {
    throw new Error('useBattle 必须在 BattleProvider 内使用')
  }
  return ctx
}
