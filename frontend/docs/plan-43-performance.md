# [M2] 性能优化（30fps+ · 包体≤150MB）实现计划 — Issue #43

> **验收标准**：中端机 30fps 稳定，包体 ≤ 150MB，启动快，内存合理
>
> **设计文档来源**：`游戏开发计划.md` 2.2 适配要求（性能基线：中端机 ≥ 30fps，高端机 ≥ 60fps；安装包 ≤ 150MB）+ 7.3 内测阶段验收（性能优化：中端机 30fps 稳定，包体 ≤ 150MB）+ 第九章 风险表（UI 卡顿：组件化关键路径 + 虚拟列表防卡顿）
>
> **技术栈**：React 18 + Vite 6 + TypeScript 5.6 + vitest，PWA
>
> **现有基础**：
> - 8 层 Context Provider 嵌套（Lbs → Stamina → Economy → Weather → Shop → Status → Dispatch → Battle）
> - 所有屏幕组件静态 `import`，无代码分割 / 懒加载
> - `vite.config.ts` 仅有 `@vitejs/plugin-react`，无构建优化 / PWA 插件 / 压缩配置
> - 无 Service Worker、无 manifest.json、无 vite-plugin-pwa
> - 相机取景 `getUserMedia` 640×480，`canvas.toDataURL('image/jpeg', 0.9)`
> - IndexedDB 通过 `idb` 库访问，`getUnlocked()` / `countUnlocked()` 全表扫描后 JS 过滤
> - 所有 UI 使用 emoji + CSS 渐变，无图片资源（后续美术资源接入时需压缩）
> - `CollectScreen` 无虚拟列表（当前 9 格 + padding，数据量增大后有卡顿风险）
> - 各 Context 的 `value` useMemo 依赖整个 `state`，任意 state 字段变化导致全部消费者重渲染

---

## 0. 现状审计与问题清单

### 0.1 构建配置审计

| 维度 | 现状 | 问题 |
|------|------|------|
| Vite 插件 | 仅 `@vitejs/plugin-react` | 无 PWA 插件、无压缩插件、无 bundle 分析 |
| 代码分割 | 无 `manualChunks` 配置 | 所有代码打进单个 chunk |
| 懒加载 | 所有组件静态 `import` | 首屏加载全部屏幕代码 |
| 压缩 | Vite 默认 esbuild minify | 未启用 gzip/brotel 预压缩 |
| PWA | 无 SW、无 manifest | 不满足 PWA 要求，无离线缓存 |
| Bundle 分析 | 无 | 无法定位包体瓶颈 |

### 0.2 React 渲染审计

| 维度 | 现状 | 问题 |
|------|------|------|
| Context value | `useMemo(() => ({...}), [state, ...])` | **核心问题**：value 依赖整个 `state` 对象，任意字段变化导致所有消费者重渲染 |
| useCallback 依赖 | `consumeStamina` 依赖 `state.currentStamina` | 每次体力变化重建回调，破坏下游 `React.memo` 优化 |
| React.memo | 无任何屏幕组件使用 `React.memo` | 父组件 `AppInner` state 变化时所有屏幕重渲染 |
| 屏幕卸载 | `renderContent()` switch 返回 | 切换 tab 时旧组件卸载、新组件挂载（无 keep-alive），相机屏每次重挂载 |
| 内联对象 | `CollectScreen` 中 `tabs` / `speciesTabs` 每次渲染重建 | 非关键但可优化 |
| 派生计算 | `CollectScreen` 中 `unlockedCount = animals.filter(...)` 未 memo | 每次渲染 O(n) 扫描 |

### 0.3 Context Provider 嵌套分析

```
LbsProvider          — state: { cityName, lat, lng, ... }
  StaminaProvider    — state: { level, currentStamina, totalCaptures, gold, ... }
    EconomyProvider  — state: { totalEarned, totalSpent, logs[], todayEarned, ... }
      WeatherProvider — state: { week, weekStart, city, source, status, ... }
        ShopProvider — state: { items, checkIn, ... }
          StatusProvider — state: { petStatuses, ... }
            DispatchProvider — state: { dispatches, ... }
              BattleProvider — state: { phase, playerPet, enemyPet, round, log[], ... }
                AppInner
```

**关键问题**：
1. `StaminaContext` 每分钟 TICK 触发 state 变化 → 全部 8 层 Provider 的子树重渲染
2. `BattleContext` 自动战斗时每 `AUTO_PLAY_INTERVAL_MS` 触发 state 变化（含 log 数组更新）→ 整棵树重渲染
3. `BattleProvider` 内部消费了 5 个其他 Context（stamina/shop/economy/status/weather），其 `useCallback` 依赖这些 context 的 value → 任意上游变化都重建 BattleContext 的所有回调
4. `EconomyContext` 的 `logs` 数组每次 trackEarn/trackSpend 都产生新数组 → state 引用变化 → 全部消费者重渲染

### 0.4 IndexedDB 查询审计

| 方法 | 现状 | 问题 |
|------|------|------|
| `getAll()` | `db.getAll('animals')` | 全表加载，数据量大时慢 |
| `getUnlocked()` | `getAll()` → `.filter()` | 全表扫描后 JS 过滤，未利用索引 |
| `countUnlocked()` | `getAll()` → `.filter().length` | 全表扫描仅为计数 |
| `markViewed()` | `get()` → 修改 → `put()` | 两次 IO，可用 cursor 优化 |
| `getByDateRange()` | `getAllFromIndex('by-date', range)` | 已用索引，OK |
| 索引 | `by-date`, `by-rarity` | `unlocked` 是 boolean 不能建索引（现有注释已说明） |

### 0.5 相机性能审计

| 维度 | 现状 | 问题 |
|------|------|------|
| 分辨率 | `width: { ideal: 640 }, height: { ideal: 480 }` | 合理，但部分设备会返回更高分辨率 |
| 拍照质量 | `canvas.toDataURL('image/jpeg', 0.9)` | 0.9 质量偏高，生成 base64 较大 |
| 帧采样 | 无（当前 mock 检测） | 真实 VLM 接入后需 2~5 FPS 自适应采样 |
| 生命周期 | `useEffect` 启动/停止 | OK，但切 tab 时相机未释放（组件卸载才停止） |
| Canvas | 隐藏 canvas 每次拍照才用 | OK |

### 0.6 CSS / 样式审计

| 维度 | 现状 | 问题 |
|------|------|------|
| 全局样式 | `index.css` 142 行 | 量小，无问题 |
| 组件样式 | 大量内联 `style` 对象 | 部分组件（`CollectScreen`）已提取为模块级常量，部分（`DiscoverScreen`）也已提取 |
| CSS 变量 | `:root` 定义主题变量 | OK，无重复定义 |
| 动画 | `@keyframes spin` + `pulse` | 量少，无性能问题 |
| will-change | 无 | 动画元素少，暂不需要 |

