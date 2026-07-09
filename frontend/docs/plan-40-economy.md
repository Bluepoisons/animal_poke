# [M2] 经济系统实现计划 — Issue #40

> **验收标准**：金币产出/消耗闭环，道具商店完整
>
> **设计文档来源**：`游戏开发计划.md` 6.4 经济系统 + 3.4 体力系统（派遣） + 5.5 等级与体力数值 + 10.7 派遣任务化
>
> **技术栈**：React 18 + Vite 6 + TypeScript 5.6 + vitest，无额外依赖
>
> **现有基础**：
> - `StaminaContext`（#31）管理 `state.gold`，提供 `addGold()` / `buyStaminaPotion()`（150 金币换 3 体力，每日限 3 次）
> - `ShopContext`（#34）提供 `buyItem()` / `useItem()` / `checkIn()` / `getItemCount()` / `addItem()`，`ITEM_DEFS` 含 6 种道具，签到 7 天递增（10→200 金币）
> - `BattleContext`（#37）`computeRewards()` 胜利金币 `{ common:15, uncommon:25, rare:40, epic:70, legendary:120 }`，失败 5 金币，平局 10 金币，15% 道具掉落
> - `App.tsx` 捕获成功后随机掉落 10~50 金币
> - `LbsContext`（#36）提供 `cityName` / `provinceName`（派遣任务可绑定城市）
> - `WeatherContext`（#38）提供天气数据
> - `StatusContext`（#39 计划已出）提供宠物状态管理

---

## 0. 设计要点与关键决策

### 0.1 经济系统定位

经济系统不是一个独立的「大 Context」，而是**横切关注点**：金币的产出和消耗分散在多个子系统中（捕获、战斗、签到、商店、派遣）。本 Issue 的核心任务是：

1. **新增派遣系统**（DispatchContext）— 全新金币产出源，宠物消耗体力+时间换取金币/道具
2. **新增经济追踪**（EconomyContext）— 统计 totalEarned / totalSpent / netFlow，提供经济看板数据
3. **补全消耗出口** — 确保金币有足够消耗路径（体力药剂已有，感冒药已有，新增：购买额外战斗场次、派遣加速）
4. **经济平衡校验** — 纯函数 `balanceCheck()` 供开发/测试时验证产出消耗比

### 0.2 为什么不把 addGold/spendGold 从 StaminaContext 迁移到 EconomyContext？

`StaminaContext.state.gold` 已是金币的唯一真实来源（single source of truth），且已被 ShopContext、BattleContext、App.tsx 广泛引用。迁移将导致大面积重构。

**决策**：保持 `StaminaContext` 作为金币持有者，`EconomyContext` 作为**追踪层**，通过拦截 `addGold` 调用来统计产出/消耗。`EconomyProvider` 包裹在 `StaminaProvider` 内，通过 `useStamina()` 读取金币状态。

### 0.3 派遣系统与体力系统的关系

设计文档 3.4 明确：派遣消耗 20 体力/次，产出金币受稀有度影响，1 小时冷却。这与捕获争夺体力资源（体力双用途策略取舍）。

**决策**：
- 派遣消耗 20 体力（`DISPATCH_STAMINA_COST`，已在 `stamina/constants.ts` 定义）
- 派遣同时上限随等级提升：`Math.min(1 + Math.floor(level / 3), 4)`（Lv1-2: 1, Lv3-5: 2, Lv6-8: 3, Lv9-10: 4）
- 冷却时间 1 小时，可花费金币加速（50 金币立即完成）
- 派遣产出：金币 + 概率道具掉落 + 亲密度 +2（设计文档 5.6）

### 0.4 派遣任务设计（简化版 10.7）

设计文档 10.7 描述了派遣任务化（每日 3 个可选、叙事文本、风险事件）。M2 阶段实现**简化版**：

- 3 种预设派遣任务类型（短/中/长），不同时长不同收益
- 不实现叙事文本和风险事件（后续迭代）
- 不实现 LLM 生成派遣叙事（需后端支持）

| 任务类型 | 时长 | 体力消耗 | 金币产出（Common 基准） | 说明 |
|---------|------|---------|----------------------|------|
| 快速探索 | 30 分钟 | 20 | 15 | 短时低收益 |
| 标准探索 | 1 小时 | 20 | 25 | 标准收益 |
| 深度探索 | 2 小时 | 20 | 40 | 长时高收益 |

> 金币产出 = 基准 × 稀有度倍率，与战斗奖励表对齐。

### 0.5 经济平衡目标

| 指标 | 目标值 | 说明 |
|------|--------|------|
| 日均产出 | 150~300 金币 | 签到(10~200) + 捕获(10~50×N) + 战斗(15~120×N) + 派遣(15~40×N) |
| 日均消耗 | 100~250 金币 | 商店购买(30~200) + 体力药剂(150×0~3) + 派遣加速(50×0~N) |
| 净流入 | 50~100 金币/日 | 正向但可控，防止通胀 |
| 货币沉淀 | 升级奖励覆盖 | Lv2-Lv10 升级奖励 100~1000 金币，作为非线性产出脉冲 |

### 0.6 Provider 嵌套顺序

```
LbsProvider
  └─ StaminaProvider              ← 金币 state.gold 持有者
       └─ EconomyProvider          ← #40（本 Issue）追踪产出/消耗
            └─ WeatherProvider
                 └─ ShopProvider
                      └─ StatusProvider   ← #39
                           └─ DispatchProvider  ← #40（本 Issue）派遣系统
                                └─ BattleProvider
                                     └─ AppInner
```

- `EconomyProvider` 在 `StaminaProvider` 内：通过 `useStamina()` 读取金币
- `EconomyProvider` 在 `ShopProvider` 外：Shop/Battle 的 addGold 调用经由 Economy 追踪
- `DispatchProvider` 在 `StatusProvider` 内：派遣可能影响宠物状态（后续）
- `DispatchProvider` 在 `BattleProvider` 外：派遣与战斗互斥使用宠物

### 0.7 金币追踪实现策略

`EconomyContext` 提供两个包装函数 `trackEarn(amount, source)` 和 `trackSpend(amount, sink)`，**不直接修改金币**，仅记录统计。调用方在 `stamina.addGold()` 后调用 `trackEarn()`。

**替代方案（更优雅但侵入性大）**：`EconomyProvider` 通过 `useStamina()` 获取 `addGold`，用 `useRef` 拦截并包装为追踪版本，通过 Context 向下传递。但这会改变所有消费者的 `useStamina()` 返回值，风险过高。

**最终决策**：采用显式追踪 — 在关键产出/消耗点（捕获、战斗结算、签到、商店购买、派遣完成）调用 `trackEarn` / `trackSpend`。侵入性小，且追踪点清晰可审计。

---

## 1. 文件结构

```
frontend/src/
├── economy/
│   ├── constants.ts           # 经济追踪常量、派遣任务定义、产出/消耗分类
│   ├── types.ts               # EconomyState / EconomyAction / DispatchState / DispatchMission 等
│   ├── logic.ts               # 纯函数：calculateDispatchReward / balanceCheck / getDispatchSlots 等
│   ├── logic.test.ts          # 纯函数单元测试（≥18 个用例）
│   ├── EconomyContext.tsx     # 经济追踪 Context + useReducer + localStorage 持久化
│   ├── useEconomy.ts          # 自定义 Hook：封装 EconomyContext 消费
│   ├── DispatchContext.tsx    # 派遣系统 Context + useReducer + localStorage + 定时结算
│   └── useDispatch.ts         # 自定义 Hook：封装 DispatchContext 消费
├── stamina/
│   └── StaminaContext.tsx     # （修改）addGold 增加可选 source 参数用于追踪
├── shop/
│   └── ShopContext.tsx        # （修改）buyItem/checkIn 后调用 economy.trackSpend/trackEarn
├── battle/
│   └── BattleContext.tsx      # （修改）finishBattle 后调用 economy.trackEarn
├── components/
│   ├── StoreScreen.tsx        # （修改）新增经济看板区域（收支统计）
│   ├── DispatchScreen.tsx     # （新增）派遣任务界面
│   ├── DetailPopup.tsx        # （修改）新增「派遣」按钮
│   └── TopBar.tsx             # （修改）金币旁显示今日净收支
├── App.tsx                    # （修改）Provider 嵌套 + 捕获金币追踪 + 派遣 tab
└── types.ts                   # （修改）MainTab 新增 'dispatch'
```

### 各文件职责

| 文件 | 职责 | 依赖 |
|------|------|------|
| `economy/constants.ts` | 经济常量 + 派遣任务定义 + 产出/消耗分类枚举。纯数据 | 无 |
| `economy/types.ts` | TypeScript 类型定义。纯类型 | `constants.ts` |
| `economy/logic.ts` | 纯函数：计算派遣奖励、经济平衡校验、派遣槽位计算、结算检查。无副作用 | `constants.ts`, `types.ts` |
| `economy/logic.test.ts` | 纯函数单元测试 | `logic.ts` |
| `economy/EconomyContext.tsx` | 经济追踪核心。Context + Reducer + localStorage。提供 trackEarn/trackSpend/getStats | `constants`, `types`, `logic`, `StaminaContext` |
| `economy/useEconomy.ts` | 对外暴露的 Hook | `EconomyContext` |
| `economy/DispatchContext.tsx` | 派遣系统核心。Context + Reducer + localStorage + 定时结算。管理派遣任务生命周期 | `constants`, `types`, `logic`, `StaminaContext`, `EconomyContext`, `ShopContext`, `LbsContext` |
| `economy/useDispatch.ts` | 对外暴露的 Hook | `DispatchContext` |
| `stamina/StaminaContext.tsx`（修改） | `addGold` 增加可选 `source` 参数 | 无新增依赖 |
| `shop/ShopContext.tsx`（修改） | 购买/签到后调用 `economy.trackSpend` / `trackEarn` | `useEconomy` |
| `battle/BattleContext.tsx`（修改） | `finishBattle` 后调用 `economy.trackEarn` | `useEconomy` |
| `components/StoreScreen.tsx`（修改） | 新增经济看板 | `useEconomy` |
| `components/DispatchScreen.tsx`（新增） | 派遣任务界面 | `useDispatch`, `useStamina`, `useAnimalStore` |
| `components/DetailPopup.tsx`（修改） | 新增「派遣」按钮 | `useDispatch` |
| `components/TopBar.tsx`（修改） | 金币旁显示净收支 | `useEconomy` |
| `App.tsx`（修改） | Provider 嵌套 + 捕获追踪 + dispatch tab | `EconomyContext`, `DispatchContext` |
| `types.ts`（修改） | MainTab 新增 `'dispatch'` | 无 |

---

## 2. 类型定义 (`economy/types.ts`)

### 2.1 产出来源分类

