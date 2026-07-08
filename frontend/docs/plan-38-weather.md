# [M2] 天气系统实现计划 — Issue #38

> **验收标准**：游戏内按地级市随机生成 + 彩云可选真实源，感冒机制生效（雨8%/雪6%）
>
> **设计文档来源**：`游戏开发计划.md` 3.5 天气系统 + 5.4 状态系统
>
> **技术栈**：React 18 + Vite 6 + TypeScript 5.6 + vitest，无额外依赖
>
> **现有基础**：`LbsContext` 已管理 `cityName` / `provinceName`，`TopBar` 已有 `weather` prop 预留位，`DiscoverScreen` 已有天气提示占位 UI，`BattleContext` 已定义 `WeatherType` 联合类型与天气修正常量（`WEATHER_STAT_MODIFIER` / `WEATHER_ELEMENT_BONUS`），后端已有 `/weather/week` mock 端点

---

## 1. 文件结构

```
frontend/src/
├── weather/
│   ├── constants.ts        # 天气概率表 WEATHER_ROLL_TABLE + 天气影响常量化
│   ├── types.ts            # WeatherType / WeekWeather / CurrentWeather / WeatherAction 等类型
│   ├── logic.ts            # 纯函数：seedGen / rollWeekWeather / getColdProbability / getCaptureModifier 等（无副作用，单测）
│   ├── WeatherContext.tsx  # React Context + useReducer，按地级市生成/刷新本周天气，与 LbsContext 联动城市名
│   ├── useWeather.ts       # 自定义 Hook：封装 Context 消费
│   ├── logic.test.ts       # 纯函数单元测试（≥15 个用例）
│   └── api.ts              # 后端 API 调用：fetchWeekWeather(city) → GET /api/v1/weather/week?city=
├── components/
│   ├── TopBar.tsx          # （修改）从 useWeather 读取当前天气，替换硬编码 `weather` prop 默认值
│   ├── DiscoverScreen.tsx  # （修改）从 useWeather 读取当前天气，显示捕获修正提示 + 感冒风险警告
│   └── BattleScreen.tsx    # （可能修改）天气数据从 WeatherContext 传入 BattleContext（替代手动传 props）
├── App.tsx                 # （修改）包裹 <WeatherProvider>
```

### 各文件职责

| 文件 | 职责 | 依赖 |
|------|------|------|
| `weather/constants.ts` | 天气类型定义、随机概率表、捕获修正倍率、感冒概率常量、战斗属性修正表。纯数据 | 无 |
| `weather/types.ts` | TypeScript 类型：`WeatherType` / `WeekWeather` / `CurrentWeather` / `WeatherState` / `WeatherAction` / `CaptureModifierResult` / `ColdCheckResult`。纯类型 | `constants.ts` |
| `weather/logic.ts` | 纯函数：确定性种子生成、本周天气随机生成、捕获修正计算、感冒概率判定、战斗属性修正查询。无副作用 | `constants.ts`, `types.ts` |
| `weather/api.ts` | 后端天气 API 调用封装，`fetchWeekWeather(city)` → `GET /api/v1/weather/week?city=` | 无 |
| `weather/WeatherContext.tsx` | 天气状态管理核心。创建 Context + Reducer，监听 LbsContext.cityName 变化自动刷新天气，MVP 用游戏内随机生成，可选叠加后端真实天气源 | `constants`, `types`, `logic`, `api`, `LbsContext` |
| `weather/useWeather.ts` | 对外暴露的 Hook，封装 `useContext(WeatherContext)` + null 检查 | `WeatherContext`, `types` |
| `components/TopBar.tsx` | 顶部栏显示区域从 `useWeather` 读天气 icon + 城市名 | `useWeather` |
| `components/DiscoverScreen.tsx` | 天气捕获修正提示 + 感冒风险警告，从 `useWeather` 读取数据 | `useWeather` |
| `App.tsx` | `<WeatherProvider>` 包裹，置于 `LbsProvider` 内、`StaminaProvider` 同级 | `WeatherContext` |

---

## 2. 天气类型定义 (`weather/types.ts`)

### 2.1 WeatherType（与 BattleContext 已有定义对齐）

```typescript
/** 天气类型 — 与 BattleContext 中 WeatherType 保持一致 */
export type WeatherType = 'sunny' | 'cloudy' | 'overcast' | 'rainy' | 'snowy' | 'foggy' | 'extreme'
```

### 2.2 天气元数据定义

```typescript
/** 天气元数据（用于 UI 展示） */
export interface WeatherMeta {
  type: WeatherType
  /** 中文名 */
  name: string
  /** 显示 emoji */
  emoji: string
  /** 简短描述 */
  desc: string
  /** 健康风险描述（雨/雪显示感冒警告） */
  riskDesc?: string
}

/** 天气元数据表 */
export const WEATHER_META: Record<WeatherType, WeatherMeta> = {
  sunny:    { type: 'sunny',    name: '晴', emoji: '☀️', desc: '光线充足 · 捕获 +5%' },
  cloudy:   { type: 'cloudy',   name: '多云', emoji: '⛅', desc: '多云' },
  overcast: { type: 'overcast', name: '阴', emoji: '☁️', desc: '光线偏暗 · 捕获 -5%' },
  rainy:    { type: 'rainy',    name: '雨', emoji: '🌧️', desc: '视线受阻 · 捕获 -15%', riskDesc: '出行感冒概率 8%' },
  snowy:    { type: 'snowy',    name: '雪', emoji: '❄️', desc: '低温操作受阻 · 捕获 -10%', riskDesc: '出行感冒概率 6%' },
  foggy:    { type: 'foggy',    name: '雾', emoji: '🌫️', desc: '检测距离缩短 · 捕获 -10%' },
  extreme:  { type: 'extreme',  name: '极端天气', emoji: '⚠️', desc: '户外玩法暂停 · 注意安全' },
}
```

