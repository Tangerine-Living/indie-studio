import { describe, expect, it } from 'vitest'
import { createImage, pixelAt, setPixel } from './pixels'
import {
  clearSelection,
  extractSelection,
  invertSelection,
  pasteImage,
  replaceColor,
  selectAll,
  selectByColor,
  selectionFromAlpha,
  selectionFromPolygon,
  selectionFromRect,
  translateSelection,
  unionSelection,
} from './selection'

const RED = { r: 255, g: 0, b: 0, a: 255 }
const NEAR_RED = { r: 250, g: 0, b: 0, a: 255 }
const BLUE = { r: 0, g: 0, b: 255, a: 255 }
const GREEN = { r: 0, g: 255, b: 0, a: 255 }
const CLEAR = { r: 0, g: 0, b: 0, a: 0 }

function selectedCount(selection: { mask: Uint8Array }): number {
  let count = 0
  for (let index = 0; index < selection.mask.length; index += 1) if (selection.mask[index]) count += 1
  return count
}

describe('selection masks', () => {
  it('builds a clamped rectangle selection', () => {
    const selection = selectionFromRect(5, 4, 3, 3, 1, 1)
    expect(selection.bounds).toEqual({ x: 1, y: 1, w: 3, h: 3 })
    expect(selectedCount(selection)).toBe(9)
    expect(selection.mask[1 * 5 + 1]).toBe(1)
    expect(selection.mask[0 * 5 + 0]).toBe(0)

    const outside = selectionFromRect(4, 4, -3, -3, -1, -1)
    expect(outside.bounds).toBeNull()
  })

  it('selects everything and inverts back to nothing', () => {
    const all = selectAll(3, 2)
    expect(all.bounds).toEqual({ x: 0, y: 0, w: 3, h: 2 })
    const none = invertSelection(all)
    expect(none.bounds).toBeNull()
    const corner = invertSelection(selectionFromRect(3, 2, 0, 0, 0, 0))
    // Only (0, 0) was selected; everything else — including (0, 1) — is now in.
    expect(corner.bounds).toEqual({ x: 0, y: 0, w: 3, h: 2 })
    expect(corner.mask[0]).toBe(0)
    expect(corner.mask[3]).toBe(1)
  })

  it('rasterizes a lasso polygon at pixel centers', () => {
    const triangle = selectionFromPolygon(5, 5, [
      { x: 0, y: 0 },
      { x: 4, y: 0 },
      { x: 0, y: 4 },
    ])
    expect(triangle.bounds).not.toBeNull()
    expect(triangle.mask[0]).toBe(1)
    expect(triangle.mask[1 * 5 + 1]).toBe(1)
    expect(triangle.mask[4 * 5 + 4]).toBe(0)
    expect(selectedCount(triangle)).toBeGreaterThan(5)
    expect(selectedCount(triangle)).toBeLessThan(15)
  })

  it('grows the wand 4-connected or across the whole image', () => {
    const image = createImage(8, 2)
    for (let y = 0; y < 2; y += 1) {
      for (let x = 0; x < 2; x += 1) setPixel(image, x, y, RED)
      for (let x = 5; x < 7; x += 1) setPixel(image, x, y, RED)
    }
    const connected = selectByColor(image, 0, 0, 0, true)
    expect(selectedCount(connected)).toBe(4)
    expect(connected.bounds).toEqual({ x: 0, y: 0, w: 2, h: 2 })

    const global = selectByColor(image, 0, 0, 0, false)
    expect(selectedCount(global)).toBe(8)
    expect(global.bounds).toEqual({ x: 0, y: 0, w: 7, h: 2 })
  })

  it('honors the wand tolerance', () => {
    const image = createImage(4, 1)
    setPixel(image, 0, 0, RED)
    setPixel(image, 1, 0, NEAR_RED)
    setPixel(image, 2, 0, BLUE)
    expect(selectedCount(selectByColor(image, 0, 0, 0, true))).toBe(1)
    expect(selectedCount(selectByColor(image, 0, 0, 10, true))).toBe(2)
  })

  it('unions and translates masks', () => {
    const left = selectionFromRect(6, 4, 0, 0, 1, 1)
    const right = selectionFromRect(6, 4, 4, 2, 5, 3)
    const union = unionSelection(left, right)
    expect(selectedCount(union)).toBe(8)
    expect(union.bounds).toEqual({ x: 0, y: 0, w: 6, h: 4 })

    const moved = translateSelection(left, 3, 1)
    expect(moved.bounds).toEqual({ x: 3, y: 1, w: 2, h: 2 })
    const clipped = translateSelection(left, -1, -1)
    expect(clipped.bounds).toEqual({ x: 0, y: 0, w: 1, h: 1 })
  })

  it('extracts only selected pixels into a cropped buffer', () => {
    const image = createImage(4, 4)
    setPixel(image, 1, 1, RED)
    setPixel(image, 2, 1, BLUE)
    setPixel(image, 2, 2, GREEN)
    const selection = selectionFromRect(4, 4, 1, 1, 2, 2)
    selection.mask[1 * 4 + 2] = 0
    const lifted = extractSelection(image, selection)
    expect(lifted).not.toBeNull()
    expect(lifted?.x).toBe(1)
    expect(lifted?.y).toBe(1)
    expect(lifted?.image.width).toBe(2)
    expect(pixelAt(lifted!.image, 0, 0)).toEqual(RED)
    expect(pixelAt(lifted!.image, 1, 0)).toEqual(CLEAR)
    expect(pixelAt(lifted!.image, 1, 1)).toEqual(GREEN)
    expect(extractSelection(image, null)).toBeNull()
  })

  it('clears only selected pixels', () => {
    const image = createImage(4, 2)
    for (let x = 0; x < 4; x += 1) setPixel(image, x, 0, RED)
    setPixel(image, 3, 1, BLUE)
    const cleared = clearSelection(image, selectionFromRect(4, 2, 0, 0, 2, 0))
    expect(cleared).toBe(3)
    expect(pixelAt(image, 0, 0)).toEqual(CLEAR)
    expect(pixelAt(image, 2, 0)).toEqual(CLEAR)
    expect(pixelAt(image, 3, 0)).toEqual(RED)
    expect(pixelAt(image, 3, 1)).toEqual(BLUE)
  })

  it('pastes source-over inside the canvas', () => {
    const target = createImage(3, 3)
    const source = createImage(2, 2)
    setPixel(source, 0, 0, RED)
    setPixel(source, 1, 1, BLUE)
    pasteImage(target, source, 2, 2)
    expect(pixelAt(target, 2, 2)).toEqual(RED)
    expect(pixelAt(target, 3, 3)).toEqual(CLEAR)
    expect(pixelAt(target, 0, 0)).toEqual(CLEAR)
  })

  it('maps a pasted image alpha into a positioned selection', () => {
    const image = createImage(2, 2)
    setPixel(image, 1, 1, RED)
    const selection = selectionFromAlpha(image, 2, 2, 4, 4)
    expect(selection.bounds).toEqual({ x: 3, y: 3, w: 1, h: 1 })
    expect(selection.mask[3 * 4 + 3]).toBe(1)
    expect(selection.mask[2 * 4 + 2]).toBe(0)
  })

  it('replaces colors by tolerance, optionally inside a selection', () => {
    const image = createImage(4, 1)
    setPixel(image, 0, 0, RED)
    setPixel(image, 1, 0, NEAR_RED)
    setPixel(image, 2, 0, BLUE)
    expect(replaceColor(image, RED, GREEN, 0)).toBe(1)
    expect(pixelAt(image, 0, 0)).toEqual(GREEN)
    expect(pixelAt(image, 1, 0)).toEqual(NEAR_RED)

    const rest = replaceColor(image, RED, BLUE, 10, selectionFromRect(4, 1, 2, 0, 3, 0))
    expect(rest).toBe(0)
    expect(pixelAt(image, 1, 0)).toEqual(NEAR_RED)

    // GREEN is 255 away from NEAR_RED, so only the near-red pixel changes.
    const all = replaceColor(image, NEAR_RED, BLUE, 10)
    expect(all).toBe(1)
    expect(pixelAt(image, 1, 0)).toEqual(BLUE)
    expect(pixelAt(image, 2, 0)).toEqual(BLUE)
  })
})