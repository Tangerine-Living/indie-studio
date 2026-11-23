import type { Platform } from './types'
import { createWailsPlatform, hasWailsBridge } from './wails'
import { createMockPlatform } from './mock'

/**
 * The app picks the desktop bridge when present, and otherwise falls back to
 * the browser mock so `vite dev` and Playwright exercise the same UI.
 */
export const platform: Platform = hasWailsBridge() ? createWailsPlatform() : createMockPlatform()

export type { Platform } from './types'
export { createMockPlatform, resetMockStorage } from './mock'
export { createWailsPlatform, hasWailsBridge } from './wails'