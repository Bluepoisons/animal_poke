import { useI18n } from '../../../i18n'
interface TopResourceBarProps {
  city: string
  weather: string
  energy: number
  coins: number
}

export default function TopResourceBar({
  city,
  weather,
  energy,
  coins,
}: TopResourceBarProps) {
  const { t } = useI18n()
  return (
    <div className="ap-resource-bar" aria-label={t('resource.label')}>
      <span className="ap-resource-bar__chip ap-resource-bar__chip--city">
        {city} · {weather}
      </span>
      <span className="ap-resource-bar__chip ap-resource-bar__chip--energy" aria-label={t('resource.energy', { value: energy })}>
        <span className="ap-resource-bar__bolt" aria-hidden="true" />
        {energy}
      </span>
      <span className="ap-resource-bar__chip ap-resource-bar__chip--coins" aria-label={t('resource.gold', { value: coins })}>
        <span className="ap-resource-bar__dot" aria-hidden="true" />
        {coins}
      </span>
    </div>
  )
}
