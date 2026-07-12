import { ProgressBar } from '../../../a11y'
import { useI18n } from '../../../i18n'
interface CaptureProbabilityBarProps {
  title: string
  successRate: number
  bestMin: number
  bestMax: number
}

export default function CaptureProbabilityBar({
  title,
  successRate,
  bestMin,
  bestMax,
}: CaptureProbabilityBarProps) {
  const { t } = useI18n()
  const rangeStart = Math.min(100, Math.max(0, bestMin))
  const rangeEnd = Math.min(100, Math.max(rangeStart, bestMax))
  return (
    <div className="ap-probability-card">
      <h2>{title}</h2>
      <div className="ap-probability" aria-label={t('capture.probability')}>
        <span style={{ width: `${rangeStart}%` }} />
        <span style={{ width: `${rangeEnd - rangeStart}%` }} />
        <span style={{ width: `${100 - rangeEnd}%` }} />
      </div>
      <ProgressBar
        value={Math.round(successRate * 100)}
        label={t('capture.probability')}
        valueText={t('capture.probability_detail', { rate: Math.round(successRate * 100), min: bestMin, max: bestMax })}
        animated={false}
      />
      <p>{t('capture.probability_detail', { rate: Math.round(successRate * 100), min: bestMin, max: bestMax })}</p>
    </div>
  )
}
