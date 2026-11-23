<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import type { GameMap, MapObjectKind } from '../domain/types'
import { objectAt, solidAt, type Cell, type MapTool } from './model'
import { drawMap, screenToWorld, worldToCell, type Camera, type ScreenRect } from './render'

const props = defineProps<{
  map: GameMap
  layerId: string
  tool: MapTool
  tile: { set: number; tile: number }
  objectKind: MapObjectKind
  images: ReadonlyMap<string, CanvasImageSource>
  showGrid: boolean
  selectedObjectId: string | null
  preview: boolean
}>()

const emit = defineEmits<{
  (e: 'begin-stroke'): void
  (e: 'paint', cells: Cell[]): void
  (e: 'erase', cells: Cell[]): void
  (e: 'collision', cells: Cell[], solid: boolean): void
  (e: 'pick', cell: Cell): void
  (e: 'select-object', id: string | null): void
  (e: 'add-object', cell: Cell): void
  (e: 'move-object', id: string, cell: Cell): void
  (e: 'zoom', zoom: number): void
  (e: 'cursor', cell: Cell | null): void
  (e: 'exit-preview'): void
}>()

const wrapper = ref<HTMLDivElement | null>(null)
const canvas = ref<HTMLCanvasElement | null>(null)
const cssWidth = ref(800)
const cssHeight = ref(600)
const hover = ref<Cell | null>(null)
const rect = ref<ScreenRect | null>(null)
const camera = reactive<Camera>({ x: 0, y: 0, zoom: 1 })

const ZOOM_STEPS = [0.5, 0.75, 1, 1.5, 2, 3, 4, 6]

let ctx: CanvasRenderingContext2D | null = null
let observer: ResizeObserver | null = null
let drawHandle = 0
let dragging: 'paint' | 'erase' | 'collide' | 'rect' | 'pan' | 'object' | null = null
let lastCell: Cell | null = null
let rectStart: Cell | null = null
let collideMode = true
let panLast = { x: 0, y: 0 }
let dragObject: { id: string; grabX: number; grabY: number } | null = null
let spaceDown = false

const player = ref<{ x: number; y: number } | null>(null)
const pressed = new Set<string>()
let moveHandle = 0
let lastFrame = 0

const zoomLabel = computed(() => `${Math.round(camera.zoom * 100)}%`)

function worldSize() {
  return {
    width: props.map.width * props.map.tileSize,
    height: props.map.height * props.map.tileSize,
  }
}

function fitCamera() {
  const world = worldSize()
  const fit = Math.min(cssWidth.value / world.width, cssHeight.value / world.height)
  let zoom = ZOOM_STEPS[0]
  for (const step of ZOOM_STEPS) {
    if (step <= fit) zoom = step
  }
  camera.zoom = zoom
  camera.x = world.width / 2
  camera.y = world.height / 2
  emit('zoom', camera.zoom)
}

function scheduleDraw() {
  if (drawHandle) return
  drawHandle = requestAnimationFrame(() => {
    drawHandle = 0
    render()
  })
}

function render() {
  if (!ctx) return
  drawMap(ctx, {
    map: props.map,
    camera,
    width: cssWidth.value,
    height: cssHeight.value,
    images: props.images,
    grid: props.showGrid,
    hover: hover.value,
    rect: rect.value,
    selectedObjectId: props.selectedObjectId,
    player: player.value,
    preview: props.preview,
  })
}

function resize() {
  const box = wrapper.value
  const el = canvas.value
  if (!box || !el) return
  const rectBox = box.getBoundingClientRect()
  cssWidth.value = Math.max(120, Math.floor(rectBox.width))
  cssHeight.value = Math.max(120, Math.floor(rectBox.height))
  const dpr = window.devicePixelRatio || 1
  el.width = Math.floor(cssWidth.value * dpr)
  el.height = Math.floor(cssHeight.value * dpr)
  el.style.width = `${cssWidth.value}px`
  el.style.height = `${cssHeight.value}px`
  try {
    ctx = el.getContext('2d')
  } catch {
    ctx = null
  }
  ctx?.setTransform(dpr, 0, 0, dpr, 0, 0)
  render()
}

function eventPoint(event: PointerEvent | WheelEvent): { x: number; y: number } {
  const el = canvas.value
  if (!el) return { x: 0, y: 0 }
  const box = el.getBoundingClientRect()
  return { x: event.clientX - box.left, y: event.clientY - box.top }
}