```typescript
/**
 * 金币产出来源分类
 */
export type GoldSource =
  | 'capture'       // 捕获掉落
  | 'battle_win'    // 战斗胜利
  | 'battle_draw'   // 战斗平局
  | 'battle_lose'   // 战斗失败安慰金
  | 'checkin'       // 每日签到
  | 'levelup'       // 升级奖励
  | 'dispatch'      // 派遣任务
  | 'region_rank'   // 区域排行奖励（预留）
  | 'achievement'   // 成就解锁（预留）
  | 'other'         // 其他

/**
 * 金币消耗去向分类
 */
export type GoldSink =
  | 'shop_buy'        // 商店购买道具
  | 'stamina_potion'  // 购买体力药剂
  | 'dispatch_speedup'// 派遣加速
  | 'battle_extra'    // 购买额外战斗场次（预留）
  | 'other'           // 其他
```

### 2.2 经济追踪记录

```typescript
/**
 * 单条经济流水记录
 */
export interface EconomyLogEntry {
  /** 记录 ID（自增） */
  id: number
  /** 流水类型 */
  type: 'earn' | 'spend'
  /** 金额（正数） */
  amount: number
  /** 来源/去向 */
  category: GoldSource | GoldSink
  /** 时间戳（Unix ms） */
  timestamp: number
  /** 备注（可选，如 "捕获 #000059"） */
  note?: string
}

/**
 * 经济系统完整状态
 */
export interface EconomyState {
  /** 累计金币产出 */
  totalEarned: number
  /** 累计金币消耗 */
  totalSpent: number
  /** 流水记录（最多保留 200 条，FIFO 淘汰） */
  logs: EconomyLogEntry[]
  /** 下一条流水 ID */
  nextLogId: number
  /** 今日产出（自然日重置） */
  todayEarned: number
  /** 今日消耗（自然日重置） */
  todaySpent: number
  /** 今日日期标记（'YYYY-MM-DD'） */
  todayDate: string
}
```

### 2.3 经济统计结果

```typescript
/**
 * 经济统计快照（供 UI 看板使用）
 */
export interface EconomyStats {
  /** 当前金币余额 */
  currentGold: number
  /** 累计产出 */
  totalEarned: number
  /** 累计消耗 */
  totalSpent: number
  /** 净流入（累计） */
  netFlow: number
  /** 今日产出 */
  todayEarned: number
  /** 今日消耗 */
  todaySpent: number
  /** 今日净流入 */
  todayNetFlow: number
  /** 产出来源分布（累计） */
  earnedBySource: Record<GoldSource, number>
  /** 消耗去向分布（累计） */
  spentBySink: Record<GoldSink, number>
}

/**
 * 经济平衡检查结果
 */
export interface BalanceCheckResult {
  /** 产出消耗比（totalEarned / totalSpent，spent=0 时为 Infinity） */
  ratio: number
  /** 是否健康（ratio 在 1.2~3.0 之间） */
  isHealthy: boolean
  /** 状态标签 */
  status: 'deflation' | 'healthy' | 'inflation'
  /** 建议文案 */
  suggestion: string
}
```

### 2.4 派遣任务类型

```typescript
/**
 * 派遣任务类型
 */
export type DispatchMissionType = 'quick' | 'standard' | 'deep'

/**
 * 派遣任务状态
 */
export type DispatchMissionStatus = 'active' | 'completed' | 'collected'

/**
 * 单次派遣任务实例
 */
export interface DispatchMission {
  /** 任务实例 ID */
  id: string
  /** 任务类型 */
  type: DispatchMissionType
  /** 派遣的宠物 ID（CardEntry.id） */
  petId: string
  /** 宠物稀有度（影响奖励） */
  petRarity: RarityTier
  /** 绑定城市（可选，影响区域声望） */
  city: string
  /** 开始时间戳（Unix ms） */
  startTime: number
  /** 完成时间戳（Unix ms） */
  endTime: number
  /** 任务状态 */
  status: DispatchMissionStatus
  /** 预期奖励（开始时计算好） */
  rewards: DispatchReward
}

/**
 * 派遣奖励
 */
export interface DispatchReward {
  /** 金币 */
  gold: number
  /** 掉落道具 ID（概率掉落） */
  droppedItem?: string
  /** 亲密度增量 */
  affinity: number
}

/**
 * 派遣任务定义（静态）
 */
export interface DispatchMissionDef {
  type: DispatchMissionType
  name: string
  description: string
  icon: string
  /** 时长（分钟） */
  durationMin: number
  /** 体力消耗 */
  staminaCost: number
  /** 基准金币（Common 宠物基准） */
  baseGold: number
  /** 道具掉落概率 */
  itemDropRate: number
  /** 道具掉落池 */
  itemDropPool: { itemId: string; weight: number }[]
}
```

### 2.5 派遣系统状态

```typescript
/**
 * 派遣系统完整状态
 */
export interface DispatchState {
  /** 当前活跃的派遣任务列表 */
  missions: DispatchMission[]
  /** 今日已派遣次数（自然日重置） */
  todayDispatchCount: number
  /** 今日日期标记 */
  todayDate: string
}

/**
 * 派遣操作结果
 */
export interface DispatchResult {
  success: boolean
  reason?: 'insufficient_stamina' | 'no_slots' | 'pet_busy' | 'pet_not_found'
  mission?: DispatchMission
}

/**
 * 派遣领取结果
 */
export interface CollectResult {
  success: boolean
  reason?: 'not_completed' | 'not_found'
  rewards?: DispatchReward
}
```

### 2.6 Reducer Action 类型

```typescript
/**
 * EconomyAction
 */
export type EconomyAction =
  | { type: 'TRACK_EARN'; amount: number; source: GoldSource; note?: string; now: number }
  | { type: 'TRACK_SPEND'; amount: number; sink: GoldSink; note?: string; now: number }
  | { type: 'RESET_DAILY'; date: string }
  | { type: 'LOAD_STATE'; state: EconomyState }

/**
 * DispatchAction
 */
export type DispatchAction =
  | { type: 'START_MISSION'; mission: DispatchMission; now: number }
  | { type: 'COMPLETE_MISSION'; missionId: string; now: number }
  | { type: 'COLLECT_MISSION'; missionId: string }
  | { type: 'SPEED_UP_MISSION'; missionId: string; now: number }
  | { type: 'RESET_DAILY'; date: string }
  | { type: 'LOAD_STATE'; state: DispatchState }
```

### 2.7 Context 暴露接口

```typescript
/**
 * EconomyContextValue
 */
export interface EconomyContextValue {
  state: EconomyState
  /** 获取经济统计快照 */
  getStats: (currentGold: number) => EconomyStats
  /** 获取平衡检查结果 */
  getBalanceCheck: () => BalanceCheckResult
  /** 记录金币产出 */
  trackEarn: (amount: number, source: GoldSource, note?: string) => void
  /** 记录金币消耗 */
  trackSpend: (amount: number, sink: GoldSink, note?: string) => void
  /** 获取最近 N 条流水 */
  getRecentLogs: (count: number) => EconomyLogEntry[]
}

/**
 * DispatchContextValue
 */
export interface DispatchContextValue {
  state: DispatchState
  /** 当前可用派遣槽位数 */
  availableSlots: number
  /** 派遣任务定义列表 */
  missionDefs: DispatchMissionDef[]
  /** 发起派遣 */
  startMission: (missionType: DispatchMissionType, petId: string, petRarity: RarityTier) => DispatchResult
  /** 领取派遣奖励 */
  collectMission: (missionId: string) => CollectResult
  /** 加速派遣（花费金币立即完成） */
  speedUpMission: (missionId: string) => { success: boolean; reason?: 'insufficient_gold' | 'not_found' | 'already_completed' }
  /** 检查并结算已完成的任务 */
  checkCompleted: () => void
  /** 获取宠物的当前派遣任务（判断是否空闲） */
  getPetMission: (petId: string) => DispatchMission | null
  /** 获取倒计时（秒） */
  getMissionCountdown: (missionId: string) => number
}
```

---

## 3. 常量定义 (`economy/constants.ts`)

```typescript
import type { DispatchMissionType, GoldSource, GoldSink, DispatchMissionDef } from './types'
import type { RarityTier } from '../types'

// ===== 经济追踪 =====

/** localStorage 存储 key */
export const ECONOMY_STORAGE_KEY = 'animal_poke_economy'

/** 流水记录最大保留条数 */
export const MAX_LOG_ENTRIES = 200

/** 经济健康区间 */
export const HEALTHY_RATIO_MIN = 1.2
export const HEALTHY_RATIO_MAX = 3.0

// ===== 派遣系统 =====

/** localStorage 存储 key */
export const DISPATCH_STORAGE_KEY = 'animal_poke_dispatch'

/** 派遣定时检查间隔（毫秒），每分钟检查一次 */
export const DISPATCH_CHECK_INTERVAL_MS = 60_000

/** 派遣加速费用（金币） */
export const DISPATCH_SPEEDUP_COST = 50

/** 每日派遣次数上限 */
export const DAILY_DISPATCH_LIMIT = 6

/** 稀有度金币倍率（与 BATTLE_GOLD_REWARDS 对齐） */
export const DISPATCH_RARITY_GOLD_MULTIPLIER: Record<RarityTier, number> = {
  common: 1.0,
  uncommon: 1.67,   // 15 × 1.67 ≈ 25
  rare: 2.67,       // 15 × 2.67 ≈ 40
  epic: 4.67,       // 15 × 4.67 ≈ 70
  legendary: 8.0,   // 15 × 8.0 = 120
}

/** 派遣亲密度奖励 */
export const DISPATCH_AFFINITY_REWARD = 2

/** 派遣道具掉落概率 */
export const DISPATCH_ITEM_DROP_RATE = 0.20

/** 派遣道具掉落池 */
export const DISPATCH_ITEM_DROP_POOL: { itemId: string; weight: number }[] = [
  { itemId: 'toy_ball', weight: 35 },
  { itemId: 'food_pack', weight: 30 },
  { itemId: 'bait', weight: 20 },
  { itemId: 'cold_medicine', weight: 10 },
  { itemId: 'premium_toy_ball', weight: 5 },
]

// ===== 派遣任务定义 =====

export const DISPATCH_MISSION_DEFS: DispatchMissionDef[] = [
  {
    type: 'quick',
    name: '快速探索',
    description: '派宠物在附近快速搜寻，30 分钟返回',
    icon: '⚡',
    durationMin: 30,
    staminaCost: 20,
    baseGold: 15,
    itemDropRate: 0.15,
    itemDropPool: DISPATCH_ITEM_DROP_POOL,
  },
  {
    type: 'standard',
    name: '标准探索',
    description: '派宠物去周边探索，1 小时返回',
    icon: '🧭',
    durationMin: 60,
    staminaCost: 20,
    baseGold: 25,
    itemDropRate: 0.20,
    itemDropPool: DISPATCH_ITEM_DROP_POOL,
  },
  {
    type: 'deep',
    name: '深度探索',
    description: '派宠物深入未知区域，2 小时返回，高收益',
    icon: '🗺️',
    durationMin: 120,
    staminaCost: 20,
    baseGold: 40,
    itemDropRate: 0.25,
    itemDropPool: DISPATCH_ITEM_DROP_POOL,
  },
]

/** 派遣任务定义映射（type → def） */
export const DISPATCH_MISSION_MAP: Record<DispatchMissionType, DispatchMissionDef> =
  Object.fromEntries(DISPATCH_MISSION_DEFS.map(d => [d.type, d])) as Record<DispatchMissionType, DispatchMissionDef>

// ===== 派遣槽位计算 =====

/**
 * 根据等级计算派遣槽位上限
 * Lv1-2: 1, Lv3-5: 2, Lv6-8: 3, Lv9-10: 4
 */
export function getMaxDispatchSlots(level: number): number {
  return Math.min(1 + Math.floor(level / 3), 4)
}

// ===== 产出/消耗初始分布 =====

export const INITIAL_EARNED_BY_SOURCE: Record<GoldSource, number> = {
  capture: 0, battle_win: 0, battle_draw: 0, battle_lose: 0,
  checkin: 0, levelup: 0, dispatch: 0, region_rank: 0, achievement: 0, other: 0,
}

export const INITIAL_SPENT_BY_SINK: Record<GoldSink, number> = {
  shop_buy: 0, stamina_potion: 0, dispatch_speedup: 0, battle_extra: 0, other: 0,
}

// ===== 一天的毫秒数 =====

export const DAY_MS = 24 * 60 * 60 * 1000
```

