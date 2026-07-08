import {
  COLD_DURATION_DAYS,
  COLD_STAT_MULTIPLIER,
  DAY_MS,
  PERMANENT_DAMAGE_ENABLED,
  PERMANENT_DAMAGE_MIN_RATE,
  PERMANENT_DAMAGE_MAX_RATE,
  PERMANENT_DAMAGE_MULTIPLIER,
  STATUS_META,
} from './constants'
import type {
  StatusEffect,
  StatusSource,
  PetStatusRecord,
  StatusDisplay,
} from './types'
import type { WeatherType } from '../battle/types'

/**
 * 创建一条感冒状态效果
 * @param source 触发来源
 * @param now 当前时间戳（Unix ms），默认 Date.now()
 * @returns 感冒状态效果实例
 */
export function createColdEffect(
  source: StatusSource,
  now: number = Date.now()
): StatusEffect {
  const durationMs = COLD_DURATION_DAYS * DAY_MS
  return {
    type: 'cold',
    source,
    startTime: now,
    durationDays: COLD_DURATION_DAYS,
    expiresAt: now + durationMs,
  }
}

/**
 * 创建一条愉悦状态效果（当日有效，次日重置）
 * @param now 当前时间戳
 * @returns 愉悦状态效果实例
 */
export function createPleasureEffect(now: number = Date.now()): StatusEffect {
  // 愉悦当日有效：计算到当天 23:59:59 的剩余时间
  const endOfDay = new Date(now)
  endOfDay.setHours(23, 59, 59, 999)
  const expiresAt = endOfDay.getTime()
  const durationMs = expiresAt - now
  return {
    type: 'pleasure',
    source: 'weather',
    startTime: now,
    durationDays: Math.ceil(durationMs / DAY_MS),
    expiresAt,
  }
}

/**
 * 检查状态效果是否已过期
 * @param effect 状态效果
 * @param now 当前时间戳
 * @returns true = 已过期
 */
export function isExpired(effect: StatusEffect, now: number = Date.now()): boolean {
  return now >= effect.expiresAt
}

/**
 * 计算宠物的属性修正倍率（感冒 + 永久损伤，不含愉悦）
 *
 * 倍率叠加规则：
 *   最终倍率 = 感冒倍率 × 永久损伤倍率
 *
 * @param record 宠物状态记录（可为 null/undefined）
 * @returns 属性修正倍率（0~1）
 */
export function getStatMultiplier(record: PetStatusRecord | null | undefined, now: number = Date.now()): number {
  if (!record) return 1.0

  let multiplier = 1.0

  // 感冒修正
  const hasCold = record.effects.some(e => e.type === 'cold' && !isExpired(e, now))
  if (hasCold) {
    multiplier *= COLD_STAT_MULTIPLIER
  }

  // 永久损伤修正
  if (record.permanentDamageMultiplier < 1.0) {
    multiplier *= record.permanentDamageMultiplier
  }

  return multiplier
}

/**
 * 自然恢复判定结果
 */
export interface RecoveryResult {
  /** 感冒是否已过期 */
  expired: boolean
  /** 是否触发了永久损伤（PERMANENT_DAMAGE_ENABLED=false 时永远为 false） */
  permanentDamageTriggered: boolean
  /** 更新后的状态记录 */
  record: PetStatusRecord
}

/**
 * 检查宠物的感冒是否过期，并处理自然恢复
 *
 * @param record 宠物状态记录
 * @param now 当前时间戳
 * @param rng 随机数生成器（可选，用于测试注入）
 * @returns 恢复结果
 */
export function checkRecovery(
  record: PetStatusRecord,
  now: number = Date.now(),
  rng: () => number = Math.random
): RecoveryResult {
  const coldEffect = record.effects.find(e => e.type === 'cold')

  if (!coldEffect || !isExpired(coldEffect, now)) {
    return {
      expired: false,
      permanentDamageTriggered: false,
      record,
    }
  }

  // 移除过期感冒效果
  const remainingEffects = record.effects.filter(e => e !== coldEffect)

  // 永久损伤判定
  let permanentDamageTriggered = false
  let newMultiplier = record.permanentDamageMultiplier

  if (PERMANENT_DAMAGE_ENABLED) {
    const rate = PERMANENT_DAMAGE_MIN_RATE +
      rng() * (PERMANENT_DAMAGE_MAX_RATE - PERMANENT_DAMAGE_MIN_RATE)
    if (rng() < rate) {
      permanentDamageTriggered = true
      newMultiplier = record.permanentDamageMultiplier * PERMANENT_DAMAGE_MULTIPLIER
    }
  }

  return {
    expired: true,
    permanentDamageTriggered,
    record: {
      ...record,
      effects: remainingEffects,
      permanentDamageMultiplier: newMultiplier,
    },
  }
}

