# [M2] 等级 / 成就系统实现计划 — Issue #41

> **验收标准**：升级流程顺畅，成就解锁正常
>
> **设计文档来源**：`游戏开发计划.md` 5.5 等级与体力数值 + 6.6 等级与成就系统
>
> **技术栈**：React 18 + Vite 6 + TypeScript 5.6 + vitest，无额外依赖
>
> **现有基础**：
> - `StaminaContext`（#31）已管理 `level` / `totalCaptures` / `gold` / `currentStamina`，`tryLevelUp()` 基于 `totalCaptures` 判定升级，`addCapture()` 触发升级并恢复满体力 + 金币奖励
> - `stamina/constants.ts` 已有 `LEVEL_TABLE`（Lv.1~10，含 `requiredCaptures` / `maxStamina` / `rewardGold`）
> - `BattleContext`（#37）`BattleRewards` 已有 `exp: number` 字段（当前固定为 0），`finishBattle()` 发放金币但未发放经验
> - `ShopContext`（#34）`checkIn()` 发放签到金币但未发放经验
> - `CardEntry` 有 `rarity` / `species` 字段，可用于成就计数
> - `WeatherContext`（#38）提供天气类型，可用于天气相关成就
> - `LbsContext`（#36）提供位置信息，可用于区域探索成就

---

## 0. 设计要点与关键决策

### 0.1 XP 系统引入：从捕获数驱动改为经验值驱动

**现状**：`StaminaContext` 以 `totalCaptures`（累计捕获数）为唯一等级驱动量，`LEVEL_TABLE` 的 `requiredCaptures` 字段定义升级阈值。

**Issue #41 要求**：XP 来源扩展为捕获（按稀有度）、战斗（胜/负）、每日签到、派遣完成四类。这意味着等级驱动量从单一「捕获数」变为多来源「经验值」。

**决策**：
1. 在 `StaminaState` 中新增 `exp` 字段（当前经验值），保留 `totalCaptures` 作为统计字段（成就系统依赖）。
2. 在 `LEVEL_TABLE` 中新增 `requiredExp` 字段，与 `requiredCaptures` 并存。升级判定改为基于 `exp >= requiredExp`。
3. `requiredExp` 的数值设计确保与原 `requiredCaptures` 方案的升级节奏一致（1 次捕获 ≈ 10 XP，因此 `requiredExp = requiredCaptures × 10`）。
4. `tryLevelUp()` 签名从 `(currentLevel, totalCaptures)` 改为 `(currentLevel, exp)`。
5. `addCapture()` 内部调用 `addExp()` 统一走经验值通道。
6. 提供数据迁移：加载旧存档时若无 `exp` 字段，则以 `totalCaptures × 10` 推算。

### 0.2 经验值（XP）数值表

| 来源 | 条件 | 经验值 | 说明 |
|------|------|--------|------|
| 捕获 | Common | 8 | 基础经验 |
| 捕获 | Uncommon | 15 | |
| 捕获 | Rare | 30 | |
| 捕获 | Epic | 60 | |
| 捕获 | Legendary | 120 | |
| 战斗 | 胜利 | 20 | 按敌方稀有度额外加成 |
| 战斗 | 失败 | 5 | 安慰经验 |
| 战斗 | 平局 | 10 | |
| 签到 | 每日签到 | 15 | 固定值，不随连签递增 |
| 派遣 | 完成 | 10 | 每次派遣完成 |

**战斗胜利稀有度加成**：在基础 20 XP 上，敌方稀有度每级额外 +5 XP（common +0, uncommon +5, rare +10, epic +15, legendary +20）。

### 0.3 等级表扩展（requiredExp）

| 等级 | requiredCaptures（旧） | requiredExp（新） | maxStamina | rewardGold | shopBonus | isMaxLevel |
|------|----------------------|-------------------|------------|------------|-----------|------------|
| Lv.1 | 0 | 0 | 120 | 0 | 0% | false |
| Lv.2 | 10 | 100 | 134 | 100 | 2% | false |
| Lv.3 | 25 | 250 | 148 | 150 | 4% | false |
| Lv.4 | 45 | 450 | 162 | 200 | 6% | false |
| Lv.5 | 70 | 700 | 176 | 300 | 8% | false |
| Lv.6 | 100 | 1000 | 190 | 400 | 10% | false |
| Lv.7 | 140 | 1400 | 204 | 500 | 12% | false |
| Lv.8 | 190 | 1900 | 218 | 600 | 14% | false |
| Lv.9 | 250 | 2500 | 232 | 700 | 16% | false |
| Lv.10 | 320 | 3200 | 240 | 1000 | 20% | true |

> `requiredExp = requiredCaptures × 10`，保证旧存档迁移后升级节奏不变。

### 0.4 成就系统归属

成就系统作为独立模块 `frontend/src/achievement/`，提供 `AchievementContext`。它**不自己维护玩家行为计数**，而是从其他 Context 读取数据并检查成就条件。

**成就检查触发时机**：
- 捕获完成 → 检查收集类成就
- 战斗结束 → 检查战斗类成就
- 签到完成 → 检查签到类成就
- 派遣完成 → 检查派遣类成就
- 等级变化 → 检查等级类成就

**决策**：`AchievementContext` 消费 `StaminaContext`（读取 level / totalCaptures），由外部系统在关键事件后调用 `checkAchievements()` 进行批量检查。成就解锁后发放奖励（金币 / 称号 / 道具），金币通过 `StaminaContext.addGold()` 发放。

### 0.5 成就分类与数量

| 类别 | 数量 | 说明 |
|------|------|------|
| collection（收集） | 7 | 捕获数量、稀有度、物种多样性 |
| battle（战斗） | 5 | 胜利场次、连胜、段位 |
| exploration（探索） | 4 | 城市数、天气类型 |
| social（社交） | 2 | 预留公会相关 |
| milestone（里程碑） | 4 | 等级、签到天数 |
| 合计 | 22 | |

### 0.6 Provider 嵌套顺序

```
StaminaProvider           ← #31（核心：level / exp / gold / stamina）
  └─ LbsProvider          ← #36
       └─ WeatherProvider  ← #38
            └─ ShopProvider ← #34（签到给金币+经验，依赖 StaminaContext）
                 └─ StatusProvider  ← #39
                      └─ BattleProvider ← #37（战斗给经验，依赖 StaminaContext）
                           └─ AchievementProvider ← #41（本 Issue，消费所有上层 Context）
                                └─ AppInner
```

`AchievementProvider` 位于最内层，可以访问所有上层 Context 的状态。

### 0.7 称号系统

成就解锁可获得称号（title）。称号存储在 `AchievementState.unlockedTitles` 中。玩家当前佩戴的称号存储在 `AchievementState.activeTitle`。称号为纯展示用，不影响数值。

---

## 1. 文件结构

```
frontend/src/
├── stamina/
│   ├── constants.ts          ← 【修改】新增 XP_REWARDS 表、LEVEL_TABLE 增加 requiredExp / shopBonus 字段
│   ├── types.ts              ← 【修改】StaminaState 增加 exp 字段、StaminaAction 增加 ADD_EXP / ADD_BATTLE_EXP
│   ├── logic.ts              ← 【修改】tryLevelUp 改为 exp 驱动、新增 addExp 纯函数、数据迁移函数
│   ├── logic.test.ts         ← 【修改】新增 XP 相关测试用例
│   ├── StaminaContext.tsx    ← 【修改】新增 addExp 方法、addCapture 内部走 addExp
│   └── useStamina.ts         ← 【修改】无需改动（类型自动推导）
│
├── achievement/
│   ├── constants.ts          ← 【新建】成就定义表 ACHIEVEMENT_DEFS、成就类别配置
│   ├── types.ts              ← 【新建】Achievement / AchievementCategory / AchievementState / AchievementContextValue
│   ├── logic.ts              ← 【新建】纯函数：checkAchievements / getAchievementProgress / getUnlockedTitles
│   ├── logic.test.ts         ← 【新建】纯函数测试（≥18 个用例）
│   ├── AchievementContext.tsx← 【新建】Reducer + Provider，管理成就解锁状态
│   └── useAchievement.ts     ← 【新建】自定义 Hook
│
├── components/
│   ├── AchievementScreen.tsx ← 【新建】成就列表页面（分类标签 + 进度条 + 解锁动画）
│   ├── LevelUpToast.tsx      ← 【新建】升级弹窗动画（全屏遮罩 + 等级数字 + 奖励列表）
│   └── TopBar.tsx            ← 【修改】显示当前等级 + 经验进度条
│
└── App.tsx                   ← 【修改】增加 AchievementProvider 嵌套
```

---

## 2. 类型定义

### 2.1 stamina/types.ts（修改）

```typescript
/** 等级表单行结构（扩展） */
export interface LevelTableRow {
  level: number
  /** 升级所需累计捕获数（保留用于兼容/展示） */
  requiredCaptures: number
  /** 升级所需累计经验值（新增，等级驱动量） */
  requiredExp: number
  /** 该等级体力上限 */
  maxStamina: number
  /** 升级奖励金币数 */
  rewardGold: number
  /** 商店稀有道具刷新率加成（新增） */
  shopBonus: number
  /** 是否为满级 */
  isMaxLevel: boolean
}

/** 体力系统完整状态（扩展 exp 字段） */
export interface StaminaState {
  level: number
  /** 当前经验值（新增） */
  exp: number
  currentStamina: number
  totalCaptures: number
  lastRecoverTime: number
  gold: number
  potionPurchasesToday: number
  potionPurchaseDate: string
}

/** 升级结果（扩展，含多级跨越信息） */
export interface LevelUpResult {
  leveledUp: boolean
  newLevel: number
  rewardGold: number
  /** 跨越的等级列表（如 [3, 4] 表示从 Lv.2 升到 Lv.4 跨越了 Lv.3 和 Lv.4） */
  crossedLevels: number[]
}

/** Reducer Action（新增 ADD_EXP） */
export type StaminaAction =
  | { type: 'TICK_RECOVERY'; now: number }
  | { type: 'CONSUME'; amount: number }
  | { type: 'ADD_STAMINA'; amount: number }
  | { type: 'ADD_CAPTURE'; count: number; rarity?: RarityTier }
  | { type: 'ADD_EXP'; amount: number }
  | { type: 'ADD_GOLD'; amount: number }
  | { type: 'BUY_POTION' }
  | { type: 'RESET_DAILY_PURCHASES'; date: string }
  | { type: 'LOAD_STATE'; state: StaminaState }

/** StaminaContextValue（新增 addExp） */
export interface StaminaContextValue {
  state: StaminaState
  maxStamina: number
  /** 当前等级所需经验（升级阈值） */
  nextLevelExp: number
  /** 当前等级已获得经验（用于进度条） */
  currentLevelExp: number
  /** 当前等级经验进度百分比 0~100 */
  expProgress: number
  nextRecoverIn: number
  consumeStamina: (amount: number) => boolean
  addStamina: (amount: number) => void
  addCapture: (count: number, rarity?: RarityTier) => LevelUpResult
  /** 增加经验值，内部自动检查升级 */
  addExp: (amount: number) => LevelUpResult
  addGold: (amount: number) => void
  buyStaminaPotion: () => BuyPotionResult
}
```

