# [M2] 每日签到（连续签到 + 断签处理）实现计划 — Issue #42

> **验收标准**：连续签到 + 断签处理合理
>
> **设计文档来源**：`游戏开发计划.md` 6.4 每日签到 + 6.2 各模块职责（每日签到：连续签到金币奖励，断签处理合理）
>
> **技术栈**：React 18 + Vite 6 + TypeScript 5.6 + vitest，无额外依赖
>
> **现有基础**：
> - `ShopContext`（#34）已有基础签到：`checkIn()` 方法、`CheckInState`（`streak` / `lastCheckInDate`）、`getCheckInReward()` 纯函数（含断签重置 + 周期循环）
> - `shop/constants.ts` 已有 `CHECK_IN_REWARDS = [10, 20, 30, 50, 80, 120, 200]`（**需对齐设计文档数值**）
> - `shop/constants.ts` 已有 `CHECK_IN_DAY7_BONUS_ITEM = 'toy_ball'`、`CHECK_IN_CYCLE_DAYS = 7`
> - `StaminaContext`（#31）管理 `gold` / `level` / `exp`（#41 计划引入），提供 `addGold()` / `addExp()`
> - `StoreScreen.tsx` 已有内联签到面板（7 格网格 + 签到按钮），**需升级为独立 Modal 组件**
> - `EconomyContext`（#40 计划）追踪产出/消耗，签到金币产出需调用 `trackEarn()`
> - `AchievementContext`（#41 计划）检查签到类成就

---

## 0. 设计要点与关键决策

### 0.1 现状分析

| 维度 | 现有实现 | Issue #42 目标 |
|------|---------|---------------|
| 奖励数值 | `[10, 20, 30, 50, 80, 120, 200]` | 对齐设计文档 `[20, 30, 40, 50, 60, 80, 150]` |
| 连续签到 | `streak` 字段，`getCheckInReward()` 含断签判断 | 保留逻辑，增强为独立纯函数 |
| 断签处理 | `lastCheckInDate !== yesterday` → 重置为第 1 天 | 保留，新增「断签提示」UI |
| 周期循环 | `streak > 7 → 1` | 保留 |
| 第 7 天奖励 | 固定送 `toy_ball` | 保留，UI 展示更醒目 |
| 经验值奖励 | 无 | 新增签到经验值（+15 XP，对齐 #41 XP 表） |
| 签到状态 | `streak` + `lastCheckInDate`（2 字段） | 扩展为 `totalCheckIns` + `maxStreak` + `lastBreakDate`（成就/统计用） |
| UI | StoreScreen 内联面板（7 格 + 按钮） | 独立 `CheckInModal` 全屏弹窗 + 日历视图 + 奖励预览 |
| 断签检测时机 | 仅签到时判断 | 新增 App 启动时自动检测并提示 |
| 经济追踪 | 无 | 对接 `EconomyContext.trackEarn()`（#40） |
| 成就联动 | 无 | 对接 `AchievementContext`（#41）签到成就检查 |

### 0.2 架构决策：增强 ShopContext，不新建 CheckInContext

**为什么不在 ShopContext 之外新建 CheckInContext？**
- 签到状态 `checkIn` 已嵌入 `ShopState`，且签到逻辑依赖 `stamina.addGold()` / `stamina.addExp()`
- 签到与商店共享 localStorage 存储 key（`animal_poke_shop`），拆分将导致两个 Context 争抢同一存档
- 签到逻辑量不足以支撑独立 Context（核心纯函数仅 4 个）
- 现有 `ShopContext.checkIn()` 已被 `StoreScreen` 消费，拆分需大面积改动

**决策**：在 `ShopContext` 内增强签到状态与逻辑，将 UI 拆分为独立的 `CheckInModal` 组件。

### 0.3 奖励表对齐设计文档

设计文档 6.4 每日签到表：

| 连续签到天数 | 金币奖励 | 额外奖励 | 经验值 |
|------------|---------|---------|--------|
| 第 1 天 | 20 | — | 15 |
| 第 2 天 | 30 | — | 15 |
| 第 3 天 | 40 | — | 15 |
| 第 4 天 | 50 | — | 15 |
| 第 5 天 | 60 | — | 15 |
| 第 6 天 | 80 | — | 15 |
| 第 7 天（满签）| 150 | 随机道具 ×1（MVP 固定送玩具球）| 30 |
| 断签 | 重置为第 1 天 | — | — |

> **经验值**：每日签到固定 +15 XP，第 7 天满签 +30 XP（对齐 #41 XP 表「签到 15 XP」）。

### 0.4 签到状态扩展

现有 `CheckInState`：
```typescript
interface CheckInState {
  streak: number       // 当前连续签到天数
  lastCheckInDate: string  // 'YYYY-MM-DD'
}
```

扩展为：
```typescript
interface CheckInState {
  streak: number           // 当前连续签到天数（0 = 未签到）
  lastCheckInDate: string  // 上次签到日期 'YYYY-MM-DD'
  totalCheckIns: number    // 累计签到总次数（成就统计用）
  maxStreak: number        // 历史最高连续签到天数（成就统计用）
  lastBreakDate: string    // 上次断签日期 'YYYY-MM-DD'（UI 提示用，空字符串表示从未断签）
}
```

> 新增字段为**增量扩展**，旧存档加载时缺失字段自动补默认值（0 / 空字符串）。

### 0.5 Provider 嵌套关系（含 #40 / #41 预留）

```
StaminaProvider              ← 金币 / 体力 / 等级 / exp 持有者
  └─ EconomyProvider         ← #40 追踪层（签到产出需调 trackEarn）
       └─ WeatherProvider
            └─ ShopProvider   ← 本 Issue：签到状态 + 道具背包
                 └─ StatusProvider
                      └─ AchievementProvider  ← #41 成就检查
                           └─ BattleProvider
                                └─ AppInner
```

- `ShopProvider` 内签到完成后调用 `economy.trackEarn(reward, 'checkin')`
- `ShopProvider` 内签到完成后调用 `achievement.checkAchievements('checkin')`（#41 预留）
- 若 `EconomyContext` / `AchievementContext` 尚未实现，通过可选回调注入，不硬依赖

---

## 1. 文件结构

```
frontend/src/
├── shop/
│   ├── constants.ts          # （修改）签到奖励表对齐设计文档 + 新增经验值常量
│   ├── types.ts              # （修改）扩展 CheckInState + 新增 CheckInReward 类型
│   ├── logic.ts              # （修改）拆分纯函数：canCheckIn / calculateReward / checkStreakBreak / getStreakInfo
│   ├── logic.test.ts         # （修改）新增 ≥12 个签到相关测试用例
│   ├── ShopContext.tsx       # （修改）增强 checkIn() 逻辑 + 新增 getCheckInStatus()
│   └── useShop.ts            # （无修改）
├── components/
│   ├── CheckInModal.tsx      # （新增）签到弹窗：7 天日历视图 + 当前连签 + 奖励预览 + 断签提示
│   ├── CheckInModal.test.tsx # （新增）弹窗渲染测试
│   └── StoreScreen.tsx      # （修改）签到面板改为打开 CheckInModal 的入口按钮
└── App.tsx                   # （无修改，Provider 嵌套不变）
```

