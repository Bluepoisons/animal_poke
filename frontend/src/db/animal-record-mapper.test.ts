import { describe, expect, it } from 'vitest'
import type { GeneratedAnimal } from '../services/capturePipeline'
import {
  generatedAnimalToRecord,
  rarityValueToTier,
  serverAnimalToRecord,
} from './animal-record-mapper'

function generatedAnimal(rarity = 3): GeneratedAnimal {
  return {
    sessionId: 'capture-123456789',
    inferenceRequestId: 'value-inf-1',
    valueInferenceId: 'value-inf-1',
    species: 'cat',
    analysis: {
      breed: 'Tabby',
      color: 'orange',
      body_type: 'normal',
      quality_score: 8,
      subject_completeness: 9,
      clarity: 8,
      lighting: 7,
      composition: 8,
      pose: 7,
      angle: 6,
    },
    value: {
      rarity,
      hp: 55,
      atk: 16,
      def: 14,
      spd: 20,
      class: 'Ranger',
      element: 'Wind',
      narrative: 'A test companion.',
    },
  }
}

describe('animal record mapping', () => {
  it.each([
    [1, 'common'],
    [2, 'uncommon'],
    [3, 'rare'],
    [4, 'epic'],
    [5, 'legendary'],
  ] as const)('maps numeric rarity %s to %s', (numeric, tier) => {
    expect(rarityValueToTier(numeric)).toBe(tier)
  })

  it('creates a complete unlocked record from a generated animal', () => {
    const record = generatedAnimalToRecord(generatedAnimal(), {
      capturedAt: Date.UTC(2026, 6, 12, 3, 4, 5),
      location: '宁波',
      latitude: 29.87,
      longitude: 121.55,
      seed: 42,
    })

    expect(record).toMatchObject({
      id: 'capture-123456789',
      uuid: 'capture-123456789',
      no: 'capture-',
      species: 'cat',
      rarity: 'rare',
      unlocked: true,
      isUnlocked: 1,
      captureDate: '2026-07-12',
      location: '宁波',
      lat: 29.87,
      lng: 121.55,
      seed: 42,
      isNew: true,
      breed: 'Tabby',
      synced: false,
      inferenceRequestId: 'value-inf-1',
    })
  })

  it('uses the same rarity and unlock semantics for sync-pull records', () => {
    const record = serverAnimalToRecord({
      uuid: 'server-1',
      species: 'dog',
      rarity: 5,
      generated_at: '2026-07-10T12:00:00.000Z',
      city: '杭州',
      latitude: 30.27,
      longitude: 120.15,
      server_version: 17,
    })

    expect(record).toMatchObject({
      id: 'server-1',
      rarity: 'legendary',
      unlocked: true,
      isUnlocked: 1,
      captureDate: '2026-07-10',
      location: '杭州',
      seed: 17,
    })
  })
})
