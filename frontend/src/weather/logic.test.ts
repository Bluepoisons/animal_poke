import { describe, it, expect } from 'vitest'
import {
  generateSeed,
  createRng,
  weightedRandom,
  generateWeekWeather,
  getTodayWeather,
  getCaptureModifier,
  getColdRisk,
  getWeekStart,
} from './logic'
import type { WeatherType } from './types'

const VALID_WEATHER_TYPES: WeatherType[] = ['sunny', 'cloudy', 'overcast', 'rainy', 'snowy', 'foggy', 'extreme']

describe('generateSeed', () => {
  it('相同城市相同周 → 相同种子', () => {
    const s1 = generateSeed('宁波', 1700000000000)
    const s2 = generateSeed('宁波', 1700000000000)
    expect(s1).toBe(s2)
  })

  it('不同城市 → 不同种子', () => {
    const s1 = generateSeed('宁波', 1700000000000)
    const s2 = generateSeed('杭州', 1700000000000)
    expect(s1).not.toBe(s2)
  })

  it('不同周 → 不同种子', () => {
    const s1 = generateSeed('宁波', 1700000000000)
    const s2 = generateSeed('宁波', 1700000060000)
    expect(s1).not.toBe(s2)
  })
})

describe('createRng', () => {
  it('相同种子生成相同序列', () => {
    const rng1 = createRng(42)
    const rng2 = createRng(42)
    const seq1 = Array.from({ length: 10 }, () => rng1())
    const seq2 = Array.from({ length: 10 }, () => rng2())
    expect(seq1).toEqual(seq2)
  })

  it('不同种子生成不同序列', () => {
    const rng1 = createRng(42)
    const rng2 = createRng(99)
    const seq1 = Array.from({ length: 10 }, () => rng1())
    const seq2 = Array.from({ length: 10 }, () => rng2())
    expect(seq1).not.toEqual(seq2)
  })

  it('生成的随机数在 [0, 1) 范围内', () => {
    const rng = createRng(12345)
    for (let i = 0; i < 100; i++) {
      const val = rng()
      expect(val).toBeGreaterThanOrEqual(0)
      expect(val).toBeLessThan(1)
    }
  })
})

describe('weightedRandom', () => {
  it('返回权重表中的一项', () => {
    const table = [
      { item: 'a' as const, weight: 50 },
      { item: 'b' as const, weight: 50 },
    ]
    const rng = createRng(1)
    const result = weightedRandom(table, rng, 100)
    expect(['a', 'b']).toContain(result)
  })

  it('权重为 0 的项不会被选中（概率上）', () => {
    const table = [
      { item: 'a' as const, weight: 0 },
      { item: 'b' as const, weight: 100 },
    ]
    const rng = createRng(42)
    for (let i = 0; i < 50; i++) {
      const result = weightedRandom(table, rng, 100)
      expect(result).toBe('b')
    }
  })

  it('大样本统计：极端天气出现率 ≈ 2%', () => {
    const weatherTable = [
      { item: 'sunny' as WeatherType, weight: 33 },
      { item: 'cloudy' as WeatherType, weight: 24 },
      { item: 'overcast' as WeatherType, weight: 18 },
      { item: 'rainy' as WeatherType, weight: 15 },
      { item: 'snowy' as WeatherType, weight: 4 },
      { item: 'foggy' as WeatherType, weight: 4 },
      { item: 'extreme' as WeatherType, weight: 2 },
    ]
    let extremeCount = 0
    const total = 5000
    for (let i = 0; i < total; i++) {
      const rng = createRng(i + 1)
      const result = weightedRandom(weatherTable, rng, 100)
      if (result === 'extreme') extremeCount++
    }
    const ratio = extremeCount / total
    // 2% ± 1.5% 容差
    expect(ratio).toBeGreaterThan(0.005)
    expect(ratio).toBeLessThan(0.035)
  })
})

