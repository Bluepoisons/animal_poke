import type {
  ChoiceOption,
  PresentationCheckpoint,
  PresentationSegment,
  PresentationSequence,
  RuntimeSnapshot,
  RuntimeStatus,
} from './types'

const DEFAULT_MAX_SEC = 45
const DEFAULT_AUTO_MS = 3500

export interface RuntimeOptions {
  reducedMotion?: boolean
  muted?: boolean
  auto?: boolean
  /** Persist checkpoint */
  save?: (cp: PresentationCheckpoint) => void
  load?: (sequenceId: string) => PresentationCheckpoint | null
  /** Called once per choice id (idempotent) */
  onChoice?: (segmentId: string, choiceId: string) => void
  now?: () => number
}

/**
 * Manifest-driven presentation runtime: skip/auto/log/rewind/pause/resume/checkpoint.
 * No rewards/side effects here — callers own idempotent choice submission.
 */
export class PresentationRuntime {
  private seq: PresentationSequence
  private index = 0
  private status: RuntimeStatus = 'idle'
  private auto: boolean
  private muted: boolean
  private reducedMotion: boolean
  private log: string[] = []
  private failedAssets = new Set<string>()
  private choiceSubmitted = new Map<string, string>()
  private startedAt = 0
  private pausedAccum = 0
  private pauseStarted = 0
  private opts: RuntimeOptions
  private now: () => number

  constructor(seq: PresentationSequence, opts: RuntimeOptions = {}) {
    if (!seq.segments?.length) throw new Error('sequence requires segments')
    this.seq = seq
    this.opts = opts
    this.auto = opts.auto ?? false
    this.muted = opts.muted ?? false
    this.reducedMotion = opts.reducedMotion ?? false
    this.now = opts.now ?? (() => Date.now())
  }

  /** Start or restore from checkpoint. */
  start(fromCheckpoint = true): RuntimeSnapshot {
    if (fromCheckpoint && this.opts.load) {
      const cp = this.opts.load(this.seq.id)
      if (cp && cp.sequenceVersion === this.seq.version) {
        this.index = Math.min(Math.max(0, cp.index), this.seq.segments.length - 1)
        this.log = [...cp.log]
        if (cp.choices) {
          for (const [sid, cid] of Object.entries(cp.choices)) this.choiceSubmitted.set(sid, cid)
        } else if (cp.choiceId && cp.segmentId) {
          this.choiceSubmitted.set(cp.segmentId, cp.choiceId)
        }
      } else {
        this.index = this.entryIndex()
      }
    } else {
      this.index = this.entryIndex()
    }
    this.startedAt = this.now()
    this.pausedAccum = 0
    this.status = this.current()?.choice ? 'awaiting_choice' : 'playing'
    this.persist()
    return this.snapshot()
  }

  current(): PresentationSegment | null {
    return this.seq.segments[this.index] ?? null
  }

  snapshot(): RuntimeSnapshot {
    const seg = this.current()
    return {
      status: this.status,
      sequenceId: this.seq.id,
      segment: seg,
      index: this.index,
      total: this.seq.segments.length,
      auto: this.auto,
      muted: this.muted,
      reducedMotion: this.reducedMotion,
      log: [...this.log],
      failedAssets: [...this.failedAssets],
      checkpoint: this.buildCheckpoint(),
      elapsedMs: this.elapsed(),
    }
  }

  setAuto(on: boolean) {
    this.auto = on
  }

  setMuted(on: boolean) {
    this.muted = on
  }

  pause() {
    if (this.status === 'playing' || this.status === 'awaiting_choice') {
      this.pauseStarted = this.now()
      this.status = 'paused'
    }
  }

  resume() {
    if (this.status !== 'paused') return this.snapshot()
    if (this.pauseStarted) {
      this.pausedAccum += this.now() - this.pauseStarted
      this.pauseStarted = 0
    }
    this.status = this.current()?.choice ? 'awaiting_choice' : 'playing'
    return this.snapshot()
  }

  /** Skip current segment → append summary; cannot skip past unanswered critical choice without choosing. */
  skip(): RuntimeSnapshot {
    const seg = this.current()
    if (!seg) return this.snapshot()
    if (seg.choice && !this.choiceSubmitted.has(seg.id)) {
      this.status = 'awaiting_choice'
      return this.snapshot()
    }
    this.appendSummary(seg)
    return this.advance()
  }

  /** Rewind one segment (keeps log tail). */
  rewind(): RuntimeSnapshot {
    if (this.index <= 0) return this.snapshot()
    this.index -= 1
    this.status = this.current()?.choice && !this.choiceSubmitted.has(this.current()!.id)
      ? 'awaiting_choice'
      : 'playing'
    this.persist()
    return this.snapshot()
  }

