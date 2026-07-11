import { ProgressBar } from '../../../a11y'
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
  return (
    <div className="ap-probability-card">
      <h2>{title}</h2>
      <div className="ap-probability" aria-hidden="true">
        <span style={{ width: '33%' }} />
        <span style={{ width: '34%' }} />
        <span style={{ width: '7%' }} />
        <span style={{ width: '26%' }} />
      </div>
      <ProgressBar
        value={Math.round(successRate * 100)}
        label="捕获成功率"
        valueText={`捕获成功率 ${Math.round(successRate * 100)}%，最佳力度 ${bestMin}-${bestMax}`}
        animated={false}
      />
      <p>
        捕获成功率 {Math.round(successRate * 100)}% · 最佳力度 {bestMin}-{bestMax}
      </p>
    </div>
  )
}
