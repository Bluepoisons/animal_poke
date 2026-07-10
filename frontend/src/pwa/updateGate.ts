/**
 * AP-040 controlled PWA update gate.
 * Defers applying a waiting service worker while capture flow is active.
 */

export type UpdateGateState = {
  needRefresh: boolean
  capturing: boolean
  deferred: boolean
}

type ApplyFn = () => void | Promise<void>

let capturing = false
let needRefresh = false
let deferred = false
let applyUpdate: ApplyFn | null = null
const listeners = new Set<() => void>()

function emit() {
  for (const l of listeners) l()
}

export function setCaptureActive(active: boolean): void {
  capturing = active
  if (!capturing && deferred && applyUpdate) {
    // Capture finished — apply deferred update
    const fn = applyUpdate
    deferred = false
    needRefresh = false
    applyUpdate = null
    void fn()
  }
  emit()
}

export function isCaptureActive(): boolean {
  return capturing
}

/**
 * Register that a new SW is waiting. If capturing, defer; else apply immediately when allowImmediate.
 */
export function onNeedRefresh(apply: ApplyFn, opts?: { allowImmediate?: boolean }): void {
  applyUpdate = apply
  needRefresh = true
  if (capturing) {
    deferred = true
    emit()
    return
  }
  if (opts?.allowImmediate !== false) {
    // Prompt mode: do not auto-apply; leave needRefresh true for UI
    deferred = false
    emit()
    return
  }
  emit()
}

/** User confirms update (or tests force apply). */
export async function applyPendingUpdate(): Promise<boolean> {
  if (!applyUpdate) return false
  if (capturing) {
    deferred = true
    emit()
    return false
  }
  const fn = applyUpdate
  applyUpdate = null
  needRefresh = false
  deferred = false
  await fn()
  emit()
  return true
}

export function getUpdateGateState(): UpdateGateState {
  return { needRefresh, capturing, deferred }
}

export function subscribeUpdateGate(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

/** test helper */
export function __resetUpdateGateForTests(): void {
  capturing = false
  needRefresh = false
  deferred = false
  applyUpdate = null
  listeners.clear()
}
