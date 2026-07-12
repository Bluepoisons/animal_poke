/**
 * AP-121 · City journal client state (last-known-good, offline-friendly).
 * Pokedex = animal facts; Journal = people/city story state.
 */
import type { ChapterId } from '../content/season1/architecture'
import { listSeasonChapters, getChapter } from '../content/season1/architecture'
import { listCharacters, type CharacterId } from '../content/characters/cast'

const STORAGE_KEY = 'ap-city-journal-v1'

export type ClueStatus = 'locked' | 'available' | 'collected' | 'missed'

export type JournalClue = {
  id: string
  chapterId: ChapterId
  title: string
  summary: string
  status: ClueStatus
  /** How to obtain if missed — never bare ??? */
  altAcquire: string
  spoiler?: boolean
}

export type JournalChoiceEcho = {
  choiceId: string
  optionId: string
  label: string
  chapterId: ChapterId
  echoIn: ChapterId
  delayedEcho: string
  madeAt: number
}

export type JournalState = {
  version: 1
  currentChapterId: ChapterId
  completedChapterIds: ChapterId[]
  clues: JournalClue[]
  choiceEchoes: JournalChoiceEcho[]
  relationNotes: { from: CharacterId; to: CharacterId; note: string }[]
  updatedAt: number
}

function seedClues(): JournalClue[] {
  return listSeasonChapters().flatMap((ch) => [
    {
      id: `clue.${ch.id}.explore`,
      chapterId: ch.id,
      title: `${ch.title} · 探索`,
      summary: ch.exploreNode.activity,
      status: ch.order === 0 ? 'available' : 'locked',
      altAcquire: ch.routes.homeMode,
    },
    {
      id: `clue.${ch.id}.collection`,
      chapterId: ch.id,
      title: `${ch.title} · 线索`,
      summary: ch.collectionTrigger.description,
      status: ch.order === 0 ? 'available' : 'locked',
      altAcquire: ch.routes.noAnimal,
    },
  ])
}

export function createInitialJournal(): JournalState {
  return {
    version: 1,
    currentChapterId: 'prologue.blank_page',
    completedChapterIds: [],
    clues: seedClues(),
    choiceEchoes: [],
    relationNotes: [],
    updatedAt: Date.now(),
  }
}

export function loadJournal(): JournalState {
  if (typeof localStorage === 'undefined') return createInitialJournal()
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return createInitialJournal()
    const parsed = JSON.parse(raw) as JournalState
    if (!parsed || parsed.version !== 1) return createInitialJournal()
    // merge new chapter clues if season pack grew
    const seed = seedClues()
    const have = new Set(parsed.clues.map((c) => c.id))
    for (const c of seed) {
      if (!have.has(c.id)) parsed.clues.push(c)
    }
    return parsed
  } catch {
    return createInitialJournal()
  }
}

export function saveJournal(state: JournalState): JournalState {
  const next = { ...state, updatedAt: Date.now() }
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
  } catch {
    /* quota — keep in-memory last-known-good */
  }
  return next
}

export function currentGoal(state: JournalState): { title: string; body: string } {
  const ch = getChapter(state.currentChapterId)
  if (!ch) return { title: '手账', body: '暂无章节' }
  return {
    title: `当前：${ch.title}`,
    body: ch.themeQuestion,
  }
}

export function lastEventSummary(state: JournalState): string {
  if (state.choiceEchoes.length) {
    const last = state.choiceEchoes[state.choiceEchoes.length - 1]
    return `上次选择：${last.label}（将在后续回响）`
  }
  const done = state.completedChapterIds
  if (done.length) {
    const ch = getChapter(done[done.length - 1])
    return `上次完成：${ch?.title ?? done[done.length - 1]}`
  }
  return '尚未记录选择；可从序章训练观察开始。'
}

export function activeChoiceInfluences(state: JournalState): JournalChoiceEcho[] {
  // choices whose echo chapter not yet completed
  return state.choiceEchoes.filter((e) => !state.completedChapterIds.includes(e.echoIn))
}

export function recordChoice(
  state: JournalState,
  input: {
    choiceId: string
    optionId: string
    label: string
    chapterId: ChapterId
    echoIn: ChapterId
    delayedEcho: string
  },
): JournalState {
  // idempotent by choiceId
  const without = state.choiceEchoes.filter((c) => c.choiceId !== input.choiceId)
  return saveJournal({
    ...state,
    choiceEchoes: [
      ...without,
      { ...input, madeAt: Date.now() },
    ],
  })
}

export function collectClue(state: JournalState, clueId: string): JournalState {
  const clues = state.clues.map((c) =>
    c.id === clueId && c.status !== 'locked' ? { ...c, status: 'collected' as const } : c,
  )
  return saveJournal({ ...state, clues })
}

export function markMissedWithAlt(state: JournalState, clueId: string): JournalState {
  const clues = state.clues.map((c) =>
    c.id === clueId && c.status !== 'collected'
      ? { ...c, status: 'missed' as const }
      : c,
  )
  return saveJournal({ ...state, clues })
}

export function completeChapter(state: JournalState, chapterId: ChapterId): JournalState {
  const chapters = listSeasonChapters()
  const idx = chapters.findIndex((c) => c.id === chapterId)
  const nextCh = chapters[idx + 1]
  const completed = state.completedChapterIds.includes(chapterId)
    ? state.completedChapterIds
    : [...state.completedChapterIds, chapterId]
  const clues = state.clues.map((c) => {
    if (c.chapterId === chapterId && c.status === 'available') {
      return { ...c, status: 'collected' as const }
    }
    if (nextCh && c.chapterId === nextCh.id && c.status === 'locked') {
      return { ...c, status: 'available' as const }
    }
    return c
  })
  return saveJournal({
    ...state,
    completedChapterIds: completed,
    currentChapterId: nextCh?.id ?? chapterId,
    clues,
  })
}

export function chapterNavItems(state: JournalState) {
  return listSeasonChapters().map((ch) => ({
    id: ch.id,
    title: ch.title,
    order: ch.order,
    status: state.completedChapterIds.includes(ch.id)
      ? ('done' as const)
      : ch.id === state.currentChapterId
        ? ('current' as const)
        : ('locked' as const),
    themeQuestion: ch.themeQuestion,
  }))
}

export function relationRecap() {
  return listCharacters().map((c) => ({
    id: c.id,
    name: c.displayName,
    role: c.role,
    fictional: c.fictional,
    desire: c.desire,
  }))
}

export function clueBoard(state: JournalState): JournalClue[] {
  return state.clues
}
