import { reactive } from 'vue'

export interface LightboxOptions {
  /** Frame URLs for animated assets; the first frame is shown on open. */
  frames?: string[]
  fps?: number
  /** Per-frame hold time in milliseconds, when the asset records it. */
  frameDurations?: number[]
  /** False stops on the last frame instead of starting over. */
  loop?: boolean
}

export const lightboxState = reactive({
  open: false,
  src: '',
  label: '',
  frames: [] as string[],
  fps: 8,
  frameDurations: [] as number[],
  loop: true,
})

export function openLightbox(src: string, label = '', options: LightboxOptions = {}) {
  const frames = options.frames?.filter(Boolean) ?? []
  lightboxState.src = frames[0] ?? src
  lightboxState.label = label
  lightboxState.frames = frames
  lightboxState.fps = Math.min(60, Math.max(1, Math.round(options.fps || 8)))
  lightboxState.frameDurations = options.frameDurations?.slice() ?? []
  lightboxState.loop = options.loop ?? true
  lightboxState.open = true
}

export function closeLightbox() {
  lightboxState.open = false
}