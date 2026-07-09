import type { HuntTarget } from '../data/types'

interface DiscoveryPinProps {
  target: HuntTarget
  selected?: boolean
  onSelect?: () => void
}

export default function DiscoveryPin({
  target,
  selected = false,
  onSelect,
}: DiscoveryPinProps) {
  const className = `ap-pin ap-pin--${target.rarity} ${selected ? 'is-selected' : ''}`

  return (
    <div
      className="ap-pin-wrapper"
      style={{ left: `${target.x * 100}%`, top: `${target.y * 100}%` }}
    >
      <button
        className={className}
        onClick={onSelect}
        aria-label={target.label}
        type="button"
      />
      <div className="ap-pin-label">{target.label}</div>
    </div>
  )
}
