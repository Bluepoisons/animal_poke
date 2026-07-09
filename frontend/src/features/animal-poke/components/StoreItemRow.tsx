import type { InventoryItem } from '../data/types'

interface StoreItemRowProps {
  item: InventoryItem
  disabled?: boolean
  onClick?: () => void
}

const iconLabels: Record<InventoryItem['icon'], string> = {
  ball: '球',
  'super-ball': '高级',
  bait: '饵',
  potion: '药',
}

export default function StoreItemRow({
  item,
  disabled = false,
  onClick,
}: StoreItemRowProps) {
  return (
    <div
      className={`ap-store-item ${disabled ? 'ap-store-item--disabled' : ''}`}
      onClick={disabled ? undefined : onClick}
      onKeyDown={(event) => {
        if (disabled) return
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          onClick?.()
        }
      }}
      role="button"
      tabIndex={disabled ? -1 : 0}
      aria-disabled={disabled}
    >
      <span
        className={`ap-store-item__icon ap-store-item__icon--${item.icon}`}
        aria-hidden="true"
      >
        {iconLabels[item.icon]}
      </span>
      <span className="ap-store-item__meta">
        <strong>{item.name}</strong>
        <span>{item.effect}</span>
      </span>
      <span className="ap-store-item__price">{item.price}</span>
    </div>
  )
}
