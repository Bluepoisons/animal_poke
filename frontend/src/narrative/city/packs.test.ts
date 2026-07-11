import { describe, expect, it } from 'vitest'
import {
  cityHarbor,
  cityInland,
  differenceReport,
  placeIsSafe,
  resolveRegionLabel,
} from './packs'

describe('AP-133 city anthology', () => {
  it('second city reuses schema but not isomorphic content', () => {
    const d = differenceReport(cityHarbor, cityInland)
    expect(d.ok).toBe(true)
    expect(d.themeSame).toBe(false)
    expect(d.sharedRoleIds).toEqual([])
    expect(cityInland.nodes.every((n) => n.homeModeAlt.length > 0)).toBe(true)
  })

  it('rejects private-looking places and falls back when assets missing', () => {
    expect(placeIsSafe(cityHarbor, '滨水区')).toBe(true)
    expect(placeIsSafe(cityHarbor, '幸福路12号')).toBe(false)
    expect(resolveRegionLabel(cityHarbor, '滨水区', false)).toBe(cityHarbor.syntheticFallbackRegion)
    expect(resolveRegionLabel(cityInland, undefined, true)).toBe(cityInland.placeLabels[0])
  })
})
