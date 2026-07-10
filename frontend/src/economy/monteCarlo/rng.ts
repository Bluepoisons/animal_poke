/** Deterministic mulberry32 RNG for Monte Carlo (AP-051) */

export type Rng = () => number

export function createMulberry32(seed: number): Rng {
  let t = seed >>> 0
  return () => {
    t += 0x6d2b79f5
    let r = Math.imul(t ^ (t >>> 15), 1 | t)
    r ^= r + Math.imul(r ^ (r >>> 7), 61 | r)
    return ((r ^ (r >>> 14)) >>> 0) / 4294967296
  }
}

/** Deterministic day clock: day index → ms since epoch (fixed base) */
export const SIM_EPOCH_MS = Date.UTC(2026, 0, 1, 0, 0, 0)

export function dayToMs(dayIndex: number): number {
  return SIM_EPOCH_MS + dayIndex * 24 * 60 * 60 * 1000
}
