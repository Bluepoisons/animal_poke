# [M1] 金币 + 道具商店实现计划 — Issue #34

> **验收标准**：玩具球可购买使用
>
> **设计文档来源**：`游戏开发计划.md` 6.4 经济系统 + 5.5 等级与体力数值 + 3.3 捕获小游戏（道具增益）
>
> **技术栈**：React 18 + Vite 6 + TypeScript 5.6 + vitest，无额外依赖
>
> **现有基础**：`StaminaContext` 已管理 `gold` / `level` / `potionPurchasesToday`，TopBar 已显示金币，App.tsx Store tab 当前为 PlaceholderScreen

---

## 1. 文件结构

```
frontend/src/
├── shop/
│   ├── constants.ts        # 道具定义 ITEM_DEFS + 签到奖励表 + 商店相关常量
│   ├── types.ts            # ItemDef / Inventory / ShopState / ShopAction / CheckInState 等类型
│   ├── logic.ts            # 纯函数：getItemDef / canBuyItem / calculateBuy / calculateCheckIn 等（无副作用，可单测）
│   ├── ShopContext.tsx     # React Context + useReducer provider，管理道具背包 + 签到状态，与 StaminaContext 协调金币
│   ├── useShop.ts          # 自定义 Hook：封装 Context 消费，暴露给组件使用的 API
│   └── logic.test.ts       # 纯函数单元测试（≥15 个用例）
├── components/
│   ├── StoreScreen.tsx     # 商店主界面：道具列表 + 购买按钮 + 背包查看 + 签到入口
│   ├── CheckInPanel.tsx    # 每日签到面板：7 天递增奖励展示 + 签到按钮
│   ├── InventoryPanel.tsx  # 背包面板：已拥有道具列表 + 使用按钮
│   └── Toast.tsx           # 轻量 Toast 提示组件（购买成功/失败/签到成功）
├── App.tsx                 # （修改）包裹 <ShopProvider>，Store tab 改为 <StoreScreen/>
└── components/
    ├── TopBar.tsx          # （无修改，金币已从 StaminaContext 读取）
    └── CaptureScreen.tsx   # （修改）从 useShop 读取玩具球激活状态，显示 +15% 成功率增益
```

### 各文件职责

| 文件 | 职责 | 依赖 |
|------|------|------|
| `shop/constants.ts` | 道具定义列表 `ITEM_DEFS`、签到奖励表 `CHECK_IN_REWARDS`、商店存储 key 等常量。纯数据，无逻辑 | 无 |
| `shop/types.ts` | TypeScript 类型定义：`ItemDef` / `ItemId` / `Inventory` / `ShopState` / `ShopAction` / `BuyResult` / `CheckInResult`。纯类型，无运行时代码 | 无 |
| `shop/logic.ts` | 纯函数计算逻辑。不依赖 React，可直接被测试文件 import | constants, types |
| `shop/ShopContext.tsx` | 道具背包状态管理核心。创建 Context + Reducer，读写 localStorage，与 StaminaContext 协调（扣金币走 `useStamina().addGold(-price)`） | constants, types, logic, StaminaContext |
| `shop/useShop.ts` | 对外暴露的 Hook。封装 `useContext(ShopContext)`，处理 null 检查 | ShopContext, types |
| `components/StoreScreen.tsx` | 商店主界面，组合道具列表 + 背包 + 签到 | useShop, useStamina, CheckInPanel, InventoryPanel, Toast |
| `components/CheckInPanel.tsx` | 7 天签到展示与签到操作 | useShop |
| `components/InventoryPanel.tsx` | 背包列表与使用道具操作 | useShop |
| `components/Toast.tsx` | 轻量提示，2 秒自动消失 | 无 |

---

## 2. 道具定义 (`constants.ts`)

### 2.1 ItemDefs 常量

依据设计文档 6.4 道具商店表，MVP 阶段选取 8 个道具（含验收标准要求的玩具球）：

