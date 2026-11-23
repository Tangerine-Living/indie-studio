/**
 * Pure raster tools. Every function works on a flat RGBA buffer so it can be
 * unit-tested without a canvas and reused by the editor, thumbnails and,
 * later, the Go-side post-processing pipeline.
 */

export interface PixelImage {
  width: number
  height: number
  /** Straight RGBA, 4 bytes per pixel, row-major. */
  data: Uint8ClampedArray
}

export interface RGBA {
  r: number
  g: number
  b: number
  a: number
}

export const TRANSPARENT: RGBA = { r: 0, g: 0, b: 0, a: 0 }

export function createImage(width: number, height: number): PixelImage {
  const w = Math.max(1, Math.floor(width))
  const h = Math.max(1, Math.floor(height))
  return { width: w, height: h, data: new Uint8ClampedArray(w * h * 4) }
}

export function cloneImage(image: PixelImage): PixelImage {
  return { width: image.width, height: image.height, data: new Uint8ClampedArray(image.data) }
}

export function inBounds(image: PixelImage, x: number, y: number): boolean {
  return x >= 0 && y >= 0 && x < image.width && y < image.height
}

export function pixelAt(image: PixelImage, x: number, y: number): RGBA {
  if (!inBounds(image, x, y)) return { ...TRANSPARENT }
  const at = (y * image.width + x) * 4
  return {
    r: image.data[at],
    g: image.data[at + 1],
    b: image.data[at + 2],
    a: image.data[at + 3],
  }
}

/** Overwrites one pixel. Out-of-bounds writes are ignored. */
export function setPixel(image: PixelImage, x: number, y: number, color: RGBA): void {
  if (!inBounds(image, x, y)) return
  const at = (y * image.width + x) * 4
  image.data[at] = color.r
  image.data[at + 1] = color.g
  image.data[at + 2] = color.b
  image.data[at + 3] = color.a
}

/** Source-over blend of one pixel, used for semi-transparent brushes. */
export function blendPixel(image: PixelImage, x: number, y: number, color: RGBA): void {
  if (!inBounds(image, x, y) || color.a === 0) return
  if (color.a === 255) {
    setPixel(image, x, y, color)
    return
  }
  const at = (y * image.width + x) * 4
  const srcA = color.a / 255
  const dstA = image.data[at + 3] / 255
  const outA = srcA + dstA * (1 - srcA)
  if (outA === 0) return
  image.data[at] = Math.round((color.r * srcA + image.data[at] * dstA * (1 - srcA)) / outA)
  image.data[at + 1] = Math.round((color.g * srcA + image.data[at + 1] * dstA * (1 - srcA)) / outA)
  image.data[at + 2] = Math.round((color.b * srcA + image.data[at + 2] * dstA * (1 - srcA)) / outA)
  image.data[at + 3] = Math.round(outA * 255)
}

/**
 * Square brush centered on (x, y). Size 1 is a single pixel; size 2 covers
 * (x, y) and the pixels below-right, matching common pixel editors. An
 * optional mask (a selection of the same size) clips the stamp.
 */
export function stamp(
  image: PixelImage,
  x: number,
  y: number,
  size: number,
  color: RGBA,
  mask?: Uint8Array | null,
): void {
  const s = Math.max(1, Math.floor(size))
  const offset = Math.floor((s - 1) / 2)
  const endX = x - offset + s - 1
  const endY = y - offset + s - 1
  for (let py = y - offset; py <= endY; py += 1) {
    for (let px = x - offset; px <= endX; px += 1) {
      if (mask && (!inBounds(image, px, py) || !mask[py * image.width + px])) continue
      // A fully transparent brush erases instead of blending.
      if (color.a === 0) setPixel(image, px, py, color)
      else blendPixel(image, px, py, color)
    }
  }
}

export interface Brush {
  size: number
  color: RGBA
  /** Optional selection mask; painting is clipped to selected pixels. */
  mask?: Uint8Array | null
}

