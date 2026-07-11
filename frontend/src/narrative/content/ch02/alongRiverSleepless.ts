/**
 * AP-126 Chapter 2 《沿河不眠》 — data-driven content pack.
 * No hard-coded page branches: consumers resolve routes via resolveChapter2().
 */

import type { PresentationSequence } from '../../presentation/types'

export type SafetyContext = {
  /** Local hour 0-23 */
  hour: number
  weather: 'clear' | 'rain' | 'storm' | 'extreme'
  isMinor: boolean
  homeMode: boolean
  /** Low mobility / cannot go out */
  lowMobility: boolean
  /** Chapter 1 choice ids that still matter */
  ch1Choices: string[]
}

export type RouteId = 'day_field' | 'home_desk' | 'memory_night' | 'ally_path' | 'planner_path'

export interface Chapter2Node {
  id: string
  title: string
  routes: RouteId[]
  /** Presentation sequence id from pack */
  sequenceId: string
  summary: string
  /** Unlocks after complete */
  unlocks?: string[]
}

export interface Chapter2Pack {
  id: 'ch02.along_river_sleepless'
  title: string
  version: string
  nodes: Chapter2Node[]
  sequences: PresentationSequence[]
  /** Friendly sim opinions only — no real animal harm */
  debateBeats: { id: string; stanceA: string; stanceB: string; noHarmNote: string }[]
}

export const chapter2Pack: Chapter2Pack = {
  id: 'ch02.along_river_sleepless',
  title: '沿河不眠',
  version: '1',
  nodes: [
    {
      id: 'n_open',
      title: '滨水灯与施工围挡',
      routes: ['day_field', 'home_desk', 'memory_night'],
      sequenceId: 'ch02.open',
      summary: '灯光、施工与夜间活动谁先占用公共河岸。',
    },
    {
      id: 'n_conflict',
      title: '多方冲突',
      routes: ['day_field', 'home_desk', 'ally_path', 'planner_path'],
      sequenceId: 'ch02.conflict',
      summary: '居民、施工方、夜跑社与护河志愿者各有诉求。',
    },
    {
      id: 'n_resolve',
      title: '没有唯一正解',
      routes: ['ally_path', 'planner_path', 'home_desk'],
      sequenceId: 'ch02.resolve',
      summary: '两个合理方案各有得失。',
      unlocks: ['layer.river_memory', 'rumor.ch03.wharf'],
    },
  ],
  debateBeats: [
    {
      id: 'db_light',
      stanceA: '加亮路灯能提升安全感与夜间可达。',
      stanceB: '过亮灯光会挤占夜行小生境与观星可能。',
      noHarmNote: '模拟战只比观点卡，不出现真实动物受伤描写。',
    },
    {
      id: 'db_space',
      stanceA: '临时施工占道换来更安全的堤岸。',
      stanceB: '长期围挡切断了日常散步与无障碍通道。',
      noHarmNote: '冲突用对话与资源标记表达，不写暴力。',
    },
  ],
  sequences: [
    {
      id: 'ch02.open',
      title: '沿河不眠 · 开场',
      version: '1',
      segments: [
        {
          id: 'o1',
          kind: 'postcard',
          summary: '明信片：河堤围挡与新装灯杆并置。',
          postcard: {
            title: '未眠的河岸',
            placeLabel: '滨水区（粗粒度）',
            body: '白天能走完的步道，夜里被灯带与围挡切成几段。',
          },
        },
        {
          id: 'o2',
          kind: 'dialogue',
          summary: '向导说明：夜间外出非必须。',
          dialogue: {
            lines: [
              { id: 'd1', speaker: '向导', text: '夜里的线索可以用便签和回忆补齐，不必出门。' },
              { id: 'd2', speaker: '你', text: '我先看白天路线。' },
            ],
          },
        },
      ],
    },
    {
      id: 'ch02.conflict',
      title: '沿河不眠 · 冲突',
      version: '1',
      segments: [
        {
          id: 'c1',
          kind: 'comic_v',
          summary: '竖屏四格：灯、围挡、夜跑、志愿者。',
          comic: {
            panels: [
              { id: 'p1', caption: '新灯杆一夜排开', alt: '灯杆示意' },
              { id: 'p2', caption: '围挡挡住无障碍坡道', alt: '围挡示意' },
              { id: 'p3', caption: '夜跑社想保留环形道', alt: '步道示意' },
              { id: 'p4', caption: '志愿者记录潮位与声景', alt: '记录本示意' },
            ],
          },
        },
        {
          id: 'c2',
          kind: 'static_image',
          summary: '关键选择：偏同盟协商还是规划公示。',
          staticImage: { caption: '两种推进方式', alt: '分叉' },
          choice: {
            prompt: '你更想先推进哪条路径？',
            options: [
              { id: 'ally', label: '找夜跑社与志愿者对谈', next: 'c3a' },
              { id: 'planner', label: '去看规划公示与工期', next: 'c3b' },
            ],
          },
        },
        {
          id: 'c3a',
          kind: 'dialogue',
          summary: '同盟路径：交换时段与声景约定。',
          dialogue: {
            lines: [{ id: 'a1', speaker: '志愿者', text: '我们可以让出清晨半小时给施工进场。' }],
          },
        },
        {
          id: 'c3b',
          kind: 'dialogue',
          summary: '规划路径：看到分期恢复无障碍通道。',
          dialogue: {
            lines: [{ id: 'b1', speaker: '公示板', text: '二期将优先恢复坡道，灯光分区控亮。' }],
          },
        },
      ],
    },
    {
      id: 'ch02.resolve',
      title: '沿河不眠 · 收束',
      version: '1',
      segments: [
        {
          id: 'r1',
          kind: 'voice_note',
          summary: '便签：两种方案都有人损失一些便利。',
          voiceNote: {
            transcript: '没有完美河岸，只有可调整的约定。',
          },
        },
        {
          id: 'r2',
          kind: 'dialogue',
          summary: '解锁沿河记忆层与第三章传闻。',
          dialogue: {
            lines: [
              { id: 'r2a', speaker: '系统', text: '沿河记忆图层已点亮。' },
              { id: 'r2b', speaker: '传闻', text: '码头那边开始流传第三章的名字……' },
            ],
          },
        },
      ],
    },
    // Home / memory equivalents
    {
      id: 'ch02.home_desk',
      title: '居家书桌线',
      version: '1',
      segments: [
        {
          id: 'h1',
          kind: 'dialogue',
          summary: '居家：通过直播回放与市民热线记录冲突。',
          dialogue: {
            lines: [
              { id: 'h1a', speaker: '你', text: '我在家整理白天拍到的告示。' },
              { id: 'h1b', speaker: '热线摘要', text: '多位居民提到坡道被占。' },
            ],
          },
        },
      ],
    },
    {
      id: 'ch02.memory_night',
      title: '夜间回忆线',
      version: '1',
      segments: [
        {
          id: 'm1',
          kind: 'postcard',
          summary: '回忆便签呈现夜景，无需外出。',
          postcard: {
            title: '昨夜的灯带',
            placeLabel: '记忆中的河堤',
            body: '你想起灯带像一条白河，却听不清水声。',
          },
        },
      ],
    },
  ],
}

