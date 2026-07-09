# [M2] 防作弊加固（相册导入/模拟器/root 检测）实现计划 — Issue #44

> **验收标准**：相册导入检测、模拟器检测、root/越狱检测、位置欺骗检测、时间篡改检测生效；服务端数据完整性校验上链；客户端加固（混淆/完整性/限流）落地
>
> **设计文档来源**：`游戏开发计划.md` 3.2 发现机制（防作弊核心：仅接受实时取景帧、禁止相册导入/模拟器/root、帧时间戳连续性校验、EXIF 与传感器交叉验证、检测框运动轨迹合理性）、3.3 捕获反作弊（投掷轨迹物理参数校验）、4.1 架构图（反作弊审计模块）、4.4 同步方案（UUID+设备ID+时间戳+推理请求ID）、4.5 隐私策略（推理请求 ID 全链路留存）、7.3 内测验收（防作弊加固：相册导入、模拟器、root 检测生效）、第九章 风险表（翻拍照片/模拟器/改包刷稀有度应对策略）
>
> **技术栈**：前端 React 18 + Vite 6 + TypeScript 5.6（PWA），后端 Go (Gin) + GORM + MySQL + JWT
>
> **现有基础**：
> - **后端反作弊已实现**：
>   - `backend/internal/services/audit.go` — `AuditService.CheckAnomaly()`：规则1（同设备 10 分钟内 ≥3 只传说级告警）、规则2（推理请求 ID 复用告警）
>   - `backend/internal/middleware/ratelimit.go` — 令牌桶限流器（100 req/min, burst 10），按 device_id / IP 限流
>   - `backend/internal/middleware/costlimit.go` — 每日调用上限（detect 100 次 / analyze 50 次 / value 20 次）
>   - `backend/internal/middleware/auth.go` — JWT 鉴权（Bearer Token，device_id 注入 context）
>   - `backend/internal/handlers/sync.go` — 同步时去重（UUID 唯一约束）+ 反作弊审计（不阻断仅告警）
>   - `backend/internal/models/models.go` — `Animal` 模型含 `InferenceRequestID`、`DeviceID`、`GeneratedAt`、`Latitude/Longitude`；`AuditLog` 模型记录告警
> - **前端现状**：
>   - `frontend/src/services/visionDetect.ts` — Mock 检测器，无真实 API 调用，无防作弊采集
>   - `frontend/src/components/DiscoverScreen.tsx` — `getUserMedia({ video: { facingMode: 'environment' } })` 拍照，`canvas.toDataURL()` 转 base64，无时间戳/传感器/EXIF 采集
>   - `frontend/src/lbs/LbsContext.tsx` — `navigator.geolocation.getCurrentPosition()` 定位，无 mock 位置检测
>   - 无反作弊服务模块、无设备指纹、无完整性签名
> - **关键缺口**：
>   1. 前端无法区分实时取景帧 vs 相册导入/翻拍
>   2. 无模拟器/root/越狱检测
>   3. 无位置欺骗（mock GPS）检测
>   4. 无时间篡改检测
>   5. 同步请求无数据完整性签名（客户端可伪造任意数值上传）
>   6. 无请求防重放机制
>   7. 前端代码未混淆/加固

---

## 1. 威胁模型（Threat Model）

### 1.1 攻击面分析

| 攻击面 | 攻击方式 | 危害等级 | 现有防御 |
|--------|---------|---------|---------|
| **相机输入** | 相册导入预存照片 | 🔴 高 | 无 |
| **相机输入** | 对屏幕翻拍照片 | 🟡 中 | 无 |
| **相机输入** | 虚拟摄像头注入（OBS/DroidCam） | 🟡 中 | 无 |
| **设备环境** | Android 模拟器（Genymotion/AVD） | 🔴 高 | 无 |
| **设备环境** | iOS 模拟器 | 🟡 中 | 无 |
| **设备环境** | Root/越狱设备篡改客户端逻辑 | 🔴 高 | 无 |
| **设备环境** | 改包/注入（Frida/Xposed）篡改请求 | 🔴 高 | 无 |
| **位置** | Mock GPS 应用伪造坐标 | 🔴 高 | 无 |
| **位置** | VPN + GPS 欺骗跨区域刷榜 | 🟡 中 | 无 |
| **时间** | 篡改设备时钟绕过冷却/签到 | 🟡 中 | 无 |
| **网络** | 重放合法捕获请求刷动物 | 🟡 中 | UUID 去重部分覆盖 |
| **网络** | 篡改同步数据（修改稀有度/属性） | 🔴 高 | 无（客户端直传数值） |
| **API** | 高频调用暴力刷检测 | 🟡 中 | 限流 + 每日上限已有 |
| **API** | 伪造推理请求 ID | 🟡 中 | ID 复用检测已有 |

### 1.2 作弊者动机与典型场景

| 场景 | 动机 | 典型手段 |
|------|------|---------|
| 刷传说级动物 | 获取高稀有度宠物用于战斗/炫耀 | 模拟器 + 相册导入 + 改包改稀有度 |
| 跨区域刷榜 | 为省份/城市冲榜 | Mock GPS 伪造坐标 |
| 绕过体力限制 | 无限捕获 | 篡改设备时间加速体力恢复 |
| 批量刷金币 | 派遣淘金币获利 | 多开模拟器批量刷 |
| 伪造数值 | 超高属性宠物碾压 PvP | 改包篡改同步请求 JSON |

### 1.3 防御策略总纲

采用**纵深防御（Defense in Depth）**策略，分三层：

```
┌─────────────────────────────────────────────────────────┐
│  第一层：客户端检测（前哨预警，可绕过但提高门槛）          │
│  - 相册导入检测 / 模拟器检测 / root 检测                  │
│  - 位置欺骗检测 / 时间篡改检测                            │
│  - 设备指纹采集 / 传感器数据采集                          │
│  - 代码混淆 / 完整性校验                                  │
├─────────────────────────────────────────────────────────┤
│  第二层：服务端验证（核心防线，不可绕过）                  │
│  - 数据完整性签名校验（HMAC）                             │
│  - 推理请求 ID 全链路验证（后端下发，不可伪造）            │
│  - 数值范围校验 / 物理轨迹合理性校验                      │
│  - 异常行为审计（频次/稀有度/位置跳跃）                    │
│  - 防重放（nonce + timestamp 窗口）                      │
├─────────────────────────────────────────────────────────┤
│  第三层：运营审计（事后追责，威慑）                       │
│  - 审计日志全链路留存                                     │
│  - 异常告警 → 人工审核                                    │
│  - 封号/降级处理（仅浏览模式）                            │
│  - 区域排行刷量检测                                       │
└─────────────────────────────────────────────────────────┘
```

**核心原则**：客户端检测仅作为预警和门槛提升，**所有关键校验必须在服务端执行**，客户端不可信。

---

## 2. 客户端检测机制（Client-Side Detection）

### 2.1 相册导入 / 翻拍检测

#### 2.1.1 实时取景帧验证

**原理**：通过 `getUserMedia` 获取的 `MediaStream` 具有 `getVideoTracks()` 返回的 `MediaStreamTrack`，其 `label` 属性包含设备标识（如 "camera2"），且 `settings` 属性包含实时参数。相册导入或虚拟摄像头注入时，这些属性会出现异常。

**实现方案**：

```typescript
// frontend/src/services/antiCheat/cameraVerify.ts

export interface CameraProof {
  /** MediaStreamTrack.label（摄像头设备名） */
  trackLabel: string
  /** track.settings() 快照（width/height/frameRate/facingMode） */
  trackSettings: MediaTrackSettings
  /** track.readyState（必须为 "live"） */
  trackState: MediaStreamTrackState
  /** 取景帧时间戳序列（连续性校验） */
  frameTimestamps: number[]
  /** 设备运动传感器数据（与帧同步采集） */
  motionData: MotionSample[]
  /** 会话 nonce（服务端下发，防重放） */
  sessionNonce: string
}

/** 采集相机证明数据 */
export async function collectCameraProof(
  stream: MediaStream,
  sessionNonce: string,
): Promise<CameraProof> {
  const tracks = stream.getVideoTracks()
  if (tracks.length === 0) throw new Error('no video tracks')

  const track = tracks[0]
  const settings = track.getSettings()

  // 可疑检测：虚拟摄像头 label 通常含 "virtual" / "obs" / "dummy"
  const suspiciousLabels = ['virtual', 'obs', 'dummy', 'screen', 'capture']
  const isSuspicious = suspiciousLabels.some(
    (s) => track.label.toLowerCase().includes(s),
  )

  return {
    trackLabel: track.label,
    trackSettings: settings,
    trackState: track.readyState,
    frameTimestamps: [], // 由 capturePhoto 填充
    motionData: [],      // 由传感器监听填充
    sessionNonce,
  }
}
```

#### 2.1.2 帧时间戳连续性校验

**原理**：实时取景帧的时间戳间隔应与帧率（2~5 FPS 采样）一致。相册导入或翻拍时，帧时间戳要么全部相同（单张照片），要么间隔不规律。

**实现方案**：

在 `DiscoverScreen.tsx` 的 `capturePhoto` 中，记录每次取帧的 `performance.now()` 时间戳，形成时间戳序列上传后端校验。

```typescript
// 在 DiscoverScreen 中采集
const frameTimestampsRef = useRef<number[]>([])

// 拍照时记录
const capturePhoto = useCallback(() => {
  frameTimestampsRef.current.push(performance.now())
  // ... 现有拍照逻辑
}, [])
```

**校验规则（服务端）**：
- 相邻帧间隔 ∈ [150ms, 600ms]（对应 2~5 FPS 采样）
- 总帧数 ≥ 2（至少两帧才证明连续取景）
- 首帧时间戳与会话开始时间差 ≤ 5min（防止超长会话）

#### 2.1.3 设备运动传感器交叉验证

**原理**：手持设备拍摄时，加速度计/陀螺仪会有微小抖动。对屏幕翻拍或静态照片导入时，传感器数据完全静止。通过 `DeviceMotionEvent` / `DeviceOrientationEvent` 采集传感器数据与帧时间戳对齐。

**实现方案**：