```typescript
/** 道具 ID 枚举（字符串字面量联合类型） */
export type ItemId =
  | 'toy_ball'          // 玩具球
  | 'premium_toy_ball'  // 高级玩具球
  | 'cold_medicine'     // 感冒药
  | 'bait'              // 诱饵
  | 'stamina_potion'    // 体力药剂
  | 'food_pack'         // 食物包
  | 'premium_snack'     // 高级零食
  | 'love_toy'          // 爱心玩具

/** 道具定义单行结构 */
export interface ItemDef {
  id: ItemId
  /** 道具名称（中文） */
  name: string
  /** 价格（金币） */
  price: number
  /** 图标 emoji */
  icon: string
  /** 道具描述 */
  description: string
  /** 道具效果说明（用于 UI 展示） */
  effect: string
  /** 道具类别 */
  category: 'capture' | 'recovery' | 'cure' | 'affinity'
  /** 是否单次使用即消耗（true = 使用后从背包消失） */
  consumable: boolean
  /** 每日限购次数（0 表示不限购） */
  dailyLimit: number
}

/** 道具定义表（全量，MVP 不做等级解锁） */
export const ITEM_DEFS: Record<ItemId, ItemDef> = {
  toy_ball: {
    id: 'toy_ball',
    name: '玩具球',
    price: 50,
    icon: '🎾',
    description: '下次捕获时使用，提升捕获成功率',
    effect: '捕获成功率 +15%',
    category: 'capture',
    consumable: true,
    dailyLimit: 0,
  },
  premium_toy_ball: {
    id: 'premium_toy_ball',
    name: '高级玩具球',
    price: 120,
    icon: '⚾',
    description: '下次捕获时使用，大幅提升捕获成功率',
    effect: '捕获成功率 +25%',
    category: 'capture',
    consumable: true,
    dailyLimit: 0,
  },
  cold_medicine: {
    id: 'cold_medicine',
    name: '感冒药',
    price: 200,
    icon: '💊',
    description: '立即解除宠物的感冒状态',
    effect: '解除感冒',
    category: 'cure',
    consumable: true,
    dailyLimit: 0,
  },
  bait: {
    id: 'bait',
    name: '诱饵',
    price: 100,
    icon: '🧀',
    description: '提升 30 分钟内稀有动物出现率',
    effect: '稀有出现率提升（30 分钟）',
    category: 'capture',
    consumable: true,
    dailyLimit: 0,
  },
  stamina_potion: {
    id: 'stamina_potion',
    name: '体力药剂',
    price: 150,
    icon: '🧪',
    description: '恢复 3 点体力',
    effect: '体力 +3',
    category: 'recovery',
    consumable: true,
    dailyLimit: 3,
  },
  food_pack: {
    id: 'food_pack',
    name: '食物包',
    price: 30,
    icon: '🥫',
    description: '补充 10 个投掷物（捕获小游戏消耗）',
    effect: '投掷物 +10',
    category: 'capture',
    consumable: true,
    dailyLimit: 0,
  },
  premium_snack: {
    id: 'premium_snack',
    name: '高级零食',
    price: 30,
    icon: '🍖',
    description: '喂食时亲密度 +10（替代基础 +5）',
    effect: '亲密度 +10',
    category: 'affinity',
    consumable: true,
    dailyLimit: 0,
  },
  love_toy: {
    id: 'love_toy',
    name: '爱心玩具',
    price: 50,
    icon: '❤️',
    description: '抚摸小游戏时亲密度 +15（替代基础 +8）',
    effect: '亲密度 +15',
    category: 'affinity',
    consumable: true,
    dailyLimit: 0,
  },
}

/** 道具 ID 列表（用于遍历） */
export const ITEM_IDS = Object.keys(ITEM_DEFS) as ItemId[]

/** 签到奖励表（7 天递增，第 7 天额外送随机道具） */
export const CHECK_IN_REWARDS: number[] = [20, 30, 40, 50, 60, 80, 150]

/** 签到满签（第 7 天）额外赠送的道具 */
export const CHECK_IN_DAY7_BONUS_ITEM: ItemId = 'toy_ball'

/** 签到周期天数 */
export const CHECK_IN_CYCLE_DAYS = 7

/** ShopContext localStorage 存储 key */
export const SHOP_STORAGE_KEY = 'animal_poke_shop'
```

### 2.2 道具总览表

| 道具 | ID | 价格 | 效果 | 类别 | 每日限购 |
|------|----|------|------|------|---------|
| 🎾 玩具球 | `toy_ball` | 50 | 捕获 +15% | capture | 不限 |
| ⚾ 高级玩具球 | `premium_toy_ball` | 120 | 捕获 +25% | capture | 不限 |
| 💊 感冒药 | `cold_medicine` | 200 | 解除感冒 | cure | 不限 |
| 🧀 诱饵 | `bait` | 100 | 稀有出现率提升 | capture | 不限 |
| 🧪 体力药剂 | `stamina_potion` | 150 | 体力 +3 | recovery | 3 次/日 |
| 🥫 食物包 | `food_pack` | 30 | 投掷物 +10 | capture | 不限 |
| 🍖 高级零食 | `premium_snack` | 30 | 亲密度 +10 | affinity | 不限 |
| ❤️ 爱心玩具 | `love_toy` | 50 | 亲密度 +15 | affinity | 不限 |

---

## 3. 经济状态管理

### 3.1 架构决策：新建 ShopContext，与 StaminaContext 协调

**为什么不扩展 StaminaContext？**
- StaminaContext 已有 8 个 Action 类型 + 7 个暴露方法，继续膨胀会违反单一职责
- 道具背包 + 签到逻辑与体力恢复/等级逻辑关注点不同
- 独立 Context 便于测试和后续替换

**协调方式：**
- 金币读取：`useStamina().state.gold` — 不重复管理
- 金币扣减：`useStamina().addGold(-price)` — 复用已有 Action
- 金币增加（签到奖励）：`useStamina().addGold(reward)` — 复用已有 Action
- 体力药剂购买后的体力恢复：调用 `useStamina().addStamina(3)` — 复用已有 Action
- 等级读取（未来用于商店刷新加成）：`useStamina().state.level`

### 3.2 类型定义 (`types.ts`)