/** Pixels a Bresenham line passes through, endpoints included. */
export function linePoints(
  x0: number,
  y0: number,
  x1: number,
  y1: number,
): Array<{ x: number; y: number }> {
  let x = Math.round(x0)
  let y = Math.round(y0)
  const tx = Math.round(x1)
  const ty = Math.round(y1)
  const dx = Math.abs(tx - x)
  const dy = Math.abs(ty - y)
  const sx = x < tx ? 1 : -1
  const sy = y < ty ? 1 : -1
  let err = dx - dy
  const points: Array<{ x: number; y: number }> = []
  for (;;) {
    points.push({ x, y })
    if (x === tx && y === ty) break
    const e2 = err * 2
    if (e2 > -dy) {
      err -= dy
      x += sx
    }
    if (e2 < dx) {
      err += dx
      y += sy
    }
  }
  return points
}

export function drawLine(
  image: PixelImage,
  x0: number,
  y0: number,
  x1: number,
  y1: number,
  brush: Brush,
): void {
  let x = Math.round(x0)
  let y = Math.round(y0)
  const tx = Math.round(x1)
  const ty = Math.round(y1)
  const dx = Math.abs(tx - x)
  const dy = Math.abs(ty - y)
  const sx = x < tx ? 1 : -1
  const sy = y < ty ? 1 : -1
  let err = dx - dy
  for (;;) {
    stamp(image, x, y, brush.size, brush.color, brush.mask)
    if (x === tx && y === ty) break
    const e2 = err * 2
    if (e2 > -dy) {
      err -= dy
      x += sx
    }
    if (e2 < dx) {
      err += dx
      y += sy
    }
  }
}

export function drawRect(
  image: PixelImage,
  x0: number,
  y0: number,
  x1: number,
  y1: number,
  brush: Brush,
  filled = false,
): void {
  const left = Math.min(x0, x1)
  const right = Math.max(x0, x1)
  const top = Math.min(y0, y1)
  const bottom = Math.max(y0, y1)
  if (filled) {
    for (let y = top; y <= bottom; y += 1) {
      drawLine(image, left, y, right, y, brush)
    }
    return
  }
  drawLine(image, left, top, right, top, brush)
  drawLine(image, right, top, right, bottom, brush)
  drawLine(image, right, bottom, left, bottom, brush)
  drawLine(image, left, bottom, left, top, brush)
}

export function drawEllipse(
  image: PixelImage,
  x0: number,
  y0: number,
  x1: number,
  y1: number,
  brush: Brush,
  filled = false,
): void {
  const left = Math.min(x0, x1)
  const right = Math.max(x0, x1)
  const top = Math.min(y0, y1)
  const bottom = Math.max(y0, y1)
  const rx = (right - left) / 2
  const ry = (bottom - top) / 2
  const cx = (left + right) / 2
  const cy = (top + bottom) / 2
  if (rx < 0.5 || ry < 0.5) {
    drawLine(image, left, top, right, bottom, brush)
    return
  }
  for (let y = top; y <= bottom; y += 1) {
    const dy = (y - cy) / (ry + 0.5)
    const span = Math.sqrt(Math.max(0, 1 - dy * dy)) * (rx + 0.5)
    // Pixel centers sit at +0.5, so the row spans from the first pixel whose
    // center is inside the ellipse to the last one, clamped to the image.
    const start = Math.max(0, Math.ceil(cx - span - 0.5))
    const end = Math.min(image.width - 1, Math.floor(cx + span - 0.5))
    if (filled) {
      drawLine(image, start, y, end, y, brush)
      continue
    }
    // Outline: paint the outer 1px of each row plus the row ends.
    stamp(image, start, y, brush.size, brush.color, brush.mask)
    stamp(image, end, y, brush.size, brush.color, brush.mask)
    const above = Math.abs(y - 1 - cy) / (ry + 0.5)
    const below = Math.abs(y + 1 - cy) / (ry + 0.5)
    if (above > 1 || below > 1 || Math.abs(dy) > 0.92) {
      drawLine(image, start, y, end, y, brush)
    }
  }
}

export interface FillOptions {
  /** Selection mask; pixels outside it are left untouched. */
  mask?: Uint8Array | null
  /** false fills every matching pixel in the image, connected or not. */
  contiguous?: boolean
}