```typescript
// frontend/src/services/antiCheat/sensorCollect.ts

export interface MotionSample {
  timestamp: number  // performance.now()
  accelX: number
  accelY: number
  accelZ: number
  rotationAlpha: number
  rotationBeta: number
  rotationGamma: number
}

/** 启动传感器采集，返回停止函数 */
export function startMotionSampling(
  samples: MotionSample[],
  intervalMs = 200,
): () => void {
  const handler = (event: DeviceMotionEvent) => {
    const accel = event.accelerationIncludingGravity
    const rot = event.rotationRate
    if (accel && rot) {
      samples.push({
        timestamp: performance.now(),
        accelX: accel.x ?? 0,
        accelY: accel.y ?? 0,
        accelZ: accel.z ?? 0,
        rotationAlpha: rot.alpha ?? 0,
        rotationBeta: rot.beta ?? 0,
        rotationGamma: rot.gamma ?? 0,
      })
    }
  }
  window.addEventListener('devicemotion', handler)
  return () => window.removeEventListener('devicemotion', handler)
}
```

**校验规则（服务端）**：
- 传感器样本数 ≥ 5（拍摄期间至少 1 秒数据）
- 加速度方差 > 阈值（手持抖动特征，如 > 0.01 m/s²）
- 若加速度方差 ≈ 0，标记为可疑（可能为翻拍/静态照片）

**注意**：iOS 13+ 需要 `DeviceMotionEvent.requestPermission()`，需用户手势触发。对不支持传感器的设备（桌面浏览器），降级为仅时间戳校验。

#### 2.1.4 EXIF / Canvas 指纹

**原理**：`getUserMedia` + `canvas.drawImage()` 生成的图片无 EXIF 元数据。若用户通过 `<input type="file" accept="image/*" capture>` 或其他方式导入相册照片，图片可能携带 EXIF（拍摄时间、GPS、设备型号等）。

**实现方案**：

在帧上传前，检查 Canvas 生成的 blob 是否包含 EXIF 数据。由于 `canvas.toDataURL()` 不携带 EXIF，若服务端收到的图片包含 EXIF，则说明非 Canvas 截取，可判定为相册导入。

### 2.2 模拟器检测

#### 2.2.1 Web 端可检测的模拟器特征

| 检测项 | 真机特征 | 模拟器特征 | 检测方式 |
|--------|---------|-----------|---------|
| User-Agent | 包含真实设备型号 | 包含 "sdk"/"emulator"/"simulator" | `navigator.userAgent` |
| GPU 渲染器 | 真实 GPU（Adreno/Mali/PowerVR） | "Android Emulator"/"SwiftShader" | WebGL `getParameter(UNMASKED_RENDERER_WEBGL)` |
| 屏幕尺寸 | 物理像素密度真实 | 常见 320×480 / 360×640 | `window.screen` / `devicePixelRatio` |
| 传感器 | 加速度计/陀螺仪有数据 | 数据全为 0 或不存在 | `DeviceMotionEvent` 存在且非零 |
| 触摸 | 多点触摸 + 真实压力 | 无触摸或单点 | `navigator.maxTouchPoints` |
| CPU 核心数 | 4/8 核 | 通常 1~2 核 | `navigator.hardwareConcurrency` |
| 内存 | ≥ 2GB | 通常 ≤ 1GB | `navigator.deviceMemory` |
| 电池 | 有电池且会放电 | API 不存在或 level=1 且 charging=true | `navigator.getBattery()` |
| 时区/语言 | 与 GPS 一致 | 可能不匹配 | `Intl.DateTimeFormat().resolvedOptions().timeZone` |

**实现方案**：

```typescript
// frontend/src/services/antiCheat/emulatorDetect.ts

export interface EmulatorCheckResult {
  isEmulator: boolean
  signals: EmulatorSignal[]
  riskScore: number  // 0~100, 越高越可疑
}

interface EmulatorSignal {
  name: string
  value: string
  suspicious: boolean
  weight: number
}

export function detectEmulator(): EmulatorCheckResult {
  const signals: EmulatorSignal[] = []

  // 1. User-Agent 检测
  const ua = navigator.userAgent.toLowerCase()
  const uaSuspicious = /sdk|emulator|simulator|genymotion|bluestacks|nox|ldplayer/i.test(ua)
  signals.push({
    name: 'user_agent',
    value: navigator.userAgent,
    suspicious: uaSuspicious,
    weight: 30,
  })

  // 2. GPU 渲染器检测
  const gl = document.createElement('canvas').getContext('webgl')
  const dbg = gl?.getExtension('WEBGL_debug_renderer_info')
  const renderer = dbg ? gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL) : 'unknown'
  const rendererSuspicious = /swiftshader|android emulator|google swiftshader|llvmpipe/i.test(String(renderer))
  signals.push({
    name: 'gpu_renderer',
    value: String(renderer),
    suspicious: rendererSuspicious,
    weight: 25,
  })

  // 3. 硬件并发数
  const cores = navigator.hardwareConcurrency || 0
  signals.push({
    name: 'cpu_cores',
    value: String(cores),
    suspicious: cores <= 2,
    weight: 10,
  })

  // 4. 设备内存（Chrome 系，单位 GB）
  const memory = (navigator as any).deviceMemory || 0
  signals.push({
    name: 'device_memory',
    value: `${memory}GB`,
    suspicious: memory > 0 && memory <= 1,
    weight: 10,
  })

  // 5. 触摸点数
  const touchPoints = navigator.maxTouchPoints || 0
  signals.push({
    name: 'touch_points',
    value: String(touchPoints),
    suspicious: touchPoints === 0 && /Mobile|Android|iPhone/i.test(ua),
    weight: 10,
  })

  // 6. 屏幕尺寸（模拟器常见低分辨率）
  const screenArea = window.screen.width * window.screen.height
  signals.push({
    name: 'screen_resolution',
    value: `${window.screen.width}x${window.screen.height}`,
    suspicious: screenArea <= 320 * 480,
    weight: 5,
  })

  // 7. 传感器存在性（异步检测，此处仅标记，实际值由 sensorCollect 填充）
  signals.push({
    name: 'sensor_available',
    value: typeof DeviceMotionEvent !== 'undefined' ? 'available' : 'missing',
    suspicious: typeof DeviceMotionEvent === 'undefined' && /Mobile|Android|iPhone/i.test(ua),
    weight: 10,
  })

  // 计算风险分
  const riskScore = signals
    .filter((s) => s.suspicious)
    .reduce((sum, s) => sum + s.weight, 0)

  return {
    isEmulator: riskScore >= 50,
    signals,
    riskScore: Math.min(100, riskScore),
  }
}
```

#### 2.2.2 Capacitor 原生层检测（如使用 Capacitor 封装）

若使用 Capacitor 封装上架，可通过原生插件获取更可靠的模拟器/root 检测信号：

- **Android**：检测 `Build.FINGERPRINT`（含 "generic"/"sdk"）、`Build.MODEL`（含 "Emulator"）、`Build.MANUFACTURER`（"Genymotion"）、`/system/bin/qemu-props` 文件存在性、`/proc/cpuinfo` 含 "goldfish"
- **iOS**：检测 `TARGET_OS_SIMULATOR` 编译宏、`sysctlbyname("hw.machine")` 返回 "x86_64"

**建议**：内测阶段引入 `@capacitor-community/device` 或自定义 Capacitor 插件做原生层检测，Web 端检测作为降级方案。

### 2.3 Root / 越狱检测

#### 2.3.1 Web 端间接检测

Web 端无法直接检测 root/越狱，但可通过以下间接信号：

| 检测项 | 正常设备 | Root/越狱设备 |
|--------|---------|-------------|
| WebView 调试可用 | `false` | `true`（开发者开启） |
| 某些原生 API 被劫持 | 正常返回 | 异常或被 hook |
| `navigator.webdriver` | `false`/`undefined` | `true`（自动化工具） |

#### 2.3.2 Capacitor 原生层检测

**Android root 检测**：
- 检查 `su` 二进制文件：`/system/bin/su`、`/system/xbin/su`、`/sbin/su`、`/vendor/bin/su`
- 检查 Magisk 特征：`/sbin/.magisk`、`/data/adb/magisk`
- 检查 Xposed 框架：尝试加载 `de.robv.android.xposed.XposedBridge` 类
- 检查 `Build.TAGS`（正式版应为 "release-keys"，非 "test-keys"）
- 检查 SELinux 状态（root 后可能 permissive）

**iOS 越狱检测**：
- 检查 Cydia 文件：`/Applications/Cydia.app`、`/private/var/lib/apt/`
- 检查 Sileo：`/Applications/Sileo.app`
- 检查 `/bin/bash`、`/usr/sbin/sshd` 存在性
- 检查 `fork()` 是否可调用（沙盒内不可 fork）

**实现方案**：创建 Capacitor 插件 `@animalpoke/anticheat`，暴露 `checkRoot()` / `checkJailbreak()` 方法。

```typescript
// frontend/src/services/antiCheat/rootDetect.ts

export interface RootCheckResult {
  isRooted: boolean
  signals: string[]
  riskScore: number
}

export async function checkRootStatus(): Promise<RootCheckResult> {
  // Web 端检测（有限）
  const webSignals: string[] = []

  // 1. webdriver 检测（自动化工具注入标志）
  if ((navigator as any).webdriver === true) {
    webSignals.push('navigator.webdriver=true')
  }

  // 2. 尝试 Capacitor 原生插件（如果可用）
  try {
    const { RootCheck } = await import('@animalpoke/anticheat')
    const result = await RootCheck.check()
    return {
      isRooted: result.isRooted,
      signals: [...webSignals, ...result.signals],
      riskScore: result.isRooted ? 100 : (webSignals.length > 0 ? 30 : 0),
    }
  } catch {
    // Capacitor 插件不可用（纯 PWA 模式），仅返回 Web 端检测结果
    return {
      isRooted: webSignals.length > 0,
      signals: webSignals,
      riskScore: webSignals.length > 0 ? 30 : 0,
    }
  }
}
```

#### 2.3.3 检测后处理策略

根据设计文档 3.2："禁止 root/越狱设备（检测到则降级为'仅浏览'模式）"

