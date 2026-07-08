import { WEATHER_ROLL_TABLE, WEATHER_TOTAL_WEIGHT, WEATHER_CAPTURE_MODIFIER, COLD_PROBABILITY, DAY_LABELS } from './constants'
import type { WeatherType, WeekWeather, CurrentWeather, CaptureModifierResult, ColdCheckResult } from './types'

/**
 * 根据城市名 + 周起始时间生成确定性种子
 * 同一城市同一周始终得到相同天气，不同城市/不同周则不同
 * 使用简单的 djb2 哈希算法
 */
export function generateSeed(city: string, weekStart: number): number {
  const key = `${city}_${weekStart}`
  let hash = 5381
  for (let i = 0; i < key.length; i++) {
    hash = ((hash << 5) + hash + key.charCodeAt(i)) | 0
  }
  return hash >>> 0 // 转为无符号 32 位整数
}

/**
 * 基于种子的伪随机数生成器（Mulberry32）
 * 确保同一种子每次生成相同序列
 */
export function createRng(seed: number): () => number {
  let s = seed
  return () => {
    s |= 0
    s = (s + 0x6D2B79F5) | 0
    let t = Math.imul(s ^ (s >>> 15), 1 | s)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

/**
 * 根据权重表随机选取一项
 * @param table 权重表
 * @param rng 随机数生成器
 * @param totalWeight 总权重
 */
export function weightedRandom<T>(
  table: { item: T; weight: number }[],
  rng: () => number,
  totalWeight: number
): T {
  const roll = rng() * totalWeight
  let cumulative = 0
  for (const entry of table) {
    cumulative += entry.weight
    if (roll < cumulative) {
      return entry.item
    }
  }
  // fallback：返回最后一项（理论上不会到达）
  return table[table.length - 1].item
}

/**
 * 根据城市 + 周起始时间生成一周天气（确定性）
 * @param city 地级市名
 * @param weekStart 本周起始时间戳（周日 0:00）
 * @returns 7 天天气数组
 */
export function generateWeekWeather(city: string, weekStart: number): WeekWeather {
  const seed = generateSeed(city, weekStart)
  const rng = createRng(seed)

  return Array.from({ length: 7 }, (_, i): CurrentWeather => {
    const weatherTable = WEATHER_ROLL_TABLE.map(e => ({ item: e.weather, weight: e.weight }))
    const weather = weightedRandom(weatherTable, rng, WEATHER_TOTAL_WEIGHT)
    return {
      weather,
      dayLabel: DAY_LABELS[i],
    }
  }) as WeekWeather
}

/**
 * 获取今天的天气（从周天气数组中按今天是周几索引）
 * @param week 本周天气数组
 * @returns 今日天气，若 week 为 null 返回 null
 */
export function getTodayWeather(week: WeekWeather | null): CurrentWeather | null {
  if (!week) return null
  const today = new Date().getDay() // 0=周日, 1=周一, ..., 6=周六
  // DAY_LABELS 索引 0 对应周日，直接对齐
  return week[today] ?? null
}

/**
 * 计算本周起始时间戳（周日 0:00）
 */
export function getWeekStart(now: number = Date.now()): number {
  const date = new Date(now)
  const day = date.getDay() // 0=周日
  const start = new Date(date.getFullYear(), date.getMonth(), date.getDate() - day)
  start.setHours(0, 0, 0, 0)
  return start.getTime()
}

/**
 * 计算天气对捕获成功率的修正
 * @param weather 当前天气类型
 * @returns 修正结果（百分比描述 + 倍率）
 */
export function getCaptureModifier(weather: WeatherType): CaptureModifierResult {
  const multiplier = WEATHER_CAPTURE_MODIFIER[weather]
  const modifier = Math.round((multiplier - 1) * 100)
  const sign = modifier >= 0 ? '+' : ''
  return {
    modifier,
    multiplier,
    description: modifier === 0 ? '标准捕获率' : `捕获率 ${sign}${modifier}%`,
  }
}

/**
 * 查询当前天气的感冒风险
 * @param weather 当前天气类型
 * @returns 感冒风险结果
 */
export function getColdRisk(weather: WeatherType): ColdCheckResult {
  const probability = COLD_PROBABILITY[weather] ?? 0
  let description: string
  if (weather === 'rainy') {
    description = '雨天出行 · 宠物感冒概率 8%'
  } else if (weather === 'snowy') {
    description = '雪天出行 · 宠物感冒概率 6%'
  } else {
    description = '当前天气无不健康状况'
  }
  return {
    isRisky: probability > 0,
    probability,
    description,
  }
}
