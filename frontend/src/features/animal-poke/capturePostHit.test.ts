import { describe, it, expect, beforeEach } from 'vitest'
import {
  createPostHitTask,
  isRevealAllowed,
  loadPostHitTask,
  needsPipelineWork,
  savePostHitTask,
  stageLabel,
} from './capturePostHit'

describe('capturePostHit (AP-062)', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  it('persists and reloads task by attempt id', () => {
    const t = createPostHitTask('a1', 'cat')
    t.stage = 'analyzing'
    t.staminaConsumed = true
    savePostHitTask(t)
    const loaded = loadPostHitTask('a1')
    expect(loaded?.stage).toBe('analyzing')
    expect(loaded?.staminaConsumed).toBe(true)
  })

  it('reveal only after save (completed or pending sync)', () => {
    expect(isRevealAllowed('hit')).toBe(false)
    expect(isRevealAllowed('analyzing')).toBe(false)
    expect(isRevealAllowed('generating')).toBe(false)
    expect(isRevealAllowed('saving')).toBe(false)
    expect(isRevealAllowed('completed')).toBe(true)
    expect(isRevealAllowed('saved_pending_sync')).toBe(true)
    expect(stageLabel('saved_pending_sync')).toContain('待同步')
  })

  it('needsPipelineWork for in-flight stages only', () => {
    const t = createPostHitTask('a2', 'dog')
    expect(needsPipelineWork(t)).toBe(false)
    t.stage = 'hit'
    expect(needsPipelineWork(t)).toBe(true)
    t.stage = 'completed'
    expect(needsPipelineWork(t)).toBe(false)
  })
})
