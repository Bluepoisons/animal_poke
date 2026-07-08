# [M2] 多物种支持（鹅/狗）— 前端实现计划

> Issue: #35 | 里程碑: M2 内测 | 最后更新: 2026-07-08

---

## 1. 概述

### 1.1 目标
将当前只支持猫（🐱 硬编码）的前端扩展为支持**猫 / 鹅 / 狗**三类动物，架构预留可扩展更多物种。所有新增功能通过可替换的接口抽象，为后续接入真实后端 API 做准备。

### 1.2 受影响文件
| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `src/types.ts` | 修改 + 新增 | CardEntry 加 species 字段；新增 SpeciesType、SPECIES_DEFS |
| `src/types.test.ts` | **新建** | SPECIES_DEFS 完备性、SpeciesType 类型校验 |
| `src/services/visionDetect.ts` | **新建** | mock VLM 检测模块（可替换接口） |
| `src/services/visionDetect.test.ts` | **新建** | mock 检测逻辑单元测试 |
| `src/components/DiscoverScreen.tsx` | 修改 | 拍照确认 → 调用 mockVisionDetect → 条件进入捕获 |
| `src/components/CaptureScreen.tsx` | 修改 | 接收 species prop；切换 emoji/投掷物/手感参数 |
| `src/components/CollectScreen.tsx` | 修改 | 物种分组展示 + 物种筛选 tab |
| `src/components/DetailPopup.tsx` | 修改 | 显示物种信息 |
| `src/App.tsx` | 修改 | 发现→捕获数据流传递 species |
| `src/components/discover-screen.test.tsx` | **新建** | DiscoverScreen 检测集成测试 |
| `src/components/capture-screen.test.tsx` | 修改 | 补充多物种适配测试用例 |
| `src/components/collect-screen.test.tsx` | **新建** | 图鉴物种分组/筛选测试 |

### 1.3 不改动的文件
- `src/stamina/` — 体力系统内部实现不变
- `src/shop/` — 商店系统内部实现不变
- `src/db/` — IndexedDB schema 兼容（species = 'cat' 作为已有数据的默认值）
- `src/hooks/useAnimalStore.ts` — 存储逻辑不变（CardEntry 兼容旧数据）

---

## 2. Species 类型设计

### 2.1 类型定义

**文件**: `src/types.ts` — 在现有类型定义下方追加：

```typescript
// ===== 物种系统（Issue #35） =====

/** 物种标识 */
export type SpeciesType = 'cat' | 'goose' | 'dog'

/** 物种定义 */
export interface SpeciesDef {
  /** 物种标识 */
  species: SpeciesType
  /** 中文名 */
  name: string
  /** 显示 emoji */
  emoji: string
  /** 投掷物中文名 */
  throwItem: string
  /** 投掷物 emoji */
  throwItemEmoji: string
  /** 手感描述（仅用于 UI 提示） */
  captureMechanics: string
  /** 充能速率（每 tick 增加百分比，默认 2） */
  chargeRate: number
  /** 最佳力度区间 [min, max]（百分比），默认 [40, 80] */
  optimalRange: [number, number]
}
```

### 2.2 SPECIES_DEFS 常量

**文件**: `src/types.ts` — 紧接着 SpeciesDef 定义：

```typescript
/** 物种定义表 */
export const SPECIES_DEFS: Record<SpeciesType, SpeciesDef> = {
  cat: {
    species: 'cat',
    name: '猫',
    emoji: '🐱',
    throwItem: '猫粮罐',
    throwItemEmoji: '🥫',
    captureMechanics: '标准抛物线',
    chargeRate: 2,
    optimalRange: [40, 80],
  },
  goose: {
    species: 'goose',
    name: '鹅',
    emoji: '🪿',
    throwItem: '面包屑球',
    throwItemEmoji: '🍞',
    captureMechanics: '弹跳略强',
    chargeRate: 2.5,          // 充能更快，手感更激进
    optimalRange: [35, 75],    // 最佳区间稍宽，体现弹跳
  },
  dog: {
    species: 'dog',
    name: '狗',
    emoji: '🐶',
    throwItem: '骨头零食',
    throwItemEmoji: '🦴',
    captureMechanics: '下落更快',
    chargeRate: 1.5,           // 充能更慢，手感更沉
    optimalRange: [45, 85],     // 最佳区间偏高，体现"需要更大力"
  },
}
```

