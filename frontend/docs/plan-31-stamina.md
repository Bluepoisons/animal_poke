# [M1] 体力系统实现计划 — Issue #31

> **验收标准**：每小时恢复 10 点，单次捕获消耗 20，上限 120→240（满级），升级恢复满体力
>
> **设计文档来源**：`游戏开发计划.md` 3.4 体力系统 + 5.5 等级与体力数值
>
> **技术栈**：React 18 + Vite 6 + TypeScript 5.6，无额外依赖

---

## 1. 文件结构

```
frontend/src/
├── stamina/
│   ├── constants.ts      # 等级表常量 LEVEL_TABLE + 体力相关常量（消耗值、恢复速率、限购次数等）
│   ├── types.ts          # StaminaState 接口、LevelTableRow 类型、Action 类型定义
│   ├── logic.ts          # 纯函数：getMaxStamina / getLevelForCaptures / calculateRecovery 等（无副作用，可单测）
│   ├── StaminaContext.tsx # React Context + useReducer provider，含自然恢复 setInterval + localStorage 持久化
│   └── useStamina.ts     # 自定义 Hook：封装 Context 消费，暴露给组件使用的 API
├── App.tsx               # （修改）包裹 <StaminaProvider>，TopBar 改为从 Context 读取
└── components/
    └── TopBar.tsx        # （修改）props 改为可选，默认从 useStamina() 读取
```

### 各文件职责

| 文件 | 职责 | 依赖 |
|------|------|------|
| `constants.ts` | 等级表数据、体力消耗/恢复/限购等常量。纯数据，无逻辑 | 无 |
| `types.ts` | TypeScript 类型定义。纯类型，无运行时代码 | 无 |
| `logic.ts` | 纯函数计算逻辑。不依赖 React，可直接被测试文件 import | constants, types |
| `StaminaContext.tsx` | 状态管理核心。创建 Context + Reducer，挂载自然恢复定时器，读写 localStorage | constants, types, logic |
| `useStamina.ts` | 对外暴露的 Hook。封装 `useContext(StaminaContext)`，处理 null 检查 | StaminaContext, types |

---

## 2. 核心数据模型

### 2.1 类型定义 (`types.ts`)

```typescript
/** 等级表单行结构 */
export interface LevelTableRow {
  level: number
  /** 升级所需累计捕获数 */
  requiredCaptures: number
  /** 该等级体力上限 */
  maxStamina: number
  /** 升级奖励金币数（Lv.1 无升级奖励，为 0） */
  rewardGold: number
  /** 是否为满级 */
  isMaxLevel: boolean
}

/** 体力系统完整状态 */
export interface StaminaState {
  /** 当前等级（1~10） */
  level: number
  /** 当前体力值（0 ~ maxStamina） */
  currentStamina: number
  /** 累计捕获数（用于判定升级） */
  totalCaptures: number
  /** 上次自然恢复结算的时间戳（Unix ms） */
  lastRecoverTime: number
  /** 当前金币数 */
  gold: number
  /** 今日已购买体力药剂次数（0~3） */
  potionPurchasesToday: number
  /** 今日购买计数的日期标记（自然日，格式 'YYYY-MM-DD'） */
  potionPurchaseDate: string
}

/** 升级结果 */
export interface LevelUpResult {
  /** 是否发生了升级 */
  leveledUp: boolean
  /** 升级后的新等级（未升级则为原等级） */
  newLevel: number
  /** 升级奖励金币数（未升级则为 0） */
  rewardGold: number
}

/** 自然恢复计算结果 */
export interface RecoveryResult {
  /** 恢复后的体力值 */
  current: number
  /** 距离下次恢复还需多少秒 */
  recoverTime: number
}

/** 购买体力药剂结果 */
export interface BuyPotionResult {
  /** 是否购买成功 */
  success: boolean
  /** 今日剩余购买次数 */
  remainingPurchases: number
  /** 失败原因（success=false 时有值） */
  reason?: 'insufficient_gold' | 'daily_limit_reached'
}

/** Reducer Action 类型 */
export type StaminaAction =
  | { type: 'TICK_RECOVERY'; now: number }
  | { type: 'CONSUME'; amount: number }
  | { type: 'ADD_STAMINA'; amount: number }
  | { type: 'ADD_CAPTURE'; count: number }
  | { type: 'ADD_GOLD'; amount: number }
  | { type: 'BUY_POTION' }
  | { type: 'RESET_DAILY_PURCHASES'; date: string }
  | { type: 'LOAD_STATE'; state: StaminaState }
```