function cellAt(event: PointerEvent | WheelEvent): Cell {
  const point = eventPoint(event)
  const world = screenToWorld(camera, cssWidth.value, cssHeight.value, point.x, point.y)
  return worldToCell(props.map, world.x, world.y)
}

function inMap(cell: Cell): boolean {
  return cell.x >= 0 && cell.y >= 0 && cell.x < props.map.width && cell.y < props.map.height
}

function clampCell(cell: Cell): Cell {
  return {
    x: Math.max(0, Math.min(props.map.width - 1, cell.x)),
    y: Math.max(0, Math.min(props.map.height - 1, cell.y)),
  }
}

/** Bresenham line, so fast strokes do not leave holes. */
function lineCells(from: Cell, to: Cell): Cell[] {
  const cells: Cell[] = []
  let x = from.x
  let y = from.y
  const dx = Math.abs(to.x - x)
  const dy = -Math.abs(to.y - y)
  const sx = x < to.x ? 1 : -1
  const sy = y < to.y ? 1 : -1
  let err = dx + dy
  for (;;) {
    cells.push({ x, y })
    if (x === to.x && y === to.y) break
    const e2 = 2 * err
    if (e2 >= dy) {
      err += dy
      x += sx
    }
    if (e2 <= dx) {
      err += dx
      y += sy
    }
  }
  return cells.filter(inMap)
}

function rectCells(a: Cell, b: Cell): Cell[] {
  const cells: Cell[] = []
  const x0 = Math.max(0, Math.min(a.x, b.x))
  const x1 = Math.min(props.map.width - 1, Math.max(a.x, b.x))
  const y0 = Math.max(0, Math.min(a.y, b.y))
  const y1 = Math.min(props.map.height - 1, Math.max(a.y, b.y))
  for (let y = y0; y <= y1; y += 1) {
    for (let x = x0; x <= x1; x += 1) cells.push({ x, y })
  }
  return cells
}

function beginStroke() {
  emit('begin-stroke')
}

function onPointerDown(event: PointerEvent) {
  if (props.preview) return
  const cell = cellAt(event)
  if (event.button === 1 || spaceDown) {
    dragging = 'pan'
    panLast = { x: event.clientX, y: event.clientY }
    canvas.value?.setPointerCapture?.(event.pointerId)
    event.preventDefault()
    return
  }
  if (event.button !== 0 && event.button !== 2) return
  const eraseClick = event.button === 2
  const tool = props.tool
  canvas.value?.setPointerCapture?.(event.pointerId)

  if (tool === 'brush') {
    beginStroke()
    if (eraseClick) {
      dragging = 'erase'
      if (inMap(cell)) emit('erase', [cell])
    } else {
      dragging = 'paint'
      if (inMap(cell)) emit('paint', [cell])
    }
    lastCell = cell
    return
  }
  if (tool === 'eraser') {
    beginStroke()
    dragging = 'erase'
    if (inMap(cell)) emit('erase', [cell])
    lastCell = cell
    return
  }
  if (tool === 'collision') {
    beginStroke()
    dragging = 'collide'
    collideMode = !eraseClick
    if (inMap(cell)) emit('collision', [cell], collideMode)
    lastCell = cell
    return
  }
  if (tool === 'rect') {
    if (!inMap(cell)) return
    rectStart = cell
    rect.value = { x0: cell.x, y0: cell.y, x1: cell.x, y1: cell.y }
    dragging = 'rect'
    return
  }
  if (tool === 'eyedropper') {
    if (inMap(cell)) emit('pick', cell)
    return
  }
  if (tool === 'object') {
    const point = eventPoint(event)
    const world = screenToWorld(camera, cssWidth.value, cssHeight.value, point.x, point.y)
    const hit = objectAt(props.map, world.x, world.y)
    if (hit) {
      beginStroke()
      emit('select-object', hit.id)
      dragging = 'object'
      dragObject = {
        id: hit.id,
        grabX: world.x / props.map.tileSize - hit.x,
        grabY: world.y / props.map.tileSize - hit.y,
      }
    } else if (!eraseClick) {
      beginStroke()
      emit('add-object', clampCell(cell))
    } else {
      emit('select-object', null)
    }
    return
  }
}

