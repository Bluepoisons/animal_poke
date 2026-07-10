/**
 * 从后端拉取本周天气（需 lat/lng；与后端契约对齐）
 * 统一走 authedRequest：Bearer + 401 singleflight 续签
 * 返回格式：{ week: [...], days: [...], source, cached, degraded, reason_code }
 */
import type { WeekWeather, WeatherType } from './types'
import { authedRequest } from '../auth/deviceAuth'

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
  _token?: string,
  _baseUrl = '',
): Promise<{ week: WeekWeather; source: 'backend' | 'internal'; meta?: WeatherAPIResponse } | null> {
  try {
    // _token/_baseUrl 保留兼容旧调用方；统一走公开配置 + 设备 Token
    void _token
    void _baseUrl
    const data = await authedRequest<WeatherAPIResponse>({
      method: 'GET',
      path: `/api/v1/weather/week?lat=${encodeURIComponent(String(lat))}&lng=${encodeURIComponent(String(lng))}`,
      allowRetry: true,
      timeoutMs: 12_000,
    })
    const rows = (data.week && data.week.length ? data.week : data.days) ?? []
    if (!Array.isArray(rows) || rows.length < 7) {
      throw new Error('后端天气数据格式异常')
    }
    const week = rows.slice(0, 7).map((d, i) => ({
      weather: mapWeather(d.weather || d.skycon),
      dayLabel: d.day_label || d.dayLabel || ['周一', '周二', '周三', '周四', '周五', '周六', '周日'][i],
    })) as WeekWeather
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
