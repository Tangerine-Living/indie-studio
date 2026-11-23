import { describe, expect, it } from 'vitest'
import type { GameMap } from '../domain/types'
import { drawMap } from './render'
import type { RenderState } from './render'

interface DrawCall {
  composite: string
  alpha: number
}

function makeContext() {
  const calls: DrawCall[] = []
  const stack: string[] = []
  const ctx = {
    globalAlpha: 1,
    globalCompositeOperation: 'source-over' as GlobalCompositeOperation,
    imageSmoothingEnabled: true,
    fillStyle: '',
    strokeStyle: '',
    lineWidth: 1,
    font: '',
    textBaseline: 'alphabetic',
    drawImage() {
      calls.push({
        composite: String(ctx.globalCompositeOperation),
        alpha: ctx.globalAlpha,
      })
    },
    clearRect() {},
    fillRect() {},
    beginPath() {},
    arc() {},
    fill() {},
    stroke() {},
    strokeRect() {},
    moveTo() {},
    lineTo() {},
    setLineDash() {},
    fillText() {},
    save() {
      stack.push(String(ctx.globalCompositeOperation))
    },
    restore() {
      ctx.globalCompositeOperation = (stack.pop() ?? 'source-over') as GlobalCompositeOperation
    },
  }
  return { ctx: ctx as unknown as CanvasRenderingContext2D, calls }
}

function makeState(lightVisible: boolean): RenderState {
  const map: GameMap = {
    schemaVersion: 1,
    id: 'm1',
    name: '测试',
    tileSize: 16,
    width: 4,
    height: 4,
    tilesets: [{ id: 't', assetId: 'a', tileSize: 16, columns: 2, tileCount: 4 }],
    layers: [
      {
        id: 'ground',
        name: '地面',
        type: 'ground',
        elevation: 0,
        visible: true,
        opacity: 1,
        tiles: [{ x: 0, y: 0, set: 0, tile: 0 }],
      },
      {
        id: 'light',
        name: '光照',
        type: 'light',
        elevation: 0,
        visible: lightVisible,
        opacity: 0.5,
        tiles: [{ x: 0, y: 0, set: 0, tile: 1 }],
      },
    ],
    objects: [],
    settings: { ySort: true, roofFade: false, heightStep: 8, ambient: '#8090a0' },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  }
  return {
    map,
    camera: { x: 0, y: 0, zoom: 1 },
    width: 200,
    height: 200,
    images: new Map<string, CanvasImageSource>([['a', {} as CanvasImageSource]]),
    grid: false,
    hover: null,
    rect: null,
    selectedObjectId: null,
    player: null,
    preview: false,
  }
}

describe('map light layers', () => {
  it('draws visible light tiles as an additive screen overlay', () => {
    const { ctx, calls } = makeContext()
    drawMap(ctx, makeState(true))

    expect(calls).toHaveLength(2)
    expect(calls[0]).toMatchObject({ composite: 'source-over' })
    expect(calls[1]).toMatchObject({ composite: 'screen', alpha: 0.5 })
  })

  it('skips a hidden light layer entirely', () => {
    const { ctx, calls } = makeContext()
    drawMap(ctx, makeState(false))

    expect(calls).toHaveLength(1)
    expect(calls[0]!.composite).toBe('source-over')
  })
})