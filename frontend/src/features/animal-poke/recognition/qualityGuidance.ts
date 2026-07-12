/**
 * AP-065 — Actionable retake / recognition guidance (not generic "failed").
 */

export type RecognitionVisualState = 'idle' | 'processing' | 'selectable' | 'ready_capture' | 'error' | 'low_confidence'

export type QualityTip = {
  /** i18n key for short title */
  titleKey: string
  /** i18n key for actionable body */
  bodyKey: string
  /** Semantic chip style */
  tone: 'info' | 'warn' | 'error' | 'success'
}

/** Map detect / quality error codes to retake tips. */
export function tipForErrorCode(code: string | null | undefined): QualityTip | null {
  if (!code) return null
  switch (code) {
    case 'frame_too_small':
    case 'quality':
      return {
        titleKey: 'recognition.tip.quality.title',
        bodyKey: 'recognition.tip.quality.body',
        tone: 'warn',
      }
    case 'duplicate_frame':
      return {
        titleKey: 'recognition.tip.duplicate.title',
        bodyKey: 'recognition.tip.duplicate.body',
        tone: 'warn',
      }
    case 'frame_too_large':
      return {
        titleKey: 'recognition.tip.large.title',
        bodyKey: 'recognition.tip.large.body',
        tone: 'info',
      }
    case 'no_animals':
      return {
        titleKey: 'recognition.tip.no_animals.title',
        bodyKey: 'recognition.tip.no_animals.body',
        tone: 'info',
      }
    case 'low_confidence':
    case 'need_select_target':
      return {
        titleKey: 'recognition.tip.low_confidence.title',
        bodyKey: 'recognition.tip.low_confidence.body',
        tone: 'warn',
      }
    case 'offline':
      return {
        titleKey: 'recognition.tip.offline.title',
        bodyKey: 'recognition.tip.offline.body',
        tone: 'error',
      }
    case 'timeout':
      return {
        titleKey: 'recognition.tip.timeout.title',
        bodyKey: 'recognition.tip.timeout.body',
        tone: 'error',
      }
    case 'rate_limited':
      return {
        titleKey: 'recognition.tip.rate.title',
        bodyKey: 'recognition.tip.rate.body',
        tone: 'warn',
      }
    case 'auth_error':
      return {
        titleKey: 'recognition.tip.auth.title',
        bodyKey: 'recognition.tip.auth.body',
        tone: 'error',
      }
    case 'camera_not_ready':
    case 'no_frame':
      return {
        titleKey: 'recognition.tip.camera.title',
        bodyKey: 'recognition.tip.camera.body',
        tone: 'error',
      }
    case 'detect_failed':
    default:
      if (code.startsWith('camera_')) {
        return {
          titleKey: 'recognition.tip.camera.title',
          bodyKey: 'recognition.tip.camera.body',
          tone: 'error',
        }
      }
      return {
        titleKey: 'recognition.tip.generic.title',
        bodyKey: 'recognition.tip.generic.body',
        tone: 'error',
      }
  }
}

/** Static observation tips always available under the viewfinder. */
export const OBSERVATION_TIP_KEYS = [
  'recognition.observe.light',
  'recognition.observe.distance',
  'recognition.observe.complete',
  'recognition.observe.steady',
  'recognition.observe.occlusion',
] as const

export function visualStateForFlow(args: {
  phase: string
  errorCode: string | null
  hasDetections: boolean
  multiSelect: boolean
  targetConfirmed: boolean
  detecting: boolean
}): RecognitionVisualState {
  if (args.detecting || args.phase === 'detecting') {
    if (args.multiSelect) return 'selectable'
    if (args.errorCode && args.errorCode !== 'need_select_target') return 'error'
    if (args.phase === 'detecting' && !args.hasDetections && !args.errorCode) return 'processing'
    if (args.multiSelect || args.errorCode === 'need_select_target') return 'selectable'
  }
  if (args.targetConfirmed || args.phase === 'target_confirmed') return 'ready_capture'
  if (args.phase === 'failed' || (args.errorCode && args.errorCode !== 'need_select_target')) {
    if (args.errorCode === 'low_confidence') return 'low_confidence'
    return 'error'
  }
  return 'idle'
}

/** Confidence band for label styling. */
export function confidenceBand(confidence: number): 'high' | 'mid' | 'low' {
  if (confidence >= 0.85) return 'high'
  if (confidence >= 0.6) return 'mid'
  return 'low'
}
