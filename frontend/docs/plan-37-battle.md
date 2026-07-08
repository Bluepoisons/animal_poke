# [M2] 战斗系统实现计划 — Issue #37

> **验收标准**：自动回合制可玩，PvE 完整
>
> **设计文档来源**：`游戏开发计划.md` 6.5 战斗系统 + 5.2 六维属性 + 3.5 天气系统 + 5.4 状态系统 + 10.2 战斗系统深化
>
> **技术栈**：React 18 + Vite 6 + TypeScript 5.6 + vitest，无额外依赖
>
> **现有基础**：`StaminaContext`（体力/等级/金币）、`ShopContext`（道具背包/签到）、`useAnimalStore`（IndexedDB 动物存储）、`SPECIES_DEFS`（物种定义）、`CardEntry`（含 id/rarity/species/seed 等）、App.tsx 有 fight tab

---

## 1. 战斗系统概述

### 1.1 核心定位

MVP 阶段实现 **1v1 自动回合制 PvE 战斗**，遵循 KISS 原则：

- **自动回合**：双方按 SPD 顺序自动普攻，玩家无需逐回合操作
- **半自动深度**：玩家可在回合间使用道具（回血/增益）、切换策略（激进/平衡/防守）、攒满能量后手动释放必杀技
- **PvE 完整**：选宠 → 匹配对手 → 自动战斗 → 结果结算 → 奖励发放，全流程闭环
- **暂不实现**：3v3 阵容、站位系统、技能树、PvP 排位、联盟战（后续迭代）

### 1.2 设计依据

| 设计文档章节 | 关键内容 | MVP 落地范围 |
|------------|---------|-------------|
| 6.5 战斗系统 | 自动回合制，按 SPD 决定行动顺序，元素克制，天气联动 | 全部落地（1v1 简化版） |
| 5.2 六维属性 | HP/ATK/DEF/SPD/Class/Element，物种差异化 | HP/ATK/DEF/SPD + CRIT/EVA（Class/Element 作为属性标签） |
| 10.2 战斗深化 | 半自动：能量条必杀、站位、技能系统 | 仅能量条必杀 + 策略切换（站位/技能后续迭代） |
| 3.5 天气系统 | 晴天全属性 +5%，雨天水 +10% 伤害等 | 落地（天气数据从 props 传入，默认晴天） |
| 5.4 状态系统 | 感冒全属性 -35% | 预留接口，MVP 暂不触发感冒（天气系统未完整接入） |

### 1.3 战斗流程图

```
┌─────────────────────────────────────────────────────────┐
│                    战斗系统流程                           │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  [fight tab]                                            │
│      │                                                  │
│      ▼                                                  │
│  ┌──────────┐    无体力     ┌────────────────┐         │
│  │ 宠物选择  │─────────────▶│ 提示体力不足    │         │
│  │ PetSelect │              └────────────────┘         │
│  └────┬─────┘                                          │
│       │ 选择宠物 + 确认（消耗 20 体力）                   │
│       ▼                                                  │
│  ┌──────────┐                                           │
│  │ 对手匹配  │ ← 基于玩家等级生成 PvE 对手               │
│  │ Matching  │                                           │
│  └────┬─────┘                                           │
│       ▼                                                  │
│  ┌──────────────────────────────────────┐               │
│  │          自动回合战斗                  │               │
│  │  ┌─────────────────────────────────┐  │               │
│  │  │ 每回合：                         │  │               │
│  │  │  1. 按 SPD 决定先后手            │  │               │
│  │  │  2. 先手普攻（计算伤害/暴击/闪避）│  │               │
│  │  │  3. 后手普攻                     │  │               │
│  │  │  4. 双方积累能量                 │  │               │
│  │  │  5. 检查死亡/回合上限            │  │               │
│  │  └─────────────────────────────────┘  │               │
│  │  玩家可（回合间）：                    │               │
│  │    • 使用道具（回血/增益）             │               │
│  │    • 切换策略（激进/平衡/防守）        │               │
│  │    • 必杀技（能量满时手动释放）        │               │
│  └──────────────────────────────────────┘               │
│       │                                                  │
│       ▼                                                  │
│  ┌──────────┐                                           │
│  │ 结果结算  │ → 胜利：金币 + 道具掉落                    │
│  │ Result   │ → 失败：少量安慰金币                       │
│  └──────────┘                                           │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 1.4 与现有 fight tab 的共存方案

当前 `fight` tab 用于 CaptureScreen（DiscoverScreen 拍照后切到 fight tab 进行捕获）。改造方案：

```
fight tab 渲染逻辑：
  if (pendingPhoto)  →  <CaptureScreen />   // 捕获流程，保持不变
  else               →  <BattleScreen />    // 战斗流程，新增
```

- 玩家从 DiscoverScreen 拍照 → `setPendingPhoto` + 切到 fight tab → 显示 CaptureScreen
- 捕获完成 → 清除 pendingPhoto → 切到 collection tab
- 玩家直接点 fight tab（无 pendingPhoto）→ 显示 BattleScreen

---

## 2. 六维属性设计

### 2.1 属性定义

| 属性 | 缩写 | 说明 | 范围 | 计算来源 |
|------|------|------|------|---------|
| 生命 | HP | 承伤上限 | 50~500 | 稀有度基础 + 物种修正 + 个体随机 |
| 攻击 | ATK | 每回合伤害 | 10~120 | 稀有度基础 + 物种修正 + 个体随机 |
| 防御 | DEF | 减伤系数 | 5~80 | 稀有度基础 + 物种修正 + 个体随机 |
| 速度 | SPD | 行动顺序判定 | 1~100 | 稀有度基础 + 物种修正 + 个体随机 |
| 暴击率 | CRIT | 暴击概率 (%) | 5~30 | 物种基础 + 稀有度加成 |
| 闪避率 | EVA | 闪避概率 (%) | 5~25 | 物种基础 + 稀有度加成 |

### 2.2 稀有度基础属性表

| 稀有度 | HP | ATK | DEF | SPD | CRIT | EVA |
|--------|-----|-----|-----|-----|------|-----|
| Common | 60 | 15 | 10 | 15 | 5% | 5% |
| Uncommon | 100 | 28 | 20 | 30 | 7% | 7% |
| Rare | 150 | 45 | 35 | 50 | 10% | 10% |
| Epic | 220 | 65 | 50 | 70 | 15% | 15% |
| Legendary | 350 | 95 | 68 | 88 | 20% | 20% |

### 2.3 物种属性修正

依据设计文档 5.2「宠物属性差异化」：

| 物种 | HP 修正 | ATK 修正 | DEF 修正 | SPD 修正 | CRIT 加成 | EVA 加成 | 定位 |
|------|--------|---------|---------|---------|----------|---------|------|
| 猫 cat | ×0.8 | ×0.9 | ×0.9 | ×1.3 | +10% | +5% | 输出 DPS |
| 狗 dog | ×1.3 | ×1.2 | ×1.0 | ×0.8 | +3% | +2% | 坦克 Tank |
| 鹅 goose | ×1.0 | ×0.8 | ×1.4 | ×0.9 | +2% | +8% | 控制 Control |

### 2.4 个体随机浮动

使用 `CardEntry.seed` 生成确定性的 ±15% 随机浮动（同一只宠物每次计算结果一致）：

```typescript
// 伪随机函数：基于 seed 生成 0.85~1.15 的浮动系数
function seedVariance(seed: number, salt: number): number {
  const x = Math.sin(seed * 9999 + salt * 7777) * 10000
  const frac = x - Math.floor(x)  // 0~1
  return 0.85 + frac * 0.3  // 0.85~1.15
}
```

### 2.5 元素系统

| 元素 | 克制关系 | 说明 |
|------|---------|------|
| 火 fire | 克草，被水克 | fire > grass > water > fire（循环） |
| 水 water | 克火，被草克 | |
| 草 grass | 克水，被火克 | |
| 光 light | 与暗互克 | 1.5 倍伤害 |
| 暗 dark | 与光互克 | 1.5 倍伤害 |

**元素分配**：基于 `seed % 5` 从 `[fire, water, grass, light, dark]` 中确定性地选取，保证同种宠物可有不同元素。

**克制倍率**：
- 有利克制：1.5×
- 不利被克：0.67×
- 同元素：1.0×
- 无克制关系：1.0×

### 2.6 属性计算公式

```
最终属性 = 稀有度基础 × 物种修正 × 个体浮动(seed)

