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
    tone === 'dark'
      ? 'ap-animal--dark'
      : tone === 'muted'
        ? 'ap-animal--muted'
        : 'ap-animal--light'

  if (species === 'unknown') {
    return (
      <svg
        className={`ap-animal ${toneClass}`}
        width={size}
        height={size}
        viewBox="0 0 120 120"
        aria-hidden="true"
      >
        <circle
          cx="60"
          cy="60"
          r="42"
          fill="none"
          stroke="currentColor"
          strokeWidth="5"
          strokeDasharray="8 6"
        />
        <text
          x="60"
          y="72"
          textAnchor="middle"
          fontSize="28"
          fontWeight="700"
          fill="currentColor"
          fontFamily="Patrick Hand, ZCOOL KuaiLe, sans-serif"
        >
          ???
        </text>
      </svg>
    )
  }

  if (species === 'cat') {
    return (
      <svg
        className={`ap-animal ${toneClass}`}
        width={size}
        height={size}
        viewBox="0 0 120 120"
        aria-hidden="true"
      >
        <path
          d="M34 48 28 18l24 20h16l24-20-6 30c12 12 10 34-4 46-14 12-42 12-56 0-14-12-16-34-4-46Z"
          fill="rgba(255,158,198,0.25)"
          stroke="currentColor"
          strokeWidth="5"
          strokeLinejoin="round"
        />
        <circle cx="46" cy="62" r="4.5" fill="currentColor" />
        <circle cx="74" cy="62" r="4.5" fill="currentColor" />
        <path
          d="M60 70v8M48 84c8 6 16 6 24 0M34 70H16M36 80H18M86 70h18M84 80h18"
          fill="none"
          stroke="currentColor"
          strokeWidth="4.5"
          strokeLinecap="round"
        />
        <path
          d="M54 74c2 3 10 3 12 0"
          fill="none"
          stroke="currentColor"
          strokeWidth="3.5"
          strokeLinecap="round"
        />
      </svg>
    )
  }

  if (species === 'dog') {
    return (
      <svg
        className={`ap-animal ${toneClass}`}
        width={size}
        height={size}
        viewBox="0 0 120 120"
        aria-hidden="true"
      >
        <path
          d="M38 34c14-10 34-10 48 0 16 10 18 38 4 54-14 16-46 16-60 0-14-16-12-44 8-54Z"
          fill="rgba(111,163,210,0.22)"
          stroke="currentColor"
          strokeWidth="5"
          strokeLinejoin="round"
        />
        <path
          d="M34 34c-12 2-22 14-18 30 3 12 14 16 24 10M86 34c12 2 22 14 18 30-3 12-14 16-24 10"
          fill="rgba(111,163,210,0.35)"
          stroke="currentColor"
          strokeWidth="4"
          strokeLinejoin="round"
        />
        <circle cx="48" cy="60" r="4.5" fill="currentColor" />
        <circle cx="72" cy="60" r="4.5" fill="currentColor" />
        <ellipse cx="60" cy="72" rx="6" ry="4" fill="currentColor" />
        <path
          d="M48 82c8 8 16 8 24 0"
          fill="none"
          stroke="currentColor"
          strokeWidth="4"
          strokeLinecap="round"
        />
      </svg>
    )
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
        d="M40 34c16-14 36-8 40 10 3 14-2 24 18 28l-10 18c-16-2-28-8-36-20 0 10 2 18 10 24-10 4-22 3-30-2 8-6 10-16 8-26-3-12-4-24 0-32Z"
        fill="rgba(242,230,107,0.35)"
        stroke="currentColor"
        strokeWidth="5"
        strokeLinejoin="round"
      />
      <circle cx="56" cy="38" r="3.5" fill="currentColor" />
      <path
        d="M50 70c10 10 24 12 36 4M38 96c-3 6-8 8-16 8M56 98c4 6 12 7 20 4"
        fill="none"
        stroke="currentColor"
        strokeWidth="4.5"
        strokeLinecap="round"
      />
      <path
        d="M68 42c8 2 12 8 10 14"
        fill="none"
        stroke="currentColor"
        strokeWidth="4"
        strokeLinecap="round"
      />
    </svg>
  )
}
