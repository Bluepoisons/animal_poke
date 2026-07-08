# [M2] 状态系统实现计划 — Issue #39

> **验收标准**：感冒判定准确可追溯（永久损伤已下线）
>
> **设计文档来源**：`游戏开发计划.md` 5.4 状态系统 + 3.5 天气系统
>
> **技术栈**：React 18 + Vite 6 + TypeScript 5.6 + vitest，无额外依赖
>
> **现有基础**：
> - `WeatherContext`（#38 计划已出）提供 `getColdRisk()` → `{ isRisky, probability, description }` 和 `getBattleModifier()` → `WeatherType`
> - `ShopContext` 已定义 `cold_medicine` 道具（200 金币，category: 'cure'），`useItem()` / `buyItem()` / `getItemCount()` 可用
> - `BattleContext` 已有 `BattlePet.stats` / `baseStats`，`cardEntryToBattlePet()` 应用天气修正
> - `battle/constants.ts` 已有 `WEATHER_STAT_MODIFIER`（晴天 1.05）和 `WEATHER_ELEMENT_BONUS`
> - `CardEntry.id` 作为宠物唯一标识，贯穿图鉴 / 战斗 / 状态系统

---

## 0. 设计要点与关键决策

### 0.1 永久损伤下线说明

设计文档 5.4 描述了感冒自然恢复后 2%~5% 概率触发永久损伤（全属性 -5%），但里程碑验收标准 7.3 明确标注「永久损伤已下线」。

**决策**：实现完整永久损伤逻辑代码（含纯函数 + 测试），但通过常量 `PERMANENT_DAMAGE_ENABLED = false` 默认关闭。后续可通过修改一个常量值重新启用，无需重构。

### 0.2 愉悦状态归属

设计文档中愉悦（晴天全属性 +5%）的数值修正已由 `BattleContext` 的 `applyWeatherModifier(stats, 'sunny')`（倍率 1.05）实现。状态系统中愉悦仅作为 **UI 标签**展示，不重复计算数值修正。

**决策**：`StatusType` 枚举包含 `'pleasure'`，但 `getStatModifier()` 不计入愉悦倍率（已由天气系统处理）。愉悦状态由 `WeatherContext.today.weather === 'sunny'` 派生，不持久化。

### 0.3 感冒触发对象

| 场景 | 触发对象 | 说明 |
|------|---------|------|
| 捕获（CaptureScreen） | 新捕获的宠物 | 雨雪天捕获的野生动物自带感冒 |
| 战斗（BattleScreen） | 玩家选择的出战宠物 | 雨雪天出战后感冒 |

### 0.4 属性修正范围

设计文档说「全属性 -35%」。战斗系统中 `BattleStats` 有 6 项（hp/atk/def/spd/crit/eva）。现有 `applyWeatherModifier` 只修正 hp/atk/def/spd（不修正 crit/eva）。感冒修正 **也只修正 hp/atk/def/spd**，与天气修正确持一致，避免 crit/eva 百分比属性被过度削减。

### 0.5 Provider 嵌套顺序

```
LbsProvider
  └─ StaminaProvider
       └─ WeatherProvider          ← #38 提供 getColdRisk()
            └─ ShopProvider
                 └─ StatusProvider  ← #39（本 Issue）
                      └─ BattleProvider
                           └─ AppInner
```

- `StatusProvider` 在 `WeatherProvider` 内：读取今日天气派生愉悦标签
- `StatusProvider` 在 `ShopProvider` 内：`cureCold()` 可检查感冒药库存
- `BattleProvider` 在 `StatusProvider` 内：`selectPet()` 读取 `getStatModifier()` 应用到 BattlePet

---

## 1. 文件结构

```
frontend/src/
├── status/
│   ├── constants.ts         # 感冒持续天数、属性修正倍率、恢复概率、特性开关
│   ├── types.ts             # StatusType / StatusEffect / PetStatusRecord / StatusState / StatusAction 等
│   ├── logic.ts             # 纯函数：applyCold / checkExpired / getStatModifier / rollPermanentDamage 等
│   ├── logic.test.ts        # 纯函数单元测试（≥18 个用例）
│   ├── StatusContext.tsx    # React Context + useReducer + localStorage 持久化 + 定时恢复检查
│   └── useStatus.ts         # 自定义 Hook：封装 Context 消费
├── battle/
│   ├── logic.ts             # （修改）新增 applyStatusMultiplier() 函数
│   └── BattleContext.tsx    # （修改）selectPet() 中读取 StatusContext.getStatModifier() 应用到 BattlePet
├── components/
│   ├── DetailPopup.tsx      # （修改）显示状态徽章 + 感冒药治疗按钮
│   ├── BattlePetSelect.tsx  # （修改）显示状态徽章 + 修正后属性
│   └── CaptureScreen.tsx    # （修改）捕获成功后触发感冒判定
├── App.tsx                  # （修改）包裹 <StatusProvider> + 捕获成功后感冒判定
```

### 各文件职责

| 文件 | 职责 | 依赖 |
|------|------|------|
| `status/constants.ts` | 状态系统常量：感冒天数、修正倍率、恢复概率、永久损伤开关。纯数据 | 无 |
| `status/types.ts` | TypeScript 类型定义。纯类型 | `constants.ts` |
| `status/logic.ts` | 纯函数：创建感冒效果、检查过期、计算属性倍率、永久损伤判定。无副作用 | `constants.ts`, `types.ts` |
| `status/StatusContext.tsx` | 状态管理核心。Context + Reducer + localStorage + 定时恢复检查。与 WeatherContext 联动愉悦标签 | `constants`, `types`, `logic`, `WeatherContext`, `ShopContext` |
| `status/useStatus.ts` | 对外暴露的 Hook | `StatusContext`, `types` |
| `battle/logic.ts`（修改） | 新增 `applyStatusMultiplier()` 纯函数 | `battle/types.ts` |
| `battle/BattleContext.tsx`（修改） | `selectPet()` 中应用 status 倍率 | `status/useStatus` |
| `components/DetailPopup.tsx`（修改） | 状态展示 + 治疗入口 | `status/useStatus`, `shop/useShop` |
| `components/BattlePetSelect.tsx`（修改） | 状态徽章 + 修正属性展示 | `status/useStatus` |
| `components/CaptureScreen.tsx`（修改） | 捕获成功后感冒判定 | `status/useStatus`, `weather/useWeather` |
| `App.tsx`（修改） | Provider 嵌套 + 捕获成功后感冒判定回调 | `StatusContext` |

