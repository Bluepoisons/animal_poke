/**
 * AP-065 — Viewfinder detection boxes + selection chrome.
 */
import { useLayoutEffect, useRef, useState, type KeyboardEvent, type RefObject } from 'react'
import { mapNormBoxToElement, type RectPx } from '../recognition/bboxMap'
import { confidenceBand } from '../recognition/qualityGuidance'

/**
 * Legacy overlay-only shape. The capture flow intentionally no longer carries
 * bounding boxes or renders this component.
 */
type OverlayDetection = {
  id: string
  species: string
  confidence: number
  boundingBox: [number, number, number, number]
}

export type DetectionOverlayProps = {
  detections: OverlayDetection[]
  selectedId: string | null
  videoWidth: number
  videoHeight: number
  /** Mirror front camera preview */
  mirrorX?: boolean
  objectFit?: 'cover' | 'contain' | 'fill'
  speciesLabel: (species: string) => string
  onSelect: (animalId: string) => void
  /** Visual mode drives border colors */
  visualState: 'processing' | 'selectable' | 'ready_capture' | 'error' | 'low_confidence' | 'idle'
}

function useElementSize(ref: RefObject<HTMLElement | null>) {
  const [size, setSize] = useState({ w: 0, h: 0 })
  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    let lastW = 0
    let lastH = 0
    const measure = () => {
      const r = el.getBoundingClientRect()
      const w = Math.round(r.width)
      const h = Math.round(r.height)
      // Avoid re-render thrash (breaks Playwright actionability "stable")
      if (w === lastW && h === lastH) return
      lastW = w
      lastH = h
      setSize({ w, h })
    }
    measure()
    const ro = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(measure) : null
    ro?.observe(el)
    return () => ro?.disconnect()
  }, [ref])
  return size
}

export default function DetectionOverlay({
  detections,
  selectedId,
  videoWidth,
  videoHeight,
  mirrorX = false,
  objectFit = 'cover',
  speciesLabel,
  onSelect,
  visualState,
}: DetectionOverlayProps) {
  const rootRef = useRef<HTMLDivElement | null>(null)
  const { w: ew, h: eh } = useElementSize(rootRef)

  if (detections.length === 0 || ew < 1 || eh < 1) {
    return (
      <div
        ref={rootRef}
        className="ap-detect-overlay"
        data-testid="detection-overlay"
        data-visual={visualState}
        aria-hidden={detections.length === 0}
      />
    )
  }

  const boxes: Array<{ d: OverlayDetection; rect: RectPx }> = detections.map((d) => ({
    d,
    rect: mapNormBoxToElement(d.boundingBox, {
      videoWidth: videoWidth || 640,
      videoHeight: videoHeight || 480,
      elementWidth: ew,
      elementHeight: eh,
      objectFit,
      mirrorX,
      clamp: true,
    }),
  }))

  const onKey = (e: KeyboardEvent, id: string) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      onSelect(id)
    }
  }

  return (
    <div
      ref={rootRef}
      className="ap-detect-overlay"
      data-testid="detection-overlay"
      data-visual={visualState}
      role="group"
      aria-label="检测到的动物"
    >
      {boxes.map(({ d, rect }, index) => {
        const selected = selectedId === d.id
        const band = confidenceBand(d.confidence)
        const label = `${speciesLabel(d.species)} ${Math.round(d.confidence * 100)}%`
        return (
          <button
            key={d.id}
            type="button"
            className={`ap-detect-box ap-detect-box--${band}${selected ? ' ap-detect-box--selected' : ''}`}
            data-testid={`detect-box-${d.id}`}
            data-selected={selected ? 'true' : 'false'}
            data-band={band}
            style={{
              left: rect.left,
              top: rect.top,
              width: Math.max(24, rect.width),
              height: Math.max(24, rect.height),
            }}
            onClick={() => onSelect(d.id)}
            onKeyDown={(e) => onKey(e, d.id)}
            aria-pressed={selected}
            aria-label={`${label}, target ${index + 1} of ${boxes.length}`}
          >
            <span className="ap-detect-box__label">{label}</span>
          </button>
        )
      })}
    </div>
  )
}