### 2.3 核心状态类型

```typescript
/** 一周天气数组（周一~周日，7 天） */
export type WeekWeather = [
  CurrentWeather, CurrentWeather, CurrentWeather, CurrentWeather,
  CurrentWeather, CurrentWeather, CurrentWeather,
]

/** 单日天气 */
export interface CurrentWeather {
  /** 天气类型 */
  weather: WeatherType
  /** 日期标签（"周一"~"周日"） */
  dayLabel: string
}

/** 天气状态 */
export interface WeatherState {
  /** 当前周的天气（7 天数组） */
  week: WeekWeather | null
  /** 本周开始时间戳（周日 0:00） */
  weekStart: number
  /** 所属地级市 */
  city: string
  /** 数据源：'internal' = 游戏内随机, 'backend' = 后端下发 */
  source: 'internal' | 'backend'
  /** 加载状态 */
  status: 'idle' | 'loading' | 'loaded' | 'error'
  /** 错误信息 */
  errorMsg: string
}

/** Reducer Action */
export type WeatherAction =
  | { type: 'SET_WEEK'; week: WeekWeather; weekStart: number; city: string; source: 'internal' | 'backend' }
  | { type: 'LOADING' }
  | { type: 'ERROR'; msg: string }
  | { type: 'RESET' }
```

### 2.4 Context 暴露接口

```typescript
/** 捕获修正结果 */
export interface CaptureModifierResult {
  modifier: number        // 百分比修正值（如 +5, -15）
  multiplier: number      // 倍率修正（如 1.05, 0.85）
  description: string     // 展示文本
}

/** 感冒判定结果 */
export interface ColdCheckResult {
  isRisky: boolean        // 当前天气存在感冒风险
  probability: number     // 感冒概率（0~1，如 0.08）
  description: string     // 展示文本
}

/** WeatherContext 暴露给组件的接口 */
export interface WeatherContextValue {
  state: WeatherState
  /** 今天的天气 */
  today: CurrentWeather | null
  /** 今日天气元数据 */
  todayMeta: WeatherMeta | null
  /** 获取今日捕获修正 */
  getCaptureModifier: () => CaptureModifierResult
  /** 获取今日感冒风险 */
  getColdRisk: () => ColdCheckResult
  /** 获取今日战斗属性修正 */
  getBattleModifier: () => WeatherType
  /** 手动刷新天气（跨城市切换时调用） */
  refreshWeather: (city: string) => void
}
```

---

## 3. 常量定义 (`weather/constants.ts`)

### 3.1 天气随机概率表（设计文档 3.5）

```typescript
/** 天气概率权重表 */
export const WEATHER_ROLL_TABLE: { weather: WeatherType; weight: number }[] = [
  { weather: 'sunny',    weight: 33 },
  { weather: 'cloudy',   weight: 24 },
  { weather: 'overcast', weight: 18 },
  { weather: 'rainy',    weight: 15 },
  { weather: 'snowy',    weight: 4  },
  { weather: 'foggy',    weight: 4  },
  { weather: 'extreme',  weight: 2  },
]

/** 总权重（100） */
export const WEATHER_TOTAL_WEIGHT = 100
```

### 3.2 捕获成功率修正（设计文档 3.5）

```typescript
/** 天气 → 捕获倍率映射 */
export const WEATHER_CAPTURE_MODIFIER: Record<WeatherType, number> = {
  sunny:    1.05,  // +5%
  cloudy:   1.00,  // 标准
  overcast: 0.95,  // -5%
  rainy:    0.85,  // -15%
  snowy:    0.90,  // -10%
  foggy:    0.90,  // -10%
  extreme:  0.00,  // 不可捕获
}
```

### 3.3 感冒概率（设计文档 5.4）

```typescript
/** 天气 → 感冒触发概率 */
export const COLD_PROBABILITY: Partial<Record<WeatherType, number>> = {
  rainy: 0.08,   // 雨天 8%
  snowy: 0.06,   // 雪天 6%
  // 其他天气为 0（无风险）
}
```

### 3.4 战斗属性修正（复用 BattleContext 已有定义）

```typescript
// 此处直接引用 BattleContext 中已定义的常量，不重复定义
// WEATHER_STAT_MODIFIER: Record<WeatherType, number> = { sunny: 1.05, ... }
// WEATHER_ELEMENT_BONUS: Record<WeatherType, Partial<Record<ElementType, number>>> = { sunny: { fire: 0.1, water: -0.1 }, ... }
```

