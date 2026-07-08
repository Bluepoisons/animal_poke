import { useContext } from 'react'
import { DispatchContext } from './DispatchContext'
import type { DispatchContextValue } from './types'

/** 自定义 Hook，封装 DispatchContext 消费 */
export function useDispatch(): DispatchContextValue {
  const context = useContext(DispatchContext)
  if (!context) {
    throw new Error('useDispatch 必须在 DispatchProvider 内使用')
  }
  return context
}