示例：传说猫的 HP
  = 350（legendary 基础）× 0.8（猫修正）× seedVariance(seed, 0)
  = 280 × (0.85~1.15)
  = 238~322
```

### 2.7 天气修正（设计文档 3.5）

| 天气 | 属性修正 | 元素修正 |
|------|---------|---------|
| 晴 sunny | 全属性 +5% | 火元素 +10% 伤害，水元素 -10% |
| 多云 cloudy | 无 | 无 |
| 阴 overcast | 无 | 无 |
| 雨 rainy | 无 | 水元素 +10% 伤害，火元素 -10% |
| 雪 snowy | 无 | 无 |
| 雾 foggy | 无 | 无 |
| 极端 extreme | 暂停户外玩法 | — |

> MVP 阶段天气数据通过 props 传入，默认 `sunny`。天气系统完整接入后从 LbsContext/后端获取。

---

## 3. 战斗流程详细设计

### 3.1 阶段状态机

```
idle ──select──▶ selecting ──confirm──▶ matching ──matched──▶ battling ──end──▶ result ──reset──▶ idle
  ▲                                                                  │
  └──────────────────────────────────────────────────────────────────┘
```

| 阶段 | 说明 | UI 展示 |
|------|------|--------|
| idle | 未进入战斗 | BattleScreen 入口（"开始战斗"按钮） |
| selecting | 选择出战宠物 | 宠物列表（从 useAnimalStore 获取已解锁宠物） |
| matching | 匹配 PvE 对手 | "匹配中..." 动画（1~2 秒模拟） |
| battling | 战斗进行中 | 血条/回合数/日志/操作按钮 |
| result | 战斗结束 | 胜利/失败动画 + 奖励展示 |

### 3.2 回合执行逻辑

每回合按以下顺序执行：

```
回合开始
  │
  ├── 1. 判定先后手：SPD 高者先手（相同则随机）
  │
  ├── 2. 先手行动：
  │     ├── a. 检查闪避（防守方 EVA 概率闪避 → 跳过伤害）
  │     ├── b. 计算基础伤害 = max(1, ATK - DEF)
  │     ├── c. 元素克制修正（×1.5 / ×0.67 / ×1.0）
  │     ├── d. 暴击判定（CRIT 概率 → ×2 伤害）
  │     ├── e. 天气元素修正
  │     ├── f. 策略修正（激进 +10%ATK/-10%DEF 等）
  │     ├── g. 扣减 HP
  │     └── h. 积累能量（攻击方 +15，防守方 +10）
  │
  ├── 3. 检查防守方是否死亡（HP ≤ 0 → 战斗结束）
  │
  ├── 4. 后手行动（同步骤 2）
  │
  ├── 5. 检查先手方是否死亡
  │
  ├── 6. 回合数 +1，检查回合上限（30 回合 → 平局）
  │
  └── 7. 回合结束，等待玩家操作或下一回合
```

### 3.3 半自动机制

**能量条必杀**：
- 每次普攻/受击积累能量（攻击 +15，受击 +10）
- 能量上限 100，满后可手动点击「必杀」按钮释放
- 必杀效果：1.8× 普攻伤害 + 必定暴击 + 跳过闪避判定
- 必杀释放后能量清零

**策略切换**（回合间可切换，不消耗回合）：

| 策略 | ATK 修正 | DEF 修正 | 说明 |
|------|---------|---------|------|
| 激进 aggressive | +10% | -10% | 拼输出，适合脆皮高攻 |
| 平衡 balanced | 0 | 0 | 默认 |
| 防守 defensive | -10% | +10% | 拖持久战，适合高血量 |

**道具使用**（回合间可使用，不消耗回合）：
- 体力药剂 → 不适用（战斗中不用体力药剂）
- 感冒药 → 不适用（MVP 暂无感冒）
- 玩具球 → 不适用（捕获专用）
- **新增战斗道具**（MVP 暂用 ShopContext 现有道具模拟，或后续扩展）：
  - 战斗回血药（恢复 30% HP）— MVP 可复用 `food_pack` 模拟
  - 战斗增益药（临时 +20% ATK，3 回合）— MVP 可复用 `toy_ball` 模拟

> KISS 原则：MVP 阶段战斗道具使用 ShopContext 的 `useItem`，通过 `ItemId` 判断是否可用于战斗。不做专门的战斗道具商店。

### 3.4 伤害公式

```
基础伤害 = max(1, attackerATK - defenderDEF)

元素倍率 = getElementMultiplier(attackerElement, defenderElement)  // 1.5 / 0.67 / 1.0

暴击倍率 = isCrit ? 2.0 : 1.0

天气倍率 = getWeatherElementBonus(weather, attackerElement)  // ±10% 或 0

策略倍率 = strategy === 'aggressive' ? 1.1 : strategy === 'defensive' ? 0.9 : 1.0