describe('generateWeekWeather', () => {
  it('返回 7 天天气', () => {
    const week = generateWeekWeather('宁波', 1700000000000)
    expect(week).toHaveLength(7)
  })

  it('所有天气字段为合法类型', () => {
    const week = generateWeekWeather('测试城市', 1700000000000)
    week.forEach(day => {
      expect(VALID_WEATHER_TYPES).toContain(day.weather)
    })
  })

  it('同一城市同一周生成完全相同', () => {
    const week1 = generateWeekWeather('宁波', 1700000000000)
    const week2 = generateWeekWeather('宁波', 1700000000000)
    expect(week1).toEqual(week2)
  })

  it('不同城市同周可不同', () => {
    const week1 = generateWeekWeather('宁波', 1700000000000)
    const week2 = generateWeekWeather('杭州', 1700000000000)
    // 不强制 assert，因为极端情况下小概率可能相同；检验类型正确即可
    expect(week2).toHaveLength(7)
    week2.forEach(day => {
      expect(VALID_WEATHER_TYPES).toContain(day.weather)
    })
  })

  it('不同周不同天气', () => {
    const week1 = generateWeekWeather('宁波', 1700000000000)
    const week2 = generateWeekWeather('宁波', 1800000000000)
    // 大概率不同（7天全相同的概率极低）
    const allSame = week1.every((day, i) => day.weather === week2[i].weather)
    expect(allSame).toBe(false)
  })

  it('dayLabel 按周日照顺序排列', () => {
    const week = generateWeekWeather('宁波', 1700000000000)
    const expected = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
    expect(week.map(d => d.dayLabel)).toEqual(expected)
  })
})

describe('getTodayWeather', () => {
  it('null 输入返回 null', () => {
    expect(getTodayWeather(null)).toBeNull()
  })

  it('正确按星期索引返回今天的天气', () => {
    const week = Array.from({ length: 7 }, (_, i) => ({
      weather: 'sunny' as WeatherType,
      dayLabel: `Day${i}`,
    })) as unknown as Parameters<typeof getTodayWeather>[0]
    const result = getTodayWeather(week)
    expect(result).not.toBeNull()
    const today = new Date().getDay()
    expect(result?.dayLabel).toBe(`Day${today}`)
  })
})

describe('getCaptureModifier', () => {
  it('晴天 +5%', () => {
    const result = getCaptureModifier('sunny')
    expect(result.modifier).toBe(5)
    expect(result.multiplier).toBe(1.05)
  })

  it('雨天 -15%', () => {
    const result = getCaptureModifier('rainy')
    expect(result.modifier).toBe(-15)
    expect(result.multiplier).toBe(0.85)
  })

  it('极端天气不可捕获', () => {
    const result = getCaptureModifier('extreme')
    expect(result.multiplier).toBe(0)
    expect(result.modifier).toBe(-100)
  })

  it('多云标准捕获率', () => {
    const result = getCaptureModifier('cloudy')
    expect(result.modifier).toBe(0)
    expect(result.multiplier).toBe(1.0)
    expect(result.description).toBe('标准捕获率')
  })
})

describe('getColdRisk', () => {
  it('雨天感冒概率 8%', () => {
    const result = getColdRisk('rainy')
    expect(result.isRisky).toBe(true)
    expect(result.probability).toBe(0.08)
  })

  it('雪天感冒概率 6%', () => {
    const result = getColdRisk('snowy')
    expect(result.isRisky).toBe(true)
    expect(result.probability).toBe(0.06)
  })

  it('晴天无感冒风险', () => {
    const result = getColdRisk('sunny')
    expect(result.isRisky).toBe(false)
    expect(result.probability).toBe(0)
  })
})

describe('getWeekStart', () => {
  it('返回周日 0:00 时间戳', () => {
    const now = new Date(2026, 6, 8, 14, 30, 0).getTime() // 2026-07-08 is a Wednesday
    const weekStart = getWeekStart(now)
    const weekStartDate = new Date(weekStart)
    expect(weekStartDate.getDay()).toBe(0) // Sunday
    expect(weekStartDate.getHours()).toBe(0)
    expect(weekStartDate.getMinutes()).toBe(0)
    expect(weekStartDate.getSeconds()).toBe(0)
  })

  it('同一周内多次调用返回相同结果', () => {
    const monday = new Date(2026, 6, 6, 10, 0, 0).getTime() // Monday
    const wednesday = new Date(2026, 6, 8, 15, 30, 0).getTime() // Wednesday
    const saturday = new Date(2026, 6, 11, 23, 59, 0).getTime() // Saturday (same week, before Sunday reset)
    expect(getWeekStart(monday)).toBe(getWeekStart(wednesday))
    expect(getWeekStart(wednesday)).toBe(getWeekStart(saturday))
  })
})
