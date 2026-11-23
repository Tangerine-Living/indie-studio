import { z } from 'zod'
import {
  AI_PROVIDERS,
  AI_TEXT_PROVIDERS,
  ASSET_STATUSES,
  ASSET_TYPES,
  AUTOTILE_MODES,
  COLUMN_TYPES,
  DEFAULT_NEGATIVE_PROMPT,
  ENGINES,
  ENTITY_KINDS,
  ENTITY_STATUSES,
  MAP_LAYER_TYPES,
  MAP_OBJECT_KINDS,
  PRIORITIES,
  PROJECT_STAGES,
  QUEST_STATUSES,
  QUEST_TYPES,
} from './types'
import type {
  AIJob,
  AISettings,
  AITestResult,
  AITextResult,
  Asset,
  AssetExportResult,
  Bootstrap,
  DataTable,
  EditorSettings,
  Entity,
  GameMap,
  GeneratedBuilding,
  ImportSummary,
  MapSummary,
  Palette,
  PostProcessPreview,
  Quest,
  QuestBrief,
  QuestLinks,
  RecentProject,
  Session,
  StyleGuide,
  TableExportResult,
  UpdateSettings,
  UpdateStatus,
} from './types'

/**
 * Runtime schemas for everything crossing the platform boundary. The backend
 * validates on write; these schemas catch tampering, hand-edited JSON and
 * mock/browser drift. Arrays use `.catch` so an old file with `null` slices
 * still opens instead of breaking the whole project.
 */
const list = <T extends z.ZodTypeAny>(item: T) => z.array(item).catch([])
const text = (fallback = '') => z.string().catch(fallback)

const styleGuideSchema = z.object({
  perspective: text('3/4-topdown-orthographic'),
  outline: text('1px-dark'),
  light: text('top-left'),
  maxColors: z.number().int().catch(16),
  negativePrompt: text(DEFAULT_NEGATIVE_PROMPT),
  stylePrompt: text(''),
})

const settingsSchema = z.object({
  tileSize: z.number().int().catch(32),
  defaultPaletteId: text('default'),
  pixelScale: z.number().int().catch(4),
  uiLanguage: text('zh-CN'),
  template: z.string().optional(),
  styleGuide: styleGuideSchema.catch({
    perspective: '3/4-topdown-orthographic',
    outline: '1px-dark',
    light: 'top-left',
    maxColors: 16,
    negativePrompt: DEFAULT_NEGATIVE_PROMPT,
    stylePrompt: '',
  }),
})

const memberSchema = z.object({
  id: text(),
  name: text(),
  role: text(),
})

const milestoneSchema = z.object({
  id: text(),
  title: text(),
  dueAt: z.string().optional(),
  done: z.boolean().catch(false),
})

export const projectSchema = z.object({
  schemaVersion: z.number().int().catch(1),
  id: text(),
  name: text('未命名项目'),
  createdAt: text(),
  updatedAt: text(),
  engine: z.enum(ENGINES).catch('godot'),
  stage: z.enum(PROJECT_STAGES).catch('concept'),
  coverAssetId: z.string().optional(),
  settings: settingsSchema,
  members: list(memberSchema),
  milestones: list(milestoneSchema),
})

const sessionSchema = z.object({
  path: text(),
  project: projectSchema,
})

const recentProjectSchema = z.object({
  path: text(),
  name: text(),
  lastOpenedAt: text(),
})

const appInfoSchema = z.object({
  name: text('Indie Studio'),
  version: text('0.0.0'),
  platform: text('unknown'),
  logPath: text(''),
  mode: z.enum(['desktop', 'browser']).catch('browser'),
})

export const bootstrapSchema = z.object({
  info: appInfoSchema,
  recent: list(recentProjectSchema),
  session: sessionSchema.nullable().catch(null),
})

const dimensionsSchema = z.object({
  w: z.number().int().catch(0),
  h: z.number().int().catch(0),
})

const pointSchema = z.object({
  x: z.number().int().catch(0),
  y: z.number().int().catch(0),
})

const assetSourceSchema = z.object({
  kind: text('imported'),
  provider: z.string().optional(),
  workflow: z.string().optional(),
})