**设计约束**:
- 新增物种只需追加一条 SPECIES_DEFS 条目，不改其他任何代码
- `chargeRate` 和 `optimalRange` 是 CaptureScreen 手感差异的唯一调参入口
- 不需要物理引擎；充能速率 + 最佳区间的组合即可模拟三种不同的投掷手感

### 2.3 CardEntry 扩展

**文件**: `src/types.ts` — 修改 CardEntry 接口：

```typescript
export interface CardEntry {
  id: string
  no: string
  rarity: RarityTier
  species?: SpeciesType      // 新增：物种标识（可选，兼容旧数据）
  unlocked: boolean
  captureDate: string
  location: string
  lat: number
  lng: number
  seed: number
  isNew?: boolean
}
```

**向后兼容**: `species` 为可选字段。读取时若 `undefined` 则默认视为 `'cat'`，确保已有 IndexedDB 数据不报错。

**辅助函数**（追加到 `src/types.ts`）：
```typescript
/** 获取 CardEntry 的物种，未设置的旧数据默认猫 */
export function getCardSpecies(entry: CardEntry): SpeciesType {
  return entry.species ?? 'cat'
}
```

---

## 3. Mock VLM 检测模块

### 3.1 接口设计

**文件**: `src/services/visionDetect.ts`（新建）

```typescript
import type { SpeciesType } from '../types'

// ===== 检测结果类型 =====

export interface DetectionResult {
  /** 检测到的物种 */
  species: SpeciesType
  /** 置信度 0~1 */
  confidence: number
  /** 归一化检测框 [x, y, width, height]，值范围 0~1 */
  boundingBox: [number, number, number, number]
}

export interface VisionDetector {
  /** 对单帧照片进行动物检测 */
  detect: (photoData: string) => Promise<DetectionResult>
}

// ===== Mock 实现 =====

/** 模拟网络延迟范围 ms */
const MOCK_LATENCY: [number, number] = [300, 1200]

/** 模拟置信度范围 */
const MOCK_CONFIDENCE: [number, number] = [0.70, 0.98]

/** 三种物种均概率 */
const SPECIES_POOL: SpeciesType[] = ['cat', 'goose', 'dog']

/** 模拟检测框（随机位置） */
function randomBoundingBox(): [number, number, number, number] {
  const x = 0.15 + Math.random() * 0.2   // 0.15~0.35
  const y = 0.20 + Math.random() * 0.15  // 0.20~0.35
  const w = 0.3 + Math.random() * 0.2    // 0.30~0.50
  const h = 0.3 + Math.random() * 0.15   // 0.30~0.45
  return [x, y, w, h]
}

/** Mock 视觉检测器 —— 设计为接口形式，方便后续替换为真实 API */
export const mockVisionDetector: VisionDetector = {
  async detect(_photoData: string): Promise<DetectionResult> {
    // 模拟网络延迟
    const latency = MOCK_LATENCY[0] + Math.random() * (MOCK_LATENCY[1] - MOCK_LATENCY[0])
    await new Promise(resolve => setTimeout(resolve, latency))

    // 随机选择物种
    const species = SPECIES_POOL[Math.floor(Math.random() * SPECIES_POOL.length)]

    // 随机置信度
    const confidence = MOCK_CONFIDENCE[0] + Math.random() * (MOCK_CONFIDENCE[1] - MOCK_CONFIDENCE[0])

    // 随机检测框
    const boundingBox = randomBoundingBox()

    return { species, confidence: Math.round(confidence * 100) / 100, boundingBox }
  },
}

// ===== 未来替换为真实 API 的接口 =====
//
// export const apiVisionDetector: VisionDetector = {
//   async detect(photoData: string): Promise<DetectionResult> {
//     const formData = new FormData()
//     const blob = await (await fetch(photoData)).blob()
//     formData.append('frame', blob, 'frame.jpg')
//
//     const res = await fetch(`${import.meta.env.VITE_BACKEND_URL}/vision/detect`, {
//       method: 'POST',
//       headers: { Authorization: `Bearer ${getToken()}` },
//       body: formData,
//     })
//
//     if (!res.ok) throw new Error(`Vision detect failed: ${res.status}`)
//     return res.json()
//   },
// }
```