```typescript
import type { ItemId } from './constants'

/** 道具背包：ItemId → 持有数量 */
export type Inventory = Partial<Record<ItemId, number>>

/** 每日限购记录：ItemId → 今日已购买次数 */
export type DailyPurchaseMap = Partial<Record<ItemId, number>>

/** 商店系统状态 */
export interface ShopState {
  /** 道具背包（ItemId → 数量） */
  inventory: Inventory
  /** 签到状态 */
  checkIn: CheckInState
  /** 每日限购记录 */
  dailyPurchases: DailyPurchaseMap
  /** 每日限购日期标记（'YYYY-MM-DD'） */
  dailyPurchaseDate: string
}

/** 签到状态 */
export interface CheckInState {
  /** 当前连续签到天数（0 = 未签到） */
  consecutiveDays: number
  /** 上次签到日期（'YYYY-MM-DD'） */
  lastCheckInDate: string
  /** 本周期内已签到天数（0~7，满 7 后重置） */
  cycleDay: number
}

/** 购买道具结果 */
export interface BuyResult {
  success: boolean
  /** 失败原因 */
  reason?: 'insufficient_gold' | 'daily_limit_reached'
  /** 购买后该道具剩余每日限购次数（不限购时为 null） */
  remainingDailyPurchases: number | null
}

/** 签到结果 */
export interface CheckInResult {
  success: boolean
  /** 签到天数（本次签到后连续天数） */
  day: number
  /** 获得金币 */
  rewardGold: number
  /** 获得道具（仅第 7 天） */
  rewardItem?: ItemId
  /** 失败原因 */
  reason?: 'already_checked_in' | 'cycle_completed'
}

/** 使用道具结果 */
export interface UseItemResult {
  success: boolean
  /** 失败原因 */
  reason?: 'not_in_inventory' | 'cannot_use'
}

/** Reducer Action 类型 */
export type ShopAction =
  | { type: 'BUY_ITEM'; itemId: ItemId }
  | { type: 'USE_ITEM'; itemId: ItemId }
  | { type: 'ADD_ITEM'; itemId: ItemId; count: number }
  | { type: 'CHECK_IN' }
  | { type: 'RESET_DAILY_PURCHASES'; date: string }
  | { type: 'LOAD_STATE'; state: ShopState }

/** ShopContext 暴露给组件的接口 */
export interface ShopContextValue {
  state: ShopState
  /** 购买道具（扣金币 → 加背包），返回购买结果 */
  buyItem: (itemId: ItemId) => BuyResult
  /** 使用道具（从背包消耗 1 个），返回使用结果 */
  useItem: (itemId: ItemId) => UseItemResult
  /** 每日签到，返回签到结果 */
  checkIn: () => CheckInResult
  /** 查询道具持有数量 */
  getItemCount: (itemId: ItemId) => number
  /** 查询某道具今日已购买次数 */
  getDailyPurchaseCount: (itemId: ItemId) => number
  /** 玩具球是否已激活（用于 CaptureScreen 显示增益） */
  isCaptureBoostActive: () => boolean
  /** 获取当前捕获增益百分比（0 / 15 / 25） */
  getCaptureBoostPercent: () => number
  /** 消耗捕获增益道具（进入捕获时调用） */
  consumeCaptureBoost: () => void
}
```

### 3.3 ShopContext 核心逻辑 (`ShopContext.tsx`)

```typescript
// 伪代码摘要，实际实现按此结构编写

export const ShopContext = createContext<ShopContextValue | null>(null)

export const ShopProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const stamina = useStamina()  // 获取金币/体力/等级
  const [state, dispatch] = useReducer(shopReducer, undefined, loadInitialState)

  // 持久化到 localStorage
  useEffect(() => {
    localStorage.setItem(SHOP_STORAGE_KEY, JSON.stringify(state))
  }, [state])

  // 每日限购重置检查
  useEffect(() => {
    if (shouldResetDailyPurchases(state.dailyPurchaseDate)) {
      dispatch({ type: 'RESET_DAILY_PURCHASES', date: getTodayString() })
    }
  }, [state.dailyPurchaseDate])

  const buyItem = useCallback((itemId: ItemId): BuyResult => {
    const def = ITEM_DEFS[itemId]
    const result = calculateBuy(
      stamina.state.gold,           // 当前金币（来自 StaminaContext）
      state.inventory[itemId] ?? 0,
      state.dailyPurchases[itemId] ?? 0,
      def
    )
    if (result.success) {
      // 扣金币（走 StaminaContext）
      stamina.addGold(-def.price)
      // 加道具到背包 + 记录限购
      dispatch({ type: 'BUY_ITEM', itemId })
      // 体力药剂特殊处理：立即恢复体力
      if (itemId === 'stamina_potion') {
        stamina.addStamina(POTION_RECOVERY)
      }
    }
    return result
  }, [stamina, state])

  // ... 其余方法类似

  return <ShopContext.Provider value={value}>{children}</ShopContext.Provider>
}
```

### 3.4 状态流转图

