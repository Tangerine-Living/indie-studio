import { describe, expect, it } from 'vitest'
import {
  parseBootstrap,
  parseMap,
  parseMaps,
  parseQuest,
  parseQuests,
  parseUpdateSettings,
  parseUpdateStatus,
  projectSchema,
  questSchema,
} from './schema'

const goProject = {
  schemaVersion: 1,
  id: 'abc123',
  name: '我的小镇',
  createdAt: '2026-11-21T00:00:00Z',
  updatedAt: '2026-11-21T01:00:00Z',
  engine: 'godot',
  settings: {
    tileSize: 32,
    defaultPaletteId: 'default',
    pixelScale: 4,
    uiLanguage: 'zh-CN',
    styleGuide: {
      perspective: '3/4-topdown-orthographic',
      outline: '1px-dark',
      light: 'top-left',
      maxColors: 16,
    },
  },
  members: [{ id: 'm1', name: '我', role: '独立开发者' }],
  milestones: [],
}

describe('project schema', () => {
  it('parses a backend-shaped project', () => {
    const project = projectSchema.parse(goProject)
    expect(project.name).toBe('我的小镇')
    expect(project.settings.tileSize).toBe(32)
    expect(project.members[0]?.name).toBe('我')
  })

  it('tolerates null slices from hand-edited JSON', () => {
    const project = projectSchema.parse({
      ...goProject,
      members: null,
      milestones: null,
    })
    expect(project.members).toEqual([])
    expect(project.milestones).toEqual([])
  })

  it('falls back on unknown enums instead of breaking the project', () => {
    const project = projectSchema.parse({ ...goProject, engine: 'unity' })
    expect(project.engine).toBe('godot')
  })
})

const goQuest = {
  schemaVersion: 1,
  id: 'q1',
  title: '杂货铺外观',
  type: 'asset',
  status: 'doing',
  priority: 'high',
  assignee: 'm1',
  dueAt: '2026-12-01',
  brief: {
    assetType: 'building',
    tileSize: 32,
    paletteId: 'default',
    style: '3/4 俯视',
    references: [],
    prompt: '杂货铺',
  },
  acceptance: ['三层以下'],
  links: { assetIds: [], mapIds: [], urls: [] },
  comments: [],
  createdAt: '2026-11-21T00:00:00Z',
  updatedAt: '2026-11-21T00:00:00Z',
}

describe('quest schema', () => {
  it('parses a quest and keeps its brief', () => {
    const quest = parseQuest(goQuest)
    expect(quest.status).toBe('doing')
    expect(quest.brief.assetType).toBe('building')
    expect(quest.acceptance).toHaveLength(1)
  })

  it('defaults missing collections', () => {
    const quest = questSchema.parse({
      ...goQuest,
      acceptance: null,
      comments: null,
      links: null,
      brief: null,
    })
    expect(quest.acceptance).toEqual([])
    expect(quest.comments).toEqual([])
    expect(quest.links.assetIds).toEqual([])
    expect(quest.brief.tileSize).toBe(32)
  })

  it('normalises unknown statuses', () => {
    const quest = parseQuest({ ...goQuest, status: 'blocked', priority: 'urgent' })
    expect(quest.status).toBe('todo')
    expect(quest.priority).toBe('medium')
  })

  it('parses a list', () => {
    expect(parseQuests([goQuest, { ...goQuest, id: 'q2' }])).toHaveLength(2)
  })
})

const goMap = {
  schemaVersion: 1,
  id: 'map1',
  name: '小镇广场',
  tileSize: 32,
  width: 40,
  height: 30,
  tilesets: [{ id: 'ts1', assetId: 'a1', tileSize: 32, columns: 8, tileCount: 64 }],
  layers: [
    {
      id: 'ground',
      name: '地面',
      type: 'ground',
      elevation: 0,
      visible: true,
      opacity: 1,
      autotile: { set: 0, base: 0, mode: '47' },
      tiles: [{ x: 1, y: 2, set: 0, tile: 5, flipH: true }],
    },
  ],
  objects: [
    {
      id: 'o1',
      name: '出生点',
      kind: 'spawn',
      x: 4,
      y: 6,
      w: 1,
      h: 1,
      elevation: 0,
      ySort: true,
      roofFade: false,
      solid: false,
      properties: { team: 'player' },
    },
  ],
  settings: { ySort: true, roofFade: true, heightStep: 16, ambient: '#8090a0' },
  createdAt: '2026-11-21T00:00:00Z',
  updatedAt: '2026-11-21T01:00:00Z',
}