const assetVersionSchema = z.object({
  id: text(),
  createdAt: text(),
  hash: text(),
  metaFile: text(),
  size: z.number().catch(0),
})

const animationSchema = z.object({
  fps: z.number().int().catch(8),
  loop: z.boolean().catch(true),
  tags: list(text()),
  frameWidth: z.number().int().positive().optional(),
  frameHeight: z.number().int().positive().optional(),
  frameCount: z.number().int().positive().optional(),
  columns: z.number().int().positive().optional(),
  frameDurations: z.array(z.number().int().positive()).optional().catch(undefined),
})

export const assetSchema = z.object({
  schemaVersion: z.number().int().catch(1),
  id: text(),
  name: text('未命名素材'),
  type: z.enum(ASSET_TYPES).catch('other'),
  tags: list(text()),
  status: z.enum(ASSET_STATUSES).catch('draft'),
  dimensions: dimensionsSchema,
  pivot: pointSchema,
  paletteId: text('default'),
  questIds: list(text()),
  license: text('original'),
  source: assetSourceSchema,
  sourceHash: z.string().optional(),
  currentVersion: z.string().optional(),
  versions: list(assetVersionSchema),
  animation: animationSchema.optional(),
  createdAt: text(),
  updatedAt: text(),
})

const duplicateImportSchema = z.object({
  path: text(),
  assetId: text(),
  name: text(),
})

const failureImportSchema = z.object({
  path: text(),
  message: text('导入失败'),
})

const importSummarySchema = z.object({
  imported: list(assetSchema),
  duplicates: list(duplicateImportSchema),
  failed: list(failureImportSchema),
})

const questBriefSchema = z.object({
  assetType: z.enum(ASSET_TYPES).or(z.literal('')).catch(''),
  tileSize: z.number().int().catch(32),
  paletteId: text('default'),
  style: text(),
  references: list(text()),
  prompt: text(),
})

const questLinksSchema = z.object({
  assetIds: list(text()),
  mapIds: list(text()),
  urls: list(text()),
})

const commentSchema = z.object({
  at: text(),
  author: text(),
  text: text(),
})

export const questSchema = z.object({
  schemaVersion: z.number().int().catch(1),
  id: text(),
  title: text('未命名任务'),
  type: z.enum(QUEST_TYPES).catch('asset'),
  status: z.enum(QUEST_STATUSES).catch('todo'),
  priority: z.enum(PRIORITIES).catch('medium'),
  assignee: z.string().optional(),
  startAt: z.string().optional(),
  dueAt: z.string().optional(),
  description: z.string().optional(),
  brief: questBriefSchema.catch(defaultBrief()),
  acceptance: list(text()),
  links: questLinksSchema.catch({ assetIds: [], mapIds: [], urls: [] }),
  comments: list(commentSchema),
  createdAt: text(),
  updatedAt: text(),
})

const entityFieldSchema = z.object({
  key: text(),
  value: text(),
})

const entityRelationSchema = z.object({
  entityId: text(),
  label: text('相关'),
})

export const entitySchema = z.object({
  schemaVersion: z.number().int().catch(1),
  id: text(),
  kind: z.enum(ENTITY_KINDS).catch('character'),
  name: text('未命名设定'),
  status: z.enum(ENTITY_STATUSES).catch('draft'),
  aliases: list(text()),
  summary: text(),
  body: text(),
  tags: list(text()),
  fields: list(entityFieldSchema),
  relations: list(entityRelationSchema),
  assetIds: list(text()),
  mapIds: list(text()),
  artPrompt: text(),
  createdAt: text(),
  updatedAt: text(),
})

export function parseEntity(value: unknown): Entity {
  return entitySchema.parse(value) as Entity
}

export function parseEntities(value: unknown): Entity[] {
  return z.array(entitySchema).parse(value) as Entity[]
}

const cellValueSchema = z.union([z.string(), z.number(), z.boolean()])

