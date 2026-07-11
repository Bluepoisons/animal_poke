/** AP-131: UI labels separating fact / player notes / fiction */
export type NarrativeLayer = 'authored_canon' | 'fictional_vignette' | 'player_note' | 'fact'

export function layerLabel(layer?: string): string {
  switch (layer) {
    case 'authored_canon':
      return '主线正典'
    case 'player_note':
      return '玩家记录'
    case 'fact':
      return '识别事实'
    case 'fictional_vignette':
    default:
      return '虚构花絮'
  }
}

export function ensureFictionMeta(input: {
  narrative?: string
  fiction?: boolean
  disclaimer?: string
  layer?: string
}): { narrative: string; fiction: boolean; disclaimer: string; layer: NarrativeLayer } {
  return {
    narrative: input.narrative || '',
    fiction: input.fiction !== false,
    disclaimer: input.disclaimer || '虚构花絮，非真实动物传记',
    layer: (input.layer as NarrativeLayer) || 'fictional_vignette',
  }
}