---

## 1. 性能审计方法论

### 1.1 测量维度与目标

| 维度 | 指标 | 目标 | 测量工具 |
|------|------|------|---------|
| 帧率 | 所有屏幕平均 FPS | 中端机 ≥ 30fps，高端机 ≥ 60fps | Chrome DevTools Performance / FPS Meter |
| 包体 | 构建产物总大小 | ≤ 150MB（含美术资源） | `vite-bundle-visualizer` / `rollup-plugin-visualizer` |
| 首屏加载 | FCP (First Contentful Paint) | ≤ 1.5s（中端机） | Lighthouse / Chrome DevTools Performance |
| 交互就绪 | TTI (Time to Interactive) | ≤ 3s（中端机） | Lighthouse |
| 内存 | 峰值堆内存 | ≤ 150MB | Chrome DevTools Memory / Performance Monitor |
| 长任务 | > 50ms 的任务 | ≤ 5 个 / 页面切换 | Chrome DevTools Performance |
| Context 渲染 | 不必要重渲染次数 | 0 / 单次交互 | React DevTools Profiler |

### 1.2 测量工具清单

| 工具 | 用途 | 引入方式 |
|------|------|---------|
| `vite-bundle-visualizer` | 包体可视化分析 | devDependency |
| `rollup-plugin-visualizer` | Rollup chunk 分析 | devDependency |
| `vite-plugin-pwa` | PWA Service Worker + manifest | dependency |
| `vite-plugin-compression` | gzip/brotel 预压缩 | devDependency |
| Chrome DevTools Performance | 帧率 / 长任务 / 内存 | 浏览器内置 |
| Chrome DevTools Memory | 堆快照 / 内存泄漏 | 浏览器内置 |
| React DevTools Profiler | 组件渲染火焰图 | 浏览器扩展 |
| Lighthouse | 综合性能评分 | Chrome 内置 |
| `why-did-you-render` | 检测不必要重渲染 | devDependency（仅 dev） |

### 1.3 审计流程

```
Step 1: 构建分析 — npm run build + bundle visualizer → 定位包体瓶颈
Step 2: Profiler 基线 — React DevTools Profiler 录制各屏幕交互 → 定位重渲染热点
Step 3: Performance 基线 — Chrome Performance 录制 5s 交互 → 帧率 / 长任务 / 内存
Step 4: Lighthouse 基线 — 移动端模式跑分 → FCP / TTI / 评分
Step 5: 内存基线 — 切换 10 次屏幕 + 5 次战斗 → 堆内存增长趋势
Step 6: 逐项优化 — 每项优化后重新测量，对比基线
Step 7: 回归验证 — 全部优化后重跑基线，确认无退化
```

### 1.4 测量环境

- **中端机基线**：Chrome DevTools → CPU 4x slowdown + Network Fast 3G 模拟
- **高端机基线**：Chrome DevTools → 无 CPU 降速 + Network Wi-Fi
- **真实设备**：iOS Safari（iPhone 12 / A14）+ Android Chrome（骁龙 6 系），通过 PWA 安装后测试

---

## 2. 包体分析与优化策略

### 2.1 包体构成预估

当前项目无图片资源、无重依赖，包体极小（预估 < 500KB）。但后续美术资源接入后包体会膨胀。

| 组成 | 预估大小 | 说明 |
|------|---------|------|
| React + ReactDOM | ~140KB (min+gzip) | 核心依赖 |
| idb | ~5KB (min+gzip) | IndexedDB 封装 |
| 业务代码 | ~100KB (min+gzip) | 8 Context + 10+ 屏幕 |
| 美术资源（未来） | 50~100MB | 动物图片 / UI 图标 / 特效序列帧 |
| 地图 SDK（未来） | 5~10MB | 腾讯地图 JS SDK |
| Three.js（未来） | ~150KB (min+gzip) | react-three-fiber 3D 投掷 |
| **合计** | **≤ 150MB** | 美术资源占大头 |

### 2.2 构建优化策略

#### 2.2.1 Vite 构建配置增强

```typescript
// vite.config.ts 目标配置
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'
import compression from 'vite-plugin-compression'
import { visualizer } from 'rollup-plugin-visualizer'

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      strategies: 'generateSW',
      registerType: 'autoUpdate',
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
        maximumFileSizeToCacheInBytes: 5 * 1024 * 1024,
        runtimeCaching: [
          // 见第 8 节 PWA 缓存策略
        ],
      },
      manifest: {
        name: 'AnimalPoke',
        short_name: 'AnimalPoke',
        display: 'standalone',
        background_color: '#FFF8F0',
        theme_color: '#FF8C42',
        icons: [
          { src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png' },
        ],
      },
    }),
    compression({
      algorithm: 'gzip',
      threshold: 10240,
    }),
    compression({
      algorithm: 'brotliCompress',
      threshold: 10240,
    }),
    visualizer({
      open: false,
      filename: 'dist/stats.html',
      gzipSize: true,
      brotliSize: true,
    }),
  ],
  build: {
    target: 'es2020',
    cssCodeSplit: true,
    minify: 'esbuild',
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks: {
          'react-vendor': ['react', 'react-dom'],
          'idb-vendor': ['idb'],
        },
      },
    },
  },
})
```

#### 2.2.2 代码分割（见第 6 节）

#### 2.2.3 资源按需加载

- 美术资源按物种分包，首期仅加载猫相关资源
- 地图 SDK 动态 `import()` 加载，仅在进入地图屏时加载
- Three.js 动态 `import()` 加载，仅在进入 3D 捕获小游戏时加载

#### 2.2.4 资源压缩

| 资源类型 | 工具 | 目标 |
|---------|------|------|
| 图片 | `vite-imagetools` 或构建前预处理 | WebP 格式，单图 ≤ 100KB |
| 字体 | `fonttools` subset | 仅保留中英文 + 数字子集，≤ 200KB |
| SVG | `svgo` | 移除冗余属性 |
| JSON 数据 | `vite-plugin-json-minify` | 压缩 mock 数据 |

### 2.3 包体监控

- CI 中执行 `npm run build` 后检查 `dist/` 总大小
- 超过 100MB（预警线）时 CI 报警
- 超过 150MB（硬限制）时 CI 失败

---

## 3. React 渲染优化

### 3.1 Context value 拆分（核心优化）

**问题**：当前所有 Context 的 `value` useMemo 依赖整个 `state`，任意字段变化导致全部消费者重渲染。

**方案**：将 Context value 拆分为「状态部分」和「操作部分」，操作部分引用稳定不随 state 变化。

#### 3.1.1 StaminaContext 优化

