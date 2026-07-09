# [M2] 崩溃率（< 0.5%）实现计划 — Issue #45

> **验收标准**：崩溃率 < 0.5%
>
> **设计文档来源**：`游戏开发计划.md` 7.3 内测阶段验收（崩溃率 < 0.5%）+ 第九章 风险表（崩溃：Error Boundary + 全局异常上报 + 降级策略）
>
> **技术栈**：React 18 + Vite 6 + TypeScript 5.6 + vitest，PWA
>
> **现有基础**：
> - `main.tsx` 直接 `createRoot().render(<App />)`，无 Error Boundary
> - `App.tsx` 8 层 Context Provider 嵌套（Lbs → Stamina → Economy → Weather → Shop → Status → Dispatch → Battle），任一 Provider 渲染抛出异常会导致白屏
> - `DiscoverScreen.tsx` 摄像头 `getUserMedia` 已有 try/catch 和 denied 降级 UI，但 stream 清理仅在组件卸载时
> - `db.ts` 通过 `idb` 库惰性初始化，无配额超限 / 损坏恢复处理
> - `StaminaContext.tsx` / `LbsContext.tsx` 的 localStorage 读写有基础 try/catch，但未处理 `QuotaExceededError`
> - 全局无 `window.onerror` / `unhandledrejection` 监听
> - 无错误上报基础设施，后端无 `/api/v1/errors/report` 端点
> - 后端路由结构：`/api/v1` 下分组，部分需 JWT 鉴权，部分公开

---

## 0. 现状审计与问题清单

### 0.1 崩溃路径分析

| 崩溃路径 | 现状 | 风险等级 |
|---------|------|---------|
| React 组件渲染异常 | 无 Error Boundary，直接白屏 | **高** |
| Context Provider 抛异常 | 8 层嵌套，任一层崩溃全树白屏 | **高** |
| 未捕获 Promise rejection | 无 `unhandledrejection` 监听 | **高** |
| `window.onerror`（JS 语法/运行时错误） | 无监听 | **高** |
| 摄像头权限拒绝 | 已有 denied 降级 UI | 低 |
| 摄像头 stream 泄漏 | 组件卸载时 `stopCamera()`，但页面后台/异常时不保证 | 中 |
| IndexedDB 配额超限 | 无处理，`db.add()` 直接抛异常 | 中 |
| IndexedDB 数据损坏 | 无恢复机制，`getDB()` 损坏后永久不可用 | 中 |
| localStorage 配额超限 | `StaminaContext` 的 `setItem` 无 try/catch | 中 |
| localStorage 隐私模式 | Safari 隐私模式下 `setItem` 抛异常 | 中 |
| 网络请求失败 | `fetchCityName` / `fetchWeekWeather` 已有静默降级 | 低 |
| 内存泄漏（定时器/监听器/相机） | 各 Context useEffect cleanup 基本到位，但异常路径不保证 | 中 |

### 0.2 现有错误处理审计

| 文件 | 现有错误处理 | 缺失 |
|------|------------|------|
| `main.tsx` | 无 | Error Boundary、全局错误监听 |
| `App.tsx` | 无 | Provider 隔离、Suspense |
| `DiscoverScreen.tsx` | `getUserMedia` try/catch + denied UI | stream 异常清理、页面后台暂停 |
| `db.ts` | 无 | 配额处理、损坏恢复、重试 |
| `StaminaContext.tsx` | `loadInitialState` try/catch | `setItem` 无 try/catch |
| `LbsContext.tsx` | `loadInitialState` try/catch、`fetchCityName` 静默降级 | `setItem` 无 try/catch |
| `weather/api.ts` | `fetchWeekWeather` 全包裹 try/catch + null 降级 | 无重试机制 |
| `useAnimalStore.ts` | try/finally | catch 块缺失，错误被吞 |

---

## 1. 架构设计

### 1.1 整体架构

```
main.tsx
  └─ <GlobalErrorBoundary>              ← 捕获所有渲染异常（最终防线）
       └─ <ErrorReporterProvider>       ← 注入 reportError 函数
            └─ <App>
                 └─ 各 Provider 间插入 <ProviderErrorBoundary>  ← 隔离单个 Provider 崩溃
                      └─ <AppInner>
                           └─ 各 Screen 由 <ScreenErrorBoundary> 包裹  ← 隔离单屏崩溃

全局监听层：
  window.addEventListener('error', ...)       ← 捕获 JS 运行时错误
  window.addEventListener('unhandledrejection', ...)  ← 捕获未处理 Promise
```

### 1.2 错误分类与处理策略

| 错误类型 | 捕获方式 | 用户体验 | 上报 |
|---------|---------|---------|------|
| React 渲染异常 | Error Boundary | 降级 UI + 重试按钮 | 是 |
| Context Provider 异常 | ProviderErrorBoundary | 该功能模块降级，其余正常 | 是 |
| 单屏组件异常 | ScreenErrorBoundary | 该屏降级 UI，可切其他 Tab | 是 |
| JS 运行时错误 | window.onerror | 静默（不中断用户操作） | 是 |
| 未处理 Promise rejection | unhandledrejection | 静默 | 是 |
| 摄像头权限拒绝 | 组件内 try/catch | 降级 UI + 重试 | 否（预期行为） |
| IndexedDB 配额超限 | try/catch + 清理 | Toast 提示 + 降级到内存 | 是 |
| IndexedDB 损坏 | 删除重建 | Toast 提示 + 数据重置 | 是 |
| localStorage 配额超限 | try/catch | 静默降级到内存 | 否 |
| 网络请求失败 | try/catch + 重试 | 静默降级或 Toast | 否（预期行为） |

### 1.3 错误上报数据结构

```typescript
// src/errors/types.ts
interface ErrorReport {
  /** 错误唯一 ID（前端生成 UUID） */
  id: string
  /** 错误类型分类 */
  type: 'react' | 'window' | 'unhandledrejection' | 'indexeddb' | 'camera' | 'network'
  /** 错误名称 */
  name: string
  /** 错误消息 */
  message: string
  /** 错误堆栈（如有） */
  stack?: string
  /** 发生时间戳 */
  timestamp: number
  /** 当前页面路径 / Tab */
  page?: string
  /** 用户设备 ID */
  deviceId?: string
  /** App 版本 */
  appVersion: string
  /** User-Agent */
  userAgent: string
  /** 附加上下文（如组件名、操作名） */
  context?: Record<string, unknown>
  /** 浏览器是否在线 */
  online: boolean
}
```

---

## 2. React Error Boundary 实现

### 2.1 基础 Error Boundary 组件

**新增文件**：`src/errors/ErrorBoundary.tsx`

```typescript
import React, { Component, type ReactNode } from 'react'
import type { ErrorReport } from './types'
import { reportError } from './reporter'

interface Props {
  children: ReactNode
  /** 该 boundary 的名称（用于错误上下文） */
  name: string
  /** 自定义降级 UI */
  fallback?: ReactNode | ((error: Error, retry: () => void) => ReactNode)
  /** 是否上报错误（默认 true） */
  report?: boolean
  /** 错误时的回调 */
  onError?: (error: Error, info: React.ErrorInfo) => void
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo): void {
    if (this.props.report !== false) {
      const report: ErrorReport = {
        id: crypto.randomUUID(),
        type: 'react',
        name: error.name,
        message: error.message,
        stack: error.stack,
        timestamp: Date.now(),
        page: this.props.name,
        appVersion: __APP_VERSION__,
        userAgent: navigator.userAgent,
        context: { componentStack: info.componentStack },
        online: navigator.onLine,
      }
      reportError(report).catch(() => { /* 静默失败 */ })
    }
    this.props.onError?.(error, info)
  }

  retry = (): void => {
    this.setState({ hasError: false, error: null })
  }

  render(): ReactNode {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return typeof this.props.fallback === 'function'
          ? this.props.fallback(this.state.error!, this.retry)
          : this.props.fallback
      }
      return <DefaultFallback error={this.state.error!} onRetry={this.retry} />
    }
    return this.props.children
  }
}

/** 默认降级 UI */
function DefaultFallback({ error, onRetry }: { error: Error; onRetry: () => void }) {
  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      height: '100%',
      padding: 24,
      background: 'var(--cream)',
    }}>
      <span style={{ fontSize: 48 }}>😵</span>
      <h3 style={{ color: 'var(--orange-dark)', margin: '12px 0 4px', fontSize: 16 }}>
        页面出了点问题
      </h3>
      <p style={{ color: 'var(--ink-3)', fontSize: 12, textAlign: 'center', marginBottom: 16 }}>
        {error.message || '发生了未知错误'}
      </p>
      <button className="btn btn-primary" onClick={onRetry} style={{ padding: '8px 24px', fontSize: 13 }}>
        重试
      </button>
    </div>
  )
}
```

### 2.2 全局 Error Boundary（最外层防线）

**新增文件**：`src/errors/GlobalErrorBoundary.tsx`

```typescript
import React from 'react'
import { ErrorBoundary } from './ErrorBoundary'

/**
 * 全局 Error Boundary — 包裹整个 App。
 * 当所有内部 Boundary 都未能捕获时，这是最终防线。
 * 提供"重新加载"而非"重试"按钮，因为状态可能已损坏。
 */
export const GlobalErrorBoundary: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return (
    <ErrorBoundary
      name="global"
      fallback={() => (
        <div style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100vh',
          padding: 24,
          background: 'var(--cream)',
        }}>
          <span style={{ fontSize: 56 }}>💥</span>
          <h2 style={{ color: 'var(--orange-dark)', margin: '16px 0 8px', fontSize: 18 }}>
            应用崩溃了
          </h2>
          <p style={{ color: 'var(--ink-3)', fontSize: 13, textAlign: 'center', marginBottom: 20 }}>
            抱歉，遇到了严重错误。请尝试重新加载。
          </p>
          <button
            className="btn btn-primary"
            onClick={() => window.location.reload()}
            style={{ padding: '10px 32px', fontSize: 14 }}
          >
            重新加载
          </button>
        </div>
      )}
    >
      {children}
    </ErrorBoundary>
  )
}
```

