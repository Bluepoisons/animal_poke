# [M2] LBS 地图系统实现计划 — Issue #36

> **验收标准**：位置展示 + 发现点刷新
>
> **设计文档来源**：`游戏开发计划.md` 3.1 核心循环（LBS 探索）、3.2 发现机制、6.2 LBS 地图与区域排行、4.6.2 后端端点 `/geo/city`
>
> **技术栈**：React 18 + Vite 6 + TypeScript 5.6 + vitest，无额外依赖
>
> **现有基础**：`MapScreen.tsx`（手绘风格地图，显示已捕获标记 + 点击弹照片浮卡）、`useAnimalStore` hook（IndexedDB CRUD）、`StaminaContext`（体力/等级/金币）、后端 `/api/v1/geo/city` 代理腾讯地图逆地理 API（当前 mock）、Vite proxy 已配置 `/api` → `localhost:8080`

---

## 1. 地理定位方案

### 1.1 技术选型：navigator.geolocation

MVP 阶段使用浏览器原生 `navigator.geolocation` API，零依赖，满足 KISS 原则。

```typescript
// 定位配置参数
const GEOLOCATION_OPTIONS: PositionOptions = {
  enableHighAccuracy: true,   // 高精度模式（GPS）
  timeout: 10_000,            // 超时 10 秒
  maximumAge: 30_000,         // 缓存 30 秒内的位置
}
```

### 1.2 权限请求流程

```
用户进入地图 → 检查 navigator.geolocation 是否存在
  ├─ 不存在 → 降级模式（显示已捕获标记，无玩家位置）
  └─ 存在 → 调用 getCurrentPosition()
       ├─ 成功 → 获取 {lat, lng, accuracy} → 请求后端 /geo/city 获取城市名
       ├─ 权限拒绝 (code=1) → 降级模式 + 提示"位置权限被拒绝"
       ├─ 位置不可用 (code=2) → 降级模式 + 提示"无法获取位置"
       └─ 超时 (code=3) → 降级模式 + 提示"定位超时"
```

### 1.3 失败降级策略

| 场景 | 降级行为 | UI 提示 |
|------|---------|---------|
| 浏览器不支持 geolocation | 仅展示已捕获标记，不显示玩家位置 | "当前设备不支持定位" |
| 权限被拒绝 | 同上 | "位置权限被拒绝，无法显示发现点" + [重新授权] 按钮 |
| 定位超时 | 使用上次缓存位置（如有），否则同上 | "定位超时，请重试" + [重试] 按钮 |
| 后端 /geo/city 失败 | 城市名显示"未知"，地图功能正常 | 无（静默降级） |

**降级核心原则**：定位失败不阻塞地图查看功能，玩家仍可浏览已捕获地点标记。

### 1.4 位置更新策略

- **进入地图时**：调用 `getCurrentPosition()` 获取一次位置
- **地图打开期间**：每 30 秒调用 `getCurrentPosition()` 刷新位置（非 `watchPosition`，避免高频回调导致重渲染）
- **页面可见性恢复**：`visibilitychange` 事件触发时立即刷新一次（复用 StaminaContext 的模式）
- **位置缓存**：最近一次有效位置存入 localStorage，作为下次进入时的初始值

---

## 2. 地图渲染方案

### 2.1 决策：保留手绘风格

**理由**：
1. 当前 `MapScreen.tsx` 已有完整的手绘风格实现（森林/河流/山丘装饰 + 区域标签 + 路径线）
2. MVP 阶段验证核心玩法（位置展示 + 发现点），不需要真实地图瓦片
3. 真实地图瓦片需要引入腾讯地图 JS SDK 或 Leaflet，增加依赖和复杂度
4. 设计文档未要求真实地图，仅要求"位置展示 + 发现点刷新"

**后续升级路径**（非本 Issue 范围）：内测阶段可引入腾讯地图 JS SDK，将手绘 canvas 替换为真实瓦片底图，标记层逻辑不变。

### 2.2 MapScreen 改造要点

当前 `MapScreen` 从 `entries`（已捕获动物）计算标记位置。改造后需同时展示：

| 标记类型 | 图标 | 来源 | 交互 |
|---------|------|------|------|
| 玩家位置 | 🧑 蓝色圆点 + 精度圈 | geolocation | 不可点击 |
| 已捕获地点 | 🐾 稀有度色 pin（现有） | useAnimalStore | 点击弹照片浮卡（现有） |
| 发现点（可探索） | ✨ 闪烁动画 pin | useDiscoveryPoints | 点击弹发现信息卡 + [前往捕获] 按钮 |

### 2.3 坐标投影方案

当前 `useCalcPositions` 基于所有 entries 的 lat/lng 范围做线性映射。改造为以玩家位置为中心的相对投影：

```typescript
// 以玩家位置为中心，半径 R 米范围内的坐标投影到 0~100% 画布
// 投影公式：使用等距方位投影（equirectangular）近似
// 1 度纬度 ≈ 111000 米，1 度经度 ≈ 111000 × cos(lat) 米
function projectToCanvas(
  point: { lat: number; lng: number },
  center: { lat: number; lng: number },
  radiusMeters: number,
): { x: number; y: number } | null {
  const latPerMeter = 1 / 111000
  const lngPerMeter = 1 / (111000 * Math.cos(center.lat * Math.PI / 180))
  const dLatMeters = (point.lat - center.lat) / latPerMeter
  const dLngMeters = (point.lng - center.lng) / lngPerMeter
  const distMeters = Math.hypot(dLatMeters, dLngMeters)
  if (distMeters > radiusMeters) return null // 超出显示范围
  const halfRange = radiusMeters
  const x = 50 + (dLngMeters / halfRange) * 45 // 留 5% 边距
  const y = 50 - (dLatMeters / halfRange) * 45
  return { x, y }
}
```

