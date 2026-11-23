/**
 * Sprite-sheet geometry and metadata. Slicing, packing and the Aseprite /
 * TexturePacker JSON payloads are pure so the editor preview and the Go-side
 * persistence share one definition of the layout.
 */
import { createImage, pixelAt, setPixel } from './pixels'
import type { PixelImage } from './pixels'

export interface SheetInfo {
  frameWidth: number
  frameHeight: number
  frameCount: number
  columns: number
  rows: number
  margin: number
  spacing: number
  fps: number
  loop: boolean
  /** Per-frame hold time in milliseconds; omitted means uniform from `fps`. */
  durations?: number[]
  /** Clip names written to Aseprite frameTags / TexturePacker animations. */
  tags?: string[]
}

const MIN_FRAME_MS = 10
const MAX_FRAME_MS = 10000
const MAX_TAGS = 12

function clampDuration(value: number): number {
  return Math.min(MAX_FRAME_MS, Math.max(MIN_FRAME_MS, Math.round(value)))
}

/** One hold time per frame, filling missing or invalid entries from FPS. */
export function sheetDurations(
  info: Pick<SheetInfo, 'frameCount' | 'fps' | 'durations'>,
): number[] {
  const uniform = Math.max(1, Math.round(1000 / Math.max(1, info.fps)))
  return Array.from({ length: Math.max(0, info.frameCount) }, (_, index) => {
    const raw = info.durations?.[index]
    return raw !== undefined && raw > 0 ? clampDuration(raw) : uniform
  })
}

/** Clip names actually written to the metadata (falls back to `default`). */
export function animationTagNames(info: Pick<SheetInfo, 'tags'>): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const raw of info.tags ?? []) {
    const tag = raw.trim()
    if (!tag || seen.has(tag)) continue
    seen.add(tag)
    out.push(tag)
    if (out.length === MAX_TAGS) break
  }
  return out.length > 0 ? out : ['default']
}

export interface SliceOptions {
  frameWidth: number
  frameHeight: number
  margin?: number
  spacing?: number
}

export interface SliceLayout {
  frameWidth: number
  frameHeight: number
  columns: number
  rows: number
  count: number
  margin: number
  spacing: number
}

export interface FrameRect {
  x: number
  y: number
  w: number
  h: number
}

function whole(value: number, fallback: number): number {
  return Number.isFinite(value) ? Math.floor(value) : fallback
}

/** Number of complete cells a sheet holds with the given frame geometry. */
export function sliceLayout(width: number, height: number, opts: SliceOptions): SliceLayout | null {
  const frameWidth = whole(opts.frameWidth, 0)
  const frameHeight = whole(opts.frameHeight, 0)
  const margin = Math.max(0, whole(opts.margin ?? 0, 0))
  const spacing = Math.max(0, whole(opts.spacing ?? 0, 0))
  if (width < 1 || height < 1 || frameWidth < 1 || frameHeight < 1) return null
  if (frameWidth > width || frameHeight > height) return null

  const usableWidth = width - margin * 2
  const usableHeight = height - margin * 2
  if (usableWidth < frameWidth || usableHeight < frameHeight) return null

  const columns = Math.floor((usableWidth + spacing) / (frameWidth + spacing))
  const rows = Math.floor((usableHeight + spacing) / (frameHeight + spacing))
  if (columns < 1 || rows < 1) return null
  return {
    frameWidth,
    frameHeight,
    columns,
    rows,
    count: columns * rows,
    margin,
    spacing,
  }
}

/** Top-left pixel of one cell in the sheet, row-major. */
export function frameOrigin(
  info: Pick<SheetInfo, 'frameWidth' | 'frameHeight' | 'columns' | 'margin' | 'spacing'>,
  index: number,
): { x: number; y: number } {
  const columns = Math.max(1, info.columns)
  const column = index % columns
  const row = Math.floor(index / columns)
  return {
    x: info.margin + column * (info.frameWidth + info.spacing),
    y: info.margin + row * (info.frameHeight + info.spacing),
  }
}

export function frameRect(
  info: Pick<SheetInfo, 'frameWidth' | 'frameHeight' | 'columns' | 'margin' | 'spacing'>,
  index: number,
): FrameRect {
  const origin = frameOrigin(info, index)
  return { x: origin.x, y: origin.y, w: info.frameWidth, h: info.frameHeight }
}

/** Extracts one cell; out-of-sheet pixels stay transparent. */
export function sliceAt(source: PixelImage, layout: SliceLayout, index: number): PixelImage {
  const out = createImage(layout.frameWidth, layout.frameHeight)
  const origin = frameOrigin(layout, index)
  for (let y = 0; y < out.height; y += 1) {
    for (let x = 0; x < out.width; x += 1) {
      setPixel(out, x, y, pixelAt(source, origin.x + x, origin.y + y))
    }
  }
  return out
}

/** Slices every complete cell into its own image, row-major. */
export function sliceSheet(source: PixelImage, opts: SliceOptions): PixelImage[] {
  const layout = sliceLayout(source.width, source.height, opts)
  if (!layout) return []
  const frames: PixelImage[] = []
  for (let index = 0; index < layout.count; index += 1) {
    frames.push(sliceAt(source, layout, index))
  }
  return frames
}