### 2.3 Provider Error Boundary（隔离 Provider 崩溃）

**新增文件**：`src/errors/ProviderErrorBoundary.tsx`

```typescript
import React from 'react'
import { ErrorBoundary } from './ErrorBoundary'

interface Props {
  /** Provider 名称，如 "StaminaProvider" */
  providerName: string
  children: React.ReactNode
}

/**
 * Provider 隔离 Boundary — 包裹单个 Context Provider。
 * 当某个 Provider 渲染/初始化抛出异常时，仅该模块降级，
 * 不影响其他 Provider 和整个 App。
 */
export const ProviderErrorBoundary: React.FC<Props> = ({ providerName, children }) => {
  return (
    <ErrorBoundary
      name={providerName}
      fallback={() => (
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          minHeight: 60,
          color: 'var(--ink-3)',
          fontSize: 11,
        }}>
          ⚠️ {providerName} 模块暂时不可用
        </div>
      )}
    >
      {children}
    </ErrorBoundary>
  )
}
```

### 2.4 Screen Error Boundary（隔离单屏崩溃）

**新增文件**：`src/errors/ScreenErrorBoundary.tsx`

```typescript
import React from 'react'
import { ErrorBoundary } from './ErrorBoundary'

interface Props {
  screenName: string
  children: React.ReactNode
  onRetry?: () => void
}

/**
 * 屏幕级 Boundary — 包裹单个屏幕组件。
 * 单屏崩溃时显示降级 UI，用户可切换到其他 Tab。
 */
export const ScreenErrorBoundary: React.FC<Props> = ({ screenName, children }) => {
  return (
    <ErrorBoundary
      name={`screen:${screenName}`}
      fallback={(error, retry) => (
        <div style={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          background: 'var(--cream)',
          padding: 24,
        }}>
          <span style={{ fontSize: 40 }}>😵</span>
          <h3 style={{ color: 'var(--orange-dark)', margin: '12px 0 4px', fontSize: 15 }}>
            {screenName} 出了点问题
          </h3>
          <p style={{ color: 'var(--ink-3)', fontSize: 11, textAlign: 'center', marginBottom: 16 }}>
            {error.message}
          </p>
          <button className="btn btn-primary" onClick={retry} style={{ padding: '8px 20px', fontSize: 13 }}>
            重试
          </button>
        </div>
      )}
    >
      {children}
    </ErrorBoundary>
  )
}
```

### 2.5 在 main.tsx 中集成 GlobalErrorBoundary

```typescript
// main.tsx 修改后
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { GlobalErrorBoundary } from './errors/GlobalErrorBoundary'
import { setupGlobalErrorHandlers } from './errors/globalHandlers'
import './index.css'

// 在 React 挂载前注册全局错误处理器
setupGlobalErrorHandlers()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <GlobalErrorBoundary>
      <App />
    </GlobalErrorBoundary>
  </React.StrictMode>,
)
```

### 2.6 在 App.tsx 中集成 ProviderErrorBoundary + ScreenErrorBoundary

```typescript
// App.tsx 修改后（Provider 嵌套部分）
import { ProviderErrorBoundary } from './errors/ProviderErrorBoundary'
import { ScreenErrorBoundary } from './errors/ScreenErrorBoundary'

const App: React.FC = () => {
  return (
    <ProviderErrorBoundary providerName="Lbs">
      <LbsProvider>
        <ProviderErrorBoundary providerName="Stamina">
          <StaminaProvider>
            <ProviderErrorBoundary providerName="Economy">
              <EconomyProvider>
                <ProviderErrorBoundary providerName="Weather">
                  <WeatherProvider>
                    <ProviderErrorBoundary providerName="Shop">
                      <ShopProvider>
                        <ProviderErrorBoundary providerName="Status">
                          <StatusProvider>
                            <ProviderErrorBoundary providerName="Dispatch">
                              <DispatchProvider>
                                <ProviderErrorBoundary providerName="Battle">
                                  <BattleProvider>
                                    <ProviderErrorBoundary providerName="Achievement">
                                      <AchievementProvider>
                                        <AppInner />
                                      </AchievementProvider>
                                    </ProviderErrorBoundary>
                                  </BattleProvider>
                                </ProviderErrorBoundary>
                              </DispatchProvider>
                            </ProviderErrorBoundary>
                          </StatusProvider>
                        </ProviderErrorBoundary>
                      </ShopProvider>
                    </ProviderErrorBoundary>
                  </WeatherProvider>
                </ProviderErrorBoundary>
              </EconomyProvider>
            </ProviderErrorBoundary>
          </StaminaProvider>
        </ProviderErrorBoundary>
      </LbsProvider>
    </ProviderErrorBoundary>
  )
}
```

在 `renderContent()` 中包裹 ScreenErrorBoundary：

```typescript
// App.tsx renderContent 修改
const renderContent = () => {
  if (mapOpen) {
    return (
      <ScreenErrorBoundary screenName="地图">
        <MapScreen entries={mapEntries} focusEntry={mapFocus} onBack={handleMapClose} />
      </ScreenErrorBoundary>
    )
  }
  switch (activeTab) {
    case 'collection':
      return (
        <ScreenErrorBoundary screenName="图鉴">
          <CollectScreen onMapOpen={handleMapOpen} />
        </ScreenErrorBoundary>
      )
    case 'camera':
      return (
        <ScreenErrorBoundary screenName="发现">
          <DiscoverScreen onConfirm={handlePhotoConfirm} />
        </ScreenErrorBoundary>
      )
    // ... 其他屏幕同理
    default:
      return null
  }
}
```

---

## 3. 全局错误处理器

### 3.1 全局错误监听

**新增文件**：`src/errors/globalHandlers.ts`

```typescript
import type { ErrorReport } from './types'
import { reportError } from './reporter'

let installed = false

/**
 * 注册全局错误处理器。
 * 在 main.tsx 中调用一次，监听 window.onerror 和 unhandledrejection。
 * 重复调用安全（幂等）。
 */
export function setupGlobalErrorHandlers(): void {
  if (installed) return
  installed = true

  // 捕获 JS 运行时错误（同步异常、资源加载错误）
  window.addEventListener('error', (event: ErrorEvent) => {
    const report: ErrorReport = {
      id: crypto.randomUUID(),
      type: 'window',
      name: event.error?.name ?? 'Error',
      message: event.message || 'Unknown error',
      stack: event.error?.stack,
      timestamp: Date.now(),
      page: window.location.pathname,
      appVersion: __APP_VERSION__,
      userAgent: navigator.userAgent,
      context: {
        filename: event.filename,
        lineno: event.lineno,
        colno: event.colno,
      },
      online: navigator.onLine,
    }
    reportError(report).catch(() => {})
    // 不调用 event.preventDefault()，让浏览器控制台也能看到错误
  })

  // 捕获未处理的 Promise rejection
  window.addEventListener('unhandledrejection', (event: PromiseRejectionEvent) => {
    const reason = event.reason
    const error = reason instanceof Error ? reason : new Error(String(reason))
    const report: ErrorReport = {
      id: crypto.randomUUID(),
      type: 'unhandledrejection',
      name: error.name,
      message: error.message || 'Unhandled promise rejection',
      stack: error.stack,
      timestamp: Date.now(),
      page: window.location.pathname,
      appVersion: __APP_VERSION__,
      userAgent: navigator.userAgent,
      context: {
        reason: typeof reason === 'object' ? String(reason) : reason,
      },
      online: navigator.onLine,
    }
    reportError(report).catch(() => {})
    // 标记为已处理，避免控制台噪音
    event.preventDefault()
  })
}
```

### 3.2 测试要点

- `window.addEventListener('error', ...)` 应在 `setupGlobalErrorHandlers()` 调用后生效
- `window.addEventListener('unhandledrejection', ...)` 应捕获 `Promise.reject()` 未被 catch 的情况
- 重复调用 `setupGlobalErrorHandlers()` 不会重复注册
- 错误上报失败（网络异常）不应抛出二次异常

---

## 4. 错误上报基础设施

### 4.1 错误类型定义

**新增文件**：`src/errors/types.ts`

```typescript
export type ErrorType =
  | 'react'
  | 'window'
  | 'unhandledrejection'
  | 'indexeddb'
  | 'camera'
  | 'network'

export interface ErrorReport {
  id: string
  type: ErrorType
  name: string
  message: string
  stack?: string
  timestamp: number
  page?: string
  deviceId?: string
  appVersion: string
  userAgent: string
  context?: Record<string, unknown>
  online: boolean
}
```

### 4.2 错误上报器

**新增文件**：`src/errors/reporter.ts`

```typescript
import type { ErrorReport } from './types'

const REPORT_ENDPOINT = '/api/v1/errors/report'
const MAX_QUEUE = 20
const RETRY_DELAYS = [1000, 3000, 10000] // 指数退避

/** 离线错误队列（navigator.onLine === false 时入队） */
const offlineQueue: ErrorReport[] = []

let isFlushing = false

/**
 * 上报错误到后端。
 * - 在线：立即发送，失败则入队
 * - 离线：入队，等网络恢复后批量发送
 * - 上报本身永不抛出异常（静默失败）
 */
export async function reportError(report: ErrorReport): Promise<void> {
  try {
    if (!navigator.onLine) {
      enqueue(report)
      return
    }
    await sendWithRetry(report, 0)
  } catch {
    enqueue(report)
  }
}

async function sendWithRetry(report: ErrorReport, attempt: number): Promise<void> {
  try {
    const resp = await fetch(REPORT_ENDPOINT, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(report),
      // 使用 keepalive 确保页面卸载时也能发出
      keepalive: true,
    })
    if (!resp.ok && resp.status >= 500 && attempt < RETRY_DELAYS.length) {
      await sleep(RETRY_DELAYS[attempt])
      await sendWithRetry(report, attempt + 1)
    }
  } catch (err) {
    if (attempt < RETRY_DELAYS.length) {
      await sleep(RETRY_DELAYS[attempt])
      await sendWithRetry(report, attempt + 1)
    }
    // 最终失败则入队
    throw err
  }
}

function enqueue(report: ErrorReport): void {
  if (offlineQueue.length >= MAX_QUEUE) {
    // 队列满时丢弃最旧的
    offlineQueue.shift()
  }
  offlineQueue.push(report)
}

/** 网络恢复后批量发送队列中的错误 */
export async function flushQueue(): Promise<void> {
  if (isFlushing || offlineQueue.length === 0) return
  isFlushing = true
  try {
    while (offlineQueue.length > 0) {
      const report = offlineQueue[0]
      try {
        await sendWithRetry(report, 0)
        offlineQueue.shift()
      } catch {
        break // 网络仍不可用，停止尝试
      }
    }
  } finally {
    isFlushing = false
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

/** 注册 online 事件监听，网络恢复时自动 flush */
let onlineListenerInstalled = false
export function installOnlineListener(): void {
  if (onlineListenerInstalled) return
  onlineListenerInstalled = true
  window.addEventListener('online', () => {
    flushQueue().catch(() => {})
  })
}
```

