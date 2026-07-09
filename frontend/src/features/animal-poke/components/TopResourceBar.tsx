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
  return (
    <div className="ap-resource-bar" aria-label="资源栏">
      <span className="ap-resource-bar__chip ap-resource-bar__chip--city">
        {city} · {weather}
      </span>
      <span className="ap-resource-bar__chip ap-resource-bar__chip--energy" aria-label={`体力 ${energy}`}>
        <span className="ap-resource-bar__bolt" aria-hidden="true" />
        {energy}
      </span>
      <span className="ap-resource-bar__chip ap-resource-bar__chip--coins" aria-label={`金币 ${coins}`}>
        <span className="ap-resource-bar__dot" aria-hidden="true" />
        {coins}
      </span>
    </div>
  )
}