**关键设计点**:
- `VisionDetector` 接口：真正的依赖反转。DiscoverScreen 只依赖接口，不关心实现
- mock 实现在 `src/services/` 目录，未来真实 API 实现可在同一文件中追加，或切换 import 路径
- 置信度在 `0.70~0.98` 范围随机，满足"≥0.85 触发发现"的验收标准
- 模拟 `300~1200ms` 随机网络延迟，让 UI 状态切换更真实

### 3.2 使用方式

```typescript
// DiscoverScreen 中使用
import { mockVisionDetector, type DetectionResult } from '../services/visionDetect'

// 拍照确认后
const result = await mockVisionDetector.detect(photoData)
if (result.confidence >= 0.85) {
  // 进入捕获状态，传递 species
  onDetected(result)
} else {
  // 提示"未发现动物"
  showNoDetectionHint()
}
```

---

## 4. 物种数据扩展

### 4.1 MOCK_ENTRIES 更新

**文件**: `src/types.ts` — 扩展 MOCK_ENTRIES 添加 species 字段：

```typescript
export const MOCK_ENTRIES: CardEntry[] = [
  { id: 'c001', no: '#000059', rarity: 'common', species: 'cat', unlocked: true, captureDate: '2026-07-08', location: '海曙区·月湖', lat: 29.87, lng: 121.55, seed: 1, isNew: true },
  { id: 'c002', no: '#000058', rarity: 'uncommon', species: 'goose', unlocked: true, captureDate: '2026-07-07', location: '鄞州区·公园', lat: 29.83, lng: 121.57, seed: 2, isNew: true },
  { id: 'c003', no: '#000057', rarity: 'rare', species: 'dog', unlocked: true, captureDate: '2026-07-06', location: '江北区·滨江', lat: 29.90, lng: 121.53, seed: 3 },
  { id: 'c004', no: '#000056', rarity: 'common', species: 'cat', unlocked: true, captureDate: '2026-07-05', location: '海曙区·老街', lat: 29.86, lng: 121.54, seed: 4 },
  { id: 'c005', no: '#000055', rarity: 'epic', species: 'goose', unlocked: true, captureDate: '2026-07-04', location: '镇海区·山林', lat: 29.95, lng: 121.60, seed: 5 },
  { id: 'c006', no: '#000054', rarity: 'common', species: 'dog', unlocked: true, captureDate: '2026-07-03', location: '鄞州区·广场', lat: 29.82, lng: 121.58, seed: 6 },
  { id: 'c007', no: '#000014', rarity: 'legendary', species: 'cat', unlocked: true, captureDate: '2026-07-01', location: '海曙区·月湖', lat: 29.87, lng: 121.55, seed: 7 },
  { id: 'c008', no: '#???', rarity: 'common', species: 'goose', unlocked: false, captureDate: '—', location: '待发现', lat: 0, lng: 0, seed: 8 },
  { id: 'c009', no: '#???', rarity: 'common', species: 'dog', unlocked: false, captureDate: '—', location: '待发现', lat: 0, lng: 0, seed: 9 },
]
```

### 4.2 CollectScreen 物种分组

**文件**: `src/components/CollectScreen.tsx` — 修改内容：

1. 在现有 `FilterTab` 基础上，**新增物种筛选 tab**：
```typescript
// 新增物种筛选类型
type SpeciesFilter = 'all' | SpeciesType

// Tab 配置追加
const speciesTabs: { key: SpeciesFilter; label: string; emoji: string }[] = [
  { key: 'all', label: '全部', emoji: '📖' },
  { key: 'cat', label: '猫', emoji: '🐱' },
  { key: 'goose', label: '鹅', emoji: '🪿' },
  { key: 'dog', label: '狗', emoji: '🐶' },
]
```

2. 卡片上显示物种 emoji：
```tsx
// 在 cardPhoto 区域显示物种 emoji
<div style={{ ...styles.cardPhoto, background: cardGradient(entry.seed) }}>
  <span style={{ fontSize: 28 }}>{SPECIES_DEFS[entry.species || 'cat'].emoji}</span>
  {entry.isNew && <span style={styles.newBadge}>NEW</span>}
</div>
```

