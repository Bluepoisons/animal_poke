import { describe, it, expect } from 'vitest'
import { createCaptureSession, settleCapture, successProbability } from './session'

describe('capture session', () => {
  it('creates unique idempotency key', () => {
    const a = createCaptureSession({ species: 'cat' })
    const b = createCaptureSession({ species: 'cat' })
    expect(a.idempotencyKey).not.toBe(b.idempotencyKey)
  })

  it('settles only once', () => {
    let stamina = 100
    const s = createCaptureSession({ power: 55 })
    const r1 = settleCapture({
      session: s,
      online: true,
      stamina,
      consumeStamina: (n) => {
        stamina -= n
        return true
      },
      rng: () => 0.1,
    })
    expect(r1.ok).toBe(true)
    const r2 = settleCapture({
      session: r1.session,
      online: true,
      stamina,
      consumeStamina: () => true,
      rng: () => 0.1,
    })
    expect(r2.ok).toBe(false)
    if (!r2.ok) expect(r2.reason).toBe('already_settled')
    expect(stamina).toBe(80)
  })

  it('blocks offline and no stamina', () => {
    const s = createCaptureSession({})
    const offline = settleCapture({
      session: s,
      online: false,
      stamina: 100,
      consumeStamina: () => true,
    })
    expect(offline.ok).toBe(false)
    const noSta = settleCapture({
      session: createCaptureSession({}),
      online: true,
      stamina: 10,
      consumeStamina: () => false,
    })
    expect(noSta.ok).toBe(false)
  })

  it('power affects probability', () => {
    expect(successProbability(55)).toBeGreaterThan(successProbability(10))
  })
})