最终伤害 = round(基础伤害 × 元素倍率 × 暴击倍率 × (1 + 天气倍率) × 策略倍率)
```

---

## 4. BattleContext 设计

### 4.1 状态结构

```typescript
/** 战斗阶段 */
type BattlePhase = 'idle' | 'selecting' | 'matching' | 'battling' | 'result'

/** 元素类型 */
type ElementType = 'fire' | 'water' | 'grass' | 'light' | 'dark'

/** 天气类型 */
type WeatherType = 'sunny' | 'cloudy' | 'overcast' | 'rainy' | 'snowy' | 'foggy' | 'extreme'

/** 策略类型 */
type StrategyType = 'aggressive' | 'balanced' | 'defensive'

/** 战斗属性 */
interface BattleStats {
  hp: number
  atk: number
  def: number
  spd: number
  crit: number    // 暴击率 0~100
  eva: number     // 闪避率 0~100
}

/** 战斗宠物（运行时状态） */
interface BattlePet {
  id: string
  name: string
  emoji: string
  species: SpeciesType
  rarity: RarityTier
  element: ElementType
  stats: BattleStats         // 最终计算后的属性（含天气修正）
  baseStats: BattleStats     // 原始属性（不含天气/策略修正，用于重新计算）
  currentHp: number          // 当前 HP
  energy: number              // 能量 0~100
  isPlayer: boolean           // true = 玩家方，false = 敌方
}

/** 日志条目 */
interface BattleLogEntry {
  round: number
  text: string
  type: 'attack' | 'crit' | 'miss' | 'ultimate' | 'item' | 'system'
}

/** 战斗结果 */
type BattleResult = 'win' | 'lose' | 'draw' | null

/** 战斗奖励 */
interface BattleRewards {
  gold: number
  exp: number
  droppedItem?: ItemId
}

/** BattleContext 完整状态 */
interface BattleState {
  phase: BattlePhase
  playerPet: BattlePet | null
  enemyPet: BattlePet | null
  round: number
  maxRounds: number
  log: BattleLogEntry[]
  result: BattleResult
  rewards: BattleRewards | null
  strategy: StrategyType
  weather: WeatherType
  isAutoPlaying: boolean        // 是否正在自动播放回合（计时器运行中）
}
```

### 4.2 Reducer Actions

```typescript
type BattleAction =
  | { type: 'ENTER_SELECT' }                          // 进入选宠
  | { type: 'SELECT_PET'; pet: BattlePet }             // 选择宠物
  | { type: 'START_MATCHING' }                         // 开始匹配
  | { type: 'MATCH_COMPLETE'; enemy: BattlePet }       // 匹配完成
  | { type: 'BATTLE_START' }                           // 战斗开始
  | { type: 'EXECUTE_ROUND'; log: BattleLogEntry[] }   // 执行一回合
  | { type: 'USE_ULTIMATE'; log: BattleLogEntry[] }    // 释放必杀
  | { type: 'SET_STRATEGY'; strategy: StrategyType }   // 切换策略
  | { type: 'USE_ITEM'; log: BattleLogEntry[] }        // 使用道具
  | { type: 'BATTLE_END'; result: BattleResult; rewards: BattleRewards }  // 战斗结束
  | { type: 'RESET' }                                  // 重置回 idle
  | { type: 'SET_WEATHER'; weather: WeatherType }      // 设置天气
  | { type: 'SET_AUTO_PLAY'; playing: boolean }        // 设置自动播放
```

### 4.3 Context 暴露接口

```typescript
interface BattleContextValue {
  state: BattleState

  // 流程控制
  enterSelect: () => void                           // 进入选宠阶段
  selectPet: (entry: CardEntry) => void             // 选择出战宠物
  startMatching: () => void                         // 开始匹配
  startBattle: () => void                           // 战斗开始

  // 回合操作
  executeNextRound: () => void                      // 执行下一回合
  useUltimate: () => boolean                        // 释放必杀（能量满才可）
  setStrategy: (strategy: StrategyType) => void     // 切换策略
  useBattleItem: (itemId: ItemId) => boolean        // 使用战斗道具
  toggleAutoPlay: () => void                        // 切换自动/手动

  // 结算
  finishBattle: () => void                          // 确认结算，回到 idle
  reset: () => void                                 // 重置
}
```

### 4.4 自动回合计时器

BattleProvider 内部使用 `useEffect` + `setInterval` 实现自动回合推进：

```typescript
// phase === 'battling' && isAutoPlaying 时，每 1.5 秒自动执行一回合
useEffect(() => {
  if (state.phase !== 'battling' || !state.isAutoPlaying) return
  const timer = setInterval(() => {
    dispatch({ type: 'EXECUTE_ROUND', log: computeRoundLog(state) })
    // 检查是否结束
  }, 1500)
  return () => clearInterval(timer)
}, [state.phase, state.isAutoPlaying])
```

- 默认 `isAutoPlaying = true`，自动每 1.5 秒推进一回合
- 玩家可点击「暂停」手动逐回合操作（使用道具/必杀/切换策略后可恢复）
- 能量满时自动播放暂停，等待玩家选择是否释放必杀（或点「跳过」继续自动）

### 4.5 Provider 嵌套位置

```typescript
// App.tsx
<StaminaProvider>
  <ShopProvider>
    <BattleProvider>        // ← 新增，依赖 Stamina + Shop
      <AppInner />
    </BattleProvider>
  </ShopProvider>
</StaminaProvider>
```

BattleProvider 需要消费 `useStamina()`（扣体力/加金币）和 `useShop()`（道具使用），因此嵌套在最内层。

---

## 5. BattleScreen 组件设计

### 5.1 组件树

```
BattleScreen
├── BattlePetSelect       (phase: selecting)
│   ├── PetCard           (单个宠物卡片，显示属性/元素/稀有度)
│   └── ConfirmButton     (确认出战)
├── BattleMatching        (phase: matching)
│   └── 匹配动画
├── BattleArena           (phase: battling)
│   ├── PetStatusPanel    (敌方信息)
│   │   ├── PetAvatar     (头像 + 元素标识)
│   │   └── HealthBar     (血条 + 能量条)
│   ├── BattleField       (中间区域，简单的 emoji 对峙)
│   ├── PetStatusPanel    (玩家方信息)
│   ├── BattleLog         (战斗日志，自动滚动)
│   └── ActionBar         (操作按钮)
│       ├── UltimateButton   (必杀，能量满时高亮)
│       ├── StrategyToggle   (策略切换)
│       ├── ItemButton       (使用道具)
│       ├── AutoPlayToggle   (自动/暂停)
│       └── NextRoundButton  (手动模式下"下一回合")
└── BattleResult          (phase: result)
    ├── ResultBanner      (胜利/失败/平局)
    └── RewardsDisplay    (金币/道具掉落)
