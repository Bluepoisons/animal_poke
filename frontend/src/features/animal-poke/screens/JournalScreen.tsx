import { useMemo, useState } from 'react'
import {
  activeChoiceInfluences,
  chapterNavItems,
  clueBoard,
  collectClue,
  completeChapter,
  currentGoal,
  lastEventSummary,
  loadJournal,
  markMissedWithAlt,
  recordChoice,
  relationRecap,
  saveJournal,
  type JournalState,
} from '../../../narrative/journal'
import { getChapter } from '../../../narrative/content/season1/architecture'
import type { ChapterId } from '../../../narrative/content/season1/architecture'

type Tab = 'overview' | 'clues' | 'relations' | 'chapters' | 'echoes'

interface JournalScreenProps {
  onToast?: (msg: string) => void
}

/**
 * AP-121 城市手账：章节目标、线索板、关系回顾、选择回响。
 * 与图鉴分工：图鉴=动物事实；手账=人与城市故事。
 */
export default function JournalScreen({ onToast }: JournalScreenProps) {
  const [state, setState] = useState<JournalState>(() => loadJournal())
  const [tab, setTab] = useState<Tab>('overview')
  const [spoilerOn, setSpoilerOn] = useState(false)

  const goal = useMemo(() => currentGoal(state), [state])
  const last = useMemo(() => lastEventSummary(state), [state])
  const influences = useMemo(() => activeChoiceInfluences(state), [state])
  const clues = useMemo(() => clueBoard(state), [state])
  const chapters = useMemo(() => chapterNavItems(state), [state])
  const relations = useMemo(() => relationRecap(), [])
  const chapter = getChapter(state.currentChapterId)

  const commit = (next: JournalState) => {
    setState(saveJournal(next))
  }

  return (
    <div
      className="ap-screen"
      data-testid="journal-screen"
      style={{ padding: 16, overflow: 'auto' }}
    >
      <header style={{ marginBottom: 12 }}>
        <h1 style={{ fontSize: 20, margin: 0, color: '#4A2C1A' }}>城市手账</h1>
        <p style={{ fontSize: 13, color: '#8D6E63', margin: '4px 0 0' }}>
          记录人与城市的关系 · 图鉴另记动物事实
        </p>
      </header>

      <div
        role="tablist"
        aria-label="手账分区"
        style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 12 }}
      >
        {(
          [
            ['overview', '总览'],
            ['clues', '线索板'],
            ['relations', '关系'],
            ['chapters', '章节'],
            ['echoes', '回响'],
          ] as const
        ).map(([id, label]) => (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={tab === id}
            data-testid={`journal-tab-${id}`}
            className={tab === id ? 'is-active' : ''}
            onClick={() => setTab(id)}
            style={{
              padding: '8px 10px',
              borderRadius: 10,
              border: tab === id ? '2px solid #FF8A4C' : '1px solid #E0C0A0',
              background: tab === id ? '#FFE0C8' : '#FFF8F0',
            }}
          >
            {label}
          </button>
        ))}
      </div>

      <label style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 12, fontSize: 13 }}>
        <input
          type="checkbox"
          checked={spoilerOn}
          onChange={(e) => setSpoilerOn(e.target.checked)}
          data-testid="journal-spoiler-toggle"
        />
        显示可能剧透的回响全文
      </label>

      {tab === 'overview' && (
        <section data-testid="journal-overview" aria-labelledby="journal-goal">
          <h2 id="journal-goal" style={{ fontSize: 16, color: '#6D4C41' }}>
            {goal.title}
          </h2>
          <p data-testid="journal-current-goal" style={{ lineHeight: 1.5 }}>
            {goal.body}
          </p>
          <p data-testid="journal-last-event" style={{ fontSize: 14, color: '#4A2C1A' }}>
            {last}
          </p>
          <p data-testid="journal-active-influences" style={{ fontSize: 13, color: '#8D6E63' }}>
            仍有影响的选择：
            {influences.length
              ? influences.map((i) => i.label).join('、')
              : '暂无（完成选择后会出现）'}
          </p>
          {chapter && (
            <div style={{ marginTop: 12, padding: 12, borderRadius: 12, background: '#FFF3E0' }}>
              <p style={{ margin: 0, fontSize: 13 }}>Home Mode：{chapter.routes.homeMode}</p>
              <p style={{ margin: '6px 0 0', fontSize: 13 }}>无相机：{chapter.routes.noCamera}</p>
              <button
                type="button"
                data-testid="journal-complete-chapter"
                style={{ marginTop: 10 }}
                onClick={() => {
                  // demo: record chapter choice default first option if none
                  const ch = getChapter(state.currentChapterId)
                  if (ch && !state.choiceEchoes.some((e) => e.choiceId === ch.choice.id)) {
                    const opt = ch.choice.options[0]
                    commit(
                      recordChoice(state, {
                        choiceId: ch.choice.id,
                        optionId: opt.id,
                        label: opt.label,
                        chapterId: ch.id,
                        echoIn: ch.choice.echoIn,
                        delayedEcho: opt.delayedEcho,
                      }),
                    )
                  }
                  const next = completeChapter(loadJournal(), state.currentChapterId)
                  commit(next)
                  onToast?.(`已完成 ${chapter.title}`)
                }}
              >
                标记本章完成（演示）
              </button>
            </div>
          )}
        </section>
      )}

      {tab === 'clues' && (
        <section data-testid="journal-clue-board" aria-label="线索板">
          <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
            {clues.map((c) => (
              <li
                key={c.id}
                data-testid={`journal-clue-${c.status}`}
                style={{
                  border: '1px solid #E0C0A0',
                  borderRadius: 12,
                  padding: 10,
                  marginBottom: 8,
                  background: '#FFFDF8',
                }}
              >
                <strong>{c.title}</strong>
                <span style={{ marginLeft: 8, fontSize: 12, color: '#8D6E63' }}>{c.status}</span>
                {c.status === 'locked' ? (
                  <p style={{ margin: '6px 0 0', fontSize: 13 }}>
                    尚未解锁 · 可替代：{c.altAcquire}
                  </p>
                ) : (
                  <>
                    <p style={{ margin: '6px 0 0', fontSize: 13 }}>{c.summary}</p>
                    {c.status === 'missed' && (
                      <p style={{ margin: '4px 0 0', fontSize: 12, color: '#B45309' }}>
                        已错过 · 替代获取：{c.altAcquire}
                      </p>
                    )}
                    {c.status === 'available' && (
                      <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
                        <button
                          type="button"
                          onClick={() => commit(collectClue(state, c.id))}
                          data-testid="journal-collect-clue"
                        >
                          收集
                        </button>
                        <button
                          type="button"
                          onClick={() => commit(markMissedWithAlt(state, c.id))}
                          data-testid="journal-miss-clue"
                        >
                          标记错过
                        </button>
                      </div>
                    )}
                  </>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}

      {tab === 'relations' && (
        <section data-testid="journal-relations" aria-label="角色关系">
          <ul style={{ listStyle: 'none', padding: 0 }}>
            {relations.map((r) => (
              <li key={r.id} style={{ marginBottom: 10, padding: 10, borderRadius: 12, background: '#FFF8F0' }}>
                <strong>
                  {r.name}
                  {r.fictional ? '（虚构）' : ''}
                </strong>
                <span style={{ color: '#8D6E63' }}> · {r.role}</span>
                <p style={{ margin: '4px 0 0', fontSize: 13 }}>{r.desire}</p>
              </li>
            ))}
          </ul>
        </section>
      )}

      {tab === 'chapters' && (
        <nav data-testid="journal-chapter-nav" aria-label="章节导航">
          <ol style={{ paddingLeft: 18 }}>
            {chapters.map((ch) => (
              <li key={ch.id} style={{ marginBottom: 8 }}>
                <button
                  type="button"
                  data-testid={`journal-chapter-${ch.status}`}
                  disabled={ch.status === 'locked'}
                  onClick={() => {
                    if (ch.status === 'locked') return
                    commit({ ...state, currentChapterId: ch.id as ChapterId })
                    setTab('overview')
                  }}
                  style={{
                    textAlign: 'left',
                    width: '100%',
                    padding: 8,
                    borderRadius: 8,
                    border: '1px solid #E0C0A0',
                    background: ch.status === 'current' ? '#FFE0C8' : '#FFFDF8',
                    opacity: ch.status === 'locked' ? 0.55 : 1,
                  }}
                >
                  {ch.order}. {ch.title} · {ch.status}
                  <div style={{ fontSize: 12, color: '#6D4C41' }}>{ch.themeQuestion}</div>
                </button>
              </li>
            ))}
          </ol>
        </nav>
      )}

      {tab === 'echoes' && (
        <section data-testid="journal-echoes" aria-label="选择回响">
          {state.choiceEchoes.length === 0 ? (
            <p>尚无选择回响。</p>
          ) : (
            <ul style={{ listStyle: 'none', padding: 0 }}>
              {state.choiceEchoes.map((e) => (
                <li key={e.choiceId} style={{ marginBottom: 10, padding: 10, borderRadius: 12, border: '1px solid #E0C0A0' }}>
                  <strong>{e.label}</strong>
                  <p style={{ fontSize: 12, color: '#8D6E63' }}>
                    于 {e.chapterId} · 回响在 {e.echoIn}
                  </p>
                  <p style={{ fontSize: 13, filter: spoilerOn ? 'none' : 'blur(4px)' }}>
                    {e.delayedEcho}
                  </p>
                </li>
              ))}
            </ul>
          )}
        </section>
      )}
    </div>
  )
}
