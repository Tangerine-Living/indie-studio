/**
 * Pixel selection masks. A selection is a doc-sized 0/1 mask plus its cached
 * bounding box, so every operation is a flat array walk and can be tested
 * without a canvas. Masks are never persisted: they live in the editor only.
 */
import {
  colorMatches,
  createImage,
  inBounds,
  pixelAt,
  setPixel,
  type PixelImage,
  type RGBA,
  TRANSPARENT,
} from './pixels'

export interface SelectionBounds {
  x: number
  y: number
  w: number
  h: number
}

export interface Selection {
  width: number
  height: number
  /** 1 = selected, 0 = not selected. Length width * height. */
  mask: Uint8Array
  /** Cached tight box around the mask, null when nothing is selected. */
  bounds: SelectionBounds | null
}

export function emptySelection(width: number, height: number): Selection {
  return {
    width: Math.max(1, Math.floor(width)),
    height: Math.max(1, Math.floor(height)),
    mask: new Uint8Array(Math.max(1, Math.floor(width)) * Math.max(1, Math.floor(height))),
    bounds: null,
  }
}

/** Wraps a mask and computes its bounding box. */
export function selectionFromMask(width: number, height: number, mask: Uint8Array): Selection {
  const w = Math.max(1, Math.floor(width))
  const h = Math.max(1, Math.floor(height))
  const copy = new Uint8Array(w * h)
  if (mask.length === w * h) copy.set(mask)
  return { width: w, height: h, mask: copy, bounds: computeBounds(copy, w, h) }
}

function computeBounds(mask: Uint8Array, width: number, height: number): SelectionBounds | null {
  let minX = width
  let minY = height
  let maxX = -1
  let maxY = -1
  for (let y = 0; y < height; y += 1) {
    const row = y * width
    for (let x = 0; x < width; x += 1) {
      if (!mask[row + x]) continue
      if (x < minX) minX = x
      if (x > maxX) maxX = x
      if (y < minY) minY = y
      if (y > maxY) maxY = y
    }
  }
  if (maxX < 0) return null
  return { x: minX, y: minY, w: maxX - minX + 1, h: maxY - minY + 1 }
}


/** Rectangle marquee; coordinates are inclusive pixel positions. */
export function selectionFromRect(
  width: number,
  height: number,
  x0: number,
  y0: number,
  x1: number,
  y1: number,
): Selection {
  const sel = emptySelection(width, height)
  const left = Math.max(0, Math.min(x0, x1))
  const right = Math.min(sel.width - 1, Math.max(x0, x1))
  const top = Math.max(0, Math.min(y0, y1))
  const bottom = Math.min(sel.height - 1, Math.max(y0, y1))
  if (right < left || bottom < top) return sel
  for (let y = top; y <= bottom; y += 1) {
    sel.mask.fill(1, y * sel.width + left, y * sel.width + right + 1)
  }
  sel.bounds = { x: left, y: top, w: right - left + 1, h: bottom - top + 1 }
  return sel
}

export function selectAll(width: number, height: number): Selection {
  return selectionFromRect(width, height, 0, 0, width - 1, height - 1)
}

/** Even-odd polygon fill sampled at pixel centers, used by the lasso. */
export function selectionFromPolygon(
  width: number,
  height: number,
  points: ReadonlyArray<{ x: number; y: number }>,
): Selection {
  const sel = emptySelection(width, height)
  if (points.length < 3) return sel
  let minY = height - 1
  let maxY = 0
  for (const point of points) {
    if (point.y < minY) minY = point.y
    if (point.y > maxY) maxY = point.y
  }
  minY = Math.max(0, Math.floor(minY))
  maxY = Math.min(sel.height - 1, Math.ceil(maxY))
  const crossings: number[] = []
  for (let y = minY; y <= maxY; y += 1) {
    const sampleY = y + 0.5
    crossings.length = 0
    for (let i = 0; i < points.length; i += 1) {
      const a = points[i]
      const b = points[(i + 1) % points.length]
      if ((a.y <= sampleY && b.y > sampleY) || (b.y <= sampleY && a.y > sampleY)) {
        const t = (sampleY - a.y) / (b.y - a.y)
        crossings.push(a.x + t * (b.x - a.x))
      }
    }
    if (crossings.length < 2) continue
    crossings.sort((m, n) => m - n)
    for (let i = 0; i + 1 < crossings.length; i += 2) {
      const start = Math.max(0, Math.ceil(crossings[i] - 0.5))
      const end = Math.min(sel.width - 1, Math.ceil(crossings[i + 1] - 0.5) - 1)
      if (end < start) continue
      sel.mask.fill(1, y * sel.width + start, y * sel.width + end + 1)
    }
  }
  sel.bounds = computeBounds(sel.mask, sel.width, sel.height)
  return sel
}

