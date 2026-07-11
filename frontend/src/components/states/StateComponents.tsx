/**
 * AP-073 standard state components.
 *
 * Seven reusable states: Loading, Empty, NoResult, Error (recoverable),
 * Fatal, Offline, Stale.
 *
 * Each provides: icon, title, body, primary/secondary action, request ID,
 * screen-reader semantics, and data-testid.
 */
import type { ReactNode } from 'react'
import './states.css'

// ---- Shared props ----

export interface StateAction {
  label: string
  onClick: () => void
  testId?: string
}

export interface StateProps {
  title: string
  body?: string
  /** Request ID for support / audit */
  requestId?: string
  /** Primary action (e.g. Retry) */
  primary?: StateAction
  /** Secondary action (e.g. Go back) */
  secondary?: StateAction
  className?: string
  children?: ReactNode
}

// ---- Icon components ----

function IconWrapper({ children }: { children: ReactNode }) {
  return (
    <div className="ap-state-icon" aria-hidden="true">
      {children}
    </div>
  )
}

function ActionButton({ action, variant = 'primary' }: { action: StateAction; variant?: 'primary' | 'secondary' }) {
  return (
    <button
      type="button"
      className={`ap-state-action ap-state-action--${variant}`}
      onClick={action.onClick}
      data-testid={action.testId ?? 'state-action'}
    >
      {action.label}
    </button>
  )
}

function RequestIdLine({ requestId }: { requestId?: string }) {
  if (!requestId) return null
  return (
    <code className="ap-state-request-id" data-testid="state-request-id" aria-label={`请求 ID: ${requestId}`}>
      {requestId}
    </code>
  )
}

function StateContainer({ testId, title, body, requestId, primary, secondary, className, children }: StateProps & { testId: string }) {
  return (
    <div className={`ap-state ${className ?? ''}`.trim()} data-testid={testId} role="status">
      {children}
      <h2 className="ap-state-title">{title}</h2>
      {body && <p className="ap-state-body">{body}</p>}
      <div className="ap-state-actions">
        {primary && <ActionButton action={primary} variant="primary" />}
        {secondary && <ActionButton action={secondary} variant="secondary" />}
      </div>
      <RequestIdLine requestId={requestId} />
    </div>
  )
}

// ---- Loading ----

export function LoadingState(props: StateProps) {
  return (
    <StateContainer {...props} testId="loading-state">
      <IconWrapper>
        <span className="ap-state-spinner" />
      </IconWrapper>
    </StateContainer>
  )
}

// ---- Empty ----

const EmptyIcon = () => (
  <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
    <rect x="8" y="12" width="32" height="28" rx="4" stroke="currentColor" strokeWidth="2" />
    <path d="M16 20h16M16 26h10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    <circle cx="38" cy="8" r="6" fill="#F2E66B" stroke="currentColor" strokeWidth="2" />
  </svg>
)

export function EmptyState(props: StateProps) {
  return (
    <StateContainer {...props} testId="empty-state">
      <IconWrapper><EmptyIcon /></IconWrapper>
    </StateContainer>
  )
}

// ---- NoResult ----

const SearchIcon = () => (
  <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
    <circle cx="22" cy="22" r="12" stroke="currentColor" strokeWidth="2" />
    <path d="M31 31l8 8" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    <path d="M24 22h-4M22 20v4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
  </svg>
)

export function NoResultState(props: StateProps) {
  return (
    <StateContainer {...props} testId="no-result-state">
      <IconWrapper><SearchIcon /></IconWrapper>
    </StateContainer>
  )
}

// ---- Recoverable Error ----

const AlertIcon = () => (
  <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
    <circle cx="24" cy="24" r="20" stroke="#E5734A" strokeWidth="2" />
    <path d="M24 14v10M24 30v0" stroke="#E5734A" strokeWidth="2" strokeLinecap="round" />
  </svg>
)

export function ErrorState(props: StateProps) {
  return (
    <StateContainer {...props} testId="error-state">
      <IconWrapper><AlertIcon /></IconWrapper>
    </StateContainer>
  )
}

// ---- Fatal ----

const FatalIcon = () => (
  <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
    <rect x="4" y="4" width="40" height="40" rx="8" stroke="#C0392B" strokeWidth="2" />
    <path d="M18 18l12 12M30 18L18 30" stroke="#C0392B" strokeWidth="2" strokeLinecap="round" />
  </svg>
)

export function FatalState(props: StateProps) {
  return (
    <StateContainer {...props} testId="fatal-state">
      <IconWrapper><FatalIcon /></IconWrapper>
    </StateContainer>
  )
}

// ---- Offline ----

const OfflineIcon = () => (
  <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
    <path d="M12 30c-4-4-4-10 0-14M36 30c4-4 4-10 0-14M16 24c-3-3-3-8 0-10M32 24c3-3 3-8 0-10" stroke="#8D6E63" strokeWidth="2" strokeLinecap="round" />
    <circle cx="24" cy="28" r="4" stroke="#8D6E63" strokeWidth="2" />
    <path d="M8 8l32 32" stroke="#E5734A" strokeWidth="2" strokeLinecap="round" />
  </svg>
)

export function OfflineState(props: StateProps) {
  return (
    <StateContainer {...props} testId="offline-state">
      <IconWrapper><OfflineIcon /></IconWrapper>
    </StateContainer>
  )
}

// ---- Stale ----

const ClockIcon = () => (
  <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
    <circle cx="24" cy="24" r="20" stroke="#F2E66B" strokeWidth="2" />
    <path d="M24 14v10l6 6" stroke="#F2E66B" strokeWidth="2" strokeLinecap="round" />
  </svg>
)

export function StaleState(props: StateProps) {
  return (
    <StateContainer {...props} testId="stale-state">
      <IconWrapper><ClockIcon /></IconWrapper>
    </StateContainer>
  )
}