**注意**：当玩家位置不可用时，回退到当前的"全量 entries 范围映射"模式。

### 2.4 视觉设计补充

- **玩家精度圈**：半透明蓝色圆圈，半径 = accuracy 值在画布上的投影大小
- **发现点闪烁**：CSS animation `pulse` 2s infinite，吸引玩家注意
- **距离标签**：每个发现点下方显示距离玩家多少米（如 "230m"）
- **刷新倒计时**：地图顶部显示"下次刷新 04:32"倒计时

---

## 3. 发现点系统

### 3.1 发现点数据结构

```typescript
/** 发现点：玩家附近可探索的动物出现点 */
export interface DiscoveryPoint {
  /** 唯一 ID */
  id: string
  /** 纬度 */
  lat: number
  /** 经度 */
  lng: number
  /** 物种（猫/鹅/狗） */
  species: Species
  /** 稀有度（决定发现点视觉颜色） */
  rarity: RarityTier
  /** 生成时间戳 */
  spawnedAt: number
  /** 过期时间戳 */
  expiresAt: number
  /** 状态：可发现 / 已进入捕获范围 / 已过期 */
  status: 'available' | 'in_range' | 'expired'
}
```

### 3.2 物种与稀有度生成

基于设计文档 5.1 稀有度体系：

```typescript
/** 稀有度掉率表（设计文档 5.1） */
export const RARITY_SPAWN_RATES: Record<RarityTier, number> = {
  common: 0.60,     // 60%
  uncommon: 0.25,   // 25%
  rare: 0.10,       // 10%
  epic: 0.04,       // 4%
  legendary: 0.01,  // 1%
}

/** MVP 阶段物种池（设计文档首期支持猫/鹅/狗） */
export const SPECIES_POOL: Species[] = ['cat', 'goose', 'dog']

/**
 * 按掉率随机生成稀有度（纯函数，可单测）
 */
export function rollRarity(rand: number = Math.random()): RarityTier {
  let acc = 0
  for (const tier of ['common', 'uncommon', 'rare', 'epic', 'legendary'] as RarityTier[]) {
    acc += RARITY_SPAWN_RATES[tier]
    if (rand < acc) return tier
  }
  return 'common' // 兜底
}

/**
 * 随机选择物种（纯函数，可单测）
 */
export function rollSpecies(rand: number = Math.random()): Species {
  const idx = Math.floor(rand * SPECIES_POOL.length)
  return SPECIES_POOL[idx]
}
```

### 3.3 发现点位置生成

在玩家位置周围 50~500 米环带内随机生成，避免太近（无探索感）或太远（超出显示范围）：

```typescript
/**
 * 在玩家附近生成随机坐标（纯函数，可单测）
 * @param center 玩家位置
 * @param minMeters 最小距离（默认 50m）
 * @param maxMeters 最大距离（默认 500m）
 * @param rand 随机数（可注入用于测试）
 */
export function generateRandomPosition(
  center: { lat: number; lng: number },
  minMeters: number = 50,
  maxMeters: number = 500,
  rand: number = Math.random(),
  angleRand: number = Math.random(),
): { lat: number; lng: number } {
  // 随机距离（minMeters ~ maxMeters）
  const distance = minMeters + rand * (maxMeters - minMeters)
  // 随机角度（0 ~ 2π）
  const angle = angleRand * Math.PI * 2
  // 转换为经纬度偏移
  const latPerMeter = 1 / 111000
  const lngPerMeter = 1 / (111000 * Math.cos(center.lat * Math.PI / 180))
  const dLat = distance * Math.cos(angle) * latPerMeter
  const dLng = distance * Math.sin(angle) * lngPerMeter
  return { lat: center.lat + dLat, lng: center.lng + dLng }
}
```

### 3.4 发现点数量

| 参数 | 值 | 说明 |
|------|------|------|
| 每次刷新生成数量 | 5~8 个 | 随机，保证地图有内容但不拥挤 |
| 同时存在上限 | 8 个 | 超出则最旧的先过期 |
| 过期时间 | 10 分钟 | 生成后 10 分钟自动消失 |

---

## 4. 发现点刷新机制

### 4.1 刷新参数

| 参数 | 值 | 说明 |
|------|------|------|
| 刷新间隔 | 5 分钟 | 每 5 分钟刷新一批发现点 |
| 刷新范围 | 玩家位置 500 米内 | 仅刷新玩家附近的发现点 |
| 距离判定阈值 | 50 米 | 玩家距发现点 ≤ 50m 时标记为 `in_range` |

### 4.2 刷新流程

```
定时器每 5 分钟触发 / 手动刷新按钮
  │
  ▼
检查玩家位置是否可用
  ├─ 不可用 → 跳过刷新，保留现有发现点
  └─ 可用 → 清除已过期发现点（expiresAt < now）
       │
       ▼
       生成新发现点（补满至 5~8 个）
       │
       ▼
       计算每个发现点与玩家的距离
       │
       ▼
       更新 status：dist ≤ 50m → 'in_range'，否则 'available'
       │
       ▼
       触发 UI 重渲染
```

### 4.3 距离计算