```typescript
// 现状（有问题）：value 依赖 state，每次 TICK 都更新
const value = useMemo(() => ({
  state,
  maxStamina,
  nextRecoverIn,
  consumeStamina,  // 依赖 state.currentStamina，每分钟重建
  addStamina,
  addCapture,      // 依赖 state.level, state.totalCaptures
  addGold,
  buyStaminaPotion,
}), [state, maxStamina, nextRecoverIn, ...])

// 优化方案 A：dispatch-based callbacks（引用永远稳定）
const consumeStamina = useCallback((amount: number): boolean => {
  // 从 ref 读取最新值，不依赖 state
  let success = false
  setStateRef(prev => {
    if (!canConsume(prev.currentStamina, amount)) return prev
    success = true
    return { ...prev, currentStamina: prev.currentStamina - amount }
  })
  return success
}, []) // 空依赖，永不重建
```

**注意**：`useReducer` 的 dispatch 本身引用稳定，但当前代码在 `useCallback` 中读取 `state.currentStamina` 做前置校验，导致回调依赖 state。改用 dispatch 内做校验（reducer 内已有 `canConsume` 检查），返回值通过外部变量或 ref 传递。

**优化方案 B（推荐，改动更小）**：拆分为两个 Context — StateContext + ActionsContext

```typescript
// StaminaStateContext：仅包含 state（频繁变化）
const StaminaStateContext = createContext<StaminaState | null>(null)
// StaminaActionsContext：仅包含操作函数（引用稳定）
const StaminaActionsContext = createContext<StaminaActions | null>(null)

// 消费者按需选择：
// 只需要操作的组件 → const { consumeStamina } = useStaminaActions()  → 不随 TICK 重渲染
// 只需要显示的组件 → const state = useStaminaState()  → 仅 TICK 时重渲染
// 两者都需要的组件 → 两个 hook 都调用（与现状一致）
```

#### 3.1.2 各 Context 拆分计划

| Context | state 变化频率 | 拆分收益 | 优先级 |
|---------|--------------|---------|-------|
| StaminaContext | 高（每分钟 TICK + 每次操作） | 高 — TICK 不再触发操作函数重建 | P0 |
| BattleContext | 极高（自动战斗每 N ms） | 高 — 战斗 log 更新不触发非战斗屏重渲染 | P0 |
| EconomyContext | 中（每次 trackEarn/Spend） | 中 — logs 数组变化不影响操作函数 | P1 |
| WeatherContext | 低（每周变化） | 低 — 变化频率低，收益小 | P2 |
| ShopContext | 低（购物时变化） | 低 | P2 |
| StatusContext | 低（感冒时变化） | 低 | P2 |
| DispatchContext | 低（派遣时变化） | 低 | P2 |
| LbsContext | 极低（仅定位时变化） | 极低 | P3 |

#### 3.1.3 BattleContext 特殊优化

`BattleContext` 是最重的 Context：
- 消费 5 个其他 Context
- 自动战斗时每 `AUTO_PLAY_INTERVAL_MS` 更新 state（含 log 数组）
- `value` memo 依赖 `state`（包含 `playerPet`、`enemyPet`、`log[]`）

**优化**：
1. 拆分为 `BattleStateContext` + `BattleActionsContext`
2. 战斗日志单独拆为 `BattleLogContext`（高频更新，仅 `BattleLog` 组件消费）
3. `playerPet` / `enemyPet` 变化时不需要重渲染日志列表以外的组件

```
BattleActionsContext  — 引用稳定（enterSelect / selectPet / ...）
BattleStateContext    — phase / round / result / rewards（低频）
BattlePetContext      — playerPet / enemyPet（中频，仅战斗 UI 消费）
BattleLogContext      — log[]（高频，仅 BattleLog 组件消费）
```

### 3.2 React.memo 审计

对以下组件添加 `React.memo`，阻止父组件 state 变化时的无必要重渲染：

| 组件 | 当前 | props 变化频率 | 添加 memo 收益 |
|------|------|--------------|--------------|
| `TopBar` | 无 memo | 低（仅体力/金币显示） | 中 — 避免切 tab 时重渲染 |
| `TabBar` | 无 memo | 低（activeTab 变化） | 低 — 本身就是 activeTab 驱动 |
| `CollectScreen` | 无 memo | 低（onMapOpen 回调） | 高 — 避免切 tab 时重渲染 |
| `DiscoverScreen` | 无 memo | 低（onConfirm 回调） | 高 — 避免切 tab 时重渲染（且相机组件重渲染开销大） |
| `CaptureScreen` | 无 memo | 中（targetSpecies / 回调） | 高 |
| `BattleScreen` | 无 memo | 无 props | 高 |
| `StoreScreen` | 无 memo | 无 props | 高 |
| `DispatchScreen` | 无 memo | 无 props | 高 |
| `MapScreen` | 无 memo | 中（entries / focus） | 中 |
| `DetailPopup` | 无 memo | 中（entry / onClose） | 中 |
| `BattleArena` | 无 memo | — | 高（战斗动画区域） |
| `BattleLog` | 无 memo | — | 高（日志滚动区域） |
| `HealthBar` | 无 memo | — | 中 |

**前提条件**：添加 `React.memo` 前，必须确保传入的 props 引用稳定（`useCallback` / `useMemo`），否则 memo 无效。

### 3.3 useMemo / useCallback 审计

#### 3.3.1 App.tsx 中的回调

当前 `App.tsx` 已使用 `useCallback` 包装所有回调，但部分回调依赖了不稳定的对象：

```typescript
// 现状：handleCaptureSuccess 依赖 economy / statusCtx / weatherCtx
// 这些是 Context value，每次上游 state 变化都会产生新引用
const handleCaptureSuccess = useCallback((entry: CardEntry) => {
  ...
  economy.trackEarn(...)  // economy 引用不稳定
  statusCtx.applyCold(...) // statusCtx 引用不稳定
  weatherCtx.getColdRisk() // weatherCtx 引用不稳定
}, [addAnimal, addCapture, addGold, economy, statusCtx, weatherCtx])
```

**优化**：配合 Context 拆分后，`economy.trackEarn` / `statusCtx.applyCold` / `weatherCtx.getColdRisk` 引用稳定，`handleCaptureSuccess` 的依赖不再变化。

#### 3.3.2 CollectScreen 中的派生计算

```typescript
// 现状：unlockedCount 未 memo
const unlockedCount = animals.filter(e => e.unlocked).length

// 优化：useMemo
const unlockedCount = useMemo(
  () => animals.filter(e => e.unlocked).length,
  [animals]
)
```

```typescript
// 现状：tabs / speciesTabs 每次渲染重建
const tabs = [{ key: 'all', label: '全部' }, ...]

// 优化：提取为模块级常量
const TABS = [{ key: 'all', label: '全部' }, ...] as const
const SPECIES_TABS = [...] as const
```

#### 3.3.3 DiscoverScreen 中的 Context 消费

```typescript
// 现状：在渲染体内 try/catch 调用 useLbs() / useWeather()
// 每次渲染都调用，且读取多个派生值
let cityName = '未知'
try {
  const lbs = useLbs()
  cityName = lbs.state.cityName || '未知'
} catch { ... }
```

