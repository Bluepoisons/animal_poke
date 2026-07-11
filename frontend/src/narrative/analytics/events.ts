/** AP-135 client-side narrative analytics event builders (privacy-safe props only). */

export type StuckReason = 'understanding' | 'pacing' | 'tech' | 'other'
export type SkipReason = 'intentional' | 'confused' | 'other'

const FORBIDDEN = ['photo', 'token', 'lat', 'lng', 'dialogue', 'transcript_raw']

export interface NarrativeEvent {
  name: string
  props: Record<string, string | number | boolean>
}

function scrub(props: Record<string, string | number | boolean>): Record<string, string | number | boolean> {
  const out: Record<string, string | number | boolean> = {}
  for (const [k, v] of Object.entries(props)) {
    const lower = k.toLowerCase()
    if (FORBIDDEN.some((f) => lower.includes(f))) continue
    out[k] = v
  }
  return out
}

export function chapterComplete(chapterVersion: string, foundOptional: boolean): NarrativeEvent {
  return {
    name: 'narrative_chapter_complete',
    props: scrub({ chapter_version: chapterVersion, had_optional: true, found_optional: foundOptional }),
  }
}

export function segmentSkip(chapterVersion: string, segmentId: string, reason: SkipReason): NarrativeEvent {
  return {
    name: 'narrative_segment_skip',
    props: scrub({ chapter_version: chapterVersion, segment_id: segmentId, reason }),
  }
}

export function choiceMade(chapterVersion: string, choiceId: string): NarrativeEvent {
  return {
    name: 'narrative_choice',
    props: scrub({ chapter_version: chapterVersion, choice_id: choiceId }),
  }
}

export function stuck(chapterVersion: string, reason: StuckReason): NarrativeEvent {
  return {
    name: 'narrative_stuck',
    props: scrub({ chapter_version: chapterVersion, reason }),
  }
}

export function techInterrupt(chapterVersion: string): NarrativeEvent {
  return {
    name: 'narrative_tech_interrupt',
    props: scrub({ chapter_version: chapterVersion }),
  }
}

/** Consented survey sample — numeric aggregates only, no free text. */
export function surveySample(
  chapterVersion: string,
  meaningfulChoice: number,
  feltLectured: number,
): NarrativeEvent {
  return {
    name: 'narrative_survey_sample',
    props: scrub({
      chapter_version: chapterVersion,
      meaningful_choice: Math.min(1, Math.max(0, meaningfulChoice)),
      felt_lectured: Math.min(1, Math.max(0, feltLectured)),
    }),
  }
}
