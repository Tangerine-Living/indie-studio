import { defineStore } from 'pinia'
import { ref } from 'vue'
import { platform } from '../platform'
import type {
  AIJob,
  AIJobRequest,
  AISettings,
  AISettingsInput,
  AISheetAdoptRequest,
  AITestResult,
  AITextRequest,
  AITextResult,
  AIProviderId,
  AppInfo,
  Asset,
  AssetExportResult,
  AssetStatus,
  AssetType,
  BuildingOptions,
  CreateBlankAssetInput,
  CreateOptions,
  DataTable,
  EditorSettings,
  Entity,
  GameMap,
  GeneratedBuilding,
  GenerateIntent,
  ImportSummary,
  MapExpandOptions,
  MapExportFormat,
  MapGenerateIntent,
  MapSummary,
  Palette,
  PostProcessOptions,
  PostProcessPreview,
  Project,
  ProjectSettings,
  Quest,
  QuestStatus,
  RecentProject,
  Route,
  Session,
  TableExportResult,
  UpdateSettings,
  UpdateStatus,
} from '../domain/types'
import type { SheetInfo } from '../editor/sheet'
import { AppError, normalizeError } from '../lib/errors'
import { nowISO } from '../lib/utils'

export type BootStatus = 'loading' | 'ready' | 'error'
export type SaveState = 'idle' | 'saving' | 'saved' | 'error'

const DEFAULT_INFO: AppInfo = {
  name: 'Indie Studio',
  version: '0.1.0-dev',
  platform: 'unknown',
  logPath: '',
  mode: 'browser',
}

