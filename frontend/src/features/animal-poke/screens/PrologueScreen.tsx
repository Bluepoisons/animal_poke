import { useMemo, useState } from 'react'
import { PresentationPlayer } from '../../../narrative/presentation'
import {
  prologueSlice,
  getPrologueSequence,
} from '../../../narrative/content/prologue'
import {
  completeChapter,
  loadJournal,
  recordChoice,
  saveJournal,
} from '../../../narrative/journal'
import { getChapter } from '../../../narrative/content/season1/architecture'
import { PROLOGUE_CHAPTER_ID } from '../../../narrative/content/prologue/blankPage'

interface PrologueScreenProps {
  onToast?: (msg: string) => void
  onFinished?: () => void
}

/**
 * AP-124 playable vertical slice: train observe → contradiction → choice → hook.
 * No camera / location required.
 */
export default function PrologueScreen({ onToast, onFinished }: PrologueScreenProps) {
  const [beatIndex, setBeatIndex] = useState(0)
  const beat = prologueSlice.beats[beatIndex]
  const sequence = useMemo(
    () => (beat ? getPrologueSequence(beat.sequenceId) : undefined),
    [beat],
  )
  const chapter = getChapter(PROLOGUE_CHAPTER_ID)

  if (!beat || !sequence) {
    return (
      <div className="ap-screen" data-testid="prologue-screen" style={{ padding: 16 }}>
        <p>序章数据缺失</p>
      </div>
    )
  }

  return (
    <div className="ap-screen" data-testid="prologue-screen" style={{ padding: 12, overflow: 'auto' }}>
      <header style={{ marginBottom: 8 }}>
        <h1 style={{ fontSize: 18, margin: 0 }}>{prologueSlice.title}</h1>
        <p style={{ fontSize: 12, color: '#8D6E63', margin: '4px 0 0' }}>
          节拍 {beatIndex + 1}/{prologueSlice.beats.length} · {beat.title} · 约 {beat.minutes} 分钟
        </p>
        <p style={{ fontSize: 12, color: '#6D4C41' }}>无需相机与定位 · Home Mode 可完成</p>
      </header>

      <PresentationPlayer
        key={sequence.id}
        sequence={sequence}
        storageKey={`ap-prologue-cp:${sequence.id}`}
        onChoice={(segmentId, choiceId) => {
          if (!chapter) return
          const opt = chapter.choice.options.find((o) => o.id === choiceId)
          if (!opt) return
          const j = loadJournal()
          saveJournal(
            recordChoice(j, {
              choiceId: chapter.choice.id,
              optionId: opt.id,
              label: opt.label,
              chapterId: chapter.id,
              echoIn: chapter.choice.echoIn,
              delayedEcho: opt.delayedEcho,
            }),
          )
          onToast?.(`已记录：${opt.label}`)
        }}
        onComplete={() => {
          const next = beatIndex + 1
          if (next < prologueSlice.beats.length) {
            setBeatIndex(next)
            onToast?.(`进入：${prologueSlice.beats[next].title}`)
            return
          }
          // finish slice → unlock ch01 via journal
          const j = completeChapter(loadJournal(), PROLOGUE_CHAPTER_ID)
          saveJournal(j)
          onToast?.('序章完成 · 开启《巷口的回声》')
          onFinished?.()
        }}
      />
    </div>
  )
}