### 各文件职责

| 文件 | 职责 | 修改类型 |
|------|------|---------|
| `shop/constants.ts` | 签到奖励表对齐设计文档数值 + 新增签到经验值常量 | 修改 |
| `shop/types.ts` | 扩展 `CheckInState`（+4 字段），新增 `CheckInReward` / `CheckInStatus` 类型 | 修改 |
| `shop/logic.ts` | 拆分签到纯函数为 `canCheckIn` / `calculateReward` / `checkStreakBreak` / `getStreakInfo` | 修改 |
| `shop/logic.test.ts` | 新增 ≥12 个签到测试用例，覆盖断签/周期/边界 | 修改 |
| `shop/ShopContext.tsx` | 增强 `checkIn()` 发放经验值，新增 `getCheckInStatus()` 供 UI 读取 | 修改 |
| `components/CheckInModal.tsx` | 签到弹窗组件：日历 + 奖励预览 + 断签提示 + 签到动画 | 新增 |
| `components/CheckInModal.test.tsx` | 弹窗渲染 + 交互测试 | 新增 |
| `components/StoreScreen.tsx` | 签到面板替换为「点击打开 Modal」入口 | 修改 |

---

## 2. 类型定义 (`types.ts`)

### 2.1 扩展 CheckInState

```typescript
/** 签到状态（扩展） */
export interface CheckInState {
  /** 当前连续签到天数（0 = 未签到过） */
  streak: number
  /** 上次签到日期（'YYYY-MM-DD'） */
  lastCheckInDate: string
  /** 累计签到总次数（跨周期累计，成就统计用） */
  totalCheckIns: number
  /** 历史最高连续签到天数（成就统计用） */
  maxStreak: number
  /** 上次断签日期（'YYYY-MM-DD'，空字符串表示从未断签） */
  lastBreakDate: string
}
```

### 2.2 新增 CheckInReward

```typescript
/** 单日签到奖励定义 */
export interface CheckInReward {
  /** 签到天数（1~7） */
  day: number
  /** 金币奖励 */
  gold: number
  /** 经验值奖励 */
  exp: number
  /** 额外道具（仅第 7 天有值） */
  bonusItem?: ItemId
  /** 是否为满签日（第 7 天） */
  isMilestone: boolean
}
```

### 2.3 新增 CheckInStatus（供 UI 读取）

```typescript
/** 签到面板状态快照（供 UI 渲染用） */
export interface CheckInStatus {
  /** 今日是否已签到 */
  hasCheckedInToday: boolean
  /** 当前连续签到天数 */
  currentStreak: number
  /** 今日签到后将达到的天数（预览用） */
  nextStreak: number
  /** 本周期内已完成签到的天数列表（[1, 2, 3] 表示已签 3 天） */
  completedDays: number[]
  /** 今日是本周期第几天（1~7） */
  todayCycleDay: number
  /** 是否处于断签状态（上次签到不是昨天且今日未签到） */
  isStreakBroken: boolean
  /** 今日签到可获得的奖励 */
  todayReward: CheckInReward
  /** 明日签到可获得的奖励（预览用） */
  tomorrowReward: CheckInReward
  /** 历史最高连签 */
  maxStreak: number
  /** 累计签到次数 */
  totalCheckIns: number
}
```

### 2.4 修改 CheckInResult

```typescript
/** 签到结果（增强） */
export interface CheckInResult {
  success: boolean
  /** 签到后天数（本次签到后的连续天数） */
  newStreak: number
  /** 获得金币 */
  reward: number
  /** 获得经验值 */
  rewardExp: number
  /** 获得道具（仅第 7 天） */
  rewardItem?: ItemId
  /** 是否可签到 */
  canCheckIn: boolean
  /** 是否发生了断签重置（本次签到因断签而重置为第 1 天） */
  wasReset: boolean
  /** 失败原因 */
  reason?: 'already_checked_in'
}
```

### 2.5 修改 ShopAction

```typescript
export type ShopAction =
  | { type: 'BUY_ITEM'; itemId: ItemId }
  | { type: 'USE_ITEM'; itemId: ItemId }
  | { type: 'ADD_ITEM'; itemId: ItemId; count: number }
  | { type: 'CHECK_IN'; reward: number; rewardExp: number; newStreak: number; wasReset: boolean; rewardItem?: ItemId }
  | { type: 'RESET_DAILY_PURCHASES'; date: string }
  | { type: 'LOAD_STATE'; state: ShopState }
```

---

## 3. 常量定义 (`constants.ts`)

### 3.1 签到奖励表（对齐设计文档）

```typescript
/**
 * 签到奖励表（7 天递增）
 * 对齐设计文档 6.4 每日签到表
 * Day 1~7：金币递增，第 7 天满签额外送道具
 */
export const CHECK_IN_REWARDS: number[] = [20, 30, 40, 50, 60, 80, 150]

/** 签到经验值奖励（每日固定 15，第 7 天满签 30） */
export const CHECK_IN_EXP_REWARDS: number[] = [15, 15, 15, 15, 15, 15, 30]

/** 签到满签（第 7 天）额外赠送的道具 */
export const CHECK_IN_DAY7_BONUS_ITEM: ItemId = 'toy_ball'

/** 签到周期天数 */
export const CHECK_IN_CYCLE_DAYS = 7

/** 每日签到基础经验值 */
export const CHECK_IN_BASE_EXP = 15

/** 满签日额外经验值 */
export const CHECK_IN_MILESTONE_EXP = 30
```

### 3.2 签到奖励完整表

| 天数 | 金币 | 经验值 | 额外道具 | 说明 |
|------|------|--------|---------|------|
| D1 | 20 | 15 | — | 起步 |
| D2 | 30 | 15 | — | |
| D3 | 40 | 15 | — | |
| D4 | 50 | 15 | — | |
| D5 | 60 | 15 | — | |
| D6 | 80 | 15 | — | 跳涨 |
| D7 | 150 | 30 | 🎾 玩具球 ×1 | 满签里程碑 |
| 断签 | 重置为 D1 | — | — | streak 归 1 |

### 3.3 奖励表完整性约束

- `CHECK_IN_REWARDS.length === CHECK_IN_CYCLE_DAYS`（7 天）
- `CHECK_IN_EXP_REWARDS.length === CHECK_IN_CYCLE_DAYS`（7 天）
- `CHECK_IN_REWARDS` 严格递增
- 第 7 天金币（150）> 第 6 天（80），跳跃 ≥ 50 金币

---

## 4. 纯逻辑函数 (`logic.ts`)

### 4.1 函数清单