---

## 2. 类型定义 (`status/types.ts`)

### 2.1 状态类型枚举

```typescript
/**
 * 宠物状态类型
 * - normal:   默认正常状态
 * - cold:     感冒（全属性 -35%，持续 5 天）
 * - pleasure: 愉悦（全属性 +5%，晴天自动获得，不持久化）
 */
export type StatusType = 'normal' | 'cold' | 'pleasure'
```

### 2.2 状态来源

```typescript
/**
 * 状态触发来源
 * - weather: 天气触发（雨/雪天感冒）
 * - battle:  战斗触发
 * - capture: 捕获触发（新捕获宠物自带）
 * - item:    道具触发
 * - system:  系统自动（如定时恢复）
 */
export type StatusSource = 'weather' | 'battle' | 'capture' | 'item' | 'system'
```

### 2.3 单条状态效果

```typescript
/**
 * 一条状态效果实例
 */
export interface StatusEffect {
  /** 状态类型 */
  type: StatusType
  /** 触发来源 */
  source: StatusSource
  /** 生效时间戳（Unix ms） */
  startTime: number
  /** 持续天数 */
  durationDays: number
  /** 过期时间戳（Unix ms）= startTime + durationDays * DAY_MS */
  expiresAt: number
}
```

### 2.4 单只宠物的状态记录

```typescript
/**
 * 单只宠物的完整状态记录
 */
export interface PetStatusRecord {
  /** 宠物 ID（对应 CardEntry.id） */
  petId: string
  /** 当前活跃的状态效果列表（不含已过期） */
  effects: StatusEffect[]
  /**
   * 永久损伤倍率（累积值）
   * - 1.0 = 无损伤
   * - 0.95 = 一次永久损伤（-5%）
   * - 0.9025 = 两次永久损伤（-5% × 2）
   * 永久损伤下线时始终为 1.0
   */
  permanentDamageMultiplier: number
  /** 感冒历史次数（统计用，不影响逻辑） */
  coldCount: number
}
```

### 2.5 完整状态

```typescript
/**
 * 状态系统完整状态
 */
export interface StatusState {
  /** petId → 宠物状态记录 的映射 */
  records: Record<string, PetStatusRecord>
}
```

### 2.6 Reducer Action

```typescript
/**
 * Reducer Action 类型
 */
export type StatusAction =
  /** 为宠物施加感冒状态 */
  | { type: 'APPLY_COLD'; petId: string; source: StatusSource; now: number }
  /** 治愈宠物感冒（使用感冒药） */
  | { type: 'CURE_COLD'; petId: string }
  /** 为宠物施加愉悦状态 */
  | { type: 'APPLY_PLEASURE'; petId: string; now: number }
  /** 移除宠物愉悦状态 */
  | { type: 'REMOVE_PLEASURE'; petId: string }
  /** 检查并处理过期状态（自然恢复 + 永久损伤判定） */
  | { type: 'CHECK_RECOVERY'; now: number }
  /** 批量清理所有过期状态 */
  | { type: 'CLEAR_EXPIRED'; now: number }
  /** 移除某只宠物的所有状态（宠物被释放时） */
  | { type: 'CLEAR_PET'; petId: string }
  /** 加载持久化状态 */
  | { type: 'LOAD_STATE'; state: StatusState }
```

### 2.7 Context 暴露接口

```typescript
/**
 * 治疗结果
 */
export interface CureColdResult {
  success: boolean
  reason?: 'no_cold' | 'no_medicine'
}

/**
 * 感冒触发结果
 */
export interface ColdTriggerResult {
  triggered: boolean
  petId: string
}

/**
 * 状态展示信息（供 UI 使用）
 */
export interface StatusDisplay {
  /** 状态类型 */
  type: StatusType
  /** 中文名 */
  label: string
  /** emoji 图标 */
  emoji: string
  /** 颜色 CSS 变量名 */
  color: string
  /** 详细描述 */
  description: string
  /** 剩余天数（感冒用，愉悦为 null） */
  remainingDays?: number
}

/**
 * StatusContext 暴露给组件的接口
 */
export interface StatusContextValue {
  state: StatusState

  // ---- 查询函数 ----

  /** 获取宠物的所有活跃状态效果 */
  getPetEffects: (petId: string) => StatusEffect[]
  /** 获取宠物的状态展示信息列表（供 UI 渲染） */
  getPetStatusDisplay: (petId: string) => StatusDisplay[]
  /** 判断宠物是否有指定状态 */
  hasStatus: (petId: string, type: StatusType) => boolean
  /** 获取宠物的属性修正倍率（感冒 + 永久损伤，不含愉悦） */
  getStatModifier: (petId: string) => number
  /** 获取宠物永久损伤倍率 */
  getPermanentDamage: (petId: string) => number
  /** 获取感冒剩余天数 */
  getColdRemainingDays: (petId: string) => number | null

  // ---- 操作函数 ----

  /** 施加感冒状态 */
  applyCold: (petId: string, source: StatusSource) => void
  /** 治愈感冒（需有感冒药，内部调用 ShopContext.useItem） */
  cureCold: (petId: string) => CureColdResult
  /** 施加愉悦状态 */
  applyPleasure: (petId: string) => void
  /** 移除愉悦状态 */
  removePleasure: (petId: string) => void
  /** 检查并处理自然恢复（定时调用） */
  checkRecovery: () => void
}
```

---

## 3. 常量定义 (`status/constants.ts`)

