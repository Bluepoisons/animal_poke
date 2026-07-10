import { describe, it, expect, beforeEach } from 'vitest'
import {
  FEATURE_FLAGS,
  isFeatureEnabled,
  markFeatureUnavailable,
  isFeatureUnavailableError,
  featureKeyFromPath,
  __resetFeatureFlagsForTests,
} from './featureFlags'
import { ApiError } from '../../api/client'

describe('featureFlags', () => {
  beforeEach(() => {
    __resetFeatureFlagsForTests()
  })

  it('hides unfinished social modules by default', () => {
    expect(FEATURE_FLAGS.dispatch).toBe(false)
    expect(FEATURE_FLAGS.pvp).toBe(false)
    expect(FEATURE_FLAGS.ranking).toBe(false)
    expect(FEATURE_FLAGS.social).toBe(false)
    expect(FEATURE_FLAGS.ops).toBe(false)
  })

  it('markFeatureUnavailable disables isFeatureEnabled', () => {
    // even if static flag were true, runtime mark wins
    markFeatureUnavailable('ranking')
    expect(isFeatureEnabled('ranking')).toBe(false)
    markFeatureUnavailable('pvp')
    expect(isFeatureEnabled('pvp')).toBe(false)
    markFeatureUnavailable('social')
    expect(isFeatureEnabled('social')).toBe(false)
    markFeatureUnavailable('ops')
    expect(isFeatureEnabled('ops')).toBe(false)
  })

  it('detects feature_unavailable ApiError', () => {
    const err = new ApiError(501, {
      error: 'feature unavailable',
      reason_code: 'feature_unavailable',
    })
    expect(isFeatureUnavailableError(err)).toBe(true)
    expect(isFeatureUnavailableError(new Error('nope'))).toBe(false)
  })

  it('maps API paths to feature keys', () => {
    expect(featureKeyFromPath('/api/v1/ranking/daily')).toBe('ranking')
    expect(featureKeyFromPath('/api/v1/pvp/match')).toBe('pvp')
    expect(featureKeyFromPath('/api/v1/social/friends')).toBe('social')
    expect(featureKeyFromPath('/api/v1/ops/metrics-summary')).toBe('ops')
    expect(featureKeyFromPath('/api/v1/geo/city')).toBe(null)
  })
})