function onPointerMove(event: PointerEvent) {
  const cell = cellAt(event)
  hover.value = inMap(cell) ? cell : null
  emit('cursor', hover.value)
  if (!dragging) {
    scheduleDraw()
    return
  }
  if (dragging === 'pan') {
    camera.x -= (event.clientX - panLast.x) / camera.zoom
    camera.y -= (event.clientY - panLast.y) / camera.zoom
    panLast = { x: event.clientX, y: event.clientY }
    scheduleDraw()
    return
  }
  if (dragging === 'paint' || dragging === 'erase' || dragging === 'collide') {
    const path = lastCell ? lineCells(lastCell, cell) : inMap(cell) ? [cell] : []
    lastCell = cell
    if (path.length === 0) return
    if (dragging === 'paint') emit('paint', path)
    else if (dragging === 'erase') emit('erase', path)
    else emit('collision', path, collideMode)
    scheduleDraw()
    return
  }
  if (dragging === 'rect' && rectStart) {
    const clamped = clampCell(cell)
    rect.value = { x0: rectStart.x, y0: rectStart.y, x1: clamped.x, y1: clamped.y }
    scheduleDraw()
    return
  }
  if (dragging === 'object' && dragObject) {
    const point = eventPoint(event)
    const world = screenToWorld(camera, cssWidth.value, cssHeight.value, point.x, point.y)
    const x = Math.round((world.x / props.map.tileSize - dragObject.grabX) * 2) / 2
    const y = Math.round((world.y / props.map.tileSize - dragObject.grabY) * 2) / 2
    emit('move-object', dragObject.id, {
      x: Math.max(0, Math.min(props.map.width, x)),
      y: Math.max(0, Math.min(props.map.height, y)),
    })
    scheduleDraw()
  }
}

function onPointerUp(event: PointerEvent) {
  canvas.value?.releasePointerCapture?.(event.pointerId)
  if (dragging === 'rect' && rect.value) {
    const cells = rectCells(
      { x: rect.value.x0, y: rect.value.y0 },
      { x: rect.value.x1, y: rect.value.y1 },
    )
    if (cells.length > 0) emit('paint', cells)
    rect.value = null
    rectStart = null
  }
  dragging = null
  lastCell = null
  dragObject = null
  scheduleDraw()
}

function onPointerLeave() {
  if (!dragging) {
    hover.value = null
    emit('cursor', null)
    scheduleDraw()
  }
}

function onWheel(event: WheelEvent) {
  if (props.preview) return
  event.preventDefault()
  const point = eventPoint(event)
  const before = screenToWorld(camera, cssWidth.value, cssHeight.value, point.x, point.y)
  const index = ZOOM_STEPS.findIndex((step) => step >= camera.zoom - 0.001)
  const nextIndex = Math.max(
    0,
    Math.min(ZOOM_STEPS.length - 1, (index < 0 ? 0 : index) + (event.deltaY < 0 ? 1 : -1)),
  )
  camera.zoom = ZOOM_STEPS[nextIndex]
  const after = screenToWorld(camera, cssWidth.value, cssHeight.value, point.x, point.y)
  camera.x += before.x - after.x
  camera.y += before.y - after.y
  emit('zoom', camera.zoom)
  scheduleDraw()
}

function onKeyDown(event: KeyboardEvent) {
  if (event.code === 'Space') spaceDown = true
  if (!props.preview) return
  if (event.key === 'Escape') {
    emit('exit-preview')
    return
  }
  const key = event.key.toLowerCase()
  if (['w', 'a', 's', 'd', 'arrowup', 'arrowdown', 'arrowleft', 'arrowright'].includes(key)) {
    pressed.add(key)
    event.preventDefault()
  }
}

function onKeyUp(event: KeyboardEvent) {
  if (event.code === 'Space') spaceDown = false
  pressed.delete(event.key.toLowerCase())
}

function onBlur() {
  spaceDown = false
  pressed.clear()
}

/** The editor's view, restored when the walk preview ends. */
let restoreCamera: Camera | null = null

function startPreview() {
  const spawn = props.map.objects.find((object) => object.kind === 'spawn')
  const tile = props.map.tileSize
  restoreCamera = { x: camera.x, y: camera.y, zoom: camera.zoom }
  player.value = spawn
    ? { x: (spawn.x + spawn.w / 2) * tile, y: (spawn.y + spawn.h) * tile - 1 }
    : { x: (props.map.width / 2) * tile, y: (props.map.height / 2) * tile }
  camera.zoom = 2
  camera.x = player.value.x
  camera.y = player.value.y - tile
  emit('zoom', camera.zoom)
  lastFrame = performance.now()
  if (!moveHandle) moveHandle = requestAnimationFrame(tickMovement)
}