```typescript
/** 一天的毫秒数 */
export const DAY_MS = 24 * 60 * 60 * 1000

// ===== 感冒参数 =====

/** 感冒持续天数 */
export const COLD_DURATION_DAYS = 5

/** 感冒属性修正倍率（全属性 -35%） */
export const COLD_STAT_MULTIPLIER = 0.65

// ===== 愉悦参数 =====

/** 愉悦属性修正倍率（全属性 +5%）— 仅供 UI 展示参考，实际修正由天气系统处理 */
export const PLEASURE_STAT_MULTIPLIER = 1.05

// ===== 永久损伤参数（已下线，代码保留） =====

/** 永久损伤功能开关（里程碑 7.3 标注「已下线」） */
export const PERMANENT_DAMAGE_ENABLED = false

/** 永久损伤最低概率（2%） */
export const PERMANENT_DAMAGE_MIN_RATE = 0.02

/** 永久损伤最高概率（5%） */
export const PERMANENT_DAMAGE_MAX_RATE = 0.05

/** 永久损伤属性修正倍率（全属性 -5%） */
export const PERMANENT_DAMAGE_MULTIPLIER = 0.95

// ===== 恢复参数 =====

/** 自然恢复完全恢复的最低概率（95%） */
export const FULL_RECOVERY_MIN_RATE = 0.95

/** 自然恢复完全恢复的最高概率（98%） */
export const FULL_RECOVERY_MAX_RATE = 0.98

// ===== 定时检查 =====

/** 恢复检查间隔（毫秒），每小时检查一次 */
export const RECOVERY_CHECK_INTERVAL_MS = 60 * 60 * 1000

// ===== 持久化 =====

/** localStorage 存储 key */
export const STATUS_STORAGE_KEY = 'animal_poke_status'

// ===== 状态展示元数据 =====

/**
 * 状态展示元数据表
 */
export const STATUS_META: Record<StatusType, {
  label: string
  emoji: string
  color: string
  description: string
}> = {
  normal: {
    label: '正常',
    emoji: '✅',
    color: 'var(--success)',
    description: '状态良好',
  },
  cold: {
    label: '感冒',
    emoji: '🤧',
    color: 'var(--warn)',
    description: '全属性 -35%',
  },
  pleasure: {
    label: '愉悦',
    emoji: '😊',
    color: 'var(--orange)',
    description: '全属性 +5%（晴天）',
  },
}
```

---

## 4. 纯逻辑函数 (`status/logic.ts`)

### 4.1 创建感冒效果

```typescript
import { COLD_DURATION_DAYS, DAY_MS } from './constants'
import type { StatusEffect, StatusSource } from './types'

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
```

### 4.2 创建愉悦效果

```typescript
import { DAY_MS } from './constants'

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
```

### 4.3 检查状态是否过期

```typescript
/**
 * 检查状态效果是否已过期
 * @param effect 状态效果
 * @param now 当前时间戳
 * @returns true = 已过期
 */
export function isExpired(effect: StatusEffect, now: number = Date.now()): boolean {
  return now >= effect.expiresAt
}
```

### 4.4 计算属性修正倍率

```typescript
import { COLD_STAT_MULTIPLIER, PERMANENT_DAMAGE_MULTIPLIER } from './constants'
import type { PetStatusRecord } from './types'

/**
 * 计算宠物的属性修正倍率（感冒 + 永久损伤，不含愉悦）
 *
 * 倍率叠加规则：
 *   最终倍率 = 感冒倍率 × 永久损伤倍率
 *
 * 示例：
 *   无状态 → 1.0
 *   仅感冒 → 0.65
 *   感冒 + 1次永久损伤 → 0.65 × 0.95 = 0.6175
 *   仅永久损伤 → 0.95
 *
 * @param record 宠物状态记录（可为 null/undefined）
 * @returns 属性修正倍率（0~1）
 */
export function getStatMultiplier(record: PetStatusRecord | null | undefined): number {
  if (!record) return 1.0

  let multiplier = 1.0

  // 感冒修正
  const hasCold = record.effects.some(e => e.type === 'cold' && !isExpired(e))
  if (hasCold) {
    multiplier *= COLD_STAT_MULTIPLIER
  }

  // 永久损伤修正
  if (record.permanentDamageMultiplier < 1.0) {
    multiplier *= record.permanentDamageMultiplier
  }

  return multiplier
}
```

### 4.5 检查过期并处理自然恢复

```typescript
import {
  COLD_DURATION_DAYS,
  DAY_MS,
  PERMANENT_DAMAGE_ENABLED,
  PERMANENT_DAMAGE_MIN_RATE,
  PERMANENT_DAMAGE_MAX_RATE,
  PERMANENT_DAMAGE_MULTIPLIER,
  FULL_RECOVERY_MIN_RATE,
  FULL_RECOVERY_MAX_RATE,
} from './constants'
import type { PetStatusRecord, StatusEffect } from './types'

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
 * 流程：
 *   1. 查找活跃的感冒效果
 *   2. 若已过期：
 *      a. 移除感冒效果
 *      b. 若 PERMANENT_DAMAGE_ENABLED，在 [0.02, 0.05] 区间随机判定永久损伤
 *      c. 若触发永久损伤，累积到 permanentDamageMultiplier
 *   3. 返回恢复结果
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
```

### 4.6 获取感冒剩余天数

```typescript
import { DAY_MS } from './constants'
import type { PetStatusRecord } from './types'

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
```

### 4.7 施加感冒到记录

```typescript
import type { PetStatusRecord, StatusSource } from './types'

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
```

### 4.8 治愈感冒

```typescript
import type { PetStatusRecord } from './types'

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
```

### 4.9 派生愉悦状态

```typescript
import type { WeatherType } from '../battle/types'

/**
 * 根据天气判断是否应显示愉悦状态
 * @param weather 当前天气类型
 * @returns true = 晴天 → 愉悦
 */
export function isPleasureWeather(weather: WeatherType): boolean {
  return weather === 'sunny'
}
```

### 4.10 清理过期效果

```typescript
import type { PetStatusRecord } from './types'

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
```

### 4.11 获取状态展示信息

```typescript
import { STATUS_META, COLD_DURATION_DAYS } from './constants'
import type { PetStatusRecord, StatusDisplay, StatusType } from './types'
import type { WeatherType } from '../battle/types'

/**
 * 获取宠物的状态展示信息列表（供 UI 渲染）
 *
 * 优先级：感冒 > 愉悦 > 正常
 * - 感冒：从持久化记录中读取
 * - 愉悦：从天气派生（晴天 → 愉悦）
 * - 正常：无活跃状态时显示
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
```

---

## 5. StatusContext 设计 (`status/StatusContext.tsx`)

### 5.1 初始状态与持久化

