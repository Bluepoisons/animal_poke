import type { WeatherType } from './types'

/** 天气概率权重表 */
export const WEATHER_ROLL_TABLE: { weather: WeatherType; weight: number }[] = [
  { weather: 'sunny',    weight: 33 },
  { weather: 'cloudy',   weight: 24 },
  { weather: 'overcast', weight: 18 },
  { weather: 'rainy',    weight: 15 },
  { weather: 'snowy',    weight: 4  },
  { weather: 'foggy',    weight: 4  },
  { weather: 'extreme',  weight: 2  },
]

/** 总权重（100） */
export const WEATHER_TOTAL_WEIGHT = 100

/** 天气 → 捕获倍率映射 */
export const WEATHER_CAPTURE_MODIFIER: Record<WeatherType, number> = {
  sunny:    1.05,  // +5%
  cloudy:   1.00,  // 标准
  overcast: 0.95,  // -5%
  rainy:    0.85,  // -15%
  snowy:    0.90,  // -10%
  foggy:    0.90,  // -10%
  extreme:  0.00,  // 不可捕获
}

/** 天气 → 感冒触发概率 */
export const COLD_PROBABILITY: Partial<Record<WeatherType, number>> = {
  rainy: 0.08,   // 雨天 8%
  snowy: 0.06,   // 雪天 6%
  // 其他天气为 0（无风险）
}

/** 天气周起始时间偏移量（周日 0:00） */
export const WEEK_RESET_DAY = 0 // 周日
export const WEEK_RESET_HOUR = 0
export const WEEK_RESET_MINUTE = 0

/** 天气缓存 localStorage key */
export const WEATHER_CACHE_KEY = 'animal_poke_weather_cache'

/** 一周天数 */
export const DAYS_IN_WEEK = 7

/** 星期标签 */
export const DAY_LABELS = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'] as const
