# Issue #32 实现计划：本地存储系统

> GitHub Issue #32 · [M1] 本地存储
> 验收标准：动物数据可持久化，重启不丢失

---

## 1. 技术选型：IndexedDB 封装库

### 推荐：`idb`（轻量 Promise 封装）

| 候选 | 体积 | 优点 | 缺点 | 结论 |
|------|------|------|------|------|
| **idb** | ~1.2KB gzipped | 原生 IndexedDB 的 Promise 封装，API 透明，无魔法，学习成本极低 | 需手动管理事务/索引 | **MVP 首选** |
| dexie | ~20KB gzipped | 功能丰富（liveQuery、hooks、ORM 风格） | 体积大，抽象层厚，MVP 用不到高级特性 | 内测阶段再评估 |
| 原生 IndexedDB | 0KB | 无依赖 | 回调地狱，代码冗长，易出错 | 不推荐 |

**选择 idb 的理由**：
1. 体积仅 1.2KB，不增加包体负担（设计文档要求安装包 ≤ 150MB）
2. API 几乎是原生 IndexedDB 的 1:1 Promise 映射，没有额外抽象层，调试方便
3. MVP 阶段数据操作简单（CRUD），不需要 Dexie 的 liveQuery / ORM 能力
4. 后续若需要响应式查询（如 Dexie 的 `useLiveQuery`），可平滑切换到 Dexie 或自行封装

### 依赖安装

```bash
cd frontend
npm install idb
```

---

## 2. 数据库结构

### 2.1 DB 基本信息

| 属性 | 值 |
|------|------|
| DB 名称 | `animal-poke-db` |
| 版本 | `1`（MVP 首版，后续按需升级） |

### 2.2 Object Stores 设计

```
animal-poke-db (v1)
├── animals          ← 动物收藏数据（多条记录）
├── player           ← 玩家进度（单条记录，keyPath 固定为 "profile"）
├── economy          ← 经济数据（单条记录，keyPath 固定为 "wallet"）
└── settings         ← 设置数据（单条记录，keyPath 固定为 "prefs"）
```

### 2.3 各 Store 详细结构

#### `animals` store — 动物收藏

```typescript
// keyPath: "id"
interface CardEntry {
  id: string          // 唯一 ID（UUID）
  no: string          // 图鉴编号，如 "#000059"
  rarity: RarityTier  // 稀有度
  unlocked: boolean   // 是否已解锁
  captureDate: string // 捕获日期，如 "2026-07-08"（ISO 格式，便于排序/查询）
  location: string    // 捕获地点描述
  lat: number         // 纬度
  lng: number         // 经度
  seed: number        // 随机种子（用于生成外观/属性）
  isNew?: boolean     // 是否为新捕获（未查看过）
  // 后续可扩展：species, breed, stats, status 等
}
```

**索引设计**：

| 索引名 | keyPath | unique | 用途 |
|--------|---------|--------|------|
| `by-date` | `captureDate` | false | 按日期筛选（今日/本周） |
| `by-rarity` | `rarity` | false | 按稀有度筛选 |
| `by-unlocked` | `unlocked` | false | 快速查询已解锁/未解锁 |

#### `player` store — 玩家进度

```typescript
// keyPath: "key"，固定值 "profile"
interface PlayerProfile {
  key: 'profile'
  level: number           // 当前等级 1~10
  totalCaptured: number   // 累计捕获数
  stamina: number          // 当前体力值
  maxStamina: number       // 体力上限
  lastStaminaUpdate: number // 体力上次恢复时间戳（ms）
  // 后续可扩展：exp, achievements 等
}
```

> 单条记录 store，用固定 keyPath 值实现"唯一行"模式。读写简单，无需索引。

#### `economy` store — 经济数据

```typescript
// keyPath: "key"，固定值 "wallet"
interface EconomyWallet {
  key: 'wallet'
  gold: number             // 金币余额
  items: Record<string, number> // 道具背包 { itemId: count }
  lastSignDate: string | null   // 上次签到日期（ISO）
  signStreak: number       // 连续签到天数
  // 后续可扩展：diamonds 等
}
```

