import { useCallback, useEffect, useRef, useState } from 'react'
import {
  applyOnboardingEvent,
  isInfoStep,
  isOnboardingActive,
  loadOnboarding,
  skipOnboarding,
  stepCopy,
  type OnboardingState,
} from '../onboarding'
import { useFocusTrap } from '../../../a11y'
import { useI18n } from '../../../i18n'

type Props = {
  /** Called when player finishes rationale and should start sensors / discover. */
  onRationaleDone?: () => void
  /** Navigate to pokedex from reveal CTA. */
  onOpenPokedex?: () => void
}

/**
 * AP-066 coach overlay: info steps are modal; task steps are non-blocking
 * banners so the player can complete real scan/throw/pokedex actions.
 */
export default function OnboardingOverlay({ onRationaleDone, onOpenPokedex }: Props) {
  const [state, setState] = useState<OnboardingState>(() => loadOnboarding())
  const dialogRef = useRef<HTMLDivElement>(null)
  const { t } = useI18n()

  const refresh = useCallback(() => {
    setState(loadOnboarding())
  }, [])

  useEffect(() => {
    const onChanged = () => refresh()
    window.addEventListener('animal-poke-onboarding-changed', onChanged)
    // Poll lightly after storage writes from other modules (same tab).
    const id = window.setInterval(refresh, 400)
    return () => {
      window.removeEventListener('animal-poke-onboarding-changed', onChanged)
      window.clearInterval(id)
    }
  }, [refresh])

  const active = isOnboardingActive(state)
  const modal = active && isInfoStep(state.step)

  useFocusTrap({
    containerRef: dialogRef,
    active: modal,
    onEscape: () => {
      setState(skipOnboarding())
      window.dispatchEvent(new CustomEvent('animal-poke-onboarding-changed'))
    },
  })

  if (!active) return null

  const copy = stepCopy(state.step)
  const stepIndex = ['rationale', 'train_scan', 'select_target', 'throw', 'reveal', 'pokedex'].indexOf(
    state.step,
  )
  const total = 6

  const handlePrimary = () => {
    if (state.step === 'rationale') {
      const next = applyOnboardingEvent('continue')
      setState(next)
      window.dispatchEvent(new CustomEvent('animal-poke-onboarding-changed', { detail: next }))
      onRationaleDone?.()
      return
    }
    if (state.step === 'reveal') {
      const next = applyOnboardingEvent('continue')
      setState(next)
      window.dispatchEvent(new CustomEvent('animal-poke-onboarding-changed', { detail: next }))
      onOpenPokedex?.()
      return
    }
  }

  const handleSkip = () => {
    const next = skipOnboarding()
    setState(next)
    window.dispatchEvent(new CustomEvent('animal-poke-onboarding-changed', { detail: next }))
  }

  const primaryLabel =
    state.step === 'rationale'
      ? t('onboarding.start_training' as never) || '开始训练'
      : state.step === 'reveal'
        ? t('onboarding.open_pokedex' as never) || '去图鉴'
        : null

  // Modal card for info steps; docked coach for task steps.
  if (modal) {
    return (
      <div
        ref={dialogRef}
        className="ap-trap-container"
        role="dialog"
        aria-modal="true"
        aria-labelledby="onb-title"
        data-testid="onboarding-overlay"
        data-onboarding-step={state.step}
        data-onboarding-mode="modal"
        style={{
          position: 'fixed',
          inset: 0,
          zIndex: 80,
          background: 'rgba(74,44,26,0.45)',
          display: 'grid',
          placeItems: 'center',
          padding: 16,
        }}
      >
        <div
          style={{
            maxWidth: 340,
            width: '100%',
            background: '#FFFDF8',
            border: '3px solid #2B2B2B',
            borderRadius: 18,
            padding: 18,
          }}
        >
          <p
            style={{ margin: '0 0 8px', fontSize: 12, color: '#8D6E63' }}
            data-testid="onboarding-progress"
          >
            {Math.max(1, stepIndex + 1)} / {total}
          </p>
          <h2 id="onb-title" style={{ marginTop: 0 }}>
            {copy.title}
          </h2>
          <p style={{ lineHeight: 1.5, fontSize: 14 }}>{copy.body}</p>
          {copy.waitHint && (
            <p style={{ fontSize: 12, color: '#8D6E63' }} data-testid="onboarding-wait-hint">
              {copy.waitHint}
            </p>
          )}
          <div style={{ display: 'flex', gap: 8, marginTop: 12, flexWrap: 'wrap' }}>
            {primaryLabel && (
              <button
                type="button"
                className="ap-map-chip"
                data-testid="onboarding-continue"
                onClick={handlePrimary}
              >
                {primaryLabel}
              </button>
            )}
            <button
              type="button"
              className="ap-map-chip"
              data-testid="onboarding-skip"
              onClick={handleSkip}
            >
              {t('onboarding.skip')}
            </button>
          </div>
        </div>
      </div>
    )
  }

  // Non-blocking coach: pointer-events none on shell, auto on card.
  return (
    <div
      data-testid="onboarding-overlay"
      data-onboarding-step={state.step}
      data-onboarding-mode="coach"
      style={{
        position: 'fixed',
        left: 0,
        right: 0,
        bottom: 72,
        zIndex: 70,
        display: 'flex',
        justifyContent: 'center',
        padding: '0 12px',
        pointerEvents: 'none',
      }}
    >
      <div
        role="status"
        aria-live="polite"
        style={{
          pointerEvents: 'auto',
          maxWidth: 360,
          width: '100%',
          background: '#FFFDF8',
          border: '2.5px solid #2B2B2B',
          borderRadius: 14,
          padding: '12px 14px',
          boxShadow: '0 6px 0 rgba(43,43,43,0.12)',
        }}
      >
        <p
          style={{ margin: '0 0 4px', fontSize: 11, color: '#8D6E63' }}
          data-testid="onboarding-progress"
        >
          {Math.max(1, stepIndex + 1)} / {total} · {copy.title}
        </p>
        <p style={{ margin: 0, lineHeight: 1.45, fontSize: 13 }}>{copy.body}</p>
        {copy.waitHint && (
          <p
            style={{ margin: '6px 0 0', fontSize: 11, color: '#8D6E63' }}
            data-testid="onboarding-wait-hint"
          >
            {copy.waitHint}
          </p>
        )}
        <div style={{ display: 'flex', gap: 8, marginTop: 8, flexWrap: 'wrap' }}>
          {primaryLabel && (
            <button
              type="button"
              className="ap-map-chip"
              data-testid="onboarding-continue"
              onClick={handlePrimary}
            >
              {primaryLabel}
            </button>
          )}
          <button
            type="button"
            className="ap-map-chip"
            data-testid="onboarding-skip"
            onClick={handleSkip}
          >
            {t('onboarding.skip')}
          </button>
        </div>
      </div>
    </div>
  )
}
