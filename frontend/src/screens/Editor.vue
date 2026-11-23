<script setup lang="ts">
import {
  ArrowLeft,
  Blend,
  Circle,
  Eraser,
  Eye,
  EyeOff,
  Grid3x3,
  Lasso,
  Layers,
  Move,
  PaintBucket,
  Pause,
  Pencil,
  Pipette,
  Play,
  Plus,
  Redo2,
  Save,
  Scissors,
  Slash,
  Square,
  SquareDashed,
  Trash2,
  Undo2,
  Wand2,
  ZoomIn,
  ZoomOut,
} from 'lucide-vue-next'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, shallowRef, watch } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { platform } from '../platform'
import { useAppStore } from '../stores/app'
import { confirmDialog } from '../lib/dialogs'
import { clamp } from '../lib/utils'
import type { Asset } from '../domain/types'
import AppModal from '../components/AppModal.vue'
import {
  activeLayer,
  activeLayerImage,
  addFrame,
  applySlices,
  blit,
  compositeDoc,
  createDoc,
  createLayer,
  layerById,
  removeFrame,
  restoreDoc,
  snapshotDoc,
  uniformFrameDuration,
  type DocSnapshot,
  type EditorDoc,
} from '../editor/document'
import {
  cloneImage,
  colorMatches,
  createImage,
  drawEllipse,
  drawLine,
  drawRect,
  floodFill,
  hexToRGBA,
  pixelAt,
  rgbaToHex,
  type Brush,
  type PixelImage,
  type RGBA,
} from '../editor/pixels'
import {
  clearSelection,
  extractSelection,
  invertSelection,
  pasteImage,
  replaceColor,
  selectAll,
  selectByColor,
  selectionFromAlpha,
  selectionFromPolygon,
  selectionFromRect,
  translateSelection,
  unionSelection,
  type Selection,
} from '../editor/selection'
import { PixelPerfectStroke } from '../editor/stroke'
import { imageFromPasteEvent, writeImageToClipboard } from '../editor/clipboard'
import {
  composeSheet,
  frameOrigin,
  sheetInfoFor,
  sheetSize,
  sliceLayout,
  sliceSheet,
} from '../editor/sheet'

const store = useAppStore()

const TOOLS = [
  { id: 'pencil', label: '铅笔', key: 'B', icon: Pencil },
  { id: 'eraser', label: '橡皮', key: 'E', icon: Eraser },
  { id: 'bucket', label: '油漆桶', key: 'G', icon: PaintBucket },
  { id: 'eyedropper', label: '吸管', key: 'I', icon: Pipette },
  { id: 'line', label: '直线', key: 'L', icon: Slash },
  { id: 'rect', label: '矩形', key: 'R', icon: Square },
  { id: 'ellipse', label: '椭圆', key: 'O', icon: Circle },
  { id: 'select', label: '矩形选区', key: 'M', icon: SquareDashed },
  { id: 'lasso', label: '套索', key: 'A', icon: Lasso },
  { id: 'wand', label: '魔棒', key: 'W', icon: Wand2 },
  { id: 'move', label: '移动', key: 'V', icon: Move },
] as const

type ToolId = (typeof TOOLS)[number]['id']
type MirrorMode = 'none' | 'x' | 'y' | 'xy'
const MIRROR_MODES: Array<{ id: MirrorMode; label: string }> = [
  { id: 'none', label: '关' },
  { id: 'x', label: '左右' },
  { id: 'y', label: '上下' },
  { id: 'xy', label: '双轴' },
]

const ZOOMS = [0.1, 0.25, 0.5, 1, 2, 3, 4, 6, 8, 12, 16, 24, 32]
const MAX_HISTORY = 60

const asset = computed(() => store.assets.find((item) => item.id === store.editorAssetId) ?? null)
const paletteColors = computed(() => {
  const paletteId = asset.value?.paletteId ?? store.session?.project.settings.defaultPaletteId
  return store.palettes.find((palette) => palette.id === paletteId)?.colors ?? []
})

const doc = shallowRef<EditorDoc | null>(null)
const loading = ref(true)
const loadError = ref<string | null>(null)
const loadWarning = ref('')
const saving = ref(false)
const dirty = ref(false)
const savedVersion = ref<string | null>(null)

const tool = ref<ToolId>('pencil')
/** The canvas cursor follows the tool so the mode is visible before clicking. */
const cursorForTool = computed(() => {
  switch (tool.value) {
    case 'move':
      return 'move'
    case 'bucket':
    case 'eyedropper':
      return 'copy'
    case 'eraser':
      return 'cell'
    default:
      return 'crosshair'
  }
})
const color = ref('#f2e6c9')
const brushSize = ref(1)
const shapeFilled = ref(false)
const pixelPerfect = ref(false)
const symmetry = ref<MirrorMode>('none')
const fillTolerance = ref(0)
const fillContiguous = ref(true)
const recolorFrom = ref('#000000')
const selection = shallowRef<Selection | null>(null)
const internalClipboard = shallowRef<{ image: PixelImage; x: number; y: number } | null>(null)
const zoom = ref(4)
const showGrid = computed(() => store.editorSettings?.showGrid ?? true)
const openMode = computed(() => store.editorSettings?.openMode ?? 'fit')
const pan = reactive({ x: 0, y: 0 })
const hover = ref<{ x: number; y: number } | null>(null)
const layerRevision = ref(0)
const frameRevision = ref(0)
const onion = ref(true)
const playing = ref(false)

/** Pixels of the original (un-sliced) source image, kept for sheet slicing. */
const sourceSheet = shallowRef<PixelImage | null>(null)
const sliceOpen = ref(false)
const sliceForm = reactive({ frameWidth: 32, frameHeight: 32, margin: 0, spacing: 0 })
const slicePreview = ref<HTMLCanvasElement | null>(null)
const exportOpen = ref(false)
const exportColumns = ref(1)

const undoStack = ref<DocSnapshot[]>([])
const redoStack = ref<DocSnapshot[]>([])

const viewport = ref<HTMLDivElement | null>(null)
const surface = ref<HTMLCanvasElement | null>(null)

let observer: ResizeObserver | null = null
/** fitZoom needs a measured viewport; the first layout may arrive later. */
let fittedOnce = false
const compositeCache = new Map<number, PixelImage>()
let thumbStale = true
let playTimer: ReturnType<typeof setTimeout> | null = null
const frameCanvases: Array<HTMLCanvasElement | null> = []
let stroke: { start: { x: number; y: number }; last: { x: number; y: number }; base: PixelImage | null } | null =
  null
let panning: { x: number; y: number } | null = null
const isPanning = ref(false)
/** Holding Space turns any drag into a pan, like the map editor. */
const spaceDown = ref(false)
/** Right-button drag erases with the current brush size. */
let rightErase = false
const canvasCursor = computed(() => {
  if (isPanning.value) return 'grabbing'
  if (spaceDown.value) return 'grab'
  return cursorForTool.value
})
let pixelStrokes: PixelPerfectStroke[] = []
let selectDrag: {
  start: { x: number; y: number }
  current: { x: number; y: number }
  add: boolean
} | null = null
let lassoDrag: { points: Array<{ x: number; y: number }>; add: boolean } | null = null
let moveDrag: {
  base: PixelImage
  content: PixelImage
  origin: { x: number; y: number }
  mask: Selection | null
  start: { x: number; y: number }
} | null = null

const layerList = computed(() => {
  layerRevision.value
  const current = doc.value
  return current ? [...current.layers].reverse() : []
})
const activeLayerId = computed(() => {
  layerRevision.value
  return doc.value?.activeLayerId ?? ''
})
const layerCount = computed(() => {
  layerRevision.value
  return doc.value?.layers.length ?? 0
})
const frameCount = computed(() => {
  frameRevision.value
  return doc.value?.frameCount ?? 1
})
const frameList = computed(() => {
  frameRevision.value
  return Array.from({ length: frameCount.value }, (_, index) => index)
})
const activeFrame = computed(() => {
  frameRevision.value
  return doc.value?.activeFrame ?? 0
})
const totalDuration = computed(() => {
  frameRevision.value
  return (doc.value?.frameDurations ?? []).reduce((sum, ms) => sum + ms, 0)
})
const sliceLayoutInfo = computed(() => {
  const source = sourceSheet.value
  return source ? sliceLayout(source.width, source.height, sliceForm) : null
})
const exportSheetSize = computed(() => {
  frameRevision.value
  const d = doc.value
  if (!d) return null
  const columns = clamp(Math.round(Number(exportColumns.value) || d.frameCount), 1, d.frameCount)
  return { ...sheetSize(d.width, d.height, d.frameCount, columns), columns }
})
const selectionSummary = computed(() => {
  const bounds = selection.value?.bounds
  return bounds ? `${bounds.w} × ${bounds.h} px` : '无选区'
})

function currentColor(): RGBA {
  return hexToRGBA(color.value) ?? { r: 244, g: 244, b: 244, a: 255 }
}

/** Every position a brush lands on under the current symmetry mode. */
function mirroredPoints(x: number, y: number, d: EditorDoc): Array<{ x: number; y: number }> {
  const points = [{ x, y }]
  const mode = symmetry.value
  if (mode === 'x' || mode === 'xy') points.push({ x: d.width - 1 - x, y })
  if (mode === 'y' || mode === 'xy') points.push({ x, y: d.height - 1 - y })
  if (mode === 'xy') points.push({ x: d.width - 1 - x, y: d.height - 1 - y })
  return points
}

function setSymmetry(mode: MirrorMode) {
  symmetry.value = mode
}

function setFillTolerance(event: Event) {
  fillTolerance.value = clamp(Math.round(Number((event.target as HTMLInputElement).value) || 0), 0, 255)
}

function setRecolorFrom(event: Event) {
  recolorFrom.value = (event.target as HTMLInputElement).value
}

/** Bumped on every pixel edit so cached canvases know when to re-upload. */
let pixelRev = 0
/** Reusable canvases for drawing PixelImages; keyed per image, reused by revision. */
const pixelCanvases = new WeakMap<PixelImage, { canvas: HTMLCanvasElement; rev: number }>()

/** Invalidates composites for every frame and every visible preview. */
function invalidateComposite() {
  compositeCache.clear()
  pixelRev += 1
  thumbStale = true
}