| 函数 | 签名 | 职责 |
|------|------|------|
| `canCheckIn` | `(lastCheckInDate: string, now?: number) => boolean` | 判断今日是否可签到 |
| `checkStreakBreak` | `(lastCheckInDate: string, now?: number) => { isBroken: boolean; breakDate: string }` | 判断是否断签 |
| `calculateReward` | `(streak: number, lastCheckInDate: string, now?: number) => CheckInResult` | 计算签到结果（核心函数） |
| `getCheckInRewardForDay` | `(day: number) => CheckInReward` | 获取指定天数的奖励定义 |
| `getStreakInfo` | `(state: CheckInState, now?: number) => CheckInStatus` | 获取签到面板状态快照 |
| `getTodayString` | `(now?: number) => string` | 已有，复用 |
| `getYesterdayString` | `(now?: number) => string` | 已有，复用 |

### 4.2 `canCheckIn` — 判断今日是否可签到

```typescript
/**
 * 判断今日是否可签到
 * @param lastCheckInDate 上次签到日期 'YYYY-MM-DD'
 * @param now 当前时间戳（可选，测试注入）
 * @returns true = 今日尚未签到
 */
export function canCheckIn(lastCheckInDate: string, now?: number): boolean {
  const today = getTodayString(now)
  return lastCheckInDate !== today
}
```

### 4.3 `checkStreakBreak` — 判断是否断签

```typescript
/**
 * 判断签到是否断签
 * 断签定义：上次签到日期不是昨天且不是今天（即间隔 ≥ 2 天）
 * @param lastCheckInDate 上次签到日期 'YYYY-MM-DD'
 * @param now 当前时间戳（可选，测试注入）
 * @returns { isBroken: 是否断签, breakDate: 断签发生的日期 }
 */
export function checkStreakBreak(
  lastCheckInDate: string,
  now?: number
): { isBroken: boolean; breakDate: string } {
  if (!lastCheckInDate) {
    // 从未签到过，不算断签
    return { isBroken: false, breakDate: '' }
  }

  const today = getTodayString(now)
  const yesterday = getYesterdayString(now)

  // 今天已签到 → 未断签
  if (lastCheckInDate === today) {
    return { isBroken: false, breakDate: '' }
  }

  // 上次签到是昨天 → 未断签
  if (lastCheckInDate === yesterday) {
    return { isBroken: false, breakDate: '' }
  }

  // 上次签到既不是昨天也不是今天 → 断签
  // breakDate 记录今日（断签在新一次签到时确认）
  return { isBroken: true, breakDate: today }
}
```

### 4.4 `calculateReward` — 核心签到计算（重构现有 `getCheckInReward`）

```typescript
/**
 * 计算签到结果（纯函数，不修改状态）
 *
 * 逻辑流程：
 * 1. 今日已签到 → 返回失败
 * 2. 判断是否断签 → 断签则重置为第 1 天
 * 3. 未断签 → streak + 1
 * 4. 周期处理：streak > 7 → 重置为第 1 天（新周期）
 * 5. 根据周期天数查奖励表
 * 6. 第 7 天额外送道具
 *
 * @param streak 当前连续签到天数
 * @param lastCheckInDate 上次签到日期 'YYYY-MM-DD'
 * @param now 当前时间戳（可选，测试注入）
 */
export function calculateReward(
  streak: number,
  lastCheckInDate: string,
  now?: number
): CheckInResult {
  const today = getTodayString(now)

  // 1. 今日已签到
  if (!canCheckIn(lastCheckInDate, now)) {
    return {
      success: false,
      canCheckIn: false,
      newStreak: streak,
      reward: 0,
      rewardExp: 0,
      wasReset: false,
      reason: 'already_checked_in',
    }
  }

  // 2. 判断是否断签
  const breakInfo = checkStreakBreak(lastCheckInDate, now)
  const wasReset = breakInfo.isBroken

  // 3. 计算新 streak
  let newStreak: number
  if (wasReset) {
    // 断签 → 重置为第 1 天
    newStreak = 1
  } else {
    // 未断签 → streak + 1
    newStreak = streak + 1
  }

  // 4. 周期处理：满 7 天后重置周期为第 1 天
  if (newStreak > CHECK_IN_CYCLE_DAYS) {
    newStreak = 1
  }

  // 5. 获取奖励（基于周期天数，索引 0~6）
  const rewardIndex = newStreak - 1
  const reward = CHECK_IN_REWARDS[rewardIndex]
  const rewardExp = CHECK_IN_EXP_REWARDS[rewardIndex]

  // 6. 第 7 天额外送道具
  const rewardItem = newStreak === CHECK_IN_CYCLE_DAYS
    ? CHECK_IN_DAY7_BONUS_ITEM
    : undefined

  return {
    success: true,
    canCheckIn: true,
    newStreak,
    reward,
    rewardExp,
    wasReset,
    rewardItem,
  }
}
```

### 4.5 `getCheckInRewardForDay` — 获取指定天数的奖励定义

```typescript
/**
 * 获取指定天数的签到奖励定义（用于 UI 展示）
 * @param day 天数 1~7
 */
export function getCheckInRewardForDay(day: number): CheckInReward {
  const index = Math.max(0, Math.min(CHECK_IN_CYCLE_DAYS - 1, day - 1))
  return {
    day: index + 1,
    gold: CHECK_IN_REWARDS[index],
    exp: CHECK_IN_EXP_REWARDS[index],
    bonusItem: index === CHECK_IN_CYCLE_DAYS - 1 ? CHECK_IN_DAY7_BONUS_ITEM : undefined,
    isMilestone: index === CHECK_IN_CYCLE_DAYS - 1,
  }
}
```

### 4.6 `getStreakInfo` — 获取签到面板状态快照

```typescript
/**
 * 获取签到面板状态快照（供 UI 渲染用）
 * @param state 当前签到状态
 * @param now 当前时间戳（可选，测试注入）
 */
export function getStreakInfo(
  state: CheckInState,
  now?: number
): CheckInStatus {
  const today = getTodayString(now)
  const hasCheckedInToday = state.lastCheckInDate === today

  const breakInfo = checkStreakBreak(state.lastCheckInDate, now)

  // 今日是本周期第几天
  let todayCycleDay: number
  if (hasCheckedInToday) {
    todayCycleDay = state.streak
  } else if (breakInfo.isBroken) {
    todayCycleDay = 1 // 断签后从第 1 天开始
  } else {
    todayCycleDay = state.streak + 1
    if (todayCycleDay > CHECK_IN_CYCLE_DAYS) {
      todayCycleDay = 1 // 周期循环
    }
  }

  // 已完成签到的天数列表
  const completedDays: number[] = []
  if (state.streak > 0 && !breakInfo.isBroken) {
    for (let i = 1; i <= state.streak; i++) {
      completedDays.push(i)
    }
  }
  // 今日已签到则也加入已完成
  if (hasCheckedInToday && !completedDays.includes(state.streak)) {
    completedDays.push(state.streak)
  }

  // 今日可获得奖励
  const todayReward = getCheckInRewardForDay(todayCycleDay)

  // 明日可获得奖励（预览）
  const tomorrowCycleDay = todayCycleDay >= CHECK_IN_CYCLE_DAYS ? 1 : todayCycleDay + 1
  const tomorrowReward = getCheckInRewardForDay(tomorrowCycleDay)

  // nextStreak：今日签到后将达到的天数
  const nextStreak = hasCheckedInToday ? state.streak : todayCycleDay

  return {
    hasCheckedInToday,
    currentStreak: state.streak,
    nextStreak,
    completedDays,
    todayCycleDay,
    isStreakBroken: breakInfo.isBroken && !hasCheckedInToday,
    todayReward,
    tomorrowReward,
    maxStreak: state.maxStreak,
    totalCheckIns: state.totalCheckIns,
  }
}
```

