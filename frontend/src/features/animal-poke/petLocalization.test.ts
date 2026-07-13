import { describe, expect, it } from 'vitest'
import { SPECIES_PACKS, localizedOr } from '../../species'
import { chineseDetectedSpeciesName, chineseSpeciesGroupName, chineseSpeciesName } from './petLocalization'

describe('petLocalization species names', () => {
  it('renders every registered species in Chinese without raw IDs', () => {
    for (const pack of SPECIES_PACKS) {
      const label = chineseSpeciesName(pack.id)
      expect(label, pack.id).toBe(localizedOr(pack.names.common, 'zh-CN'))
      expect(label, pack.id).toMatch(/[\u3400-\u9fff]/)
      expect(label, pack.id).not.toMatch(/[A-Za-z_]/)
    }
  })

  it('uses product-approved generic names', () => {
    expect(chineseSpeciesName('bird')).toBe('鸟')
    expect(chineseSpeciesName('frog')).toBe('青蛙')
    expect(chineseSpeciesName('big_cat')).toBe('大型猫科动物')
    expect(chineseSpeciesName('other_animal')).toBe('其他动物')
  })

  it('does not leak an unknown English species label', () => {
    expect(chineseSpeciesName('unknown animal')).toBe('动物伙伴')
  })

  it('shows a safe specific Chinese label for other_animal only', () => {
    expect(chineseDetectedSpeciesName('other_animal', '赤狐')).toBe('赤狐')
    expect(chineseDetectedSpeciesName('other_animal', 'red fox')).toBe('其他动物')
    expect(chineseDetectedSpeciesName('fox', '赤狐')).toBe('动物伙伴')
  })

  it('renders descriptor groups in Chinese', () => {
    expect(chineseSpeciesGroupName('companion')).toBe('伙伴动物')
    expect(chineseSpeciesGroupName('aquatic')).toBe('水生动物')
    expect(chineseSpeciesGroupName('other')).toBe('其他动物')
  })
})