---

## 4. 纯逻辑函数 (`economy/logic.ts`)

### 4.1 获取今日日期标记

```typescript
/**
 * 获取今日日期标记（自然日，格式 'YYYY-MM-DD'）
 */
export function getTodayString(now?: number): string {
  const date = now ? new Date(now) : new Date()
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

/** 检查是否需要重置每日统计 */
export function shouldResetDaily(date: string, now?: number): boolean {
  return date !== getTodayString(now)
}
```

### 4.2 计算派遣奖励

```typescript
import { DISPATCH_RARITY_GOLD_MULTIPLIER, DISPATCH_AFFINITY_REWARD, DISPATCH_ITEM_DROP_RATE } from './constants'
import { DISPATCH_MISSION_MAP } from './constants'
import type { DispatchMissionType, DispatchReward } from './types'
import type { RarityTier } from '../types'

/**
 * 计算派遣任务奖励（纯函数）
 *
 * 金币 = 任务基准金币 × 稀有度倍率
 * 道具 = 按概率掉落（使用传入的 rng 确保可测试）
 * 亲密度 = 固定 2
 *
 * @param missionType 任务类型
 * @param rarity 宠物稀有度
 * @param rng 随机数生成器（默认 Math.random）
 * @returns 派遣奖励
 */
export function calculateDispatchReward(
  missionType: DispatchMissionType,
  rarity: RarityTier,
  rng: () => number = Math.random
): DispatchReward {
  const def = DISPATCH_MISSION_MAP[missionType]
  const gold = Math.round(def.baseGold * DISPATCH_RARITY_GOLD_MULTIPLIER[rarity])

  // 道具掉落判定
  let droppedItem: string | undefined
  if (rng() < def.itemDropRate) {
    droppedItem = rollDispatchItemDrop(def.itemDropPool, rng)
  }

  return {
    gold,
    droppedItem,
    affinity: DISPATCH_AFFINITY_REWARD,
  }
}

/**
 * 加权随机选取掉落道具
 */
export function rollDispatchItemDrop(
  pool: { itemId: string; weight: number }[],
  rng: () => number = Math.random
): string {
  const total = pool.reduce((sum, item) => sum + item.weight, 0)
  let rand = rng() * total
  for (const item of pool) {
    rand -= item.weight
    if (rand <= 0) return item.itemId
  }
  return pool[pool.length - 1].itemId
}
```

### 4.3 创建派遣任务实例

```typescript
import { DISPATCH_MISSION_MAP } from './constants'
import type { DispatchMission, DispatchMissionType } from './types'
import type { RarityTier } from '../types'

/**
 * 创建派遣任务实例
 * @param missionType 任务类型
 * @param petId 宠物 ID
 * @param petRarity 宠物稀有度
 * @param city 绑定城市
 * @param now 当前时间戳
 * @param rng 随机数生成器
 * @returns 派遣任务实例
 */
export function createMission(
  missionType: DispatchMissionType,
  petId: string,
  petRarity: RarityTier,
  city: string,
  now: number,
  rng: () => number = Math.random
): DispatchMission {
  const def = DISPATCH_MISSION_MAP[missionType]
  const durationMs = def.durationMin * 60 * 1000
  const rewards = calculateDispatchReward(missionType, petRarity, rng)

  return {
    id: `dispatch_${now}_${Math.floor(rng() * 10000)}`,
    type: missionType,
    petId,
    petRarity,
    city,
    startTime: now,
    endTime: now + durationMs,
    status: 'active',
    rewards,
  }
}
```

### 4.4 检查任务是否完成

```typescript
import type { DispatchMission } from './types'

/**
 * 检查派遣任务是否已完成（时间到期）
 */
export function isMissionCompleted(mission: DispatchMission, now: number = Date.now()): boolean {
  return mission.status === 'active' && now >= mission.endTime
}
```

### 4.5 获取任务倒计时

```typescript
/**
 * 获取派遣任务剩余倒计时（秒）
 * 已完成返回 0
 */
export function getMissionCountdown(mission: DispatchMission, now: number = Date.now()): number {
  if (now >= mission.endTime) return 0
  return Math.ceil((mission.endTime - now) / 1000)
}
```

### 4.6 计算可用派遣槽位

```typescript
import { getMaxDispatchSlots } from './constants'
import type { DispatchState } from './types'

/**
 * 计算当前可用派遣槽位数
 * 可用 = 上限 - 活跃任务数（含已完成未领取）
 */
export function getAvailableSlots(state: DispatchState, level: number): number {
  const maxSlots = getMaxDispatchSlots(level)
  const activeCount = state.missions.filter(m => m.status !== 'collected').length
  return Math.max(0, maxSlots - activeCount)
}
```

### 4.7 检查宠物是否正在派遣

```typescript
import type { DispatchState } from './types'

/**
 * 获取宠物的当前派遣任务
 * @returns 派遣任务或 null（空闲）
 */
export function getPetMission(state: DispatchState, petId: string): DispatchMission | null {
  return state.missions.find(m => m.petId === petId && m.status !== 'collected') ?? null
}
```

### 4.8 经济平衡检查

```typescript
import { HEALTHY_RATIO_MIN, HEALTHY_RATIO_MAX } from './constants'
import type { EconomyState, BalanceCheckResult } from './types'

/**
 * 经济平衡检查
 *
 * ratio = totalEarned / totalSpent
 * - ratio < 1.2 → deflation（通缩，消耗不足或产出过低）
 * - 1.2 ≤ ratio ≤ 3.0 → healthy（健康）
 * - ratio > 3.0 → inflation（通胀，产出过高或消耗不足）
 *
 * @param state 经济状态
 * @returns 平衡检查结果
 */
export function balanceCheck(state: EconomyState): BalanceCheckResult {
  const ratio = state.totalSpent > 0
    ? state.totalEarned / state.totalSpent
    : Infinity

  let status: BalanceCheckResult['status']
  let isHealthy: boolean
  let suggestion: string

  if (ratio === Infinity) {
    status = 'inflation'
    isHealthy = false
    suggestion = '尚无消耗记录，建议增加道具购买或派遣加速'
  } else if (ratio < HEALTHY_RATIO_MIN) {
    status = 'deflation'
    isHealthy = false
    suggestion = `产出消耗比 ${ratio.toFixed(2)} 偏低，消耗过多或产出不足，建议增加派遣或提高捕获频率`
  } else if (ratio > HEALTHY_RATIO_MAX) {
    status = 'inflation'
    isHealthy = false
    suggestion = `产出消耗比 ${ratio.toFixed(2)} 偏高，产出过剩，建议增加商店购买或体力药剂消耗`
  } else {
    status = 'healthy'
    isHealthy = true
    suggestion = `经济健康，产出消耗比 ${ratio.toFixed(2)}`
  }

  return { ratio, isHealthy, status, suggestion }
}
```

### 4.9 统计产出来源分布

```typescript
import { INITIAL_EARNED_BY_SOURCE, INITIAL_SPENT_BY_SINK } from './constants'
import type { EconomyState, EconomyStats, GoldSource, GoldSink } from './types'

/**
 * 从流水日志中统计产出来源分布
 */
export function calculateEarnedBySource(state: EconomyState): Record<GoldSource, number> {
  const result = { ...INITIAL_EARNED_BY_SOURCE }
  for (const log of state.logs) {
    if (log.type === 'earn') {
      result[log.category as GoldSource] = (result[log.category as GoldSource] ?? 0) + log.amount
    }
  }
  return result
}

/**
 * 从流水日志中统计消耗去向分布
 */
export function calculateSpentBySink(state: EconomyState): Record<GoldSink, number> {
  const result = { ...INITIAL_SPENT_BY_SINK }
  for (const log of state.logs) {
    if (log.type === 'spend') {
      result[log.category as GoldSink] = (result[log.category as GoldSink] ?? 0) + log.amount
    }
  }
  return result
}
```

### 4.10 获取经济统计快照

```typescript
/**
 * 生成经济统计快照
 * @param state 经济状态
 * @param currentGold 当前金币余额（从 StaminaContext 读取）
 * @returns 统计快照
 */
export function getEconomyStats(state: EconomyState, currentGold: number): EconomyStats {
  return {
    currentGold,
    totalEarned: state.totalEarned,
    totalSpent: state.totalSpent,
    netFlow: state.totalEarned - state.totalSpent,
    todayEarned: state.todayEarned,
    todaySpent: state.todaySpent,
    todayNetFlow: state.todayEarned - state.todaySpent,
    earnedBySource: calculateEarnedBySource(state),
    spentBySink: calculateSpentBySink(state),
  }
}
```

### 4.11 加速费用计算

```typescript
import { DISPATCH_SPEEDUP_COST } from './constants'
import type { DispatchMission } from './types'

/**
 * 计算加速费用
 * 当前为固定费用 50 金币（后续可按剩余时间递减）
 */
export function getSpeedUpCost(mission: DispatchMission): number {
  return DISPATCH_SPEEDUP_COST
}
```

---

## 5. EconomyContext 设计 (`economy/EconomyContext.tsx`)

### 5.1 初始状态与持久化

