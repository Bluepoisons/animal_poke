/**
 * Shared accessibility hooks — focus trap, Escape, route announcer, progressbar, live region.
 * AP-071: Accessibility — focus management, dynamic progress, and page-switch semantics.
 */
import { useEffect, useRef, useState } from 'react'

// ---------- FocusTrap ----------

/** Focusable selector: links, buttons, inputs, selects, textareas, [tabindex] >= 0. */
const FOCUSABLE = 'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])'

export interface FocusTrapOptions {
  /** Root element ref (e.g. dialog container). Trap is active when `active` is true. */
  containerRef: React.RefObject<HTMLElement | null>
  active: boolean
  /** Called when Escape is pressed while trap is active (also closes on click outside if enabled). */
  onEscape?: () => void
  /** ID of the trigger element to restore focus to on close. */
  triggerId?: string
  /** Also close when clicking the backdrop (outside the container). Requires a `data-trap-backdrop` attribute on the backdrop element. */
  closeOnBackdrop?: boolean
  /** Unmount-time callback (cleanup). */
  onClose?: () => void
}

/**
 * useFocusTrap — traps Tab/Shift+Tab within `containerRef` and handles Escape.
 */
export function useFocusTrap(opts: FocusTrapOptions): void {
  const { containerRef, active, onEscape, triggerId, closeOnBackdrop } = opts
  const triggerRef = useRef<HTMLElement | null>(null)

  // Capture trigger on mount
  useEffect(() => {
    if (triggerId) {
      triggerRef.current = document.getElementById(triggerId)
    } else if (active) {
      triggerRef.current = document.activeElement as HTMLElement
    }
  }, [triggerId, active])

  useEffect(() => {
    if (!active || !containerRef.current) return

    const el = containerRef.current
    const focusable = () => Array.from(el.querySelectorAll<HTMLElement>(FOCUSABLE)).filter((candidate) => (
      !candidate.closest('[aria-hidden="true"], [hidden]') &&
      getComputedStyle(candidate).visibility !== 'hidden'
    ))

    // Move focus into the first focusable child
    const first = focusable()
    if (first.length > 0) {
      requestAnimationFrame(() => first[0].focus())
    } else {
      el.setAttribute('tabindex', '-1')
      el.focus()
    }

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onEscape?.()
        return
      }
      if (e.key !== 'Tab') return

      const items = focusable()
      if (items.length === 0) { e.preventDefault(); return }

      const firstItem = items[0]
      const lastItem = items[items.length - 1]
      if (e.shiftKey && document.activeElement === firstItem) {
        e.preventDefault()
        lastItem.focus()
      } else if (!e.shiftKey && document.activeElement === lastItem) {
        e.preventDefault()
        firstItem.focus()
      }
    }

    const onClickOutside = (e: MouseEvent) => {
      if (!closeOnBackdrop) return
      const target = e.target as HTMLElement | null
      if (target && target.closest('[data-trap-backdrop]') && !el.contains(target)) {
        onEscape?.()
      }
    }

    document.addEventListener('keydown', onKeyDown)
    if (closeOnBackdrop) {
      document.addEventListener('mousedown', onClickOutside)
    }

    return () => {
      document.removeEventListener('keydown', onKeyDown)
      if (closeOnBackdrop) {
        document.removeEventListener('mousedown', onClickOutside)
      }
      // Restore focus
      triggerRef.current?.focus()
    }
  }, [active, containerRef, onEscape, closeOnBackdrop])
}

// ---------- RouteAnnouncer ----------

/**
 * RouteAnnouncer — announces SPA route changes to screen readers
 * via an off-screen aria-live region.
 */
export function useRouteAnnouncer(title: string): void {
  useEffect(() => {
    if (typeof document === 'undefined') return
    const el = document.getElementById('ap-route-announcer')
    if (el) {
      el.textContent = '' // clear first so successive navigations to same title re-announce
      requestAnimationFrame(() => { el.textContent = title })
    }
  }, [title])
}

/**
 * RouteAnnouncerElement — render once (e.g. in App or layout root).
 * Visually hidden but read by screen readers on every textContent update.
 */
export function RouteAnnouncerElement(): JSX.Element {
  return (
    <div
      id="ap-route-announcer"
      role="status"
      aria-live="assertive"
      aria-atomic="true"
      style={{
        position: 'absolute',
        width: 1,
        height: 1,
        padding: 0,
        margin: -1,
        overflow: 'hidden',
        clip: 'rect(0,0,0,0)',
        whiteSpace: 'nowrap',
        border: 0,
      }}
    />
  )
}

// ---------- ProgressBar ----------

export interface ProgressBarProps {
  value: number      // current value
  max?: number       // default 100
  label: string      // accessible name
  /** Optional value text override. Defaults to `${value} / ${max}`. */
  valueText?: string
  size?: 'sm' | 'md'
  animated?: boolean
  className?: string
  id?: string
}

/**
 * ProgressBar — correct role="progressbar", aria-valuenow/min/max, accessible label.
 */
export function ProgressBar({
  value, max = 100, label, valueText, size = 'md', animated = true, className = '', id,
}: ProgressBarProps): JSX.Element {
  const pct = max > 0 ? Math.min(100, Math.max(0, Math.round((value / max) * 100))) : 0
  const displayText = valueText ?? `${value} / ${max}`

  return (
    <div
      id={id}
      role="progressbar"
      aria-valuenow={value}
      aria-valuemin={0}
      aria-valuemax={max}
      aria-label={label}
      aria-valuetext={displayText}
      className={`ap-progressbar ap-progressbar--${size} ${animated ? 'ap-progressbar--animated' : ''} ${className}`}
      style={{ '--ap-progress-pct': `${pct}%` } as React.CSSProperties}
    >
      <div className="ap-progressbar__track">
        <div className="ap-progressbar__fill" />
      </div>
      <span className="ap-progressbar__label" aria-hidden="true">{displayText}</span>
    </div>
  )
}

// ---------- ErrorLiveRegion ----------

export interface ErrorLiveRegionProps {
  message: string | null
  polite?: boolean
}

/**
 * ErrorLiveRegion — role="alert" for errors (assertive) or polite for status updates.
 * Empty message hides the element visually but keeps it in the DOM.
 */
export function ErrorLiveRegion({ message, polite }: ErrorLiveRegionProps): JSX.Element {
  const role = polite ? 'status' : 'alert'
  return (
    <div
      role={role}
      aria-live={polite ? 'polite' : 'assertive'}
      data-testid="error-live-region"
      style={{
        minHeight: '1.5em',
        fontSize: 13,
        color: message ? '#B71C1C' : 'transparent',
        transition: 'color 0.15s',
      }}
    >
      {message || '\u00A0'}
    </div>
  )
}

// ---------- useErrorLiveRegion ----------

/**
 * useErrorLiveRegion — returns [element, setMessage].
 * The element should be rendered once; calling setMessage updates it.
 */
export function useErrorLiveRegion(): [JSX.Element, (msg: string | null) => void] {
  const [message, setMessage] = useState<string | null>(null)
  const el = <ErrorLiveRegion message={message} />
  return [el, setMessage]
}