### 4.7 现有 `getCheckInReward` 兼容处理

```typescript
/**
 * @deprecated 使用 calculateReward 替代
 * 兼容旧调用方的别名
 */
export function getCheckInReward(
  streak: number,
  lastCheckInDate: string,
  now?: number
): CheckInResult {
  return calculateReward(streak, lastCheckInDate, now)
}
```

---

## 5. ShopContext 增强 (`ShopContext.tsx`)

### 5.1 初始状态扩展

```typescript
const initialState: ShopState = {
  inventory: {},
  checkIn: {
    streak: 0,
    lastCheckInDate: '',
    totalCheckIns: 0,
    maxStreak: 0,
    lastBreakDate: '',
  },
  dailyPurchases: {},
  dailyPurchaseDate: getTodayString(),
}
```

### 5.2 旧存档迁移

```typescript
function migrateCheckInState(raw: unknown): CheckInState {
  const fallback: CheckInState = {
    streak: 0,
    lastCheckInDate: '',
    totalCheckIns: 0,
    maxStreak: 0,
    lastBreakDate: '',
  }
  if (typeof raw !== 'object' || raw === null) return fallback
  const obj = raw as Partial<CheckInState>
  return {
    streak: typeof obj.streak === 'number' ? obj.streak : 0,
    lastCheckInDate: typeof obj.lastCheckInDate === 'string' ? obj.lastCheckInDate : '',
    totalCheckIns: typeof obj.totalCheckIns === 'number' ? obj.totalCheckIns : 0,
    maxStreak: typeof obj.maxStreak === 'number' ? obj.maxStreak : 0,
    lastBreakDate: typeof obj.lastBreakDate === 'string' ? obj.lastBreakDate : '',
  }
}

function loadInitialState(): ShopState {
  try {
    const saved = localStorage.getItem(SHOP_STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as ShopState
      // 字段校验...
      // 迁移签到状态（旧存档可能缺字段）
      parsed.checkIn = migrateCheckInState(parsed.checkIn)
      // 每日限购重置检查...
      return parsed
    }
  } catch (e) {
    console.warn('加载商店存档失败，使用默认值:', e)
  }
  return initialState
}
```

### 5.3 CHECK_IN Reducer 增强

```typescript
case 'CHECK_IN': {
  const prevStreak = state.checkIn.streak
  const newStreak = action.newStreak
  const newMaxStreak = Math.max(state.checkIn.maxStreak, newStreak)

  // 断签时记录断签日期
  const newLastBreakDate = action.wasReset
    ? getTodayString()
    : state.checkIn.lastBreakDate

  return {
    ...state,
    checkIn: {
      streak: newStreak,
      lastCheckInDate: getTodayString(),
      totalCheckIns: state.checkIn.totalCheckIns + 1,
      maxStreak: newMaxStreak,
      lastBreakDate: newLastBreakDate,
    },
    // 第 7 天额外送道具
    inventory: action.rewardItem
      ? {
          ...state.inventory,
          [action.rewardItem]: (state.inventory[action.rewardItem] ?? 0) + 1,
        }
      : state.inventory,
  }
}
```

### 5.4 checkIn() 方法增强

```typescript
const checkIn = useCallback((): CheckInResult => {
  const result = calculateReward(
    state.checkIn.streak,
    state.checkIn.lastCheckInDate
  )

  if (result.success) {
    // 发放金币奖励
    stamina.addGold(result.reward)

    // 发放经验值奖励（#41 预留：stamina.addExp 方法）
    if (result.rewardExp > 0 && typeof stamina.addExp === 'function') {
      stamina.addExp(result.rewardExp)
    }

    // 更新签到状态
    dispatch({
      type: 'CHECK_IN',
      reward: result.reward,
      rewardExp: result.rewardExp,
      newStreak: result.newStreak,
      wasReset: result.wasReset,
      rewardItem: result.rewardItem,
    })

    // 经济追踪（#40 预留）
    if (typeof economyTrackEarn === 'function') {
      economyTrackEarn(result.reward, 'checkin')
    }
  }

  return result
}, [stamina, state.checkIn, economyTrackEarn])
```

### 5.5 新增 getCheckInStatus()

```typescript
/**
 * 获取签到面板状态快照（供 UI 渲染用）
 * 封装 getStreakInfo 纯函数，注入当前 state
 */
const getCheckInStatus = useCallback((): CheckInStatus => {
  return getStreakInfo(state.checkIn)
}, [state.checkIn])
```

### 5.6 ShopContextValue 接口更新

```typescript
export interface ShopContextValue {
  state: ShopState
  buyItem: (itemId: ItemId) => BuyResult
  useItem: (itemId: ItemId) => UseItemResult
  /** 每日签到（增强版） */
  checkIn: () => CheckInResult
  /** 获取签到面板状态快照（新增） */
  getCheckInStatus: () => CheckInStatus
  getItemCount: (itemId: ItemId) => number
  getDailyPurchaseCount: (itemId: ItemId) => number
  getCaptureBoost: () => number
  consumeCaptureBoost: () => void
  isCaptureBoostActive: () => boolean
  addItem: (itemId: ItemId) => void
}
```

### 5.7 状态流转图

```
签到流程（增强后）:
  用户打开签到 Modal
    → getCheckInStatus() 读取 CheckInStatus
    → UI 渲染：日历视图 + 当前连签 + 今日奖励 + 断签提示
    → 用户点击 [签到] 按钮
    → checkIn()
      → calculateReward(streak, lastCheckInDate)  // 纯函数
      → 成功:
        a. stamina.addGold(reward)               // 发金币（StaminaContext）
        b. stamina.addExp(rewardExp)              // 发经验值（#41 预留）
        c. dispatch({ type: 'CHECK_IN', ... })    // 更新签到状态 + 道具
        d. economy.trackEarn(reward, 'checkin')   // 经济追踪（#40 预留）
      → 返回 CheckInResult（含 wasReset 断签标记）
    → UI 根据 result 显示:
        - 正常签到: "签到成功！获得 {reward} 🪙 + {rewardExp} XP"
        - 断签重置: "签到中断，重新从第 1 天开始。获得 {reward} 🪙"
        - 满签额外: "+ {itemName} ×1 🎁"
        - 已签到: "今日已签到"
```

