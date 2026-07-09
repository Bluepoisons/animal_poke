interface PageTitleProps {
  title: string
  rightText?: string
  rightTone?: 'yellow' | 'purple'
}

export default function PageTitle({
  title,
  rightText,
  rightTone = 'yellow',
}: PageTitleProps) {
  return (
    <>
      <h1 className="ap-page-title">{title}</h1>
      {rightText && (
        <div
          className={`ap-page-right ${
            rightTone === 'purple' ? 'ap-page-right--purple' : ''
          }`}
        >
          {rightText}
        </div>
      )}
    </>
  )
}