---

## 3. 状态管理方案

### 推荐：React Context + useReducer

**理由**：

1. **依赖零增量**：当前 `package.json` 只有 react + react-dom，不引入 zustand 可保持依赖最小化，符合 KISS。
2. **状态范围可控**：体力系统是单一领域状态，Action 类型有限（~8 种），Reducer 完全可控。不需要 zustand 的 selector 细粒度订阅优化。
3. **与现有代码一致**：现有代码全部使用 React 内置 Hook（useState / useCallback / useMemo），Context + useReducer 是同范式。
4. **后续可迁移**：如果 #32 之后状态变复杂（图鉴 + 宠物 + 社交），可无痛迁移到 zustand——Reducer 逻辑是纯函数，搬迁成本低。

### Context 结构

```typescript
interface StaminaContextValue {
  state: StaminaState
  /** 当前等级对应的体力上限 */
  maxStamina: number
  /** 距离下次恢复的秒数（用于 UI 倒计时显示） */
  nextRecoverIn: number
  /** 消耗体力，返回是否成功（体力不足返回 false） */
  consumeStamina: (amount: number) => boolean
  /** 增加体力（不超过上限） */
  addStamina: (amount: number) => void
  /** 增加捕获数，内部自动检查升级 */
  addCapture: (count: number) => LevelUpResult
  /** 增加金币 */
  addGold: (amount: number) => void
  /** 购买体力药剂（150 金币换 3 体力，每日限 3 次） */
  buyStaminaPotion: () => BuyPotionResult
}
```

---

## 4. 核心逻辑函数签名 (`logic.ts`)

### 4.1 等级表常量 (`constants.ts`)

```typescript
/** 等级表（索引 0 = Lv.1，索引 9 = Lv.10） */
export const LEVEL_TABLE: LevelTableRow[] = [
  { level: 1,  requiredCaptures: 0,   maxStamina: 120, rewardGold: 0,    isMaxLevel: false },
  { level: 2,  requiredCaptures: 10,  maxStamina: 134, rewardGold: 100,  isMaxLevel: false },
  { level: 3,  requiredCaptures: 25,  maxStamina: 148, rewardGold: 150,  isMaxLevel: false },
  { level: 4,  requiredCaptures: 45,  maxStamina: 162, rewardGold: 200,  isMaxLevel: false },
  { level: 5,  requiredCaptures: 70,  maxStamina: 176, rewardGold: 300,  isMaxLevel: false },
  { level: 6,  requiredCaptures: 100, maxStamina: 190, rewardGold: 400,  isMaxLevel: false },
  { level: 7,  requiredCaptures: 140, maxStamina: 204, rewardGold: 500,  isMaxLevel: false },
  { level: 8,  requiredCaptures: 190, maxStamina: 218, rewardGold: 600,  isMaxLevel: false },
  { level: 9,  requiredCaptures: 250, maxStamina: 232, rewardGold: 700,  isMaxLevel: false },
  { level: 10, requiredCaptures: 320, maxStamina: 240, rewardGold: 1000, isMaxLevel: true  },
]

/** 每小时恢复体力点数 */
export const STAMINA_RECOVERY_PER_HOUR = 10

/** 自然恢复检查间隔（毫秒），每分钟检查一次 */
export const RECOVERY_TICK_MS = 60_000

/** 单次捕获体力消耗 */
export const CAPTURE_STAMINA_COST = 20

/** 单次派遣淘金币体力消耗 */
export const DISPATCH_STAMINA_COST = 20

/** 体力药剂恢复量 */
export const POTION_RECOVERY = 3

/** 体力药剂价格（金币） */
export const POTION_PRICE = 150

/** 每日限购次数 */
export const POTION_DAILY_LIMIT = 3
```

### 4.2 纯函数签名 (`logic.ts`)