#### `settings` store — 设置数据

```typescript
// keyPath: "key"，固定值 "prefs"
interface AppSettings {
  key: 'prefs'
  soundEnabled: boolean    // 音效开关
  musicEnabled: boolean    // 背景音乐开关
  privacyConsented: boolean // 隐私协议是否已同意
  // 后续可扩展：language, notifications 等
}
```

### 2.4 DB 初始化代码（db.ts 核心片段）

```typescript
import { openDB, type IDBPDatabase } from 'idb'
import type { CardEntry, RarityTier } from '../types'

// 动物收藏记录的 DB 版本（与 CardEntry 对齐）
interface AnimalRecord extends CardEntry {}

export const DB_NAME = 'animal-poke-db'
export const DB_VERSION = 1

let dbPromise: Promise<IDBPDatabase> | null = null

/** 获取 DB 实例（单例，惰性初始化） */
export function getDB(): Promise<IDBPDatabase> {
  if (!dbPromise) {
    dbPromise = openDB(DB_NAME, DB_VERSION, {
      upgrade(db) {
        // animals store：带索引
        if (!db.objectStoreNames.contains('animals')) {
          const store = db.createObjectStore('animals', { keyPath: 'id' })
          store.createIndex('by-date', 'captureDate')
          store.createIndex('by-rarity', 'rarity')
          store.createIndex('by-unlocked', 'unlocked')
        }
        // player / economy / settings：单条记录 store
        if (!db.objectStoreNames.contains('player')) {
          db.createObjectStore('player', { keyPath: 'key' })
        }
        if (!db.objectStoreNames.contains('economy')) {
          db.createObjectStore('economy', { keyPath: 'key' })
        }
        if (!db.objectStoreNames.contains('settings')) {
          db.createObjectStore('settings', { keyPath: 'key' })
        }
      },
    })
  }
  return dbPromise
}
```

---

## 3. 加密方案

### MVP 阶段：不加密

**结论**：MVP 阶段**不需要加密**，直接明文存储。理由如下：

| 考量 | 分析 |
|------|------|
| 浏览器沙箱 | IndexedDB 受同源策略保护，其他网站无法访问 |
| MVP 数据 | 仅动物元数据 + 玩家进度，无敏感个人信息（不含照片、不含密码） |
| 反作弊 | MVP 阶段无 PvP / 排行，篡改本地数据不影响他人 |
| 开发成本 | Web Crypto API + 密钥管理增加 ~200 行代码和调试复杂度 |
| KISS 原则 | 加密是内测阶段（#33+）反作弊需求，MVP 阶段过早引入 |

### 后续加密规划（内测阶段，非本次实现）

当引入 PvP 排位和区域排行时，需防止本地数据篡改：

```
方案：Web Crypto API + AES-GCM
- 密钥：首次启动生成 CryptoKey，存于 IndexedDB 单独 store（key 仅在当前设备/浏览器有效）
- 加密粒度：整条记录 JSON → encrypt → 存密文；读取时 decrypt → JSON.parse
- 注意：密钥本身存浏览器，理论上可被提取，但提高了篡改门槛
- 替代方案：关键数值（金币、等级）由后端签发 HMAC 签名，客户端只读
```

> **建议**：内测阶段引入后端同步后，关键数值（金币、等级、体力）以服务端为准，本地仅做缓存。加密问题自然消解为"签名校验"问题。

---

## 4. Repository 模式：数据访问层设计

### 4.1 架构概览

```
React 组件
    ↕（自定义 hook）
useAnimalStore / usePlayerStore / useEconomyStore
    ↕（Repository 调用）
AnimalRepository    PlayerRepository    EconomyRepository    SettingsRepository
    ↕（idb 封装）
IndexedDB (animal-poke-db)
```

**设计原则**：
- Repository 封装所有 IndexedDB 操作，组件不直接调用 `idb`
- Repository 方法返回 Promise，hook 层负责状态管理
- 每个 Repository 对应一个数据域，职责单一

### 4.2 各 Repository 接口定义

#### AnimalRepository