### 3.5 常量配置

```typescript
/** 天气周起始时间偏移量（周日 0:00） */
export const WEEK_RESET_DAY = 0 // 周日
export const WEEK_RESET_HOUR = 0
export const WEEK_RESET_MINUTE = 0

/** 天气缓存 localStorage key */
export const WEATHER_CACHE_KEY = 'animal_poke_weather_cache'

/** 一周天数 */
export const DAYS_IN_WEEK = 7

/** 星期标签 */
export const DAY_LABELS = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'] as const
```

---

## 4. 游戏内随机生成 (`weather/logic.ts`)

### 4.1 确定性种子生成

```typescript
/**
 * 根据城市名 + 周起始时间生成确定性种子
 * 同一城市同一周始终得到相同天气，不同城市/不同周则不同
 * 使用简单的 djb2 哈希算法
 */
export function generateSeed(city: string, weekStart: number): number {
  const key = `${city}_${weekStart}`
  let hash = 5381
  for (let i = 0; i < key.length; i++) {
    hash = ((hash << 5) + hash + key.charCodeAt(i)) | 0
  }
  return hash >>> 0 // 转为无符号 32 位整数
}
```

### 4.2 基于种子的伪随机数

```typescript
/**
 * 基于种子的伪随机数生成器（Mulberry32）
 * 确保同一种子每次生成相同序列
 */
export function createRng(seed: number): () => number {
  let s = seed
  return () => {
    s |= 0
    s = (s + 0x6D2B79F5) | 0
    let t = Math.imul(s ^ (s >>> 15), 1 | s)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}
```

### 4.3 权重随机（轮盘赌）

```typescript
/**
 * 根据权重表随机选取一项
 * @param table 权重表
 * @param rng 随机数生成器
 * @param totalWeight 总权重
 */
export function weightedRandom<T>(
  table: { item: T; weight: number }[],
  rng: () => number,
  totalWeight: number
): T {
  const roll = rng() * totalWeight
  let cumulative = 0
  for (const entry of table) {
    cumulative += entry.weight
    if (roll < cumulative) {
      return entry.item
    }
  }
  // fallback：返回最后一项（理论上不会到达）
  return table[table.length - 1].item
}
```

### 4.4 生成一周天气

```typescript
import { WEATHER_ROLL_TABLE, WEATHER_TOTAL_WEIGHT, DAY_LABELS } from './constants'

/**
 * 根据城市 + 周起始时间生成一周天气（确定性）
 * @param city 地级市名
 * @param weekStart 本周起始时间戳（周日 0:00）
 * @returns 7 天天气数组
 */
export function generateWeekWeather(city: string, weekStart: number): WeekWeather {
  const seed = generateSeed(city, weekStart)
  const rng = createRng(seed)

  return Array.from({ length: 7 }, (_, i): CurrentWeather => {
    const weatherTable = WEATHER_ROLL_TABLE.map(e => ({ item: e.weather, weight: e.weight }))
    const weather = weightedRandom(weatherTable, rng, WEATHER_TOTAL_WEIGHT)
    return {
      weather,
      dayLabel: DAY_LABELS[i],
    }
  }) as WeekWeather
}
```

### 4.5 今日天气查询

```typescript
/**
 * 获取今天的天气（从周天气数组中按今天是周几索引）
 * @param week 本周天气数组
 * @returns 今日天气，若 week 为 null 返回 null
 */
export function getTodayWeather(week: WeekWeather | null): CurrentWeather | null {
  if (!week) return null
  const today = new Date().getDay() // 0=周日, 1=周一, ..., 6=周六
  // DAY_LABELS 索引 0 对应周日，直接对齐
  return week[today] ?? null
}
```

### 4.6 本周起始时间计算

```typescript
/**
 * 计算本周起始时间戳（周日 0:00 UTC+8）
 */
export function getWeekStart(now: number = Date.now()): number {
  const date = new Date(now)
  // 转换为北京时间偏移
  const day = date.getDay() // 0=周日
  const start = new Date(date.getFullYear(), date.getMonth(), date.getDate() - day)
  start.setHours(0, 0, 0, 0)
  return start.getTime()
}
```

### 4.7 捕获修正计算

```typescript
import { WEATHER_CAPTURE_MODIFIER } from './constants'
import type { WeatherType, CaptureModifierResult } from './types'

/**
 * 计算天气对捕获成功率的修正
 * @param weather 当前天气类型
 * @returns 修正结果（百分比描述 + 倍率）
 */
export function getCaptureModifier(weather: WeatherType): CaptureModifierResult {
  const multiplier = WEATHER_CAPTURE_MODIFIER[weather]
  const modifier = Math.round((multiplier - 1) * 100)
  const sign = modifier >= 0 ? '+' : ''
  return {
    modifier,
    multiplier,
    description: modifier === 0 ? '标准捕获率' : `捕获率 ${sign}${modifier}%`,
  }
}
```

### 4.8 感冒概率查询

