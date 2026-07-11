# AP-123 Narrative Audio Strategy

## Goals

- Partial VO only for **core openings and turning points** (season-1 cost control).
- Everything else: on-screen text + optional short reaction VO / ambient beds.
- Silent / hard-of-hearing / missing-asset players always get **complete information**.

## Buses

| Bus | Content | Default | Notes |
|---|---|---|---|
| `voice` | Partial dialogue VO | User opt-in or explicit play | Never autoplay jump-scare lines |
| `ambience` | City soundscape loops | Soft, ducked under voice | Loop crossfade ≤ 1s |
| `music` | Sparse beds | Off until user enables | No stingers on first session |
| `ui` | Soft UI ticks | On | ≤ −18 LUFS momentary |

## Accessibility

1. **Captions** for every VO line: text, speaker, start/end ms.
2. **SFX descriptions** in caption track when ambience carries plot (`[闸门低鸣]`).
3. **Mute / volume / text-only** toggles; system mute forces text-only.
4. **Background / phone call**: pause all buses; do not resume without user gesture.
5. **Locale fallback**: `zh` → `en` → transcript-only.

## Asset rules

Naming: `{locale}/{chapter}/{role_or_bus}/{id}_v{n}.{ext}`  
Example: `zh/ch02/voice/guide_open_v1.opus`

| Field | Requirement |
|---|---|
| Loudness | Dialogue integrated ≈ −16 LUFS; peaks ≤ −1 dBTP |
| Format | Opus/WebM preferred; AAC fallback |
| License | Self-made or documented chain in `docs/assets/audio-license.json` |
| Version | Bump `v{n}` on re-record; old version withdrawable via content manifest |
| Withdraw | Manifest removes URL; client falls back to transcript |

## Runtime contract (code)

`frontend/src/narrative/audio/*`:

- `AudioPolicy` — no autoplay of startling sounds; background stop.
- `CaptionTrack` — speaker + text + optional sfx description.
- `resolveClip()` — missing/failed → transcript only.
- Integrates with AP-122 `voice_note` segments.

## Out of scope

- Full-cast dubbing for every line.
- Real-device Bluetooth matrix (documented as manual QA).

## Manual QA checklist (not CI)

- [ ] System mute → full plot via captions
- [ ] Background tab → audio stops
- [ ] Missing mp3 → transcript shown
- [ ] zh/en/fallback locales
