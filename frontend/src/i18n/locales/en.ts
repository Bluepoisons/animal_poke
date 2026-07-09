/** English translations */
import type { TranslationKey } from './zh'

export const en: Record<TranslationKey, string> = {
  // Common
  'app.name': 'Animal Poke',
  'common.confirm': 'Confirm',
  'common.cancel': 'Cancel',
  'common.retry': 'Retry',
  'common.loading': 'Loading…',
  'common.close': 'Close',
  'common.save': 'Save',
  'common.back': 'Back',

  // Navigation
  'tab.camera': 'Discover',
  'tab.collection': 'Collection',
  'tab.fight': 'Battle',
  'tab.store': 'Store',
  'tab.dispatch': 'Dispatch',
  'tab.achievement': 'Achievements',

  // Stamina
  'stamina.label': 'Stamina',
  'stamina.insufficient': '⚡ Not enough stamina',
  'stamina.full': 'Stamina full',

  // Discover
  'discover.scanning': 'Scanning…',
  'discover.detected': 'Animal found! Start capture',
  'discover.notfound': 'No animal found',
  'discover.error': 'Detection failed, please retry',
  'discover.cameraDenied': 'Camera permission denied',
  'discover.cameraPrompt': 'Please allow camera access to discover animals',

  // Capture
  'capture.throw': 'Throw',
  'capture.success': 'Capture successful!',
  'capture.fail': 'So close! Try again',
  'capture.power': 'Power',

  // Collection
  'collection.title': 'Collection',
  'collection.empty': 'No animals collected yet',
  'collection.unlocked': 'Collected',
  'collection.total': 'Total',
  'collection.filter.all': 'All',
  'collection.filter.today': 'Today',
  'collection.filter.week': 'This Week',
  'collection.filter.nearby': 'Nearby',

  // Battle
  'battle.title': 'Battle',
  'battle.win': 'Victory!',
  'battle.lose': 'Defeat…',
  'battle.attack': 'Attack',
  'battle.selectPet': 'Select Pet',

  // Store
  'store.title': 'Store',
  'store.checkIn': 'Daily Check-in',
  'store.checkInClaimed': 'Already checked in today',
  'store.gold': 'Gold',
  'store.buy': 'Buy',

  // Achievement
  'achievement.title': 'Achievements',
  'achievement.locked': 'Locked',

  // Error
  'error.title': 'Something went wrong',
  'error.crash': 'App crashed',
  'error.reload': 'Reload',
  'error.provider': 'Module temporarily unavailable',

  // Settings
  'settings.language': 'Language',
  'settings.chinese': '中文',
  'settings.english': 'English',
}