### 2.2 achievement/types.ts（新建）

```typescript
import type { RarityTier, SpeciesType } from '../types'
import type { WeatherType } from '../weather/types'

/** 成就类别 */
export type AchievementCategory =
  | 'collection'    // 收集类
  | 'battle'        // 战斗类
  | 'exploration'   // 探索类
  | 'social'        // 社交类
  | 'milestone'     // 里程碑类

/** 成就稀有度（影响视觉样式） */
export type AchievementRarity = 'bronze' | 'silver' | 'gold' | 'platinum'

/** 成就奖励 */
export interface AchievementReward {
  /** 金币奖励（0 表示无） */
  gold: number
  /** 称号奖励（可选） */
  title?: string
  /** 道具奖励（可选，ItemId） */
  item?: string
}

/** 成就定义（静态配置） */
export interface AchievementDef {
  /** 成就唯一 ID */
  id: string
  /** 成就名称（中文） */
  name: string
  /** 成就描述 */
  description: string
  /** 成就类别 */
  category: AchievementCategory
  /** 成就稀有度 */
  rarity: AchievementRarity
  /** 图标 emoji */
  icon: string
  /** 目标值（如捕获 50 只 = 50） */
  target: number
  /** 奖励 */
  reward: AchievementReward
  /** 解锁条件类型（对应 checkAchievements 中的判定逻辑） */
  condition: AchievementCondition
}

/** 成就解锁条件类型 */
export type AchievementCondition =
  | { type: 'total_captures'; target: number }
  | { type: 'captures_by_rarity'; rarity: RarityTier; target: number }
  | { type: 'captures_by_species'; species: SpeciesType; target: number }
  | { type: 'all_species_diversity'; target: number }  // 每物种各 N 种品种
  | { type: 'total_battles_won'; target: number }
  | { type: 'total_battles'; target: number }
  | { type: 'win_streak'; target: number }
  | { type: 'level_reached'; target: number }
  | { type: 'check_in_days'; target: number }
  | { type: 'cities_visited'; target: number }
  | { type: 'weather_types_experienced'; target: number }
  | { type: 'rain_captures_no_cold'; target: number }
  | { type: 'legendary_captured' }
  | { type: 'all_weather_types' }

/** 成就解锁记录 */
export interface UnlockedAchievement {
  /** 成就 ID */
  id: string
  /** 解锁时间戳（Unix ms） */
  unlockedAt: number
}

/** 成就系统状态 */
export interface AchievementState {
  /** 已解锁成就列表 */
  unlocked: UnlockedAchievement[]
  /** 已解锁称号列表 */
  unlockedTitles: string[]
  /** 当前佩戴的称号 */
  activeTitle: string | null
  /** 成就弹窗队列（待展示的新解锁成就） */
  pendingNotifications: string[]
}

/** 成就进度信息（UI 展示用） */
export interface AchievementProgress {
  /** 成就 ID */
  id: string
  /** 当前进度值 */
  current: number
  /** 目标值 */
  target: number
  /** 是否已解锁 */
  unlocked: boolean
  /** 进度百分比 0~100 */
  percent: number
}

/** 成就检查输入（从各 Context 汇总的统计数据） */
export interface AchievementStats {
  totalCaptures: number
  totalBattlesWon: number
  totalBattles: number
  currentWinStreak: number
  maxWinStreak: number
  level: number
  checkInStreak: number
  citiesVisited: number
  weatherTypesExperienced: WeatherType[]
  /** 按稀有度统计捕获数 */
  capturesByRarity: Record<RarityTier, number>
  /** 按物种统计捕获数 */
  capturesBySpecies: Record<SpeciesType, number>
  /** 是否捕获过传说级 */
  hasLegendary: boolean
  /** 雨天捕获无感冒次数 */
  rainCapturesNoCold: number
}

/** 成就解锁结果 */
export interface AchievementCheckResult {
  /** 新解锁的成就 ID 列表 */
  newlyUnlocked: string[]
  /** 是否有变化 */
  changed: boolean
}

/** Reducer Action */
export type AchievementAction =
  | { type: 'UNLOCK_ACHIEVEMENTS'; ids: string[]; unlockedAt: number }
  | { type: 'SET_ACTIVE_TITLE'; title: string }
  | { type: 'ENQUEUE_NOTIFICATION'; id: string }
  | { type: 'DEQUEUE_NOTIFICATION' }
  | { type: 'LOAD_STATE'; state: AchievementState }

/** AchievementContextValue */
export interface AchievementContextValue {
  state: AchievementState
  /** 所有成就定义列表 */
  definitions: AchievementDef[]
  /** 检查并解锁成就（传入当前统计数据） */
  checkAchievements: (stats: AchievementStats) => AchievementCheckResult
  /** 获取单个成就进度 */
  getProgress: (id: string, stats: AchievementStats) => AchievementProgress
  /** 获取所有成就进度列表 */
  getAllProgress: (stats: AchievementStats) => AchievementProgress[]
  /** 按类别获取成就进度 */
  getByCategory: (category: AchievementCategory, stats: AchievementStats) => AchievementProgress[]
  /** 获取已解锁成就数量 */
  getUnlockedCount: () => number
  /** 获取总成就数量 */
  getTotalCount: () => number
  /** 设置当前佩戴称号 */
  setActiveTitle: (title: string | null) => void
  /** 消费一个待展示通知 */
  consumeNotification: () => string | null
}
```

---

## 3. 常量定义

### 3.1 stamina/constants.ts（修改）

```typescript
import type { LevelTableRow } from './types'
import type { RarityTier } from '../types'

/** 等级表（扩展 requiredExp + shopBonus） */
export const LEVEL_TABLE: LevelTableRow[] = [
  { level: 1,  requiredCaptures: 0,   requiredExp: 0,    maxStamina: 120, rewardGold: 0,    shopBonus: 0,   isMaxLevel: false },
  { level: 2,  requiredCaptures: 10,  requiredExp: 100,  maxStamina: 134, rewardGold: 100,  shopBonus: 2,   isMaxLevel: false },
  { level: 3,  requiredCaptures: 25,  requiredExp: 250,  maxStamina: 148, rewardGold: 150,  shopBonus: 4,   isMaxLevel: false },
  { level: 4,  requiredCaptures: 45,  requiredExp: 450,  maxStamina: 162, rewardGold: 200,  shopBonus: 6,   isMaxLevel: false },
  { level: 5,  requiredCaptures: 70,  requiredExp: 700,  maxStamina: 176, rewardGold: 300,  shopBonus: 8,   isMaxLevel: false },
  { level: 6,  requiredCaptures: 100, requiredExp: 1000, maxStamina: 190, rewardGold: 400,  shopBonus: 10,  isMaxLevel: false },
  { level: 7,  requiredCaptures: 140, requiredExp: 1400, maxStamina: 204, rewardGold: 500,  shopBonus: 12,  isMaxLevel: false },
  { level: 8,  requiredCaptures: 190, requiredExp: 1900, maxStamina: 218, rewardGold: 600,  shopBonus: 14,  isMaxLevel: false },
  { level: 9,  requiredCaptures: 250, requiredExp: 2500, maxStamina: 232, rewardGold: 700,  shopBonus: 16,  isMaxLevel: false },
  { level: 10, requiredCaptures: 320, requiredExp: 3200, maxStamina: 240, rewardGold: 1000, shopBonus: 20,  isMaxLevel: true  },
]

/** 捕获经验值表（按稀有度） */
export const CAPTURE_XP: Record<RarityTier, number> = {
  common: 8,
  uncommon: 15,
  rare: 30,
  epic: 60,
  legendary: 120,
}

/** 战斗经验值 */
export const BATTLE_WIN_XP = 20
export const BATTLE_LOSE_XP = 5
export const BATTLE_DRAW_XP = 10

/** 战斗胜利稀有度额外加成 */
export const BATTLE_WIN_RARITY_BONUS: Record<RarityTier, number> = {
  common: 0,
  uncommon: 5,
  rare: 10,
  epic: 15,
  legendary: 20,
}

/** 每日签到经验值 */
export const CHECK_IN_XP = 15

/** 派遣完成经验值 */
export const DISPATCH_XP = 10

/** 满级等级 */
export const MAX_LEVEL = 10

// ... 其余已有常量不变（STAMINA_RECOVERY_PER_HOUR 等）
```

### 3.2 achievement/constants.ts（新建）

