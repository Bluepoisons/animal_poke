/** AP-122 data-driven narrative presentation segments (manifest-friendly). */

export type SegmentKind =
  | 'comic_v'
  | 'dialogue'
  | 'postcard'
  | 'voice_note'
  | 'static_image'

export interface ComicPanel {
  id: string
  /** Optional image URL; missing → text-only panel */
  imageUrl?: string
  caption: string
  alt: string
}

export interface DialogueLine {
  id: string
  speaker: string
  portraitUrl?: string
  text: string
}

export interface ChoiceOption {
  id: string
  label: string
  /** Next segment id; empty ends sequence */
  next?: string
}

export interface PresentationSegment {
  id: string
  kind: SegmentKind
  /** Soft budget; runtime caps at 45s default */
  maxSeconds?: number
  /** Auto-advance delay ms when autoplay on and no choice */
  autoMs?: number
  /** Readable skip summary for this segment */
  summary: string
  comic?: { panels: ComicPanel[] }
  dialogue?: { lines: DialogueLine[] }
  postcard?: {
    title: string
    placeLabel: string
    body: string
    imageUrl?: string
  }
  voiceNote?: {
    audioUrl?: string
    transcript: string
    speaker?: string
  }
  staticImage?: {
    imageUrl?: string
    caption: string
    alt: string
  }
  /** If present, runtime pauses until player chooses (no auto/skip past without recording) */
  choice?: {
    prompt: string
    options: ChoiceOption[]
  }
}

export interface PresentationSequence {
  id: string
  title: string
  version: string
  segments: PresentationSegment[]
  /** Entry segment id (default first) */
  entryId?: string
}

export interface PresentationCheckpoint {
  sequenceId: string
  sequenceVersion: string
  segmentId: string
  index: number
  /** Choice id already submitted for this segment (idempotent) */
  choiceId?: string
  /** segmentId -> choiceId for idempotent restore */
  choices?: Record<string, string>
  skipped: boolean
  /** Accumulated log of summaries */
  log: string[]
  updatedAt: number
}

export type RuntimeStatus = 'idle' | 'playing' | 'paused' | 'awaiting_choice' | 'completed' | 'degraded'

export interface RuntimeSnapshot {
  status: RuntimeStatus
  sequenceId: string
  segment: PresentationSegment | null
  index: number
  total: number
  auto: boolean
  muted: boolean
  reducedMotion: boolean
  log: string[]
  /** Resource failures → degraded text/static path */
  failedAssets: string[]
  checkpoint: PresentationCheckpoint | null
  elapsedMs: number
}