3. 标题行显示各物种计数：
```tsx
// 统计各物种已捕获数量
const speciesCounts = useMemo(() => {
  const counts: Record<string, number> = { cat: 0, goose: 0, dog: 0 }
  animals.filter(e => e.unlocked).forEach(e => {
    const s = getCardSpecies(e)
    if (counts[s] !== undefined) counts[s]++
  })
  return counts
}, [animals])

// 渲染
<div style={styles.speciesCounts}>
  <span>🐱 {speciesCounts.cat}</span>
  <span>🪿 {speciesCounts.goose}</span>
  <span>🐶 {speciesCounts.dog}</span>
</div>
```

4. 物种筛选逻辑（在现有 `filtered` useMemo 中叠加）：
```typescript
const filtered = useMemo(() => {
  return animals.filter(e => {
    // 现有筛选逻辑...
    if (!e.unlocked && filter !== 'all') return false
    switch (filter) {
      case 'today': return e.captureDate === todayStr
      case 'week': return e.captureDate >= weekStart && e.captureDate <= todayStr
      case 'nearby': return e.location.includes('海曙区')
      default: return true
    }
    // 物种筛选
    if (speciesFilter !== 'all' && getCardSpecies(e) !== speciesFilter) return false
    return true
  })
}, [filter, speciesFilter, animals])
```

### 4.3 DetailPopup 物种信息

**文件**: `src/components/DetailPopup.tsx` — 新增一行显示物种：

```tsx
<div style={styles.metaRow}>
  <span>{SPECIES_DEFS[getCardSpecies(entry)].emoji}</span>
  <span style={styles.metaLabel}>物种</span>
  <span style={styles.metaValue}>{SPECIES_DEFS[getCardSpecies(entry)].name}</span>
</div>
```

---

## 5. CaptureScreen 物种适配

### 5.1 Props 扩展

```typescript
export interface CaptureScreenProps {
  /** 当前目标物种 */
  targetSpecies?: SpeciesType       // 新增，默认 'cat'
  onCaptureSuccess?: (entry: CardEntry) => void
  onCaptureFail?: () => void
}
```

### 5.2 物种依赖的动态值

```typescript
// 根据 targetSpecies 获取物种定义
const def = SPECIES_DEFS[targetSpecies || 'cat']

// 使用 def 的字段替换所有硬编码：
// 🐱 → def.emoji
// 🥫 → def.throwItemEmoji
// "投掷" → def.throwItemEmoji + " 投掷"
// CHARGE_RATE → def.chargeRate  (替换全局常量)
// [OPTIMAL_MIN, OPTIMAL_MAX] → def.optimalRange  (替换全局常量)
```

### 5.3 手感差异实现

**核心思路**：不同物种只调整两个参数，不需要物理引擎：

| 物种 | chargeRate | optimalRange | 手感体验 |
|------|-----------|-------------|---------|
| 猫 | 2 (标准) | [40, 80] | 标准抛物线 —— 充能匀速，最佳区间居中 |
| 鹅 | 2.5 (快) | [35, 75] | 弹跳略强 —— 充能快，需要更快松手；最佳区间宽且偏左 |
| 狗 | 1.5 (慢) | [45, 85] | 下落更快 —— 充能慢，需要更多耐心；最佳区间偏右且窄 |

**具体代码变更**：

删除全局常量：
```typescript
// 删除
const CHARGE_RATE = 2
const OPTIMAL_MIN = 40
const OPTIMAL_MAX = 80
```

改为从 `def` 读取：
```typescript
const chargeRate = def.chargeRate
const [optimalMin, optimalMax] = def.optimalRange
```

### 5.4 generateCardEntry 更新

