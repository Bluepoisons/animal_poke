import { useContext } from 'react'
import { StaminaContext } from './StaminaContext'
import type { StaminaContextValue } from './types'

/** 自定义 Hook，封装 StaminaContext 消费，处理 null 检查 */
export function useStamina(): StaminaContextValue {
  const context = useContext(StaminaContext)
  if (!context) {
    throw new Error('useStamina 必须在 StaminaProvider 内使用')
  }
  return context
}
