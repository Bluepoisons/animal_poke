import type { WeekWeather } from './types'

/**
 * 从后端拉取本周天气（可选真实天气源）
 * @param city 地级市名
 * @returns 本周天气数组，后端不可用时返回 null（降级为游戏内随机）
 */
export async function fetchWeekWeather(city: string): Promise<WeekWeather | null> {
  try {
    const resp = await fetch(`/api/v1/weather/week?city=${encodeURIComponent(city)}`)
    if (!resp.ok) throw new Error(`weather/week 请求失败: ${resp.status}`)
    const data = await resp.json()
    // 期望后端返回格式：{ week: [{ weather, dayLabel }, ...] }
    if (data?.week && Array.isArray(data.week) && data.week.length === 7) {
      return data.week as WeekWeather
    }
    throw new Error('后端天气数据格式异常')
  } catch {
    // 静默降级：后端不可用时使用游戏内随机生成
    return null
  }
}