**优化**：提取为子组件 `WeatherStrip`，仅该子组件消费 WeatherContext / LbsContext，避免 DiscoverScreen 因天气变化重渲染。

### 3.4 StrictMode 影响

当前 `main.tsx` 使用 `React.StrictMode`，开发模式下会双次调用渲染函数和 Effect。

**策略**：保留 StrictMode（开发阶段价值大），但性能测试时在 production build（`npm run build && npm run preview`）上测量。

---

## 4. Context Provider 优化

### 4.1 Provider 嵌套层级优化

当前 8 层嵌套不会产生运行时性能问题（React Provider 嵌套本身无开销），但影响代码可维护性。

**不调整嵌套层级**，仅在内部拆分 State/Actions Context（见 3.1）。

### 4.2 Provider 懒初始化

部分 Context 的初始化逻辑较重（如 `StaminaContext` 的 `loadInitialState` 做 localStorage 读取 + JSON 解析 + 离线恢复计算），可以延迟到首次 `useStamina()` 调用时。

**优先级低**：当前初始化耗时 < 5ms，不是瓶颈。仅在启动时间不达标时优化。

### 4.3 Context 消费粒度优化

**原则**：组件只消费它实际需要的 Context。

| 组件 | 当前消费的 Context | 实际需要的 | 优化 |
|------|------------------|----------|------|
| `TopBar` | 未知（需检查） | Stamina (金币/体力) + Weather (天气) | 精确消费 |
| `TabBar` | 无 | 无 | OK |
| `DiscoverScreen` | Lbs + Weather | Lbs (城市) + Weather (天气/感冒) | 提取 WeatherStrip 子组件 |
| `CaptureScreen` | Stamina + Shop | Stamina (体力) + Shop (增益) | OK |
| `CollectScreen` | AnimalStore (hook) | AnimalStore | OK |
| `BattleScreen` | Battle (内部消费 Stamina/Shop/Economy/Status/Weather) | Battle | OK（BattleContext 内部消费） |
| `StoreScreen` | Shop + Stamina | Shop (商品) + Stamina (金币) | OK |
| `DispatchScreen` | Dispatch + Economy + Stamina | Dispatch + Economy + Stamina | OK |

### 4.4 避免 Context 穿透渲染

`BattleProvider` 内部消费了 5 个其他 Context，当上游 Context 变化时，`BattleProvider` 本身会重渲染，进而可能触发 `BattleContext` 的 value 变化。

**优化**：确保 `BattleProvider` 的 `value` memo 不直接依赖上游 Context value，而是仅依赖自身 `state` 和经过 `useCallback` 稳定化的操作函数。

---

## 5. 图片 / 资源优化

### 5.1 当前状态

项目当前无图片资源，全部使用 emoji + CSS 渐变。后续美术资源接入时需要以下策略。

### 5.2 图片优化策略

| 策略 | 说明 | 实施时机 |
|------|------|---------|
| 格式选择 | WebP（兼容 iOS 14+ / Android 8+）| 美术资源接入时 |
| 多分辨率 | @1x / @2x / @3x 按设备 DPR 加载 | 美术资源接入时 |
| 懒加载 | `loading="lazy"` + IntersectionObserver | 美术资源接入时 |
| 压缩 | 构建前 `sharp` / `squoosh` 预处理，单图 ≤ 100KB | 美术资源接入时 |
| 雪碧图 | 小图标合并为 sprite，减少请求数 | UI 图标接入时 |
| 内联 | < 4KB 的小图内联为 base64 | 构建工具自动处理 |

### 5.3 字体优化

```css
/* 当前 index.css 引用了系统字体，无自定义字体 */
font-family: "Baloo 2", "Nunito", "PingFang SC", "Microsoft YaHei", sans-serif;
```

**现状**：`"Baloo 2"` / `"Nunito"` 未通过 `@font-face` 加载，实际回退到系统中文字体。如果后续需要卡通字体：
- 使用 `font-display: swap` 避免 FOIT
- 字体子集化（仅保留所需字符）
- preload 关键字体

### 5.4 美术资源按需下载

设计文档提到"后续按需下载美术资源包"：

```
首包（≤ 5MB）：核心 UI + 猫物种资源 → 快速启动
美术包 1（~20MB）：鹅物种资源 → 进入鹅相关界面时下载
美术包 2（~20MB）：狗物种资源 → 进入狗相关界面时下载
特效包（~10MB）：传说粒子 / 战斗特效 → 首次触发时下载
```

---

## 6. 代码分割策略

### 6.1 路由级懒加载（Tab 级）

当前导航基于 `activeTab` state（无 react-router），可使用 `React.lazy` + `Suspense` 实现 Tab 级懒加载。

```typescript
// App.tsx 优化
const DiscoverScreen = React.lazy(() => import('./components/DiscoverScreen'))
const CaptureScreen = React.lazy(() => import('./components/CaptureScreen'))
const CollectScreen = React.lazy(() => import('./components/CollectScreen'))
const MapScreen = React.lazy(() => import('./components/MapScreen'))
const BattleScreen = React.lazy(() => import('./components/BattleScreen'))
const StoreScreen = React.lazy(() => import('./components/StoreScreen'))
const DispatchScreen = React.lazy(() => import('./components/DispatchScreen'))

// renderContent 包裹 Suspense
const renderContent = () => (
  <Suspense fallback={<LoadingScreen />}>
    {mapOpen ? <MapScreen .../> : /* switch(activeTab) */}
  </Suspense>
)
```

**注意**：每个屏幕组件需添加 `export default`。

### 6.2 功能级懒加载

| 功能模块 | 触发时机 | 分割方式 |
|---------|---------|---------|
| 地图 SDK | 进入 MapScreen | 动态 `import()` |
| Three.js 3D 投掷 | 进入 3D 捕获小游戏 | 动态 `import()` |
| VLM 帧上传 | 发现阶段拍照后 | 动态 `import()`（上传服务模块） |
| 图鉴详情弹窗 | 点击图鉴卡片 | `React.lazy` |

### 6.3 Vendor 分包

```typescript
// vite.config.ts rollupOptions.output.manualChunks
manualChunks: {
  'react-vendor': ['react', 'react-dom'],
  'idb-vendor': ['idb'],
  // 后续新增：
  // 'map-vendor': ['tencent-map'],
  // 'three-vendor': ['three', '@react-three/fiber'],
}
```

### 6.4 预加载策略

```typescript
// 用户 hover TabBar 时预加载对应屏幕
const handleTabHover = (tab: MainTab) => {
  switch (tab) {
    case 'collection': import('./components/CollectScreen'); break
    case 'store': import('./components/StoreScreen'); break
    // ...
  }
}
```

---

## 7. 相机 / 屏幕性能优化

### 7.1 相机生命周期优化

