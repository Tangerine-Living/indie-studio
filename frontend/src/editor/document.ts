/**
 * Editor document model: a fixed-size raster with ordered layers and an
 * animation timeline. Every layer holds one buffer per frame, so frames stay
 * aligned by construction and compositing is a plain per-frame walk.
 */
import { blendPixel, cloneImage, createImage, pixelAt, setPixel } from './pixels'
import type { PixelImage } from './pixels'
import { newID } from '../lib/utils'

export interface Layer {
  id: string
  name: string
  visible: boolean
  /** 0..1 */
  opacity: number
  /** One image per animation frame; length always equals doc.frameCount. */
  frames: PixelImage[]
}

export interface EditorDoc {
  width: number
  height: number
  layers: Layer[]
  activeLayerId: string
  frameCount: number
  activeFrame: number
  /** Playback rate; also the default when exporting animation tags. */
  fps: number
  /** Per-frame hold time in milliseconds; always one entry per frame. */
  frameDurations: number[]
  /** Clip names saved with the next sprite-sheet version. */
  tags: string[]
}

/** Uniform hold time for one frame at `fps`, in whole milliseconds. */
export function uniformFrameDuration(fps: number): number {
  return Math.max(1, Math.round(1000 / Math.max(1, Math.round(fps) || 8)))
}

export function createLayer(name: string, width: number, height: number, frameCount = 1): Layer {
  const count = Math.max(1, Math.floor(frameCount))
  return {
    id: newID(),
    name,
    visible: true,
    opacity: 1,
    frames: Array.from({ length: count }, () => createImage(width, height)),
  }
}

export function createDoc(width: number, height: number, fps = 8): EditorDoc {
  const w = Math.max(1, Math.min(4096, Math.floor(width)))
  const h = Math.max(1, Math.min(4096, Math.floor(height)))
  const base = createLayer('图层 1', w, h, 1)
  const rate = Math.max(1, Math.min(60, Math.round(fps) || 8))
  return {
    width: w,
    height: h,
    layers: [base],
    activeLayerId: base.id,
    frameCount: 1,
    activeFrame: 0,
    fps: rate,
    frameDurations: [uniformFrameDuration(rate)],
    tags: [],
  }
}

export function activeLayer(doc: EditorDoc): Layer {
  return doc.layers.find((layer) => layer.id === doc.activeLayerId) ?? doc.layers[doc.layers.length - 1]
}

export function layerById(doc: EditorDoc, id: string): Layer | undefined {
  return doc.layers.find((layer) => layer.id === id)
}

export function layerFrame(layer: Layer, frame: number): PixelImage {
  return layer.frames[frame] ?? layer.frames[0]
}

/** Buffer the drawing tools edit: the active layer on the active frame. */
export function activeLayerImage(doc: EditorDoc): PixelImage {
  return layerFrame(activeLayer(doc), doc.activeFrame)
}

/** Composites visible layers bottom-up for one frame, honoring layer opacity. */
export function compositeDoc(doc: EditorDoc, frame = doc.activeFrame): PixelImage {
  const out = createImage(doc.width, doc.height)
  for (const layer of doc.layers) {
    if (!layer.visible || layer.opacity <= 0) continue
    const source = layerFrame(layer, frame)
    const src = source.data
    const dst = out.data
    const opacity = layer.opacity
    for (let at = 0; at < dst.length; at += 4) {
      const sa = (src[at + 3] / 255) * opacity
      if (sa <= 0) continue
      const da = dst[at + 3] / 255
      const outA = sa + da * (1 - sa)
      if (outA <= 0) continue
      dst[at] = Math.round((src[at] * sa + dst[at] * da * (1 - sa)) / outA)
      dst[at + 1] = Math.round((src[at + 1] * sa + dst[at + 1] * da * (1 - sa)) / outA)
      dst[at + 2] = Math.round((src[at + 2] * sa + dst[at + 2] * da * (1 - sa)) / outA)
      dst[at + 3] = Math.round(outA * 255)
    }
  }
  return out
}

/** Copies `src` onto `dst` at an offset, source-over. */
export function blit(dst: PixelImage, src: PixelImage, offsetX = 0, offsetY = 0): void {
  for (let y = 0; y < src.height; y += 1) {
    for (let x = 0; x < src.width; x += 1) {
      const color = pixelAt(src, x, y)
      if (color.a === 0) continue
      blendPixel(dst, x + offsetX, y + offsetY, color)
    }
  }
}

