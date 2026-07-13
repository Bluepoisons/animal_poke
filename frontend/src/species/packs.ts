/**
 * 内置物种内容包（与 backend/internal/speciespack/builtin.go 同步的前端镜像）。
 *
 * manifest gate 会静态扫描每个 descriptor 的 contentId/status 字面量；
 * 其余重复的认证、玩法和安全文案由工厂统一补齐。
 */
import type { SpeciesGroup, SpeciesPack } from './types'

type SpeciesSeed = Pick<SpeciesPack, 'id' | 'contentId' | 'status'> & {
  zh: string
  en: string
  scientific?: string
  emoji: string
  group: SpeciesGroup
  aliases?: string[]
  contains?: string[]
  containsExclude?: string[]
  remoteOnly?: boolean
  gameplay?: Partial<NonNullable<SpeciesPack['gameplay']>>
}

const DEFAULT_STATS = { hp: 1, atk: 1, def: 1, spd: 1, crit: 4, eva: 4 }
const DEFAULT_RARITY_WEIGHTS = [
  { tier: 'common', weight: 60 },
  { tier: 'uncommon', weight: 25 },
  { tier: 'rare', weight: 10 },
  { tier: 'epic', weight: 4 },
  { tier: 'legendary', weight: 1 },
]

const GROUP_HABITAT: Record<SpeciesGroup, { zh: string; en: string }> = {
  companion: { zh: '城市、社区与家庭生活环境', en: 'Homes, neighborhoods, and urban habitats' },
  farm: { zh: '农场、牧场与乡村环境', en: 'Farms, pastures, and rural habitats' },
  wildlife: { zh: '森林、草地与自然保护区域', en: 'Forests, grasslands, and protected habitats' },
  bird: { zh: '林地、湿地与城市绿地上空', en: 'Woodlands, wetlands, and urban green spaces' },
  reptile: { zh: '湿地、林地与温暖的自然环境', en: 'Wetlands, woodlands, and warm natural habitats' },
  amphibian: { zh: '溪流、池塘与潮湿林地', en: 'Streams, ponds, and damp woodlands' },
  aquatic: { zh: '河流、湖泊与海洋水域', en: 'Rivers, lakes, and marine habitats' },
  insect: { zh: '花丛、草地与林缘环境', en: 'Flower patches, grasslands, and woodland edges' },
  other: { zh: '多样的自然与人居环境', en: 'Varied natural and human habitats' },
}

