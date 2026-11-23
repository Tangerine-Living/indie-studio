import type { GameMap, MapLayer, MapObject } from '../domain/types'
import { OBJECT_KIND_LABELS, objectDepth } from './model'

export interface Camera {
  /** World-pixel coordinate at the centre of the viewport. */
  x: number
  y: number
  zoom: number
}

export interface ScreenRect {
  x0: number
  y0: number
  x1: number
  y1: number
}

export interface RenderState {
  map: GameMap
  camera: Camera
  width: number
  height: number
  images: ReadonlyMap<string, CanvasImageSource>
  grid: boolean
  hover: { x: number; y: number } | null
  rect: ScreenRect | null
  selectedObjectId: string | null
  player: { x: number; y: number } | null
  preview: boolean
}

const SELECT_COLOR = '#6ec6e8'
const COLLISION_COLOR = 'rgba(214, 84, 84, 0.45)'

export function screenToWorld(camera: Camera, width: number, height: number, sx: number, sy: number) {
  return {
    x: (sx - width / 2) / camera.zoom + camera.x,
    y: (sy - height / 2) / camera.zoom + camera.y,
  }
}

export function worldToCell(map: GameMap, worldX: number, worldY: number) {
  return { x: Math.floor(worldX / map.tileSize), y: Math.floor(worldY / map.tileSize) }
}

function drawTileLayer(
  ctx: CanvasRenderingContext2D,
  state: RenderState,
  layer: MapLayer,
  originX: number,
  originY: number,
): void {
  const { map, camera, images } = state
  if (!layer.visible) return
  const zoom = camera.zoom
  const heightStep = map.settings.heightStep
  const elevation = layer.elevation * heightStep

  const left = (0 - originX) / zoom
  const top = (0 - originY) / zoom + elevation
  const right = (state.width - originX) / zoom
  const bottom = (state.height - originY) / zoom + elevation
  const minX = Math.max(0, Math.floor(left / map.tileSize) - 1)
  const maxX = Math.min(map.width - 1, Math.ceil(right / map.tileSize) + 1)
  const minY = Math.max(0, Math.floor(top / map.tileSize) - 1)
  const maxY = Math.min(map.height - 1, Math.ceil(bottom / map.tileSize) + 1)

  ctx.globalAlpha = layer.opacity
  ctx.imageSmoothingEnabled = false
  for (const tile of layer.tiles) {
    if (tile.x < minX || tile.x > maxX || tile.y < minY || tile.y > maxY) continue
    const set = map.tilesets[tile.set]
    if (!set) continue
    const image = images.get(set.assetId)
    if (!image) continue
    const col = tile.tile % set.columns
    const row = Math.floor(tile.tile / set.columns)
    const size = map.tileSize * zoom
    const dx = originX + tile.x * map.tileSize * zoom
    const dy = originY + (tile.y * map.tileSize - elevation) * zoom
    const rotate = ((tile.rotate ?? 0) * Math.PI) / 180
    const flipH = tile.flipH ?? false
    const flipV = tile.flipV ?? false
    if (rotate === 0 && !flipH && !flipV) {
      ctx.drawImage(image, col * set.tileSize, row * set.tileSize, set.tileSize, set.tileSize, dx, dy, size, size)
      continue
    }
    ctx.save()
    ctx.translate(dx + size / 2, dy + size / 2)
    ctx.rotate(rotate)
    ctx.scale(flipH ? -1 : 1, flipV ? -1 : 1)
    ctx.drawImage(
      image,
      col * set.tileSize,
      row * set.tileSize,
      set.tileSize,
      set.tileSize,
      -size / 2,
      -size / 2,
      size,
      size,
    )
    ctx.restore()
  }
  ctx.globalAlpha = 1
}

/**
 * Light layers are painted with atlas tiles too, but shown as an additive glow
 * on top of the scene. `screen` keeps the map readable while overlapping light
 * tiles brighten each other; hiding the layer removes the effect immediately.
 */
