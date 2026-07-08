import { describe, it, expect } from 'vitest'
import {
  createColdEffect,
  createPleasureEffect,
  isExpired,
  getStatMultiplier,
  getColdRemainingDays,
  applyColdToRecord,
  cureColdFromRecord,
  checkRecovery,
  clearExpiredEffects,
  isPleasureWeather,
  getStatusDisplay,
} from './logic'
import { COLD_DURATION_DAYS, DAY_MS, COLD_STAT_MULTIPLIER } from './constants'
import type { PetStatusRecord } from './types'

// ===== 辅助函数 =====

function makeRecord(overrides: Partial<PetStatusRecord> = {}): PetStatusRecord {
  return {
    petId: 'test_pet',
    effects: [],
    permanentDamageMultiplier: 1.0,
    coldCount: 0,
    ...overrides,
  }
}

function makeColdRecord(now: number = Date.now()): PetStatusRecord {
  const effect = createColdEffect('weather', now)
  return makeRecord({ effects: [effect], coldCount: 1 })
}

const NOW = 1700000000000 // 固定时间戳用于测试

// ===== createColdEffect 测试 =====

describe('createColdEffect', () => {
  it('创建正确的感冒效果字段', () => {
    const effect = createColdEffect('weather', NOW)
    expect(effect.type).toBe('cold')
    expect(effect.source).toBe('weather')
    expect(effect.startTime).toBe(NOW)
    expect(effect.durationDays).toBe(COLD_DURATION_DAYS)
    expect(effect.expiresAt).toBe(NOW + COLD_DURATION_DAYS * DAY_MS)
  })

  it('不同 source 正确传递', () => {
    expect(createColdEffect('battle', NOW).source).toBe('battle')
    expect(createColdEffect('capture', NOW).source).toBe('capture')
  })
})

// ===== createPleasureEffect 测试 =====

describe('createPleasureEffect', () => {
  it('当日有效，过期时间为当天 23:59:59', () => {
    const noon = new Date(2026, 6, 8, 12, 0, 0).getTime()
    const effect = createPleasureEffect(noon)
    const endOfDay = new Date(2026, 6, 8, 23, 59, 59, 999).getTime()
    expect(effect.expiresAt).toBe(endOfDay)
    expect(effect.type).toBe('pleasure')
    expect(effect.source).toBe('weather')
  })
})

// ===== isExpired 测试 =====

describe('isExpired', () => {
  it('未过期返回 false', () => {
    const effect = createColdEffect('weather', NOW)
    expect(isExpired(effect, NOW + 1000)).toBe(false)
  })

  it('已过期返回 true', () => {
    const effect = createColdEffect('weather', NOW)
    expect(isExpired(effect, NOW + COLD_DURATION_DAYS * DAY_MS + 1)).toBe(true)
  })

  it('恰好过期时间点返回 true', () => {
    const effect = createColdEffect('weather', NOW)
    expect(isExpired(effect, effect.expiresAt)).toBe(true)
  })
})

// ===== getStatMultiplier 测试 =====

describe('getStatMultiplier', () => {
  it('无记录返回 1.0', () => {
    expect(getStatMultiplier(null)).toBe(1.0)
    expect(getStatMultiplier(undefined)).toBe(1.0)
  })

  it('仅有感冒返回 0.65', () => {
    const record = makeColdRecord(NOW)
    expect(getStatMultiplier(record, NOW)).toBe(COLD_STAT_MULTIPLIER)
  })

  it('感冒+永久损伤叠加', () => {
    const record = makeColdRecord(NOW)
    record.permanentDamageMultiplier = 0.95
    expect(getStatMultiplier(record, NOW)).toBeCloseTo(COLD_STAT_MULTIPLIER * 0.95, 5)
  })

  it('过期感冒不计入', () => {
    const record = makeColdRecord(NOW)
    expect(getStatMultiplier(record, NOW + COLD_DURATION_DAYS * DAY_MS + 1)).toBe(1.0)
  })

  it('无活跃效果的记录返回 1.0', () => {
    const record = makeRecord()
    expect(getStatMultiplier(record)).toBe(1.0)
  })
})

// ===== getColdRemainingDays 测试 =====

describe('getColdRemainingDays', () => {
  it('无感冒返回 null', () => {
    expect(getColdRemainingDays(null)).toBeNull()
    expect(getColdRemainingDays(makeRecord())).toBeNull()
  })

  it('刚创建返回 5 天', () => {
    const record = makeColdRecord(NOW)
    expect(getColdRemainingDays(record, NOW)).toBe(5)
  })

  it('剩余 2.5 天返回 3（向上取整）', () => {
    const effect = createColdEffect('weather', NOW)
    const record = makeRecord({ effects: [effect] })
    const halfWay = NOW + 2.5 * DAY_MS
    expect(getColdRemainingDays(record, halfWay)).toBe(3)
  })

  it('过期返回 null', () => {
    const record = makeColdRecord(NOW)
    expect(getColdRemainingDays(record, NOW + COLD_DURATION_DAYS * DAY_MS + 1)).toBeNull()
  })
})

// ===== applyColdToRecord 测试 =====

