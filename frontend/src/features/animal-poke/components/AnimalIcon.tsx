interface AnimalIconProps {
  species: 'cat' | 'goose' | 'dog' | 'unknown'
  size?: number
  tone?: 'light' | 'dark' | 'muted'
}

export default function AnimalIcon({
  species,
  size = 96,
  tone = 'light',
}: AnimalIconProps) {
  const toneClass =
    tone === 'dark' ? 'ap-animal--dark' : tone === 'muted' ? 'ap-animal--muted' : 'ap-animal--light'

  if (species === 'unknown') {
    return (
      <svg
        className={`ap-animal ${toneClass}`}
        width={size}
        height={size}
        viewBox="0 0 120 120"
        aria-hidden="true"
      >
        <text x="60" y="80" textAnchor="middle" fontSize="48" fontWeight="900" fill="currentColor">
          ???
        </text>
      </svg>
    )
  }

  const paths: Record<'cat' | 'goose' | 'dog', string> = {
    goose: 'M39 28c18-14 40-5 42 15 2 18-3 29 22 30l-13 23c-19-2-34-10-42-24-1 12 2 21 11 28-12 5-24 4-34-1 9-8 12-18 9-30-4-13-4-26 5-41Z',
    cat: 'M30 50 27 18l25 21h16l25-21-3 32c10 14 7 34-9 45-15 11-43 11-58 0-16-11-19-31-9-45Z',
    dog: 'M38 28c14-9 35-9 49 0 17 11 19 40 5 57-15 18-50 18-65 0-14-17-12-46 11-57Z',
  }

  const ears: Record<'cat' | 'goose' | 'dog', string> = {
    goose: 'M38 30 21 37l18 5',
    cat: '',
    dog: '',
  }

  const extraBody: Record<'cat' | 'goose' | 'dog', JSX.Element | null> = {
    goose: (
      <>
        <circle cx="55" cy="33" r="4" fill="currentColor" />
        <path d="M51 71c12 12 28 13 42 4M36 99c-4 8-10 10-20 10M57 101c5 7 14 8 24 5" fill="none" stroke="currentColor" strokeWidth="7" strokeLinecap="round" />
      </>
    ),
    cat: (
      <>
        <circle cx="46" cy="63" r="5" fill="currentColor" />
        <circle cx="74" cy="63" r="5" fill="currentColor" />
        <path d="M60 72v8M48 84c8 6 16 6 24 0M35 72H14M37 82H17M85 72h21M83 82h20" fill="none" stroke="currentColor" strokeWidth="6" strokeLinecap="round" />
      </>
    ),
    dog: (
      <>
        <path d="M35 30c-14 0-24 12-21 30 2 14 14 18 24 12M85 30c14 0 24 12 21 30-2 14-14 18-24 12" fill="currentColor" />
        <circle cx="48" cy="61" r="5" fill="currentColor" />
        <circle cx="73" cy="61" r="5" fill="currentColor" />
        <path d="M60 71v7M48 82c8 8 16 8 24 0" fill="none" stroke="currentColor" strokeWidth="6" strokeLinecap="round" />
      </>
    ),
  }

  return (
    <svg
      className={`ap-animal ${toneClass}`}
      width={size}
      height={size}
      viewBox="0 0 120 120"
      aria-hidden="true"
    >
      <path
        d={paths[species]}
        fill="none"
        stroke="currentColor"
        strokeWidth="7"
        strokeLinejoin="round"
      />
      {ears[species] && (
        <path
          d={ears[species]}
          fill="none"
          stroke="currentColor"
          strokeWidth="7"
          strokeLinejoin="round"
        />
      )}
      {extraBody[species]}
    </svg>
  )
}