/**
 * Magic wand. With `contiguous` the region grows 4-connected from the click,
 * otherwise every pixel within tolerance is selected. `limit` narrows the
 * search to an existing mask (used for Shift-adding a region).
 */
export function selectByColor(
  image: PixelImage,
  x: number,
  y: number,
  tolerance = 0,
  contiguous = true,
  limit?: Selection | null,
): Selection {
  const sel = emptySelection(image.width, image.height)
  if (!inBounds(image, x, y)) return sel
  const target = pixelAt(image, x, y)
  const tol = Math.max(0, Math.min(255, tolerance))
  const allowed = limit?.mask
  if (!contiguous) {
    for (let py = 0; py < image.height; py += 1) {
      for (let px = 0; px < image.width; px += 1) {
        const index = py * image.width + px
        if (allowed && !allowed[index]) continue
        if (colorMatches(pixelAt(image, px, py), target, tol)) sel.mask[index] = 1
      }
    }
    sel.bounds = computeBounds(sel.mask, sel.width, sel.height)
    return sel
  }
  const visited = new Uint8Array(image.width * image.height)
  const stack: number[] = [x, y]
  while (stack.length > 0) {
    const py = stack.pop() as number
    const px = stack.pop() as number
    if (px < 0 || py < 0 || px >= image.width || py >= image.height) continue
    const index = py * image.width + px
    if (visited[index]) continue
    visited[index] = 1
    if (allowed && !allowed[index]) continue
    if (!colorMatches(pixelAt(image, px, py), target, tol)) continue
    sel.mask[index] = 1
    stack.push(px + 1, py, px - 1, py, px, py + 1, px, py - 1)
  }
  sel.bounds = computeBounds(sel.mask, sel.width, sel.height)
  return sel
}

export function unionSelection(a: Selection, b: Selection): Selection {
  const sel = selectionFromMask(a.width, a.height, a.mask)
  if (a.width === b.width && a.height === b.height) {
    for (let i = 0; i < sel.mask.length; i += 1) if (b.mask[i]) sel.mask[i] = 1
  }
  sel.bounds = computeBounds(sel.mask, sel.width, sel.height)
  return sel
}

export function invertSelection(selection: Selection): Selection {
  const sel = selectionFromMask(selection.width, selection.height, selection.mask)
  for (let i = 0; i < sel.mask.length; i += 1) sel.mask[i] = sel.mask[i] ? 0 : 1
  sel.bounds = computeBounds(sel.mask, sel.width, sel.height)
  return sel
}

/** Moves a mask by (dx, dy), clipping anything pushed off the canvas. */
export function translateSelection(selection: Selection, dx: number, dy: number): Selection {
  const sel = emptySelection(selection.width, selection.height)
  for (let y = 0; y < selection.height; y += 1) {
    const targetY = y + dy
    if (targetY < 0 || targetY >= sel.height) continue
    for (let x = 0; x < selection.width; x += 1) {
      if (!selection.mask[y * selection.width + x]) continue
      const targetX = x + dx
      if (targetX < 0 || targetX >= sel.width) continue
      sel.mask[targetY * sel.width + targetX] = 1
    }
  }
  sel.bounds = computeBounds(sel.mask, sel.width, sel.height)
  return sel
}

