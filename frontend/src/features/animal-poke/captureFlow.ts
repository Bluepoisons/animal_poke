/**
 * 生产版发现 → 识别 → 捕获 流程状态机（#162 / AP-001）
 * 唯一真相源：photo、detection、selected target、attempt id
 */
import type { SpeciesType } from '../../types'
import type { DetectionResult } from '../../services/visionDetect'
import { getSpeciesThreshold } from '../../services/visionDetect'
import { capturableSpeciesIds, isCapturableSpecies } from '../../species'

export type CaptureFlowPhase =
  | 'idle'
  | 'camera_ready'
  | 'detecting'
  | 'target_confirmed'
  | 'capturing'
  | 'generating'
  | 'saving'
  | 'syncing'
  | 'completed'
  | 'failed'

export type DetectedAnimal = DetectionResult & {
  id: string
  label?: string
}

export type CaptureFlowState = {
  phase: CaptureFlowPhase
  photoBlob: Blob | null
  detectInferenceId: string | null
  detections: DetectedAnimal[]
  selectedBox: DetectedAnimal | null
  targetId: string | null
  captureAttemptId: string | null
  errorCode: string | null
  errorMessage: string | null
  updatedAt: number
}

export type CaptureFlowEvent =
  | { type: 'CAMERA_READY' }
  | { type: 'CAMERA_ERROR'; code: string; message?: string }
  | { type: 'START_DETECT'; photoBlob: Blob }
  | {
      type: 'DETECT_SUCCESS'
      detections: DetectedAnimal[]
      detectInferenceId: string
    }
  | { type: 'DETECT_FAIL'; code: string; message?: string }
  | { type: 'SELECT_TARGET'; animalId: string }
  | { type: 'CONFIRM_TARGET' }
  | { type: 'ENTER_CAPTURE'; attemptId: string }
  | { type: 'GENERATING' }
  | { type: 'SAVING' }
  | { type: 'SYNCING' }
  | { type: 'COMPLETE' }
  | { type: 'FAIL'; code: string; message?: string }
  | { type: 'RESET' }
  /** Replace full flow snapshot (enter-capture race / recovery). */
  | { type: 'HYDRATE'; state: CaptureFlowState }

export const SUPPORTED_SPECIES: readonly SpeciesType[] = capturableSpeciesIds()

export function createInitialCaptureFlow(): CaptureFlowState {
  return {
    phase: 'idle',
    photoBlob: null,
    detectInferenceId: null,
    detections: [],
    selectedBox: null,
    targetId: null,
    captureAttemptId: null,
    errorCode: null,
    errorMessage: null,
    updatedAt: Date.now(),
  }
}

export function isSupportedSpecies(species: string): species is SpeciesType {
  return isCapturableSpecies(species)
}

/** 过滤：受支持物种 + 达到阈值 */
export function filterEligibleDetections(detections: DetectedAnimal[]): DetectedAnimal[] {
  return detections.filter((d) => {
    if (!isSupportedSpecies(d.species)) return false
    return d.confidence >= getSpeciesThreshold(d.species)
  })
}

/** 是否允许进入捕获页（已确认目标，或已选中目标待确认） */
export function canEnterCapture(state: CaptureFlowState): boolean {
  if (!state.photoBlob || !state.selectedBox || !state.detectInferenceId) return false
  if (!isSupportedSpecies(state.selectedBox.species)) return false
  return (
    state.phase === 'target_confirmed' ||
    state.phase === 'capturing' ||
    state.phase === 'completed' ||
    (state.phase === 'detecting' && !!state.selectedBox)
  )
}

