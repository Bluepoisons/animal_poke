import { describe, it, expect, beforeEach } from 'vitest'
import {
  getConsent,
  grantConsent,
  revokeConsent,
  isConsentGranted,
  verifyAge,
  getAgeVerification,
  isMinorAllowedTime,
  createDeletionRequest,
  isConsentOutdated,
  MINOR_ALLOWED_HOURS,
  getMinorAccountDefaults,
  getAdultAccountDefaults,
  resolveAccountDefaults,
  setStrictMinorDefaults,
} from './index'

describe('compliance', () => {
  beforeEach(() => {
    localStorage.clear()
    setStrictMinorDefaults(true)
  })

  describe('consent', () => {
    it('defaults to pending', () => {
      expect(getConsent().status).toBe('pending')
    })

    it('grant sets status and timestamp', () => {
      const record = grantConsent()
      expect(record.status).toBe('granted')
      expect(record.grantedAt).toBeGreaterThan(0)
      expect(isConsentGranted()).toBe(true)
    })

    it('revoke sets denied status', () => {
      grantConsent()
      revokeConsent()
      expect(getConsent().status).toBe('denied')
      expect(isConsentGranted()).toBe(false)
    })

    it('persists across reads', () => {
      grantConsent()
      expect(getConsent().status).toBe('granted')
      expect(getConsent().grantedAt).toBeGreaterThan(0)
    })

    it('stores photo and location scopes by default', () => {
      const r = grantConsent()
      expect(r.scopes).toEqual(expect.arrayContaining(['photo', 'location']))
      expect(r.version).toBe('v1')
      expect(r.serverSynced).toBe(true)
    })

    it('normalizes legacy local records without scopes', () => {
      localStorage.setItem(
        'animal-poke-consent',
        JSON.stringify({ status: 'granted', grantedAt: 1, version: 'v1' }),
      )
      const r = getConsent()
      expect(r.scopes).toEqual(expect.arrayContaining(['photo', 'location']))
    })
  })

  describe('age verification', () => {
    it('marks under-18 as minor', () => {
      const birthYear = new Date().getFullYear() - 15
      const result = verifyAge(birthYear)
      expect(result.isMinor).toBe(true)
    })

    it('marks 18+ as non-minor', () => {
      const birthYear = new Date().getFullYear() - 25
      const result = verifyAge(birthYear)
      expect(result.isMinor).toBe(false)
    })

    it('persists to localStorage', () => {
      verifyAge(new Date().getFullYear() - 20)
      const stored = getAgeVerification()
      expect(stored).not.toBeNull()
      expect(stored!.isMinor).toBe(false)
    })
  })

  describe('minor time restrictions', () => {
    it('allows play during allowed hours', () => {
      const noon = new Date('2026-07-09T12:00:00')
      expect(isMinorAllowedTime(noon)).toBe(true)
    })

    it('blocks play outside allowed hours', () => {
      const lateNight = new Date('2026-07-09T23:00:00')
      expect(isMinorAllowedTime(lateNight)).toBe(false)
    })

    it('blocks before start hour', () => {
      const earlyMorning = new Date('2026-07-09T06:00:00')
      expect(isMinorAllowedTime(earlyMorning)).toBe(false)
    })

    it('allowed hours are 8-22', () => {
      expect(MINOR_ALLOWED_HOURS.start).toBe(8)
      expect(MINOR_ALLOWED_HOURS.end).toBe(22)
    })
  })

  describe('account defaults (AP-056)', () => {
    it('strict minor disables location and social', () => {
      const d = getMinorAccountDefaults(true)
      expect(d.audience).toBe('minor')
      expect(d.strict).toBe(true)
      expect(d.locationScope).toBe('none')
      expect(d.socialEnabled).toBe(false)
      expect(d.friendsDefault).toBe(false)
      expect(d.shareCaptureDefault).toBe(false)
      expect(d.playHoursStart).toBe(8)
      expect(d.playHoursEnd).toBe(22)
    })

    it('non-strict minor keeps city location and social on', () => {
      const d = getMinorAccountDefaults(false)
      expect(d.locationScope).toBe('city')
      expect(d.socialEnabled).toBe(true)
    })

    it('resolveAccountDefaults uses strict flag for minors', () => {
      setStrictMinorDefaults(true)
      const minor = resolveAccountDefaults(true)
      expect(minor.strict).toBe(true)
      expect(minor.locationScope).toBe('none')
      const adult = resolveAccountDefaults(false)
      expect(adult.audience).toBe('adult')
      expect(getAdultAccountDefaults().socialEnabled).toBe(true)
    })
  })

  describe('data deletion', () => {
    it('creates request with unique ID', () => {
      const req = createDeletionRequest()
      expect(req.requestId).toBeTruthy()
      expect(req.status).toBe('pending')
      expect(req.requestedAt).toBeGreaterThan(0)
    })

    it('creates unique IDs for multiple requests', () => {
      const req1 = createDeletionRequest()
      const req2 = createDeletionRequest()
      expect(req1.requestId).not.toBe(req2.requestId)
    })
  })

  describe('consent version', () => {
    it('is not outdated after fresh grant', () => {
      grantConsent()
      expect(isConsentOutdated()).toBe(false)
    })
  })
})
