/** AP-120: map capture/vision failures to fail-forward reasons */
export type FailForwardReason = 'miss' | 'weather' | 'no_camera' | 'permission' | 'offline'

export function reasonFromCaptureFailure(input: {
  offline?: boolean
  noCamera?: boolean
  permissionDenied?: boolean
  weatherBlock?: boolean
  lowConfidence?: boolean
}): FailForwardReason {
  if (input.offline) return 'offline'
  if (input.noCamera || input.permissionDenied) return 'permission'
  if (input.weatherBlock) return 'weather'
  return 'miss'
}

export function failForwardAdvancesStory(missCount: number): boolean {
  return missCount >= 1
}
