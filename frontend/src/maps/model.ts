import type {
  Asset,
  AutotileMode,
  GameMap,
  MapLayer,
  MapLayerType,
  MapObject,
  MapObjectKind,
  MapTile,
  MapTileset,
} from '../domain/types'
import { newID, nowISO } from '../lib/utils'

/** Tools the map canvas offers; kept apart from layer types. */
export type MapTool = 'brush' | 'eraser' | 'rect' | 'eyedropper' | 'collision' | 'object'

export interface Cell {
  x: number
  y: number
}

export const TILE_SIZE_OPTIONS = [16, 24, 32, 48] as const

export const LAYER_PRESETS: ReadonlyArray<{ type: MapLayerType; name: string }> = [
  { type: 'ground', name: '地面' },
  { type: 'decoration', name: '装饰' },
  { type: 'building', name: '建筑' },
  { type: 'roof', name: '屋顶' },
  { type: 'object', name: '对象' },
  { type: 'collision', name: '碰撞' },
  { type: 'light', name: '光照' },
]

export const OBJECT_KIND_LABELS: Record<MapObjectKind, string> = {
  spawn: '出生点',
  npc: 'NPC',
  chest: '宝箱',
  trigger: '触发器',
  building: '建筑',
  prop: '道具',
}

export const OBJECT_KIND_COLORS: Record<MapObjectKind, string> = {
  spawn: '#6ec6e8',
  npc: '#f2b84b',
  chest: '#d98f4a',
  trigger: '#8ccf6f',
  building: '#b85c38',
  prop: '#c94f4f',
}

export function tileIndex(layer: MapLayer): Map<string, MapTile> {
  const index = new Map<string, MapTile>()
  for (const tile of layer.tiles) index.set(`${tile.x},${tile.y}`, tile)
  return index
}

export function findLayer(map: GameMap, id: string): MapLayer | undefined {
  return map.layers.find((layer) => layer.id === id)
}


/* ------------------------------------------------------------- autotiling */

const CORNERS = [
  { bit: 1, a: 1, b: 2 }, // north-east: N + E
  { bit: 2, a: 2, b: 4 }, // south-east: E + S
  { bit: 4, a: 4, b: 8 }, // south-west: S + W
  { bit: 8, a: 8, b: 1 }, // north-west: W + N
]

/** Number of filled inner corners for a cardinal mask (N=1,E=2,S=4,W=8). */
export function cornerCount(mask: number): number {
  return CORNERS.reduce((count, corner) => {
    return count + ((mask & corner.a) !== 0 && (mask & corner.b) !== 0 ? 1 : 0)
  }, 0)
}

export function autotileCount(mode: AutotileMode): number {
  return mode === '16' ? 16 : 47
}

/**
 * Variant index for a cardinal mask. 16-mode maps the mask straight to a
 * variant. 47-mode adds the inner-corner combinations, laid out mask by mask
 * (mask order 0..15, corners NE, SE, SW, NW); `corners` marks which diagonal
 * neighbours belong to the same terrain. A 47-tile atlas must follow that
 * order; atlases with an arbitrary layout should use 16-mode.
 */
export function autotileVariant(mask: number, mode: AutotileMode, corners = 0): number {
  if (mode === '16') return mask
  let offset = 0
  for (let m = 0; m < mask; m += 1) offset += 1 << cornerCount(m)
  const enabled = CORNERS.filter(
    (corner) => (mask & corner.a) !== 0 && (mask & corner.b) !== 0,
  )
  let sub = 0
  enabled.forEach((corner, i) => {
    if ((corners & corner.bit) !== 0) sub |= 1 << i
  })
  return offset + sub
}

export function autotileFamily(layer: MapLayer): { set: number; base: number; count: number } | null {
  const cfg = layer.autotile
  if (!cfg) return null
  return { set: cfg.set, base: cfg.base, count: autotileCount(cfg.mode) }
}

export function isFamilyTile(
  tile: MapTile | undefined,
  family: { set: number; base: number; count: number } | null,
): boolean {
  if (!tile || !family) return false
  return (
    tile.set === family.set && tile.tile >= family.base && tile.tile < family.base + family.count
  )
}

/** Cardinal mask for one cell, treating outside-the-map neighbours as the same terrain. */
export function cardinalMask(
  map: Pick<GameMap, 'width' | 'height'>,
  layer: MapLayer,
  index: Map<string, MapTile>,
  x: number,
  y: number,
): number {
  const family = autotileFamily(layer)
  const same = (nx: number, ny: number): boolean => {
    if (nx < 0 || ny < 0 || nx >= map.width || ny >= map.height) return true
    return isFamilyTile(index.get(`${nx},${ny}`), family)
  }
  let mask = 0
  if (same(x, y - 1)) mask |= 1
  if (same(x + 1, y)) mask |= 2
  if (same(x, y + 1)) mask |= 4
  if (same(x - 1, y)) mask |= 8
  return mask
}

