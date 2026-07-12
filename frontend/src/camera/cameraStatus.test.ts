import { describe, it, expect } from 'vitest'
import {
  mapMediaError,
  guidanceForStatus,
  oppositeFacing,
  isRecoverableInApp,
} from './cameraStatus'

describe('mapMediaError (DOMException table)', () => {
  const cases: Array<[string, string, string]> = [
    ['NotAllowedError', 'denied', 'permission_denied'],
    ['PermissionDeniedError', 'denied', 'permission_denied'],
    ['SecurityError', 'denied', 'permission_denied'],
    ['NotReadableError', 'busy', 'camera_busy'],
    ['TrackStartError', 'busy', 'camera_busy'],
    ['AbortError', 'busy', 'camera_busy'],
    ['NotFoundError', 'unavailable', 'no_camera'],
    ['DevicesNotFoundError', 'unavailable', 'no_camera'],
    ['OverconstrainedError', 'unavailable', 'overconstrained'],
    ['ConstraintNotSatisfiedError', 'unavailable', 'overconstrained'],
    ['TypeError', 'unavailable', 'camera_api_unavailable'],
    ['TrackEnded', 'ended', 'track_ended'],
    ['WeirdError', 'unavailable', 'WeirdError'],
    ['', 'unavailable', 'camera_error'],
  ]

  it.each(cases)('%s → %s / %s', (name, status, error) => {
    expect(mapMediaError(name)).toEqual({ status, error })
  })
})

describe('guidanceForStatus', () => {
  it('marks ready as live preview with switch action', () => {
    const g = guidanceForStatus('ready')
    expect(g.livePreview).toBe(true)
    expect(g.placeholderKind).toBe('none')
    expect(g.action).toBe('switch_facing')
  })

  it('denied → open_settings, unavailable placeholder', () => {
    const g = guidanceForStatus('denied')
    expect(g.action).toBe('open_settings')
    expect(g.livePreview).toBe(false)
    expect(g.placeholderKind).toBe('unavailable')
  })

  it('insecure → use_https', () => {
    expect(guidanceForStatus('insecure').action).toBe('use_https')
  })

  it('ended / busy → retry + training/unavailable', () => {
    expect(guidanceForStatus('ended').action).toBe('retry')
    expect(guidanceForStatus('busy').action).toBe('retry')
    expect(guidanceForStatus('ended').placeholderKind).toBe('training')
  })

  it('never treats non-ready as live detection frame', () => {
    const statuses = [
      'idle',
      'requesting',
      'denied',
      'unavailable',
      'busy',
      'stopped',
      'insecure',
      'ended',
    ] as const
    for (const s of statuses) {
      const g = guidanceForStatus(s)
      expect(g.livePreview).toBe(false)
      expect(g.placeholderKind).not.toBe('none')
    }
  })
})

describe('facing helpers', () => {
  it('oppositeFacing toggles', () => {
    expect(oppositeFacing('environment')).toBe('user')
    expect(oppositeFacing('user')).toBe('environment')
  })

  it('isRecoverableInApp', () => {
    expect(isRecoverableInApp('busy')).toBe(true)
    expect(isRecoverableInApp('ended')).toBe(true)
    expect(isRecoverableInApp('denied')).toBe(false)
    expect(isRecoverableInApp('insecure')).toBe(false)
  })
})
