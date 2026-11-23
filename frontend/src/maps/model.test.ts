import { describe, expect, it } from 'vitest'
import type { GameMap, MapLayer } from '../domain/types'
import {
  autotileCount,
  autotileVariant,
  cardinalMask,
  collisionAt,
  createMap,
  createObject,
  objectAt,
  objectDepth,
  paintCells,
  repaintAutotile,
  setCollision,
  solidAt,
  tileIndex,
} from './model'

function makeMap(width = 8, height = 8): GameMap {
  const map = createMap({
    name: '测试地图',
    width,
    height,
    tileSize: 32,
    tilesets: [{ id: 'ts', assetId: 'atlas', tileSize: 32, columns: 4, tileCount: 16 }],
  })
  return map
}

function ground(map: GameMap, autotile?: MapLayer['autotile']): MapLayer {
  const layer = map.layers.find((item) => item.type === 'ground')!
  return autotile ? { ...layer, autotile } : layer
}

function withAutotile(map: GameMap, autotile: MapLayer['autotile']): GameMap {
  return {
    ...map,
    layers: map.layers.map((layer) => (layer.type === 'ground' ? { ...layer, autotile } : layer)),
  }
}

describe('map autotiling', () => {
  it('maps the cardinal mask straight to a 16-tile variant', () => {
    expect(autotileCount('16')).toBe(16)
    for (let mask = 0; mask < 16; mask += 1) {
      expect(autotileVariant(mask, '16')).toBe(mask)
    }
  })

  it('lays out exactly 47 variants for the inner-corner mode', () => {
    expect(autotileCount('47')).toBe(47)
    const seen = new Set<number>()
    for (let mask = 0; mask < 16; mask += 1) {
      // Every combination of inner corners allowed by this mask.
      const corners: number[] = []
      const pairs = [
        [1, 2, 1],
        [2, 4, 2],
        [4, 8, 4],
        [8, 1, 8],
      ] as const
      for (const [a, b, bit] of pairs) {
        if ((mask & a) !== 0 && (mask & b) !== 0) corners.push(bit)
      }
      for (let sub = 0; sub < 1 << corners.length; sub += 1) {
        let cornerMask = 0
        corners.forEach((bit, i) => {
          if ((sub & (1 << i)) !== 0) cornerMask |= bit
        })
        seen.add(autotileVariant(mask, '47', cornerMask))
      }
    }
    expect(seen.size).toBe(47)
    expect(Math.min(...seen)).toBe(0)
    expect(Math.max(...seen)).toBe(46)
  })

  it('computes cardinal masks from same-family neighbours only', () => {
    const map = makeMap(5, 5)
    const layer = ground(map, { set: 0, base: 0, mode: '16' })
    const index = tileIndex(layer)
    index.set('2,2', { x: 2, y: 2, set: 0, tile: 0 })
    index.set('3,2', { x: 3, y: 2, set: 0, tile: 5 })
    index.set('2,3', { x: 2, y: 3, set: 0, tile: 9 })
    expect(cardinalMask(map, layer, index, 2, 2)).toBe(2 | 4)
    expect(cardinalMask(map, layer, index, 3, 2)).toBe(8)
  })

  it('repaints borders after painting and keeps outside-the-map cells open', () => {
    const map = withAutotile(makeMap(5, 5), { set: 0, base: 0, mode: '16' })
    const layer = ground(map)
    const painted = paintCells(map, layer.id, [{ x: 1, y: 1 }, { x: 2, y: 1 }], 0, 0, true)
    const index = tileIndex(ground(painted))
    expect(index.get('1,1')?.tile).toBe(2)
    expect(index.get('2,1')?.tile).toBe(8)
    // A patch that touches the map edge counts the outside as the same terrain,
    // so the border keeps following the painted shape.
    const edge = paintCells(map, layer.id, [{ x: 0, y: 0 }], 0, 0, true)
    const edgeTile = tileIndex(ground(edge)).get('0,0')
    expect(edgeTile?.tile).toBe(1 | 8)
  })

  it('leaves non-family tiles untouched when repainting', () => {
    const map = makeMap(4, 4)
    const layer = ground(map, { set: 0, base: 0, mode: '16' })
    const mixed: MapLayer = {
      ...layer,
      tiles: [
        { x: 0, y: 0, set: 0, tile: 0 },
        { x: 1, y: 1, set: 0, tile: 99 },
      ],
    }
    const repainted = repaintAutotile(mixed, map)
    expect(repainted.tiles.find((tile) => tile.x === 1 && tile.y === 1)?.tile).toBe(99)
  })
})

describe('map painting and collision', () => {
  it('overwrites a cell instead of duplicating it and sorts the result', () => {
    let map = makeMap(4, 4)
    const layerId = ground(map).id
    map = paintCells(map, layerId, [{ x: 2, y: 1 }, { x: 0, y: 3 }], 0, 1)
    map = paintCells(map, layerId, [{ x: 2, y: 1 }], 0, 7)
    const tiles = ground(map).tiles
    expect(tiles).toHaveLength(2)
    expect(tiles.find((tile) => tile.x === 2 && tile.y === 1)?.tile).toBe(7)
    expect(tiles[0]).toMatchObject({ x: 2, y: 1 })
    expect(tiles[1]).toMatchObject({ x: 0, y: 3 })
  })

  it('ignores cells outside the grid', () => {
    const map = makeMap(3, 3)
    const next = paintCells(map, ground(map).id, [{ x: -1, y: 0 }, { x: 3, y: 3 }], 0, 1)
    expect(ground(next).tiles).toHaveLength(0)
  })

  it('adds and removes collision cells, and blocks solid objects', () => {
    let map = makeMap(4, 4)
    map = setCollision(map, [{ x: 1, y: 1 }], true)
    expect(collisionAt(map, 1, 1)).toBe(true)
    expect(solidAt(map, 1.5 * 32, 1.5 * 32)).toBe(true)
    expect(solidAt(map, -1, 0)).toBe(true)

    map = setCollision(map, [{ x: 1, y: 1 }], false)
    expect(collisionAt(map, 1, 1)).toBe(false)
    expect(solidAt(map, 1.5 * 32, 1.5 * 32)).toBe(false)

    const chest = { ...createObject('chest', 2, 2), solid: true }
    map = { ...map, objects: [chest] }
    expect(solidAt(map, 2.5 * 32, 2.5 * 32)).toBe(true)
    expect(objectAt(map, 2.5 * 32, 2.5 * 32)?.id).toBe(chest.id)
    expect(objectAt(map, 0.5 * 32, 0.5 * 32)).toBeNull()
  })
})

describe('map objects', () => {
  it('sorts depth by the bottom edge plus elevation', () => {
    const map = makeMap()
    const object = { ...createObject('npc', 2, 3), w: 1, h: 2, elevation: 2 }
    expect(objectDepth(object, map)).toBe((3 + 2) * 32 + 2 * map.settings.heightStep)
  })

  it('creates kind-specific defaults', () => {
    expect(createObject('spawn', 1, 1)).toMatchObject({ ySort: true, solid: false })
    expect(createObject('npc', 1, 1)).toMatchObject({ solid: true })
    expect(createObject('trigger', 1, 1)).toMatchObject({ ySort: false, solid: false })
  })
})