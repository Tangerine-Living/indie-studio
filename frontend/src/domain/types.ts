/**
 * Front-end mirror of the Go project model (`internal/project/model.go`).
 * Keep the JSON field names camelCase and in sync with the Go struct tags.
 */

export const SCHEMA_VERSION = 1

export const QUEST_STATUSES = ['backlog', 'todo', 'doing', 'review', 'done'] as const
export type QuestStatus = (typeof QUEST_STATUSES)[number]

export const QUEST_TYPES = ['asset', 'code', 'design', 'audio', 'marketing'] as const
export type QuestType = (typeof QUEST_TYPES)[number]

export const PRIORITIES = ['low', 'medium', 'high'] as const
export type Priority = (typeof PRIORITIES)[number]

export const ASSET_STATUSES = ['draft', 'review', 'approved', 'deprecated'] as const
export type AssetStatus = (typeof ASSET_STATUSES)[number]

export const ASSET_TYPES = ['sprite', 'building', 'tile', 'prop', 'ui', 'fx', 'other'] as const
export type AssetType = (typeof ASSET_TYPES)[number]

export const ENGINES = ['godot', 'tiled', 'generic'] as const
export type Engine = (typeof ENGINES)[number]

export const PROJECT_STAGES = ['concept', 'prototype', 'production', 'polish', 'release'] as const
export type ProjectStage = (typeof PROJECT_STAGES)[number]

/** Kinds of setting entries in the lore library. */
export const ENTITY_KINDS = ['character', 'place', 'faction', 'item', 'event', 'term'] as const
export type EntityKind = (typeof ENTITY_KINDS)[number]

/** How settled a setting entry is. */
export const ENTITY_STATUSES = ['idea', 'draft', 'stable'] as const
export type EntityStatus = (typeof ENTITY_STATUSES)[number]

/** Column types of the game-data tables. */
export const COLUMN_TYPES = ['text', 'int', 'number', 'bool', 'enum', 'ref'] as const
export type ColumnType = (typeof COLUMN_TYPES)[number]

/** One stored cell; the column type decides which variant is valid. */
export type CellValue = string | number | boolean

export interface TableColumn {
  id: string
  name: string
  /** Engine-facing field name, unique inside the table. */
  key: string
  type: ColumnType
  required: boolean
  options: string[]
  /** Target table for `ref` columns. */
  refTableId?: string
  min?: number
  max?: number
  note?: string
}

export interface TableRow {
  id: string
  /** Values keyed by column ID, so renaming a key never moves data. */
  values: Record<string, CellValue>
}

/** A typed game-data table such as items, skills or enemies. */
export interface DataTable {
  schemaVersion: number
  id: string
  name: string
  /** Engine-facing file/field name, unique across the project. */
  key: string
  note: string
  columns: TableColumn[]
  rows: TableRow[]
  createdAt: string
  updatedAt: string
}

/** One blocking problem that keeps a table from exporting. */
export interface TableIssue {
  rowId?: string
  columnId?: string
  message: string
}

/** Result of writing exports/tables/<key>/. */
export interface TableExportResult {
  dir: string
  files: string[]
}

export const TILE_SIZES = [16, 24, 32, 48] as const

/** Pixels that should never appear in generated art. */
export const DEFAULT_NEGATIVE_PROMPT = '模糊，抗锯齿，照片写实，3D 渲染，文字，水印，多余肢体'

export interface StyleGuide {
  perspective: string
  outline: string
  light: string
  maxColors: number
  /** Prefilled in the generation workbench; empty falls back to the default. */
  negativePrompt: string
  /** Fixed style anchor added to every AI prompt draft. */
  stylePrompt: string
}

export interface ProjectSettings {
  tileSize: number
  defaultPaletteId: string
  pixelScale: number
  uiLanguage: string
  template?: string
  styleGuide: StyleGuide
}

export interface Member {
  id: string
  name: string
  role: string
}

export interface Milestone {
  id: string
  title: string
  dueAt?: string
  done: boolean
}

export interface Project {
  schemaVersion: number
  id: string
  name: string
  createdAt: string
  updatedAt: string
  engine: Engine
  /** Where the project sits in production; shown as a stepper on the dashboard. */
  stage: ProjectStage
  /** Asset whose current version renders as the project cover. */
  coverAssetId?: string
  settings: ProjectSettings
  members: Member[]
  milestones: Milestone[]
}

export interface Session {
  path: string
  project: Project
}