```typescript
/** 生成随机 CardEntry（含物种） */
function generateCardEntry(species: SpeciesType): CardEntry {
  return {
    id: `c_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
    no: `#${String(Math.floor(Math.random() * 60 + 1)).padStart(6, '0')}`,
    rarity: randomRarity(),
    species,                              // ← 新增
    unlocked: true,
    captureDate: new Date().toISOString().split('T')[0],
    location: '宁波·未知区域',
    lat: 29.87 + (Math.random() - 0.5) * 0.05,
    lng: 121.55 + (Math.random() - 0.5) * 0.05,
    seed: Math.floor(Math.random() * 1000),
    isNew: true,
  }
}
```

### 5.5 UI 变化汇总

| 状态 | 当前（猫硬编码） | 改造后 |
|------|---------------|--------|
| 目标动物 emoji | `🐱` | `def.emoji` (🐱/🪿/🐶) |
| 投掷按钮文字 | `🥫 投掷` | `{def.throwItemEmoji} 投掷` |
| 食物库存 pill | `🥫 ×8` | `{def.throwItemEmoji} ×8` |
| 投掷动画 emoji | `🥫` | `def.throwItemEmoji` |
| 充能速率 | 固定 2%/tick | `def.chargeRate` |
| 最佳区间 | 固定 [40, 80] | `def.optimalRange` |
| 投掷力度条提示 | 无物种说明 | 显示物种名 + 手感提示 |

---

## 6. DiscoverScreen 检测集成

### 6.1 数据流

```
拍照按钮 → capturePhoto() → photoData (base64)
  → onConfirm(photoData) → App.tsx
    → setPendingPhoto(photoData); setActiveTab('fight')
      → CaptureScreen 显示

【改造后新增】
拍照按钮 → capturePhoto() → photoData
  → 调用 mockVisionDetector.detect(photoData)
    → 显示检测结果（物种 emoji + 置信度百分比）
      ├─ confidence ≥ 0.85 → "发现！" → onConfirm(photoData, species) → 进入捕获
      └─ confidence < 0.85 → "未发现动物" → 可重新拍照
```

### 6.2 Props 变更

```typescript
interface DiscoverScreenProps {
  onConfirm?: (photoData: string, species: SpeciesType) => void  // 新增 species 参数
}
```

### 6.3 检测状态机

新增状态：
```typescript
type DetectionState = 'detecting' | 'detected' | 'not_found' | 'error'
```

状态流转：
```
captured → [调用 mockVisionDetector] → detecting
  → 检测返回
    ├─ confidence ≥ 0.85 → detected（显示物种信息，可进入捕获）
    ├─ confidence < 0.85 → not_found（显示"未发现动物"，可重拍）
    └─ 网络错误 → error（显示错误提示，可重试）
```

### 6.4 UI 设计

**检测中状态** (`detecting`)：
```
┌─────────────────────────┐
│                         │
│     [照片预览]           │
│                         │
│     🔍 扫描中…           │
│     □□□□□□□□□□          │
│     (脉冲动画)           │
│                         │
│   [取消]  [重拍]         │
└─────────────────────────┘
```

**检测到动物** (`detected`)：
```
┌─────────────────────────┐
│                         │
│     [照片预览]           │
│                         │
│ ┌─────────────────────┐ │
│ │ 🐱 猫 · 置信度 97%  │ │
│ └─────────────────────┘ │
│                         │
│   [重拍]  [🐾 开始捕获]  │
└─────────────────────────┘
```

**未发现动物** (`not_found`)：
```
┌─────────────────────────┐
│                         │
│     [照片预览]           │
│                         │
│ ┌─────────────────────┐ │
│ │ 😿 未发现动物        │ │
│ │ 请换个角度再试        │ │
│ └─────────────────────┘ │
│                         │
│        [📷 重拍]         │
└─────────────────────────┘
```

### 6.5 App.tsx 数据流适配

```typescript
// DiscoverScreen → App → CaptureScreen 新增 species 传递

// AppInner 新增状态
const [pendingSpecies, setPendingSpecies] = useState<SpeciesType>('cat')

// handlePhotoConfirm 签名改为:
const handlePhotoConfirm = useCallback((photoData: string, species: SpeciesType) => {
  setPendingPhoto(photoData)
  setPendingSpecies(species)
  setActiveTab('fight')   // fight tab 渲染 CaptureScreen
}, [])

// CaptureScreen 渲染时传入:
<CaptureScreen
  targetSpecies={pendingSpecies}
  onCaptureSuccess={handleCaptureSuccess}
  onCaptureFail={handleCaptureFail}
