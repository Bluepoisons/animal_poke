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
    <div className="ap-resource-bar">
      <strong>
        {city} · {weather}
      </strong>
      <span>⚡ {energy}</span>
      <span>● {coins}</span>
    </div>
  )
}