const SPECIES_SEEDS: SpeciesSeed[] = [
  {
    id: 'cat', contentId: 'species.cat', status: 'capturable', zh: '猫', en: 'Cat',
    scientific: 'Felis catus', emoji: '🐱', group: 'companion',
    aliases: ['kitten', 'kitty', 'feline', '小猫', '猫咪', '英短猫'],
    contains: ['小猫', '猫咪', '英短猫', '猫', 'cat'],
    containsExclude: ['cattle', 'caterpillar', 'catfish', 'big cat', 'large cat', 'wild cat'],
    gameplay: {
      throwItem: { 'zh-CN': '观察贴纸', en: 'Observe sticker' },
      captureMechanics: { 'zh-CN': '标准抛物线', en: 'Standard arc' },
      chargeRate: 2, optimalRange: [40, 80], chargeSpeed: 1.15, detectThreshold: 0.85,
      statModifiers: { hp: 0.8, atk: 0.9, def: 0.9, spd: 1.3, crit: 10, eva: 5 },
    },
  },
  {
    id: 'dog', contentId: 'species.dog', status: 'capturable', zh: '狗', en: 'Dog',
    scientific: 'Canis familiaris', emoji: '🐶', group: 'companion',
    aliases: ['puppy', 'canine', '小狗', '狗狗', '犬'], contains: ['小狗', '狗狗', '狗', '犬', 'dog'],
    gameplay: {
      throwItem: { 'zh-CN': '镜头信号', en: 'Lens signal' },
      captureMechanics: { 'zh-CN': '下落更快', en: 'Faster fall' },
      chargeRate: 1.5, optimalRange: [45, 85], chargeSpeed: 1, detectThreshold: 0.85,
      statModifiers: { hp: 1.3, atk: 1.2, def: 1, spd: 0.8, crit: 3, eva: 2 },
    },
  },
  {
    id: 'rabbit', contentId: 'species.rabbit', status: 'capturable', zh: '兔', en: 'Rabbit',
    scientific: 'Oryctolagus cuniculus', emoji: '🐰', group: 'companion',
    aliases: ['bunny', 'hare', 'leveret', '野兔'], contains: ['野兔', '兔', 'rabbit', 'bunny', 'hare'],
  },
  {
    id: 'horse', contentId: 'species.horse', status: 'capturable', zh: '马', en: 'Horse',
    emoji: '🐴', group: 'farm', aliases: ['pony', 'equine'], contains: ['马', 'horse', 'pony'],
    containsExclude: ['seahorse'],
  },
  {
    id: 'cow', contentId: 'species.cow', status: 'capturable', zh: '牛', en: 'Cow',
    emoji: '🐮', group: 'farm', aliases: ['cattle', 'calf', 'bovine'], contains: ['牛', 'cow', 'cattle', 'bovine'],
  },
  {
    id: 'sheep', contentId: 'species.sheep', status: 'capturable', zh: '羊', en: 'Sheep',
    emoji: '🐑', group: 'farm', aliases: ['lamb', 'ewe', 'ram'], contains: ['绵羊', 'sheep', 'lamb'],
  },
  {
    id: 'goat', contentId: 'species.goat', status: 'capturable', zh: '山羊', en: 'Goat',
    emoji: '🐐', group: 'farm', aliases: ['kid goat'], contains: ['山羊', 'goat'],
  },
  {
    id: 'pig', contentId: 'species.pig', status: 'capturable', zh: '猪', en: 'Pig',
    emoji: '🐷', group: 'farm', aliases: ['piglet', 'swine', 'hog'], contains: ['猪', 'pig', 'swine'],
  },
  {
    id: 'deer', contentId: 'species.deer', status: 'capturable', zh: '鹿', en: 'Deer',
    emoji: '🦌', group: 'wildlife', aliases: ['doe', 'stag', 'fawn'], contains: ['鹿', 'deer', 'fawn'],
  },
  {
    id: 'squirrel', contentId: 'species.squirrel', status: 'capturable', zh: '松鼠', en: 'Squirrel',
    emoji: '🐿️', group: 'wildlife', aliases: ['chipmunk'], contains: ['松鼠', 'squirrel', 'chipmunk'],
  },
  {
    id: 'monkey', contentId: 'species.monkey', status: 'capturable', zh: '猴', en: 'Monkey',
    emoji: '🐒', group: 'wildlife', aliases: ['ape', 'primate'], contains: ['猴', 'monkey', 'primate'],
  },
  {
    id: 'bear', contentId: 'species.bear', status: 'capturable', zh: '熊', en: 'Bear',
    emoji: '🐻', group: 'wildlife', contains: ['熊', 'bear'], remoteOnly: true,
  },
  {
    id: 'elephant', contentId: 'species.elephant', status: 'capturable', zh: '大象', en: 'Elephant',
    emoji: '🐘', group: 'wildlife', contains: ['大象', '象', 'elephant'], remoteOnly: true,
  },
  {
    id: 'big_cat', contentId: 'species.big_cat', status: 'capturable', zh: '大型猫科动物', en: 'Big cat',
    emoji: '🐅', group: 'wildlife',
    aliases: ['large cat', 'wild cat', 'tiger', 'lion', 'leopard', 'jaguar', 'cheetah', 'cougar', 'panther', 'lynx'],
    contains: ['大型猫科', '老虎', '狮子', '豹子', '猎豹', 'big cat', 'large cat', 'wild cat', 'tiger', 'lion', 'leopard', 'jaguar', 'cheetah', 'panther'],
    remoteOnly: true,
  },
  {
    id: 'bird', contentId: 'species.bird', status: 'capturable', zh: '鸟', en: 'Bird',
    emoji: '🐦', group: 'bird', aliases: ['avian', 'songbird', 'swan', 'sparrow', '鸟类'],
    contains: ['鸟类', '天鹅', '麻雀', '蜂鸟', '鸟', 'bird', 'avian', 'swan', 'sparrow'],
  },
  {
    id: 'goose', contentId: 'species.goose', status: 'capturable', zh: '鹅', en: 'Goose',
    scientific: 'Anser sp.', emoji: '🪿', group: 'bird',
    aliases: ['geese', 'gander', 'gosling', '大鹅'], contains: ['大鹅', 'goose', 'geese', '鹅'], containsExclude: ['mongoose'],
    gameplay: {
      throwItem: { 'zh-CN': '友好光点', en: 'Friendly spark' },
      captureMechanics: { 'zh-CN': '弹跳略强', en: 'Slight bounce' },
      chargeRate: 2.5, optimalRange: [35, 75], chargeSpeed: 0.9, detectThreshold: 0.75,
      statModifiers: { hp: 1, atk: 0.8, def: 1.4, spd: 0.9, crit: 2, eva: 8 },
    },
  },
  {
    id: 'duck', contentId: 'species.duck', status: 'capturable', zh: '鸭', en: 'Duck',
    emoji: '🦆', group: 'bird', aliases: ['duckling', 'mallard', '鸭子'], contains: ['鸭子', '鸭', 'duck', 'mallard'],
  },
  {
    id: 'chicken', contentId: 'species.chicken', status: 'capturable', zh: '鸡', en: 'Chicken',
    emoji: '🐔', group: 'farm', aliases: ['hen', 'rooster', 'chick', 'poultry'], contains: ['鸡', 'chicken', 'rooster', 'hen'],
  },
  {
    id: 'pigeon', contentId: 'species.pigeon', status: 'capturable', zh: '鸽子', en: 'Pigeon',
    emoji: '🕊️', group: 'bird', aliases: ['dove'], contains: ['鸽', 'pigeon', 'dove'],
  },
  {
    id: 'parrot', contentId: 'species.parrot', status: 'capturable', zh: '鹦鹉', en: 'Parrot',
    emoji: '🦜', group: 'bird', aliases: ['macaw', 'cockatoo'], contains: ['鹦鹉', 'parrot', 'macaw', 'cockatoo'],
  },
  {
    id: 'eagle', contentId: 'species.eagle', status: 'capturable', zh: '鹰', en: 'Eagle',
    emoji: '🦅', group: 'bird', aliases: ['hawk', 'falcon', 'raptor'], contains: ['鹰', 'eagle', 'hawk', 'falcon'], remoteOnly: true,
  },
  {
    id: 'turtle', contentId: 'species.turtle', status: 'capturable', zh: '龟', en: 'Turtle',
    emoji: '🐢', group: 'reptile', aliases: ['tortoise', 'terrapin'], contains: ['龟', 'turtle', 'tortoise'], remoteOnly: true,
  },
  {
    id: 'lizard', contentId: 'species.lizard', status: 'capturable', zh: '蜥蜴', en: 'Lizard',
    emoji: '🦎', group: 'reptile', aliases: ['gecko', 'iguana'], contains: ['蜥蜴', '壁虎', 'lizard', 'gecko', 'iguana'],
  },
  {
    id: 'snake', contentId: 'species.snake', status: 'capturable', zh: '蛇', en: 'Snake',
    emoji: '🐍', group: 'reptile', aliases: ['serpent'], contains: ['蛇', 'snake', 'serpent'],
  },
  {
    id: 'crocodile', contentId: 'species.crocodile', status: 'capturable', zh: '鳄鱼', en: 'Crocodile',
    emoji: '🐊', group: 'reptile', aliases: ['alligator', 'caiman'], contains: ['鳄', 'crocodile', 'alligator', 'caiman'], remoteOnly: true,
  },
  {
    id: 'frog', contentId: 'species.frog', status: 'capturable', zh: '青蛙', en: 'Frog',
    emoji: '🐸', group: 'amphibian', aliases: ['toad', 'bullfrog', '牛蛙'],
    contains: ['青蛙', '牛蛙', '蟾蜍', '蛙', 'frog', 'bullfrog', 'toad'],
  },
  {
    id: 'salamander', contentId: 'species.salamander', status: 'capturable', zh: '蝾螈', en: 'Salamander',
    emoji: '🦎', group: 'amphibian', aliases: ['newt', 'axolotl'], contains: ['蝾螈', '娃娃鱼', '六角恐龙', 'salamander', 'newt', 'axolotl'],
  },
  {
    id: 'fish', contentId: 'species.fish', status: 'capturable', zh: '鱼', en: 'Fish',
    emoji: '🐟', group: 'aquatic', aliases: ['bony fish', 'seahorse', 'piranha', '海马', '食人鱼'],
    contains: ['海马', '食人鱼', '鱼', 'fish', 'seahorse', 'piranha'], containsExclude: ['starfish', 'jellyfish'],
  },
  {
    id: 'shark', contentId: 'species.shark', status: 'capturable', zh: '鲨鱼', en: 'Shark',
    emoji: '🦈', group: 'aquatic', contains: ['鲨', 'shark'], remoteOnly: true,
  },
  {
    id: 'dolphin', contentId: 'species.dolphin', status: 'capturable', zh: '海豚', en: 'Dolphin',
    emoji: '🐬', group: 'aquatic', aliases: ['porpoise'], contains: ['海豚', 'dolphin', 'porpoise'], remoteOnly: true,
  },
  {
    id: 'whale', contentId: 'species.whale', status: 'capturable', zh: '鲸', en: 'Whale',
    emoji: '🐋', group: 'aquatic', aliases: ['orca'], contains: ['鲸', 'whale', 'orca'], remoteOnly: true,
  },
  {
    id: 'octopus', contentId: 'species.octopus', status: 'capturable', zh: '章鱼', en: 'Octopus',
    emoji: '🐙', group: 'aquatic', aliases: ['cephalopod'], contains: ['章鱼', 'octopus', 'cephalopod'],
  },
  {
    id: 'crab', contentId: 'species.crab', status: 'capturable', zh: '螃蟹', en: 'Crab',
    emoji: '🦀', group: 'insect', aliases: ['crustacean'], contains: ['蟹', 'crab', 'crustacean'],
  },
  {
    id: 'butterfly', contentId: 'species.butterfly', status: 'capturable', zh: '蝴蝶', en: 'Butterfly',
    emoji: '🦋', group: 'insect', aliases: ['moth'], contains: ['蝴蝶', '蛾', 'butterfly', 'moth'],
  },
  {
    id: 'bee', contentId: 'species.bee', status: 'capturable', zh: '蜜蜂', en: 'Bee',
    emoji: '🐝', group: 'insect', aliases: ['bumblebee', 'honeybee', 'wasp'], contains: ['蜂', '蜜蜂', 'bee', 'bumblebee', 'honeybee', 'wasp'],
    containsExclude: ['蜂鸟'],
  },
  {
    id: 'other_animal', contentId: 'species.other_animal', status: 'capturable', zh: '其他动物', en: 'Other animal',
    emoji: '🐾', group: 'other', contains: ['其他动物', 'other animal'],
  },
]