/>
```

---

## 7. 文件清单与依赖关系

```
src/types.ts
  ├─ 新增 SpeciesType, SpeciesDef, SPECIES_DEFS, getCardSpecies()
  ├─ CardEntry 新增 species?: SpeciesType
  └─ MOCK_ENTRIES 新增 species 字段
      │
src/services/visionDetect.ts  ← 新建（独立模块，无依赖）
  ├─ DetectionResult 接口
  ├─ VisionDetector 接口
  └─ mockVisionDetector 实现
      │
src/components/DiscoverScreen.tsx
  ├─ 导入 mockVisionDetector
  ├─ 新增 detection state machine
  ├─ 拍照后调用 detect() → 门槛判断
  └─ onConfirm 新增 species 参数
      │
src/components/CaptureScreen.tsx
  ├─ 新增 targetSpecies prop
  ├─ 从 SPECIES_DEFS 读取手感参数
  ├─ generateCardEntry 新增 species 参数
  └─ UI: emoji / 投掷物根据 species 切换
      │
src/components/CollectScreen.tsx
  ├─ 新增物种筛选 tab
  ├─ 卡片显示物种 emoji
  └─ 标题行显示各物种计数
      │
src/components/DetailPopup.tsx
  └─ 新增物种信息展示行
      │
src/App.tsx
  ├─ 新增 pendingSpecies 状态
  └─ 传递 species 给 CaptureScreen
```

---

## 8. 测试用例清单（共 25 个）

### 8.1 types.test.ts（5 个）— 新建

| # | 测试用例 | 断言 |
|---|---------|------|
| 1 | SPECIES_DEFS 包含全部三种物种 | keys 为 ['cat', 'goose', 'dog'] |
| 2 | 所有 SpeciesDef 包含必需字段 | emoji, name, throwItem, throwItemEmoji 非空 |
| 3 | getCardSpecies 无 species 字段时返回 'cat' | getCardSpecies({}) → 'cat' |
| 4 | getCardSpecies 有 species 时正确返回 | getCardSpecies({ species: 'dog' }) → 'dog' |
| 5 | CardEntry 兼容不含 species 的旧数据 | TypeScript 编译通过 |

### 8.2 visionDetect.test.ts（5 个）— 新建

| # | 测试用例 | 断言 |
|---|---------|------|
| 6 | detect 返回包含 species 的结果 | result.species in ['cat','goose','dog'] |
| 7 | detect 返回置信度在 0~1 范围 | result.confidence >= 0 && <= 1 |
| 8 | detect 返回有效 boundingBox | bbox 四个值都在 0~1 范围 |
| 9 | detect 模拟网络延迟 | 调用耗时 ≥ 300ms |
| 10 | 多次调用返回结果具有随机性 | 3 次调用不完全相同 |

### 8.3 discover-screen.test.tsx（5 个）— 新建

| # | 测试用例 | 断言 |
|---|---------|------|
| 11 | 拍照后触发检测流程 | camera state → 'captured' → detection state → 'detecting' |
| 12 | 置信度 ≥ 0.85 时显示"发现"并允许进入捕获 | "开始捕获"按钮可见，物种 emoji 显示 |
| 13 | 置信度 < 0.85 时显示"未发现动物" | "未发现动物"文案可见，"开始捕获"按钮不可见 |
| 14 | 检测失败时显示错误提示 + 重试按钮 | 错误 UI 渲染，点击重试重新调用 detect |
| 15 | mockVisionDetector 被调用时传入了 photoData | detect 参数包含 base64 数据 |

### 8.4 capture-screen.test.tsx（5 个）— 追加到现有文件

| # | 测试用例 | 断言 |
|---|---------|------|
| 16 | cat 物种渲染 🐱 emoji + 🥫 投掷按钮 | screen 包含 '🐱' 和 '🥫' |
| 17 | goose 物种渲染 🪿 emoji + 🍞 投掷按钮 | screen 包含 '🪿' 和 '🍞' |
| 18 | dog 物种渲染 🐶 emoji + 🦴 投掷按钮 | screen 包含 '🐶' 和 '🦴' |
| 19 | 捕获成功后 generateCardEntry 包含 species 字段 | entry.species === targetSpecies |
| 20 | 不同物种使用不同的充能速率 | goose(2.5) charge 增长快于 dog(1.5) |

### 8.5 collect-screen.test.tsx（5 个）— 新建

| # | 测试用例 | 断言 |
|---|---------|------|
| 21 | 物种筛选 tab 全部渲染 | '全部' '猫' '鹅' '狗' 四个 tab 可见 |
| 22 | 选择"猫"tab 后仅显示猫物种卡片 | filtered entries 全部 species='cat' |
| 23 | 选择"鹅"tab 后仅显示鹅物种卡片 | filtered entries 全部 species='goose' |
| 24 | 卡片上显示对应物种 emoji | cat 卡显示 🐱, goose 卡显示 🪿, dog 卡显示 🐶 |
| 25 | 物种计数统计正确 | speciesCounts 与实际 unlocked 数据一致 |

---

## 9. 验收标准对照

| # | 设计文档要求 | 实现对照 |
|---|------------|---------|
| 1 | 首期支持猫/鹅/狗三类 | SPECIES_DEFS 包含 cat/goose/dog 三条 |
| 2 | 架构预留可扩展更多物种 | SpeciesType + SPECIES_DEFS，新增物种仅加一条条目 |
| 3 | 猫=猫粮罐·标准抛物线 | cat: 🥫, chargeRate=2, optimalRange=[40,80] |
| 4 | 鹅=面包屑球·弹跳略强 | goose: 🍞, chargeRate=2.5, optimalRange=[35,75] |
| 5 | 狗=骨头零食·下落更快 | dog: 🦴, chargeRate=1.5, optimalRange=[45,85] |
| 6 | VLM 检测置信度≥0.85触发发现 | mockVisionDetect.confidence ≥ 0.85 → "发现"状态 |
| 7 | <0.85 不进入捕获 | confidence < 0.85 → "未发现动物"提示 |
| 8 | 弱网降级为扫描式交互 | mock 模拟 300~1200ms 延迟；真实 API 接口预留 |
| 9 | 捕获小游戏物种适配 | CaptureScreen 根据 targetSpecies 切换 emoji+手感参数 |
| 10 | 物种信息在图鉴可见 | CollectScreen 物种分组+emoji，DetailPopup 显示物种 |
| 11 | 三类动物检测准确率达标 | 留给后端；前端 mock 保证 3 物种均匀随机 |
| 12 | 不修改 StaminaContext/ShopContext 内部实现 | plan 中明确标注不改文件清单 |

---

## 10. 实施顺序（建议）

```
Phase 1: 类型层（types.ts）
  └─ 新增 SpeciesType, SpeciesDef, SPECIES_DEFS
  └─ CardEntry 加 species 字段 + getCardSpecies()
  └─ MOCK_ENTRIES 加 species