export interface RecentProject {
  path: string
  name: string
  lastOpenedAt: string
}

export interface AppInfo {
  name: string
  version: string
  platform: string
  logPath: string
  mode: 'desktop' | 'browser'
}

export interface Bootstrap {
  info: AppInfo
  recent: RecentProject[]
  session: Session | null
}

export interface Dimensions {
  w: number
  h: number
}

export interface Point {
  x: number
  y: number
}

export interface AssetSource {
  kind: string
  provider?: string
  workflow?: string
}

export interface AssetVersion {
  id: string
  createdAt: string
  hash: string
  metaFile: string
  size: number
}

export interface Animation {
  fps: number
  loop: boolean
  tags: string[]
  frameWidth?: number
  frameHeight?: number
  frameCount?: number
  columns?: number
  /** Per-frame hold time in milliseconds; empty means uniform from `fps`. */
  frameDurations?: number[]
}

/** Formats a map can be exported to. */
export type MapExportFormat = 'tiled' | 'godot' | 'all'

/** Result of copying one asset version into the project's exports/ tree. */
export interface AssetExportResult {
  /** Absolute path of the exported version folder. */
  dir: string
  /** Paths inside `dir`, using forward slashes. */
  files: string[]
  /** Map exports only: which format produced the files. */
  format?: MapExportFormat
}

export interface Asset {
  schemaVersion: number
  id: string
  name: string
  type: AssetType
  tags: string[]
  status: AssetStatus
  dimensions: Dimensions
  pivot: Point
  paletteId: string
  questIds: string[]
  license: string
  source: AssetSource
  /** SHA-256 of the original imported file, used for duplicate detection. */
  sourceHash?: string
  currentVersion?: string
  versions: AssetVersion[]
  animation?: Animation
  createdAt: string
  updatedAt: string
}

export interface ImportDuplicate {
  path: string
  assetId: string
  name: string
}

export interface ImportFailure {
  path: string
  message: string
}

/** Per-file outcome of a batch asset import. */
export interface ImportSummary {
  imported: Asset[]
  duplicates: ImportDuplicate[]
  failed: ImportFailure[]
}

export interface QuestBrief {
  assetType: AssetType | ''
  tileSize: number
  paletteId: string
  style: string
  references: string[]
  prompt: string
}

export interface QuestLinks {
  assetIds: string[]
  mapIds: string[]
  urls: string[]
}

export interface Comment {
  at: string
  author: string
  text: string
}

/** Cuts one finished AI image into a cleaned directional or FX sprite sheet. */
export interface AISheetAdoptRequest {
  jobId: string
  index: number
  name: string
  assetType: AssetType
  columns: number
  rows: number
  frameSize: number
  fps: number
  tags: string[]
  paletteId?: string
  maxColors?: number
  outline?: boolean
  keyBackground?: boolean
}

/** Per-side tile growth for a map canvas expansion. */
export interface MapExpandOptions {
  top: number
  right: number
  bottom: number
  left: number
}

/** Brief handed from a quest to the generation workbench. */
export interface GenerateIntent {
  questId: string
  questTitle: string
  assetType: AssetType | ''
  tileSize: number
  paletteId: string
  style: string
  prompt: string
  /** Set when the brief came from a lore entry or a data-table row. */
  sourceKind?: 'entity' | 'table'
  sourceLabel?: string
  sourceId?: string
  /** True when the AI prompt draft failed and the raw record was used instead. */
  aiFallback?: boolean
  /** Why the AI draft failed, shown as a tooltip on the fallback chip. */
  aiFallbackDetail?: string
}

/** Context handed from the map editor to the generation workbench. */
export interface MapGenerateIntent {
  mapId: string
  mapName: string
  width: number
  height: number
  tileSize: number
  /** Tilesets already on the map, so new assets can match their look. */
  tilesets: string[]
  assetType: AssetType
  prompt: string
}

export interface Quest {
  schemaVersion: number
  id: string
  title: string
  type: QuestType
  status: QuestStatus
  priority: Priority
  assignee?: string
  startAt?: string
  dueAt?: string
  description?: string
  brief: QuestBrief
  acceptance: string[]
  links: QuestLinks
  comments: Comment[]
  createdAt: string
  updatedAt: string
}

/** One free-form key/value fact on a setting entry. */
export interface EntityField {
  key: string
  value: string
}

/** A directed link to another setting entry, labelled in the author's words. */
export interface EntityRelation {
  entityId: string
  label: string
}