export function reduceCaptureFlow(
  state: CaptureFlowState,
  event: CaptureFlowEvent,
): CaptureFlowState {
  const touch = (partial: Partial<CaptureFlowState>): CaptureFlowState => ({
    ...state,
    ...partial,
    updatedAt: Date.now(),
  })

  switch (event.type) {
    case 'RESET':
      return createInitialCaptureFlow()

    case 'HYDRATE':
      return { ...event.state, updatedAt: Date.now() }

    case 'CAMERA_READY':
      // Never clobber an in-progress recognition / capture pipeline.
      // Re-firing CAMERA_READY (effect re-run, remount, facing switch) used to
      // reset target_confirmed → camera_ready and break Enter Capture (AP-065 E2E).
      if (
        state.phase === 'detecting' ||
        state.phase === 'capturing' ||
        state.phase === 'target_confirmed' ||
        state.phase === 'completed' ||
        state.phase === 'generating' ||
        state.phase === 'saving' ||
        state.phase === 'syncing'
      ) {
        return state
      }
      return touch({ phase: 'camera_ready', errorCode: null, errorMessage: null })

    case 'CAMERA_ERROR':
      // Keep detection payload if we already confirmed a target; only surface
      // camera faults when idle / preparing / already failed.
      if (
        state.phase === 'target_confirmed' ||
        state.phase === 'capturing' ||
        state.phase === 'detecting' ||
        state.phase === 'completed' ||
        state.phase === 'generating' ||
        state.phase === 'saving' ||
        state.phase === 'syncing'
      ) {
        return state
      }
      return touch({
        phase: 'failed',
        errorCode: event.code,
        errorMessage: event.message || event.code,
      })

    case 'START_DETECT':
      return touch({
        phase: 'detecting',
        photoBlob: event.photoBlob,
        detections: [],
        selectedBox: null,
        targetId: null,
        detectInferenceId: null,
        captureAttemptId: null,
        errorCode: null,
        errorMessage: null,
      })

    case 'DETECT_SUCCESS': {
      const eligible = filterEligibleDetections(event.detections)
      if (eligible.length === 0) {
        return touch({
          phase: 'failed',
          detections: event.detections,
          detectInferenceId: event.detectInferenceId,
          selectedBox: null,
          errorCode: 'no_eligible_animal',
          errorMessage: '未发现可捕获的动物（物种或置信度不足）',
        })
      }
      // 单目标自动选中；多目标等待 SELECT_TARGET
      if (eligible.length === 1) {
        return touch({
          phase: 'target_confirmed',
          detections: eligible,
          detectInferenceId: event.detectInferenceId,
          selectedBox: eligible[0],
          targetId: eligible[0].id,
          errorCode: null,
          errorMessage: null,
        })
      }
      return touch({
        phase: 'detecting', // 等待选择，UI 用 detections.length>1 展示选择层
        detections: eligible,
        detectInferenceId: event.detectInferenceId,
        selectedBox: null,
        targetId: null,
        errorCode: 'need_select_target',
        errorMessage: '检测到多个动物，请选择目标',
      })
    }

    case 'DETECT_FAIL':
      return touch({
        phase: 'failed',
        errorCode: event.code,
        errorMessage: event.message || event.code,
        selectedBox: null,
      })

    case 'SELECT_TARGET': {
      const animal = state.detections.find((d) => d.id === event.animalId)
      if (!animal) return state
      return touch({
        selectedBox: animal,
        targetId: animal.id,
        errorCode: null,
        errorMessage: null,
      })
    }

    case 'CONFIRM_TARGET': {
      if (!state.selectedBox || !state.photoBlob || !state.detectInferenceId) {
        return touch({
          phase: 'failed',
          errorCode: 'target_incomplete',
          errorMessage: '目标信息不完整',
        })
      }
      return touch({
        phase: 'target_confirmed',
        errorCode: null,
        errorMessage: null,
      })
    }

    case 'ENTER_CAPTURE':
      if (!state.selectedBox || !state.photoBlob || !state.detectInferenceId) {
        return touch({
          phase: 'failed',
          errorCode: 'capture_guard',
          errorMessage: '未完成识别，不能进入捕获',
        })
      }
      if (!isSupportedSpecies(state.selectedBox.species)) {
        return touch({
          phase: 'failed',
          errorCode: 'unsupported_species',
          errorMessage: '不支持的物种',
        })
      }
      return touch({
        phase: 'capturing',
        captureAttemptId: event.attemptId,
        targetId: state.targetId || state.selectedBox.id,
        errorCode: null,
        errorMessage: null,
      })

    case 'GENERATING':
      return touch({ phase: 'generating' })
    case 'SAVING':
      return touch({ phase: 'saving' })
    case 'SYNCING':
      return touch({ phase: 'syncing' })
    case 'COMPLETE':
      return touch({ phase: 'completed' })
    case 'FAIL':
      return touch({
        phase: 'failed',
        errorCode: event.code,
        errorMessage: event.message || event.code,
      })
    default:
      return state
  }
}