```typescript
import React, { createContext, useReducer, useEffect, useCallback, useMemo } from 'react'
import { useWeather } from '../weather/useWeather'
import { useShop } from '../shop/useShop'
import {
  STATUS_STORAGE_KEY,
  RECOVERY_CHECK_INTERVAL_MS,
} from './constants'
import type {
  StatusState, StatusAction, StatusContextValue,
  CureColdResult, StatusDisplay, StatusEffect, StatusSource,
} from './types'
import {
  applyColdToRecord,
  cureColdFromRecord,
  checkRecovery,
  clearExpiredEffects,
  getStatMultiplier,
  getColdRemainingDays,
  getStatusDisplay,
  isExpired,
} from './logic'

/** 默认初始状态 */
const initialState: StatusState = {
  records: {},
}

/** 从 localStorage 加载状态 */
function loadInitialState(): StatusState {
  try {
    const saved = localStorage.getItem(STATUS_STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as StatusState
      if (typeof parsed.records !== 'object' || parsed.records === null) {
        throw new Error('状态存档字段校验失败')
      }
      return parsed
    }
  } catch (e) {
    console.warn('加载状态存档失败，使用默认值:', e)
  }
  return initialState
}
```

### 5.2 Reducer

```typescript
function statusReducer(state: StatusState, action: StatusAction): StatusState {
  switch (action.type) {
    case 'APPLY_COLD': {
      const existing = state.records[action.petId] ?? null
      const { record, added } = applyColdToRecord(existing, action.source, action.now)
      if (!added) return state
      return {
        ...state,
        records: {
          ...state.records,
          [action.petId]: { ...record, petId: action.petId },
        },
      }
    }

    case 'CURE_COLD': {
      const existing = state.records[action.petId]
      if (!existing) return state
      const { record, cured } = cureColdFromRecord(existing)
      if (!cured || !record) return state
      return {
        ...state,
        records: {
          ...state.records,
          [action.petId]: record,
        },
      }
    }

    case 'APPLY_PLEASURE': {
      // 愉悦不持久化，仅作为 UI 派生状态
      // 此 action 为 no-op，愉悦由 getStatusDisplay 从天气派生
      return state
    }

    case 'REMOVE_PLEASURE': {
      return state
    }

    case 'CHECK_RECOVERY': {
      const newRecords: Record<string, PetStatusRecord> = {}
      let changed = false

      for (const [petId, record] of Object.entries(state.records)) {
        // 先清理非感冒过期效果
        const cleaned = clearExpiredEffects(record, action.now)
        // 再检查感冒恢复
        const result = checkRecovery(cleaned, action.now)
        if (result.expired || cleaned !== record) {
          changed = true
        }
        // 仅保留有活跃状态或有永久损伤的记录
        if (result.record.effects.length > 0 || result.record.permanentDamageMultiplier < 1.0) {
          newRecords[petId] = result.record
        }
      }

      return changed ? { records: newRecords } : state
    }

    case 'CLEAR_EXPIRED': {
      const newRecords: Record<string, PetStatusRecord> = {}
      let changed = false

      for (const [petId, record] of Object.entries(state.records)) {
        const cleaned = clearExpiredEffects(record, action.now)
        if (cleaned.effects.length !== record.effects.length) {
          changed = true
        }
        if (cleaned.effects.length > 0 || cleaned.permanentDamageMultiplier < 1.0) {
          newRecords[petId] = cleaned
        }
      }

      return changed ? { records: newRecords } : state
    }

    case 'CLEAR_PET': {
      if (!state.records[action.petId]) return state
      const newRecords = { ...state.records }
      delete newRecords[action.petId]
      return { records: newRecords }
    }

    case 'LOAD_STATE': {
      return action.state
    }

    default:
      return state
  }
}
```

### 5.3 Provider 实现

```typescript
export const StatusContext = createContext<StatusContextValue | null>(null)

export const StatusProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(statusReducer, undefined, loadInitialState)
  const weather = useWeather()
  const shop = useShop()

  // 当前天气（用于派生愉悦标签）
  const currentWeather = weather.getBattleModifier()

  // localStorage 持久化
  useEffect(() => {
    localStorage.setItem(STATUS_STORAGE_KEY, JSON.stringify(state))
  }, [state])

  // 定时恢复检查（每小时）
  useEffect(() => {
    const interval = setInterval(() => {
      dispatch({ type: 'CHECK_RECOVERY', now: Date.now() })
    }, RECOVERY_CHECK_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [])

  // 页面可见性恢复时检查
  useEffect(() => {
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        dispatch({ type: 'CHECK_RECOVERY', now: Date.now() })
      }
    }
    document.addEventListener('visibilitychange', handleVisibility)
    return () => document.removeEventListener('visibilitychange', handleVisibility)
  }, [])

  // ---- 查询函数 ----

  const getPetEffects = useCallback((petId: string): StatusEffect[] => {
    const record = state.records[petId]
    if (!record) return []
    return record.effects.filter(e => !isExpired(e))
  }, [state.records])

  const getPetStatusDisplay = useCallback((petId: string): StatusDisplay[] => {
    const record = state.records[petId] ?? null
    return getStatusDisplay(record, currentWeather)
  }, [state.records, currentWeather])

  const hasStatus = useCallback((petId: string, type: StatusType): boolean => {
    const effects = getPetEffects(petId)
    if (effects.some(e => e.type === type)) return true
    // 愉悦从天气派生
    if (type === 'pleasure' && currentWeather === 'sunny') return true
    return false
  }, [getPetEffects, currentWeather])

  const getStatModifier = useCallback((petId: string): number => {
    return getStatMultiplier(state.records[petId])
  }, [state.records])

  const getPermanentDamage = useCallback((petId: string): number => {
    return state.records[petId]?.permanentDamageMultiplier ?? 1.0
  }, [state.records])

  const getColdRemainingDaysFn = useCallback((petId: string): number | null => {
    return getColdRemainingDays(state.records[petId])
  }, [state.records])

  // ---- 操作函数 ----

  const applyCold = useCallback((petId: string, source: StatusSource) => {
    dispatch({ type: 'APPLY_COLD', petId, source, now: Date.now() })
  }, [])

  const cureCold = useCallback((petId: string): CureColdResult => {
    // 检查是否有感冒
    const record = state.records[petId]
    const hasCold = record?.effects.some(e => e.type === 'cold' && !isExpired(e))
    if (!hasCold) {
      return { success: false, reason: 'no_cold' }
    }

    // 检查是否有感冒药
    const medicineCount = shop.getItemCount('cold_medicine')
    if (medicineCount <= 0) {
      return { success: false, reason: 'no_medicine' }
    }

    // 使用感冒药
    const useResult = shop.useItem('cold_medicine')
    if (!useResult.success) {
      return { success: false, reason: 'no_medicine' }
    }

    // 治愈感冒
    dispatch({ type: 'CURE_COLD', petId })
    return { success: true }
  }, [state.records, shop])

  const applyPleasure = useCallback((_petId: string) => {
    // 愉悦不持久化，no-op
  }, [])

  const removePleasure = useCallback((_petId: string) => {
    // 愉悦不持久化，no-op
  }, [])

  const checkRecoveryFn = useCallback(() => {
    dispatch({ type: 'CHECK_RECOVERY', now: Date.now() })
  }, [])

  const value = useMemo<StatusContextValue>(() => ({
    state,
    getPetEffects,
    getPetStatusDisplay,
    hasStatus,
    getStatModifier,
    getPermanentDamage,
    getColdRemainingDays: getColdRemainingDaysFn,
    applyCold,
    cureCold,
    applyPleasure,
    removePleasure,
    checkRecovery: checkRecoveryFn,
  }), [
    state, getPetEffects, getPetStatusDisplay, hasStatus,
    getStatModifier, getPermanentDamage, getColdRemainingDaysFn,
    applyCold, cureCold, applyPleasure, removePleasure, checkRecoveryFn,
  ])

  return (
    <StatusContext.Provider value={value}>
      {children}
    </StatusContext.Provider>
  )
}
```