```
购买流程:
  用户点击购买 → buyItem(itemId)
    → calculateBuy(gold, inventory, dailyCount, def)  // 纯函数校验
    → 成功: stamina.addGold(-price) + dispatch(BUY_ITEM)
    → 失败: 返回 reason (insufficient_gold / daily_limit_reached)
    → Toast 提示

使用道具流程:
  用户点击使用 → useItem(itemId)
    → 检查 inventory[itemId] > 0
    → 成功: dispatch(USE_ITEM) → 数量 -1
    → 特殊处理: 玩具球 → 设置激活标记，CaptureScreen 读取

签到流程:
  用户点击签到 → checkIn()
    → calculateCheckIn(consecutiveDays, lastCheckInDate, cycleDay)  // 纯函数
    → 成功: stamina.addGold(reward) + dispatch(CHECK_IN)
    → 第 7 天额外: dispatch(ADD_ITEM, toy_ball)
    → Toast 提示
```

---

## 4. 商店 UI 设计

### 4.1 StoreScreen 组件结构

```
┌─────────────────────────────────┐
│         TopBar (已有)            │
├─────────────────────────────────┤
│  ┌───────────────────────────┐  │
│  │     CheckInPanel           │  │  ← 签到面板（7 天奖励展示 + 签到按钮）
│  │  [✔20][✔30][✔40][✔50]    │  │
│  │  [✔60][✔80][✔150+🎁]      │  │
│  │      [ 📅 签到 ]            │  │
│  └───────────────────────────┘  │
│                                 │
│  ┌─ Tab: 商店 | 背包 ─────────┐  │
│  │                             │  │
│  │  商店列表:                   │  │
│  │  ┌─────────────────────┐    │  │
│  │  │ 🎾 玩具球    50 🪙  │    │  │  ← 道具卡片（图标+名称+价格+购买按钮）
│  │  │ 捕获成功率 +15%      │    │  │
│  │  │        [购买]        │    │  │
│  │  └─────────────────────┘    │  │
│  │  ┌─────────────────────┐    │  │
│  │  │ ⚾ 高级玩具球 120 🪙 │    │  │
│  │  │ 捕获成功率 +25%      │    │  │
│  │  │        [购买]        │    │  │
│  │  └─────────────────────┘    │  │
│  │  ... (更多道具)              │  │
│  │                             │  │
│  │  背包列表:                   │  │
│  │  ┌─────────────────────┐    │  │
│  │  │ 🎾 玩具球 ×2        │    │  │  ← 背包卡片（图标+名称+数量+使用按钮）
│  │  │        [使用]        │    │  │
│  │  └─────────────────────┘    │  │
│  └─────────────────────────────┘  │
│                                 │
│  ┌─────────────────────────┐    │  │  ← Toast 浮层
│  │  ✅ 购买成功！-50 🪙     │    │  │
│  └─────────────────────────┘    │  │
├─────────────────────────────────┤
│         TabBar (已有)            │
└─────────────────────────────────┘
```

### 4.2 StoreScreen 组件设计

```typescript
/**
 * 商店主界面
 * - 上方: CheckInPanel 签到面板
 * - 中部: Tab 切换（商店列表 / 背包列表）
 * - 下方: Toast 浮层
 */
const StoreScreen: React.FC = () => {
  const [tab, setTab] = useState<'shop' | 'inventory'>('shop')
  const [toast, setToast] = useState<string | null>(null)

  const showToast = useCallback((msg: string) => {
    setToast(msg)
    setTimeout(() => setToast(null), 2000)
  }, [])

  return (
    <div style={styles.container}>
      <CheckInPanel onToast={showToast} />
      <div style={styles.tabBar}>
        <button onClick={() => setTab('shop')}>商店</button>
        <button onClick={() => setTab('inventory')}>背包</button>
      </div>
      {tab === 'shop' ? (
        <ShopList onToast={showToast} />
      ) : (
        <InventoryPanel onToast={showToast} />
      )}
      {toast && <Toast message={toast} />}
    </div>
  )
}
```

### 4.3 道具卡片样式

- 卡片为白色圆角卡片（`borderRadius: 16`），暖橙阴影
- 左侧大 emoji 图标（fontSize: 32）
- 右侧上方：道具名称 + 价格（🪙 图标）
- 右侧下方：效果说明（灰色小字）
- 最右侧：购买/使用按钮
- 金币不足时按钮 disabled + 灰色
- 限购道具显示 "今日剩余 N 次"

---

## 5. 签到系统

### 5.1 签到奖励表

依据设计文档 6.4 每日签到表：

| 连续签到天数 | 金币奖励 | 额外奖励 |
|------------|---------|---------|
| 第 1 天 | 20 | — |
| 第 2 天 | 30 | — |
| 第 3 天 | 40 | — |
| 第 4 天 | 50 | — |
| 第 5 天 | 60 | — |
| 第 6 天 | 80 | — |
| 第 7 天（满签） | 150 | 随机道具 ×1（MVP 固定送玩具球） |
| 断签 | 重置为第 1 天 | — |

### 5.2 签到逻辑 (`logic.ts`)