```

### 5.2 关键 UI 规格

**血条**：
- 宽度 100%，高度 12px，圆角 6px
- 颜色：HP > 50% 绿色，25~50% 黄色，< 25% 红色
- 动画：HP 变化时 300ms ease-out 过渡

**能量条**：
- 位于血条下方，高度 6px
- 满能量时发光脉冲动画
- 颜色：渐变蓝 → 紫

**回合数**：
- 顶部居中显示「回合 X / 30」

**战斗日志**：
- 底部半透明面板，高度 120px
- 自动滚动到最新
- 不同类型日志不同颜色（暴击金色，闪避灰色，必杀紫色）

**操作按钮**：
- 底部固定栏，4~5 个按钮均分宽度
- 必杀按钮：能量满时可用，否则灰色禁用
- 策略按钮：点击弹出 3 项选择（激进/平衡/防守）

### 5.3 样式方案

沿用项目现有 CSS 变量主题（暖橙卡通风格），使用内联 style 或 CSS Module。与 StoreScreen / CollectScreen 风格一致。

---

## 6. PvE 对手生成

### 6.1 难度曲线

基于玩家等级（StaminaContext.state.level）生成对手：

| 玩家等级 | 对手稀有度池 | 对手属性倍率 | 说明 |
|---------|-------------|------------|------|
| Lv.1~2 | common 70%, uncommon 25%, rare 5% | ×0.9 | 新手友好 |
| Lv.3~4 | common 30%, uncommon 45%, rare 20%, epic 5% | ×1.0 | 标准 |
| Lv.5~6 | uncommon 30%, rare 40%, epic 25%, legendary 5% | ×1.1 | 进阶 |
| Lv.7~8 | rare 30%, epic 40%, legendary 30% | ×1.2 | 挑战 |
| Lv.9~10 | epic 40%, legendary 60% | ×1.3 | 终局 |

### 6.2 对手生成算法

```typescript
function generateEnemy(playerLevel: number): BattlePet {
  // 1. 根据等级确定稀有度池
  const pool = ENEMY_RARITY_POOLS[clampLevel(playerLevel)]
  const rarity = weightedRandom(pool)

  // 2. 随机物种
  const species = randomSpecies()  // cat / goose / dog

  // 3. 生成随机 seed（对手没有真实 CardEntry，用随机 seed）
  const seed = Math.floor(Math.random() * 100000)

  // 4. 计算属性
  const baseStats = computeBattleStats(rarity, species, seed)

  // 5. 应用难度倍率
  const multiplier = getDifficultyMultiplier(playerLevel)
  const scaledStats = scaleStats(baseStats, multiplier)

  // 6. 随机元素
  const element = pickElement(seed)

  return { id: 'enemy_' + Date.now(), stats: scaledStats, ... }
}
```

### 6.3 对手命名

按物种 + 稀有度 + 随机修饰词生成名称：

```
模板：{修饰词}·{物种名}
修饰词池：流浪的 / 凶猛的 / 神秘的 / 虚弱的 / 狂暴的 / 沉睡的
示例：流浪的·橘猫 / 凶猛的·鹅 / 神秘的·柴犬
```

---

## 7. 战斗奖励

### 7.1 胜利奖励

| 奖励项 | 计算方式 | 说明 |
|--------|---------|------|
| 金币 | 对手稀有度基础金币 × 难度倍率 | 参照派遣产出表 |
| 经验 | 预留（MVP 暂不实现宠物升级） | 后续迭代 |
| 道具掉落 | 15% 概率掉落随机道具 | 仅胜利时掉落 |

**金币基础值**（同派遣产出表 6.4）：

| 对手稀有度 | 基础金币 |
|-----------|---------|
| Common | 15 |
| Uncommon | 25 |
| Rare | 40 |
| Epic | 70 |
| Legendary | 120 |

**道具掉落池**（15% 概率，从中随机 1 个）：

| 道具 | 权重 |
|------|------|
| toy_ball（玩具球）| 40 |
| bait（诱饵）| 25 |
| food_pack（食物包）| 25 |
| cold_medicine（感冒药）| 10 |

### 7.2 失败奖励

| 奖励项 | 计算方式 |
|--------|---------|
| 金币 | 5 金币（安慰奖） |
| 道具 | 无 |

### 7.3 平局奖励

| 奖励项 | 计算方式 |
|--------|---------|
| 金币 | 10 金币 |
| 道具 | 无 |

### 7.4 体力消耗

- 每次战斗消耗 **20 体力**（与捕获/派遣一致）
- 体力不足时无法开始战斗，显示提示
- 体力在「确认出战」时扣除（非战斗结束时扣除）

---

## 8. 与现有系统集成

### 8.1 集成关系图

```
┌─────────────────────────────────────────────┐
│              BattleContext                   │
│  (战斗状态管理 + 回合逻辑 + 奖励计算)         │
└──────┬──────────┬──────────┬────────────────┘
       │          │          │
       ▼          ▼          ▼
┌──────────┐ ┌──────────┐ ┌──────────────┐
│StaminaCtx│ │ ShopCtx  │ │useAnimalStore│
│          │ │          │ │              │
• consume  │ • useItem │ • animals 列表  │
  Stamina  │   (战斗用)│   (选宠来源)    │
• addGold  │           │                │
  (奖励)   │           │                │
└──────────┘ └──────────┘ └──────────────┘
```

### 8.2 StaminaContext 集成

| 调用时机 | 调用方法 | 说明 |
|---------|---------|------|
| 确认出战 | `consumeStamina(20)` | 扣 20 体力，不足则阻止战斗 |
| 战斗胜利 | `addGold(rewards.gold)` | 发放金币奖励 |
| 战斗失败 | `addGold(5)` | 安慰金币 |
| 战斗平局 | `addGold(10)` | 平局金币 |

```typescript
// BattleContext 内部
const stamina = useStamina()

const startBattle = useCallback(() => {
  if (!stamina.consumeStamina(BATTLE_STAMINA_COST)) {
    return { success: false, reason: 'insufficient_stamina' }
  }
  // ... 开始战斗
}, [stamina])

const finishBattle = useCallback((result) => {
  const gold = result === 'win' ? rewards.gold : result === 'draw' ? 10 : 5
  stamina.addGold(gold)
  // ... 道具掉落走 ShopContext
}, [stamina, rewards])
```

### 8.3 ShopContext 集成

| 调用时机 | 调用方法 | 说明 |
|---------|---------|------|
| 战斗中使用道具 | `useItem(itemId)` | 消耗背包中的道具 |
| 道具掉落 | 后续需 `addItem`（当前 ShopContext 有 ADD_ITEM action 但未暴露） | 胜利掉落道具入背包 |
| 查询道具数量 | `getItemCount(itemId)` | 战斗 UI 显示可用道具 |

```typescript
const shop = useShop()