/** Cached composite for one frame; rebuilt only after a pixel edit. */
function frameComposite(d: EditorDoc, frame: number): PixelImage {
  // A single visible full-opacity layer is already the composite: no copy.
  const visible = d.layers.filter((layer) => layer.visible && layer.opacity > 0)
  if (visible.length === 1 && visible[0].opacity >= 0.999) {
    return visible[0].frames[frame] ?? visible[0].frames[0]
  }
  let composite = compositeCache.get(frame)
  if (!composite) {
    composite = compositeDoc(d, frame)
    compositeCache.set(frame, composite)
  }
  return composite
}

function currentComposite(d: EditorDoc): PixelImage {
  return frameComposite(d, d.activeFrame)
}

function setTool(id: ToolId) {
  tool.value = id
}

/* ----------------------------------------------------------- lifecycle */

async function init() {
  const current = asset.value
  fittedOnce = false
  loading.value = true
  loadError.value = ''
  loadWarning.value = ''
  undoStack.value = []
  redoStack.value = []
  selection.value = null
  if (playing.value) stopPlayback()
  if (!store.editorAssetId || !current) {
    loadError.value = '素材不存在，可能已被删除'
    loading.value = false
    return
  }
  const next = createDoc(
    current.dimensions.w || 32,
    current.dimensions.h || 32,
    current.animation?.fps ?? 8,
  )
  doc.value = next
  // Opening a version is not an edit; only drawing should read as unsaved.
  savedVersion.value = current.currentVersion ?? null
  dirty.value = false

  // A saved sprite sheet reopens with all its frames, timing and tags.
  const animation = current.animation
  const frameTotal = animation?.frameCount ?? 0
  if (animation && frameTotal > 1) {
    next.fps = Math.max(1, Math.min(60, animation.fps || next.fps))
    next.tags = [...(animation.tags ?? [])]
    ensureFrameSlots(next, frameTotal)
    next.frameDurations =
      animation.frameDurations?.length === frameTotal
        ? [...animation.frameDurations]
        : Array.from({ length: frameTotal }, () => uniformFrameDuration(next.fps))
  }
  if (paletteColors.value.length > 0) {
    color.value = paletteColors.value[paletteColors.value.length - 1]
  }
  const box = viewport.value
  if (box && typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(() => {
      if (!fittedOnce) applyInitialView()
      render()
    })
    observer.observe(box)
  }
  // A hand-drawn asset has no source image; it starts on an empty canvas.
  let loaded = false
  if (current.source.kind !== 'blank') {
    loaded = await loadSource(current.id, next)
  }
  if (next.frameCount > 1) {
    const framesLoaded = await loadAnimationFrames(current.id, next)
    if (framesLoaded > 0) loaded = true
  }
  if (!loaded) {
    loaded = await loadVersionFallback(current, next)
  }
  if (!loaded && (current.source.kind !== 'blank' || current.currentVersion)) {
    loadWarning.value = '无法读取素材图片，画布是空的；请检查素材文件是否还在，或在素材详情重新导出一份。'
  }
  applyInitialView()
  render()
  // The layout can settle one frame later (route transition); repaint then so
  // the first view already shows the art without requiring a click.
  requestAnimationFrame(() => {
    if (!fittedOnce) applyInitialView()
    render()
  })
  loading.value = false
}

function loadImage(url: string): Promise<HTMLImageElement | null> {
  return new Promise((resolve) => {
    try {
      const image = new Image()
      image.onload = () => resolve(image)
      image.onerror = () => resolve(null)
      image.src = url
    } catch {
      resolve(null)
    }
  })
}

/** Rasterizes the asset source into a doc-sized buffer (nearest neighbour). */
function rasterize(image: HTMLImageElement, width: number, height: number): PixelImage | null {
  try {
    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const ctx = canvas.getContext('2d')
    if (!ctx) return null
    ctx.imageSmoothingEnabled = false
    ctx.drawImage(image, 0, 0, width, height)
    const data = ctx.getImageData(0, 0, width, height)
    return { width, height, data: data.data }
  } catch {
    return null
  }
}

/** Grows the document to the stored frame count without dropping edits. */
function ensureFrameSlots(d: EditorDoc, count: number) {
  d.frameCount = Math.max(1, Math.min(256, count))
  const fallback = uniformFrameDuration(d.fps)
  d.frameDurations = Array.from(
    { length: d.frameCount },
    (_, index) => d.frameDurations[index] ?? fallback,
  )
  for (const layer of d.layers) {
    while (layer.frames.length < d.frameCount) layer.frames.push(createImage(d.width, d.height))
    if (layer.frames.length > d.frameCount) layer.frames.length = d.frameCount
  }
}

/** Loads frames/NNN.png into every frame of the first layer. */
async function loadAnimationFrames(assetId: string, d: EditorDoc): Promise<number> {
  const base = d.layers[0]
  if (!base) return 0
  const images = await Promise.all(
    Array.from({ length: d.frameCount }, async (_, index) => {
      try {
        const url = await platform.assetURL(
          assetId,
          `frames/${String(index).padStart(3, '0')}.png`,
        )
        const image = await loadImage(url)
        return image ? rasterize(image, d.width, d.height) : null
      } catch {
        return null
      }
    }),
  )
  let loaded = 0
  images.forEach((pixels, index) => {
    if (!pixels || !base.frames[index]) return
    blit(base.frames[index], pixels)
    loaded += 1
  })
  // Frames land after the first layout pass; without this the cached
  // composite stays empty and the editor looks blank until the first stroke.
  invalidateComposite()
  layerRevision.value += 1
  return loaded
}

async function loadSource(assetId: string, next: EditorDoc): Promise<boolean> {
  try {
    const source = await platform.assetSource(assetId)
    const image = await loadImage(source.url)
    if (!image || next.layers.length === 0) return false
    const pixels = rasterize(image, next.width, next.height)
    if (!pixels) return false
    sourceSheet.value = cloneImage(pixels)
    blit(next.layers[0].frames[0], pixels)
    invalidateComposite()
    layerRevision.value += 1
    return true
  } catch {
    // A blank canvas is a valid starting point when the source is unreadable.
    return false
  }
}

/** Last resort: the current version image, then the thumbnail. */
async function loadVersionFallback(current: Asset, d: EditorDoc): Promise<boolean> {
  if (d.layers.length === 0) return false
  const candidates: string[] = []
  if (current.currentVersion) candidates.push(`versions/${current.currentVersion}/output.png`)
  candidates.push('thumb.png')
  for (const file of candidates) {
    try {
      const url = await platform.assetURL(current.id, file)
      const image = await loadImage(url)
      if (!image) continue
      const pixels = rasterize(image, d.width, d.height)
      if (!pixels) continue
      sourceSheet.value = cloneImage(pixels)
      blit(d.layers[0].frames[0], pixels)
      invalidateComposite()
      layerRevision.value += 1
      return true
    } catch {
      // try the next candidate
    }
  }
  return false
}

/* -------------------------------------------------------------- drawing */

function ensureCanvasSize() {
  const canvas = surface.value
  const box = viewport.value
  if (!canvas || !box) return null
  const cw = box.clientWidth
  const ch = box.clientHeight
  if (cw <= 0 || ch <= 0) return null
  const ratio = Math.min(2, window.devicePixelRatio || 1)
  const width = Math.round(cw * ratio)
  const height = Math.round(ch * ratio)
  if (canvas.width !== width || canvas.height !== height) {
    canvas.width = width
    canvas.height = height
  }
  return { canvas, cw, ch }
}

function originFor(cw: number, ch: number, d: EditorDoc, z: number) {
  return {
    x: Math.round((cw - d.width * z) / 2 + pan.x),
    y: Math.round((ch - d.height * z) / 2 + pan.y),
  }
}

let renderHandle = 0

/** Coalesces high-frequency pointer events into one paint per frame. */
function scheduleRender() {
  if (renderHandle) return
  renderHandle = requestAnimationFrame(() => {
    renderHandle = 0
    render()
  })
}

function render() {
  const sized = ensureCanvasSize()
  const d = doc.value
  if (!sized || !d) return
  const { canvas, cw, ch } = sized
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const ratio = canvas.width / cw
  ctx.setTransform(ratio, 0, 0, ratio, 0, 0)
  ctx.clearRect(0, 0, cw, ch)

  const z = zoom.value
  const origin = originFor(cw, ch, d, z)
  const dw = d.width * z
  const dh = d.height * z

  // Only the on-screen part of the document is painted; a 1024px canvas at a
  // high zoom would otherwise scale and outline the whole image every frame.
  const viewLeft = Math.max(origin.x, 0)
  const viewTop = Math.max(origin.y, 0)
  const viewRight = Math.min(origin.x + dw, cw)
  const viewBottom = Math.min(origin.y + dh, ch)
  const viewW = viewRight - viewLeft
  const viewH = viewBottom - viewTop
  const onScreen = viewW > 0 && viewH > 0
  const region = {
    x: Math.max(0, Math.floor((viewLeft - origin.x) / z)),
    y: Math.max(0, Math.floor((viewTop - origin.y) / z)),
    w: 0,
    h: 0,
  }
  region.w = Math.min(d.width - region.x, Math.ceil(viewW / z) + 1)
  region.h = Math.min(d.height - region.y, Math.ceil(viewH / z) + 1)

  ctx.save()
  ctx.beginPath()
  ctx.rect(origin.x, origin.y, dw, dh)
  ctx.clip()
  if (onScreen) {
    drawCanvasBacking(ctx, viewLeft, viewTop, viewW, viewH)

    if (onion.value && !playing.value) {
      const previous = d.activeFrame - 1
      const next = d.activeFrame + 1
      if (previous >= 0) {
        paintComposite(
          ctx,
          frameComposite(d, previous),
          origin,
          z,
          0.3,
          'rgba(201,79,79,0.6)',
          region,
        )
      }
      if (next < d.frameCount) {
        paintComposite(
          ctx,
          frameComposite(d, next),
          origin,
          z,
          0.3,
          'rgba(110,198,232,0.6)',
          region,
        )
      }
    }
    paintComposite(ctx, currentComposite(d), origin, z, 1, null, region)

    if (showGrid.value && z >= 6) drawGrid(ctx, origin.x, origin.y, dw, dh, z, cw, ch)
    renderSelection(ctx, origin, z)
  }
  ctx.restore()

  ctx.strokeStyle = 'rgba(255,255,255,0.18)'
  ctx.lineWidth = 1
  ctx.strokeRect(origin.x - 0.5, origin.y - 0.5, dw + 1, dh + 1)

  const point = hover.value
  if (point && point.x >= 0 && point.y >= 0 && point.x < d.width && point.y < d.height) {
    ctx.strokeStyle = 'rgba(110,198,232,0.9)'
    ctx.lineWidth = 1
    ctx.strokeRect(origin.x + point.x * z - 0.5, origin.y + point.y * z - 0.5, z + 1, z + 1)
  }

  if (thumbStale) {
    thumbStale = false
    void nextTick(renderThumbs)
  }
}

