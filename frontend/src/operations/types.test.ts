import { describe, it, expect } from 'vitest'
import { getMockAnnouncements, getMockEvents, getMockDashboard, isEventActive, markAnnouncementRead } from './types'

describe('operations', () => {
  it('generates dashboard metrics', () => {
    const data = getMockDashboard()
    expect(data.metrics.length).toBeGreaterThan(0)
    expect(data.metrics[0].label).toBeTruthy()
  })

  it('generates announcements', () => {
    const anns = getMockAnnouncements()
    expect(anns.length).toBeGreaterThan(0)
    expect(anns[0].title).toBeTruthy()
  })

  it('marks announcement as read', () => {
    const anns = getMockAnnouncements()
    const updated = markAnnouncementRead(anns, anns[0].id)
    expect(updated[0].isRead).toBe(true)
  })

  it('checks if event is active', () => {
    const events = getMockEvents()
    expect(isEventActive(events[0])).toBe(true)
  })

  it('returns false for expired event', () => {
    const events = getMockEvents()
    const futureTime = events[0].endDate + 1000
    expect(isEventActive(events[0], futureTime)).toBe(false)
  })
})
