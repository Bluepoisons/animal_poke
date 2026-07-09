import { describe, it, expect } from 'vitest'
import {
  createShareCard,
  createFriendRequest,
  isFriendListFull,
  isAlreadyFriend,
  generateShareText,
  MAX_FRIENDS,
  type Friend,
} from './types'

describe('social', () => {
  it('creates share card with correct data', () => {
    const card = createShareCard('c001', '橘猫', 2, '2026-07-09', '北京·朝阳')
    expect(card.animalId).toBe('c001')
    expect(card.animalName).toBe('橘猫')
    expect(card.rarity).toBe(2)
    expect(card.shareUrl).toBeNull()
  })

  it('creates friend request with unique ID', () => {
    const req1 = createFriendRequest('u1', 'Alice', '🐱')
    const req2 = createFriendRequest('u1', 'Alice', '🐱')
    expect(req1.id).not.toBe(req2.id)
    expect(req1.status).toBe('pending')
    expect(req1.fromName).toBe('Alice')
  })

  it('checks friend list capacity', () => {
    const empty: Friend[] = []
    expect(isFriendListFull(empty)).toBe(false)
    const full: Friend[] = Array(MAX_FRIENDS).fill(null).map((_, i) => ({ id: `f${i}`, name: `F${i}`, avatar: '', level: 1, status: 'offline' as const, addedAt: 0, lastActiveAt: 0 }))
    expect(isFriendListFull(full)).toBe(true)
  })

  it('checks if already friend', () => {
    const friends: Friend[] = [{ id: 'u1', name: 'A', avatar: '', level: 1, status: 'offline', addedAt: 0, lastActiveAt: 0 }]
    expect(isAlreadyFriend(friends, 'u1')).toBe(true)
    expect(isAlreadyFriend(friends, 'u2')).toBe(false)
  })

  it('generates share text with rarity name', () => {
    const card = createShareCard('c001', '橘猫', 4, '2026-07-09', '北京')
    const text = generateShareText(card)
    expect(text).toContain('橘猫')
    expect(text).toContain('神话')
  })
})