const tableColumnSchema = z.object({
  id: text(),
  name: text('列'),
  key: text('col'),
  type: z.enum(COLUMN_TYPES).catch('text'),
  required: z.boolean().catch(false),
  options: list(text()),
  refTableId: z.string().optional().catch(undefined),
  min: z.number().optional().catch(undefined),
  max: z.number().optional().catch(undefined),
  note: z.string().optional().catch(undefined),
})

const tableRowSchema = z.object({
  id: text(),
  values: z.record(z.string(), cellValueSchema).catch({}),
})

export const dataTableSchema = z.object({
  schemaVersion: z.number().int().catch(1),
  id: text(),
  name: text('未命名数值表'),
  key: text('table'),
  note: text(),
  columns: list(tableColumnSchema),
  rows: list(tableRowSchema),
  createdAt: text(),
  updatedAt: text(),
})

export function parseTable(value: unknown): DataTable {
  return dataTableSchema.parse(value) as DataTable
}

export function parseTables(value: unknown): DataTable[] {
  return z.array(dataTableSchema).parse(value) as DataTable[]
}

const tableExportSchema = z.object({
  dir: text(),
  files: list(text()),
})

export function parseTableExport(value: unknown): TableExportResult {
  return tableExportSchema.parse(value) as TableExportResult
}

export const paletteSchema = z.object({
  schemaVersion: z.number().int().catch(1),
  id: text(),
  name: text('未命名调色板'),
  colors: list(text()),
  source: z.string().optional(),
  locked: z.boolean().catch(false),
})

const mapTilesetSchema = z.object({
  id: text(),
  assetId: text(),
  tileSize: z.number().int().catch(32),
  columns: z.number().int().positive().catch(1),
  tileCount: z.number().int().positive().catch(1),
})

const mapAutotileSchema = z.object({
  set: z.number().int().catch(0),
  base: z.number().int().catch(0),
  mode: z.enum(AUTOTILE_MODES).catch('16'),
})

const mapTileSchema = z.object({
  x: z.number().int().catch(0),
  y: z.number().int().catch(0),
  set: z.number().int().catch(0),
  tile: z.number().int().catch(0),
  flipH: z.boolean().optional().catch(undefined),
  flipV: z.boolean().optional().catch(undefined),
  rotate: z
    .union([z.literal(0), z.literal(90), z.literal(180), z.literal(270)])
    .optional()
    .catch(undefined),
})

const mapLayerSchema = z.object({
  id: text(),
  name: text('图层'),
  type: z.enum(MAP_LAYER_TYPES).catch('ground'),
  elevation: z.number().int().catch(0),
  visible: z.boolean().catch(true),
  opacity: z.number().catch(1),
  autotile: mapAutotileSchema.optional(),
  tiles: list(mapTileSchema),
})

const mapObjectSchema = z.object({
  id: text(),
  name: text('对象'),
  kind: z.enum(MAP_OBJECT_KINDS).catch('prop'),
  x: z.number().catch(0),
  y: z.number().catch(0),
  w: z.number().catch(1),
  h: z.number().catch(1),
  elevation: z.number().int().catch(0),
  ySort: z.boolean().catch(true),
  roofFade: z.boolean().catch(false),
  solid: z.boolean().catch(false),
  assetId: z.string().optional(),
  color: z.string().optional(),
  properties: z.record(z.string(), z.string()).optional().catch(undefined),
})

const mapSettingsSchema = z.object({
  ySort: z.boolean().catch(true),
  roofFade: z.boolean().catch(true),
  heightStep: z.number().int().catch(16),
  ambient: text('#8090a0'),
})

export const mapSchema = z.object({
  schemaVersion: z.number().int().catch(1),
  id: text(),
  name: text('未命名地图'),
  tileSize: z.number().int().catch(32),
  width: z.number().int().catch(32),
  height: z.number().int().catch(24),
  tilesets: list(mapTilesetSchema),
  layers: list(mapLayerSchema),
  objects: list(mapObjectSchema),
  settings: mapSettingsSchema.catch({
    ySort: true,
    roofFade: true,
    heightStep: 16,
    ambient: '#8090a0',
  }),
  createdAt: text(),
  updatedAt: text(),
})

