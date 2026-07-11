/**
 * AccessibleDialog — shared modal with focus trap, Escape close, focus restoration.
 * AP-071: Replaces ad-hoc dialogs across features.
 */
import { useRef } from 'react'
import { useFocusTrap } from './index'

export interface AccessibleDialogProps {
  open: boolean
  onClose: () => void
  title: string
  children: React.ReactNode
  /** ID of the trigger element to restore focus when dialog closes. */
  triggerId?: string
}

export default function AccessibleDialog({
  open, onClose, title, children, triggerId,
}: AccessibleDialogProps): JSX.Element | null {
  const containerRef = useRef<HTMLDivElement>(null)

  useFocusTrap({
    containerRef,
    active: open,
    onEscape: onClose,
    triggerId,
    closeOnBackdrop: true,
  })

  if (!open) return null

  return (
    <div
      data-trap-backdrop
      className="ap-dialog-backdrop"
      onClick={onClose}
      role="presentation"
    >
      <div
        ref={containerRef}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="ap-trap-container"
        style={{
          background: '#FFFDF8',
          borderRadius: 16,
          padding: 16,
          maxWidth: 340,
          width: '100%',
          border: '2px solid #2B2B2B',
          outline: 'none',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {children}
      </div>
    </div>
  )
}