```typescript
/**
 * 计算两点间距离（Haversine 公式简化版，MVP 用平面近似足够）
 */
export function calculateDistance(
  a: { lat: number; lng: number },
  b: { lat: number; lng: number },
): number {
  const latPerMeter = 1 / 111000
  const lngPerMeter = 1 / (111000 * Math.cos(a.lat * Math.PI / 180))
  const dLat = (a.lat - b.lat) / latPerMeter
  const dLng = (a.lng - b.lng) / lngPerMeter
  return Math.round(Math.hypot(dLat, dLng))
}
```

### 4.4 in_range 发现点与捕获流程联动

```
玩家移动 → 距离 ≤ 50m → 发现点 status 变为 'in_range'
  │
  ▼
发现点显示 "可捕获" 高亮 + [前往捕获] 按钮
  │
  ▼
玩家点击 [前往捕获] → 传递 discoveryPoint 信息 → 切换到 DiscoverScreen（相机取景）
  │
  ▼
DiscoverScreen 拍照确认 → CaptureScreen → 捕获成功 → 写入 IndexedDB + 体力结算
  │
  ▼
捕获成功后移除该发现点（从列表中删除）
```

**注意**：发现点仅提供位置引导和物种/稀有度预览，不影响实际捕获流程。捕获仍走现有 DiscoverScreen → CaptureScreen 链路。

### 4.5 诱饵道具联动（预留）

设计文档 6.4 诱饵道具"提升 30 分钟内稀有动物出现率"。MVP 阶段预留接口：

```typescript
/** 诱饵激活时，稀有度 roll 加成（epic/legendary 概率提升） */
export function rollRarityWithBait(baitActive: boolean, rand: number = Math.random()): RarityTier {
  if (!baitActive) return rollRarity(rand)
  // 诱饵激活时：epic +5%, legendary +2%，从 common/uncommon 中扣除
  const adjustedRates: Record<RarityTier, number> = {
    common: 0.53,      // 60% - 7%
    uncommon: 0.25,    // 不变
    rare: 0.12,        // 10% + 2%
    epic: 0.09,        // 4% + 5%
    legendary: 0.03,   // 1% + 2%
  }
  // ... 按 adjustedRates roll
}
```

> 诱饵联动为预留设计，MVP 阶段不实现，仅保留函数签名和注释。

---

## 5. 与现有系统集成

### 5.1 架构决策：新建 LbsContext，独立于 StaminaContext

**为什么独立 Context？**
- LBS 状态（玩家位置、发现点列表、刷新计时器）与体力/金币系统关注点完全不同
- 避免 StaminaContext 继续膨胀（已有 8 个 Action 类型）
- 发现点是临时状态（非持久化），与 StaminaContext 的持久化模式不同
- 便于测试和后续替换（如未来接入服务端下发发现点）

### 5.2 与 useAnimalStore 集成

| 集成点 | 方向 | 说明 |
|--------|------|------|
| 已捕获标记 | useAnimalStore → LbsContext | MapScreen 读取 `animals` 数组，投影到画布上显示 🐾 pin |
| 捕获成功后移除发现点 | LbsContext → useAnimalStore | 捕获成功 → 从发现点列表移除该点 + 调用 `addAnimal(entry)` 写入 DB |

### 5.3 与 StaminaContext 集成

| 集成点 | 方向 | 说明 |
|--------|------|------|
| 体力检查 | StaminaContext → LbsContext | 进入捕获前检查体力是否 ≥ 20，不足时提示"体力不足" |
| 捕获结算 | LbsContext → StaminaContext | 捕获成功后调用 `addCapture(1)` + `addGold(drop)` |
| 城市信息 | LbsContext → TopBar | 获取到城市名后更新 TopBar 显示（当前硬编码"宁波·晴"） |

### 5.4 与后端 /geo/city 集成

```typescript
/** 调用后端逆地理编码获取城市名 */
async function fetchCityName(lat: number, lng: number): Promise<GeoCityResponse> {
  const resp = await fetch(`/api/v1/geo/city?lat=${lat}&lng=${lng}`)
  if (!resp.ok) {
    throw new Error(`geo/city 请求失败: ${resp.status}`)
  }
  return resp.json()
}

/** 后端返回结构（对应 GeoCityResult） */
interface GeoCityResponse {
  city: string
  district: string
  province: string
  cached: boolean
}
```

### 5.5 Provider 嵌套关系

```
StaminaProvider (金币/体力/等级)
  └── ShopProvider (道具背包/签到)
        └── LbsProvider (定位/发现点/刷新)
              └── App 内容 (MapScreen / DiscoverScreen / ...)
```

LbsProvider 在最内层，因为它需要读取 StaminaContext（体力检查）但不被其他系统依赖。

---

## 6. 文件结构

```
frontend/src/
├── lbs/
│   ├── constants.ts           # LBS 常量：刷新间隔、距离阈值、掉率表、物种池
│   ├── types.ts               # DiscoveryPoint / Species / GeoLocation / LbsState / LbsAction 类型
│   ├── logic.ts               # 纯函数：rollRarity / rollSpecies / generateRandomPosition / calculateDistance / projectToCanvas / shouldRefresh / filterExpired 等
│   ├── logic.test.ts          # 纯函数单元测试（≥15 个用例）
│   ├── LbsContext.tsx         # React Context + useReducer provider，管理定位状态 + 发现点列表 + 刷新计时器
│   └── useLbs.ts              # 自定义 Hook：封装 Context 消费，暴露给组件使用的 API
├── components/
│   ├── MapScreen.tsx          # （修改）增加玩家位置标记 + 发现点标记 + 距离标签 + 刷新倒计时
│   └── DiscoveryCard.tsx      # 发现点信息浮卡：物种/稀有度/距离 + [前往捕获] 按钮
├── App.tsx                    # （修改）包裹 <LbsProvider>，MapScreen 接入 useLbs 数据
└── components/
    └── TopBar.tsx             # （修改）从 useLbs 读取城市名（替换硬编码 "宁波·晴"）
```