export const mapSummarySchema = z.object({
  id: text(),
  name: text('未命名地图'),
  tileSize: z.number().int().catch(32),
  width: z.number().int().catch(32),
  height: z.number().int().catch(24),
  updatedAt: text(),
})

export function parseMaps(value: unknown): MapSummary[] {
  return z.array(mapSummarySchema).parse(value) as MapSummary[]
}

export function parseMap(value: unknown): GameMap {
  return mapSchema.parse(value) as GameMap
}

function defaultBrief(): QuestBrief {
  return { assetType: '', tileSize: 32, paletteId: 'default', style: '', references: [], prompt: '' }
}

/** Narrowing helpers keep the rest of the app free of `unknown`. */
export function parseBootstrap(value: unknown): Bootstrap {
  return bootstrapSchema.parse(value) as Bootstrap
}

export function parseSession(value: unknown): Session {
  return sessionSchema.parse(value) as Session
}

export function parseQuest(value: unknown): Quest {
  return questSchema.parse(value) as Quest
}

export function parseQuests(value: unknown): Quest[] {
  return z.array(questSchema).parse(value) as Quest[]
}

export function parseAsset(value: unknown): Asset {
  return assetSchema.parse(value) as Asset
}

const assetExportSchema = z.object({
  dir: text(),
  files: list(text()),
  format: z.enum(['tiled', 'godot', 'all']).optional().catch(undefined),
})

export function parseAssetExport(value: unknown): AssetExportResult {
  return assetExportSchema.parse(value) as AssetExportResult
}

const assetSourceInfoSchema = z.object({
  file: text(),
  url: text(),
})

export function parseAssetSourceInfo(value: unknown): { file: string; url: string } {
  return assetSourceInfoSchema.parse(value)
}

export function parseAssets(value: unknown): Asset[] {
  return z.array(assetSchema).parse(value) as Asset[]
}

export function parseImportSummary(value: unknown): ImportSummary {
  return importSummarySchema.parse(value) as ImportSummary
}

export function parsePalettes(value: unknown): Palette[] {
  return z.array(paletteSchema).parse(value) as Palette[]
}

export function parseRecent(value: unknown): RecentProject[] {
  return z.array(recentProjectSchema).parse(value) as RecentProject[]
}


const generatedBuildingSchema = z.object({
  seed: z.number().int(),
  width: z.number().int().positive(),
  height: z.number().int().positive(),
  pivot: z.object({ x: z.number().int(), y: z.number().int() }),
  collision: z.object({
    x: z.number().int(),
    y: z.number().int(),
    w: z.number().int(),
    h: z.number().int(),
  }),
  image: text(),
})

export function parseGeneratedBuildings(value: unknown): GeneratedBuilding[] {
  return z.array(generatedBuildingSchema).parse(value) as GeneratedBuilding[]
}

const seamSchema = z.object({
  leftRight: z.number().catch(0),
  topBottom: z.number().catch(0),
})

const postProcessPreviewSchema = z.object({
  image: text(),
  width: z.number().int().positive(),
  height: z.number().int().positive(),
  colors: list(text()),
  seam: seamSchema.catch({ leftRight: 0, topBottom: 0 }),
  simulated: z.boolean().optional(),
})

export function parsePostProcessPreview(value: unknown): PostProcessPreview {
  return postProcessPreviewSchema.parse(value) as PostProcessPreview
}

const aiProviderSchema = z.enum(AI_PROVIDERS).catch('comfyui')

const comfySettingsSchema = z.object({
  baseUrl: text('http://127.0.0.1:8188'),
  workflowName: text('默认工作流'),
  workflow: text(''),
})

const openAISettingsSchema = z.object({
  baseUrl: text('https://api.openai.com/v1'),
  model: text('gpt-image-1'),
  editModel: text(''),
  textModel: text('gpt-4o-mini'),
  size: text('1024x1024'),
  hasKey: z.boolean().catch(false),
  textProvider: z.enum(AI_TEXT_PROVIDERS).catch('shared'),
  textBaseUrl: text(''),
  textHasKey: z.boolean().catch(false),
})

