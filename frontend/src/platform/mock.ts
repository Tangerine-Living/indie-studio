import type {
  AIJob,
  AIJobImage,
  AISettings,
  AISettingsInput,
  AITestResult,
  AIProviderId,
  Asset,
  AssetType,
  Bootstrap,
  BuildingOptions,
  CreateOptions,
  DataTable,
  Entity,
  GameMap,
  GeneratedBuilding,
  ImportSummary,
  MapLayer,
  MapSummary,
  MapTile,
  Palette,
  PostProcessOptions,
  PostProcessPreview,
  Project,
  Quest,
  QuestBrief,
  RecentProject,
  Session,
  TableColumn,
  EditorSettings,
  UpdateSettings,
} from '../domain/types'
import { ASSET_TYPES, COLUMN_TYPES, DEFAULT_NEGATIVE_PROMPT } from '../domain/types'
import { normalizeTableKey, tableIssues } from '../domain/tables'
import type { Platform } from './types'
import { sheetSize } from '../editor/sheet'
import { AppError } from '../lib/errors'
import { newID, nowISO, uniq } from '../lib/utils'

/**
 * Browser / Playwright adapter. Mirrors the Go project model and the shape of
 * backend errors so the UI can run (and be tested) without Wails.
 */

const DEFAULT_STORAGE_KEY = 'pixelbench.mock.v1'
const MAX_RECENT = 12

const PALETTE_COLORS = [
  '#0f1b2a',
  '#1e2f45',
  '#2f4b5e',
  '#3e6b5a',
  '#4f8f4a',
  '#6fae5f',
  '#8ccf6f',
  '#b8e986',
  '#f2e6c9',
  '#e8c07a',
  '#d98f4a',
  '#b85c38',
  '#c94f4f',
  '#f2b84b',
  '#6ec6e8',
  '#f4f4f4',
]

interface MockProjectRecord {
  session: Session
  quests: Quest[]
  entities?: Entity[]
  tables?: DataTable[]
  assets: Asset[]
  maps?: GameMap[]
  palettes: Palette[]
  /** Editor output per asset, used in place of the generated placeholder. */
  thumbs?: Record<string, string>
}

interface MockAIState {
  settings: AISettingsInput
  jobs: AIJob[]
}

interface MockDB {
  projects: Record<string, MockProjectRecord>
  recent: RecentProject[]
  current: string | null
  ai?: MockAIState
}

export interface MockOptions {
  storageKey?: string
  persist?: boolean
  seedDemo?: boolean
}

const MOCK_ID = /^[A-Za-z0-9_-]{1,64}$/
const ENTITY_KIND_SET = new Set(['character', 'place', 'faction', 'item', 'event', 'term'])
const ENTITY_STATUS_SET = new Set(['idea', 'draft', 'stable'])
const COLUMN_TYPE_SET = new Set<string>(COLUMN_TYPES)

function cleanMockList(values: string[] | undefined, max = 32): string[] {
  const out: string[] = []
  const seen = new Set<string>()
  for (const raw of values ?? []) {
    const value = raw.trim()
    if (!value) continue
    const key = value.toLocaleLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(value)
    if (out.length >= max) break
  }
  return out
}

function cleanMockIDs(values: string[] | undefined, max = 200): string[] {
  const out: string[] = []
  const seen = new Set<string>()
  for (const raw of values ?? []) {
    const value = raw.trim()
    if (!MOCK_ID.test(value) || seen.has(value)) continue
    seen.add(value)
    out.push(value)
    if (out.length >= max) break
  }
  return out
}

/** Mirrors project.ValidateEntity closely enough for browser previews. */
function normalizeMockEntity(input: Entity): Entity {
  const name = input.name.trim()
  if (!name) throw new AppError('INVALID_ENTITY', '设定名称不能为空')
  if (name.length > 120) throw new AppError('INVALID_ENTITY', '设定名称不能超过 120 个字符')
  const id = input.id || newID()
  if (!MOCK_ID.test(id)) throw new AppError('INVALID_ENTITY', '设定 ID 不合法')
  const now = nowISO()
  const fields: Entity['fields'] = []
  const seenKeys = new Set<string>()
  for (const field of input.fields ?? []) {
    const key = field.key.trim()
    if (!key) continue
    const lower = key.toLocaleLowerCase()
    if (seenKeys.has(lower)) continue
    seenKeys.add(lower)
    fields.push({ key: key.slice(0, 40), value: field.value.trim().slice(0, 2000) })
    if (fields.length >= 100) break
  }
  const relations: Entity['relations'] = []
  const seenRelations = new Set<string>()
  for (const relation of input.relations ?? []) {
    const target = relation.entityId.trim()
    if (!MOCK_ID.test(target) || target === id) continue
    const label = relation.label.trim().slice(0, 60) || '相关'
    const key = `${target}\u0000${label}`
    if (seenRelations.has(key)) continue
    seenRelations.add(key)
    relations.push({ entityId: target, label })
    if (relations.length >= 200) break
  }
  return {
    schemaVersion: 1,
    id,
    kind: ENTITY_KIND_SET.has(input.kind) ? input.kind : 'character',
    name,
    status: ENTITY_STATUS_SET.has(input.status) ? input.status : 'draft',
    aliases: cleanMockList(input.aliases),
    summary: input.summary.trim().slice(0, 300),
    body: input.body.trim().slice(0, 100000),
    tags: cleanMockList(input.tags),
    fields,
    relations,
    assetIds: cleanMockIDs(input.assetIds),
    mapIds: cleanMockIDs(input.mapIds),
    artPrompt: (input.artPrompt ?? '').trim().slice(0, 4000),
    createdAt: input.createdAt || now,
    updatedAt: now,
  }
}

/** Mirrors project.ValidateTable closely enough for browser previews. */
function normalizeMockTable(input: DataTable): DataTable {
  const name = input.name.trim()
  if (!name) throw new AppError('INVALID_TABLE', '数值表名称不能为空')
  if (name.length > 80) throw new AppError('INVALID_TABLE', '数值表名称不能超过 80 个字符')
  const id = input.id || newID()
  if (!MOCK_ID.test(id)) throw new AppError('INVALID_TABLE', '数值表 ID 不合法')
  const now = nowISO()

  const columns: TableColumn[] = []
  const usedIDs = new Set<string>()
  const usedKeys = new Set<string>()
  for (const [index, raw] of (input.columns ?? []).slice(0, 64).entries()) {
    let columnId = raw.id
    if (!columnId || !MOCK_ID.test(columnId) || usedIDs.has(columnId)) columnId = newID()
    usedIDs.add(columnId)

    const type = COLUMN_TYPE_SET.has(raw.type) ? raw.type : 'text'
    const base = normalizeTableKey(raw.key)
    let key = base
    for (let suffix = 2; usedKeys.has(key); suffix += 1) key = `${base}_${suffix}`
    usedKeys.add(key)

    const column: TableColumn = {
      id: columnId,
      name: raw.name.trim().slice(0, 40) || `第 ${index + 1} 列`,
      key,
      type,
      required: Boolean(raw.required),
      options: [],
    }
    if (type === 'enum') {
      column.options = cleanMockList(raw.options, 64).map((option) => option.slice(0, 40))
    }
    if (type === 'ref') {
      const ref = raw.refTableId?.trim() ?? ''
      if (ref && MOCK_ID.test(ref)) column.refTableId = ref
    }
    if (type === 'int' || type === 'number') {
      if (typeof raw.min === 'number' && Number.isFinite(raw.min)) column.min = raw.min
      if (typeof raw.max === 'number' && Number.isFinite(raw.max)) column.max = raw.max
      if (column.min !== undefined && column.max !== undefined && column.min > column.max) {
        ;[column.min, column.max] = [column.max, column.min]
      }
    }
    if (raw.note?.trim()) column.note = raw.note.trim().slice(0, 200)
    columns.push(column)
  }

  const rows: DataTable['rows'] = []
  const usedRows = new Set<string>()
  for (const raw of (input.rows ?? []).slice(0, 5000)) {
    let rowId = raw.id
    if (!rowId || !MOCK_ID.test(rowId) || usedRows.has(rowId)) rowId = newID()
    usedRows.add(rowId)
    const values: Record<string, string | number | boolean> = {}
    for (const col of columns) {
      const value = raw.values?.[col.id]
      if (value === undefined || value === null) continue
      switch (col.type) {
        case 'text':
          if (typeof value === 'string') values[col.id] = value.slice(0, 2000)
          break
        case 'enum':
          if (typeof value === 'string') values[col.id] = value.trim().slice(0, 40)
          break
        case 'ref':
          if (typeof value === 'string' && value.trim()) values[col.id] = value.trim()
          break
        case 'int':
          if (typeof value === 'number' && Number.isFinite(value)) values[col.id] = Math.trunc(value)
          break
        case 'number':
          if (typeof value === 'number' && Number.isFinite(value)) values[col.id] = value
          break
        case 'bool':
          if (typeof value === 'boolean') values[col.id] = value
          break
      }
    }
    rows.push({ id: rowId, values })
  }

  return {
    schemaVersion: 1,
    id,
    name,
    key: normalizeTableKey(input.key),
    note: (input.note ?? '').trim().slice(0, 2000),
    columns,
    rows,
    createdAt: input.createdAt || now,
    updatedAt: now,
  }
}

function clone<T>(value: T): T {
  // JSON round-trip also handles Vue reactive proxies, which structuredClone
  // refuses to clone.
  return JSON.parse(JSON.stringify(value)) as T
}

function storage(): Storage | null {
  try {
    return typeof window !== 'undefined' ? window.localStorage : null
  } catch {
    return null
  }
}