### 各文件职责

| 文件 | 职责 | 依赖 |
|------|------|------|
| `lbs/constants.ts` | 刷新间隔 `REFRESH_INTERVAL_MS`、距离阈值 `DISCOVERY_RANGE_M` / `CAPTURE_RANGE_M`、掉率表 `RARITY_SPAWN_RATES`、物种池 `SPECIES_POOL`、过期时间 `DISCOVERY_TTL_MS`、localStorage key。纯数据 | 无 |
| `lbs/types.ts` | `Species` / `RarityTier`（复用 types.ts）/ `GeoLocation` / `DiscoveryPoint` / `LbsState` / `LbsAction` / `LbsContextValue`。纯类型 | 无 |
| `lbs/logic.ts` | 纯函数：`rollRarity` / `rollSpecies` / `generateRandomPosition` / `calculateDistance` / `projectToCanvas` / `shouldRefresh` / `filterExpired` / `generateDiscoveryPoints` / `updatePointStatus`。不依赖 React | constants, types |
| `lbs/logic.test.ts` | 所有纯函数的单元测试 | logic, constants |
| `lbs/LbsContext.tsx` | LBS 状态管理核心。创建 Context + Reducer，管理玩家位置、城市名、发现点列表、刷新计时器、定位状态。读写 localStorage（缓存位置） | constants, types, logic |
| `lbs/useLbs.ts` | 对外暴露的 Hook。封装 `useContext(LbsContext)`，处理 null 检查 | LbsContext, types |
| `components/MapScreen.tsx`（修改） | 改造现有手绘地图：增加玩家位置标记 + 精度圈 + 发现点 pin + 距离标签 + 刷新倒计时 + 降级提示 | useLbs, useAnimalStore, RARITY_COLORS, DiscoveryCard |
| `components/DiscoveryCard.tsx` | 发现点点击后弹出的信息卡：物种 emoji + 稀有度名称 + 距离 + [前往捕获] 按钮 | useLbs, useStamina, types |

---

## 7. 类型定义 (`types.ts`)

```typescript
import type { RarityTier } from '../types'

/** 物种枚举 */
export type Species = 'cat' | 'goose' | 'dog'

/** 地理位置（纬度 + 经度 + 精度） */
export interface GeoLocation {
  lat: number
  lng: number
  /** 定位精度（米），可选 */
  accuracy?: number
}

/** 发现点：玩家附近可探索的动物出现点 */
export interface DiscoveryPoint {
  /** 唯一 ID（UUID 或时间戳+随机数） */
  id: string
  /** 纬度 */
  lat: number
  /** 经度 */
  lng: number
  /** 物种 */
  species: Species
  /** 稀有度 */
  rarity: RarityTier
  /** 生成时间戳（Unix ms） */
  spawnedAt: number
  /** 过期时间戳（Unix ms） */
  expiresAt: number
  /** 状态 */
  status: 'available' | 'in_range' | 'expired'
}

/** LBS 系统完整状态 */
export interface LbsState {
  /** 定位状态 */
  geoStatus: 'idle' | 'locating' | 'located' | 'denied' | 'unsupported' | 'timeout'
  /** 玩家位置（null = 未获取） */
  playerLocation: GeoLocation | null
  /** 城市名（后端逆地理返回） */
  cityName: string
  /** 省份名 */
  provinceName: string
  /** 发现点列表 */
  discoveryPoints: DiscoveryPoint[]
  /** 上次刷新时间戳 */
  lastRefreshTime: number
  /** 错误信息（定位失败时） */
  errorMsg: string
}

/** Reducer Action 类型 */
export type LbsAction =
  | { type: 'GEO_START' }
  | { type: 'GEO_SUCCESS'; location: GeoLocation }
  | { type: 'GEO_FAIL'; status: 'denied' | 'unsupported' | 'timeout'; errorMsg: string }
  | { type: 'GEO_CITY_SUCCESS'; city: string; province: string }
  | { type: 'GEO_CITY_FAIL' }
  | { type: 'REFRESH_POINTS'; points: DiscoveryPoint[]; now: number }
  | { type: 'UPDATE_POINT_STATUS'; updates: { id: string; status: DiscoveryPoint['status'] }[] }
  | { type: 'REMOVE_POINT'; id: string }
  | { type: 'CLEAR_EXPIRED'; now: number }
  | { type: 'LOAD_STATE'; state: Partial<LbsState> }

/** LbsContext 暴露给组件的接口 */
export interface LbsContextValue {
  state: LbsState
  /** 请求定位（调用 navigator.geolocation） */
  requestLocation: () => void
  /** 手动刷新发现点 */
  refreshPoints: () => void
  /** 移除发现点（捕获成功后调用） */
  removePoint: (id: string) => void
  /** 获取玩家附近的 in_range 发现点 */
  getInRangePoints: () => DiscoveryPoint[]
  /** 距离下次刷新的秒数 */
  nextRefreshIn: number
}
```

---

## 8. 纯函数设计 (`logic.ts`)

### 8.1 函数清单

