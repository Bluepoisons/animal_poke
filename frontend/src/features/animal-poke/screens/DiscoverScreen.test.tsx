import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '../../../i18n'
import { createInitialCaptureFlow } from '../captureFlow'
import DiscoverScreen from './DiscoverScreen'

const confirmSpeciesCorrectionMock = vi.hoisted(() => vi.fn())

vi.mock('../../../services/visionDetect', async () => {
  const actual = await vi.importActual<typeof import('../../../services/visionDetect')>('../../../services/visionDetect')
  return { ...actual, confirmSpeciesCorrection: confirmSpeciesCorrectionMock }
})

vi.mock('../../../camera/useCamera', () => ({
  useCamera: () => ({
    status: 'ready',
    error: null,
    facing: 'environment',
    videoRef: { current: null },
    start: vi.fn(),
    stop: vi.fn(),
    retry: vi.fn(),
    switchFacing: vi.fn(),
    captureFrame: vi.fn(),
  }),
}))

vi.mock('../../../performance', () => ({
  usePerfMode: () => ({ decision: {}, shouldPauseCamera: false }),
  compressImageForUpload: vi.fn(),
}))

vi.mock('../../../settings', () => ({
  useSettings: () => ({ settings: { homeMode: false } }),
}))

describe('DiscoverScreen species correction', () => {
  beforeEach(() => confirmSpeciesCorrectionMock.mockReset())

  it('uses one searchable registry picker and confirms a concrete other animal on the server', async () => {
    confirmSpeciesCorrectionMock.mockResolvedValue({
      inference_id: 'inf-corrected-fox',
      parent_inference_id: 'inf-cat',
      target_id: '0',
      original_species: 'cat',
      species: 'other_animal',
      label: '赤狐',
      confidence: 0.92,
      source: 'user_confirmation',
    })
    const dispatch = vi.fn()
    const selected = { id: 'det-cat', targetId: '0', species: 'cat', label: '猫', confidence: 0.92 }
    const flow = {
      ...createInitialCaptureFlow(),
      phase: 'target_confirmed' as const,
      detectInferenceId: 'inf-cat',
      detections: [selected],
      selectedBox: selected,
    }

    const { container } = render(
      <I18nProvider>
        <DiscoverScreen
          energy={100}
          coins={0}
          flow={flow}
          dispatch={dispatch}
          onNavigate={vi.fn()}
          onEnterCapture={vi.fn()}
        />
      </I18nProvider>,
    )

    const input = screen.getByRole('combobox')
    const options = [...container.querySelectorAll('datalist option')].map((option) => option.getAttribute('value'))
    expect(options).toHaveLength(34)
    expect(options).not.toContain('其他动物')
    expect(options).toContain('青蛙')
    expect(options).toContain('鸟')

    fireEvent.change(input, { target: { value: '赤狐' } })
    fireEvent.click(screen.getByRole('button', { name: /确认纠正|Confirm correction/ }))
    await waitFor(() => expect(dispatch).toHaveBeenCalledWith(expect.objectContaining({
      type: 'CORRECTION_SUCCESS',
      detectInferenceId: 'inf-corrected-fox',
      animal: expect.objectContaining({ species: 'other_animal', label: '赤狐', targetId: '0' }),
    })))
    expect(confirmSpeciesCorrectionMock).toHaveBeenCalledWith({
      detectInferenceId: 'inf-cat',
      targetId: '0',
      species: 'other_animal',
      speciesLabelZh: '赤狐',
    })
  })

  it('keeps the original detection when server confirmation rejects the label', async () => {
    confirmSpeciesCorrectionMock.mockResolvedValue({ error: 'species_label_invalid' })
    const dispatch = vi.fn()
    const selected = { id: 'det-cat', targetId: '0', species: 'cat', label: '猫', confidence: 0.92 }
    const flow = {
      ...createInitialCaptureFlow(),
      phase: 'target_confirmed' as const,
      detectInferenceId: 'inf-cat',
      detections: [selected],
      selectedBox: selected,
    }
    render(
      <I18nProvider>
        <DiscoverScreen energy={100} coins={0} flow={flow} dispatch={dispatch} onNavigate={vi.fn()} onEnterCapture={vi.fn()} />
      </I18nProvider>,
    )
    fireEvent.change(screen.getByRole('combobox'), { target: { value: '桌子' } })
    fireEvent.click(screen.getByRole('button', { name: /确认纠正|Confirm correction/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent('纠错未通过')
    expect(dispatch).not.toHaveBeenCalledWith(expect.objectContaining({ type: 'CORRECTION_SUCCESS' }))
  })
})
