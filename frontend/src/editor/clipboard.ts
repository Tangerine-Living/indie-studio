/**
 * System clipboard bridge for the pixel editor. PNG data is exchanged as a
 * Blob (never base64) and only lives for the duration of the copy/paste.
 * Every entry point is best-effort: WebView2, a plain browser and the test
 * environment all report unavailable clipboard features instead of throwing.
 */
import { type PixelImage } from './pixels'

function canvasFromImage(image: PixelImage): HTMLCanvasElement | null {
  try {
    const canvas = document.createElement('canvas')
    canvas.width = image.width
    canvas.height = image.height
    const ctx = canvas.getContext('2d')
    if (!ctx) return null
    const frame = ctx.createImageData(image.width, image.height)
    frame.data.set(image.data)
    ctx.putImageData(frame, 0, 0)
    return canvas
  } catch {
    return null
  }
}

/** Encodes a pixel buffer as a PNG blob for the system clipboard. */
export function imageToBlob(image: PixelImage): Promise<Blob | null> {
  const canvas = canvasFromImage(image)
  if (!canvas) return Promise.resolve(null)
  return new Promise((resolve) => {
    try {
      canvas.toBlob((blob) => resolve(blob), 'image/png')
    } catch {
      resolve(null)
    }
  })
}

/** Decodes a PNG (or any browser-supported image) blob into straight RGBA. */
export async function blobToImage(blob: Blob): Promise<PixelImage | null> {
  try {
    const bitmap = await createImageBitmap(blob)
    const canvas = document.createElement('canvas')
    canvas.width = bitmap.width
    canvas.height = bitmap.height
    const ctx = canvas.getContext('2d')
    if (!ctx) return null
    ctx.imageSmoothingEnabled = false
    ctx.drawImage(bitmap, 0, 0)
    const data = ctx.getImageData(0, 0, bitmap.width, bitmap.height)
    bitmap.close?.()
    return { width: bitmap.width, height: bitmap.height, data: data.data }
  } catch {
    return null
  }
}

export function canWriteClipboardImage(): boolean {
  return (
    typeof navigator !== 'undefined' &&
    typeof navigator.clipboard !== 'undefined' &&
    typeof navigator.clipboard.write === 'function' &&
    typeof ClipboardItem !== 'undefined'
  )
}

export function canReadClipboardImage(): boolean {
  return (
    typeof navigator !== 'undefined' &&
    typeof navigator.clipboard !== 'undefined' &&
    typeof navigator.clipboard.read === 'function'
  )
}

/** Best-effort PNG copy. Returns false when the platform disallows it. */
export async function writeImageToClipboard(image: PixelImage): Promise<boolean> {
  if (!canWriteClipboardImage()) return false
  const blob = await imageToBlob(image)
  if (!blob) return false
  try {
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
    return true
  } catch {
    return false
  }
}

/** Best-effort PNG read; may trigger the browser's paste permission prompt. */
export async function readImageFromClipboard(): Promise<PixelImage | null> {
  if (!canReadClipboardImage()) return null
  try {
    const items = await navigator.clipboard.read()
    for (const item of items) {
      if (!item.types.includes('image/png')) continue
      return await blobToImage(await item.getType('image/png'))
    }
  } catch {
    return null
  }
  return null
}

/** Image carried by a native paste event (no permission prompt needed). */
export function imageFromPasteEvent(event: ClipboardEvent): Promise<PixelImage | null> {
  const data = event.clipboardData
  if (!data) return Promise.resolve(null)
  for (const item of Array.from(data.items ?? [])) {
    if (!item.type.startsWith('image/')) continue
    const file = item.getAsFile()
    if (file) return blobToImage(file)
  }
  return Promise.resolve(null)
}