```typescript
/**
 * 根据等级获取体力上限
 * @param level 玩家等级（1~10）
 * @returns 该等级的体力上限
 */
export function getMaxStamina(level: number): number

/**
 * 根据累计捕获数计算应处等级
 * @param totalCaptures 累计捕获数
 * @returns 对应等级（1~10）
 */
export function getLevelForCaptures(totalCaptures: number): number

/**
 * 计算自然恢复后的体力值
 * 基于时间戳差值计算，不是每 6 分钟触发一次
 *
 * @param lastRecoverTime 上次恢复时间戳（Unix ms）
 * @param currentStamina 当前体力值
 * @param maxStamina 体力上限
 * @param now 当前时间戳（Unix ms），可选，默认 Date.now()
 * @returns { current: 恢复后体力, recoverTime: 距下次恢复秒数 }
 */
export function calculateRecovery(
  lastRecoverTime: number,
  currentStamina: number,
  maxStamina: number,
  now?: number
): RecoveryResult

/**
 * 检查并执行升级
 * 比较累计捕获数与等级表，如果跨越一个或多个等级则返回升级结果
 * 注意：可能跨多级升级（如从 Lv.2 直接升到 Lv.4），奖励金币累加
 *
 * @param currentLevel 当前等级
 * @param totalCaptures 累计捕获数
 * @returns { leveledUp, newLevel, rewardGold }
 */
export function tryLevelUp(
  currentLevel: number,
  totalCaptures: number
): LevelUpResult

/**
 * 检查体力是否足够消耗
 * @param currentStamina 当前体力
 * @param amount 消耗量
 * @returns 是否足够
 */
export function canConsume(currentStamina: number, amount: number): boolean

/**
 * 获取今日日期标记（自然日，格式 'YYYY-MM-DD'）
 * 用于每日限购重置判断
 * @param now 时间戳，可选
 * @returns 日期字符串
 */
export function getTodayString(now?: number): string

/**
 * 检查购买日期是否需要重置
 * 如果 potionPurchaseDate 不是今天，则剩余次数应重置为 POTION_DAILY_LIMIT
 *
 * @param potionPurchaseDate 存储中的购买日期
 * @param now 当前时间戳，可选
 * @returns 是否需要重置
 */
export function shouldResetDailyPurchases(
  potionPurchaseDate: string,
  now?: number
): boolean

/**
 * 计算购买体力药剂的结果（纯函数，不修改状态）
 *
 * @param gold 当前金币
 * @param potionPurchasesToday 今日已购次数
 * @returns 购买结果
 */
export function calculateBuyPotion(
  gold: number,
  potionPurchasesToday: number
): BuyPotionResult
```

### 4.3 关键逻辑实现要点

#### `calculateRecovery` 实现思路

```
每小时恢复 10 点 = 每 6 分钟恢复 1 点 = 每 360 秒恢复 1 点

算法：
1. elapsed = (now - lastRecoverTime) / 1000  // 经过的秒数
2. recovered = floor(elapsed / 360)           // 应恢复的点数
3. newStamina = min(currentStamina + recovered, maxStamina)
4. // 如果已经满了，recoverTime = 0
5. // 否则 recoverTime = 360 - (elapsed % 360)
6. // 注意：如果 currentStamina + recovered 已满，recoverTime = 0
```

**关键细节**：`lastRecoverTime` 的更新策略——每次 TICK 时，如果体力未满，将 `lastRecoverTime` 更新为 `lastRecoverTime + recovered * 360_000`（即只推进已恢复部分的整周期时间，余数保留）。如果体力已满，将 `lastRecoverTime` 更新为 `now`（满后不再累积恢复）。

#### `tryLevelUp` 实现思路

```
遍历 LEVEL_TABLE，从当前等级+1 开始找：
- 如果 totalCaptures >= entry.requiredCaptures，记录该等级
- 可能跨越多个等级（如从 Lv.2 突然到 Lv.4），累加所有跨越等级的 rewardGold
- 升级时体力恢复满值（在 Reducer 中处理，不在纯函数中）
```

---

## 5. 自然恢复实现

### 方案：setInterval 每分钟检查 + 基于时间戳计算

```typescript
// StaminaContext.tsx 中的 useEffect
useEffect(() => {
  const interval = setInterval(() => {
    dispatch({ type: 'TICK_RECOVERY', now: Date.now() })
  }, RECOVERY_TICK_MS) // 60_000ms = 1 分钟

  return () => clearInterval(interval)
}, [])
```

### Reducer 中 TICK_RECOVERY 处理

```typescript
case 'TICK_RECOVERY': {
  const maxStamina = getMaxStamina(state.level)
  const { current, recoverTime } = calculateRecovery(
    state.lastRecoverTime,
    state.currentStamina,
    maxStamina,
    action.now
  )

  // 如果体力没变化，不产生新状态（避免不必要的 re-render）
  if (current === state.currentStamina) {
    return state
  }

  // 计算新的 lastRecoverTime
  const elapsed = action.now - state.lastRecoverTime
  const recoveredPoints = current - state.currentStamina
  const consumedTime = recoveredPoints * 360_000 // 恢复的点数 × 6分钟的毫秒数
  const newLastRecoverTime = current >= maxStamina
    ? action.now // 满了，重置为当前时间
    : state.lastRecoverTime + consumedTime // 推进已恢复的整周期

  return {
    ...state,
    currentStamina: current,
    lastRecoverTime: newLastRecoverTime,
  }
}
```

