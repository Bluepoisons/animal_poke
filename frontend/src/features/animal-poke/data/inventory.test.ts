import { describe, it, expect } from 'vitest'
import { inventoryItems, LEGACY_ITEM_ID_MAP } from './inventory'
import { ITEM_DEFS } from '../../../shop/constants'

describe('store inventory ids', () => {
  it('uses canonical shop ItemIds', () => {
    for (const item of inventoryItems) {
      expect(ITEM_DEFS[item.id as keyof typeof ITEM_DEFS]).toBeTruthy()
      expect(item.id.includes('-')).toBe(false)
    }
  })

  it('maps legacy demo ids', () => {
    expect(LEGACY_ITEM_ID_MAP['toy-ball']).toBe('toy_ball')
    expect(LEGACY_ITEM_ID_MAP['advanced-ball']).toBe('premium_toy_ball')
    expect(LEGACY_ITEM_ID_MAP['energy-potion']).toBe('stamina_potion')
  })
})
