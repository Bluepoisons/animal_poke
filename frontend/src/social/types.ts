/** 社交系统 — 好友、分享 */
export type FriendStatus = 'online' | 'offline' | 'inBattle'

export interface Friend {
  id: string
  name: string
  avatar: string
  level: number
  status: FriendStatus
  addedAt: number
  lastActiveAt: number
}

export interface FriendRequest {
  id: string
  fromId: string
  fromName: string
  fromAvatar: string
  sentAt: number
  status: 'pending' | 'accepted' | 'rejected'
}

export interface ShareCard {
  animalId: string
  animalName: string
  rarity: number
  captureDate: string
  captureLocation: string
  shareUrl: string | null
}

/** 生成分享卡片数据 */
export function createShareCard(
  animalId: string,
  animalName: string,
  rarity: number,
  captureDate: string,
  captureLocation: string,
): ShareCard {
  return {
    animalId,
    animalName,
    rarity,
    captureDate,
    captureLocation,
    shareUrl: null,
  }
}

/** 生成好友请求 */
export function createFriendRequest(
  fromId: string,
  fromName: string,
  fromAvatar: string,
): FriendRequest {
  return {
    id: `req-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    fromId,
    fromName,
    fromAvatar,
    sentAt: Date.now(),
    status: 'pending',
  }
}

/** 最大好友数 */
export const MAX_FRIENDS = 200

/** 好友列表是否已满 */
export function isFriendListFull(friends: Friend[]): boolean {
  return friends.length >= MAX_FRIENDS
}

/** 检查是否已是好友 */
export function isAlreadyFriend(friends: Friend[], friendId: string): boolean {
  return friends.some(f => f.id === friendId)
}

/** 分享内容文案模板 */
export function generateShareText(card: ShareCard): string {
  const rarityNames = ['普通', '稀有', '史诗', '传说', '神话']
  const rarityName = rarityNames[Math.min(card.rarity, rarityNames.length - 1)] ?? '普通'
  return `我在 Animal Poke 收集到了 ${rarityName} 级的 ${card.animalName}！🎯`
}
