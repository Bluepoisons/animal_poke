/**
 * 内置物种内容包（与 backend/content/species 同步的前端镜像）
 * 新增物种：追加一个 pack，无需改业务 switch。
 */
import type { SpeciesPack } from './types'

export const SPECIES_PACKS: SpeciesPack[] = [
  {
    id: 'cat',
    version: '1.0.0',
    contentId: 'species.cat',
    status: 'capturable',
    certification: { goldenSetVersion: '1.0.0', modelTrack: 'detect' },
    names: {
      common: { 'zh-CN': '猫', en: 'Cat' },
      scientific: 'Felis catus',
      aliases: ['kitten', 'feline'],
      contains: ['猫', 'cat'],
      containsExclude: ['cattle', 'caterpillar'],
    },
    habitat: { 'zh-CN': '城市与乡村人居环境', en: 'Urban and rural human habitats' },
    observationTips: {
      'zh-CN': '保持距离，避免强闪光与突然靠近',
      en: 'Keep distance; avoid strong flash and sudden approaches',
    },
    welfare: { level: 'companion' },
    protection: { status: 'none' },
    assets: { emoji: '🐱', icon: 'species/cat.png', throwItemEmoji: '🥫' },
    gameplay: {
      throwItem: { 'zh-CN': '观察贴纸', en: 'Observe sticker' },
      captureMechanics: { 'zh-CN': '标准抛物线', en: 'Standard arc' },
      chargeRate: 2,
      optimalRange: [40, 80],
      chargeSpeed: 1.15,
      detectThreshold: 0.85,
      statModifiers: { hp: 0.8, atk: 0.9, def: 0.9, spd: 1.3, crit: 10, eva: 5 },
      rarityWeights: [
        { tier: 'common', weight: 70 },
        { tier: 'uncommon', weight: 25 },
        { tier: 'rare', weight: 12 },
        { tier: 'epic', weight: 3 },
        { tier: 'legendary', weight: 1 },
      ],
    },
  },
  {
    id: 'dog',
    version: '1.0.0',
    contentId: 'species.dog',
    status: 'capturable',
    certification: { goldenSetVersion: '1.0.0', modelTrack: 'detect' },
    names: {
      common: { 'zh-CN': '狗', en: 'Dog' },
      scientific: 'Canis familiaris',
      aliases: ['puppy', 'canine'],
      contains: ['狗', '犬', 'dog'],
    },
    habitat: { 'zh-CN': '社区、公园与人行道', en: 'Neighborhoods, parks, and sidewalks' },
    observationTips: {
      'zh-CN': '征得主人同意后再观察与记录',
      en: 'Ask the owner before observing and recording',
    },
    welfare: { level: 'companion' },
    protection: { status: 'none' },
    assets: { emoji: '🐶', icon: 'species/dog.png', throwItemEmoji: '🦴' },
    gameplay: {
      throwItem: { 'zh-CN': '镜头信号', en: 'Lens signal' },
      captureMechanics: { 'zh-CN': '下落更快', en: 'Faster fall' },
      chargeRate: 1.5,
      optimalRange: [45, 85],
      chargeSpeed: 1.0,
      detectThreshold: 0.85,
      statModifiers: { hp: 1.3, atk: 1.2, def: 1.0, spd: 0.8, crit: 3, eva: 2 },
      rarityWeights: [
        { tier: 'common', weight: 55 },
        { tier: 'uncommon', weight: 30 },
        { tier: 'rare', weight: 15 },
        { tier: 'epic', weight: 5 },
        { tier: 'legendary', weight: 2 },
      ],
    },
  },
  {
    id: 'goose',
    version: '1.0.0',
    contentId: 'species.goose',
    status: 'capturable',
    certification: { goldenSetVersion: '1.0.0', modelTrack: 'detect' },
    names: {
      common: { 'zh-CN': '鹅', en: 'Goose' },
      scientific: 'Anser sp.',
      aliases: ['geese', 'gander', 'gosling'],
      contains: ['goose', 'geese', '鹅'],
      containsExclude: ['mongoose'],
    },
    habitat: {
      'zh-CN': '湖泊、公园水域与湿地边缘',
      en: 'Lakes, park waterways, and wetland edges',
    },
    observationTips: {
      'zh-CN': '勿投喂面包，保持安全距离，警惕护巢行为',
      en: 'Do not feed bread; keep a safe distance; beware nesting defense',
    },
    welfare: { level: 'wildlife' },
    protection: { status: 'none' },
    assets: { emoji: '🪿', icon: 'species/goose.png', throwItemEmoji: '🍞' },
    gameplay: {
      throwItem: { 'zh-CN': '友好光点', en: 'Friendly spark' },
      captureMechanics: { 'zh-CN': '弹跳略强', en: 'Slight bounce' },
      chargeRate: 2.5,
      optimalRange: [35, 75],
      chargeSpeed: 0.9,
      detectThreshold: 0.75,
      statModifiers: { hp: 1.0, atk: 0.8, def: 1.4, spd: 0.9, crit: 2, eva: 8 },
      rarityWeights: [
        { tier: 'common', weight: 45 },
        { tier: 'uncommon', weight: 35 },
        { tier: 'rare', weight: 20 },
        { tier: 'epic', weight: 7 },
        { tier: 'legendary', weight: 3 },
      ],
    },
  },
  // 第四物种试点：未黄金集认证 → 仅百科
  {
    id: 'rabbit',
    version: '1.0.0',
    contentId: 'species.rabbit',
    status: 'catalog_only',
    names: {
      common: { 'zh-CN': '兔', en: 'Rabbit' },
      scientific: 'Oryctolagus cuniculus',
      aliases: ['bunny', 'hare', 'leveret'],
      contains: ['兔', 'rabbit', 'bunny'],
    },
    habitat: { 'zh-CN': '草地、灌丛与公园边缘', en: 'Grassland, scrub, and park edges' },
    observationTips: {
      'zh-CN': '黎明黄昏更易遇见；保持安静，勿追逐',
      en: 'More active at dawn and dusk; stay quiet and do not chase',
    },
    welfare: {
      level: 'wildlife',
      notes: {
        'zh-CN': '试点百科物种：未通过识别黄金集认证前不可捕获或发奖',
        en: 'Pilot encyclopedia species: not capturable until recognition-certified',
      },
    },
    protection: { status: 'none' },
    assets: { emoji: '🐰', icon: 'species/rabbit.png', placeholderTone: 'muted' },
    gameplay: { detectThreshold: 0.85 },
    i18n: {
      'zh-CN': {
        blurb: '第四物种试点：仅百科展示',
        certification_note: '待黄金集认证后方可开放识别捕获',
      },
      en: {
        blurb: 'Fourth-species pilot: encyclopedia only',
        certification_note: 'Capture unlocks only after golden-set certification',
      },
    },
  },
]
