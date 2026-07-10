/**
 * 扫描成本与额度模型（#183 / AP-022）
 * MVP：手动扫描（非连续 FPS 上传）
 */
const KEY = 'animal-poke-scan-budget'
const DAY_MS = 86_400_000

export type ScanMode = 'manual' | 'low_fps_continuous'

export interface ScanBudgetState {
  /** 当日已用 detect 次数 */
  usedToday: number
  /** 日额度 */
  dailyQuota: number
  /** 免费额度（计入 dailyQuota） */
  freeQuota: number
  dayKey: string // YYYY-MM-DD
  mode: ScanMode
  lastScanAt: number
  /** 最小扫描间隔 ms（防连点） */
  minIntervalMs: number
}

const DEFAULT: ScanBudgetState = {
  usedToday: 0,
  dailyQuota: 100,
  freeQuota: 100,
  dayKey: '',
  mode: 'manual',
  lastScanAt: 0,
  minIntervalMs: 1500,
}

function todayKey(now = Date.now()): string {
  return new Date(now).toISOString().slice(0, 10)
}

export function loadScanBudget(now = Date.now()): ScanBudgetState {
  try {
    const raw = localStorage.getItem(KEY)
    const parsed = raw ? ({ ...DEFAULT, ...JSON.parse(raw) } as ScanBudgetState) : { ...DEFAULT }
    const key = todayKey(now)
    if (parsed.dayKey !== key) {
      return { ...parsed, dayKey: key, usedToday: 0, mode: 'manual' }
    }
    return parsed
  } catch {
    return { ...DEFAULT, dayKey: todayKey(now) }
  }
}

export function saveScanBudget(s: ScanBudgetState): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(s))
  } catch {
    /* ignore */
  }
}

export type ScanGateResult =
  | { ok: true; remaining: number; state: ScanBudgetState }
  | { ok: false; reason: 'quota_exhausted' | 'too_fast' | 'low_power_manual_only'; remaining: number; state: ScanBudgetState }

/** 本地质量门禁：极简 hash + 尺寸检查（非 AI） */
export async function localFrameQualityGate(blob: Blob): Promise<{ ok: boolean; reason?: string }> {
  if (blob.size < 2_000) return { ok: false, reason: 'frame_too_small' }
  if (blob.size > 4_000_000) return { ok: false, reason: 'frame_too_large' }
  // 重复帧：对前 64 bytes 做简易指纹（MVP）
  const buf = new Uint8Array(await blob.slice(0, 64).arrayBuffer())
  let h = 0
  for (const b of buf) h = (h * 31 + b) >>> 0
  const last = Number(sessionStorage.getItem('ap-last-frame-hash') || '0')
  if (last && last === h) return { ok: false, reason: 'duplicate_frame' }
  sessionStorage.setItem('ap-last-frame-hash', String(h))
  return { ok: true }
}

export function canStartScan(state = loadScanBudget(), now = Date.now()): ScanGateResult {
  const s = loadScanBudget(now)
  const remaining = Math.max(0, s.dailyQuota - s.usedToday)
  if (remaining <= 0) {
    return { ok: false, reason: 'quota_exhausted', remaining: 0, state: s }
  }
  if (now - s.lastScanAt < s.minIntervalMs) {
    return { ok: false, reason: 'too_fast', remaining, state: s }
  }
  return { ok: true, remaining, state: s }
}

/** 成功发起一次 detect 后记账 */
export function recordScanAttempt(state = loadScanBudget(), now = Date.now()): ScanBudgetState {
  const s = loadScanBudget(now)
  const next: ScanBudgetState = {
    ...s,
    usedToday: s.usedToday + 1,
    lastScanAt: now,
    dayKey: todayKey(now),
    mode: 'manual',
  }
  saveScanBudget(next)
  return next
}

export function scanModeCopy(mode: ScanMode): string {
  return mode === 'manual'
    ? '手动扫描（省额度）'
    : '低频连续扫描（耗电/耗额度，MVP 默认关闭）'
}