```typescript
/**
 * 计算签到结果（纯函数）
 * @param consecutiveDays 当前连续签到天数
 * @param lastCheckInDate 上次签到日期 'YYYY-MM-DD'
 * @param cycleDay 本周期已签到天数（0~7）
 * @param now 当前时间戳（可选，用于测试注入）
 * @returns CheckInResult
 */
export function calculateCheckIn(
  consecutiveDays: number,
  lastCheckInDate: string,
  cycleDay: number,
  now?: number
): CheckInResult {
  const today = getTodayString(now)

  // 今日已签到
  if (lastCheckInDate === today) {
    return { success: false, day: consecutiveDays, rewardGold: 0, reason: 'already_checked_in' }
  }

  // 判断是否断签：上次签到不是昨天则断签
  const yesterday = getYesterdayString(now)
  const isBroken = lastCheckInDate !== yesterday

  // 断签重置为第 1 天
  const newConsecutiveDays = isBroken ? 1 : consecutiveDays + 1

  // 周期处理：满 7 天后重置周期
  const newCycleDay = cycleDay >= CHECK_IN_CYCLE_DAYS ? 1 : cycleDay + 1

  // 获取奖励（基于周期天数）
  const rewardIndex = newCycleDay - 1
  const rewardGold = CHECK_IN_REWARDS[rewardIndex]
  const rewardItem = newCycleDay === CHECK_IN_CYCLE_DAYS ? CHECK_IN_DAY7_BONUS_ITEM : undefined

  return {
    success: true,
    day: newCycleDay,
    rewardGold,
    rewardItem,
  }
}
```

### 5.3 签到面板 UI

```
┌───────────────────────────────────┐
│  📅 每日签到                       │
│                                   │
│  [1✓][2✓][3✓][4 ][5 ][6 ][7 ]   │  ← 7 个格子，已签到的显示 ✓ + 金币数
│   20   30   40   50   60   80  150 │  ← 每格下方显示金币奖励
│                              +🎁   │  ← 第 7 格额外显示礼物图标
│                                   │
│         [ 📅 今日签到 ]             │  ← 签到按钮（已签到时 disabled）
│      连续签到 3 天                  │
└───────────────────────────────────┘
```

- 已签到的格子：绿色背景 + ✓ + 金币数
- 未签到的格子：灰色背景 + 金币数
- 第 7 天格子：金色边框 + 🎁 图标
- 签到按钮：今日已签到时显示 "✓ 今日已签到" 并 disabled

---

## 6. 购买流程

### 6.1 购买逻辑 (`logic.ts`)

```typescript
/**
 * 计算购买道具结果（纯函数，不修改状态）
 * @param gold 当前金币
 * @param currentCount 当前持有数量
 * @param dailyPurchased 今日已购买次数
 * @param def 道具定义
 * @returns BuyResult
 */
export function calculateBuy(
  gold: number,
  currentCount: number,
  dailyPurchased: number,
  def: ItemDef
): BuyResult {
  // 检查每日限购
  if (def.dailyLimit > 0 && dailyPurchased >= def.dailyLimit) {
    return {
      success: false,
      reason: 'daily_limit_reached',
      remainingDailyPurchases: 0,
    }
  }
  // 检查金币
  if (gold < def.price) {
    return {
      success: false,
      reason: 'insufficient_gold',
      remainingDailyPurchases: def.dailyLimit > 0 ? def.dailyLimit - dailyPurchased : null,
    }
  }
  return {
    success: true,
    remainingDailyPurchases: def.dailyLimit > 0 ? def.dailyLimit - dailyPurchased - 1 : null,
  }
}
```

### 6.2 购买流程时序

```
1. 用户点击 [购买] 按钮
2. StoreScreen 调用 useShop().buyItem(itemId)
3. ShopContext.buyItem:
   a. calculateBuy(gold, inventory, dailyCount, def) → BuyResult
   b. 若 success:
      - stamina.addGold(-def.price)     // 扣金币（StaminaContext）
      - dispatch({ type: 'BUY_ITEM', itemId })  // 加背包（ShopContext）
      - 若 itemId === 'stamina_potion':
        stamina.addStamina(3)            // 恢复体力（StaminaContext）
   c. 返回 BuyResult
4. StoreScreen 根据 result 显示 Toast:
   - success: "✅ 购买成功！-{price} 🪙"
   - insufficient_gold: "❌ 金币不足！"
   - daily_limit_reached: "❌ 今日限购已用完！"
```

### 6.3 Reducer BUY_ITEM 处理

```typescript
case 'BUY_ITEM': {
  const itemId = action.itemId
  const def = ITEM_DEFS[itemId]
  const currentCount = state.inventory[itemId] ?? 0
  const newInventory = { ...state.inventory, [itemId]: currentCount + 1 }

  // 限购道具记录购买次数
  let newDailyPurchases = state.dailyPurchases
  if (def.dailyLimit > 0) {
    const currentDaily = state.dailyPurchases[itemId] ?? 0
    newDailyPurchases = { ...state.dailyPurchases, [itemId]: currentDaily + 1 }
  }

  return {
    ...state,
    inventory: newInventory,
    dailyPurchases: newDailyPurchases,
  }
}
```