```typescript
import type { AchievementDef, AchievementCategory } from './types'

/** 成就类别显示配置 */
export const CATEGORY_LABELS: Record<AchievementCategory, string> = {
  collection: '收集',
  battle: '战斗',
  exploration: '探索',
  social: '社交',
  milestone: '里程碑',
}

/** 成就稀有度显示配置 */
export const RARITY_LABELS: Record<string, string> = {
  bronze: '铜',
  silver: '银',
  gold: '金',
  platinum: '铂',
}

/** 成就稀有度颜色（CSS 变量） */
export const RARITY_COLORS: Record<string, string> = {
  bronze: 'var(--rarity-common)',
  silver: 'var(--rarity-uncommon)',
  gold: 'var(--rarity-epic)',
  platinum: 'var(--rarity-legendary)',
}

/** 成就定义表（22 项） */
export const ACHIEVEMENT_DEFS: AchievementDef[] = [
  // ===== 收集类（7 项） =====
  {
    id: 'first_capture',
    name: '初出茅庐',
    description: '收藏第 1 只动物',
    category: 'collection',
    rarity: 'bronze',
    icon: '🎯',
    target: 1,
    reward: { gold: 50 },
    condition: { type: 'total_captures', target: 1 },
  },
  {
    id: 'collector_50',
    name: '收藏家',
    description: '收藏 50 只动物',
    category: 'collection',
    rarity: 'silver',
    icon: '📦',
    target: 50,
    reward: { gold: 200, title: '收藏家' },
    condition: { type: 'total_captures', target: 50 },
  },
  {
    id: 'dex_master_200',
    name: '图鉴大师',
    description: '收藏 200 只动物',
    category: 'collection',
    rarity: 'gold',
    icon: '📖',
    target: 200,
    reward: { gold: 500, title: '图鉴大师' },
    condition: { type: 'total_captures', target: 200 },
  },
  {
    id: 'rare_collector_10',
    name: '稀有猎人',
    description: '捕获 10 只稀有(Rare)动物',
    category: 'collection',
    rarity: 'silver',
    icon: '💎',
    target: 10,
    reward: { gold: 150 },
    condition: { type: 'captures_by_rarity', rarity: 'rare', target: 10 },
  },
  {
    id: 'epic_collector_5',
    name: '史诗收藏',
    description: '捕获 5 只史诗(Epic)动物',
    category: 'collection',
    rarity: 'gold',
    icon: '🔮',
    target: 5,
    reward: { gold: 300, title: '史诗收藏家' },
    condition: { type: 'captures_by_rarity', rarity: 'epic', target: 5 },
  },
  {
    id: 'legendary_captured',
    name: '传说降临',
    description: '捕获第 1 只传说级动物',
    category: 'collection',
    rarity: 'platinum',
    icon: '⭐',
    target: 1,
    reward: { gold: 500, title: '传说捕手' },
    condition: { type: 'legendary_captured' },
  },
  {
    id: 'cat_lover_10',
    name: '爱猫人士',
    description: '捕获 10 只猫',
    category: 'collection',
    rarity: 'bronze',
    icon: '🐱',
    target: 10,
    reward: { gold: 100 },
    condition: { type: 'captures_by_species', species: 'cat', target: 10 },
  },

  // ===== 战斗类（5 项） =====
  {
    id: 'first_battle_win',
    name: '初战告捷',
    description: '赢得第 1 场战斗',
    category: 'battle',
    rarity: 'bronze',
    icon: '⚔️',
    target: 1,
    reward: { gold: 50 },
    condition: { type: 'total_battles_won', target: 1 },
  },
  {
    id: 'battler_50',
    name: '斗士',
    description: '参与 50 场战斗',
    category: 'battle',
    rarity: 'silver',
    icon: '🛡️',
    target: 50,
    reward: { gold: 200 },
    condition: { type: 'total_battles', target: 50 },
  },
  {
    id: 'win_streak_5',
    name: '连胜达人',
    description: '获得 5 连胜',
    category: 'battle',
    rarity: 'silver',
    icon: '🔥',
    target: 5,
    reward: { gold: 200, title: '连胜达人' },
    condition: { type: 'win_streak', target: 5 },
  },
  {
    id: 'battler_100',
    name: '百战不殆',
    description: '参与 100 场战斗',
    category: 'battle',
    rarity: 'gold',
    icon: '🗡️',
    target: 100,
    reward: { gold: 400, title: '百战勇士' },
    condition: { type: 'total_battles', target: 100 },
  },
  {
    id: 'battle_master_50_wins',
    name: '常胜将军',
    description: '赢得 50 场战斗',
    category: 'battle',
    rarity: 'gold',
    icon: '👑',
    target: 50,
    reward: { gold: 500, title: '常胜将军' },
    condition: { type: 'total_battles_won', target: 50 },
  },

  // ===== 探索类（4 项） =====
  {
    id: 'explorer_5_cities',
    name: '区域探索者',
    description: '在 5 个不同城市捕获过动物',
    category: 'exploration',
    rarity: 'silver',
    icon: '🗺️',
    target: 5,
    reward: { gold: 200 },
    condition: { type: 'cities_visited', target: 5 },
  },
  {
    id: 'rain_warrior',
    name: '雨天勇士',
    description: '雨天捕获 10 只且无感冒',
    category: 'exploration',
    rarity: 'silver',
    icon: '🌧️',
    target: 10,
    reward: { gold: 150, title: '雨天勇士' },
    condition: { type: 'rain_captures_no_cold', target: 10 },
  },
  {
    id: 'weather_master',
    name: '天气大师',
    description: '经历所有 7 种天气类型',
    category: 'exploration',
    rarity: 'gold',
    icon: '🌤️',
    target: 7,
    reward: { gold: 300, title: '天气大师' },
    condition: { type: 'all_weather_types' },
  },
  {
    id: 'explorer_10_cities',
    name: '环球旅行家',
    description: '在 10 个不同城市捕获过动物',
    category: 'exploration',
    rarity: 'gold',
    icon: '🧭',
    target: 10,
    reward: { gold: 500, title: '环球旅行家' },
    condition: { type: 'cities_visited', target: 10 },
  },

  // ===== 社交类（2 项，预留） =====
  {
    id: 'guild_member',
    name: '社交新星',
    description: '加入一个公会',
    category: 'social',
    rarity: 'bronze',
    icon: '🤝',
    target: 1,
    reward: { gold: 100 },
    condition: { type: 'cities_visited', target: 0 }, // 预留，暂时不可解锁
  },
  {
    id: 'guild_leader',
    name: '社团领袖',
    description: '创建一个公会',
    category: 'social',
    rarity: 'silver',
    icon: '🏅',
    target: 1,
    reward: { gold: 200, title: '社团领袖' },
    condition: { type: 'cities_visited', target: 999 }, // 预留，暂时不可解锁
  },

  // ===== 里程碑类（4 项） =====
  {
    id: 'level_5',
    name: '小有所成',
    description: '达到等级 5',
    category: 'milestone',
    rarity: 'silver',
    icon: '⬆️',
    target: 5,
    reward: { gold: 200 },
    condition: { type: 'level_reached', target: 5 },
  },
  {
    id: 'level_10',
    name: '满级达成',
    description: '达到等级 10（满级）',
    category: 'milestone',
    rarity: 'platinum',
    icon: '🏆',
    target: 10,
    reward: { gold: 1000, title: '巅峰大师' },
    condition: { type: 'level_reached', target: 10 },
  },
  {
    id: 'check_in_7_days',
    name: '坚持就是胜利',
    description: '连续签到 7 天',
    category: 'milestone',
    rarity: 'bronze',
    icon: '📅',
    target: 7,
    reward: { gold: 100 },
    condition: { type: 'check_in_days', target: 7 },
  },
  {
    id: 'check_in_30_days',
    name: '月度坚持',
    description: '累计签到 30 天',
    category: 'milestone',
    rarity: 'gold',
    icon: '📆',
    target: 30,
    reward: { gold: 500, title: '坚持之星' },
    condition: { type: 'check_in_days', target: 30 },
  },
]

/** 成就 ID → 定义映射（便于快速查找） */
export const ACHIEVEMENT_MAP: Record<string, AchievementDef> = Object.fromEntries(
  ACHIEVEMENT_DEFS.map((def) => [def.id, def])
)

/** AchievementContext localStorage 存储 key */
export const ACHIEVEMENT_STORAGE_KEY = 'animal_poke_achievement'
```

---

## 4. 纯逻辑函数

### 4.1 stamina/logic.ts（修改）

```typescript
/**
 * 根据经验值计算应处等级（1~10）
 * 改为 exp 驱动，替代原 getLevelForCaptures
 */
export function getLevelForExp(exp: number): number {
  for (let i = LEVEL_TABLE.length - 1; i >= 0; i--) {
    if (exp >= LEVEL_TABLE[i].requiredExp) {
      return LEVEL_TABLE[i].level
    }
  }
  return 1
}

/** 根据累计捕获数计算应处等级（保留，兼容旧逻辑） */
export function getLevelForCaptures(totalCaptures: number): number {
  for (let i = LEVEL_TABLE.length - 1; i >= 0; i--) {
    if (totalCaptures >= LEVEL_TABLE[i].requiredCaptures) {
      return LEVEL_TABLE[i].level
    }
  }
  return 1
}

/**
 * 检查并执行升级（改为 exp 驱动）
 * 从当前等级+1 开始遍历等级表，累计所有可跨越等级的奖励金币
 *
 * @param currentLevel 当前等级
 * @param exp 当前经验值
 * @returns 升级结果（含跨越的等级列表）
 */
export function tryLevelUp(currentLevel: number, exp: number): LevelUpResult {
  if (currentLevel >= MAX_LEVEL) {
    return { leveledUp: false, newLevel: currentLevel, rewardGold: 0, crossedLevels: [] }
  }

  let newLevel = currentLevel
  let rewardGold = 0
  const crossedLevels: number[] = []

  for (let i = currentLevel; i < LEVEL_TABLE.length; i++) {
    const entry = LEVEL_TABLE[i]
    if (exp >= entry.requiredExp) {
      newLevel = entry.level
      rewardGold += entry.rewardGold
      crossedLevels.push(entry.level)
    }
  }

  if (newLevel > currentLevel) {
    return { leveledUp: true, newLevel, rewardGold, crossedLevels }
  }

  return { leveledUp: false, newLevel: currentLevel, rewardGold: 0, crossedLevels: [] }
}

/**
 * 获取当前等级的经验值信息（用于 UI 进度条）
 * @param level 当前等级
 * @param exp 当前经验值
 * @returns { currentLevelExp, nextLevelExp, progress }
 */
export function getExpProgress(level: number, exp: number): {
  currentLevelExp: number
  nextLevelExp: number
  progress: number
} {
  const currentEntry = LEVEL_TABLE[Math.max(0, Math.min(LEVEL_TABLE.length - 1, level - 1))]
  const currentLevelExpBase = currentEntry.requiredExp

  if (level >= MAX_LEVEL) {
    return { currentLevelExp: exp - currentLevelExpBase, nextLevelExp: 0, progress: 100 }
  }

  const nextEntry = LEVEL_TABLE[level] // level 是 1-based，索引 level = 下一级
  const nextLevelExpTotal = nextEntry.requiredExp - currentEntry.requiredExp
  const currentLevelExp = exp - currentLevelExpBase
  const progress = Math.min(100, Math.round((currentLevelExp / nextLevelExpTotal) * 100))

  return { currentLevelExp, nextLevelExp: nextLevelExpTotal, progress }
}

/**
 * 获取指定稀有度的捕获经验值
 */
export function getCaptureXp(rarity: RarityTier): number {
  return CAPTURE_XP[rarity]
}

/**
 * 计算战斗经验值
 * @param result 战斗结果 ('win' | 'lose' | 'draw')
 * @param enemyRarity 敌方稀有度
 */
export function getBattleXp(result: 'win' | 'lose' | 'draw', enemyRarity: RarityTier): number {
  if (result === 'win') return BATTLE_WIN_XP + BATTLE_WIN_RARITY_BONUS[enemyRarity]
  if (result === 'draw') return BATTLE_DRAW_XP
  return BATTLE_LOSE_XP
}

/**
 * 旧存档数据迁移：若无 exp 字段，以 totalCaptures × 10 推算
 */
export function migrateState(saved: Partial<StaminaState>): StaminaState {
  const defaultState: StaminaState = {
    level: 1,
    exp: 0,
    currentStamina: 120,
    totalCaptures: 0,
    lastRecoverTime: Date.now(),
    gold: 0,
    potionPurchasesToday: 0,
    potionPurchaseDate: getTodayString(),
  }
  const merged = { ...defaultState, ...saved }
  // 如果没有 exp 字段但有 totalCaptures，推算 exp
  if (typeof saved.exp !== 'number' && typeof saved.totalCaptures === 'number') {
    merged.exp = saved.totalCaptures * 10
  }
  // 确保 level 与 exp 一致
  merged.level = getLevelForExp(merged.exp)
  return merged
}

// ===== 以下函数保持不变 =====
// getMaxStamina(level)
// calculateRecovery(lastRecoverTime, currentStamina, maxStamina, now?)
// canConsume(currentStamina, amount)
// getTodayString(now?)
// shouldResetDailyPurchases(potionPurchaseDate, now?)
// calculateBuyPotion(gold, potionPurchasesToday)
```