function drawLightLayers(
  ctx: CanvasRenderingContext2D,
  state: RenderState,
  originX: number,
  originY: number,
): void {
  const { map, camera, images } = state
  const zoom = camera.zoom
  const heightStep = map.settings.heightStep
  const left = (0 - originX) / zoom
  const right = (state.width - originX) / zoom

  for (const layer of map.layers) {
    if (layer.type !== 'light' || !layer.visible || layer.opacity <= 0) continue
    const elevation = layer.elevation * heightStep
    const top = (0 - originY) / zoom + elevation
    const bottom = (state.height - originY) / zoom + elevation
    const minX = Math.max(0, Math.floor(left / map.tileSize) - 1)
    const maxX = Math.min(map.width - 1, Math.ceil(right / map.tileSize) + 1)
    const minY = Math.max(0, Math.floor(top / map.tileSize) - 1)
    const maxY = Math.min(map.height - 1, Math.ceil(bottom / map.tileSize) + 1)
    ctx.save()
    ctx.globalCompositeOperation = 'screen'
    ctx.globalAlpha = layer.opacity
    ctx.imageSmoothingEnabled = false
    for (const tile of layer.tiles) {
      if (tile.x < minX || tile.x > maxX || tile.y < minY || tile.y > maxY) continue
      const dx = originX + tile.x * map.tileSize * zoom
      const dy = originY + (tile.y * map.tileSize - elevation) * zoom
      const size = map.tileSize * zoom
      const set = map.tilesets[tile.set]
      const image = set ? images.get(set.assetId) : null
      if (!set || !image) continue
      const col = tile.tile % set.columns
      const row = Math.floor(tile.tile / set.columns)
      ctx.drawImage(
        image,
        col * set.tileSize,
        row * set.tileSize,
        set.tileSize,
        set.tileSize,
        dx,
        dy,
        size,
        size,
      )
    }
    ctx.restore()
  }
}

function drawCollision(
  ctx: CanvasRenderingContext2D,
  state: RenderState,
  originX: number,
  originY: number,
): void {
  const layer = state.map.layers.find((item) => item.type === 'collision')
  if (!layer || !layer.visible) return
  const size = state.map.tileSize * state.camera.zoom
  ctx.fillStyle = COLLISION_COLOR
  for (const tile of layer.tiles) {
    ctx.fillRect(
      originX + tile.x * state.map.tileSize * state.camera.zoom,
      originY + tile.y * state.map.tileSize * state.camera.zoom,
      size,
      size,
    )
  }
}

function drawObject(
  ctx: CanvasRenderingContext2D,
  state: RenderState,
  object: MapObject,
  originX: number,
  originY: number,
  selected: boolean,
): void {
  const { map, camera } = state
  const zoom = camera.zoom
  const color = object.color || '#8b99ab'
  const elevation = object.elevation * map.settings.heightStep
  const x = originX + object.x * map.tileSize * zoom
  const y = originY + (object.y * map.tileSize - elevation) * zoom
  const w = object.w * map.tileSize * zoom
  const h = object.h * map.tileSize * zoom
  ctx.fillStyle = color
  ctx.globalAlpha = selected ? 0.5 : 0.34
  ctx.fillRect(x, y, w, h)
  ctx.globalAlpha = 1
  ctx.strokeStyle = selected ? SELECT_COLOR : color
  ctx.lineWidth = selected ? 2 : 1
  if (object.solid) ctx.setLineDash([4, 3])
  else ctx.setLineDash([])
  ctx.strokeRect(x + 0.5, y + 0.5, Math.max(1, w - 1), Math.max(1, h - 1))
  ctx.setLineDash([])
  if (zoom >= 1) {
    ctx.fillStyle = selected ? SELECT_COLOR : '#d5dde8'
    ctx.font = '11px ui-sans-serif, system-ui, sans-serif'
    ctx.textBaseline = 'bottom'
    const label = `${OBJECT_KIND_LABELS[object.kind]} · ${object.name}`
    ctx.fillText(label, x + 2, y - 2, Math.max(w, 60))
  }
}

function drawPlayer(
  ctx: CanvasRenderingContext2D,
  state: RenderState,
  originX: number,
  originY: number,
): void {
  if (!state.player) return
  const zoom = state.camera.zoom
  const size = state.map.tileSize * zoom
  const x = originX + state.player.x * zoom
  const y = originY + state.player.y * zoom
  ctx.fillStyle = 'rgba(110, 198, 232, 0.35)'
  ctx.beginPath()
  ctx.arc(x, y, size * 0.34, 0, Math.PI * 2)
  ctx.fill()
  ctx.strokeStyle = '#6ec6e8'
  ctx.lineWidth = 2
  ctx.stroke()
  ctx.fillStyle = '#e8f6ff'
  ctx.beginPath()
  ctx.arc(x, y - size * 0.18, size * 0.12, 0, Math.PI * 2)
  ctx.fill()
}

function roofShouldFade(state: RenderState): boolean {
  if (!state.preview || !state.player || !state.map.settings.roofFade) return false
  const cellX = Math.floor(state.player.x / state.map.tileSize)
  const cellY = Math.floor(state.player.y / state.map.tileSize)
  const roof = state.map.layers.find((layer) => layer.type === 'roof')
  if (!roof) return false
  return roof.tiles.some(
    (tile) => Math.abs(tile.x - cellX) <= 4 && Math.abs(tile.y - cellY) <= 4,
  )
}

