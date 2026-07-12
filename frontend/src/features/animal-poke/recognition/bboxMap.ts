/**
 * AP-065 — Map normalized detection boxes onto a viewfinder element.
 * BBox schema: [x, y, width, height] in 0..1 of the source video frame (AP-034).
 */

export type NormBox = readonly [number, number, number, number]

export type ObjectFitMode = 'cover' | 'contain' | 'fill'

export type RectPx = {
  left: number
  top: number
  width: number
  height: number
}

export type MapBoxOptions = {
  videoWidth: number
  videoHeight: number
  elementWidth: number
  elementHeight: number
  objectFit?: ObjectFitMode
  /** Mirror horizontally (front camera preview). */
  mirrorX?: boolean
  /** Clamp result inside the element (default true). */
  clamp?: boolean
}

/** Clamp a normalized box to [0,1] with non-negative size. */
export function clampNormBox(box: NormBox): [number, number, number, number] {
  let [x, y, w, h] = box
  if (!Number.isFinite(x)) x = 0
  if (!Number.isFinite(y)) y = 0
  if (!Number.isFinite(w)) w = 0
  if (!Number.isFinite(h)) h = 0
  w = Math.max(0, w)
  h = Math.max(0, h)
  x = Math.min(1, Math.max(0, x))
  y = Math.min(1, Math.max(0, y))
  if (x + w > 1) w = 1 - x
  if (y + h > 1) h = 1 - y
  return [x, y, w, h]
}

/**
 * Content rect of the video inside the element under object-fit.
 * Returns offset + displayed video size in element pixels.
 */
export function contentRect(
  videoWidth: number,
  videoHeight: number,
  elementWidth: number,
  elementHeight: number,
  objectFit: ObjectFitMode = 'cover',
): { offsetX: number; offsetY: number; displayW: number; displayH: number; scale: number } {
  const vw = Math.max(1, videoWidth)
  const vh = Math.max(1, videoHeight)
  const ew = Math.max(1, elementWidth)
  const eh = Math.max(1, elementHeight)

  if (objectFit === 'fill') {
    return { offsetX: 0, offsetY: 0, displayW: ew, displayH: eh, scale: ew / vw }
  }

  const scale =
    objectFit === 'contain' ? Math.min(ew / vw, eh / vh) : Math.max(ew / vw, eh / vh)
  const displayW = vw * scale
  const displayH = vh * scale
  const offsetX = (ew - displayW) / 2
  const offsetY = (eh - displayH) / 2
  return { offsetX, offsetY, displayW, displayH, scale }
}

/** Map one normalized box to element CSS pixel rect. */
export function mapNormBoxToElement(box: NormBox, opts: MapBoxOptions): RectPx {
  const fit = opts.objectFit ?? 'cover'
  const [x0, y0, w0, h0] = clampNormBox(box)
  const { offsetX, offsetY, displayW, displayH } = contentRect(
    opts.videoWidth,
    opts.videoHeight,
    opts.elementWidth,
    opts.elementHeight,
    fit,
  )

  let nx = x0
  let nw = w0
  if (opts.mirrorX) {
    // Flip around vertical center of the frame
    nx = 1 - x0 - w0
    nw = w0
  }

  let left = offsetX + nx * displayW
  let top = offsetY + y0 * displayH
  let width = nw * displayW
  let height = h0 * displayH

  if (opts.clamp !== false) {
    const ew = Math.max(1, opts.elementWidth)
    const eh = Math.max(1, opts.elementHeight)
    // Intersect with element bounds
    const right = Math.min(ew, left + width)
    const bottom = Math.min(eh, top + height)
    left = Math.max(0, left)
    top = Math.max(0, top)
    width = Math.max(0, right - left)
    height = Math.max(0, bottom - top)
  }

  return { left, top, width, height }
}

/** Max absolute pixel error between mapped corners and expected (for tests). */
export function maxCornerError(a: RectPx, b: RectPx): number {
  const cornersA = [
    [a.left, a.top],
    [a.left + a.width, a.top],
    [a.left, a.top + a.height],
    [a.left + a.width, a.top + a.height],
  ]
  const cornersB = [
    [b.left, b.top],
    [b.left + b.width, b.top],
    [b.left, b.top + b.height],
    [b.left + b.width, b.top + b.height],
  ]
  let max = 0
  for (let i = 0; i < 4; i++) {
    max = Math.max(
      max,
      Math.abs(cornersA[i][0] - cornersB[i][0]),
      Math.abs(cornersA[i][1] - cornersB[i][1]),
    )
  }
  return max
}