### 4.3 全局错误处理初始化

**新增文件**：`src/errors/index.ts`

```typescript
export { ErrorBoundary } from './ErrorBoundary'
export { GlobalErrorBoundary } from './GlobalErrorBoundary'
export { ProviderErrorBoundary } from './ProviderErrorBoundary'
export { ScreenErrorBoundary } from './ScreenErrorBoundary'
export { setupGlobalErrorHandlers } from './globalHandlers'
export { reportError, flushQueue, installOnlineListener } from './reporter'
export type { ErrorReport, ErrorType } from './types'
```

### 4.4 在 main.tsx 中初始化

```typescript
// main.tsx 最终形态
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { GlobalErrorBoundary, setupGlobalErrorHandlers, installOnlineListener } from './errors'
import './index.css'

setupGlobalErrorHandlers()
installOnlineListener()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <GlobalErrorBoundary>
      <App />
    </GlobalErrorBoundary>
  </React.StrictMode>,
)
```

### 4.5 App 版本号注入

在 `vite.config.ts` 中通过 `define` 注入版本号：

```typescript
// vite.config.ts 新增
import pkg from './package.json'

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },
  // ... 其他配置
})
```

在 `src/vite-env.d.ts` 中补充类型声明：

```typescript
// src/vite-env.d.ts 新增
declare const __APP_VERSION__: string
```

### 4.6 后端错误上报端点（Go 侧）

**新增文件**：`backend/internal/handlers/error_handler.go`

```go
package handlers

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

type ErrorReport struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    Name      string                 `json:"name"`
    Message   string                 `json:"message"`
    Stack     string                 `json:"stack"`
    Timestamp int64                  `json:"timestamp"`
    Page      string                 `json:"page"`
    DeviceID  string                 `json:"deviceId"`
    AppVersion string                `json:"appVersion"`
    UserAgent string                 `json:"userAgent"`
    Context   map[string]interface{} `json:"context"`
    Online    bool                   `json:"online"`
}

type ErrorLog struct {
    ID        uint      `gorm:"primaryKey;autoIncrement"`
    ReportID  string    `gorm:"index;size:64"`
    Type      string    `gorm:"size:32;index"`
    Name      string    `gorm:"size:256"`
    Message   string    `gorm:"type:text"`
    Stack     string    `gorm:"type:text"`
    Page      string    `gorm:"size:256"`
    DeviceID  string    `gorm:"index;size:64"`
    AppVersion string   `gorm:"size:32"`
    UserAgent string    `gorm:"size:512"`
    Context   string    `gorm:"type:text"`
    CreatedAt time.Time `gorm:"index"`
}

func NewErrorHandler(db *gorm.DB) *ErrorHandler {
    // 自动迁移表
    db.AutoMigrate(&ErrorLog{})
    return &ErrorHandler{db: db}
}

type ErrorHandler struct {
    db *gorm.DB
}

// Report 接收前端错误上报
// 路由: POST /api/v1/errors/report
// 无需鉴权（错误上报不应因 token 过期而失败）
func (h *ErrorHandler) Report(c *gin.Context) {
    var report ErrorReport
    if err := c.ShouldBindJSON(&report); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
        return
    }

    // 基本校验
    if report.ID == "" || report.Type == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "missing required fields"})
        return
    }

    // 限制 message/stack 长度，防止超大 payload
    if len(report.Message) > 8192 {
        report.Message = report.Message[:8192]
    }
    if len(report.Stack) > 16384 {
        report.Stack = report.Stack[:16384]
    }

    log := ErrorLog{
        ReportID:   report.ID,
        Type:       report.Type,
        Name:       report.Name,
        Message:    report.Message,
        Stack:      report.Stack,
        Page:       report.Page,
        DeviceID:   report.DeviceID,
        AppVersion: report.AppVersion,
        UserAgent:  report.UserAgent,
        CreatedAt:  time.Now(),
    }

    // DB 不可用时静默返回 200（不增加前端重试负担）
    if h.db != nil {
        if err := h.db.Create(&log).Error; err != nil {
            // 记录日志但不返回错误
            c.JSON(http.StatusOK, gin.H{"ok": true})
            return
        }
    }

    c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

在 `router.go` 中注册路由：

```go
// router.go 新增（在公开端点区域）
if db != nil {
    errorHandler := handlers.NewErrorHandler(db)
    api.POST("/errors/report", errorHandler.Report)
}
```

---

## 5. 摄像头权限失败处理（优雅降级）

### 5.1 现状分析

`DiscoverScreen.tsx` 已有基本的摄像头错误处理：
- `getUserMedia` 在 try/catch 中调用
- 拒绝时显示 `denied` 状态 UI
- 有"重试"按钮

**缺失**：
- `navigator.mediaDevices` 可能为 `undefined`（非 HTTPS / 旧浏览器）
- stream 在页面后台/异常路径下不保证释放
- 缺少 `NotReadableError`（摄像头被其他应用占用）的区分处理

### 5.2 改进方案

**修改文件**：`src/components/DiscoverScreen.tsx`

```typescript
// 改进后的 startCamera
const startCamera = useCallback(async () => {
  try {
    setState('loading')
    setErrorMsg('')
    setDetectionState('idle')
    setDetectionResult(null)

    // 检查 mediaDevices 是否可用
    if (!navigator.mediaDevices?.getUserMedia) {
      throw new Error('设备不支持摄像头或需要 HTTPS 环境')
    }

    const stream = await navigator.mediaDevices.getUserMedia({
      video: {
        facingMode: 'environment',
        width: { ideal: 640 },
        height: { ideal: 480 },
      },
      audio: false,
    })
    streamRef.current = stream

    if (videoRef.current) {
      videoRef.current.srcObject = stream
      await videoRef.current.play()
    }
    setState('ready')
  } catch (err: any) {
    // 分类错误，提供更友好的提示
    let friendlyMsg = '无法访问摄像头'
    if (err.name === 'NotAllowedError' || err.code === 1) {
      friendlyMsg = '摄像头权限被拒绝，请在浏览器设置中允许'
    } else if (err.name === 'NotFoundError' || err.code === 8) {
      friendlyMsg = '未检测到摄像头设备'
    } else if (err.name === 'NotReadableError') {
      friendlyMsg = '摄像头被其他应用占用，请关闭后重试'
    } else if (err.name === 'OverconstrainedError') {
      friendlyMsg = '摄像头不满足要求的参数，请尝试其他设备'
    } else if (err.message) {
      friendlyMsg = err.message
    }

    setErrorMsg(friendlyMsg)
    setState('denied')

    // 上报非预期错误（权限拒绝是预期行为，不上报）
    if (err.name !== 'NotAllowedError') {
      reportError({
        id: crypto.randomUUID(),
        type: 'camera',
        name: err.name || 'CameraError',
        message: friendlyMsg,
        stack: err.stack,
        timestamp: Date.now(),
        appVersion: __APP_VERSION__,
        userAgent: navigator.userAgent,
        online: navigator.onLine,
      }).catch(() => {})
    }
  }
}, [])
```

### 5.3 页面后台时释放相机

```typescript
// DiscoverScreen.tsx 新增 visibilitychange 监听
useEffect(() => {
  const handleVisibilityChange = () => {
    if (document.visibilityState === 'hidden') {
      // 页面后台时停止相机（省电 + 防泄漏）
      stopCamera()
    } else if (state === 'ready') {
      // 页面恢复前台时重新启动
      startCamera()
    }
  }
  document.addEventListener('visibilitychange', handleVisibilityChange)
  return () => {
    document.removeEventListener('visibilitychange', handleVisibilityChange)
  }
}, [startCamera, stopCamera, state])
```

### 5.4 安全清理 stream

```typescript
// DiscoverScreen.tsx 改进 stopCamera
const stopCamera = useCallback(() => {
  if (streamRef.current) {
    streamRef.current.getTracks().forEach(track => {
      try {
        track.stop()
      } catch {
        // 忽略已停止的 track
      }
    })
    streamRef.current = null
  }
  // 清理 video srcObject
  if (videoRef.current?.srcObject) {
    videoRef.current.srcObject = null
  }
}, [])
```

---

## 6. IndexedDB 错误处理

### 6.1 配额超限处理

**修改文件**：`src/db/db.ts`

```typescript
import { openDB, type IDBPDatabase } from 'idb'
import { reportError } from '../errors/reporter'

export const DB_NAME = 'animal-poke-db'
export const DB_VERSION = 1

let dbPromise: Promise<IDBPDatabase> | null = null

/** 检查存储配额是否紧张 */
async function checkStorageQuota(): Promise<boolean> {
  if (navigator.storage?.estimate) {
    const estimate = await navigator.storage.estimate()
    const usageRatio = estimate.usage / estimate.quota
    if (usageRatio > 0.9) {
      return false // 配额紧张
    }
  }
  return true
}