| 风险分 | 处理策略 |
|--------|---------|
| 0~29（低风险） | 正常游戏 |
| 30~49（中风险） | 正常游戏，服务端标记设备为"关注"，加强审计 |
| 50~79（高风险） | 允许发现/捕获但服务端强制二次校验（人工审核延迟发放） |
| 80~100（极高风险） | 降级为"仅浏览"模式，禁止发现/捕获/同步 |

### 2.4 位置欺骗检测

#### 2.4.1 浏览器 Geolocation API 的 mock 检测

**原理**：`Position` 对象的 `coords` 包含一些可用于判断欺骗的字段。

| 检测项 | 说明 |
|--------|------|
| `position.coords.accuracy` | Mock GPS 通常返回极高精度（< 5m），真机一般为 10~100m |
| `position.timestamp` | 与 `Date.now()` 差值过大说明可能被篡改 |
| 位置跳跃 | 两次定位距离/时间比超过物理速度（如 30s 内移动 100km） |

**实现方案**：

```typescript
// frontend/src/services/antiCheat/locationVerify.ts

export interface LocationProof {
  lat: number
  lng: number
  accuracy: number
  timestamp: number        // position.timestamp
  clientTimestamp: number  // Date.now() at collection
  mockDetected: boolean
  mockSignals: string[]
}

export function analyzeLocation(
  current: GeoLocation,
  previous?: GeoLocation,
): LocationProof {
  const signals: string[] = []
  const now = Date.now()

  // 1. 精度过高（mock GPS 常见特征）
  if (current.accuracy < 3) {
    signals.push(`accuracy_too_high: ${current.accuracy}m`)
  }

  // 2. timestamp 偏差
  // 注：position.timestamp 来自系统，若与 Date.now() 偏差大则可疑

  // 3. 位置跳跃检测
  if (previous) {
    const distance = haversineDistance(current, previous)
    const timeDiff = (now - previous.timestamp) / 1000  // 秒
    const speed = distance / timeDiff  // m/s
    // 步行 ~1.4 m/s，跑步 ~5 m/s，驾车 ~30 m/s，高铁 ~100 m/s
    if (speed > 150) {
      signals.push(`speed_anomaly: ${speed.toFixed(1)}m/s`)
    }
  }

  return {
    lat: current.lat,
    lng: current.lng,
    accuracy: current.accuracy,
    timestamp: current.timestamp,
    clientTimestamp: now,
    mockDetected: signals.length > 0,
    mockSignals: signals,
  }
}

function haversineDistance(a: GeoLocation, b: GeoLocation): number {
  const R = 6371000  // 地球半径 m
  const dLat = ((b.lat - a.lat) * Math.PI) / 180
  const dLng = ((b.lng - a.lng) * Math.PI) / 180
  const lat1 = (a.lat * Math.PI) / 180
  const lat2 = (b.lat * Math.PI) / 180
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLng / 2) ** 2
  return 2 * R * Math.asin(Math.sqrt(h))
}
```

#### 2.4.2 服务端交叉验证

服务端收到坐标后，与以下数据交叉验证：
- **IP 地理位置**：通过 IP 查地理位置（`geoip`），与 GPS 坐标比较。若 IP 在北京、GPS 在纽约，明显欺骗
- **历史轨迹**：同设备最近一次定位的距离/时间比，检测物理不可能的移动
- **腾讯地图逆地理编码**：验证坐标对应的行政区划是否合理

### 2.5 时间篡改检测

#### 2.5.1 客户端时间 vs 服务端时间

**原理**：客户端 `Date.now()` 可被篡改（改系统时间），但服务端时间不可篡改。

**实现方案**：

```typescript
// frontend/src/services/antiCheat/timeVerify.ts

export interface TimeSyncResult {
  clientTime: number
  serverTime: number
  offset: number       // clientTime - serverTime
  isManipulated: boolean
}

/** 与服务端时间同步，检测偏差 */
export async function checkTimeSync(): Promise<TimeSyncResult> {
  const clientTime = Date.now()
  try {
    const resp = await fetch('/api/v1/health', { method: 'GET' })
    const serverTimeHeader = resp.headers.get('X-Server-Time')
    const serverTime = serverTimeHeader
      ? parseInt(serverTimeHeader, 10)
      : Date.now()
    const offset = clientTime - serverTime
    // 偏差超过 5 分钟判定为可疑
    return {
      clientTime,
      serverTime,
      offset,
      isManipulated: Math.abs(offset) > 5 * 60 * 1000,
    }
  } catch {
    return {
      clientTime,
      serverTime: clientTime,
      offset: 0,
      isManipulated: false,
    }
  }
}
```

#### 2.5.2 服务端时间源

- **响应头注入**：所有 API 响应注入 `X-Server-Time: <unix_ms>` 头
- **体力恢复校验**：体力恢复基于服务端时间，客户端时间篡改无效（服务端记录上次体力值与时间戳，计算恢复量）
- **签到校验**：签到日期以服务端时间为准，客户端时间不影响

#### 2.5.3 体力防篡改设计

体力恢复必须**完全在服务端计算**：
- 客户端发送"使用体力"请求 → 服务端校验当前体力（基于上次记录的时间戳 + 恢复速率计算） → 扣减 → 返回新值
- 客户端仅做 UI 展示，不存储体力真实值（或仅缓存，以服务端为准）

> **注**：当前 `StaminaContext` 纯客户端实现，需在 M2 阶段将体力状态迁移至服务端管理。此为防作弊的关键改动，但属于独立 Issue 范围，本计划仅标注依赖关系。

---

## 3. 服务端验证策略（Server-Side Validation）

### 3.1 数据完整性签名（HMAC）

**核心问题**：当前 `syncRequest` 中客户端直接传 `Rarity`、`HP`、`ATK` 等数值，攻击者可篡改为任意值。

**方案**：引入 HMAC 签名链路，关键数据由服务端生成并签名，客户端仅透传。

#### 3.1.1 推理请求 ID 全链路绑定

```
完整链路：
1. 客户端 POST /vision/detect（上传帧 + CameraProof）
   → 服务端校验 CameraProof
   → 服务端调 VLM
   → 服务端生成 inference_request_id（UUID）
   → 服务端返回 { detection_result, inference_request_id, server_timestamp, hmac_signature }

2. 客户端 POST /vision/analyze（上传帧 + inference_request_id）
   → 服务端校验 inference_request_id 有效性（存在、未过期、属于该设备）
   → 服务端调 VLM
   → 服务端返回 { analysis_result, inference_request_id, server_timestamp, hmac_signature }

3. 客户端 POST /value/generate（传 inference_request_id + analysis_result）
   → 服务端校验 inference_request_id + hmac
   → 服务端调 LLM
   → 服务端返回 { rarity, hp, atk, def, spd, ..., inference_request_id, server_timestamp, hmac_signature }

4. 客户端 POST /sync/animal（传所有元数据 + hmac_signature 链）
   → 服务端校验 hmac_signature 链完整性
   → 服务端校验数值与推理结果一致
   → 落库
```

**关键设计**：每一步的推理结果由服务端生成并签名（HMAC-SHA256），客户端不可篡改。同步时服务端验证签名链，确保数值来自真实推理链路。

#### 3.1.2 HMAC 签名实现

```go
// backend/internal/services/integrity.go

package services

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "time"
)

// IntegrityService 数据完整性签名服务。
type IntegrityService struct {
    secret []byte
}

func NewIntegrityService(secret string) *IntegrityService {
    return &IntegrityService{secret: []byte(secret)}
}

// SignedPayload 带签名的载荷。
type SignedPayload struct {
    Data      json.RawMessage `json:"data"`
    Timestamp int64           `json:"ts"`     // 服务端时间戳 (unix ms)
    Nonce     string          `json:"nonce"`  // 随机 nonce
    HMAC      string          `json:"hmac"`   // HMAC-SHA256(data + ts + nonce + secret)
}

// Sign 对数据签名。
func (s *IntegrityService) Sign(data interface{}) (*SignedPayload, error) {
    raw, err := json.Marshal(data)
    if err != nil {
        return nil, fmt.Errorf("marshal: %w", err)
    }
    ts := time.Now().UnixMilli()
    nonce := generateNonce()
    msg := fmt.Sprintf("%s:%d:%s", string(raw), ts, nonce)
    mac := hmac.New(sha256.New, s.secret)
    mac.Write([]byte(msg))
    sig := hex.EncodeToString(mac.Sum(nil))
    return &SignedPayload{
        Data:      raw,
        Timestamp: ts,
        Nonce:     nonce,
        HMAC:      sig,
    }, nil
}

// Verify 验证签名。
func (s *IntegrityService) Verify(payload *SignedPayload) error {
    msg := fmt.Sprintf("%s:%d:%s", string(payload.Data), payload.Timestamp, payload.Nonce)
    mac := hmac.New(sha256.New, s.secret)
    mac.Write([]byte(msg))
    expected := hex.EncodeToString(mac.Sum(nil))
    if !hmac.Equal([]byte(expected), []byte(payload.HMAC)) {
        return fmt.Errorf("hmac mismatch")
    }
    // 检查时间窗口（防重放，允许 10 分钟内）
    age := time.Now().UnixMilli() - payload.Timestamp
    if age > 10*60*1000 || age < -60*1000 {
        return fmt.Errorf("timestamp out of window")
    }
    return nil
}
```

#### 3.1.3 推理请求 ID 生命周期管理

```go
// backend/internal/repo/inference.go

// InferenceSession 推理会话（内存或 Redis 存储）
type InferenceSession struct {
    RequestID    string
    DeviceID     string
    CreatedAt    time.Time
    ExpiresAt    time.Time
    DetectResult *DetectResult     // 检测结果
    AnalysisResult *AnalysisResult  // 分析结果
    ValueResult  *ValueResult      // 数值结果
    Status       string            // "detected" -> "analyzed" -> "valued" -> "synced"
}
```

**生命周期**：
1. `/vision/detect` 成功 → 创建会话，`status=detected`
2. `/vision/analyze` 成功 → 更新会话，`status=analyzed`
3. `/value/generate` 成功 → 更新会话，`status=valued`
4. `/sync/animal` 成功 → 标记会话 `status=synced`，防止复用
5. 会话 10 分钟未完成自动过期清理