describe('map schema', () => {
  it('parses a backend-shaped map', () => {
    const map = parseMap(goMap)
    expect(map.name).toBe('小镇广场')
    expect(map.layers[0]?.autotile?.mode).toBe('47')
    expect(map.layers[0]?.tiles[0]?.flipH).toBe(true)
    expect(map.objects[0]?.properties).toEqual({ team: 'player' })
    expect(map.settings.heightStep).toBe(16)
  })

  it('defaults missing collections and settings', () => {
    const map = parseMap({ ...goMap, layers: null, objects: null, tilesets: null, settings: null })
    expect(map.layers).toEqual([])
    expect(map.objects).toEqual([])
    expect(map.tilesets).toEqual([])
    expect(map.settings).toMatchObject({ ySort: true, roofFade: true, heightStep: 16 })
  })

  it('normalises unknown layer and object types', () => {
    const map = parseMap({
      ...goMap,
      layers: [{ ...goMap.layers[0], type: 'water' }],
      objects: [{ ...goMap.objects[0], kind: 'boss' }],
    })
    expect(map.layers[0]?.type).toBe('ground')
    expect(map.objects[0]?.kind).toBe('prop')
  })

  it('parses map summaries without layers', () => {
    const summaries = parseMaps([
      { id: 'map1', name: '小镇广场', tileSize: 32, width: 40, height: 30, updatedAt: '2026-11-21T01:00:00Z' },
      { name: '无标题' },
    ])
    expect(summaries).toHaveLength(2)
    expect(summaries[0]?.name).toBe('小镇广场')
    expect(summaries[1]?.name).toBe('无标题')
    expect(summaries[1]?.tileSize).toBe(32)
  })
})

describe('update schema', () => {
  it('defaults a missing or invalid updater config', () => {
    const settings = parseUpdateSettings({ channel: 'nightly', manifestUrl: null })
    expect(settings.channel).toBe('stable')
    expect(settings.autoCheck).toBe(true)
    expect(settings).not.toHaveProperty('manifestUrl')

    // The old beta channel is migrated to dev.
    expect(parseUpdateSettings({ channel: 'beta' }).channel).toBe('dev')
  })

  it('parses a check result and tolerates missing download fields', () => {
    const status = parseUpdateStatus({
      currentVersion: '0.9.0',
      available: false,
      channel: 'beta',
      checkedAt: '2026-11-22T00:00:00Z',
      message: '已是最新版本',
    })
    expect(status.channel).toBe('dev')
    expect(status.available).toBe(false)
    expect(status.downloadUrl).toBeUndefined()
  })
})

describe('bootstrap schema', () => {
  it('accepts a missing session and null recent', () => {
    const boot = parseBootstrap({
      info: {
        name: 'PixelBench',
        version: '0.1.0-dev',
        platform: 'windows',
        logPath: 'C:\\logs',
        mode: 'desktop',
      },
      recent: null,
      session: null,
    })
    expect(boot.session).toBeNull()
    expect(boot.recent).toEqual([])
    expect(boot.info.mode).toBe('desktop')
  })

  it('parses a session', () => {
    const boot = parseBootstrap({
      info: { name: 'PixelBench', version: '0.1.0-dev', platform: 'windows', logPath: '', mode: 'desktop' },
      recent: [],
      session: { path: 'C:\\project', project: goProject },
    })
    expect(boot.session?.path).toBe('C:\\project')
    expect(boot.session?.project.id).toBe('abc123')
  })
})