### 4.2 achievement/logic.ts（新建）

```typescript
import { ACHIEVEMENT_DEFS, ACHIEVEMENT_MAP } from './constants'
import type {
  AchievementDef,
  AchievementStats,
  AchievementProgress,
  AchievementCheckResult,
  UnlockedAchievement,
} from './types'

/**
 * 根据成就条件和统计数据，判定单个成就是否应解锁
 * @param def 成就定义
 * @param stats 当前统计数据
 * @returns 是否满足解锁条件
 */
export function isAchievementUnlocked(def: AchievementDef, stats: AchievementStats): boolean {
  const { condition } = def
  switch (condition.type) {
    case 'total_captures':
      return stats.totalCaptures >= condition.target
    case 'captures_by_rarity':
      return stats.capturesByRarity[condition.rarity] >= condition.target
    case 'captures_by_species':
      return stats.capturesBySpecies[condition.species] >= condition.target
    case 'all_species_diversity':
      // 每物种各 N 种品种（简化：检查每物种捕获数 >= target）
      return stats.capturesBySpecies.cat >= condition.target
        && stats.capturesBySpecies.goose >= condition.target
        && stats.capturesBySpecies.dog >= condition.target
    case 'total_battles_won':
      return stats.totalBattlesWon >= condition.target
    case 'total_battles':
      return stats.totalBattles >= condition.target
    case 'win_streak':
      return stats.maxWinStreak >= condition.target
    case 'level_reached':
      return stats.level >= condition.target
    case 'check_in_days':
      return stats.checkInStreak >= condition.target
    case 'cities_visited':
      return stats.citiesVisited >= condition.target
    case 'weather_types_experienced':
      return stats.weatherTypesExperienced.length >= condition.target
    case 'rain_captures_no_cold':
      return stats.rainCapturesNoCold >= condition.target
    case 'legendary_captured':
      return stats.hasLegendary
    case 'all_weather_types':
      return stats.weatherTypesExperienced.length >= 7
    default:
      return false
  }
}

/**
 * 获取单个成就的当前进度值
 * @param def 成就定义
 * @param stats 当前统计数据
 * @returns 当前进度值
 */
export function getAchievementCurrent(def: AchievementDef, stats: AchievementStats): number {
  const { condition } = def
  switch (condition.type) {
    case 'total_captures':
      return stats.totalCaptures
    case 'captures_by_rarity':
      return stats.capturesByRarity[condition.rarity]
    case 'captures_by_species':
      return stats.capturesBySpecies[condition.species]
    case 'all_species_diversity':
      return Math.min(
        stats.capturesBySpecies.cat,
        stats.capturesBySpecies.goose,
        stats.capturesBySpecies.dog
      )
    case 'total_battles_won':
      return stats.totalBattlesWon
    case 'total_battles':
      return stats.totalBattles
    case 'win_streak':
      return stats.maxWinStreak
    case 'level_reached':
      return stats.level
    case 'check_in_days':
      return stats.checkInStreak
    case 'cities_visited':
      return stats.citiesVisited
    case 'weather_types_experienced':
      return stats.weatherTypesExperienced.length
    case 'rain_captures_no_cold':
      return stats.rainCapturesNoCold
    case 'legendary_captured':
      return stats.hasLegendary ? 1 : 0
    case 'all_weather_types':
      return stats.weatherTypesExperienced.length
    default:
      return 0
  }
}

/**
 * 批量检查所有成就，返回新解锁的成就 ID 列表
 * @param stats 当前统计数据
 * @param alreadyUnlocked 已解锁的成就 ID 集合
 * @returns 新解锁的成就 ID 列表
 */
export function checkAchievements(
  stats: AchievementStats,
  alreadyUnlocked: Set<string>
): AchievementCheckResult {
  const newlyUnlocked: string[] = []

  for (const def of ACHIEVEMENT_DEFS) {
    if (alreadyUnlocked.has(def.id)) continue
    if (isAchievementUnlocked(def, stats)) {
      newlyUnlocked.push(def.id)
    }
  }

  return {
    newlyUnlocked,
    changed: newlyUnlocked.length > 0,
  }
}

/**
 * 获取单个成就的完整进度信息
 */
export function getAchievementProgress(
  def: AchievementDef,
  stats: AchievementStats,
  unlocked: boolean
): AchievementProgress {
  const current = getAchievementCurrent(def, stats)
  const target = def.condition.type === 'legendary_captured' || def.condition.type === 'all_weather_types'
    ? def.target
    : (def.condition as { target: number }).target
  const percent = target > 0 ? Math.min(100, Math.round((current / target) * 100)) : 0

  return {
    id: def.id,
    current,
    target,
    unlocked,
    percent: unlocked ? 100 : percent,
  }
}

/**
 * 获取所有成就的进度列表
 */
export function getAllAchievementProgress(
  stats: AchievementStats,
  unlockedIds: Set<string>
): AchievementProgress[] {
  return ACHIEVEMENT_DEFS.map((def) =>
    getAchievementProgress(def, stats, unlockedIds.has(def.id))
  )
}

/**
 * 从成就定义中提取所有可获得的称号
 */
export function getAllTitles(): string[] {
  const titles: string[] = []
  for (const def of ACHIEVEMENT_DEFS) {
    if (def.reward.title) {
      titles.push(def.reward.title)
    }
  }
  return titles
}

/**
 * 从已解锁成就中提取已获得的称号
 */
export function getUnlockedTitles(unlocked: UnlockedAchievement[]): string[] {
  const titles: string[] = []
  const unlockedIds = new Set(unlocked.map((u) => u.id))
  for (const def of ACHIEVEMENT_DEFS) {
    if (unlockedIds.has(def.id) && def.reward.title) {
      titles.push(def.reward.title)
    }
  }
  return titles
}

/**
 * 计算成就完成率
 */
export function getCompletionRate(unlockedCount: number): number {
  const total = ACHIEVEMENT_DEFS.length
  return total > 0 ? Math.round((unlockedCount / total) * 100) : 0
}

/**
 * 计算成就奖励金币总数（已解锁的）
 */
export function getTotalRewardGold(unlockedIds: Set<string>): number {
  let total = 0
  for (const def of ACHIEVEMENT_DEFS) {
    if (unlockedIds.has(def.id)) {
      total += def.reward.gold
    }
  }
  return total
}
```

---

## 5. AchievementContext 设计

### 5.1 Reducer 设计

```typescript
function achievementReducer(state: AchievementState, action: AchievementAction): AchievementState {
  switch (action.type) {
    case 'UNLOCK_ACHIEVEMENTS': {
      const newUnlocked = action.ids.map((id) => ({
        id,
        unlockedAt: action.unlockedAt,
      }))
      // 提取新解锁的称号
      const newTitles = action.ids
        .map((id) => ACHIEVEMENT_MAP[id]?.reward.title)
        .filter((t): t is string => !!t)

      return {
        ...state,
        unlocked: [...state.unlocked, ...newUnlocked],
        unlockedTitles: [...state.unlockedTitles, ...newTitles],
        pendingNotifications: [...state.pendingNotifications, ...action.ids],
      }
    }

    case 'SET_ACTIVE_TITLE': {
      return { ...state, activeTitle: action.title }
    }

    case 'ENQUEUE_NOTIFICATION': {
      return {
        ...state,
        pendingNotifications: [...state.pendingNotifications, action.id],
      }
    }

    case 'DEQUEUE_NOTIFICATION': {
      const [, ...rest] = state.pendingNotifications
      return { ...state, pendingNotifications: rest }
    }

    case 'LOAD_STATE': {
      return action.state
    }

    default:
      return state
  }
}
```

### 5.2 Provider 设计