const useBattleItem = useCallback((itemId: ItemId) => {
  const result = shop.useItem(itemId)
  if (!result.success) return false
  // 应用道具效果（回血/增益）
  // ...
  return true
}, [shop])
```

> **注意**：ShopContext 当前未暴露 `addItem` 方法。道具掉落需要新增 `addItem` 到 ShopContextValue（对应 reducer 已有 `ADD_ITEM` action，只需在 provider 中暴露）。

### 8.4 useAnimalStore 集成

| 调用时机 | 说明 |
|---------|------|
| 选宠阶段 | 从 `animals` 列表获取已解锁的宠物，展示为可选列表 |
| 选中宠物 | 将 `CardEntry` 转换为 `BattlePet`（计算属性） |

```typescript
const { animals } = useAnimalStore()

// 过滤已解锁的宠物作为可选列表
const availablePets = animals.filter(a => a.unlocked)

// CardEntry → BattlePet 转换
function cardEntryToBattlePet(entry: CardEntry, weather: WeatherType): BattlePet {
  const species = getCardSpecies(entry)
  const element = pickElement(entry.seed)
  const baseStats = computeBattleStats(entry.rarity, species, entry.seed)
  const stats = applyWeatherModifier(baseStats, weather)
  return {
    id: entry.id,
    name: SPECIES_DEFS[species].name,
    emoji: SPECIES_DEFS[species].emoji,
    species,
    rarity: entry.rarity,
    element,
    baseStats,
    stats,
    currentHp: stats.hp,
    energy: 0,
    isPlayer: true,
  }
}
```

### 8.5 App.tsx 改动

```typescript
// 1. 新增 BattleProvider 包裹
<BattleProvider>
  <AppInner />
</BattleProvider>

// 2. fight tab 渲染逻辑改为条件渲染
case 'fight':
  if (pendingPhoto) {
    return <CaptureScreen ... />  // 捕获流程，保持不变
  }
  return <BattleScreen />          // 战斗流程，新增

// 3. TopBar 天气信息传入 BattleProvider（后续 LbsContext 接入后自动更新）
```

---

## 9. 文件结构

```
frontend/src/
├── battle/
│   ├── constants.ts          # 战斗常量：属性基础值表、物种修正、元素克制表、策略定义、难度曲线、奖励表
│   ├── types.ts              # BattlePet / BattleStats / BattleState / BattleAction / ElementType / StrategyType 等
│   ├── logic.ts              # 纯函数：属性计算、伤害公式、回合执行、对手生成、奖励计算、元素克制
│   ├── BattleContext.tsx     # React Context + useReducer provider，管理战斗全流程状态
│   ├── useBattle.ts          # 自定义 Hook：封装 Context 消费
│   └── logic.test.ts         # 纯函数单元测试（≥25 个用例）
├── components/
│   ├── BattleScreen.tsx      # 战斗主界面：根据 phase 渲染不同子组件
│   ├── BattlePetSelect.tsx   # 宠物选择界面：展示可用宠物列表，显示属性/元素/稀有度
│   ├── BattleArena.tsx       # 战斗进行中：双方血条/能量/回合/日志/操作按钮
│   ├── BattleResult.tsx      # 战斗结果：胜利/失败动画 + 奖励展示
│   ├── HealthBar.tsx          # 血条 + 能量条组件（可复用）
│   └── BattleLog.tsx          # 战斗日志面板（自动滚动）
├── shop/
│   └── ShopContext.tsx        # （修改）暴露 addItem 方法供战斗掉落使用
├── App.tsx                    # （修改）包裹 BattleProvider，fight tab 条件渲染 BattleScreen
└── types.ts                   # （无修改，CardEntry 不变）
```

### 各文件职责

| 文件 | 职责 | 依赖 |
|------|------|------|
| `battle/constants.ts` | 属性基础值表 `RARITY_BASE_STATS`、物种修正 `SPECIES_MODIFIERS`、元素克制表 `ELEMENT_CHART`、策略定义 `STRATEGY_DEFS`、难度曲线 `ENEMY_RARITY_POOLS`、奖励表。纯数据 | 无 |
| `battle/types.ts` | TypeScript 类型定义。纯类型 | 无 |
| `battle/logic.ts` | 纯函数计算逻辑。不依赖 React | constants, types |
| `battle/BattleContext.tsx` | 战斗状态管理核心。Context + Reducer，自动回合计时器 | constants, types, logic, StaminaContext, ShopContext |
| `battle/useBattle.ts` | 对外暴露的 Hook | BattleContext, types |
| `components/BattleScreen.tsx` | 战斗主界面路由器 | useBattle |
| `components/BattlePetSelect.tsx` | 选宠界面 | useAnimalStore, useBattle |
| `components/BattleArena.tsx` | 战斗战斗区域 | useBattle, HealthBar, BattleLog |
| `components/BattleResult.tsx` | 结果展示 | useBattle |
| `components/HealthBar.tsx` | 可复用血条 | 无 |
| `components/BattleLog.tsx` | 日志面板 | 无 |

---

## 10. battle/constants.ts 详细定义

### 10.1 属性基础值表

```typescript
export const RARITY_BASE_STATS: Record<RarityTier, BattleStats> = {
  common:    { hp: 60,  atk: 15, def: 10, spd: 15, crit: 5,  eva: 5  },
  uncommon:  { hp: 100, atk: 28, def: 20, spd: 30, crit: 7,  eva: 7  },
  rare:      { hp: 150, atk: 45, def: 35, spd: 50, crit: 10, eva: 10 },
  epic:      { hp: 220, atk: 65, def: 50, spd: 70, crit: 15, eva: 15 },
  legendary: { hp: 350, atk: 95, def: 68, spd: 88, crit: 20, eva: 20 },
}
```

### 10.2 物种修正

```typescript
export const SPECIES_STAT_MODIFIERS: Record<SpeciesType, {
  hp: number; atk: number; def: number; spd: number; crit: number; eva: number
}> = {
  cat:   { hp: 0.8, atk: 0.9, def: 0.9, spd: 1.3, crit: 10, eva: 5  },
  dog:   { hp: 1.3, atk: 1.2, def: 1.0, spd: 0.8, crit: 3,  eva: 2  },
  goose: { hp: 1.0, atk: 0.8, def: 1.4, spd: 0.9, crit: 2,  eva: 8  },
}
```

### 10.3 元素克制表

```typescript
export const ELEMENT_CHART: Record<ElementType, Record<ElementType, number>> = {
  fire:   { fire: 1.0, water: 0.67, grass: 1.5, light: 1.0, dark: 1.0 },
  water:  { fire: 1.5, water: 1.0,  grass: 0.67, light: 1.0, dark: 1.0 },
  grass:  { fire: 0.67, water: 1.5, grass: 1.0, light: 1.0, dark: 1.0 },
  light:  { fire: 1.0, water: 1.0, grass: 1.0, light: 1.0, dark: 1.5 },
  dark:   { fire: 1.0, water: 1.0, grass: 1.0, light: 1.5, dark: 1.0 },
}
```

### 10.4 策略定义

```typescript
export const STRATEGY_DEFS: Record<StrategyType, { atkMod: number; defMod: number; label: string }> = {
  aggressive: { atkMod: 1.1, defMod: 0.9, label: '激进' },
  balanced:   { atkMod: 1.0, defMod: 1.0, label: '平衡' },
  defensive:  { atkMod: 0.9, defMod: 1.1, label: '防守' },
}
```

### 10.5 战斗常量

```typescript
export const BATTLE_STAMINA_COST = 20      // 战斗体力消耗
export const MAX_ROUNDS = 30               // 最大回合数
export const MAX_ENERGY = 100              // 能量上限
export const ENERGY_PER_ATTACK = 15        // 攻击获得能量
export const ENERGY_PER_HIT = 10           // 受击获得能量
export const CRIT_MULTIPLIER = 2.0         // 暴击伤害倍率
export const ULTIMATE_MULTIPLIER = 1.8     // 必杀伤害倍率
export const AUTO_PLAY_INTERVAL_MS = 1500  // 自动回合间隔

