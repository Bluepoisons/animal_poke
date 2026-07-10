/**
 * Offline analytics event queue (localStorage, capped).
 */

import { safeGetItem, safeSetItem, safeRemoveItem } from '../utils/safeStorage'
import type { AnalyticsEventBase } from './schema'

const QUEUE_KEY = 'ap_analytics_queue'
const MAX_QUEUE = 50

let memoryQueue: AnalyticsEventBase[] | null = null

function load(): AnalyticsEventBase[] {
  if (memoryQueue) return memoryQueue
  const raw = safeGetItem<AnalyticsEventBase[] | null>(QUEUE_KEY, null)
  memoryQueue = Array.isArray(raw) ? raw.slice(-MAX_QUEUE) : []
  return memoryQueue
}

function persist(q: AnalyticsEventBase[]): void {
  memoryQueue = q.slice(-MAX_QUEUE)
  safeSetItem(QUEUE_KEY, memoryQueue)
}

export function enqueueEvent(event: AnalyticsEventBase): void {
  const q = load()
  // Dedupe by event_id
  if (q.some((e) => e.event_id === event.event_id)) return
  q.push(event)
  persist(q)
}

export function peekQueue(): AnalyticsEventBase[] {
  return [...load()]
}

export function queueSize(): number {
  return load().length
}

/** Remove successfully sent events by id. */
export function dequeueEvents(eventIds: string[]): void {
  if (eventIds.length === 0) return
  const drop = new Set(eventIds)
  const next = load().filter((e) => !drop.has(e.event_id))
  persist(next)
}

export function replaceQueue(events: AnalyticsEventBase[]): void {
  persist(events)
}

export function clearQueue(): void {
  memoryQueue = []
  safeRemoveItem(QUEUE_KEY)
}

export function _resetQueueForTesting(): void {
  memoryQueue = null
  safeRemoveItem(QUEUE_KEY)
}

export const ANALYTICS_MAX_QUEUE = MAX_QUEUE
