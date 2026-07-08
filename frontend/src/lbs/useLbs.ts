import { useContext } from 'react'
import { LbsContext } from './LbsContext'
import type { LbsContextValue } from './types'

/** 自定义 Hook，封装 LbsContext 消费，处理 null 检查 */
export function useLbs(): LbsContextValue {
  const context = useContext(LbsContext)
  if (!context) {
    throw new Error('useLbs 必须在 LbsProvider 内使用')
  }
  return context
}