/** Diagonal occupancy bits (NE=1, SE=2, SW=4, NW=8) for the 47-tile variants. */
export function diagonalMask(
  map: Pick<GameMap, 'width' | 'height'>,
  layer: MapLayer,
  index: Map<string, MapTile>,
  x: number,
  y: number,
): number {
  const family = autotileFamily(layer)
  const same = (nx: number, ny: number): boolean => {
    if (nx < 0 || ny < 0 || nx >= map.width || ny >= map.height) return true
    return isFamilyTile(index.get(`${nx},${ny}`), family)
  }
  let mask = 0
  if (same(x + 1, y - 1)) mask |= 1
  if (same(x + 1, y + 1)) mask |= 2
  if (same(x - 1, y + 1)) mask |= 4
  if (same(x - 1, y - 1)) mask |= 8
  return mask
}

/**
 * Repaints every tile of the layer's autotile family from its neighbours.
 * Call after each paint so borders follow the shape being drawn.
 */
export function repaintAutotile(layer: MapLayer, map: Pick<GameMap, 'width' | 'height'>): MapLayer {
  const cfg = layer.autotile
  if (!cfg) return layer
  const index = tileIndex(layer)
  const family = autotileFamily(layer)
  const tiles = layer.tiles.map((tile) => {
    if (!isFamilyTile(tile, family)) return tile
    const mask = cardinalMask(map, layer, index, tile.x, tile.y)
    const corners = cfg.mode === '47' ? diagonalMask(map, layer, index, tile.x, tile.y) : 0
    return { ...tile, set: cfg.set, tile: cfg.base + autotileVariant(mask, cfg.mode, corners) }
  })
  return { ...layer, tiles }
}

/* -------------------------------------------------------------- painting */

function replaceLayer(map: GameMap, layerId: string, next: MapLayer): GameMap {
  return { ...map, layers: map.layers.map((layer) => (layer.id === layerId ? next : layer)) }
}

/** Paints (or overwrites) cells on one layer, then refreshes autotile borders. */
export function paintCells(
  map: GameMap,
  layerId: string,
  cells: Cell[],
  set: number,
  tile: number,
  autotile = true,
): GameMap {
  const layer = findLayer(map, layerId)
  if (!layer) return map
  const index = tileIndex(layer)
  for (const cell of cells) {
    if (cell.x < 0 || cell.y < 0 || cell.x >= map.width || cell.y >= map.height) continue
    index.set(`${cell.x},${cell.y}`, { x: cell.x, y: cell.y, set, tile })
  }
  let next: MapLayer = {
    ...layer,
    tiles: [...index.values()].sort((a, b) => a.y - b.y || a.x - b.x),
  }
  if (autotile && layer.autotile) next = repaintAutotile(next, map)
  return replaceLayer(map, layerId, next)
}

export function eraseCells(map: GameMap, layerId: string, cells: Cell[]): GameMap {
  const layer = findLayer(map, layerId)
  if (!layer) return map
  const doomed = new Set(cells.map((cell) => `${cell.x},${cell.y}`))
  let next: MapLayer = {
    ...layer,
    tiles: layer.tiles.filter((tile) => !doomed.has(`${tile.x},${tile.y}`)),
  }
  if (layer.autotile) next = repaintAutotile(next, map)
  return replaceLayer(map, layerId, next)
}

/** Adds or removes collision cells; the collision layer stores zero tiles. */
export function setCollision(map: GameMap, cells: Cell[], solid: boolean): GameMap {
  const layer = findLayer(map, 'collision') ?? map.layers.find((item) => item.type === 'collision')
  if (!layer) return map
  const index = tileIndex(layer)
  for (const cell of cells) {
    if (cell.x < 0 || cell.y < 0 || cell.x >= map.width || cell.y >= map.height) continue
    if (solid) index.set(`${cell.x},${cell.y}`, { x: cell.x, y: cell.y, set: 0, tile: 0 })
    else index.delete(`${cell.x},${cell.y}`)
  }
  const next: MapLayer = {
    ...layer,
    tiles: [...index.values()].sort((a, b) => a.y - b.y || a.x - b.x),
  }
  return replaceLayer(map, layer.id, next)
}

export function collisionAt(map: GameMap, x: number, y: number): boolean {
  const layer = map.layers.find((item) => item.type === 'collision')
  if (!layer) return false
  return layer.tiles.some((tile) => tile.x === x && tile.y === y)
}