```typescript
import React, { createContext, useReducer, useEffect, useCallback, useMemo } from 'react'
import { useStamina } from '../stamina/useStamina'
import {
  ECONOMY_STORAGE_KEY,
  MAX_LOG_ENTRIES,
  INITIAL_EARNED_BY_SOURCE,
  INITIAL_SPENT_BY_SINK,
} from './constants'
import type {
  EconomyState, EconomyAction, EconomyContextValue,
  EconomyStats, BalanceCheckResult, EconomyLogEntry,
  GoldSource, GoldSink,
} from './types'
import {
  getTodayString, shouldResetDaily,
  balanceCheck, getEconomyStats,
  calculateEarnedBySource, calculateSpentBySink,
} from './logic'

/** 默认初始状态 */
const initialState: EconomyState = {
  totalEarned: 0,
  totalSpent: 0,
  logs: [],
  nextLogId: 1,
  todayEarned: 0,
  todaySpent: 0,
  todayDate: getTodayString(),
}

/** 从 localStorage 加载状态 */
function loadInitialState(): EconomyState {
  try {
    const saved = localStorage.getItem(ECONOMY_STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as EconomyState
      // 基本字段校验
      if (
        typeof parsed.totalEarned !== 'number' ||
        typeof parsed.totalSpent !== 'number' ||
        !Array.isArray(parsed.logs) ||
        typeof parsed.nextLogId !== 'number' ||
        typeof parsed.todayDate !== 'string'
      ) {
        throw new Error('经济存档字段校验失败')
      }
      // 加载时检查每日重置
      if (shouldResetDaily(parsed.todayDate)) {
        parsed.todayEarned = 0
        parsed.todaySpent = 0
        parsed.todayDate = getTodayString()
      }
      return parsed
    }
  } catch (e) {
    console.warn('加载经济存档失败，使用默认值:', e)
  }
  return initialState
}
```

### 5.2 Reducer

```typescript
function economyReducer(state: EconomyState, action: EconomyAction): EconomyState {
  switch (action.type) {
    case 'TRACK_EARN': {
      const entry: EconomyLogEntry = {
        id: state.nextLogId,
        type: 'earn',
        amount: action.amount,
        category: action.source,
        timestamp: action.now,
        note: action.note,
      }
      // FIFO 淘汰：超过 MAX_LOG_ENTRIES 时移除最旧的
      const logs = [...state.logs, entry]
      if (logs.length > MAX_LOG_ENTRIES) {
        logs.splice(0, logs.length - MAX_LOG_ENTRIES)
      }
      return {
        ...state,
        totalEarned: state.totalEarned + action.amount,
        todayEarned: state.todayEarned + action.amount,
        logs,
        nextLogId: state.nextLogId + 1,
      }
    }

    case 'TRACK_SPEND': {
      const entry: EconomyLogEntry = {
        id: state.nextLogId,
        type: 'spend',
        amount: action.amount,
        category: action.sink,
        timestamp: action.now,
        note: action.note,
      }
      const logs = [...state.logs, entry]
      if (logs.length > MAX_LOG_ENTRIES) {
        logs.splice(0, logs.length - MAX_LOG_ENTRIES)
      }
      return {
        ...state,
        totalSpent: state.totalSpent + action.amount,
        todaySpent: state.todaySpent + action.amount,
        logs,
        nextLogId: state.nextLogId + 1,
      }
    }

    case 'RESET_DAILY': {
      return {
        ...state,
        todayEarned: 0,
        todaySpent: 0,
        todayDate: action.date,
      }
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
export const EconomyContext = createContext<EconomyContextValue | null>(null)

export const EconomyProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const stamina = useStamina()
  const [state, dispatch] = useReducer(economyReducer, undefined, loadInitialState)

  // localStorage 持久化
  useEffect(() => {
    localStorage.setItem(ECONOMY_STORAGE_KEY, JSON.stringify(state))
  }, [state])

  // 每日重置检查
  useEffect(() => {
    if (shouldResetDaily(state.todayDate)) {
      dispatch({ type: 'RESET_DAILY', date: getTodayString() })
    }
  }, [state.todayDate])

  // ---- 操作函数 ----

  const trackEarn = useCallback((amount: number, source: GoldSource, note?: string) => {
    if (amount <= 0) return
    dispatch({ type: 'TRACK_EARN', amount, source, note, now: Date.now() })
  }, [])

  const trackSpend = useCallback((amount: number, sink: GoldSink, note?: string) => {
    if (amount <= 0) return
    dispatch({ type: 'TRACK_SPEND', amount, sink, note, now: Date.now() })
  }, [])

  // ---- 查询函数 ----

  const getStats = useCallback((currentGold: number): EconomyStats => {
    return getEconomyStats(state, currentGold)
  }, [state])

  const getBalanceCheck = useCallback((): BalanceCheckResult => {
    return balanceCheck(state)
  }, [state])

  const getRecentLogs = useCallback((count: number): EconomyLogEntry[] => {
    return [...state.logs].reverse().slice(0, count)
  }, [state.logs])

  const value = useMemo<EconomyContextValue>(() => ({
    state,
    getStats,
    getBalanceCheck,
    trackEarn,
    trackSpend,
    getRecentLogs,
  }), [state, getStats, getBalanceCheck, trackEarn, trackSpend, getRecentLogs])

  return (
    <EconomyContext.Provider value={value}>
      {children}
    </EconomyContext.Provider>
  )
}
```

### 5.4 useEconomy Hook (`economy/useEconomy.ts`)

```typescript
import { useContext } from 'react'
import { EconomyContext } from './EconomyContext'
import type { EconomyContextValue } from './types'

/** 自定义 Hook，封装 EconomyContext 消费 */
export function useEconomy(): EconomyContextValue {
  const context = useContext(EconomyContext)
  if (!context) {
    throw new Error('useEconomy 必须在 EconomyProvider 内使用')
  }
  return context
}
```

---

## 6. DispatchContext 设计 (`economy/DispatchContext.tsx`)

### 6.1 初始状态与持久化

```typescript
import React, { createContext, useReducer, useEffect, useCallback, useMemo } from 'react'
import { useStamina } from '../stamina/useStamina'
import { useEconomy } from './useEconomy'
import { useShop } from '../shop/useShop'
import { useLbs } from '../lbs/useLbs'
import {
  DISPATCH_STORAGE_KEY,
  DISPATCH_CHECK_INTERVAL_MS,
  DISPATCH_SPEEDUP_COST,
  DISPATCH_MISSION_DEFS,
  getMaxDispatchSlots,
} from './constants'
import type {
  DispatchState, DispatchAction, DispatchContextValue,
  DispatchResult, CollectResult,
  DispatchMissionType, DispatchMission,
} from './types'
import type { RarityTier } from '../types'
import {
  getTodayString, shouldResetDaily,
  createMission, isMissionCompleted,
  getAvailableSlots, getPetMission,
  getMissionCountdown, getSpeedUpCost,
} from './logic'

/** 默认初始状态 */
const initialState: DispatchState = {
  missions: [],
  todayDispatchCount: 0,
  todayDate: getTodayString(),
}

/** 从 localStorage 加载状态 */
function loadInitialState(): DispatchState {
  try {
    const saved = localStorage.getItem(DISPATCH_STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as DispatchState
      if (
        !Array.isArray(parsed.missions) ||
        typeof parsed.todayDispatchCount !== 'number' ||
        typeof parsed.todayDate !== 'string'
      ) {
        throw new Error('派遣存档字段校验失败')
      }
      if (shouldResetDaily(parsed.todayDate)) {
        parsed.todayDispatchCount = 0
        parsed.todayDate = getTodayString()
      }
      return parsed
    }
  } catch (e) {
    console.warn('加载派遣存档失败，使用默认值:', e)
  }
  return initialState
}
```

### 6.2 Reducer

```typescript
function dispatchReducer(state: DispatchState, action: DispatchAction): DispatchState {
  switch (action.type) {
    case 'START_MISSION': {
      return {
        ...state,
        missions: [...state.missions, action.mission],
        todayDispatchCount: state.todayDispatchCount + 1,
      }
    }

    case 'COMPLETE_MISSION': {
      return {
        ...state,
        missions: state.missions.map(m =>
          m.id === action.missionId ? { ...m, status: 'completed' as const } : m
        ),
      }
    }

    case 'COLLECT_MISSION': {
      return {
        ...state,
        missions: state.missions.map(m =>
          m.id === action.missionId ? { ...m, status: 'collected' as const } : m
        ),
      }
    }

    case 'SPEED_UP_MISSION': {
      return {
        ...state,
        missions: state.missions.map(m =>
          m.id === action.missionId
            ? { ...m, status: 'completed' as const, endTime: action.now }
            : m
        ),
      }
    }

    case 'RESET_DAILY': {
      return {
        ...state,
        todayDispatchCount: 0,
        todayDate: action.date,
      }
    }

    case 'LOAD_STATE': {
      return action.state
    }

    default:
      return state
  }
}
```

### 6.3 Provider 实现