function normalizeRoot(path: string): string {
  let root = path.trim().replace(/^["']|["']$/g, '').replace(/\//g, '\\')
  root = root.replace(/\\+$/g, '')
  if (/\\project\.json$/i.test(root)) root = root.slice(0, -'\\project.json'.length)
  return root
}

function recordKey(path: string): string {
  return normalizeRoot(path).toLowerCase()
}

function joinPath(parent: string, name: string): string {
  const base = normalizeRoot(parent)
  return base ? `${base}\\${name}` : name
}

function validateName(name: string): string {
  const trimmed = name.trim()
  if (!trimmed) throw new AppError('INVALID_NAME', '项目名称不能为空')
  if (trimmed.length > 80) throw new AppError('INVALID_NAME', '项目名称不能超过 80 个字符')
  if (/[<>:"/\\|?*]/.test(trimmed)) {
    throw new AppError('INVALID_NAME', '项目名称不能包含 < > : " / \\ | ? *')
  }
  if (/[. ]$/.test(trimmed)) throw new AppError('INVALID_NAME', '项目名称不能以点或空格结尾')
  if (/^(con|prn|aux|nul|com[1-9]|lpt[1-9])$/i.test(trimmed)) {
    throw new AppError('INVALID_NAME', `${trimmed} 是 Windows 保留名称，请换一个`)
  }
  return trimmed
}

function hashString(value: string): number {
  let hash = 2166136261
  for (let i = 0; i < value.length; i += 1) {
    hash ^= value.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  return hash >>> 0
}

/** Mirrors the Go export folder sanitizer closely enough for previews. */
function safeExportName(name: string): string {
  const cleaned = name
    .trim()
    .replace(/[<>:"/\\|?*\u0000-\u001f]/g, '_')
    .replace(/^[ .]+|[ .]+$/g, '')
  const capped = Array.from(cleaned).slice(0, 64).join('')
  if (!capped || capped === '.' || capped === '..') return 'asset'
  if (/^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])$/i.test(capped)) return `_${capped}`
  return capped
}

function mockExportFolder(record: MockProjectRecord, asset: Asset): string {
  const base = safeExportName(asset.name)
  const shared = record.assets.some(
    (other) => other.id !== asset.id && safeExportName(other.name) === base,
  )
  if (!shared) return base
  const suffix = asset.id.replace(/[^A-Za-z0-9]/g, '').slice(0, 8) || 'asset'
  return `${base}-${suffix}`
}

function mockMapFolder(record: MockProjectRecord, map: GameMap): string {
  const base = safeExportName(map.name)
  const shared = recordMaps(record).some(
    (other) => other.id !== map.id && safeExportName(other.name) === base,
  )
  if (!shared) return base
  const suffix = map.id.replace(/[^A-Za-z0-9]/g, '').slice(0, 8) || 'map'
  return `${base}-${suffix}`
}

function emptyDB(): MockDB {
  return { projects: {}, recent: [], current: null }
}

function loadDB(key: string, persist: boolean): MockDB {
  if (!persist) return emptyDB()
  const store = storage()
  if (!store) return emptyDB()
  try {
    const raw = store.getItem(key)
    if (!raw) return emptyDB()
    const parsed = JSON.parse(raw) as Partial<MockDB>
    if (!parsed || typeof parsed !== 'object' || !parsed.projects) return emptyDB()
    return {
      projects: parsed.projects as Record<string, MockProjectRecord>,
      recent: Array.isArray(parsed.recent) ? parsed.recent : [],
      current: typeof parsed.current === 'string' ? parsed.current : null,
    }
  } catch {
    return emptyDB()
  }
}

function saveDB(key: string, db: MockDB, persist: boolean) {
  if (!persist) return
  const store = storage()
  if (!store) return
  try {
    store.setItem(key, JSON.stringify(db))
  } catch {
    /* quota exceeded: the in-memory session keeps working */
  }
}

export function resetMockStorage(storageKey = DEFAULT_STORAGE_KEY) {
  try {
    storage()?.removeItem(storageKey)
  } catch {
    /* ignore */
  }
}

/* ------------------------------------------------------------------ seeds */

function baseBrief(project: Project, overrides: Partial<QuestBrief> = {}): QuestBrief {
  return {
    assetType: 'sprite',
    tileSize: project.settings.tileSize,
    paletteId: project.settings.defaultPaletteId,
    style: '3/4 俯视，1px 深色描边，右上受光',
    references: [],
    prompt: '',
    ...overrides,
  }
}

function makeQuest(
  project: Project,
  input: {
    title: string
    type: Quest['type']
    status: Quest['status']
    priority: Quest['priority']
    description?: string
    dueAt?: string
    brief?: Partial<QuestBrief>
    acceptance?: string[]
    assetIds?: string[]
    createdAt?: string
  },
): Quest {
  const now = nowISO()
  return {
    schemaVersion: 1,
    id: newID(),
    title: input.title,
    type: input.type,
    status: input.status,
    priority: input.priority,
    assignee: project.members[0]?.id ?? 'm1',
    dueAt: input.dueAt,
    description: input.description,
    brief: baseBrief(project, input.brief),
    acceptance: input.acceptance ?? [],
    links: { assetIds: input.assetIds ?? [], mapIds: [], urls: [] },
    comments: [],
    createdAt: input.createdAt ?? now,
    updatedAt: input.createdAt ?? now,
  }
}

const ATLAS_TILES = [
  '#4f8f4a', '#6fae5f', '#d98f4a', '#8b99ab',
  '#3e6b8a', '#e8c07a', '#7a7f86', '#b04e3e',
  '#3e5a7a', '#8a5a36', '#5c3a24', '#2f5b3e',
  '#3e6b4a', '#f2b84b', '#78543a', '#0f1b2a',
]

/** A 4×4 demo atlas that mirrors internal/project/seed.go. */
function tileAtlasSVG(): string {
  const tile = 32
  const parts = [
    `<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128" shape-rendering="crispEdges">`,
  ]
  for (let index = 0; index < ATLAS_TILES.length; index += 1) {
    const x = (index % 4) * tile
    const y = Math.trunc(index / 4) * tile
    const fill = ATLAS_TILES[index]!
    parts.push(`<rect x="${x}" y="${y}" width="${tile}" height="${tile}" fill="${fill}"/>`)
    for (let py = 0; py < tile; py += 4) {
      for (let px = 0; px < tile; px += 4) {
        if ((px * 7 + py * 13 + index * 5) % 19 === 0) {
          parts.push(
            `<rect x="${x + px}" y="${y + py}" width="4" height="4" fill="#000" opacity="0.08"/>`,
          )
        }
      }
    }
    if (index === 3 || index === 6) {
      for (let py = 0; py < tile; py += 8) {
        parts.push(`<rect x="${x}" y="${y + py}" width="${tile}" height="1" fill="#2f4b5e" opacity="0.6"/>`)
      }
    }
    if (index === 4) {
      for (let py = 4; py < tile; py += 9) {
        parts.push(`<rect x="${x + 3}" y="${y + py}" width="${tile - 6}" height="2" fill="#6ea8c8" opacity="0.7"/>`)
      }
    }
    if (index === 7 || index === 8) {
      for (let py = 0; py < tile; py += 6) {
        parts.push(`<rect x="${x}" y="${y + py}" width="${tile}" height="1" fill="#000" opacity="0.16"/>`)
      }
    }
    if (index === 9) {
      for (let px = 0; px < tile; px += 8) {
        parts.push(`<rect x="${x + px}" y="${y}" width="1" height="${tile}" fill="#000" opacity="0.2"/>`)
      }
    }
    if (index === 10) {
      parts.push(`<rect x="${x + 3}" y="${y + 3}" width="${tile - 6}" height="${tile - 6}" fill="#8a5a36"/>`)
      parts.push(`<rect x="${x + 19}" y="${y + 16}" width="3" height="3" fill="#f2d878"/>`)
    }
    if (index === 11) {
      parts.push(`<rect x="${x + 14}" y="${y + 20}" width="4" height="10" fill="#78543a"/>`)
      parts.push(`<circle cx="${x + 16}" cy="${y + 12}" r="10" fill="#2f5b3e"/>`)
    }
    if (index === 12) {
      parts.push(`<circle cx="${x + 16}" cy="${y + 18}" r="10" fill="#3e6b4a"/>`)
    }
    if (index === 13) {
      parts.push(`<rect x="${x + 6}" y="${y + 6}" width="3" height="3" fill="#f2b84b"/>`)
      parts.push(`<rect x="${x + 20}" y="${y + 18}" width="3" height="3" fill="#f2e6c9"/>`)
    }
    if (index === 14) {
      parts.push(`<rect x="${x}" y="${y + 12}" width="${tile}" height="3" fill="#78543a"/>`)
      parts.push(`<rect x="${x + 12}" y="${y}" width="3" height="${tile}" fill="#78543a"/>`)
    }
  }
  parts.push('</svg>')
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(parts.join(''))}`
}

function townMapTiles(width: number, height: number): MapLayer[] {
  const grid = new Map<string, MapTile>()
  const put = (x: number, y: number, tile: number) => {
    grid.set(`${x},${y}`, { x, y, set: 0, tile })
  }
  for (let y = 0; y < height; y += 1) {
    for (let x = 0; x < width; x += 1) put(x, y, 0)
  }
  const rect = (x0: number, y0: number, x1: number, y1: number, tile: number) => {
    for (let y = y0; y <= y1; y += 1) {
      for (let x = x0; x <= x1; x += 1) put(x, y, tile)
    }
  }
  rect(12, 9, 27, 20, 3)
  rect(19, 0, 20, height - 1, 2)
  rect(0, 15, width - 1, 16, 2)
  rect(3, 3, 9, 8, 5)
  rect(4, 4, 8, 7, 4)
  put(1, 1, 1)
  put(30, 4, 1)
  put(34, 22, 1)

  const ground = [...grid.values()].sort((a, b) => a.y - b.y || a.x - b.x)
  const decoration: MapTile[] = []
  for (const [x, y] of [[1, 3], [2, 20], [36, 6], [37, 25], [10, 26], [30, 27]]) {
    decoration.push({ x, y, set: 0, tile: 11 })
  }
  for (const [x, y] of [[14, 7], [25, 6], [11, 22], [28, 22], [33, 12]]) {
    decoration.push({ x, y, set: 0, tile: 12 })
  }
  for (const [x, y] of [[11, 10], [28, 10], [11, 19], [28, 19], [16, 6], [23, 23]]) {
    decoration.push({ x, y, set: 0, tile: 13 })
  }

  const building: MapTile[] = []
  const roof: MapTile[] = []
  for (let x = 23; x <= 28; x += 1) {
    roof.push({ x, y: 8, set: 0, tile: 7 }, { x, y: 9, set: 0, tile: 7 })
    building.push({ x, y: 10, set: 0, tile: 9 }, { x, y: 11, set: 0, tile: 9 })
  }
  building.push({ x: 25, y: 11, set: 0, tile: 10 })

  const collision: MapTile[] = []
  for (let y = 3; y <= 8; y += 1) {
    for (let x = 3; x <= 9; x += 1) collision.push({ x, y, set: 0, tile: 0 })
  }
  for (let y = 8; y <= 11; y += 1) {
    for (let x = 23; x <= 28; x += 1) collision.push({ x, y, set: 0, tile: 0 })
  }

  return [
    { id: 'ground', name: '地面', type: 'ground', elevation: 0, visible: true, opacity: 1, tiles: ground },
    { id: 'decoration', name: '装饰', type: 'decoration', elevation: 0, visible: true, opacity: 1, tiles: decoration },
    { id: 'building', name: '建筑', type: 'building', elevation: 0, visible: true, opacity: 1, tiles: building },
    { id: 'roof', name: '屋顶', type: 'roof', elevation: 0, visible: true, opacity: 1, tiles: roof },
    { id: 'objects', name: '对象', type: 'object', elevation: 0, visible: true, opacity: 1, tiles: [] },
    { id: 'collision', name: '碰撞', type: 'collision', elevation: 0, visible: true, opacity: 0.5, tiles: collision },
  ]
}

function makeTownMap(project: Project, atlasId: string): GameMap {
  const now = nowISO()
  return {
    schemaVersion: 1,
    id: newID(),
    name: '小镇广场',
    tileSize: project.settings.tileSize,
    width: 40,
    height: 30,
    tilesets: [
      {
        id: 'town-atlas',
        assetId: atlasId,
        tileSize: project.settings.tileSize,
        columns: 4,
        tileCount: 16,
      },
    ],
    layers: townMapTiles(40, 30),
    objects: [
      { id: newID(), name: '玩家出生点', kind: 'spawn', x: 19.5, y: 16.5, w: 1, h: 1, elevation: 0, ySort: true, roofFade: false, solid: false, color: '#6ec6e8' },
      { id: newID(), name: '面包师', kind: 'npc', x: 16, y: 12.5, w: 1, h: 1, elevation: 0, ySort: true, roofFade: false, solid: true, color: '#f2b84b', properties: { dialog: '今天的面包刚出炉。' } },
      { id: newID(), name: '宝箱', kind: 'chest', x: 26, y: 18, w: 1, h: 1, elevation: 0, ySort: true, roofFade: false, solid: true, color: '#d98f4a', properties: { item: '铜币' } },
      { id: newID(), name: '广场入口', kind: 'trigger', x: 19, y: 28, w: 2, h: 1, elevation: 0, ySort: false, roofFade: false, solid: false, color: '#8ccf6f', properties: { event: 'enter-square' } },
    ],
    settings: { ySort: true, roofFade: true, heightStep: 16, ambient: '#8090a0' },
    createdAt: now,
    updatedAt: now,
  }
}

function makeDemoTables(): DataTable[] {
  const now = nowISO()
  const col = (
    name: string,
    key: string,
    type: TableColumn['type'],
    extra: Partial<TableColumn> = {},
  ): TableColumn => ({ id: newID(), name, key, type, required: false, options: [], ...extra })
  const row = (values: Record<string, string | number | boolean>) => ({ id: newID(), values })

  const itemName = col('名称', 'name', 'text', { required: true })
  const itemKind = col('类型', 'kind', 'enum', { required: true, options: ['消耗品', '装备', '材料'] })
  const itemPrice = col('售价', 'price', 'int', { min: 0 })
  const itemStack = col('堆叠上限', 'stack', 'int', { min: 1 })
  const itemSellable = col('可出售', 'sellable', 'bool')
  const items: DataTable = {
    schemaVersion: 1,
    id: newID(),
    name: '物品',
    key: 'items',
    note: '背包里的每件东西都在这里。掉落表通过引用指向本表的行。',
    columns: [itemName, itemKind, itemPrice, itemStack, itemSellable],
    rows: [
      row({ [itemName.id]: '苹果', [itemKind.id]: '消耗品', [itemPrice.id]: 12, [itemStack.id]: 99, [itemSellable.id]: true }),
      row({ [itemName.id]: '面包', [itemKind.id]: '消耗品', [itemPrice.id]: 8, [itemStack.id]: 99, [itemSellable.id]: true }),
      row({ [itemName.id]: '铁剑', [itemKind.id]: '装备', [itemPrice.id]: 320, [itemStack.id]: 1, [itemSellable.id]: true }),
      row({ [itemName.id]: '旧地图', [itemKind.id]: '材料', [itemPrice.id]: 60, [itemStack.id]: 1, [itemSellable.id]: false }),
    ],
    createdAt: now,
    updatedAt: now,
  }

  const skillName = col('名称', 'name', 'text', { required: true })
  const skillKind = col('类型', 'kind', 'enum', { required: true, options: ['攻击', '辅助', '被动'] })
  const skillCooldown = col('冷却（秒）', 'cooldown', 'number', { min: 0 })
  const skillCost = col('消耗', 'cost', 'int', { min: 0 })
  const skillNote = col('说明', 'note', 'text')
  const skills: DataTable = {
    schemaVersion: 1,
    id: newID(),
    name: '技能',
    key: 'skills',
    note: '战斗与探索技能。冷却可以是小数，消耗按整数计算。',
    columns: [skillName, skillKind, skillCooldown, skillCost, skillNote],
    rows: [
      row({ [skillName.id]: '挥砍', [skillKind.id]: '攻击', [skillCooldown.id]: 0.8, [skillCost.id]: 0, [skillNote.id]: '向前方挥出一击，命中后短暂硬直。' }),
      row({ [skillName.id]: '突刺', [skillKind.id]: '攻击', [skillCooldown.id]: 2.5, [skillCost.id]: 6, [skillNote.id]: '向前突进两格，可以穿过低矮障碍。' }),
      row({ [skillName.id]: '回复术', [skillKind.id]: '辅助', [skillCooldown.id]: 8, [skillCost.id]: 12, [skillNote.id]: '恢复少量生命，对友方也生效。' }),
      row({ [skillName.id]: '疾行', [skillKind.id]: '被动', [skillCooldown.id]: 0, [skillCost.id]: 0, [skillNote.id]: '脱离战斗时移动速度提高 20%。' }),
    ],
    createdAt: now,
    updatedAt: now,
  }

  const enemyName = col('名称', 'name', 'text', { required: true })
  const enemyHP = col('生命', 'hp', 'int', { min: 1 })
  const enemyAttack = col('攻击', 'attack', 'int', { min: 0 })
  const enemySpeed = col('速度', 'speed', 'number', { min: 0, max: 10 })
  const enemyDrop = col('掉落', 'drop', 'ref', { refTableId: items.id })
  const enemies: DataTable = {
    schemaVersion: 1,
    id: newID(),
    name: '敌人',
    key: 'enemies',
    note: '镇外会遇到的敌人。掉落引用「物品」表的行，删掉那一行后这里会报错。',
    columns: [enemyName, enemyHP, enemyAttack, enemySpeed, enemyDrop],
    rows: [
      row({ [enemyName.id]: '野狗', [enemyHP.id]: 30, [enemyAttack.id]: 4, [enemySpeed.id]: 6, [enemyDrop.id]: items.rows[1]!.id }),
      row({ [enemyName.id]: '盗贼', [enemyHP.id]: 60, [enemyAttack.id]: 7, [enemySpeed.id]: 7, [enemyDrop.id]: items.rows[0]!.id }),
      row({ [enemyName.id]: '骷髅兵', [enemyHP.id]: 90, [enemyAttack.id]: 6, [enemySpeed.id]: 3, [enemyDrop.id]: items.rows[2]!.id }),
    ],
    createdAt: now,
    updatedAt: now,
  }
  return [items, skills, enemies]
}

function makeProjectRecord(root: string, name: string, opts: CreateOptions): MockProjectRecord {
  const now = nowISO()
  const project: Project = {
    schemaVersion: 1,
    id: newID(),
    name,
    createdAt: now,
    updatedAt: now,
    engine: opts.engine,
    stage: 'prototype',
    settings: {
      tileSize: opts.tileSize,
      defaultPaletteId: 'default',
      pixelScale: 4,
      uiLanguage: 'zh-CN',
      template: opts.template,
      styleGuide: {
        perspective: '3/4-topdown-orthographic',
        outline: '1px-dark',
        light: 'top-left',
        maxColors: 16,
        negativePrompt: DEFAULT_NEGATIVE_PROMPT,
        stylePrompt: '',
      },
    },
    members: [{ id: 'm1', name: '我', role: '独立开发者' }],
    milestones: opts.template === 'town-square'
      ? [
          { id: 'ms1', title: '可玩 Demo', dueAt: '2027-02-01', done: false },
          { id: 'ms2', title: '商店页面', dueAt: '2027-06-01', done: false },
        ]
      : [],
  }

  const palette: Palette = {
    schemaVersion: 1,
    id: 'default',
    name: '默认调色板',
    colors: [...PALETTE_COLORS],
    source: 'builtin',
    locked: false,
  }

  // A blank project ships the scaffold and palette only, like the Go backend.
  if (opts.template === 'blank') {
    return {
      session: { path: root, project },
      quests: [],
      entities: [],
      tables: [],
      assets: [],
      maps: [],
      palettes: [palette],
    }
  }

  const grass: Asset = {
    schemaVersion: 1,
    id: newID(),
    name: '草地占位图',
    type: 'tile',
    tags: ['占位', '草地'],
    status: 'approved',
    dimensions: { w: 32, h: 32 },
    pivot: { x: 16, y: 16 },
    paletteId: 'default',
    questIds: [],
    license: 'original',
    source: { kind: 'procedural', provider: 'seed' },
    currentVersion: '0001',
    versions: [
      {
        id: '0001',
        createdAt: now,
        hash: hashString(name + 'grass').toString(16).padStart(8, '0').repeat(4),
        metaFile: 'versions/0001/meta.json',
        size: 2048,
      },
    ],
    createdAt: now,
    updatedAt: now,
  }

  const atlas: Asset = {
    ...clone(grass),
    id: newID(),
    name: '小镇瓦片图集',
    type: 'tile',
    tags: ['瓦片', '示例'],
    status: 'approved',
    dimensions: { w: 128, h: 128 },
    pivot: { x: 64, y: 64 },
    source: { kind: 'procedural', provider: 'seed' },
    createdAt: now,
    updatedAt: now,
  }

  const shop: Asset = {
    ...clone(grass),
    id: newID(),
    name: '杂货铺（草稿）',
    type: 'building',
    tags: ['城镇', '商铺'],
    status: 'draft',
    dimensions: { w: 128, h: 160 },
    pivot: { x: 64, y: 152 },
    source: { kind: 'generated', provider: 'mock', workflow: 'building-v1' },
    currentVersion: undefined,
    versions: [],
    createdAt: now,
    updatedAt: now,
  }

  const traveler: Asset = {
    ...clone(grass),
    id: newID(),
    name: '旅行者（草稿）',
    type: 'sprite',
    tags: ['角色', '四方向'],
    status: 'review',
    dimensions: { w: 32, h: 32 },
    pivot: { x: 16, y: 30 },
    animation: { fps: 8, loop: true, tags: ['idle', 'walk'] },
    createdAt: now,
    updatedAt: now,
  }

  const icon: Asset = {
    ...clone(grass),
    id: newID(),
    name: '背包图标',
    type: 'ui',
    tags: ['界面', '图标'],
    status: 'approved',
    dimensions: { w: 16, h: 16 },
    pivot: { x: 8, y: 8 },
    createdAt: now,
    updatedAt: now,
  }

  const barrel: Asset = {
    ...clone(grass),
    id: newID(),
    name: '木桶',
    type: 'prop',
    tags: ['道具', '室外'],
    status: 'approved',
    dimensions: { w: 16, h: 16 },
    pivot: { x: 8, y: 14 },
    createdAt: now,
    updatedAt: now,
  }

  const quests: Quest[] = [
    makeQuest(project, {
      title: '创建第一张角色素材',
      type: 'asset',
      status: 'todo',
      priority: 'high',
      description: '用生成台做出第一个四方向行走的角色，并走完像素完美后处理。',
      dueAt: '2026-12-05',
      brief: {
        assetType: 'sprite',
        prompt: '一个背着布包的旅行者，四方向行走循环，透明背景',
      },
      acceptance: ['四方向各 4 帧', '不超过 16 色', '在 32×32 网格内对齐'],
      assetIds: [traveler.id],
    }),
    makeQuest(project, {
      title: '草地瓦片 16 变体',
      type: 'asset',
      status: 'doing',
      priority: 'medium',
      description: '让地面不那么单调：草丛、花、碎石、泥土边缘。',
      dueAt: '2026-11-28',
      brief: { assetType: 'tile', prompt: '草地瓦片变体，可无缝拼接' },
      acceptance: ['16 种变体', '四边无缝', '与默认调色板一致'],
      assetIds: [grass.id],
    }),
    makeQuest(project, {
      title: '小镇广场贴图验收',
      type: 'asset',
      status: 'review',
      priority: 'high',
      description: '检查建筑、地面与道具的比例和描边是否统一。',
      dueAt: '2026-11-30',
      brief: { assetType: 'building', prompt: '小镇广场整体贴图' },
      acceptance: ['比例统一', '屋檐阴影方向一致'],
      assetIds: [grass.id, shop.id],
    }),
    makeQuest(project, {
      title: '主菜单图标像素化',
      type: 'design',
      status: 'backlog',
      priority: 'low',
      description: '开始 / 设置 / 退出三个图标，保持同一视觉语言。',
      brief: { assetType: 'ui', prompt: '像素风主菜单图标' },
      acceptance: ['三枚图标尺寸一致'],
      assetIds: [icon.id],
    }),
    makeQuest(project, {
      title: '导出第一版图集',
      type: 'code',
      status: 'done',
      priority: 'medium',
      description: '打包 PNG + JSON，确认 pivot 与动画标签可用。',
      acceptance: ['Godot 可导入', 'pivot 正确'],
      assetIds: [barrel.id],
    }),
  ]

  // Wire both sides of the quest <-> asset relation for the seeded data.
  for (const quest of quests) {
    for (const asset of [grass, shop, traveler, icon, barrel]) {
      if (quest.links.assetIds.includes(asset.id) && !asset.questIds.includes(quest.id)) {
        asset.questIds.push(quest.id)
      }
    }
  }

  const map = makeTownMap(project, atlas.id)
  const squareQuest = quests.find((quest) => quest.title === '小镇广场贴图验收')
  if (squareQuest) squareQuest.links.mapIds = [map.id]

  const plaza: Entity = {
    schemaVersion: 1,
    id: newID(),
    kind: 'place',
    name: '小镇广场',
    status: 'stable',
    aliases: ['石铺广场'],
    summary: '小镇唯一的石铺广场，白天是集市，入夜只剩一盏路灯。',
    body: '广场铺着磨损的青石板，中间那圈颜色最深，是常年摆摊留下的痕迹。\n\n北侧是镇长家，东侧通往码头。节日时会在这里搭台，平时午后就只剩几条野狗和收摊的小贩。',
    tags: ['城镇', '集市'],
    fields: [
      { key: '人口', value: '约 400 人' },
      { key: '归属', value: '雾湾镇' },
    ],
    relations: [],
    assetIds: [atlas.id, grass.id],
    mapIds: [map.id],
    artPrompt: '',
    createdAt: now,
    updatedAt: now,
  }
  const wanderer: Entity = {
    schemaVersion: 1,
    id: newID(),
    kind: 'character',
    name: '旅人',
    status: 'draft',
    aliases: ['外乡人'],
    summary: '刚从北边来到小镇的年轻人，背着一只补过三次的布包。',
    body: '他不肯说自己的名字，只说要在这里等一个人。\n\n说话很少，付钱很准时。布包里除了干粮，还有一张被雨泡皱的地图。',
    tags: ['主角'],
    fields: [
      { key: '年龄', value: '二十出头' },
      { key: '随身物', value: '布包、旧地图' },
    ],
    relations: [{ entityId: plaza.id, label: '初到' }],
    assetIds: [traveler.id],
    mapIds: [],
    artPrompt: '',
    createdAt: now,
    updatedAt: now,
  }

  return {
    session: { path: root, project },
    quests,
    entities: [plaza, wanderer],
    tables: makeDemoTables(),
    assets: [grass, atlas, shop, traveler, icon, barrel],
    maps: [map],
    palettes: [palette],
    thumbs: { [atlas.id]: tileAtlasSVG() },
  }
}

/* -------------------------------------------------------------- data ops */

function touchRecent(db: MockDB, record: MockProjectRecord) {
  const entry: RecentProject = {
    path: record.session.path,
    name: record.session.project.name,
    lastOpenedAt: nowISO(),
  }
  db.recent = [entry, ...db.recent.filter((item) => recordKey(item.path) !== recordKey(entry.path))]
    .slice(0, MAX_RECENT)
}

function sortedRecent(db: MockDB): RecentProject[] {
  return [...db.recent].sort(
    (a, b) => new Date(b.lastOpenedAt).getTime() - new Date(a.lastOpenedAt).getTime(),
  )
}

function currentRecord(db: MockDB): MockProjectRecord | null {
  if (!db.current) return null
  return db.projects[db.current] ?? null
}

function requireRecord(db: MockDB): MockProjectRecord {
  const record = currentRecord(db)
  if (!record) throw new AppError('NO_PROJECT', '当前没有打开的项目')
  return record
}

function recordMaps(record: MockProjectRecord): GameMap[] {
  if (!record.maps) record.maps = []
  return record.maps
}

function mapSummary(map: GameMap): MapSummary {
  return {
    id: map.id,
    name: map.name,
    tileSize: map.tileSize,
    width: map.width,
    height: map.height,
    updatedAt: map.updatedAt,
  }
}

const TILE_SIZES = new Set([16, 24, 32, 48])

/** Mirrors project.ValidateMap closely enough for browser previews. */
function normalizeMockMap(input: GameMap): GameMap {
  const map = clone(input)
  const name = map.name.trim()
  if (!name) throw new AppError('INVALID_MAP', '地图名称不能为空')
  if (!TILE_SIZES.has(map.tileSize)) {
    throw new AppError('INVALID_TILE_SIZE', '瓦片尺寸必须是 16、24、32 或 48')
  }
  if (!Number.isInteger(map.width) || !Number.isInteger(map.height) || map.width < 1 || map.height < 1 || map.width > 1024 || map.height > 1024) {
    throw new AppError('INVALID_MAP', '地图尺寸必须在 1 到 1024 格之间')
  }
  if (!Array.isArray(map.layers) || map.layers.length === 0) {
    throw new AppError('INVALID_MAP', '地图至少需要一个图层')
  }
  const now = nowISO()
  map.schemaVersion = 1
  map.id = map.id || newID()
  map.name = name
  map.createdAt = map.createdAt || now
  map.updatedAt = now
  map.tilesets = Array.isArray(map.tilesets) ? map.tilesets : []
  map.objects = Array.isArray(map.objects) ? map.objects : []
  map.settings = {
    ySort: map.settings?.ySort ?? true,
    roofFade: map.settings?.roofFade ?? true,
    heightStep: map.settings?.heightStep || 16,
    ambient: map.settings?.ambient || '#8090a0',
  }
  for (const layer of map.layers) {
    const cells = new Map<string, MapTile>()
    for (const tile of layer.tiles ?? []) {
      if (tile.x < 0 || tile.y < 0 || tile.x >= map.width || tile.y >= map.height) continue
      if (layer.type !== 'collision') {
        const set = map.tilesets[tile.set]
        if (!set || tile.tile < 0 || tile.tile >= set.tileCount) continue
      }
      cells.set(`${tile.x},${tile.y}`, { ...tile })
    }
    layer.tiles = [...cells.values()].sort((a, b) => a.y - b.y || a.x - b.x)
    layer.opacity = layer.opacity > 0 && layer.opacity <= 1 ? layer.opacity : 1
  }
  for (const obj of map.objects) {
    obj.x = Math.min(map.width, Math.max(0, obj.x))
    obj.y = Math.min(map.height, Math.max(0, obj.y))
    obj.w = Math.min(map.width, Math.max(0.25, obj.w))
    obj.h = Math.min(map.height, Math.max(0.25, obj.h))
    obj.properties = obj.properties ?? {}
  }
  return map
}

const TYPE_TINT: Record<string, { base: string; light: string; dark: string }> = {
  tile: { base: '#4f8f4a', light: '#8ccf6f', dark: '#2f4b5e' },
  building: { base: '#b85c38', light: '#e8c07a', dark: '#2f4b5e' },
  sprite: { base: '#3e6b5a', light: '#6ec6e8', dark: '#0f1b2a' },
  prop: { base: '#d98f4a', light: '#f2e6c9', dark: '#b85c38' },
  ui: { base: '#6fae5f', light: '#f2b84b', dark: '#1e2f45' },
  fx: { base: '#c94f4f', light: '#f2b84b', dark: '#0f1b2a' },
  other: { base: '#5f6d7e', light: '#8b99ab', dark: '#1d2835' },
}

/** Reads width/height straight from a PNG data URL header. */
function pngSize(dataURL: string): { w: number; h: number } | null {
  const comma = dataURL.indexOf(',')
  if (comma < 0) return null
  try {
    const binary = atob(dataURL.slice(comma + 1))
    if (binary.length < 24) return null
    const bytes = new Uint8Array(binary.length)
    for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i)
    if (bytes[0] !== 0x89 || bytes[1] !== 0x50 || bytes[2] !== 0x4e || bytes[3] !== 0x47) return null
    const w = ((bytes[16] << 24) | (bytes[17] << 16) | (bytes[18] << 8) | bytes[19]) >>> 0
    const h = ((bytes[20] << 24) | (bytes[21] << 16) | (bytes[22] << 8) | bytes[23]) >>> 0
    if (!w || !h || w > 16384 || h > 16384) return null
    return { w, h }
  } catch {
    return null
  }
}

/** Transparent SVG sized like one animation frame (browser mock only). */
function blankFrame(width: number, height: number): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${Math.max(1, width)}" height="${Math.max(1, height)}"/>`
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
}

function assetThumb(asset: Pick<Asset, 'id' | 'type'>): string {
  const tint = TYPE_TINT[asset.type] ?? TYPE_TINT.other
  let state = hashString(asset.id) || 1
  const next = () => {
    state = (state * 1664525 + 1013904223) >>> 0
    return state
  }
  const blocks: string[] = []
  for (let i = 0; i < 22; i += 1) {
    const x = (next() % 15) * 4
    const y = (next() % 15) * 4
    const color = next() % 3 === 0 ? tint.light : next() % 3 === 0 ? tint.dark : tint.base
    blocks.push(`<rect x="${x}" y="${y}" width="4" height="4" fill="${color}"/>`)
  }
  const svg = [
    '<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64" shape-rendering="crispEdges">',
    `<rect width="64" height="64" fill="${tint.base}"/>`,
    ...blocks,
    `<rect y="56" width="64" height="8" fill="${tint.dark}" opacity="0.45"/>`,
    `<rect width="64" height="64" fill="none" stroke="${tint.dark}" stroke-width="2"/>`,
    '</svg>',
  ].join('')
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`
}

/* ------------------------------------------------------- building drafts */

const BUILDING_COLORS = {
  wall: '#c8a87c',
  roof: '#b04e3e',
  trim: '#78543a',
  door: '#5c3a24',
}

function clampInt(value: number, min: number, max: number): number {
  if (!Number.isFinite(value)) return min
  return Math.min(max, Math.max(min, Math.round(value)))
}

function normalizeHex(value: string, fallback: string): string {
  const text = value.trim().replace(/^#/, '')
  if (/^[0-9a-f]{3}$/i.test(text)) {
    return `#${text[0]}${text[0]}${text[1]}${text[1]}${text[2]}${text[2]}`.toLowerCase()
  }
  if (/^[0-9a-f]{6}$/i.test(text)) return `#${text.toLowerCase()}`
  return fallback
}

/** Mirrors `BuildingOptions.normalized()` in internal/generators/building.go. */
function normalizeBuildingOptions(raw: BuildingOptions): BuildingOptions {
  return {
    seed: Math.trunc(raw.seed) || 0,
    width: clampInt(raw.width, 1, 16),
    depth: clampInt(raw.depth, 1, 16),
    floors: clampInt(raw.floors, 1, 4),
    variants: clampInt(raw.variants, 1, 8),
    windows: clampInt(raw.windows, 0, 12),
    tileSize: [16, 24, 32, 48].includes(raw.tileSize) ? raw.tileSize : 32,
    wallColor: normalizeHex(raw.wallColor ?? '', BUILDING_COLORS.wall),
    roofColor: normalizeHex(raw.roofColor ?? '', BUILDING_COLORS.roof),
    trimColor: normalizeHex(raw.trimColor ?? '', BUILDING_COLORS.trim),
    doorColor: normalizeHex(raw.doorColor ?? '', BUILDING_COLORS.door),
    sign: Boolean(raw.sign),
    shadow: Boolean(raw.shadow),
    outline: Boolean(raw.outline),
    weathered: Math.min(1, Math.max(0, Number.isFinite(raw.weathered) ? raw.weathered : 0)),
  }
}

/** Small deterministic PRNG so mock previews match a given seed run to run. */
function seededRandom(seed: number): () => number {
  let state = (Math.trunc(seed) >>> 0) || 1
  return () => {
    state = (Math.imul(state, 1664525) + 1013904223) >>> 0
    return state / 4294967296
  }
}

function hexToRgb(hex: string): [number, number, number] {
  const n = Number.parseInt(hex.slice(1), 16)
  return [(n >> 16) & 0xff, (n >> 8) & 0xff, n & 0xff]
}

function shadeHex(hex: string, delta: number): string {
  const [r, g, b] = hexToRgb(hex).map((channel) =>
    Math.min(255, Math.max(0, Math.round(channel + delta))),
  )
  return `rgb(${r} ${g} ${b})`
}

function rectSVG(x: number, y: number, w: number, h: number, fill: string, extra = ''): string {
  if (w <= 0 || h <= 0) return ''
  return `<rect x="${x}" y="${y}" width="${w}" height="${h}" fill="${fill}"${extra}/>`
}

interface BuildingGeometry {
  pad: number
  eave: number
  width: number
  height: number
  bodyW: number
  bodyD: number
  wallH: number
  roof: { x: number; y: number; w: number; h: number }
  wall: { x: number; y: number; w: number; h: number }
  footprint: { x: number; y: number; w: number; h: number }
  shadow: number
}

function buildingGeometry(opts: BuildingOptions): BuildingGeometry {
  const tile = opts.tileSize
  const bodyW = opts.width * tile
  const bodyD = opts.depth * tile
  const wallH = Math.max(tile, Math.trunc((opts.floors * tile * 3) / 4))
  const pad = 6
  const eave = Math.max(2, Math.trunc(tile / 6))
  const shadow = opts.shadow ? 4 : 0
  return {
    pad,
    eave,
    width: pad * 2 + bodyW,
    height: pad * 2 + bodyD + wallH + shadow,
    bodyW,
    bodyD,
    wallH,
    roof: { x: pad - eave, y: pad - eave, w: bodyW + eave * 2, h: bodyD + eave * 2 },
    wall: { x: pad, y: pad + bodyD - eave, w: bodyW, h: wallH + eave },
    footprint: { x: pad, y: pad, w: bodyW, h: bodyD },
    shadow,
  }
}

/**
 * Browser stand-in for the Go rasterizer: a crisp SVG built from the same
 * geometry (footprint, wall height, roof slab, doorway) so layouts match.
 */
function buildingSVG(
  opts: BuildingOptions,
  geom: BuildingGeometry,
  windows: number,
  rand: () => number,
): string {
  const tile = opts.tileSize
  const wall = opts.wallColor
  const roof = opts.roofColor
  const trim = opts.trimColor
  const door = opts.doorColor
  const stroke = opts.outline ? ' stroke="#0c1016" stroke-width="1"' : ''
  const parts: string[] = ['<svg xmlns="http://www.w3.org/2000/svg" shape-rendering="crispEdges"']
  parts.push(` width="${geom.width}" height="${geom.height}" viewBox="0 0 ${geom.width} ${geom.height}">`)

  if (opts.shadow) {
    parts.push(
      rectSVG(
        geom.footprint.x + geom.shadow,
        geom.footprint.y + geom.shadow,
        geom.footprint.w,
        geom.footprint.h,
        '#080c12',
        ' opacity="0.24"',
      ),
    )
  }

  // Roof slab with shingle courses and a ridge band.
  parts.push(rectSVG(geom.roof.x, geom.roof.y, geom.roof.w, geom.roof.h, roof, stroke))
  parts.push(
    rectSVG(geom.roof.x, geom.roof.y, geom.roof.w, geom.roof.h, 'none', ` stroke="${shadeHex(roof, -26)}"`),
  )
  for (let y = geom.roof.y + 4; y < geom.roof.y + geom.roof.h - 2; y += 4) {
    parts.push(rectSVG(geom.roof.x + 2, y, geom.roof.w - 4, 1, shadeHex(roof, -14)))
  }
  const ridgeY = geom.roof.y + Math.trunc(geom.roof.h / 2)
  parts.push(rectSVG(geom.roof.x + 1, ridgeY - 1, geom.roof.w - 2, 2, shadeHex(roof, 18)))

  // Wall with siding, lit left edge and darkened right edge.
  parts.push(rectSVG(geom.wall.x, geom.wall.y, geom.wall.w, geom.wall.h, wall, stroke))
  for (let y = geom.wall.y + 4; y < geom.wall.y + geom.wall.h - 2; y += 4) {
    parts.push(rectSVG(geom.wall.x + 2, y, geom.wall.w - 4, 1, shadeHex(wall, -12)))
  }
  parts.push(rectSVG(geom.wall.x, geom.wall.y, 2, geom.wall.h, shadeHex(wall, 10)))
  parts.push(
    rectSVG(geom.wall.x + geom.wall.w - 2, geom.wall.y, 2, geom.wall.h, shadeHex(wall, -22)),
  )

  const floorHeight = Math.trunc(geom.wall.h / opts.floors)
  const windowCount = windows || Math.max(1, opts.width - 1)
  const windowW = Math.max(4, Math.trunc(tile / 2))
  const windowH = Math.max(4, Math.min(Math.trunc(tile / 2), Math.trunc(floorHeight / 2)))
  const inset = Math.max(3, Math.trunc(windowW / 2))
  const span = geom.wall.w - inset * 2
  const step = windowCount > 1 ? Math.trunc((span - windowW) / (windowCount - 1)) : 0
  const doorW = Math.max(6, Math.trunc((tile * 2) / 3))
  const doorH = Math.min(geom.wall.h, tile)
  const doorX = geom.wall.x + Math.trunc((geom.wall.w - doorW) / 2)
  const doorY = geom.wall.y + geom.wall.h - doorH

  for (let floor = 0; floor < opts.floors; floor += 1) {
    const centerY = geom.wall.y + floor * floorHeight + Math.trunc(floorHeight / 2)
    const top = centerY - Math.trunc(windowH / 2)
    for (let index = 0; index < windowCount; index += 1) {
      let x = geom.wall.x + inset + index * step
      if (x + windowW > geom.wall.x + geom.wall.w - inset) {
        x = geom.wall.x + geom.wall.w - inset - windowW
      }
      const lastFloor = floor === opts.floors - 1
      if (lastFloor && x < doorX + doorW + 2 && x + windowW > doorX - 2) continue
      parts.push(rectSVG(x, top, windowW, windowH, trim))
      const glass = shadeHex(rand() < 0.5 ? '#4e8ca8' : '#8ab0b8', 0)
      parts.push(rectSVG(x + 1, top + 1, windowW - 2, windowH - 2, glass))
      if (windowW > 4) {
        parts.push(rectSVG(x + Math.trunc(windowW / 2), top + 1, 1, windowH - 2, trim))
      }
      if (windowH > 4) {
        parts.push(rectSVG(x + 1, top + Math.trunc(windowH / 2), windowW - 2, 1, trim))
      }
    }
  }

  parts.push(rectSVG(doorX, doorY, doorW, doorH, trim, stroke))
  parts.push(rectSVG(doorX + 1, doorY + 1, doorW - 2, doorH - 1, door))
  parts.push(rectSVG(doorX + doorW - 4, doorY + Math.trunc(doorH / 2), 1, 1, '#f2d878'))

  if (opts.sign) {
    const signW = doorW + Math.max(6, Math.trunc(tile / 2))
    const signH = Math.max(5, Math.trunc(tile / 2))
    const signX = doorX + Math.trunc((doorW - signW) / 2)
    const signY = doorY - signH - 3
    if (signY > geom.wall.y + 2) {
      parts.push(rectSVG(signX, signY, signW, signH, shadeHex(trim, -18), stroke))
      for (let x = signX + 3; x < signX + signW - 3; x += 4) {
        parts.push(rectSVG(x, signY + Math.trunc(signH / 2), 2, 1, shadeHex(trim, 60)))
      }
    }
  }

  // Weathering: sprinkle dark/light single pixels across the wall and roof.
  if (opts.weathered > 0) {
    const probability = opts.weathered * 0.05
    const scatter = (x: number, y: number, w: number, h: number, base: string) => {
      for (let py = y + 1; py < y + h - 1; py += 2) {
        for (let px = x + 1; px < x + w - 1; px += 2) {
          if (rand() >= probability) continue
          parts.push(
            rectSVG(px, py, 1, 1, shadeHex(base, rand() < 0.5 ? 10 : -12), ' opacity="0.6"'),
          )
        }
      }
    }
    scatter(geom.wall.x, geom.wall.y, geom.wall.w, geom.wall.h, wall)
    scatter(geom.roof.x, geom.roof.y, geom.roof.w, geom.roof.h, roof)
  }

  parts.push('</svg>')
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(parts.join(''))}`
}

function generateMockBuildings(raw: BuildingOptions): GeneratedBuilding[] {
  const opts = normalizeBuildingOptions(raw)
  const geom = buildingGeometry(opts)
  const results: GeneratedBuilding[] = []
  for (let index = 0; index < opts.variants; index += 1) {
    const seed = opts.seed + index * 7919
    const rand = seededRandom(seed)
    let windows = opts.windows
    if (windows === 0 && opts.width > 2 && Math.floor(rand() * 3) === 0) {
      windows = Math.max(1, opts.width - 2)
    }
    results.push({
      seed,
      width: geom.width,
      height: geom.height,
      pivot: {
        x: geom.pad + Math.trunc(geom.bodyW / 2),
        y: geom.pad + geom.bodyD + geom.wallH,
      },
      collision: {
        x: geom.footprint.x,
        y: geom.footprint.y,
        w: geom.footprint.w,
        h: geom.footprint.h,
      },
      image: buildingSVG({ ...opts, windows }, geom, windows, rand),
    })
  }
  return results
}

/* ------------------------------------------------ pixel-perfect (mock) */

/**
 * The browser cannot decode raster pixels without a canvas, so the mock mirrors
 * the Go geometry and palette rules but previews the existing thumbnail. The
 * UI labels the result as simulated.
 */
function postProcessTargetSize(
  asset: Asset,
  options: PostProcessOptions,
): { w: number; h: number } {
  const sourceW = Math.max(1, asset.dimensions.w)
  const sourceH = Math.max(1, asset.dimensions.h)
  let w = Math.round(options.width)
  let h = Math.round(options.height)
  if (!Number.isFinite(w) || w <= 0) w = 0
  if (!Number.isFinite(h) || h <= 0) h = 0
  if (w === 0 && h === 0) {
    w = sourceW
    h = sourceH
  } else if (h === 0) {
    h = Math.max(1, Math.round((sourceH * w) / sourceW))
  } else if (w === 0) {
    w = Math.max(1, Math.round((sourceW * h) / sourceH))
  }
  return {
    w: Math.min(2048, Math.max(1, w)),
    h: Math.min(2048, Math.max(1, h)),
  }
}

function postProcessColors(
  record: MockProjectRecord,
  asset: Asset,
  options: PostProcessOptions,
): string[] {
  if (options.colors.length > 0) return uniq(options.colors.filter(Boolean))
  const palette = record.palettes.find((item) => item.id === asset.paletteId)
  const max = Math.min(64, Math.max(2, Math.round(options.maxColors) || 16))
  return (palette?.colors ?? []).slice(0, max)
}

/* ------------------------------------------------------------- AI drafts */

const DEFAULT_AI_WORKFLOW = JSON.stringify(
  {
    '3': { class_type: 'KSampler', _meta: { title: 'SEED' }, inputs: { seed: 42, steps: 24, cfg: 7, sampler_name: 'euler', scheduler: 'normal', denoise: 1, model: ['4', 0], positive: ['6', 0], negative: ['7', 0], latent_image: ['5', 0] } },
    '4': { class_type: 'CheckpointLoaderSimple', inputs: { ckpt_name: 'sd_xl_base_1.0.safetensors' } },
    '5': { class_type: 'EmptyLatentImage', _meta: { title: 'SIZE' }, inputs: { width: 1024, height: 1024, batch_size: 1 } },
    '6': { class_type: 'CLIPTextEncode', _meta: { title: 'PROMPT' }, inputs: { text: '', clip: ['4', 1] } },
    '7': { class_type: 'CLIPTextEncode', _meta: { title: 'NEGATIVE' }, inputs: { text: '', clip: ['4', 1] } },
    '8': { class_type: 'VAEDecode', inputs: { samples: ['3', 0], vae: ['4', 2] } },
    '9': { class_type: 'SaveImage', inputs: { filename_prefix: 'pixelbench', images: ['8', 0] } },
  },
  null,
  2,
)

function defaultAISettings(): AISettingsInput {
  return {
    defaultProvider: 'comfyui',
    comfy: {
      baseUrl: 'http://127.0.0.1:8188',
      workflowName: '默认文生图',
      workflow: DEFAULT_AI_WORKFLOW,
    },
    openai: {
      baseUrl: 'https://api.openai.com/v1',
      model: 'gpt-image-1',
      editModel: '',
      textModel: 'gpt-4o-mini',
      size: '1024x1024',
      textProvider: 'shared',
      textBaseUrl: '',
    },
  }
}

function aiState(db: MockDB): MockAIState {
  if (!db.ai) db.ai = { settings: defaultAISettings(), jobs: [] }
  if (!db.ai.jobs) db.ai.jobs = []
  return db.ai
}

function aiSettingsView(settings: AISettingsInput): AISettings {
  return {
    defaultProvider: settings.defaultProvider,
    comfy: {
      baseUrl: settings.comfy.baseUrl,
      workflowName: settings.comfy.workflowName,
      workflow: settings.comfy.workflow,
    },
    openai: {
      baseUrl: settings.openai.baseUrl,
      model: settings.openai.model,
      editModel: settings.openai.editModel ?? '',
      textModel: settings.openai.textModel,
      size: settings.openai.size,
      hasKey: Boolean(settings.openai.apiKey),
      textProvider: settings.openai.textProvider === 'own' ? 'own' : 'shared',
      textBaseUrl: settings.openai.textProvider === 'own' ? (settings.openai.textBaseUrl ?? '') : '',
      textHasKey: Boolean(settings.openai.textApiKey),
    },
  }
}

/** Browser stand-in for a generated image: deterministic pixel blocks. */
function aiImageSVG(job: AIJob, index: number): string {
  const width = Math.min(512, Math.max(64, job.width))
  const height = Math.min(512, Math.max(64, job.height))
  const rand = seededRandom(hashString(`${job.id}:${index}`) || 1)
  const parts: string[] = [
    `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}" shape-rendering="crispEdges">`,
    `<rect width="${width}" height="${height}" fill="#1b2431"/>`,
  ]
  const cells = 16
  const size = Math.max(4, Math.trunc(width / cells))
  for (let y = 0; y < cells; y += 1) {
    for (let x = 0; x < cells; x += 1) {
      if (rand() < 0.45) continue
      const color = PALETTE_COLORS[Math.trunc(rand() * PALETTE_COLORS.length)] ?? '#4f8f4a'
      parts.push(`<rect x="${x * size}" y="${y * size}" width="${size}" height="${size}" fill="${color}" opacity="0.85"/>`)
    }
  }
  parts.push('</svg>')
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(parts.join(''))}`
}

/** Inpaint results live only in memory; the raw images are never persisted. */
const mockInpaint = new Map<string, { url: string; width: number; height: number }>()

function aiInpaintSVG(base: string, width: number, height: number, seedText: string): string {
  const w = Math.max(1, Math.min(2048, Math.trunc(width)))
  const h = Math.max(1, Math.min(2048, Math.trunc(height)))
  const rand = seededRandom(hashString(seedText) || 1)
  const dx = 2 + Math.trunc(rand() * Math.max(2, w / 40))
  const dy = 1 + Math.trunc(rand() * Math.max(1, h / 40))
  const parts = [
    `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}" shape-rendering="crispEdges">`,
    `<image href="${base}" width="${w}" height="${h}" preserveAspectRatio="none"/>`,
    `<image href="${base}" width="${w}" height="${h}" preserveAspectRatio="none" transform="translate(${dx} ${dy})" opacity="0.7"/>`,
    '</svg>',
  ]
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(parts.join(''))}`
}

function aiJobImages(job: AIJob): AIJobImage[] {
  if (job.mode === 'inpaint') {
    const entry = mockInpaint.get(job.id)
    if (entry) {
      return [
        {
          index: 0,
          file: 'inpaint-1.svg',
          url: entry.url,
          width: entry.width,
          height: entry.height,
        },
      ]
    }
  }
  return Array.from({ length: job.count }, (_, index) => ({
    index,
    file: `mock-${index}.svg`,
    url: aiImageSVG(job, index),
    width: Math.min(512, Math.max(64, job.width)),
    height: Math.min(512, Math.max(64, job.height)),
  }))
}

/* --------------------------------------------------------------- platform */

export function createMockPlatform(options: MockOptions = {}): Platform {
  const storageKey = options.storageKey ?? DEFAULT_STORAGE_KEY
  const persist = options.persist ?? true
  const seedDemo = options.seedDemo ?? true
  let db = loadDB(storageKey, persist)

  if (Object.keys(db.projects).length === 0 && seedDemo) {
    const demoOptions: CreateOptions = {
      parentDir: 'C:\\PixelBench\\示例项目',
      name: '小镇广场',
      engine: 'godot',
      tileSize: 32,
      template: 'town-square',
      paletteId: 'default',
    }
    const root = joinPath(demoOptions.parentDir, demoOptions.name)
    const record = makeProjectRecord(root, demoOptions.name, demoOptions)
    db.projects[recordKey(root)] = record
    db.recent = [
      { path: root, name: demoOptions.name, lastOpenedAt: nowISO() },
    ]
    saveDB(storageKey, db, persist)
  }

  const commit = () => saveDB(storageKey, db, persist)

  let mockUpdateSettings: UpdateSettings = {
    channel: 'stable',
    autoCheck: true,
  }

  let mockEditorSettings: EditorSettings = {
    showGrid: true,
    openMode: 'fit',
  }

  const aiListeners = new Set<(jobs: AIJob[]) => void>()

  function emitAIJobs() {
    if (aiListeners.size === 0) return
    const jobs = clone(aiState(db).jobs)
    for (const listener of aiListeners) listener(jobs)
  }

  function patchAIJob(id: string, mutate: (job: AIJob) => boolean | void) {
    const state = aiState(db)
    const job = state.jobs.find((item) => item.id === id)
    if (!job) return
    if (mutate(job) === false) return
    job.updatedAt = nowISO()
    commit()
    emitAIJobs()
  }

  /** Mock jobs advance on short timers so the queue UI is exercised. */
  function scheduleAIJob(job: AIJob) {
    setTimeout(() => {
      patchAIJob(job.id, (current) => {
        if (current.state !== 'queued') return false
        current.state = 'running'
        current.progress = 25
        current.stage = '模拟生成中'
      })
    }, 40)
    setTimeout(() => {
      patchAIJob(job.id, (current) => {
        if (current.state !== 'running') return false
        current.progress = 70
        current.stage = '模拟生成中'
      })
    }, 140)
    setTimeout(() => {
      patchAIJob(job.id, (current) => {
        if (current.state !== 'running') return false
        current.state = 'done'
        current.progress = 100
        current.stage = '完成'
        current.images = aiJobImages(current)
      })
    }, 320)
  }

  async function promptPath(title: string, fallback: string): Promise<string> {
    try {
      const value = window.prompt(title, fallback)
      if (value && value.trim()) return value.trim()
      return ''
    } catch {
      return ''
    }
  }

  return {
    kind: 'mock',

    async bootstrap(): Promise<Bootstrap> {
      const record = currentRecord(db)
      return {
        info: {
          name: 'Indie Studio',
          version: '0.1.0-dev',
          platform: 'browser',
          logPath: '',
          mode: 'browser',
        },
        recent: sortedRecent(db),
        session: record ? clone(record.session) : null,
      }
    },

    async listRecent() {
      return sortedRecent(db)
    },

    async createProject(opts) {
      const name = validateName(opts.name)
      const parent = normalizeRoot(opts.parentDir)
      if (!parent) throw new AppError('INVALID_PARENT', '请选择项目存放的父文件夹')
      const root = joinPath(parent, name)
      const key = recordKey(root)
      if (db.projects[key]) {
        throw new AppError('PROJECT_EXISTS', '同名项目已存在', root)
      }
      const record = makeProjectRecord(root, name, { ...opts, name })
      db.projects[key] = record
      db.current = key
      touchRecent(db, record)
      commit()
      return clone(record.session)
    },

    async openProject(path) {
      const key = recordKey(path)
      const record = db.projects[key]
      if (!record) {
        throw new AppError('PROJECT_NOT_FOUND', '该文件夹中没有 project.json', path)
      }
      db.current = key
      touchRecent(db, record)
      commit()
      return clone(record.session)
    },

    async closeProject() {
      db.current = null
      commit()
    },

    async getSession() {
      const record = currentRecord(db)
      return record ? clone(record.session) : null
    },

    async saveProject(project) {
      const record = requireRecord(db)
      const name = validateName(project.name)
      record.session = {
        path: record.session.path,
        project: { ...clone(project), name, updatedAt: nowISO() },
      }
      touchRecent(db, record)
      commit()
      return clone(record.session)
    },

    async listQuests() {
      const record = currentRecord(db)
      if (!record) return []
      return clone(
        [...record.quests].sort(
          (a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
        ),
      )
    },

    async saveQuest(quest) {
      const record = requireRecord(db)
      const title = quest.title.trim()
      if (!title) throw new AppError('INVALID_QUEST', '任务标题不能为空')
      const now = nowISO()
      const next: Quest = {
        ...clone(quest),
        schemaVersion: 1,
        id: quest.id || newID(),
        title,
        createdAt: quest.createdAt || now,
        updatedAt: now,
        acceptance: [...quest.acceptance],
        links: {
          assetIds: uniq([...quest.links.assetIds]),
          mapIds: uniq([...quest.links.mapIds]),
          urls: [...quest.links.urls],
        },
        comments: [...quest.comments],
      }
      const index = record.quests.findIndex((item) => item.id === next.id)
      if (index >= 0) record.quests[index] = next
      else record.quests.push(next)
      // Keep asset manifests in sync with quest links.
      for (const asset of record.assets) {
        const linked = next.links.assetIds.includes(asset.id)
        const has = asset.questIds.includes(next.id)
        if (linked && !has) asset.questIds.push(next.id)
        if (!linked && has) asset.questIds = asset.questIds.filter((id) => id !== next.id)
      }
      commit()
      return clone(next)
    },

    async deleteQuest(id) {
      const record = requireRecord(db)
      const before = record.quests.length
      record.quests = record.quests.filter((quest) => quest.id !== id)
      if (record.quests.length === before) {
        throw new AppError('QUEST_NOT_FOUND', '任务不存在')
      }
      for (const asset of record.assets) {
        asset.questIds = asset.questIds.filter((questId) => questId !== id)
      }
      commit()
    },

    async listEntities() {
      const record = currentRecord(db)
      if (!record) return []
      return clone(
        [...(record.entities ?? [])].sort(
          (a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
        ),
      )
    },

    async saveEntity(entity) {
      const record = requireRecord(db)
      const next = normalizeMockEntity(entity)
      record.entities ??= []
      const index = record.entities.findIndex((item) => item.id === next.id)
      if (index >= 0) record.entities[index] = next
      else record.entities.push(next)
      commit()
      return clone(next)
    },

    async deleteEntity(id) {
      const record = requireRecord(db)
      record.entities ??= []
      const before = record.entities.length
      record.entities = record.entities.filter((entity) => entity.id !== id)
      if (record.entities.length === before) {
        throw new AppError('ENTITY_NOT_FOUND', '设定不存在')
      }
      // Relations to a deleted entry would otherwise dangle in the index.
      for (const entity of record.entities) {
        entity.relations = entity.relations.filter((relation) => relation.entityId !== id)
      }
      commit()
    },

    async listTables() {
      const record = currentRecord(db)
      if (!record) return []
      return clone(
        [...(record.tables ?? [])].sort(
          (a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
        ),
      )
    },

    async saveTable(table) {
      const record = requireRecord(db)
      const next = normalizeMockTable(table)
      record.tables ??= []
      const conflict = record.tables.find((item) => item.id !== next.id && item.key === next.key)
      if (conflict) {
        throw new AppError(
          'TABLE_KEY_TAKEN',
          `表键「${next.key}」已被「${conflict.name}」使用`,
          '表键导出为一组文件，项目内必须唯一。',
        )
      }
      const index = record.tables.findIndex((item) => item.id === next.id)
      if (index >= 0) record.tables[index] = next
      else record.tables.push(next)
      commit()
      return clone(next)
    },

    async deleteTable(id) {
      const record = requireRecord(db)
      record.tables ??= []
      const target = record.tables.find((table) => table.id === id)
      if (!target) throw new AppError('TABLE_NOT_FOUND', '数值表不存在')
      const users = record.tables.filter(
        (table) =>
          table.id !== id &&
          table.columns.some((column) => column.type === 'ref' && column.refTableId === id),
      )
      if (users.length > 0) {
        throw new AppError(
          'TABLE_IN_USE',
          '这张表还被其他表引用，不能删除',
          `${users.map((table) => table.name).join('、')} 把它的行当作引用目标。先把这些列改成别的表。`,
        )
      }
      record.tables = record.tables.filter((table) => table.id !== id)
      commit()
    },

    async exportTable(id) {
      const record = requireRecord(db)
      const tables = record.tables ?? []
      const table = tables.find((item) => item.id === id)
      if (!table) throw new AppError('TABLE_NOT_FOUND', '数值表不存在')
      const issues = tableIssues(table, tables)
      if (issues.length > 0) {
        throw new AppError(
          'TABLE_INVALID',
          `数值表还有 ${issues.length} 个问题，先修正再导出`,
          issues.slice(0, 5).map((issue) => issue.message).join('\n'),
        )
      }
      return {
        dir: `${record.session.path}\\exports\\tables\\${table.key}`,
        files: [`${table.key}.json`, `${table.key}.csv`],
      }
    },

    async listAssets() {
      const record = currentRecord(db)
      if (!record) return []
      return clone(
        [...record.assets].sort(
          (a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
        ),
      )
    },

    async saveAsset(asset) {
      const record = requireRecord(db)
      const index = record.assets.findIndex((item) => item.id === asset.id)
      if (index < 0) throw new AppError('ASSET_NOT_FOUND', '素材文件不存在', asset.id)
      const name = asset.name.trim()
      if (!name) throw new AppError('INVALID_ASSET', '素材名称不能为空')
      const next: Asset = { ...clone(asset), name, updatedAt: nowISO() }
      record.assets[index] = next
      commit()
      return clone(next)
    },

    async deleteAsset(id) {
      const record = requireRecord(db)
      const index = record.assets.findIndex((item) => item.id === id)
      if (index < 0) throw new AppError('ASSET_NOT_FOUND', '素材文件不存在', id)
      record.assets.splice(index, 1)
      if (record.thumbs) delete record.thumbs[id]
      commit()
    },

    async listMaps() {
      const record = currentRecord(db)
      if (!record) return []
      return recordMaps(record)
        .map(mapSummary)
        .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
    },

    async loadMap(id) {
      const record = requireRecord(db)
      const map = recordMaps(record).find((item) => item.id === id)
      if (!map) throw new AppError('MAP_NOT_FOUND', '地图不存在', id)
      return clone(map)
    },

    async saveMap(map) {
      const record = requireRecord(db)
      const next = normalizeMockMap(map)
      const maps = recordMaps(record)
      const index = maps.findIndex((item) => item.id === next.id)
      if (index >= 0) maps[index] = next
      else maps.push(next)
      commit()
      return clone(next)
    },

    async expandMap(id, options) {
      const record = requireRecord(db)
      const map = recordMaps(record).find((item) => item.id === id)
      if (!map) throw new AppError('MAP_NOT_FOUND', '地图不存在', id)
      const edges = [options.top, options.right, options.bottom, options.left].map((edge) =>
        Math.max(0, Math.trunc(edge)),
      )
      if (edges.some((edge) => edge > 256)) {
        throw new AppError('INVALID_MAP', '单边最多扩展 256 格')
      }
      const [top, right, bottom, left] = edges as [number, number, number, number]
      if (top + right + bottom + left === 0) {
        throw new AppError('INVALID_MAP', '请至少在一个方向扩展')
      }
      const next = clone(map)
      next.width += left + right
      next.height += top + bottom
      if (next.width > 1024 || next.height > 1024) {
        throw new AppError('INVALID_MAP', '扩展后地图不能超过 1024 格')
      }
      if (left || top) {
        for (const layer of next.layers) {
          for (const tile of layer.tiles) {
            tile.x += left
            tile.y += top
          }
        }
        for (const object of next.objects) {
          object.x += left
          object.y += top
        }
      }
      const saved = normalizeMockMap(next)
      const index = recordMaps(record).findIndex((item) => item.id === saved.id)
      if (index >= 0) record.maps![index] = saved
      commit()
      return clone(saved)
    },

    async deleteMap(id) {
      const record = requireRecord(db)
      const before = recordMaps(record).length
      record.maps = recordMaps(record).filter((map) => map.id !== id)
      if (record.maps.length === before) {
        throw new AppError('MAP_NOT_FOUND', '地图不存在', id)
      }
      commit()
    },

    async exportMap(id, format = 'tiled') {
      const record = requireRecord(db)
      const map = recordMaps(record).find((item) => item.id === id)
      if (!map) throw new AppError('MAP_NOT_FOUND', '地图不存在', id)
      const folder = mockMapFolder(record, map)
      const dir = `${record.session.path}\\exports\\maps\\${format === 'godot' ? `${folder}-godot` : folder}`
      const base = safeExportName(map.name)
      const images = map.tilesets.map((set, index) => {
        const asset = record.assets.find((item) => item.id === set.assetId)
        return `tileset-${index + 1}-${safeExportName(asset?.name ?? set.id)}.png`
      })
      const files: string[] = []
      if (format !== 'godot') {
        images.forEach((image) => files.push(image.replace(/\.png$/, '.tsx'), image))
        files.push(`${base}.tmx`, `${base}.json`)
      }
      if (format !== 'tiled') {
        files.push(
          'project.godot',
          `${base}.tscn`,
          `${base}.tres`,
          `${base}.json`,
          'pixelbench_tiles.gd',
          'roof_fade.gd',
          'README.txt',
          ...images,
        )
      }
      return { dir, files, format }
    },

    async listPalettes() {
      const record = currentRecord(db)
      if (!record) return []
      return clone(record.palettes)
    },

    async savePalette(palette) {
      const record = requireRecord(db)
      const name = palette.name.trim()
      if (!name) throw new AppError('INVALID_PALETTE', '调色板名称不能为空')
      if (palette.colors.length < 1) throw new AppError('INVALID_PALETTE', '调色板至少需要一种颜色')
      const next: Palette = { ...clone(palette), name, colors: [...palette.colors] }
      const index = record.palettes.findIndex((item) => item.id === next.id)
      if (index >= 0) record.palettes[index] = next
      else record.palettes.push(next)
      commit()
      return clone(next)
    },

    async deletePalette(id) {
      const record = requireRecord(db)
      if (id === record.session.project.settings.defaultPaletteId) {
        throw new AppError('PALETTE_DEFAULT', '不能删除项目默认调色板，先把默认切换成其他调色板')
      }
      const assets = record.assets.filter((asset) => asset.paletteId === id).length
      const quests = record.quests.filter((quest) => quest.brief?.paletteId === id).length
      if (assets + quests > 0) {
        throw new AppError(
          'PALETTE_IN_USE',
          `这个调色板还被 ${assets} 个素材、${quests} 个任务引用，先改掉引用再删`,
        )
      }
      const index = record.palettes.findIndex((item) => item.id === id)
      if (index < 0) throw new AppError('PALETTE_NOT_FOUND', '调色板不存在')
      record.palettes.splice(index, 1)
      commit()
    },

    async getAISettings() {
      return aiSettingsView(aiState(db).settings)
    },

    async saveAISettings(input) {
      const state = aiState(db)
      const next = state.settings
      next.defaultProvider = input.defaultProvider === 'openai' ? 'openai' : 'comfyui'
      next.comfy = {
        baseUrl: input.comfy.baseUrl.trim() || 'http://127.0.0.1:8188',
        workflowName: input.comfy.workflowName.trim() || '默认工作流',
        workflow: input.comfy.workflow,
      }
      next.openai = {
        baseUrl: input.openai.baseUrl.trim() || 'https://api.openai.com/v1',
        model: input.openai.model.trim() || 'gpt-image-1',
        editModel: (input.openai.editModel ?? '').trim(),
        textModel: input.openai.textModel.trim() || 'gpt-4o-mini',
        size: input.openai.size.trim() || '1024x1024',
        apiKey: next.openai.apiKey,
        textProvider: input.openai.textProvider === 'own' ? 'own' : 'shared',
        textBaseUrl: (input.openai.textBaseUrl ?? '').trim().replace(/\/+$/, ''),
        textApiKey: next.openai.textApiKey,
      }
      if (input.openai.clearKey) {
        delete next.openai.apiKey
      } else if (input.openai.apiKey?.trim()) {
        next.openai.apiKey = input.openai.apiKey.trim()
      }
      if (input.openai.textClearKey) {
        delete next.openai.textApiKey
      } else if (input.openai.textApiKey?.trim()) {
        next.openai.textApiKey = input.openai.textApiKey.trim()
      }
      commit()
      return aiSettingsView(next)
    },

    async testAIProvider(id: AIProviderId): Promise<AITestResult> {
      const settings = aiState(db).settings
      const target = id === 'openai' ? settings.openai.baseUrl : settings.comfy.baseUrl
      if (!target.trim()) {
        throw new AppError('PROVIDER_WORKFLOW', '请先填写接口地址')
      }
      if (id === 'openai' && !settings.openai.apiKey) {
        throw new AppError('PROVIDER_AUTH', '请先填写 API Key')
      }
      if (id === 'comfyui') {
        return {
          ok: true,
          message: '浏览器预览：已模拟连接',
          detail: `${target}（浏览器模式不会真正请求）`,
        }
      }
      const own = settings.openai.textProvider === 'own'
      const textBase = own ? (settings.openai.textBaseUrl ?? '') : settings.openai.baseUrl
      const textKey = own ? settings.openai.textApiKey : settings.openai.apiKey
      const text: AITestResult = textBase.trim() && textKey
        ? {
            ok: true,
            message: '浏览器预览：文本模型已模拟连接',
            detail: `${settings.openai.textModel} @ ${textBase}（浏览器模式不会真正请求）`,
          }
        : {
            ok: false,
            message: '文本接口未配置',
            detail: '填写文本接口地址与 API Key，或改为沿用图像接口。',
          }
      return {
        ok: true,
        message: '浏览器预览：已模拟连接',
        detail: `${target}（浏览器模式不会真正请求）`,
        text,
      }
    },

    async generateText(request) {
      const settings = aiState(db).settings
      const own = settings.openai.textProvider === 'own'
      const textBase = own ? (settings.openai.textBaseUrl ?? '') : settings.openai.baseUrl
      const textKey = own ? settings.openai.textApiKey : settings.openai.apiKey
      if (!textBase?.trim()) {
        throw new AppError('PROVIDER_AUTH', '文本接口是独立配置，但还没填地址')
      }
      if (!textKey) {
        throw new AppError('PROVIDER_AUTH', 'AI 文案和 AI 起稿需要文本接口的 API Key，先在设置里填写')
      }
      const user = request.user.trim()
      if (!user) throw new AppError('INVALID_TEXT', '没有可交给模型的内容')
      await new Promise((resolve) => setTimeout(resolve, 250))
      const promptTask = /像素美术|素材提示词|生成的提示词/i.test(request.system)
      const lines = user
        .split(/\r?\n/)
        .map((line) => line.trim())
        .filter(Boolean)
      // Lore and table briefs both carry a 名称 line, but not always first.
      const name = user.match(/名称[：:]\s*([^\n，,]+)/)?.[1]?.trim() || '角色'
      const text = promptTask
        ? `${name}，像素风单件素材，3/4 俯视（正交），主体居中，轮廓清晰，1 像素深色描边，受光来自左上，16 色以内，透明背景，底部对齐，无文字无水印`
        : `【浏览器模拟文案】${lines.slice(0, 3).join('；')}`
      return { text, model: 'mock-text' }
    },

    async submitAIJob(request) {
      const record = requireRecord(db)
      const prompt = request.prompt.trim()
      if (!prompt) throw new AppError('INVALID_PROMPT', '请先填写提示词')
      const mode = request.mode === 'inpaint' ? 'inpaint' : 'generate'
      if (mode === 'inpaint' && (!request.baseImage || !request.mask)) {
        throw new AppError('INVALID_INPAINT', '局部重绘需要原图与蒙版')
      }
      const now = nowISO()
      const job: AIJob = {
        id: newID(),
        provider: request.provider === 'openai' ? 'openai' : 'comfyui',
        state: 'queued',
        progress: 0,
        stage: '排队中',
        prompt,
        negativePrompt: request.negativePrompt.trim(),
        width: clampInt(request.width, mode === 'inpaint' ? 1 : 64, 2048),
        height: clampInt(request.height, mode === 'inpaint' ? 1 : 64, 2048),
        count: mode === 'inpaint' ? 1 : clampInt(request.count, 1, 4),
        seed: Math.trunc(request.seed) || 0,
        assetType: ASSET_TYPES.includes(request.assetType as AssetType)
          ? request.assetType
          : 'sprite',
        name: request.name.trim(),
        mode,
        targetAssetId: request.targetAssetId?.trim() || undefined,
        images: [],
        createdAt: now,
        updatedAt: now,
      }
      if (mode === 'inpaint') {
        mockInpaint.set(job.id, {
          url: aiInpaintSVG(request.baseImage!, job.width, job.height, job.id),
          width: job.width,
          height: job.height,
        })
      }
      // The demo project is the only thing a mock job needs to exist in.
      void record
      aiState(db).jobs.push(job)
      commit()
      emitAIJobs()
      scheduleAIJob(job)
      return clone(job)
    },

    async cancelAIJob(id) {
      patchAIJob(id, (job) => {
        if (job.state !== 'queued' && job.state !== 'running') return false
        job.state = 'canceled'
        job.progress = 0
        job.stage = '已取消'
      })
      mockInpaint.delete(id)
      const job = aiState(db).jobs.find((item) => item.id === id)
      if (!job) throw new AppError('JOB_NOT_FOUND', '任务不存在', id)
      return clone(job)
    },

    async deleteAIJob(id) {
      const state = aiState(db)
      const index = state.jobs.findIndex((item) => item.id === id)
      if (index < 0) throw new AppError('JOB_NOT_FOUND', '任务不存在', id)
      const job = state.jobs[index]!
      if (job.state === 'queued' || job.state === 'running') {
        throw new AppError('JOB_ACTIVE', '任务进行中，请先取消再删除')
      }
      state.jobs.splice(index, 1)
      mockInpaint.delete(id)
      commit()
      emitAIJobs()
    },

    async listAIJobs() {
      return clone(aiState(db).jobs)
    },

    async adoptAIJobImage(jobId, index, name) {
      const record = requireRecord(db)
      const job = aiState(db).jobs.find((item) => item.id === jobId)
      if (!job) throw new AppError('JOB_NOT_FOUND', '任务不存在', jobId)
      if (job.state !== 'done') throw new AppError('JOB_NOT_READY', '任务尚未完成，不能采用')
      if (job.mode === 'inpaint') {
        throw new AppError('INPAINT_ADOPT', '局部重绘的结果请在素材详情里采用为版本')
      }
      const image = job.images[index]
      if (!image) throw new AppError('JOB_NOT_READY', '生成结果序号超出范围')
      if (image.adoptedAssetId) {
        const existing = record.assets.find((asset) => asset.id === image.adoptedAssetId)
        if (existing) return clone(existing)
      }
      const now = nowISO()
      const id = newID()
      const asset: Asset = {
        schemaVersion: 1,
        id,
        name: name.trim() || job.name.trim() || `AI 素材 ${index + 1}`,
        type: job.assetType,
        tags: ['生成', 'AI'],
        status: 'draft',
        dimensions: { w: image.width, h: image.height },
        pivot: { x: Math.trunc(image.width / 2), y: Math.max(0, image.height - 1) },
        paletteId: record.session.project.settings.defaultPaletteId,
        questIds: [],
        license: 'generated',
        source: { kind: 'generated', provider: job.provider, workflow: 'ai-image' },
        currentVersion: '0001',
        versions: [
          {
            id: '0001',
            createdAt: now,
            hash: hashString(`${id}:${job.id}:${index}`).toString(16).padStart(8, '0').repeat(8),
            metaFile: 'versions/0001/meta.json',
            size: Math.round((image.url.length * 3) / 4),
          },
        ],
        createdAt: now,
        updatedAt: now,
      }
      record.assets.push(asset)
      record.thumbs = { ...record.thumbs, [asset.id]: image.url }
      image.adoptedAssetId = asset.id
      commit()
      emitAIJobs()
      return clone(asset)
    },

    async adoptAIJobSheet(request) {
      const record = requireRecord(db)
      const job = aiState(db).jobs.find((item) => item.id === request.jobId)
      if (!job) throw new AppError('JOB_NOT_FOUND', '任务不存在', request.jobId)
      if (job.state !== 'done') throw new AppError('JOB_NOT_READY', '任务尚未完成，不能采用')
      const image = job.images[request.index]
      if (!image) throw new AppError('JOB_NOT_READY', '生成结果序号超出范围')
      const columns = Math.min(16, Math.max(1, Math.trunc(request.columns || 1)))
      const rows = Math.min(16, Math.max(1, Math.trunc(request.rows || 1)))
      if (columns * rows > 64) throw new AppError('INVALID_SHEET', '一张精灵表最多 64 帧')
      const frameSize = Math.min(512, Math.max(8, Math.trunc(request.frameSize || 64)))
      const now = nowISO()
      const id = newID()
      const tags = ['生成', 'AI', ...request.tags.map((tag) => tag.trim()).filter(Boolean)]
      const asset: Asset = {
        schemaVersion: 1,
        id,
        name: request.name.trim() || job.name.trim() || 'AI 精灵表',
        type: request.assetType || job.assetType,
        tags,
        status: 'draft',
        dimensions: { w: frameSize + 2, h: frameSize + 2 },
        pivot: { x: Math.trunc((frameSize + 2) / 2), y: frameSize + 1 },
        paletteId: request.paletteId || record.session.project.settings.defaultPaletteId,
        questIds: [],
        license: 'generated',
        source: { kind: 'generated', provider: job.provider, workflow: 'ai-sheet' },
        animation: {
          fps: Math.min(60, Math.max(1, Math.trunc(request.fps || 8))),
          loop: true,
          tags: [...request.tags],
          frameWidth: frameSize + 2,
          frameHeight: frameSize + 2,
          frameCount: columns * rows,
          columns,
        },
        currentVersion: '0001',
        versions: [
          {
            id: '0001',
            createdAt: now,
            hash: hashString(`${id}:${job.id}:${request.index}:sheet`).toString(16).padStart(8, '0').repeat(8),
            metaFile: 'versions/0001/meta.json',
            size: Math.round((image.url.length * 3) / 4),
          },
        ],
        createdAt: now,
        updatedAt: now,
      }
      record.assets.push(asset)
      record.thumbs = { ...record.thumbs, [asset.id]: image.url }
      commit()
      return clone(asset)
    },

    onAIJobs(listener) {
      aiListeners.add(listener)
      return () => {
        aiListeners.delete(listener)
      }
    },

    async importAssets(paths, type) {
      const record = requireRecord(db)
      const summary: ImportSummary = { imported: [], duplicates: [], failed: [] }
      const now = nowISO()
      for (const raw of paths) {
        const path = raw.trim()
        if (!path) continue
        if (!/\.(png|jpe?g|gif|webp|bmp)$/i.test(path)) {
          summary.failed.push({ path, message: '不支持的格式（支持 PNG / JPEG / GIF / WebP / BMP）' })
          continue
        }
        const hash = `mock:${path.toLowerCase()}`
        const existing = record.assets.find((asset) => asset.sourceHash === hash)
        if (existing) {
          summary.duplicates.push({ path, assetId: existing.id, name: existing.name })
          continue
        }
        const base = path.replace(/\\/g, '/').split('/').pop() ?? ''
        const name = base.replace(/\.[^.]+$/, '').trim() || '未命名素材'
        const seed = hashString(path)
        const w = 16 + (seed % 5) * 8
        const h = 16 + ((seed >>> 3) % 5) * 8
        const asset: Asset = {
          schemaVersion: 1,
          id: newID(),
          name,
          type: type as AssetType,
          tags: ['导入'],
          status: 'draft',
          dimensions: { w, h },
          pivot: { x: w / 2, y: h - 1 },
          paletteId: record.session.project.settings.defaultPaletteId,
          questIds: [],
          license: 'imported',
          source: { kind: 'imported' },
          sourceHash: hash,
          versions: [],
          createdAt: now,
          updatedAt: now,
        }
        record.assets.push(asset)
        summary.imported.push(clone(asset))
      }
      commit()
      return clone(summary)
    },

    async createBlankAsset(input) {
      const record = requireRecord(db)
      const name = input.name.trim()
      if (!name) throw new AppError('INVALID_ASSET', '素材名称不能为空')
      if (name.length > 80) throw new AppError('INVALID_ASSET', '素材名称不能超过 80 个字符')
      if (!ASSET_TYPES.includes(input.type)) {
        throw new AppError('INVALID_ASSET', '素材类型不合法', input.type)
      }
      const width = Math.round(Number(input.width) || 0)
      const height = Math.round(Number(input.height) || 0)
      if (width < 1 || height < 1 || width > 2048 || height > 2048) {
        throw new AppError('INVALID_ASSET', '画布尺寸需在 1–2048 像素之间')
      }
      const now = nowISO()
      const asset: Asset = {
        schemaVersion: 1,
        id: newID(),
        name,
        type: input.type,
        tags: [],
        status: 'draft',
        dimensions: { w: width, h: height },
        pivot: { x: width / 2, y: height - 1 },
        paletteId: input.paletteId || record.session.project.settings.defaultPaletteId,
        questIds: [],
        license: 'original',
        source: { kind: 'blank' },
        sourceHash: '',
        versions: [],
        createdAt: now,
        updatedAt: now,
      }
      record.assets.push(asset)
      commit()
      return clone(asset)
    },

    async pickImageFiles() {
      const answer = await promptPath(
        '输入图片路径（多个用 ; 分隔）',
        'C:\\PixelBench\\导入\\新素材.png',
      )
      if (!answer) return []
      return answer
        .split(/[;\n]/)
        .map((item) => item.trim())
        .filter(Boolean)
    },

    async pickDirectory(title = '选择文件夹') {
      return promptPath(title, 'C:\\PixelBench')
    },

    async pickProjectFile() {
      return promptPath('打开 Indie Studio 项目（输入 project.json 路径）', 'C:\\PixelBench\\示例项目\\小镇广场\\project.json')
    },

    async openProjectFolder() {
      /* browser preview has no Explorer; intentionally a no-op */
    },

    async openLogsFolder() {
      /* browser preview has no log folder; intentionally a no-op */
    },

    async assetURL(assetId) {
      const record = currentRecord(db)
      const asset = record?.assets.find((item) => item.id === assetId)
      if (!asset) throw new AppError('ASSET_NOT_FOUND', '素材文件不存在', assetId)
      return record?.thumbs?.[assetId] ?? assetThumb(asset)
    },

    async assetSource(assetId) {
      const record = currentRecord(db)
      const asset = record?.assets.find((item) => item.id === assetId)
      if (!asset) throw new AppError('ASSET_NOT_FOUND', '素材文件不存在', assetId)
      const animation = asset.animation
      if (animation?.frameWidth && animation?.frameHeight) {
        // The browser mock cannot crop PNG bytes without a canvas; reopen an
        // empty frame of the right size instead of the whole sheet.
        return {
          file: 'frames/000.png',
          url: blankFrame(animation.frameWidth, animation.frameHeight),
        }
      }
      return {
        file: record?.thumbs?.[assetId] ? 'thumb.png' : 'source/placeholder.png',
        url: record?.thumbs?.[assetId] ?? assetThumb(asset),
      }
    },

    async saveAssetVersion(assetId, pngDataURL) {
      const record = requireRecord(db)
      const asset = record.assets.find((item) => item.id === assetId)
      if (!asset) throw new AppError('ASSET_NOT_FOUND', '素材文件不存在', assetId)
      const data = pngDataURL.trim()
      if (!data.startsWith('data:image/png')) {
        throw new AppError('INVALID_VERSION', '图片数据格式不正确')
      }
      const size = pngSize(data)
      if (!size) throw new AppError('INVALID_VERSION', '无法解析图片数据')
      const now = nowISO()
      const last = asset.versions.reduce((max, version) => Math.max(max, Number(version.id) || 0), 0)
      const id = String(last + 1).padStart(4, '0')
      const oldDimensions = asset.dimensions
      const stillPivot =
        oldDimensions.w > 0 &&
        oldDimensions.h > 0 &&
        (oldDimensions.w !== size.w || oldDimensions.h !== size.h)
          ? {
              x: Math.max(
                0,
                Math.min(size.w - 1, Math.round((asset.pivot.x * size.w) / oldDimensions.w)),
              ),
              y: Math.max(
                0,
                Math.min(size.h - 1, Math.round((asset.pivot.y * size.h) / oldDimensions.h)),
              ),
            }
          : asset.pivot
      const next: Asset = {
        ...clone(asset),
        dimensions: size,
        pivot: stillPivot,
        animation: undefined,
        currentVersion: id,
        versions: [
          ...asset.versions,
          {
            id,
            createdAt: now,
            hash: hashString(data).toString(16).padStart(8, '0').repeat(8),
            metaFile: `versions/${id}/meta.json`,
            size: Math.round((data.length * 3) / 4),
          },
        ],
        updatedAt: now,
      }
      const index = record.assets.findIndex((item) => item.id === assetId)
      record.assets[index] = next
      record.thumbs = { ...record.thumbs, [assetId]: data }
      commit()
      return clone(next)
    },

    async saveAssetSheet(assetId, pngDataURL, sheet) {
      const record = requireRecord(db)
      const asset = record.assets.find((item) => item.id === assetId)
      if (!asset) throw new AppError('ASSET_NOT_FOUND', '素材文件不存在', assetId)
      const data = pngDataURL.trim()
      if (!data.startsWith('data:image/png')) {
        throw new AppError('INVALID_VERSION', '图片数据格式不正确')
      }
      const size = pngSize(data)
      if (!size) throw new AppError('INVALID_VERSION', '无法解析图片数据')
      const expected = sheetSize(
        sheet.frameWidth,
        sheet.frameHeight,
        sheet.frameCount,
        sheet.columns,
        sheet.margin,
        sheet.spacing,
      )
      if (size.w !== expected.width || size.h !== expected.height) {
        throw new AppError(
          'INVALID_SHEET',
          '精灵表尺寸与布局不一致',
          `图片为 ${size.w}×${size.h}，按布局应为 ${expected.width}×${expected.height}`,
        )
      }
      const now = nowISO()
      const last = asset.versions.reduce((max, version) => Math.max(max, Number(version.id) || 0), 0)
      const id = String(last + 1).padStart(4, '0')
      const uniform = Math.max(1, Math.round(1000 / Math.max(1, sheet.fps)))
      const tags = Array.from(
        new Set((sheet.tags ?? []).map((tag) => tag.trim()).filter(Boolean)),
      ).slice(0, 12)
      const oldDimensions = asset.dimensions
      const pivot =
        oldDimensions.w > 0 &&
        oldDimensions.h > 0 &&
        (oldDimensions.w !== sheet.frameWidth || oldDimensions.h !== sheet.frameHeight)
          ? {
              x: Math.max(
                0,
                Math.min(
                  sheet.frameWidth - 1,
                  Math.round((asset.pivot.x * sheet.frameWidth) / oldDimensions.w),
                ),
              ),
              y: Math.max(
                0,
                Math.min(
                  sheet.frameHeight - 1,
                  Math.round((asset.pivot.y * sheet.frameHeight) / oldDimensions.h),
                ),
              ),
            }
          : asset.pivot
      const next: Asset = {
        ...clone(asset),
        dimensions: { w: sheet.frameWidth, h: sheet.frameHeight },
        pivot,
        animation: {
          fps: sheet.fps,
          loop: sheet.loop,
          tags: tags.length > 0 ? tags : (asset.animation?.tags ?? []),
          frameWidth: sheet.frameWidth,
          frameHeight: sheet.frameHeight,
          frameCount: sheet.frameCount,
          columns: sheet.columns,
          ...(sheet.durations && sheet.durations.length > 0
            ? {
                frameDurations: Array.from({ length: sheet.frameCount }, (_, index) => {
                  const raw = sheet.durations?.[index]
                  return raw !== undefined && raw > 0
                    ? Math.min(10000, Math.max(10, Math.round(raw)))
                    : uniform
                }),
              }
            : {}),
        },
        currentVersion: id,
        versions: [
          ...asset.versions,
          {
            id,
            createdAt: now,
            hash: hashString(data).toString(16).padStart(8, '0').repeat(8),
            metaFile: `versions/${id}/meta.json`,
            size: Math.round((data.length * 3) / 4),
          },
        ],
        updatedAt: now,
      }
      const index = record.assets.findIndex((item) => item.id === assetId)
      record.assets[index] = next
      record.thumbs = { ...record.thumbs, [assetId]: data }
      commit()
      return clone(next)
    },

    async exportAsset(assetId) {
      const record = requireRecord(db)
      const asset = record.assets.find((item) => item.id === assetId)
      if (!asset) throw new AppError('ASSET_NOT_FOUND', '素材文件不存在', assetId)
      if (!asset.currentVersion) {
        throw new AppError('NO_VERSION', '这个素材还没有可导出的版本')
      }
      const dir = `${record.session.path}\\exports\\${mockExportFolder(record, asset)}\\${asset.currentVersion}`
      const files = ['output.png', 'meta.json']
      if (asset.animation) {
        files.push('frames-aseprite.json', 'frames-texturepacker.json')
        const count = asset.animation.frameCount ?? 0
        for (let index = 0; index < count; index += 1) {
          files.push(`frames/${String(index).padStart(3, '0')}.png`)
        }
      }
      files.push('export.json')
      return { dir, files }
    },

    async openExportsFolder() {
      /* browser preview has no Explorer; intentionally a no-op */
    },

    async generateBuildings(options) {
      const drafts = generateMockBuildings(options)
      return clone(drafts)
    },

    async adoptBuilding(options, seed, name) {
      const record = requireRecord(db)
      const opts = normalizeBuildingOptions({ ...options, seed, variants: 1 })
      const geom = buildingGeometry(opts)
      const rand = seededRandom(opts.seed)
      let windows = opts.windows
      if (windows === 0 && opts.width > 2 && Math.floor(rand() * 3) === 0) {
        windows = Math.max(1, opts.width - 2)
      }
      const now = nowISO()
      const id = newID()
      const trimmed = name.trim() || `建筑 ${opts.seed}`
      const image = buildingSVG({ ...opts, windows }, geom, windows, rand)
      const asset: Asset = {
        schemaVersion: 1,
        id,
        name: trimmed,
        type: 'building',
        tags: ['生成'],
        status: 'draft',
        dimensions: { w: geom.width, h: geom.height },
        pivot: {
          x: geom.pad + Math.trunc(geom.bodyW / 2),
          y: geom.pad + geom.bodyD + geom.wallH,
        },
        paletteId: record.session.project.settings.defaultPaletteId,
        questIds: [],
        license: 'generated',
        source: { kind: 'generated', provider: 'procedural', workflow: 'building-v1' },
        currentVersion: '0001',
        versions: [
          {
            id: '0001',
            createdAt: now,
            hash: hashString(`${id}:${opts.seed}`).toString(16).padStart(8, '0').repeat(8),
            metaFile: 'versions/0001/meta.json',
            size: Math.round((image.length * 3) / 4),
          },
        ],
        createdAt: now,
        updatedAt: now,
      }
      record.assets.push(asset)
      record.thumbs = { ...record.thumbs, [asset.id]: image }
      commit()
      return clone(asset)
    },

    async previewPostProcess(assetId, options): Promise<PostProcessPreview> {
      const record = requireRecord(db)
      const asset = record.assets.find((item) => item.id === assetId)
      if (!asset) throw new AppError('ASSET_NOT_FOUND', '素材文件不存在', assetId)
      const size = postProcessTargetSize(asset, options)
      return {
        image: record.thumbs?.[assetId] ?? assetThumb(asset),
        width: size.w,
        height: size.h,
        colors: postProcessColors(record, asset, options),
        seam: { leftRight: 0, topBottom: 0 },
        simulated: true,
      }
    },

    async savePostProcessedVersion(assetId, options) {
      const record = requireRecord(db)
      const asset = record.assets.find((item) => item.id === assetId)
      if (!asset) throw new AppError('ASSET_NOT_FOUND', '素材文件不存在', assetId)
      const size = postProcessTargetSize(asset, options)
      const now = nowISO()
      const last = asset.versions.reduce((max, version) => Math.max(max, Number(version.id) || 0), 0)
      const id = String(last + 1).padStart(4, '0')
      const next: Asset = {
        ...clone(asset),
        dimensions: { w: size.w, h: size.h },
        animation: undefined,
        currentVersion: id,
        versions: [
          ...asset.versions,
          {
            id,
            createdAt: now,
            hash: hashString(`${assetId}:${id}:${size.w}x${size.h}`)
              .toString(16)
              .padStart(8, '0')
              .repeat(8),
            metaFile: `versions/${id}/meta.json`,
            size: size.w * size.h * 4,
          },
        ],
        updatedAt: now,
      }
      const index = record.assets.findIndex((item) => item.id === assetId)
      record.assets[index] = next
      commit()
      return clone(next)
    },

    async getEditorSettings() {
      return clone(mockEditorSettings)
    },

    async saveEditorSettings(settings) {
      mockEditorSettings = {
        showGrid: settings.showGrid !== false,
        openMode: settings.openMode === 'actual' ? 'actual' : 'fit',
      }
      return clone(mockEditorSettings)
    },

    async getUpdateSettings() {
      return clone(mockUpdateSettings)
    },

    async saveUpdateSettings(settings) {
      mockUpdateSettings = {
        channel: settings.channel === 'dev' ? 'dev' : 'stable',
        autoCheck: settings.autoCheck,
      }
      return clone(mockUpdateSettings)
    },

    async checkForUpdates() {
      return {
        currentVersion: '0.1.0-dev',
        available: false,
        channel: mockUpdateSettings.channel,
        checkedAt: new Date().toISOString(),
        message: '浏览器预览模式不检查更新。',
      }
    },

    async downloadUpdate() {
      throw new AppError('UPDATE_UNAVAILABLE', '浏览器预览模式无法下载更新', 'mock')
    },
  }
}