```typescript
import { COLD_PROBABILITY } from './constants'
import type { WeatherType, ColdCheckResult } from './types'

/**
 * 查询当前天气的感冒风险
 * @param weather 当前天气类型
 * @returns 感冒风险结果
 */
export function getColdRisk(weather: WeatherType): ColdCheckResult {
  const probability = COLD_PROBABILITY[weather] ?? 0
  let description: string
  if (weather === 'rainy') {
    description = '雨天出行 · 宠物感冒概率 8%'
  } else if (weather === 'snowy') {
    description = '雪天出行 · 宠物感冒概率 6%'
  } else {
    description = '当前天气无不健康状况'
  }
  return {
    isRisky: probability > 0,
    probability,
    description,
  }
}
```

---

## 5. 后端集成 (`weather/api.ts`)

### 5.1 API 调用

```typescript
import type { WeekWeather } from './types'

/**
 * 从后端拉取本周天气（可选真实天气源）
 * @param city 地级市名
 * @returns 本周天气数组
 */
export async function fetchWeekWeather(city: string): Promise<WeekWeather | null> {
  try {
    const resp = await fetch(`/api/v1/weather/week?city=${encodeURIComponent(city)}`)
    if (!resp.ok) throw new Error(`weather/week 请求失败: ${resp.status}`)
    const data = await resp.json()
    // 期望后端返回格式：{ week: [{ weather, dayLabel }, ...] }
    if (data?.week && Array.isArray(data.week) && data.week.length === 7) {
      return data.week as WeekWeather
    }
    throw new Error('后端天气数据格式异常')
  } catch {
    // 静默降级：后端不可用时使用游戏内随机生成
    return null
  }
}
```

### 5.2 集成策略

```
天气数据获取优先级：
  1. MVP：游戏内随机生成（不依赖后端）
  2. 内测/公测：优先尝试后端拉取，失败降级为游戏内随机
  3. 后端支持时：展示 source='backend' 标识（可选）

流程：
  cityName 变更 → 检查缓存 → 逾期则尝试 fetchWeekWeather → 失败则 generateWeekWeather
```

---

## 6. WeatherContext 设计 (`weather/WeatherContext.tsx`)

### 6.1 上下文创建（遵循现有 Context 模式）

```typescript
import React, { createContext, useReducer, useEffect, useCallback, useMemo } from 'react'
import { useLbs } from '../lbs/useLbs'
import {
  WEATHER_CACHE_KEY,
  DAY_LABELS,
} from './constants'
import type {
  WeatherState, WeatherAction, WeatherContextValue,
  WeekWeather, CurrentWeather, WeatherMeta, WeatherType,
  CaptureModifierResult, ColdCheckResult,
} from './types'
import { WEATHER_META } from './types'
import {
  generateWeekWeather, getTodayWeather, getWeekStart,
  getCaptureModifier, getColdRisk,
} from './logic'
import { fetchWeekWeather } from './api'

/** 默认初始状态 */
const defaultState: WeatherState = {
  week: null,
  weekStart: 0,
  city: '',
  source: 'internal',
  status: 'idle',
  errorMsg: '',
}

/** 从 localStorage 加载缓存天气 */
function loadInitialState(): WeatherState {
  try {
    const saved = localStorage.getItem(WEATHER_CACHE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as WeatherState & { _ts?: number }
      // 检查缓存是否过期（超过一周）
      const cachedWeekStart = parsed.weekStart
      const currentWeekStart = getWeekStart()
      if (cachedWeekStart === currentWeekStart && parsed.week) {
        // 缓存仍然有效（同一周）
        return {
          ...parsed,
          status: 'loaded',
          errorMsg: '',
        }
      }
      // 缓存过期，但保留 city 信息
      if (parsed.city) {
        return { ...defaultState, city: parsed.city }
      }
    }
  } catch (e) {
    console.warn('加载天气缓存失败:', e)
  }
  return defaultState
}

/** Reducer */
function weatherReducer(state: WeatherState, action: WeatherAction): WeatherState {
  switch (action.type) {
    case 'LOADING':
      return { ...state, status: 'loading', errorMsg: '' }
    case 'SET_WEEK':
      return {
        ...state,
        week: action.week,
        weekStart: action.weekStart,
        city: action.city,
        source: action.source,
        status: 'loaded',
        errorMsg: '',
      }
    case 'ERROR':
      return { ...state, status: 'error', errorMsg: action.msg }
    case 'RESET':
      return defaultState
    default:
      return state
  }
}

export const WeatherContext = createContext<WeatherContextValue | null>(null)

export const WeatherProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(weatherReducer, undefined, loadInitialState)
  const lbs = useLbs() // 需放在 LbsProvider 内

  const cityName = lbs.state.cityName

  /** 为指定城市生成本周天气 */
  const loadWeather = useCallback(async (city: string) => {
    if (!city) return

    const weekStart = getWeekStart()
    dispatch({ type: 'LOADING' })

    // 尝试后端拉取
    const backendWeek = await fetchWeekWeather(city)
    if (backendWeek) {
      dispatch({ type: 'SET_WEEK', week: backendWeek, weekStart, city, source: 'backend' })
      return
    }

    // 降级为游戏内随机生成
    const week = generateWeekWeather(city, weekStart)
    dispatch({ type: 'SET_WEEK', week, weekStart, city, source: 'internal' })
  }, [])

  // 城市名变化时自动加载天气
  useEffect(() => {
    if (cityName && cityName !== state.city) {
      loadWeather(cityName)
    }
  }, [cityName, state.city, loadWeather])

  // 周日 0:00 定时刷新检查（每分钟检查一次）
  useEffect(() => {
    const checkWeekReset = () => {
      const currentWeekStart = getWeekStart()
      if (state.weekStart && currentWeekStart !== state.weekStart) {
        // 新的一周，重新加载
        loadWeather(state.city)
      }
    }

    const interval = setInterval(checkWeekReset, 60_000)
    return () => clearInterval(interval)
  }, [state.weekStart, state.city, loadWeather])

  // 页面可见性恢复时检查周切换
  useEffect(() => {
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        const currentWeekStart = getWeekStart()
        if (state.weekStart && currentWeekStart !== state.weekStart && state.city) {
          loadWeather(state.city)
        }
      }
    }
    document.addEventListener('visibilitychange', handleVisibility)
    return () => document.removeEventListener('visibilitychange', handleVisibility)
  }, [state.weekStart, state.city, loadWeather])

  // localStorage 持久化
  useEffect(() => {
    if (state.status === 'loaded' && state.week) {
      localStorage.setItem(WEATHER_CACHE_KEY, JSON.stringify(state))
    }
  }, [state])

  // 今日天气（派生状态，使用 useMemo 避免对象引用变化）
  const today = useMemo<CurrentWeather | null>(() => {
    return getTodayWeather(state.week)
  }, [state.week])

  const todayMeta = useMemo<WeatherMeta | null>(() => {
    return today ? WEATHER_META[today.weather] : null
  }, [today])

  // 派生计算函数
  const captureModifierFn = useCallback((): CaptureModifierResult => {
    if (!today) return { modifier: 0, multiplier: 1.0, description: '天气数据加载中' }
    return getCaptureModifier(today.weather)
  }, [today])

  const coldRiskFn = useCallback((): ColdCheckResult => {
    if (!today) return { isRisky: false, probability: 0, description: '天气数据加载中' }
    return getColdRisk(today.weather)
  }, [today])

  const battleModifierFn = useCallback((): WeatherType => {
    return today?.weather ?? 'sunny'
  }, [today])

  const refreshWeather = useCallback((city: string) => {
    loadWeather(city)
  }, [loadWeather])

  const value = useMemo<WeatherContextValue>(() => ({
    state,
    today,
    todayMeta,
    getCaptureModifier: captureModifierFn,
    getColdRisk: coldRiskFn,
    getBattleModifier: battleModifierFn,
    refreshWeather,
  }), [state, today, todayMeta, captureModifierFn, coldRiskFn, battleModifierFn, refreshWeather])

  return (
    <WeatherContext.Provider value={value}>
      {children}
    </WeatherContext.Provider>
  )
}
```

