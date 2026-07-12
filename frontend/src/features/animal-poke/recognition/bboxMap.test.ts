import { describe, it, expect } from 'vitest'
import {
  clampNormBox,
  contentRect,
  mapNormBoxToElement,
  maxCornerError,
} from './bboxMap'

describe('clampNormBox', () => {
  it('clamps overflow and negative size', () => {
    const overflow = clampNormBox([0.9, 0.9, 0.5, 0.5])
    expect(overflow[0]).toBeCloseTo(0.9)
    expect(overflow[1]).toBeCloseTo(0.9)
    expect(overflow[2]).toBeCloseTo(0.1)
    expect(overflow[3]).toBeCloseTo(0.1)
    expect(clampNormBox([-0.2, 0.1, 0.5, 0.5])).toEqual([0, 0.1, 0.5, 0.5])
    expect(clampNormBox([0.1, 0.1, -1, 0.2])).toEqual([0.1, 0.1, 0, 0.2])
  })
})

describe('contentRect object-fit', () => {
  it('cover crops the longer side (landscape video in square)', () => {
    // 1280x720 into 280x280 → scale = 280/720
    const r = contentRect(1280, 720, 280, 280, 'cover')
    expect(r.scale).toBeCloseTo(280 / 720, 5)
    expect(r.displayH).toBeCloseTo(280, 5)
    expect(r.displayW).toBeCloseTo(1280 * (280 / 720), 5)
    expect(r.offsetY).toBeCloseTo(0, 5)
    expect(r.offsetX).toBeLessThan(0) // horizontal crop
  })

  it('contain letterboxes', () => {
    const r = contentRect(1280, 720, 280, 280, 'contain')
    expect(r.scale).toBeCloseTo(280 / 1280, 5)
    expect(r.offsetX).toBeCloseTo(0, 5)
    expect(r.offsetY).toBeGreaterThan(0)
  })
})

describe('mapNormBoxToElement', () => {
  it('1:1 video and element: identity mapping', () => {
    const rect = mapNormBoxToElement([0.25, 0.25, 0.5, 0.5], {
      videoWidth: 100,
      videoHeight: 100,
      elementWidth: 100,
      elementHeight: 100,
      objectFit: 'cover',
    })
    expect(rect.left).toBeCloseTo(25, 5)
    expect(rect.top).toBeCloseTo(25, 5)
    expect(rect.width).toBeCloseTo(50, 5)
    expect(rect.height).toBeCloseTo(50, 5)
  })

  it('mirrorX flips horizontal for front camera', () => {
    const base = mapNormBoxToElement([0.1, 0.2, 0.2, 0.3], {
      videoWidth: 100,
      videoHeight: 100,
      elementWidth: 100,
      elementHeight: 100,
    })
    const mirrored = mapNormBoxToElement([0.1, 0.2, 0.2, 0.3], {
      videoWidth: 100,
      videoHeight: 100,
      elementWidth: 100,
      elementHeight: 100,
      mirrorX: true,
    })
    // original left 10 → mirrored left = 100 - 10 - 20 = 70
    expect(mirrored.left).toBeCloseTo(70, 5)
    expect(mirrored.top).toBeCloseTo(base.top, 5)
    expect(mirrored.width).toBeCloseTo(base.width, 5)
  })

  it('cover mapping error under threshold for multi aspect ratios', () => {
    const cases: Array<{ vw: number; vh: number; ew: number; eh: number }> = [
      { vw: 1280, vh: 720, ew: 280, eh: 280 }, // landscape → square
      { vw: 720, vh: 1280, ew: 280, eh: 280 }, // portrait → square
      { vw: 1920, vh: 1080, ew: 360, eh: 200 }, // landscape element
      { vw: 640, vh: 480, ew: 200, eh: 360 }, // portrait element
    ]
    for (const c of cases) {
      const box: [number, number, number, number] = [0.2, 0.3, 0.4, 0.25]
      const mapped = mapNormBoxToElement(box, {
        videoWidth: c.vw,
        videoHeight: c.vh,
        elementWidth: c.ew,
        elementHeight: c.eh,
        objectFit: 'cover',
        clamp: false,
      })
      const { offsetX, offsetY, displayW, displayH } = contentRect(
        c.vw,
        c.vh,
        c.ew,
        c.eh,
        'cover',
      )
      const expected = {
        left: offsetX + 0.2 * displayW,
        top: offsetY + 0.3 * displayH,
        width: 0.4 * displayW,
        height: 0.25 * displayH,
      }
      // Threshold: ≤ 1.5px corner error (sub-pixel rounding ok)
      expect(maxCornerError(mapped, expected)).toBeLessThanOrEqual(1.5)
    }
  })

  it('clamps boxes that extend past element after cover crop', () => {
    const rect = mapNormBoxToElement([0, 0, 1, 1], {
      videoWidth: 1280,
      videoHeight: 720,
      elementWidth: 100,
      elementHeight: 100,
      objectFit: 'cover',
      clamp: true,
    })
    expect(rect.left).toBeGreaterThanOrEqual(0)
    expect(rect.top).toBeGreaterThanOrEqual(0)
    expect(rect.left + rect.width).toBeLessThanOrEqual(100.001)
    expect(rect.top + rect.height).toBeLessThanOrEqual(100.001)
  })

  it('multi-target boxes stay distinct in display space', () => {
    const a = mapNormBoxToElement([0.1, 0.1, 0.2, 0.2], {
      videoWidth: 640,
      videoHeight: 480,
      elementWidth: 280,
      elementHeight: 280,
    })
    const b = mapNormBoxToElement([0.6, 0.5, 0.2, 0.2], {
      videoWidth: 640,
      videoHeight: 480,
      elementWidth: 280,
      elementHeight: 280,
    })
    // centers should differ
    const cax = a.left + a.width / 2
    const cay = a.top + a.height / 2
    const cbx = b.left + b.width / 2
    const cby = b.top + b.height / 2
    expect(Math.hypot(cax - cbx, cay - cby)).toBeGreaterThan(40)
  })
})
