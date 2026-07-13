import type { GeneratedAnimal } from '../services/capturePipeline'
import { normalizeSpeciesId, type RarityTier } from '../types'
import type { AnimalRecord } from './types'

const RARITY_TIERS: readonly RarityTier[] = [
  'common',
  'uncommon',
  'rare',
  'epic',
  'legendary',
]

function finiteNumber(value: unknown, fallback: number): number {
  const number = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(number) ? number : fallback
}

function captureDate(timestamp: number): string {
  const date = new Date(timestamp)
  return Number.isNaN(date.getTime())
    ? new Date().toISOString().slice(0, 10)
    : date.toISOString().slice(0, 10)
}

function fallbackId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `animal-${Date.now()}`
}

/** Normalize API rarity values (1-5) and already-normalized local tiers. */
export function rarityValueToTier(value: unknown): RarityTier {
  if (typeof value === 'string' && (RARITY_TIERS as readonly string[]).includes(value)) {
    return value as RarityTier
  }
  const numeric = Math.round(finiteNumber(value, 1))
  const index = Math.min(RARITY_TIERS.length - 1, Math.max(0, numeric - 1))
  return RARITY_TIERS[index]
}

export interface GeneratedAnimalRecordOptions {
  capturedAt?: number
  location?: string
  coords?: { lat?: number; lng?: number }
  seed?: number
}

/** Convert a completed capture pipeline result into the canonical IndexedDB record. */
export function generatedAnimalToRecord(
  animal: GeneratedAnimal,
  options: GeneratedAnimalRecordOptions = {},
): AnimalRecord {
  const capturedAt = finiteNumber(options.capturedAt, Date.now())
  const latitude = finiteNumber(options.coords?.lat, 0)
  const longitude = finiteNumber(options.coords?.lng, 0)
  const id = animal.sessionId

  return {
    id,
    uuid: id,
    no: id.slice(0, 8),
    species: normalizeSpeciesId(animal.species),
    speciesLabelZh: animal.speciesLabelZh,
    rarity: rarityValueToTier(animal.value.rarity),
    unlocked: true,
    isUnlocked: 1,
    captureDate: captureDate(capturedAt),
    location: options.location?.trim() || '未知',
    lat: latitude,
    lng: longitude,
    seed: Math.round(finiteNumber(options.seed, capturedAt)) % 100000,
    isNew: true,
    nickname: undefined,
    breed: animal.analysis.breed,
    hp: animal.value.hp,
    atk: animal.value.atk,
    def: animal.value.def,
    spd: animal.value.spd,
    className: animal.value.class,
    element: animal.value.element,
    narrative: animal.value.narrative,
    photoDataUrl: animal.photoDataUrl,
    fiction: animal.value.fiction ?? true,
    disclaimer: animal.value.disclaimer ?? '虚构花絮，非真实个体传记',
    layer: animal.value.layer ?? 'fictional_vignette',
    capturedAt,
    inferenceRequestId: animal.valueInferenceId || animal.inferenceRequestId,
    synced: false,
  }
}

/** Map a sync-pull payload with the same local record semantics as a new capture. */
export function serverAnimalToRecord(raw: Record<string, unknown>): AnimalRecord {
  const id = String(raw.uuid || raw.id || fallbackId())
  const generatedAt = String(raw.generated_at || raw.created_at || new Date().toISOString())
  const unlocked = !raw.deleted_at && raw.deleted !== true && raw.tombstone !== true

  return {
    id,
    uuid: String(raw.uuid || id),
    no: String(raw.uuid || id).slice(0, 8),
    rarity: rarityValueToTier(raw.rarity),
    species: normalizeSpeciesId(raw.species),
    speciesLabelZh: String(raw.species_label_zh || raw.speciesLabelZh || '').trim() || undefined,
    unlocked,
    isUnlocked: unlocked ? 1 : 0,
    captureDate: generatedAt.slice(0, 10),
    location: String(raw.city || '未知'),
    lat: finiteNumber(raw.latitude, 0),
    lng: finiteNumber(raw.longitude, 0),
    seed: Math.round(finiteNumber(raw.server_version, Date.now())) % 100000,
    nickname: String(raw.nickname || '').trim() || undefined,
    breed: String(raw.breed || '').trim() || undefined,
    hp: finiteNumber(raw.hp, 0) || undefined,
    atk: finiteNumber(raw.atk, 0) || undefined,
    def: finiteNumber(raw.def, 0) || undefined,
    spd: finiteNumber(raw.spd, 0) || undefined,
    className: String(raw.class || '').trim() || undefined,
    element: String(raw.element || '').trim() || undefined,
    capturedAt: Date.parse(generatedAt) || undefined,
    inferenceRequestId: String(raw.inference_request_id || '').trim() || undefined,
    synced: true,
  }
}