Phase 2: Mock 检测层（services/visionDetect.ts）
  └─ DetectionResult + VisionDetector 接口
  └─ mockVisionDetector 实现

Phase 3: DiscoverScreen 集成
  └─ 拍照后调用 mockVisionDetector
  └─ 检测状态机 + UI 三种状态

Phase 4: CaptureScreen 物种适配
  └─ targetSpecies prop
  └─ 从 SPECIES_DEFS 读取手感参数
  └─ generateCardEntry 加 species

Phase 5: App.tsx 数据流串联
  └─ pendingSpecies 状态传递

Phase 6: CollectScreen + DetailPopup
  └─ 物种筛选 + 计数 + emoji 展示

Phase 7: 测试
  └─ 5 个测试文件，25 个用例
```

---

## 11. 风险与注意事项

| 风险 | 缓解措施 |
|------|---------|
| CardEntry 旧数据无 species 字段 | `species?: SpeciesType` + `getCardSpecies()` 默认 'cat' |
| IndexedDB schema 兼容 | species 可选，存量数据自动 fallback，无需迁移 |
| mock 替换为真实 API 时遗漏 | VisionDetector 接口统一入口，全局搜 `mockVisionDetector` 即可定位所有调用点 |
| 物种手感差异不明显 | chargeRate 1.5/2/2.5 差距足够，内测反馈后可微调 |
| 物种筛选与日期筛选交互复杂 | 两个维度独立 filter，AND 逻辑叠加，简单清晰 |

---

*本计划基于设计文档 v1.4 及当前 `frontend/src/` 代码现状编写。*