**问题**：当前切 tab 时 `DiscoverScreen` 卸载才停止相机，切回来又重新申请权限+启动。

**优化**：
- 方案 A：使用 `keep-alive`（React 无原生支持，需第三方库如 `react-activation`）— 引入新依赖，不推荐
- 方案 B（推荐）：在 `DiscoverScreen` 内部监听 `document.visibilityState`，页面不可见时暂停相机视频流（`video.pause()`），可见时恢复（`video.play()`），不释放 stream
- 方案 C：App 层面控制——切 tab 时不卸载 `DiscoverScreen`，用 CSS `display: none` 隐藏。但需调整 `renderContent` 逻辑

**选择方案 B**：最小改动，收益明确。

### 7.2 拍照质量优化

```typescript
// 现状：0.9 质量，base64 较大
const data = canvas.toDataURL('image/jpeg', 0.9)

// 优化：降至 0.7（VLM 检测不需要高质量），或使用 toBlob
canvas.toBlob(
  (blob) => { /* 上传 blob 而非 base64，减少 ~33% 体积 */ },
  'image/jpeg',
  0.7
)
```

**收益**：base64 → Blob 减少 ~33% 体积；0.9 → 0.7 质量减少 ~50% 体积。

### 7.3 帧采样策略（VLM 接入后）

设计文档要求 2~5 FPS 自适应帧采样：

```typescript
// 自适应帧采样器
class FrameSampler {
  private fps = 2  // 初始 2 FPS
  private intervalId: number | null = null

  start(video: HTMLVideoElement, onFrame: (blob: Blob) => void) {
    this.intervalId = window.setInterval(() => {
      const canvas = document.createElement('canvas')
      canvas.width = 320  // 降采样
      canvas.height = 240
      const ctx = canvas.getContext('2d')!
      ctx.drawImage(video, 0, 0, 320, 240)
      canvas.toBlob(blob => blob && onFrame(blob), 'image/jpeg', 0.7)
    }, 1000 / this.fps)
  }

  // 命中后提升采样率（AR 锚定跟踪阶段）
  boost() { this.fps = 5; this.restart() }
  // 未命中降回
  normal() { this.fps = 2; this.restart() }
}
```

### 7.4 Canvas 2D 动画优化（CaptureScreen）

当前 `CaptureScreen` 充能动画使用 `setInterval(50ms)` → 20 FPS 更新 UI。对于充能进度条这种简单动画可接受，但如果后续升级为抛物线投掷动画：

- 使用 `requestAnimationFrame` 替代 `setInterval`
- Canvas 2D 绘制而非 DOM 操作（避免 layout/paint）
- 动画元素使用 `will-change: transform` 提示合成层

### 7.5 战斗动画优化

当前 `BattleContext` 自动战斗使用 `setInterval(AUTO_PLAY_INTERVAL_MS)`：

- 战斗 log 更新触发 `BattleScreen` 重渲染
- `BattleLog` 组件渲染所有日志条目

**优化**：
1. `BattleLog` 使用 `React.memo` + 仅依赖 `log.length`（而非整个 log 数组）
2. 日志条目使用虚拟列表（如果 log 超过 50 条）
3. HP 条动画使用 CSS `transition` 而非 JS 逐帧更新
4. `HealthBar` 组件 `React.memo`，仅 `currentHp` / `maxHp` 变化时重渲染

---

## 8. IndexedDB 查询优化

### 8.1 索引优化

```typescript
// db.ts 现有索引
store.createIndex('by-date', 'captureDate')
store.createIndex('by-rarity', 'rarity')

// 新增索引
store.createIndex('by-unlocked', 'isUnlocked')  // 需将 unlocked boolean 改为 0/1 数字
store.createIndex('by-species', 'species')
store.createIndex('by-date-unlocked', ['captureDate', 'isUnlocked'])  // 复合索引
```

**注意**：IndexedDB 索引 key 不能是 boolean，需将 `unlocked: boolean` 改为 `isUnlocked: 0 | 1`，或新增一个数字字段。

### 8.2 查询方法优化

```typescript
// getUnlocked() — 现状：全表扫描
async getUnlocked(): Promise<AnimalRecord[]> {
  const db = await getDB()
  const all = await db.getAll('animals')
  return all.filter(a => a.unlocked)
}

// 优化：使用索引
async getUnlocked(): Promise<AnimalRecord[]> {
  const db = await getDB()
  return db.getAllFromIndex('animals', 'by-unlocked', 1)
}

// countUnlocked() — 现状：全表扫描
async countUnlocked(): Promise<number> {
  const db = await getDB()
  const all = await db.getAll('animals')
  return all.filter(a => a.unlocked).length
}

// 优化：使用 count
async countUnlocked(): Promise<number> {
  const db = await getDB()
  return db.countFromIndex('animals', 'by-unlocked', 1)
}

// markViewed() — 现状：get + put
async markViewed(id: string): Promise<void> {
  const record = await db.get('animals', id)
  if (record) {
    record.isNew = false
    await db.put('animals', record)
  }
}

// 优化：使用 cursor update（单次 IO）
async markViewed(id: string): Promise<void> {
  const db = await getDB()
  const tx = db.transaction('animals', 'readwrite')
  const cursor = await tx.store.openCursor(id)
  if (cursor) {
    const record = cursor.value
    record.isNew = false
    await cursor.update(record)
  }
  await tx.done
}
```

### 8.3 分页加载

```typescript
// getAll() — 现状：全表加载
async getAll(): Promise<AnimalRecord[]> {
  const db = await getDB()
  return db.getAll('animals')
}

// 优化：分页 + 游标
async getPage(offset: number, limit: number): Promise<AnimalRecord[]> {
  const db = await getDB()
  const tx = db.transaction('animals', 'readonly')
  let cursor = await tx.store.openCursor()
  // 跳过 offset 条
  for (let i = 0; i < offset && cursor; i++) {
    cursor = await cursor.continue()
  }
  // 读取 limit 条
  const results: AnimalRecord[] = []
  for (let i = 0; i < limit && cursor; i++) {
    results.push(cursor.value)
    cursor = await cursor.continue()
  }
  await tx.done
  return results
}
```

### 8.4 useAnimalStore 优化

```typescript
// 现状：一次性加载全部
useEffect(() => {
  AnimalRepository.getAll().then(setAnimals)
}, [])

// 优化：分页加载 + 按需加载
const [page, setPage] = useState(0)
const PAGE_SIZE = 20

useEffect(() => {
  AnimalRepository.getPage(page * PAGE_SIZE, PAGE_SIZE).then(list => {
    setAnimals(prev => page === 0 ? list : [...prev, ...list])
  })
}, [page])

// 滚动到底部时加载下一页
const loadMore = useCallback(() => setPage(p => p + 1), [])
```

### 8.5 虚拟列表

当图鉴数据量超过 50 条时，`CollectScreen` 的网格渲染所有卡片会导致卡顿。

