interface PageTitleProps {
  title: string
  subtitle?: string
  rightText?: string
  rightTone?: 'yellow' | 'pink' | 'blue'
}

export default function PageTitle({
  title,
  subtitle,
  rightText,
  rightTone = 'yellow',
}: PageTitleProps) {
  const toneClass =
    rightTone === 'pink'
      ? 'ap-page-right--pink'
      : rightTone === 'blue'
        ? 'ap-page-right--blue'
        : ''

  return (
    <header className="ap-page-head">
      <div>
        <h1 className="ap-page-title">
          <span className="ap-highlight">{title}</span>
        </h1>
        {subtitle ? <p className="ap-page-sub">{subtitle}</p> : null}
      </div>
      {rightText ? (
        <div className={`ap-page-right ${toneClass}`}>{rightText}</div>
      ) : null}
    </header>
  )
}
