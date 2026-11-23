import { describe, expect, it } from 'vitest'
import {
  applyBriefTemplate,
  briefNeedsTemplate,
  briefTemplateFor,
  templateAcceptanceFor,
} from './briefTemplates'
import type { AssetType, QuestBrief } from '../domain/types'

function brief(overrides: Partial<QuestBrief> = {}): QuestBrief {
  return {
    assetType: 'sprite',
    tileSize: 32,
    paletteId: 'default',
    style: '',
    references: [],
    prompt: '',
    ...overrides,
  }
}

describe('brief templates', () => {
  it('has a non-empty template for every asset type', () => {
    const types: AssetType[] = ['sprite', 'building', 'tile', 'prop', 'ui', 'fx', 'other']
    for (const type of types) {
      const template = briefTemplateFor(type)
      expect(template, type).not.toBeNull()
      expect(template!.style.length, type).toBeGreaterThan(0)
      expect(template!.prompt.length, type).toBeGreaterThan(0)
      expect(template!.acceptance.length, type).toBeGreaterThan(0)
    }
    expect(briefTemplateFor('')).toBeNull()
  })

  it('fills blank fields and expands title and tile placeholders', () => {
    const result = applyBriefTemplate({
      brief: brief({ assetType: 'building', tileSize: 48 }),
      acceptance: ['已有验收项'],
      title: '  杂货铺  ',
    })
    expect(result.brief.style).toContain('3/4 俯视')
    expect(result.brief.prompt).toContain('「杂货铺」')
    expect(result.brief.prompt).toContain('48px')
    expect(result.brief.tileSize).toBe(48)
    expect(result.acceptance[0]).toBe('已有验收项')
    expect(result.acceptance).toContain('锚点在底部中心')
  })

  it('keeps user-written copy unless overwrite is requested', () => {
    const original = brief({ style: '我自己的风格', prompt: '我自己的提示词' })
    const kept = applyBriefTemplate({
      brief: original,
      acceptance: [],
      title: '旅行者',
    })
    expect(kept.brief.style).toBe('我自己的风格')
    expect(kept.brief.prompt).toBe('我自己的提示词')

    const replaced = applyBriefTemplate({
      brief: original,
      acceptance: [],
      title: '旅行者',
      overwrite: true,
    })
    expect(replaced.brief.style).not.toBe('我自己的风格')
    expect(replaced.brief.prompt).toContain('「旅行者」')
  })

  it('falls back to a neutral title and dedupes acceptance items', () => {
    const result = applyBriefTemplate({
      brief: brief({ assetType: 'tile', tileSize: 0 }),
      acceptance: [' 四边无缝 ', '四边无缝'],
      title: '   ',
    })
    expect(result.brief.prompt).toContain('「新素材」')
    expect(result.brief.prompt).toContain('32×32')
    expect(result.acceptance.filter((item) => item === '四边无缝')).toHaveLength(1)
  })

  it('detects briefs that still need a template', () => {
    expect(briefNeedsTemplate(brief())).toBe(true)
    expect(briefNeedsTemplate(brief({ prompt: '已填写' }))).toBe(false)
    expect(briefNeedsTemplate(brief({ assetType: '' }))).toBe(false)
  })

  it('lists expanded acceptance items for dropping stale template copy', () => {
    expect(templateAcceptanceFor('tile', '草地', 32)).toEqual([
      '四边无缝',
      '与项目瓦片尺寸一致',
      '与默认调色板一致',
    ])
    expect(templateAcceptanceFor('', '草地', 32)).toEqual([])
  })
})