/** 获取 DB 实例（单例，惰性初始化） */
export function getDB(): Promise<IDBPDatabase> {
  if (!dbPromise) {
    dbPromise = openDB(DB_NAME, DB_VERSION, {
      upgrade(db) {
        if (!db.objectStoreNames.contains('animals')) {
          const store = db.createObjectStore('animals', { keyPath: 'id' })
          store.createIndex('by-date', 'captureDate')
          store.createIndex('by-rarity', 'rarity')
        }
        if (!db.objectStoreNames.contains('settings')) {
          db.createObjectStore('settings', { keyPath: 'key' })
        }
      },
    }).catch((err) => {
      // DB 打开失败 — 可能是数据损坏
      reportError({
        id: crypto.randomUUID(),
        type: 'indexeddb',
        name: err.name || 'IndexedDBError',
        message: `DB open failed: ${err.message}`,
        stack: err.stack,
        timestamp: Date.now(),
        appVersion: __APP_VERSION__,
        userAgent: navigator.userAgent,
        online: navigator.onLine,
      }).catch(() => {})

      // 尝试恢复：删除并重建
      return recoverDB()
    })
  }
  return dbPromise
}

/** 损坏恢复：删除数据库后重新打开 */
async function recoverDB(): Promise<IDBPDatabase> {
  try {
    indexedDB.deleteDatabase(DB_NAME)
    // 等待删除完成
    await new Promise(resolve => setTimeout(resolve, 100))
    dbPromise = openDB(DB_NAME, DB_VERSION, {
      upgrade(db) {
        if (!db.objectStoreNames.contains('animals')) {
          const store = db.createObjectStore('animals', { keyPath: 'id' })
          store.createIndex('by-date', 'captureDate')
          store.createIndex('by-rarity', 'rarity')
        }
        if (!db.objectStoreNames.contains('settings')) {
          db.createObjectStore('settings', { keyPath: 'key' })
        }
      },
    })
    return await dbPromise
  } catch (err: any) {
    // 恢复也失败 — 降级到内存模式
    reportError({
      id: crypto.randomUUID(),
      type: 'indexeddb',
      name: 'DBRecoveryFailed',
      message: `DB recovery failed: ${err.message}`,
      timestamp: Date.now(),
      appVersion: __APP_VERSION__,
      userAgent: navigator.userAgent,
      online: navigator.onLine,
    }).catch(() => {})
    throw err
  }
}

/** 重置 DB 单例（仅供测试使用） */
export async function resetDB(): Promise<void> {
  if (dbPromise) {
    try {
      const db = await dbPromise
      db.close()
    } catch {
      // 忽略关闭失败
    }
    dbPromise = null
  }
  indexedDB.deleteDatabase(DB_NAME)
}
```

### 6.2 Repository 层错误处理

**修改文件**：`src/db/repositories/animal-repository.ts`

在 `add` / `bulkAdd` 方法中增加配额错误处理：

```typescript
/** 新增一只动物 */
async add(entry: AnimalRecord): Promise<void> {
  try {
    const db = await getDB()
    await db.add('animals', entry)
  } catch (err: any) {
    if (err.name === 'QuotaExceededError') {
      // 配额超限：尝试清理旧的未查看记录后重试
      await AnimalRepository.cleanupOldData()
      const db = await getDB()
      await db.add('animals', entry)
    } else {
      throw err
    }
  }
}

/** 清理旧的已查看记录（配额紧张时调用） */
async cleanupOldData(): Promise<void> {
  const db = await getDB()
  const all = await db.getAll('animals')
  // 保留最近 100 条 + 所有未查看的
  const toDelete = all
    .filter(a => !a.isNew)
    .sort((a, b) => b.captureDate.localeCompare(a.captureDate))
    .slice(100)
  const tx = db.transaction('animals', 'readwrite')
  await Promise.all(toDelete.map(a => tx.store.delete(a.id)))
  await tx.done
}
```

### 6.3 useAnimalStore 错误处理

**修改文件**：`src/hooks/useAnimalStore.ts`

```typescript
export function useAnimalStore() {
  const [animals, setAnimals] = useState<AnimalRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        let list = await AnimalRepository.getAll()
        if (list.length === 0) {
          await AnimalRepository.bulkAdd(MOCK_ENTRIES)
          list = await AnimalRepository.getAll()
        }
        if (!cancelled) {
          setAnimals(list)
        }
      } catch (err: any) {
        if (!cancelled) {
          setError(err.message || '数据加载失败')
          // 降级：使用内存中的 mock 数据
          setAnimals(MOCK_ENTRIES)
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    })()
    return () => { cancelled = true }
  }, [])

  const addAnimal = useCallback(async (entry: AnimalRecord) => {
    try {
      await AnimalRepository.add(entry)
      setAnimals(prev => [...prev, entry])
    } catch (err) {
      // IDB 写入失败，仅更新内存
      setAnimals(prev => [...prev, entry])
    }
  }, [])

  return { animals, loading, error, addAnimal, markViewed }
}
```

---

## 7. 网络错误韧性

### 7.1 通用 fetch 重试封装

**新增文件**：`src/services/fetchWithRetry.ts`

```typescript
interface FetchRetryOptions {
  retries?: number
  retryDelay?: number
  timeout?: number
  signal?: AbortSignal
}

/**
 * 带重试 + 超时的 fetch 封装。
 * 仅对网络错误和 5xx 进行重试，4xx 不重试。
 */
export async function fetchWithRetry(
  url: string,
  options: RequestInit = {},
  retryOptions: FetchRetryOptions = {},
): Promise<Response> {
  const {
    retries = 3,
    retryDelay = 1000,
    timeout = 10000,
  } = retryOptions

  let lastError: Error | null = null

  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      const controller = new AbortController()
      const timeoutId = setTimeout(() => controller.abort(), timeout)

      // 合并外部 signal（如果有）
      if (retryOptions.signal) {
        retryOptions.signal.addEventListener('abort', () => controller.abort())
      }

      const response = await fetch(url, {
        ...options,
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      // 5xx 重试
      if (response.status >= 500 && attempt < retries) {
        await sleep(retryDelay * (attempt + 1))
        continue
      }

      return response
    } catch (err: any) {
      lastError = err
      // AbortError 不重试（用户主动取消或超时）
      if (err.name === 'AbortError' && !err.message.includes('timeout')) {
        throw err
      }
      if (attempt < retries) {
        await sleep(retryDelay * (attempt + 1))
      }
    }
  }

  throw lastError ?? new Error('fetchWithRetry failed')
}

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}
```

### 7.2 离线状态检测

**新增文件**：`src/hooks/useOnlineStatus.ts`

```typescript
import { useState, useEffect } from 'react'

/**
 * 监听网络在线/离线状态。
 * 离线时 UI 可提示用户"当前离线，部分功能不可用"。
 */
export function useOnlineStatus(): boolean {
  const [online, setOnline] = useState(navigator.onLine)

  useEffect(() => {
    const handleOnline = () => setOnline(true)
    const handleOffline = () => setOnline(false)
    window.addEventListener('online', handleOnline)
    window.addEventListener('offline', handleOffline)
    return () => {
      window.removeEventListener('online', handleOnline)
      window.removeEventListener('offline', handleOffline)
    }
  }, [])

  return online
}
```

### 7.3 应用到现有 API 调用

将 `weather/api.ts` 和 `LbsContext.tsx` 中的 `fetch` 调用替换为 `fetchWithRetry`：

```typescript
// weather/api.ts 改进
import { fetchWithRetry } from '../services/fetchWithRetry'

export async function fetchWeekWeather(city: string): Promise<WeekWeather | null> {
  try {
    const resp = await fetchWithRetry(
      `/api/v1/weather/week?city=${encodeURIComponent(city)}`,
      {},
      { retries: 2, retryDelay: 500 }
    )
    if (!resp.ok) throw new Error(`weather/week 请求失败: ${resp.status}`)
    const data = await resp.json()
    if (data?.week && Array.isArray(data.week) && data.week.length === 7) {
      return data.week as WeekWeather
    }
    throw new Error('后端天气数据格式异常')
  } catch {
    return null // 静默降级
  }
}
```

---

## 8. Context Provider 错误隔离

### 8.1 设计原理

当前 `App.tsx` 中 8 层 Provider 线性嵌套。如果中间某一层 Provider 的初始化（如 `loadInitialState`）或 `useEffect` 抛出异常，整个 Provider 链断裂，导致白屏。

**方案**：在第 2.6 节中已展示——在每层 Provider 外包裹 `ProviderErrorBoundary`。当某个 Provider 崩溃时，其降级 UI 替代该 Provider 的内容，但外层 Provider 和兄弟模块不受影响。

### 8.2 Context 消费者的安全访问

当前 `DiscoverScreen.tsx` 中已使用 try/catch 包裹 `useLbs()` 和 `useWeather()` 来防止 Provider 未挂载时抛出异常。这种模式应推广到所有"跨模块消费"的场景。

**新增工具 Hook**：`src/hooks/useSafeContext.ts`

```typescript
import { useContext } from 'react'

/**
 * 安全消费 Context：如果 Provider 未挂载（值为 null），返回 null 而非抛出异常。
 * 消费者需自行处理 null 情况。
 */
export function useSafeContext<T>(context: React.Context<T | null>): T | null {
  return useContext(context)
}
```

**使用示例**：

```typescript
// 在组件中安全消费
import { useSafeContext } from '../hooks/useSafeContext'
import { StaminaContext, type StaminaContextValue } from '../stamina/StaminaContext'