function unique(values: string[]): string[] {
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))]
}

function createSpeciesPack(seed: SpeciesSeed): SpeciesPack {
  const habitat = GROUP_HABITAT[seed.group]
  const remoteNote = seed.remoteOnly
    ? '仅可在安全距离外观察，请勿靠近、追逐、触摸或投喂。'
    : '保持安全距离，不追逐、不触摸、不投喂。'
  const gameplay = seed.gameplay

  return {
    id: seed.id,
    group: seed.group,
    version: '1.0.0',
    contentId: seed.contentId,
    status: seed.status,
    certification: { goldenSetVersion: '1.0.0', modelTrack: 'detect' },
    names: {
      common: { 'zh-CN': seed.zh, en: seed.en },
      scientific: seed.scientific,
      aliases: unique(seed.aliases ?? []),
      contains: unique([
        seed.id.replace(/_/g, ' '),
        seed.en.toLowerCase(),
        seed.zh,
        ...(seed.contains ?? []),
      ]),
      containsExclude: unique(seed.containsExclude ?? []),
    },
    habitat: { 'zh-CN': habitat.zh, en: habitat.en },
    observationTips: {
      'zh-CN': remoteNote,
      en: seed.remoteOnly
        ? 'Observe only from a safe distance; never approach, chase, touch, or feed.'
        : 'Keep a safe distance; do not chase, touch, or feed.',
    },
    welfare: {
      level: seed.group === 'companion' ? 'companion' : seed.group === 'farm' ? 'domestic' : 'wildlife',
      notes: { 'zh-CN': remoteNote },
    },
    protection: {
      status: seed.remoteOnly ? 'observe_only' : 'none',
      notes: seed.remoteOnly ? { 'zh-CN': '高风险或受保护动物，仅限远距观察。' } : undefined,
    },
    assets: { emoji: seed.emoji, throwItemEmoji: '✨' },
    gameplay: {
      throwItem: gameplay?.throwItem ?? { 'zh-CN': '观察光点', en: 'Observation spark' },
      captureMechanics: gameplay?.captureMechanics ?? { 'zh-CN': '标准观察轨迹', en: 'Standard observation arc' },
      chargeRate: gameplay?.chargeRate ?? 2,
      optimalRange: gameplay?.optimalRange ?? [40, 80],
      chargeSpeed: gameplay?.chargeSpeed ?? 1,
      detectThreshold: gameplay?.detectThreshold ?? 0.75,
      statModifiers: { ...DEFAULT_STATS, ...gameplay?.statModifiers },
      rarityWeights: gameplay?.rarityWeights ?? DEFAULT_RARITY_WEIGHTS.map((entry) => ({ ...entry })),
    },
  }
}

export const SPECIES_PACKS: SpeciesPack[] = SPECIES_SEEDS.map(createSpeciesPack)
