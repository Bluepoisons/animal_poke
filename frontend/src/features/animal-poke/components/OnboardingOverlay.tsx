import { useState } from 'react'
import {
  advanceOnboarding,
  loadOnboarding,
  skipOnboarding,
  stepCopy,
  type OnboardingState,
} from '../onboarding'

export default function OnboardingOverlay() {
  const [state, setState] = useState<OnboardingState>(() => loadOnboarding())
  if (state.step === 'done' || state.skipped) return null
  const copy = stepCopy(state.step)
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="onb-title"
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
        <h2 id="onb-title" style={{ marginTop: 0 }}>
          {copy.title}
        </h2>
        <p style={{ lineHeight: 1.5, fontSize: 14 }}>{copy.body}</p>
        <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
          <button
            type="button"
            className="ap-map-chip"
            onClick={() => setState(advanceOnboarding())}
          >
            继续
          </button>
          <button type="button" className="ap-map-chip" onClick={() => setState(skipOnboarding())}>
            跳过教学
          </button>
        </div>
      </div>
    </div>
  )
}