```typescript
class AnimalRepository {
  /** 获取所有动物 */
  getAll(): Promise<CardEntry[]>
  /** 按 ID 获取单个动物 */
  getById(id: string): Promise<CardEntry | undefined>
  /** 仅获取已解锁的动物 */
  getUnlocked(): Promise<CardEntry[]>
  /** 按日期范围筛选 */
  getByDateRange(start: string, end: string): Promise<CardEntry[]>
  /** 新增一只动物（捕获成功后调用） */
  add(entry: CardEntry): Promise<void>
  /** 更新动物信息 */
  update(entry: CardEntry): Promise<void>
  /** 标记为已查看（isNew = false） */
  markViewed(id: string): Promise<void>
  /** 删除动物 */
  delete(id: string): Promise<void>
  /** 获取已解锁总数 */
  countUnlocked(): Promise<number>
}
```

#### PlayerRepository

```typescript
class PlayerRepository {
  /** 获取玩家档案（不存在则返回默认值） */
  get(): Promise<PlayerProfile>
  /** 保存玩家档案（整体替换） */
  save(profile: PlayerProfile): Promise<void>
  /** 增加捕获数（并检查升级） */
  incrementCapture(): Promise<PlayerProfile>
  /** 消耗体力 */
  consumeStamina(amount: number): Promise<PlayerProfile>
  /** 恢复体力（自然恢复 + 升级恢复） */
  recoverStamina(): Promise<PlayerProfile>
}
```

#### EconomyRepository

```typescript
class EconomyRepository {
  /** 获取钱包数据 */
  get(): Promise<EconomyWallet>
  /** 保存钱包数据 */
  save(wallet: EconomyWallet): Promise<void>
  /** 增减金币（正数增加，负数消耗） */
  addGold(amount: number): Promise<EconomyWallet>
  /** 添加道具 */
  addItem(itemId: string, count: number): Promise<void>
  /** 消耗道具 */
  removeItem(itemId: string, count: number): Promise<boolean>
  /** 签到 */
  sign(): Promise<{ success: boolean; reward: number; streak: number }>
}
```

#### SettingsRepository

```typescript
class SettingsRepository {
  /** 获取设置 */
  get(): Promise<AppSettings>
  /** 保存设置 */
  save(settings: Partial<AppSettings>): Promise<void>
}
```

### 4.3 默认数据工厂

```typescript
// 新玩家初始数据
function createDefaultPlayer(): PlayerProfile {
  return {
    key: 'profile',
    level: 1,
    totalCaptured: 0,
    stamina: 120,
    maxStamina: 120,
    lastStaminaUpdate: Date.now(),
  }
}

function createDefaultEconomy(): EconomyWallet {
  return {
    key: 'wallet',
    gold: 0,
    items: {},
    lastSignDate: null,
    signStreak: 0,
  }
}

function createDefaultSettings(): AppSettings {
  return {
    key: 'prefs',
    soundEnabled: true,
    musicEnabled: true,
    privacyConsented: false,
  }
}
```

---

## 5. 与 React 集成

### 5.1 方案：StorageProvider Context + 自定义 Hook

**选择理由**：
- player / economy / settings 是全局状态（TopBar、多页面共享），适合 Context
- animals 是列表数据，适合页面级 hook 按需加载
- 不引入 Redux / Zustand 等状态库，保持 KISS

### 5.2 StorageProvider 设计

```typescript
// StorageProvider.tsx
interface StorageContextValue {
  player: PlayerProfile | null
  economy: EconomyWallet | null
  settings: AppSettings | null
  loading: boolean         // 首次加载中
  // 更新方法（调用 Repository 后同步更新 Context state）
  updatePlayer: (updater: (p: PlayerProfile) => PlayerProfile) => Promise<void>
  updateEconomy: (updater: (e: EconomyWallet) => EconomyWallet) => Promise<void>
  updateSettings: (updater: (s: AppSettings) => AppSettings) => Promise<void>
}
```

**App.tsx 集成方式**：