```typescript
export const DispatchContext = createContext<DispatchContextValue | null>(null)

export const DispatchProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const stamina = useStamina()
  const economy = useEconomy()
  const shop = useShop()
  const lbs = useLbs()

  const [state, dispatch] = useReducer(dispatchReducer, undefined, loadInitialState)

  // localStorage 持久化
  useEffect(() => {
    localStorage.setItem(DISPATCH_STORAGE_KEY, JSON.stringify(state))
  }, [state])

  // 每日重置检查
  useEffect(() => {
    if (shouldResetDaily(state.todayDate)) {
      dispatch({ type: 'RESET_DAILY', date: getTodayString() })
    }
  }, [state.todayDate])

  // 定时检查已完成任务
  useEffect(() => {
    const interval = setInterval(() => {
      const now = Date.now()
      for (const mission of state.missions) {
        if (isMissionCompleted(mission, now)) {
          dispatch({ type: 'COMPLETE_MISSION', missionId: mission.id, now })
        }
      }
    }, DISPATCH_CHECK_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [state.missions])

  // 页面可见性恢复时检查
  useEffect(() => {
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        const now = Date.now()
        for (const mission of state.missions) {
          if (isMissionCompleted(mission, now)) {
            dispatch({ type: 'COMPLETE_MISSION', missionId: mission.id, now })
          }
        }
      }
    }
    document.addEventListener('visibilitychange', handleVisibility)
    return () => document.removeEventListener('visibilitychange', handleVisibility)
  }, [state.missions])

  // 首次加载时检查（处理离线期间完成的任务）
  useEffect(() => {
    const now = Date.now()
    for (const mission of state.missions) {
      if (isMissionCompleted(mission, now)) {
        dispatch({ type: 'COMPLETE_MISSION', missionId: mission.id, now })
      }
    }
  }, []) // 仅首次加载

  // ---- 派生值 ----

  const availableSlots = useMemo(
    () => getAvailableSlots(state, stamina.state.level),
    [state.missions, stamina.state.level]
  )

  // ---- 操作函数 ----

  const startMission = useCallback(
    (missionType: DispatchMissionType, petId: string, petRarity: RarityTier): DispatchResult => {
      // 检查槽位
      if (availableSlots <= 0) {
        return { success: false, reason: 'no_slots' }
      }

      // 检查宠物是否正在派遣
      const existing = getPetMission(state, petId)
      if (existing) {
        return { success: false, reason: 'pet_busy' }
      }

      // 检查体力
      const def = DISPATCH_MISSION_DEFS.find(d => d.type === missionType)!
      if (stamina.state.currentStamina < def.staminaCost) {
        return { success: false, reason: 'insufficient_stamina' }
      }

      // 扣体力
      stamina.consumeStamina(def.staminaCost)

      // 创建任务
      const now = Date.now()
      const city = lbs.state.cityName || '未知'
      const mission = createMission(missionType, petId, petRarity, city, now)

      dispatch({ type: 'START_MISSION', mission, now })

      return { success: true, mission }
    },
    [availableSlots, state, stamina, lbs.state.cityName]
  )

  const collectMission = useCallback((missionId: string): CollectResult => {
    const mission = state.missions.find(m => m.id === missionId)
    if (!mission) {
      return { success: false, reason: 'not_found' }
    }
    if (mission.status !== 'completed') {
      return { success: false, reason: 'not_completed' }
    }

    // 发放奖励
    stamina.addGold(mission.rewards.gold)
    economy.trackEarn(mission.rewards.gold, 'dispatch', `派遣: ${mission.type}`)

    // 道具掉落
    if (mission.rewards.droppedItem) {
      shop.addItem(mission.rewards.droppedItem as any)
    }

    dispatch({ type: 'COLLECT_MISSION', missionId })

    return { success: true, rewards: mission.rewards }
  }, [state.missions, stamina, economy, shop])

  const speedUpMission = useCallback((missionId: string): {
    success: boolean
    reason?: 'insufficient_gold' | 'not_found' | 'already_completed'
  } => {
    const mission = state.missions.find(m => m.id === missionId)
    if (!mission) {
      return { success: false, reason: 'not_found' }
    }
    if (mission.status !== 'active') {
      return { success: false, reason: 'already_completed' }
    }

    const cost = getSpeedUpCost(mission)
    if (stamina.state.gold < cost) {
      return { success: false, reason: 'insufficient_gold' }
    }

    // 扣金币
    stamina.addGold(-cost)
    economy.trackSpend(cost, 'dispatch_speedup', `加速: ${mission.type}`)

    // 立即完成
    dispatch({ type: 'SPEED_UP_MISSION', missionId, now: Date.now() })

    return { success: true }
  }, [state.missions, stamina, economy])

  const checkCompleted = useCallback(() => {
    const now = Date.now()
    for (const mission of state.missions) {
      if (isMissionCompleted(mission, now)) {
        dispatch({ type: 'COMPLETE_MISSION', missionId: mission.id, now })
      }
    }
  }, [state.missions])

  const getPetMissionFn = useCallback((petId: string): DispatchMission | null => {
    return getPetMission(state, petId)
  }, [state])

  const getMissionCountdownFn = useCallback((missionId: string): number => {
    const mission = state.missions.find(m => m.id === missionId)
    if (!mission) return 0
    return getMissionCountdown(mission)
  }, [state.missions])

  const value = useMemo<DispatchContextValue>(() => ({
    state,
    availableSlots,
    missionDefs: DISPATCH_MISSION_DEFS,
    startMission,
    collectMission,
    speedUpMission,
    checkCompleted,
    getPetMission: getPetMissionFn,
    getMissionCountdown: getMissionCountdownFn,
  }), [
    state, availableSlots, startMission, collectMission, speedUpMission,
    checkCompleted, getPetMissionFn, getMissionCountdownFn,
  ])

  return (
    <DispatchContext.Provider value={value}>
      {children}
    </DispatchContext.Provider>
  )
}
```

### 6.4 useDispatch Hook (`economy/useDispatch.ts`)

```typescript
import { useContext } from 'react'
import { DispatchContext } from './DispatchContext'
import type { DispatchContextValue } from './types'

/** 自定义 Hook，封装 DispatchContext 消费 */
export function useDispatch(): DispatchContextValue {
  const context = useContext(DispatchContext)
  if (!context) {
    throw new Error('useDispatch 必须在 DispatchProvider 内使用')
  }
  return context
}
```

---

## 7. 现有代码修改

### 7.1 `stamina/StaminaContext.tsx` — addGold 增加可选 source 参数

```typescript
// types.ts — StaminaAction.ADD_GOLD 增加可选 source
| { type: 'ADD_GOLD'; amount: number; source?: string }

// StaminaContext.tsx — addGold 传递 source（仅用于调试/日志，不影响逻辑）
const addGold = useCallback((amount: number) => {
  dispatch({ type: 'ADD_GOLD', amount })
}, [])
// 注意：addGold 签名不变，追踪由 EconomyContext 显式调用 trackEarn/trackSpend 完成
```

> **决策**：`StaminaContext.addGold` 签名不变。追踪通过 `EconomyContext.trackEarn/trackSpend` 显式调用，在各产出/消耗点完成。这样避免修改 StaminaContext 的任何代码，降低风险。

### 7.2 `shop/ShopContext.tsx` — 购买和签到后追踪

```typescript
import { useEconomy } from '../economy/useEconomy'

// ShopProvider 内：
const economy = useEconomy()

// buyItem 中，购买成功后追踪消耗：
const buyItem = useCallback((itemId: ItemId): BuyResult => {
  // ... 现有逻辑 ...
  if (result.success) {
    stamina.addGold(-def.price)
    // 追踪消耗
    economy.trackSpend(def.price, 'shop_buy', `购买: ${def.name}`)
  }
  return result
}, [stamina, state.dailyPurchases, economy])

// checkIn 中，签到成功后追踪产出：
const checkIn = useCallback((): CheckInResult => {
  // ... 现有逻辑 ...
  if (result.success) {
    stamina.addGold(result.reward)
    // 追踪产出
    economy.trackEarn(result.reward, 'checkin', `签到第${result.newStreak}天`)
  }
  return result
}, [stamina, state.checkIn, economy])

// buyStaminaPotion（委托给 StaminaContext）的追踪在 ShopContext.buyItem('stamina_potion') 中处理：
if (itemId === 'stamina_potion') {
  const result = stamina.buyStaminaPotion()
  if (result.success) {
    dispatch({ type: 'BUY_ITEM', itemId })
    // 追踪体力药剂消耗
    economy.trackSpend(POTION_PRICE, 'stamina_potion', '购买体力药剂')
    return { ... }
  }
  return { ... }
}
```

### 7.3 `battle/BattleContext.tsx` — 战斗结算后追踪

```typescript
import { useEconomy } from '../economy/useEconomy'

// BattleProvider 内：
const economy = useEconomy()

// finishBattle 中追踪战斗奖励：
const finishBattle = useCallback(() => {
  if (state.rewards) {
    stamina.addGold(state.rewards.gold)
    // 追踪战斗产出
    const source = state.result === 'win' ? 'battle_win'
      : state.result === 'draw' ? 'battle_draw'
      : 'battle_lose'
    economy.trackEarn(state.rewards.gold, source, `战斗${state.result ?? ''}`)

    if (state.rewards.droppedItem) {
      shop.addItem(state.rewards.droppedItem as any)
    }
  }
  dispatch({ type: 'RESET' })
}, [state.rewards, state.result, stamina, shop, economy])
```

### 7.4 `App.tsx` — Provider 嵌套 + 捕获追踪 + 派遣 tab

```typescript
import { EconomyProvider } from './economy/EconomyContext'
import { DispatchProvider } from './economy/DispatchContext'
import { useEconomy } from './economy/useEconomy'

// AppInner 内：
const economy = useEconomy()

// handleCaptureSuccess 中追踪捕获金币：
const handleCaptureSuccess = useCallback((entry: CardEntry) => {
  addAnimal(entry)
  addCapture(1)
  const goldDrop = Math.floor(Math.random() * 41) + 10
  addGold(goldDrop)
  // 追踪捕获产出
  economy.trackEarn(goldDrop, 'capture', `捕获 ${entry.no}`)
  setPendingPhoto(null)
  setActiveTab('collection')
}, [addAnimal, addCapture, addGold, economy])

// 升级奖励追踪：在 addCapture 返回 LevelUpResult 后追踪
const handleCaptureSuccess = useCallback((entry: CardEntry) => {
  addAnimal(entry)
  const levelUpResult = addCapture(1)
  const goldDrop = Math.floor(Math.random() * 41) + 10
  addGold(goldDrop)
  economy.trackEarn(goldDrop, 'capture', `捕获 ${entry.no}`)
  // 升级奖励追踪
  if (levelUpResult.leveledUp) {
    economy.trackEarn(levelUpResult.rewardGold, 'levelup', `升级至 Lv.${levelUpResult.newLevel}`)
  }
  setPendingPhoto(null)
  setActiveTab('collection')
}, [addAnimal, addCapture, addGold, economy])

// Provider 嵌套：
const App: React.FC = () => {
  return (
    <LbsProvider>
      <StaminaProvider>
        <EconomyProvider>
          <WeatherProvider>
            <ShopProvider>
              <StatusProvider>
                <DispatchProvider>
                  <BattleProvider>
                    <AppInner />
                  </BattleProvider>
                </DispatchProvider>
              </StatusProvider>
            </ShopProvider>
          </WeatherProvider>
        </EconomyProvider>
      </StaminaProvider>
    </LbsProvider>
  )
}
```

### 7.5 `types.ts` — MainTab 新增 'dispatch'

```typescript
// 修改前：
export type MainTab = 'profile' | 'collection' | 'camera' | 'fight' | 'store'

// 修改后：
export type MainTab = 'profile' | 'collection' | 'camera' | 'fight' | 'store' | 'dispatch'
```

### 7.6 `components/TabBar.tsx` — 新增派遣 tab

```typescript
// 新增派遣 tab 配置
{ id: 'dispatch', icon: '🗺️', label: '派遣' }
```

---

## 8. UI 集成

### 8.1 经济看板 (`components/StoreScreen.tsx` 修改)

在商店界面顶部新增经济看板区域：

```typescript
import { useEconomy } from '../economy/useEconomy'

// StoreScreen 内：
const economy = useEconomy()
const stats = economy.getStats(stamina.state.gold)
const balance = economy.getBalanceCheck()

// 看板 UI：
<div style={styles.economyDashboard}>
  <div style={styles.dashboardRow}>
    <div style={styles.dashboardCard}>
      <span style={styles.cardLabel}>💰 当前金币</span>
      <span style={styles.cardValue}>{stats.currentGold}</span>
    </div>
    <div style={styles.dashboardCard}>
      <span style={styles.cardLabel}>📈 今日产出</span>
      <span style={{ ...styles.cardValue, color: 'var(--success)' }}>+{stats.todayEarned}</span>
    </div>
    <div style={styles.dashboardCard}>
      <span style={styles.cardLabel}>📉 今日消耗</span>
      <span style={{ ...styles.cardValue, color: 'var(--warn)' }}>-{stats.todaySpent}</span>
    </div>
  </div>
  <div style={styles.balanceBar}>
    <span style={{ color: balance.isHealthy ? 'var(--success)' : 'var(--warn)' }}>
      {balance.status === 'healthy' ? '✅' : '⚠️'} {balance.suggestion}
    </span>
  </div>
</div>
```

