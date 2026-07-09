import type { InventoryItem } from '../data/types'

interface StoreItemRowProps {
  item: InventoryItem
  disabled?: boolean
  onClick?: () => void
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
      role="button"
      tabIndex={disabled ? -1 : 0}
      aria-disabled={disabled}
    >
      <span className="ap-store-item__icon">{item.icon}</span>
      <span>
        {item.name} {item.effect}
      </span>
      <span className="ap-store-item__price">{item.price}</span>
    </div>
  )
}