---

## 6. UI 集成 — CheckInModal

### 6.1 组件结构

```
┌─────────────────────────────────────────┐
│              CheckInModal                │
├─────────────────────────────────────────┤
│                                         │
│  ┌─────────────────────────────────┐    │
│  │  📅 每日签到            [×]     │    │  ← 标题栏 + 关闭按钮
│  └─────────────────────────────────┘    │
│                                         │
│  ┌─────────────────────────────────┐    │
│  │     连续签到 3 天 🔥             │    │  ← 当前连签展示（火焰图标）
│  │     历史最高 5 天               │    │  ← 历史最高（小字）
│  └─────────────────────────────────┘    │
│                                         │
│  ┌─────────────────────────────────┐    │
│  │  7 天日历网格                    │    │
│  │  ┌────┬────┬────┬────┬────┐    │    │
│  │  │ D1 │ D2 │ D3 │ D4 │ D5 │    │    │  ← 横向 7 格
│  │  │ ✓  │ ✓  │ ✓  │ 今 │ D5 │    │    │  ← 已签✓ / 今日高亮 / 未签
│  │  │ 20 │ 30 │ 40 │ 50 │ 60 │    │    │  ← 每格金币
│  │  └────┴────┴────┴────┴────┘    │    │
│  │  ┌────┬────┐                    │    │
│  │  │ D6 │ D7 │                    │    │  ← 第 6、7 天单独展示
│  │  │ 80 │150 │                    │    │
│  │  │    │ 🎁 │                    │    │  ← 第 7 天礼物标记
│  │  └────┴────┘                    │    │
│  └─────────────────────────────────┘    │
│                                         │
│  ┌─────────────────────────────────┐    │
│  │  ⚠️ 签到中断！重新从第 1 天开始  │    │  ← 断签提示（仅断签时显示）
│  └─────────────────────────────────┘    │
│                                         │
│  ┌─────────────────────────────────┐    │
│  │  今日奖励：50 🪙 + 15 XP        │    │  ← 今日奖励预览
│  │  明日奖励：60 🪙 + 15 XP        │    │  ← 明日奖励预览
│  └─────────────────────────────────┘    │
│                                         │
│  ┌─────────────────────────────────┐    │
│  │        [ 📅 立即签到 ]           │    │  ← 签到按钮（已签时 disabled）
│  └─────────────────────────────────┘    │
│                                         │
└─────────────────────────────────────────┘
```

### 6.2 CheckInModal 组件设计

```typescript
/**
 * 签到弹窗组件
 *
 * Props:
 * @param onClose 关闭弹窗回调
 *
 * 功能：
 * - 7 天日历网格（已签/今日/未签 3 种状态）
 * - 当前连签天数 + 历史最高
 * - 断签提示（仅断签时显示）
 * - 今日/明日奖励预览
 * - 签到按钮 + 签到结果 Toast
 */
interface CheckInModalProps {
  onClose: () => void
}

const CheckInModal: React.FC<CheckInModalProps> = ({ onClose }) => {
  const shop = useShop()
  const [toast, setToast] = useState<string | null>(null)
  const [showBreakNotice, setShowBreakNotice] = useState(false)

  const status = shop.getCheckInStatus()

  const handleCheckIn = useCallback(() => {
    const result = shop.checkIn()
    if (result.success) {
      let msg = `签到成功！+${result.reward} 🪙 +${result.rewardExp} XP`
      if (result.rewardItem) {
        msg += ` +${ITEM_DEFS[result.rewardItem].name} ×1 🎁`
      }
      if (result.wasReset) {
        msg = `断签重置，重新从第 1 天开始\n${msg}`
        setShowBreakNotice(true)
      }
      setToast(msg)
      setTimeout(() => setToast(null), 3000)
    } else {
      setToast('今日已签到')
      setTimeout(() => setToast(null), 2000)
    }
  }, [shop])

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={e => e.stopPropagation()} style={styles.modalContent}>
        {/* 标题栏 */}
        <div style={styles.header}>
          <span style={styles.title}>📅 每日签到</span>
          <button onClick={onClose} style={styles.closeBtn}>×</button>
        </div>

        {/* 连签展示 */}
        <div style={styles.streakBox}>
          <div style={styles.streakMain}>
            连续签到 {status.hasCheckedInToday ? status.currentStreak : status.nextStreak} 天 🔥
          </div>
          <div style={styles.streakSub}>
            历史最高 {status.maxStreak} 天 · 累计签到 {status.totalCheckIns} 次
          </div>
        </div>

        {/* 断签提示 */}
        {status.isStreakBroken && (
          <div style={styles.breakNotice}>
            ⚠️ 上次签到已中断，今日签到将重新从第 1 天开始
          </div>
        )}

        {/* 7 天日历网格 */}
        <div style={styles.calendarGrid}>
          {Array.from({ length: CHECK_IN_CYCLE_DAYS }, (_, i) => {
            const day = i + 1
            const reward = getCheckInRewardForDay(day)
            const isDone = status.completedDays.includes(day)
            const isToday = day === status.todayCycleDay && !status.hasCheckedInToday
            const isDay7 = day === CHECK_IN_CYCLE_DAYS

            return (
              <div
                key={day}
                style={{
                  ...styles.calendarCell,
                  ...(isDone ? styles.cellDone : {}),
                  ...(isToday ? styles.cellToday : {}),
                  ...(isDay7 ? styles.cellDay7 : {}),
                }}
              >
                <span style={styles.cellDay}>D{day}</span>
                <span style={styles.cellGold}>{reward.gold}🪙</span>
                <span style={styles.cellExp}>+{reward.exp}XP</span>
                {isDay7 && <span style={styles.cellBonus}>🎁</span>}
                {isDone && <span style={styles.cellCheck}>✓</span>}
              </div>
            )
          })}
        </div>

        {/* 奖励预览 */}
        <div style={styles.rewardPreview}>
          <div style={styles.rewardRow}>
            <span>今日奖励：</span>
            <span>{status.todayReward.gold} 🪙 + {status.todayReward.exp} XP</span>
            {status.todayReward.bonusItem && (
              <span>+ {ITEM_DEFS[status.todayReward.bonusItem].name} 🎁</span>
            )}
          </div>
          <div style={styles.rewardRow}>
            <span>明日奖励：</span>
            <span>{status.tomorrowReward.gold} 🪙 + {status.tomorrowReward.exp} XP</span>
            {status.tomorrowReward.bonusItem && (
              <span>+ {ITEM_DEFS[status.tomorrowReward.bonusItem].name} 🎁</span>
            )}
          </div>
        </div>

        {/* 签到按钮 */}
        <button
          className="btn btn-primary"
          style={{
            ...styles.checkInBtn,
            ...(status.hasCheckedInToday ? styles.disabledBtn : {}),
          }}
          disabled={status.hasCheckedInToday}
          onClick={handleCheckIn}
        >
          {status.hasCheckedInToday ? '✓ 今日已签到' : '📅 立即签到'}
        </button>

        {/* Toast */}
        {toast && (
          <div style={styles.toast}>{toast}</div>
        )}
      </div>
    </div>
  )
}

export default CheckInModal
```

