import { useContext } from 'react'
import { WeatherContext } from './WeatherContext'
import type { WeatherContextValue } from './types'

/** 自定义 Hook，封装 WeatherContext 消费，处理 null 检查 */
export function useWeather(): WeatherContextValue {
  const context = useContext(WeatherContext)
  if (!context) {
    throw new Error('useWeather 必须在 WeatherProvider 内使用')
  }
  return context
}
