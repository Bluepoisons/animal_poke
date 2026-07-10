import { describe, it, expect, vi, beforeEach } from 'vitest'
import { enqueueSync, flushSyncQueue, listSyncQueue, __resetSyncQueueForTests } from './queue'
import * as auth from '../auth/deviceAuth'

describe('sync queue', () => {
  beforeEach(() => {
    __resetSyncQueueForTests()
    vi.restoreAllMocks()
    vi.spyOn(auth, 'getAccessToken').mockResolvedValue('tok')
  })

  it('persists pending items across flush failure then syncs', async () => {
    enqueueSync({ client_uuid: 'u1', species: 'cat' }, 'k1')
    expect(listSyncQueue()).toHaveLength(1)

    let calls = 0
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(async () => {
        calls++
        if (calls === 1) return new Response('err', { status: 500 })
        return new Response(JSON.stringify({ ok: true }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }),
    )

    await flushSyncQueue()
    expect(listSyncQueue()[0].status).toBe('failed')

    // requeue as pending
    const items = listSyncQueue()
    items[0].status = 'pending'
    localStorage.setItem('ap_sync_queue_v1', JSON.stringify(items))

    await flushSyncQueue()
    expect(listSyncQueue()[0].status).toBe('synced')
  })

  it('treats 409 as success (idempotent)', async () => {
    enqueueSync({ client_uuid: 'u2' }, 'k2')
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(new Response('{}', { status: 409 })),
    )
    await flushSyncQueue()
    expect(listSyncQueue()[0].status).toBe('synced')
  })
})
