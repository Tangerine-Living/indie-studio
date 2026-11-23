import type {
  AIJob,
  AIJobRequest,
  AISheetAdoptRequest,
  AISettings,
  AISettingsInput,
  AITestResult,
  AITextRequest,
  AITextResult,
  AIProviderId,
  Asset,
  AssetExportResult,
  AssetType,
  Bootstrap,
  BuildingOptions,
  CreateBlankAssetInput,
  CreateOptions,
  DataTable,
  Entity,
  GameMap,
  GeneratedBuilding,
  ImportSummary,
  MapExpandOptions,
  MapExportFormat,
  MapSummary,
  Palette,
  PostProcessOptions,
  PostProcessPreview,
  Project,
  Quest,
  RecentProject,
  Session,
  TableExportResult,
  EditorSettings,
  UpdateSettings,
  UpdateStatus,
} from '../domain/types'
import type { SheetInfo } from '../editor/sheet'

/** Relative path and loadable URL of an asset's original image. */
export interface AssetSourceInfo {
  file: string
  url: string
}

/**
 * The single shell boundary. Everything above this interface is plain web
 * code; only `wails.ts` knows about the desktop bindings. Swapping the shell
 * later means writing one more adapter.
 */
export interface Platform {
  readonly kind: 'wails' | 'mock'
  bootstrap(): Promise<Bootstrap>
  listRecent(): Promise<RecentProject[]>

  createProject(options: CreateOptions): Promise<Session>
  openProject(path: string, force?: boolean): Promise<Session>
  closeProject(): Promise<void>
  getSession(): Promise<Session | null>
  saveProject(project: Project): Promise<Session>

  listQuests(): Promise<Quest[]>
  saveQuest(quest: Quest): Promise<Quest>
  deleteQuest(id: string): Promise<void>

  listEntities(): Promise<Entity[]>
  saveEntity(entity: Entity): Promise<Entity>
  /** Moves the setting entry into .workbench/trash/entities/. */
  deleteEntity(id: string): Promise<void>

  listTables(): Promise<DataTable[]>
  saveTable(table: DataTable): Promise<DataTable>
  /** Moves the table into .workbench/trash/tables/; refuses while referenced. */
  deleteTable(id: string): Promise<void>
  /** Writes JSON and CSV under exports/tables/<key>/. */
  exportTable(id: string): Promise<TableExportResult>

  listAssets(): Promise<Asset[]>
  /** Writes one asset manifest (used for status/review changes). */
  saveAsset(asset: Asset): Promise<Asset>
  /** Moves the whole asset folder into .workbench/trash/assets/. */
  deleteAsset(id: string): Promise<void>

  listMaps(): Promise<MapSummary[]>
  loadMap(id: string): Promise<GameMap>
  saveMap(map: GameMap): Promise<GameMap>
  /** Grows the map canvas and shifts existing tiles and objects. */
  expandMap(id: string, options: MapExpandOptions): Promise<GameMap>
  deleteMap(id: string): Promise<void>
  /** Writes Tiled, Godot or both formats under exports/maps/<name>/[-godot]/. */
  exportMap(id: string, format?: MapExportFormat): Promise<AssetExportResult>
  listPalettes(): Promise<Palette[]>
  /** Writes one palette JSON; used by the palette editor and GPL/PAL/HEX import. */
  savePalette(palette: Palette): Promise<Palette>
  deletePalette(id: string): Promise<void>
  /** Copies files into the project, deduplicating by content hash. */
  importAssets(paths: string[], type: AssetType): Promise<ImportSummary>
  createBlankAsset(input: CreateBlankAssetInput): Promise<Asset>
  /** Source image the pixel editor loads. */
  assetSource(assetId: string): Promise<AssetSourceInfo>
  /** Saves a PNG data URL as an immutable asset version and refreshes the thumb. */
  saveAssetVersion(assetId: string, pngDataURL: string): Promise<Asset>
  /** Saves an animation as a sprite-sheet version; frames and JSON are derived. */
  saveAssetSheet(assetId: string, pngDataURL: string, sheet: SheetInfo): Promise<Asset>
  /** Copies the current version into exports/<name>/<version>/. */
  exportAsset(assetId: string): Promise<AssetExportResult>
  openExportsFolder(): Promise<void>
  /** Draws procedural building drafts; each variant is a PNG data URL. */
  generateBuildings(options: BuildingOptions): Promise<GeneratedBuilding[]>
  /** Persists one generated variant as a draft building asset. */
  adoptBuilding(options: BuildingOptions, seed: number, name: string): Promise<Asset>
  /** Runs the pixel-perfect chain without writing anything. */
  previewPostProcess(assetId: string, options: PostProcessOptions): Promise<PostProcessPreview>
  /** Saves the processed result as the asset's next immutable version. */
  savePostProcessedVersion(assetId: string, options: PostProcessOptions): Promise<Asset>

  pickDirectory(title?: string): Promise<string>
  pickProjectFile(): Promise<string>
  /** Native multi-select picker for image files. */
  pickImageFiles(): Promise<string[]>
  openProjectFolder(): Promise<void>
  openLogsFolder(): Promise<void>

  /** Masked AI provider settings (keys never cross the boundary). */
  getAISettings(): Promise<AISettings>
  saveAISettings(input: AISettingsInput): Promise<AISettings>
  /** Connectivity check that does not spend a generation call. */
  testAIProvider(id: AIProviderId): Promise<AITestResult>
  /** Chat completion for copywriting and AI prompt drafting. */
  generateText(request: AITextRequest): Promise<AITextResult>
  submitAIJob(request: AIJobRequest): Promise<AIJob>
  cancelAIJob(id: string): Promise<AIJob>
  /** Removes a finished, failed or canceled job and its generated images. */
  deleteAIJob(id: string): Promise<void>
  listAIJobs(): Promise<AIJob[]>
  /** Stores one finished result as a draft asset. */
  adoptAIJobImage(jobId: string, index: number, name: string): Promise<Asset>
  /** Slices one result into a cleaned directional / FX sheet with animation metadata. */
  adoptAIJobSheet(request: AISheetAdoptRequest): Promise<Asset>
  /** Subscribes to queue updates; returns an unsubscribe function. */
  onAIJobs(listener: (jobs: AIJob[]) => void): () => void

  /** URL the webview can load without base64 IPC. */
  assetURL(assetId: string, file: string): Promise<string>
  getEditorSettings(): Promise<EditorSettings>
  saveEditorSettings(settings: EditorSettings): Promise<EditorSettings>
  getUpdateSettings(): Promise<UpdateSettings>
  saveUpdateSettings(settings: UpdateSettings): Promise<UpdateSettings>
  /** Fetches and verifies the signed release manifest for the chosen channel. */
  checkForUpdates(): Promise<UpdateStatus>
  /** Downloads the installer, verifies its hash, returns the local path. */
  downloadUpdate(status: UpdateStatus): Promise<string>
}