function paintThumb(canvas: HTMLCanvasElement, index: number) {
  const d = doc.value
  if (!d) return
  if (canvas.width !== d.width || canvas.height !== d.height) {
    canvas.width = d.width
    canvas.height = d.height
  }
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const composite = frameComposite(d, index)
  const frame = ctx.createImageData(composite.width, composite.height)
  frame.data.set(composite.data)
  ctx.putImageData(frame, 0, 0)
}

function renderThumbs() {
  const count = frameCount.value
  for (let index = 0; index < count; index += 1) {
    const canvas = frameCanvases[index]
    if (canvas) paintThumb(canvas, index)
  }
}

const frameRefs = new Map<number, (el: Element | ComponentPublicInstance | null) => void>()

/** Stable per-index ref callback so playback re-renders don't repaint every thumb. */
function frameCanvasRef(index: number) {
  let callback = frameRefs.get(index)
  if (!callback) {
    callback = (el: Element | ComponentPublicInstance | null) => {
      const canvas = el instanceof HTMLCanvasElement ? el : null
      frameCanvases[index] = canvas
      if (canvas) paintThumb(canvas, index)
    }
    frameRefs.set(index, callback)
  }
  return callback
}

/** PixelImage → canvas, uploaded again only when a pixel edit bumps pixelRev. */
function cachedImageCanvas(image: PixelImage): HTMLCanvasElement {
  let entry = pixelCanvases.get(image)
  if (!entry) {
    entry = { canvas: document.createElement('canvas'), rev: -1 }
    pixelCanvases.set(image, entry)
  }
  const canvas = entry.canvas
  if (canvas.width !== image.width || canvas.height !== image.height) {
    canvas.width = image.width
    canvas.height = image.height
    entry.rev = -1
  }
  if (entry.rev !== pixelRev) {
    const ctx = canvas.getContext('2d')
    if (ctx) {
      const frame = ctx.createImageData(image.width, image.height)
      frame.data.set(image.data)
      ctx.putImageData(frame, 0, 0)
      entry.rev = pixelRev
    }
  }
  return canvas
}

/**
 * Draws one composite, cropped to the on-screen pixel region. Cropping keeps
 * the GPU work proportional to the viewport instead of the document size.
 */
function paintComposite(
  ctx: CanvasRenderingContext2D,
  composite: PixelImage,
  origin: { x: number; y: number },
  z: number,
  alpha: number,
  tint: string | null,
  region: { x: number; y: number; w: number; h: number },
) {
  if (region.w <= 0 || region.h <= 0) return
  const source = tint ? tintedCanvas(composite, tint) : cachedImageCanvas(composite)
  ctx.save()
  ctx.globalAlpha = alpha
  ctx.imageSmoothingEnabled = false
  ctx.drawImage(
    source,
    region.x,
    region.y,
    region.w,
    region.h,
    origin.x + region.x * z,
    origin.y + region.y * z,
    region.w * z,
    region.h * z,
  )
  ctx.restore()
}

/** Onion-skin tint, one cached canvas per image and tint colour. */
const tintCanvases = new WeakMap<
  PixelImage,
  { canvas: HTMLCanvasElement; rev: number; tint: string }
>()

function tintedCanvas(image: PixelImage, tint: string): HTMLCanvasElement {
  let entry = tintCanvases.get(image)
  if (!entry) {
    entry = { canvas: document.createElement('canvas'), rev: -1, tint: '' }
    tintCanvases.set(image, entry)
  }
  const canvas = entry.canvas
  if (canvas.width !== image.width || canvas.height !== image.height) {
    canvas.width = image.width
    canvas.height = image.height
    entry.rev = -1
  }
  if (entry.rev !== pixelRev || entry.tint !== tint) {
    const ctx = canvas.getContext('2d')
    if (!ctx) return cachedImageCanvas(image)
    const frame = ctx.createImageData(image.width, image.height)
    frame.data.set(image.data)
    ctx.putImageData(frame, 0, 0)
    ctx.globalCompositeOperation = 'source-atop'
    ctx.fillStyle = tint
    ctx.fillRect(0, 0, image.width, image.height)
    ctx.globalCompositeOperation = 'source-over'
    entry.rev = pixelRev
    entry.tint = tint
  }
  return canvas
}

/**
 * The canvas backing for transparent pixels. A checkerboard reads as noise at
 * pixel scale, so the editor uses one calm neutral tone instead; the outline
 * drawn around the document marks where the canvas ends.
 */
function drawCanvasBacking(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number) {
  ctx.fillStyle = '#1b222c'
  ctx.fillRect(x, y, w, h)
}

function drawGrid(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  z: number,
  cw: number,
  ch: number,
) {
  ctx.strokeStyle = 'rgba(255,255,255,0.07)'
  ctx.lineWidth = 1
  ctx.beginPath()
  // Only the visible slice of the document gets grid lines.
  const firstX = Math.max(1, Math.ceil((Math.max(x, 0) - x) / z))
  const lastX = Math.floor((Math.min(x + w, cw) - x) / z)
  for (let col = firstX; col <= lastX; col += 1) {
    const px = x + col * z
    ctx.moveTo(px + 0.5, Math.max(y, 0))
    ctx.lineTo(px + 0.5, Math.min(y + h, ch))
  }
  const firstY = Math.max(1, Math.ceil((Math.max(y, 0) - y) / z))
  const lastY = Math.floor((Math.min(y + h, ch) - y) / z)
  for (let row = firstY; row <= lastY; row += 1) {
    const py = y + row * z
    ctx.moveTo(Math.max(x, 0), py + 0.5)
    ctx.lineTo(Math.min(x + w, cw), py + 0.5)
  }
  ctx.stroke()
}

/** Black/white marching ants for the current selection path. */
function dashedStroke(ctx: CanvasRenderingContext2D) {
  ctx.lineWidth = 1
  ctx.setLineDash([4, 4])
  ctx.lineDashOffset = 0
  ctx.strokeStyle = 'rgba(0,0,0,0.85)'
  ctx.stroke()
  ctx.lineDashOffset = 4
  ctx.strokeStyle = 'rgba(255,255,255,0.95)'
  ctx.stroke()
  ctx.setLineDash([])
}

function renderSelection(
  ctx: CanvasRenderingContext2D,
  origin: { x: number; y: number },
  z: number,
) {
  if (selectDrag) {
    const left = Math.min(selectDrag.start.x, selectDrag.current.x)
    const right = Math.max(selectDrag.start.x, selectDrag.current.x)
    const top = Math.min(selectDrag.start.y, selectDrag.current.y)
    const bottom = Math.max(selectDrag.start.y, selectDrag.current.y)
    ctx.save()
    ctx.beginPath()
    ctx.rect(
      origin.x + left * z + 0.5,
      origin.y + top * z + 0.5,
      (right - left + 1) * z - 1,
      (bottom - top + 1) * z - 1,
    )
    dashedStroke(ctx)
    ctx.restore()
    return
  }
  if (lassoDrag && lassoDrag.points.length > 1) {
    ctx.save()
    ctx.beginPath()
    const first = lassoDrag.points[0]
    ctx.moveTo(origin.x + (first.x + 0.5) * z, origin.y + (first.y + 0.5) * z)
    for (let index = 1; index < lassoDrag.points.length; index += 1) {
      const point = lassoDrag.points[index]
      ctx.lineTo(origin.x + (point.x + 0.5) * z, origin.y + (point.y + 0.5) * z)
    }
    ctx.closePath()
    dashedStroke(ctx)
    ctx.restore()
    return
  }
  const sel = selection.value
  const bounds = sel?.bounds
  if (!sel || !bounds) return
  // Very large masks are summarized by their box; tracing every edge would
  // cost more than the rest of a frame on multi-megapixel documents.
  if (bounds.w * bounds.h > 262_144) {
    ctx.save()
    ctx.beginPath()
    ctx.rect(
      origin.x + bounds.x * z + 0.5,
      origin.y + bounds.y * z + 0.5,
      bounds.w * z - 1,
      bounds.h * z - 1,
    )
    dashedStroke(ctx)
    ctx.restore()
    return
  }
  ctx.save()
  ctx.beginPath()
  for (let y = bounds.y; y < bounds.y + bounds.h; y += 1) {
    for (let x = bounds.x; x < bounds.x + bounds.w; x += 1) {
      const index = y * sel.width + x
      if (!sel.mask[index]) continue
      const left = origin.x + x * z + 0.5
      const top = origin.y + y * z + 0.5
      const right = left + z
      const bottom = top + z
      if (x === 0 || !sel.mask[index - 1]) {
        ctx.moveTo(left, top)
        ctx.lineTo(left, bottom)
      }
      if (x === sel.width - 1 || !sel.mask[index + 1]) {
        ctx.moveTo(right, top)
        ctx.lineTo(right, bottom)
      }
      if (y === 0 || !sel.mask[index - sel.width]) {
        ctx.moveTo(left, top)
        ctx.lineTo(right, top)
      }
      if (y === sel.height - 1 || !sel.mask[index + sel.width]) {
        ctx.moveTo(left, bottom)
        ctx.lineTo(right, bottom)
      }
    }
  }
  dashedStroke(ctx)
  ctx.restore()
}

function pixelFromEvent(event: PointerEvent | WheelEvent): { x: number; y: number } | null {
  const d = doc.value
  const box = viewport.value
  if (!d || !box) return null
  const rect = box.getBoundingClientRect()
  const z = zoom.value
  const origin = originFor(rect.width, rect.height, d, z)
  const x = Math.floor((event.clientX - rect.left - origin.x) / z)
  const y = Math.floor((event.clientY - rect.top - origin.y) / z)
  return { x, y }
}

function pushUndo() {
  const d = doc.value
  if (!d) return
  undoStack.value.push(snapshotDoc(d))
  if (undoStack.value.length > MAX_HISTORY) undoStack.value.shift()
  redoStack.value = []
}

function paintShape(
  image: PixelImage,
  id: ToolId,
  from: { x: number; y: number },
  to: { x: number; y: number },
  brush: Brush,
) {
  if (id === 'line') {
    drawLine(image, from.x, from.y, to.x, to.y, brush)
  } else if (id === 'rect') {
    drawRect(image, from.x, from.y, to.x, to.y, brush, shapeFilled.value)
  } else if (id === 'ellipse') {
    drawEllipse(image, from.x, from.y, to.x, to.y, brush, shapeFilled.value)
  }
}

