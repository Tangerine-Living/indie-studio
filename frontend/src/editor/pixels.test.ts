import { describe, expect, it } from 'vitest'
import {
  cloneImage,
  createImage,
  drawEllipse,
  drawLine,
  drawRect,
  floodFill,
  hexToRGBA,
  linePoints,
  pixelAt,
  rgbaToHex,
  stamp,
  upscaleNearest,
} from './pixels'

const RED = { r: 255, g: 0, b: 0, a: 255 }
const BLUE = { r: 0, g: 0, b: 255, a: 255 }
const CLEAR = { r: 0, g: 0, b: 0, a: 0 }

function painted(image: ReturnType<typeof createImage>): number {
  let count = 0
  for (let i = 3; i < image.data.length; i += 4) if (image.data[i] > 0) count += 1
  return count
}

describe('pixel tools', () => {
  it('creates blank, correctly sized images', () => {
    const image = createImage(4, 3)
    expect(image.width).toBe(4)
    expect(image.height).toBe(3)
    expect(image.data.length).toBe(4 * 3 * 4)
    expect(painted(image)).toBe(0)
  })

  it('clones deeply enough to keep history independent', () => {
    const image = createImage(2, 2)
    stamp(image, 0, 0, 1, RED)
    const copy = cloneImage(image)
    stamp(image, 1, 1, 1, BLUE)
    expect(pixelAt(copy, 1, 1)).toEqual(CLEAR)
  })

  it('round-trips hex colors with and without alpha', () => {
    expect(hexToRGBA('#6ec6e8')).toEqual({ r: 110, g: 198, b: 232, a: 255 })
    expect(hexToRGBA('#abc')).toEqual({ r: 170, g: 187, b: 204, a: 255 })
    expect(hexToRGBA('11223380')).toEqual({ r: 17, g: 34, b: 51, a: 128 })
    expect(hexToRGBA('nope')).toBeNull()
    expect(rgbaToHex({ r: 255, g: 0, b: 16, a: 255 })).toBe('#ff0010')
    expect(rgbaToHex({ r: 255, g: 0, b: 16, a: 128 })).toBe('#ff001080')
  })

  it('stamps square brushes and erases with alpha 0', () => {
    const image = createImage(4, 4)
    stamp(image, 2, 2, 1, RED)
    expect(painted(image)).toBe(1)
    expect(pixelAt(image, 2, 2)).toEqual(RED)

    stamp(image, 1, 1, 2, BLUE)
    expect(pixelAt(image, 1, 1)).toEqual(BLUE)
    expect(pixelAt(image, 2, 2)).toEqual(BLUE)
    expect(pixelAt(image, 0, 0)).toEqual(CLEAR)

    stamp(image, 1, 1, 2, CLEAR)
    expect(pixelAt(image, 1, 1)).toEqual(CLEAR)
    expect(pixelAt(image, 2, 2)).toEqual(CLEAR)
    // The 2×2 eraser also removed the earlier single red pixel at (2, 2).
    expect(painted(image)).toBe(0)
  })

  it('draws lines with a brush and ignores out-of-bounds pixels', () => {
    const image = createImage(5, 5)
    drawLine(image, -2, 2, 6, 2, { size: 1, color: RED })
    expect(painted(image)).toBe(5)
    expect(pixelAt(image, 0, 2)).toEqual(RED)
    expect(pixelAt(image, 4, 2)).toEqual(RED)

    drawLine(image, 0, 0, 4, 4, { size: 1, color: BLUE })
    expect(pixelAt(image, 2, 2)).toEqual(BLUE)
    expect(pixelAt(image, 3, 3)).toEqual(BLUE)
  })

  it('draws rectangle outlines and filled rectangles', () => {
    const outline = createImage(5, 5)
    drawRect(outline, 0, 0, 4, 4, { size: 1, color: RED })
    expect(pixelAt(outline, 0, 0)).toEqual(RED)
    expect(pixelAt(outline, 2, 0)).toEqual(RED)
    expect(pixelAt(outline, 4, 4)).toEqual(RED)
    expect(pixelAt(outline, 2, 2)).toEqual(CLEAR)
    expect(painted(outline)).toBe(16)

    const filled = createImage(5, 5)
    drawRect(filled, 1, 1, 3, 3, { size: 1, color: BLUE }, true)
    expect(painted(filled)).toBe(9)
    expect(pixelAt(filled, 2, 2)).toEqual(BLUE)
  })

  it('draws ellipses inside their bounding box', () => {
    const image = createImage(9, 9)
    drawEllipse(image, 0, 0, 8, 8, { size: 1, color: RED }, false)
    expect(pixelAt(image, 4, 0)).toEqual(RED)
    expect(pixelAt(image, 4, 8)).toEqual(RED)
    expect(pixelAt(image, 0, 4)).toEqual(RED)
    expect(pixelAt(image, 8, 4)).toEqual(RED)
    expect(pixelAt(image, 0, 0)).toEqual(CLEAR)
    expect(pixelAt(image, 4, 4)).toEqual(CLEAR)
  })

  it('flood fills only connected regions with matching color', () => {
    const image = createImage(4, 4)
    drawRect(image, 0, 0, 1, 3, { size: 1, color: RED }, true)
    const filled = floodFill(image, 0, 0, BLUE, 0)
    expect(filled).toBe(8)
    expect(pixelAt(image, 0, 3)).toEqual(BLUE)
    expect(pixelAt(image, 1, 3)).toEqual(BLUE)
    expect(pixelAt(image, 2, 0)).toEqual(CLEAR)
  })

  it('clips stamps and lines to a selection mask', () => {
    const image = createImage(3, 3)
    const mask = new Uint8Array(9)
    mask[0] = 1
    mask[4] = 1
    drawLine(image, 0, 0, 2, 2, { size: 1, color: RED, mask })
    expect(pixelAt(image, 0, 0)).toEqual(RED)
    expect(pixelAt(image, 1, 1)).toEqual(RED)
    expect(pixelAt(image, 2, 2)).toEqual(CLEAR)
    expect(pixelAt(image, 0, 1)).toEqual(CLEAR)
  })

  it('fills disconnected pixels and honors a mask', () => {
    const image = createImage(5, 1)
    for (const x of [0, 2, 4]) stamp(image, x, 0, 1, RED)
    const connected = cloneImage(image)
    expect(floodFill(connected, 0, 0, BLUE, 0)).toBe(1)
    expect(pixelAt(connected, 2, 0)).toEqual(RED)

    const global = cloneImage(image)
    expect(floodFill(global, 0, 0, BLUE, 0, { contiguous: false })).toBe(3)
    expect(pixelAt(global, 4, 0)).toEqual(BLUE)

    const masked = cloneImage(image)
    const mask = new Uint8Array(5)
    mask[0] = 1
    mask[2] = 1
    expect(floodFill(masked, 0, 0, BLUE, 0, { contiguous: false, mask })).toBe(2)
    expect(pixelAt(masked, 2, 0)).toEqual(BLUE)
    expect(pixelAt(masked, 4, 0)).toEqual(RED)
  })

  it('enumerates bresenham pixels for pixel-perfect strokes', () => {
    expect(linePoints(0, 0, 2, 1)).toEqual([
      { x: 0, y: 0 },
      { x: 1, y: 0 },
      { x: 2, y: 1 },
    ])
    expect(linePoints(1, 1, 1, 1)).toEqual([{ x: 1, y: 1 }])
  })

  it('upscales with nearest neighbour', () => {
    const image = createImage(2, 1)
    stamp(image, 0, 0, 1, RED)
    stamp(image, 1, 0, 1, BLUE)
    const big = upscaleNearest(image, 3)
    expect(big.width).toBe(6)
    expect(big.height).toBe(3)
    expect(pixelAt(big, 2, 2)).toEqual(RED)
    expect(pixelAt(big, 3, 0)).toEqual(BLUE)
  })
})