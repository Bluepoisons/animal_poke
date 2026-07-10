import { describe, it, expect, beforeEach, vi } from 'vitest'
import {
  setCaptureActive,
  onNeedRefresh,
  applyPendingUpdate,
  getUpdateGateState,
  __resetUpdateGateForTests,
} from './updateGate'

describe('PWA update gate (AP-040)', () => {
  beforeEach(() => {
    __resetUpdateGateForTests()
  })

  it('defers apply while capturing', async () => {
    const apply = vi.fn()
    setCaptureActive(true)
    onNeedRefresh(apply)
    expect(getUpdateGateState().deferred).toBe(true)
    expect(getUpdateGateState().needRefresh).toBe(true)
    const ok = await applyPendingUpdate()
    expect(ok).toBe(false)
    expect(apply).not.toHaveBeenCalled()
  })

  it('applies when user confirms and not capturing', async () => {
    const apply = vi.fn()
    setCaptureActive(false)
    onNeedRefresh(apply)
    const ok = await applyPendingUpdate()
    expect(ok).toBe(true)
    expect(apply).toHaveBeenCalledTimes(1)
    expect(getUpdateGateState().needRefresh).toBe(false)
  })

  it('auto-applies deferred update when capture ends', async () => {
    const apply = vi.fn()
    setCaptureActive(true)
    onNeedRefresh(apply)
    expect(apply).not.toHaveBeenCalled()
    setCaptureActive(false)
    expect(apply).toHaveBeenCalledTimes(1)
  })
})