function applyStroke(from: { x: number; y: number }, to: { x: number; y: number }) {
  const d = doc.value
  if (!d) return
  const image = activeLayerImage(d)
  const toolId: ToolId = rightErase ? 'eraser' : tool.value
  const brush: Brush = {
    size: brushSize.value,
    color: toolId === 'eraser' ? { r: 0, g: 0, b: 0, a: 0 } : currentColor(),
    mask: selection.value?.mask ?? null,
  }
  if (toolId === 'pencil' || toolId === 'eraser') {
    if (pixelStrokes.length > 0) {
      const targets = mirroredPoints(to.x, to.y, d)
      for (let index = 0; index < pixelStrokes.length; index += 1) {
        const target = targets[index] ?? targets[0]
        pixelStrokes[index].moveTo(target.x, target.y)
      }
    } else {
      const starts = mirroredPoints(from.x, from.y, d)
      const ends = mirroredPoints(to.x, to.y, d)
      for (let index = 0; index < starts.length; index += 1) {
        drawLine(image, starts[index].x, starts[index].y, ends[index].x, ends[index].y, brush)
      }
    }
    invalidateComposite()
    return
  }
  if (!stroke?.base) return
  image.data.set(stroke.base.data)
  invalidateComposite()
  const starts = mirroredPoints(stroke.start.x, stroke.start.y, d)
  const ends = mirroredPoints(to.x, to.y, d)
  for (let index = 0; index < starts.length; index += 1) {
    paintShape(image, toolId, starts[index], ends[index], brush)
  }
}

/** Creates one accented stroke accumulator per symmetry position. */
function startPixelStrokes(point: { x: number; y: number }, erasing = false) {
  pixelStrokes = []
  const toolId: ToolId = erasing ? 'eraser' : tool.value
  if (!(toolId === 'pencil' || toolId === 'eraser')) return
  if (!pixelPerfect.value || brushSize.value !== 1) return
  const d = doc.value
  if (!d) return
  const image = activeLayerImage(d)
  const strokeColor = toolId === 'eraser' ? { r: 0, g: 0, b: 0, a: 0 } : currentColor()
  const mask = selection.value?.mask ?? null
  for (const position of mirroredPoints(point.x, point.y, d)) {
    const accumulator = new PixelPerfectStroke(image, strokeColor, mask)
    accumulator.start(position.x, position.y)
    pixelStrokes.push(accumulator)
  }
}

function pickColor(x: number, y: number) {
  const d = doc.value
  if (!d) return
  const picked = pixelAt(currentComposite(d), x, y)
  if (picked.a === 0) return
  color.value = rgbaToHex(picked).slice(0, 7)
}

function onPointerDown(event: PointerEvent) {
  const d = doc.value
  if (!d) return
  const point = pixelFromEvent(event)
  if (!point) return
  if (playing.value) stopPlayback()
  const canvas = event.currentTarget as HTMLCanvasElement | null

  // Space+drag, middle mouse and Alt+left pan. Right-button paints with the
  // eraser instead of panning, matching the map editor's clear gesture.
  if (event.button === 1 || spaceDown.value || (event.button === 0 && event.altKey)) {
    panning = { x: event.clientX, y: event.clientY }
    isPanning.value = true
    canvas?.setPointerCapture(event.pointerId)
    event.preventDefault()
    return
  }
  if (event.button !== 0 && event.button !== 2) return
  rightErase = event.button === 2
  if (!rightErase && tool.value === 'eyedropper') {
    pickColor(point.x, point.y)
    return
  }

  if (!rightErase && (tool.value === 'select' || tool.value === 'lasso')) {
    canvas?.setPointerCapture(event.pointerId)
    if (tool.value === 'select') {
      selectDrag = { start: point, current: point, add: event.shiftKey }
    } else {
      lassoDrag = { points: [point], add: event.shiftKey }
    }
    render()
    return
  }
  if (!rightErase && tool.value === 'wand') {
    const picked = selectByColor(
      activeLayerImage(d),
      point.x,
      point.y,
      fillTolerance.value,
      fillContiguous.value,
    )
    selection.value =
      event.shiftKey && selection.value ? unionSelection(selection.value, picked) : picked
    render()
    return
  }

  const layer = activeLayer(d)
  if (!layer.visible || layer.opacity <= 0) return

  if (!rightErase && tool.value === 'move') {
    if (point.x < 0 || point.y < 0 || point.x >= d.width || point.y >= d.height) return
    const image = activeLayerImage(d)
    canvas?.setPointerCapture(event.pointerId)
    pushUndo()
    if (selection.value?.bounds) {
      const lifted = extractSelection(image, selection.value)
      if (!lifted) return
      moveDrag = {
        base: cloneImage(image),
        content: lifted.image,
        origin: { x: lifted.x, y: lifted.y },
        mask: selection.value,
        start: point,
      }
      clearSelection(image, selection.value)
    } else {
      moveDrag = {
        base: createImage(d.width, d.height),
        content: cloneImage(image),
        origin: { x: 0, y: 0 },
        mask: null,
        start: point,
      }
      image.data.fill(0)
    }
    // Pasting at zero offset immediately keeps a click-without-drag lossless.
    applyMove(point)
    dirty.value = true
    render()
    return
  }

  canvas?.setPointerCapture(event.pointerId)
  pushUndo()
  const base =
    !rightErase && (tool.value === 'line' || tool.value === 'rect' || tool.value === 'ellipse')
      ? cloneImage(activeLayerImage(d))
      : null
  stroke = { start: point, last: point, base }
  if (!rightErase && tool.value === 'bucket') {
    floodFill(activeLayerImage(d), point.x, point.y, currentColor(), fillTolerance.value, {
      mask: selection.value?.mask ?? null,
      contiguous: fillContiguous.value,
    })
    invalidateComposite()
  } else {
    startPixelStrokes(point, rightErase)
    applyStroke(point, point)
  }
  dirty.value = true
  render()
}

function applyMove(point: { x: number; y: number }) {
  const d = doc.value
  const drag = moveDrag
  if (!d || !drag) return
  const image = activeLayerImage(d)
  image.data.set(drag.base.data)
  const dx = point.x - drag.start.x
  const dy = point.y - drag.start.y
  if (drag.mask) {
    clearSelection(image, drag.mask)
    pasteImage(image, drag.content, drag.origin.x + dx, drag.origin.y + dy)
    selection.value = translateSelection(drag.mask, dx, dy)
  } else {
    pasteImage(image, drag.content, drag.origin.x + dx, drag.origin.y + dy)
  }
  invalidateComposite()
  dirty.value = true
}

function onPointerMove(event: PointerEvent) {
  const point = pixelFromEvent(event)
  hover.value = point
  if (panning) {
    pan.x += event.clientX - panning.x
    pan.y += event.clientY - panning.y
    panning = { x: event.clientX, y: event.clientY }
    scheduleRender()
    return
  }
  if (selectDrag) {
    if (point) selectDrag.current = point
    scheduleRender()
    return
  }
  if (lassoDrag) {
    const last = lassoDrag.points[lassoDrag.points.length - 1]
    if (point && (point.x !== last.x || point.y !== last.y)) lassoDrag.points.push(point)
    scheduleRender()
    return
  }
  if (moveDrag) {
    if (point) applyMove(point)
    scheduleRender()
    return
  }
  if (stroke) {
    if (!point) return
    if (point.x === stroke.last.x && point.y === stroke.last.y) {
      scheduleRender()
      return
    }
    applyStroke(stroke.last, point)
    stroke.last = point
    dirty.value = true
  }
  scheduleRender()
}

function endStroke(event: PointerEvent) {
  const canvas = event.currentTarget as HTMLCanvasElement | null
  if (canvas?.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId)
  const d = doc.value
  if (selectDrag) {
    const drag = selectDrag
    selectDrag = null
    if (d) {
      if (drag.start.x === drag.current.x && drag.start.y === drag.current.y) {
        if (!drag.add) selection.value = null
      } else {
        const next = selectionFromRect(d.width, d.height, drag.start.x, drag.start.y, drag.current.x, drag.current.y)
        selection.value = drag.add && selection.value ? unionSelection(selection.value, next) : next
      }
    }
  }
  if (lassoDrag) {
    const drag = lassoDrag
    lassoDrag = null
    if (d && drag.points.length >= 3) {
      const next = selectionFromPolygon(d.width, d.height, drag.points)
      selection.value = drag.add && selection.value ? unionSelection(selection.value, next) : next
    } else if (!drag.add) {
      selection.value = null
    }
  }
  stroke = null
  pixelStrokes = []
  moveDrag = null
  panning = null
  rightErase = false
  isPanning.value = false
  render()
}

function onPointerLeave() {
  hover.value = null
  render()
}

function onWheel(event: WheelEvent) {
  zoom.value = zoom.value !== 0 && event.deltaY < 0 ? nextZoom(1) : nextZoom(-1)
  scheduleRender()
}

/* ----------------------------------------------------------------- view */

function nextZoom(direction: number): number {
  // The fitted zoom can sit between steps, so compare by value, not index.
  if (direction > 0) {
    return ZOOMS.find((level) => level > zoom.value + 1e-6) ?? ZOOMS[ZOOMS.length - 1]
  }
  return [...ZOOMS].reverse().find((level) => level < zoom.value - 1e-6) ?? ZOOMS[0]
}

function zoomIn() {
  zoom.value = nextZoom(1)
  render()
}

function zoomOut() {
  zoom.value = nextZoom(-1)
  render()
}

/** Applies the saved opening view: fit the window or show 100%. */
function applyInitialView() {
  if (openMode.value === 'actual') {
    zoom.value = 1
    pan.x = 0
    pan.y = 0
    fittedOnce = true
    render()
    return
  }
  fitZoom()
}

function fitZoom() {
  const box = viewport.value
  const d = doc.value
  if (!box || !d) return
  const cw = box.clientWidth
  const ch = box.clientHeight
  if (cw <= 0 || ch <= 0) return
  // Large images (AI renders) need a fractional zoom to fit at all; without it
  // they open at 1× showing only their transparent middle.
  const fit = Math.min((cw - 32) / d.width, (ch - 32) / d.height)
  if (fit < 1) {
    zoom.value = Math.max(0.05, Math.round(fit * 100) / 100)
  } else {
    let best = 1
    for (const level of ZOOMS) {
      if (d.width * level <= cw - 32 && d.height * level <= ch - 32) best = level
    }
    zoom.value = best
  }
  pan.x = 0
  pan.y = 0
  fittedOnce = true
  render()
}

