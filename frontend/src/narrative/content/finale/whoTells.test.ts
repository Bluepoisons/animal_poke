import { describe, it, expect } from 'vitest'
import { endingSummary, resolveEnding, validateFinale } from './whoTells'

describe('AP-129 finale multi-ending', () => {
  it('validates and ignores collection rate', () => {
    expect(validateFinale()).toEqual([])
  })

  it('resolves multivocal vs curated vs blank', () => {
    expect(
      resolveEnding({
        choiceOptionIds: ['multivocal_show', 'multivocal_board'],
        noteHandling: 'multivocal',
        collectionRate: 0.99,
      }),
    ).toBe('ending.multivocal')
    expect(
      resolveEnding({
        choiceOptionIds: ['curated_arc', 'managed_board'],
        noteHandling: 'overwrite',
      }),
    ).toBe('ending.curated')
    expect(
      resolveEnding({
        choiceOptionIds: ['blank_first', 'blank_wall', 'preserve_gaps'],
        noteHandling: 'preserve_gaps',
        collectionRate: 0,
      }),
    ).toBe('ending.blank_first')
  })

  it('has human-readable ending summary', () => {
    expect(endingSummary('ending.multivocal').length).toBeGreaterThan(4)
  })
})