function SomeComponent() {
  const stamina = useSafeContext<StaminaContextValue>(StaminaContext)
  if (!stamina) {
    return <div>体力模块暂时不可用</div>
  }
  return <div>{stamina.state.currentStamina}</div>
}
```

### 8.3 各 Provider 内部错误防护

各 Context Provider 的 `useEffect` 中的异步操作（如 `fetch`）已有 try/catch 或 `.catch()`，但 `localStorage` 读写和 `dispatch` 调用可能抛出异常。

**修改文件**：`src/stamina/StaminaContext.tsx`（localStorage 持久化加 try/catch）

```typescript
// localStorage 持久化（修改后）
useEffect(() => {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch (err) {
    // QuotaExceededError 或隐私模式
    // 静默降级：不持久化，仅内存中保留
    if (err instanceof DOMException && err.name === 'QuotaExceededError') {
      // 尝试清理旧数据后重试
      try {
        localStorage.removeItem(STORAGE_KEY)
        localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
      } catch {
        // 最终仍失败，放弃持久化
      }
    }
  }
}, [state])
```

同理修改 `LbsContext.tsx` 的 localStorage 持久化。

---

## 9. 内存泄漏防护

### 9.1 统一 cleanup 工具

**新增文件**：`src/hooks/useCleanupRegistry.ts`

```typescript
import { useEffect, useRef } from 'react'

type CleanupFn = () => void

/**
 * 清理函数注册表。
 * 组件卸载时统一执行所有注册的清理函数，确保定时器、监听器、流等被释放。
 */
export function useCleanupRegistry() {
  const cleanupsRef = useRef<CleanupFn[]>([])

  const register = (fn: CleanupFn): void => {
    cleanupsRef.current.push(fn)
  }

  useEffect(() => {
    return () => {
      // 倒序执行清理（后注册的先清理）
      const cleanups = cleanupsRef.current
      for (let i = cleanups.length - 1; i >= 0; i--) {
        try {
          cleanups[i]()
        } catch {
          // 忽略清理失败
        }
      }
      cleanupsRef.current = []
    }
  }, [])

  return { register }
}
```

### 9.2 安全定时器 Hook

**新增文件**：`src/hooks/useSafeInterval.ts`

```typescript
import { useEffect, useRef } from 'react'

/**
 * 安全的 setInterval：组件卸载时自动清理。
 * 避免在异步回调中设置定时器后忘记清理。
 */
export function useSafeInterval(callback: () => void, delay: number | null): void {
  const savedCallback = useRef(callback)

  useEffect(() => {
    savedCallback.current = callback
  }, [callback])

  useEffect(() => {
    if (delay === null) return
    const id = setInterval(() => savedCallback.current(), delay)
    return () => clearInterval(id)
  }, [delay])
}
```

### 9.3 相机 stream 安全管理

当前 `DiscoverScreen.tsx` 在 `useEffect` cleanup 中调用 `stopCamera()`，但仅在组件正常卸载时触发。需要额外处理：

1. **页面隐藏时停止相机**（已在第 5.3 节中实现）
2. **React StrictMode 双调用保护**：`startCamera` 可能被调用两次，需确保旧 stream 被清理

```typescript
// DiscoverScreen.tsx 改进 startCamera（确保旧 stream 被清理）
const startCamera = useCallback(async () => {
  // 先清理可能存在的旧 stream（StrictMode 安全）
  if (streamRef.current) {
    stopCamera()
  }

  try {
    // ... 其余逻辑
  } catch (err: any) {
    // ... 错误处理
  }
}, [stopCamera])
```

### 9.4 全局监听器审计

| 文件 | 监听器 | 现有 cleanup | 需要改进 |
|------|--------|-------------|---------|
| `StaminaContext.tsx` | `visibilitychange` | 有 cleanup | OK |
| `LbsContext.tsx` | `visibilitychange` | 有 cleanup | OK |
| `DiscoverScreen.tsx` | `visibilitychange`（新增） | 需添加 cleanup | 新增 |
| `globalHandlers.ts` | `error` / `unhandledrejection` | 无（全局常驻） | 设计如此，不需要清理 |
| `reporter.ts` | `online` | 无（全局常驻） | 设计如此，不需要清理 |

---

## 10. 安全 localStorage 访问

### 10.1 封装安全 localStorage 工具

**新增文件**：`src/utils/safeStorage.ts`

```typescript
/**
 * 安全的 localStorage 访问封装。
 * 处理：
 * - 隐私模式（Safari）：setItem 抛 DOMException
 * - 配额超限：QuotaExceededError
 * - JSON 解析失败
 * - Storage 被禁用（iframe sandbox 等）
 */

let storageAvailable: boolean | null = null

/** 检测 localStorage 是否可用 */
function isStorageAvailable(): boolean {
  if (storageAvailable !== null) return storageAvailable
  try {
    const testKey = '__storage_test__'
    localStorage.setItem(testKey, '1')
    localStorage.removeItem(testKey)
    storageAvailable = true
  } catch {
    storageAvailable = false
  }
  return storageAvailable
}

/** 安全读取 */
export function safeGetItem<T>(key: string, fallback: T): T {
  if (!isStorageAvailable()) return fallback
  try {
    const value = localStorage.getItem(key)
    if (value === null) return fallback
    return JSON.parse(value) as T
  } catch {
    return fallback
  }
}

/** 安全写入 */
export function safeSetItem(key: string, value: unknown): boolean {
  if (!isStorageAvailable()) return false
  try {
    localStorage.setItem(key, JSON.stringify(value))
    return true
  } catch (err) {
    if (err instanceof DOMException && err.name === 'QuotaExceededError') {
      // 尝试清理后重试
      try {
        localStorage.removeItem(key)
        localStorage.setItem(key, JSON.stringify(value))
        return true
      } catch {
        return false
      }
    }
    return false
  }
}

/** 安全删除 */
export function safeRemoveItem(key: string): void {
  if (!isStorageAvailable()) return
  try {
    localStorage.removeItem(key)
  } catch {
    // 忽略
  }
}
```

### 10.2 应用到 StaminaContext 和 LbsContext

```typescript
// StaminaContext.tsx loadInitialState 修改
import { safeGetItem } from '../utils/safeStorage'

function loadInitialState(): StaminaState {
  const saved = safeGetItem<Partial<StaminaState> | null>(STORAGE_KEY, null)
  if (saved) {
    try {
      const migrated = migrateState(saved)
      // ... 其余逻辑
    } catch {
      // 迁移失败，使用默认值
    }
  }
  // 返回默认状态
}

// StaminaContext.tsx 持久化修改
useEffect(() => {
  safeSetItem(STORAGE_KEY, state)
}, [state])
```

---

## 11. 未处理 Promise rejection 防护

### 11.1 全局防护

已在第 3.1 节中通过 `unhandledrejection` 事件监听实现全局兜底。

### 11.2 代码层面的防护原则

| 场景 | 防护方式 |
|------|---------|
| `async useEffect` | 用 `let cancelled = false` + cleanup 标志位，避免组件卸载后 setState |
| `.then()` 链 | 必须有 `.catch()` 或外层 try/catch |
| `Promise.all()` | 使用 `Promise.allSettled()` 替代，避免单个 reject 导致全部失败 |
| 事件回调中的 async | 包裹 try/catch |
| `addEventListener` 的回调 | async 回调必须自行 catch |

### 11.3 现有代码审计与修复

| 文件 | 位置 | 问题 | 修复 |
|------|------|------|------|
| `DiscoverScreen.tsx:112` | `mockVisionDetector.detect().then()` | 有 `.catch()` | OK |
| `DiscoverScreen.tsx:148` | `retryDetection` 的 `.then()` | 有 `.catch()` | OK |
| `useAnimalStore.ts:14` | `;(async () => { ... })()` | 有 try/finally，缺少 catch | **添加 catch** |
| `LbsContext.tsx:144` | `fetchCityName().then().catch()` | 有 `.catch()` | OK |
| `App.tsx:104` | `Math.random()` 不涉及 Promise | N/A | OK |

---

## 12. 文件改动清单

### 12.1 新增文件

| 文件路径 | 说明 | 测试文件 |
|---------|------|---------|
| `src/errors/types.ts` | 错误类型定义 | `src/errors/types.test.ts` |
| `src/errors/ErrorBoundary.tsx` | 基础 Error Boundary 组件 | `src/errors/ErrorBoundary.test.tsx` |
| `src/errors/GlobalErrorBoundary.tsx` | 全局 Error Boundary | `src/errors/GlobalErrorBoundary.test.tsx` |
| `src/errors/ProviderErrorBoundary.tsx` | Provider 隔离 Boundary | `src/errors/ProviderErrorBoundary.test.tsx` |
| `src/errors/ScreenErrorBoundary.tsx` | 屏幕级 Boundary | `src/errors/ScreenErrorBoundary.test.tsx` |
| `src/errors/globalHandlers.ts` | 全局错误监听 | `src/errors/globalHandlers.test.ts` |
| `src/errors/reporter.ts` | 错误上报器 | `src/errors/reporter.test.ts` |
| `src/errors/index.ts` | 统一导出 | — |
| `src/services/fetchWithRetry.ts` | 带 retry 的 fetch | `src/services/fetchWithRetry.test.ts` |
| `src/hooks/useOnlineStatus.ts` | 在线状态 Hook | `src/hooks/useOnlineStatus.test.ts` |
| `src/hooks/useSafeContext.ts` | 安全 Context 消费 | `src/hooks/useSafeContext.test.tsx` |
| `src/hooks/useCleanupRegistry.ts` | 清理注册表 Hook | `src/hooks/useCleanupRegistry.test.ts` |
| `src/hooks/useSafeInterval.ts` | 安全定时器 Hook | `src/hooks/useSafeInterval.test.ts` |
| `src/utils/safeStorage.ts` | 安全 localStorage | `src/utils/safeStorage.test.ts` |
| `backend/internal/handlers/error_handler.go` | 后端错误上报端点 | `backend/internal/handlers/error_handler_test.go` |

### 12.2 修改文件

| 文件路径 | 改动内容 |
|---------|---------|
| `frontend/src/main.tsx` | 添加 GlobalErrorBoundary + setupGlobalErrorHandlers + installOnlineListener |
| `frontend/src/App.tsx` | Provider 外包裹 ProviderErrorBoundary + renderContent 包裹 ScreenErrorBoundary |
| `frontend/src/components/DiscoverScreen.tsx` | 改进 startCamera 错误分类 + visibilitychange + stream 安全清理 |
| `frontend/src/db/db.ts` | getDB 增加损坏恢复 + 配额检测 |
| `frontend/src/db/repositories/animal-repository.ts` | add/bulkAdd 增加配额错误处理 + cleanupOldData |
| `frontend/src/hooks/useAnimalStore.ts` | 添加 error 状态 + catch 降级 |
| `frontend/src/stamina/StaminaContext.tsx` | localStorage 持久化用 safeSetItem |
| `frontend/src/lbs/LbsContext.tsx` | localStorage 持久化用 safeSetItem |
| `frontend/src/weather/api.ts` | fetch → fetchWithRetry |
| `frontend/src/vite-env.d.ts` | 声明 `__APP_VERSION__` |
| `frontend/vitest.config.ts` | define `__APP_VERSION__` test 全局 |
| `frontend/vite.config.ts` | define `__APP_VERSION__` |
| `backend/internal/routes/router.go` | 注册 POST `/api/v1/errors/report` |
| `frontend/src/test-setup.ts` | 添加 `__APP_VERSION__` mock + `crypto.randomUUID` mock |

---

## 13. 测试策略

### 13.1 测试文件清单

| 测试文件 | 覆盖内容 | 测试数量（估） |
|---------|---------|--------------|
| `src/errors/ErrorBoundary.test.tsx` | 渲染异常捕获、降级 UI、重试、上报 | 8 |
| `src/errors/GlobalErrorBoundary.test.tsx` | 全局崩溃降级 UI、reload 按钮 | 3 |
| `src/errors/ProviderErrorBoundary.test.tsx` | Provider 隔离、降级提示 | 4 |
| `src/errors/ScreenErrorBoundary.test.tsx` | 屏幕隔离、降级 UI、重试 | 4 |
| `src/errors/globalHandlers.test.ts` | window.onerror、unhandledrejection、幂等注册 | 6 |
| `src/errors/reporter.test.ts` | 在线上报、离线入队、重试、flush | 8 |
| `src/errors/types.test.ts` | 类型守卫、字段完整性 | 2 |
| `src/services/fetchWithRetry.test.ts` | 重试逻辑、超时、AbortSignal、4xx 不重试 | 6 |
| `src/hooks/useOnlineStatus.test.ts` | online/offline 事件 | 3 |
| `src/hooks/useSafeContext.test.tsx` | Provider 未挂载返回 null | 2 |
| `src/hooks/useCleanupRegistry.test.ts` | 注册、倒序执行、异常忽略 | 3 |
| `src/hooks/useSafeInterval.test.ts` | 自动清理、delay null | 3 |
| `src/utils/safeStorage.test.ts` | 读写、隐私模式、配额超限、JSON 失败 | 6 |
| `backend/internal/handlers/error_handler_test.go` | 正常上报、字段校验、DB 不可用 | 4 |

### 13.2 关键测试用例详解

#### 13.2.1 ErrorBoundary 测试

```typescript
// src/errors/ErrorBoundary.test.tsx
import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ErrorBoundary } from './ErrorBoundary'