/* -------------------------------------------------------------- history */

function undo() {
  const d = doc.value
  const previous = undoStack.value.pop()
  if (!d || !previous) return
  redoStack.value.push(snapshotDoc(d))
  restoreDoc(d, previous)
  selection.value = null
  invalidateComposite()
  layerRevision.value += 1
  frameRevision.value += 1
  dirty.value = true
  render()
}

function redo() {
  const d = doc.value
  const next = redoStack.value.pop()
  if (!d || !next) return
  undoStack.value.push(snapshotDoc(d))
  restoreDoc(d, next)
  selection.value = null
  invalidateComposite()
  layerRevision.value += 1
  frameRevision.value += 1
  dirty.value = true
  render()
}

/* ------------------------------------------------------------- selection */

function selectAllPixels() {
  const d = doc.value
  if (!d) return
  selection.value = selectAll(d.width, d.height)
  render()
}

function deselect() {
  if (!selection.value) return
  selection.value = null
  render()
}

function invertSelected() {
  if (!selection.value) return
  selection.value = invertSelection(selection.value)
  render()
}

/** Copies the selection (or the whole active layer) to memory and the OS. */
function copySelection(): boolean {
  const d = doc.value
  if (!d) return false
  const image = activeLayerImage(d)
  const lifted = extractSelection(image, selection.value)
  const clip = lifted
    ? { image: lifted.image, x: lifted.x, y: lifted.y }
    : { image: cloneImage(image), x: 0, y: 0 }
  internalClipboard.value = clip
  void writeImageToClipboard(clip.image)
  return true
}

function cutSelection() {
  const d = doc.value
  if (!d || !selection.value?.bounds) return
  const layer = activeLayer(d)
  if (!layer.visible || layer.opacity <= 0) return
  if (!copySelection()) return
  pushUndo()
  clearSelection(activeLayerImage(d), selection.value)
  invalidateComposite()
  dirty.value = true
  render()
}

function deleteSelectionContents() {
  const d = doc.value
  if (!d || !selection.value?.bounds) return
  const layer = activeLayer(d)
  if (!layer.visible || layer.opacity <= 0) return
  pushUndo()
  clearSelection(activeLayerImage(d), selection.value)
  invalidateComposite()
  dirty.value = true
  render()
}

function pastePixels(image: PixelImage, x: number, y: number) {
  const d = doc.value
  if (!d) return
  const layer = activeLayer(d)
  if (!layer.visible) return
  pushUndo()
  const target = activeLayerImage(d)
  pasteImage(target, image, x, y)
  selection.value = selectionFromAlpha(image, x, y, d.width, d.height)
  invalidateComposite()
  dirty.value = true
  render()
}

/** Pastes an image centred on the canvas, used for system-clipboard pastes. */
function pasteCentered(image: PixelImage) {
  const d = doc.value
  if (!d) return
  pastePixels(image, Math.floor((d.width - image.width) / 2), Math.floor((d.height - image.height) / 2))
}

function pasteFromInternal() {
  const clip = internalClipboard.value
  if (!clip) return
  pastePixels(clip.image, clip.x, clip.y)
}

async function onWindowPaste(event: ClipboardEvent) {
  if (sliceOpen.value || exportOpen.value) return
  const target = event.target as HTMLElement | null
  if (target && /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)) return
  const image = await imageFromPasteEvent(event)
  if (!image) return
  event.preventDefault()
  pasteCentered(image)
}

/** Swaps every pixel equal to the chosen source color for the current color. */
function applyRecolor() {
  const d = doc.value
  const from = hexToRGBA(recolorFrom.value)
  if (!d || !from) return
  const layer = activeLayer(d)
  if (!layer.visible || layer.opacity <= 0) return
  const image = activeLayerImage(d)
  const tolerance = fillTolerance.value
  let matches = 0
  for (let y = 0; y < image.height && matches === 0; y += 1) {
    for (let x = 0; x < image.width; x += 1) {
      if (selection.value && !selection.value.mask[y * selection.value.width + x]) continue
      if (colorMatches(pixelAt(image, x, y), from, tolerance)) {
        matches += 1
        break
      }
    }
  }
  if (matches === 0) return
  pushUndo()
  replaceColor(image, from, currentColor(), tolerance, selection.value)
  invalidateComposite()
  dirty.value = true
  render()
}

/* --------------------------------------------------------------- layers */

function selectLayer(id: string) {
  const d = doc.value
  if (!d) return
  d.activeLayerId = id
  layerRevision.value += 1
}

function addLayer() {
  const d = doc.value
  if (!d) return
  pushUndo()
  const layer = createLayer(`图层 ${d.layers.length + 1}`, d.width, d.height, d.frameCount)
  d.layers.push(layer)
  d.activeLayerId = layer.id
  invalidateComposite()
  layerRevision.value += 1
  dirty.value = true
  render()
}

function removeLayer(id: string) {
  const d = doc.value
  if (!d || d.layers.length <= 1) return
  const index = d.layers.findIndex((layer) => layer.id === id)
  if (index < 0) return
  pushUndo()
  d.layers.splice(index, 1)
  if (d.activeLayerId === id) {
    d.activeLayerId = d.layers[Math.min(index, d.layers.length - 1)].id
  }
  invalidateComposite()
  layerRevision.value += 1
  dirty.value = true
  render()
}

function toggleVisible(id: string) {
  const d = doc.value
  const layer = d ? layerById(d, id) : undefined
  if (!layer) return
  layer.visible = !layer.visible
  invalidateComposite()
  layerRevision.value += 1
  dirty.value = true
  render()
}

function setOpacity(id: string, event: Event) {
  const d = doc.value
  const layer = d ? layerById(d, id) : undefined
  if (!layer) return
  const value = Number((event.target as HTMLInputElement).value)
  layer.opacity = clamp(value / 100, 0, 1)
  invalidateComposite()
  layerRevision.value += 1
  dirty.value = true
  render()
}

/* ---------------------------------------------------------------- frames */

function selectFrame(index: number) {
  const d = doc.value
  if (!d) return
  if (playing.value) stopPlayback()
  if (index < 0 || index >= d.frameCount || index === d.activeFrame) return
  d.activeFrame = index
  frameRevision.value += 1
  render()
}

function duplicateFrame() {
  const d = doc.value
  if (!d) return
  pushUndo()
  addFrame(d, true)
  invalidateComposite()
  frameRevision.value += 1
  dirty.value = true
  render()
}

function deleteFrame() {
  const d = doc.value
  if (!d || d.frameCount <= 1) return
  pushUndo()
  if (!removeFrame(d, d.activeFrame)) return
  invalidateComposite()
  frameRevision.value += 1
  dirty.value = true
  render()
}

function setFps(event: Event) {
  const d = doc.value
  if (!d) return
  const value = clamp(Math.round(Number((event.target as HTMLInputElement).value) || 8), 1, 60)
  d.fps = value
  // The base rate owns uniform timing; explicit per-frame hold times are reset.
  d.frameDurations = Array.from({ length: d.frameCount }, () => uniformFrameDuration(value))
  if (playing.value) {
    stopPlayback()
    startPlayback()
  }
  frameRevision.value += 1
  render()
}

function setFrameDuration(event: Event) {
  const d = doc.value
  if (!d) return
  const value = clamp(
    Math.round(Number((event.target as HTMLInputElement).value) || uniformFrameDuration(d.fps)),
    10,
    10000,
  )
  d.frameDurations[d.activeFrame] = value
  if (playing.value) {
    stopPlayback()
    startPlayback()
  }
  frameRevision.value += 1
  render()
}

function setTags(event: Event) {
  const d = doc.value
  if (!d) return
  const tags = (event.target as HTMLInputElement).value
    .split(/[,，]/)
    .map((tag) => tag.trim())
    .filter(Boolean)
  d.tags = Array.from(new Set(tags)).slice(0, 12)
  frameRevision.value += 1
}

function stopPlayback() {
  playing.value = false
  if (playTimer) {
    clearTimeout(playTimer)
    playTimer = null
  }
  render()
}

function startPlayback() {
  const d = doc.value
  if (!d || d.frameCount <= 1) return
  playing.value = true
  if (playTimer) clearTimeout(playTimer)
  const tick = () => {
    const current = doc.value
    if (!current || !playing.value) {
      stopPlayback()
      return
    }
    current.activeFrame = (current.activeFrame + 1) % current.frameCount
    frameRevision.value += 1
    render()
    const hold = current.frameDurations[current.activeFrame] ?? uniformFrameDuration(current.fps)
    playTimer = setTimeout(tick, Math.max(16, hold))
  }
  const first = d.frameDurations[d.activeFrame] ?? uniformFrameDuration(d.fps)
  playTimer = setTimeout(tick, Math.max(16, first))
}

function togglePlayback() {
  if (playing.value) stopPlayback()
  else startPlayback()
}

function toggleOnion() {
  onion.value = !onion.value
  render()
}

/* ---------------------------------------------------------------- sheet */

function openSlice() {
  const source = sourceSheet.value
  if (!source) return
  const tile = store.session?.project.settings.tileSize || 32
  sliceForm.frameWidth = clamp(tile, 1, source.width)
  sliceForm.frameHeight = clamp(tile, 1, source.height)
  sliceForm.margin = 0
  sliceForm.spacing = 0
  sliceOpen.value = true
  void nextTick(drawSlicePreview)
}

function drawSlicePreview() {
  const canvas = slicePreview.value
  const source = sourceSheet.value
  if (!canvas || !source) return
  const maxWidth = 520
  const maxHeight = 240
  const scale = Math.min(maxWidth / source.width, maxHeight / source.height, 1)
  const width = Math.max(1, Math.round(source.width * scale))
  const height = Math.max(1, Math.round(source.height * scale))
  canvas.width = width
  canvas.height = height
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const temp = document.createElement('canvas')
  temp.width = source.width
  temp.height = source.height
  const tctx = temp.getContext('2d')
  if (!tctx) return
  const image = tctx.createImageData(source.width, source.height)
  image.data.set(source.data)
  tctx.putImageData(image, 0, 0)
  ctx.imageSmoothingEnabled = false
  ctx.clearRect(0, 0, width, height)
  ctx.drawImage(temp, 0, 0, width, height)

  const layout = sliceLayoutInfo.value
  if (!layout) return
  ctx.strokeStyle = 'rgba(110,198,232,0.85)'
  ctx.lineWidth = 1
  for (let index = 0; index < layout.count; index += 1) {
    const origin = frameOrigin(layout, index)
    const x = Math.round(origin.x * scale)
    const y = Math.round(origin.y * scale)
    const w = Math.max(1, Math.round(layout.frameWidth * scale))
    const h = Math.max(1, Math.round(layout.frameHeight * scale))
    ctx.strokeRect(x + 0.5, y + 0.5, w - 1, h - 1)
  }
}

