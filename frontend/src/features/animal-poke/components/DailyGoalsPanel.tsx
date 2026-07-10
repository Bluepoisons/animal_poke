import type { GoalProgress, ReturnSummary } from '../../../progression/types'
import type { GoalNavigateTo } from '../../../progression/types'
import type { ScreenId } from '../data/types'

interface DailyGoalsPanelProps {
  goals: GoalProgress[]
  returnSummary: ReturnSummary
  showReturnBanner: boolean
  onDismissReturn: () => void
  onNavigate: (screen: ScreenId) => void
  onSafeExplore: () => void
  onSeasonCheckin: () => void
  staminaEmpty: boolean
}

function navTarget(to: GoalNavigateTo): ScreenId | null {
  if (to === 'none' || to === 'dispatch') return null
  return to
}

export default function DailyGoalsPanel({
  goals,
  returnSummary,
  showReturnBanner,
  onDismissReturn,
  onNavigate,
  onSafeExplore,
  onSeasonCheckin,
  staminaEmpty,
}: DailyGoalsPanelProps) {
  const handleGoalClick = (g: GoalProgress) => {
    if (g.completed || !g.executable) return
    if (g.def.action === 'safe_explore') {
      onSafeExplore()
      const screen = navTarget(g.def.navigateTo)
      if (screen) onNavigate(screen)
      return
    }
    if (g.def.action === 'season_checkin') {
      onSeasonCheckin()
      return
    }
    const screen = navTarget(g.def.navigateTo)
    if (screen) onNavigate(screen)
  }

  return (
    <section className="ap-goals" aria-label="今日目标">
      {showReturnBanner && returnSummary.isReturning && (
        <div className="ap-goals__return" role="status">
          <div className="ap-goals__return-head">
            <strong>{returnSummary.headline}</strong>
            <button type="button" className="ap-goals__dismiss" onClick={onDismissReturn} aria-label="关闭回流提示">
              ×
            </button>
          </div>
          <ul className="ap-goals__return-lines">
            {returnSummary.summaryLines.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
          {returnSummary.nextStep && (
            <button
              type="button"
              className="ap-goals__next"
              onClick={() => handleGoalClick(returnSummary.nextStep!)}
            >
              下一步：{returnSummary.nextStep.def.title}
            </button>
          )}
        </div>
      )}

      <div className="ap-goals__header">
        <span className="ap-goals__eyebrow">DAILY GOALS</span>
        <h2 className="ap-goals__title">今日目标</h2>
        {staminaEmpty && (
          <p className="ap-goals__hint">体力已耗尽 · 仍可做免费活动</p>
        )}
      </div>

      <ol className="ap-goals__list">
        {goals.map((g) => {
          const pct = Math.round((g.current / Math.max(1, g.target)) * 100)
          return (
            <li key={g.def.id} className={`ap-goals__item ${g.completed ? 'is-done' : ''} ${g.executable ? 'is-exec' : ''}`}>
              <button
                type="button"
                className="ap-goals__btn"
                onClick={() => handleGoalClick(g)}
                disabled={g.completed || !g.executable}
              >
                <div className="ap-goals__row">
                  <span className="ap-goals__name">
                    {g.def.free ? '○ ' : '● '}
                    {g.def.title}
                  </span>
                  <span className="ap-goals__count">
                    {g.current}/{g.target}
                  </span>
                </div>
                <p className="ap-goals__desc">{g.def.description}</p>
                <div className="ap-goals__bar" aria-hidden="true">
                  <span style={{ width: `${Math.min(100, pct)}%` }} />
                </div>
                {g.def.free && <span className="ap-goals__free">免费</span>}
              </button>
            </li>
          )
        })}
      </ol>
    </section>
  )
}