/* --------------------------------------------------------------- frames */

/**
 * Inserts a new frame after the active one. By default it duplicates the
 * current frame, matching how animators refine an existing pose.
 */
export function addFrame(doc: EditorDoc, copyCurrent = true): number {
  const index = Math.min(doc.activeFrame + 1, doc.frameCount)
  const hold = doc.frameDurations[doc.activeFrame] ?? uniformFrameDuration(doc.fps)
  for (const layer of doc.layers) {
    const source = layerFrame(layer, doc.activeFrame)
    layer.frames.splice(index, 0, copyCurrent ? cloneImage(source) : createImage(doc.width, doc.height))
  }
  doc.frameDurations.splice(index, 0, hold)
  doc.frameCount += 1
  doc.activeFrame = index
  return index
}

export function removeFrame(doc: EditorDoc, index: number): boolean {
  if (doc.frameCount <= 1 || index < 0 || index >= doc.frameCount) return false
  for (const layer of doc.layers) layer.frames.splice(index, 1)
  doc.frameDurations.splice(index, 1)
  if (doc.frameDurations.length === 0) doc.frameDurations.push(uniformFrameDuration(doc.fps))
  doc.frameCount -= 1
  doc.activeFrame = Math.min(doc.activeFrame, doc.frameCount - 1)
  return true
}

/* --------------------------------------------------------------- history */

/** Immutable per-layer pixel copy + layer metadata for undo. */
export interface DocSnapshot {
  width: number
  height: number
  activeLayerId: string
  activeFrame: number
  layers: Array<Pick<Layer, 'id' | 'name' | 'visible' | 'opacity'> & { frames: PixelImage[] }>
  frameDurations: number[]
  tags: string[]
}

export function snapshotDoc(doc: EditorDoc): DocSnapshot {
  return {
    width: doc.width,
    height: doc.height,
    activeLayerId: doc.activeLayerId,
    activeFrame: doc.activeFrame,
    layers: doc.layers.map((layer) => ({
      id: layer.id,
      name: layer.name,
      visible: layer.visible,
      opacity: layer.opacity,
      frames: layer.frames.map(cloneImage),
    })),
    frameDurations: [...doc.frameDurations],
    tags: [...doc.tags],
  }
}

export function restoreDoc(doc: EditorDoc, snapshot: DocSnapshot): void {
  doc.layers = snapshot.layers.map((layer) => ({
    id: layer.id,
    name: layer.name,
    visible: layer.visible,
    opacity: layer.opacity,
    frames: layer.frames.map(cloneImage),
  }))
  doc.width = snapshot.width
  doc.height = snapshot.height
  doc.activeLayerId = snapshot.activeLayerId
  doc.frameCount = doc.layers[0]?.frames.length ?? 1
  doc.activeFrame = Math.min(snapshot.activeFrame, doc.frameCount - 1)
  const fallback = uniformFrameDuration(doc.fps)
  doc.frameDurations = Array.from(
    { length: doc.frameCount },
    (_, index) => snapshot.frameDurations?.[index] ?? fallback,
  )
  doc.tags = [...(snapshot.tags ?? [])]
}

/** Replaces every layer frame with sliced cells of the sheet. */
export function applySlices(doc: EditorDoc, slices: PixelImage[]): boolean {
  if (slices.length === 0) return false
  const width = slices[0].width
  const height = slices[0].height
  if (doc.layers.length === 0) {
    const layer = createLayer('图层 1', width, height, slices.length)
    layer.frames = slices.map(cloneImage)
    doc.layers.push(layer)
    doc.activeLayerId = layer.id
  }
  doc.width = width
  doc.height = height
  doc.frameCount = slices.length
  doc.activeFrame = 0
  doc.frameDurations = Array.from({ length: slices.length }, () => uniformFrameDuration(doc.fps))
  doc.layers.forEach((layer, index) => {
    layer.frames = slices.map((slice, frame) =>
      index === 0 ? cloneImage(slice) : createImage(width, height),
    )
  })
  return true
}


/** Fills one layer frame with a color; alpha 0 clears it. */
export function fillLayer(
  layer: Layer,
  color: { r: number; g: number; b: number; a: number },
  frame = 0,
): void {
  const image = layerFrame(layer, frame)
  for (let y = 0; y < image.height; y += 1) {
    for (let x = 0; x < image.width; x += 1) setPixel(image, x, y, color)
  }
}