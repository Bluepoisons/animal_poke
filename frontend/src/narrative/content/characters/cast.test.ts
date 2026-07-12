import { describe, it, expect } from 'vitest'
import {
  canKnow,
  getCharacter,
  listCharacters,
  relationsFor,
  season1Cast,
  validateCast,
  voiceBlindCards,
} from './cast'
import { listSeasonChapters } from '../season1/architecture'

describe('AP-117 cast pack', () => {
  it('validates cast structure', () => {
    expect(validateCast()).toEqual([])
  })

  it('has four residents including fictional aide', () => {
    const list = listCharacters()
    expect(list).toHaveLength(4)
    expect(getCharacter('journal_aide')?.fictional).toBe(true)
    expect(list.every((c) => c.arcs.length === 3)).toBe(true)
  })

  it('each character relates to at least two others with multi-chapter shifts', () => {
    for (const c of listCharacters()) {
      expect(relationsFor(c.id).length).toBeGreaterThanOrEqual(2)
    }
  })

  it('knowledge gates block finale spoilers early', () => {
    const order = Object.fromEntries(listSeasonChapters().map((c) => [c.id, c.order])) as Record<
      string,
      number
    >
    expect(canKnow('archivist', 'finale_exhibition', 'prologue.blank_page', order as never)).toBe(
      false,
    )
    expect(canKnow('archivist', 'finale_exhibition', 'finale.who_tells_the_city', order as never)).toBe(
      true,
    )
  })

  it('voice cards are distinguishable without names', () => {
    const cards = voiceBlindCards()
    const markers = cards.map((c) => c.markers.join('|'))
    expect(new Set(markers).size).toBe(cards.length)
  })

  it('pack version present', () => {
    expect(season1Cast.version).toBeTruthy()
  })
})