export function drawMap(ctx: CanvasRenderingContext2D, state: RenderState): void {
  const { map, camera, width, height } = state
  const zoom = camera.zoom
  const originX = width / 2 - camera.x * zoom
  const originY = height / 2 - camera.y * zoom
  const worldW = map.width * map.tileSize
  const worldH = map.height * map.tileSize

  ctx.clearRect(0, 0, width, height)
  ctx.fillStyle = '#0b1118'
  ctx.fillRect(0, 0, width, height)

  // Map plate, so the empty area outside the grid reads as out-of-bounds.
  ctx.fillStyle = '#111a24'
  ctx.fillRect(originX, originY, worldW * zoom, worldH * zoom)

  const roof = map.layers.find((layer) => layer.type === 'roof')
  for (const layer of map.layers) {
    if (layer.type === 'collision' || layer.type === 'roof' || layer.type === 'light') continue
    if (layer.type === 'object') continue
    drawTileLayer(ctx, state, layer, originX, originY)
  }

  const objectLayer = map.layers.find((layer) => layer.type === 'object')
  if (objectLayer?.visible !== false) {
    const entries: Array<{ depth: number; object: MapObject }> = map.objects.map((object) => ({
      depth: objectDepth(object, map),
      object,
    }))
    entries.sort((a, b) => a.depth - b.depth)
    for (const entry of entries) {
      drawObject(ctx, state, entry.object, originX, originY, entry.object.id === state.selectedObjectId)
    }
  }

  drawPlayer(ctx, state, originX, originY)

  if (roof) {
    const faded = roofShouldFade(state)
    const opacity = faded ? 0.32 : roof.opacity
    const offset = roof.elevation * map.settings.heightStep
    ctx.globalAlpha = opacity
    if (roof.visible) {
      for (const tile of roof.tiles) {
        const set = map.tilesets[tile.set]
        const image = set ? state.images.get(set.assetId) : null
        if (!set || !image) continue
        const col = tile.tile % set.columns
        const row = Math.floor(tile.tile / set.columns)
        const size = map.tileSize * zoom
        ctx.drawImage(
          image,
          col * set.tileSize,
          row * set.tileSize,
          set.tileSize,
          set.tileSize,
          originX + tile.x * map.tileSize * zoom,
          originY + (tile.y * map.tileSize - offset) * zoom,
          size,
          size,
        )
      }
    }
    ctx.globalAlpha = 1
  }

  drawLightLayers(ctx, state, originX, originY)
  drawCollision(ctx, state, originX, originY)

  if (state.grid && zoom >= 1) {
    const size = map.tileSize * zoom
    ctx.strokeStyle = 'rgba(160, 190, 220, 0.10)'
    ctx.lineWidth = 1
    ctx.beginPath()
    const startX = Math.max(0, Math.floor((0 - originX) / size))
    const endX = Math.min(map.width, Math.ceil((width - originX) / size))
    const startY = Math.max(0, Math.floor((0 - originY) / size))
    const endY = Math.min(map.height, Math.ceil((height - originY) / size))
    for (let x = startX; x <= endX; x += 1) {
      const sx = Math.round(originX + x * size) + 0.5
      ctx.moveTo(sx, Math.max(0, originY))
      ctx.lineTo(sx, Math.min(height, originY + worldH * zoom))
    }
    for (let y = startY; y <= endY; y += 1) {
      const sy = Math.round(originY + y * size) + 0.5
      ctx.moveTo(Math.max(0, originX), sy)
      ctx.lineTo(Math.min(width, originX + worldW * zoom), sy)
    }
    ctx.stroke()
  }

  if (state.hover) {
    const size = map.tileSize * zoom
    ctx.strokeStyle = 'rgba(110, 198, 232, 0.9)'
    ctx.lineWidth = 1.5
    ctx.strokeRect(
      originX + state.hover.x * size + 0.5,
      originY + state.hover.y * size + 0.5,
      size - 1,
      size - 1,
    )
  }

  if (state.rect) {
    const size = map.tileSize * zoom
    const x = Math.min(state.rect.x0, state.rect.x1)
    const y = Math.min(state.rect.y0, state.rect.y1)
    const w = (Math.abs(state.rect.x1 - state.rect.x0) + 1) * size
    const h = (Math.abs(state.rect.y1 - state.rect.y0) + 1) * size
    ctx.fillStyle = 'rgba(110, 198, 232, 0.18)'
    ctx.fillRect(originX + x * size, originY + y * size, w, h)
    ctx.strokeStyle = SELECT_COLOR
    ctx.lineWidth = 1.5
    ctx.strokeRect(originX + x * size + 0.5, originY + y * size + 0.5, w - 1, h - 1)
  }

  ctx.strokeStyle = 'rgba(110, 198, 232, 0.35)'
  ctx.lineWidth = 1
  ctx.strokeRect(originX + 0.5, originY + 0.5, worldW * zoom - 1, worldH * zoom - 1)
}