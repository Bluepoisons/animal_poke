import { describe, it, expect } from 'vitest'
import { discoveryToHuntTarget, projectToMap } from './lbsMap'
import type { DiscoveryPoint } from '../../lbs/types'

describe('lbsMap projection', () => {
  it('projects player-relative points into canvas bounds', () => {
    const player = { lat: 30.0, lng: 120.0 }
    const p = projectToMap(player, { lat: 30.001, lng: 120.001 })
    expect(p.x).toBeGreaterThan(0.08)
    expect(p.x).toBeLessThan(0.92)
    expect(p.distanceMeters).toBeGreaterThan(0)
  })

  it('maps discovery point to hunt target', () => {
    const point: DiscoveryPoint = {
      id: 'p1',
      lat: 30.001,
      lng: 120.0,
      species: 'cat',
      rarity: 'rare',
      spawnedAt: Date.now(),
      expiresAt: Date.now() + 600_000,
      status: 'available',
    }
    const t = discoveryToHuntTarget(point, { lat: 30, lng: 120 })
    expect(t.species).toBe('cat')
    expect(t.rarity).toBe('rare')
    expect(t.id).toBe('p1')
    expect(t.label).toBe('猫 · 等待探索')
    expect(t.label).not.toContain('cat')
  })
})