### 3.2 同步数据校验增强

在 `sync.go` 的 `SyncAnimal` 中增加校验：

```go
// 校验链路（新增）
func (h *SyncHandler) SyncAnimal(c *gin.Context) {
    // ... 现有绑定逻辑 ...

    // 新增：校验推理请求 ID 链路
    if req.InferenceRequestID != "" {
        session, err := h.inferenceRepo.Get(req.InferenceRequestID)
        if err != nil || session == nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid inference_request_id"})
            return
        }
        // 校验设备归属
        if session.DeviceID != deviceID {
            c.JSON(http.StatusForbidden, gin.H{"error": "inference_request_id device mismatch"})
            return
        }
        // 校验会话状态
        if session.Status != "valued" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "inference session not completed"})
            return
        }
        // 校验数值与推理结果一致
        if req.Rarity != session.ValueResult.Rarity {
            c.JSON(http.StatusBadRequest, gin.H{"error": "rarity mismatch with inference result"})
            return
        }
        // 标记会话已使用
        h.inferenceRepo.MarkSynced(req.InferenceRequestID)
    }

    // 新增：位置校验
    if req.Latitude != 0 && req.Longitude != 0 {
        // IP 地理位置交叉验证
        clientIP := c.ClientIP()
        ipLocation := h.geoIPService.Lookup(clientIP)
        if ipLocation != nil {
            distance := haversine(req.Latitude, req.Longitude, ipLocation.Lat, ipLocation.Lng)
            if distance > 500_000 { // 500km
                h.auditService.SaveAlert(deviceID, "anomaly",
                    fmt.Sprintf("GPS/IP location mismatch: GPS=(%f,%f), IP=(%f,%f), distance=%.0fkm",
                        req.Latitude, req.Longitude, ipLocation.Lat, ipLocation.Lng, distance/1000),
                    req.InferenceRequestID, "")
            }
        }
    }

    // 新增：时间校验
    if time.Since(generatedAt) > 10*time.Minute {
        c.JSON(http.StatusBadRequest, gin.H{"error": "generated_at too old"})
        return
    }
    if generatedAt.After(time.Now().Add(1*time.Minute)) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "generated_at in future"})
        return
    }

    // ... 现有去重 + 审计 + 落库 ...
}
```

### 3.3 数值范围校验

```go
// 校验所有数值在合法范围内
func validateAnimalStats(req *syncRequest) error {
    if req.Rarity < 1 || req.Rarity > 5 {
        return fmt.Errorf("rarity out of range: %d", req.Rarity)
    }
    if req.HP < 50 || req.HP > 500 {
        return fmt.Errorf("HP out of range: %d", req.HP)
    }
    if req.ATK < 10 || req.ATK > 120 {
        return fmt.Errorf("ATK out of range: %d", req.ATK)
    }
    if req.DEF < 5 || req.DEF > 80 {
        return fmt.Errorf("DEF out of range: %d", req.DEF)
    }
    if req.SPD < 1 || req.SPD > 100 {
        return fmt.Errorf("SPD out of range: %d", req.SPD)
    }
    validClasses := map[string]bool{"tank": true, "dps": true, "support": true, "control": true}
    if !validClasses[req.Class] {
        return fmt.Errorf("invalid class: %s", req.Class)
    }
    validElements := map[string]bool{"fire": true, "water": true, "grass": true, "light": true, "dark": true}
    if !validElements[req.Element] {
        return fmt.Errorf("invalid element: %s", req.Element)
    }
    return nil
}
```

### 3.4 捕获投掷轨迹校验

设计文档 3.3："投掷轨迹需符合物理参数范围，异常抛物线（如瞬时位移）标记为可疑。"

```go
// backend/internal/services/trajectory.go

// TrajectoryPoint 投掷轨迹点
type TrajectoryPoint struct {
    T int     `json:"t"`     // 相对开始时间 ms
    X float64 `json:"x"`     // 归一化 x 坐标 0~1
    Y float64 `json:"y"`     // 归一化 y 坐标 0~1
}

// ValidateTrajectory 校验投掷轨迹合理性。
func ValidateTrajectory(points []TrajectoryPoint) error {
    if len(points) < 5 {
        return fmt.Errorf("trajectory too short: %d points", len(points))
    }

    // 1. 时间连续性：相邻点间隔 16~50ms (60fps ~ 20fps)
    for i := 1; i < len(points); i++ {
        dt := points[i].T - points[i-1].T
        if dt < 8 || dt > 100 {
            return fmt.Errorf("time gap anomaly at point %d: dt=%dms", i, dt)
        }
    }

    // 2. 空间连续性：相邻点位移不超过最大物理速度
    for i := 1; i < len(points); i++ {
        dx := points[i].X - points[i-1].X
        dy := points[i].Y - points[i-1].Y
        dist := math.Sqrt(dx*dx + dy*dy)
        dt := float64(points[i].T-points[i-1].T) / 1000.0
        speed := dist / dt
        // 屏幕宽度归一化后，10/s 是合理上限（极快滑动）
        if speed > 10 {
            return fmt.Errorf("speed anomaly at point %d: speed=%.2f", i, speed)
        }
    }

    // 3. 抛物线拟合度：y 应先减小（上升）后增大（下降），近似抛物线
    // 简化检测：检查 Y 是否有先减后增趋势
    hasUpward := false
    hasDownward := false
    for i := 1; i < len(points); i++ {
        dy := points[i].Y - points[i-1].Y
        if dy < 0 { hasUpward = true }
        if dy > 0 { hasDownward = true }
    }
    if !hasUpward || !hasDownward {
        return fmt.Errorf("trajectory not parabolic")
    }

    return nil
}
```

### 3.5 异常行为审计增强

在现有 `AuditService.CheckAnomaly()` 基础上扩展：

| 规则 | 现有 | 新增 | 说明 |
|------|------|------|------|
| 高稀有度频次 | ✅ 10 分钟内 ≥3 只传说 | ✅ 扩展：1 小时内 ≥5 只史诗+ | 扩大时间窗口和稀有度范围 |
| 推理 ID 复用 | ✅ | ✅ | 保持 |
| 位置跳跃 | ❌ | **新增** | 同设备 30 分钟内移动 > 100km |
| 异常时段 | ❌ | **新增** | 凌晨 2~5 点高频捕获（bot 特征） |
| 捕获频率 | ❌ | **新增** | 同设备 1 分钟内 > 10 次捕获请求 |
| 设备指纹变更 | ❌ | **新增** | 同 device_id 的设备指纹突变 |
| 数值偏离 | ❌ | **新增** | 同步数值与推理会话记录不匹配 |

```go
// 扩展 CheckAnomaly
func (s *AuditService) CheckAnomaly(deviceID string, animal *models.Animal) []string {
    var alerts []string

    // === 现有规则 ===
    // 规则 1: 高稀有度频次（保持）
    // 规则 2: 推理 ID 复用（保持）

    // === 新增规则 ===
    // 规则 3: 位置跳跃
    lastAnimal, _ := s.animalRepo.FindLastByDeviceID(deviceID)
    if lastAnimal != nil {
        distance := haversine(animal.Latitude, animal.Longitude,
            lastAnimal.Latitude, lastAnimal.Longitude)
        timeDiff := time.Since(lastAnimal.GeneratedAt)
        if distance > 100_000 && timeDiff < 30*time.Minute {
            msg := fmt.Sprintf("位置跳跃异常: %.0fkm in %v", distance/1000, timeDiff)
            alerts = append(alerts, msg)
            s.saveAlert(deviceID, "anomaly", msg, animal.InferenceRequestID, "")
        }
    }

    // 规则 4: 捕获频率异常
    recentCount, _ := s.animalRepo.CountRecentByDevice(deviceID, time.Now().Add(-1*time.Minute))
    if recentCount >= 10 {
        msg := fmt.Sprintf("捕获频率异常: 1 分钟内 %d 次同步", recentCount)
        alerts = append(alerts, msg)
        s.saveAlert(deviceID, "anomaly", msg, animal.InferenceRequestID, "")
    }

    // 规则 5: 异常时段高频
    hour := time.Now().Hour()
    if hour >= 2 && hour <= 5 {
        count, _ := s.animalRepo.CountRecentByDevice(deviceID, time.Now().Add(-1*time.Hour))
        if count >= 20 {
            msg := fmt.Sprintf("异常时段高频: 凌晨 2~5 点 1 小时内 %d 次", count)
            alerts = append(alerts, msg)
            s.saveAlert(deviceID, "anomaly", msg, animal.InferenceRequestID, "")
        }
    }

    return alerts
}
```

### 3.6 防重放机制

```go
// backend/internal/middleware/replay.go

// nonce 缓存（内存，生产环境用 Redis）
var nonceCache = sync.Map{}

// AntiReplay 防重放中间件，校验请求中的 X-Request-Nonce 和 X-Request-Timestamp。
func AntiReplay() gin.HandlerFunc {
    return func(c *gin.Context) {
        nonce := c.GetHeader("X-Request-Nonce")
        tsStr := c.GetHeader("X-Request-Timestamp")
        if nonce == "" || tsStr == "" {
            c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing nonce or timestamp"})
            return
        }

        ts, err := strconv.ParseInt(tsStr, 10, 64)
        if err != nil {
            c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid timestamp"})
            return
        }

        // 时间窗口校验（5 分钟内）
        now := time.Now().Unix()
        if math.Abs(float64(now-ts)) > 300 {
            c.AboutWithStatusJSON(http.StatusRequestTimeout, gin.H{"error": "request expired"})
            return
        }

        // nonce 唯一性校验
        if _, loaded := nonceCache.LoadOrStore(nonce, time.Now().Unix()); loaded {
            c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "nonce already used"})
            return
        }

        c.Next()
    }
}
```

### 3.7 健康检查端点注入服务端时间

```go
// backend/internal/handlers/health.go

func Health() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Server-Time", strconv.FormatInt(time.Now().UnixMilli(), 10))
        c.JSON(200, gin.H{"status": "ok"})
    }
}
```

所有响应中间件注入 `X-Server-Time`：

