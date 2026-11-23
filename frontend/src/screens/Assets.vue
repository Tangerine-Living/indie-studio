<script setup lang="ts">
import {
  BookOpen,
  Check,
  Download,
  FolderOpen,
  ImageOff,
  Images,
  LayoutGrid,
  List,
  Loader2,
  MapPin,
  Maximize2,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Sparkles,
  Trash2,
  Upload,
  Wand2,
} from 'lucide-vue-next'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ASSET_STATUSES, ASSET_TYPES, DITHER_MODES } from '../domain/types'
import type {
  Animation,
  Asset,
  AssetStatus,
  AssetType,
  DitherMode,
  GameMap,
  PostProcessOptions,
  PostProcessPreview,
} from '../domain/types'
import { platform } from '../platform'
import { useAppStore } from '../stores/app'
import { ASSET_STATUS_CLASS } from '../lib/badges'
import { confirmDialog } from '../lib/dialogs'
import { openLightbox } from '../lib/lightbox'
import { formatBytes, formatDate, relativeTime, shortPath } from '../lib/utils'
import AppModal from '../components/AppModal.vue'
import InpaintDialog from '../components/InpaintDialog.vue'
import AssetThumb from '../components/AssetThumb.vue'
import EmptyState from '../components/EmptyState.vue'

const store = useAppStore()
const { t } = useI18n()

function animationLabel(animation: Animation): string {
  const parts: string[] = []
  if (animation.frameCount) parts.push(`${animation.frameCount} 帧`)
  parts.push(`${animation.fps} FPS`)
  if (animation.tags.length) parts.push(animation.tags.join(' / '))
  return parts.join(' · ')
}

async function openAssetPreview(asset: Asset) {
  const label = `${asset.name} · ${asset.dimensions.w}×${asset.dimensions.h}`
  const animation = asset.animation
  if (animation?.frameCount && animation.frameCount > 1) {
    try {
      const frames = await Promise.all(
        Array.from({ length: animation.frameCount }, (_, index) =>
          platform.assetURL(asset.id, `frames/${String(index).padStart(3, '0')}.png`),
        ),
      )
      openLightbox(frames[0], label, {
        frames,
        fps: animation.fps,
        frameDurations: animation.frameDurations,
        loop: animation.loop,
      })
      return
    } catch {
      /* frames missing on older assets; fall back to the stored version */
    }
  }
  const file = asset.currentVersion ? `versions/${asset.currentVersion}/output.png` : 'thumb.png'
  try {
    openLightbox(await platform.assetURL(asset.id, file), label)
  } catch {
    try {
      openLightbox(await platform.assetURL(asset.id, 'thumb.png'), asset.name)
    } catch {
      /* neither image is available; keep the dialog as is */
    }
  }
}

const search = ref('')
const typeFilter = ref<AssetType | 'all'>('all')
const statusFilter = ref<AssetStatus | 'all'>('all')
const selectedId = ref<string | null>(null)
const importType = ref<AssetType>('sprite')
const notice = ref<string | null>(null)

/** Grid shows thumbnails; list shows sortable-looking metadata rows. Persisted per machine. */
type ViewMode = 'grid' | 'list'
const VIEW_KEY = 'pb.assets.view'
const viewMode = ref<ViewMode>(
  typeof localStorage !== 'undefined' && localStorage.getItem(VIEW_KEY) === 'list' ? 'list' : 'grid',
)
watch(viewMode, (mode) => {
  try {
    localStorage.setItem(VIEW_KEY, mode)
  } catch {
    /* private mode / quota: the toggle still works for this session */
  }
})

/** Multi-select lives apart from the detail modal: a card can be checked without opening it. */
const checkedIds = ref<string[]>([])
const batchBusy = ref(false)
const batchStatusChoice = ref<'' | AssetStatus>('')
const batchTag = ref('')
let lastCheckedId: string | null = null

onMounted(() => {
  const id = store.consumeFocus('asset')
  if (id && store.assets.some((asset) => asset.id === id)) selectedId.value = id
  window.addEventListener('keydown', onPostKeyDown)
  window.addEventListener('keyup', onPostKeyUp)
  window.addEventListener('blur', onPostBlur)
})

// The command palette can name an asset while this screen is already open.
watch(
  () => store.focusRequest,
  () => {
    const id = store.consumeFocus('asset')
    if (id && store.assets.some((asset) => asset.id === id)) selectedId.value = id
  },
)
let noticeTimer: ReturnType<typeof setTimeout> | null = null

const DITHER_LABELS: Record<string, string> = {
  none: '无抖动',
  floyd: 'Floyd–Steinberg',
  bayer: 'Bayer 4×4',
  noise: '确定性噪声',
}

const POST_COLORS = ['#111821', '#f2f6fa', '#e8c07a', '#b85c38', '#4f8f4a', '#6ec6e8']
/** Common pixel-art long sides; picking one keeps the downscale close to an integer ratio. */
const SIZE_PRESETS = [16, 24, 32, 48, 64]

/** Darkest palette colour by perceived luminance, the usual outline choice. */
function darkestColor(colors: string[]): string | null {
  let best: string | null = null
  let bestLum = Infinity
  for (const hex of colors) {
    const value = hex.replace('#', '')
    if (value.length !== 6) continue
    const r = parseInt(value.slice(0, 2), 16) / 255
    const g = parseInt(value.slice(2, 4), 16) / 255
    const b = parseInt(value.slice(4, 6), 16) / 255
    const lum = 0.2126 * r + 0.7152 * g + 0.0722 * b
    if (lum < bestLum) {
      bestLum = lum
      best = hex.startsWith('#') ? hex : `#${hex}`
    }
  }
  return best
}

const newAssetOpen = ref(false)
const inpaintOpen = ref(false)
const inpaintTarget = ref<Asset | null>(null)

function openInpaint() {
  const asset = selected.value
  if (!asset || !asset.currentVersion) return
  inpaintTarget.value = asset
  inpaintOpen.value = true
}
const newAssetError = ref('')
const newAsset = reactive({
  name: '',
  type: 'sprite' as AssetType,
  width: 32,
  height: 32,
})

function openNewAsset() {
  newAsset.name = ''
  newAsset.type = importType.value
  newAsset.width = 32
  newAsset.height = 32
  newAssetError.value = ''
  newAssetOpen.value = true
}

function applyNewAssetSize(size: number) {
  newAsset.width = size
  newAsset.height = size
}

async function createNewAsset() {
  newAssetError.value = ''
  if (!newAsset.name.trim()) {
    newAssetError.value = '请输入素材名称'
    return
  }
  const width = Math.round(Number(newAsset.width) || 0)
  const height = Math.round(Number(newAsset.height) || 0)
  if (width < 1 || height < 1 || width > 2048 || height > 2048) {
    newAssetError.value = '画布尺寸需在 1–2048 像素之间'
    return
  }
  const asset = await store.createBlankAsset({
    name: newAsset.name.trim(),
    type: newAsset.type,
    width,
    height,
  })
  if (!asset) return
  newAssetOpen.value = false
  selectedId.value = null
  postOpen.value = false
  // Straight into the editor on the empty canvas.
  store.openEditor(asset.id)
}

const postOpen = ref(false)
/** Integer preview zoom; fractional scaling would blur the result we judge. */
const POST_ZOOMS = [1, 2, 4, 8]
/** Small fractional steps are only for zooming out below fit. */
const POST_ZOOM_LEVELS = [0.5, 0.75, 1, 2, 3, 4, 6, 8]
const postZoom = ref(1)
const postPan = reactive({ x: 0, y: 0 })
const postSpace = ref(false)
const postPanning = ref<{ x: number; y: number } | null>(null)

function stepPostZoom(direction: number) {
  const current = postZoom.value
  let next = current
  if (direction > 0) {
    next = POST_ZOOM_LEVELS.find((level) => level > current + 1e-6) ?? POST_ZOOM_LEVELS[POST_ZOOM_LEVELS.length - 1]
  } else {
    next = [...POST_ZOOM_LEVELS].reverse().find((level) => level < current - 1e-6) ?? POST_ZOOM_LEVELS[0]
  }
  postZoom.value = next
}

function resetPostView() {
  postZoom.value = postPreview.value && Math.max(postPreview.value.width, postPreview.value.height) <= 64 ? 4 : 1
  postPan.x = 0
  postPan.y = 0
  postPanning.value = null
}

function onPostWheel(event: WheelEvent) {
  stepPostZoom(event.deltaY < 0 ? 1 : -1)
}

