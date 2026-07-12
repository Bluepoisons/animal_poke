/**
 * AP-064 — Camera status → player-facing reason + recovery action.
 * Pure helpers (no DOM) for unit tests and Discover UX.
 */
import type { CameraFacing, CameraStatus } from './useCamera'

export type CameraRecoveryAction =
  | 'retry'
  | 'open_settings'
  | 'use_https'
  | 'switch_facing'
  | 'wait'
  | 'none'

export type CameraGuidance = {
  /** i18n key for short status line */
  reasonKey: string
  /** i18n key for next-step copy */
  nextKey: string
  action: CameraRecoveryAction
  /** Whether viewfinder shows live media (false → training placeholder) */
  livePreview: boolean
  /** Badge when not live — must never look like a detection result */
  placeholderKind: 'none' | 'training' | 'unavailable'
}

/** Map DOMException / MediaError names to CameraStatus + error code. */
export function mapMediaError(name: string): { status: CameraStatus; error: string } {
  switch (name) {
    case 'NotAllowedError':
    case 'PermissionDeniedError':
    case 'SecurityError':
      return { status: 'denied', error: 'permission_denied' }
    case 'NotReadableError':
    case 'TrackStartError':
    case 'AbortError':
      return { status: 'busy', error: 'camera_busy' }
    case 'NotFoundError':
    case 'DevicesNotFoundError':
      return { status: 'unavailable', error: 'no_camera' }
    case 'OverconstrainedError':
    case 'ConstraintNotSatisfiedError':
      return { status: 'unavailable', error: 'overconstrained' }
    case 'TypeError':
      return { status: 'unavailable', error: 'camera_api_unavailable' }
    case 'TrackEnded':
      return { status: 'ended', error: 'track_ended' }
    default:
      return { status: 'unavailable', error: name || 'camera_error' }
  }
}

export function guidanceForStatus(
  status: CameraStatus,
  error?: string,
): CameraGuidance {
  switch (status) {
    case 'idle':
      return {
        reasonKey: 'camera.status.idle',
        nextKey: 'camera.next.idle',
        action: 'retry',
        livePreview: false,
        placeholderKind: 'training',
      }
    case 'requesting':
      return {
        reasonKey: 'camera.status.requesting',
        nextKey: 'camera.next.requesting',
        action: 'wait',
        livePreview: false,
        placeholderKind: 'training',
      }
    case 'ready':
      return {
        reasonKey: 'camera.status.ready',
        nextKey: 'camera.next.ready',
        action: 'switch_facing',
        livePreview: true,
        placeholderKind: 'none',
      }
    case 'denied':
      return {
        reasonKey: 'camera.status.denied',
        nextKey: 'camera.next.denied',
        action: 'open_settings',
        livePreview: false,
        placeholderKind: 'unavailable',
      }
    case 'busy':
      return {
        reasonKey: 'camera.status.busy',
        nextKey: 'camera.next.busy',
        action: 'retry',
        livePreview: false,
        placeholderKind: 'unavailable',
      }
    case 'unavailable':
      return {
        reasonKey:
          error === 'camera_api_unavailable'
            ? 'camera.status.api_unavailable'
            : error === 'overconstrained'
              ? 'camera.status.overconstrained'
              : 'camera.status.unavailable',
        nextKey: 'camera.next.unavailable',
        action: 'retry',
        livePreview: false,
        placeholderKind: 'unavailable',
      }
    case 'stopped':
      return {
        reasonKey: 'camera.status.stopped',
        nextKey: 'camera.next.stopped',
        action: 'retry',
        livePreview: false,
        placeholderKind: 'training',
      }
    case 'insecure':
      return {
        reasonKey: 'camera.status.insecure',
        nextKey: 'camera.next.insecure',
        action: 'use_https',
        livePreview: false,
        placeholderKind: 'unavailable',
      }
    case 'ended':
      return {
        reasonKey: 'camera.status.ended',
        nextKey: 'camera.next.ended',
        action: 'retry',
        livePreview: false,
        placeholderKind: 'training',
      }
    default:
      return {
        reasonKey: 'camera.status.unavailable',
        nextKey: 'camera.next.unavailable',
        action: 'retry',
        livePreview: false,
        placeholderKind: 'unavailable',
      }
  }
}

export function oppositeFacing(facing: CameraFacing): CameraFacing {
  return facing === 'environment' ? 'user' : 'environment'
}

/** True when OS may still recover without leaving the app (retry-able). */
export function isRecoverableInApp(status: CameraStatus): boolean {
  return status === 'busy' || status === 'ended' || status === 'stopped' || status === 'idle' || status === 'unavailable'
}