### 8.2 派遣任务界面 (`components/DispatchScreen.tsx` 新增)

```typescript
import { useDispatch } from '../economy/useDispatch'
import { useStamina } from '../stamina/useStamina'
import { useAnimalStore } from '../hooks/useAnimalStore'

const DispatchScreen: React.FC = () => {
  const dispatch = useDispatch()
  const stamina = useStamina()
  const { animals } = useAnimalStore()

  // 活跃任务列表
  const activeMissions = dispatch.state.missions.filter(m => m.status !== 'collected')

  // 可派遣宠物（未在派遣中的已解锁宠物）
  const availablePets = animals.filter(a => !dispatch.getPetMission(a.id))

  return (
    <div className="dispatch-screen">
      {/* 顶部状态栏 */}
      <div style={styles.statusBar}>
        <span>🗺️ 可用槽位: {dispatch.availableSlots}</span>
        <span>⚡ 体力: {stamina.state.currentStamina}/{stamina.maxStamina}</span>
        <span>📊 今日派遣: {dispatch.state.todayDispatchCount}</span>
      </div>

      {/* 活跃任务 */}
      {activeMissions.length > 0 && (
        <div style={styles.activeSection}>
          <h3>进行中的派遣</h3>
          {activeMissions.map(mission => (
            <MissionCard
              key={mission.id}
              mission={mission}
              countdown={dispatch.getMissionCountdown(mission.id)}
              onCollect={() => {
                const result = dispatch.collectMission(mission.id)
                if (result.success && result.rewards) {
                  alert(`领取成功！获得 ${result.rewards.gold} 金币`)
                }
              }}
              onSpeedUp={() => {
                const result = dispatch.speedUpMission(mission.id)
                if (!result.success) {
                  alert(result.reason === 'insufficient_gold' ? '金币不足' : '无法加速')
                }
              }}
            />
          ))}
        </div>
      )}

      {/* 任务类型选择 */}
      <div style={styles.missionTypes}>
        <h3>派遣任务</h3>
        {dispatch.missionDefs.map(def => (
          <MissionTypeCard
            key={def.type}
            def={def}
            disabled={dispatch.availableSlots <= 0 || stamina.state.currentStamina < def.staminaCost}
            pets={availablePets}
            onSelect={(petId, petRarity) => {
              const result = dispatch.startMission(def.type, petId, petRarity)
              if (!result.success) {
                alert({
                  no_slots: '没有可用槽位',
                  pet_busy: '宠物正在派遣中',
                  insufficient_stamina: '体力不足',
                  pet_not_found: '宠物不存在',
                }[result.reason ?? ''])
              }
            }}
          />
        ))}
      </div>
    </div>
  )
}
```

### 8.3 宠物详情派遣按钮 (`components/DetailPopup.tsx` 修改)

```typescript
import { useDispatch } from '../economy/useDispatch'

// DetailPopup 内：
const dispatchCtx = useDispatch()
const petMission = dispatchCtx.getPetMission(entry.id)

// 在操作按钮区域新增：
{!petMission && dispatchCtx.availableSlots > 0 && (
  <button
    className="btn btn-secondary"
    onClick={() => setActiveTab('dispatch')}
  >
    🗺️ 派遣
  </button>
)}
{petMission && (
  <div style={styles.dispatchStatus}>
    <span>🗺️ 派遣中: {dispatchCtx.missionDefs.find(d => d.type === petMission.type)?.name}</span>
    <span>⏰ {formatCountdown(dispatchCtx.getMissionCountdown(petMission.id))}</span>
  </div>
)}
```

### 8.4 TopBar 净收支显示 (`components/TopBar.tsx` 修改)

```typescript
import { useEconomy } from '../economy/useEconomy'

// TopBar 内：
const economy = useEconomy()
const stats = economy.getStats(stamina.state.gold)

// 金币旁显示今日净收支：
<div style={styles.goldDisplay}>
  <span>💰 {stamina.state.gold}</span>
  {stats.todayNetFlow !== 0 && (
    <span style={{
      fontSize: 10,
      color: stats.todayNetFlow > 0 ? 'var(--success)' : 'var(--warn)'
    }}>
      {stats.todayNetFlow > 0 ? '+' : ''}{stats.todayNetFlow} 今日
    </span>
  )}
</div>
```

---

## 9. 经济闭环流程图

```
┌─────────────────────────────────────────────────────────────────┐
│                    经济系统闭环                                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─ 产出 Sources ──────────────────────────────────────────┐   │
│  │                                                         │   │
│  │  捕获成功         10~50 金币/次     ← App.tsx 追踪      │   │
│  │  升级奖励         100~1000 金币     ← StaminaContext    │   │
│  │  战斗胜利         15~120 金币       ← BattleContext     │   │
│  │  战斗平局/失败    5~10 金币         ← BattleContext     │   │
│  │  每日签到         10~200 金币       ← ShopContext       │   │
│  │  派遣任务         15~120 金币/次    ← DispatchContext   │   │
│  │                                                         │   │
│  └─────────────────────────────────────────────────────────┘   │
│                          │                                      │
│                          ▼                                      │
│                ┌─────────────────┐                              │
│                │  StaminaContext  │                              │
│                │  state.gold      │ ← 金币唯一真实来源            │
│                └─────────────────┘                              │
│                          │                                      │
│                          ▼                                      │
│  ┌─ 消耗 Sinks ────────────────────────────────────────────┐   │
│  │                                                         │   │
│  │  商店购买道具     30~200 金币/件   ← ShopContext        │   │
│  │  体力药剂         150 金币/次     ← ShopContext         │   │
│  │  感冒药           200 金币/件     ← ShopContext         │   │
│  │  派遣加速         50 金币/次      ← DispatchContext     │   │
│  │  购买战斗场次     50 金币/场(预留) ← 后续 Issue          │   │
│  │                                                         │   │
│  └─────────────────────────────────────────────────────────┘   │
│                          │                                      │
│                          ▼                                      │
│                ┌─────────────────┐                              │
│                │  EconomyContext  │ ← 追踪层（统计/看板）        │
│                │  trackEarn       │                              │
│                │  trackSpend      │                              │
│                │  getStats        │                              │
│                │  balanceCheck    │                              │
│                └─────────────────┘                              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

┌─ 派遣系统流程 ─────────────────────────────────────────────────┐
│                                                               │
│  选择宠物 + 选择任务类型                                       │
│       │                                                       │
│       ▼                                                       │
│  检查：槽位 > 0？宠物空闲？体力 ≥ 20？                         │
│       ├─ 否 → 返回错误原因                                     │
│       └─ 是 → 扣 20 体力，创建派遣任务                         │
│                │                                               │
│                ▼                                               │
│         任务进行中（30min / 1h / 2h）                          │
│                │                                               │
│       ┌────────┴──────────────────────┐                       │
│       ▼                               ▼                       │
│  等待自然完成                    花费 50 金币加速               │
│  (定时检查 + 页面恢复检查)       (扣金币 + trackSpend)          │
│       │                               │                       │
│       └───────────┬───────────────────┘                       │
│                   ▼                                           │
│           任务完成（status='completed'）                       │
│                   │                                           │
│                   ▼                                           │
│           玩家点击「领取」                                     │
│       │                                                       │
│       ▼                                                       │
│  发放奖励：金币(addGold + trackEarn) + 道具(addItem)          │
│       │                                                       │
│       ▼                                                       │
│  任务归档（status='collected'）                                │
│                                                               │
└───────────────────────────────────────────────────────────────┘
```

---

## 10. 测试用例 (`economy/logic.test.ts`)

### 10.1 测试用例列表（共 20 个）

| # | 测试用例 | 测试函数 | 预期结果 |
|---|---------|---------|---------|
| 1 | getTodayString 返回正确格式 | `getTodayString` | 'YYYY-MM-DD' 格式 |
| 2 | shouldResetDaily 不同日期返回 true | `shouldResetDaily` | '2026-07-07' vs '2026-07-08' → true |
| 3 | shouldResetDaily 相同日期返回 false | `shouldResetDaily` | 同日 → false |
| 4 | calculateDispatchReward Common 快速探索 | `calculateDispatchReward` | gold=15, affinity=2 |
| 5 | calculateDispatchReward Legendary 深度探索 | `calculateDispatchReward` | gold=320 (40×8.0), affinity=2 |
| 6 | calculateDispatchReward 道具掉落 | `calculateDispatchReward` | rng mock < 0.20 → droppedItem 有值 |
| 7 | calculateDispatchReward 无道具掉落 | `calculateDispatchReward` | rng mock ≥ 0.20 → droppedItem undefined |
| 8 | rollDispatchItemDrop 加权随机 | `rollDispatchItemDrop` | 返回池中存在的 itemId |
| 9 | createMission 正确字段 | `createMission` | type/petId/startTime/endTime 正确 |
| 10 | createMission 结束时间计算 | `createMission` | endTime = startTime + durationMin × 60000 |
| 11 | isMissionCompleted 未到期返回 false | `isMissionCompleted` | now < endTime → false |
| 12 | isMissionCompleted 到期返回 true | `isMissionCompleted` | now ≥ endTime → true |
| 13 | getMissionCountdown 剩余秒数 | `getMissionCountdown` | 正确倒计时 |
| 14 | getMissionCountdown 已完成返回 0 | `getMissionCountdown` | now ≥ endTime → 0 |
| 15 | getAvailableSlots 空闲时返回上限 | `getAvailableSlots` | Lv5 无任务 → 2 |
| 16 | getAvailableSlots 有任务时减少 | `getAvailableSlots` | Lv5 有 1 任务 → 1 |
| 17 | getPetMission 空闲宠物返回 null | `getPetMission` | 无任务 → null |
| 18 | getPetMission 派遣中宠物返回任务 | `getPetMission` | 有任务 → DispatchMission |
| 19 | balanceCheck 健康区间 | `balanceCheck` | ratio=2.0 → healthy |
| 20 | balanceCheck 通胀 | `balanceCheck` | ratio=5.0 → inflation |
| 21 | balanceCheck 通缩 | `balanceCheck` | ratio=0.5 → deflation |
| 22 | balanceCheck 无消耗记录 | `balanceCheck` | totalSpent=0 → inflation, Infinity |
| 23 | getMaxDispatchSlots 等级映射 | `getMaxDispatchSlots` | Lv1→1, Lv3→2, Lv6→3, Lv9→4 |