/** Pixel size of a sheet packed from `count` frames in `columns`. */
export function sheetSize(
  frameWidth: number,
  frameHeight: number,
  count: number,
  columns: number,
  margin = 0,
  spacing = 0,
): { width: number; height: number; rows: number } {
  const width = Math.max(1, whole(frameWidth, 1))
  const height = Math.max(1, whole(frameHeight, 1))
  const total = Math.max(1, whole(count, 1))
  const cols = Math.max(1, Math.min(total, whole(columns, 1)))
  const rows = Math.ceil(total / cols)
  return {
    width: margin * 2 + cols * width + Math.max(0, cols - 1) * spacing,
    height: margin * 2 + rows * height + Math.max(0, rows - 1) * spacing,
    rows,
  }
}

/** Packs frames row-major into a sheet; returns null for an empty list. */
export function composeSheet(
  frames: PixelImage[],
  columns: number,
  margin = 0,
  spacing = 0,
): PixelImage | null {
  if (frames.length === 0) return null
  const first = frames[0]
  const size = sheetSize(first.width, first.height, frames.length, columns, margin, spacing)
  const out = createImage(size.width, size.height)
  const info: SheetInfo = {
    frameWidth: first.width,
    frameHeight: first.height,
    frameCount: frames.length,
    columns: Math.max(1, Math.min(frames.length, whole(columns, 1))),
    rows: size.rows,
    margin,
    spacing,
    fps: 8,
    loop: true,
  }
  for (let index = 0; index < frames.length; index += 1) {
    const frame = frames[index]
    const origin = frameOrigin(info, index)
    for (let y = 0; y < frame.height; y += 1) {
      for (let x = 0; x < frame.width; x += 1) {
        setPixel(out, origin.x + x, origin.y + y, pixelAt(frame, x, y))
      }
    }
  }
  return out
}

export function sheetInfoFor(
  frameWidth: number,
  frameHeight: number,
  count: number,
  columns: number,
  fps: number,
  loop: boolean,
  margin = 0,
  spacing = 0,
  durations?: number[],
  tags?: string[],
): SheetInfo {
  const size = sheetSize(frameWidth, frameHeight, count, columns, margin, spacing)
  return {
    frameWidth,
    frameHeight,
    frameCount: count,
    columns: Math.max(1, Math.min(count, whole(columns, 1))),
    rows: size.rows,
    margin,
    spacing,
    fps: Math.max(1, Math.min(60, whole(fps, 8) || 8)),
    loop,
    ...(durations && durations.length > 0 ? { durations: [...durations] } : {}),
    ...(tags && tags.length > 0 ? { tags: [...tags] } : {}),
  }
}

export function frameName(index: number): string {
  return String(index).padStart(3, '0')
}

/** Aseprite "JSON (Hash)" array exported beside the sheet. */
export function buildAsepriteJSON(info: SheetInfo, sheetWidth: number, sheetHeight: number) {
  const durations = sheetDurations(info)
  const frames: Record<string, unknown> = {}
  for (let index = 0; index < info.frameCount; index += 1) {
    const rect = frameRect(info, index)
    frames[frameName(index)] = {
      frame: rect,
      rotated: false,
      trimmed: false,
      spriteSourceSize: { x: 0, y: 0, w: rect.w, h: rect.h },
      sourceSize: { w: rect.w, h: rect.h },
      duration: durations[index],
    }
  }
  const last = Math.max(0, info.frameCount - 1)
  const frameTags = animationTagNames(info).map((name) => ({
    name,
    from: 0,
    to: last,
    direction: 'forward',
  }))
  return {
    frames,
    meta: {
      app: 'pixelbench',
      version: '1.0',
      image: 'output.png',
      format: 'RGBA8888',
      size: { w: sheetWidth, h: sheetHeight },
      scale: '1',
      frameTags,
      layers: [{ name: 'Layer 1', opacity: 255, blendMode: 'normal' }],
      slices: [],
    },
  }
}

/** TexturePacker "JSON (Hash)" file exported beside the sheet. */
export function buildTexturePackerJSON(info: SheetInfo, sheetWidth: number, sheetHeight: number) {
  const frames: Record<string, unknown> = {}
  const animation: string[] = []
  for (let index = 0; index < info.frameCount; index += 1) {
    const rect = frameRect(info, index)
    const name = frameName(index)
    animation.push(name)
    frames[name] = {
      frame: rect,
      rotated: false,
      trimmed: false,
      spriteSourceSize: { x: 0, y: 0, w: rect.w, h: rect.h },
      sourceSize: { w: rect.w, h: rect.h },
    }
  }
  const animations: Record<string, string[]> = {}
  for (const name of animationTagNames(info)) animations[name] = animation
  return {
    frames,
    animations,
    meta: {
      app: 'pixelbench',
      version: '1.0',
      image: 'output.png',
      format: 'RGBA8888',
      size: { w: sheetWidth, h: sheetHeight },
      scale: '1',
    },
  }
}