```tsx
// App.tsx（改造后）
const App: React.FC = () => {
  return (
    <StorageProvider>
      <AppContent />
    </StorageProvider>
  )
}

const AppContent: React.FC = () => {
  const { player, economy, loading } = useStorage()
  if (loading) return <LoadingScreen />
  return (
    <div className="phone-frame">
      <TopBar
        level={player.level}
        stamina={player.stamina}
        maxStamina={player.maxStamina}
        gold={economy.gold}
        location="宁波·晴"
        weather="☀️"
      />
      {/* ... */}
    </div>
  )
}
```

### 5.3 自定义 Hook

#### useAnimalStore — 动物收藏

```typescript
function useAnimalStore() {
  const [animals, setAnimals] = useState<CardEntry[]>([])
  const [loading, setLoading] = useState(true)

  // 初始化加载
  useEffect(() => {
    animalRepo.getAll().then(list => {
      setAnimals(list)
      setLoading(false)
    })
  }, [])

  // 捕获新动物
  const addAnimal = useCallback(async (entry: CardEntry) => {
    await animalRepo.add(entry)
    setAnimals(prev => [...prev, entry])
  }, [])

  // 标记已查看
  const markViewed = useCallback(async (id: string) => {
    await animalRepo.markViewed(id)
    setAnimals(prev => prev.map(a => a.id === id ? { ...a, isNew: false } : a))
  }, [])

  return { animals, loading, addAnimal, markViewed }
}
```

#### usePlayerStore / useEconomyStore

这两个通过 `useStorage()` Context 获取，无需单独 hook。若后续需要更细粒度的逻辑（如体力定时恢复），再抽取为独立 hook。

### 5.4 体力定时恢复（与 #31 协调）

```typescript
// useStaminaRecovery.ts
function useStaminaRecovery() {
  const { player, updatePlayer } = useStorage()

  useEffect(() => {
    if (!player) return
    // 每 6 分钟恢复 1 点体力
    const interval = setInterval(async () => {
      const now = Date.now()
      const elapsed = now - player.lastStaminaUpdate
      const recovered = Math.floor(elapsed / (6 * 60 * 1000)) // 每 6 分钟 +1
      if (recovered > 0 && player.stamina < player.maxStamina) {
        await updatePlayer(p => ({
          ...p,
          stamina: Math.min(p.maxStamina, p.stamina + recovered),
          lastStaminaUpdate: now,
        }))
      }
    }, 60 * 1000) // 每分钟检查一次

    return () => clearInterval(interval)
  }, [player, updatePlayer])
}
```

---

## 6. 迁移策略：localStorage → IndexedDB

### 6.1 背景

Issue #31（体力系统）可能使用 `localStorage` 做 MVP 持久化。#32 需要将数据迁移到 IndexedDB，并确保 #31 的体力数据不丢失。

### 6.2 迁移方案

在 `StorageProvider` 初始化时执行一次性迁移：

```typescript
// migrateFromLocalStorage.ts

const MIGRATION_KEY = 'animal-poke:migrated-v1'
const STAMINA_KEY = 'animal-poke:stamina'      // #31 使用的 localStorage key
const PLAYER_KEY = 'animal-poke:player'         // #31 可能使用的其他 key

/** 从 localStorage 迁移到 IndexedDB（仅执行一次） */
async function migrateFromLocalStorage(): Promise<void> {
  // 检查是否已迁移
  if (localStorage.getItem(MIGRATION_KEY)) return

  const db = await getDB()

  // 1. 迁移体力数据
  const staminaData = localStorage.getItem(STAMINA_KEY)
  if (staminaData) {
    const { stamina, lastUpdate } = JSON.parse(staminaData)
    // 读取或创建 player 记录，合并体力数据
    const existing = await db.get('player', 'profile')
    await db.put('player', {
      ...createDefaultPlayer(),
      ...existing,
      stamina,
      lastStaminaUpdate: lastUpdate ?? Date.now(),
    })
    localStorage.removeItem(STAMINA_KEY)
  }

  // 2. 迁移其他 localStorage key（如有）
  // ...

  // 3. 标记迁移完成
  localStorage.setItem(MIGRATION_KEY, new Date().toISOString())
}
```

### 6.3 迁移时序

