import { useCallback, useEffect, useMemo, useState } from 'react'
import { PresentationRuntime, segmentFallbackText } from './runtime'
import type { PresentationSequence, RuntimeSnapshot } from './types'

export interface PresentationPlayerProps {
  sequence: PresentationSequence
  storageKey?: string
  muted?: boolean
  reducedMotion?: boolean
  onComplete?: (log: string[]) => void
  onChoice?: (segmentId: string, choiceId: string) => void
}

function loadCp(key: string, sequenceId: string) {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return null
    const cp = JSON.parse(raw)
    return cp?.sequenceId === sequenceId ? cp : null
  } catch {
    return null
  }
}

function saveCp(key: string, cp: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(cp))
  } catch {
    /* ignore quota */
  }
}

/** Vertical narrative runtime UI shell (comic / dialogue / postcard / voice / static). */
export function PresentationPlayer({
  sequence,
  storageKey = `ap-narrative-cp:${sequence.id}`,
  muted = false,
  reducedMotion = false,
  onComplete,
  onChoice,
}: PresentationPlayerProps) {
  const rt = useMemo(
    () =>
      new PresentationRuntime(sequence, {
        muted,
        reducedMotion,
        auto: false,
        load: (id) => loadCp(storageKey, id),
        save: (cp) => saveCp(storageKey, cp),
        onChoice,
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps -- sequence identity by id+version
    [sequence.id, sequence.version, storageKey],
  )

  const [snap, setSnap] = useState<RuntimeSnapshot>(() => rt.start(true))

  useEffect(() => {
    rt.setMuted(muted)
  }, [muted, rt])

  useEffect(() => {
    if (snap.status === 'completed') onComplete?.(snap.log)
  }, [snap.status, snap.log, onComplete])

  const refresh = useCallback(() => setSnap(rt.snapshot()), [rt])

  const seg = snap.segment
  const text = seg ? segmentFallbackText(seg, (u) => rt.isDegraded(u)) : ''

  return (
    <section
      className="ap-narrative-player"
      data-testid="presentation-player"
      data-status={snap.status}
      aria-label={sequence.title}
    >
      <header className="ap-narrative-player__header">
        <h2>{sequence.title}</h2>
        <p>
          {snap.index + 1}/{snap.total}
          {snap.status === 'degraded' ? ' · 降级文本' : ''}
          {snap.muted || muted ? ' · 静音' : ''}
        </p>
      </header>

      {seg && (
        <div className="ap-narrative-player__body" data-kind={seg.kind}>
          {seg.kind === 'comic_v' && (
            <ol className="ap-comic-v">
              {(seg.comic?.panels ?? []).map((p) => (
                <li key={p.id}>
                  {!rt.isDegraded(p.imageUrl) && p.imageUrl ? (
                    <img
                      src={p.imageUrl}
                      alt={p.alt}
                      onError={() => {
                        rt.markAssetFailed(p.imageUrl!)
                        refresh()
                      }}
                    />
                  ) : null}
                  <p>{p.caption}</p>
                </li>
              ))}
            </ol>
          )}

          {seg.kind === 'dialogue' && (
            <ul className="ap-dialogue">
              {(seg.dialogue?.lines ?? []).map((l) => (
                <li key={l.id}>
                  <strong>{l.speaker}</strong>
                  <p>{l.text}</p>
                </li>
              ))}
            </ul>
          )}

          {seg.kind === 'postcard' && seg.postcard && (
            <article className="ap-postcard">
              <h3>{seg.postcard.title}</h3>
              <p className="ap-postcard__place">{seg.postcard.placeLabel}</p>
              {!rt.isDegraded(seg.postcard.imageUrl) && seg.postcard.imageUrl ? (
                <img
                  src={seg.postcard.imageUrl}
                  alt={seg.postcard.title}
                  onError={() => {
                    rt.markAssetFailed(seg.postcard!.imageUrl!)
                    refresh()
                  }}
                />
              ) : null}
              <p>{seg.postcard.body}</p>
            </article>
          )}

          {seg.kind === 'voice_note' && seg.voiceNote && (
            <figure className="ap-voice-note">
              <figcaption>{seg.voiceNote.speaker ?? '语音便签'}</figcaption>
              {!muted && seg.voiceNote.audioUrl && !rt.isDegraded(seg.voiceNote.audioUrl) ? (
                <audio
                  controls
                  src={seg.voiceNote.audioUrl}
                  onError={() => {
                    rt.markAssetFailed(seg.voiceNote!.audioUrl!)
                    refresh()
                  }}
                />
              ) : null}
              <p data-testid="voice-transcript">{seg.voiceNote.transcript}</p>
            </figure>
          )}

          {seg.kind === 'static_image' && seg.staticImage && (
            <figure>
              {!rt.isDegraded(seg.staticImage.imageUrl) && seg.staticImage.imageUrl ? (
                <img
                  src={seg.staticImage.imageUrl}
                  alt={seg.staticImage.alt}
                  onError={() => {
                    rt.markAssetFailed(seg.staticImage!.imageUrl!)
                    refresh()
                  }}
                />
              ) : null}
              <figcaption>{seg.staticImage.caption}</figcaption>
            </figure>
          )}

          {/* Always expose text path for weak network / screen readers */}
          <p className="ap-narrative-fallback" data-testid="segment-fallback">
            {text}
          </p>

          {seg.choice && snap.status === 'awaiting_choice' && (
            <div className="ap-narrative-choices" role="group" aria-label={seg.choice.prompt}>
              <p>{seg.choice.prompt}</p>
              {seg.choice.options.map((o) => (
                <button
                  key={o.id}
                  type="button"
                  data-testid={`choice-${o.id}`}
                  onClick={() => setSnap(rt.choose(o.id))}
                >
                  {o.label}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      <footer className="ap-narrative-player__controls">
        <button type="button" data-testid="btn-pause" onClick={() => { rt.pause(); setSnap(rt.snapshot()) }}>
          暂停
        </button>
        <button type="button" data-testid="btn-resume" onClick={() => setSnap(rt.resume())}>
          继续
        </button>
        <button type="button" data-testid="btn-rewind" onClick={() => setSnap(rt.rewind())}>
          回退
        </button>
        <button type="button" data-testid="btn-skip" onClick={() => setSnap(rt.skip())}>
          跳过
        </button>
        <button
          type="button"
          data-testid="btn-auto"
          onClick={() => {
            rt.setAuto(!snap.auto)
            refresh()
          }}
        >
          自动: {snap.auto ? '开' : '关'}
        </button>
      </footer>

      <aside className="ap-narrative-log" data-testid="presentation-log" aria-live="polite">
        <h3>摘要</h3>
        <ol>
          {snap.log.map((line, i) => (
            <li key={i}>{line}</li>
          ))}
        </ol>
      </aside>
    </section>
  )
}
