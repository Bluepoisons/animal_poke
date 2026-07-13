/** 可关闭音效/触觉偏好（#213） */
import { chineseRarityName } from './petLocalization'

const KEY = 'animal-poke-feedback-prefs'

export interface FeedbackPrefs {
  soundEnabled: boolean
  hapticsEnabled: boolean
  rareRevealEnabled: boolean
}

const DEFAULTS: FeedbackPrefs = {
  soundEnabled: true,
  hapticsEnabled: true,
  rareRevealEnabled: true,
}

export function loadFeedbackPrefs(): FeedbackPrefs {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return { ...DEFAULTS }
    return { ...DEFAULTS, ...JSON.parse(raw) }
  } catch {
    return { ...DEFAULTS }
  }
}

export function saveFeedbackPrefs(p: FeedbackPrefs): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(p))
  } catch {
    /* ignore */
  }
}

export function hapticTap(enabled = loadFeedbackPrefs().hapticsEnabled): void {
  if (!enabled) return
  try {
    navigator.vibrate?.(12)
  } catch {
    /* ignore */
  }
}

export function announceRareReveal(
  rarity: string,
  prefs = loadFeedbackPrefs(),
): string | null {
  if (!prefs.rareRevealEnabled) return null
  if (rarity === 'epic' || rarity === 'legendary' || rarity === '4' || rarity === '5') {
    hapticTap(prefs.hapticsEnabled)
    return `稀有揭晓：${chineseRarityName(rarity === '4' ? 'epic' : rarity === '5' ? 'legendary' : rarity)}`
  }
  return null
}
