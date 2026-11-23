import {
  parseAIJob,
  parseAIJobs,
  parseAISettings,
  parseAITestResult,
  parseAITextResult,
  parseAsset,
  parseAssetExport,
  parseAssetSourceInfo,
  parseAssets,
  parseBootstrap,
  parseEntities,
  parseEntity,
  parseEditorSettings,
  parseGeneratedBuildings,
  parseImportSummary,
  parseMap,
  parseMaps,
  parsePalettes,
  parsePostProcessPreview,
  parseQuest,
  parseQuests,
  parseRecent,
  parseSession,
  parseTable,
  parseTableExport,
  parseTables,
  parseUpdateSettings,
  parseUpdateStatus,
} from '../domain/schema'
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
  EditorSettings,
  BuildingOptions,
  CreateBlankAssetInput,
  GeneratedBuilding,
  PostProcessOptions,
  PostProcessPreview,
} from '../domain/types'
import type { Platform } from './types'
import type { SheetInfo } from '../editor/sheet'
import { AppError, normalizeError } from '../lib/errors'

interface WailsBindings {
  GetBootstrap(): Promise<unknown>
  CreateProject(options: unknown): Promise<unknown>
  OpenProject(path: string, force: boolean): Promise<unknown>
  CloseProject(): Promise<void>
  GetSession(): Promise<unknown>
  SaveProject(project: unknown): Promise<unknown>
  ListQuests(): Promise<unknown>
  SaveQuest(quest: unknown): Promise<unknown>
  DeleteQuest(id: string): Promise<void>
  ListEntities(): Promise<unknown>
  SaveEntity(entity: unknown): Promise<unknown>
  DeleteEntity(id: string): Promise<void>
  ListTables(): Promise<unknown>
  SaveTable(table: unknown): Promise<unknown>
  DeleteTable(id: string): Promise<void>
  ExportTable(id: string): Promise<unknown>
  ListAssets(): Promise<unknown>
  SaveAsset(asset: unknown): Promise<unknown>
  DeleteAsset(id: string): Promise<void>
  ListMaps(): Promise<unknown>
  LoadMap(id: string): Promise<unknown>
  SaveMap(map: unknown): Promise<unknown>
  ExpandMap(id: string, options: unknown): Promise<unknown>
  DeleteMap(id: string): Promise<void>
  ExportMap(id: string, format: string): Promise<unknown>
  ListPalettes(): Promise<unknown>
  SavePalette(palette: unknown): Promise<unknown>
  DeletePalette(id: string): Promise<void>
  ImportAssets(options: { paths: string[]; type: string }): Promise<unknown>
  CreateBlankAsset(input: CreateBlankAssetInput): Promise<unknown>
  PickDirectory(title: string): Promise<string>
  PickProjectFile(): Promise<string>
  PickImageFiles(): Promise<string[] | null>
  OpenProjectFolder(): Promise<void>
  OpenLogsFolder(): Promise<void>
  AdoptAIJobSheet(request: AISheetAdoptRequest): Promise<unknown>
  GetAssetURL(assetId: string, file: string): Promise<string>
  GetAssetSource(assetId: string): Promise<unknown>
  SaveAssetVersion(assetId: string, dataURL: string): Promise<unknown>
  SaveAssetSheet(assetId: string, dataURL: string, sheet: SheetInfo): Promise<unknown>
  ExportAsset(assetId: string): Promise<unknown>
  OpenExportsFolder(): Promise<void>
  GenerateBuildings(options: BuildingOptions): Promise<GeneratedBuilding[]>
  AdoptBuilding(options: BuildingOptions, seed: number, name: string): Promise<unknown>
  PreviewPostProcess(assetId: string, options: PostProcessOptions): Promise<unknown>
  SavePostProcessedVersion(assetId: string, options: PostProcessOptions): Promise<unknown>
  GetAISettings(): Promise<unknown>
  SaveAISettings(input: AISettingsInput): Promise<unknown>
  TestAIProvider(id: AIProviderId): Promise<unknown>
  GenerateText(req: AITextRequest): Promise<unknown>
  SubmitAIJob(request: AIJobRequest): Promise<unknown>
  CancelAIJob(id: string): Promise<unknown>
  DeleteAIJob(id: string): Promise<void>
  ListAIJobs(): Promise<unknown>
  AdoptAIJobImage(jobId: string, index: number, name: string): Promise<unknown>
  GetEditorSettings(): Promise<unknown>
  SaveEditorSettings(settings: unknown): Promise<unknown>
  GetUpdateSettings(): Promise<unknown>
  SaveUpdateSettings(settings: unknown): Promise<unknown>
  CheckForUpdates(): Promise<unknown>
  DownloadUpdate(status: unknown): Promise<string>
}