```typescript
export const AchievementProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const stamina = useStamina()
  const [state, dispatch] = useReducer(achievementReducer, undefined, loadInitialState)

  // localStorage 持久化
  useEffect(() => {
    localStorage.setItem(ACHIEVEMENT_STORAGE_KEY, JSON.stringify(state))
  }, [state])

  /**
   * 检查并解锁成就
   * 由外部系统在关键事件后调用，传入完整的统计数据
   */
  const checkAchievements = useCallback((stats: AchievementStats): AchievementCheckResult => {
    const alreadyUnlocked = new Set(state.unlocked.map((u) => u.id))
    const result = checkAchievementsLogic(stats, alreadyUnlocked)

    if (result.changed) {
      dispatch({
        type: 'UNLOCK_ACHIEVEMENTS',
        ids: result.newlyUnlocked,
        unlockedAt: Date.now(),
      })
      // 发放金币奖励
      let totalGold = 0
      for (const id of result.newlyUnlocked) {
        const def = ACHIEVEMENT_MAP[id]
        if (def) totalGold += def.reward.gold
      }
      if (totalGold > 0) {
        stamina.addGold(totalGold)
      }
    }

    return result
  }, [state.unlocked, stamina])

  const getProgress = useCallback((id: string, stats: AchievementStats): AchievementProgress => {
    const def = ACHIEVEMENT_MAP[id]
    if (!def) throw new Error(`成就 ${id} 不存在`)
    const unlocked = state.unlocked.some((u) => u.id === id)
    return getAchievementProgress(def, stats, unlocked)
  }, [state.unlocked])

  const getAllProgress = useCallback((stats: AchievementStats): AchievementProgress[] => {
    const unlockedIds = new Set(state.unlocked.map((u) => u.id))
    return getAllAchievementProgress(stats, unlockedIds)
  }, [state.unlocked])

  const getByCategory = useCallback((
    category: AchievementCategory,
    stats: AchievementStats
  ): AchievementProgress[] => {
    const unlockedIds = new Set(state.unlocked.map((u) => u.id))
    return ACHIEVEMENT_DEFS
      .filter((def) => def.category === category)
      .map((def) => getAchievementProgress(def, stats, unlockedIds.has(def.id)))
  }, [state.unlocked])

  const getUnlockedCount = useCallback((): number => state.unlocked.length, [state.unlocked])
  const getTotalCount = useCallback((): number => ACHIEVEMENT_DEFS.length, [])

  const setActiveTitle = useCallback((title: string | null) => {
    dispatch({ type: 'SET_ACTIVE_TITLE', title: title ?? '' })
  }, [])

  const consumeNotification = useCallback((): string | null => {
    if (state.pendingNotifications.length === 0) return null
    const next = state.pendingNotifications[0]
    dispatch({ type: 'DEQUEUE_NOTIFICATION' })
    return next
  }, [state.pendingNotifications])

  const value = useMemo<AchievementContextValue>(() => ({
    state,
    definitions: ACHIEVEMENT_DEFS,
    checkAchievements,
    getProgress,
    getAllProgress,
    getByCategory,
    getUnlockedCount,
    getTotalCount,
    setActiveTitle,
    consumeNotification,
  }), [state, checkAchievements, getProgress, getAllProgress, getByCategory, getUnlockedCount, getTotalCount, setActiveTitle, consumeNotification])

  return (
    <AchievementContext.Provider value={value}>
      {children}
    </AchievementContext.Provider>
  )
}
```

### 5.3 初始状态加载

```typescript
function loadInitialState(): AchievementState {
  try {
    const saved = localStorage.getItem(ACHIEVEMENT_STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as AchievementState
      if (
        typeof parsed.unlocked !== 'object' ||
        !Array.isArray(parsed.unlocked) ||
        !Array.isArray(parsed.unlockedTitles) ||
        !Array.isArray(parsed.pendingNotifications)
      ) {
        throw new Error('成就存档字段校验失败')
      }
      return parsed
    }
  } catch (e) {
    console.warn('加载成就存档失败，使用默认值:', e)
  }
  return {
    unlocked: [],
    unlockedTitles: [],
    activeTitle: null,
    pendingNotifications: [],
  }
}
```

---

## 6. 与现有 StaminaContext 集成

### 6.1 StaminaContext 修改清单

#### 6.1.1 `loadInitialState()` 修改

```typescript
function loadInitialState(): StaminaState {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as Partial<StaminaState>
      // 使用 migrateState 进行数据迁移
      const migrated = migrateState(parsed)
      // 离线恢复逻辑（保持不变）
      if (shouldResetDailyPurchases(migrated.potionPurchaseDate)) {
        migrated.potionPurchasesToday = 0
        migrated.potionPurchaseDate = getTodayString()
      }
      const maxStamina = getMaxStamina(migrated.level)
      const oldStamina = migrated.currentStamina
      const { current } = calculateRecovery(migrated.lastRecoverTime, oldStamina, maxStamina)
      if (current > oldStamina) {
        const recoveredPoints = current - oldStamina
        const consumedTime = recoveredPoints * 360_000
        migrated.lastRecoverTime = current >= maxStamina
          ? Date.now()
          : migrated.lastRecoverTime + consumedTime
      }
      migrated.currentStamina = current
      return migrated
    }
  } catch (e) {
    console.warn('加载体力存档失败，使用默认值:', e)
  }
  return {
    level: 1,
    exp: 0,
    currentStamina: 120,
    totalCaptures: 0,
    lastRecoverTime: Date.now(),
    gold: 0,
    potionPurchasesToday: 0,
    potionPurchaseDate: getTodayString(),
  }
}
```

#### 6.1.2 Reducer 修改

```typescript
case 'ADD_EXP': {
  const newExp = state.exp + action.amount
  const levelUpResult = tryLevelUp(state.level, newExp)
  if (levelUpResult.leveledUp) {
    const newMaxStamina = getMaxStamina(levelUpResult.newLevel)
    return {
      ...state,
      level: levelUpResult.newLevel,
      exp: newExp,
      currentStamina: newMaxStamina,
      gold: state.gold + levelUpResult.rewardGold,
    }
  }
  return { ...state, exp: newExp }
}

case 'ADD_CAPTURE': {
  // 计算捕获经验值
  const xp = action.rarity ? getCaptureXp(action.rarity) : CAPTURE_XP.common
  const newExp = state.exp + xp
  const newTotalCaptures = state.totalCaptures + action.count
  const levelUpResult = tryLevelUp(state.level, newExp)
  if (levelUpResult.leveledUp) {
    const newMaxStamina = getMaxStamina(levelUpResult.newLevel)
    return {
      ...state,
      level: levelUpResult.newLevel,
      exp: newExp,
      totalCaptures: newTotalCaptures,
      currentStamina: newMaxStamina,
      gold: state.gold + levelUpResult.rewardGold,
    }
  }
  return {
    ...state,
    exp: newExp,
    totalCaptures: newTotalCaptures,
  }
}
```

#### 6.1.3 新增 `addExp` 方法

```typescript
const addExp = useCallback((amount: number): LevelUpResult => {
  const result = tryLevelUp(state.level, state.exp + amount)
  dispatch({ type: 'ADD_EXP', amount })
  return result
}, [state.level, state.exp])
```

#### 6.1.4 新增经验值派生值

```typescript
const { currentLevelExp, nextLevelExp, progress: expProgress } = useMemo(
  () => getExpProgress(state.level, state.exp),
  [state.level, state.exp]
)
```

### 6.2 BattleContext 修改

在 `finishBattle()` 中发放战斗经验：

```typescript
const finishBattle = useCallback(() => {
  if (state.rewards) {
    stamina.addGold(state.rewards.gold)
    // 发放战斗经验（新增）
    if (state.rewards.exp > 0) {
      stamina.addExp(state.rewards.exp)
    }
    if (state.rewards.droppedItem) {
      shop.addItem(state.rewards.droppedItem as any)
    }
  }
  // ... 感冒判定逻辑不变
  dispatch({ type: 'RESET' })
}, [state.rewards, state.playerPet, stamina, shop, weatherCtx, status])
```

同时修改 `computeRewards()` 中的 `exp` 计算：

```typescript
// battle/logic.ts
export function computeRewards(
  result: BattleResult,
  enemyRarity: RarityTier,
  difficultyMultiplier: number
): BattleRewards {
  if (result === 'win') {
    const gold = Math.round(BATTLE_GOLD_REWARDS[enemyRarity] * difficultyMultiplier)
    const exp = getBattleXp('win', enemyRarity)
    const droppedItem = rollItemDrop()
    return { gold, exp, droppedItem }
  }
  if (result === 'draw') {
    return { gold: BATTLE_DRAW_GOLD, exp: getBattleXp('draw', enemyRarity) }
  }
  return { gold: BATTLE_LOSE_GOLD, exp: getBattleXp('lose', enemyRarity) }
}
```

### 6.3 ShopContext 修改

在 `checkIn()` 中发放签到经验：

```typescript
const checkIn = useCallback((): CheckInResult => {
  const result = getCheckInReward(state.checkIn.streak, state.checkIn.lastCheckInDate)

  if (result.success) {
    stamina.addGold(result.reward)
    // 发放签到经验（新增）
    stamina.addExp(CHECK_IN_XP)

    dispatch({
      type: 'CHECK_IN',
      reward: result.reward,
      newStreak: result.newStreak,
      rewardItem: result.rewardItem,
    })
  }

  return result
}, [stamina, state.checkIn])
```

### 6.4 成就检查触发点

成就检查不放在 Context 内部自动触发，而是由 App 层在关键事件后显式调用，避免循环依赖。

**触发点设计**（在 `App.tsx` 或相关组件中）：

```typescript
// AppInner 中
const achievement = useAchievement()
const stamina = useStamina()

// 构建统计数据
const buildStats = useCallback((): AchievementStats => {
  return {
    totalCaptures: stamina.state.totalCaptures,
    totalBattlesWon: battleWins,      // 需额外追踪
    totalBattles: battleTotal,        // 需额外追踪
    currentWinStreak: winStreak,      // 需额外追踪
    maxWinStreak: maxWinStreak,       // 需额外追踪
    level: stamina.state.level,
    checkInStreak: shop.state.checkIn.streak,
    citiesVisited: visitedCities.size,
    weatherTypesExperienced: experiencedWeathers,
    capturesByRarity: rarityCounts,
    capturesBySpecies: speciesCounts,
    hasLegendary: rarityCounts.legendary > 0,
    rainCapturesNoCold: rainCaptureCount,
  }
}, [stamina.state, /* ... */])

// 捕获成功后
const handleCaptureSuccess = useCallback((entry: CardEntry) => {
  addAnimal(entry)
  const result = stamina.addCapture(1, entry.rarity)
  // ... 金币掉落、感冒判定
  // 检查成就
  achievement.checkAchievements(buildStats())
  setActiveTab('collection')
}, [/* ... */])

// 战斗结束后
const handleBattleFinish = useCallback(() => {
  battle.finishBattle()
  // 检查成就
  achievement.checkAchievements(buildStats())
}, [/* ... */])

// 签到后
const handleCheckIn = useCallback(() => {
  shop.checkIn()
  // 检查成就
  achievement.checkAchievements(buildStats())
}, [/* ... */])
```