function applySlice() {
  const source = sourceSheet.value
  const layout = sliceLayoutInfo.value
  const d = doc.value
  if (!source || !layout || !d) return
  const slices = sliceSheet(source, sliceForm)
  if (slices.length === 0) return
  pushUndo()
  applySlices(d, slices)
  selection.value = null
  invalidateComposite()
  layerRevision.value += 1
  frameRevision.value += 1
  dirty.value = true
  sliceOpen.value = false
  fitZoom()
  render()
}

function openExport() {
  const count = frameCount.value
  exportColumns.value = count <= 8 ? count : Math.ceil(Math.sqrt(count))
  exportOpen.value = true
}

/* ----------------------------------------------------------------- save */

function imageCanvas(image: PixelImage): HTMLCanvasElement | null {
  const canvas = document.createElement('canvas')
  canvas.width = image.width
  canvas.height = image.height
  const ctx = canvas.getContext('2d')
  if (!ctx) return null
  const frame = ctx.createImageData(image.width, image.height)
  frame.data.set(image.data)
  ctx.putImageData(frame, 0, 0)
  return canvas
}

function compositeCanvas(d: EditorDoc): HTMLCanvasElement | null {
  return imageCanvas(compositeDoc(d))
}

async function save() {
  const d = doc.value
  const id = store.editorAssetId
  if (!d || !id || saving.value) return
  if (playing.value) stopPlayback()
  const canvas = compositeCanvas(d)
  if (!canvas) {
    store.setError(new Error('无法导出画布'))
    return
  }
  let dataURL = ''
  try {
    dataURL = canvas.toDataURL('image/png')
  } catch (raw) {
    store.setError(raw)
    return
  }
  saving.value = true
  const saved = await store.saveAssetVersion(id, dataURL)
  saving.value = false
  if (saved) {
    dirty.value = false
    savedVersion.value = saved.currentVersion ?? null
  }
}

async function exportSheet() {
  const d = doc.value
  const id = store.editorAssetId
  if (!d || !id || saving.value) return
  if (playing.value) stopPlayback()
  const columns = clamp(Math.round(Number(exportColumns.value) || d.frameCount), 1, d.frameCount)
  const frames: PixelImage[] = []
  for (let index = 0; index < d.frameCount; index += 1) frames.push(frameComposite(d, index))
  const sheet = composeSheet(frames, columns)
  if (!sheet) return
  const canvas = imageCanvas(sheet)
  if (!canvas) {
    store.setError(new Error('无法导出精灵表'))
    return
  }
  let dataURL = ''
  try {
    dataURL = canvas.toDataURL('image/png')
  } catch (raw) {
    store.setError(raw)
    return
  }
  const info = sheetInfoFor(
    d.width,
    d.height,
    d.frameCount,
    columns,
    d.fps,
    true,
    0,
    0,
    d.frameDurations,
    d.tags,
  )
  saving.value = true
  const saved = await store.saveAssetSheet(id, dataURL, info)
  saving.value = false
  if (saved) {
    dirty.value = false
    savedVersion.value = saved.currentVersion ?? null
    exportOpen.value = false
  }
}

async function goBack() {
  stopPlayback()
  if (dirty.value) {
    const ok = await confirmDialog({
      title: '离开像素编辑器',
      message: '当前画布有未保存的修改。离开后这些修改会丢失，已保存的版本不受影响。',
      confirmText: '放弃修改',
      tone: 'danger',
    })
    if (!ok) return
  }
  store.closeEditor()
}

/* ------------------------------------------------------------- keyboard */

function setColor(event: Event) {
  color.value = (event.target as HTMLInputElement).value
}

function onKeyDown(event: KeyboardEvent) {
  if (sliceOpen.value || exportOpen.value) return
  const target = event.target as HTMLElement | null
  if (target && /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)) return
  if (event.code === 'Space' && target?.tagName !== 'BUTTON') {
    spaceDown.value = true
    event.preventDefault()
    return
  }
  const mod = event.ctrlKey || event.metaKey
  const key = event.key.toLowerCase()
  if (mod && key === 's') {
    event.preventDefault()
    event.stopPropagation()
    void save()
    return
  }
  if (mod && key === 'z') {
    event.preventDefault()
    if (event.shiftKey) redo()
    else undo()
    return
  }
  if (mod && key === 'y') {
    event.preventDefault()
    redo()
    return
  }
  if (mod && key === 'a') {
    event.preventDefault()
    selectAllPixels()
    return
  }
  if (mod && key === 'd') {
    event.preventDefault()
    deselect()
    return
  }
  if (mod && key === 'c') {
    event.preventDefault()
    copySelection()
    return
  }
  if (mod && key === 'x') {
    event.preventDefault()
    cutSelection()
    return
  }
  if (mod && key === 'v') {
    // Without an internal copy, leave the event alone so the native paste
    // (system clipboard image) reaches onWindowPaste.
    if (internalClipboard.value) {
      event.preventDefault()
      pasteFromInternal()
    }
    return
  }
  if (mod) return
  if (key === 'delete' || key === 'backspace') {
    if (selection.value?.bounds) {
      event.preventDefault()
      deleteSelectionContents()
      return
    }
    return
  }
  if (key === 'enter' && target?.tagName !== 'BUTTON') {
    event.preventDefault()
    togglePlayback()
    return
  }
  if (key === 'arrowleft') {
    event.preventDefault()
    selectFrame(activeFrame.value - 1)
    return
  }
  if (key === 'arrowright') {
    event.preventDefault()
    selectFrame(activeFrame.value + 1)
    return
  }
  const shortcuts: Record<string, ToolId> = {
    b: 'pencil',
    e: 'eraser',
    g: 'bucket',
    i: 'eyedropper',
    l: 'line',
    r: 'rect',
    o: 'ellipse',
    m: 'select',
    a: 'lasso',
    w: 'wand',
    v: 'move',
  }
  if (shortcuts[key]) {
    tool.value = shortcuts[key]
    return
  }
  if (key === '[') {
    brushSize.value = Math.max(1, brushSize.value - 1)
    return
  }
  if (key === ']') {
    brushSize.value = Math.min(8, brushSize.value + 1)
    return
  }
  if (key === '=' || key === '+') {
    zoomIn()
    return
  }
  if (key === '-') {
    zoomOut()
    return
  }
  if (key === 'escape') {
    if (selection.value || selectDrag || lassoDrag) {
      selection.value = null
      selectDrag = null
      lassoDrag = null
      render()
      return
    }
    void goBack()
  }
}

watch(sliceOpen, (open) => {
  if (open) void nextTick(drawSlicePreview)
})

watch(sliceForm, () => {
  if (sliceOpen.value) void nextTick(drawSlicePreview)
})

// Toggling the grid in settings repaints the open canvas immediately.
watch(
  () => store.editorSettings?.showGrid,
  () => render(),
)

function onKeyUp(event: KeyboardEvent) {
  if (event.code === 'Space') spaceDown.value = false
}

function onWindowBlur() {
  spaceDown.value = false
  rightErase = false
}

onMounted(async () => {
  window.addEventListener('keydown', onKeyDown)
  window.addEventListener('keyup', onKeyUp)
  window.addEventListener('blur', onWindowBlur)
  window.addEventListener('paste', onWindowPaste)
  if (!store.editorSettings) await store.loadEditorSettings()
  await nextTick()
  await init()
})

// Switching assets while the editor stays mounted must load the new one.
watch(
  () => store.editorAssetId,
  (id, previous) => {
    if (id && id !== previous) void init()
  },
)

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeyDown)
  window.removeEventListener('keyup', onKeyUp)
  window.removeEventListener('blur', onWindowBlur)
  window.removeEventListener('paste', onWindowPaste)
  observer?.disconnect()
  observer = null
  if (renderHandle) {
    cancelAnimationFrame(renderHandle)
    renderHandle = 0
  }
  if (playTimer) {
    clearTimeout(playTimer)
    playTimer = null
  }
  stroke = null
  panning = null
  isPanning.value = false
  selectDrag = null
  lassoDrag = null
  moveDrag = null
  pixelStrokes = []
})
</script>