function onPostPointerDown(event: PointerEvent) {
  if (!postSpace.value || event.button !== 0) return
  postPanning.value = { x: event.clientX, y: event.clientY }
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
  event.preventDefault()
}

function onPostPointerMove(event: PointerEvent) {
  if (!postPanning.value) return
  postPan.x += event.clientX - postPanning.value.x
  postPan.y += event.clientY - postPanning.value.y
  postPanning.value = { x: event.clientX, y: event.clientY }
}

function onPostPointerUp() {
  postPanning.value = null
}

function onPostKeyDown(event: KeyboardEvent) {
  if (!postOpen.value || event.code !== 'Space') return
  const target = event.target as HTMLElement | null
  if (target && /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)) return
  postSpace.value = true
  event.preventDefault()
}

function onPostKeyUp(event: KeyboardEvent) {
  if (event.code !== 'Space') return
  postSpace.value = false
  postPanning.value = null
}

function onPostBlur() {
  postSpace.value = false
  postPanning.value = null
}

const postCursor = computed(() => {
  if (postPanning.value) return 'grabbing'
  if (postSpace.value) return 'grab'
  return 'default'
})
const postViewStyle = computed(() => ({
  transform: `translate(${postPan.x}px, ${postPan.y}px) scale(${postZoom.value})`,
}))
const postPreview = ref<PostProcessPreview | null>(null)
/** Payload the visible preview was rendered from, to catch edited settings. */
const postPreviewKey = ref('')
const postBeforeURL = ref('')
const postBusy = ref(false)
const postSaving = ref(false)
const paletteMode = ref<'palette' | 'auto'>('palette')
const postOptions = reactive<PostProcessOptions>({
  width: 0,
  height: 0,
  trim: false,
  alphaThreshold: 128,
  maxColors: 16,
  colors: POST_COLORS,
  dither: 'none',
  ditherStrength: 0.8,
  cleanup: true,
  outline: false,
  outlineColor: '#0c1016',
  checkSeam: false,
})

const SOURCE_KIND: Record<string, string> = {
  imported: '导入',
  generated: '生成',
  procedural: '程序化',
  blank: '手绘',
}

function showNotice(text: string) {
  notice.value = text
  if (noticeTimer) clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => {
    if (notice.value === text) notice.value = null
  }, 5000)
}

onBeforeUnmount(() => {
  if (noticeTimer) clearTimeout(noticeTimer)
  window.removeEventListener('keydown', onPostKeyDown)
  window.removeEventListener('keyup', onPostKeyUp)
  window.removeEventListener('blur', onPostBlur)
})

async function runImport() {
  const paths = await platform.pickImageFiles()
  if (paths.length === 0) return
  const summary = await store.importAssets(paths, importType.value)
  if (!summary) return
  const parts: string[] = []
  if (summary.imported.length > 0) parts.push(`导入 ${summary.imported.length} 个`)
  if (summary.duplicates.length > 0) {
    parts.push(`跳过 ${summary.duplicates.length} 个重复（${summary.duplicates[0].name}）`)
  }
  if (summary.failed.length > 0) parts.push(`${summary.failed.length} 个失败`)
  let text = parts.join(' · ') || '没有可导入的文件'
  if (summary.failed.length > 0) text += `：${summary.failed[0].message}`
  showNotice(text)
}

const selected = computed<Asset | null>(
  () => store.assets.find((asset) => asset.id === selectedId.value) ?? null,
)
const approvedCount = computed(
  () => store.assets.filter((asset) => asset.status === 'approved').length,
)
const paletteColors = computed(
  () =>
    store.palettes.find((palette) => palette.id === selected.value?.paletteId)?.colors ?? [],
)
const linkedQuests = computed(() =>
  store.quests.filter((quest) => selected.value?.questIds.includes(quest.id)),
)
const linkedEntities = computed(() =>
  store.entities.filter(
    (entity) => selected.value !== null && entity.assetIds.includes(selected.value.id),
  ),
)

function openEntity(entityId: string) {
  selectedId.value = null
  store.requestFocus('entity', entityId)
  store.setRoute('lore')
}
const reviewLinkedCount = computed(
  () => linkedQuests.value.filter((quest) => quest.status === 'review').length,
)
const currentVersion = computed(() =>
  selected.value?.versions.find((version) => version.id === selected.value?.currentVersion),
)

const checkedAssets = computed(() =>
  store.assets.filter((asset) => checkedIds.value.includes(asset.id)),
)
const allFilteredChecked = computed(
  () =>
    filtered.value.length > 0 &&
    filtered.value.every((asset) => checkedIds.value.includes(asset.id)),
)

/* ------------------------------------------------ map usage of one asset */

interface MapReference {
  id: string
  name: string
  roles: string[]
}

const mapRefs = ref<Record<string, MapReference[]>>({})
const mapRefsPending = new Set<string>()
const mapRefsLoading = ref(false)
const currentMapRefs = computed<MapReference[]>(() =>
  selected.value ? mapRefs.value[selected.value.id] ?? [] : [],
)

async function loadMapRefs(assetId: string) {
  if (mapRefs.value[assetId] || mapRefsPending.has(assetId)) return
  mapRefsPending.add(assetId)
  mapRefsLoading.value = true
  try {
    const refs: MapReference[] = []
    for (const summary of await platform.listMaps()) {
      let map: GameMap
      try {
        map = await platform.loadMap(summary.id)
      } catch {
        // One unreadable map must not hide the references in the others.
        continue
      }
      const roles: string[] = []
      if (map.tilesets.some((set) => set.assetId === assetId)) roles.push('图集')
      if (map.objects.some((object) => object.assetId === assetId)) roles.push('对象')
      if (roles.length > 0) refs.push({ id: map.id, name: map.name, roles })
    }
    mapRefs.value = { ...mapRefs.value, [assetId]: refs }
  } catch {
    /* listMaps itself failed; the detail just shows no references */
    mapRefs.value = { ...mapRefs.value, [assetId]: [] }
  } finally {
    mapRefsPending.delete(assetId)
    mapRefsLoading.value = mapRefsPending.size > 0
  }
}

function openMapReference(ref: MapReference) {
  selectedId.value = null
  store.requestFocus('map', ref.id)
  store.setRoute('maps')
}

watch(selectedId, (id) => {
  if (id) void loadMapRefs(id)
})

const filtered = computed(() => {
  const needle = search.value.trim().toLowerCase()
  return store.assets.filter((asset) => {
    if (typeFilter.value !== 'all' && asset.type !== typeFilter.value) return false
    if (statusFilter.value !== 'all' && asset.status !== statusFilter.value) return false
    if (!needle) return true
    return (
      asset.name.toLowerCase().includes(needle) ||
      asset.tags.some((tag) => tag.toLowerCase().includes(needle))
    )
  })
})

const filtersActive = computed(
  () => typeFilter.value !== 'all' || statusFilter.value !== 'all' || search.value.trim() !== '',
)

function clearFilters() {
  typeFilter.value = 'all'
  statusFilter.value = 'all'
  search.value = ''
}

function value(event: Event): string {
  return (event.target as HTMLSelectElement).value
}

function setTypeFilter(event: Event) {
  typeFilter.value = value(event) as AssetType | 'all'
}

function setStatusFilter(event: Event) {
  statusFilter.value = value(event) as AssetStatus | 'all'
}

function onModalOpen(open: boolean) {
  if (!open) {
    selectedId.value = null
    postOpen.value = false
    compareOpen.value = false
  }
}

function onPostModal(open: boolean) {
  if (!open) postOpen.value = false
}

function postPayload(): PostProcessOptions {
  return {
    ...postOptions,
    colors: paletteMode.value === 'palette' ? [...postOptions.colors] : [],
  }
}

async function runPreview() {
  const asset = selected.value
  if (!asset) return
  const payload = postPayload()
  postBusy.value = true
  try {
    const preview = await store.previewPostProcess(asset.id, payload)
    if (preview && postOpen.value) {
      postPreview.value = preview
      postPreviewKey.value = JSON.stringify(payload)
      postZoom.value = Math.max(preview.width, preview.height) <= 64 ? 4 : 1
      postPan.x = 0
      postPan.y = 0
    }
  } finally {
    postBusy.value = false
  }
}

/** Settings changed after the preview, so the shown image is no longer it. */
const postStale = computed(
  () => Boolean(postPreview.value) && postPreviewKey.value !== JSON.stringify(postPayload()),
)