### 5.4 关键设计决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 感冒触发 | `applyCold(petId, source)` 立即写入 records | 调用方在捕获/战斗完成后调用 |
| 感冒治愈 | `cureCold(petId)` 内部调用 `shop.useItem` | 封装完整流程，UI 只需一次调用 |
| 愉悦状态 | 不持久化，从天气派生 | 天气系统已处理 +5% 数值，愉悦仅为 UI 标签 |
| 恢复检查 | 定时（每小时）+ 页面可见性恢复 | 覆盖离线场景，app 重启时 loadInitialState 后立即检查 |
| 永久损伤 | 代码完整但 `PERMANENT_DAMAGE_ENABLED=false` | 里程碑已下线，保留代码便于后续启用 |
| 持久化策略 | localStorage 全量序列化 | 与 StaminaContext / ShopContext 一致 |
| 记录清理 | `CHECK_RECOVERY` 时移除空记录 | 避免 records 无限增长 |

---

## 6. battle/logic.ts 修改

### 6.1 新增 `applyStatusMultiplier` 函数

在 `frontend/src/battle/logic.ts` 中新增：

```typescript
/**
 * 应用状态修正倍率到战斗属性
 * 与 applyWeatherModifier 一致，仅修正 hp/atk/def/spd，不修正 crit/eva
 *
 * @param stats 原始属性（已含天气修正）
 * @param multiplier 状态修正倍率（1.0 = 无修正，0.65 = 感冒 -35%）
 * @returns 修正后的属性
 */
export function applyStatusMultiplier(stats: BattleStats, multiplier: number): BattleStats {
  if (multiplier === 1.0) return stats
  return {
    hp:   Math.round(stats.hp   * multiplier),
    atk:  Math.round(stats.atk  * multiplier),
    def:  Math.round(stats.def  * multiplier),
    spd:  Math.round(stats.spd  * multiplier),
    crit: stats.crit,
    eva:  stats.eva,
  }
}
```

### 6.2 BattleContext.tsx 修改

`selectPet` 方法中应用状态修正：

```typescript
// 修改前：
const selectPet = useCallback((entry: CardEntry): boolean => {
  // ...
  const pet = cardEntryToBattlePet(entry, state.weather)
  dispatch({ type: 'SELECT_PET', pet })
  return true
}, [stamina, state.weather])

// 修改后：
const selectPet = useCallback((entry: CardEntry): boolean => {
  // ...
  const pet = cardEntryToBattlePet(entry, state.weather)
  // 应用状态修正（感冒 -35% 等）
  const statusMultiplier = status.getStatModifier(entry.id)
  if (statusMultiplier !== 1.0) {
    pet.stats = applyStatusMultiplier(pet.stats, statusMultiplier)
    pet.currentHp = pet.stats.hp
  }
  dispatch({ type: 'SELECT_PET', pet })
  return true
}, [stamina, state.weather, status])
```

`BattleProvider` 中新增 `useStatus()` 依赖：

```typescript
import { useStatus } from '../status/useStatus'
// ...
const status = useStatus()
```

---

## 7. UI 集成

### 7.1 DetailPopup 状态展示 + 治疗按钮 (`components/DetailPopup.tsx`)

```typescript
import { useStatus } from '../status/useStatus'

// 在组件内：
const statusCtx = useStatus()
const statusDisplays = statusCtx.getPetStatusDisplay(entry.id)
const hasCold = statusCtx.hasStatus(entry.id, 'cold')
const coldRemaining = statusCtx.getColdRemainingDays(entry.id)

// 在 metaList 后新增状态区域：
<div style={styles.statusSection}>
  <div style={styles.metaRow}>
    <span>📊</span>
    <span style={styles.metaLabel}>状态</span>
  </div>
  {statusDisplays.map((s, i) => (
    <div key={i} style={{ ...styles.statusBadge, color: s.color }}>
      <span style={{ fontSize: 16 }}>{s.emoji}</span>
      <span style={{ fontWeight: 600 }}>{s.label}</span>
      <span style={{ fontSize: 10, color: 'var(--ink-3)' }}>{s.description}</span>
      {s.remainingDays && (
        <span style={{ fontSize: 10, color: 'var(--warn)' }}>
          剩余 {s.remainingDays} 天
        </span>
      )}
    </div>
  ))}
</div>

// 感冒治疗按钮（仅感冒时显示）：
{hasCold && (
  <button
    className="btn btn-primary"
    style={styles.cureBtn}
    onClick={() => {
      const result = statusCtx.cureCold(entry.id)
      if (!result.success) {
        // 提示：无感冒药 → 引导去商店购买
        alert(result.reason === 'no_medicine'
          ? '没有感冒药！请去商店购买（200 金币）'
          : '宠物未感冒')
      }
    }}
  >
    💊 使用感冒药治疗
  </button>
)}
```

