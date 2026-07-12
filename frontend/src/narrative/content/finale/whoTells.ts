/**
 * AP-129 Finale 《把城市交给谁讲》 multi-ending exhibition.
 * Endings depend on relations/choices/note handling — not collection rate.
 */
import type { PresentationSequence } from '../../presentation/types'
import { season1Architecture, type SeasonOutcome } from '../season1/architecture'
import { getCharacter } from '../characters/cast'

export type EndingId = 'ending.multivocal' | 'ending.curated' | 'ending.blank_first'

export type FinaleInput = {
  /** option ids chosen across season */
  choiceOptionIds: string[]
  noteHandling: 'preserve_gaps' | 'overwrite' | 'multivocal'
  /** sum of trust deltas if tracked; optional */
  trustSum?: number
  /** collection rate must NOT affect ending */
  collectionRate?: number
}

export interface FinalePack {
  id: 'finale.who_tells_the_city'
  title: string
  version: string
  sequences: PresentationSequence[]
  outcomes: SeasonOutcome[]
}

const aide = getCharacter('journal_aide')!
const archivist = getCharacter('archivist')!
const photo = getCharacter('street_photographer')!
const planner = getCharacter('urban_planner')!

export const finalePack: FinalePack = {
  id: 'finale.who_tells_the_city',
  title: '把城市交给谁讲',
  version: '1',
  outcomes: season1Architecture.outcomes,
  sequences: [
    {
      id: 'finale.curate',
      title: '策展清单',
      version: '1',
      segments: [
        {
          id: 'f1',
          kind: 'dialogue',
          summary: '汇总四季选择。',
          dialogue: {
            lines: [
              { id: 'a', speaker: aide.displayName, text: '我们把仍有回响的选择摆上桌。收集率不打分。' },
              { id: 'b', speaker: archivist.displayName, text: '我想看空白能不能成为展项。' },
              { id: 'c', speaker: photo.displayName, text: '署名可以分享。' },
              { id: 'd', speaker: planner.displayName, text: '模型失败也要上墙。' },
            ],
          },
        },
        {
          id: 'f2',
          kind: 'dialogue',
          summary: '终章定稿选择。',
          dialogue: {
            lines: [{ id: 'e', speaker: aide.displayName, text: '展览主叙事？' }],
          },
          choice: {
            prompt: '终章展览主叙事？',
            options: [
              { id: 'multivocal_show', label: '多声部并存', next: 'f3' },
              { id: 'curated_arc', label: '策展者编排主线', next: 'f3' },
              { id: 'blank_first', label: '空白优先', next: 'f3' },
            ],
          },
        },
        {
          id: 'f3',
          kind: 'postcard',
          summary: '展厅开放（虚构社区展厅）。',
          postcard: {
            title: '城市手账展',
            placeLabel: '社区展厅（虚构）',
            body: '结局由关系、选择与资料处理方式决定——不是收集率。',
          },
        },
      ],
    },
  ],
}

/** Score endings without using collectionRate */
export function resolveEnding(input: FinaleInput): EndingId {
  void input.collectionRate // explicitly ignored
  const set = new Set(input.choiceOptionIds)
  const scores: Record<EndingId, number> = {
    'ending.multivocal': 0,
    'ending.curated': 0,
    'ending.blank_first': 0,
  }

  if (set.has('multivocal_board') || set.has('multivocal_show') || input.noteHandling === 'multivocal') {
    scores['ending.multivocal'] += 2
  }
  if (set.has('preserve_gaps') || set.has('blank_wall') || set.has('blank_first') || input.noteHandling === 'preserve_gaps') {
    scores['ending.blank_first'] += 2
  }
  if (set.has('managed_board') || set.has('curated_arc') || input.noteHandling === 'overwrite') {
    scores['ending.curated'] += 2
  }
  if (set.has('multivocal_show')) scores['ending.multivocal'] += 3
  if (set.has('curated_arc')) scores['ending.curated'] += 3
  if (set.has('blank_first')) scores['ending.blank_first'] += 3

  // stable tie-break: multivocal > blank_first > curated
  const order: EndingId[] = ['ending.multivocal', 'ending.blank_first', 'ending.curated']
  return order.sort((a, b) => scores[b] - scores[a])[0]
}

export function endingSummary(id: EndingId): string {
  return finalePack.outcomes.find((o) => o.id === id)?.summary ?? id
}

export function validateFinale(pack = finalePack): string[] {
  const errors: string[] = []
  if (pack.outcomes.length < 2) errors.push('need ≥2 endings')
  for (const o of pack.outcomes) {
    if (!o.independentOfCollectionRate) errors.push(`${o.id} depends on collection rate`)
  }
  if (!pack.sequences.some((s) => s.segments.some((seg) => seg.choice && seg.choice.options.length >= 2))) {
    errors.push('missing finale choice')
  }
  // prove collection rate ignored
  const a = resolveEnding({
    choiceOptionIds: ['multivocal_show', 'preserve_gaps'],
    noteHandling: 'preserve_gaps',
    collectionRate: 0,
  })
  const b = resolveEnding({
    choiceOptionIds: ['multivocal_show', 'preserve_gaps'],
    noteHandling: 'preserve_gaps',
    collectionRate: 1,
  })
  if (a !== b) errors.push('collection rate must not change ending')
  return errors
}
