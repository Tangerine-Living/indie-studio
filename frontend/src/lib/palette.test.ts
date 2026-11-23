import { describe, expect, it } from 'vitest'
import {
  dedupeColors,
  hexToRgb,
  normalizeColor,
  parsePaletteText,
  paletteFromText,
  rgbToHex,
  serializePalette,
  trimPalette,
} from './palette'

describe('palette interchange', () => {
  it('normalizes short hex and rejects non-colors', () => {
    expect(normalizeColor('#ABC')).toBe('#aabbcc')
    expect(normalizeColor('Ff0000')).toBe('#ff0000')
    expect(normalizeColor('not-a-color')).toBeNull()
    expect(rgbToHex(255, 128, 0)).toBe('#ff8000')
    expect(hexToRgb('#ff8000')).toEqual({ r: 255, g: 128, b: 0 })
    expect(dedupeColors(['#ff0000', '#FF0000', 'nope', '#00f'])).toEqual(['#ff0000', '#0000ff'])
  })

  it('parses a GIMP palette with a name', () => {
    const text = [
      'GIMP Palette',
      'Name: 小镇暖色',
      'Columns: 4',
      '# a comment',
      ' 17  24  33  ink',
      '232 192 122  sand',
    ].join('\n')
    expect(parsePaletteText(text)).toEqual({
      name: '小镇暖色',
      colors: ['#111821', '#e8c07a'],
    })
  })

  it('parses a JASC palette and hex lists', () => {
    const pal = ['JASC-PAL', '0100', '2', '255 0 0', '0 128 255'].join('\r\n')
    expect(parsePaletteText(pal, '海')?.colors).toEqual(['#ff0000', '#0080ff'])
    expect(parsePaletteText('#123456\nabc\n#ABCDEF\n', '名单')).toEqual({
      name: '名单',
      colors: ['#123456', '#aabbcc', '#abcdef'],
    })
    expect(parsePaletteText('name, no colors')).toBeNull()
  })

  it('round-trips through every format', () => {
    const palette = paletteFromText('p1', '测试', ['#111821', '#e8c07a', '#4f8f4a'])
    for (const format of ['gpl', 'pal', 'hex'] as const) {
      const parsed = parsePaletteText(serializePalette(palette, format), 'fallback')
      expect(parsed?.colors, format).toEqual(palette.colors)
    }
    expect(serializePalette(palette, 'pal').startsWith('JASC-PAL\r\n0100\r\n3')).toBe(true)
    expect(serializePalette(palette, 'gpl')).toContain('Name: 测试')
  })

  it('trims to a color budget without duplicates', () => {
    expect(trimPalette(['#fff', '#FFF', '#000', '#f00'], 2)).toEqual(['#ffffff', '#000000'])
    expect(trimPalette(['#fff'], 0)).toEqual(['#ffffff'])
  })
})