```typescript
// 使用虚拟列表（轻量实现，不引入 react-window）
// 或后续引入 react-window（~6KB gzip）
```

**MVP 阶段**：图鉴仅 9 格 + padding，无需虚拟列表。
**内测阶段**：数据量增大时引入 `react-window` 或自实现虚拟滚动。

---

## 9. CSS 优化

### 9.1 现状评估

当前 CSS 量小（`index.css` 142 行），无性能问题。后续优化方向：

### 9.2 优化策略

| 策略 | 说明 | 优先级 |
|------|------|-------|
| CSS Code Split | Vite 默认已启用 `cssCodeSplit: true` | 已有 |
| 内联关键 CSS | 首屏关键样式内联到 HTML | P2（首屏不达标时） |
| 避免 `@import` | 不在 CSS 中使用 `@import`（阻塞渲染） | 已遵守 |
| `will-change` | 仅对动画元素使用 | P2 |
| `contain` | 对独立区域使用 CSS containment | P2 |
| 合成层 | 动画使用 `transform` / `opacity`（不触发 layout） | 已遵守 |
| 选择器深度 | 保持选择器 ≤ 3 层 | 已遵守 |

### 9.3 内联样式优化

部分组件使用内联 `style` 对象，虽然已提取为模块级常量（如 `CollectScreen` 的 `styles`），但仍有优化空间：

```typescript
// 动态样式仍用内联（如进度条宽度）
<div style={{ ...styles.powerFill, width: `${charge}%` }} />

// 这是合理的——动态值必须内联
```

**结论**：当前内联样式使用方式合理，无需大改。

---

## 10. PWA 缓存策略

### 10.1 Service Worker 策略

使用 `vite-plugin-pwa` 的 `generateSW` 模式（自动生成 SW）：

```typescript
VitePWA({
  strategies: 'generateSW',
  registerType: 'autoUpdate',
  workbox: {
    globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
    maximumFileSizeToCacheInBytes: 5 * 1024 * 1024,  // 5MB
    runtimeCaching: [
      {
        // API 请求：NetworkFirst（在线优先，离线降级缓存）
        urlPattern: /\/api\//,
        handler: 'NetworkFirst',
        options: {
          cacheName: 'api-cache',
          expiration: { maxEntries: 50, maxAgeSeconds: 86400 },
        },
      },
      {
        // 图片资源：CacheFirst（缓存优先）
        urlPattern: /\.(?:png|jpg|jpeg|svg|webp)$/,
        handler: 'CacheFirst',
        options: {
          cacheName: 'image-cache',
          expiration: { maxEntries: 100, maxAgeSeconds: 7 * 86400 },
        },
      },
      {
        // 字体：CacheFirst
        urlPattern: /\.(?:woff2?|ttf|otf)$/,
        handler: 'CacheFirst',
        options: {
          cacheName: 'font-cache',
          expiration: { maxEntries: 10, maxAgeSeconds: 30 * 86400 },
        },
      },
      {
        // JS / CSS：StaleWhileRevalidate（先缓存后更新）
        urlPattern: /\.(?:js|css)$/,
        handler: 'StaleWhileRevalidate',
        options: {
          cacheName: 'static-cache',
          expiration: { maxEntries: 50, maxAgeSeconds: 7 * 86400 },
        },
      },
    ],
  },
})
```

### 10.2 缓存层级

| 层级 | 内容 | 策略 | 失效 |
|------|------|------|------|
| Pre-cache | App Shell（HTML/JS/CSS） | 构建时注入 manifest | 版本更新时 |
| Runtime: API | 后端响应 | NetworkFirst | 24h |
| Runtime: 图片 | 动物图片 / UI 图标 | CacheFirst | 7d |
| Runtime: 字体 | 自定义字体 | CacheFirst | 30d |
| Runtime: 静态 | JS / CSS | StaleWhileRevalidate | 7d |

### 10.3 离线体验

设计文档要求"断网时仅可浏览本地图鉴"：

- App Shell（HTML/JS/CSS）pre-cache → 离线可打开
- 图鉴数据已在 IndexedDB → 离线可浏览
- 离线时禁用发现/捕获/战斗按钮，显示"网络异常"提示
- SW 拦截 `/api/` 请求，离线时返回缓存或友好错误

### 10.4 更新策略

- `registerType: 'autoUpdate'` — 新版本自动下载，下次启动生效
- 用户可通过弹窗提示"新版本已就绪，点击刷新"

---

## 11. 测试 / 基准方法

### 11.1 性能测试用例

| 测试场景 | 操作步骤 | 测量指标 | 目标 |
|---------|---------|---------|------|
| 首屏加载 | 冷启动 → FCP | FCP | ≤ 1.5s |
| Tab 切换 | 点击各 Tab 5 次 | FPS / 长任务 | ≥ 30fps, 0 长任务 |
| 相机启动 | 进入 DiscoverScreen → 取景就绪 | 延迟 | ≤ 2s |
| 拍照检测 | 拍照 → 检测结果 | 延迟 | ≤ 3s（含网络） |
| 充能投掷 | 充能 → 投掷 → 结果 | FPS | ≥ 30fps |
| 自动战斗 | 开始战斗 → 结束 | FPS / 内存增长 | ≥ 30fps, ≤ 10MB |
| 图鉴滚动 | 滚动图鉴列表 | FPS | ≥ 30fps |
| 体力恢复 | 等待 TICK | 不必要重渲染数 | 0（非体力显示组件） |
| 内存泄漏 | 切换屏幕 20 次 | 堆内存 | 无持续增长 |

### 11.2 自动化测试

```typescript
// vitest 性能测试（逻辑层面）
describe('Performance', () => {
  it('CollectScreen filtered memo 不应在 animals 未变时重新计算', () => {
    // 验证 useMemo 依赖正确
  })

  it('StaminaContext dispatch 引用应稳定', () => {
    // 验证 useCallback 依赖为空
  })
})
```

### 11.3 CI 性能门禁

```yaml
# CI 性能检查
- name: Build
  run: npm run build

- name: Bundle size check
  run: |
    SIZE=$(du -sh dist/ | cut -f1)
    # 解析为字节并比较
    # > 150MB → fail
    # > 100MB → warn

- name: Lighthouse CI
  run: |
    npm install -g @lhci/cli
    lhci autorun --collect.url=http://localhost:4173
```

### 11.4 持续监控

- 每次 PR 触发 build + bundle size 对比
- Lighthouse 评分趋势追踪
- `why-did-you-render` 在 dev 模式输出不必要渲染警告

---

## 12. 验收标准

### 12.1 硬性指标

