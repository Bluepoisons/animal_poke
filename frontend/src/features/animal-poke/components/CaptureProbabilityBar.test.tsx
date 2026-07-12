import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { I18nProvider } from '../../../i18n'
import CaptureProbabilityBar from './CaptureProbabilityBar'

describe('CaptureProbabilityBar', () => {
  it('renders the supplied optimal range instead of a fixed global band', () => {
    const { container } = render(
      <I18nProvider>
        <CaptureProbabilityBar title="Capture" successRate={0.78} bestMin={45} bestMax={85} />
      </I18nProvider>,
    )

    const segments = Array.from(container.querySelectorAll('.ap-probability > span')) as HTMLSpanElement[]
    expect(segments).toHaveLength(3)
    expect(segments.map((segment) => segment.style.width)).toEqual(['45%', '40%', '15%'])
  })
})
