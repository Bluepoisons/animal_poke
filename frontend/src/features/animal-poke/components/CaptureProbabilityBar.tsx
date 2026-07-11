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
  return (
    <div className="ap-probability-card">
      <h2>{title}</h2>
      <div className="ap-probability" aria-label={t('capture.probability')}>
        <span style={{ width: '33%' }} />
        <span style={{ width: '34%' }} />
        <span style={{ width: '7%' }} />
        <span style={{ width: '26%' }} />
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