| 函数 | 签名 | 说明 |
|------|------|------|
| `rollRarity` | `(rand?: number) => RarityTier` | 按掉率表随机生成稀有度 |
| `rollSpecies` | `(rand?: number) => Species` | 随机选择物种 |
| `generateRandomPosition` | `(center, minMeters?, maxMeters?, rand?, angleRand?) => GeoLocation` | 在玩家附近环带随机生成坐标 |
| `calculateDistance` | `(a, b) => number` | 计算两点间距离（米） |
| `projectToCanvas` | `(point, center, radiusMeters) => {x, y} \| null` | 将 GPS 坐标投影到画布百分比坐标 |
| `shouldRefresh` | `(lastRefreshTime, now, intervalMs) => boolean` | 判断是否到了刷新时间 |
| `filterExpired` | `(points, now) => DiscoveryPoint[]` | 过滤掉已过期的发现点 |
| `generateDiscoveryPoints` | `(center, count, now, rand?) => DiscoveryPoint[]` | 批量生成发现点 |
| `updatePointStatus` | `(points, playerLocation, rangeMeters) => DiscoveryPoint[]` | 根据玩家位置更新发现点状态 |
| `createPointId` | `() => string` | 生成发现点唯一 ID |

### 8.2 核心函数实现

```typescript
/**
 * 批量生成发现点（纯函数）
 * @param center 玩家位置
 * @param count 生成数量
 * @param now 当前时间戳
 * @param rand 随机数生成器（可注入用于测试）
 */
export function generateDiscoveryPoints(
  center: GeoLocation,
  count: number,
  now: number,
  rand: () => number = Math.random,
): DiscoveryPoint[] {
  const points: DiscoveryPoint[] = []
  for (let i = 0; i < count; i++) {
    const pos = generateRandomPosition(center, 50, 500, rand(), rand())
    const rarity = rollRarity(rand())
    const species = rollSpecies(rand())
    points.push({
      id: createPointId(),
      lat: pos.lat,
      lng: pos.lng,
      species,
      rarity,
      spawnedAt: now,
      expiresAt: now + DISCOVERY_TTL_MS,
      status: 'available',
    })
  }
  return points
}

/**
 * 根据玩家位置更新发现点状态
 * 距离 ≤ CAPTURE_RANGE_M → 'in_range'，否则 'available'
 */
export function updatePointStatus(
  points: DiscoveryPoint[],
  playerLocation: GeoLocation,
  rangeMeters: number = CAPTURE_RANGE_M,
): DiscoveryPoint[] {
  return points.map(p => {
    const dist = calculateDistance(
      { lat: p.lat, lng: p.lng },
      playerLocation,
    )
    return {
      ...p,
      status: dist <= rangeMeters ? 'in_range' : 'available',
    }
  })
}
```

---

## 9. LbsContext 核心逻辑 (`LbsContext.tsx`)

### 9.1 Provider 结构

```typescript
export const LbsContext = createContext<LbsContextValue | null>(null)

export const LbsProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(lbsReducer, undefined, loadInitialState)

  // 定位请求
  const requestLocation = useCallback(() => {
    if (!navigator.geolocation) {
      dispatch({ type: 'GEO_FAIL', status: 'unsupported', errorMsg: '设备不支持定位' })
      return
    }
    dispatch({ type: 'GEO_START' })
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        dispatch({
          type: 'GEO_SUCCESS',
          location: { lat: pos.coords.latitude, lng: pos.coords.longitude, accuracy: pos.coords.accuracy },
        })
        // 获取到位置后请求城市名
        fetchCityName(pos.coords.latitude, pos.coords.longitude)
          .then(res => dispatch({ type: 'GEO_CITY_SUCCESS', city: res.city, province: res.province }))
          .catch(() => dispatch({ type: 'GEO_CITY_FAIL' }))
      },
      (err) => {
        const status = err.code === 1 ? 'denied' : err.code === 3 ? 'timeout' : 'denied'
        dispatch({ type: 'GEO_FAIL', status, errorMsg: err.message })
      },
      GEOLOCATION_OPTIONS,
    )
  }, [])

  // 定时刷新发现点（每 5 分钟）
  useEffect(() => {
    if (state.geoStatus !== 'located' || !state.playerLocation) return
    const interval = setInterval(() => {
      refreshPoints()
    }, REFRESH_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [state.geoStatus, state.playerLocation])

  // 定时清理过期发现点（每 30 秒）
  useEffect(() => {
    const interval = setInterval(() => {
      dispatch({ type: 'CLEAR_EXPIRED', now: Date.now() })
    }, 30_000)
    return () => clearInterval(interval)
  }, [])

  // 页面可见性恢复时刷新位置
  useEffect(() => {
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        requestLocation()
      }
    }
    document.addEventListener('visibilitychange', handleVisibility)
    return () => document.removeEventListener('visibilitychange', handleVisibility)
  }, [requestLocation])

  // 缓存位置到 localStorage
  useEffect(() => {
    if (state.playerLocation) {
      localStorage.setItem(LBS_CACHE_KEY, JSON.stringify({
        playerLocation: state.playerLocation,
        cityName: state.cityName,
      }))
    }
  }, [state.playerLocation, state.cityName])

  const refreshPoints = useCallback(() => {
    if (!state.playerLocation) return
    const now = Date.now()
    const existing = filterExpired(state.discoveryPoints, now)
    const need = Math.max(0, 5 - existing.length) // 至少保持 5 个
    if (need === 0) {
      dispatch({ type: 'REFRESH_POINTS', points: existing, now })
      return
    }
    const newPoints = generateDiscoveryPoints(state.playerLocation, need + 3, now)
    dispatch({ type: 'REFRESH_POINTS', points: [...existing, ...newPoints].slice(0, 8), now })
  }, [state.playerLocation, state.discoveryPoints])

  // ... 暴露 value
  return <LbsContext.Provider value={value}>{children}</LbsContext.Provider>
}
```