function stopPreview() {
  if (moveHandle) cancelAnimationFrame(moveHandle)
  moveHandle = 0
  player.value = null
  pressed.clear()
  if (restoreCamera) {
    camera.x = restoreCamera.x
    camera.y = restoreCamera.y
    camera.zoom = restoreCamera.zoom
    restoreCamera = null
    emit('zoom', camera.zoom)
  }
  scheduleDraw()
}

function tickMovement(now: number) {
  moveHandle = requestAnimationFrame(tickMovement)
  const delta = Math.min(64, now - lastFrame)
  lastFrame = now
  const current = player.value
  if (!current) return
  const speed = 0.14 * delta
  let dx = 0
  let dy = 0
  if (pressed.has('w') || pressed.has('arrowup')) dy -= speed
  if (pressed.has('s') || pressed.has('arrowdown')) dy += speed
  if (pressed.has('a') || pressed.has('arrowleft')) dx -= speed
  if (pressed.has('d') || pressed.has('arrowright')) dx += speed
  if (dx !== 0 || dy !== 0) {
    const length = Math.hypot(dx, dy)
    const nx = (dx / length) * speed
    const ny = (dy / length) * speed
    const map = props.map
    if (!solidAt(map, current.x + nx, current.y)) current.x += nx
    if (!solidAt(map, current.x, current.y + ny)) current.y += ny
  }
  camera.x += (current.x - camera.x) * 0.25
  camera.y += (current.y - props.map.tileSize - camera.y) * 0.25
  render()
}

watch(
  () => props.map.id,
  () => {
    resize()
    fitCamera()
    scheduleDraw()
  },
)
watch(() => props.map.layers, scheduleDraw, { deep: true })
watch(() => props.map.objects, scheduleDraw, { deep: true })
watch(() => props.images, scheduleDraw, { deep: false })
watch(() => props.showGrid, scheduleDraw)
watch(() => props.selectedObjectId, scheduleDraw)
watch(
  () => props.preview,
  (on) => {
    if (on) startPreview()
    else stopPreview()
  },
)

onMounted(() => {
  resize()
  fitCamera()
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(() => {
      const previous = camera.zoom
      resize()
      if (previous === 1) fitCamera()
      scheduleDraw()
    })
    if (wrapper.value) observer.observe(wrapper.value)
  }
  window.addEventListener('keydown', onKeyDown)
  window.addEventListener('keyup', onKeyUp)
  window.addEventListener('blur', onBlur)
})

onBeforeUnmount(() => {
  observer?.disconnect()
  if (drawHandle) cancelAnimationFrame(drawHandle)
  if (moveHandle) cancelAnimationFrame(moveHandle)
  window.removeEventListener('keydown', onKeyDown)
  window.removeEventListener('keyup', onKeyUp)
  window.removeEventListener('blur', onBlur)
})

defineExpose({
  fit: () => {
    fitCamera()
    scheduleDraw()
  },
  zoomOut: () => {
    const index = ZOOM_STEPS.findIndex((step) => step >= camera.zoom - 0.001)
    camera.zoom = ZOOM_STEPS[Math.max(0, (index < 0 ? 0 : index) - 1)]
    emit('zoom', camera.zoom)
    scheduleDraw()
  },
  zoomIn: () => {
    const index = ZOOM_STEPS.findIndex((step) => step >= camera.zoom - 0.001)
    camera.zoom = ZOOM_STEPS[Math.min(ZOOM_STEPS.length - 1, (index < 0 ? 0 : index) + 1)]
    emit('zoom', camera.zoom)
    scheduleDraw()
  },
})
</script>

<template>
  <div ref="wrapper" class="relative h-full w-full overflow-hidden">
    <canvas
      ref="canvas"
      class="block h-full w-full cursor-crosshair touch-none"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointerleave="onPointerLeave"
      @wheel="onWheel"
      @contextmenu.prevent
    />
    <div
      v-if="preview"
      class="pointer-events-none absolute bottom-3 left-1/2 -translate-x-1/2 rounded-lg border border-line bg-panel/85 px-3 py-1.5 text-[11px] text-muted backdrop-blur"
    >
      WASD / 方向键行走 · Esc 退出预览
    </div>
  </div>
</template>