```
App 启动
  │
  ▼
StorageProvider 初始化
  │
  ├─ 1. 打开 IndexedDB
  ├─ 2. 执行 migrateFromLocalStorage()
  │     ├─ 检查 migration 标记 → 已迁移则跳过
  │     ├─ 读取 localStorage 旧数据
  │     ├─ 写入 IndexedDB
  │     └─ 标记迁移完成
  ├─ 3. 从 IndexedDB 加载 player / economy / settings
  └─ 4. 设置 loading = false，渲染 UI
```

### 6.4 迁移注意事项

- 迁移是幂等的（多次执行不会重复迁移，有标记位）
- 迁移失败不阻塞启动（catch 错误，使用默认数据）
- #31 若尚未实现或未使用 localStorage，迁移函数自动跳过（无数据可迁）
- 迁移后 `localStorage.removeItem` 清理旧数据，避免占用空间

---

## 7. 测试方案

### 7.1 测试工具

| 工具 | 用途 | 安装 |
|------|------|------|
| **Vitest** | 测试框架（与 Vite 原生集成） | `npm i -D vitest` |
| **fake-indexeddb** | Node.js 环境模拟 IndexedDB | `npm i -D fake-indexeddb` |

### 7.2 安装与配置

```bash
npm i -D vitest fake-indexeddb
```

`vitest.config.ts`（或合并到 `vite.config.ts`）：

```typescript
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/db/test-setup.ts'],
  },
})
```

`src/db/test-setup.ts`：

```typescript
import 'fake-indexeddb/auto'
// 每个 test 文件执行前清理 DB
import { beforeEach } from 'vitest'
import { DB_NAME } from './db'

beforeEach(() => {
  // fake-indexeddb 会自动创建独立的 DB 实例
  indexedDB.deleteDatabase(DB_NAME)
})
```

### 7.3 测试用例规划

#### Repository 层测试（核心）

```
test/animal-repository.test.ts
├─ add() → 写入后可 getById() 读到
├─ getAll() → 返回全部动物，按时间倒序
├─ getUnlocked() → 仅返回 unlocked=true 的记录
├─ getByDateRange() → 日期筛选正确
├─ markViewed() → isNew 从 true 变为 false
├─ delete() → 删除后 getById 返回 undefined
└─ countUnlocked() → 计数正确

test/player-repository.test.ts
├─ get() → 无数据时返回默认值
├─ save() → 写入后可读回
├─ incrementCapture() → totalCaptured +1，触发升级检查
├─ consumeStamina() → 体力减少，不足时抛错
└─ recoverStamina() → 按时间差恢复，不超过上限

test/economy-repository.test.ts
├─ get() → 无数据时返回默认值
├─ addGold() → 正数增加，负数减少，余额不足时拒绝
├─ addItem() / removeItem() → 背包增减正确
└─ sign() → 首次签到、连续签到、断签重置

test/migration.test.ts
├─ 有 localStorage 数据 → 迁移到 IndexedDB 后 localStorage 清空
├─ 无 localStorage 数据 → 跳过迁移，使用默认值
└─ 重复执行迁移 → 幂等，不重复写入
```

#### Hook / Provider 测试（可选，MVP 可跳过）

```
test/useAnimalStore.test.ts
├─ 初始化加载已有数据
├─ addAnimal 后列表更新
└─ markViewed 后 isNew 状态更新
```

### 7.4 验收标准手动验证

在浏览器 DevTools 中验证：
1. Application → IndexedDB → `animal-poke-db` → `animals` store 有数据
2. 刷新页面后，图鉴列表数据不丢失
3. 清空 IndexedDB 后刷新，回退到默认数据（不崩溃）

---

## 8. 文件结构

### 8.1 需要创建的文件