### 9.2 状态流转图

```
定位流程:
  进入地图 → requestLocation()
    → GEO_START (geoStatus='locating')
    → getCurrentPosition 成功
      → GEO_SUCCESS (playerLocation 更新)
      → fetchCityName → GEO_CITY_SUCCESS (cityName 更新)
    → getCurrentPosition 失败
      → GEO_FAIL (geoStatus='denied'/'timeout', errorMsg 更新)

刷新流程:
  定时器 5min / 手动刷新 → refreshPoints()
    → 检查 playerLocation 可用
    → filterExpired 清除过期点
    → generateDiscoveryPoints 补充新点
    → REFRESH_POINTS (discoveryPoints 更新, lastRefreshTime 更新)

距离更新:
  玩家位置变化 → updatePointStatus
    → 计算每个发现点与玩家距离
    → ≤ 50m → status='in_range'
    → UPDATE_POINT_STATUS

捕获联动:
  点击 [前往捕获] → 检查体力 ≥ 20
    → 体力不足 → Toast "体力不足"
    → 体力充足 → 传递 discoveryPoint → DiscoverScreen
    → 捕获成功 → removePoint(id) + addAnimal + addCapture + addGold
```

---

## 10. MapScreen 改造

### 10.1 改造范围

| 区域 | 现状 | 改造 |
|------|------|------|
| 数据源 | props.entries（已捕获动物） | + useLbs().state（玩家位置 + 发现点） |
| 坐标投影 | 全量 entries 范围映射 | 以玩家位置为中心的相对投影（降级时回退原逻辑） |
| 标记 | 仅已捕获 🐾 pin | + 玩家 🧑 pin + 发现点 ✨ pin |
| 顶部 | 标题 "🐾 Hunt Map" | + 城市名 + 刷新倒计时 |
| 降级 | 无 | 定位失败时显示提示条 + 重试按钮 |
| 点击 | 弹已捕获照片浮卡 | 发现点点击弹 DiscoveryCard |

### 10.2 发现点标记视觉

```
   ✨ ← 闪烁动画 pin（稀有度色）
  ╱
 250m ← 距离标签（灰色小字）
```

- `available` 状态：闪烁动画，稀有度颜色边框
- `in_range` 状态：放大 + 脉冲光圈 + "可捕获" 标签
- `expired` 状态：灰色半透明（被清理前可能短暂可见）

### 10.3 降级模式 UI

```
┌─────────────────────────────────┐
│  ‹  🐾 Hunt Map · 未知城市      │
├─────────────────────────────────┤
│  ┌───────────────────────────┐  │
│  │  ⚠️ 位置权限被拒绝          │  │  ← 降级提示条
│  │  无法显示发现点 [重新授权]  │  │
│  └───────────────────────────┘  │
│                                 │
│  （手绘地图 + 已捕获标记正常显示）│
│                                 │
└─────────────────────────────────┘
```

---

## 11. 测试用例

### 11.1 测试文件：`lbs/logic.test.ts`

共 18 个测试用例，覆盖所有纯函数：