### 为什么不用每 6 分钟触发

- 如果用户关闭页面 1 小时后重新打开，需要一次性恢复 10 点。基于时间戳计算可以正确处理离线恢复。
- `setInterval` 只负责触发检查，实际恢复量由 `calculateRecovery` 纯函数计算。
- 每 1 分钟检查一次，UI 上的倒计时显示精度为分钟级，体验足够。

### 页面可见性优化

```typescript
// 页面从后台切回前台时立即触发一次恢复检查
useEffect(() => {
  const handleVisibilityChange = () => {
    if (document.visibilityState === 'visible') {
      dispatch({ type: 'TICK_RECOVERY', now: Date.now() })
    }
  }
  document.addEventListener('visibilitychange', handleVisibilityChange)
  return () => document.removeEventListener('visibilitychange', handleVisibilityChange)
}, [])
```

---

## 6. 持久化方案

### localStorage 存储策略

```typescript
const STORAGE_KEY = 'animal_poke_stamina'

/** 保存到 localStorage（在每次 state 变化时自动执行） */
useEffect(() => {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
}, [state])

/** 从 localStorage 加载初始状态 */
function loadInitialState(): StaminaState {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as StaminaState
      // 加载时检查每日限购是否需要重置
      if (shouldResetDailyPurchases(parsed.potionPurchaseDate)) {
        parsed.potionPurchasesToday = 0
        parsed.potionPurchaseDate = getTodayString()
      }
      // 加载时立即计算离线恢复
      const maxStamina = getMaxStamina(parsed.level)
      const { current } = calculateRecovery(
        parsed.lastRecoverTime,
        parsed.currentStamina,
        maxStamina
      )
      parsed.currentStamina = current
      return parsed
    }
  } catch (e) {
    console.warn('加载体力存档失败，使用默认值:', e)
  }

  // 默认初始状态：Lv.1，满体力，0 捕获，0 金币
  return {
    level: 1,
    currentStamina: 120,
    totalCaptures: 0,
    lastRecoverTime: Date.now(),
    gold: 0,
    potionPurchasesToday: 0,
    potionPurchaseDate: getTodayString(),
  }
}
```

### 存储字段

完整存储 `StaminaState` 的所有字段（7 个字段，JSON 约 150 字节，localStorage 5MB 限制完全无压力）。

### 注意事项

- **#32 升级路径**：后续图鉴/宠物数据量大时升级到 IndexedDB。体力系统数据量小，localStorage 足够。
- **数据校验**：加载时做基本字段校验（level 范围 1~10，currentStamina >= 0 等），异常时 fallback 到默认值。
- **版本号**：存储 key 不加版本号，后续如有 breaking change 再迁移。

---

## 7. 与 App.tsx 集成方案

### 7.1 修改 `App.tsx`

```tsx
// App.tsx 修改点
import { StaminaProvider } from './stamina/StaminaContext'

const App: React.FC = () => {
  // ... 现有逻辑保持不变 ...

  return (
    <StaminaProvider>
      <div className="phone-frame">
        <TopBar
          location="宁波·晴"
          weather="☀️"
        />
        <div style={{ ... }}>
          {renderContent()}
        </div>
        {!mapOpen && <TabBar activeTab={activeTab} onTabChange={setActiveTab} />}
      </div>
    </StaminaProvider>
  )
}
```

### 7.2 修改 `TopBar.tsx`

```tsx
// TopBar.tsx 修改点
import { useStamina } from '../stamina/useStamina'

interface TopBarProps {
  location: string
  weather: string
  // 以下字段改为可选，优先从 Context 读取
  level?: number
  stamina?: number
  maxStamina?: number
  gold?: number
}

const TopBar: React.FC<TopBarProps> = ({ location, weather, ...props }) => {
  // 从 Context 读取体力系统状态
  const stamina = useStamina()

  const level = props.level ?? stamina.state.level
  const currentStamina = props.stamina ?? stamina.state.currentStamina
  const maxStamina = props.maxStamina ?? stamina.maxStamina
  const gold = props.gold ?? stamina.state.gold

  return (
    <div style={styles.container}>
      <div style={styles.left}>
        <div style={styles.avatar}>🐱</div>
        <span className="pill">Lv.{level}</span>
        <span className="pill">⚡ {currentStamina}/{maxStamina}</span>
        <span className="pill">🪙 {gold}</span>
      </div>
      <span className="pill">{weather} {location}</span>
    </div>
  )
}
```