/** Tile under the cursor on a layer, for the eyedropper. */
export function tileAt(layer: MapLayer, x: number, y: number): MapTile | null {
  return layer.tiles.find((tile) => tile.x === x && tile.y === y) ?? null
}

/* --------------------------------------------------------------- objects */

export function objectDepth(object: MapObject, map: GameMap): number {
  return (object.y + object.h) * map.tileSize + object.elevation * map.settings.heightStep
}

export function createObject(
  kind: MapObjectKind,
  cellX: number,
  cellY: number,
  index = 0,
): MapObject {
  const color = OBJECT_KIND_COLORS[kind]
  return {
    id: newID(),
    name: `${OBJECT_KIND_LABELS[kind]}${index > 0 ? ` ${index + 1}` : ''}`,
    kind,
    x: cellX,
    y: cellY,
    w: 1,
    h: 1,
    elevation: 0,
    ySort: kind !== 'trigger',
    roofFade: false,
    solid: kind === 'npc' || kind === 'chest' || kind === 'building',
    color,
    properties: {},
  }
}

export function addObject(map: GameMap, object: MapObject): GameMap {
  return { ...map, objects: [...map.objects, object] }
}

export function updateObject(map: GameMap, object: MapObject): GameMap {
  return {
    ...map,
    objects: map.objects.map((item) => (item.id === object.id ? object : item)),
  }
}

export function removeObject(map: GameMap, id: string): GameMap {
  return { ...map, objects: map.objects.filter((item) => item.id !== id) }
}

/** Topmost object containing a world-pixel point. */
export function objectAt(map: GameMap, px: number, py: number): MapObject | null {
  for (let i = map.objects.length - 1; i >= 0; i -= 1) {
    const object = map.objects[i]
    const left = object.x * map.tileSize
    const top = object.y * map.tileSize
    if (px >= left && py >= top && px <= left + object.w * map.tileSize && py <= top + object.h * map.tileSize) {
      return object
    }
  }
  return null
}

/** Blocks movement when a collision cell or a solid object covers a world point. */
export function solidAt(map: GameMap, px: number, py: number): boolean {
  if (px < 0 || py < 0 || px >= map.width * map.tileSize || py >= map.height * map.tileSize) {
    return true
  }
  const cx = Math.floor(px / map.tileSize)
  const cy = Math.floor(py / map.tileSize)
  if (collisionAt(map, cx, cy)) return true
  return map.objects.some((object) => {
    if (!object.solid) return false
    const left = object.x * map.tileSize
    const top = object.y * map.tileSize
    return px >= left && py >= top && px <= left + object.w * map.tileSize && py <= top + object.h * map.tileSize
  })
}

/* ------------------------------------------------------------ map create */

function makeLayer(type: MapLayerType, name: string): MapLayer {
  return {
    id: type === 'object' ? 'objects' : type,
    name,
    type,
    elevation: 0,
    visible: true,
    opacity: type === 'collision' ? 0.5 : 1,
    tiles: [],
  }
}

export function defaultLayers(): MapLayer[] {
  return (['ground', 'decoration', 'building', 'roof', 'object', 'collision'] as MapLayerType[]).map(
    (type) => makeLayer(type, LAYER_PRESETS.find((preset) => preset.type === type)?.name ?? type),
  )
}

export function tilesetFromAsset(asset: Asset, mapTileSize = 32): MapTileset | null {
  if (asset.type !== 'tile' || asset.dimensions.w < mapTileSize || asset.dimensions.h < mapTileSize) {
    return null
  }
  const columns = Math.floor(asset.dimensions.w / mapTileSize)
  const rows = Math.floor(asset.dimensions.h / mapTileSize)
  if (columns < 1 || rows < 1) return null
  return {
    id: `ts-${asset.id.replace(/[^A-Za-z0-9]/g, '').slice(0, 10) || 'atlas'}`,
    assetId: asset.id,
    tileSize: mapTileSize,
    columns,
    tileCount: columns * rows,
  }
}

export interface NewMapOptions {
  name: string
  width: number
  height: number
  tileSize: number
  tilesets: MapTileset[]
}

export function createMap(options: NewMapOptions): GameMap {
  const now = nowISO()
  return {
    schemaVersion: 1,
    id: newID(),
    name: options.name.trim() || '未命名地图',
    tileSize: options.tileSize,
    width: Math.max(1, Math.min(1024, Math.round(options.width))),
    height: Math.max(1, Math.min(1024, Math.round(options.height))),
    tilesets: options.tilesets,
    layers: defaultLayers(),
    objects: [],
    settings: {
      ySort: true,
      roofFade: true,
      heightStep: Math.round(options.tileSize / 2) || 16,
      ambient: '#8090a0',
    },
    createdAt: now,
    updatedAt: now,
  }
}

