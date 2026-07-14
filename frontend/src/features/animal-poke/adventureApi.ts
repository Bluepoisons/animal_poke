import { authedRequest } from '../../auth/deviceAuth'

export type AdventureThemeId =
  | 'mistwood'
  | 'sky_ruins'
  | 'tide_isles'
  | 'starlight_city'
  | 'crystal_caves'
  | 'dream_garden'

export type AdventureChoice = {
  id: 'courage' | 'curiosity' | 'kindness'
  label: string
  description: string
}

export type AdventureStory = {
  adventure_id: string
  theme: AdventureThemeId
  title: string
  location: string
  opening: string
  encounter_title: string
  encounter: string
  companion_line: string
  choices: AdventureChoice[]
  fiction: true
  disclaimer: string
  source: 'ai' | 'mock' | 'template'
  degraded?: boolean
  reason_code?: string
  prompt_version: string
}

export type AdventureSouvenir = {
  name: string
  description: string
}

export type CompanionSnapshot = {
  animal_uuid: string
  bond_xp: number
  bond_level: number
  decor_stage: number
  title?: string
}

export type AdventureCompletion = {
  adventure_id: string
  status: 'completed'
  choice: AdventureChoice & { outcome?: string; bond_delta?: number }
  outcome: string
  souvenir: AdventureSouvenir
  companion?: CompanionSnapshot
  unlocked_nodes?: string[]
  idempotent: boolean
}

export type AdventureHistoryItem = {
  adventure_id: string
  animal_uuid: string
  theme: AdventureThemeId
  title: string
  status: 'generated' | 'completed'
  choice_id?: string
  outcome?: string
  souvenir?: string
  bond_delta: number
  created_at: string
  completed_at?: string
}

export function newAdventureOperationId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  return `adventure-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

function assertAdventureStory(raw: AdventureStory): AdventureStory {
  if (!raw?.adventure_id || !raw.title || !raw.opening || !raw.encounter || raw.choices?.length !== 3) {
    throw new Error('adventure_response_invalid')
  }
  return raw
}

export async function generateAdventure(
  animalUUID: string,
  theme: AdventureThemeId,
  operationId: string,
  signal?: AbortSignal,
): Promise<AdventureStory> {
  const story = await authedRequest<AdventureStory>({
    method: 'POST',
    path: '/api/v1/adventures',
    body: JSON.stringify({ animal_uuid: animalUUID, theme, operation_id: operationId }),
    idempotencyKey: operationId,
    allowRetry: true,
    timeoutMs: 55_000,
    signal,
  })
  return assertAdventureStory(story)
}

export async function completeAdventure(
  adventureId: string,
  choiceId: AdventureChoice['id'],
): Promise<AdventureCompletion> {
  return authedRequest<AdventureCompletion>({
    method: 'POST',
    path: `/api/v1/adventures/${encodeURIComponent(adventureId)}/choices`,
    body: JSON.stringify({ choice_id: choiceId }),
    idempotencyKey: `adventure-choice-${adventureId}`,
    allowRetry: true,
    timeoutMs: 20_000,
  })
}

export async function fetchAdventureHistory(animalUUID: string): Promise<AdventureHistoryItem[]> {
  const response = await authedRequest<{ items?: AdventureHistoryItem[] }>({
    method: 'GET',
    path: `/api/v1/adventures?animal_uuid=${encodeURIComponent(animalUUID)}&limit=6`,
    allowRetry: true,
  })
  return Array.isArray(response.items) ? response.items : []
}

export async function fetchAdventureCompanion(animalUUID: string): Promise<CompanionSnapshot> {
  const response = await authedRequest<{ companion: CompanionSnapshot }>({
    method: 'GET',
    path: `/api/v1/growth/companions/${encodeURIComponent(animalUUID)}`,
    allowRetry: true,
  })
  return response.companion
}
