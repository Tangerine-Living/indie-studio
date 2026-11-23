/**
 * Pixel-perfect freehand strokes. A plain Bresenham line painted with a 1px
 * brush produces doubled elbows on shallow diagonals; this accumulator drops
 * the elbow pixel when three consecutive painted pixels form an L. Because a
 * stroke may revisit a pixel, the original color is tracked per pixel and
 * restored only when the last reference disappears.
 */
import { inBounds, linePoints, pixelAt, setPixel, type PixelImage, type RGBA } from './pixels'

export interface StrokePoint {
  x: number
  y: number
}

function orthoAdjacent(a: StrokePoint, b: StrokePoint): boolean {
  return Math.abs(a.x - b.x) + Math.abs(a.y - b.y) === 1
}

function diagonalAdjacent(a: StrokePoint, b: StrokePoint): boolean {
  return Math.abs(a.x - b.x) === 1 && Math.abs(a.y - b.y) === 1
}

export class PixelPerfectStroke {
  private readonly image: PixelImage
  private readonly color: RGBA
  private readonly mask: Uint8Array | null
  private points: StrokePoint[] = []
  private counts = new Map<number, number>()
  private originals = new Map<number, RGBA>()
  private last: StrokePoint | null = null

  constructor(image: PixelImage, color: RGBA, mask?: Uint8Array | null) {
    this.image = image
    this.color = color
    this.mask = mask ?? null
  }

  /** Paints the first pixel and anchors the stroke. */
  start(x: number, y: number): void {
    this.last = { x, y }
    this.addPoint(x, y)
  }

  /** Extends the stroke to (x, y) through every intermediate pixel. */
  moveTo(x: number, y: number): void {
    if (!this.last) {
      this.start(x, y)
      return
    }
    const points = linePoints(this.last.x, this.last.y, x, y)
    for (let index = 1; index < points.length; index += 1) {
      this.addPoint(points[index].x, points[index].y)
    }
    this.last = { x, y }
  }

  private addPoint(x: number, y: number): void {
    if (!inBounds(this.image, x, y)) return
    if (this.mask && !this.mask[y * this.image.width + x]) return
    this.paint(x, y)
    this.points.push({ x, y })
    this.fixCorners()
  }

  private paint(x: number, y: number): void {
    const index = y * this.image.width + x
    if (!this.counts.has(index)) {
      this.originals.set(index, pixelAt(this.image, x, y))
      this.counts.set(index, 0)
    }
    this.counts.set(index, (this.counts.get(index) ?? 0) + 1)
    setPixel(this.image, x, y, this.color)
  }

  private unpaint(x: number, y: number): void {
    const index = y * this.image.width + x
    const count = this.counts.get(index) ?? 0
    if (count > 1) {
      this.counts.set(index, count - 1)
      return
    }
    this.counts.delete(index)
    const original = this.originals.get(index)
    if (original) setPixel(this.image, x, y, original)
    else setPixel(this.image, x, y, { r: 0, g: 0, b: 0, a: 0 })
  }

  /** Drops the middle pixel of an L-shaped triple, repeatedly if needed. */
  private fixCorners(): void {
    while (this.points.length >= 3) {
      const n = this.points.length
      const a = this.points[n - 3]
      const b = this.points[n - 2]
      const c = this.points[n - 1]
      if (!orthoAdjacent(a, b) || !orthoAdjacent(b, c) || !diagonalAdjacent(a, c)) break
      this.points.splice(n - 2, 1)
      this.unpaint(b.x, b.y)
    }
  }
}

/** Pure path filter used by tests and future previews. */
export function pixelPerfectPath(points: ReadonlyArray<StrokePoint>): StrokePoint[] {
  const out: StrokePoint[] = []
  for (const point of points) {
    out.push(point)
    while (out.length >= 3) {
      const n = out.length
      const a = out[n - 3]
      const b = out[n - 2]
      const c = out[n - 1]
      if (!orthoAdjacent(a, b) || !orthoAdjacent(b, c) || !diagonalAdjacent(a, c)) break
      out.splice(n - 2, 1)
    }
  }
  return out
}