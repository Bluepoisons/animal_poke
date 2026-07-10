/**
 * Privacy-friendly analytics funnel (AP-057).
 */

export {
  ANALYTICS_SCHEMA_VERSION,
  FUNNEL_EVENT_NAMES,
  FUNNEL_STAGES,
  FORBIDDEN_FIELD_KEYS,
  isFunnelEventName,
  isForbiddenKey,
  stripForbiddenFields,
  assertSchemaVersion,
  type FunnelEventName,
  type FunnelEventProps,
  type AnalyticsEventBase,
  type CoarseLocation,
  type DetectOutcome,
  type GenerateStageName,
} from './schema'

export {
  getOrCreateSessionId,
  getSessionStartedAt,
  rotateSessionId,
  clearSessionForTesting,
} from './session'

export {
  enqueueEvent,
  peekQueue,
  queueSize,
  dequeueEvents,
  clearQueue,
  ANALYTICS_MAX_QUEUE,
} from './queue'

export {
  computeDetectRates,
  computeStageDropOff,
  computePercentiles,
  computeCaptureCallRate,
  countRepeatClicks,
  computeRetention,
  type RateResult,
  type StageDropOff,
  type PercentileResult,
} from './metrics'

export {
  validateExperiment,
  defineExperiment,
  assignVariant,
  shouldStopExperiment,
  type ExperimentDefinition,
  type ExperimentStopCondition,
  type WelfareGuardrails,
  type ExperimentValidationResult,
  type AssignmentResult,
} from './experiment'

export {
  trackEvent,
  buildEvent,
  flushAnalyticsQueue,
  setSampleRate,
  getSampleRate,
  setAnalyticsConsent,
  isAnalyticsAllowed,
  setAnalyticsTransport,
  installAnalyticsOnlineListener,
  onAnalyticsConsentRevoked,
  _resetAnalyticsForTesting,
  type TrackOptions,
  type AnalyticsConsentState,
} from './client'