| 指标 | 目标 | 验收方法 |
|------|------|---------|
| 帧率 | 中端机（CPU 4x slowdown）≥ 30fps | Chrome Performance 录制 5s 交互 |
| 包体 | `dist/` 总大小 ≤ 150MB | `du -sh dist/` |
| 首屏 FCP | ≤ 1.5s（中端机） | Lighthouse |
| 交互就绪 TTI | ≤ 3s（中端机） | Lighthouse |
| 内存峰值 | ≤ 150MB | Chrome Memory Monitor |
| 崩溃率 | < 0.5% | 内测统计 |

### 12.2 质量指标

| 指标 | 目标 | 验收方法 |
|------|------|---------|
| 不必要重渲染 | 0 / 单次交互 | React DevTools Profiler |
| 长任务（> 50ms） | ≤ 5 / 页面切换 | Chrome Performance |
| Context 消费者 | 操作函数引用稳定（空依赖） | 代码审查 + 单元测试 |
| PWA 评分 | Lighthouse PWA ≥ 90 | Lighthouse |
| 离线可用 | 图鉴浏览可用 | 断网测试 |

### 12.3 验收 Checklist

- [ ] `vite.config.ts` 配置 manualChunks + compression + visualizer
- [ ] `vite-plugin-pwa` 集成，SW 注册 + manifest 生成
- [ ] 所有屏幕组件 `React.lazy` + `Suspense` 懒加载
- [ ] `StaminaContext` 拆分为 State/Actions Context（或 dispatch-based callbacks）
- [ ] `BattleContext` 拆分 log / state / actions
- [ ] `EconomyContext` 拆分 logs / state / actions
- [ ] 所有屏幕组件添加 `React.memo`
- [ ] `CollectScreen` 的 `unlockedCount` / `tabs` memo 化
- [ ] `DiscoverScreen` 提取 `WeatherStrip` 子组件
- [ ] `canvas.toDataURL` 改为 `toBlob` + 质量 0.7
- [ ] IndexedDB `getUnlocked()` / `countUnlocked()` 使用索引
- [ ] IndexedDB `markViewed()` 使用 cursor update
- [ ] `useAnimalStore` 支持分页加载
- [ ] gzip + brotli 预压缩启用
- [ ] CI bundle size 门禁生效
- [ ] 中端机 30fps 验证通过
- [ ] 离线图鉴浏览验证通过

---

## 13. 实施步骤

### Phase 1: 构建基础设施（无代码改动风险）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 1.1 | 安装 dev 依赖：`vite-plugin-pwa`、`vite-plugin-compression`、`rollup-plugin-visualizer` | `package.json` |
| 1.2 | 增强 `vite.config.ts`：manualChunks + compression + visualizer + PWA | `vite.config.ts` |
| 1.3 | 添加 PWA manifest + icons | `public/` |
| 1.4 | 执行 `npm run build` 生成基线 bundle 报告 | `dist/stats.html` |
| 1.5 | CI 添加 bundle size 门禁脚本 | `.github/workflows/` 或 CI 配置 |

### Phase 2: 代码分割（中风险，需测试）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 2.1 | 所有屏幕组件添加 `export default`（如未有） | 各屏幕组件 |
| 2.2 | `App.tsx` 改为 `React.lazy` + `Suspense` 懒加载 | `App.tsx` |
| 2.3 | 验证 Tab 切换正常、Suspense fallback 正确 | 手动测试 |
| 2.4 | 添加 Tab hover 预加载 | `TabBar.tsx` |

### Phase 3: Context 渲染优化（高风险，核心改动）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 3.1 | `StaminaContext` 拆分 State/Actions Context | `StaminaContext.tsx`、`useStamina.ts` |
| 3.2 | `EconomyContext` 拆分 logs / state / actions | `EconomyContext.tsx`、`useEconomy.ts` |
| 3.3 | `BattleContext` 拆分 log / state / actions | `BattleContext.tsx`、`useBattle.ts` |
| 3.4 | `WeatherContext` / `ShopContext` / `StatusContext` / `DispatchContext` 拆分 | 各 Context |
| 3.5 | 更新所有消费组件的 import | 各组件 |
| 3.6 | 全量回归测试 | `vitest` + 手动 |

### Phase 4: React.memo + useMemo 审计（低风险）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 4.1 | 所有屏幕组件添加 `React.memo` | 各屏幕组件 |
| 4.2 | `CollectScreen` memo 化 `unlockedCount` / `tabs` | `CollectScreen.tsx` |
| 4.3 | `DiscoverScreen` 提取 `WeatherStrip` 子组件 | `DiscoverScreen.tsx` |
| 4.4 | `App.tsx` 回调依赖检查（配合 Context 拆分） | `App.tsx` |
| 4.5 | 子组件（`HealthBar` / `BattleLog` / `DetailPopup`）添加 memo | 各组件 |

### Phase 5: 相机 / Canvas 优化（中风险）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 5.1 | `canvas.toDataURL(0.9)` → `toBlob(0.7)` | `DiscoverScreen.tsx` |
| 5.2 | 相机 visibility 暂停/恢复 | `DiscoverScreen.tsx` |
| 5.3 | `CaptureScreen` 充能改 `requestAnimationFrame`（可选） | `CaptureScreen.tsx` |
| 5.4 | `BattleScreen` 动画优化（CSS transition 替代 JS 逐帧） | `BattleScreen.tsx` / `HealthBar.tsx` |

### Phase 6: IndexedDB 优化（中风险）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 6.1 | `db.ts` 新增索引（`by-unlocked` / `by-species`）+ DB_VERSION 升级 | `db.ts` |
| 6.2 | `animal-repository.ts` 优化查询方法 | `animal-repository.ts` |
| 6.3 | `useAnimalStore` 支持分页 | `useAnimalStore.ts` |
| 6.4 | 数据迁移脚本（boolean → 0/1） | `db.ts` upgrade |
| 6.5 | 回归测试（`animal-repository.test.ts`） | 测试文件 |

### Phase 7: PWA 缓存（中风险）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 7.1 | 配置 `vite-plugin-pwa` runtimeCaching 规则 | `vite.config.ts` |
| 7.2 | 离线降级 UI（断网提示） | `App.tsx` / 新组件 |
| 7.3 | 更新提示 UI（新版本就绪弹窗） | `App.tsx` / 新组件 |
| 7.4 | 离线图鉴浏览验证 | 手动测试 |

### Phase 8: 性能验证（最终验收）

| 步骤 | 内容 | 输出 |
|------|------|------|
| 8.1 | Production build + bundle visualizer | `dist/stats.html` |
| 8.2 | Lighthouse 移动端跑分 | 评分报告 |
| 8.3 | Chrome Performance 录制各屏幕交互 | 帧率 / 长任务报告 |
| 8.4 | React DevTools Profiler 录制 | 渲染火焰图 |
| 8.5 | Chrome Memory 堆快照（切换屏幕 20 次） | 内存增长报告 |
| 8.6 | 中端机真机测试（iOS Safari + Android Chrome） | 真机帧率 |
| 8.7 | 填写验收 Checklist | 验收报告 |

---

## 14. 风险与应对