/** Rescales the target box to a common long side and refreshes the preview. */
function applySizePreset(longSide: number) {
  const asset = selected.value
  if (!asset) return
  const w = Math.max(1, asset.dimensions.w)
  const h = Math.max(1, asset.dimensions.h)
  const scale = longSide / Math.max(w, h)
  postOptions.width = Math.max(1, Math.round(w * scale))
  postOptions.height = Math.max(1, Math.round(h * scale))
  void runPreview()
}

const compareOpen = ref(false)
const compareBase = ref('')
const compareTarget = ref('')
const compareUrls = ref({ base: '', target: '' })
const compareBusy = ref(false)

function versionURL(asset: Asset, versionId: string): Promise<string> {
  return platform.assetURL(asset.id, `versions/${versionId}/output.png`)
}

async function loadCompareImages() {
  const asset = selected.value
  if (!asset) return
  compareBusy.value = true
  compareUrls.value = { base: '', target: '' }
  try {
    const [base, target] = await Promise.all([
      versionURL(asset, compareBase.value),
      versionURL(asset, compareTarget.value),
    ])
    compareUrls.value = { base, target }
  } catch (raw) {
    store.setError(raw)
  } finally {
    compareBusy.value = false
  }
}

/** Opens an A/B comparison; the current version is the default right pane. */
function openCompare(versionId: string) {
  const asset = selected.value
  if (!asset) return
  compareBase.value = versionId
  compareTarget.value = asset.currentVersion ?? versionId
  compareOpen.value = true
  void loadCompareImages()
}

function onCompareModal(open: boolean) {
  if (!open) compareOpen.value = false
}

function openPostProcess() {
  const asset = selected.value
  if (!asset || asset.animation) return
  const palette = store.palettes.find((item) => item.id === asset.paletteId)
  const paletteColors = palette?.colors ?? []
  const w = Math.max(1, asset.dimensions.w)
  const h = Math.max(1, asset.dimensions.h)
  const longest = Math.max(w, h)
  // Large AI renders start at a 128px long side; small sprites keep their size.
  const scale = longest > 256 ? 128 / longest : 1
  paletteMode.value = paletteColors.length > 0 ? 'palette' : 'auto'
  Object.assign(postOptions, {
    width: Math.max(1, Math.round(w * scale)),
    height: Math.max(1, Math.round(h * scale)),
    trim: false,
    alphaThreshold: 128,
    maxColors: paletteColors.length > 0 ? Math.min(16, paletteColors.length) : 16,
    colors: paletteColors.length > 0 ? [...paletteColors] : [...POST_COLORS],
    dither: 'none' as DitherMode,
    ditherStrength: 0.8,
    cleanup: true,
    outline: asset.type === 'sprite' || asset.type === 'building',
    outlineColor: darkestColor(paletteColors) ?? '#0c1016',
    checkSeam: asset.type === 'tile',
  })
  postPreview.value = null
  postPreviewKey.value = ''
  postBeforeURL.value = ''
  postOpen.value = true
  void platform
    .assetSource(asset.id)
    .then((source) => {
      if (postOpen.value) postBeforeURL.value = source.url
    })
    .catch(() => {
      /* the preview pane just stays empty */
    })
  void runPreview()
}

async function savePostProcessed() {
  const asset = selected.value
  if (!asset || !postPreview.value) return
  postSaving.value = true
  try {
    const saved = await store.savePostProcessedVersion(asset.id, postPayload())
    if (!saved) return
    postOpen.value = false
    showNotice(
      `已保存 v${saved.currentVersion} 像素化版本（${saved.dimensions.w}×${saved.dimensions.h}）`,
    )
  } finally {
    postSaving.value = false
  }
}

function openEditor() {
  const asset = selected.value
  if (!asset) return
  store.openEditor(asset.id)
  selectedId.value = null
  postOpen.value = false
}

const exportBusy = ref(false)

async function exportSelected() {
  const asset = selected.value
  if (!asset || exportBusy.value) return
  exportBusy.value = true
  try {
    const result = await platform.exportAsset(asset.id)
    showNotice(`已导出到 ${shortPath(result.dir, 64)}（${result.files.length} 个文件）`)
  } catch (raw) {
    store.setError(raw)
  } finally {
    exportBusy.value = false
  }
}

async function openExports() {
  try {
    await platform.openExportsFolder()
  } catch (raw) {
    store.setError(raw)
  }
}

const statusBusy = ref(false)

/** Saves the review status; approving also completes linked review quests. */
async function setStatus(event: Event) {
  const asset = selected.value
  const status = value(event) as AssetStatus
  if (!asset || asset.status === status || statusBusy.value) return
  statusBusy.value = true
  try {
    const result = await store.setAssetStatus(asset.id, status)
    if (!result) return
    const label = t(`assetStatus.${status}`)
    if (result.promoted.length > 0) {
      const titles = result.promoted.map((quest) => `「${quest.title}」`).join('、')
      showNotice(`已标记为${label}；${titles} 已自动标记完成`)
    } else {
      showNotice(`已标记为${label}`)
    }
  } finally {
    statusBusy.value = false
  }
}

/* ------------------------------------------------------ batch selection */

function isChecked(id: string): boolean {
  return checkedIds.value.includes(id)
}

function toggleChecked(id: string) {
  checkedIds.value = isChecked(id)
    ? checkedIds.value.filter((item) => item !== id)
    : [...checkedIds.value, id]
  lastCheckedId = id
}

/** Ctrl/Cmd toggles, Shift extends from the last click; a plain click opens the detail. */
function onCardClick(asset: Asset, event: MouseEvent) {
  if (event.shiftKey && lastCheckedId) {
    const start = filtered.value.findIndex((item) => item.id === lastCheckedId)
    const end = filtered.value.findIndex((item) => item.id === asset.id)
    if (start >= 0 && end >= 0) {
      const [from, to] = start <= end ? [start, end] : [end, start]
      const next = new Set(checkedIds.value)
      for (const item of filtered.value.slice(from, to + 1)) next.add(item.id)
      checkedIds.value = [...next]
      return
    }
  }
  if (event.ctrlKey || event.metaKey) {
    toggleChecked(asset.id)
    return
  }
  selectedId.value = asset.id
  lastCheckedId = asset.id
}

function selectAllFiltered() {
  const next = new Set(filtered.value.map((asset) => asset.id))
  checkedIds.value = [...next]
}

function clearChecked() {
  checkedIds.value = []
  lastCheckedId = null
}

async function applyBatchStatus(event: Event) {
  const status = value(event) as AssetStatus | ''
  batchStatusChoice.value = ''
  const targets = checkedAssets.value
  if (!status || targets.length === 0 || batchBusy.value) return
  batchBusy.value = true
  let changed = 0
  let failed = 0
  let promoted = 0
  try {
    for (const asset of targets) {
      if (asset.status === status) {
        changed += 1
        continue
      }
      const result = await store.setAssetStatus(asset.id, status)
      if (!result) {
        failed += 1
        continue
      }
      changed += 1
      promoted += result.promoted.length
    }
    const label = t(`assetStatus.${status}`)
    const parts = [`${changed} 个素材标记为${label}`]
    if (promoted > 0) parts.push(`${promoted} 条任务自动完成`)
    if (failed > 0) parts.push(`${failed} 个失败`)
    showNotice(parts.join('，'))
  } finally {
    batchBusy.value = false
  }
}

async function applyBatchTag(mode: 'add' | 'remove') {
  const tag = batchTag.value.trim()
  const targets = checkedAssets.value
  if (!tag || targets.length === 0 || batchBusy.value) return
  batchBusy.value = true
  try {
    const { changed, failed } = await store.applyAssetTags(
      targets.map((asset) => asset.id),
      tag,
      mode,
    )
    const verb = mode === 'add' ? '添加' : '移除'
    const parts = [`${changed} 个素材${verb}标签「${tag}」`]
    if (failed > 0) parts.push(`${failed} 个失败`)
    showNotice(parts.join('，'))
    if (mode === 'add' && changed > 0) batchTag.value = ''
  } finally {
    batchBusy.value = false
  }
}

async function exportChecked() {
  const targets = checkedAssets.value
  if (targets.length === 0 || batchBusy.value) return
  batchBusy.value = true
  let exported = 0
  let files = 0
  let failed = 0
  try {
    for (const asset of targets) {
      if (!asset.currentVersion) {
        failed += 1
        continue
      }
      try {
        const result = await platform.exportAsset(asset.id)
        exported += 1
        files += result.files.length
      } catch {
        failed += 1
      }
    }
    const parts = [`导出 ${exported} 个素材（${files} 个文件）`]
    if (failed > 0) parts.push(`${failed} 个跳过或失败`)
    showNotice(parts.join('，'))
  } finally {
    batchBusy.value = false
  }
}