interface WailsRuntime {
  EventsOn(event: string, callback: (...args: unknown[]) => void): () => void
}

declare global {
  interface Window {
    go?: { main?: { App?: WailsBindings } }
    runtime?: WailsRuntime
  }
}

export function hasWailsBridge(): boolean {
  return typeof window !== 'undefined' && Boolean(window.go?.main?.App)
}

function bindings(): WailsBindings {
  const app = window.go?.main?.App
  if (!app) {
    throw new AppError('NO_BINDING', '未检测到桌面端桥接，请通过 Wails 启动应用')
  }
  return app
}

async function call<T>(fn: (app: WailsBindings) => Promise<T>): Promise<T> {
  try {
    return await fn(bindings())
  } catch (raw) {
    throw normalizeError(raw)
  }
}

export function createWailsPlatform(): Platform {
  return {
    kind: 'wails',

    async bootstrap() {
      return parseBootstrap(await call((app) => app.GetBootstrap()))
    },

    async listRecent() {
      const boot = await call((app) => app.GetBootstrap())
      return parseRecent((boot as { recent?: unknown }).recent ?? [])
    },

    async createProject(options) {
      return parseSession(await call((app) => app.CreateProject(options)))
    },

    async openProject(path, force = false) {
      return parseSession(await call((app) => app.OpenProject(path, force)))
    },

    async closeProject() {
      await call((app) => app.CloseProject())
    },

    async getSession() {
      const raw = await call((app) => app.GetSession())
      return raw ? parseSession(raw) : null
    },

    async saveProject(project) {
      return parseSession(await call((app) => app.SaveProject(project)))
    },

    async listQuests() {
      return parseQuests((await call((app) => app.ListQuests())) ?? [])
    },

    async saveQuest(quest) {
      return parseQuest(await call((app) => app.SaveQuest(quest)))
    },

    async deleteQuest(id) {
      await call((app) => app.DeleteQuest(id))
    },

    async listEntities() {
      return parseEntities((await call((app) => app.ListEntities())) ?? [])
    },

    async saveEntity(entity) {
      return parseEntity(await call((app) => app.SaveEntity(entity)))
    },

    async deleteEntity(id) {
      await call((app) => app.DeleteEntity(id))
    },

    async listTables() {
      return parseTables((await call((app) => app.ListTables())) ?? [])
    },

    async saveTable(table) {
      return parseTable(await call((app) => app.SaveTable(table)))
    },

    async deleteTable(id) {
      await call((app) => app.DeleteTable(id))
    },

    async exportTable(id) {
      return parseTableExport(await call((app) => app.ExportTable(id)))
    },

    async listAssets() {
      return parseAssets((await call((app) => app.ListAssets())) ?? [])
    },

    async saveAsset(asset) {
      return parseAsset(await call((app) => app.SaveAsset(asset)))
    },

    async deleteAsset(id) {
      await call((app) => app.DeleteAsset(id))
    },

    async listMaps() {
      return parseMaps((await call((app) => app.ListMaps())) ?? [])
    },

    async loadMap(id) {
      return parseMap(await call((app) => app.LoadMap(id)))
    },

    async saveMap(map) {
      return parseMap(await call((app) => app.SaveMap(map)))
    },

    async expandMap(id, options) {
      return parseMap(await call((app) => app.ExpandMap(id, options)))
    },

    async deleteMap(id) {
      await call((app) => app.DeleteMap(id))
    },

    async exportMap(id, format = 'tiled') {
      return parseAssetExport(await call((app) => app.ExportMap(id, format)))
    },

    async listPalettes() {
      return parsePalettes((await call((app) => app.ListPalettes())) ?? [])
    },

    async savePalette(palette) {
      const [saved] = parsePalettes([await call((app) => app.SavePalette(palette))])
      return saved!
    },

    async deletePalette(id) {
      await call((app) => app.DeletePalette(id))
    },

    async importAssets(paths, type) {
      return parseImportSummary(await call((app) => app.ImportAssets({ paths, type })))
    },

    async createBlankAsset(input) {
      return parseAsset(await call((app) => app.CreateBlankAsset(input)))
    },

    async pickDirectory(title = '选择文件夹') {
      return (await call((app) => app.PickDirectory(title))) ?? ''
    },

    async pickProjectFile() {
      return (await call((app) => app.PickProjectFile())) ?? ''
    },

    async pickImageFiles() {
      return (await call((app) => app.PickImageFiles())) ?? []
    },

    async openProjectFolder() {
      await call((app) => app.OpenProjectFolder())
    },

    async openLogsFolder() {
      await call((app) => app.OpenLogsFolder())
    },

    async assetURL(assetId, file) {
      return call((app) => app.GetAssetURL(assetId, file))
    },

    async assetSource(assetId) {
      return parseAssetSourceInfo(await call((app) => app.GetAssetSource(assetId)))
    },

    async saveAssetVersion(assetId, pngDataURL) {
      return parseAsset(await call((app) => app.SaveAssetVersion(assetId, pngDataURL)))
    },

    async saveAssetSheet(assetId, pngDataURL, sheet) {
      return parseAsset(await call((app) => app.SaveAssetSheet(assetId, pngDataURL, sheet)))
    },

    async exportAsset(assetId) {
      return parseAssetExport(await call((app) => app.ExportAsset(assetId)))
    },

    async openExportsFolder() {
      await call((app) => app.OpenExportsFolder())
    },

    async generateBuildings(options) {
      return parseGeneratedBuildings(await call((app) => app.GenerateBuildings(options)))
    },

    async adoptBuilding(options, seed, name) {
      return parseAsset(await call((app) => app.AdoptBuilding(options, seed, name)))
    },

    async previewPostProcess(assetId, options): Promise<PostProcessPreview> {
      return parsePostProcessPreview(
        await call((app) => app.PreviewPostProcess(assetId, options)),
      )
    },

    async savePostProcessedVersion(assetId, options) {
      return parseAsset(await call((app) => app.SavePostProcessedVersion(assetId, options)))
    },

    async getAISettings(): Promise<AISettings> {
      return parseAISettings(await call((app) => app.GetAISettings()))
    },

    async saveAISettings(input) {
      return parseAISettings(await call((app) => app.SaveAISettings(input)))
    },

    async testAIProvider(id): Promise<AITestResult> {
      return parseAITestResult(await call((app) => app.TestAIProvider(id)))
    },

    async generateText(request): Promise<AITextResult> {
      return parseAITextResult(await call((app) => app.GenerateText(request)))
    },

    async submitAIJob(request): Promise<AIJob> {
      return parseAIJob(await call((app) => app.SubmitAIJob(request)))
    },

    async cancelAIJob(id) {
      return parseAIJob(await call((app) => app.CancelAIJob(id)))
    },

    async deleteAIJob(id) {
      await call((app) => app.DeleteAIJob(id))
    },

    async listAIJobs() {
      return parseAIJobs((await call((app) => app.ListAIJobs())) ?? [])
    },

    async adoptAIJobImage(jobId, index, name) {
      return parseAsset(await call((app) => app.AdoptAIJobImage(jobId, index, name)))
    },

    async adoptAIJobSheet(request) {
      return parseAsset(await call((app) => app.AdoptAIJobSheet(request)))
    },

    onAIJobs(listener) {
      const runtime = window.runtime
      if (!runtime?.EventsOn) return () => {}
      const off = runtime.EventsOn('ai:jobs', (payload) => {
        try {
          listener(parseAIJobs(payload))
        } catch {
          /* ignore malformed events; the next one wins */
        }
      })
      return typeof off === 'function' ? off : () => {}
    },

    async getEditorSettings() {
      return parseEditorSettings(await call((app) => app.GetEditorSettings()))
    },

    async saveEditorSettings(settings) {
      return parseEditorSettings(await call((app) => app.SaveEditorSettings(settings)))
    },

    async getUpdateSettings() {
      return parseUpdateSettings(await call((app) => app.GetUpdateSettings()))
    },

    async saveUpdateSettings(settings) {
      return parseUpdateSettings(await call((app) => app.SaveUpdateSettings(settings)))
    },

    async checkForUpdates() {
      return parseUpdateStatus(await call((app) => app.CheckForUpdates()))
    },

    async downloadUpdate(status) {
      return call((app) => app.DownloadUpdate(status))
    },
  }
}