---

## 7. 使用道具 — 玩具球与 CaptureScreen 联动

### 7.1 玩具球激活机制

**设计决策**：MVP 阶段采用 "使用后激活，下次捕获消耗" 模式，简单直观。

```typescript
// ShopContext 内部状态
// activeCaptureBoost: ItemId | null  — null 表示无增益

// useItem('toy_ball') 时:
//   → dispatch USE_ITEM (背包 -1)
//   → 设置 activeCaptureBoost = 'toy_ball'

// CaptureScreen 进入时:
//   → 读取 useShop().getCaptureBoostPercent()
//   → 显示 "🎾 玩具球激活：捕获 +15%"

// 捕获完成时:
//   → useShop().consumeCaptureBoost()  // 清除激活状态
```

### 7.2 ShopContext 暴露的捕获增益 API

```typescript
/** 玩具球是否已激活（等待下次捕获消耗） */
isCaptureBoostActive: () => boolean

/** 获取当前捕获增益百分比（0 / 15 / 25） */
getCaptureBoostPercent: () => number

/** 消耗捕获增益道具（捕获开始/完成时调用） */
consumeCaptureBoost: () => void
```

### 7.3 CaptureScreen 修改

```typescript
const CaptureScreen: React.FC = () => {
  const shop = useShop()
  const boostPercent = shop.getCaptureBoostPercent()
  const baseRate = 70  // 基础成功率 70%
  const boostedRate = baseRate + boostPercent

  return (
    <div style={styles.container}>
      {/* 顶部信息 */}
      <div style={styles.topInfo}>
        {boostPercent > 0 && (
          <span className="pill" style={{ background: 'var(--orange)' }}>
            🎾 玩具球 +{boostPercent}%
          </span>
        )}
        <span className="pill">基础 {baseRate}%</span>
        {boostPercent > 0 && (
          <span className="pill" style={{ background: 'var(--success)' }}>
            总 {boostedRate}%
          </span>
        )}
      </div>
      {/* ... 其余 UI 不变 ... */}
    </div>
  )
}
```

### 7.4 使用道具流程

```
1. 用户在背包面板点击 [使用] 按钮
2. InventoryPanel 调用 useShop().useItem(itemId)
3. ShopContext.useItem:
   a. 检查 inventory[itemId] > 0
   b. dispatch({ type: 'USE_ITEM', itemId })  // 背包 -1
   c. 若 itemId === 'toy_ball' 或 'premium_toy_ball':
      → 设置 activeCaptureBoost = itemId
   d. 若 itemId === 'stamina_potion':
      → stamina.addStamina(3)
   e. 若 itemId === 'cold_medicine':
      → MVP 暂无感冒系统，Toast 提示 "感冒药已使用"（预留）
4. Toast 提示 "✅ {itemName} 已使用"
```

### 7.5 USE_ITEM Reducer 处理

```typescript
case 'USE_ITEM': {
  const itemId = action.itemId
  const currentCount = state.inventory[itemId] ?? 0
  if (currentCount <= 0) return state

  const newCount = currentCount - 1
  const newInventory = { ...state.inventory }
  if (newCount <= 0) {
    delete newInventory[itemId]
  } else {
    newInventory[itemId] = newCount
  }

  return {
    ...state,
    inventory: newInventory,
    // activeCaptureBoost 在 ShopContext 外部管理（useRef 或额外 state）
  }
}
```

---

## 8. App.tsx 集成

### 8.1 Provider 嵌套

```typescript
// App.tsx 修改
import { ShopProvider } from './shop/ShopContext'
import StoreScreen from './components/StoreScreen'

// renderContent 中 store tab:
case 'store':
  return <StoreScreen />

// Provider 嵌套（ShopProvider 在 StaminaProvider 内部，因为它依赖 useStamina）
<StaminaProvider>
  <ShopProvider>
    <div className="phone-frame">
      {/* ... */}
    </div>
  </ShopProvider>
</StaminaProvider>
```

### 8.2 Provider 依赖关系

```
StaminaProvider (金币/体力/等级)
  └── ShopProvider (道具背包/签到，消费金币)
        └── App 内容 (StoreScreen / CaptureScreen / ...)
```

---

## 9. 测试用例

### 9.1 测试文件：`shop/logic.test.ts`

共 18 个测试用例，覆盖所有纯函数：