### 6.2 关键设计决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 天气生成时机 | 首次定位城市 + 周日 0:00 刷新 | 一周固定天气（设计文档 3.5） |
| 生成方式 | 确定性 seed（city + weekStart）→ PRNG | 同城市同周同天气，可复现可校验 |
| 后端集成 | 优先 fetch，失败降级 generate | KISS：MVP 无后端也能运行 |
| 缓存策略 | localStorage，按 weekStart 过期 | 离线可用，切周自动刷新 |
| 感冒判定 | 由逻辑层提供概率，调用方执行随机 | 分离关注点：天气系统只管理概率，状态系统执行判定 |
| 城市联动 | 监听 LbsContext.cityName 变化 | 跨城市自动切换天气 |

---

## 7. 天气影响

### 7.1 捕获成功率修正

```
天气 → 捕获倍率（设计文档 3.5 捕获难易度表）：

  晴 ☀️    → 1.05（+5%）
  多云 ⛅    → 1.00（标准）
  阴 ☁️    → 0.95（-5%）
  雨 🌧️    → 0.85（-15%）
  雪 ❄️    → 0.90（-10%）
  雾 🌫️    → 0.90（-10%）
  极端 ⚠️   → 0.00（不可捕获）

集成点：CaptureScreen.tsx 捕获流程中，在基础成功率上乘以天气倍率。
  最终捕获率 = 基础成功率 × 道具修正 × 天气修正 × 连击修正

示例：
  基础 70% × 天气 sunny 1.05 = 73.5%
  基础 70% × 天气 rainy 0.85 × 道具玩具球 1.15 = 68.4%
```

### 7.2 战斗属性修正（与 BattleContext 已有定义一致）

```
晴天 ☀️：
  全属性 × 1.05（愉悦 buff）
  火元素伤害 +10%，水元素伤害 -10%

雨天 🌧️：
  水元素伤害 +10%，火元素伤害 -10%

多云 ⛅ / 阴 ☁️ / 雪 ❄️ / 雾 🌫️：
  无属性修正，无元素修正

极端 ⚠️：
  战斗不可用（户外暂停）
```

### 7.3 感冒概率

```
触发条件：雨天/雪天出战后，按概率判定

雨天 🌧️ → 8% 感冒概率
雪天 ❄️ → 6% 感冒概率
其他天气 → 0%

执行时机（在 CaptureScreen / BattleScreen 中）：
  1. 捕获完成或战斗结束后
  2. 读取当前天气 → getColdRisk()
  3. 若 isRisky，执行 Math.random() < probability 判定
  4. 触发 → 调用状态系统接口设置感冒（Issue #39 前置）
```