### 7.2 BattlePetSelect 状态徽章 (`components/BattlePetSelect.tsx`)

```typescript
import { useStatus } from '../status/useStatus'

// 在组件内：
const statusCtx = useStatus()

// 在每只宠物卡片中，名字旁添加状态徽章：
{statusCtx.getPetStatusDisplay(entry.id).map((s, i) => {
  if (s.type === 'normal') return null
  return (
    <span
      key={i}
      style={{
        fontSize: 10,
        fontWeight: 600,
        color: s.color,
        background: 'var(--orange-50)',
        borderRadius: 8,
        padding: '2px 6px',
      }}
    >
      {s.emoji} {s.label}
      {s.remainingDays ? ` ${s.remainingDays}天` : ''}
    </span>
  )
})}

// 修正后的属性展示：
{pet && (() => {
  const mod = statusCtx.getStatModifier(entry.id)
  const modifiedPet = mod !== 1.0
    ? { ...pet, stats: applyStatusMultiplier(pet.stats, mod) }
    : pet
  return (
    <div style={{ fontSize: 11, color: 'var(--ink-3)', marginTop: 3, display: 'flex', gap: 8 }}>
      <span>HP {modifiedPet.stats.hp}{mod !== 1.0 && <span style={{color:'var(--warn)'}}>↓</span>}</span>
      <span>ATK {modifiedPet.stats.atk}{mod !== 1.0 && <span style={{color:'var(--warn)'}}>↓</span>}</span>
      <span>DEF {modifiedPet.stats.def}{mod !== 1.0 && <span style={{color:'var(--warn)'}}>↓</span>}</span>
      <span>SPD {modifiedPet.stats.spd}{mod !== 1.0 && <span style={{color:'var(--warn)'}}>↓</span>}</span>
    </div>
  )
})()}
```

### 7.3 CaptureScreen 感冒触发 (`components/CaptureScreen.tsx`)

在捕获成功后触发感冒判定。修改 `App.tsx` 中的 `handleCaptureSuccess`：

```typescript
import { useStatus } from './status/useStatus'
import { useWeather } from './weather/useWeather'

// AppInner 内：
const statusCtx = useStatus()
const weatherCtx = useWeather()

const handleCaptureSuccess = useCallback((entry: CardEntry) => {
  addAnimal(entry)
  addCapture(1)
  const goldDrop = Math.floor(Math.random() * 41) + 10
  addGold(goldDrop)

  // 感冒判定：雨雪天捕获的宠物有概率自带感冒
  const coldRisk = weatherCtx.getColdRisk()
  if (coldRisk.isRisky && Math.random() < coldRisk.probability) {
    statusCtx.applyCold(entry.id, 'capture')
    // 可选：显示提示「新捕获的宠物感冒了！」
  }

  setPendingPhoto(null)
  setActiveTab('collection')
}, [addAnimal, addCapture, addGold, statusCtx, weatherCtx])
```

战斗结束后的感冒判定在 `BattleContext.finishBattle` 或 `BattleScreen` 中处理：

```typescript
// BattleContext.tsx — finishBattle 中新增：
const finishBattle = useCallback(() => {
  if (state.rewards) {
    stamina.addGold(state.rewards.gold)
    if (state.rewards.droppedItem) {
      shop.addItem(state.rewards.droppedItem as any)
    }
  }

  // 感冒判定：雨雪天出战宠物有概率感冒
  if (state.playerPet) {
    const coldRisk = weather.getColdRisk()
    if (coldRisk.isRisky && Math.random() < coldRisk.probability) {
      status.applyCold(state.playerPet.id, 'battle')
    }
  }

  dispatch({ type: 'RESET' })
}, [state.rewards, state.playerPet, stamina, shop, weather, status])
```

### 7.4 App.tsx Provider 嵌套

```typescript
const App: React.FC = () => {
  return (
    <LbsProvider>
      <StaminaProvider>
        <WeatherProvider>
          <ShopProvider>
            <StatusProvider>
              <BattleProvider>
                <AppInner />
              </BattleProvider>
            </StatusProvider>
          </ShopProvider>
        </WeatherProvider>
      </StaminaProvider>
    </LbsProvider>
  )
}
```

---

## 8. 感冒完整流程图