```typescript
import { describe, it, expect } from 'vitest'
import {
  calculateBuy,
  calculateCheckIn,
  getItemDef,
  getItemCount,
  canUseItem,
  shouldResetDailyPurchases,
  getYesterdayString,
  calculateCaptureBoost,
} from './logic'
import { ITEM_DEFS, CHECK_IN_REWARDS, CHECK_IN_CYCLE_DAYS } from './constants'
import type { ItemId } from './constants'

// 固定时间戳：2025-01-15 12:00:00 UTC（周三）
const BASE_TIME = 1736932800000

describe('getItemDef', () => {
  it('#1 获取玩具球定义：id/toy_ball, price/50, consumable/true', () => {
    const def = getItemDef('toy_ball')
    expect(def.id).toBe('toy_ball')
    expect(def.price).toBe(50)
    expect(def.consumable).toBe(true)
  })

  it('#2 获取体力药剂定义：dailyLimit/3', () => {
    const def = getItemDef('stamina_potion')
    expect(def.dailyLimit).toBe(3)
  })
})

describe('calculateBuy — 正常购买', () => {
  it('#3 玩具球 gold=100, dailyPurchased=0 => success, remaining=null', () => {
    const result = calculateBuy(100, 0, 0, ITEM_DEFS.toy_ball)
    expect(result.success).toBe(true)
    expect(result.remainingDailyPurchases).toBeNull()
  })

  it('#4 体力药剂 gold=200, dailyPurchased=0 => success, remaining=2', () => {
    const result = calculateBuy(200, 0, 0, ITEM_DEFS.stamina_potion)
    expect(result.success).toBe(true)
    expect(result.remainingDailyPurchases).toBe(2)
  })
})

describe('calculateBuy — 金币不足', () => {
  it('#5 gold=30 < price=50 => insufficient_gold', () => {
    const result = calculateBuy(30, 0, 0, ITEM_DEFS.toy_ball)
    expect(result.success).toBe(false)
    expect(result.reason).toBe('insufficient_gold')
  })

  it('#6 gold=0 购买食物包 => insufficient_gold', () => {
    const result = calculateBuy(0, 0, 0, ITEM_DEFS.food_pack)
    expect(result.success).toBe(false)
    expect(result.reason).toBe('insufficient_gold')
  })
})

describe('calculateBuy — 每日限购', () => {
  it('#7 体力药剂 dailyPurchased=3 => daily_limit_reached', () => {
    const result = calculateBuy(999, 0, 3, ITEM_DEFS.stamina_potion)
    expect(result.success).toBe(false)
    expect(result.reason).toBe('daily_limit_reached')
    expect(result.remainingDailyPurchases).toBe(0)
  })

  it('#8 体力药剂 dailyPurchased=2 (最后一次) => success, remaining=0', () => {
    const result = calculateBuy(200, 0, 2, ITEM_DEFS.stamina_potion)
    expect(result.success).toBe(true)
    expect(result.remainingDailyPurchases).toBe(0)
  })

  it('#9 玩具球 dailyLimit=0 不限购，dailyPurchased=99 仍可购买', () => {
    const result = calculateBuy(100, 0, 99, ITEM_DEFS.toy_ball)
    expect(result.success).toBe(true)
  })
})

describe('calculateBuy — 边界值', () => {
  it('#10 gold 刚好等于 price => success', () => {
    const result = calculateBuy(50, 0, 0, ITEM_DEFS.toy_ball)
    expect(result.success).toBe(true)
  })

  it('#11 gold=price-1 => insufficient_gold', () => {
    const result = calculateBuy(49, 0, 0, ITEM_DEFS.toy_ball)
    expect(result.success).toBe(false)
    expect(result.reason).toBe('insufficient_gold')
  })
})

describe('calculateCheckIn — 正常签到', () => {
  it('#12 首次签到 consecutiveDays=0, cycleDay=0 => day=1, reward=20', () => {
    const result = calculateCheckIn(0, '', 0, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.day).toBe(1)
    expect(result.rewardGold).toBe(20)
    expect(result.rewardItem).toBeUndefined()
  })

  it('#13 连续第 3 天签到 => day=3, reward=40', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = calculateCheckIn(2, yesterday, 2, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.day).toBe(3)
    expect(result.rewardGold).toBe(40)
  })

  it('#14 第 7 天满签 => day=7, reward=150, rewardItem=toy_ball', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = calculateCheckIn(6, yesterday, 6, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.day).toBe(7)
    expect(result.rewardGold).toBe(150)
    expect(result.rewardItem).toBe('toy_ball')
  })
})

describe('calculateCheckIn — 断签与重复', () => {
  it('#15 今日已签到 => already_checked_in', () => {
    const today = getTodayString(BASE_TIME)
    const result = calculateCheckIn(3, today, 3, BASE_TIME)
    expect(result.success).toBe(false)
    expect(result.reason).toBe('already_checked_in')
  })

  it('#16 断签后重新签到 => day=1, reward=20', () => {
    // 上次签到是 3 天前，不是昨天
    const result = calculateCheckIn(5, '2025-01-12', 5, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.day).toBe(1)
    expect(result.rewardGold).toBe(20)
  })
})

describe('calculateCheckIn — 周期重置', () => {
  it('#17 满 7 天后第 8 天签到 => 周期重置为 day=1, reward=20', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = calculateCheckIn(7, yesterday, 7, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.day).toBe(1)
    expect(result.rewardGold).toBe(20)
  })
})

describe('calculateCaptureBoost', () => {
  it('#18 无激活道具 => boost=0%', () => {
    expect(calculateCaptureBoost(null)).toBe(0)
  })

  it('#19 激活玩具球 => boost=15%', () => {
    expect(calculateCaptureBoost('toy_ball')).toBe(15)
  })

  it('#20 激活高级玩具球 => boost=25%', () => {
    expect(calculateCaptureBoost('premium_toy_ball')).toBe(25)
  })
})

describe('shouldResetDailyPurchases', () => {
  it('#21 跨日重置：日期不是今天 => true', () => {
    const today = getTodayString(BASE_TIME)
    expect(shouldResetDailyPurchases('2025-01-14', BASE_TIME)).toBe(true)
    expect(shouldResetDailyPurchases(today, BASE_TIME)).toBe(false)
  })
})
```