```
frontend/src/
├── db/
│   ├── db.ts                        # IndexedDB 连接 + schema 定义
│   ├── types.ts                     # DB 相关类型（PlayerProfile, EconomyWallet, AppSettings）
│   ├── repositories/
│   │   ├── animal-repository.ts     # 动物收藏 CRUD
│   │   ├── player-repository.ts     # 玩家进度 CRUD
│   │   ├── economy-repository.ts    # 经济数据 CRUD
│   │   └── settings-repository.ts   # 设置 CRUD
│   ├── migrate-from-localstorage.ts # localStorage → IndexedDB 迁移
│   └── test-setup.ts                # 测试环境配置（fake-indexeddb）
├── hooks/
│   ├── useStorage.ts                # StorageProvider Context + useStorage hook
│   ├── useAnimalStore.ts            # 动物收藏 hook
│   └── useStaminaRecovery.ts        # 体力定时恢复 hook
├── components/
│   └── LoadingScreen.tsx            # 数据加载占位屏
├── test/
│   ├── animal-repository.test.ts
│   ├── player-repository.test.ts
│   ├── economy-repository.test.ts
│   └── migration.test.ts
└── App.tsx                          # 【修改】包裹 StorageProvider

frontend/
├── vitest.config.ts                 # 测试配置（或合并到 vite.config.ts）
└── package.json                     # 【修改】新增 idb, vitest, fake-indexeddb 依赖
```

### 8.2 需要修改的文件

| 文件 | 修改内容 |
|------|---------|
| `App.tsx` | 包裹 `<StorageProvider>`，TopBar 改为从 Context 获取数据 |
| `CollectScreen.tsx` | 从 `MOCK_ENTRIES` 改为 `useAnimalStore()` hook |
| `types.ts` | `captureDate` 格式从 `"07.08"` 改为 ISO `"2026-07-08"`（便于排序/查询） |
| `package.json` | 新增 `idb`、`vitest`、`fake-indexeddb` 依赖，新增 `test` script |

### 8.3 实现顺序（建议按此顺序提交 PR）

```
1. db/db.ts + db/types.ts           ← DB 连接 + 类型定义
2. db/repositories/*.ts              ← 4 个 Repository
3. db/migrate-from-localstorage.ts   ← 迁移逻辑
4. hooks/useStorage.ts               ← Context Provider
5. hooks/useAnimalStore.ts           ← 动物 hook
6. hooks/useStaminaRecovery.ts       ← 体力恢复（与 #31 协调）
7. App.tsx + CollectScreen.tsx 改造   ← 接入组件
8. test/*.test.ts                    ← 测试
9. package.json + vitest.config.ts   ← 依赖与配置
```

---

## 9. 验收标准对照

### Issue #32 验收标准

> 动物数据可持久化，重启不丢失

### 验收检查清单

| # | 验收项 | 验证方式 | 状态 |
|---|--------|---------|------|
| 1 | 捕获动物后，数据写入 IndexedDB | DevTools → Application → IndexedDB → `animals` store 有新记录 | ☐ |
| 2 | 刷新页面后，图鉴列表数据不丢失 | 刷新后 CollectScreen 显示已捕获的动物 | ☐ |
| 3 | 关闭浏览器重新打开，数据仍在 | 重新打开后图鉴数据完整 | ☐ |
| 4 | 清空 IndexedDB 后不崩溃 | 清空后刷新，回退默认数据，无白屏 | ☐ |
| 5 | 体力数据从 localStorage 迁移到 IndexedDB | 若 #31 使用了 localStorage，迁移后旧数据可读 | ☐ |
| 6 | Repository 单元测试通过 | `npm test` 全绿 | ☐ |

### MVP 阶段明确不做（KISS）

- [ ] 数据加密（内测阶段引入后端同步后处理）
- [] 云端同步（公测阶段 Go 后端 /sync/animal 端点）
- [ ] 多账号 / 多设备同步
- [ ] 数据导出 / 导入功能
- [ | liveQuery 响应式查询（idb 不支持，需要时切 Dexie）

---

## 附：依赖变更总结

### 新增 dependencies

```json
{
  "idb": "^8.0.0"
}
```

### 新增 devDependencies

```json
{
  "vitest": "^2.0.0",
  "fake-indexeddb": "^6.0.0",
  "jsdom": "^25.0.0"
}
```

### 新增 scripts

```json
{
  "test": "vitest run",
  "test:watch": "vitest"
}
```

---

*文档版本：v1.0 · 2026-07-08 · Issue #32 实现计划*
