/**
 * 从后端拉取本周天气（需 lat/lng；与后端契约对齐）
 * 返回格式：{ week: [...], days: [...], source, cached, degraded, reason_code }
 */
import type { WeekWeather, WeatherType } from './types'

export interface WeatherAPIResponse {
  week?: Array<{
    weather?: string
    skycon?: string
    day_label?: string
    dayLabel?: string
    date?: string
  }>
  days?: Array<{
    weather?: string
    skycon?: string
    day_label?: string
    dayLabel?: string
    date?: string
  }>
  source?: string
  cached?: boolean
  degraded?: boolean
  reason_code?: string
}

const SKYCON_TO_FRONTEND: Record<string, WeatherType> = {
  CLEAR: 'sunny',
  CLOUDY: 'cloudy',
  RAIN: 'rainy',
  SNOW: 'snowy',
  HAZE: 'foggy',
  WIND: 'overcast',
  sunny: 'sunny',
  cloudy: 'cloudy',
  overcast: 'overcast',
  rainy: 'rainy',
  snowy: 'snowy',
  foggy: 'foggy',
  extreme: 'extreme',
}

function mapWeather(raw?: string): WeatherType {
  if (!raw) return 'sunny'
  return SKYCON_TO_FRONTEND[raw] ?? 'sunny'
}

export async function fetchWeekWeather(
  lat: number,
  lng: number,
  token?: string,
  baseUrl = '',
): Promise<{ week: WeekWeather; source: 'backend' | 'internal'; meta?: WeatherAPIResponse } | null> {
  try {
    const headers: Record<string, string> = {}
    if (token) headers.Authorization = `Bearer ${token}`
    const resp = await fetch(
      `${baseUrl}/api/v1/weather/week?lat=${encodeURIComponent(String(lat))}&lng=${encodeURIComponent(String(lng))}`,
      { headers },
    )
    if (!resp.ok) throw new Error(`weather/week 请求失败: ${resp.status}`)
    const data = (await resp.json()) as WeatherAPIResponse
    const rows = (data.week && data.week.length ? data.week : data.days) ?? []
    if (!Array.isArray(rows) || rows.length < 7) {
      throw new Error('后端天气数据格式异常')
    }
    const week = rows.slice(0, 7).map((d, i) => ({
      weather: mapWeather(d.weather || d.skycon),
      dayLabel: d.day_label || d.dayLabel || ['周一', '周二', '周三', '周四', '周五', '周六', '周日'][i],
    })) as WeekWeather
    // 仅当非 degraded 的 random/mock 时视为 backend 真实源；degraded 仍标记 backend 但前端可区分
    return { week, source: 'backend', meta: data }
  } catch {
    return null
  }
}

/** @deprecated 使用 lat/lng 版本 */
export async function fetchWeekWeatherByCity(city: string): Promise<WeekWeather | null> {
  void city
  return null
}