### 10.2 测试代码骨架

```typescript
import { describe, it, expect } from 'vitest'
import {
  getTodayString,
  shouldResetDaily,
  calculateDispatchReward,
  rollDispatchItemDrop,
  createMission,
  isMissionCompleted,
  getMissionCountdown,
  getAvailableSlots,
  getPetMission,
  balanceCheck,
  getEconomyStats,
  calculateEarnedBySource,
  calculateSpentBySink,
} from './logic'
import { getMaxDispatchSlots } from './constants'
import type { EconomyState, DispatchState } from './types'
import type { RarityTier } from '../types'

const NOW = 1700000000000

// ===== 辅助函数 =====
function makeEconomyState(overrides: Partial<EconomyState> = {}): EconomyState {
  return {
    totalEarned: 0,
    totalSpent: 0,
    logs: [],
    nextLogId: 1,
    todayEarned: 0,
    todaySpent: 0,
    todayDate: getTodayString(NOW),
    ...overrides,
  }
}

function makeDispatchState(overrides: Partial<DispatchState> = {}): DispatchState {
  return {
    missions: [],
    todayDispatchCount: 0,
    todayDate: getTodayString(NOW),
    ...overrides,
  }
}

// ===== 日期工具测试 =====
describe('getTodayString', () => {
  it('返回 YYYY-MM-DD 格式', () => {
    const result = getTodayString(NOW)
    expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })
})

describe('shouldResetDaily', () => {
  it('不同日期返回 true', () => {
    expect(shouldResetDaily('2026-07-07', NOW)).toBe(true)
  })

  it('相同日期返回 false', () => {
    const today = getTodayString(NOW)
    expect(shouldResetDaily(today, NOW)).toBe(false)
  })
})

// ===== 派遣奖励计算 =====
describe('calculateDispatchReward', () => {
  it('Common 快速探索：gold=15, affinity=2', () => {
    const reward = calculateDispatchReward('quick', 'common', () => 0.99)
    expect(reward.gold).toBe(15)
    expect(reward.affinity).toBe(2)
  })

  it('Legendary 深度探索：gold=320', () => {
    const reward = calculateDispatchReward('deep', 'legendary', () => 0.99)
    expect(reward.gold).toBe(320) // 40 × 8.0
    expect(reward.affinity).toBe(2)
  })

  it('道具掉落：rng < dropRate 时掉落', () => {
    const reward = calculateDispatchReward('standard', 'common', () => 0.01)
    expect(reward.droppedItem).toBeDefined()
  })

  it('无道具掉落：rng ≥ dropRate 时不掉落', () => {
    const reward = calculateDispatchReward('standard', 'common', () => 0.99)
    expect(reward.droppedItem).toBeUndefined()
  })
})

// ===== 道具掉落池 =====
describe('rollDispatchItemDrop', () => {
  it('返回池中存在的 itemId', () => {
    const pool = [
      { itemId: 'toy_ball', weight: 50 },
      { itemId: 'food_pack', weight: 50 },
    ]
    const result = rollDispatchItemDrop(pool, () => 0.3)
    expect(['toy_ball', 'food_pack']).toContain(result)
  })
})

// ===== 创建派遣任务 =====
describe('createMission', () => {
  it('正确创建任务字段', () => {
    const mission = createMission('quick', 'pet001', 'common', '宁波市', NOW, () => 0.5)
    expect(mission.type).toBe('quick')
    expect(mission.petId).toBe('pet001')
    expect(mission.petRarity).toBe('common')
    expect(mission.city).toBe('宁波市')
    expect(mission.startTime).toBe(NOW)
    expect(mission.status).toBe('active')
  })

  it('快速探索结束时间 = startTime + 30min', () => {
    const mission = createMission('quick', 'pet001', 'common', '宁波市', NOW, () => 0.5)
    expect(mission.endTime).toBe(NOW + 30 * 60 * 1000)
  })

  it('深度探索结束时间 = startTime + 120min', () => {
    const mission = createMission('deep', 'pet001', 'common', '宁波市', NOW, () => 0.5)
    expect(mission.endTime).toBe(NOW + 120 * 60 * 1000)
  })
})

// ===== 任务完成检查 =====
describe('isMissionCompleted', () => {
  it('未到期返回 false', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    expect(isMissionCompleted(mission, NOW + 1000)).toBe(false)
  })

  it('到期返回 true', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    expect(isMissionCompleted(mission, NOW + 31 * 60 * 1000)).toBe(true)
  })
})

// ===== 倒计时 =====
describe('getMissionCountdown', () => {
  it('返回剩余秒数', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    // 30min 任务，过了 10min，剩 20min = 1200 秒
    expect(getMissionCountdown(mission, NOW + 10 * 60 * 1000)).toBe(1200)
  })

  it('已完成返回 0', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    expect(getMissionCountdown(mission, NOW + 31 * 60 * 1000)).toBe(0)
  })
})

// ===== 槽位计算 =====
describe('getAvailableSlots', () => {
  it('Lv5 无任务返回 2', () => {
    const state = makeDispatchState()
    expect(getAvailableSlots(state, 5)).toBe(2)
  })

  it('Lv5 有 1 个活跃任务返回 1', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    const state = makeDispatchState({ missions: [mission] })
    expect(getAvailableSlots(state, 5)).toBe(1)
  })

  it('Lv1 上限为 1', () => {
    const state = makeDispatchState()
    expect(getAvailableSlots(state, 1)).toBe(1)
  })
})

// ===== 宠物派遣状态 =====
describe('getPetMission', () => {
  it('空闲宠物返回 null', () => {
    const state = makeDispatchState()
    expect(getPetMission(state, 'pet001')).toBeNull()
  })

  it('派遣中宠物返回任务', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    const state = makeDispatchState({ missions: [mission] })
    expect(getPetMission(state, 'pet001')).not.toBeNull()
  })
})

// ===== 经济平衡检查 =====
describe('balanceCheck', () => {
  it('ratio=2.0 为健康', () => {
    const state = makeEconomyState({ totalEarned: 200, totalSpent: 100 })
    const result = balanceCheck(state)
    expect(result.ratio).toBe(2.0)
    expect(result.isHealthy).toBe(true)
    expect(result.status).toBe('healthy')
  })

  it('ratio=5.0 为通胀', () => {
    const state = makeEconomyState({ totalEarned: 500, totalSpent: 100 })
    const result = balanceCheck(state)
    expect(result.isHealthy).toBe(false)
    expect(result.status).toBe('inflation')
  })

  it('ratio=0.5 为通缩', () => {
    const state = makeEconomyState({ totalEarned: 50, totalSpent: 100 })
    const result = balanceCheck(state)
    expect(result.isHealthy).toBe(false)
    expect(result.status).toBe('deflation')
  })

  it('无消耗记录为通胀（Infinity）', () => {
    const state = makeEconomyState({ totalEarned: 100, totalSpent: 0 })
    const result = balanceCheck(state)
    expect(result.ratio).toBe(Infinity)
    expect(result.status).toBe('inflation')
  })
})

// ===== 槽位上限 =====
describe('getMaxDispatchSlots', () => {
  it('Lv1 → 1', () => {
    expect(getMaxDispatchSlots(1)).toBe(1)
  })
  it('Lv3 → 2', () => {
    expect(getMaxDispatchSlots(3)).toBe(2)
  })
  it('Lv6 → 3', () => {
    expect(getMaxDispatchSlots(6)).toBe(3)
  })
  it('Lv9 → 4', () => {
    expect(getMaxDispatchSlots(9)).toBe(4)
  })
  it('Lv10 → 4（上限）', () => {
    expect(getMaxDispatchSlots(10)).toBe(4)
  })
})

// ===== 统计快照 =====
describe('getEconomyStats', () => {
  it('正确计算净流入', () => {
    const state = makeEconomyState({ totalEarned: 500, totalSpent: 300, todayEarned: 100, todaySpent: 50 })
    const stats = getEconomyStats(state, 200)
    expect(stats.currentGold).toBe(200)
    expect(stats.netFlow).toBe(200)
    expect(stats.todayNetFlow).toBe(50)
  })
})
```

---

## 11. 实现步骤

| 步骤 | 任务 | 预估产出 | 依赖 |
|------|------|---------|------|
| Step 1 | 创建 `economy/constants.ts` | 常量 + 派遣任务定义 + 稀有度倍率表 | 无 |
| Step 2 | 创建 `economy/types.ts` | 全部 TypeScript 类型定义 | constants.ts |
| Step 3 | 创建 `economy/logic.ts`（纯函数） | 12+ 个纯函数 | constants.ts, types.ts |
| Step 4 | 创建 `economy/logic.test.ts`，确保全部通过 | 20+ 个测试用例，绿色通过 | logic.ts |
| Step 5 | 创建 `economy/EconomyContext.tsx` | Context + Reducer + Provider + 持久化 | constants, types, logic, StaminaContext |
| Step 6 | 创建 `economy/useEconomy.ts` | 自定义 Hook | EconomyContext |
| Step 7 | 创建 `economy/DispatchContext.tsx` | Context + Reducer + Provider + 定时结算 | constants, types, logic, StaminaContext, EconomyContext, ShopContext, LbsContext |
| Step 8 | 创建 `economy/useDispatch.ts` | 自定义 Hook | DispatchContext |
| Step 9 | 修改 `shop/ShopContext.tsx`：购买/签到后追踪 | trackSpend/trackEarn 调用 | useEconomy |
| Step 10 | 修改 `battle/BattleContext.tsx`：finishBattle 后追踪 | trackEarn 调用 | useEconomy |
| Step 11 | 修改 `App.tsx`：Provider 嵌套 + 捕获金币追踪 + 升级奖励追踪 | EconomyProvider + DispatchProvider 包裹 | EconomyContext, DispatchContext |
| Step 12 | 修改 `types.ts`：MainTab 新增 'dispatch' | 类型扩展 | 无 |
| Step 13 | 修改 `components/TabBar.tsx`：新增派遣 tab | tab 配置 | types.ts |
| Step 14 | 创建 `components/DispatchScreen.tsx` | 派遣任务界面 | useDispatch, useStamina, useAnimalStore |
| Step 15 | 修改 `components/StoreScreen.tsx`：经济看板 | 收支统计 + 平衡状态 | useEconomy |
| Step 16 | 修改 `components/TopBar.tsx`：净收支显示 | 今日净收支标签 | useEconomy |
| Step 17 | 修改 `components/DetailPopup.tsx`：派遣按钮 | 派遣入口 + 派遣状态展示 | useDispatch |
| Step 18 | 运行全部测试 `npx vitest run` | 确保无回归 | 全部 |
| Step 19 | 手动测试：派遣→完成→领取→金币入账全流程 | 端到端验证 | 全部 |
| Step 20 | 手动测试：经济看板数据正确 | 收支统计验证 | 全部 |