```go
// backend/internal/middleware/servertime.go
func ServerTime() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Server-Time", strconv.FormatInt(time.Now().UnixMilli(), 10))
        c.Next()
    }
}
```

---

## 4. 照片真实性验证（Photo Authenticity Verification）

### 4.1 多维验证体系

```
┌──────────────────────────────────────────────────┐
│              照片真实性验证流水线                   │
├──────────────────────────────────────────────────┤
│                                                  │
│  客户端采集层                                     │
│  ├─ MediaStream.track.label（摄像头标识）          │
│  ├─ track.readyState（必须 "live"）               │
│  ├─ 帧时间戳序列（performance.now() 连续性）       │
│  ├─ DeviceMotion 传感器样本（与帧时间对齐）         │
│  ├─ Canvas 生成无 EXIF（反证：有 EXIF = 相册导入） │
│  └─ 拍照会话 nonce（服务端下发，绑定会话）          │
│                                                  │
│  服务端校验层                                     │
│  ├─ nonce 有效性校验（存在、未过期、未使用）         │
│  ├─ 帧时间戳连续性（间隔 ∈ [150ms, 600ms]）        │
│  ├─ 传感器方差校验（> 阈值，排除静态翻拍）           │
│  ├─ 图片 EXIF 检测（有 EXIF = 相册导入）           │
│  ├─ 图片 perceptual hash 去重（防止重复使用同一图） │
│  ├─ VLM 翻拍检测（Prompt 中加入翻拍判断指令）      │
│  └─ 检测框运动轨迹合理性（多帧检测框位移一致性）    │
│                                                  │
└──────────────────────────────────────────────────┘
```

### 4.2 VLM 翻拍检测 Prompt 增强

在 `backend/internal/ai/prompts/detect.go` 中增强 Prompt：

```
请分析这张图片是否为：
1. 直接用相机拍摄的实时画面
2. 对屏幕/打印照片的翻拍（特征：屏幕像素网格、反光、边框、摩尔纹）
3. 静态照片导入（特征：无实时取景特征、可能含 EXIF）

如果是翻拍或导入，请在返回结果中标记 screen_capture=true。
```

### 4.3 图片 Perceptual Hash 去重

```go
// backend/internal/services/imagehash.go

// 计算图片感知哈希（pHash），用于检测同一张照片被重复使用
func ComputePHash(imageData []byte) (string, error) {
    // 1. 解码图片
    // 2. 缩放至 32x32 灰度
    // 3. DCT 变换
    // 4. 取左上 8x8 的 DCT 系数
    // 5. 计算均值，生成 64bit 哈希
    // ...
}

// 汉明距离，判断两张图是否相似
func HammingDistance(hash1, hash2 string) int {
    // ...
}
```

服务端维护最近 N 天的图片 pHash 库，新上传图片与库中哈希比对，汉明距离 < 5 判定为相似图片（可能是同一张照片重复使用）。

### 4.4 客户端拍照会话 Nonce 机制

```typescript
// frontend/src/services/antiCheat/session.ts

let sessionNonce: string | null = null
let sessionNonceExpires = 0

/** 从服务端获取拍照会话 nonce */
export async function ensureSessionNonce(): Promise<string> {
  if (sessionNonce && Date.now() < sessionNonceExpires) {
    return sessionNonce
  }
  const resp = await fetch('/api/v1/anticheat/session', {
    method: 'POST',
    headers: { Authorization: `Bearer ${getToken()}` },
  })
  const data = await resp.json()
  sessionNonce = data.nonce
  sessionNonceExpires = Date.now() + 5 * 60 * 1000  // 5 分钟有效
  return sessionNonce
}
```

**服务端端点**：

```go
// POST /api/v1/anticheat/session
// 返回: { "nonce": "<uuid>", "expires_at": "<rfc3339>" }
// 会话 nonce 绑定 device_id，10 分钟过期，仅可使用一次
```

---

## 5. 模拟器/Root 检测方案（汇总）

### 5.1 检测矩阵

| 平台 | 检测层 | 检测项 | 可靠性 | 备注 |
|------|--------|--------|--------|------|
| Web (PWA) | 浏览器 API | User-Agent | 🟡 中 | 可伪造 |
| Web (PWA) | 浏览器 API | WebGL GPU 渲染器 | 🟢 高 | 难伪造 |
| Web (PWA) | 浏览器 API | 硬件并发/内存/触摸 | 🟡 中 | 可伪造 |
| Web (PWA) | 浏览器 API | 传感器存在性 | 🟢 高 | 模拟器难模拟真实传感器 |
| Web (PWA) | 浏览器 API | navigator.webdriver | 🟢 高 | 自动化工具标志 |
| Capacitor | 原生 Android | Build.FINGERPRINT/MODEL | 🟢 高 | 原生 API，难伪造 |
| Capacitor | 原生 Android | su 二进制/Magisk 文件 | 🟢 高 | 文件系统检查 |
| Capacitor | 原生 Android | Xposed/Frida 检测 | 🟡 中 | 高级 hook 可绕过 |
| Capacitor | 原生 iOS | Cydia/Sileo 文件 | 🟢 高 | 文件系统检查 |
| Capacitor | 原生 iOS | fork()/bash 检测 | 🟢 高 | 沙盒内不可执行 |

### 5.2 风险评分模型

综合所有检测信号计算 `riskScore`（0~100）：

```
riskScore = Σ(signal.weight for signal in signals if signal.suspicious)

权重分配：
  GPU 渲染器异常     25
  User-Agent 异常    20
  su/root 文件存在   30（原生层）
  Xposed/Frida 检出  25（原生层）
  传感器缺失         15
  CPU 核心 ≤ 2       10
  内存 ≤ 1GB         10
  触摸点 = 0         10
  webdriver=true     20
  分辨率极低          5

riskScore ≥ 50 → isEmulator/isRooted = true
```

### 5.3 检测结果上报

客户端将检测结果打包上报服务端，服务端结合自身检测做最终判定：

```typescript
// frontend/src/services/antiCheat/report.ts

export interface DeviceSecurityReport {
  emulatorCheck: EmulatorCheckResult
  rootCheck: RootCheckResult
  locationProof: LocationProof
  timeSync: TimeSyncResult
  appIntegrity: AppIntegrityResult
  collectedAt: number
  hmac: string  // 服务端下发 nonce 的 HMAC
}

export async function collectSecurityReport(): Promise<DeviceSecurityReport> {
  const [emulatorCheck, rootCheck, timeSync] = await Promise.all([
    Promise.resolve(detectEmulator()),
    checkRootStatus(),
    checkTimeSync(),
  ])

  return {
    emulatorCheck,
    rootCheck,
    locationProof: {} as LocationProof, // 由 LbsContext 填充
    timeSync,
    appIntegrity: checkAppIntegrity(),
    collectedAt: Date.now(),
    hmac: '',  // 由服务端 session nonce 计算
  }
}
```

### 5.4 服务端判定策略

```go
// backend/internal/services/security.go

type DeviceRiskLevel int

const (
    RiskLow      DeviceRiskLevel = iota  // 0~29
    RiskMedium                            // 30~49
    RiskHigh                              // 50~79
    RiskCritical                          // 80~100
)

func (s *SecurityService) EvaluateRisk(report *DeviceSecurityReport) DeviceRiskLevel {
    score := 0
    if report.EmulatorCheck.IsEmulator { score += report.EmulatorCheck.RiskScore }
    if report.RootCheck.IsRooted { score += report.RootCheck.RiskScore }
    if report.TimeSync.IsManipulated { score += 20 }
    if report.LocationProof.MockDetected { score += 30 }

    switch {
    case score >= 80: return RiskCritical
    case score >= 50: return RiskHigh
    case score >= 30: return RiskMedium
    default: return RiskLow
    }
}
```

| 风险等级 | 服务端响应 |
|---------|-----------|
| Low | 正常处理 |
| Medium | 正常处理，设备标记为"关注"，审计日志加粗 |
| High | 允许请求但延迟发放奖励（标记 pending_review），人工审核 |
| Critical | 拒绝发现/捕获/同步请求，返回 403 + "设备环境不安全"提示 |

---

## 6. 位置欺骗检测（详细）

### 6.1 客户端检测

| 检测项 | 实现 | 已在 2.4 节详述 |
|--------|------|----------------|
| 精度异常 | `accuracy < 3m` | ✅ |
| 位置跳跃 | 两次定位距离/时间比超物理极限 | ✅ |
| timestamp 偏差 | `position.timestamp` vs `Date.now()` | ✅ |

### 6.2 服务端交叉验证

```go
// backend/internal/services/geoVerify.go

type GeoVerifyService struct {
    geoIPService *GeoIPService
    animalRepo   *repo.AnimalRepo
}

func (s *GeoVerifyService) VerifyLocation(
    deviceID string,
    lat, lng float64,
    clientIP string,
) []string {
    var alerts []string

    // 1. IP 地理位置交叉验证
    ipLoc := s.geoIPService.Lookup(clientIP)
    if ipLoc != nil {
        distance := haversine(lat, lng, ipLoc.Lat, ipLoc.Lng)
        if distance > 500_000 { // 500km
            alerts = append(alerts, fmt.Sprintf("GPS/IP mismatch: %.0fkm", distance/1000))
        }
    }

    // 2. 历史轨迹连续性
    lastAnimal, _ := s.animalRepo.FindLastByDeviceID(deviceID)
    if lastAnimal != nil {
        distance := haversine(lat, lng, lastAnimal.Latitude, lastAnimal.Longitude)
        timeDiff := time.Since(lastAnimal.GeneratedAt)
        speed := distance / timeDiff.Seconds()
        if speed > 150 { // 150 m/s ≈ 540 km/h，超过高铁
            alerts = append(alerts, fmt.Sprintf("speed anomaly: %.1f m/s", speed))
        }
    }

    // 3. 坐标合理性（非 0,0 / 非明显测试坐标）
    if (lat == 0 && lng == 0) ||
       (lat == 37.7749 && lng == -122.4194) || // San Francisco 测试坐标
       (lat == 39.9042 && lng == 116.4074 && clientIP != "China") {
        alerts = append(alerts, "suspicious coordinates")
    }

    return alerts
}
```