### 6.3 StoreScreen 修改

将现有内联签到面板替换为入口按钮：

```typescript
// StoreScreen.tsx 修改部分

const [checkInModalOpen, setCheckInModalOpen] = useState(false)

// 签到入口区域（替换原有内联面板）
<div style={styles.checkInEntry}>
  <div style={styles.checkInSummary}>
    <span>📅 每日签到</span>
    <span style={styles.checkInStreakText}>
      连续 {shop.state.checkIn.streak} 天
    </span>
  </div>
  <button
    className="btn btn-primary"
    style={styles.checkInOpenBtn}
    onClick={() => setCheckInModalOpen(true)}
  >
    {getTodayString() === shop.state.checkIn.lastCheckInDate ? '✓ 已签到' : '去签到'}
  </button>
</div>

{/* 签到弹窗 */}
{checkInModalOpen && (
  <CheckInModal onClose={() => setCheckInModalOpen(false)} />
)}
```

### 6.4 样式设计要点

- Modal 遮罩：半透明黑色背景 `rgba(0,0,0,0.5)`
- Modal 内容：白色圆角卡片（`borderRadius: 20`），居中定位
- 日历网格：7 列 `grid`，每格圆角小卡片
- 已签到格：绿色背景 `var(--success)` + 半透明 + ✓ 标记
- 今日格：橙色边框 `var(--orange)` + 浅橙背景 `var(--orange-50)`
- 第 7 天格：金色边框 `var(--coin)` + 🎁 图标
- 断签提示：浅红背景 + ⚠️ 图标
- 签到按钮：全宽暖橙色主按钮
- 签到成功动画：金币飞出 + 数值跳动（MVP 用 CSS transition，不做复杂动画）

---

## 7. 测试用例 (`logic.test.ts`)

### 7.1 新增签到测试用例（≥12 个）

以下为新增的签到专项测试，编号从 #28 开始（接续现有 #1~#27）：

```typescript
// 固定时间戳：2025-01-15 12:00:00 UTC（周三）
const BASE_TIME = 1736932800000

describe('canCheckIn — 判断今日是否可签到', () => {
  it('#28 从未签到 lastCheckInDate="" => true', () => {
    expect(canCheckIn('', BASE_TIME)).toBe(true)
  })

  it('#29 今日已签到 lastCheckInDate=today => false', () => {
    const today = getTodayString(BASE_TIME)
    expect(canCheckIn(today, BASE_TIME)).toBe(false)
  })

  it('#30 昨日签到，今日未签 => true', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    expect(canCheckIn(yesterday, BASE_TIME)).toBe(true)
  })
})

describe('checkStreakBreak — 断签判断', () => {
  it('#31 从未签到 => isBroken=false', () => {
    const result = checkStreakBreak('', BASE_TIME)
    expect(result.isBroken).toBe(false)
    expect(result.breakDate).toBe('')
  })

  it('#32 上次签到是昨天 => isBroken=false', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = checkStreakBreak(yesterday, BASE_TIME)
    expect(result.isBroken).toBe(false)
  })

  it('#33 上次签到是3天前 => isBroken=true, breakDate=today', () => {
    const today = getTodayString(BASE_TIME)
    const result = checkStreakBreak('2025-01-12', BASE_TIME)
    expect(result.isBroken).toBe(true)
    expect(result.breakDate).toBe(today)
  })

  it('#34 今日已签到 => isBroken=false', () => {
    const today = getTodayString(BASE_TIME)
    const result = checkStreakBreak(today, BASE_TIME)
    expect(result.isBroken).toBe(false)
  })
})

describe('calculateReward — 奖励计算（对齐设计文档数值）', () => {
  it('#35 首次签到 streak=0 => newStreak=1, reward=20, rewardExp=15', () => {
    const result = calculateReward(0, '', BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.canCheckIn).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.reward).toBe(20)
    expect(result.rewardExp).toBe(15)
    expect(result.wasReset).toBe(false)
    expect(result.rewardItem).toBeUndefined()
  })

  it('#36 连续第3天 streak=2, last=yesterday => newStreak=3, reward=40', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = calculateReward(2, yesterday, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(3)
    expect(result.reward).toBe(40)
    expect(result.rewardExp).toBe(15)
    expect(result.wasReset).toBe(false)
  })

  it('#37 第7天满签 streak=6, last=yesterday => newStreak=7, reward=150, rewardExp=30, rewardItem=toy_ball', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = calculateReward(6, yesterday, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(7)
    expect(result.reward).toBe(150)
    expect(result.rewardExp).toBe(30)
    expect(result.rewardItem).toBe('toy_ball')
    expect(result.wasReset).toBe(false)
  })

  it('#38 满7天后第8天 streak=7, last=yesterday => 周期重置 newStreak=1, reward=20, wasReset=false', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = calculateReward(7, yesterday, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.reward).toBe(20)
    expect(result.rewardExp).toBe(15)
    // 周期重置不算断签（wasReset 仅表示断签导致的重置）
    expect(result.wasReset).toBe(false)
  })
})

describe('calculateReward — 断签处理', () => {
  it('#39 断签后重新签到 streak=5, last=3天前 => newStreak=1, wasReset=true, reward=20', () => {
    const result = calculateReward(5, '2025-01-12', BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.reward).toBe(20)
    expect(result.wasReset).toBe(true)
  })

  it('#40 今日已签到 => success=false, reason=already_checked_in', () => {
    const today = getTodayString(BASE_TIME)
    const result = calculateReward(3, today, BASE_TIME)
    expect(result.success).toBe(false)
    expect(result.canCheckIn).toBe(false)
    expect(result.reason).toBe('already_checked_in')
    expect(result.wasReset).toBe(false)
  })

  it('#41 断签后周期重置：streak=7, last=5天前 => newStreak=1, wasReset=true', () => {
    const result = calculateReward(7, '2025-01-10', BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.wasReset).toBe(true)
  })
})

describe('getCheckInRewardForDay — 奖励定义查询', () => {
  it('#42 D1 => gold=20, exp=15, isMilestone=false', () => {
    const reward = getCheckInRewardForDay(1)
    expect(reward.day).toBe(1)
    expect(reward.gold).toBe(20)
    expect(reward.exp).toBe(15)
    expect(reward.isMilestone).toBe(false)
    expect(reward.bonusItem).toBeUndefined()
  })

  it('#43 D7 => gold=150, exp=30, isMilestone=true, bonusItem=toy_ball', () => {
    const reward = getCheckInRewardForDay(7)
    expect(reward.day).toBe(7)
    expect(reward.gold).toBe(150)
    expect(reward.exp).toBe(30)
    expect(reward.isMilestone).toBe(true)
    expect(reward.bonusItem).toBe('toy_ball')
  })

  it('#44 D0 越界 => clamp 到 D1', () => {
    const reward = getCheckInRewardForDay(0)
    expect(reward.day).toBe(1)
  })

  it('#45 D8 越界 => clamp 到 D7', () => {
    const reward = getCheckInRewardForDay(8)
    expect(reward.day).toBe(7)
  })
})

describe('getStreakInfo — 面板状态快照', () => {
  it('#46 未签到状态 streak=0 => hasCheckedInToday=false, todayCycleDay=1, isStreakBroken=false', () => {
    const state: CheckInState = {
      streak: 0, lastCheckInDate: '',
      totalCheckIns: 0, maxStreak: 0, lastBreakDate: '',
    }
    const info = getStreakInfo(state, BASE_TIME)
    expect(info.hasCheckedInToday).toBe(false)
    expect(info.todayCycleDay).toBe(1)
    expect(info.isStreakBroken).toBe(false)
    expect(info.todayReward.gold).toBe(20)
  })

  it('#47 连签3天且今日未签 => nextStreak=4, todayCycleDay=4, completedDays=[1,2,3]', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const state: CheckInState = {
      streak: 3, lastCheckInDate: yesterday,
      totalCheckIns: 10, maxStreak: 5, lastBreakDate: '',
    }
    const info = getStreakInfo(state, BASE_TIME)
    expect(info.hasCheckedInToday).toBe(false)
    expect(info.nextStreak).toBe(4)
    expect(info.todayCycleDay).toBe(4)
    expect(info.completedDays).toEqual([1, 2, 3])
    expect(info.isStreakBroken).toBe(false)
  })

  it('#48 断签状态 streak=5, last=3天前 => isStreakBroken=true, todayCycleDay=1', () => {
    const state: CheckInState = {
      streak: 5, lastCheckInDate: '2025-01-12',
      totalCheckIns: 20, maxStreak: 7, lastBreakDate: '',
    }
    const info = getStreakInfo(state, BASE_TIME)
    expect(info.isStreakBroken).toBe(true)
    expect(info.todayCycleDay).toBe(1)
    expect(info.todayReward.gold).toBe(20)
  })

  it('#49 今日已签到 streak=3 => hasCheckedInToday=true, todayCycleDay=3', () => {
    const today = getTodayString(BASE_TIME)
    const state: CheckInState = {
      streak: 3, lastCheckInDate: today,
      totalCheckIns: 3, maxStreak: 3, lastBreakDate: '',
    }
    const info = getStreakInfo(state, BASE_TIME)
    expect(info.hasCheckedInToday).toBe(true)
    expect(info.todayCycleDay).toBe(3)
    expect(info.nextStreak).toBe(3)
  })

  it('#50 满7天周期循环：streak=7, last=yesterday => todayCycleDay=1（新周期）', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const state: CheckInState = {
      streak: 7, lastCheckInDate: yesterday,
      totalCheckIns: 14, maxStreak: 7, lastBreakDate: '',
    }
    const info = getStreakInfo(state, BASE_TIME)
    expect(info.todayCycleDay).toBe(1)
    expect(info.isStreakBroken).toBe(false)
    expect(info.todayReward.gold).toBe(20)
  })
})

describe('奖励表完整性校验', () => {
  it('#51 CHECK_IN_REWARDS 长度为 7 且严格递增', () => {
    expect(CHECK_IN_REWARDS).toHaveLength(7)
    for (let i = 1; i < CHECK_IN_REWARDS.length; i++) {
      expect(CHECK_IN_REWARDS[i]).toBeGreaterThan(CHECK_IN_REWARDS[i - 1])
    }
  })

  it('#52 CHECK_IN_EXP_REWARDS 长度为 7 且第 7 天 > 其他天', () => {
    expect(CHECK_IN_EXP_REWARDS).toHaveLength(7)
    const day7Exp = CHECK_IN_EXP_REWARDS[6]
    for (let i = 0; i < 6; i++) {
      expect(day7Exp).toBeGreaterThan(CHECK_IN_EXP_REWARDS[i])
    }
  })

  it('#53 D7 金币奖励（150）对齐设计文档', () => {
    expect(CHECK_IN_REWARDS[6]).toBe(150)
  })
})
```

