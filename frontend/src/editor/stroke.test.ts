import { describe, expect, it } from 'vitest'
import { createImage, pixelAt, setPixel } from './pixels'
import { PixelPerfectStroke, pixelPerfectPath } from './stroke'

const RED = { r: 255, g: 0, b: 0, a: 255 }
const BLUE = { r: 0, g: 0, b: 255, a: 255 }
const CLEAR = { r: 0, g: 0, b: 0, a: 0 }

describe('pixel-perfect strokes', () => {
  it('removes the elbow pixel of a two-step corner', () => {
    const image = createImage(3, 3)
    const stroke = new PixelPerfectStroke(image, BLUE)
    stroke.start(0, 0)
    stroke.moveTo(1, 0)
    stroke.moveTo(1, 1)
    expect(pixelAt(image, 0, 0)).toEqual(BLUE)
    expect(pixelAt(image, 1, 0)).toEqual(CLEAR)
    expect(pixelAt(image, 1, 1)).toEqual(BLUE)
  })

  it('restores the pixel underneath a removed corner', () => {
    const image = createImage(3, 3)
    setPixel(image, 1, 0, RED)
    const stroke = new PixelPerfectStroke(image, BLUE)
    stroke.start(0, 0)
    stroke.moveTo(1, 0)
    stroke.moveTo(1, 1)
    expect(pixelAt(image, 1, 0)).toEqual(RED)
  })

  it('cleans the elbow at the end of a longer shallow run', () => {
    const image = createImage(4, 3)
    const stroke = new PixelPerfectStroke(image, BLUE)
    stroke.start(0, 0)
    stroke.moveTo(2, 0)
    stroke.moveTo(2, 1)
    expect(pixelAt(image, 0, 0)).toEqual(BLUE)
    expect(pixelAt(image, 1, 0)).toEqual(BLUE)
    expect(pixelAt(image, 2, 0)).toEqual(CLEAR)
    expect(pixelAt(image, 2, 1)).toEqual(BLUE)
  })

  it('clips painting to the selection mask', () => {
    const image = createImage(3, 3)
    const mask = new Uint8Array(9)
    mask[0] = 1
    const stroke = new PixelPerfectStroke(image, BLUE, mask)
    stroke.start(0, 0)
    stroke.moveTo(1, 1)
    expect(pixelAt(image, 0, 0)).toEqual(BLUE)
    expect(pixelAt(image, 1, 0)).toEqual(CLEAR)
    expect(pixelAt(image, 1, 1)).toEqual(CLEAR)
  })

  it('filters a recorded path into a clean diagonal', () => {
    const path = pixelPerfectPath([
      { x: 0, y: 0 },
      { x: 1, y: 0 },
      { x: 1, y: 1 },
      { x: 2, y: 1 },
      { x: 2, y: 2 },
    ])
    expect(path).toEqual([
      { x: 0, y: 0 },
      { x: 1, y: 1 },
      { x: 2, y: 2 },
    ])
  })
})