describe('applyColdToRecord', () => {
  it('新记录添加成功', () => {
    const { record, added } = applyColdToRecord(null, 'capture', NOW)
    expect(added).toBe(true)
    expect(record.effects).toHaveLength(1)
    expect(record.effects[0].type).toBe('cold')
    expect(record.coldCount).toBe(1)
  })

  it('已有感冒不重复添加', () => {
    const existing = makeColdRecord(NOW)
    const { record, added } = applyColdToRecord(existing, 'battle', NOW)
    expect(added).toBe(false)
    expect(record.effects).toHaveLength(1)
    expect(record.coldCount).toBe(1)
  })

  it('无记录时 petId 为空（由 reducer 填充）', () => {
    const { record } = applyColdToRecord(null, 'weather', NOW)
    expect(record.petId).toBe('')
  })
})

// ===== cureColdFromRecord 测试 =====

describe('cureColdFromRecord', () => {
  it('移除感冒成功', () => {
    const record = makeColdRecord(NOW)
    const { record: newRecord, cured } = cureColdFromRecord(record)
    expect(cured).toBe(true)
    expect(newRecord!.effects).toHaveLength(0)
  })

  it('无感冒返回 cured=false', () => {
    const record = makeRecord()
    const { cured } = cureColdFromRecord(record)
    expect(cured).toBe(false)
  })

  it('null 记录返回 cured=false', () => {
    const { record, cured } = cureColdFromRecord(null)
    expect(cured).toBe(false)
    expect(record).toBeNull()
  })

  it('治愈不影响永久损伤', () => {
    const record = makeColdRecord(NOW)
    record.permanentDamageMultiplier = 0.95
    const { record: newRecord } = cureColdFromRecord(record)
    expect(newRecord!.permanentDamageMultiplier).toBe(0.95)
  })
})

// ===== checkRecovery 测试 =====

describe('checkRecovery', () => {
  it('未过期不处理', () => {
    const record = makeColdRecord(NOW)
    const result = checkRecovery(record, NOW + 1000)
    expect(result.expired).toBe(false)
    expect(result.permanentDamageTriggered).toBe(false)
    expect(result.record.effects).toHaveLength(1)
  })

  it('过期移除感冒', () => {
    const record = makeColdRecord(NOW)
    const result = checkRecovery(record, NOW + COLD_DURATION_DAYS * DAY_MS + 1)
    expect(result.expired).toBe(true)
    expect(result.record.effects).toHaveLength(0)
  })

  it('永久损伤下线时不触发', () => {
    const record = makeColdRecord(NOW)
    const result = checkRecovery(record, NOW + COLD_DURATION_DAYS * DAY_MS + 1)
    expect(result.permanentDamageTriggered).toBe(false)
    expect(result.record.permanentDamageMultiplier).toBe(1.0)
  })
})

// ===== isPleasureWeather 测试 =====

describe('isPleasureWeather', () => {
  it('晴天返回 true', () => {
    expect(isPleasureWeather('sunny')).toBe(true)
  })

  it('雨天返回 false', () => {
    expect(isPleasureWeather('rainy')).toBe(false)
  })

  it('雪天返回 false', () => {
    expect(isPleasureWeather('snowy')).toBe(false)
  })
})

// ===== clearExpiredEffects 测试 =====

describe('clearExpiredEffects', () => {
  it('清理过期的愉悦效果', () => {
    const pleasureEffect = createPleasureEffect(NOW)
    pleasureEffect.expiresAt = NOW - 1000 // 已过期
    const record = makeRecord({ effects: [pleasureEffect] })
    const cleaned = clearExpiredEffects(record, NOW)
    expect(cleaned.effects).toHaveLength(0)
  })

  it('不清理未过期的感冒效果', () => {
    const coldEffect = createColdEffect('weather', NOW)
    const record = makeRecord({ effects: [coldEffect] })
    const cleaned = clearExpiredEffects(record, NOW)
    expect(cleaned.effects).toHaveLength(1)
  })
})

// ===== getStatusDisplay 测试 =====

describe('getStatusDisplay', () => {
  it('无记录 + 非晴天 → 正常', () => {
    const displays = getStatusDisplay(null, 'cloudy', NOW)
    expect(displays).toHaveLength(1)
    expect(displays[0].type).toBe('normal')
  })

  it('无记录 + 晴天 → 愉悦', () => {
    const displays = getStatusDisplay(null, 'sunny', NOW)
    expect(displays).toHaveLength(1)
    expect(displays[0].type).toBe('pleasure')
  })

  it('有感冒 + 晴天 → 感冒 + 愉悦', () => {
    const record = makeColdRecord(NOW)
    const displays = getStatusDisplay(record, 'sunny', NOW)
    expect(displays).toHaveLength(2)
    expect(displays[0].type).toBe('cold')
    expect(displays[1].type).toBe('pleasure')
  })

  it('感冒剩余天数正确显示', () => {
    const record = makeColdRecord(NOW)
    const displays = getStatusDisplay(record, 'cloudy', NOW)
    expect(displays[0].remainingDays).toBe(5)
  })
})