function byRecentlyUpdated<T extends { updatedAt: string }>(items: T[]): T[] {
  return [...items].sort(
    (a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
  )
}

export const useAppStore = defineStore('app', () => {
  const status = ref<BootStatus>('loading')
  const bootError = ref<string | null>(null)
  const info = ref<AppInfo>(DEFAULT_INFO)
  const recent = ref<RecentProject[]>([])
  const session = ref<Session | null>(null)
  const quests = ref<Quest[]>([])
  const entities = ref<Entity[]>([])
  const tables = ref<DataTable[]>([])
  const assets = ref<Asset[]>([])
  const maps = ref<MapSummary[]>([])
  const palettes = ref<Palette[]>([])
  const route = ref<Route>('dashboard')
  const editorAssetId = ref<string | null>(null)
  /** One-shot target for the command palette to open after switching routes. */
  const pendingFocus = ref<{
    kind: 'quest' | 'entity' | 'table' | 'asset' | 'map'
    id: string
  } | null>(null)
  /** Bumped on every focus request so a mounted screen can react too. */
  const focusRequest = ref(0)
  const busy = ref(false)
  const saveState = ref<SaveState>('idle')
  const error = ref<AppError | null>(null)
  const lockedPath = ref<string | null>(null)
  const aiSettings = ref<AISettings | null>(null)
  const aiJobs = ref<AIJob[]>([])
  /** Set when a quest hands its brief to the generation workbench. */
  const generateIntent = ref<GenerateIntent | null>(null)
  /** Set when the map editor hands its context to the generation workbench. */
  const mapGenerateIntent = ref<MapGenerateIntent | null>(null)
  const editorSettings = ref<EditorSettings | null>(null)
  const updateSettings = ref<UpdateSettings | null>(null)
  const updateStatus = ref<UpdateStatus | null>(null)
  const updateBusy = ref(false)
  const updateDownloadPath = ref<string | null>(null)

  let saveTimer: ReturnType<typeof setTimeout> | null = null
  let editSeq = 0
  let bootInFlight = false
  let aiUnsub: (() => void) | null = null

  function cancelScheduledSave() {
    if (saveTimer) {
      clearTimeout(saveTimer)
      saveTimer = null
    }
  }

  async function saveProjectNow(): Promise<void> {
    const snapshot = session.value
    if (!snapshot) return
    const seq = editSeq
    try {
      const saved = await platform.saveProject(snapshot.project)
      const current = session.value
      if (!current || current.path !== snapshot.path) return
      if (editSeq === seq) {
        session.value = saved
        saveState.value = 'saved'
      }
    } catch (raw) {
      error.value = normalizeError(raw)
      saveState.value = 'error'
    }
  }

  async function loadLists() {
    const [
      nextQuests,
      nextEntities,
      nextTables,
      nextAssets,
      nextMaps,
      nextPalettes,
      nextRecent,
    ] = await Promise.all([
      platform.listQuests(),
      platform.listEntities(),
      platform.listTables(),
      platform.listAssets(),
      platform.listMaps(),
      platform.listPalettes(),
      platform.listRecent(),
    ])
    return { nextQuests, nextEntities, nextTables, nextAssets, nextMaps, nextPalettes, nextRecent }
  }

  async function boot(): Promise<void> {
    if (bootInFlight) return
    bootInFlight = true
    status.value = 'loading'
    bootError.value = null
    try {
      const payload = await platform.bootstrap()
      info.value = payload.info
      recent.value = payload.recent
      session.value = payload.session
      status.value = 'ready'
      route.value = 'dashboard'
      editorAssetId.value = null
      generateIntent.value = null
      mapGenerateIntent.value = null
      bindAIEvents()
      void loadAISettings()
      void checkUpdatesAtBoot()
      if (payload.session) {
        const lists = await loadLists()
        quests.value = lists.nextQuests
        entities.value = lists.nextEntities
        tables.value = lists.nextTables
        assets.value = lists.nextAssets
        maps.value = lists.nextMaps
        palettes.value = lists.nextPalettes
        recent.value = lists.nextRecent
        await refreshAIJobs()
      }
    } catch (raw) {
      const parsed = normalizeError(raw)
      status.value = 'error'
      bootError.value = parsed.message
    } finally {
      bootInFlight = false
    }
  }

  /** A screen can veto leaving while it holds unsaved edits. */
  const leaveGuard = ref<(() => Promise<boolean>) | null>(null)

  function setLeaveGuard(guard: (() => Promise<boolean>) | null) {
    leaveGuard.value = guard
  }

  async function runLeaveGuard(): Promise<boolean> {
    const guard = leaveGuard.value
    if (!guard) return true
    return guard()
  }

  async function setRoute(next: Route) {
    if (next === route.value) return
    if (!(await runLeaveGuard())) return
    route.value = next
  }

  function requestFocus(kind: 'quest' | 'entity' | 'table' | 'asset' | 'map', id: string) {
    pendingFocus.value = { kind, id }
    focusRequest.value += 1
  }

  /** Returns the pending id once, so the target screen can open it. */
  function consumeFocus(kind: 'quest' | 'entity' | 'table' | 'asset' | 'map'): string | null {
    const pending = pendingFocus.value
    if (!pending || pending.kind !== kind) return null
    pendingFocus.value = null
    return pending.id
  }

  async function loadProjectData(): Promise<void> {
    if (!session.value) return
    try {
      const lists = await loadLists()
      if (!session.value) return
      quests.value = lists.nextQuests
      entities.value = lists.nextEntities
      tables.value = lists.nextTables
      assets.value = lists.nextAssets
      maps.value = lists.nextMaps
      palettes.value = lists.nextPalettes
      recent.value = lists.nextRecent
      await refreshAIJobs()
    } catch (raw) {
      error.value = normalizeError(raw)
    }
  }

  async function createProject(options: CreateOptions): Promise<boolean> {
    cancelScheduledSave()
    editSeq += 1
    busy.value = true
    try {
      const next = await platform.createProject(options)
      const lists = await loadLists()
      session.value = next
      quests.value = lists.nextQuests
      entities.value = lists.nextEntities
      tables.value = lists.nextTables
      assets.value = lists.nextAssets
      maps.value = lists.nextMaps
      palettes.value = lists.nextPalettes
      recent.value = lists.nextRecent
      route.value = 'dashboard'
      editorAssetId.value = null
      generateIntent.value = null
      mapGenerateIntent.value = null
      bindAIEvents()
      await refreshAIJobs()
      error.value = null
      lockedPath.value = null
      saveState.value = 'idle'
      return true
    } catch (raw) {
      error.value = normalizeError(raw)
      return false
    } finally {
      busy.value = false
    }
  }

  async function openProject(path: string, force = false): Promise<boolean> {
    const trimmed = path.trim()
    if (!trimmed) return false
    cancelScheduledSave()
    editSeq += 1
    busy.value = true
    try {
      const next = await platform.openProject(trimmed, force)
      const lists = await loadLists()
      session.value = next
      quests.value = lists.nextQuests
      entities.value = lists.nextEntities
      tables.value = lists.nextTables
      assets.value = lists.nextAssets
      maps.value = lists.nextMaps
      palettes.value = lists.nextPalettes
      recent.value = lists.nextRecent
      route.value = 'dashboard'
      editorAssetId.value = null
      generateIntent.value = null
      mapGenerateIntent.value = null
      bindAIEvents()
      await refreshAIJobs()
      error.value = null
      lockedPath.value = null
      saveState.value = 'idle'
      return true
    } catch (raw) {
      const parsed = normalizeError(raw)
      error.value = parsed
      lockedPath.value = parsed.code === 'PROJECT_LOCKED' ? trimmed : null
      return false
    } finally {
      busy.value = false
    }
  }

  async function forceOpen(): Promise<void> {
    const path = lockedPath.value
    if (!path) return
    await openProject(path, true)
  }

  async function closeProject(): Promise<void> {
    if (!(await runLeaveGuard())) return
    if (saveState.value === 'saving') {
      cancelScheduledSave()
      await saveProjectNow()
    }
    cancelScheduledSave()
    try {
      await platform.closeProject()
      recent.value = await platform.listRecent()
      session.value = null
      quests.value = []
      entities.value = []
      tables.value = []
      assets.value = []
      maps.value = []
      palettes.value = []
      aiJobs.value = []
      route.value = 'dashboard'
      editorAssetId.value = null
      generateIntent.value = null
      mapGenerateIntent.value = null
      saveState.value = 'idle'
      lockedPath.value = null
      error.value = null
    } catch (raw) {
      error.value = normalizeError(raw)
    }
  }

  function patchProject(updater: (project: Project) => Project) {
    const current = session.value
    if (!current) return
    const project = updater(current.project)
    editSeq += 1
    session.value = { ...current, project }
    saveState.value = 'saving'
    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = setTimeout(() => {
      saveTimer = null
      void saveProjectNow()
    }, 600)
  }

  function patchSettings(patch: Partial<ProjectSettings>) {
    patchProject((project) => ({
      ...project,
      settings: { ...project.settings, ...patch },
    }))
  }

  async function saveNow(): Promise<void> {
    if (saveTimer) {
      clearTimeout(saveTimer)
      saveTimer = null
    }
    await saveProjectNow()
  }

  async function saveQuest(quest: Quest): Promise<Quest | null> {
    busy.value = true
    try {
      const saved = await platform.saveQuest(quest)
      const lists = await loadLists()
      quests.value = lists.nextQuests
      entities.value = lists.nextEntities
      tables.value = lists.nextTables
      assets.value = lists.nextAssets
      maps.value = lists.nextMaps
      palettes.value = lists.nextPalettes
      recent.value = lists.nextRecent
      error.value = null
      return saved
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  /* ------------------------------------------------------------ maps */

  /** Loads the full map document for the editor. */
  async function openMap(id: string): Promise<GameMap | null> {
    try {
      const map = await platform.loadMap(id)
      error.value = null
      return map
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    }
  }

  /** Saves a map and refreshes the summary list shown in the map picker. */
  async function saveMap(map: GameMap): Promise<GameMap | null> {
    // A screen's unmount flush can land after the project was closed.
    if (!session.value) return null
    try {
      const saved = await platform.saveMap(map)
      maps.value = await platform.listMaps()
      error.value = null
      return saved
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    }
  }

  /** Grows the canvas on any side; existing tiles and objects shift with it. */
  async function expandMap(
    id: string,
    options: MapExpandOptions,
  ): Promise<GameMap | null> {
    busy.value = true
    try {
      const map = await platform.expandMap(id, options)
      maps.value = await platform.listMaps()
      error.value = null
      return map
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  async function deleteMap(id: string): Promise<boolean> {
    try {
      await platform.deleteMap(id)
      maps.value = await platform.listMaps()
      error.value = null
      return true
    } catch (raw) {
      error.value = normalizeError(raw)
      return false
    }
  }

  async function exportMap(id: string, format: MapExportFormat = 'tiled'): Promise<AssetExportResult | null> {
    busy.value = true
    try {
      const result = await platform.exportMap(id, format)
      error.value = null
      return result
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  async function saveEntity(entity: Entity): Promise<Entity | null> {
    if (!session.value) return null
    try {
      const saved = await platform.saveEntity(entity)
      entities.value = byRecentlyUpdated([
        ...entities.value.filter((item) => item.id !== saved.id),
        saved,
      ])
      error.value = null
      return saved
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    }
  }

  /** Deletes one setting entry; relations pointing at it are cleaned on save. */
  async function deleteEntity(id: string): Promise<boolean> {
    try {
      await platform.deleteEntity(id)
      entities.value = await platform.listEntities()
      error.value = null
      return true
    } catch (raw) {
      error.value = normalizeError(raw)
      return false
    }
  }

  async function saveTable(table: DataTable): Promise<DataTable | null> {
    if (!session.value) return null
    try {
      const saved = await platform.saveTable(table)
      tables.value = byRecentlyUpdated([
        ...tables.value.filter((item) => item.id !== saved.id),
        saved,
      ])
      error.value = null
      return saved
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    }
  }

  async function deleteTable(id: string): Promise<boolean> {
    try {
      await platform.deleteTable(id)
      tables.value = await platform.listTables()
      error.value = null
      return true
    } catch (raw) {
      error.value = normalizeError(raw)
      return false
    }
  }

  async function exportTable(id: string): Promise<TableExportResult | null> {
    busy.value = true
    try {
      const result = await platform.exportTable(id)
      error.value = null
      return result
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  async function deleteQuest(id: string): Promise<boolean> {
    busy.value = true
    try {
      await platform.deleteQuest(id)
      const lists = await loadLists()
      quests.value = lists.nextQuests
      entities.value = lists.nextEntities
      tables.value = lists.nextTables
      assets.value = lists.nextAssets
      maps.value = lists.nextMaps
      palettes.value = lists.nextPalettes
      recent.value = lists.nextRecent
      error.value = null
      return true
    } catch (raw) {
      error.value = normalizeError(raw)
      return false
    } finally {
      busy.value = false
    }
  }

  /**
 * Saves a new review status for one asset. Approving an asset also completes
 * every linked quest that is waiting in review, so the task board follows the
 * asset review instead of needing a second manual pass.
 */
async function setAssetStatus(
  id: string,
  status: AssetStatus,
): Promise<{ asset: Asset; promoted: Quest[] } | null> {
  const current = assets.value.find((item) => item.id === id)
  if (!current) return null
  busy.value = true
  try {
    const saved = await platform.saveAsset({ ...current, status })
    assets.value = byRecentlyUpdated(
      assets.value.map((asset) => (asset.id === saved.id ? saved : asset)),
    )
    const promoted: Quest[] = []
    if (status === 'approved') {
      const linked = quests.value.filter(
        (quest) =>
          quest.status === 'review' &&
          (saved.questIds.includes(quest.id) || quest.links.assetIds.includes(saved.id)),
      )
      for (const quest of linked) {
        try {
          promoted.push(
            await platform.saveQuest({ ...quest, status: 'done', updatedAt: nowISO() }),
          )
        } catch {
          /* one unreadable quest must not roll back the asset approval */
        }
      }
      if (promoted.length > 0) {
        quests.value = byRecentlyUpdated(
          quests.value.map((quest) => promoted.find((item) => item.id === quest.id) ?? quest),
        )
      }
    }
    error.value = null
    return { asset: saved, promoted }
  } catch (raw) {
    error.value = normalizeError(raw)
    return null
  } finally {
    busy.value = false
  }
}

  /** Moves an asset folder to the project trash and refreshes every list. */
  async function deleteAsset(id: string): Promise<boolean> {
    busy.value = true
    try {
      await platform.deleteAsset(id)
      const lists = await loadLists()
      assets.value = lists.nextAssets
      quests.value = lists.nextQuests
      entities.value = lists.nextEntities
      tables.value = lists.nextTables
      maps.value = lists.nextMaps
      palettes.value = lists.nextPalettes
      recent.value = lists.nextRecent
      if (session.value?.project.coverAssetId === id) {
        // The cover image is gone; drop the pointer so the dashboard falls back.
        patchProject((current) => ({ ...current, coverAssetId: undefined }))
      }
      error.value = null
      return true
    } catch (raw) {
      error.value = normalizeError(raw)
      return false
    } finally {
      busy.value = false
    }
  }

  /**
   * Adds or removes one tag on many assets. Matching is case-insensitive so
   * "Town" and "town" do not both end up on the same asset.
   */
  async function applyAssetTags(
    ids: string[],
    tag: string,
    mode: 'add' | 'remove',
  ): Promise<{ changed: number; failed: number }> {
    const clean = tag.trim()
    const result = { changed: 0, failed: 0 }
    if (!clean || ids.length === 0) return result
    const wanted = clean.toLocaleLowerCase()
    busy.value = true
    try {
      const updates: Asset[] = []
      for (const id of ids) {
        const asset = assets.value.find((item) => item.id === id)
        if (!asset) {
          result.failed += 1
          continue
        }
        const has = asset.tags.some((item) => item.toLocaleLowerCase() === wanted)
        if (mode === 'add' ? has : !has) continue
        const tags =
          mode === 'add'
            ? [...asset.tags, clean]
            : asset.tags.filter((item) => item.toLocaleLowerCase() !== wanted)
        try {
          updates.push(await platform.saveAsset({ ...asset, tags }))
          result.changed += 1
        } catch {
          result.failed += 1
        }
      }
      if (updates.length > 0) {
        const saved = new Map(updates.map((asset) => [asset.id, asset]))
        assets.value = byRecentlyUpdated(assets.value.map((asset) => saved.get(asset.id) ?? asset))
      }
      return result
    } finally {
      busy.value = false
    }
  }

  /** Saves one palette and keeps the selectors sorted by name. */
  async function savePalette(palette: Palette): Promise<Palette | null> {
    try {
      const saved = await platform.savePalette(palette)
      palettes.value = [...palettes.value.filter((item) => item.id !== saved.id), saved].sort(
        (a, b) => a.name.localeCompare(b.name, 'zh-CN'),
      )
      error.value = null
      return saved
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    }
  }

/** Deletes one palette; the backend protects the default and referenced ones. */
  async function deletePalette(id: string): Promise<boolean> {
    try {
      await platform.deletePalette(id)
      palettes.value = palettes.value.filter((item) => item.id !== id)
      error.value = null
      return true
    } catch (raw) {
      error.value = normalizeError(raw)
      return false
    }
  }

  async function importAssets(paths: string[], type: AssetType): Promise<ImportSummary | null> {
    busy.value = true
    try {
      const summary = await platform.importAssets(paths, type)
      if (summary.imported.length > 0) {
        const lists = await loadLists()
        assets.value = lists.nextAssets
        recent.value = lists.nextRecent
      }
      error.value = null
      return summary
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  /** Creates an empty asset for hand drawing; the editor opens on a blank canvas. */
  async function createBlankAsset(input: CreateBlankAssetInput): Promise<Asset | null> {
    busy.value = true
    try {
      const asset = await platform.createBlankAsset(input)
      const lists = await loadLists()
      assets.value = lists.nextAssets
      recent.value = lists.nextRecent
      error.value = null
      return asset
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  /** Opens the pixel editor on one asset. */
  function openEditor(id: string) {
    editorAssetId.value = id
    route.value = 'editor'
  }

  function closeEditor() {
    editorAssetId.value = null
    route.value = 'assets'
  }

  /** Hands a quest's brief to the generation workbench and navigates there. */
  function openGenerateFromQuest(quest: Quest) {
    generateIntent.value = {
      questId: quest.id,
      questTitle: quest.title,
      assetType: quest.brief.assetType,
      tileSize: quest.brief.tileSize,
      paletteId: quest.brief.paletteId,
      style: quest.brief.style.trim(),
      prompt: quest.brief.prompt.trim(),
    }
    route.value = 'generate'
  }

  /** Style guide phrased for a generation prompt. */
  function styleGuideText(): string {
    const guide = session.value?.project.settings.styleGuide
    if (!guide) return ''
    const perspective =
      guide.perspective === 'top-down'
        ? '正俯视'
        : guide.perspective === 'side-view'
          ? '侧视'
          : '3/4 俯视（正交）'
    const outline =
      guide.outline === 'none'
        ? '无描边'
        : guide.outline === '1px-colored'
          ? '1 像素同色系描边'
          : '1 像素深色描边'
    const light = guide.light === 'top-right' ? '右上' : guide.light === 'top' ? '顶部' : '左上'
    const parts = [`${perspective}，${outline}，受光来自${light}，${guide.maxColors} 色以内`]
    if (guide.stylePrompt?.trim()) parts.push(guide.stylePrompt.trim())
    return parts.join('，')
  }

  /**
   * Hands a lore entry or data-table row to the generation workbench. The
   * caller composes the prompt from its own labels, so references read well.
   */
  function openGenerateFromRecord(input: {
    name: string
    prompt: string
    assetType: AssetType
    sourceKind: 'entity' | 'table'
    sourceLabel: string
    sourceId: string
    style?: string
    /** The prompt already folds the style in; don't append the guide again. */
    promptStyled?: boolean
    aiFallback?: boolean
    aiFallbackDetail?: string
  }) {
    generateIntent.value = {
      questId: '',
      questTitle: '',
      assetType: input.assetType,
      tileSize: session.value?.project.settings.tileSize ?? 32,
      paletteId: session.value?.project.settings.defaultPaletteId ?? 'default',
      style: input.promptStyled ? '' : (input.style ?? '').trim() || styleGuideText(),
      prompt: input.prompt,
      sourceKind: input.sourceKind,
      sourceLabel: input.sourceLabel,
      sourceId: input.sourceId,
      aiFallback: input.aiFallback,
      aiFallbackDetail: input.aiFallbackDetail,
    }
    route.value = 'generate'
  }

  /** Reads and clears the pending intent; called when the generate screen mounts. */
  function consumeGenerateIntent(): GenerateIntent | null {
    const intent = generateIntent.value
    generateIntent.value = null
    return intent
  }

  function clearGenerateIntent() {
    generateIntent.value = null
  }

  /**
   * Hands the map's context to the generation workbench so new tiles, props or
   * FX can be generated to fit the existing scene, then navigates there.
   */
  function openGenerateFromMap(map: GameMap, prompt: string, assetType: AssetType = 'tile') {
    openMapGenerate({
      mapId: map.id,
      mapName: map.name,
      width: map.width,
      height: map.height,
      tileSize: map.tileSize,
      tilesets: map.tilesets.map((set) => set.id),
      assetType,
      prompt,
    })
  }

  /** Same handoff for a map that has not been created yet (new-map dialog). */
  function openMapGenerate(intent: MapGenerateIntent) {
    mapGenerateIntent.value = { ...intent, prompt: intent.prompt.trim() }
    route.value = 'generate'
  }

  /** Reads and clears the pending map intent; called when the generate screen mounts. */
  function consumeMapGenerateIntent(): MapGenerateIntent | null {
    const intent = mapGenerateIntent.value
    mapGenerateIntent.value = null
    return intent
  }

  /** Persists the editor composite as the asset's next immutable version. */
  async function saveAssetVersion(assetId: string, pngDataURL: string): Promise<Asset | null> {
    busy.value = true
    try {
      const saved = await platform.saveAssetVersion(assetId, pngDataURL)
      assets.value = byRecentlyUpdated(
        assets.value.map((asset) => (asset.id === saved.id ? saved : asset)),
      )
      error.value = null
      return saved
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  /** Persists the whole animation as a sprite-sheet version. */
  async function saveAssetSheet(
    assetId: string,
    pngDataURL: string,
    sheet: SheetInfo,
  ): Promise<Asset | null> {
    busy.value = true
    try {
      const saved = await platform.saveAssetSheet(assetId, pngDataURL, sheet)
      assets.value = byRecentlyUpdated(
        assets.value.map((asset) => (asset.id === saved.id ? saved : asset)),
      )
      error.value = null
      return saved
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  /** Draws procedural drafts without touching the project. */
  async function generateBuildings(
    options: BuildingOptions,
  ): Promise<GeneratedBuilding[] | null> {
    busy.value = true
    try {
      const drafts = await platform.generateBuildings(options)
      error.value = null
      return drafts
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  /** Adopts one draft variant as a new draft building asset. */
  async function adoptBuilding(
    options: BuildingOptions,
    seed: number,
    name: string,
  ): Promise<Asset | null> {
    busy.value = true
    try {
      const saved = await platform.adoptBuilding(options, seed, name)
      assets.value = byRecentlyUpdated([
        ...assets.value.filter((asset) => asset.id !== saved.id),
        saved,
      ])
      error.value = null
      return saved
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  /**
   * Runs the pixel-perfect chain for an A/B preview. It deliberately does not
   * flip the global busy flag: previewing is cheap and user-driven.
   */
  async function previewPostProcess(
    assetId: string,
    options: PostProcessOptions,
  ): Promise<PostProcessPreview | null> {
    try {
      const preview = await platform.previewPostProcess(assetId, options)
      error.value = null
      return preview
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    }
  }

  /** Saves the processed result as the asset's next immutable version. */
  async function savePostProcessedVersion(
    assetId: string,
    options: PostProcessOptions,
  ): Promise<Asset | null> {
    busy.value = true
    try {
      const saved = await platform.savePostProcessedVersion(assetId, options)
      assets.value = byRecentlyUpdated(
        assets.value.map((asset) => (asset.id === saved.id ? saved : asset)),
      )
      error.value = null
      return saved
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  /* ------------------------------------------------------------ AI drafts */

  /** Binds the queue push channel once; later calls restart the subscription. */
  function bindAIEvents() {
    aiUnsub?.()
    aiUnsub = platform.onAIJobs((jobs) => {
      aiJobs.value = jobs
    })
  }

  async function loadAISettings(): Promise<AISettings | null> {
    try {
      const settings = await platform.getAISettings()
      aiSettings.value = settings
      error.value = null
      return settings
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    }
  }

  async function saveAISettings(input: AISettingsInput): Promise<AISettings | null> {
    busy.value = true
    try {
      const settings = await platform.saveAISettings(input)
      aiSettings.value = settings
      error.value = null
      return settings
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  async function testAIProvider(id: AIProviderId): Promise<AITestResult | null> {
    try {
      const result = await platform.testAIProvider(id)
      error.value = null
      return result
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    }
  }

  async function refreshAIJobs(): Promise<void> {
    try {
      aiJobs.value = await platform.listAIJobs()
    } catch (raw) {
      error.value = normalizeError(raw)
    }
  }

  async function submitAIJob(request: AIJobRequest): Promise<AIJob | null> {
    busy.value = true
    try {
      const job = await platform.submitAIJob(request)
      aiJobs.value = [job, ...aiJobs.value.filter((item) => item.id !== job.id)]
      error.value = null
      return job
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  /**
   * Chat completion used by lore/table copywriting and AI prompt drafting.
   * Returns the error as well so callers can show the real reason inline.
   * `silent` skips the global toast: those callers already explain inline.
   */
  async function generateText(
    request: AITextRequest,
    options: { silent?: boolean } = {},
  ): Promise<{ result: AITextResult | null; error: AppError | null }> {
    try {
      const result = await platform.generateText(request)
      error.value = null
      return { result, error: null }
    } catch (raw) {
      const failure = normalizeError(raw)
      if (!options.silent) error.value = failure
      return { result: null, error: failure }
    }
  }

  async function cancelAIJob(id: string): Promise<AIJob | null> {
    try {
      const job = await platform.cancelAIJob(id)
      aiJobs.value = aiJobs.value.map((item) => (item.id === job.id ? job : item))
      error.value = null
      return job
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    }
  }

  /**
 * Removes one finished, failed or canceled job and its generated images.
 * Already adopted assets keep their own copies, so they are unaffected.
 */
async function deleteAIJob(id: string): Promise<boolean> {
  try {
    await platform.deleteAIJob(id)
    aiJobs.value = aiJobs.value.filter((job) => job.id !== id)
    error.value = null
    return true
  } catch (raw) {
    error.value = normalizeError(raw)
    await refreshAIJobs()
    return false
  }
}

/** Adopts one finished result as a draft asset in the library. */
  async function adoptAIJobImage(
    jobId: string,
    index: number,
    name: string,
  ): Promise<Asset | null> {
    busy.value = true
    try {
      const saved = await platform.adoptAIJobImage(jobId, index, name)
      assets.value = byRecentlyUpdated([
        ...assets.value.filter((asset) => asset.id !== saved.id),
        saved,
      ])
      error.value = null
      return saved
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  /** Slices one finished AI image into a cleaned directional / FX sheet asset. */
  async function adoptAIJobSheet(request: AISheetAdoptRequest): Promise<Asset | null> {
    busy.value = true
    try {
      const saved = await platform.adoptAIJobSheet(request)
      assets.value = byRecentlyUpdated([
        ...assets.value.filter((asset) => asset.id !== saved.id),
        saved,
      ])
      error.value = null
      return saved
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      busy.value = false
    }
  }

  const DEFAULT_UPDATE_SETTINGS: UpdateSettings = {
    channel: 'stable',
    autoCheck: true,
  }

  async function loadEditorSettings(): Promise<EditorSettings | null> {
    try {
      editorSettings.value = await platform.getEditorSettings()
    } catch {
      // A missing config is normal; keep defaults.
      if (!editorSettings.value) editorSettings.value = { showGrid: true, openMode: 'fit' }
    }
    return editorSettings.value
  }

  async function saveEditorSettings(next: EditorSettings): Promise<EditorSettings | null> {
    try {
      editorSettings.value = await platform.saveEditorSettings(next)
      error.value = null
      return editorSettings.value
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    }
  }

  async function loadUpdateSettings(): Promise<UpdateSettings | null> {
    try {
      updateSettings.value = await platform.getUpdateSettings()
    } catch {
      // A missing or unreadable config is normal; defaults stay in place.
      if (!updateSettings.value) updateSettings.value = { ...DEFAULT_UPDATE_SETTINGS }
    }
    return updateSettings.value
  }

  async function saveUpdateSettings(patch: Partial<UpdateSettings>): Promise<void> {
    const next = { ...(updateSettings.value ?? DEFAULT_UPDATE_SETTINGS), ...patch }
    updateSettings.value = next
    try {
      updateSettings.value = await platform.saveUpdateSettings(next)
    } catch (raw) {
      error.value = normalizeError(raw)
    }
  }

  /** Checks the signed manifest. A silent boot check never raises the error bar. */
  async function checkUpdates(silent = false): Promise<UpdateStatus | null> {
    if (updateBusy.value) return updateStatus.value
    updateBusy.value = true
    updateDownloadPath.value = null
    try {
      updateStatus.value = await platform.checkForUpdates()
      return updateStatus.value
    } catch (raw) {
      if (!silent) {
        updateStatus.value = null
        error.value = normalizeError(raw)
      }
      return null
    } finally {
      updateBusy.value = false
    }
  }

  async function checkUpdatesAtBoot(): Promise<void> {
    const settings = await loadUpdateSettings()
    if (!settings?.autoCheck) return
    await checkUpdates(true)
  }

  async function downloadUpdate(): Promise<string | null> {
    if (!updateStatus.value) return null
    updateBusy.value = true
    try {
      updateDownloadPath.value = await platform.downloadUpdate(updateStatus.value)
      return updateDownloadPath.value
    } catch (raw) {
      error.value = normalizeError(raw)
      return null
    } finally {
      updateBusy.value = false
    }
  }

  async function moveQuest(id: string, target: QuestStatus): Promise<void> {
    const quest = quests.value.find((item) => item.id === id)
    if (!quest || quest.status === target) return
    const next: Quest = { ...quest, status: target, updatedAt: nowISO() }
    quests.value = byRecentlyUpdated(
      quests.value.map((item) => (item.id === id ? next : item)),
    )
    await saveQuest(next)
  }

  function setError(raw: unknown) {
    error.value = normalizeError(raw)
  }

  function clearError() {
    error.value = null
  }

  return {
    status,
    bootError,
    info,
    recent,
    session,
    quests,
    entities,
    tables,
    assets,
    maps,
    palettes,
    route,
    editorAssetId,
    busy,
    saveState,
    error,
    lockedPath,
    aiSettings,
    aiJobs,
    generateIntent,
    mapGenerateIntent,
    editorSettings,
    updateSettings,
    updateStatus,
    updateBusy,
    updateDownloadPath,
    boot,
    setRoute,
    setLeaveGuard,
    focusRequest,
    requestFocus,
    consumeFocus,
    createBlankAsset,
    generateText,
    styleGuideText,
    openEditor,
    closeEditor,
    setAssetStatus,
    applyAssetTags,
    deleteAsset,
    savePalette,
    deletePalette,
    openGenerateFromQuest,
    openGenerateFromRecord,
    consumeGenerateIntent,
    clearGenerateIntent,
    openGenerateFromMap,
    openMapGenerate,
    consumeMapGenerateIntent,
    saveAssetVersion,
    saveAssetSheet,
    previewPostProcess,
    savePostProcessedVersion,
    generateBuildings,
    adoptBuilding,
    loadAISettings,
    saveAISettings,
    testAIProvider,
    loadEditorSettings,
    saveEditorSettings,
    loadUpdateSettings,
    saveUpdateSettings,
    checkUpdates,
    checkUpdatesAtBoot,
    downloadUpdate,
    refreshAIJobs,
    submitAIJob,
    cancelAIJob,
    deleteAIJob,
    adoptAIJobImage,
    adoptAIJobSheet,
    loadProjectData,
    createProject,
    openProject,
    forceOpen,
    closeProject,
    patchProject,
    patchSettings,
    saveNow,
    saveQuest,
    deleteQuest,
    saveEntity,
    deleteEntity,
    saveTable,
    deleteTable,
    exportTable,
    openMap,
    saveMap,
    expandMap,
    deleteMap,
    exportMap,
    moveQuest,
    importAssets,
    setError,
    clearError,
  }
})