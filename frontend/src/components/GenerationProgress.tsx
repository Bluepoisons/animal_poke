import React from 'react'
import type { GenerationStage, GeneratedAnimal, PipelineProgress } from '../services/capturePipeline'

export interface GenerationProgressProps {
  progress: PipelineProgress
  onRetry?: () => void
  onCancel?: () => void
  onClose?: () => void
}

const STAGE_ORDER: GenerationStage[] = ['upload', 'analyze', 'value', 'save', 'done']

function stageLabel(s: GenerationStage): string {
  switch (s) {
    case 'upload':
      return '上传'
    case 'analyze':
      return '分析'
    case 'value':
      return '生成'
    case 'save':
      return '保存'
    case 'done':
      return '完成'
    default:
      return s
  }
}

const GenerationProgress: React.FC<GenerationProgressProps> = ({
  progress,
  onRetry,
  onCancel,
  onClose,
}) => {
  const result = progress.result
  const busy = ['upload', 'analyze', 'value', 'save'].includes(progress.stage)

  return (
    <div style={styles.overlay} role="dialog" aria-modal="true" aria-label="生成进度">
      <div style={styles.card}>
        <div style={styles.title}>✨ 生成宠物档案</div>
        <div style={styles.barTrack}>
          <div style={{ ...styles.barFill, width: `${progress.percent}%` }} />
        </div>
        <div style={styles.message}>{progress.message}</div>

        <div style={styles.steps}>
          {STAGE_ORDER.filter((s) => s !== 'done').map((s) => {
            const idx = STAGE_ORDER.indexOf(s)
            const cur = STAGE_ORDER.indexOf(progress.stage === 'error' || progress.stage === 'cancelled' ? 'upload' : progress.stage)
            const done = progress.stage === 'done' || (cur > idx && progress.stage !== 'idle')
            const active = progress.stage === s
            return (
              <span
                key={s}
                style={{
                  ...styles.step,
                  ...(done ? styles.stepDone : {}),
                  ...(active ? styles.stepActive : {}),
                }}
              >
                {stageLabel(s)}
              </span>
            )
          })}
        </div>

        {progress.stage === 'error' && (
          <div style={styles.errorBox}>
            <div style={{ fontWeight: 700 }}>生成失败</div>
            <div style={{ fontSize: 12, marginTop: 4 }}>{progress.error}</div>
            <div style={styles.actions}>
              {onRetry && (
                <button className="btn btn-primary" style={styles.btn} onClick={onRetry}>
                  从失败阶段重试
                </button>
              )}
              {onClose && (
                <button className="btn" style={styles.btnSecondary} onClick={onClose}>
                  关闭
                </button>
              )}
            </div>
          </div>
        )}

        {progress.stage === 'cancelled' && (
          <div style={styles.actions}>
            {onRetry && (
              <button className="btn btn-primary" style={styles.btn} onClick={onRetry}>
                重新生成
              </button>
            )}
            {onClose && (
              <button className="btn" style={styles.btnSecondary} onClick={onClose}>
                关闭
              </button>
            )}
          </div>
        )}

        {busy && onCancel && (
          <button className="btn" style={{ ...styles.btnSecondary, marginTop: 12 }} onClick={onCancel}>
            取消
          </button>
        )}

        {progress.stage === 'done' && result && <ResultSummary result={result} onClose={onClose} />}
      </div>
    </div>
  )
}

function ResultSummary({
  result,
  onClose,
}: {
  result: GeneratedAnimal
  onClose?: () => void
}) {
  const { analysis, value } = result
  return (
    <div style={styles.result}>
      <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--orange-dark)' }}>
        {analysis.breed} · ★{value.rarity} · {value.class}/{value.element}
      </div>
      <div style={{ fontSize: 12, color: 'var(--ink-2)', marginTop: 6 }}>
        HP {value.hp} · ATK {value.atk} · DEF {value.def} · SPD {value.spd}
      </div>
      <div style={{ fontSize: 12, color: 'var(--ink-3)', marginTop: 8, lineHeight: 1.4 }}>
        {value.narrative}
      </div>
      {onClose && (
        <button className="btn btn-primary" style={{ ...styles.btn, marginTop: 14 }} onClick={onClose}>
          收入图鉴
        </button>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(74,44,26,0.35)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 1000,
    padding: 16,
  },
  card: {
    width: '100%',
    maxWidth: 340,
    background: 'var(--white, #fff)',
    borderRadius: 20,
    padding: '20px 18px',
    boxShadow: '0 8px 0 rgba(230,115,0,0.12), 0 4px 20px rgba(74,44,26,0.15)',
  },
  title: {
    fontSize: 16,
    fontWeight: 700,
    color: 'var(--orange-dark, #E67300)',
    marginBottom: 12,
  },
  barTrack: {
    height: 10,
    borderRadius: 8,
    background: 'var(--orange-50, #FFF0E0)',
    overflow: 'hidden',
  },
  barFill: {
    height: '100%',
    background: 'var(--orange, #FF8C42)',
    transition: 'width 0.25s ease',
  },
  message: {
    marginTop: 10,
    fontSize: 13,
    color: 'var(--ink-2, #4A2C1A)',
    fontWeight: 600,
  },
  steps: {
    display: 'flex',
    gap: 6,
    marginTop: 12,
    flexWrap: 'wrap' as const,
  },
  step: {
    fontSize: 11,
    padding: '3px 8px',
    borderRadius: 12,
    border: '2px solid var(--orange-100, #FFD8B5)',
    color: 'var(--ink-3, #8B6B55)',
    background: 'var(--cream, #FFF8F0)',
  },
  stepActive: {
    border: '2px solid var(--orange, #FF8C42)',
    color: 'var(--orange-dark, #E67300)',
    fontWeight: 700,
  },
  stepDone: {
    background: 'var(--orange, #FF8C42)',
    color: '#fff',
    border: '2px solid var(--orange, #FF8C42)',
  },
  errorBox: {
    marginTop: 12,
    padding: 10,
    borderRadius: 12,
    background: 'rgba(220,38,38,0.08)',
    color: 'var(--ink, #4A2C1A)',
  },
  actions: {
    display: 'flex',
    gap: 8,
    marginTop: 12,
  },
  btn: {
    flex: 1,
    padding: '8px 0',
    borderRadius: 14,
    fontFamily: 'inherit',
  },
  btnSecondary: {
    flex: 1,
    padding: '8px 0',
    borderRadius: 14,
    fontFamily: 'inherit',
    background: 'var(--white, #fff)',
    border: '2px solid var(--orange-100, #FFD8B5)',
  },
  result: {
    marginTop: 14,
    paddingTop: 12,
    borderTop: '1px solid var(--orange-100, #FFD8B5)',
  },
}

export default React.memo(GenerationProgress)
