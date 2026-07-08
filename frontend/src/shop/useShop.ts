import { useContext } from 'react'
import { ShopContext } from './ShopContext'
import type { ShopContextValue } from './types'

/** 自定义 Hook，封装 ShopContext 消费，处理 null 检查 */
export function useShop(): ShopContextValue {
  const context = useContext(ShopContext)
  if (!context) {
    throw new Error('useShop 必须在 ShopProvider 内使用')
  }
  return context
}