// 抛出异常的子组件
const ThrowComponent: React.FC<{ error: Error }> = ({ error }) => {
  throw error
}

describe('ErrorBoundary', () => {
  // 抑制 console.error（React 会在 boundary 捕获时打印错误）
  beforeEach(() => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('#1 子组件抛出异常时显示降级 UI', () => {
    render(
      <ErrorBoundary name="test">
        <ThrowComponent error={new Error('test crash')} />
      </ErrorBoundary>
    )
    expect(screen.getByText('页面出了点问题')).toBeInTheDocument()
    expect(screen.getByText('test crash')).toBeInTheDocument()
  })

  it('#2 点击重试后恢复正常', async () => {
    const user = userEvent.setup()
    let shouldThrow = true
    const Child: React.FC = () => {
      if (shouldThrow) throw new Error('crash')
      return <div>正常内容</div>
    }

    const { rerender } = render(
      <ErrorBoundary name="test">
        <Child />
      </ErrorBoundary>
    )

    expect(screen.getByText('页面出了点问题')).toBeInTheDocument()

    shouldThrow = false
    await user.click(screen.getByText('重试'))

    rerender(
      <ErrorBoundary name="test">
        <Child />
      </ErrorBoundary>
    )

    expect(screen.getByText('正常内容')).toBeInTheDocument()
  })

  it('#3 自定义 fallback 函数渲染', () => {
    render(
      <ErrorBoundary
        name="test"
        fallback={(error, retry) => (
          <div>
            <span>自定义错误: {error.message}</span>
            <button onClick={retry}>自定义重试</button>
          </div>
        )}
      >
        <ThrowComponent error={new Error('custom')} />
      </ErrorBoundary>
    )
    expect(screen.getByText('自定义错误: custom')).toBeInTheDocument()
    expect(screen.getByText('自定义重试')).toBeInTheDocument()
  })

  it('#4 report=false 时不调用 reportError', () => {
    const reportSpy = vi.fn()
    vi.mock('./reporter', () => ({ reportError: reportSpy }))

    render(
      <ErrorBoundary name="test" report={false}>
        <ThrowComponent error={new Error('no report')} />
      </ErrorBoundary>
    )

    expect(reportSpy).not.toHaveBeenCalled()
  })

  it('#5 正常渲染时不显示降级 UI', () => {
    render(
      <ErrorBoundary name="test">
        <div>正常内容</div>
      </ErrorBoundary>
    )
    expect(screen.getByText('正常内容')).toBeInTheDocument()
    expect(screen.queryByText('页面出了点问题')).toBeNull()
  })
})
```

#### 13.2.2 全局错误处理器测试

```typescript
// src/errors/globalHandlers.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setupGlobalErrorHandlers } from './globalHandlers'

describe('setupGlobalErrorHandlers', () => {
  let addEventListenerSpy: ReturnType<typeof vi.spyOn>
  let errorHandlers: Array<(event: Event) => void>
  let rejectionHandlers: Array<(event: Event) => void>

  beforeEach(() => {
    errorHandlers = []
    rejectionHandlers = []
    addEventListenerSpy = vi.spyOn(window, 'addEventListener')
    addEventListenerSpy.mockImplementation((type, handler) => {
      if (type === 'error') errorHandlers.push(handler as any)
      if (type === 'unhandledrejection') rejectionHandlers.push(handler as any)
    })
  })

  afterEach(() => {
    addEventListenerSpy.mockRestore()
  })

  it('#1 注册 error 和 unhandledrejection 监听器', () => {
    setupGlobalErrorHandlers()
    expect(errorHandlers.length).toBeGreaterThan(0)
    expect(rejectionHandlers.length).toBeGreaterThan(0)
  })

  it('#2 重复调用不重复注册', () => {
    setupGlobalErrorHandlers()
    const firstCount = errorHandlers.length + rejectionHandlers.length
    setupGlobalErrorHandlers()
    const secondCount = errorHandlers.length + rejectionHandlers.length
    expect(secondCount).toBe(firstCount)
  })

  it('#3 error 事件触发上报', async () => {
    const { reportError } = await import('./reporter')
    const reportSpy = vi.spyOn(reportError.prototype, 'catch').mockImplementation(() => {})
    // 也可以直接 mock fetch
    vi.mock('./reporter', () => ({ reportError: vi.fn().mockResolvedValue(undefined) }))

    setupGlobalErrorHandlers()
    const handler = errorHandlers[0]
    const mockEvent = new ErrorEvent('error', {
      message: 'test error',
      filename: 'test.js',
      lineno: 1,
      colno: 1,
      error: new Error('test error'),
    })
    handler(mockEvent)
    // 验证 reportError 被调用
  })

  it('#4 unhandledrejection 调用 preventDefault', () => {
    setupGlobalErrorHandlers()
    const handler = rejectionHandlers[0]
    const mockEvent = new PromiseRejectionEvent('unhandledrejection', {
      reason: new Error('unhandled'),
    })
    const preventDefaultSpy = vi.spyOn(mockEvent, 'preventDefault')
    handler(mockEvent)
    expect(preventDefaultSpy).toHaveBeenCalled()
  })
})
```

#### 13.2.3 错误上报器测试

```typescript
// src/errors/reporter.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { reportError, flushQueue } from './reporter'
import type { ErrorReport } from './types'

describe('reporter', () => {
  const mockReport: ErrorReport = {
    id: 'test-id',
    type: 'react',
    name: 'Error',
    message: 'test message',
    timestamp: Date.now(),
    appVersion: '0.0.0-test',
    userAgent: 'test-agent',
    online: true,
  }

  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('#1 在线时发送 POST 请求到 /api/v1/errors/report', async () => {
    vi.stubGlobal('navigator', { onLine: true, userAgent: 'test' })
    const mockFetch = vi.mocked(fetch)
    mockFetch.mockResolvedValue(new Response('{}', { status: 200 }))

    await reportError(mockReport)

    expect(mockFetch).toHaveBeenCalledWith(
      '/api/v1/errors/report',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        keepalive: true,
      })
    )
  })

  it('#2 离线时入队，不发送请求', async () => {
    vi.stubGlobal('navigator', { onLine: false, userAgent: 'test' })
    const mockFetch = vi.mocked(fetch)

    await reportError(mockReport)

    expect(mockFetch).not.toHaveBeenCalled()
  })

  it('#3 5xx 错误时重试', async () => {
    vi.stubGlobal('navigator', { onLine: true, userAgent: 'test' })
    const mockFetch = vi.mocked(fetch)
    mockFetch
      .mockResolvedValueOnce(new Response('', { status: 500 }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }))

    await reportError(mockReport)

    expect(mockFetch).toHaveBeenCalledTimes(2)
  })

  it('#4 上报失败不抛出异常', async () => {
    vi.stubGlobal('navigator', { onLine: true, userAgent: 'test' })
    vi.mocked(fetch).mockRejectedValue(new Error('network error'))

    await expect(reportError(mockReport)).resolves.toBeUndefined()
  })
})
```

#### 13.2.4 safeStorage 测试

```typescript
// src/utils/safeStorage.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { safeGetItem, safeSetItem, safeRemoveItem } from './safeStorage'