  /** Record choice (idempotent on same choiceId). */
  choose(choiceId: string): RuntimeSnapshot {
    const seg = this.current()
    if (!seg?.choice) return this.snapshot()
    const opt = seg.choice.options.find((o) => o.id === choiceId)
    if (!opt) return this.snapshot()
    const prev = this.choiceSubmitted.get(seg.id)
    if (prev && prev !== choiceId) {
      // already chose differently — ignore (no re-submit)
      return this.snapshot()
    }
    if (!prev) {
      this.choiceSubmitted.set(seg.id, choiceId)
      this.opts.onChoice?.(seg.id, choiceId)
    }
    this.appendSummary(seg, opt)
    if (opt.next) {
      const ni = this.seq.segments.findIndex((s) => s.id === opt.next)
      if (ni >= 0) {
        this.index = ni
        this.status = this.current()?.choice && !this.choiceSubmitted.has(this.current()!.id)
          ? 'awaiting_choice'
          : 'playing'
        this.persist()
        return this.snapshot()
      }
    }
    return this.advance()
  }

  /** Mark asset failure → degraded mode for that URL. */
  markAssetFailed(url: string) {
    if (url) this.failedAssets.add(url)
    if (this.failedAssets.size > 0 && this.status === 'playing') {
      this.status = 'degraded'
    }
  }

  /** Whether segment should render static/text fallback for a URL. */
  isDegraded(url?: string): boolean {
    if (!url) return true
    return this.failedAssets.has(url)
  }

  /** Auto-advance tick; returns true if advanced. Respects 45s cap and choices. */
  tickAuto(deltaMs: number): RuntimeSnapshot {
    if (!this.auto || this.reducedMotion) return this.snapshot()
    if (this.status !== 'playing') return this.snapshot()
    const seg = this.current()
    if (!seg || seg.choice) return this.snapshot()
    const cap = (seg.maxSeconds ?? DEFAULT_MAX_SEC) * 1000
    if (this.elapsed() > cap) return this.skip()
    const need = seg.autoMs ?? DEFAULT_AUTO_MS
    if (deltaMs >= need) return this.skip()
    return this.snapshot()
  }

  /** Readable full log / skip summary. */
  fullSummary(): string {
    return this.log.join('\n')
  }

  private advance(): RuntimeSnapshot {
    if (this.index >= this.seq.segments.length - 1) {
      this.status = 'completed'
      this.persist()
      return this.snapshot()
    }
    this.index += 1
    const next = this.current()
    this.status = next?.choice && !this.choiceSubmitted.has(next.id) ? 'awaiting_choice' : 'playing'
    this.persist()
    return this.snapshot()
  }

  private appendSummary(seg: PresentationSegment, choice?: ChoiceOption) {
    const line = choice ? `${seg.summary} → ${choice.label}` : seg.summary
    if (!this.log.includes(line)) this.log.push(line)
  }

  private entryIndex(): number {
    if (this.seq.entryId) {
      const i = this.seq.segments.findIndex((s) => s.id === this.seq.entryId)
      if (i >= 0) return i
    }
    return 0
  }

  private elapsed(): number {
    const base = this.now() - this.startedAt - this.pausedAccum
    if (this.status === 'paused' && this.pauseStarted) {
      return base - (this.now() - this.pauseStarted)
    }
    return Math.max(0, base)
  }

  private buildCheckpoint(): PresentationCheckpoint {
    const seg = this.current()
    return {
      sequenceId: this.seq.id,
      sequenceVersion: this.seq.version,
      segmentId: seg?.id ?? '',
      index: this.index,
      choiceId: seg ? this.choiceSubmitted.get(seg.id) : undefined,
      choices: Object.fromEntries(this.choiceSubmitted.entries()),
      skipped: this.log.length > 0,
      log: [...this.log],
      updatedAt: this.now(),
    }
  }

  private persist() {
    this.opts.save?.(this.buildCheckpoint())
  }
}

/** Resolve display text for a segment under mute/degrade. */
export function segmentFallbackText(seg: PresentationSegment, degraded: (url?: string) => boolean): string {
  switch (seg.kind) {
    case 'comic_v':
      return (seg.comic?.panels ?? [])
        .map((p) => (degraded(p.imageUrl) ? p.caption : p.caption))
        .join(' / ')
    case 'dialogue':
      return (seg.dialogue?.lines ?? []).map((l) => `${l.speaker}: ${l.text}`).join('\n')
    case 'postcard':
      return `${seg.postcard?.title ?? ''} — ${seg.postcard?.placeLabel ?? ''}: ${seg.postcard?.body ?? ''}`
    case 'voice_note':
      // Mute/no audio → transcript always available
      return seg.voiceNote?.transcript ?? seg.summary
    case 'static_image':
      return seg.staticImage?.caption ?? seg.summary
    default:
      return seg.summary
  }
}