### 7.2 测试覆盖总览

| 类别 | 用例编号 | 用例数 | 覆盖函数 |
|------|---------|--------|---------|
| canCheckIn 判断 | #28~#30 | 3 | `canCheckIn` |
| checkStreakBreak 断签 | #31~#34 | 4 | `checkStreakBreak` |
| calculateReward 正常 | #35~#38 | 4 | `calculateReward` |
| calculateReward 断签 | #39~#41 | 3 | `calculateReward`（断签/重复/周期断签） |
| getCheckInRewardForDay | #42~#45 | 4 | `getCheckInRewardForDay` |
| getStreakInfo 面板状态 | #46~#50 | 5 | `getStreakInfo` |
| 奖励表完整性 | #51~#53 | 3 | 常量校验 |
| **新增合计** | | **26** | |
| 原有保留 | #1~#27 | 27 | 道具购买/旧签到逻辑 |
| **总计** | | **53** | |

---

## 8. 实现步骤

| 步骤 | 内容 | 涉及文件 | 依赖 |
|------|------|---------|------|
| 1 | 更新 `constants.ts`：对齐签到奖励表 + 新增经验值常量 | `shop/constants.ts` | 无 |
| 2 | 扩展 `types.ts`：`CheckInState` +4 字段，新增 `CheckInReward` / `CheckInStatus`，修改 `CheckInResult` / `ShopAction` | `shop/types.ts` | 步骤 1 |
| 3 | 重构 `logic.ts`：拆分 `canCheckIn` / `checkStreakBreak` / `calculateReward` / `getCheckInRewardForDay` / `getStreakInfo`，保留 `getCheckInReward` 别名 | `shop/logic.ts` | 步骤 1, 2 |
| 4 | 编写/更新 `logic.test.ts`：新增 26 个签到测试用例，确保全部通过 | `shop/logic.test.ts` | 步骤 3 |
| 5 | 增强 `ShopContext.tsx`：迁移旧存档 + 增强 Reducer + `checkIn()` 发经验值 + 新增 `getCheckInStatus()` | `shop/ShopContext.tsx` | 步骤 1~3 |
| 6 | 新增 `CheckInModal.tsx`：7 天日历 + 连签展示 + 断签提示 + 奖励预览 + 签到按钮 | `components/CheckInModal.tsx` | 步骤 5 |
| 7 | 修改 `StoreScreen.tsx`：签到面板替换为入口按钮 + 弹窗触发 | `components/StoreScreen.tsx` | 步骤 6 |
| 8 | 新增 `CheckInModal.test.tsx`：弹窗渲染 + 签到交互测试 | `components/CheckInModal.test.tsx` | 步骤 6 |
| 9 | 全量测试 `vitest run` + 手动验收 | — | 步骤 1~8 |