### 9.2 测试覆盖总览

| 类别 | 用例数 | 覆盖函数 |
|------|--------|---------|
| getItemDef | 2 | 道具定义查询 |
| calculateBuy 正常 | 2 | 购买成功路径 |
| calculateBuy 金币不足 | 2 | 金币校验 |
| calculateBuy 限购 | 3 | 每日限购校验 |
| calculateBuy 边界值 | 2 | 金币边界 |
| calculateCheckIn 正常 | 3 | 签到递增 + 满签 |
| calculateCheckIn 断签/重复 | 2 | 断签重置 + 重复签到 |
| calculateCheckIn 周期重置 | 1 | 7 天周期满后重置 |
| calculateCaptureBoost | 3 | 捕获增益计算 |
| shouldResetDailyPurchases | 1 | 日期重置 |
| **合计** | **21** | |

---

## 10. 验收标准对照

| 验收标准 | 实现方案 | 验证方式 |
|---------|---------|---------|
| 玩具球可购买 | `buyItem('toy_ball')` → 扣 50 金币 → 背包 +1 | 手动测试 + 单测 #3 |
| 玩具球可使用 | `useItem('toy_ball')` → 背包 -1 → 激活捕获增益 | 手动测试 + 单测 #19 |
| 使用后 +15% 捕获率 | CaptureScreen 读取 `getCaptureBoostPercent()` → 显示 "总 85%" | 手动测试 |
| 金币正确扣减 | 购买走 `stamina.addGold(-price)`，TopBar 金币实时更新 | 手动测试 |
| 背包持久化 | ShopState 序列化到 localStorage `animal_poke_shop` | 刷新页面验证 |
| 签到 7 天递增 | `CHECK_IN_REWARDS` 表 + `calculateCheckIn` 纯函数 | 单测 #12~#17 |
| 断签重置 | `calculateCheckIn` 判断非昨日则重置为第 1 天 | 单测 #16 |
| 体力药剂每日限购 3 次 | `dailyLimit=3` + `calculateBuy` 校验 + 每日重置 | 单测 #7, #8 |

---

## 11. 实现顺序建议

| 步骤 | 内容 | 预估文件 |
|------|------|---------|
| 1 | 创建 `shop/constants.ts` + `shop/types.ts` | 2 文件 |
| 2 | 实现 `shop/logic.ts` 纯函数 | 1 文件 |
| 3 | 编写 `shop/logic.test.ts` 并全部通过 | 1 文件 |
| 4 | 实现 `shop/ShopContext.tsx` + `shop/useShop.ts` | 2 文件 |
| 5 | 实现 `components/Toast.tsx` | 1 文件 |
| 6 | 实现 `components/CheckInPanel.tsx` | 1 文件 |
| 7 | 实现 `components/InventoryPanel.tsx` | 1 文件 |
| 8 | 实现 `components/StoreScreen.tsx` | 1 文件 |
| 9 | 修改 `App.tsx`：接入 ShopProvider + StoreScreen | 1 文件 |
| 10 | 修改 `components/CaptureScreen.tsx`：接入捕获增益显示 | 1 文件 |
| 11 | 全量测试 `vitest run` + 手动验收 | — |

---

## 12. 注意事项

1. **不要重复管理金币**：金币状态始终从 `StaminaContext.state.gold` 读取，ShopContext 不持有金币副本
2. **购买时金币扣减的时序**：先 `calculateBuy` 校验 → 再 `addGold(-price)` → 再 `dispatch(BUY_ITEM)`。若 addGold 和 dispatch 之间有渲染间隙不影响正确性（最坏情况是金币扣了但道具没加上，但 dispatch 是同步的不会有此问题）
3. **体力药剂的双重效果**：购买后立即恢复体力（`addStamina(3)`），这是设计文档中体力药剂的既有行为（StaminaContext 已有 `BUY_POTION`，但商店购买走 ShopContext 统一流程）
4. **玩具球激活状态管理**：使用 `useRef` 在 ShopContext 内部管理 `activeCaptureBoost`，不放入 reducer state（避免持久化激活状态，刷新后激活失效是合理的）
5. **签到日期判断**：使用自然日（`YYYY-MM-DD`），与 StaminaContext 的 `getTodayString` 复用同一逻辑
6. **MVP 不实现的部分**：商店每日刷新、稀有道具池、等级影响刷新率、派遣淘金币 — 这些留给后续 Issue
7. **代码注释用中文**：与现有代码风格一致