### 6.3 区域排行防刷

设计文档 6.3："每日每省捕获数去重（同设备同日只计 1 次有效）" + "异常增长告警"

```go
// backend/internal/services/rankVerify.go

// 检测区域排行刷量
func (s *RankService) CheckRankFraud(deviceID string, province string) bool {
    // 同设备当日捕获数限制
    todayCount, _ := s.animalRepo.CountTodayByDevice(deviceID)
    if todayCount > 50 { // 单日 50 只以上判定为刷量
        return true
    }

    // 同设备当日不同省份切换次数（正常一天不会跨多个省）
    provinces, _ := s.animalRepo.CountTodayProvinces(deviceID)
    if provinces > 3 {
        return true
    }

    return false
}
```

---

## 7. 限流与异常检测（Rate Limiting & Anomaly Detection）

### 7.1 现有限流（已有）

| 层级 | 实现 | 配置 |
|------|------|------|
| IP 限流 | `RateLimitByIP` | 100 req/min, burst 10 |
| 设备限流 | `RateLimitByDevice` | 100 req/min, burst 10 |
| 每日检测上限 | `CostLimitByType("detect")` | 100 次/天 |
| 每日分析上限 | `CostLimitByType("analyze")` | 50 次/天 |
| 每日生成上限 | `CostLimitByType("value")` | 20 次/天 |

### 7.2 新增限流规则

| 规则 | 实现 | 说明 |
|------|------|------|
| 捕获频率限流 | 设备级：1 次/10 秒，10 次/小时 | 防止高频刷捕获 |
| 同步频率限流 | 设备级：1 次/5 秒 | 防止高频同步 |
| 地理端点限流 | IP+GPS 区域：同一 IP 短时间多 GPS 坐标 | 防止 VPN 刷区域 |
| 非法请求限流 | 累计 10 次校验失败 → 封禁 1 小时 | 防止暴力探测 |

### 7.3 异常检测规则汇总

| 规则 | 触发条件 | 响应 | 现有/新增 |
|------|---------|------|----------|
| 高稀有度频次 | 10 分钟内 ≥3 只传说 | 告警 | ✅ 现有 |
| 推理 ID 复用 | 同一 ID 多只动物 | 告警 | ✅ 现有 |
| 位置跳跃 | 30 分钟内 > 100km | 告警 | 新增 |
| 捕获频率 | 1 分钟内 > 10 次 | 告警 | 新增 |
| 异常时段 | 凌晨 2~5 点 1 小时 > 20 次 | 告警 | 新增 |
| 数值偏离 | 同步数值 ≠ 推理结果 | 拒绝 | 新增 |
| 签名失败 | HMAC 校验失败 | 拒绝 + 计数 | 新增 |
| nonce 重放 | nonce 已使用 | 拒绝 + 计数 | 新增 |
| 图片重复 | pHash 汉明距离 < 5 | 告警 | 新增 |
| 设备指纹突变 | 同 device_id 指纹变化 | 告警 | 新增 |

### 7.4 异常处置策略

```
告警计数器（按设备）
  ├─ 1~2 次告警 → 仅记录日志，不阻断
  ├─ 3~5 次告警 → 标记设备为"高风险"，后续请求强制二次校验
  ├─ 6~10 次告警 → 临时封禁 1 小时，返回 403
  └─ > 10 次告警 → 永久封禁设备，降级为"仅浏览"模式
```

---

## 8. 数据完整性（Hashing & Signatures）

### 8.1 签名链路总览

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│  Detect  │────▶│ Analyze  │────▶│  Value   │────▶│   Sync   │
│          │     │          │     │          │     │          │
│ 返回:     │     │ 返回:     │     │ 返回:     │     │ 校验:     │
│ result   │     │ result   │     │ result   │     │ HMAC 链   │
│ req_id   │     │ req_id   │     │ req_id   │     │ 数值一致  │
│ HMAC     │     │ HMAC     │     │ HMAC     │     │ nonce 未用│
│ nonce    │     │ nonce    │     │ nonce    │     │ 时间窗口  │
└──────────┘     └──────────┘     └──────────┘     └──────────┘
```

### 8.2 设备指纹（Device Fingerprint）

```typescript
// frontend/src/services/antiCheat/fingerprint.ts

export interface DeviceFingerprint {
  /** User-Agent 哈希 */
  uaHash: string
  /** 屏幕分辨率 + pixel depth */
  screenHash: string
  /** GPU 渲染器哈希 */
  gpuHash: string
  /** 字体列表哈希 */
  fontHash: string
  /** 时区 + 语言 */
  localeHash: string
  /** 硬件特征（CPU/内存/触摸） */
  hardwareHash: string
  /** 综合指纹 */
  fingerprint: string
}

export function collectFingerprint(): DeviceFingerprint {
  // 各维度采集后 SHA-256 哈希
  // 综合指纹 = SHA-256(uaHash + screenHash + gpuHash + fontHash + localeHash + hardwareHash)
  // ...
}
```

**用途**：
- 同 device_id 的指纹突变 → 可能 token 泄露被冒用
- 不同 device_id 的指纹相同 → 可能是模拟器多开
- 区域排行去重辅助（设计文档 6.3："设备指纹去重"）

### 8.3 请求签名

每个关键请求（detect/analyze/value/sync）必须携带：

| Header | 说明 |
|--------|------|
| `Authorization: Bearer <jwt>` | 设备鉴权（现有） |
| `X-Request-Nonce` | 请求唯一标识（UUID），防重放 |
| `X-Request-Timestamp` | 客户端时间戳（unix 秒） |
| `X-Device-Fingerprint` | 设备指纹哈希 |
| `X-Security-Report` | Base64 编码的安全报告（可选，定期上报） |

### 8.4 客户端代码混淆与加固

| 加固项 | 工具/方案 | 优先级 |
|--------|----------|--------|
| 代码压缩 | Vite build 默认 esbuild minify | ✅ 现有 |
| 变量名混淆 | `vite-plugin-obfuscator`（javascript-obfuscator） | P0 |
| 字符串加密 | javascript-obfuscator stringArray | P0 |
| 控制流平坦化 | javascript-obfuscator controlFlowFlattening | P1 |
| 反调试 | `debugger` 语句 + 时间检测 | P1 |
| Source Map | 生产环境关闭 (`sourcemap: false`) | P0 |
| 完整性校验 | SRI (Subresource Integrity) hash | P1 |

**Vite 配置示例**：

```typescript
// vite.config.ts
import obfuscator from 'vite-plugin-obfuscator'