> **注意**：战斗统计（胜场数、总场次、连胜数）需要额外追踪。建议在 `StaminaState` 中新增统计字段，或在 `AchievementState` 中维护统计计数器。最简方案是在 `StaminaState` 中新增 `totalBattlesWon` / `totalBattles` / `maxWinStreak` / `currentWinStreak` 字段。

### 6.5 StaminaState 战斗统计扩展（建议）

```typescript
// stamina/types.ts 追加字段
export interface StaminaState {
  // ... 已有字段
  /** 累计战斗胜场 */
  totalBattlesWon: number
  /** 累计战斗总场次 */
  totalBattles: number
  /** 当前连胜数 */
  currentWinStreak: number
  /** 历史最高连胜 */
  maxWinStreak: number
}
```

对应 Reducer Action：

```typescript
| { type: 'RECORD_BATTLE'; result: 'win' | 'lose' | 'draw' }
```

对应 Reducer 逻辑：

```typescript
case 'RECORD_BATTLE': {
  const won = action.result === 'win'
  const newWinStreak = won ? state.currentWinStreak + 1 : 0
  return {
    ...state,
    totalBattles: state.totalBattles + 1,
    totalBattlesWon: state.totalBattlesWon + (won ? 1 : 0),
    currentWinStreak: newWinStreak,
    maxWinStreak: Math.max(state.maxWinStreak, newWinStreak),
  }
}
```

---

## 7. UI 集成

### 7.1 TopBar 修改：等级 + 经验进度条

```
┌──────────────────────────────────────────┐
│  📍 宁波 · 晴朗                          │
│  Lv.5 ▓▓▓▓▓▓▓▓░░░░ 350/450  💰 1280     │
│  ⚡ 160/176                              │
└──────────────────────────────────────────┘
```

- `Lv.N` 显示当前等级
- 进度条显示 `currentLevelExp / nextLevelExp`
- 点击等级区域可跳转成就页面

### 7.2 AchievementScreen 成就列表页

```
┌──────────────────────────────────────────┐
│  ← 成就                          12/22   │
│  完成率 55%                              │
├──────────────────────────────────────────┤
│ [全部] [收集] [战斗] [探索] [社交] [里程碑] │
├──────────────────────────────────────────┤
│  🎯 初出茅庐              ✅ 已解锁       │
│  收藏第 1 只动物                         │
│  奖励: 50 金币                          │
├──────────────────────────────────────────┤
│  📦 收藏家               ▓▓▓▓░░ 35/50   │
│  收藏 50 只动物                         │
│  奖励: 200 金币 + 称号「收藏家」          │
├──────────────────────────────────────────┤
│  ⭐ 传说降临             ░░░░░░ 0/1      │
│  捕获第 1 只传说级动物                    │
│  奖励: 500 金币 + 称号「传说捕手」        │
├──────────────────────────────────────────┤
│  🏆 称号: [ 无 ] ▼                       │
└──────────────────────────────────────────┘
```

- 分类标签切换筛选
- 每项显示图标、名称、描述、进度条、奖励
- 已解锁显示 ✅，未解锁显示进度条
- 底部称号选择器

### 7.3 LevelUpToast 升级弹窗

```
┌──────────────────────────────────────────┐
│                                          │
│              ⬆️ 等级提升！                │
│                                          │
│           ╔═══════════╗                 │
│           ║   Lv. 6   ║                 │
│           ╚═══════════╝                 │
│                                          │
│         💰 +400 金币                     │
│         ⚡ 体力已恢复满                   │
│         📈 体力上限提升至 190             │
│         🛒 商店稀有道具率 +10%            │
│                                          │
│         [ 确认 ]                         │
│                                          │
└──────────────────────────────────────────┘
```

- 全屏半透明遮罩
- 等级数字放大动画
- 奖励列表逐行显示
- 点击确认关闭

### 7.4 AchievementToast 成就解锁弹窗

```
┌──────────────────────────────────────────┐
│  🏆 成就解锁！                           │
│  🎯 初出茅庐                             │
│  收藏第 1 只动物                         │
│  奖励: 50 金币                          │
│                              [ 确定 ]    │
└──────────────────────────────────────────┘
```

- 从顶部滑入
- 队列展示（一次一个）
- 3 秒自动消失或点击关闭

### 7.5 App.tsx Provider 嵌套修改

```typescript
const App: React.FC = () => {
  return (
    <StaminaProvider>
      <LbsProvider>
        <WeatherProvider>
          <ShopProvider>
            <StatusProvider>
              <BattleProvider>
                <AchievementProvider>
                  <AppInner />
                </AchievementProvider>
              </BattleProvider>
            </StatusProvider>
          </ShopProvider>
        </WeatherProvider>
      </LbsProvider>
    </StaminaProvider>
  )
}
```

---

## 8. 测试用例

### 8.1 stamina/logic.test.ts 新增用例

```typescript
describe('getLevelForExp', () => {
  it('#28 exp=0 返回 Lv.1', () => {
    expect(getLevelForExp(0)).toBe(1)
  })
  it('#29 exp=100 刚好升 Lv.2', () => {
    expect(getLevelForExp(100)).toBe(2)
  })
  it('#30 exp=500 跨级到 Lv.4', () => {
    expect(getLevelForExp(500)).toBe(4)
  })
  it('#31 exp=9999 满级后仍为 Lv.10', () => {
    expect(getLevelForExp(9999)).toBe(10)
  })
})

describe('tryLevelUp (exp 驱动)', () => {
  it('#32 未达升级条件 level=2, exp=150 => 不升级', () => {
    const result = tryLevelUp(2, 150)
    expect(result.leveledUp).toBe(false)
    expect(result.newLevel).toBe(2)
    expect(result.rewardGold).toBe(0)
    expect(result.crossedLevels).toEqual([])
  })
  it('#33 刚好升级一级 level=2, exp=250 => Lv.3, rewardGold=150', () => {
    const result = tryLevelUp(2, 250)
    expect(result.leveledUp).toBe(true)
    expect(result.newLevel).toBe(3)
    expect(result.rewardGold).toBe(150)
    expect(result.crossedLevels).toEqual([3])
  })
  it('#34 跨级升级 level=2, exp=500 => Lv.4, rewardGold=350', () => {
    const result = tryLevelUp(2, 500)
    expect(result.leveledUp).toBe(true)
    expect(result.newLevel).toBe(4)
    expect(result.rewardGold).toBe(350) // 150 + 200
    expect(result.crossedLevels).toEqual([3, 4])
  })
  it('#35 满级后不升级 level=10, exp=9999', () => {
    const result = tryLevelUp(10, 9999)
    expect(result.leveledUp).toBe(false)
    expect(result.newLevel).toBe(10)
    expect(result.rewardGold).toBe(0)
  })
})

describe('getExpProgress', () => {
  it('#36 Lv.1 exp=0 => progress=0', () => {
    const result = getExpProgress(1, 0)
    expect(result.currentLevelExp).toBe(0)
    expect(result.nextLevelExp).toBe(100)
    expect(result.progress).toBe(0)
  })
  it('#37 Lv.1 exp=50 => progress=50', () => {
    const result = getExpProgress(1, 50)
    expect(result.currentLevelExp).toBe(50)
    expect(result.nextLevelExp).toBe(100)
    expect(result.progress).toBe(50)
  })
  it('#38 Lv.5 exp=700 => 刚好升级边界 progress=0（新等级起始）', () => {
    const result = getExpProgress(5, 700)
    expect(result.currentLevelExp).toBe(0)
    expect(result.nextLevelExp).toBe(300) // 1000 - 700
    expect(result.progress).toBe(0)
  })
  it('#39 Lv.10 满级 => progress=100', () => {
    const result = getExpProgress(10, 5000)
    expect(result.progress).toBe(100)
  })
})

describe('getCaptureXp', () => {
  it('#40 common 捕获得 8 XP', () => {
    expect(getCaptureXp('common')).toBe(8)
  })
  it('#41 legendary 捕获得 120 XP', () => {
    expect(getCaptureXp('legendary')).toBe(120)
  })
})

describe('getBattleXp', () => {
  it('#42 胜利 common 敌 => 20 XP', () => {
    expect(getBattleXp('win', 'common')).toBe(20)
  })
  it('#43 胜利 legendary 敌 => 40 XP', () => {
    expect(getBattleXp('win', 'legendary')).toBe(40) // 20 + 20
  })
  it('#44 失败 => 5 XP', () => {
    expect(getBattleXp('lose', 'rare')).toBe(5)
  })
  it('#45 平局 => 10 XP', () => {
    expect(getBattleXp('draw', 'epic')).toBe(10)
  })
})

describe('migrateState', () => {
  it('#46 旧存档无 exp 字段，按 totalCaptures×10 推算', () => {
    const oldSave = { level: 3, totalCaptures: 25 } as any
    const migrated = migrateState(oldSave)
    expect(migrated.exp).toBe(250)
    expect(migrated.level).toBe(3)
  })
  it('#47 新存档有 exp 字段，保持不变', () => {
    const newSave = { level: 2, exp: 150, totalCaptures: 12 } as any
    const migrated = migrateState(newSave)
    expect(migrated.exp).toBe(150)
    expect(migrated.level).toBe(2)
  })
  it('#48 缺失字段补全默认值', () => {
    const partial = { level: 1 } as any
    const migrated = migrateState(partial)
    expect(migrated.currentStamina).toBe(120)
    expect(migrated.gold).toBe(0)
    expect(migrated.exp).toBe(0)
  })
})
```

### 8.2 achievement/logic.test.ts（新建，≥18 用例）