/**
 * 获取感冒剩余天数（向上取整）
 * @param record 宠物状态记录
 * @param now 当前时间戳
 * @returns 剩余天数（1~5），无感冒返回 null
 */
export function getColdRemainingDays(
  record: PetStatusRecord | null | undefined,
  now: number = Date.now()
): number | null {
  if (!record) return null

  const coldEffect = record.effects.find(e => e.type === 'cold')
  if (!coldEffect || isExpired(coldEffect, now)) return null

  const remainingMs = coldEffect.expiresAt - now
  return Math.max(1, Math.ceil(remainingMs / DAY_MS))
}

/**
 * 为宠物状态记录添加感冒效果
 * 若已有活跃感冒，不重复添加（返回原记录）
 *
 * @param record 宠物状态记录（可为 null，null 时创建新记录）
 * @param source 触发来源
 * @param now 当前时间戳
 * @returns 更新后的记录 + 是否成功添加
 */
export function applyColdToRecord(
  record: PetStatusRecord | null | undefined,
  source: StatusSource,
  now: number = Date.now()
): { record: PetStatusRecord; added: boolean } {
  // 已有活跃感冒 → 不重复
  if (record) {
    const hasCold = record.effects.some(e => e.type === 'cold' && !isExpired(e, now))
    if (hasCold) {
      return { record, added: false }
    }
  }

  const effect = createColdEffect(source, now)
  const baseRecord = record ?? {
    petId: '',
    effects: [],
    permanentDamageMultiplier: 1.0,
    coldCount: 0,
  }

  return {
    record: {
      ...baseRecord,
      effects: [...baseRecord.effects, effect],
      coldCount: baseRecord.coldCount + 1,
    },
    added: true,
  }
}

/**
 * 从宠物状态记录中移除感冒效果（不触发永久损伤判定）
 *
 * @param record 宠物状态记录
 * @returns 更新后的记录 + 是否成功移除
 */
export function cureColdFromRecord(
  record: PetStatusRecord | null | undefined
): { record: PetStatusRecord | null; cured: boolean } {
  if (!record) return { record: null, cured: false }

  const coldExists = record.effects.some(e => e.type === 'cold')
  if (!coldExists) return { record, cured: false }

  return {
    record: {
      ...record,
      effects: record.effects.filter(e => e.type !== 'cold'),
    },
    cured: true,
  }
}

/**
 * 根据天气判断是否应显示愉悦状态
 * @param weather 当前天气类型
 * @returns true = 晴天 → 愉悦
 */
export function isPleasureWeather(weather: WeatherType): boolean {
  return weather === 'sunny'
}

/**
 * 清理宠物记录中的所有过期效果（愉悦过期自动移除）
 * 注意：感冒过期不在此处理，需走 checkRecovery 流程（含永久损伤判定）
 *
 * @param record 宠物状态记录
 * @param now 当前时间戳
 * @returns 清理后的记录
 */
export function clearExpiredEffects(
  record: PetStatusRecord,
  now: number = Date.now()
): PetStatusRecord {
  // 仅清理非感冒的过期效果（愉悦等）
  const effects = record.effects.filter(
    e => e.type === 'cold' || !isExpired(e, now)
  )
  return { ...record, effects }
}

/**
 * 获取宠物的状态展示信息列表（供 UI 渲染）
 *
 * 优先级：感冒 > 愉悦 > 正常
 *
 * @param record 宠物状态记录
 * @param weather 当前天气
 * @param now 当前时间戳
 * @returns 状态展示信息列表
 */
export function getStatusDisplay(
  record: PetStatusRecord | null | undefined,
  weather: WeatherType | null,
  now: number = Date.now()
): StatusDisplay[] {
  const displays: StatusDisplay[] = []

  // 感冒
  if (record) {
    const coldEffect = record.effects.find(
      e => e.type === 'cold' && !isExpired(e, now)
    )
    if (coldEffect) {
      const meta = STATUS_META['cold']
      displays.push({
        type: 'cold',
        label: meta.label,
        emoji: meta.emoji,
        color: meta.color,
        description: meta.description,
        remainingDays: getColdRemainingDays(record, now) ?? undefined,
      })
    }
  }

  // 愉悦（从天气派生）
  if (weather && isPleasureWeather(weather)) {
    const meta = STATUS_META['pleasure']
    displays.push({
      type: 'pleasure',
      label: meta.label,
      emoji: meta.emoji,
      color: meta.color,
      description: meta.description,
    })
  }

  // 无状态 → 正常
  if (displays.length === 0) {
    const meta = STATUS_META['normal']
    displays.push({
      type: 'normal',
      label: meta.label,
      emoji: meta.emoji,
      color: meta.color,
      description: meta.description,
    })
  }

  return displays
}
