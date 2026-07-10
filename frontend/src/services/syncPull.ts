/**
 * 启动/联网时 pull 分页 + 与本地合并（#186）
 */
import { AnimalRepository } from '../db/repositories/animal-repository'
import { authedRequest } from '../auth/deviceAuth'
import type { AnimalRecord } from '../db/types'
import type { CardEntry, RarityTier, SpeciesType } from '../types'

const CURSOR_KEY = 'animal-poke-sync-pull-cursor'

export function loadPullCursor(): number {
  try {
    return Number(localStorage.getItem(CURSOR_KEY) || '0') || 0
  } catch {
    return 0
  }
}

export function savePullCursor(v: number): void {
  try {
    // 空页不得回退到 0
    if (v > 0) localStorage.setItem(CURSOR_KEY, String(v))
  } catch {
    /* ignore */
  }
}

export type PullPage = {
  items: Array<Record<string, unknown>>
  next_version?: number
  next_cursor?: number
  has_more?: boolean
}

function mapServerAnimal(raw: Record<string, unknown>): AnimalRecord {
  const rarityNum = typeof raw.rarity === 'number' ? raw.rarity : 1
  const rarityMap: RarityTier[] = ['common', 'uncommon', 'rare', 'epic', 'legendary']
  const rarity = rarityMap[Math.min(4, Math.max(0, rarityNum - 1))] || 'common'
  const species = (String(raw.species || 'cat') as SpeciesType) || 'cat'
  const id = String(raw.uuid || raw.id || crypto.randomUUID?.() || `pull-${Date.now()}`)
  const entry: CardEntry = {
    id,
    no: String(raw.uuid || id).slice(0, 8),
    rarity,
    species,
    unlocked: !raw.deleted_at && raw.deleted !== true,
    captureDate: String(raw.generated_at || raw.created_at || new Date().toISOString()).slice(0, 10),
    location: String(raw.city || '未知'),
    lat: Number(raw.latitude || 0),
    lng: Number(raw.longitude || 0),
    seed: Number(raw.server_version || Date.now()) % 100000,
  }
  return { ...entry, isUnlocked: entry.unlocked ? 1 : 0 }
}

/** 分页 pull 直到 has_more=false 或空页；空页保持 cursor */
export async function pullAnimalsFromServer(opts?: {
  pageSize?: number
  maxPages?: number
}): Promise<{ pulled: number; cursor: number }> {
  const pageSize = opts?.pageSize ?? 50
  const maxPages = opts?.maxPages ?? 20
  let cursor = loadPullCursor()
  let pulled = 0

  for (let page = 0; page < maxPages; page++) {
    const data = await authedRequest<PullPage>({
      method: 'GET',
      path: `/api/v1/sync/animals?since_version=${encodeURIComponent(String(cursor))}&limit=${pageSize}`,
      allowRetry: true,
      timeoutMs: 20_000,
    })
    const items = Array.isArray(data.items) ? data.items : []
    if (items.length === 0) {
      // 空页：不把 cursor 重置为 0
      break
    }

    for (const raw of items) {
      const rec = mapServerAnimal(raw)
      if (raw.deleted_at || raw.deleted === true || raw.tombstone === true) {
        // tombstone：本地删除
        try {
          await AnimalRepository.delete(rec.id)
        } catch {
          /* repo may not have delete - mark locked */
          try {
            const existing = await AnimalRepository.getById(rec.id)
            if (existing) {
              await AnimalRepository.add({ ...existing, unlocked: false, isUnlocked: 0 })
            }
          } catch {
            /* ignore */
          }
        }
      } else {
        const existing = await AnimalRepository.getById(rec.id)
        if (!existing) {
          await AnimalRepository.add(rec)
        }
      }
      pulled += 1
      const ver = Number(raw.server_version || raw.ServerVersion || 0)
      if (ver > cursor) cursor = ver
    }

    const next = Number(data.next_version ?? data.next_cursor ?? cursor)
    if (next > cursor) cursor = next
    savePullCursor(cursor)

    if (data.has_more === false) break
    if (items.length < pageSize) break
  }

  savePullCursor(cursor)
  return { pulled, cursor }
}