<template>
  <div class="flex h-full min-h-0 flex-col bg-canvas">
    <header class="flex h-12 shrink-0 items-center gap-2 border-b border-line bg-panel/60 px-3">
      <button type="button" class="pb-btn h-8 px-2 py-1 text-xs" title="返回素材库" @click="goBack()">
        <ArrowLeft :size="14" />
        返回
      </button>
      <div class="min-w-0">
        <div class="truncate text-xs font-medium text-ink">{{ asset?.name ?? '像素编辑器' }}</div>
        <div class="font-mono text-[10px] text-faint">
          {{ doc?.width ?? 0 }} × {{ doc?.height ?? 0 }} px · {{ layerCount }} 图层 · {{ frameCount }} 帧
        </div>
      </div>
      <span
        class="pb-chip ml-2"
        :class="dirty ? 'border-amber/40 text-amber' : 'text-faint'"
        :title="savedVersion ? `当前版本 v${savedVersion}` : '尚未保存过版本'"
      >
        {{ saving ? '保存中' : dirty ? '未保存' : savedVersion ? `v${savedVersion}` : '未保存' }}
      </span>

      <div class="ml-auto flex items-center gap-1.5">
        <button
          type="button"
          class="pb-btn h-8 px-2 py-1 text-xs"
          :disabled="undoStack.length === 0"
          title="撤销 (Ctrl+Z)"
          @click="undo()"
        >
          <Undo2 :size="14" />
        </button>
        <button
          type="button"
          class="pb-btn h-8 px-2 py-1 text-xs"
          :disabled="redoStack.length === 0"
          title="重做 (Ctrl+Shift+Z)"
          @click="redo()"
        >
          <Redo2 :size="14" />
        </button>
        <div class="mx-1 flex items-center gap-0.5 rounded-md border border-line bg-panel-2/60 px-0.5">
          <button type="button" class="pb-icon-btn" title="缩小 (-)" @click="zoomOut()">
            <ZoomOut :size="14" />
          </button>
          <span class="w-9 text-center font-mono text-[10px] text-ink">{{ zoom }}×</span>
          <button type="button" class="pb-icon-btn" title="放大 (+)" @click="zoomIn()">
            <ZoomIn :size="14" />
          </button>
          <button type="button" class="pb-icon-btn text-[10px]" title="适应窗口" @click="fitZoom()">适应</button>
        </div>
        <button
          type="button"
          class="pb-btn h-8 px-2 py-1 text-xs"
          :disabled="!sourceSheet"
          title="按网格把原始素材切片成动画帧"
          @click="openSlice()"
        >
          <Scissors :size="14" />
          切片
        </button>
        <button
          type="button"
          class="pb-btn h-8 px-2 py-1 text-xs"
          title="把全部帧打包成精灵表并保存为版本"
          @click="openExport()"
        >
          <Grid3x3 :size="14" />
          精灵表
        </button>
        <button
          type="button"
          class="pb-btn h-8 py-1 text-xs"
          :disabled="saving || !dirty"
          title="把当前帧合成为新的不可变版本 (Ctrl+S)"
          @click="save()"
        >
          <Save :size="14" />
          {{ saving ? '保存中' : '保存版本' }}
        </button>
      </div>
    </header>

    <div class="flex min-h-0 flex-1">
      <aside class="flex w-12 shrink-0 flex-col items-center gap-1 overflow-y-auto border-r border-line bg-panel/50 py-2">
        <button
          v-for="item in TOOLS"
          :key="item.id"
          type="button"
          class="flex size-9 items-center justify-center rounded-md border border-transparent text-muted transition-colors hover:border-line hover:bg-panel-2 hover:text-ink"
          :class="tool === item.id ? 'border-accent/50 bg-accent/10 text-accent' : ''"
          :title="`${item.label} (${item.key})`"
          @click="setTool(item.id)"
        >
          <component :is="item.icon" :size="16" />
        </button>
      </aside>

      <div class="relative min-w-0 flex-1">
        <div ref="viewport" class="absolute inset-0 overflow-hidden bg-[#151a23]">
          <canvas
            ref="surface"
            class="block h-full w-full touch-none"
            :style="{ cursor: canvasCursor }"
            :title="`${TOOLS.find((item) => item.id === tool)?.label ?? ''}工具`"
            @pointerdown="onPointerDown"
            @pointermove="onPointerMove"
            @pointerup="endStroke"
            @pointercancel="endStroke"
            @pointerleave="onPointerLeave"
            @wheel.prevent="onWheel"
            @contextmenu.prevent
          />
        </div>
        <div
          v-if="loading"
          class="absolute inset-0 flex items-center justify-center bg-canvas/70 text-xs text-muted"
        >
          正在载入素材…
        </div>
        <div
          v-else-if="loadError"
          class="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-canvas/80 text-xs text-muted"
        >
          <span class="text-ink">{{ loadError }}</span>
          <button type="button" class="pb-btn h-8 py-1 text-xs" @click="goBack()">返回素材库</button>
        </div>
        <p
          v-else-if="loadWarning"
          class="pointer-events-none absolute inset-x-0 top-2 mx-auto w-fit max-w-[90%] rounded-lg border border-amber/30 bg-amber/15 px-3 py-1.5 text-[11px] text-amber"
        >
          {{ loadWarning }}
        </p>
      </div>

      <aside class="flex w-[248px] shrink-0 flex-col gap-4 overflow-y-auto border-l border-line bg-panel/50 p-3">
        <section>
          <div class="pb-label">颜色</div>
          <div class="mt-1.5 flex items-center gap-2">
            <input
              type="color"
              class="h-8 w-10 cursor-pointer rounded border border-line bg-transparent p-0.5"
              :value="color.slice(0, 7)"
              title="自定义颜色"
              @input="setColor"
            />
            <div class="min-w-0">
              <div class="font-mono text-[11px] text-ink">{{ color }}</div>
            </div>
          </div>
          <div v-if="paletteColors.length > 0" class="mt-2 flex flex-wrap gap-1">
            <button
              v-for="swatch in paletteColors"
              :key="swatch"
              type="button"
              class="size-5 rounded border border-black/40 transition-transform hover:scale-110"
              :class="color.toLowerCase() === swatch.toLowerCase() ? 'ring-1 ring-accent ring-offset-1 ring-offset-panel' : ''"
              :style="{ background: swatch }"
              :title="swatch"
              @click="color = swatch"
            />
          </div>
        </section>

        <section>
          <div class="flex items-center justify-between">
            <div class="pb-label">选区</div>
            <span class="font-mono text-[10px] text-faint">{{ selectionSummary }}</span>
          </div>
          <div class="mt-1.5 grid grid-cols-2 gap-1">
            <button type="button" class="pb-btn h-6 px-1 py-0 text-[10px]" title="全选 (Ctrl+A)" @click="selectAllPixels()">
              全选
            </button>
            <button
              type="button"
              class="pb-btn h-6 px-1 py-0 text-[10px]"
              :disabled="!selection"
              title="取消选区 (Ctrl+D)"
              @click="deselect()"
            >
              取消
            </button>
            <button
              type="button"
              class="pb-btn h-6 px-1 py-0 text-[10px]"
              :disabled="!selection?.bounds"
              title="反选"
              @click="invertSelected()"
            >
              反选
            </button>
            <button
              type="button"
              class="pb-btn h-6 px-1 py-0 text-[10px]"
              :disabled="!selection?.bounds"
              title="剪切 (Ctrl+X)"
              @click="cutSelection()"
            >
              剪切
            </button>
            <button
              type="button"
              class="pb-btn h-6 px-1 py-0 text-[10px]"
              :disabled="!selection?.bounds"
              title="复制 (Ctrl+C)"
              @click="copySelection()"
            >
              复制
            </button>
            <button
              type="button"
              class="pb-btn h-6 px-1 py-0 text-[10px]"
              :disabled="!internalClipboard"
              title="粘贴 (Ctrl+V)；系统图片可直接 Ctrl+V 粘贴"
              @click="pasteFromInternal()"
            >
              粘贴
            </button>
            <button
              type="button"
              class="pb-btn col-span-2 h-6 px-1 py-0 text-[10px]"
              :disabled="!selection?.bounds"
              title="清除选区内容 (Delete)"
              @click="deleteSelectionContents()"
            >
              清除选区内容
            </button>
          </div>
          <p class="mt-1 text-[10px] text-faint">作用于当前图层与当前帧。Shift 加选，Esc 取消。</p>
        </section>

        <section>
          <div class="pb-label">笔刷与形状</div>
          <div class="mt-1.5 flex items-center gap-1">
            <button
              v-for="size in [1, 2, 3, 4]"
              :key="size"
              type="button"
              class="flex h-7 flex-1 items-center justify-center rounded border text-[11px]"
              :class="
                brushSize === size
                  ? 'border-accent/50 bg-accent/10 text-accent'
                  : 'border-line bg-panel-2/40 text-muted hover:text-ink'
              "
              @click="brushSize = size"
            >
              {{ size }}px
            </button>
          </div>
          <button
            type="button"
            class="mt-2 flex h-7 w-full items-center justify-between rounded border px-2 text-[11px]"
            :class="
              shapeFilled
                ? 'border-accent/50 bg-accent/10 text-accent'
                : 'border-line bg-panel-2/40 text-muted hover:text-ink'
            "
            title="矩形与椭圆是否填充"
            @click="shapeFilled = !shapeFilled"
          >
            <span>填充形状</span>
            <span class="font-mono text-[10px]">{{ shapeFilled ? '开' : '关' }}</span>
          </button>
          <button
            type="button"
            class="mt-1 flex h-7 w-full items-center justify-between rounded border px-2 text-[11px]"
            :class="
              pixelPerfect
                ? 'border-accent/50 bg-accent/10 text-accent'
                : 'border-line bg-panel-2/40 text-muted hover:text-ink'
            "
            title="手绘时去掉折角处多余的像素（仅 1px 笔刷）"
            @click="pixelPerfect = !pixelPerfect"
          >
            <span>像素完美手绘</span>
            <span class="font-mono text-[10px]">{{ pixelPerfect ? '开' : '关' }}</span>
          </button>
          <div class="mt-2">
            <div class="mb-1 text-[10px] text-faint">对称绘制</div>
            <div class="grid grid-cols-4 gap-1">
              <button
                v-for="mode in MIRROR_MODES"
                :key="mode.id"
                type="button"
                class="flex h-6 items-center justify-center rounded border text-[10px]"
                :class="
                  symmetry === mode.id
                    ? 'border-accent/50 bg-accent/10 text-accent'
                    : 'border-line bg-panel-2/40 text-muted hover:text-ink'
                "
                :title="`对称：${mode.label}`"
                @click="setSymmetry(mode.id)"
              >
                {{ mode.label }}
              </button>
            </div>
          </div>
        </section>

        <section>
          <div class="pb-label">填充与替换</div>
          <div class="mt-1.5 flex items-center gap-2">
            <span class="text-[10px] text-faint">容差</span>
            <input
              type="range"
              min="0"
              max="255"
              :value="fillTolerance"
              class="h-1 flex-1 cursor-pointer accent-[#6ec6e8]"
              title="油漆桶与魔棒的颜色容差"
              @input="setFillTolerance"
            />
            <span class="w-7 text-right font-mono text-[10px] text-ink">{{ fillTolerance }}</span>
          </div>
          <button
            type="button"
            class="mt-1 flex h-6 w-full items-center justify-between rounded border px-2 text-[10px]"
            :class="
              fillContiguous
                ? 'border-accent/50 bg-accent/10 text-accent'
                : 'border-line bg-panel-2/40 text-muted hover:text-ink'
            "
            title="关闭后填充整张图（或选区内）所有同色像素"
            @click="fillContiguous = !fillContiguous"
          >
            <span>邻近填充</span>
            <span class="font-mono">{{ fillContiguous ? '开' : '关' }}</span>
          </button>
          <div class="mt-1.5 flex items-center gap-1.5">
            <input
              type="color"
              :value="recolorFrom.slice(0, 7)"
              class="h-6 w-8 cursor-pointer rounded border border-line bg-transparent p-0.5"
              title="要被替换的原色"
              @input="setRecolorFrom"
            />
            <span class="text-[10px] text-faint">→</span>
            <span
              class="size-5 rounded border border-black/40"
              :style="{ background: color }"
              :title="`替换为 ${color}`"
            />
            <button
              type="button"
              class="pb-btn ml-auto h-6 px-2 py-0 text-[10px]"
              title="把当前图层（或选区）中等于原色的像素换成当前颜色"
              @click="applyRecolor()"
            >
              替换
            </button>
          </div>
          <button
            type="button"
            class="mt-1 w-full text-left text-[10px] text-faint hover:text-ink"
            title="把当前颜色设为原色"
            @click="recolorFrom = color.slice(0, 7)"
          >
            以当前色为原色
          </button>
        </section>

        <section class="min-h-0">
          <div class="flex items-center justify-between">
            <div class="pb-label flex items-center gap-1">
              <Layers :size="12" />
              图层
            </div>
            <div class="flex items-center gap-0.5">
              <button type="button" class="pb-icon-btn" title="新建图层" @click="addLayer()">
                <Plus :size="13" />
              </button>
              <button
                type="button"
                class="pb-icon-btn"
                :disabled="layerCount <= 1"
                title="删除当前图层"
                @click="removeLayer(activeLayerId)"
              >
                <Trash2 :size="13" />
              </button>
            </div>
          </div>
          <ul class="mt-1.5 space-y-1">
            <li v-for="layer in layerList" :key="layer.id">
              <div
                class="cursor-pointer rounded-md border px-2 py-1.5"
                :class="
                  layer.id === activeLayerId
                    ? 'border-accent/50 bg-accent/10'
                    : 'border-line bg-panel-2/40 hover:border-line'
                "
                @click="selectLayer(layer.id)"
              >
                <div class="flex items-center gap-1.5">
                  <button
                    type="button"
                    class="text-muted hover:text-ink"
                    :title="layer.visible ? '隐藏图层' : '显示图层'"
                    @click.stop="toggleVisible(layer.id)"
                  >
                    <Eye v-if="layer.visible" :size="12" />
                    <EyeOff v-else :size="12" />
                  </button>
                  <span class="min-w-0 flex-1 truncate text-[11px] text-ink">{{ layer.name }}</span>
                  <span class="font-mono text-[9px] text-faint">{{ Math.round(layer.opacity * 100) }}%</span>
                </div>
                <input
                  type="range"
                  min="0"
                  max="100"
                  :value="Math.round(layer.opacity * 100)"
                  class="mt-1 h-1 w-full cursor-pointer accent-[#6ec6e8]"
                  title="图层不透明度"
                  @click.stop
                  @input="setOpacity(layer.id, $event)"
                />
              </div>
            </li>
          </ul>
        </section>
      </aside>
    </div>

    <section class="flex h-[100px] shrink-0 flex-col border-t border-line bg-panel/60">
      <div class="flex h-9 shrink-0 items-center gap-2 border-b border-line/70 px-3">
        <button
          type="button"
          class="pb-icon-btn"
          :class="playing ? 'bg-accent/10 text-accent' : ''"
          :disabled="frameCount < 2"
          :title="playing ? '暂停 (Enter)' : '播放 (Enter)'"
          @click="togglePlayback()"
        >
          <Pause v-if="playing" :size="14" />
          <Play v-else :size="14" />
        </button>
        <template v-if="frameCount > 1">
          <label class="flex items-center gap-1 text-[10px] text-faint" title="基准帧率；修改会重置为均匀逐帧时长">
            <span class="font-mono">FPS</span>
            <input
              type="number"
              min="1"
              max="60"
              :value="doc?.fps ?? 8"
              class="h-6 w-12 rounded border border-line bg-panel-2/60 px-1 font-mono text-[11px] text-ink"
              @change="setFps"
            />
          </label>
          <label class="flex items-center gap-1 text-[10px] text-faint" title="当前帧停留时长（毫秒）；播放按每帧各自的时长推进">
            <span class="font-mono">帧 ms</span>
            <input
              type="number"
              min="10"
              max="10000"
              step="10"
              :value="doc?.frameDurations?.[activeFrame] ?? uniformFrameDuration(doc?.fps ?? 8)"
              class="h-6 w-14 rounded border border-line bg-panel-2/60 px-1 font-mono text-[11px] text-ink"
              @change="setFrameDuration"
            />
          </label>
        </template>
        <span v-else class="text-[10px] text-faint" title="单帧素材没有帧率；加一帧或切片后出现">单帧素材</span>
        <button
          type="button"
          class="flex h-6 items-center gap-1 rounded border px-2 text-[10px]"
          :class="
            onion
              ? 'border-accent/50 bg-accent/10 text-accent'
              : 'border-line bg-panel-2/40 text-muted hover:text-ink'
          "
          title="洋葱皮：上一帧红色、下一帧青色半透明显示"
          @click="toggleOnion()"
        >
          <Blend :size="12" />
          洋葱皮
        </button>
        <span class="font-mono text-[10px] text-faint">第 {{ activeFrame + 1 }} / {{ frameCount }} 帧</span>
        <div class="ml-auto flex items-center gap-1">
          <button type="button" class="pb-icon-btn" title="复制当前帧到下一帧" @click="duplicateFrame()">
            <Plus :size="13" />
          </button>
          <button
            type="button"
            class="pb-icon-btn"
            :disabled="frameCount <= 1"
            title="删除当前帧"
            @click="deleteFrame()"
          >
            <Trash2 :size="13" />
          </button>
        </div>
      </div>
      <div class="flex min-h-0 flex-1 items-center gap-2 overflow-x-auto px-3 py-2">
        <button
          v-for="index in frameList"
          :key="index"
          type="button"
          class="relative flex h-12 min-w-10 shrink-0 items-center justify-center overflow-hidden rounded border bg-[#151a23] px-1"
          :class="index === activeFrame ? 'border-accent' : 'border-line hover:border-muted'"
          :title="`第 ${index + 1} 帧`"
          @click="selectFrame(index)"
        >
          <canvas :ref="frameCanvasRef(index)" class="h-9 w-auto max-w-16 [image-rendering:pixelated]" />
          <span class="absolute bottom-0 right-1 font-mono text-[9px] text-faint">{{ index + 1 }}</span>
        </button>
      </div>
    </section>

    <footer
      class="flex h-7 shrink-0 items-center gap-4 border-t border-line bg-panel/70 px-3 font-mono text-[10px] text-faint"
    >
      <span>{{ hover ? `x ${hover.x}  y ${hover.y}` : '—' }}</span>
      <span>{{ tool }}</span>
      <span class="ml-auto">
        B/E/G/I/L/R/O 工具 · M 选区 · A 套索 · W 魔棒 · V 移动 · [ ] 笔刷 · +/- 缩放 · Ctrl+A/D
        选区 · Ctrl+C/X/V · Enter 播放 · ← → 切帧 · 空格拖动平移 · 右键擦除
      </span>
    </footer>

    <AppModal
      v-model:open="sliceOpen"
      title="从精灵表切片"
      description="按网格把原始素材切成动画帧，替换当前画布，可用撤销恢复。"
    >
      <div class="grid grid-cols-2 gap-3">
        <label class="flex flex-col gap-1">
          <span class="pb-label">帧宽 (px)</span>
          <input v-model.number="sliceForm.frameWidth" type="number" min="1" class="pb-input font-mono" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="pb-label">帧高 (px)</span>
          <input v-model.number="sliceForm.frameHeight" type="number" min="1" class="pb-input font-mono" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="pb-label">外边距 (px)</span>
          <input v-model.number="sliceForm.margin" type="number" min="0" class="pb-input font-mono" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="pb-label">间隙 (px)</span>
          <input v-model.number="sliceForm.spacing" type="number" min="0" class="pb-input font-mono" />
        </label>
      </div>

      <div v-if="sliceLayoutInfo" class="mt-3 text-xs text-muted">
        每帧 {{ sliceLayoutInfo.frameWidth }} × {{ sliceLayoutInfo.frameHeight }} ·
        {{ sliceLayoutInfo.columns }} 列 × {{ sliceLayoutInfo.rows }} 行 =
        <span class="text-ink">{{ sliceLayoutInfo.count }} 帧</span>
      </div>
      <div v-else class="mt-3 text-xs text-red">按当前尺寸切不出完整帧，请调整帧宽高或边距。</div>

      <div class="mt-3 flex justify-center rounded-lg border border-line bg-[#151a23] p-2">
        <canvas ref="slicePreview" class="max-h-56 w-auto [image-rendering:pixelated]" />
      </div>

      <template #footer>
        <button type="button" class="pb-btn h-8 px-3 py-1 text-xs" @click="sliceOpen = false">取消</button>
        <button
          type="button"
          class="pb-btn h-8 px-3 py-1 text-xs"
          :class="sliceLayoutInfo ? 'pb-btn-primary' : ''"
          :disabled="!sliceLayoutInfo"
          @click="applySlice()"
        >
          应用切片
        </button>
      </template>
    </AppModal>

    <AppModal
      v-model:open="exportOpen"
      title="保存精灵表版本"
      description="把全部帧打包成一张 PNG，并附逐帧图与 Aseprite / TexturePacker JSON。"
    >
      <div class="grid grid-cols-2 gap-3">
        <label class="flex flex-col gap-1">
          <span class="pb-label">列数</span>
          <input v-model.number="exportColumns" type="number" min="1" class="pb-input font-mono" />
        </label>
        <div class="rounded border border-line bg-panel-2/40 px-2 py-1.5 font-mono text-[11px] text-muted">
          <div>帧 {{ frameCount }} · {{ doc?.fps ?? 8 }} FPS · {{ totalDuration }} ms</div>
          <div v-if="exportSheetSize">
            图片 {{ exportSheetSize.width }} × {{ exportSheetSize.height }} px
          </div>
        </div>
      </div>
      <label class="mt-3 flex flex-col gap-1">
        <span class="pb-label">动画标签（逗号分隔）</span>
        <input
          :value="(doc?.tags ?? []).join(', ')"
          class="pb-input"
          placeholder="idle, walk"
          @change="setTags"
        />
        <span class="mt-1 text-[11px] text-faint">
          写入 frameTags 与 animations，留空为 default
        </span>
      </label>
      <p class="mt-3 text-xs text-faint">
        每帧 {{ doc?.width ?? 0 }} × {{ doc?.height ?? 0 }}，循环播放，时长取自时间轴。
        顶部的「保存版本」只保存当前帧。
      </p>

      <template #footer>
        <button type="button" class="pb-btn h-8 px-3 py-1 text-xs" @click="exportOpen = false">取消</button>
        <button
          type="button"
          class="pb-btn pb-btn-primary h-8 px-3 py-1 text-xs"
          :disabled="saving || frameCount < 1"
          @click="exportSheet()"
        >
          {{ saving ? '保存中…' : '保存精灵表' }}
        </button>
      </template>
    </AppModal>
  </div>
</template>