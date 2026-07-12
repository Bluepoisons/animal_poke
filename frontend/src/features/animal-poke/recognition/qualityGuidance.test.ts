import { describe, it, expect } from 'vitest'
import {
  tipForErrorCode,
  visualStateForFlow,
  confidenceBand,
  OBSERVATION_TIP_KEYS,
} from './qualityGuidance'

describe('tipForErrorCode', () => {
  it('maps quality / duplicate / no_animals to actionable tips', () => {
    expect(tipForErrorCode('frame_too_small')?.tone).toBe('warn')
    expect(tipForErrorCode('duplicate_frame')?.titleKey).toContain('duplicate')
    expect(tipForErrorCode('no_animals')?.bodyKey).toContain('no_animals')
  })

  it('errors never use success tone', () => {
    for (const code of [
      'detect_failed',
      'timeout',
      'offline',
      'auth_error',
      'camera_not_ready',
      'rate_limited',
    ]) {
      const tip = tipForErrorCode(code)
      expect(tip).toBeTruthy()
      expect(tip!.tone).not.toBe('success')
    }
  })
})

describe('visualStateForFlow', () => {
  it('processing while detecting with no results', () => {
    expect(
      visualStateForFlow({
        phase: 'detecting',
        errorCode: null,
        hasDetections: false,
        multiSelect: false,
        targetConfirmed: false,
        detecting: true,
      }),
    ).toBe('processing')
  })

  it('selectable for multi-target', () => {
    expect(
      visualStateForFlow({
        phase: 'detecting',
        errorCode: 'need_select_target',
        hasDetections: true,
        multiSelect: true,
        targetConfirmed: false,
        detecting: false,
      }),
    ).toBe('selectable')
  })

  it('ready_capture when target confirmed — not error green confusion', () => {
    expect(
      visualStateForFlow({
        phase: 'target_confirmed',
        errorCode: null,
        hasDetections: true,
        multiSelect: false,
        targetConfirmed: true,
        detecting: false,
      }),
    ).toBe('ready_capture')
  })

  it('failed phase is error (never success)', () => {
    expect(
      visualStateForFlow({
        phase: 'failed',
        errorCode: 'no_animals',
        hasDetections: false,
        multiSelect: false,
        targetConfirmed: false,
        detecting: false,
      }),
    ).toBe('error')
  })
})

describe('confidenceBand', () => {
  it('bands', () => {
    expect(confidenceBand(0.9)).toBe('high')
    expect(confidenceBand(0.7)).toBe('mid')
    expect(confidenceBand(0.4)).toBe('low')
  })
})

describe('observation tips', () => {
  it('covers light distance complete steady occlusion', () => {
    expect(OBSERVATION_TIP_KEYS.length).toBeGreaterThanOrEqual(5)
  })
})