export const ELEMENT_TYPES: ElementType[] = ['fire', 'water', 'grass', 'light', 'dark']
```

### 10.6 对手稀有度池

```typescript
export const ENEMY_RARITY_POOLS: Record<string, { tier: RarityTier; weight: number }[]> = {
  low:    [ { tier: 'common', weight: 70 }, { tier: 'uncommon', weight: 25 }, { tier: 'rare', weight: 5 } ],
  mid:    [ { tier: 'common', weight: 30 }, { tier: 'uncommon', weight: 45 }, { tier: 'rare', weight: 20 }, { tier: 'epic', weight: 5 } ],
  high:   [ { tier: 'uncommon', weight: 30 }, { tier: 'rare', weight: 40 }, { tier: 'epic', weight: 25 }, { tier: 'legendary', weight: 5 } ],
  elite:  [ { tier: 'rare', weight: 30 }, { tier: 'epic', weight: 40 }, { tier: 'legendary', weight: 30 } ],
  boss:   [ { tier: 'epic', weight: 40 }, { tier: 'legendary', weight: 60 } ],
}

export const LEVEL_TO_POOL_MAP: Record<number, 'low' | 'mid' | 'high' | 'elite' | 'boss'> = {
  1: 'low', 2: 'low',
  3: 'mid', 4: 'mid',
  5: 'high', 6: 'high',
  7: 'elite', 8: 'elite',
  9: 'boss', 10: 'boss',
}

export const DIFFICULTY_MULTIPLIERS: Record<string, number> = {
  low: 0.9, mid: 1.0, high: 1.1, elite: 1.2, boss: 1.3,
}
```

### 10.7 奖励表

```typescript
export const BATTLE_GOLD_REWARDS: Record<RarityTier, number> = {
  common: 15, uncommon: 25, rare: 40, epic: 70, legendary: 120,
}

export const BATTLE_LOSE_GOLD = 5
export const BATTLE_DRAW_GOLD = 10

export const ITEM_DROP_RATE = 0.15  // 15% 掉落概率

export const ITEM_DROP_POOL: { itemId: ItemId; weight: number }[] = [
  { itemId: 'toy_ball', weight: 40 },
  { itemId: 'bait', weight: 25 },
  { itemId: 'food_pack', weight: 25 },
  { itemId: 'cold_medicine', weight: 10 },
]

export const ENEMY_NAME_PREFIXES = ['流浪的', '凶猛的', '神秘的', '虚弱的', '狂暴的', '沉睡的', '机警的', '慵懒的']
```

### 10.8 天气修正

```typescript
export const WEATHER_STAT_MODIFIER: Record<WeatherType, number> = {
  sunny: 1.05, cloudy: 1.0, overcast: 1.0, rainy: 1.0, snowy: 1.0, foggy: 1.0, extreme: 1.0,
}

export const WEATHER_ELEMENT_BONUS: Record<WeatherType, Partial<Record<ElementType, number>>> = {
  sunny:  { fire: 0.1, water: -0.1 },
  cloudy: {},
  overcast: {},
  rainy:  { water: 0.1, fire: -0.1 },
  snowy:  {},
  foggy:  {},
  extreme: {},
}
```

---

## 11. battle/logic.ts 核心函数

### 11.1 函数清单

| 函数 | 签名 | 说明 |
|------|------|------|
| `computeBattleStats` | `(rarity, species, seed) => BattleStats` | 从稀有度+物种+seed 计算六维属性 |
| `seedVariance` | `(seed, salt) => number` | 基于 seed 的确定性随机 0.85~1.15 |
| `pickElement` | `(seed) => ElementType` | 基于 seed 选取元素 |
| `getElementMultiplier` | `(atk, def) => number` | 元素克制倍率 |
| `applyWeatherModifier` | `(stats, weather) => BattleStats` | 应用天气属性修正 |
| `getWeatherElementBonus` | `(weather, element) => number` | 天气元素伤害加成 |
| `computeDamage` | `(attacker, defender, strategy, weather) => { damage, isCrit, isMiss }` | 计算单次伤害 |
| `executeRound` | `(player, enemy, strategy, weather, round) => { player, enemy, logs }` | 执行一回合（纯函数，返回新状态） |
| `checkBattleEnd` | `(player, enemy, round) => BattleResult \| null` | 检查战斗是否结束 |
| `generateEnemy` | `(playerLevel) => BattlePet` | 生成 PvE 对手 |
| `generateEnemyName` | `(species) => string` | 生成对手名称 |
| `computeRewards` | `(result, enemyRarity, difficultyMultiplier) => BattleRewards` | 计算战斗奖励 |
| `rollItemDrop` | `() => ItemId \| null` | 道具掉落判定 |
| `weightedRandom` | `(items: { weight: number }[], seed?: number) => number` | 加权随机选取索引 |
| `scaleStats` | `(stats, multiplier) => BattleStats` | 按倍率缩放属性 |
| `cardEntryToBattlePet` | `(entry, weather) => BattlePet` | CardEntry 转 BattlePet |
| `applyStrategy` | `(stats, strategy) => { atk, def }` | 应用策略修正 |

### 11.2 核心函数伪代码

```typescript
/** 计算六维属性 */
export function computeBattleStats(
  rarity: RarityTier,
  species: SpeciesType,
  seed: number
): BattleStats {
  const base = RARITY_BASE_STATS[rarity]
  const mod = SPECIES_STAT_MODIFIERS[species]

  return {
    hp:   Math.round(base.hp   * mod.hp   * seedVariance(seed, 1)),
    atk:  Math.round(base.atk  * mod.atk  * seedVariance(seed, 2)),
    def:  Math.round(base.def  * mod.def  * seedVariance(seed, 3)),
    spd:  Math.round(base.spd  * mod.spd  * seedVariance(seed, 4)),
    crit: base.crit + mod.crit,  // 暴击率直接加算
    eva:  base.eva + mod.eva,     // 闪避率直接加算
  }
}