export const aiSettingsSchema = z.object({
  defaultProvider: aiProviderSchema,
  comfy: comfySettingsSchema.catch({
    baseUrl: 'http://127.0.0.1:8188',
    workflowName: '默认工作流',
    workflow: '',
  }),
  openai: openAISettingsSchema.catch({
    baseUrl: 'https://api.openai.com/v1',
    model: 'gpt-image-1',
    editModel: '',
    textModel: 'gpt-4o-mini',
    size: '1024x1024',
    hasKey: false,
    textProvider: 'shared',
    textBaseUrl: '',
    textHasKey: false,
  }),
})

export function parseAISettings(value: unknown): AISettings {
  return aiSettingsSchema.parse(value) as AISettings
}

const aiTestResultSchema: z.ZodType<AITestResult> = z.lazy(() =>
  z.object({
    ok: z.boolean().catch(false),
    message: text('未知状态'),
    detail: z.string().optional(),
    text: aiTestResultSchema.optional(),
  }),
)

export function parseAITestResult(value: unknown): AITestResult {
  return aiTestResultSchema.parse(value)
}

const aiTextResultSchema = z.object({
  text: text(),
  model: text(),
})

export function parseAITextResult(value: unknown): AITextResult {
  return aiTextResultSchema.parse(value) as AITextResult
}

const aiJobImageSchema = z.object({
  index: z.number().int().catch(0),
  file: text(),
  url: text(),
  width: z.number().int().catch(0),
  height: z.number().int().catch(0),
  adoptedAssetId: z.string().optional().catch(undefined),
})

export const aiJobSchema = z.object({
  id: text(),
  provider: aiProviderSchema,
  state: z.enum(['queued', 'running', 'done', 'failed', 'canceled']).catch('failed'),
  progress: z.number().catch(0),
  stage: text(),
  prompt: text(),
  negativePrompt: text(),
  width: z.number().int().catch(0),
  height: z.number().int().catch(0),
  count: z.number().int().catch(1),
  seed: z.number().catch(0),
  assetType: z.enum(ASSET_TYPES).catch('sprite'),
  name: text(),
  mode: z.enum(['generate', 'inpaint']).optional().catch(undefined),
  targetAssetId: z.string().optional().catch(undefined),
  error: z.string().optional(),
  images: list(aiJobImageSchema),
  createdAt: text(),
  updatedAt: text(),
})

export function parseAIJobs(value: unknown): AIJob[] {
  return z.array(aiJobSchema).parse(value) as AIJob[]
}

export function parseAIJob(value: unknown): AIJob {
  return aiJobSchema.parse(value) as AIJob
}

const releaseChannelSchema = z
  .enum(['stable', 'dev', 'beta'])
  .catch('stable')
  .transform((value) => (value === 'beta' ? 'dev' : value))

const editorSettingsSchema = z.object({
  showGrid: z.boolean().catch(true),
  openMode: z.enum(['fit', 'actual']).catch('fit'),
})

export function parseEditorSettings(value: unknown): EditorSettings {
  return editorSettingsSchema.parse(value) as EditorSettings
}

const updateSettingsSchema = z.object({
  channel: releaseChannelSchema,
  autoCheck: z.boolean().catch(true),
})

const updateStatusSchema = z.object({
  currentVersion: text(),
  latestVersion: z.string().optional().catch(undefined),
  available: z.boolean().catch(false),
  channel: releaseChannelSchema,
  notes: z.string().optional().catch(undefined),
  pageUrl: z.string().optional().catch(undefined),
  downloadUrl: z.string().optional().catch(undefined),
  downloadSha256: z.string().optional().catch(undefined),
  downloadSize: z.number().optional().catch(undefined),
  publishedAt: z.string().optional().catch(undefined),
  checkedAt: text(),
  message: text(),
})

export function parseUpdateSettings(value: unknown): UpdateSettings {
  return updateSettingsSchema.parse(value) as UpdateSettings
}

export function parseUpdateStatus(value: unknown): UpdateStatus {
  return updateStatusSchema.parse(value) as UpdateStatus
}

export type { QuestLinks, StyleGuide }