```typescript
import { describe, it, expect } from 'vitest'
import {
  rollRarity,
  rollSpecies,
  generateRandomPosition,
  calculateDistance,
  projectToCanvas,
  shouldRefresh,
  filterExpired,
  generateDiscoveryPoints,
  updatePointStatus,
  createPointId,
} from './logic'
import { REFRESH_INTERVAL_MS, DISCOVERY_TTL_MS, CAPTURE_RANGE_M } from './constants'

// 测试用固定坐标（宁波市海曙区）
const NINGBO = { lat: 29.87, lng: 121.55 }

describe('rollRarity', () => {
  it('#1 rand=0.0 → common (0% 落在 common 区间起点)', () => {
    expect(rollRarity(0.0)).toBe('common')
  })

  it('#2 rand=0.59 → common (59% 仍在 common 60% 区间内)', () => {
    expect(rollRarity(0.59)).toBe('common')
  })

  it('#3 rand=0.60 → uncommon (刚好超过 common 60% 边界)', () => {
    expect(rollRarity(0.60)).toBe('uncommon')
  })

  it('#4 rand=0.95 → epic (95% 落在 epic 4% 区间 0.85~0.95)', () => {
    expect(rollRarity(0.94)).toBe('epic')
  })

  it('#5 rand=0.99 → legendary (99% 落在 legendary 1% 区间 0.96~1.0)', () => {
    expect(rollRarity(0.99)).toBe('legendary')
  })
})

describe('rollSpecies', () => {
  it('#6 rand=0.0 → cat', () => {
    expect(rollSpecies(0.0)).toBe('cat')
  })

  it('#7 rand=0.5 → goose (中间值)', () => {
    expect(rollSpecies(0.4)).toBe('goose')
  })

  it('#8 rand=0.9 → dog', () => {
    expect(rollSpecies(0.8)).toBe('dog')
  })
})

describe('generateRandomPosition', () => {
  it('#9 生成的坐标在 minMeters~maxMeters 距离范围内', () => {
    const pos = generateRandomPosition(NINGBO, 50, 500, 0.5, 0.0)
    const dist = calculateDistance(NINGBO, pos)
    expect(dist).toBeGreaterThanOrEqual(40) // 容差 10m
    expect(dist).toBeLessThanOrEqual(510)
  })

  it('#10 rand=0, angleRand=0 → 正北方向，距离=minMeters', () => {
    const pos = generateRandomPosition(NINGBO, 100, 500, 0.0, 0.0)
    expect(pos.lat).toBeGreaterThan(NINGBO.lat) // 正北 → 纬度增大
    expect(pos.lng).toBeCloseTo(NINGBO.lng, 5) // 经度几乎不变
  })

  it('#11 rand=1 → 距离=maxMeters', () => {
    const pos = generateRandomPosition(NINGBO, 50, 500, 1.0, 0.0)
    const dist = calculateDistance(NINGBO, pos)
    expect(dist).toBeGreaterThan(450) // 接近 500m
  })
})

describe('calculateDistance', () => {
  it('#12 同一点距离为 0', () => {
    expect(calculateDistance(NINGBO, NINGBO)).toBe(0)
  })

  it('#13 向北 100m 的点距离约 100m', () => {
    const north100m = { lat: NINGBO.lat + 100 / 111000, lng: NINGBO.lng }
    expect(calculateDistance(NINGBO, north100m)).toBe(100)
  })

  it('#14 向东 100m 的点距离约 100m（考虑经度修正）', () => {
    const east100m = { lat: NINGBO.lat, lng: NINGBO.lng + 100 / (111000 * Math.cos(NINGBO.lat * Math.PI / 180)) }
    expect(calculateDistance(NINGBO, east100m)).toBe(100)
  })
})

describe('projectToCanvas', () => {
  it('#15 中心点投影到画布中心 (50, 50)', () => {
    const result = projectToCanvas(NINGBO, NINGBO, 500)
    expect(result).not.toBeNull()
    expect(result!.x).toBeCloseTo(50, 1)
    expect(result!.y).toBeCloseTo(50, 1)
  })

  it('#16 超出半径的点返回 null', () => {
    const farPoint = { lat: NINGBO.lat + 0.01, lng: NINGBO.lng } // 约 1.1km 外
    expect(projectToCanvas(farPoint, NINGBO, 500)).toBeNull()
  })
})

describe('shouldRefresh', () => {
  it('#17 距上次刷新超过间隔 → true', () => {
    const now = Date.now()
    const lastRefresh = now - REFRESH_INTERVAL_MS - 1
    expect(shouldRefresh(lastRefresh, now, REFRESH_INTERVAL_MS)).toBe(true)
  })

  it('#18 距上次刷新未超过间隔 → false', () => {
    const now = Date.now()
    const lastRefresh = now - 1000
    expect(shouldRefresh(lastRefresh, now, REFRESH_INTERVAL_MS)).toBe(false)
  })
})

describe('filterExpired', () => {
  it('#19 过滤掉 expiresAt < now 的发现点', () => {
    const now = Date.now()
    const points = [
      { id: '1', spawnedAt: now - 600000, expiresAt: now - 1000, status: 'available' as const, species: 'cat' as const, rarity: 'common' as const, lat: 29.87, lng: 121.55 },
      { id: '2', spawnedAt: now - 1000, expiresAt: now + 600000, status: 'available' as const, species: 'dog' as const, rarity: 'rare' as const, lat: 29.88, lng: 121.56 },
    ]
    const result = filterExpired(points, now)
    expect(result).toHaveLength(1)
    expect(result[0].id).toBe('2')
  })
})

describe('generateDiscoveryPoints', () => {
  it('#20 生成指定数量的发现点，且所有点在 50~500m 范围内', () => {
    const now = Date.now()
    const points = generateDiscoveryPoints(NINGBO, 5, now, () => 0.5)
    expect(points).toHaveLength(5)
    points.forEach(p => {
      const dist = calculateDistance(NINGBO, { lat: p.lat, lng: p.lng })
      expect(dist).toBeGreaterThanOrEqual(40)
      expect(dist).toBeLessThanOrEqual(510)
      expect(p.expiresAt).toBe(now + DISCOVERY_TTL_MS)
      expect(p.spawnedAt).toBe(now)
      expect(p.status).toBe('available')
    })
  })
})

describe('updatePointStatus', () => {
  it('#21 距离 ≤ CAPTURE_RANGE_M → in_range', () => {
    const now = Date.now()
    const points = [{
      id: '1', spawnedAt: now, expiresAt: now + 600000, status: 'available' as const,
      species: 'cat' as const, rarity: 'common' as const,
      lat: NINGBO.lat + 30 / 111000, lng: NINGBO.lng, // 北方 30m
    }]
    const result = updatePointStatus(points, NINGBO)
    expect(result[0].status).toBe('in_range')
  })

  it('#22 距离 > CAPTURE_RANGE_M → available', () => {
    const now = Date.now()
    const points = [{
      id: '1', spawnedAt: now, expiresAt: now + 600000, status: 'available' as const,
      species: 'cat' as const, rarity: 'common' as const,
      lat: NINGBO.lat + 200 / 111000, lng: NINGBO.lng, // 北方 200m
    }]
    const result = updatePointStatus(points, NINGBO)
    expect(result[0].status).toBe('available')
  })
})

describe('createPointId', () => {
  it('#23 连续调用生成不同 ID', () => {
    const id1 = createPointId()
    const id2 = createPointId()
    expect(id1).not.toBe(id2)
    expect(id1).toBeTruthy()
  })
})
```

### 11.2 测试覆盖总览