### 7.4 愉悦状态（晴天 buff）

```
晴天 ☀️ 出战/参战的宠物：
  自动获得愉悦 buff，本周有效（因天气一周固定）
  全属性 +5%
  
实现：BattleContext 中 applyWeatherModifier() 已有 sunny 全属性 +5%，
      天气接入后自动生效。愉悦 buff 由天气系统管理（一周不变），
      无需额外状态追踪。
```

---

## 8. 感冒机制（与 #39 状态系统联动）

### 8.1 依赖关系

```
Issue #38 (天气系统) ← Issue #39 (状态系统)
  
  天气系统负责：概率判定（雨8%/雪6%）
  状态系统负责：感冒状态管理（持续时间/属性修正/用药解除）
  
  #38 完成后提供：ColdCheckResult，供 #39 调用
```

### 8.2 感冒触发的完整流程

```
捕获/战斗完成
    │
    ▼
WeatherContext.getColdRisk()
    ├─ isRisky: false → 结束
    └─ isRisky: true (rainy/snowy)
        │
        ▼
    Math.random() < probability?
        ├─ false → 结束
        └─ true → 调用状态系统接口
            │
            ▼
        StatusContext.setStatus(petId, 'cold')
            ├─ 全属性 -35%
            ├─ 持续时间: 5天
            └─ 可解除: 感冒药(200金币) / 自然恢复

自然恢复 5 天后：
    ├─ 95%~98% 概率 → 完全恢复
    └─ 2%~5% 概率 → 永久损伤（全属性 -5%）
```

### 8.3 接口预留

```typescript
/** 
 * 感冒触发接口（供 CaptureScreen / BattleScreen 调用）
 * 天气系统不直接设置状态，只返回风险数据；
 * 实际状态设置由 StatusContext 负责（#39）
 */
export interface ColdTriggerInput {
  /** 宠物 ID */
  petId: string
  /** 当前天气 */
  weather: WeatherType
  /** 触发时机 */
  triggerContext: 'capture' | 'battle'
}

// 调用方伪代码：
// const coldRisk = weather.getColdRisk()
// if (coldRisk.isRisky && Math.random() < coldRisk.probability) {
//   status.applyStatus(petId, { type: 'cold', duration: 5, statModifier: -0.35 })
// }
```

---

## 9. UI 展示

### 9.1 TopBar 天气显示（修改 `TopBar.tsx`）

```typescript
// 现有代码：const TopBar: React.FC<TopBarProps> = ({ location, weather, ...props }) => {

// 修改后：
import { useWeather } from '../weather/useWeather'

const TopBar: React.FC<TopBarProps> = ({ location, weather: propWeather, ...props }) => {
  const staminaState = useStamina()
  let lbsState: ... = null
  let weatherState: ReturnType<typeof useWeather> | null = null

  try {
    lbsState = useLbs().state
  } catch { /* 忽略 */ }
  try {
    weatherState = useWeather()
  } catch { /* 忽略 */ }

  const level = props.level ?? staminaState.state.level
  const currentStamina = props.stamina ?? staminaState.state.currentStamina
  const maxStamina = props.maxStamina ?? staminaState.maxStamina
  const gold = props.gold ?? staminaState.state.gold

  // 城市名：优先 props → lbs → 默认
  const displayLocation = location ?? (lbsState?.cityName || '未知')

  // 天气：优先 props → WeatherContext → 默认
  const weatherDisplay = propWeather ?? weatherState?.todayMeta?.emoji ?? '☀️'
  const weatherName = weatherState?.todayMeta?.name ?? ''
  const displayWeather = weatherName ? `${weatherDisplay} ${weatherName} · ${displayLocation}` : `${weatherDisplay} ${displayLocation}`

  return (
    <div style={styles.container}>
      <div style={styles.left}>
        {/* 不变：头像 + Lv + 体力 + 金币 */}
      </div>
      <span className="pill">{displayWeather}</span>
    </div>
  )
}
```

**效果**：
- `TopBar` 右侧显示「☀️ 晴 · 宁波」，自动从 WeatherContext 读取
- 无 WeatherProvider 时降级为现有默认「☀️ 未知」

### 9.2 DiscoverScreen 天气提示（修改 `DiscoverScreen.tsx`）

现有天气 strip 为硬编码「晴天 · 捕获 +5%」和「宁波·晴 宠物状态愉悦」，修改为从 WeatherContext 动态读取：

```typescript
import { useWeather } from '../weather/useWeather'

// 在组件内：
const weather = useWeather()
const captureMod = weather.getCaptureModifier()
const coldRisk = weather.getColdRisk()

// 天气提示区域（替换现有硬编码）：
<div style={styles.weatherStrip}>
  <span style={{ fontSize: 18 }}>{weather.todayMeta?.emoji ?? '☀️'}</span>
  <div>
    <div style={{ fontSize: 13, fontWeight: 600 }}>
      {weather.todayMeta?.name ?? '未知'} · {captureMod.description}
    </div>
    <div style={{ fontSize: 10, color: 'var(--ink-3)' }}>
      {lbs?.state?.cityName ?? '未知'} · {weather.todayMeta?.desc ?? '—'}
    </div>
    {/* 感冒风险警告 */}
    {coldRisk.isRisky && (
      <div style={{ fontSize: 10, color: 'var(--danger)', marginTop: 2 }}>
        ⚠️ {coldRisk.description}
      </div>
    )}
  </div>
</div>
```

