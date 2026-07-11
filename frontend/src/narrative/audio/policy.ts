/** AP-123 audio policy + caption/fallback helpers (no startling autoplay). */

export type AudioBus = 'voice' | 'ambience' | 'music' | 'ui'

export interface CaptionCue {
  id: string
  speaker?: string
  text: string
  /** Non-speech plot-bearing sound description */
  sfxDescription?: string
  startMs: number
  endMs: number
}

export interface AudioClipRef {
  id: string
  bus: AudioBus
  /** Optional; missing → transcript path */
  url?: string
  locale: string
  version: number
  licenseId: string
  captions: CaptionCue[]
  /** Full text alternative when muted / failed */
  transcript: string
  /** If true, must never autoplay */
  startling?: boolean
}

export interface AudioPlaybackState {
  muted: boolean
  systemMuted: boolean
  backgrounded: boolean
  textOnly: boolean
  volumes: Record<AudioBus, number>
}

export function defaultPlaybackState(): AudioPlaybackState {
  return {
    muted: false,
    systemMuted: false,
    backgrounded: false,
    textOnly: false,
    volumes: { voice: 1, ambience: 0.4, music: 0.3, ui: 0.5 },
  }
}

/** Whether a clip may start without a fresh user gesture. */
export function mayAutoplay(clip: AudioClipRef, state: AudioPlaybackState): boolean {
  if (state.muted || state.systemMuted || state.backgrounded || state.textOnly) return false
  if (clip.startling) return false
  if (clip.bus === 'voice') return false // voice always gesture / explicit
  if (!clip.url) return false
  return clip.bus === 'ambience' || clip.bus === 'music' || clip.bus === 'ui'
}

/** Effective presentation when audio unavailable. */
export function resolvePlayback(
  clip: AudioClipRef,
  state: AudioPlaybackState,
  assetFailed = false,
): { mode: 'audio' | 'captions_only'; transcript: string; captions: CaptionCue[] } {
  const forceText =
    state.muted || state.systemMuted || state.textOnly || state.backgrounded || assetFailed || !clip.url
  return {
    mode: forceText ? 'captions_only' : 'audio',
    transcript: clip.transcript,
    captions: clip.captions,
  }
}

/** Caption line at time t. */
export function activeCaptions(cues: CaptionCue[], tMs: number): CaptionCue[] {
  return cues.filter((c) => tMs >= c.startMs && tMs <= c.endMs)
}

/** Locale fallback chain. */
export function pickClipLocale(
  clips: AudioClipRef[],
  preferred: string[],
): AudioClipRef | null {
  for (const loc of preferred) {
    const hit = clips.find((c) => c.locale === loc && c.url)
    if (hit) return hit
  }
  for (const loc of preferred) {
    const hit = clips.find((c) => c.locale === loc)
    if (hit) return hit
  }
  return clips[0] ?? null
}

/** Background / call: stop all playback flags. */
export function onAppBackground(state: AudioPlaybackState): AudioPlaybackState {
  return { ...state, backgrounded: true }
}

export function onAppForeground(state: AudioPlaybackState): AudioPlaybackState {
  // Do not auto-resume; clear flag only
  return { ...state, backgrounded: false }
}
