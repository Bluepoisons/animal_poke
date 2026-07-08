import { useContext } from 'react'
import { EconomyContext } from './EconomyContext'
import type { EconomyContextValue } from './types'

/** 自定义 Hook，封装 EconomyContext 消费 */
export function useEconomy(): EconomyContextValue {
  const context = useContext(EconomyContext)
  if (!context) {
    throw new Error('useEconomy 必须在 EconomyProvider 内使用')
  }
  return context
}
