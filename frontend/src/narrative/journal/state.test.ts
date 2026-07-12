import { describe, it, expect, beforeEach } from 'vitest'
import {
  activeChoiceInfluences,
  completeChapter,
  createInitialJournal,
  currentGoal,
  lastEventSummary,
  loadJournal,
  markMissedWithAlt,
  recordChoice,
  saveJournal,
} from './state'

describe('AP-121 journal state', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('persists last-known-good journal', () => {
    const s = createInitialJournal()
    s.currentChapterId = 'ch01.alley_echo'
    saveJournal(s)
    expect(loadJournal().currentChapterId).toBe('ch01.alley_echo')
  })

  it('exposes current goal and last event in plain language', () => {
    const s = createInitialJournal()
    const goal = currentGoal(s)
    expect(goal.title).toMatch(/空白页|当前/)
    expect(goal.body.length).toBeGreaterThan(4)
    expect(lastEventSummary(s)).toBeTruthy()
  })

  it('records choice idempotently and tracks active influences', () => {
    let s = createInitialJournal()
    s = recordChoice(s, {
      choiceId: 'choice.prologue.keep_both',
      optionId: 'keep_both',
      label: '两行都留着',
      chapterId: 'prologue.blank_page',
      echoIn: 'ch01.alley_echo',
      delayedEcho: 'echo',
    })
    s = recordChoice(s, {
      choiceId: 'choice.prologue.keep_both',
      optionId: 'merge_single',
      label: '合并',
      chapterId: 'prologue.blank_page',
      echoIn: 'ch01.alley_echo',
      delayedEcho: 'echo2',
    })
    expect(s.choiceEchoes).toHaveLength(1)
    expect(s.choiceEchoes[0].optionId).toBe('merge_single')
    expect(activeChoiceInfluences(s)).toHaveLength(1)
  })

  it('missed clues keep alt acquire text', () => {
    let s = createInitialJournal()
    const id = s.clues[0].id
    s = markMissedWithAlt(s, id)
    const c = s.clues.find((x) => x.id === id)!
    expect(c.status).toBe('missed')
    expect(c.altAcquire.length).toBeGreaterThan(0)
    expect(c.altAcquire).not.toMatch(/^\?+$/)
  })

  it('completing chapter unlocks next clues', () => {
    let s = createInitialJournal()
    s = completeChapter(s, 'prologue.blank_page')
    expect(s.completedChapterIds).toContain('prologue.blank_page')
    expect(s.currentChapterId).toBe('ch01.alley_echo')
    expect(s.clues.some((c) => c.chapterId === 'ch01.alley_echo' && c.status === 'available')).toBe(
      true,
    )
  })
})
