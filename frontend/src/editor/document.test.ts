import { describe, expect, it } from 'vitest'
import {
  activeLayer,
  activeLayerImage,
  addFrame,
  applySlices,
  compositeDoc,
  createDoc,
  createLayer,
  fillLayer,
  removeFrame,
  restoreDoc,
  snapshotDoc,
} from './document'
import { createImage, pixelAt, setPixel, stamp } from './pixels'

const RED = { r: 255, g: 0, b: 0, a: 255 }
const BLUE = { r: 0, g: 0, b: 255, a: 255 }

describe('editor document', () => {
  it('starts with one active transparent layer and frame', () => {
    const doc = createDoc(8, 6, 8)
    expect(doc.width).toBe(8)
    expect(doc.height).toBe(6)
    expect(doc.frameCount).toBe(1)
    expect(doc.activeFrame).toBe(0)
    expect(doc.layers).toHaveLength(1)
    expect(doc.layers[0]?.frames).toHaveLength(1)
    expect(activeLayer(doc).id).toBe(doc.activeLayerId)
    expect(pixelAt(compositeDoc(doc), 0, 0).a).toBe(0)
  })

  it('composites layers bottom-up with opacity', () => {
    const doc = createDoc(2, 2)
    fillLayer(doc.layers[0], RED)
    const top = createLayer('顶部', 2, 2)
    fillLayer(top, BLUE)
    top.opacity = 0.5
    doc.layers.push(top)

    const merged = pixelAt(compositeDoc(doc), 0, 0)
    expect(merged.a).toBe(255)
    expect(merged.r).toBe(128)
    expect(merged.b).toBe(128)
  })

  it('ignores hidden layers', () => {
    const doc = createDoc(2, 2)
    fillLayer(doc.layers[0], RED)
    const top = createLayer('隐藏', 2, 2)
    fillLayer(top, BLUE)
    top.visible = false
    doc.layers.push(top)
    expect(pixelAt(compositeDoc(doc), 1, 1)).toEqual(RED)
  })

  it('duplicates the current frame across every layer', () => {
    const doc = createDoc(4, 4)
    const top = createLayer('顶部', 4, 4)
    doc.layers.push(top)
    doc.activeLayerId = top.id
    stamp(activeLayerImage(doc), 1, 1, 1, RED)
    stamp(doc.layers[0].frames[0], 2, 2, 1, BLUE)

    const index = addFrame(doc)
    expect(index).toBe(1)
    expect(doc.frameCount).toBe(2)
    expect(doc.activeFrame).toBe(1)
    expect(doc.layers.every((layer) => layer.frames.length === 2)).toBe(true)
    // The duplicated frame starts as an exact copy, on every layer.
    expect(pixelAt(compositeDoc(doc, 1), 1, 1)).toEqual(RED)
    expect(pixelAt(compositeDoc(doc, 1), 2, 2)).toEqual(BLUE)

    // Frames then diverge independently.
    stamp(doc.layers[0].frames[1], 3, 3, 1, BLUE)
    expect(pixelAt(compositeDoc(doc, 1), 3, 3)).toEqual(BLUE)
    expect(pixelAt(compositeDoc(doc, 0), 3, 3).a).toBe(0)
  })

  it('adds blank frames and removes frames, keeping one frame minimum', () => {
    const doc = createDoc(3, 3)
    fillLayer(doc.layers[0], RED)
    addFrame(doc, false)
    expect(doc.frameCount).toBe(2)
    expect(pixelAt(compositeDoc(doc, 1), 0, 0).a).toBe(0)

    expect(removeFrame(doc, 0)).toBe(true)
    expect(doc.frameCount).toBe(1)
    expect(doc.activeFrame).toBe(0)
    expect(removeFrame(doc, 0)).toBe(false)
    expect(removeFrame(doc, 5)).toBe(false)
  })

  it('snapshots and restores pixels, frames and metadata', () => {
    const doc = createDoc(4, 4)
    stamp(activeLayerImage(doc), 1, 1, 1, RED)
    doc.layers[0].opacity = 0.4
    addFrame(doc)
    stamp(activeLayerImage(doc), 2, 2, 1, BLUE)
    const snapshot = snapshotDoc(doc)

    removeFrame(doc, 1)
    stamp(activeLayerImage(doc), 3, 3, 1, RED)
    doc.layers[0].opacity = 1
    doc.layers.push(createLayer('新图层', 4, 4))

    restoreDoc(doc, snapshot)
    expect(doc.layers).toHaveLength(1)
    expect(doc.frameCount).toBe(2)
    expect(doc.activeFrame).toBe(1)
    expect(doc.layers[0].opacity).toBe(0.4)
    expect(pixelAt(doc.layers[0].frames[0], 1, 1)).toEqual(RED)
    expect(pixelAt(doc.layers[0].frames[1], 2, 2)).toEqual(BLUE)
    expect(pixelAt(doc.layers[0].frames[0], 3, 3).a).toBe(0)
  })

  it('keeps per-frame durations and tags aligned through frame edits', () => {
    const doc = createDoc(4, 4, 10)
    expect(doc.frameDurations).toEqual([100])
    doc.frameDurations[0] = 70
    doc.tags = ['idle']

    addFrame(doc)
    expect(doc.frameDurations).toEqual([70, 70])
    doc.frameDurations[1] = 300

    const snapshot = snapshotDoc(doc)
    removeFrame(doc, 0)
    expect(doc.frameDurations).toEqual([300])
    expect(doc.frameCount).toBe(1)

    restoreDoc(doc, snapshot)
    expect(doc.frameDurations).toEqual([70, 300])
    expect(doc.tags).toEqual(['idle'])
  })

  it('applies sliced sheet cells as frames', () => {
    const doc = createDoc(8, 6)
    const first = createImage(4, 3)
    setPixel(first, 0, 0, RED)
    const second = createImage(4, 3)
    setPixel(second, 3, 2, BLUE)

    expect(applySlices(doc, [first, second])).toBe(true)
    expect(doc.width).toBe(4)
    expect(doc.height).toBe(3)
    expect(doc.frameCount).toBe(2)
    expect(doc.frameDurations).toEqual([125, 125])
    expect(doc.layers[0].frames).toHaveLength(2)
    expect(pixelAt(compositeDoc(doc, 0), 0, 0)).toEqual(RED)
    expect(pixelAt(compositeDoc(doc, 1), 3, 2)).toEqual(BLUE)
    expect(applySlices(doc, [])).toBe(false)
  })

  it('restores the canvas size when undoing a slice', () => {
    const doc = createDoc(8, 6)
    const before = snapshotDoc(doc)
    applySlices(doc, [createImage(3, 3), createImage(3, 3)])
    expect(doc.width).toBe(3)

    restoreDoc(doc, before)
    expect(doc.width).toBe(8)
    expect(doc.height).toBe(6)
    expect(doc.frameCount).toBe(1)
  })
})