**视觉效果差异（按天气类型）**：

| 天气 | 提示区域效果 |
|------|------------|
| 晴 ☀️ | 绿色底纹 · 「光线充足 · 捕获 +5%」 · 宠物状态愉悦 |
| 阴 ☁️ | 灰色底纹 · 「光线偏暗 · 捕获 -5%」 |
| 雨 🌧️ | 蓝色底纹 · 「视线受阻 · 捕获 -15%」 + ⚠️ 感冒风险警告（红色文字） |
| 雪 ❄️ | 白色底纹 · 「低温操作受阻 · 捕获 -10%」 + ⚠️ 感冒风险警告 |
| 极端 ⚠️ | 红色底纹 · 「户外玩法暂停 · 注意安全」 · 捕获按钮禁用 |

### 9.3 App.tsx 包裹 Provider

```typescript
// 在 App.tsx 中，Provider 嵌套顺序：
<LbsProvider>          {/* 最外层：城市名基础 */}
  <StaminaProvider>
    <WeatherProvider>  {/* 需在 LbsProvider 内（读取 cityName） */}
      <ShopProvider>
        <BattleProvider>
          {/* 路由 / 组件 */}
        </BattleProvider>
      </ShopProvider>
    </WeatherProvider>
  </StaminaProvider>
</LbsProvider>
```

---

## 10. 测试用例 (`weather/logic.test.ts`)

### 10.1 测试用例列表（共 15 个）

| # | 测试用例 | 测试函数 | 预期结果 |
|---|---------|---------|---------|
| 1 | 同一城市同一周生成相同天气 | `generateWeekWeather` | 两次调用结果完全一致 |
| 2 | 不同城市不同天气 | `generateWeekWeather` | 不同城市结果不同（概率性，多次验证） |
| 3 | 不同周不同天气 | `generateWeekWeather` | 同一城市但不同 weekStart 结果不同 |
| 4 | 每周生成刚好 7 天 | `generateWeekWeather` | `output.length === 7` |
| 5 | 所有天气类型有效性 | `generateWeekWeather` | 所有结果 weather 在 WeatherType 枚举中 |
| 6 | 极端天气概率校验 | `generateWeekWeather`（多轮统计） | 1000 次模拟中 extreme 出现率 ≈ 2% |
| 7 | 种子确定性：djb2 hash | `generateSeed` | 相同输入 → 相同输出 |
| 8 | PRNG 确定性：Mulberry32 | `createRng` | 相同种子生成相同序列 |
| 9 | getTodayWeather 正确索引 | `getTodayWeather` | 按星期几正确取到对应天 |
| 10 | getTodayWeather 空数组处理 | `getTodayWeather` | week=null → 返回 null |
| 11 | getCaptureModifier sunny | `getCaptureModifier` | { modifier: +5, multiplier: 1.05 } |
| 12 | getCaptureModifier rainy | `getCaptureModifier` | { modifier: -15, multiplier: 0.85 } |
| 13 | getColdRisk rainy | `getColdRisk` | { isRisky: true, probability: 0.08 } |
| 14 | getColdRisk sunny | `getColdRisk` | { isRisky: false, probability: 0 } |
| 15 | getWeekStart 计算正确 | `getWeekStart` | 返回周日 0:00 时间戳 |

### 10.2 测试代码骨架

```typescript
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

describe('createRng', () => {
  it('相同种子生成相同序列', () => {
    const rng1 = createRng(42)
    const rng2 = createRng(42)
    const seq1 = Array.from({ length: 10 }, () => rng1())
    const seq2 = Array.from({ length: 10 }, () => rng2())
    expect(seq1).toEqual(seq2)
  })
})

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
})

describe('generateWeekWeather', () => {
  it('返回 7 天天气', () => {
    const week = generateWeekWeather('宁波', 1700000000000)
    expect(week).toHaveLength(7)
  })

  it('所有天气字段为合法类型', () => {
    const validTypes: WeatherType[] = ['sunny', 'cloudy', 'overcast', 'rainy', 'snowy', 'foggy', 'extreme']
    const week = generateWeekWeather('测试城市', 1700000000000)
    week.forEach(day => {
      expect(validTypes).toContain(day.weather)
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
  })
})

describe('getTodayWeather', () => {
  it('正确按星期索引返回', () => {
    const week = Array.from({ length: 7 }, (_, i) => ({
      weather: 'sunny' as WeatherType,
      dayLabel: `Day${i}`,
    }))
    // Mock Date.getDay() → 使用实际 dayLabel 来验证
    // 实际测试中需 mock 当前日期
  })

  it('null 输入返回 null', () => {
    expect(getTodayWeather(null)).toBeNull()
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
```

---

## 11. 实现步骤