/** Builds a selection from a pasted image's non-transparent pixels. */
export function selectionFromAlpha(
  image: PixelImage,
  offsetX: number,
  offsetY: number,
  width: number,
  height: number,
): Selection {
  const sel = emptySelection(width, height)
  for (let y = 0; y < image.height; y += 1) {
    for (let x = 0; x < image.width; x += 1) {
      const color = pixelAt(image, x, y)
      if (color.a === 0) continue
      const px = x + offsetX
      const py = y + offsetY
      if (px < 0 || py < 0 || px >= sel.width || py >= sel.height) continue
      sel.mask[py * sel.width + px] = 1
    }
  }
  sel.bounds = computeBounds(sel.mask, sel.width, sel.height)
  return sel
}

/** Erases every pixel inside the mask. Returns how many pixels were cleared. */
export function clearSelection(image: PixelImage, selection: Selection): number {
  let cleared = 0
  for (let y = 0; y < image.height; y += 1) {
    for (let x = 0; x < image.width; x += 1) {
      if (!selection.mask[y * selection.width + x]) continue
      const at = (y * image.width + x) * 4
      if (image.data[at + 3] === 0) continue
      setPixel(image, x, y, TRANSPARENT)
      cleared += 1
    }
  }
  return cleared
}

/** Crops the selected pixels into a new buffer; null for an empty selection. */
export function extractSelection(
  image: PixelImage,
  selection: Selection | null,
): { image: PixelImage; x: number; y: number } | null {
  const bounds = selection?.bounds
  if (!selection || !bounds) return null
  const out = createImage(bounds.w, bounds.h)
  for (let y = 0; y < bounds.h; y += 1) {
    for (let x = 0; x < bounds.w; x += 1) {
      const source = (bounds.y + y) * selection.width + (bounds.x + x)
      if (!selection.mask[source]) continue
      setPixel(out, x, y, pixelAt(image, bounds.x + x, bounds.y + y))
    }
  }
  return { image: out, x: bounds.x, y: bounds.y }
}

/** Source-over paste of a cropped buffer at (x, y). */
export function pasteImage(
  image: PixelImage,
  source: PixelImage,
  x: number,
  y: number,
): void {
  for (let py = 0; py < source.height; py += 1) {
    for (let px = 0; px < source.width; px += 1) {
      const color = pixelAt(source, px, py)
      if (color.a === 0) continue
      const tx = x + px
      const ty = y + py
      if (!inBounds(image, tx, ty)) continue
      const at = (ty * image.width + tx) * 4
      if (color.a === 255) {
        image.data[at] = color.r
        image.data[at + 1] = color.g
        image.data[at + 2] = color.b
        image.data[at + 3] = 255
        continue
      }
      const srcA = color.a / 255
      const dstA = image.data[at + 3] / 255
      const outA = srcA + dstA * (1 - srcA)
      if (outA <= 0) continue
      image.data[at] = Math.round((color.r * srcA + image.data[at] * dstA * (1 - srcA)) / outA)
      image.data[at + 1] = Math.round((color.g * srcA + image.data[at + 1] * dstA * (1 - srcA)) / outA)
      image.data[at + 2] = Math.round((color.b * srcA + image.data[at + 2] * dstA * (1 - srcA)) / outA)
      image.data[at + 3] = Math.round(outA * 255)
    }
  }
}

/**
 * Palette recolor: swaps every pixel equal to `from` (within tolerance) for
 * `to`. When a selection is given only selected pixels change.
 */
export function replaceColor(
  image: PixelImage,
  from: RGBA,
  to: RGBA,
  tolerance = 0,
  selection?: Selection | null,
): number {
  let changed = 0
  const tol = Math.max(0, Math.min(255, tolerance))
  for (let y = 0; y < image.height; y += 1) {
    for (let x = 0; x < image.width; x += 1) {
      if (selection && !selection.mask[y * selection.width + x]) continue
      const color = pixelAt(image, x, y)
      if (!colorMatches(color, from, tol)) continue
      if (color.r === to.r && color.g === to.g && color.b === to.b && color.a === to.a) continue
      setPixel(image, x, y, to)
      changed += 1
    }
  }
  return changed
}