export default defineConfig({
  build: {
    sourcemap: false, // 生产环境关闭
    plugins: [
      obfuscator({
        options: {
          compact: true,
          controlFlowFlattening: true,
          controlFlowFlatteningThreshold: 0.5,
          stringArray: true,
          stringArrayEncoding: ['base64'],
          stringArrayThreshold: 0.75,
          transformObjectKeys: true,
          unicodeEscapeSequence: false,
        },
      }),
    ],
  },
})
```

> **注意**：混淆会增加包体大小和运行时性能开销。需在性能（Issue #43）与防作弊之间平衡。建议仅混淆 `antiCheat` 模块和 API 通信模块，不混淆 UI 组件。

---

## 9. 测试用例（Test Cases）

### 9.1 客户端测试

#### 9.1.1 相机验证测试

| 用例 ID | 描述 | 前置条件 | 预期结果 |
|---------|------|---------|---------|
| AC-CAM-01 | 正常实时拍照 | getUserMedia 成功 | CameraProof 包含 track.label、live state、帧时间戳序列 |
| AC-CAM-02 | 相册导入检测 | 通过 input file 导入 | 服务端检测到 EXIF → 拒绝 |
| AC-CAM-03 | 虚拟摄像头检测 | OBS Virtual Camera | track.label 含 "virtual" → 标记可疑 |
| AC-CAM-04 | 帧时间戳连续性 | 正常 2~5 FPS 采样 | 间隔 ∈ [150ms, 600ms] |
| AC-CAM-05 | 传感器数据采集 | 手持设备拍摄 | 加速度方差 > 阈值 |
| AC-CAM-06 | 传感器静止检测 | 手机固定拍摄 | 加速度方差 < 阈值 → 标记可疑（非阻断） |
| AC-CAM-07 | 桌面浏览器降级 | 无 DeviceMotionEvent | 降级为仅时间戳校验，不阻断 |

#### 9.2.2 模拟器检测测试

| 用例 ID | 描述 | 前置条件 | 预期结果 |
|---------|------|---------|---------|
| AC-EMU-01 | Chrome DevTools 模拟 | 设备工具栏模拟手机 | riskScore ≥ 30（UA + 触摸） |
| AC-EMU-02 | Android AVD 模拟器 | AVD 运行 PWA | riskScore ≥ 50（GPU + UA + 硬件） |
| AC-EMU-03 | Genymotion 模拟器 | Genymotion 运行 | riskScore ≥ 70 |
| AC-EMU-04 | 真机检测 | iPhone / Android 真机 | riskScore < 30 |
| AC-EMU-05 | 传感器检测 | 模拟器无传感器 | 信号标记 suspicious |

#### 9.1.3 Root 检测测试

| 用例 ID | 描述 | 前置条件 | 预期结果 |
|---------|------|---------|---------|
| AC-ROOT-01 | 真机未 root | 正常真机 | isRooted=false |
| AC-ROOT-02 | Magisk root | Magisk 已 root | isRooted=true（原生层） |
| AC-ROOT-03 | Xposed 注入 | Xposed 已安装 | isRooted=true（原生层） |
| AC-ROOT-04 | 越狱 iOS | Cydia 已安装 | isRooted=true（原生层） |
| AC-ROOT-05 | webdriver 检测 | Puppeteer 驱动 | navigator.webdriver=true → 标记 |

#### 9.1.4 位置欺骗检测测试

| 用例 ID | 描述 | 前置条件 | 预期结果 |
|---------|------|---------|---------|
| AC-LOC-01 | 正常 GPS 定位 | 真机室外 | mockDetected=false |
| AC-LOC-02 | Mock GPS 应用 | Android 开发者选项模拟位置 | accuracy 异常 / timestamp 偏差 |
| AC-LOC-03 | 位置跳跃 | 30 秒内坐标变化 100km | speed_anomaly 信号 |
| AC-LOC-04 | IP/GPS 交叉验证 | VPN 改 IP 但 GPS 不变 | 服务端告警 GPS/IP mismatch |

#### 9.1.5 时间篡改检测

| 用例 ID | 描述 | 前置条件 | 预期结果 |
|---------|------|---------|---------|
| AC-TIME-01 | 正常时间同步 | 设备时间准确 | offset < 5min |
| AC-TIME-02 | 时间超前 | 设备时间快 1 小时 | isManipulated=true |
| AC-TIME-03 | 时间滞后 | 设备时间慢 1 小时 | isManipulated=true |
| AC-TIME-04 | 体力恢复防篡改 | 篡改设备时间 | 服务端基于自身时间计算，无效 |

### 9.2 服务端测试

#### 9.2.1 数据完整性测试

| 用例 ID | 描述 | 前置条件 | 预期结果 |
|---------|------|---------|---------|
| SV-INT-01 | 正常同步 | 合法推理链路 | 201 Created |
| SV-INT-02 | 篡改稀有度 | rarity ≠ 推理会话记录 | 400 Bad Request |
| SV-INT-03 | 篡改属性 | HP > 500 | 400 Bad Request |
| SV-INT-04 | 无效推理 ID | inference_request_id 不存在 | 400 Bad Request |
| SV-INT-05 | 推理 ID 设备不匹配 | A 设备的 ID 给 B 设备用 | 403 Forbidden |
| SV-INT-06 | 推理 ID 复用 | 已 synced 的 ID 再次使用 | 400 Bad Request |
| SV-INT-07 | 推理会话过期 | 创建后 > 10 分钟才同步 | 400 Bad Request |
| SV-INT-08 | 重放请求 | 相同 nonce 二次请求 | 409 Conflict |

#### 9.2.2 异常检测测试

| 用例 ID | 描述 | 前置条件 | 预期结果 |
|---------|------|---------|---------|
| SV-ANO-01 | 高稀有度频次 | 10 分钟内 3 只传说 | 告警记录 |
| SV-ANO-02 | 位置跳跃 | 30 分钟内 100km | 告警记录 |
| SV-ANO-03 | 高频捕获 | 1 分钟 10 次 | 告警记录 |
| SV-ANO-04 | 凌晨高频 | 2~5 点 1 小时 20 次 | 告警记录 |
| SV-ANO-05 | 推理 ID 复用 | 同 ID 两只动物 | 告警记录 |

#### 9.2.3 限流测试

| 用例 ID | 描述 | 前置条件 | 预期结果 |
|---------|------|---------|---------|
| SV-RL-01 | IP 限流 | 100 req/min 超限 | 429 Too Many Requests |
| SV-RL-02 | 每日检测上限 | > 100 次 detect | 429 |
| SV-RL-03 | 每日生成上限 | > 20 次 value | 429 |
| SV-RL-04 | 捕获频率限流 | 10 秒内 2 次同步 | 429 |

#### 9.2.4 轨迹校验测试

| 用例 ID | 描述 | 前置条件 | 预期结果 |
|---------|------|---------|---------|
| SV-TRJ-01 | 正常抛物线 | 合理投掷轨迹 | 通过 |
| SV-TRJ-02 | 轨迹过短 | < 5 个点 | 拒绝 |
| SV-TRJ-03 | 瞬时位移 | 速度 > 10/s | 拒绝 |
| SV-TRJ-04 | 非抛物线 | 单调递增/递减 | 拒绝 |
| SV-TRJ-05 | 时间间隔异常 | dt > 100ms | 拒绝 |

### 9.3 集成测试

| 用例 ID | 描述 | 测试步骤 | 预期结果 |
|---------|------|---------|---------|
| INT-01 | 正常捕获全流程 | 发现→拍照→检测→分析→生成→同步 | 201 Created，无告警 |
| INT-02 | 相册导入全流程 | 用相册照片走捕获 | 服务端拒绝（EXIF / 传感器 / pHash） |
| INT-03 | 模拟器捕获 | 模拟器中走全流程 | 风险分 ≥ 50，降级或告警 |
| INT-04 | 篡改数值同步 | 正常流程后改 rarity 同步 | 服务端拒绝（数值不一致） |
| INT-05 | 重放同步请求 | 复制合法请求二次发送 | 服务端拒绝（nonce 重复） |

---

## 10. 实施步骤（Implementation Steps）

### 阶段一：客户端检测层（前端）

| 步骤 | 文件 | 内容 | 依赖 |
|------|------|------|------|
| 1.1 | `frontend/src/services/antiCheat/index.ts` | 创建 antiCheat 模块入口 | - |
| 1.2 | `frontend/src/services/antiCheat/cameraVerify.ts` | 相机证明采集（track label/settings/state + 帧时间戳） | - |
| 1.3 | `frontend/src/services/antiCheat/sensorCollect.ts` | DeviceMotion 传感器采集 | - |
| 1.4 | `frontend/src/services/antiCheat/emulatorDetect.ts` | 模拟器检测（UA/GPU/硬件/触摸） | - |
| 1.5 | `frontend/src/services/antiCheat/rootDetect.ts` | Root/越狱检测（Web 端 + Capacitor 插件） | Capacitor 插件 |
| 1.6 | `frontend/src/services/antiCheat/locationVerify.ts` | 位置欺骗检测（精度/跳跃/timestamp） | - |
| 1.7 | `frontend/src/services/antiCheat/timeVerify.ts` | 时间同步检测 | 后端 X-Server-Time |
| 1.8 | `frontend/src/services/antiCheat/fingerprint.ts` | 设备指纹采集 | - |
| 1.9 | `frontend/src/services/antiCheat/session.ts` | 拍照会话 nonce 管理 | 后端 /anticheat/session |
| 1.10 | `frontend/src/services/antiCheat/report.ts` | 安全报告汇总与上报 | 以上全部 |
| 1.11 | `frontend/src/components/DiscoverScreen.tsx` | 集成 cameraVerify + sensorCollect 到拍照流程 | 1.2, 1.3 |
| 1.12 | `frontend/src/lbs/LbsContext.tsx` | 集成 locationVerify 到定位流程 | 1.6 |
| 1.13 | `frontend/src/services/apiClient.ts` | 创建统一 API 客户端，注入 nonce/timestamp/fingerprint headers | 1.8 |
| 1.14 | `frontend/src/services/visionDetect.ts` | 替换 mockVisionDetector，接入真实 API + 安全 headers | 1.13 |

### 阶段二：服务端验证层（后端）

| 步骤 | 文件 | 内容 | 依赖 |
|------|------|------|------|
| 2.1 | `backend/internal/services/integrity.go` | HMAC 签名服务 | - |
| 2.2 | `backend/internal/repo/inference.go` | 推理会话仓储（内存/Redis） | - |
| 2.3 | `backend/internal/services/security.go` | 设备安全评估服务 | - |
| 2.4 | `backend/internal/services/geoVerify.go` | 位置交叉验证（IP/历史轨迹） | GeoIP 库 |
| 2.5 | `backend/internal/services/trajectory.go` | 投掷轨迹校验 | - |
| 2.6 | `backend/internal/services/imagehash.go` | 图片 pHash 去重 | - |
| 2.7 | `backend/internal/middleware/servertime.go` | X-Server-Time 响应头中间件 | - |
| 2.8 | `backend/internal/middleware/replay.go` | 防重放中间件（nonce + timestamp） | - |
| 2.9 | `backend/internal/middleware/security.go` | 安全报告接收与风险判定中间件 | 2.3 |
| 2.10 | `backend/internal/handlers/anticheat.go` | 反作弊端点（session nonce 下发、安全报告接收） | 2.1, 2.3 |
| 2.11 | `backend/internal/handlers/vision.go` | 增强：创建推理会话、签名返回结果 | 2.1, 2.2 |
| 2.12 | `backend/internal/handlers/value.go` | 增强：校验推理链路、签名返回结果 | 2.1, 2.2 |
| 2.13 | `backend/internal/handlers/sync.go` | 增强：推理 ID 链路校验、数值一致性校验、位置交叉验证、时间窗口 | 2.1, 2.2, 2.4 |
| 2.14 | `backend/internal/handlers/health.go` | 注入 X-Server-Time | 2.7 |
| 2.15 | `backend/internal/services/audit.go` | 扩展异常检测规则（位置跳跃/频率/时段） | - |
| 2.16 | `backend/internal/repo/animal.go` | 新增 FindLastByDeviceID / CountRecentByDevice / CountTodayByDevice / CountTodayProvinces | - |
| 2.17 | `backend/internal/routes/router.go` | 注册新路由 + 中间件链 | 以上全部 |

### 阶段三：集成与加固

| 步骤 | 文件 | 内容 | 依赖 |
|------|------|------|------|
| 3.1 | `backend/internal/ai/prompts/detect.go` | VLM Prompt 增强（翻拍检测指令） | - |
| 3.2 | `frontend/vite.config.ts` | 配置 javascript-obfuscator | 阶段一完成 |
| 3.3 | `backend/internal/middleware/security.go` | 风险等级响应策略（Low/Medium/High/Critical） | 2.3, 2.9 |
| 3.4 | `backend/internal/config/config.go` | 新增 `ANTICHEAT_SECRET` 环境变量 | - |
| 3.5 | `frontend/src/services/antiCheat/types.ts` | 统一类型定义 | - |
| 3.6 | 前端测试文件 | 单元测试 + 集成测试 | 阶段一 |
| 3.7 | 后端测试文件 | 单元测试 + 集成测试 | 阶段二 |

### 阶段四：Capacitor 原生插件（可选，内测阶段）

| 步骤 | 内容 | 依赖 |
|------|------|------|
| 4.1 | 创建 `@animalpoke/anticheat` Capacitor 插件 | Capacitor 集成 |
| 4.2 | Android: root/su/Magisk/Xposed 检测 | 4.1 |
| 4.3 | iOS: Cydia/Sileo/fork 检测 | 4.1 |
| 4.4 | 前端 `rootDetect.ts` 接入原生插件 | 1.5, 4.1 |

### 实施顺序与依赖图

```
阶段一（前端检测层）
  1.1 ─┬─ 1.2 ── 1.11 (DiscoverScreen 集成)
       ├─ 1.3 ── 1.11
       ├─ 1.4 ── 1.10
       ├─ 1.5 ── 1.10
       ├─ 1.6 ── 1.12 (LbsContext 集成)
       ├─ 1.7 ←── 2.7 (依赖后端 X-Server-Time)
       ├─ 1.8 ── 1.13 (apiClient)
       ├─ 1.9 ←── 2.10 (依赖后端 session 端点)
       └─ 1.10 (汇总)