/** A character, place, faction, item, event or term in the setting library. */
export interface Entity {
  schemaVersion: number
  id: string
  kind: EntityKind
  name: string
  status: EntityStatus
  aliases: string[]
  summary: string
  body: string
  tags: string[]
  fields: EntityField[]
  relations: EntityRelation[]
  assetIds: string[]
  mapIds: string[]
  /** Confirmed generation prompt reused for this entry. */
  artPrompt: string
  createdAt: string
  updatedAt: string
}

export interface Palette {
  schemaVersion: number
  id: string
  name: string
  colors: string[]
  source?: string
  locked: boolean
}

export interface CreateOptions {
  parentDir: string
  name: string
  engine: Engine
  tileSize: number
  template: string
  paletteId: string
}

/** A new hand-drawn asset: an empty canvas the editor opens on. */
export interface CreateBlankAssetInput {
  name: string
  type: AssetType
  width: number
  height: number
  paletteId?: string
}

/** Inputs of the procedural top-down building generator. */
export interface BuildingOptions {
  seed: number
  width: number
  depth: number
  tileSize: number
  floors: number
  wallColor: string
  roofColor: string
  trimColor: string
  doorColor: string
  windows: number
  sign: boolean
  weathered: number
  shadow: boolean
  outline: boolean
  variants: number
}

export const DITHER_MODES = ['none', 'floyd', 'bayer', 'noise'] as const
export type DitherMode = (typeof DITHER_MODES)[number]

export interface SeamReport {
  /** Mismatch fraction of the left/right wrap seam (0 = seamless). */
  leftRight: number
  /** Mismatch fraction of the top/bottom wrap seam. */
  topBottom: number
}

/** Inputs of the pixel-perfect post-processing chain. */
export interface PostProcessOptions {
  width: number
  height: number
  trim: boolean
  alphaThreshold: number
  maxColors: number
  colors: string[]
  dither: DitherMode
  ditherStrength: number
  cleanup: boolean
  outline: boolean
  outlineColor: string
  checkSeam: boolean
}

/** A/B preview of one post-processing run. */
export interface PostProcessPreview {
  /** PNG data URL of the processed result. */
  image: string
  width: number
  height: number
  colors: string[]
  seam: SeamReport
  /** Browser mock cannot rasterise; it returns the source pic instead. */
  simulated?: boolean
}

/** AI image providers. Keys are stored by the desktop app, never in a project. */
export const AI_PROVIDERS = ['comfyui', 'openai'] as const
export type AIProviderId = (typeof AI_PROVIDERS)[number]

export interface ComfySettings {
  baseUrl: string
  workflowName: string
  workflow: string
}

/** Where chat requests go: the image endpoint, or a dedicated text one. */
export const AI_TEXT_PROVIDERS = ['shared', 'own'] as const
export type AITextProviderMode = (typeof AI_TEXT_PROVIDERS)[number]

export interface OpenAISettings {
  baseUrl: string
  model: string
  /** Optional model for masked edits; empty reuses the image model. */
  editModel: string
  /** Chat model used for copywriting and AI prompt drafting. */
  textModel: string
  size: string
  hasKey: boolean
  textProvider: AITextProviderMode
  textBaseUrl: string
  textHasKey: boolean
}

/** Masked provider settings returned by the backend. */
export interface AISettings {
  defaultProvider: AIProviderId
  comfy: ComfySettings
  openai: OpenAISettings
}

export interface OpenAISettingsInput {
  baseUrl: string
  model: string
  editModel?: string
  textModel: string
  size: string
  /** Leave blank to keep the stored key. */
  apiKey?: string
  clearKey?: boolean
  /** Omit to keep the stored mode. */
  textProvider?: AITextProviderMode
  textBaseUrl?: string
  /** Leave blank to keep the stored text key. */
  textApiKey?: string
  textClearKey?: boolean
}

export interface AISettingsInput {
  defaultProvider: AIProviderId
  comfy: ComfySettings
  openai: OpenAISettingsInput
}

export interface AITextRequest {
  system: string
  user: string
  maxTokens?: number
  temperature?: number
}

export interface AITextResult {
  text: string
  model: string
}

export interface AITestResult {
  ok: boolean
  message: string
  detail?: string
  /** Present for OpenAI-compatible providers: the text-model check. */
  text?: AITestResult
}

export type AIJobState = 'queued' | 'running' | 'done' | 'failed' | 'canceled'

