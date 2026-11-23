import { describe, expect, it } from 'vitest'
import { createImage } from './pixels'
import {
  canReadClipboardImage,
  canWriteClipboardImage,
  imageFromPasteEvent,
  imageToBlob,
  readImageFromClipboard,
  writeImageToClipboard,
} from './clipboard'

describe('system clipboard bridge', () => {
  it('degrades to no-ops when canvas or clipboard APIs are unavailable', async () => {
    const image = createImage(2, 2)
    expect(canReadClipboardImage()).toBe(false)
    expect(canWriteClipboardImage()).toBe(false)
    expect(await imageToBlob(image)).toBeNull()
    expect(await writeImageToClipboard(image)).toBe(false)
    expect(await readImageFromClipboard()).toBeNull()
    expect(await imageFromPasteEvent(new Event('paste') as ClipboardEvent)).toBeNull()
  })
})