```typescript
import { describe, it, expect } from 'vitest'
import {
  isAchievementUnlocked,
  getAchievementCurrent,
  checkAchievements,
  getAchievementProgress,
  getAllAchievementProgress,
  getAllTitles,
  getUnlockedTitles,
  getCompletionRate,
  getTotalRewardGold,
} from './logic'
import { ACHIEVEMENT_DEFS, ACHIEVEMENT_MAP } from './constants'
import type { AchievementStats } from './types'

// 构建测试用统计数据
function makeStats(overrides: Partial<AchievementStats> = {}): AchievementStats {
  return {
    totalCaptures: 0,
    totalBattlesWon: 0,
    totalBattles: 0,
    currentWinStreak: 0,
    maxWinStreak: 0,
    level: 1,
    checkInStreak: 0,
    citiesVisited: 0,
    weatherTypesExperienced: [],
    capturesByRarity: { common: 0, uncommon: 0, rare: 0, epic: 0, legendary: 0 },
    capturesBySpecies: { cat: 0, goose: 0, dog: 0 },
    hasLegendary: false,
    rainCapturesNoCold: 0,
    ...overrides,
  }
}

describe('isAchievementUnlocked', () => {
  it('#1 total_captures=1 解锁 first_capture', () => {
    const def = ACHIEVEMENT_MAP['first_capture']
    const stats = makeStats({ totalCaptures: 1 })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })
  it('#2 total_captures=0 未解锁 first_capture', () => {
    const def = ACHIEVEMENT_MAP['first_capture']
    const stats = makeStats({ totalCaptures: 0 })
    expect(isAchievementUnlocked(def, stats)).toBe(false)
  })
  it('#3 captures_by_rarity rare=10 解锁 rare_collector_10', () => {
    const def = ACHIEVEMENT_MAP['rare_collector_10']
    const stats = makeStats({
      capturesByRarity: { common: 5, uncommon: 3, rare: 10, epic: 0, legendary: 0 },
    })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })
  it('#4 captures_by_species cat=10 解锁 cat_lover_10', () => {
    const def = ACHIEVEMENT_MAP['cat_lover_10']
    const stats = makeStats({
      capturesBySpecies: { cat: 10, goose: 2, dog: 3 },
    })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })
  it('#5 level=5 解锁 level_5', () => {
    const def = ACHIEVEMENT_MAP['level_5']
    const stats = makeStats({ level: 5 })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })
  it('#6 level=4 未解锁 level_5', () => {
    const def = ACHIEVEMENT_MAP['level_5']
    const stats = makeStats({ level: 4 })
    expect(isAchievementUnlocked(def, stats)).toBe(false)
  })
  it('#7 hasLegendary=true 解锁 legendary_captured', () => {
    const def = ACHIEVEMENT_MAP['legendary_captured']
    const stats = makeStats({ hasLegendary: true })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })
  it('#8 all_weather_types 7种天气解锁 weather_master', () => {
    const def = ACHIEVEMENT_MAP['weather_master']
    const stats = makeStats({
      weatherTypesExperienced: ['sunny', 'cloudy', 'overcast', 'rainy', 'snowy', 'foggy', 'extreme'],
    })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })
  it('#9 win_streak=5 解锁 win_streak_5', () => {
    const def = ACHIEVEMENT_MAP['win_streak_5']
    const stats = makeStats({ maxWinStreak: 5 })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })
  it('#10 win_streak=4 未解锁 win_streak_5', () => {
    const def = ACHIEVEMENT_MAP['win_streak_5']
    const stats = makeStats({ maxWinStreak: 4 })
    expect(isAchievementUnlocked(def, stats)).toBe(false)
  })
})

describe('getAchievementCurrent', () => {
  it('#11 total_captures=35 时 collector_50 进度=35', () => {
    const def = ACHIEVEMENT_MAP['collector_50']
    const stats = makeStats({ totalCaptures: 35 })
    expect(getAchievementCurrent(def, stats)).toBe(35)
  })
  it('#12 legendary_captured 未捕获时进度=0', () => {
    const def = ACHIEVEMENT_MAP['legendary_captured']
    const stats = makeStats({ hasLegendary: false })
    expect(getAchievementCurrent(def, stats)).toBe(0)
  })
  it('#13 weather_master 3种天气时进度=3', () => {
    const def = ACHIEVEMENT_MAP['weather_master']
    const stats = makeStats({
      weatherTypesExperienced: ['sunny', 'rainy', 'snowy'],
    })
    expect(getAchievementCurrent(def, stats)).toBe(3)
  })
})

describe('checkAchievements', () => {
  it('#14 首次捕获解锁 first_capture', () => {
    const stats = makeStats({ totalCaptures: 1 })
    const alreadyUnlocked = new Set<string>()
    const result = checkAchievements(stats, alreadyUnlocked)
    expect(result.changed).toBe(true)
    expect(result.newlyUnlocked).toContain('first_capture')
  })
  it('#15 已解锁的成就不重复解锁', () => {
    const stats = makeStats({ totalCaptures: 1 })
    const alreadyUnlocked = new Set(['first_capture'])
    const result = checkAchievements(stats, alreadyUnlocked)
    expect(result.newlyUnlocked).not.toContain('first_capture')
  })
  it('#16 一次满足多个条件时批量解锁', () => {
    const stats = makeStats({
      totalCaptures: 50,
      totalBattlesWon: 1,
      level: 5,
    })
    const alreadyUnlocked = new Set<string>()
    const result = checkAchievements(stats, alreadyUnlocked)
    expect(result.newlyUnlocked.length).toBeGreaterThanOrEqual(3)
    expect(result.newlyUnlocked).toContain('collector_50')
    expect(result.newlyUnlocked).toContain('first_battle_win')
    expect(result.newlyUnlocked).toContain('level_5')
  })
  it('#17 无新成就时 changed=false', () => {
    const stats = makeStats({ totalCaptures: 0 })
    const alreadyUnlocked = new Set<string>()
    const result = checkAchievements(stats, alreadyUnlocked)
    expect(result.changed).toBe(false)
    expect(result.newlyUnlocked).toEqual([])
  })
})

describe('getAchievementProgress', () => {
  it('#18 未解锁成就 percent=50', () => {
    const def = ACHIEVEMENT_MAP['collector_50']
    const stats = makeStats({ totalCaptures: 25 })
    const progress = getAchievementProgress(def, stats, false)
    expect(progress.current).toBe(25)
    expect(progress.target).toBe(50)
    expect(progress.unlocked).toBe(false)
    expect(progress.percent).toBe(50)
  })
  it('#19 已解锁成就 percent=100', () => {
    const def = ACHIEVEMENT_MAP['collector_50']
    const stats = makeStats({ totalCaptures: 60 })
    const progress = getAchievementProgress(def, stats, true)
    expect(progress.unlocked).toBe(true)
    expect(progress.percent).toBe(100)
  })
})

describe('getAllAchievementProgress', () => {
  it('#20 返回所有成就的进度', () => {
    const stats = makeStats()
    const unlockedIds = new Set<string>()
    const list = getAllAchievementProgress(stats, unlockedIds)
    expect(list.length).toBe(ACHIEVEMENT_DEFS.length)
  })
})

describe('getUnlockedTitles', () => {
  it('#21 从已解锁成就中提取称号', () => {
    const unlocked = [
      { id: 'collector_50', unlockedAt: 0 },
      { id: 'first_capture', unlockedAt: 0 },
    ]
    const titles = getUnlockedTitles(unlocked)
    expect(titles).toContain('收藏家')
    expect(titles).not.toContain('初出茅庐') // first_capture 无称号奖励
  })
  it('#22 无称号成就不返回', () => {
    const unlocked = [{ id: 'first_capture', unlockedAt: 0 }]
    const titles = getUnlockedTitles(unlocked)
    expect(titles).toEqual([])
  })
})

describe('getCompletionRate', () => {
  it('#23 0 个解锁 => 0%', () => {
    expect(getCompletionRate(0)).toBe(0)
  })
  it('#24 全部解锁 => 100%', () => {
    expect(getCompletionRate(ACHIEVEMENT_DEFS.length)).toBe(100)
  })
  it('#25 半数解锁 => 50%', () => {
    expect(getCompletionRate(Math.floor(ACHIEVEMENT_DEFS.length / 2))).toBe(
      Math.round((Math.floor(ACHIEVEMENT_DEFS.length / 2) / ACHIEVEMENT_DEFS.length) * 100)
    )
  })
})

describe('getTotalRewardGold', () => {
  it('#26 计算已解锁成就的金币总奖励', () => {
    const unlockedIds = new Set(['first_capture', 'collector_50'])
    const total = getTotalRewardGold(unlockedIds)
    expect(total).toBe(250) // 50 + 200
  })
})
```

**测试用例统计**：stamina 新增 21 个（#28~#48），achievement 新增 26 个（#1~#26），合计 47 个。

---

## 9. 实现步骤

### 步骤 1：扩展 StaminaContext（XP 系统）

| 序号 | 文件 | 操作 | 说明 |
|------|------|------|------|
| 1.1 | `stamina/constants.ts` | 修改 | `LEVEL_TABLE` 增加 `requiredExp` / `shopBonus`；新增 `CAPTURE_XP` / `BATTLE_WIN_XP` / `BATTLE_LOSE_XP` / `BATTLE_DRAW_XP` / `BATTLE_WIN_RARITY_BONUS` / `CHECK_IN_XP` / `DISPATCH_XP` / `MAX_LEVEL` |
| 1.2 | `stamina/types.ts` | 修改 | `StaminaState` 增加 `exp` / `totalBattlesWon` / `totalBattles` / `currentWinStreak` / `maxWinStreak`；`LevelTableRow` 增加 `requiredExp` / `shopBonus`；`LevelUpResult` 增加 `crossedLevels`；`StaminaAction` 增加 `ADD_EXP` / `RECORD_BATTLE`；`StaminaContextValue` 增加 `addExp` / `nextLevelExp` / `currentLevelExp` / `expProgress` |
| 1.3 | `stamina/logic.ts` | 修改 | 新增 `getLevelForExp` / `getExpProgress` / `getCaptureXp` / `getBattleXp` / `migrateState`；修改 `tryLevelUp` 为 exp 驱动 |
| 1.4 | `stamina/StaminaContext.tsx` | 修改 | `loadInitialState` 使用 `migrateState`；Reducer 增加 `ADD_EXP` / `RECORD_BATTLE`；`ADD_CAPTURE` 改走 exp；新增 `addExp` / `recordBattle` |
| 1.5 | `stamina/logic.test.ts` | 修改 | 新增 #28~#48 共 21 个测试用例 |

### 步骤 2：新建 Achievement 模块