### 7.3 集成范围说明

| 组件 | 修改内容 | 说明 |
|------|---------|------|
| `App.tsx` | 包裹 `<StaminaProvider>`，移除硬编码 props | 入口层，改动最小 |
| `TopBar.tsx` | 从 `useStamina()` 读取值，props 改为可选 | 显示层，需接入 Context |
| 其他组件 | **本次不修改** | CaptureScreen / DiscoverScreen 等暂不接入体力消耗逻辑，后续 Issue 处理 |

> **注意**：DiscoverScreen 中有硬编码文案 `"消耗 1 食物 · 20 体力"`，本次不修改该文案，仅保证 TopBar 数值实时更新。捕获消耗逻辑在后续 Issue 中接入。

---

## 8. 测试用例清单

### 8.1 等级表函数测试

| # | 用例 | 输入 | 预期输出 | 说明 |
|---|------|------|---------|------|
| 1 | `getMaxStamina` Lv.1 | `level=1` | `120` | 初始等级上限 |
| 2 | `getMaxStamina` Lv.10 | `level=10` | `240` | 满级上限 |
| 3 | `getMaxStamina` 边界 | `level=0` | `120`（fallback 到 Lv.1）或抛异常 | 越界处理 |
| 4 | `getLevelForCaptures` 初始 | `captures=0` | `1` | 0 捕获 = Lv.1 |
| 5 | `getLevelForCaptures` 刚好升级 | `captures=10` | `2` | 10 只正好升 Lv.2 |
| 6 | `getLevelForCaptures` 跨级 | `captures=50` | `4` | 超过 Lv.3(25) 但不到 Lv.5(70) |
| 7 | `getLevelForCaptures` 满级后 | `captures=500` | `10` | 超过满级条件仍为 10 |

### 8.2 自然恢复测试

| # | 用例 | 输入 | 预期输出 | 说明 |
|---|------|------|---------|------|
| 8 | 刚恢复 0 分钟 | `elapsed=0s` | `current` 不变, `recoverTime=360` | 无恢复 |
| 9 | 恢复 6 分钟 | `elapsed=360s` | `current+1`, `recoverTime=360` | 刚好恢复 1 点 |
| 10 | 恢复 1 小时 | `elapsed=3600s` | `current+10`, `recoverTime=360` | 恢复 10 点 |
| 11 | 恢复到上限 | `current=115, max=120, elapsed=1800s` | `current=120`, `recoverTime=0` | 不超过上限 |
| 12 | 已满体力不恢复 | `current=120, max=120, elapsed=3600s` | `current=120`, `recoverTime=0` | 满时不恢复 |

### 8.3 消耗与增加测试

| # | 用例 | 输入 | 预期输出 | 说明 |
|---|------|------|---------|------|
| 13 | 正常消耗 | `stamina=70, cost=20` | `success=true, remaining=50` | 正常扣除 |
| 14 | 体力不足消耗 | `stamina=10, cost=20` | `success=false, remaining=10` | 不足不扣 |
| 15 | 体力刚好足够 | `stamina=20, cost=20` | `success=true, remaining=0` | 边界值 |
| 16 | 增加体力不超上限 | `current=118, add=5, max=120` | `current=120` | 不超上限 |

### 8.4 升级测试

| # | 用例 | 输入 | 预期输出 | 说明 |
|---|------|------|---------|------|
| 17 | 未达升级条件 | `level=2, captures=15` | `leveledUp=false` | 15 < 25 |
| 18 | 刚好升级一级 | `level=2, captures=25` | `leveledUp=true, newLevel=3, rewardGold=150` | 25 = Lv.3 条件 |
| 19 | 跨级升级 | `level=2, captures=50` | `leveledUp=true, newLevel=4, rewardGold=350` | 奖励累加 150+200 |
| 20 | 满级后不升级 | `level=10, captures=500` | `leveledUp=false` | 已满级 |

### 8.5 体力药剂购买测试