---

## 12. 验收标准对照

| # | 验收标准 | 实现情况 | 验收方式 |
|---|---------|---------|---------|
| 1 | 金币产出/消耗闭环 | 产出源：捕获(10-50) + 战斗(5-120) + 签到(10-200) + 升级(100-1000) + 派遣(15-320)；消耗口：商店(30-200) + 体力药剂(150) + 派遣加速(50) | 手动验证全流程 |
| 2 | 金币产出源：捕获奖励 10-50 金币 | `App.tsx` `handleCaptureSuccess` 中 `Math.floor(Math.random() * 41) + 10` + `trackEarn('capture')` | 代码审查 + 手动验证 |
| 3 | 金币产出源：战斗奖励 | `BATTLE_GOLD_REWARDS` 表 + `BattleContext.finishBattle` 中 `trackEarn('battle_win')` | 代码审查 + 手动验证 |
| 4 | 金币产出源：每日签到 | `CHECK_IN_REWARDS` 表 + `ShopContext.checkIn` 中 `trackEarn('checkin')` | 代码审查 + 手动验证 |
| 5 | 金币产出源：派遣任务 | `calculateDispatchReward()` + `DispatchContext.collectMission` 中 `trackEarn('dispatch')` | 单测 + 手动验证 |
| 6 | 金币消耗口：商店购买 | `ShopContext.buyItem` 中 `trackSpend('shop_buy')` | 代码审查 + 手动验证 |
| 7 | 金币消耗口：感冒药 200 金币 | `ITEM_DEFS.cold_medicine.price = 200`（已有） + buyItem 追踪 | 代码审查 |
| 8 | 金币消耗口：体力恢复 | `POTION_PRICE = 150`（已有） + buyItem('stamina_potion') 追踪 `trackSpend('stamina_potion')` | 代码审查 + 手动验证 |
| 9 | 金币消耗口：捕获增益道具 | `toy_ball: 50, premium_toy_ball: 120, bait: 100`（已有） + buyItem 追踪 | 代码审查 |
| 10 | 经济平衡：追踪总产出 vs 消耗 | `EconomyState.totalEarned / totalSpent` + `balanceCheck()` 返回 ratio + status | 单测 + 看板验证 |
| 11 | 派遣系统：派遣宠物进行限时任务 | `DispatchContext.startMission()` 创建任务，30min/1h/2h 三种类型 | 手动验证 |
| 12 | 派遣系统：稀有度影响产出 | `DISPATCH_RARITY_GOLD_MULTIPLIER` 5 级倍率（1.0~8.0） | 单测验证 |
| 13 | 派遣系统：冷却时间 | 30min/1h/2h 三种任务时长，`isMissionCompleted()` 检查到期 | 单测 + 手动验证 |
| 14 | 派遣系统：同时派遣上限随等级 | `getMaxDispatchSlots(level)` Lv1→1, Lv3→2, Lv6→3, Lv9→4 | 单测验证 |
| 15 | 派遣系统：加速完成 | `speedUpMission()` 50 金币立即完成 + `trackSpend('dispatch_speedup')` | 手动验证 |
| 16 | 价格平衡：道具成本 vs 奖励率 | `balanceCheck()` 提供 ratio + suggestion，看板展示 | 看板验证 |
| 17 | 道具商店完整 | 6 种道具已有（#34），购买/使用/签到完整 | 已验收 |
| 18 | 经济看板 UI | StoreScreen 显示收支统计 + 平衡状态 + TopBar 显示今日净收支 | 手动验证 |

---

## 13. 依赖关系图

```
Issue #31 体力系统 ✅ (已完成，gold 管理)
Issue #34 商店系统 ✅ (已完成，道具/签到)
Issue #35 物种系统 ✅ (已完成，RarityTier/CardEntry)
Issue #36 LBS 系统 ✅ (已完成，cityName)
Issue #37 战斗系统 ✅ (已完成，奖励表/computeRewards)
Issue #38 天气系统 ✅ (已完成，getColdRisk/getBattleModifier)
Issue #39 状态系统 📋 (计划已出，StatusContext)
    │
    │  依赖：#31 addGold / state.gold
    │  依赖：#34 buyItem / checkIn / addItem
    │  依赖：#36 cityName（派遣绑定城市）
    │  依赖：#37 RarityTier（派遣稀有度倍率）
    │
    ▼
Issue #40 经济系统 (本 Issue)
    │
    │  产出：EconomyContext（追踪层）/ DispatchContext（派遣系统）
    │  修改：ShopContext / BattleContext / App.tsx 追踪调用
    │
    ▼
后续：购买额外战斗场次 / 派遣任务化（叙事文本）/ 区域排行奖励 / 亲密度系统
```

### 前置依赖

| 依赖 | 来源 | 状态 | 说明 |
|------|------|------|------|
| `StaminaContext.addGold` / `state.gold` | #31 | ✅ 已实现 | 金币持有者 |
| `StaminaContext.consumeStamina` | #31 | ✅ 已实现 | 派遣扣体力 |
| `StaminaContext.buyStaminaPotion` | #31 | ✅ 已实现 | 体力药剂消耗追踪 |
| `ShopContext.buyItem` / `checkIn` / `addItem` | #34 | ✅ 已实现 | 购买/签到追踪 |
| `ShopContext.useItem` | #34 | ✅ 已实现 | 感冒药使用 |
| `BattleContext.finishBattle` | #37 | ✅ 已实现 | 战斗奖励追踪 |
| `LbsContext.state.cityName` | #36 | ✅ 已实现 | 派遣绑定城市 |
| `RarityTier` | #35 | ✅ 已实现 | 派遣稀有度倍率 |
| `CardEntry.id` | #35 | ✅ 已实现 | 宠物唯一标识 |
| `StatusContext` | #39 | 📋 计划已出 | Provider 嵌套顺序（可并行开发） |

---

## 14. 风险与注意事项

| 风险 | 缓解措施 |
|------|---------|
| **追踪调用遗漏** | `trackEarn` / `trackSpend` 需在每个产出/消耗点显式调用。遗漏不影响游戏逻辑，仅影响统计准确性。通过代码审查确保覆盖所有 addGold 调用点 |
| **Provider 嵌套层次深** | 8 层 Provider（Lbs > Stamina > Economy > Weather > Shop > Status > Dispatch > Battle），需确保顺序正确。EconomyProvider 在 StaminaProvider 内（读取 gold），DispatchProvider 在 ShopProvider 内（调用 addItem） |
| **派遣与捕获争夺体力** | 设计意图 — 体力双用途形成策略取舍。20 体力 = 1 次捕获或 1 次派遣，玩家自行决策 |
| **离线派遣结算** | 玩家离线期间任务到期，app 重启时需立即结算。`DispatchProvider` 首次加载 `useEffect` 中检查所有任务 + 页面可见性恢复时检查 |
| **流水日志无限增长** | `MAX_LOG_ENTRIES = 200`，FIFO 淘汰最旧记录。Reducer 中 `splice` 处理 |
| **每日重置时序** | `todayEarned` / `todaySpent` 在日期变更时重置。`EconomyProvider` 的 `useEffect` 检查 `shouldResetDaily` |
| **派遣加速金币扣除时机** | `speedUpMission` 中先检查金币 → 扣金币 → trackSpend → dispatch SPEED_UP。顺序确保金币不足时不执行 |
| **#39 状态系统未实现** | 本计划假设 #39 已提供 StatusProvider。若 #39 尚未实现，Provider 嵌套中暂时省略 StatusProvider 即可，不影响经济系统功能 |
| **经济平衡仅追踪不干预** | `balanceCheck` 仅提供数据和建议，不自动调整产出/消耗率。平衡调节需手动修改常量（后续运营工具化） |
| **金币为负数 edge case** | `addGold(-amount)` 在金币不足时可能导致负数。现有 `ShopContext.buyItem` 已在购买前检查金币。`speedUpMission` 也检查金币。但 `addGold` 本身不防负数 — 这与现有设计一致（#31 遗留），本 Issue 不修改 |
| **多宠物同时派遣** | `getPetMission` 按 petId 查找，支持多宠物独立派遣。`availableSlots` 控制同时上限 |
| **派遣任务 ID 冲突** | `createMission` 使用 `dispatch_${now}_${rng}` 生成 ID，冲突概率极低。生产环境可用 UUID 替代 |

---

## 15. 补充：经济平衡分析

### 15.1 日均产出估算（Lv5 玩家）

| 产出源 | 单次金额 | 日均次数 | 日均产出 |
|--------|---------|---------|---------|
| 捕获掉落 | 10~50（均值 30） | 3~5 次 | 90~150 |
| 战斗胜利 | 15~120（均值 40，Lv5 对手以 uncommon/rare 为主） | 3~5 次 | 120~200 |
| 签到 | 10~200（均值 60，7 天周期） | 1 次 | 60 |
| 派遣 | 15~320（均值 40，quick/standard 为主） | 2~3 次 | 80~120 |
| **合计** | | | **350~530** |

### 15.2 日均消耗估算（Lv5 玩家）

| 消耗口 | 单次金额 | 日均次数 | 日均消耗 |
|--------|---------|---------|---------|
| 食物包 | 30 | 1~2 次 | 30~60 |
| 玩具球 | 50 | 1~2 次 | 50~100 |
| 感冒药 | 200 | 0~1 次（雨雪天） | 0~200 |
| 体力药剂 | 150 | 0~1 次 | 0~150 |
| 派遣加速 | 50 | 0~2 次 | 0~100 |
| **合计** | | | **80~610** |

### 15.3 平衡结论

- **日均净流入**：约 -260 ~ +450 金币，中位数约 +100 金币/日
- **升级奖励脉冲**：Lv2→Lv10 累计 3950 金币，非线性产出，平滑日常消耗
- **健康 ratio**：日均产出 400 / 日均消耗 250 ≈ 1.6，落在健康区间 [1.2, 3.0]
- **通胀风险**：高等级玩家（Lv8+）战斗奖励高 + 派遣槽位多，日均产出可能达 600+。消耗侧需通过感冒药（雨雪天高频）、体力药剂（每日 3 次上限）消耗

### 15.4 调参方向

若测试中发现经济失衡：
- **通胀**：提高道具价格 / 降低派遣基准金币 / 增加派遣加速费用
- **通缩**：提高签到奖励 / 增加战斗胜利金币 / 降低道具价格

所有调参仅需修改 `economy/constants.ts` 和 `shop/constants.ts` 中的常量，无需修改逻辑代码。

---

*计划基于设计文档 v1.4 (2026-07-08) 与现有代码基（React 18 + Vite 6 + TypeScript 5.6）编写。依赖 #31 体力系统（gold 管理）、#34 商店系统（道具/签到）、#37 战斗系统（奖励表）均已实现。#39 状态系统（计划已出）可并行开发，Provider 嵌套已预留位置。*