export type ResolvedRoute = {
  route: RouteId
  reason: string
  outdoorRequired: boolean
  sequence: PresentationSequence
  ch1AttitudeMod: string | null
}

/** Safety-first route picker: night/storm/minor/home never require outdoor. */
export function resolveChapter2Route(ctx: SafetyContext, nodeId: string): ResolvedRoute {
  const node = chapter2Pack.nodes.find((n) => n.id === nodeId) ?? chapter2Pack.nodes[0]
  const night = ctx.hour >= 21 || ctx.hour < 6
  const badWeather = ctx.weather === 'storm' || ctx.weather === 'extreme'
  const mustStayIn = ctx.homeMode || ctx.isMinor || ctx.lowMobility || night || badWeather

  let route: RouteId = 'day_field'
  let reason = '白天实地观察'
  if (mustStayIn) {
    if (night) {
      route = 'memory_night'
      reason = '夜间不要求外出，走回忆/便签'
    } else {
      route = 'home_desk'
      reason = '居家或低行动能力等价路线'
    }
  } else if (ctx.ch1Choices.includes('trust_guide')) {
    route = 'ally_path'
    reason = '第一章信任向导 → 同盟协商入口'
  } else if (ctx.ch1Choices.includes('read_plan')) {
    route = 'planner_path'
    reason = '第一章阅读规划 → 公示入口'
  }

  // Map route to sequence
  let sequenceId = node.sequenceId
  if (route === 'home_desk') sequenceId = 'ch02.home_desk'
  if (route === 'memory_night') sequenceId = 'ch02.memory_night'
  const sequence =
    chapter2Pack.sequences.find((s) => s.id === sequenceId) ??
    chapter2Pack.sequences.find((s) => s.id === node.sequenceId)!

  // Ch1 attitude: alters one dialogue flavor string (data only)
  let ch1AttitudeMod: string | null = null
  if (ctx.ch1Choices.includes('protect_habitat')) {
    ch1AttitudeMod = '志愿者对你更熟络，愿意分享潮位本。'
  } else if (ctx.ch1Choices.includes('prioritize_safety')) {
    ch1AttitudeMod = '夜跑社更愿意听你谈照明分区。'
  }

  return {
    route,
    reason,
    outdoorRequired: !mustStayIn && route === 'day_field',
    sequence,
    ch1AttitudeMod,
  }
}

export function completionUnlocks(nodeId: string): string[] {
  return chapter2Pack.nodes.find((n) => n.id === nodeId)?.unlocks ?? []
}