```
┌─────────────────────────────────────────────────────────┐
│                    感冒生命周期                          │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  捕获/战斗完成                                           │
│       │                                                 │
│       ▼                                                 │
│  WeatherContext.getColdRisk()                           │
│       │                                                 │
│       ├─ isRisky: false → 结束（无不健康状况）            │
│       └─ isRisky: true (rainy/snowy)                    │
│            │                                            │
│            ▼                                            │
│       Math.random() < probability?                      │
│            ├─ false → 结束                               │
│            └─ true → StatusContext.applyCold(petId)     │
│                 │                                       │
│                 ▼                                       │
│            ┌─ 感冒状态生效 ──────────────────────┐       │
│            │  · type: 'cold'                     │       │
│            │  · startTime: now                   │       │
│            │  · durationDays: 5                  │       │
│            │  · expiresAt: now + 5 * DAY_MS      │       │
│            │  · 全属性 -35%（getStatModifier=0.65）│      │
│            └─────────────────────────────────────┘       │
│                 │                                       │
│       ┌────────┴────────────────────┐                   │
│       ▼                             ▼                   │
│  使用感冒药                      等待 5 天自然恢复        │
│  (DetailPopup 治疗)              (CHECK_RECOVERY)        │
│       │                             │                   │
│       ▼                             ▼                   │
│  cureCold(petId)               checkRecovery(record)    │
│  · shop.useItem('cold_medicine')  · 移除感冒效果          │
│  · dispatch CURE_COLD            · PERMANENT_DAMAGE_     │
│  · 立即恢复，无永久损伤              ENABLED=false →       │
│  · 不触发永久损伤判定                不判定永久损伤         │
│       │                             │                   │
│       ▼                             ▼                   │
│    恢复正常                      完全恢复（100%）         │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## 9. 测试用例 (`status/logic.test.ts`)

### 9.1 测试用例列表（共 20 个）

| # | 测试用例 | 测试函数 | 预期结果 |
|---|---------|---------|---------|
| 1 | createColdEffect 创建正确字段 | `createColdEffect` | type='cold', durationDays=5, expiresAt=now+5天 |
| 2 | createColdEffect 不同 source | `createColdEffect` | source 字段正确传递 |
| 3 | createPleasureEffect 当日有效 | `createPleasureEffect` | expiresAt 为当天 23:59:59 |
| 4 | isExpired 未过期返回 false | `isExpired` | now < expiresAt → false |
| 5 | isExpired 已过期返回 true | `isExpired` | now >= expiresAt → true |
| 6 | isExpired 恰好过期时间点 | `isExpired` | now === expiresAt → true |
| 7 | getStatMultiplier 无记录返回 1.0 | `getStatMultiplier` | null → 1.0 |
| 8 | getStatMultiplier 仅有感冒返回 0.65 | `getStatMultiplier` | 有 cold effect → 0.65 |
| 9 | getStatMultiplier 感冒+永久损伤叠加 | `getStatMultiplier` | 0.65 × 0.95 = 0.6175 |
| 10 | getStatMultiplier 过期感冒不计入 | `getStatMultiplier` | 过期 cold → 1.0 |
| 11 | getColdRemainingDays 无感冒返回 null | `getColdRemainingDays` | null → null |
| 12 | getColdRemainingDays 5天感冒返回 5 | `getColdRemainingDays` | 刚创建 → 5 |
| 13 | getColdRemainingDays 过期返回 null | `getColdRemainingDays` | now > expiresAt → null |
| 14 | applyColdToRecord 新记录添加成功 | `applyColdToRecord` | added=true, coldCount=1 |
| 15 | applyColdToRecord 已有感冒不重复 | `applyColdToRecord` | added=false |
| 16 | cureColdFromRecord 移除感冒 | `cureColdFromRecord` | cured=true, effects 无 cold |
| 17 | cureColdFromRecord 无感冒返回 false | `cureColdFromRecord` | cured=false |
| 18 | checkRecovery 未过期不处理 | `checkRecovery` | expired=false |
| 19 | checkRecovery 过期移除感冒 | `checkRecovery` | expired=true, effects 无 cold |
| 20 | isPleasureWeather 晴天返回 true | `isPleasureWeather` | 'sunny' → true, 'rainy' → false |

### 9.2 测试代码骨架

```typescript
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
    expect(getStatMultiplier(record)).toBe(COLD_STAT_MULTIPLIER)
  })

  it('感冒+永久损伤叠加', () => {
    const record = makeColdRecord(NOW)
    record.permanentDamageMultiplier = 0.95
    expect(getStatMultiplier(record)).toBeCloseTo(COLD_STAT_MULTIPLIER * 0.95, 5)
  })

  it('过期感冒不计入', () => {
    const record = makeColdRecord(NOW)
    // 过期时间已过
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
    expect(record.coldCount).toBe(1) // 不递增
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
```

---

## 10. 实现步骤

| 步骤 | 任务 | 预估产出 | 依赖 |
|------|------|---------|------|
| Step 1 | 创建 `status/constants.ts` | 常量定义 + STATUS_META 元数据表 | 无 |
| Step 2 | 创建 `status/types.ts` | 全部 TypeScript 类型定义 | constants.ts |
| Step 3 | 创建 `status/logic.ts`（纯函数，先写测试） | 11 个纯函数 | constants.ts, types.ts |
| Step 4 | 创建 `status/logic.test.ts`，确保全部通过 | 20+ 个测试用例，绿色通过 | logic.ts |
| Step 5 | 创建 `status/StatusContext.tsx` | Context + Reducer + Provider + 持久化 | constants, types, logic, WeatherContext, ShopContext |
| Step 6 | 创建 `status/useStatus.ts` | 自定义 Hook | StatusContext |
| Step 7 | 修改 `battle/logic.ts`：新增 `applyStatusMultiplier()` | 1 个纯函数 | battle/types.ts |
| Step 8 | 修改 `battle/BattleContext.tsx`：`selectPet()` 应用状态修正 + `finishBattle()` 感冒判定 | 状态修正集成 | useStatus, applyStatusMultiplier |
| Step 9 | 修改 `App.tsx`：Provider 嵌套 + 捕获后感冒判定 | StatusProvider 包裹 | StatusContext |
| Step 10 | 修改 `components/DetailPopup.tsx`：状态展示 + 治疗按钮 | 状态徽章 + 感冒药按钮 | useStatus |
| Step 11 | 修改 `components/BattlePetSelect.tsx`：状态徽章 + 修正属性 | 状态徽章 + 属性预览 | useStatus |
| Step 12 | 修改 `components/CaptureScreen.tsx`：捕获成功感冒判定（如由 App.tsx 处理则跳过） | 感冒触发 | useStatus, useWeather |
| Step 13 | 运行全部测试 `npx vitest run` | 确保无回归 | 全部 |
| Step 14 | 手动测试：捕获→感冒→治疗→自然恢复全流程 | 端到端验证 | 全部 |

---

## 11. 验收标准对照

| # | 验收标准 | 实现情况 | 验收方式 |
|---|---------|---------|---------|
| 1 | 感冒判定准确可追溯 | `applyColdToRecord` 记录 startTime/source/duration，持久化到 localStorage | 单测 + 手动验证 |
| 2 | 雨 8% / 雪 6% 概率触发 | 由 #38 `getColdRisk()` 提供概率，`App.tsx` / `BattleContext` 执行随机判定 | 联调验证 |
| 3 | 感冒效果：全属性 -35% | `COLD_STAT_MULTIPLIER = 0.65`，`getStatMultiplier()` 返回 0.65，`applyStatusMultiplier` 应用到 BattlePet | 单测 + 战斗属性对比 |
| 4 | 感冒持续 5 天 | `COLD_DURATION_DAYS = 5`，`createColdEffect` 计算 expiresAt | 单测 |
| 5 | 感冒药 200 金币立即解除 | `cureCold()` 调用 `shop.useItem('cold_medicine')` + dispatch CURE_COLD | 手动验证 + DetailPopup 治疗按钮 |
| 6 | 5 天自然恢复 | `checkRecovery()` 检查过期 + 移除效果，定时每小时检查 + 页面可见性恢复检查 | 单测 + 模拟时间验证 |
| 7 | 永久损伤已下线 | `PERMANENT_DAMAGE_ENABLED = false`，`checkRecovery` 中跳过判定 | 单测验证 `permanentDamageTriggered=false` |
| 8 | 状态追踪：开始时间、持续时间、类型、来源 | `StatusEffect` 接口包含 startTime/durationDays/type/source/expiresAt | 类型检查 + 单测 |
| 9 | 状态在 UI 展示（DetailPopup） | `getPetStatusDisplay()` 返回展示信息，DetailPopup 渲染徽章 + 剩余天数 + 治疗按钮 | 手动验证 |
| 10 | 状态在战斗选宠展示（BattlePetSelect） | 状态徽章 + 修正后属性预览（↓ 标记） | 手动验证 |
| 11 | 愉悦状态（晴天 +5%） | `isPleasureWeather('sunny')` → UI 显示愉悦标签；数值修正由 `WEATHER_STAT_MODIFIER['sunny']=1.05` 已实现 | 手动验证 |
| 12 | 感冒判定可追溯 | localStorage 持久化 + coldCount 统计 + source 字段记录触发来源 | 控制台检查 localStorage |

---

## 12. 依赖关系图

```
Issue #35 物种系统 ✅ (已完成，CardEntry.id)
Issue #36 LBS 系统 ✅ (已完成，cityName)
Issue #37 战斗系统 ✅ (已完成，BattlePet/BattleStats)
Issue #38 天气系统 📋 (计划已出， getColdRisk() / getBattleModifier())
    │
    │  依赖：#38 提供 ColdCheckResult 接口
    │  依赖：#37 BattlePet.stats / cardEntryToBattlePet
    │  依赖：ShopContext cold_medicine 道具（已定义）
    │
    ▼
Issue #39 状态系统 (本 Issue)
    │
    │  产出：StatusContext / useStatus / getStatModifier / applyCold / cureCold
    │
    ▼
后续：捕获模块集成感冒判定 / 战斗模块集成感冒判定 / 图鉴页状态展示
```

### 前置依赖

| 依赖 | 来源 | 状态 | 说明 |
|------|------|------|------|
| `WeatherContext.getColdRisk()` | #38 | 计划已出 | 返回 `{ isRisky, probability, description }` |
| `WeatherContext.getBattleModifier()` | #38 | 计划已出 | 返回当前 `WeatherType` |
| `ShopContext.getItemCount('cold_medicine')` | #34 | ✅ 已实现 | 查询感冒药库存 |
| `ShopContext.useItem('cold_medicine')` | #34 | ✅ 已实现 | 消耗感冒药 |
| `BattlePet.stats` / `baseStats` | #37 | ✅ 已实现 | 战斗属性结构 |
| `cardEntryToBattlePet()` | #37 | ✅ 已实现 | CardEntry → BattlePet 转换 |
| `CardEntry.id` | #35 | ✅ 已实现 | 宠物唯一标识 |

---

## 13. 风险与注意事项

| 风险 | 缓解措施 |
|------|---------|
| **#38 天气系统尚未实现代码** | 本计划假设 #38 已提供 `getColdRisk()` / `getBattleModifier()` 接口。若 #38 尚未实现，需先完成 #38 或临时 mock 这两个函数 |
| **感冒触发需 WeatherContext** | CaptureScreen / BattleContext 中调用 `weather.getColdRisk()`，需确保 WeatherProvider 已包裹 |
| **Provider 嵌套层次深** | 6 层 Provider 嵌套（Lbs > Stamina > Weather > Shop > Status > Battle），需确保顺序正确，StatusProvider 在 WeatherProvider 和 ShopProvider 内 |
| **持久化数据迁移** | 若后续 StatusEffect 结构变化，需处理 localStorage 旧数据兼容。`loadInitialState` 中已做基本字段校验 |
| **离线恢复 edge case** | 玩家离线 5 天以上，app 重启时 `CHECK_RECOVERY` 应立即执行。当前在 `useEffect` 页面可见性恢复时触发，但首次加载未触发 — **建议在 Provider 初始化时也 dispatch 一次 CHECK_RECOVERY** |
| **愉悦状态不持久化** | 愉悦从天气派生，刷新页面后若天气仍为晴天则自动恢复。若天气数据未加载（WeatherContext.status='idle'），愉悦标签不显示，不影响逻辑 |
| **多个宠物同时感冒** | records 为 Map 结构，支持多宠物独立追踪。性能无瓶颈（宠物数量有限） |
| **战斗中感冒触发时机** | `finishBattle` 中触发感冒，但此时 BattlePet 已重置。需在 `dispatch RESET` 前读取 `state.playerPet.id` |
| **永久损伤下线但代码保留** | `PERMANENT_DAMAGE_ENABLED = false` 确保不触发。测试中验证 `permanentDamageTriggered` 始终为 false |
| **感冒药购买流程** | `cureCold()` 内部调用 `shop.useItem`，需先有感冒药。若无药，返回 `{ success: false, reason: 'no_medicine' }`，UI 引导去商店 |

---

## 14. 补充：首次加载恢复检查

为确保离线期间过期的感冒在 app 重启时立即处理，在 `StatusProvider` 初始化时追加一次恢复检查：

```typescript
export const StatusProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(statusReducer, undefined, loadInitialState)

  // ... 其他 hooks ...

  // 首次加载时检查恢复（处理离线期间过期的感冒）
  useEffect(() => {
    dispatch({ type: 'CHECK_RECOVERY', now: Date.now() })
  }, []) // 仅首次加载执行一次

  // ... rest ...
}
```

---

*计划基于设计文档 v1.4 (2026-07-08) 与现有代码基（React 18 + Vite 6 + TypeScript 5.6）编写。依赖 #38 天气系统接口（`getColdRisk` / `getBattleModifier`），需 #38 实现后方可联调。*