---

## 9. 验收标准

| 验收标准 | 实现方案 | 验证方式 |
|---------|---------|---------|
| 连续签到递增奖励 | `CHECK_IN_REWARDS = [20, 30, 40, 50, 60, 80, 150]` + `calculateReward()` 纯函数 | 单测 #35~#38 |
| 断签重置为第 1 天 | `checkStreakBreak()` 判断非昨日 → `calculateReward()` 重置 streak=1 | 单测 #39, #41 |
| 第 7 天额外道具 | `CHECK_IN_DAY7_BONUS_ITEM = 'toy_ball'` + `calculateReward()` 第 7 天返回 `rewardItem` | 单测 #37 |
| 签到经验值 | `CHECK_IN_EXP_REWARDS` + `stamina.addExp()` 调用 | 单测 #35~#38 验证 rewardExp |
| 周期循环（满 7 天后重置） | `calculateReward()` 中 `newStreak > 7 → 1` | 单测 #38, #50 |
| 签到 UI 日历视图 | `CheckInModal` 7 天网格 + 已签/今日/未签 3 种状态 | 手动测试 |
| 断签提示 UI | `CheckInModal` 中 `isStreakBroken` 条件渲染断签警告 | 手动测试 + `getStreakInfo` 单测 #48 |
| 今日/明日奖励预览 | `getStreakInfo()` 返回 `todayReward` / `tomorrowReward` | 单测 #46~#50 |
| 签到状态持久化 | `ShopState` 序列化到 localStorage `animal_poke_shop` | 刷新页面验证 |
| 旧存档兼容 | `migrateCheckInState()` 补全缺失字段 | 手动测试（加载旧存档） |
| 累计签到/历史最高 | `totalCheckIns` / `maxStreak` 字段，UI 展示 | 手动测试 + 单测 #47 |
| 今日已签到禁用 | `canCheckIn()` 返回 false → 按钮 disabled | 单测 #29, #40 |

---

## 10. 依赖与风险

### 10.1 依赖关系

| 依赖 | 来源 | 说明 |
|------|------|------|
| `StaminaContext.addGold()` | #31 已完成 | 签到发金币，已对接 |
| `StaminaContext.addExp()` | #41 计划中 | 签到发经验值；#41 未完成时 `typeof stamina.addExp === 'function'` 判断跳过 |
| `EconomyContext.trackEarn()` | #40 计划中 | 经济追踪；#40 未完成时通过可选回调跳过 |
| `AchievementContext.checkAchievements()` | #41 计划中 | 签到成就检查；#41 未完成时跳过 |
| `ITEM_DEFS` / `ItemId` | #34 已完成 | 第 7 天奖励道具定义 |
| `getTodayString` / `getYesterdayString` | #34 已完成 | 日期工具函数 |

### 10.2 兼容性风险

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 旧存档缺少 `totalCheckIns` / `maxStreak` / `lastBreakDate` | 加载崩溃或 NaN | `migrateCheckInState()` 函数补默认值 |
| 奖励表数值变更（10→20 等） | 玩家感知奖励变多（正面）或变少（负面） | 本计划对齐设计文档，数值变更为设计意图 |
| `getCheckInReward` → `calculateReward` 重命名 | 外部调用方报错 | 保留 `getCheckInReward` 别名（`@deprecated` 标记） |
| `CheckInResult` 新增 `rewardExp` / `wasReset` 字段 | 旧代码访问未定义字段 | TypeScript 类型保证编译期检查；运行时旧代码不读新字段 |

### 10.3 技术风险

| 风险 | 影响 | 应对策略 |
|------|------|---------|
| 签到 Modal 与现有 StoreScreen 样式冲突 | UI 错位 | Modal 使用独立样式对象，不依赖 StoreScreen 的 styles |
| `stamina.addExp` 方法不存在 | 运行时错误 | `typeof stamina.addExp === 'function'` 防御判断 |
| 时区差异导致日期判断错误 | 签到日期偏移 | `getTodayString` 使用本地时区 `new Date()`，与 StaminaContext 一致 |
| 断签检测时机不当 | 玩家打开 Modal 时未显示断签提示 | `getStreakInfo` 每次渲染实时计算，不依赖额外的检测触发 |

### 10.4 后续扩展预留

| 扩展方向 | 预留方式 | 对应 Issue |
|---------|---------|-----------|
| 补签卡道具（消耗道具恢复断签） | `lastBreakDate` 字段已记录断签日期 | 后续 Issue |
| 月度签到日历（30 天） | `totalCheckIns` 已累计 | 后续 Issue |
| 签到成就（连签 7/30/100 天） | `maxStreak` / `totalCheckIns` 已记录 | #41 |
| 服务器校验签到 | 纯函数设计便于后端复用 | 后端 Issue |
| 签到推送通知 | `canCheckIn()` 纯函数可在外部调用 | 后续 Issue |

---

## 11. 注意事项

1. **奖励表数值必须对齐设计文档**：现有 `constants.ts` 中 `CHECK_IN_REWARDS = [10, 20, 30, 50, 80, 120, 200]` 需改为 `[20, 30, 40, 50, 60, 80, 150]`，这是本 Issue 的核心变更之一。

2. **`getCheckInReward` 别名保留**：现有 `StoreScreen.tsx` 和 `ShopContext.tsx` 调用了 `getCheckInReward`，重构为 `calculateReward` 后需保留别名避免编译错误。实际调用方应逐步迁移到 `calculateReward`。

3. **经验值发放的条件判断**：`stamina.addExp` 是 #41 计划引入的方法。在 #41 完成前，通过 `typeof stamina.addExp === 'function'` 判断是否调用。#41 完成后移除条件判断。

4. **经济追踪的条件判断**：`economy.trackEarn` 是 #40 计划引入的方法。同上处理。

5. **`CheckInModal` 使用 `useShop()` 而非直接传 props**：Modal 内部通过 `useShop()` 获取签到状态和方法，与 `StoreScreen` 解耦。仅通过 `onClose` props 控制关闭。

6. **断签提示的时机**：`getStreakInfo()` 在每次渲染时实时计算 `isStreakBroken`。若 `lastCheckInDate` 不是昨天且不是今天，则 `isStreakBroken = true`。玩家打开 Modal 即可看到断签提示，无需额外的「App 启动检测」逻辑。

7. **`wasReset` vs 周期重置的区别**：
   - `wasReset = true`：断签导致的重置（上次签到不是昨天），UI 显示断签提示。
   - 周期重置（streak 7→1）：连续签到满 7 天后的自然循环，不算断签，`wasReset = false`。

8. **代码注释用中文**：与现有代码风格一致。
