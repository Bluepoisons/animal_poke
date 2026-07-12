/**
 * AP-125 Chapter 1 《巷口的回声》 — data-driven content pack.
 */
import type { PresentationSequence } from '../../presentation/types'
import { getCharacter } from '../characters/cast'
import type { ChapterId } from '../season1/architecture'

export type Ch1RouteId = 'street_walk' | 'home_hotline' | 'shop_side' | 'photo_side'

export interface Chapter1Node {
  id: string
  title: string
  routes: Ch1RouteId[]
  sequenceId: string
  summary: string
  unlocks?: string[]
}

export interface Chapter1Pack {
  id: 'ch01.alley_echo'
  title: string
  version: string
  chapterId: ChapterId
  nodes: Chapter1Node[]
  sequences: PresentationSequence[]
  perspectives: { id: string; role: string; claim: string }[]
}

const archivist = getCharacter('archivist')!
const photo = getCharacter('street_photographer')!
const planner = getCharacter('urban_planner')!
const aide = getCharacter('journal_aide')!

export const chapter1Pack: Chapter1Pack = {
  id: 'ch01.alley_echo',
  title: '巷口的回声',
  version: '1',
  chapterId: 'ch01.alley_echo',
  perspectives: [
    { id: 'resident', role: '老街居民', claim: '告示栏要能走、能看、不能被摊位堵死。' },
    { id: 'vendor', role: '商户', claim: '统一管理才干净，不然谁都往上贴。' },
    { id: 'photographer', role: '街拍者', claim: '现场语气也是公共记忆的一部分。' },
  ],
  nodes: [
    {
      id: 'n_board',
      title: '巷口告示栏',
      routes: ['street_walk', 'home_hotline'],
      sequenceId: 'ch01.board',
      summary: '多方在同一块板上写不同的句子。',
    },
    {
      id: 'n_conflict',
      title: '空间占用',
      routes: ['street_walk', 'shop_side', 'photo_side', 'home_hotline'],
      sequenceId: 'ch01.conflict',
      summary: '通行、摊位与拍摄互相挤压。',
    },
    {
      id: 'n_choice',
      title: '告示栏规则',
      routes: ['street_walk', 'home_hotline'],
      sequenceId: 'ch01.choice',
      summary: '多声道并存还是统一管理。',
      unlocks: ['echo.ch02.who_speaks_first'],
    },
  ],
  sequences: [
    {
      id: 'ch01.board',
      title: '巷口 · 告示栏',
      version: '1',
      segments: [
        {
          id: 'b1',
          kind: 'postcard',
          summary: '粗粒度巷口，无精确门牌。',
          postcard: {
            title: '告示栏',
            placeLabel: '老街巷口（粗粒度）',
            body: '同一面木板，贴着三张不同笔迹的纸。',
          },
        },
        {
          id: 'b2',
          kind: 'dialogue',
          summary: '三角色开场。',
          dialogue: {
            lines: [
              { id: 'd1', speaker: archivist.displayName, text: archivist.voice.sampleLine },
              { id: 'd2', speaker: photo.displayName, text: photo.voice.sampleLine },
              { id: 'd3', speaker: aide.displayName, text: '也可以只听热线记录，不必走到巷口。' },
            ],
          },
        },
      ],
    },
    {
      id: 'ch01.conflict',
      title: '巷口 · 冲突',
      version: '1',
      segments: [
        {
          id: 'c1',
          kind: 'dialogue',
          summary: '三方立场。',
          dialogue: {
            lines: [
              { id: 'c1a', speaker: '老街居民', text: '先保证轮椅和婴儿车能过。' },
              { id: 'c1b', speaker: '商户', text: '乱贴一通，顾客以为店关门了。' },
              { id: 'c1c', speaker: photo.displayName, text: '把现场全擦掉，档案就只剩表格。' },
              { id: 'c1d', speaker: planner.displayName, text: planner.voice.sampleLine },
            ],
          },
        },
        {
          id: 'c2',
          kind: 'comic_v',
          summary: '竖屏：通道被挤窄。',
          comic: {
            panels: [
              { id: 'p1', caption: '摊位外扩半米。', alt: '通道变窄示意' },
              { id: 'p2', caption: '镜头抬起又放下。', alt: '摄影者犹豫' },
            ],
          },
        },
      ],
    },
    {
      id: 'ch01.choice',
      title: '巷口 · 选择',
      version: '1',
      segments: [
        {
          id: 'ch1',
          kind: 'dialogue',
          summary: '价值选择。',
          dialogue: {
            lines: [{ id: 'x1', speaker: aide.displayName, text: '告示栏规则由谁定？没有唯一正解。' }],
          },
          choice: {
            prompt: '告示栏规则由谁定？',
            options: [
              { id: 'multivocal_board', label: '多声道并存', next: 'ch2' },
              { id: 'managed_board', label: '统一管理', next: 'ch2' },
            ],
          },
        },
        {
          id: 'ch2',
          kind: 'dialogue',
          summary: '回响预告。',
          dialogue: {
            lines: [
              {
                id: 'x2',
                speaker: aide.displayName,
                text: '你的选择会改变滨水章节谁先开口。',
              },
            ],
          },
        },
      ],
    },
  ],
}

export type Ch1Context = {
  homeMode: boolean
  noCamera: boolean
  badWeather: boolean
  /** prologue choice option id if any */
  prologueOptionId?: string
}

export function resolveChapter1(ctx: Ch1Context): {
  route: Ch1RouteId
  entryNodeId: string
  sequenceIds: string[]
} {
  const route: Ch1RouteId =
    ctx.homeMode || ctx.badWeather || ctx.noCamera ? 'home_hotline' : 'street_walk'
  return {
    route,
    entryNodeId: 'n_board',
    sequenceIds: chapter1Pack.nodes.map((n) => n.sequenceId),
  }
}

export function validateChapter1(pack = chapter1Pack): string[] {
  const errors: string[] = []
  if (pack.nodes.length < 3) errors.push('need ≥3 nodes')
  if (pack.perspectives.length < 3) errors.push('need ≥3 perspectives')
  for (const n of pack.nodes) {
    if (!pack.sequences.some((s) => s.id === n.sequenceId)) errors.push(`missing seq ${n.sequenceId}`)
  }
  const choice = pack.sequences.find((s) => s.id === 'ch01.choice')
  if (!choice?.segments.some((s) => s.choice && s.choice.options.length >= 2)) {
    errors.push('missing multi-option choice')
  }
  return errors
}