/**
 * Flood fill with per-channel tolerance. `tolerance` is 0..255 and applies to
 * every channel delta. Connected filling is 4-connected.
 */
export function floodFill(
  image: PixelImage,
  x: number,
  y: number,
  color: RGBA,
  tolerance = 0,
  options: FillOptions = {},
): number {
  const sx = Math.floor(x)
  const sy = Math.floor(y)
  if (!inBounds(image, sx, sy)) return 0
  const mask = options.mask ?? null
  if (mask && !mask[sy * image.width + sx]) return 0
  const target = pixelAt(image, sx, sy)
  if (matches(target, color, 0)) return 0
  const tol = Math.max(0, Math.min(255, tolerance))
  let filled = 0
  if (options.contiguous === false) {
    for (let py = 0; py < image.height; py += 1) {
      for (let px = 0; px < image.width; px += 1) {
        const index = py * image.width + px
        if (mask && !mask[index]) continue
        if (!matches(pixelAt(image, px, py), target, tol)) continue
        setPixel(image, px, py, color)
        filled += 1
      }
    }
    return filled
  }
  const visited = new Uint8Array(image.width * image.height)
  const stack: number[] = [sx, sy]
  while (stack.length > 0) {
    const py = stack.pop() as number
    const px = stack.pop() as number
    if (px < 0 || py < 0 || px >= image.width || py >= image.height) continue
    const index = py * image.width + px
    if (visited[index]) continue
    visited[index] = 1
    if (mask && !mask[index]) continue
    if (!matches(pixelAt(image, px, py), target, tol)) continue
    setPixel(image, px, py, color)
    filled += 1
    stack.push(px + 1, py, px - 1, py, px, py + 1, px, py - 1)
  }
  return filled
}

/** Per-channel color comparison used by fills, the wand and palette recolor. */
export function colorMatches(a: RGBA, b: RGBA, tolerance: number): boolean {
  return (
    Math.abs(a.r - b.r) <= tolerance &&
    Math.abs(a.g - b.g) <= tolerance &&
    Math.abs(a.b - b.b) <= tolerance &&
    Math.abs(a.a - b.a) <= tolerance
  )
}

function matches(a: RGBA, b: RGBA, tolerance: number): boolean {
  return colorMatches(a, b, tolerance)
}

/* ------------------------------------------------------------------ colors */

const HEX = /^#?([0-9a-f]{3,8})$/i

export function hexToRGBA(hex: string): RGBA | null {
  const match = HEX.exec(hex.trim())
  if (!match) return null
  let value = match[1]
  if (value.length === 3) {
    value = value
      .split('')
      .map((char) => char + char)
      .join('')
  }
  if (value.length === 6) value += 'ff'
  if (value.length !== 8) return null
  return {
    r: parseInt(value.slice(0, 2), 16),
    g: parseInt(value.slice(2, 4), 16),
    b: parseInt(value.slice(4, 6), 16),
    a: parseInt(value.slice(6, 8), 16),
  }
}

export function rgbaToHex(color: RGBA): string {
  const part = (n: number) => Math.round(Math.max(0, Math.min(255, n))).toString(16).padStart(2, '0')
  const base = `#${part(color.r)}${part(color.g)}${part(color.b)}`
  return color.a >= 255 ? base : `${base}${part(color.a)}`
}

/** Nearest-neighbour integer upscale, used for previews and thumbnails. */
export function upscaleNearest(image: PixelImage, factor: number): PixelImage {
  const scale = Math.max(1, Math.floor(factor))
  if (scale === 1) return cloneImage(image)
  const out = createImage(image.width * scale, image.height * scale)
  for (let y = 0; y < out.height; y += 1) {
    const sy = Math.floor(y / scale)
    for (let x = 0; x < out.width; x += 1) {
      const sx = Math.floor(x / scale)
      const from = (sy * image.width + sx) * 4
      const to = (y * out.width + x) * 4
      out.data[to] = image.data[from]
      out.data[to + 1] = image.data[from + 1]
      out.data[to + 2] = image.data[from + 2]
      out.data[to + 3] = image.data[from + 3]
    }
  }
  return out
}