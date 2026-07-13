import type { AnimalRecord } from '../../db/types'
import { findSpeciesIdByLabel, getSpeciesPack, localizedOr, type SpeciesGroup } from '../../species'

const defaultEnglishNames = ['Milo', 'Luna', 'Coco', 'Nori', 'Maple', 'Pip', 'Momo', 'Sunny']

const classNames: Record<string, string> = {
  Warrior: '战士',
  Mage: '法师',
  Ranger: '游侠',
  Tank: '守护者',
  Support: '辅助者',
  Assassin: '影袭者',
}

const elementNames: Record<string, string> = {
  Fire: '火',
  Water: '水',
  Grass: '草木',
  Electric: '雷电',
  Ice: '冰霜',
  Dark: '暗影',
  Light: '光明',
  Earth: '大地',
  Wind: '风',
}

const breedNames: Record<string, string> = {
  'British Shorthair': '英国短毛猫',
  'Golden Retriever': '金毛寻回犬',
  Tabby: '虎斑猫',
  mix: '混种',
  mixed: '混种',
}

const rarityNames: Record<string, string> = {
  common: '普通',
  uncommon: '少见',
  rare: '稀有',
  epic: '史诗',
  legendary: '传说',
}

const speciesGroupNames: Record<SpeciesGroup, string> = {
  companion: '伙伴动物',
  farm: '农场动物',
  wildlife: '野生动物',
  bird: '鸟类',
  reptile: '爬行动物',
  amphibian: '两栖动物',
  aquatic: '水生动物',
  insect: '昆虫与节肢动物',
  other: '其他动物',
}

function hasChinese(value: string): boolean {
  return /[\u3400-\u9fff]/.test(value)
}

function isChineseDisplayText(value: string, allowedEnglish = ''): boolean {
  const checked = allowedEnglish ? value.split(allowedEnglish).join('') : value
  return hasChinese(value) && !/[A-Za-z]/.test(checked)
}

export function stablePetIndex(value: string): number {
  let hash = 0
  for (const char of value) {
    hash = (Math.imul(hash, 31) + (char.codePointAt(0) ?? 0)) >>> 0
  }
  return hash
}

export function displayPetName(record: Pick<AnimalRecord, 'id' | 'nickname'>): string {
  const nickname = record.nickname?.trim()
  if (nickname) return nickname
  return defaultEnglishNames[stablePetIndex(record.id) % defaultEnglishNames.length]
}

export function chineseSpeciesName(species?: string): string {
  const value = species?.trim() || ''
  const speciesId = findSpeciesIdByLabel(value) ?? value
  const registeredName = localizedOr(getSpeciesPack(speciesId)?.names.common, 'zh-CN')
  return registeredName || (isChineseDisplayText(value) ? value : '动物伙伴')
}

/** `other_animal` 可展示后端已经确认并净化过的具体中文标签。 */
export function chineseDetectedSpeciesName(species?: string, label?: string): string {
  const specificLabel = label?.trim() || ''
  if (species === 'other_animal' && isChineseDisplayText(specificLabel)) return specificLabel
  return chineseSpeciesName(species)
}

export function chineseSpeciesGroupName(group: SpeciesGroup): string {
  return speciesGroupNames[group]
}

export function chineseRarityName(rarity?: string): string {
  const value = rarity?.trim() || ''
  return rarityNames[value] || (isChineseDisplayText(value) ? value : '普通')
}

export function chineseBreedName(breed?: string, species?: string, speciesLabelZh?: string): string {
  const value = breed?.trim() || ''
  if (breedNames[value]) return breedNames[value]
  if (isChineseDisplayText(value)) return value
  return value ? '品种待确认' : chineseDetectedSpeciesName(species, speciesLabelZh)
}

export function chineseClassName(className?: string): string {
  const value = className?.trim() || ''
  return classNames[value] || (isChineseDisplayText(value) ? value : '旅者')
}

export function chineseElementName(element?: string): string {
  const value = element?.trim() || ''
  return elementNames[value] || (isChineseDisplayText(value) ? value : '星光')
}

export function chinesePetSubtitle(record: Pick<AnimalRecord, 'breed' | 'species' | 'speciesLabelZh' | 'className' | 'element'>): string {
  return `${chineseBreedName(record.breed, record.species, record.speciesLabelZh)} · ${chineseClassName(record.className)} / ${chineseElementName(record.element)}属性`
}

export function chinesePetDescription(record: AnimalRecord, name = displayPetName(record)): string {
  const narrative = record.narrative?.trim() || ''
  if (isChineseDisplayText(narrative, name)) return narrative
  const speciesName = chineseDetectedSpeciesName(record.species, record.speciesLabelZh)
  return `${name} 是一位${chineseElementName(record.element)}属性的${chineseClassName(record.className)}${speciesName}伙伴。关于它的故事会在一次次幻想探险中，由你们共同写下。`
}