/** 计算单次伤害 */
export function computeDamage(
  attacker: BattlePet,
  defender: BattlePet,
  strategy: StrategyType,
  weather: WeatherType,
  isUltimate: boolean = false
): { damage: number; isCrit: boolean; isMiss: boolean } {
  // 闪避判定（必杀技不可闪避）
  if (!isUltimate && Math.random() * 100 < defender.stats.eva) {
    return { damage: 0, isCrit: false, isMiss: true }
  }

  // 策略修正
  const strat = STRATEGY_DEFS[strategy]
  const atk = attacker.stats.atk * strat.atkMod
  const def = defender.stats.def * strat.defMod

  // 基础伤害
  let damage = Math.max(1, atk - def)

  // 元素克制
  damage *= getElementMultiplier(attacker.element, defender.element)

  // 天气元素加成
  const weatherBonus = getWeatherElementBonus(weather, attacker.element)
  damage *= (1 + weatherBonus)

  // 暴击判定（必杀技必定暴击）
  const isCrit = isUltimate || Math.random() * 100 < attacker.stats.crit
  if (isCrit) {
    damage *= isUltimate ? ULTIMATE_MULTIPLIER : CRIT_MULTIPLIER
  }

  return { damage: Math.round(damage), isCrit, isMiss: false }
}