export interface AIJobImage {
  index: number
  file: string
  url: string
  width: number
  height: number
  /** Asset this result was saved as, once adopted. */
  adoptedAssetId?: string
}

export interface AIJob {
  id: string
  provider: AIProviderId
  state: AIJobState
  progress: number
  stage: string
  prompt: string
  negativePrompt: string
  width: number
  height: number
  count: number
  seed: number
  assetType: AssetType
  name: string
  mode?: 'generate' | 'inpaint'
  targetAssetId?: string
  error?: string
  images: AIJobImage[]
  createdAt: string
  updatedAt: string
}

export interface AIJobRequest {
  provider: AIProviderId
  prompt: string
  negativePrompt: string
  width: number
  height: number
  count: number
  seed: number
  assetType: AssetType
  name: string
  mode?: 'generate' | 'inpaint'
  targetAssetId?: string
  baseImage?: string
  mask?: string
}

/** One generated building draft returned by the Go generator. */
export interface GeneratedBuilding {
  seed: number
  width: number
  height: number
  pivot: Point
  collision: { x: number; y: number; w: number; h: number }
  /** PNG data URL for preview. */
  image: string
}

export const MAP_LAYER_TYPES = [
  'ground',
  'decoration',
  'building',
  'roof',
  'object',
  'collision',
  'light',
] as const
export type MapLayerType = (typeof MAP_LAYER_TYPES)[number]

export const MAP_OBJECT_KINDS = ['spawn', 'npc', 'chest', 'trigger', 'building', 'prop'] as const
export type MapObjectKind = (typeof MAP_OBJECT_KINDS)[number]

export const AUTOTILE_MODES = ['16', '47'] as const
export type AutotileMode = (typeof AUTOTILE_MODES)[number]

/** One tile atlas inside a map, backed by an asset. */
export interface MapTileset {
  id: string
  assetId: string
  tileSize: number
  columns: number
  tileCount: number
}

/** Variant family used by terrain brush autotiling. */
export interface MapAutotile {
  set: number
  base: number
  mode: AutotileMode
}

/** One painted cell; coordinates are tile units and storage is sparse. */
export interface MapTile {
  x: number
  y: number
  set: number
  tile: number
  flipH?: boolean
  flipV?: boolean
  rotate?: 0 | 90 | 180 | 270
}

export interface MapLayer {
  id: string
  name: string
  type: MapLayerType
  elevation: number
  visible: boolean
  opacity: number
  autotile?: MapAutotile
  tiles: MapTile[]
}

export interface MapObject {
  id: string
  name: string
  kind: MapObjectKind
  /** Top-left in tile units; may be fractional. */
  x: number
  y: number
  w: number
  h: number
  elevation: number
  ySort: boolean
  roofFade: boolean
  solid: boolean
  assetId?: string
  color?: string
  properties?: Record<string, string>
}

export interface MapSettings {
  ySort: boolean
  roofFade: boolean
  /** Pixels of screen offset per elevation level. */
  heightStep: number
  ambient: string
}

/** A 2.5D orthographic top-down map. */
export interface GameMap {
  schemaVersion: number
  id: string
  name: string
  tileSize: number
  width: number
  height: number
  tilesets: MapTileset[]
  layers: MapLayer[]
  objects: MapObject[]
  settings: MapSettings
  createdAt: string
  updatedAt: string
}

export interface MapSummary {
  id: string
  name: string
  tileSize: number
  width: number
  height: number
  updatedAt: string
}

export type EditorOpenMode = 'fit' | 'actual'

/** App-wide pixel-editor preferences, stored outside the project. */
export interface EditorSettings {
  showGrid: boolean
  openMode: EditorOpenMode
}

export type ReleaseChannel = 'stable' | 'dev'

/** App-level updater configuration, stored in %APPDATA%\PixelBench\update.json. */
export interface UpdateSettings {
  channel: ReleaseChannel
  autoCheck: boolean
}

/** Result of one signed-manifest update check. */
export interface UpdateStatus {
  currentVersion: string
  latestVersion?: string
  available: boolean
  channel: ReleaseChannel
  notes?: string
  pageUrl?: string
  downloadUrl?: string
  downloadSha256?: string
  downloadSize?: number
  publishedAt?: string
  checkedAt: string
  message: string
}

export type Route =
  | 'dashboard'
  | 'quests'
  | 'lore'
  | 'tables'
  | 'assets'
  | 'editor'
  | 'generate'
  | 'maps'
  | 'export'
  | 'settings'