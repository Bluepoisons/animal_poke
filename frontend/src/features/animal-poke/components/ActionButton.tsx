interface ActionButtonProps {
  children: React.ReactNode
  onClick?: () => void
  disabled?: boolean
  tone?: 'pink' | 'blue' | 'yellow'
}

export default function ActionButton({
  children,
  onClick,
  disabled = false,
  tone = 'pink',
}: ActionButtonProps) {
  const toneClass =
    tone === 'blue'
      ? 'ap-action-button--blue'
      : tone === 'yellow'
        ? 'ap-action-button--yellow'
        : ''

  return (
    <button
      className={`ap-action-button ${toneClass}`}
      onClick={onClick}
      disabled={disabled}
      type="button"
    >
      {children}
    </button>
  )
}