/** 执行一回合（纯函数） */
export function executeRound(
  player: BattlePet,
  enemy: BattlePet,
  strategy: StrategyType,
  weather: WeatherType,
  round: number
): {
  player: BattlePet
  enemy: BattlePet
  logs: BattleLogEntry[]
} {
  const logs: BattleLogEntry[] = []
  let p = { ...player }
  let e = { ...enemy }

  // 判定先后手
  const playerFirst = p.stats.spd >= e.stats.spd
    ? true
    : e.stats.spd > p.stats.spd
      ? false
      : Math.random() < 0.5

  const order = playerFirst ? [p, e] : [e, p]
  const labels = playerFirst ? ['我方', '敌方'] : ['敌方', '我方']

  for (let i = 0; i < 2; i++) {
    const attacker = order[i]
    const defender = order[i === 0 ? 1 : 0]
    const attackerRef = attacker === p ? p : e
    const defenderRef = defender === p ? p : e

    // 检查防守方是否已死亡
    if (defenderRef.currentHp <= 0) break

    const result = computeDamage(attacker, defender, strategy, weather)

    if (result.isMiss) {
      logs.push({ round, text: `${labels[i]}攻击被闪避！`, type: 'miss' })
    } else {
      defenderRef.currentHp = Math.max(0, defenderRef.currentHp - result.damage)
      logs.push({
        round,
        text: result.isCrit
          ? `${labels[i]}暴击！造成 ${result.damage} 伤害`
          : `${labels[i]}攻击，造成 ${result.damage} 伤害`,
        type: result.isCrit ? 'crit' : 'attack',
      })
    }

    // 积累能量
    attackerRef.energy = Math.min(MAX_ENERGY, attackerRef.energy + ENERGY_PER_ATTACK)
    defenderRef.energy = Math.min(MAX_ENERGY, defenderRef.energy + ENERGY_PER_HIT)

    // 检查死亡
    if (defenderRef.currentHp <= 0) break
  }

  return { player: p, enemy: e, logs }
}
```

---

## 12. 测试用例（≥ 25 个）

### 12.1 属性计算测试（8 个）

| # | 测试名 | 验证点 |
|---|--------|--------|
| 1 | `computeBattleStats: common cat 有正确的基础属性` | HP≈48(60×0.8×variance), ATK≈13.5(15×0.9), SPD≈19.5(15×1.3) |
| 2 | `computeBattleStats: legendary dog 有高 HP 和 ATK` | HP≈455(350×1.3), ATK≈114(95×1.2), 均在 variance 范围内 |
| 3 | `computeBattleStats: rare goose 有高 DEF` | DEF≈49(35×1.4)，高于同稀有度猫/狗 |
| 4 | `computeBattleStats: 同 seed 同 rarity 同 species 结果一致` | 确定性：调用两次结果相同 |
| 5 | `computeBattleStats: cat 的 SPD 始终高于同稀有度 dog` | 物种差异验证 |
| 6 | `computeBattleStats: dog 的 HP 始终高于同稀有度 cat` | 物种差异验证 |
| 7 | `computeBattleStats: goose 的 DEF 始终高于同稀有度 cat` | 物种差异验证 |
| 8 | `seedVariance: 返回值在 0.85~1.15 范围内` | 边界验证 |

### 12.2 元素系统测试（5 个）

| # | 测试名 | 验证点 |
|---|--------|--------|
| 9 | `getElementMultiplier: fire vs grass = 1.5` | 火克草 |
| 10 | `getElementMultiplier: grass vs fire = 0.67` | 草被火克 |
| 11 | `getElementMultiplier: light vs dark = 1.5` | 光暗互克 |
| 12 | `getElementMultiplier: fire vs fire = 1.0` | 同元素 |
| 13 | `getElementMultiplier: fire vs light = 1.0` | 无克制关系 |

### 12.3 伤害计算测试（6 个）

| # | 测试名 | 验证点 |
|---|--------|--------|
| 14 | `computeDamage: 基础伤害 = max(1, ATK - DEF)` | 确定性伤害计算 |
| 15 | `computeDamage: 元素克制 1.5x 伤害` | fire 打 grass 伤害为正常的 1.5 倍 |
| 16 | `computeDamage: 闪避时伤害为 0` | EVA 触发时 damage=0, isMiss=true |
| 17 | `computeDamage: 必杀技不可被闪避` | isUltimate=true 时 isMiss 始终为 false |
| 18 | `computeDamage: 必杀技必定暴击` | isUltimate=true 时 isCrit 始终为 true |
| 19 | `computeDamage: 策略修正正确应用` | aggressive 时 ATK+10%，defensive 时 DEF+10% |

### 12.4 回合执行测试（4 个）

| # | 测试名 | 验证点 |
|---|--------|--------|
| 20 | `executeRound: SPD 高者先手` | 玩家 SPD > 敌方 → 玩家先攻击 |
| 21 | `executeRound: 回合后双方能量增加` | 攻击方+15，受击方+10 |
| 22 | `executeRound: 防守方死亡后不执行后手攻击` | 先手击杀后手 → 后手不行动 |
| 23 | `checkBattleEnd: HP=0 返回胜负结果` | player HP=0 → lose, enemy HP=0 → win |

### 12.5 对手生成测试（3 个）

| # | 测试名 | 验证点 |
|---|--------|--------|
| 24 | `generateEnemy: Lv.1 对手稀有度只含 common/uncommon/rare` | 低等级不出 epic/legendary |
| 25 | `generateEnemy: Lv.10 对手稀有度含 epic/legendary` | 高等级出强力对手 |
| 26 | `generateEnemy: 对手属性随等级提升而增强` | Lv.10 对手 HP > Lv.1 对手 HP（同稀有度对比） |

### 12.6 奖励计算测试（4 个）

| # | 测试名 | 验证点 |
|---|--------|--------|
| 27 | `computeRewards: 胜利获得金币（基于对手稀有度）` | common=15, legendary=120 |
| 28 | `computeRewards: 失败获得 5 金币安慰奖` | 失败固定 5 金币 |
| 29 | `computeRewards: 平局获得 10 金币` | 平局固定 10 金币 |
| 30 | `rollItemDrop: 15% 概率掉落道具` | 多次调用统计概率在 10~20% 范围 |

### 12.7 集成测试（2 个）

| # | 测试名 | 验证点 |
|---|--------|--------|
| 31 | `cardEntryToBattlePet: 正确转换 CardEntry 为 BattlePet` | 属性计算正确，currentHp=stats.hp，energy=0 |
| 32 | `applyWeatherModifier: 晴天全属性 +5%` | sunny 时所有属性为原来的 1.05 倍 |

---

## 13. 验收标准对照

### Issue #37 验收标准：自动回合制可玩，PvE 完整

| 验收项 | 实现方案 | 状态 |
|--------|---------|------|
| 自动回合制 | 按 SPD 自动执行回合，每 1.5 秒推进，无需手动操作 | ✅ 计划 |
| 可玩 | 选宠 → 匹配 → 战斗 → 结算 全流程闭环 | ✅ 计划 |
| PvE 完整 | 基于玩家等级生成对手，含难度曲线 + 奖励 | ✅ 计划 |
| 六维属性 | HP/ATK/DEF/SPD/CRIT/EVA，基于稀有度+物种+seed | ✅ 计划 |
| 元素克制 | 火>草>水>火循环 + 光暗互克，1.5x | ✅ 计划 |
| 物种差异化 | 猫速/狗血/鹅防，体现在属性修正 | ✅ 计划 |
| 天气影响 | 晴天+5%全属性，雨天水+10%/火-10% | ✅ 计划 |
| 半自动 | 能量条必杀 + 策略切换 + 道具使用 | ✅ 计划 |
| 体力消耗 | 战斗消耗 20 体力（StaminaContext 集成） | ✅ 计划 |
| 奖励系统 | 金币（基于对手稀有度）+ 道具掉落（15%） | ✅ 计划 |
| 与现有系统集成 | StaminaContext（体力/金币）+ ShopContext（道具）+ useAnimalStore（选宠） | ✅ 计划 |

### 设计文档 6.5 验收指标

| 指标 | 目标 | MVP 达成情况 |
|------|------|-------------|
| 单局 ≤ 3min | 30 回合 × 1.5s = 45s + 操作时间 ≤ 2min | ✅ |
| 策略深度足够 | 元素克制 + 策略切换 + 必杀时机 | ✅（基础层，3v3/技能树后续迭代） |

### 后续迭代预留（不在本次 Issue 范围）

| 功能 | 预留方式 | 迭代 Issue |
|------|---------|-----------|
| 3v3 阵容 | BattlePet 数组化，当前固定 1v1 | M3 |
| 站位系统 | BattlePet 增加 position 字段 | M3 |
| 技能系统 | BattlePet 增加 skills 数组 | M3 |
| PvP 排位 | 匹配逻辑切换为 PvP 模式 | M3 |
| 天梯段位 | 新增 RankContext | M3 |
| 宠物升级/经验 | rewards.exp 已预留 | M3 |
| 感冒状态影响 | weather + status modifier 接口已预留 | M2 后续 |
| 亲密度加成 | stats 计算预留 affinity 参数 | M3 |

---

## 14. 实现顺序建议

```
Step 1: battle/constants.ts + battle/types.ts     ← 纯数据/类型，无依赖
Step 2: battle/logic.ts + battle/logic.test.ts     ← 纯函数，可独立测试
Step 3: battle/BattleContext.tsx + useBattle.ts    ← 状态管理，依赖 logic + stamina + shop
Step 4: components/HealthBar.tsx + BattleLog.tsx   ← 可复用 UI 组件
Step 5: components/BattlePetSelect.tsx              ← 选宠界面
Step 6: components/BattleArena.tsx                  ← 战斗主界面
Step 7: components/BattleResult.tsx                 ← 结果展示
Step 8: components/BattleScreen.tsx                 ← 主界面路由
Step 9: App.tsx 改动 + ShopContext.addItem 暴露    ← 集成
Step 10: 集成测试 + 调试                             ← 端到端验证
```

---

## 15. 风险与注意事项

| 风险 | 影响 | 应对 |
|------|------|------|
| CardEntry 无战斗属性字段 | 需运行时计算 | 通过 rarity + species + seed 计算属性，不改 CardEntry 结构 |
| 天气系统未完整接入 | 天气修正无法生效 | 默认 sunny，天气通过 props 传入，LbsContext 完成后自动接入 |
| ShopContext 未暴露 addItem | 道具掉落无法入背包 | 本 Issue 中为 ShopContext 新增 `addItem` 方法暴露 |
| 战斗中切换 tab | 状态可能丢失 | BattleProvider 挂载在 App 顶层，不随 tab 切换卸载 |
| 自动回合性能 | 频繁 re-render | Reducer + useMemo 优化，日志数组限制最大 50 条 |
| 1v1 vs 3v3 架构差异 | 后续迁移成本 | BattlePet 设计为数组友好的单元素，logic 函数参数为单只宠物，3v3 迭代时改为数组调用即可 |

---

*本计划基于 `游戏开发计划.md` v1.4，对应 Issue #37 [M2] 战斗系统。遵循 KISS 原则，MVP 实现 1v1 自动回合制 PvE 完整流程，预留 3v3/PvP/技能树扩展接口。*
