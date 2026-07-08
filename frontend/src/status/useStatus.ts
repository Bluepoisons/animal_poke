import { useContext } from 'react'
import { StatusContext } from './StatusContext'
import type { StatusContextValue } from './types'

/** 自定义 Hook，封装 StatusContext 消费，处理 null 检查 */
export function useStatus(): StatusContextValue {
  const context = useContext(StatusContext)
  if (!context) {
    throw new Error('useStatus 必须在 StatusProvider 内使用')
  }
  return context
}