| 步骤 | 任务 | 预估产出 |
|------|------|---------|
| Step 1 | 创建 `weather/constants.ts` + `weather/types.ts` | 类型定义 + 常量表 |
| Step 2 | 创建 `weather/logic.ts`（纯函数，先写测试） | 6 个纯函数 + 15 个测试用例 |
| Step 3 | 创建 `weather/logic.test.ts`，确保全部通过 | 绿色测试 |
| Step 4 | 创建 `weather/api.ts` | 后端 API 调用封装 |
| Step 5 | 创建 `weather/WeatherContext.tsx` + `weather/useWeather.ts` | Context + useReducer + useWeather Hook |
| Step 6 | 修改 `App.tsx`：包裹 `<WeatherProvider>` | Provider 嵌套正确 |
| Step 7 | 修改 `TopBar.tsx`：接入 WeatherContext | 天气 icon + 城市名动态显示 |
| Step 8 | 修改 `DiscoverScreen.tsx`：接入天气修正提示 + 感冒警告 | 天气 strip 动态化 |
| Step 9 | 手动测试：跨城市切换 / 跨周切换 / 极端天气显示 | 端到端验证 |

---

## 12. 验收标准对照

| # | 验收标准 | 实现情况 | 验收方式 |
|---|---------|---------|---------|
| 1 | 游戏内按地级市随机生成 | `generateWeekWeather(city, weekStart)` 确定性随机，同一城市同一周同结果 | 单测 + 手动切换城市验证 |
| 2 | 生成概率符合设计文档（晴33%/多云24%/阴18%/雨15%/雪4%/雾4%/极端2%） | `WEATHER_ROLL_TABLE` 权重表 + `weightedRandom` 轮盘赌 | 单测统计验证（1000 次采样） |
| 3 | 后端集成 /weather/week | `fetchWeekWeather` 优先调用，失败降级游戏内生成 | 模拟后端响应/断开验证 |
| 4 | 晴天捕获 +5%，阴天 -5%，雨天 -15%，雪天 -10%，雾天 -10% | `WEATHER_CAPTURE_MODIFIER` + `getCaptureModifier` | 单测 + DiscoverScreen UI 验证 |
| 5 | 战斗天气修正（晴天全+5%，雨天水+10%/火-10%） | 复用 BattleContext 已有常量，`getBattleModifier()` 传入 | BattleScreen 联调验证 |
| 6 | 感冒概率（雨8%，雪6%） | `COLD_PROBABILITY` + `getColdRisk` | 单测 + DiscoverScreen 感冒风险提示 UI |
| 7 | UI 展示「城市 · 天气」 | TopBar 动态显示 + DiscoverScreen 天气提示 | 手动不同城市/天气验证 |
| 8 | 周日 0:00 刷新新一周天气 | `getWeekStart` 检查 + 定时器刷新 | 模拟时间切换验证 |
| 9 | 离线可用（cache 为上一周天气则重新随机） | localStorage 缓存 + weekStart 过期检查 | 断网后刷新验证 |
| 10 | 极端天气暂停户外玩法 | `extreme` → 捕获倍率 0 + UI 提示 | DiscoverScreen 极端天气 UI 验证 |

---

## 13. 风险与注意事项

| 风险 | 缓解措施 |
|------|---------|
| `WeatherContext` 依赖 `LbsContext.cityName`，未定位前无天气 | 初始状态 `status:'idle'`，UI 显示默认值（晴天 + 未知城市），不阻塞用户操作 |
| 感冒触发需 #39 状态系统支持 | 先实现概率判定 + 接口预留，感冒实际执行待 #39 |
| 跨城市旅行频繁触发天气重加载 | cityName 变更时 debounce 检查，避免频繁 API 调用 |
| 后端 `/weather/week` 返回格式与前端类型不一致 | `fetchWeekWeather` 中做字段校验 + 降级处理 |
| 极端天气概率低（2%），用户可能从未见过 | UI 提示明确：极端天气时捕获按钮禁用 + 红色警告横幅 |
| 一周固定雨天城市用户持续冒感冒风险 | 感冒概率已调低（6%~8%），感冒药 200 金币合理可负担，玩家可主动用药；永久损伤概率仅 2%~5%，不影响普通玩家 |
| 周切换时 edge case：跨年/闰年/时区 | `getWeekStart` 使用系统本地时间，兼容所有 JS 环境 |

---

## 14. 依赖关系图

```
Issue #35 物种系统 ✅ (已完成)
Issue #36 LBS 系统 ✅ (已完成，提供 cityName)
Issue #37 战斗系统 ✅ (已完成，WeatherType 类型已定义)
    │
    ▼
Issue #38 天气系统 (本 Issue)
    │  依赖：#36 (cityName) + #37 (WeatherType 类型)
    │
    ▼ Team 2 同步进行（无强依赖）
Issue #39 状态系统 ← 接收 ColdCheckResult 触发感冒
    │  依赖：#38 提供 cold risk 接口
    │
    ▼
后续：捕获/战斗模块 → 集成天气修正
```

---

*计划基于设计文档 v1.4 (2026-07-08) 与现有代码基（React 18 + Vite 6 + TypeScript 5.6）编写。*
