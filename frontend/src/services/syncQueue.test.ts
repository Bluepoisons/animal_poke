import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { resetDB } from '../db/db'
import {
  enqueueAnimalSync,
  flushSyncQueue,
  buildIdempotencyKey,
  enqueueGeneratedAnimal,
  SYNCING_LEASE_MS,
} from './syncQueue'
import { SyncQueueRepository } from '../db/repositories/sync-queue-repository'
import { __resetAuthForTests } from '../auth/deviceAuth'
import type { GeneratedAnimal } from './capturePipeline'

function seedAuth() {
  localStorage.setItem('ap_device_id', 'dev-1')
  localStorage.setItem('ap_access_token', 'tok')
  localStorage.setItem('ap_token_expires_at', new Date(Date.now() + 3600_000).toISOString())
}

describe('syncQueue', () => {
  beforeEach(async () => {
    await resetDB()
    __resetAuthForTests()
    localStorage.clear()
    seedAuth()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('enqueues with stable idempotency key and dedupes', async () => {
    const payload = {
      uuid: 'u1',
      species: 'cat',
      rarity: 2,
      generated_at: new Date().toISOString(),
      inference_request_id: 'inf-1',
    }
    const a = await enqueueAnimalSync(payload)
    const b = await enqueueAnimalSync(payload)
    expect(a.id).toBe(b.id)
    expect(a.idempotencyKey).toBe(buildIdempotencyKey('u1'))
    const all = await SyncQueueRepository.getAll()
    expect(all).toHaveLength(1)
  })

  it('flush marks synced on 201', async () => {
    await enqueueAnimalSync({
      uuid: 'u2',
      species: 'goose',
      rarity: 3,
      generated_at: new Date().toISOString(),
      inference_request_id: 'inf-2',
    })
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      headers: new Headers({ 'Content-Type': 'application/json' }),
      json: async () => ({ status: 'synced', uuid: 'u2' }),
    })
    vi.stubGlobal('fetch', fetchMock)

    const r = await flushSyncQueue()
    expect(r.synced).toBe(1)
    const item = await SyncQueueRepository.getByIdempotencyKey(buildIdempotencyKey('u2'))
    expect(item?.status).toBe('synced')
    const [, init] = fetchMock.mock.calls[0]
    expect(init.headers['Idempotency-Key']).toBe('sync:animal:u2')
    expect(init.headers.Authorization).toBe('Bearer tok')
  })

  it('treats 409 as synced', async () => {
    await enqueueAnimalSync({
      uuid: 'u3',
      species: 'dog',
      rarity: 1,
      generated_at: new Date().toISOString(),
    })
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 409,
        headers: new Headers({ 'Content-Type': 'application/json' }),
        json: async () => ({ error: 'animal already exists', reason_code: 'duplicate_animal' }),
      }),
    )
    const r = await flushSyncQueue()
    expect(r.synced).toBe(1)
    const item = await SyncQueueRepository.getByIdempotencyKey(buildIdempotencyKey('u3'))
    expect(item?.status).toBe('synced')
  })

  it('enqueueGeneratedAnimal maps fields', async () => {
    const animal: GeneratedAnimal = {
      sessionId: 'sess-9',
      inferenceRequestId: 'inf-9',
      valueInferenceId: 'inf-9',
      species: 'cat',
      analysis: {
        breed: 'Tabby',
        color: 'orange',
        body_type: 'slim',
        quality_score: 7,
        subject_completeness: 8,
        clarity: 7,
        lighting: 6,
        composition: 7,
        pose: 5,
        angle: 5,
      },
      value: {
        rarity: 2,
        hp: 40,
        atk: 12,
        def: 10,
        spd: 14,
        class: 'Assassin',
        element: 'Dark',
        narrative: 'quiet hunter',
      },
    }
    const item = await enqueueGeneratedAnimal(animal)
    expect(item.payload.uuid).toBe('sess-9')
    expect(item.payload.breed).toBe('Tabby')
    expect(item.payload.class).toBe('Assassin')
  })

  it('does not treat inference_invalid 409 as synced', async () => {
    await enqueueAnimalSync({
      uuid: 'u-inf-bad',
      species: 'cat',
      rarity: 1,
      generated_at: new Date().toISOString(),
      inference_request_id: 'bad-inf',
    })
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 409,
        headers: { get: () => 'application/json' },
        json: async () => ({ error: 'invalid or reused inference', reason_code: 'inference_invalid' }),
      }),
    )
    // authedRequest may wrap fetch — if tests use authedRequest path, mock may differ
    const r = await flushSyncQueue()
    const item = await SyncQueueRepository.getByIdempotencyKey(buildIdempotencyKey('u-inf-bad'))
    expect(r.failed).toBe(1)
    expect(item?.status).toBe('failed')
    expect(item?.nextAttemptAt).toBe(Number.MAX_SAFE_INTEGER)
  })

  it('retries an abandoned syncing item with the same idempotency key', async () => {
    const now = Date.now()
    const queued = await enqueueAnimalSync({
      uuid: 'u-stale-syncing',
      species: 'cat',
      rarity: 2,
      generated_at: new Date(now).toISOString(),
      inference_request_id: 'inf-stale',
    })
    await SyncQueueRepository.put({
      ...queued,
      status: 'syncing',
      updatedAt: now - SYNCING_LEASE_MS - 1,
    })
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      headers: new Headers({ 'Content-Type': 'application/json' }),
      json: async () => ({ status: 'synced', uuid: queued.payload.uuid }),
    })
    vi.stubGlobal('fetch', fetchMock)

    const result = await flushSyncQueue(now)

    expect(result).toEqual({ synced: 1, failed: 0 })
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0][1].headers['Idempotency-Key']).toBe(queued.idempotencyKey)
    expect((await SyncQueueRepository.getById(queued.id))?.status).toBe('synced')
  })

  it('does not steal an active syncing lease', async () => {
    const now = Date.now()
    const queued = await enqueueAnimalSync({
      uuid: 'u-active-syncing',
      species: 'dog',
      rarity: 1,
      generated_at: new Date(now).toISOString(),
    })
    await SyncQueueRepository.put({
      ...queued,
      status: 'syncing',
      updatedAt: now - SYNCING_LEASE_MS + 1,
    })
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)

    const result = await flushSyncQueue(now)

    expect(result).toEqual({ synced: 0, failed: 0 })
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