| 序号 | 文件 | 操作 | 说明 |
|------|------|------|------|
| 2.1 | `achievement/types.ts` | 新建 | 所有类型定义 |
| 2.2 | `achievement/constants.ts` | 新建 | `ACHIEVEMENT_DEFS`（22 项）、`ACHIEVEMENT_MAP`、`CATEGORY_LABELS`、`RARITY_LABELS`、`ACHIEVEMENT_STORAGE_KEY` |
| 2.3 | `achievement/logic.ts` | 新建 | `isAchievementUnlocked` / `getAchievementCurrent` / `checkAchievements` / `getAchievementProgress` / `getAllAchievementProgress` / `getAllTitles` / `getUnlockedTitles` / `getCompletionRate` / `getTotalRewardGold` |
| 2.4 | `achievement/logic.test.ts` | 新建 | #1~#26 共 26 个测试用例 |
| 2.5 | `achievement/AchievementContext.tsx` | 新建 | Reducer + Provider + 持久化 |
| 2.6 | `achievement/useAchievement.ts` | 新建 | 自定义 Hook |

### 步骤 3：集成到现有系统

| 序号 | 文件 | 操作 | 说明 |
|------|------|------|------|
| 3.1 | `battle/logic.ts` | 修改 | `computeRewards` 的 `exp` 字段改用 `getBattleXp()` 计算 |
| 3.2 | `battle/BattleContext.tsx` | 修改 | `finishBattle` 调用 `stamina.addExp()` + `stamina.recordBattle()` |
| 3.3 | `shop/ShopContext.tsx` | 修改 | `checkIn` 调用 `stamina.addExp(CHECK_IN_XP)` |
| 3.4 | `App.tsx` | 修改 | 增加 `AchievementProvider`；`AppInner` 在捕获/战斗/签到后调用 `achievement.checkAchievements()` |

### 步骤 4：UI 组件

| 序号 | 文件 | 操作 | 说明 |
|------|------|------|------|
| 4.1 | `components/TopBar.tsx` | 修改 | 显示等级 + 经验进度条 |
| 4.2 | `components/AchievementScreen.tsx` | 新建 | 成就列表页（分类标签 + 进度条） |
| 4.3 | `components/LevelUpToast.tsx` | 新建 | 升级弹窗动画 |
| 4.4 | `components/AchievementToast.tsx` | 新建 | 成就解锁通知 |

### 步骤 5：验证

| 序号 | 操作 | 说明 |
|------|------|------|
| 5.1 | `npm test` | 全量测试通过 |
| 5.2 | `npm run build` | TypeScript 编译无错误 |
| 5.3 | 手动测试 | 捕获/战斗/签到触发经验+升级+成就解锁 |

---

## 10. 验收标准

### 10.1 功能验收

| 编号 | 验收项 | 验证方法 |
|------|--------|---------|
| AC-1 | 捕获动物后获得对应稀有度的经验值 | 捕获 common 获得 8 XP，legendary 获得 120 XP |
| AC-2 | 战斗胜利后获得经验值（含稀有度加成） | 胜利 legendary 敌获得 40 XP |
| AC-3 | 每日签到获得 15 经验值 | 签到后经验值 +15 |
| AC-4 | 经验值达到阈值时自动升级 | exp ≥ 100 时升到 Lv.2 |
| AC-5 | 升级时恢复满体力 | 升级后 currentStamina = newMaxStamina |
| AC-6 | 升级时发放金币奖励 | 升到 Lv.2 获得 100 金币 |
| AC-7 | 跨级升级时累计所有跨越等级的奖励 | exp 从 99 跳到 500，升到 Lv.4，获得 350 金币 |
| AC-8 | 满级后不再升级 | Lv.10 时 exp 继续增加但等级不变 |
| AC-9 | 成就在条件满足时自动解锁 | 首次捕获后「初出茅庐」解锁 |
| AC-10 | 成就解锁后发放金币奖励 | 解锁「收藏家」获得 200 金币 |
| AC-11 | 成就解锁后获得称号 | 解锁「收藏家」获得称号「收藏家」 |
| AC-12 | 已解锁成就不重复解锁 | 再次触发相同条件不再弹出解锁通知 |
| AC-13 | 成就进度条正确显示 | 35/50 显示 70% 进度 |
| AC-14 | 成就分类筛选正确 | 点击「战斗」标签只显示战斗类成就 |
| AC-15 | 旧存档可正确迁移 | 无 exp 字段的旧存档加载后 exp = totalCaptures × 10 |
| AC-16 | 升级弹窗正确显示奖励 | 显示金币、体力恢复、体力上限提升 |
| AC-17 | 成就解锁通知队列展示 | 多个成就同时解锁时逐个展示 |
| AC-18 | 战斗统计正确追踪 | 胜场数、总场次、连胜数准确 |
| AC-19 | 称号可切换佩戴 | 选择不同称号后 TopBar 显示更新 |
| AC-20 | localStorage 持久化成就状态 | 刷新页面后已解锁成就不丢失 |

### 10.2 技术验收

| 编号 | 验收项 |
|------|--------|
| TV-1 | `npm test` 全量通过（含新增 47 个测试用例） |
| TV-2 | `npm run build` 编译无 TypeScript 错误 |
| TV-3 | 无 console.warn / console.error |
| TV-4 | 成就检查不产生性能问题（22 项全量检查 < 1ms） |
| TV-5 | Provider 嵌套顺序正确，无 Context 循环依赖 |

---

## 11. 依赖与风险

### 11.1 依赖

| 依赖项 | 说明 |
|--------|------|
| StaminaContext (#31) | 等级/经验/金币/体力管理，本 Issue 需修改其 types/constants/logic/Context |
| BattleContext (#37) | 战斗奖励发放，需修改 `computeRewards` 和 `finishBattle` |
| ShopContext (#34) | 签到经验发放，需修改 `checkIn` |
| WeatherContext (#38) | 提供天气类型，用于天气相关成就统计 |
| LbsContext (#36) | 提供位置信息，用于城市探索成就统计 |
| StatusContext (#39) | 提供感冒状态，用于雨天勇士成就统计 |

### 11.2 风险与应对

| 风险 | 影响 | 应对策略 |
|------|------|---------|
| **旧存档兼容** | 旧存档无 `exp` 字段，加载时可能崩溃或等级重置为 1 | `migrateState()` 函数检测缺失字段并推算 `exp = totalCaptures × 10`，确保升级节奏不变 |
| **Provider 循环依赖** | `AchievementContext` 消费 `StaminaContext`，而 `StaminaContext` 不依赖 `AchievementContext` | `AchievementProvider` 位于最内层，单向依赖 `StaminaContext`，无循环 |
| **成就检查性能** | 每次捕获/战斗/签到后全量检查 22 项成就 | 22 项遍历为纯内存比较，<1ms，无需优化；若未来成就扩展到 100+ 可考虑事件驱动过滤 |
| **统计数据收集** | `AchievementStats` 需要从多个 Context 汇总数据 | 在 `AppInner` 层构建 `buildStats()` 函数统一收集，避免散落在各组件中 |
| **战斗统计缺失** | 当前 `StaminaState` 无战斗场次/连胜字段 | 在 `StaminaState` 中新增 `totalBattlesWon` / `totalBattles` / `currentWinStreak` / `maxWinStreak`，通过 `RECORD_BATTLE` action 维护 |
| **成就奖励金币重复发放** | Reducer 重复 dispatch 导致同一成就多次奖励 | `checkAchievements` 纯函数先过滤已解锁集合，只返回新解锁 ID；Reducer 中 `UNLOCK_ACHIEVEMENTS` 只处理新 ID |
| **成就通知队列丢失** | 页面刷新时 `pendingNotifications` 未持久化 | `pendingNotifications` 持久化到 localStorage；但为避免刷新后弹窗过多，加载时清空队列（成就已解锁，仅通知不重复） |
| **社交成就预留** | `guild_member` / `guild_leader` 成就当前无法解锁 | 条件设为 `cities_visited: 0` / `cities_visited: 999`（不可能达成），公会系统上线后修改条件 |
| **exp 溢出** | 长期游玩 exp 持续增长 | JavaScript number 精度足够（最大安全整数 2^53 - 1），Lv.10 后 exp 继续累加但不再升级，无溢出风险 |
| **派遣经验未接入** | 派遣系统尚未实现，`DISPATCH_XP` 暂无调用方 | 常量已定义，派遣系统上线时调用 `stamina.addExp(DISPATCH_XP)` 即可 |

---

## 12. 数据流总览

```
┌─────────────────────────────────────────────────────────────────┐
│                        数据流架构                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  捕获成功                                                        │
│  ├──> StaminaContext.addCapture(1, rarity)                      │
│  │    ├──> exp += CAPTURE_XP[rarity]                            │
│  │    ├──> totalCaptures += 1                                   │
│  │    ├──> capturesByRarity[rarity] += 1                        │
│  │    ├──> tryLevelUp(level, exp) ──> 升级?                     │
│  │    │    └──> 恢复满体力 + 金币奖励                            │
│  │    └──> return LevelUpResult                                 │
│  │                                                               │
│  ├──> AchievementContext.checkAchievements(stats)               │
│  │    ├──> 遍历 22 项成就定义                                    │
│  │    ├──> 过滤已解锁                                            │
│  │    ├──> 返回新解锁列表                                        │
│  │    └──> 发放金币 + 称号 + 通知                                │
│  │                                                               │
│  └──> UI: LevelUpToast / AchievementToast                       │
│                                                                 │
│  战斗结束                                                        │
│  ├──> StaminaContext.addExp(battleXp)                           │
│  ├──> StaminaContext.recordBattle('win' | 'lose' | 'draw')      │
│  ├──> StaminaContext.addGold(rewards.gold)                      │
│  └──> AchievementContext.checkAchievements(stats)               │
│                                                                 │
│  每日签到                                                        │
│  ├──> StaminaContext.addGold(reward)                            │
│  ├──> StaminaContext.addExp(CHECK_IN_XP)                        │
│  └──> AchievementContext.checkAchievements(stats)               │
│                                                                 │
│  派遣完成（未来）                                                 │
│  ├──> StaminaContext.addExp(DISPATCH_XP)                        │
│  └──> AchievementContext.checkAchievements(stats)               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

*本计划覆盖 Issue #41 全部要求：XP 驱动的等级系统（4 种经验来源）、22 项成就（5 大类别）、称号系统、UI 集成（进度条/升级弹窗/成就列表/通知队列）、47 个测试用例、旧存档迁移方案。*