阶段二（后端验证层）
  2.1 ── 2.11, 2.12, 2.13
  2.2 ── 2.11, 2.12, 2.13
  2.3 ── 2.9, 2.10
  2.4 ── 2.13
  2.5 ── (独立)
  2.6 ── (独立)
  2.7 ── 2.14 (health)
  2.8 ── 2.17 (router)
  2.15 ── (扩展 audit)
  2.16 ── 2.15

阶段三（集成加固）
  3.1 ←── 阶段二
  3.2 ←── 阶段一
  3.3 ←── 2.3, 2.9
  3.4 ←── 2.1
```

---

## 11. 验收标准（Acceptance Criteria）

### 11.1 功能验收

| 编号 | 验收项 | 验收方法 | 通过标准 |
|------|--------|---------|---------|
| AC-01 | 相册导入检测 | 通过 input file 导入照片走捕获流程 | 服务端拒绝，返回"非实时拍摄"错误 |
| AC-02 | 虚拟摄像头检测 | OBS Virtual Camera 走捕获流程 | track.label 含 "virtual"，标记可疑 |
| AC-03 | 模拟器检测 | Android AVD / Genymotion 运行 | riskScore ≥ 50，降级或告警 |
| AC-04 | Root 检测 | Magisk root 的真机 | isRooted=true，降级为仅浏览 |
| AC-05 | 越狱检测 | 越狱 iOS 设备 | isRooted=true，降级为仅浏览 |
| AC-06 | Mock GPS 检测 | 开发者选项模拟位置 | mockDetected=true，服务端告警 |
| AC-07 | 时间篡改检测 | 设备时间偏移 1 小时 | isManipulated=true |
| AC-08 | 体力防篡改 | 篡改设备时间加速体力恢复 | 无效，服务端基于自身时间计算 |
| AC-09 | 数值篡改拦截 | 修改同步请求中的 rarity | 服务端返回 400 "rarity mismatch" |
| AC-10 | 推理 ID 链路 | 跳过 detect 直接调 sync | 服务端返回 400 "invalid inference_request_id" |
| AC-11 | 防重放 | 复用 nonce 二次请求 | 服务端返回 409 "nonce already used" |
| AC-12 | 异常告警 | 10 分钟内 3 只传说 | 审计日志记录告警 |
| AC-13 | 位置跳跃告警 | 30 分钟内 100km 移动 | 审计日志记录告警 |
| AC-14 | 代码混淆 | 检查生产构建产物 | 变量名混淆 + 字符串加密 + 无 sourcemap |
| AC-15 | 设备指纹 | 同 device_id 不同指纹 | 告警记录 |

### 11.2 性能验收

| 编号 | 验收项 | 通过标准 |
|------|--------|---------|
| AC-PERF-01 | 反作弊采集开销 | < 50ms（不阻塞拍照流程） |
| AC-PERF-02 | 服务端校验开销 | < 10ms per request（HMAC + nonce 检查） |
| AC-PERF-03 | 代码混淆包体影响 | 增量 < 30% |
| AC-PERF-04 | 帧率影响 | 拍照流程仍 ≥ 30fps |

### 11.3 安全验收

| 编号 | 验收项 | 通过标准 |
|------|--------|---------|
| AC-SEC-01 | 篡改同步数据 | 无法绕过服务端校验 |
| AC-SEC-02 | 重放攻击 | nonce 机制生效 |
| AC-SEC-03 | 伪造推理 ID | 服务端校验设备归属 + 状态 |
| AC-SEC-04 | 中间人篡改 | HTTPS + HMAC 签名双重保护 |
| AC-SEC-05 | 客户端反编译 | 代码混淆 + 无 sourcemap + 无敏感 key |

---

## 12. 风险与局限性（Risks & Limitations）

### 12.1 技术局限

| 局限性 | 说明 | 缓解措施 |
|--------|------|---------|
| Web 端反作弊天然薄弱 | PWA/Web 环境下客户端代码可被查看和修改，所有客户端检测理论上可绕过 | 核心校验全在服务端，客户端仅提高门槛 |
| 模拟器检测可绕过 | 高级模拟器可伪造 GPU/UA/传感器 | 多维交叉检测 + 服务端行为分析 |
| Root 检测可绕过 | Magisk Hide / Zygisk 可隐藏 root | Capacitor 原生层多层检测 + 服务端行为分析 |
| 传感器检测不通用 | 桌面浏览器无传感器，部分旧机型不支持 | 降级策略：无传感器时仅用时间戳校验 |
| pHash 误判 | 不同照片可能有相似 pHash | 汉明距离阈值可调，仅告警不阻断 |
| 代码混淆影响性能 | 控制流平坦化增加运行时开销 | 仅混淆安全模块，UI 组件不混淆 |
| GeoIP 不精确 | IP 地理位置数据库精度有限 | 仅作辅助告警，不单独阻断 |

### 12.2 误判风险

| 误判场景 | 影响 | 缓解措施 |
|---------|------|---------|
| 真机高精度 GPS | accuracy < 3m 被误判 | 仅告警不阻断，结合其他信号综合判定 |
| 高铁/飞机上游戏 | 位置跳跃被误判 | 速度阈值设为 150m/s（高于高铁），仅告警 |
| 翻译者/外籍用户 | IP 与 GPS 不匹配 | 阈值 500km，仅告警不阻断 |
| 旧设备性能低 | 传感器采样率低 | 降级为时间戳校验 |
| 凌晨出行的正常玩家 | 异常时段告警 | 仅告警不阻断，需结合频率规则 |

### 12.3 设计文档对齐

| 设计文档要求 | 本计划覆盖 | 状态 |
|-------------|-----------|------|
| 仅接受实时相机取景帧 | §2.1 相机验证 + §4 照片真实性 | ✅ |
| 禁止相册导入 | §2.1.1-2.1.4 + §4.1-4.3 | ✅ |
| 禁止模拟器 | §2.2 + §5 | ✅ |
| 禁止 root/越狱 | §2.3 + §5 | ✅ |
| 帧时间戳连续性校验 | §2.1.2 | ✅ |
| EXIF 与传感器交叉验证 | §2.1.3 + §2.1.4 + §4.1 | ✅ |
| 检测框运动轨迹合理性 | §3.4 投掷轨迹校验（注：检测框轨迹需 VLM 多帧检测支持，MVP 阶段单帧检测暂不适用，预留接口） | ⚠️ 预留 |
| 投掷轨迹物理参数校验 | §3.4 | ✅ |
| 推理请求 ID 全链路留存 | §3.1 推理 ID 生命周期 | ✅ |
| 服务端异常行为审计 | §3.5 扩展规则 | ✅ |
| 降级为"仅浏览"模式 | §2.3.3 + §5.4 | ✅ |
| 设备指纹去重（区域排行） | §8.2 | ✅ |
| 异常增长告警（区域排行） | §6.3 | ✅ |

### 12.4 后续演进方向

| 方向 | 说明 | 优先级 |
|------|------|--------|
| VLM 多帧检测框轨迹 | 设计文档提及"检测框运动轨迹合理性"，需 VLM 支持多帧检测，M2 预留接口 | P1（内测） |
| 行为分析 ML 模型 | 基于历史行为训练异常检测模型，替代硬编码规则 | P2（公测） |
| 设备风控评分系统 | 累积告警记录，动态调整设备信任分 | P2（公测） |
| 原生反调试 | Capacitor 原生层增加反调试/反注入 | P1（内测） |
| 服务端证书绑定 | 防止中间人代理抓包 | P2（公测） |
| 区块链存证 | 高稀有度动物捕获链路存链，防事后篡改 | P3（远期） |

---

## 附录 A：新增 API 端点

| 端点 | 方法 | 用途 | 鉴权 |
|------|------|------|------|
| `/api/v1/anticheat/session` | POST | 获取拍照会话 nonce | JWT |
| `/api/v1/anticheat/report` | POST | 上报设备安全报告 | JWT |
| `/api/v1/health` | GET | 健康检查 + 获取服务端时间 | 无 |

## 附录 B：新增数据库表

### `inference_sessions`（推理会话表，可用 Redis 替代）

| 字段 | 类型 | 说明 |
|------|------|------|
| request_id | varchar(128) PK | 推理请求 ID (UUID) |
| device_id | varchar(64) | 设备 ID |
| status | varchar(32) | detected / analyzed / valued / synced / expired |
| detect_result | JSON | 检测结果 |
| analysis_result | JSON | 分析结果 |
| value_result | JSON | 数值结果 |
| created_at | datetime | 创建时间 |
| expires_at | datetime | 过期时间 |

### `device_fingerprints`（设备指纹表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint PK | 自增主键 |
| device_id | varchar(64) | 设备 ID |
| fingerprint | varchar(64) | 指纹哈希 |
| risk_score | int | 风险分 |
| risk_level | varchar(16) | low / medium / high / critical |
| alert_count | int | 累计告警次数 |
| created_at | datetime | 首次记录 |
| updated_at | datetime | 最近更新 |

## 附录 C：新增环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `ANTICHEAT_SECRET` | HMAC 签名密钥 | （必须配置） |
| `INFERENCE_SESSION_TTL` | 推理会话过期时间 | `10m` |
| `NONCE_CACHE_TTL` | nonce 缓存时间 | `5m` |
| `GEOIP_DB_PATH` | GeoIP 数据库路径 | `/etc/animalpoke/GeoLite2-City.mmdb` |
| `OBFUSCATE_BUILD` | 是否启用代码混淆 | `true` |
