interface ActionButtonProps {
  children: React.ReactNode
  onClick?: () => void
  disabled?: boolean
}

export default function ActionButton({
  children,
  onClick,
  disabled = false,
}: ActionButtonProps) {
  return (
    <button
      className="ap-action-button"
      onClick={onClick}
      disabled={disabled}
      type="button"
    >
      {children}
    </button>
  )
}
