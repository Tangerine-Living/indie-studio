import { beforeEach, describe, expect, it } from 'vitest'
import { closeLightbox, lightboxState, openLightbox } from './lightbox'

describe('lightbox state', () => {
  beforeEach(() => closeLightbox())

  it('shows the first frame when an animated asset opens', () => {
    openLightbox('fallback.png', '角色', {
      frames: ['frames/000.png', 'frames/001.png'],
      fps: 12,
      frameDurations: [80, 120],
    })
    expect(lightboxState.open).toBe(true)
    expect(lightboxState.src).toBe('frames/000.png')
    expect(lightboxState.frames).toHaveLength(2)
    expect(lightboxState.fps).toBe(12)
    expect(lightboxState.frameDurations).toEqual([80, 120])
    expect(lightboxState.label).toBe('角色')
  })

  it('clears animation state for a single image', () => {
    openLightbox('a.png', 'A', { frames: ['a.png', 'b.png'], fps: 30 })
    openLightbox('c.png', 'C')
    expect(lightboxState.src).toBe('c.png')
    expect(lightboxState.frames).toEqual([])
    expect(lightboxState.fps).toBe(8)
    expect(lightboxState.frameDurations).toEqual([])
    expect(lightboxState.loop).toBe(true)
  })

  it('keeps a non-looping animation from wrapping', () => {
    openLightbox('a.png', 'A', { frames: ['a.png', 'b.png'], loop: false })
    expect(lightboxState.loop).toBe(false)
  })

  it('closes without losing where the image came from', () => {
    openLightbox('a.png', 'A')
    closeLightbox()
    expect(lightboxState.open).toBe(false)
    expect(lightboxState.src).toBe('a.png')
  })
})