| # | 用例 | 输入 | 预期输出 | 说明 |
|---|------|------|---------|------|
| 21 | 正常购买 | `gold=300, purchased=0` | `success=true, remaining=2` | 扣 150 金币，恢复 3 体力 |
| 22 | 金币不足 | `gold=100, purchased=0` | `success=false, reason='insufficient_gold'` | 不足 150 |
| 23 | 达到每日限购 | `gold=999, purchased=3` | `success=false, reason='daily_limit_reached'` | 每日 3 次上限 |
| 24 | 跨日重置 | `purchased=3, date='昨天'` | `purchased=0` | 自然日重置 |

### 8.6 持久化与恢复测试

| # | 用例 | 操作 | 预期 | 说明 |
|---|------|------|------|------|
| 25 | 离线恢复 | 存档后等 1 小时重新加载 | 体力增加 10 | 基于 lastRecoverTime 计算 |
| 26 | 离线恢复到满 | 体力 115，离线 2 小时 | 体力 = 120（不超上限） | 上限保护 |
| 27 | 存档读写往返 | 保存后立即加载 | 状态完全一致 | 序列化/反序列化正确 |

---

## 9. 验收标准对照

| Issue #31 验收标准 | 实现方案 | 对应文件/函数 |
|-------------------|---------|-------------|
| **每小时恢复 10 点** | `calculateRecovery()` 基于时间戳计算：每 360 秒恢复 1 点，每小时恢复 10 点。`setInterval` 每 60 秒触发一次 TICK，页面可见性变化时额外触发。 | `logic.ts: calculateRecovery` + `StaminaContext.tsx: TICK_RECOVERY` |
| **单次捕获消耗 20** | `consumeStamina(20)` → Reducer `CONSUME` action 扣除 20，体力不足返回 false。常量 `CAPTURE_STAMINA_COST = 20`。 | `constants.ts: CAPTURE_STAMINA_COST` + `StaminaContext.tsx: consumeStamina` |
| **上限 120→240（满级）** | `LEVEL_TABLE` 定义 Lv.1=120 ~ Lv.10=240。`getMaxStamina(level)` 查表返回。消耗/恢复均以当前等级上限为天花板。 | `constants.ts: LEVEL_TABLE` + `logic.ts: getMaxStamina` |
| **升级恢复满体力** | `addCapture()` 内部调用 `tryLevelUp()`，若升级则 Reducer 将 `currentStamina` 设为新等级的 `maxStamina`。 | `logic.ts: tryLevelUp` + `StaminaContext.tsx: ADD_CAPTURE` |

### 补充实现（设计文档提及，Issue 未列但属于体力系统范畴）

| 规则 | 实现方案 | 状态 |
|------|---------|------|
| 派遣淘金币消耗 20 体力 | 常量 `DISPATCH_STAMINA_COST = 20`，复用 `consumeStamina()` | 常量定义，接入待后续 Issue |
| 体力为 0 无法捕获/派遣 | `consumeStamina()` 返回 false，调用方据此拦截 | 逻辑就绪，UI 拦截待后续 Issue |
| 体力药剂：150 金币恢复 3 点，每日限 3 次 | `buyStaminaPotion()` + `calculateBuyPotion()` + 每日重置 | 本次完整实现 |
| 升级金币奖励 | `tryLevelUp()` 返回 `rewardGold`，Reducer 中 `ADD_CAPTURE` 同时增加金币 | 本次完整实现 |

---

## 10. 实现步骤（建议顺序）

| 步骤 | 内容 | 产出文件 |
|------|------|---------|
| 1 | 创建 `constants.ts`，定义等级表和所有常量 | `stamina/constants.ts` |
| 2 | 创建 `types.ts`，定义所有接口和 Action 类型 | `stamina/types.ts` |
| 3 | 创建 `logic.ts`，实现所有纯函数 | `stamina/logic.ts` |
| 4 | 编写纯函数单元测试 | `stamina/logic.test.ts` |
| 5 | 创建 `StaminaContext.tsx`，实现 Reducer + Provider + 定时器 + 持久化 | `stamina/StaminaContext.tsx` |
| 6 | 创建 `useStamina.ts`，封装 Hook | `stamina/useStamina.ts` |
| 7 | 修改 `App.tsx`，包裹 Provider | `App.tsx` |
| 8 | 修改 `TopBar.tsx`，从 Context 读取数据 | `components/TopBar.tsx` |
| 9 | 手动测试：刷新页面、等待恢复、模拟消耗 | — |

> 步骤 1~4 可并行开发（纯函数无 React 依赖）。步骤 5~6 依赖 1~3。步骤 7~8 依赖 5~6。