| 类别 | 用例数 | 覆盖函数 |
|------|--------|---------|
| rollRarity | 5 | 稀有度掉率生成（边界值 + 各区间） |
| rollSpecies | 3 | 物种随机选择 |
| generateRandomPosition | 3 | 随机坐标生成（范围 + 方向 + 距离） |
| calculateDistance | 3 | 距离计算（同点 + 纬度 + 经度） |
| projectToCanvas | 2 | 画布投影（中心点 + 超出范围） |
| shouldRefresh | 2 | 刷新时机判断 |
| filterExpired | 1 | 过期过滤 |
| generateDiscoveryPoints | 1 | 批量生成（数量 + 范围 + 字段完整性） |
| updatePointStatus | 2 | 状态更新（in_range / available） |
| createPointId | 1 | ID 唯一性 |
| **合计** | **23** | |

---

## 12. 验收标准对照

| 验收标准 | 实现方案 | 验证方式 |
|---------|---------|---------|
| 位置展示 | `navigator.geolocation.getCurrentPosition()` 获取玩家 GPS → 地图中心展示 🧑 标记 + 精度圈 + 城市名 | 手动测试：进入地图 → 授权定位 → 看到玩家标记 |
| 发现点刷新 | `LbsContext` 每 5 分钟自动刷新 + 手动刷新按钮，在玩家 500m 内生成 5~8 个发现点 | 手动测试：等待/点击刷新 → 地图出现新 ✨ 标记 |
| 发现点交互 | 点击发现点 → 弹出 DiscoveryCard（物种/稀有度/距离）→ 距离 ≤ 50m 时 [前往捕获] 可用 | 手动测试：点击发现点 → 查看信息卡 |
| 定位降级 | 权限拒绝/超时/不支持 → 显示提示条 + 重试按钮，地图仍展示已捕获标记 | 手动测试：拒绝定位权限 → 验证降级 UI |
| 与现有系统集成 | useAnimalStore（已捕获标记）+ StaminaContext（体力检查/捕获结算）+ 后端 /geo/city（城市名） | 手动测试：捕获流程完整跑通 |
| 发现点过期 | 10 分钟后自动清除，每 30 秒检查一次 | 单测 #19 |
| 距离计算准确 | 纬度/经度修正后的平面近似距离 | 单测 #12~#14 |
| 稀有度掉率正确 | 60%/25%/10%/4%/1% 五级掉率 | 单测 #1~#5 |

---

## 13. 实现顺序建议

| 步骤 | 内容 | 预估文件 |
|------|------|---------|
| 1 | 创建 `lbs/constants.ts` + `lbs/types.ts` | 2 文件 |
| 2 | 实现 `lbs/logic.ts` 纯函数 | 1 文件 |
| 3 | 编写 `lbs/logic.test.ts` 并全部通过 | 1 文件 |
| 4 | 实现 `lbs/LbsContext.tsx` + `lbs/useLbs.ts` | 2 文件 |
| 5 | 实现 `components/DiscoveryCard.tsx` | 1 文件 |
| 6 | 改造 `components/MapScreen.tsx`：接入 useLbs + 玩家标记 + 发现点标记 + 降级提示 | 1 文件（修改） |
| 7 | 修改 `App.tsx`：包裹 `<LbsProvider>` | 1 文件（修改） |
| 8 | 修改 `components/TopBar.tsx`：从 useLbs 读取城市名 | 1 文件（修改） |
| 9 | 全量测试 `vitest run` + 手动验收 | — |

---

## 14. 注意事项

1. **不使用 watchPosition**：`watchPosition` 回调频率高，会导致频繁重渲染。改用定时 `getCurrentPosition`（每 30 秒），性能可控。后续如需实时追踪可升级。

2. **发现点是临时状态，不持久化**：发现点列表仅存在内存中（Reducer state），不写 localStorage。刷新页面后发现点重置——这是合理行为，因为玩家位置可能已变化。仅缓存玩家位置和城市名到 localStorage。

3. **坐标投影精度**：MVP 使用等距方位投影（equirectangular）近似，在小范围（500m）内误差可忽略。后续接入真实地图瓦片时替换为墨卡托投影。

4. **HTTPS 要求**：`navigator.geolocation` 仅在 HTTPS 或 localhost 下可用。开发环境（localhost:5173）正常，生产部署需 HTTPS。

5. **后端 /geo/city 当前为 mock**：后端 `GeoService.GetCity` 在未配置腾讯地图 Key 时返回空 `{}`。前端需处理 `city=""` 的情况，显示"未知城市"。

6. **发现点不影响实际捕获流程**：发现点仅提供位置引导和物种/稀有度预览。玩家仍需走 DiscoverScreen（拍照）→ CaptureScreen（投掷捕获）完整链路。发现点的物种/稀有度是"预览"，实际捕获后由云端 VLM/LLM 重新生成。

7. **MVP 不实现的部分**：诱饵道具对发现点稀有度的加成（预留函数签名）、服务端下发发现点、真实地图瓦片、watchPosition 实时追踪、区域声望联动 — 这些留给后续 Issue。

8. **代码注释用中文**：与现有代码风格一致（参见 StaminaContext、ShopContext 注释风格）。

9. **物种类型复用**：`Species` 类型定义在 `lbs/types.ts`，不修改全局 `types.ts`（避免影响现有代码）。`RarityTier` 从 `types.ts` 导入复用。

10. **MapScreen 兼容性**：MapScreen 改造后仍需支持"从 CollectScreen 点击地图按钮进入"的场景（props.entries + props.onBack 不变），新增的 LBS 功能通过 useLbs 获取，不影响原有 props 接口。
