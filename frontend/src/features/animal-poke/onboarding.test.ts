import { describe, it, expect, beforeEach } from 'vitest'
import {
  applyOnboardingEvent,
  isOnboardingActive,
  loadOnboarding,
  ONBOARDING_STORAGE_KEY,
  reduceOnboardingEvent,
  resetOnboarding,
  shouldBlockEconomyForCapture,
  shouldDeferSensors,
  skipOnboarding,
  type OnboardingState,
} from './onboarding'

function fresh(): OnboardingState {
  return resetOnboarding()
}

describe('AP-066 event-driven onboarding', () => {
  beforeEach(() => {
    localStorage.clear()
    fresh()
  })

  it('starts at rationale and defers sensors', () => {
    const s = loadOnboarding()
    expect(s.step).toBe('rationale')
    expect(isOnboardingActive(s)).toBe(true)
    expect(shouldDeferSensors(s)).toBe(true)
  })

  it('only continue advances rationale; other events are no-ops', () => {
    let s = loadOnboarding()
    s = reduceOnboardingEvent(s, 'detect_success')
    expect(s.step).toBe('rationale')
    s = reduceOnboardingEvent(s, 'continue')
    expect(s.step).toBe('train_scan')
    expect(shouldDeferSensors(s)).toBe(false)
  })

  it('happy path: scan → select → throw → reveal → pokedex → done', () => {
    let s = reduceOnboardingEvent(fresh(), 'continue')
    expect(s.step).toBe('train_scan')

    s = reduceOnboardingEvent(s, 'scan_started')
    expect(s.step).toBe('train_scan')

    s = reduceOnboardingEvent(s, 'detect_success')
    expect(s.step).toBe('select_target')

    s = reduceOnboardingEvent(s, 'target_selected')
    expect(s.step).toBe('select_target')

    s = reduceOnboardingEvent(s, 'target_confirmed')
    expect(s.step).toBe('throw')
    expect(shouldBlockEconomyForCapture(s)).toBe(true)

    s = reduceOnboardingEvent(s, 'throw_started')
    expect(s.step).toBe('throw')

    s = reduceOnboardingEvent(s, 'capture_failed')
    expect(s.step).toBe('throw')

    s = reduceOnboardingEvent(s, 'capture_success')
    expect(s.step).toBe('reveal')
    expect(s.trainingCaptureDone).toBe(true)

    s = reduceOnboardingEvent(s, 'continue')
    expect(s.step).toBe('pokedex')

    s = reduceOnboardingEvent(s, 'pokedex_opened')
    expect(s.step).toBe('done')
    expect(s.active).toBe(false)
    expect(s.completedAt).toBeTruthy()
  })

  it('reveal + pokedex_opened can skip intermediate pokedex coach', () => {
    let s = reduceOnboardingEvent(fresh(), 'continue')
    s = reduceOnboardingEvent(s, 'detect_success')
    s = reduceOnboardingEvent(s, 'target_confirmed')
    s = reduceOnboardingEvent(s, 'capture_success')
    expect(s.step).toBe('reveal')
    s = reduceOnboardingEvent(s, 'pokedex_opened')
    expect(s.step).toBe('done')
  })

  it('duplicate events are idempotent (no regression)', () => {
    let s = reduceOnboardingEvent(fresh(), 'continue')
    s = reduceOnboardingEvent(s, 'detect_success')
    const mid = s.step
    s = reduceOnboardingEvent(s, 'detect_success')
    s = reduceOnboardingEvent(s, 'detect_success')
    expect(s.step).toBe(mid)
    s = reduceOnboardingEvent(s, 'continue') // wrong step — no-op
    expect(s.step).toBe(mid)
  })

  it('skip completes and stops advancement', () => {
    const s = skipOnboarding()
    expect(s.skipped).toBe(true)
    expect(s.step).toBe('done')
    const again = applyOnboardingEvent('detect_success')
    expect(again.step).toBe('done')
  })

  it('reset restarts from rationale', () => {
    applyOnboardingEvent('continue')
    applyOnboardingEvent('detect_success')
    const s = resetOnboarding()
    expect(s.step).toBe('rationale')
    expect(s.trainingCaptureDone).toBe(false)
  })

  it('migrates completed legacy v1 to done', () => {
    localStorage.clear()
    localStorage.setItem(
      'animal-poke-onboarding-v1',
      JSON.stringify({ step: 'done', skipped: false, completedAt: 123 }),
    )
    const s = loadOnboarding()
    expect(s.step).toBe('done')
    expect(s.active).toBe(false)
    expect(localStorage.getItem(ONBOARDING_STORAGE_KEY)).toBeTruthy()
  })

  it('migrates mid-flow legacy scan_tip → train_scan', () => {
    localStorage.clear()
    localStorage.setItem(
      'animal-poke-onboarding-v1',
      JSON.stringify({ step: 'scan_tip', skipped: false, completedAt: null }),
    )
    const s = loadOnboarding()
    expect(s.step).toBe('train_scan')
    expect(s.active).toBe(true)
  })

  it('home_training path flag persists on events', () => {
    let s = reduceOnboardingEvent(fresh(), 'continue', { path: 'home_training' })
    expect(s.path).toBe('home_training')
    s = reduceOnboardingEvent(s, 'detect_success', { path: 'home_training' })
    expect(s.path).toBe('home_training')
    expect(s.step).toBe('select_target')
  })

  it('applyOnboardingEvent persists across load', () => {
    applyOnboardingEvent('continue')
    applyOnboardingEvent('detect_success')
    const s = loadOnboarding()
    expect(s.step).toBe('select_target')
  })
})