| 风险 | 影响 | 概率 | 应对策略 |
|------|------|------|---------|
| Context 拆分引入 bug | 消费组件取不到值 / 取到旧值 | 中 | 逐步拆分，每步 vitest 回归；保留旧 hook 的兼容 re-export |
| `React.lazy` 导致首屏闪烁 | Suspense fallback 闪现 | 低 | 预加载关键屏幕；fallback 使用骨架屏 |
| IndexedDB 索引升级失败 | 旧数据无新字段，索引为空 | 中 | upgrade 回调中迁移数据；添加字段默认值 |
| PWA SW 缓存过期导致用户卡在旧版 | 用户无法获取更新 | 中 | `autoUpdate` + 更新提示弹窗；SW skipWaiting |
| `toBlob` 兼容性 | 旧浏览器不支持 | 低 | iOS 14+ / Android 8+ 均支持，添加 fallback |
| 内存泄漏（相机 stream 未释放） | 切屏后相机仍运行 | 中 | `useEffect` cleanup 确认 `streamRef.current.getTracks().stop()` |
| `React.memo` 过度使用 | 调试困难（props 变化但组件不更新） | 低 | 仅对屏幕级组件和列表项使用；profiler 验证有效性 |
| 美术资源接入后包体超限 | ≤ 150MB 目标破灭 | 中 | 资源按需下载；CI 门禁；WebP 压缩 |
| 中端机 30fps 不达标 | 验收失败 | 中 | 分级降级（低端机关闭粒子特效 / 降低帧率） |
| `why-did-you-render` 噪音 | 开发体验下降 | 低 | 仅在 dev 模式启用；过滤已知无害渲染 |

### 14.1 降级方案

如果优化后仍不达标：

| 不达标项 | 降级方案 |
|---------|---------|
| 中端机 < 30fps | 关闭粒子特效；降低动画帧率；减少同时渲染的列表项 |
| 包体 > 150MB | 美术资源改为按需下载（不打入首包）；地图 SDK 改用轻量替代 |
| FCP > 1.5s | 内联关键 CSS；preload 关键 JS；骨架屏 |
| 内存 > 150MB | 图片懒加载更激进；IndexedDB 分页更小；清理日志上限 |

---

## 附录 A: 文件改动清单

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `vite.config.ts` | 修改 | PWA + compression + manualChunks + visualizer |
| `package.json` | 修改 | 新增 dev 依赖 |
| `tsconfig.json` | 修改 | `noUnusedLocals` / `noUnusedParameters` 改为 true（可选） |
| `public/manifest.json` | 新增 | PWA manifest（由 vite-plugin-pwa 生成） |
| `public/icons/` | 新增 | PWA 图标 192/512 |
| `src/App.tsx` | 修改 | React.lazy + Suspense + 回调依赖修正 |
| `src/main.tsx` | 修改 | SW 注册（由 vite-plugin-pwa 自动注入） |
| `src/stamina/StaminaContext.tsx` | 修改 | 拆分 State/Actions Context |
| `src/stamina/useStamina.ts` | 修改 | 适配拆分后的 Context |
| `src/economy/EconomyContext.tsx` | 修改 | 拆分 logs / state / actions |
| `src/economy/useEconomy.ts` | 修改 | 适配拆分后的 Context |
| `src/battle/BattleContext.tsx` | 修改 | 拆分 log / state / actions |
| `src/battle/useBattle.ts` | 修改 | 适配拆分后的 Context |
| `src/weather/WeatherContext.tsx` | 修改 | 拆分（低优先级） |
| `src/shop/ShopContext.tsx` | 修改 | 拆分（低优先级） |
| `src/status/StatusContext.tsx` | 修改 | 拆分（低优先级） |
| `src/economy/DispatchContext.tsx` | 修改 | 拆分（低优先级） |
| `src/components/CollectScreen.tsx` | 修改 | memo + useMemo + 模块常量 |
| `src/components/DiscoverScreen.tsx` | 修改 | memo + toBlob + WeatherStrip 提取 + visibility 暂停 |
| `src/components/CaptureScreen.tsx` | 修改 | memo + RAF（可选） |
| `src/components/BattleScreen.tsx` | 修改 | memo |
| `src/components/StoreScreen.tsx` | 修改 | memo |
| `src/components/DispatchScreen.tsx` | 修改 | memo |
| `src/components/MapScreen.tsx` | 修改 | memo |
| `src/components/TopBar.tsx` | 修改 | memo |
| `src/components/DetailPopup.tsx` | 修改 | memo |
| `src/components/BattleArena.tsx` | 修改 | memo |
| `src/components/BattleLog.tsx` | 修改 | memo + 虚拟列表（可选） |
| `src/components/HealthBar.tsx` | 修改 | memo |
| `src/db/db.ts` | 修改 | 新增索引 + DB_VERSION 升级 |
| `src/db/repositories/animal-repository.ts` | 修改 | 索引查询 + cursor update + 分页 |
| `src/hooks/useAnimalStore.ts` | 修改 | 分页加载 |
| `src/components/OfflineNotice.tsx` | 新增 | 离线提示组件 |
| `src/components/UpdatePrompt.tsx` | 新增 | 更新提示组件 |

## 附录 B: 优先级排序

| 优先级 | 优化项 | 预期收益 | 风险 |
|-------|--------|---------|------|
| P0 | Vite 构建配置（manualChunks + compression） | 包体减小 30~50% | 低 |
| P0 | 代码分割（React.lazy） | 首屏加载减少 50~70% | 低 |
| P0 | PWA Service Worker | 离线可用 + 二次访问秒开 | 低 |
| P0 | StaminaContext 拆分 | 消除每分钟全树重渲染 | 中 |
| P0 | BattleContext 拆分 | 消除战斗时全树重渲染 | 中 |
| P1 | React.memo 全屏幕组件 | 消除切 tab 时无必要重渲染 | 低 |
| P1 | DiscoverScreen toBlob + 质量降低 | 上传体积减 60% | 低 |
| P1 | IndexedDB 索引优化 | 查询性能提升 5~10x | 中 |
| P1 | CollectScreen memo 审计 | 消除 O(n) 重复计算 | 低 |
| P2 | EconomyContext 拆分 | 消除 trackEarn/Spend 时重渲染 | 中 |
| P2 | DiscoverScreen WeatherStrip 提取 | 天气变化不触发相机组件重渲染 | 低 |
| P2 | 相机 visibility 暂停 | 省电 + 减少后台 GPU 占用 | 低 |
| P2 | useAnimalStore 分页 | 大数据量防卡顿 | 中 |
| P3 | 其他 Context 拆分 | 收益递减 | 中 |
| P3 | CSS containment / will-change | 动画流畅度微调 | 低 |
| P3 | why-did-you-render | 开发期检测 | 低 |
| P3 | 虚拟列表 | 数据量 > 50 时才需要 | 低 |