function confirmDelete(label: string): Promise<boolean> {
  return confirmDialog({
    title: '删除素材',
    message: `将${label}移入项目回收站？地图图集与任务里的引用会失效。`,
    detail: '素材目录、原图与所有版本会一起移动到 .workbench/trash/assets/，可在文件系统中找回。',
    confirmText: '移入回收站',
    tone: 'danger',
  })
}

async function deleteChecked() {
  const targets = checkedAssets.value
  if (targets.length === 0 || batchBusy.value) return
  if (!(await confirmDelete(`${targets.length} 个素材`))) return
  batchBusy.value = true
  let deleted = 0
  let failed = 0
  try {
    for (const asset of targets) {
      if (await store.deleteAsset(asset.id)) deleted += 1
      else failed += 1
    }
    checkedIds.value = []
    lastCheckedId = null
    const parts = [`${deleted} 个素材已移入回收站`]
    if (failed > 0) parts.push(`${failed} 个失败`)
    showNotice(parts.join('，'))
  } finally {
    batchBusy.value = false
  }
}

async function deleteSelected() {
  const asset = selected.value
  if (!asset || batchBusy.value) return
  if (!(await confirmDelete(`「${asset.name}」`))) return
  batchBusy.value = true
  try {
    if (await store.deleteAsset(asset.id)) {
      selectedId.value = null
      showNotice(`「${asset.name}」已移入回收站`)
    }
  } finally {
    batchBusy.value = false
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-baseline gap-3">
        <h2 class="text-lg font-semibold tracking-tight text-ink">素材库</h2>
        <span class="font-mono text-[11px] text-faint">{{ store.assets.length }} 个素材</span>
        <span class="font-mono text-[11px] text-faint">{{ approvedCount }} 个已批准</span>
        <span v-if="filtersActive" class="font-mono text-[11px] text-faint">
          筛选出 {{ filtered.length }} 个
        </span>
      </div>
      <div class="flex flex-wrap items-center justify-end gap-2">
        <div class="flex rounded-lg border border-line bg-panel p-0.5" role="group" aria-label="素材视图">
          <button
            type="button"
            class="grid size-8 place-items-center rounded-md transition-colors"
            :class="viewMode === 'grid' ? 'bg-panel-3 text-ink' : 'text-faint hover:text-ink'"
            :aria-pressed="viewMode === 'grid'"
            title="网格视图"
            @click="viewMode = 'grid'"
          >
            <LayoutGrid :size="14" />
          </button>
          <button
            type="button"
            class="grid size-8 place-items-center rounded-md transition-colors"
            :class="viewMode === 'list' ? 'bg-panel-3 text-ink' : 'text-faint hover:text-ink'"
            :aria-pressed="viewMode === 'list'"
            title="列表视图"
            @click="viewMode = 'list'"
          >
            <List :size="14" />
          </button>
        </div>
        <div class="relative">
          <Search :size="13" class="absolute top-1/2 left-2.5 -translate-y-1/2 text-faint" />
          <input
          v-model="search"
          class="pb-input h-9 w-48 py-1 pl-8 text-xs"
          placeholder="搜索名称或标签"
          aria-label="搜索素材"
        />
        </div>
        <select
          class="pb-input h-9 w-28 py-1 text-xs"
          :value="typeFilter"
          aria-label="按类型筛选"
          @change="setTypeFilter"
        >
          <option value="all">全部类型</option>
          <option v-for="type in ASSET_TYPES" :key="type" :value="type">
            {{ t(`assetType.${type}`) }}
          </option>
        </select>
        <select
          class="pb-input h-9 w-28 py-1 text-xs"
          :value="statusFilter"
          aria-label="按状态筛选"
          @change="setStatusFilter"
        >
          <option value="all">全部状态</option>
          <option v-for="status in ASSET_STATUSES" :key="status" :value="status">
            {{ t(`assetStatus.${status}`) }}
          </option>
        </select>
        <button
          v-if="filtersActive"
          type="button"
          class="pb-btn pb-btn-ghost h-9 shrink-0 px-2 py-1 text-[11px] whitespace-nowrap"
          @click="clearFilters()"
        >
          清除筛选
        </button>
        <span class="shrink-0 text-[10px] text-faint whitespace-nowrap">导入为</span>
        <select
          v-model="importType"
          class="pb-input h-9 w-24 py-1 text-xs"
          :disabled="store.busy"
          aria-label="新导入素材的类型"
          title="新导入素材的默认类型"
        >
          <option v-for="type in ASSET_TYPES" :key="type" :value="type">
            {{ t(`assetType.${type}`) }}
          </option>
        </select>
        <button
          type="button"
          class="pb-btn h-9 py-1"
          :disabled="store.busy"
          title="支持 PNG / JPEG / GIF / WebP / BMP，按内容哈希去重"
          @click="runImport()"
        >
          <Loader2 v-if="store.busy" :size="14" class="animate-spin" />
          <Upload v-else :size="14" />
          导入
        </button>
        <button
          type="button"
          class="pb-btn h-9 py-1"
          :disabled="store.busy"
          title="新建一张空画布，直接在像素编辑器里手绘"
          @click="openNewAsset()"
        >
          <Plus :size="14" />
          新建素材
        </button>
        <button
          type="button"
          class="pb-btn pb-btn-primary h-9 py-1"
          title="AI 生成角色、建筑、瓦片与特效，后处理成调色板锁定的像素素材"
          @click="store.setRoute('generate')"
        >
          <Sparkles :size="14" />
          AI 生成
        </button>
      </div>
    </div>

    <div
      v-if="notice"
      class="pb-fade-in rounded-lg border border-line bg-panel-2 px-3 py-2 text-[11px] text-ink"
    >
      {{ notice }}
    </div>

    <div
      v-if="checkedIds.length > 0"
      class="flex flex-wrap items-center gap-2 rounded-lg border border-accent/30 bg-accent/5 px-3 py-2 text-[11px]"
    >
      <span class="font-medium text-ink">已选 {{ checkedIds.length }} 个</span>
      <span class="text-faint">Ctrl+点击逐个选择，Shift+点击选择范围</span>
      <button
        type="button"
        class="pb-btn h-7 px-2 py-0 text-[11px]"
        :disabled="allFilteredChecked || filtered.length === 0"
        @click="selectAllFiltered()"
      >
        全选当前 {{ filtered.length }} 个
      </button>
      <button type="button" class="pb-btn h-7 px-2 py-0 text-[11px]" @click="clearChecked()">清空</button>
      <span class="mx-1 h-5 w-px bg-line" />
      <select
        class="pb-input h-7 w-28 py-0 text-[11px]"
        :value="batchStatusChoice"
        :disabled="batchBusy"
        aria-label="批量设置评审状态"
        @change="applyBatchStatus"
      >
        <option value="">标记为…</option>
        <option v-for="status in ASSET_STATUSES" :key="status" :value="status">
          {{ t(`assetStatus.${status}`) }}
        </option>
      </select>
      <span class="mx-1 h-5 w-px bg-line" />
      <input
        v-model="batchTag"
        class="pb-input h-7 w-24 py-0 text-[11px]"
        maxlength="24"
        placeholder="标签"
        aria-label="批量标签"
        :disabled="batchBusy"
        @keydown.enter.prevent="applyBatchTag('add')"
      />
      <button
        type="button"
        class="pb-btn h-7 px-2 py-0 text-[11px]"
        :disabled="batchBusy || !batchTag.trim()"
        title="给所选素材添加这个标签"
        @click="applyBatchTag('add')"
      >
        加标签
      </button>
      <button
        type="button"
        class="pb-btn h-7 px-2 py-0 text-[11px]"
        :disabled="batchBusy || !batchTag.trim()"
        title="从所选素材移除这个标签"
        @click="applyBatchTag('remove')"
      >
        移除标签
      </button>
      <span class="mx-1 h-5 w-px bg-line" />
      <button
        type="button"
        class="pb-btn h-7 px-2 py-0 text-[11px]"
        :disabled="batchBusy"
        title="把所选素材的当前版本复制到 exports/"
        @click="exportChecked()"
      >
        <Loader2 v-if="batchBusy" :size="12" class="animate-spin" />
        <Download v-else :size="12" />
        批量导出
      </button>
      <button
        type="button"
        class="pb-btn h-7 px-2 py-0 text-[11px] hover:text-danger"
        :disabled="batchBusy"
        title="把所选素材的整个目录移到项目回收站"
        @click="deleteChecked()"
      >
        <Loader2 v-if="batchBusy" :size="12" class="animate-spin" />
        <Trash2 v-else :size="12" />
        删除
      </button>
    </div>

    <EmptyState
      v-if="store.assets.length === 0"
      title="还没有素材"
      hint="用 AI 生成或导入图片，再处理成调色板锁定的像素素材。"
    >
      <template #icon><Images :size="24" /></template>
      <template #action>
        <div class="flex flex-wrap justify-center gap-2">
          <button
            type="button"
            class="pb-btn pb-btn-primary text-xs"
            @click="store.setRoute('generate')"
          >
            <Sparkles :size="13" />
            AI 生成素材
          </button>
          <button type="button" class="pb-btn text-xs" :disabled="store.busy" @click="runImport()">
            <Upload :size="13" />
            导入素材
          </button>
          <button type="button" class="pb-btn text-xs" @click="openNewAsset()">
            <Plus :size="13" />
            手绘素材
          </button>
        </div>
      </template>
    </EmptyState>

    <EmptyState
      v-else-if="filtered.length === 0"
      title="没有匹配的素材"
      hint="换个关键词，或把类型与状态筛选恢复为「全部」。"
    >
      <template #icon><ImageOff :size="22" /></template>
    </EmptyState>

    <div
      v-else-if="viewMode === 'grid'"
      class="grid grid-cols-[repeat(auto-fill,minmax(158px,1fr))] gap-3"
    >
      <button
        v-for="asset in filtered"
        :key="asset.id"
        type="button"
        class="group relative flex flex-col overflow-hidden rounded-xl border bg-panel text-left transition-colors"
        :class="
          isChecked(asset.id)
            ? 'border-accent/70 bg-accent/5'
            : 'border-line hover:border-accent/40 hover:bg-panel-2'
        "
        @click="onCardClick(asset, $event)"
      >
        <span
          role="checkbox"
          :aria-checked="isChecked(asset.id)"
          :aria-label="`选择 ${asset.name}`"
          :title="isChecked(asset.id) ? '取消选择' : '加入批量选择（Ctrl+点击 / Shift+点击范围）'"
          class="absolute top-2 left-2 z-10 grid size-4 place-items-center rounded border transition-opacity"
          :class="
            isChecked(asset.id)
              ? 'border-accent bg-accent text-canvas opacity-100'
              : 'border-white/35 bg-black/45 text-transparent opacity-0 group-hover:opacity-100'
          "
          @click.stop="toggleChecked(asset.id)"
        >
          <Check :size="10" :stroke-width="3" />
        </span>
        <div class="p-2 pb-0">
          <div class="aspect-square w-full">
            <AssetThumb :asset="asset" size="fill" class="h-full w-full" />
          </div>
        </div>
        <div class="min-w-0 p-2.5">
          <div class="truncate text-xs font-medium text-ink">{{ asset.name }}</div>
          <div class="mt-1 flex items-center justify-between gap-1">
            <span class="text-[10px] text-faint">
              {{ t(`assetType.${asset.type}`) }} · {{ asset.dimensions.w }}×{{ asset.dimensions.h }}
            </span>
            <span class="pb-chip" :class="ASSET_STATUS_CLASS[asset.status]">
              {{ t(`assetStatus.${asset.status}`) }}
            </span>
          </div>
          <div v-if="asset.tags.length > 0" class="mt-1.5 flex flex-wrap gap-1">
            <span
              v-for="tag in asset.tags.slice(0, 3)"
              :key="tag"
              class="rounded bg-panel-3 px-1 py-0.5 text-[9px] text-faint"
            >
              {{ tag }}
            </span>
          </div>
        </div>
      </button>
    </div>

    <div v-else class="overflow-hidden rounded-xl border border-line bg-panel">
      <div
        class="hidden items-center gap-3 border-b border-line bg-panel-2/60 px-3 py-2 text-[10px] text-faint sm:flex"
      >
        <span class="w-4" />
        <span class="w-10" />
        <span class="flex-1">名称</span>
        <span class="w-16">类型</span>
        <span class="w-20">状态</span>
        <span class="w-24 text-right">尺寸</span>
        <span class="w-28 text-right">最近更新</span>
      </div>
      <button
        v-for="asset in filtered"
        :key="asset.id"
        type="button"
        class="group flex w-full items-center gap-3 border-b border-line-soft px-3 py-2 text-left transition-colors last:border-b-0"
        :class="isChecked(asset.id) ? 'bg-accent/5' : 'hover:bg-panel-2'"
        @click="onCardClick(asset, $event)"
      >
        <span
          role="checkbox"
          :aria-checked="isChecked(asset.id)"
          :aria-label="`选择 ${asset.name}`"
          :title="isChecked(asset.id) ? '取消选择' : '加入批量选择（Ctrl+点击 / Shift+点击范围）'"
          class="grid size-4 shrink-0 place-items-center rounded border transition-colors"
          :class="
            isChecked(asset.id)
              ? 'border-accent bg-accent text-canvas'
              : 'border-line bg-panel-2 text-transparent group-hover:border-faint'
          "
          @click.stop="toggleChecked(asset.id)"
        >
          <Check :size="10" :stroke-width="3" />
        </span>
        <span class="size-10 shrink-0 overflow-hidden rounded-md border border-line bg-panel-2/40">
          <AssetThumb :asset="asset" :size="40" class="size-full" />
        </span>
        <span class="min-w-0 flex-1">
          <span class="block truncate text-xs font-medium text-ink">{{ asset.name }}</span>
          <span v-if="asset.tags.length > 0" class="mt-0.5 block truncate text-[9px] text-faint">
            {{ asset.tags.slice(0, 4).join(' · ') }}
          </span>
        </span>
        <span class="w-16 shrink-0 text-[10px] text-faint">{{ t(`assetType.${asset.type}`) }}</span>
        <span class="w-20 shrink-0">
          <span class="pb-chip" :class="ASSET_STATUS_CLASS[asset.status]">
            {{ t(`assetStatus.${asset.status}`) }}
          </span>
        </span>
        <span class="hidden w-24 shrink-0 text-right font-mono text-[10px] text-faint sm:block">
          {{ asset.dimensions.w }}×{{ asset.dimensions.h }}
        </span>
        <span class="hidden w-28 shrink-0 text-right text-[10px] text-faint sm:block">
          {{ relativeTime(asset.updatedAt) }}
        </span>
      </button>
    </div>

    <AppModal
      :open="Boolean(selected)"
      wide
      :title="selected?.name ?? ''"
      :description="selected ? `素材 ID ${selected.id}` : ''"
      @update:open="onModalOpen"
    >
      <div v-if="selected" class="grid gap-5 sm:grid-cols-[240px_1fr]">
        <div>
          <button
            type="button"
            class="group relative flex aspect-square w-full cursor-zoom-in items-center justify-center rounded-xl border border-line bg-panel-2/30 p-3"
            :title="selected.animation ? `查看动画（${selected.animation.frameCount ?? '多'} 帧）` : '查看原图'"
            @click="openAssetPreview(selected)"
          >
            <AssetThumb :asset="selected" :size="200" />
            <span
              class="pointer-events-none absolute right-1.5 bottom-1.5 rounded-md bg-black/60 p-1 text-white/80 opacity-0 transition-opacity group-hover:opacity-100"
            >
              <Maximize2 :size="12" />
            </span>
          </button>
          <div class="mt-2 flex items-center justify-between text-[10px] text-faint">
            <span>{{ selected.dimensions.w }} × {{ selected.dimensions.h }} px</span>
            <span class="font-mono">pivot {{ selected.pivot.x }},{{ selected.pivot.y }}</span>
          </div>
          <div v-if="paletteColors.length > 0" class="mt-3">
            <div class="pb-label">调色板</div>
            <div class="mt-1.5 flex flex-wrap gap-1">
              <span
                v-for="color in paletteColors"
                :key="color"
                :title="color"
                class="size-4 rounded border border-black/30"
                :style="{ background: color }"
              />
            </div>
          </div>
        </div>

        <div class="space-y-4">
          <div class="flex flex-wrap items-center gap-2">
            <span class="pb-chip">{{ t(`assetType.${selected.type}`) }}</span>
            <span class="pb-chip" :class="ASSET_STATUS_CLASS[selected.status]">
              {{ t(`assetStatus.${selected.status}`) }}
            </span>
            <span class="pb-chip">许可证 {{ selected.license }}</span>
            <span v-if="selected.animation" class="pb-chip">
              {{ animationLabel(selected.animation) }}
            </span>
          </div>

          <div class="flex flex-wrap items-center gap-2 rounded-lg border border-line-soft bg-canvas/40 px-2.5 py-1.5">
            <span class="pb-label">评审状态</span>
            <select
              class="pb-input h-7 w-24 py-0 text-[11px]"
              :value="selected.status"
              :disabled="statusBusy || store.busy"
              @change="setStatus"
            >
              <option v-for="status in ASSET_STATUSES" :key="status" :value="status">
                {{ t(`assetStatus.${status}`) }}
              </option>
            </select>
            <span v-if="reviewLinkedCount > 0" class="text-[10px] text-accent">
              批准后，{{ reviewLinkedCount }} 条待验收任务会自动完成
            </span>
            <span v-else class="text-[10px] text-faint">批准代表这张素材可以进地图与导出</span>
          </div>

          <dl class="grid grid-cols-2 gap-x-4 gap-y-2 rounded-xl border border-line bg-panel-2/40 p-3 text-[11px]">
            <div>
              <dt class="text-faint">来源</dt>
              <dd class="mt-0.5 truncate font-mono text-[10px] text-ink">
                {{ SOURCE_KIND[selected.source.kind] ?? selected.source.kind }}
              </dd>
            </div>
            <div>
              <dt class="text-faint">提供方</dt>
              <dd class="mt-0.5 truncate font-mono text-[10px] text-ink">{{ selected.source.provider ?? '—' }}</dd>
            </div>
            <div>
              <dt class="text-faint">工作流</dt>
              <dd class="mt-0.5 truncate font-mono text-[10px] text-ink">{{ selected.source.workflow ?? '—' }}</dd>
            </div>
            <div>
              <dt class="text-faint">当前版本</dt>
              <dd class="mt-0.5 truncate font-mono text-[10px] text-ink">{{ selected.currentVersion ?? '无' }}</dd>
            </div>
            <div>
              <dt class="text-faint">创建</dt>
              <dd class="mt-0.5 truncate font-mono text-[10px] text-ink">{{ formatDate(selected.createdAt) }}</dd>
            </div>
            <div>
              <dt class="text-faint">更新</dt>
              <dd class="mt-0.5 truncate font-mono text-[10px] text-ink">{{ relativeTime(selected.updatedAt) }}</dd>
            </div>
          </dl>

          <div v-if="selected.tags.length > 0">
            <div class="pb-label">标签</div>
            <div class="mt-1.5 flex flex-wrap gap-1.5">
              <span v-for="tag in selected.tags" :key="tag" class="pb-chip">{{ tag }}</span>
            </div>
          </div>

          <div>
            <div class="pb-label">版本</div>
            <p
              v-if="selected.versions.length === 0"
              class="mt-1.5 rounded-lg border border-dashed border-line px-3 py-3 text-[11px] text-faint"
            >
              这个素材还没有正式版本。生成或编辑后，每次结果都会保存为不可变版本。
            </p>
            <ul v-else class="mt-1.5 divide-y divide-line-soft rounded-lg border border-line">
              <li
                v-for="version in selected.versions"
                :key="version.id"
                class="flex items-center gap-2 px-3 py-2 text-[11px]"
              >
                <span class="font-mono text-ink">v{{ version.id }}</span>
                <span
                  v-if="currentVersion?.id === version.id"
                  class="rounded bg-green/10 px-1 text-[9px] text-green"
                >
                  当前
                </span>
                <span class="ml-auto font-mono text-[10px] text-faint">{{ formatBytes(version.size) }}</span>
                <span class="font-mono text-[10px] text-faint">{{ version.hash.slice(0, 10) }}</span>
                <button
                  type="button"
                  class="pb-btn h-6 shrink-0 px-1.5 py-0 text-[10px]"
                  title="并排对比两个版本"
                  @click="openCompare(version.id)"
                >
                  对比
                </button>
              </li>
            </ul>
          </div>

          <div>
            <div class="pb-label">关联任务</div>
            <p v-if="linkedQuests.length === 0" class="mt-1.5 text-[11px] text-faint">没有关联任务</p>
            <ul v-else class="mt-1.5 space-y-1">
              <li
                v-for="quest in linkedQuests"
                :key="quest.id"
                class="rounded-md border border-line bg-panel-2/40 px-2.5 py-1.5 text-[11px] text-ink"
              >
                {{ quest.title }}
              </li>
            </ul>
          </div>

          <div>
            <div class="pb-label">设定引用</div>
            <p v-if="linkedEntities.length === 0" class="mt-1.5 text-[11px] text-faint">
              还没有设定使用这个素材。
            </p>
            <ul v-else class="mt-1.5 space-y-1">
              <li v-for="entity in linkedEntities" :key="entity.id">
                <button
                  type="button"
                  class="flex w-full items-center gap-2 rounded-md border border-line bg-panel-2/40 px-2.5 py-1.5 text-left text-[11px] text-ink transition-colors hover:border-accent/40"
                  @click="openEntity(entity.id)"
                >
                  <BookOpen :size="12" class="text-accent" />
                  <span class="min-w-0 flex-1 truncate">{{ entity.name }}</span>
                  <span class="text-[9px] text-faint">{{ t(`entityKind.${entity.kind}`) }}</span>
                </button>
              </li>
            </ul>
          </div>

          <div>
            <div class="flex items-center justify-between">
              <div class="pb-label">地图引用</div>
              <span v-if="mapRefsLoading" class="text-[10px] text-faint">检查中…</span>
            </div>
            <p
              v-if="!mapRefsLoading && currentMapRefs.length === 0"
              class="mt-1.5 text-[11px] text-faint"
            >
              还没有地图引用这个素材。
            </p>
            <ul v-else-if="currentMapRefs.length > 0" class="mt-1.5 space-y-1">
              <li v-for="ref in currentMapRefs" :key="ref.id">
                <button
                  type="button"
                  class="flex w-full items-center gap-2 rounded-md border border-line bg-panel-2/40 px-2.5 py-1.5 text-left text-[11px] text-ink transition-colors hover:border-accent/40"
                  @click="openMapReference(ref)"
                >
                  <MapPin :size="12" class="text-accent" />
                  <span class="min-w-0 flex-1 truncate">{{ ref.name }}</span>
                  <span class="text-[9px] text-faint">{{ ref.roles.join(' / ') }}</span>
                </button>
              </li>
            </ul>
          </div>
        </div>
      </div>

      <template #footer>
        <span class="mr-auto text-[11px] text-faint">原图始终保留，每次保存生成新版本。</span>
        <button type="button" class="pb-btn" @click="selectedId = null">关闭</button>
        <button
          type="button"
          class="pb-btn hover:text-danger"
          :disabled="batchBusy"
          title="把素材目录、原图与所有版本移到项目回收站"
          @click="deleteSelected()"
        >
          <Loader2 v-if="batchBusy" :size="14" class="animate-spin" />
          <Trash2 v-else :size="14" />
          删除
        </button>
        <button
          type="button"
          class="pb-btn px-2"
          title="打开 exports 目录"
          @click="openExports()"
        >
          <FolderOpen :size="14" />
        </button>
        <button
          type="button"
          class="pb-btn"
          :disabled="exportBusy || !selected?.currentVersion"
          :title="selected?.currentVersion ? '把当前版本复制到 exports/' : '还没有可导出的版本'"
          @click="exportSelected()"
        >
          <Loader2 v-if="exportBusy" :size="14" class="animate-spin" />
          <Download v-else :size="14" />
          导出
        </button>
        <button
          v-if="selected?.source.kind === 'generated' || selected?.source.kind === 'procedural'"
          type="button"
          class="pb-btn"
          :disabled="!selected?.currentVersion || Boolean(selected?.animation)"
          :title="
            selected?.animation
              ? '动画素材暂不支持局部重绘'
              : selected?.currentVersion
                ? '涂出区域后重新生成，只替换标注的地方'
                : '先保存一个版本再局部重绘'
          "
          @click="openInpaint()"
        >
          <Wand2 :size="14" />
          局部重绘
        </button>
        <button
          type="button"
          class="pb-btn"
          :disabled="
            Boolean(selected?.animation) ||
            (selected?.source.kind === 'blank' && !selected?.currentVersion)
          "
          :title="
            selected?.animation
              ? '动画素材暂不支持像素化'
              : selected?.source.kind === 'blank' && !selected?.currentVersion
                ? '先在像素编辑器里画几笔并保存一个版本'
                : '像素完美后处理：量化、抖动、去杂点与描边'
          "
          @click="openPostProcess()"
        >
          <Sparkles :size="14" />
          像素化
        </button>
        <button type="button" class="pb-btn pb-btn-primary" @click="openEditor()">
          <Pencil :size="14" />
          像素编辑
        </button>
      </template>
    </AppModal>

    <InpaintDialog v-model:open="inpaintOpen" :asset="inpaintTarget" />

    <AppModal
      :open="newAssetOpen"
      title="新建素材"
      description="创建一张空白画布并直接打开像素编辑器；保存版本时写入素材库。"
      @update:open="newAssetOpen = $event"
    >
      <div class="space-y-3">
        <label class="flex flex-col gap-1">
          <span class="pb-label">素材名称</span>
          <input
            v-model="newAsset.name"
            class="pb-input h-8 py-1 text-xs"
            maxlength="80"
            placeholder="例如：主角 / 传送门"
            aria-label="新建素材名称"
          />
        </label>
        <div class="grid grid-cols-2 gap-2">
          <label class="flex flex-col gap-1">
            <span class="pb-label">类型</span>
            <select v-model="newAsset.type" class="pb-input h-8 py-1 text-xs" aria-label="新建素材类型">
              <option v-for="type in ASSET_TYPES" :key="type" :value="type">
                {{ t(`assetType.${type}`) }}
              </option>
            </select>
          </label>
          <div class="flex flex-col gap-1">
            <span class="pb-label">快速尺寸</span>
            <div class="flex flex-wrap gap-1">
              <button
                v-for="size in [16, 24, 32, 48, 64]"
                :key="size"
                type="button"
                class="pb-chip px-1.5 py-0.5 font-mono text-[10px] hover:border-accent hover:text-accent"
                @click="applyNewAssetSize(size)"
              >
                {{ size }}
              </button>
            </div>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-2">
          <label class="flex flex-col gap-1">
            <span class="pb-label">宽（px）</span>
            <input
              v-model.number="newAsset.width"
              type="number"
              min="1"
              max="2048"
              class="pb-input h-8 py-1 text-xs"
              aria-label="新建素材宽度"
            />
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">高（px）</span>
            <input
              v-model.number="newAsset.height"
              type="number"
              min="1"
              max="2048"
              class="pb-input h-8 py-1 text-xs"
              aria-label="新建素材高度"
            />
          </label>
        </div>
        <p v-if="newAssetError" class="rounded-lg border border-red/25 bg-red/10 px-3 py-2 text-xs text-red">
          {{ newAssetError }}
        </p>
      </div>
      <template #footer>
        <button type="button" class="pb-btn" @click="newAssetOpen = false">取消</button>
        <button
          type="button"
          class="pb-btn pb-btn-primary"
          :disabled="store.busy"
          @click="createNewAsset()"
        >
          <Loader2 v-if="store.busy" :size="14" class="animate-spin" />
          <Plus v-else :size="14" />
          创建并开始绘制
        </button>
      </template>
    </AppModal>

    <AppModal
      :open="postOpen"
      wide
      title="像素完美处理"
      description="按目标尺寸重采样，限制到调色板颜色，可抖动、去杂点，最后加 1px 描边。"
      @update:open="onPostModal"
    >
      <div v-if="selected" class="grid gap-5 sm:grid-cols-[300px_1fr]">
        <div class="space-y-3">
          <div class="grid grid-cols-2 gap-2">
            <label class="flex flex-col gap-1">
              <span class="pb-label">目标宽</span>
              <input v-model.number="postOptions.width" type="number" min="1" max="2048" class="pb-input h-8 py-1 text-xs" />
            </label>
            <label class="flex flex-col gap-1">
              <span class="pb-label">目标高</span>
              <input v-model.number="postOptions.height" type="number" min="1" max="2048" class="pb-input h-8 py-1 text-xs" />
            </label>
          </div>

          <div class="flex flex-wrap items-center gap-1">
            <span class="pb-label mr-1">长边</span>
            <button
              v-for="size in SIZE_PRESETS"
              :key="size"
              type="button"
              class="pb-chip px-2 py-0.5 font-mono text-[10px] hover:border-accent hover:text-accent"
              :title="`把长边缩到 ${size}px 并重新预览`"
              @click="applySizePreset(size)"
            >
              {{ size }}
            </button>
          </div>

          <div class="grid grid-cols-2 gap-2">
            <label class="flex flex-col gap-1">
              <span class="pb-label">调色板</span>
              <select v-model="paletteMode" class="pb-input h-8 py-1 text-xs">
                <option value="palette">项目调色板</option>
                <option value="auto">自动取色</option>
              </select>
            </label>
            <label class="flex flex-col gap-1">
              <span class="pb-label">{{ paletteMode === 'auto' ? '颜色数' : '上限' }}</span>
              <input
                v-model.number="postOptions.maxColors"
                type="number"
                min="2"
                max="64"
                class="pb-input h-8 py-1 text-xs"
                :disabled="paletteMode === 'palette'"
                :title="
                  paletteMode === 'palette'
                    ? '使用项目调色板时，颜色由调色板本身决定'
                    : '自动取色时最多保留的颜色数'
                "
              />
            </label>
          </div>

          <div class="flex flex-wrap gap-1">
            <span
              v-for="color in paletteMode === 'palette' ? postOptions.colors : postPreview?.colors ?? []"
              :key="color"
              :title="color"
              class="size-4 rounded border border-black/30"
              :style="{ background: color }"
            />
          </div>

          <div class="grid grid-cols-2 gap-2">
            <label class="flex flex-col gap-1">
              <span class="pb-label">抖动</span>
              <select v-model="postOptions.dither" class="pb-input h-8 py-1 text-xs">
                <option v-for="mode in DITHER_MODES" :key="mode" :value="mode">
                  {{ DITHER_LABELS[mode] }}
                </option>
              </select>
            </label>
            <label class="flex flex-col gap-1">
              <span class="pb-label">抖动强度</span>
              <input
                v-model.number="postOptions.ditherStrength"
                type="range"
                min="0.1"
                max="1"
                step="0.05"
                class="accent-[#f2b84b]"
                :disabled="postOptions.dither === 'none'"
              />
            </label>
          </div>

          <div class="grid grid-cols-2 gap-2">
            <label class="flex flex-col gap-1">
              <span class="pb-label">透明阈值</span>
              <input v-model.number="postOptions.alphaThreshold" type="number" min="1" max="255" class="pb-input h-8 py-1 text-xs" />
            </label>
            <label class="flex flex-col gap-1">
              <span class="pb-label">描边颜色</span>
              <input v-model="postOptions.outlineColor" type="color" class="h-8 w-full rounded border border-line bg-canvas" />
            </label>
          </div>

          <div class="space-y-2 rounded-lg border border-line-soft bg-canvas/60 p-3">
            <label class="flex items-center gap-2 text-xs text-muted">
              <input v-model="postOptions.trim" type="checkbox" class="size-4 accent-[#f2b84b]" />
              裁切透明边
            </label>
            <label class="flex items-center gap-2 text-xs text-muted">
              <input v-model="postOptions.cleanup" type="checkbox" class="size-4 accent-[#f2b84b]" />
              去杂点 / 去抗锯齿孤点
            </label>
            <label class="flex items-center gap-2 text-xs text-muted">
              <input v-model="postOptions.outline" type="checkbox" class="size-4 accent-[#f2b84b]" />
              1px 深色描边
            </label>
            <label class="flex items-center gap-2 text-xs text-muted">
              <input v-model="postOptions.checkSeam" type="checkbox" class="size-4 accent-[#f2b84b]" />
              瓦片接缝检查
            </label>
          </div>

          <button type="button" class="pb-btn w-full" :disabled="postBusy" @click="runPreview()">
            <Loader2 v-if="postBusy" :size="14" class="animate-spin" />
            <RefreshCw v-else :size="14" />
            重新预览
          </button>
        </div>

        <div class="min-w-0 space-y-3">
          <div class="grid grid-cols-2 gap-3">
            <div>
              <div class="pb-label">原图</div>
              <div
                class="transparency relative mt-1 h-[240px] overflow-hidden rounded-lg border border-line"
                :style="{ cursor: postCursor }"
                @wheel.prevent="onPostWheel"
                @pointerdown="onPostPointerDown"
                @pointermove="onPostPointerMove"
                @pointerup="onPostPointerUp"
                @pointercancel="onPostPointerUp"
                @pointerleave="onPostPointerUp"
              >
                <div class="flex h-full w-full items-center justify-center">
                  <img
                    v-if="postBeforeURL"
                    :src="postBeforeURL"
                    alt="原图"
                    class="pixelated block max-h-full max-w-full shrink-0"
                    :style="postPreview
                      ? { width: `${postPreview.width}px`, height: `${postPreview.height}px`, ...postViewStyle }
                      : postViewStyle"
                  />
                  <span v-else class="text-[11px] text-faint">加载中…</span>
                </div>
              </div>
            </div>
            <div>
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div class="pb-label">处理结果</div>
                <div v-if="postPreview" class="flex flex-wrap items-center gap-1">
                  <button type="button" class="pb-icon-btn size-6 text-[12px]" title="缩小" @click="stepPostZoom(-1)">−</button>
                  <span class="w-9 text-center font-mono text-[10px] text-ink">{{ postZoom }}×</span>
                  <button type="button" class="pb-icon-btn size-6 text-[12px]" title="放大" @click="stepPostZoom(1)">+</button>
                  <button type="button" class="pb-btn h-6 px-1.5 py-0 text-[10px]" title="复位缩放与位置" @click="resetPostView()">
                    还原
                  </button>
                  <span class="mx-0.5 h-4 w-px bg-line"></span>
                  <button
                    v-for="zoomLevel in POST_ZOOMS"
                    :key="zoomLevel"
                    type="button"
                    class="pb-chip px-1.5 py-0 text-[10px]"
                    :class="postZoom === zoomLevel ? 'border-accent text-accent' : ''"
                    :disabled="
                      postPreview.width * zoomLevel > 2048 || postPreview.height * zoomLevel > 2048
                    "
                    :title="`按 ${zoomLevel}× 整数放大查看`"
                    @click="postZoom = zoomLevel"
                  >
                    {{ zoomLevel }}×
                  </button>
                </div>
              </div>
              <div
                class="transparency relative mt-1 h-[240px] overflow-hidden rounded-lg border border-line"
                :style="{ cursor: postCursor }"
                @wheel.prevent="onPostWheel"
                @pointerdown="onPostPointerDown"
                @pointermove="onPostPointerMove"
                @pointerup="onPostPointerUp"
                @pointercancel="onPostPointerUp"
                @pointerleave="onPostPointerUp"
              >
                <div class="flex h-full w-full items-center justify-center">
                  <img
                    v-if="postPreview && !postBusy"
                    :src="postPreview.image"
                    alt="处理结果"
                    class="pixelated block max-h-full max-w-full shrink-0"
                    :style="{
                      width: `${postPreview.width}px`,
                      height: `${postPreview.height}px`,
                      imageRendering: 'pixelated',
                      ...postViewStyle,
                    }"
                  />
                  <Loader2 v-else-if="postBusy" :size="18" class="animate-spin text-faint" />
                  <span v-else class="text-[11px] text-faint">点击「重新预览」</span>
                </div>
              </div>
            </div>
          </div>

          <p class="text-[10px] text-faint">
            两侧同步缩放与平移：空格拖动，滚轮缩放，点「还原」复位；整数倍率下像素最清晰。
          </p>

          <div v-if="postPreview" class="flex flex-wrap items-center gap-2 text-[11px] text-faint">
            <span class="pb-chip font-mono">{{ postPreview.width }} × {{ postPreview.height }}</span>
            <span class="pb-chip font-mono">{{ postPreview.colors.length }} 色</span>
            <span v-if="postOptions.checkSeam" class="pb-chip font-mono">
              接缝 左右 {{ Math.round(postPreview.seam.leftRight * 100) }}% · 上下
              {{ Math.round(postPreview.seam.topBottom * 100) }}%
            </span>
            <span v-if="postPreview.simulated" class="text-accent">浏览器模拟预览</span>
          </div>

          <div v-if="postPreview && postPreview.colors.length > 0" class="flex flex-wrap gap-1">
            <span
              v-for="color in postPreview.colors"
              :key="color"
              :title="color"
              class="size-5 rounded border border-black/30"
              :style="{ background: color }"
            />
          </div>

          <p class="text-[10px] leading-relaxed text-faint">
            保存后成为素材的新版本，原图仍保留在 source/ 中。
          </p>
        </div>
      </div>

      <template #footer>
        <span v-if="postStale" class="mr-auto text-[11px] text-amber">
          参数已修改，先「重新预览」再保存。
        </span>
        <span v-else-if="postPreview?.simulated" class="mr-auto text-[11px] text-faint">
          浏览器模式无法栅格化，保存的是模拟版本。
        </span>
        <button type="button" class="pb-btn" @click="postOpen = false">取消</button>
        <button
          type="button"
          class="pb-btn pb-btn-primary"
          :disabled="postSaving || postBusy || !postPreview || postStale"
          @click="savePostProcessed()"
        >
          <Loader2 v-if="postSaving" :size="14" class="animate-spin" />
          <Sparkles v-else :size="14" />
          保存为新版本
        </button>
      </template>
    </AppModal>

    <AppModal
      :open="compareOpen"
      wide
      title="版本对比"
      :description="selected ? `${selected.name} · 只读取不可变版本文件` : ''"
      @update:open="onCompareModal"
    >
      <div v-if="selected" class="space-y-3">
        <div class="grid grid-cols-2 gap-3">
          <label class="flex flex-col gap-1">
            <span class="pb-label">基准版本</span>
            <select v-model="compareBase" class="pb-input h-8 py-1 text-xs" @change="loadCompareImages()">
              <option v-for="version in selected.versions" :key="version.id" :value="version.id">
                v{{ version.id }}
              </option>
            </select>
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">对比版本</span>
            <select v-model="compareTarget" class="pb-input h-8 py-1 text-xs" @change="loadCompareImages()">
              <option v-for="version in selected.versions" :key="version.id" :value="version.id">
                v{{ version.id }}
              </option>
            </select>
          </label>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <div class="flex items-center justify-between text-[10px] text-faint">
              <span class="font-mono text-ink">v{{ compareBase }}</span>
              <span v-if="compareBase === selected.currentVersion">当前</span>
            </div>
            <div class="transparency mt-1 flex h-[260px] items-center justify-center rounded-lg border border-line p-2">
              <Loader2 v-if="compareBusy" :size="18" class="animate-spin text-faint" />
              <img
                v-else-if="compareUrls.base"
                :src="compareUrls.base"
                alt="基准版本"
                class="pixelated max-h-full w-auto max-w-full"
              />
              <span v-else class="text-[11px] text-faint">没有可显示的图片</span>
            </div>
          </div>
          <div>
            <div class="flex items-center justify-between text-[10px] text-faint">
              <span class="font-mono text-ink">v{{ compareTarget }}</span>
              <span v-if="compareTarget === selected.currentVersion">当前</span>
            </div>
            <div class="transparency mt-1 flex h-[260px] items-center justify-center rounded-lg border border-line p-2">
              <Loader2 v-if="compareBusy" :size="18" class="animate-spin text-faint" />
              <img
                v-else-if="compareUrls.target"
                :src="compareUrls.target"
                alt="对比版本"
                class="pixelated max-h-full w-auto max-w-full"
              />
              <span v-else class="text-[11px] text-faint">没有可显示的图片</span>
            </div>
          </div>
        </div>

        <p v-if="compareBase === compareTarget" class="text-[10px] text-faint">
          两侧选择了同一个版本；切换其中一个下拉框即可比较差异。
        </p>
        <p v-else class="text-[10px] text-faint">
          两版并排按最近邻显示。生成与后处理的参数记录在各自的
          <span class="font-mono">versions/&lt;版本&gt;/meta.json</span>。
          <template v-if="store.info.mode === 'browser'">浏览器预览用缩略图代替版本文件。</template>
        </p>
      </div>

      <template #footer>
        <span class="mr-auto text-[11px] text-faint">对比不会修改任何版本文件。</span>
        <button type="button" class="pb-btn" @click="compareOpen = false">关闭</button>
      </template>
    </AppModal>
  </div>
</template>