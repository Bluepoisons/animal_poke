/**
 * AP-124 · Prologue 《第一张空白页》 vertical slice (15–20 min design budget).
 * Completable without camera/location; training observation only.
 */
import type { PresentationSequence } from '../../presentation/types'
import type { ChapterId } from '../season1/architecture'
import { getChapter } from '../season1/architecture'
import { getCharacter } from '../characters/cast'

export const PROLOGUE_CHAPTER_ID: ChapterId = 'prologue.blank_page'

export type PrologueBeatId =
  | 'train_observe'
  | 'contradiction'
  | 'choice'
  | 'hook'

export interface PrologueSlice {
  id: 'prologue.blank_page.slice'
  title: string
  version: string
  budgetMinutes: { min: number; max: number }
  /** Ordered playable beats */
  beats: {
    id: PrologueBeatId
    title: string
    sequenceId: string
    minutes: number
  }[]
  sequences: PresentationSequence[]
  /** Cast introduced in slice */
  castIds: string[]
  /** Ends by unlocking */
  unlocksChapter: ChapterId
  noCamera: true
  noLocation: true
  homeMode: true
}

export const prologueSlice: PrologueSlice = {
  id: 'prologue.blank_page.slice',
  title: '第一张空白页',
  version: '1',
  budgetMinutes: { min: 15, max: 20 },
  castIds: ['archivist', 'street_photographer', 'journal_aide'],
  unlocksChapter: 'ch01.alley_echo',
  noCamera: true,
  noLocation: true,
  homeMode: true,
  beats: [
    { id: 'train_observe', title: '训练观察', sequenceId: 'prologue.train', minutes: 5 },
    { id: 'contradiction', title: '矛盾注释', sequenceId: 'prologue.contradiction', minutes: 5 },
    { id: 'choice', title: '没有标准答案', sequenceId: 'prologue.choice', minutes: 5 },
    { id: 'hook', title: '巷口有回声', sequenceId: 'prologue.hook', minutes: 3 },
  ],
  sequences: [
    {
      id: 'prologue.train',
      title: '训练观察',
      version: '1',
      segments: [
        {
          id: 't1',
          kind: 'dialogue',
          summary: '助手说明可用训练帧，不必出门。',
          dialogue: {
            lines: [
              {
                id: 't1a',
                speaker: getCharacter('journal_aide')!.displayName,
                text: '我们用训练素材做第一次观察。不必开相机，也不必出门。',
              },
              {
                id: 't1b',
                speaker: '你',
                text: '好，我先记一条。',
              },
            ],
          },
        },
        {
          id: 't2',
          kind: 'postcard',
          summary: '训练帧：窗边一只猫的静帧（合成）。',
          postcard: {
            title: '训练帧 · 窗边',
            placeLabel: '室内训练台',
            body: '观察记录：猫，坐姿，光从左侧来。这是合成训练素材，不是野外拍摄。',
          },
        },
      ],
    },
    {
      id: 'prologue.contradiction',
      title: '矛盾注释',
      version: '1',
      segments: [
        {
          id: 'c1',
          kind: 'dialogue',
          summary: '档案员与街拍者给出不同注释。',
          dialogue: {
            lines: [
              {
                id: 'c1a',
                speaker: getCharacter('archivist')!.displayName,
                text: getCharacter('archivist')!.voice.sampleLine,
              },
              {
                id: 'c1b',
                speaker: getCharacter('street_photographer')!.displayName,
                text: getCharacter('street_photographer')!.voice.sampleLine,
              },
              {
                id: 'c1c',
                speaker: getCharacter('journal_aide')!.displayName,
                text: '同一页出现了两行互相矛盾的注释。',
              },
            ],
          },
        },
        {
          id: 'c2',
          kind: 'comic_v',
          summary: '竖屏分镜：空白页被两支笔同时写。',
          comic: {
            panels: [
              {
                id: 'p1',
                caption: '第一行：可核验。',
                alt: '档案员写下核验标记',
              },
              {
                id: 'p2',
                caption: '第二行：现场感。',
                alt: '街拍者写下光的描述',
              },
            ],
          },
        },
      ],
    },
    {
      id: 'prologue.choice',
      title: '没有标准答案',
      version: '1',
      segments: [
        {
          id: 'ch1',
          kind: 'dialogue',
          summary: '价值选择：如何处理矛盾注释。',
          dialogue: {
            lines: [
              {
                id: 'ch1a',
                speaker: getCharacter('journal_aide')!.displayName,
                text: '没有唯一正确答案。你怎么处理这两行？',
              },
            ],
          },
          choice: {
            prompt: getChapter(PROLOGUE_CHAPTER_ID)!.choice.prompt,
            options: getChapter(PROLOGUE_CHAPTER_ID)!.choice.options.map((o) => ({
              id: o.id,
              label: o.label,
              next: 'ch2',
            })),
          },
        },
        {
          id: 'ch2',
          kind: 'dialogue',
          summary: '选择已记录，延迟回响将在后章出现。',
          dialogue: {
            lines: [
              {
                id: 'ch2a',
                speaker: getCharacter('journal_aide')!.displayName,
                text: '好。这个决定会在后面的章节留下回声。',
              },
            ],
          },
        },
      ],
    },
    {
      id: 'prologue.hook',
      title: '巷口有回声',
      version: '1',
      segments: [
        {
          id: 'h1',
          kind: 'postcard',
          summary: '便签悬念，开启第一章。',
          postcard: {
            title: '未署名便签',
            placeLabel: '手账末页',
            body: '巷口有回声。不必现在就去——你可以先从热线记录读起。',
          },
        },
        {
          id: 'h2',
          kind: 'dialogue',
          summary: '自然衔接到《巷口的回声》。',
          dialogue: {
            lines: [
              {
                id: 'h2a',
                speaker: getCharacter('journal_aide')!.displayName,
                text: '下一章是《巷口的回声》。你已经是记录者了。',
              },
            ],
          },
        },
      ],
    },
  ],
}

export function prologueSequences(): PresentationSequence[] {
  return prologueSlice.sequences
}

export function getPrologueSequence(id: string): PresentationSequence | undefined {
  return prologueSlice.sequences.find((s) => s.id === id)
}

export function validatePrologueSlice(slice: PrologueSlice = prologueSlice): string[] {
  const errors: string[] = []
  const sum = slice.beats.reduce((a, b) => a + b.minutes, 0)
  if (sum < slice.budgetMinutes.min || sum > slice.budgetMinutes.max + 2) {
    errors.push(`beat minutes ${sum} outside budget ${slice.budgetMinutes.min}-${slice.budgetMinutes.max}`)
  }
  if (slice.castIds.length < 3) errors.push('need ≥3 cast introductions')
  if (!slice.noCamera || !slice.homeMode) errors.push('must be playable without camera / home mode')
  for (const b of slice.beats) {
    if (!slice.sequences.some((s) => s.id === b.sequenceId)) {
      errors.push(`missing sequence ${b.sequenceId}`)
    }
  }
  const choiceSeq = slice.sequences.find((s) => s.id === 'prologue.choice')
  const hasChoice = choiceSeq?.segments.some((seg) => seg.choice && seg.choice.options.length >= 2)
  if (!hasChoice) errors.push('choice beat missing multi-option choice')
  return errors
}

export function rhythmTable(): { beat: string; minutes: number; sequenceId: string }[] {
  return prologueSlice.beats.map((b) => ({
    beat: b.title,
    minutes: b.minutes,
    sequenceId: b.sequenceId,
  }))
}
