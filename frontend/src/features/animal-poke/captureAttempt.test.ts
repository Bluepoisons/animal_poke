import { describe, it, expect } from 'vitest'
import {
  createEncounter,
  beginCharge,
  updatePower,
  settleAttempt,
  canRetry,
  startNextAttempt,
  currentAttempt,
} from './captureAttempt'

describe('captureAttempt', () => {
  it('charges power and settles once per attempt', () => {
    let enc = createEncounter('cat', 3)
    enc = beginCharge(enc)
    enc = updatePower(enc, 55)
    const r1 = settleAttempt(enc, {
      online: true,
      stamina: 100,
      consumeStamina: () => true,
      rng: () => 0, // always success if p>0
    })
    expect(r1.ok).toBe(true)
    expect(r1.enc.locked).toBe(true)
    const r2 = settleAttempt(r1.enc, {
      online: true,
      stamina: 100,
      consumeStamina: () => true,
      rng: () => 0,
    })
    expect(r2.reason).toBe('already_settled')
  })

  it('allows retry after fail until maxAttempts', () => {
    let enc = createEncounter('dog', 2)
    enc = beginCharge(enc)
    enc = updatePower(enc, 10)
    const fail = settleAttempt(enc, {
      online: true,
      stamina: 100,
      consumeStamina: () => true,
      rng: () => 0.99,
    })
    expect(fail.ok).toBe(false)
    expect(canRetry(fail.enc)).toBe(true)
    enc = startNextAttempt(fail.enc)
    expect(currentAttempt(enc)?.index).toBe(1)
    enc = beginCharge(enc)
    enc = updatePower(enc, 10)
    const fail2 = settleAttempt(enc, {
      online: true,
      stamina: 100,
      consumeStamina: () => true,
      rng: () => 0.99,
    })
    expect(canRetry(fail2.enc)).toBe(false)
    expect(fail2.enc.locked).toBe(true)
  })

  it('does not consume stamina twice on same attempt', () => {
    let spent = 0
    let enc = createEncounter('goose', 3)
    enc = beginCharge(enc)
    enc = updatePower(enc, 50)
    // first settle offline after... actually no_stamina path
    const r = settleAttempt(enc, {
      online: true,
      stamina: 100,
      consumeStamina: () => {
        spent += 1
        return true
      },
      rng: () => 0.99,
    })
    expect(spent).toBe(1)
    // already settled - no more spend
    settleAttempt(r.enc, {
      online: true,
      stamina: 100,
      consumeStamina: () => {
        spent += 1
        return true
      },
      rng: () => 0,
    })
    expect(spent).toBe(1)
  })

  it('uses each species optimal range for the capture roll', () => {
    const settle = (species: string, power: number, roll: number) => {
      let enc = createEncounter(species, 1)
      enc = beginCharge(enc)
      enc = updatePower(enc, power)
      return settleAttempt(enc, {
        online: true,
        stamina: 100,
        consumeStamina: () => true,
        rng: () => roll,
      })
    }

    // Power 82 is inside dog [45, 85], but outside cat [40, 80].
    expect(settle('dog', 82, 0.76).ok).toBe(true)
    expect(settle('cat', 82, 0.76).ok).toBe(false)

    // Power 40 is inside cat [40, 80], but outside dog [45, 85].
    expect(settle('cat', 40, 0.7).ok).toBe(true)
    expect(settle('dog', 40, 0.7).ok).toBe(false)
  })
})