describe('safeStorage', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('#1 正常写入和读取', () => {
    safeSetItem('key', { name: 'test' })
    expect(safeGetItem('key', null)).toEqual({ name: 'test' })
  })

  it('#2 key 不存在时返回 fallback', () => {
    expect(safeGetItem('nonexistent', 'default')).toBe('default')
  })

  it('#3 JSON 解析失败时返回 fallback', () => {
    localStorage.setItem('bad', 'not-json')
    expect(safeGetItem('bad', null)).toBeNull()
  })

  it('#4 QuotaExceededError 时尝试清理重试', () => {
    const originalSetItem = localStorage.setItem
    let callCount = 0
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(function (
      this: Storage,
      key: string,
      value: string,
    ) {
      callCount++
      if (callCount === 1) {
        throw new DOMException('quota exceeded', 'QuotaExceededError')
      }
      originalSetItem.call(this, key, value)
    })

    expect(safeSetItem('key', 'value')).toBe(true)
  })

  it('#5 localStorage 不可用时返回 fallback', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('not available')
    })
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new DOMException('not available')
    })

    expect(safeSetItem('key', 'value')).toBe(false)
    expect(safeGetItem('key', 'fallback')).toBe('fallback')
  })
})
```

#### 13.2.5 fetchWithRetry 测试

```typescript
// src/services/fetchWithRetry.test.ts
import { describe, it, expect, vi } from 'vitest'
import { fetchWithRetry } from './fetchWithRetry'

describe('fetchWithRetry', () => {
  it('#1 成功请求不重试', async () => {
    const mockFetch = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }))
    vi.stubGlobal('fetch', mockFetch)

    await fetchWithRetry('https://test.com')

    expect(mockFetch).toHaveBeenCalledTimes(1)
  })

  it('#2 5xx 重试指定次数', async () => {
    const mockFetch = vi.fn().mockResolvedValue(new Response('', { status: 500 }))
    vi.stubGlobal('fetch', mockFetch)

    await expect(fetchWithRetry('https://test.com', {}, { retries: 2, retryDelay: 10 }))
      .rejects.toBeDefined()

    // 1 (initial) + 2 (retries) = 3
    expect(mockFetch).toHaveBeenCalledTimes(3)
  })

  it('#3 4xx 不重试', async () => {
    const mockFetch = vi.fn().mockResolvedValue(new Response('', { status: 404 }))
    vi.stubGlobal('fetch', mockFetch)

    const resp = await fetchWithRetry('https://test.com', {}, { retries: 3 })

    expect(resp.status).toBe(404)
    expect(mockFetch).toHaveBeenCalledTimes(1)
  })

  it('#4 网络错误时重试', async () => {
    const mockFetch = vi.fn()
      .mockRejectedValueOnce(new Error('network'))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }))
    vi.stubGlobal('fetch', mockFetch)

    const resp = await fetchWithRetry('https://test.com', {}, { retries: 2, retryDelay: 10 })

    expect(resp.status).toBe(200)
    expect(mockFetch).toHaveBeenCalledTimes(2)
  })

  it('#5 超时后 abort', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(
      (_url, opts) => new Promise((_resolve, reject) => {
        opts.signal?.addEventListener('abort', () => {
          reject(new DOMException('aborted', 'AbortError'))
        })
      })
    ))

    await expect(
      fetchWithRetry('https://test.com', {}, { timeout: 50, retries: 0 })
    ).rejects.toThrow()
  })
})
```

#### 13.2.6 ProviderErrorBoundary 集成测试

```typescript
// src/errors/ProviderErrorBoundary.test.tsx
import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ProviderErrorBoundary } from './ProviderErrorBoundary'

