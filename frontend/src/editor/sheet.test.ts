import { describe, expect, it } from 'vitest'
import {
  buildAsepriteJSON,
  buildTexturePackerJSON,
  composeSheet,
  frameRect,
  sheetDurations,
  sheetInfoFor,
  sheetSize,
  sliceLayout,
  sliceSheet,
} from './sheet'
import { createImage, pixelAt, setPixel } from './pixels'

const RED = { r: 255, g: 0, b: 0, a: 255 }
const BLUE = { r: 0, g: 0, b: 255, a: 255 }
const GREEN = { r: 0, g: 255, b: 0, a: 255 }

describe('sprite sheets', () => {
  it('computes complete cells with margin and spacing', () => {
    // 1 margin + 2 frames of 4 + 1 gap + 1 margin = 12, height 11 for two rows
    const layout = sliceLayout(12, 11, { frameWidth: 4, frameHeight: 4, margin: 1, spacing: 1 })
    expect(layout).toEqual({
      frameWidth: 4,
      frameHeight: 4,
      columns: 2,
      rows: 2,
      count: 4,
      margin: 1,
      spacing: 1,
    })
  })

  it('rejects frames that cannot fit', () => {
    expect(sliceLayout(8, 8, { frameWidth: 0, frameHeight: 8 })).toBeNull()
    expect(sliceLayout(8, 8, { frameWidth: 9, frameHeight: 8 })).toBeNull()
    expect(sliceLayout(8, 8, { frameWidth: 8, frameHeight: 8, margin: 1 })).toBeNull()
  })

  it('slices row-major and copies pixels exactly', () => {
    const sheet = createImage(4, 4)
    setPixel(sheet, 0, 0, RED)
    setPixel(sheet, 2, 0, BLUE)
    setPixel(sheet, 0, 2, GREEN)
    setPixel(sheet, 3, 3, RED)

    const frames = sliceSheet(sheet, { frameWidth: 2, frameHeight: 2 })
    expect(frames).toHaveLength(4)
    expect(pixelAt(frames[0], 0, 0)).toEqual(RED)
    expect(pixelAt(frames[1], 0, 0)).toEqual(BLUE)
    expect(pixelAt(frames[2], 0, 0)).toEqual(GREEN)
    expect(pixelAt(frames[3], 1, 1)).toEqual(RED)
    expect(pixelAt(frames[0], 1, 1).a).toBe(0)
  })

  it('packs frames into a sheet and slices them back unchanged', () => {
    const first = createImage(2, 2)
    setPixel(first, 1, 0, RED)
    const second = createImage(2, 2)
    setPixel(second, 0, 1, BLUE)

    const sheet = composeSheet([first, second], 2, 1, 1)
    expect(sheet).not.toBeNull()
    expect(sheet!.width).toBe(1 + 2 + 1 + 2 + 1)
    expect(sheet!.height).toBe(1 + 2 + 1)

    const frames = sliceSheet(sheet!, { frameWidth: 2, frameHeight: 2, margin: 1, spacing: 1 })
    expect(frames).toHaveLength(2)
    expect(pixelAt(frames[0], 1, 0)).toEqual(RED)
    expect(pixelAt(frames[1], 0, 1)).toEqual(BLUE)
  })

  it('computes packed sheet dimensions', () => {
    expect(sheetSize(32, 32, 6, 3)).toEqual({ width: 96, height: 64, rows: 2 })
    expect(sheetSize(16, 24, 5, 2, 2, 1)).toEqual({ width: 2 * 2 + 2 * 16 + 1, height: 4 + 3 * 24 + 2 * 1, rows: 3 })
  })

  it('describes frame rects and animation metadata', () => {
    const info = sheetInfoFor(8, 6, 5, 3, 10, true, 1, 1)
    expect(info.rows).toBe(2)
    expect(frameRect(info, 0)).toEqual({ x: 1, y: 1, w: 8, h: 6 })
    expect(frameRect(info, 3)).toEqual({ x: 1, y: 1 + 6 + 1, w: 8, h: 6 })
    expect(frameRect(info, 4)).toEqual({ x: 1 + 8 + 1, y: 1 + 6 + 1, w: 8, h: 6 })

    const aseprite = buildAsepriteJSON(info, 1 * 2 + 3 * 8 + 2, 1 * 2 + 2 * 6 + 1)
    expect(aseprite.frames).toHaveProperty('000')
    expect(aseprite.frames).toHaveProperty('004')
    const first = aseprite.frames['000'] as { duration: number; frame: { w: number } }
    expect(first.duration).toBe(100)
    expect(first.frame.w).toBe(8)
    expect(aseprite.meta.frameTags[0].to).toBe(4)

    const packer = buildTexturePackerJSON(info, 28, 15)
    expect(packer.animations.default).toHaveLength(5)
    expect(packer.animations.default[1]).toBe('001')
    expect((packer.frames['004'] as { frame: { y: number } }).frame.y).toBe(8)
  })

  it('writes per-frame durations and clip tags', () => {
    const info = sheetInfoFor(4, 4, 3, 3, 10, true, 0, 0, [60, 0, 250], [' idle ', 'idle', 'walk', ''])
    expect(sheetDurations(info)).toEqual([60, 100, 250])

    const aseprite = buildAsepriteJSON(info, 12, 4)
    const frames = aseprite.frames as Record<string, { duration: number }>
    expect(frames['000'].duration).toBe(60)
    expect(frames['001'].duration).toBe(100)
    expect(frames['002'].duration).toBe(250)
    expect(aseprite.meta.frameTags.map((tag) => tag.name)).toEqual(['idle', 'walk'])
    expect(aseprite.meta.frameTags[0].to).toBe(2)

    const packer = buildTexturePackerJSON(info, 12, 4)
    expect(Object.keys(packer.animations)).toEqual(['idle', 'walk'])
    expect(packer.animations.walk).toEqual(['000', '001', '002'])
  })

  it('falls back to a default tag and uniform timing when unset', () => {
    const info = sheetInfoFor(4, 4, 2, 2, 8, true)
    const aseprite = buildAsepriteJSON(info, 8, 4)
    expect(aseprite.meta.frameTags).toHaveLength(1)
    expect(aseprite.meta.frameTags[0].name).toBe('default')
    expect((aseprite.frames['001'] as { duration: number }).duration).toBe(125)
    expect(Object.keys(buildTexturePackerJSON(info, 8, 4).animations)).toEqual(['default'])
  })
})