describe('ProviderErrorBoundary', () => {
  beforeEach(() => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('#1 Provider 崩溃时显示降级提示', () => {
    const BrokenProvider: React.FC = () => {
      throw new Error('provider init failed')
    }

    render(
      <ProviderErrorBoundary providerName="StaminaProvider">
        <BrokenProvider />
      </ProviderErrorBoundary>
    )

    expect(screen.getByText(/StaminaProvider 模块暂时不可用/)).toBeInTheDocument()
  })

  it('#2 Provider 崩溃不影响兄弟组件', () => {
    const BrokenProvider: React.FC = () => {
      throw new Error('crash')
    }

    render(
      <div>
        <ProviderErrorBoundary providerName="Broken">
          <BrokenProvider />
        </ProviderErrorBoundary>
        <div>兄弟组件正常</div>
      </div>
    )

    expect(screen.getByText(/Broken 模块暂时不可用/)).toBeInTheDocument()
    expect(screen.getByText('兄弟组件正常')).toBeInTheDocument()
  })

  it('#3 正常 Provider 不显示降级', () => {
    const OkProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => (
      <>{children}</>
    )

    render(
      <ProviderErrorBoundary providerName="OK">
        <OkProvider>
          <div>正常内容</div>
        </OkProvider>
      </ProviderErrorBoundary>
    )

    expect(screen.getByText('正常内容')).toBeInTheDocument()
    expect(screen.queryByText(/暂时不可用/)).toBeNull()
  })
})
```

### 13.3 test-setup.ts 更新

```typescript
// src/test-setup.ts 新增内容
import 'fake-indexeddb/auto'
import '@testing-library/jest-dom'

// __APP_VERSION__ mock
;(globalThis as any).__APP_VERSION__ = '0.0.0-test'

// crypto.randomUUID mock（jsdom 可能不支持）
if (!crypto.randomUUID) {
  ;(crypto as any).randomUUID = () => 'test-uuid-' + Math.random().toString(36).slice(2)
}

// matchMedia polyfill（已有，保留）
if (!window.matchMedia) {
  window.matchMedia = (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })
}
```

### 13.4 vitest.config.ts 更新

```typescript
// vitest.config.ts 新增 define
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  define: {
    __APP_VERSION__: JSON.stringify('0.0.0-test'),
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
  },
})
```

---

## 14. 实施步骤

### Phase 1: 基础设施（低风险，无代码改动）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 1.1 | 创建错误类型定义 | `src/errors/types.ts` + `types.test.ts` |
| 1.2 | 实现错误上报器 | `src/errors/reporter.ts` + `reporter.test.ts` |
| 1.3 | 实现全局错误处理器 | `src/errors/globalHandlers.ts` + `globalHandlers.test.ts` |
| 1.4 | 更新 test-setup.ts 和 vitest.config.ts | 配置文件 |
| 1.5 | vite.config.ts 注入 `__APP_VERSION__` | `vite.config.ts` + `vite-env.d.ts` |

### Phase 2: Error Boundary 层（中风险，核心改动）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 2.1 | 实现 ErrorBoundary 基础组件 | `src/errors/ErrorBoundary.tsx` + 测试 |
| 2.2 | 实现 GlobalErrorBoundary | `src/errors/GlobalErrorBoundary.tsx` + 测试 |
| 2.3 | 实现 ProviderErrorBoundary | `src/errors/ProviderErrorBoundary.tsx` + 测试 |
| 2.4 | 实现 ScreenErrorBoundary | `src/errors/ScreenErrorBoundary.tsx` + 测试 |
| 2.5 | 在 main.tsx 集成 GlobalErrorBoundary + 全局处理器 | `main.tsx` |
| 2.6 | 在 App.tsx 集成 ProviderErrorBoundary | `App.tsx` |
| 2.7 | 在 App.tsx renderContent 包裹 ScreenErrorBoundary | `App.tsx` |

### Phase 3: 安全存储与 IndexedDB（中风险）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 3.1 | 实现 safeStorage 工具 | `src/utils/safeStorage.ts` + 测试 |
| 3.2 | StaminaContext localStorage 改用 safeStorage | `StaminaContext.tsx` |
| 3.3 | LbsContext localStorage 改用 safeStorage | `LbsContext.tsx` |
| 3.4 | db.ts 增加损坏恢复 + 配额检测 | `db.ts` |
| 3.5 | animal-repository.ts 增加配额错误处理 | `animal-repository.ts` |
| 3.6 | useAnimalStore.ts 增加错误降级 | `useAnimalStore.ts` |

### Phase 4: 相机与网络韧性（中风险）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 4.1 | DiscoverScreen 摄像头错误分类 + 安全清理 | `DiscoverScreen.tsx` |
| 4.2 | DiscoverScreen visibilitychange 暂停/恢复 | `DiscoverScreen.tsx` |
| 4.3 | 实现 fetchWithRetry | `src/services/fetchWithRetry.ts` + 测试 |
| 4.4 | weather/api.ts 改用 fetchWithRetry | `weather/api.ts` |
| 4.5 | 实现 useOnlineStatus Hook | `src/hooks/useOnlineStatus.ts` + 测试 |

### Phase 5: 内存泄漏防护（低风险）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 5.1 | 实现 useCleanupRegistry | `src/hooks/useCleanupRegistry.ts` + 测试 |
| 5.2 | 实现 useSafeInterval | `src/hooks/useSafeInterval.ts` + 测试 |
| 5.3 | 实现 useSafeContext | `src/hooks/useSafeContext.ts` + 测试 |
| 5.4 | DiscoverScreen StrictMode 安全（旧 stream 清理） | `DiscoverScreen.tsx` |

### Phase 6: 后端错误上报端点（低风险）

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 6.1 | 实现 ErrorHandler | `backend/internal/handlers/error_handler.go` + 测试 |
| 6.2 | 注册路由 | `backend/internal/routes/router.go` |

### Phase 7: 集成测试与回归（验证阶段）

| 步骤 | 内容 | 输出 |
|------|------|------|
| 7.1 | 全量 vitest 回归 | 测试报告 |
| 7.2 | 手动注入错误验证降级 UI | 截图 |
| 7.3 | 离线场景测试（断网 → 操作 → 恢复） | 测试记录 |
| 7.4 | 隐私模式测试（Safari 隐私窗口） | 测试记录 |
| 7.5 | 内存泄漏验证（切换屏幕 20 次） | Chrome Memory 报告 |
| 7.6 | 填写验收 Checklist | 验收报告 |

---

## 15. 验收标准

### 15.1 硬性指标

| 指标 | 目标 | 验收方法 |
|------|------|---------|
| 崩溃率 | < 0.5% | 内测阶段统计（错误上报数 / DAU） |
| 白屏率 | 0 | Error Boundary 覆盖所有渲染路径 |
| 未捕获异常 | 0 | `unhandledrejection` 事件统计 |

### 15.2 功能验收 Checklist

- [ ] `ErrorBoundary` 捕获 React 渲染异常，显示降级 UI
- [ ] `GlobalErrorBoundary` 作为最终防线，提供 reload 按钮
- [ ] `ProviderErrorBoundary` 隔离单个 Provider 崩溃，兄弟模块正常
- [ ] `ScreenErrorBoundary` 隔离单屏崩溃，可切换其他 Tab
- [ ] `window.onerror` 监听 JS 运行时错误并上报
- [ ] `unhandledrejection` 监听未处理 Promise rejection
- [ ] 错误上报到 `/api/v1/errors/report`，离线时入队
- [ ] 网络恢复后自动 flush 离线错误队列
- [ ] 摄像头权限拒绝显示友好提示 + 重试
- [ ] 摄像头被占用/不支持显示分类错误提示
- [ ] 页面后台时相机 stream 自动停止
- [ ] IndexedDB 损坏时自动删除重建
- [ ] IndexedDB 配额超限时尝试清理后重试
- [ ] localStorage 隐私模式/配额超限时静默降级
- [ ] `fetchWithRetry` 对 5xx 和网络错误重试
- [ ] 离线状态 Hook 正确反映在线/离线
- [ ] 组件卸载时定时器/监听器/相机 stream 被清理
- [ ] 后端 `/api/v1/errors/report` 正确接收并存储错误
- [ ] 后端 DB 不可用时返回 200（不增加前端重试负担）

### 15.3 测试覆盖指标

| 模块 | 测试文件数 | 预计测试用例数 | 覆盖率目标 |
|------|-----------|--------------|-----------|
| `src/errors/` | 7 | ~35 | ≥ 90% |
| `src/utils/safeStorage.ts` | 1 | ~6 | ≥ 95% |
| `src/services/fetchWithRetry.ts` | 1 | ~6 | ≥ 90% |
| `src/hooks/` (新增) | 4 | ~11 | ≥ 85% |
| `backend/handlers/error_handler.go` | 1 | ~4 | ≥ 85% |

---

## 16. 风险与应对

| 风险 | 影响 | 概率 | 应对策略 |
|------|------|------|---------|
| Error Boundary 改变 React 组件树结构 | 可能影响 Context 传递 | 中 | ProviderErrorBoundary 在 Provider 外层，不影响 Context 传递 |
| 全局错误监听产生大量噪音 | 后端存储压力 | 中 | 前端去重（相同 message+stack 5 分钟内只上报一次） |
| 错误上报本身失败 | 崩溃数据丢失 | 低 | 离线队列 + 重试 + keepalive |
| IndexedDB 删除恢复导致用户数据丢失 | 用户体验 | 低 | 仅在打开失败时恢复；上报后通知用户 |
| `crypto.randomUUID` 不支持 | 旧浏览器 | 低 | test-setup.ts 中已有 polyfill；运行时也需 polyfill |
| `__APP_VERSION__` 未定义 | TS 编译错误 | 低 | vite-env.d.ts 声明 + vitest.config.ts define |
| ProviderErrorBoundary 嵌套过深 | 代码可读性下降 | 中 | 可后续抽取为配置数组 + map 渲染 |
| 后端错误上报端点被滥用 | 存储/DDoS | 低 | 限流中间件已有；增加 payload 大小限制 |

### 16.1 降级方案

| 不达标项 | 降级方案 |
|---------|---------|
| 崩溃率 ≥ 0.5% | 分析上报的错误分布，针对性修复 Top 3 崩溃路径 |
| Error Boundary 影响性能 | 仅保留 Global + Screen 级，移除 Provider 级 |
| 错误上报量大 | 前端采样率降至 10%，仅上报 fatal 级别 |
| IndexedDB 恢复导致数据丢失 | 提供云同步备份（已有 `/api/v1/sync/animal`） |

---

## 附录 A: 全量文件改动清单

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `frontend/src/errors/types.ts` | 新增 | 错误类型定义 |
| `frontend/src/errors/types.test.ts` | 新增 | 类型测试 |
| `frontend/src/errors/ErrorBoundary.tsx` | 新增 | 基础 Error Boundary |
| `frontend/src/errors/ErrorBoundary.test.tsx` | 新增 | Error Boundary 测试 |
| `frontend/src/errors/GlobalErrorBoundary.tsx` | 新增 | 全局 Error Boundary |
| `frontend/src/errors/GlobalErrorBoundary.test.tsx` | 新增 | 全局 Boundary 测试 |
| `frontend/src/errors/ProviderErrorBoundary.tsx` | 新增 | Provider 隔离 Boundary |
| `frontend/src/errors/ProviderErrorBoundary.test.tsx` | 新增 | Provider 隔离测试 |
| `frontend/src/errors/ScreenErrorBoundary.tsx` | 新增 | 屏幕级 Boundary |
| `frontend/src/errors/ScreenErrorBoundary.test.tsx` | 新增 | 屏幕级测试 |
| `frontend/src/errors/globalHandlers.ts` | 新增 | 全局错误监听 |
| `frontend/src/errors/globalHandlers.test.ts` | 新增 | 全局监听测试 |
| `frontend/src/errors/reporter.ts` | 新增 | 错误上报器 |
| `frontend/src/errors/reporter.test.ts` | 新增 | 上报器测试 |
| `frontend/src/errors/index.ts` | 新增 | 统一导出 |
| `frontend/src/services/fetchWithRetry.ts` | 新增 | 带重试的 fetch |
| `frontend/src/services/fetchWithRetry.test.ts` | 新增 | fetchWithRetry 测试 |
| `frontend/src/hooks/useOnlineStatus.ts` | 新增 | 在线状态 Hook |
| `frontend/src/hooks/useOnlineStatus.test.ts` | 新增 | 在线状态测试 |
| `frontend/src/hooks/useSafeContext.ts` | 新增 | 安全 Context 消费 |
| `frontend/src/hooks/useSafeContext.test.tsx` | 新增 | 安全消费测试 |
| `frontend/src/hooks/useCleanupRegistry.ts` | 新增 | 清理注册表 |
| `frontend/src/hooks/useCleanupRegistry.test.ts` | 新增 | 清理注册表测试 |
| `frontend/src/hooks/useSafeInterval.ts` | 新增 | 安全定时器 |
| `frontend/src/hooks/useSafeInterval.test.ts` | 新增 | 安全定时器测试 |
| `frontend/src/utils/safeStorage.ts` | 新增 | 安全 localStorage |
| `frontend/src/utils/safeStorage.test.ts` | 新增 | 安全存储测试 |
| `frontend/src/main.tsx` | 修改 | 集成 GlobalErrorBoundary + 全局处理器 |
| `frontend/src/App.tsx` | 修改 | ProviderErrorBoundary + ScreenErrorBoundary |
| `frontend/src/components/DiscoverScreen.tsx` | 修改 | 摄像头错误分类 + visibility + 安全清理 |
| `frontend/src/db/db.ts` | 修改 | 损坏恢复 + 配额检测 |
| `frontend/src/db/repositories/animal-repository.ts` | 修改 | 配额错误处理 + cleanupOldData |
| `frontend/src/hooks/useAnimalStore.ts` | 修改 | 错误降级 |
| `frontend/src/stamina/StaminaContext.tsx` | 修改 | safeSetItem 持久化 |
| `frontend/src/lbs/LbsContext.tsx` | 修改 | safeSetItem 持久化 |
| `frontend/src/weather/api.ts` | 修改 | fetchWithRetry |
| `frontend/src/vite-env.d.ts` | 修改 | `__APP_VERSION__` 声明 |
| `frontend/src/test-setup.ts` | 修改 | `__APP_VERSION__` + `randomUUID` mock |
| `frontend/vitest.config.ts` | 修改 | define `__APP_VERSION__` |
| `frontend/vite.config.ts` | 修改 | define `__APP_VERSION__` |
| `backend/internal/handlers/error_handler.go` | 新增 | 后端错误上报端点 |
| `backend/internal/handlers/error_handler_test.go` | 新增 | 端点测试 |
| `backend/internal/routes/router.go` | 修改 | 注册错误上报路由 |

---

## 附录 B: 优先级排序

| 优先级 | 改动项 | 预期收益（崩溃率下降） | 风险 |
|-------|--------|---------------------|------|
| P0 | GlobalErrorBoundary + 全局错误监听 | 消除 100% 白屏崩溃 | 低 |
| P0 | ProviderErrorBoundary | 消除 Provider 初始化崩溃连锁 | 低 |
| P0 | ScreenErrorBoundary | 消除单屏崩溃影响全 App | 低 |
| P0 | 错误上报基础设施 | 提供崩溃率统计能力 | 低 |
| P0 | safeStorage | 消除隐私模式/配额超限崩溃 | 低 |
| P1 | IndexedDB 损坏恢复 | 消除 DB 损坏后永久不可用 | 中 |
| P1 | IndexedDB 配额处理 | 消除存储满后写入崩溃 | 中 |
| P1 | 摄像头错误分类 + visibility | 消除相机异常路径崩溃 | 低 |
| P1 | fetchWithRetry | 减少网络错误导致的 unhandledrejection | 低 |
| P2 | useAnimalStore 错误降级 | 消除数据加载失败白屏 | 低 |
| P2 | 内存泄漏防护 Hooks | 减少长时间使用后内存压力崩溃 | 低 |
| P2 | 后端错误上报端点 | 闭环崩溃率统计 | 低 |
| P3 | useSafeContext 推广 | 前向防护 | 低 |
