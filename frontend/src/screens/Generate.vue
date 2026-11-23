<script setup lang="ts">
import {
  Ban,
  Bot,
  Building2,
  Check,
  ChevronDown,
  Dices,
  Film,
  Grid3x3,
  Images,
  Loader2,
  Map as MapIcon,
  Sparkles,
  Trash2,
  Wand2,
} from 'lucide-vue-next'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import type {
  AIJob,
  AIJobImage,
  AIJobState,
  AIProviderId,
  AISheetAdoptRequest,
  Asset,
  AssetType,
  BuildingOptions,
  GeneratedBuilding,
  GenerateIntent,
  MapGenerateIntent,
} from '../domain/types'
import {
  AI_PROVIDERS,
  ASSET_TYPES,
  DEFAULT_NEGATIVE_PROMPT,
  TILE_SIZES,
} from '../domain/types'
import { useAppStore } from '../stores/app'
import { confirmDialog } from '../lib/dialogs'
import { openLightbox } from '../lib/lightbox'
import AppModal from '../components/AppModal.vue'
import ProgressBar from '../components/ProgressBar.vue'

const store = useAppStore()

const options = reactive<BuildingOptions>({
  seed: 20261121,
  width: 4,
  depth: 3,
  tileSize: 32,
  floors: 2,
  wallColor: '#c8a87c',
  roofColor: '#b04e3e',
  trimColor: '#78543a',
  doorColor: '#5c3a24',
  windows: 0,
  sign: true,
  weathered: 0.25,
  shadow: true,
  outline: true,
  variants: 4,
})

const drafts = ref<GeneratedBuilding[]>([])
const notice = ref<string | null>(null)
const adoptTarget = ref<GeneratedBuilding | null>(null)
const adoptName = ref('')
let noticeTimer: ReturnType<typeof setTimeout> | null = null

const project = computed(() => store.session?.project ?? null)

/* ------------------------------------------------------------ AI drafts */

type Mode = 'procedural' | 'ai'
// AI generation is the primary workflow; procedural buildings are the offline fallback.
const mode = ref<Mode>('ai')

const ASSET_TYPE_LABELS: Record<AssetType, string> = {
  sprite: '角色/精灵',
  building: '建筑',
  tile: '瓦片',
  prop: '道具',
  ui: 'UI 图标',
  fx: '特效',
  other: '其他',
}

const JOB_STATE_LABEL: Record<AIJobState, string> = {
  queued: '排队中',
  running: '生成中',
  done: '已完成',
  failed: '失败',
  canceled: '已取消',
}

const JOB_STATE_TONE: Record<AIJobState, string> = {
  queued: 'text-faint',
  running: 'text-accent',
  done: 'text-green',
  failed: 'text-[#e07a7a]',
  canceled: 'text-faint',
}

const AI_SIZES = [
  { w: 512, h: 512, label: '512²' },
  { w: 768, h: 512, label: '768×512' },
  { w: 512, h: 768, label: '512×768' },
  { w: 768, h: 768, label: '768²' },
  { w: 1024, h: 1024, label: '1024²' },
] as const

const aiProvider = ref<AIProviderId>('comfyui')
const aiProviderTouched = ref(false)
const aiPrompt = ref('')
const aiNegative = ref(
  store.session?.project.settings.styleGuide.negativePrompt?.trim() ||
    DEFAULT_NEGATIVE_PROMPT,
)
const aiWidth = ref(512)
const aiHeight = ref(512)
const aiCount = ref(2)
const aiSeed = ref(20261121)
const aiAssetType = ref<AssetType>('sprite')
const aiName = ref('')
const aiSubmitting = ref(false)
const aiAdoptTarget = ref<{ job: AIJob; image: AIJobImage } | null>(null)
const aiAdoptName = ref('')

watch(
  () => store.aiSettings,
  (settings) => {
    if (settings && !aiProviderTouched.value) aiProvider.value = settings.defaultProvider
  },
  { immediate: true },
)

const activeJobs = computed(() =>
  store.aiJobs.filter((job) => job.state === 'queued' || job.state === 'running'),
)
const doneJobs = computed(() => store.aiJobs.filter((job) => job.state === 'done'))

/* ----------------------------------------------------- AI asset presets */

interface AIPreset {
  key: string
  label: string
  type: AssetType
  width: number
  height: number
  sheet?: SheetPreset
  prompt: string
}

/**
 * One-click starting points for the AI-first workflow. They fill the whole
 * form, including a layout that the sheet slicer can consume as-is.
 */
const AI_PRESETS: AIPreset[] = [
  {
    key: 'character4',
    label: '角色 · 4 向',
    type: 'sprite',
    width: 768,
    height: 512,
    sheet: '4dir',
    prompt:
      '像素风游戏角色，正面 / 左 / 右 / 背面 4 个方向各一帧行走姿态，4 列 × 1 行等距排列，每格主体居中、脚底对齐，透明背景，无文字，无边框',
  },
  {
    key: 'character8',
    label: '角色 · 8 向',
    type: 'sprite',
    width: 1024,
    height: 1024,
    sheet: '8dir',
    prompt:
      '像素风游戏角色，8 个方向各一帧行走姿态，4 列 × 2 行等距排列，每格主体居中、脚底对齐，透明背景，无文字，无边框',
  },
  {
    key: 'tileset',
    label: '地形图集',
    type: 'tile',
    width: 768,
    height: 768,
    prompt:
      '像素风 2.5D 俯视地形图集，草地、泥路、水面、石地、树篱等，按等距网格排列的 16 格自动拼接瓦片（每类 16 变体），纯色背景，格线对齐，无文字',
  },
  {
    key: 'building',
    label: '建筑',
    type: 'building',
    width: 768,
    height: 768,
    prompt:
      '像素风 2.5D 俯视建筑，木结构杂货铺，暖色招牌，屋顶与墙体分层清晰，透明背景，主体居中，底边对齐',
  },
  {
    key: 'prop',
    label: '道具套装',
    type: 'prop',
    width: 512,
    height: 512,
    prompt:
      '像素风 2.5D 俯视游戏道具组合（木箱、木桶、路灯、摊位），单个素材主体居中，透明背景，底边对齐，无文字',
  },
  {
    key: 'ui',
    label: 'UI 图标',
    type: 'ui',
    width: 512,
    height: 512,
    prompt: '像素风游戏 UI 图标，16 色以内，1 像素深色描边，透明背景，图标居中，无文字',
  },
  {
    key: 'fx',
    label: '特效序列',
    type: 'fx',
    width: 768,
    height: 512,
    sheet: 'fx',
    prompt: '像素风游戏特效序列，横向 4 帧等距排列，帧间过渡连贯，纯色背景，无文字',
  },
]

function applyAIPreset(preset: AIPreset) {
  aiAssetType.value = preset.type
  aiWidth.value = preset.width
  aiHeight.value = preset.height
  aiPrompt.value = preset.prompt
  if (preset.sheet) applySheetPreset(preset.sheet)
}

/* -------------------------------------------------- direction / FX sheets */

type SheetPreset = '4dir' | '8dir' | 'fx'

const SHEET_PRESETS: Record<SheetPreset, { label: string; columns: number; rows: number; assetType: AssetType; fps: number; tags: string }> = {
  '4dir': { label: '4 方向', columns: 4, rows: 1, assetType: 'sprite', fps: 8, tags: '方向图,行走' },
  '8dir': { label: '8 方向', columns: 4, rows: 2, assetType: 'sprite', fps: 8, tags: '方向图,行走' },
  fx: { label: '特效序列', columns: 4, rows: 1, assetType: 'fx', fps: 12, tags: '特效,序列帧' },
}

const SHEET_PRESET_OPTIONS = (Object.keys(SHEET_PRESETS) as SheetPreset[]).map((key) => ({
  key,
  ...SHEET_PRESETS[key],
}))

const aiSheetTarget = ref<{ job: AIJob; image: AIJobImage } | null>(null)
const aiSheetName = ref('')
const sheetPreviewBox = ref<HTMLElement | null>(null)
const sheetPreviewSize = ref({ w: 0, h: 0 })

/** Fits the sheet inside the preview box and remembers its rendered size, so
 *  the cut grid can sit exactly on the image. */
function syncSheetPreview(event: Event) {
  const img = event.target as HTMLImageElement
  const box = sheetPreviewBox.value
  if (!img.naturalWidth || !img.naturalHeight || !box) return
  const maxW = Math.max(1, box.clientWidth - 24)
  const maxH = Math.max(1, box.clientHeight - 24)
  const scale = Math.min(maxW / img.naturalWidth, maxH / img.naturalHeight, 1)
  sheetPreviewSize.value = {
    w: Math.max(1, Math.round(img.naturalWidth * scale)),
    h: Math.max(1, Math.round(img.naturalHeight * scale)),
  }
}

const sheetGridStyle = computed(() => {
  const columns = Math.max(1, Math.trunc(asNumber(aiSheet.columns, 1)))
  const rows = Math.max(1, Math.trunc(asNumber(aiSheet.rows, 1)))
  return {
    backgroundImage:
      'linear-gradient(to right, rgba(242,184,75,0.9) 2px, transparent 2px), linear-gradient(to bottom, rgba(242,184,75,0.9) 2px, transparent 2px)',
    backgroundSize: `${100 / columns}% ${100 / rows}%`,
    boxShadow: 'inset 0 0 0 2px rgba(242,184,75,0.75)',
  }
})

watch(aiSheetTarget, () => {
  sheetPreviewSize.value = { w: 0, h: 0 }
})

const aiSheet = reactive({
  preset: '4dir' as SheetPreset,
  columns: 4,
  rows: 1,
  frameSize: 32,
  fps: 8,
  assetType: 'sprite' as AssetType,
  tags: '方向图,行走',
  outline: true,
  keyBackground: true,
})

/** Applies a preset that matches the layout the image was generated in. */
function applySheetPreset(preset: SheetPreset) {
  const spec = SHEET_PRESETS[preset]
  aiSheet.preset = preset
  aiSheet.columns = spec.columns
  aiSheet.rows = spec.rows
  aiSheet.assetType = spec.assetType
  aiSheet.fps = spec.fps
  aiSheet.tags = spec.tags
}

/**
 * Appends the grid layout the slicer expects, so a plain character prompt
 * becomes a 4×1 or 4×2 sheet with evenly spaced cells.
 */
function appendSheetLayout(preset: SheetPreset) {
  const spec = SHEET_PRESETS[preset]
  applySheetPreset(preset)
  const layout =
    preset === 'fx'
      ? '一张横向排列的 4 帧像素特效序列图，每帧等宽等高、均匀排成 1 行'
      : `同一角色按 ${spec.columns} 列 × ${spec.rows} 行等距排列的${spec.label}行走姿态，每格一个方向，脚底对齐`
  aiPrompt.value = [aiPrompt.value.trim(), layout, '透明或纯色背景，无文字，无边框，格线对齐']
    .filter(Boolean)
    .join('，')
  aiWidth.value = preset === '8dir' ? 1024 : 768
  aiHeight.value = preset === '8dir' ? 1024 : 512
}

function openAISheet(job: AIJob, imageIndex: number) {
  const image = job.images.find((item) => item.index === imageIndex)
  if (!image) return
  aiSheetTarget.value = { job, image }
  aiSheetName.value = `${job.name.trim() || 'AI 素材'} 精灵表`
  const tile = project.value?.settings.tileSize
  aiSheet.frameSize = tile && tile >= 8 && tile <= 128 ? tile : 32
  applySheetPreset('4dir')
}

function onAISheetModal(open: boolean) {
  if (!open) aiSheetTarget.value = null
}

async function confirmAISheet() {
  const target = aiSheetTarget.value
  if (!target) return
  const request: AISheetAdoptRequest = {
    jobId: target.job.id,
    index: target.image.index,
    name: aiSheetName.value,
    assetType: aiSheet.assetType,
    columns: Math.trunc(asNumber(aiSheet.columns, 1)),
    rows: Math.trunc(asNumber(aiSheet.rows, 1)),
    frameSize: Math.trunc(asNumber(aiSheet.frameSize, 32)),
    fps: Math.trunc(asNumber(aiSheet.fps, 8)),
    tags: aiSheet.tags
      .split(/[,，]/)
      .map((tag) => tag.trim())
      .filter(Boolean),
    outline: aiSheet.outline,
    keyBackground: aiSheet.keyBackground,
    paletteId: project.value?.settings.defaultPaletteId,
  }
  const saved = await store.adoptAIJobSheet(request)
  if (!saved) return
  aiSheetTarget.value = null
  showNotice(
    `已把「${saved.name}」保存为 ${saved.animation?.frameCount ?? request.columns * request.rows} 帧精灵表（${request.columns}×${request.rows}，${request.frameSize}px/帧），可在素材库查看或直接放进地图。`,
  )
}

/* ------------------------------------------------------- quest handoff */

const questIntent = ref<GenerateIntent | null>(null)

/** Applies a quest brief: AI mode, asset type, name, dimensions and prompt. */
function adoptQuestIntent(intent: GenerateIntent) {
  questIntent.value = intent
  mode.value = 'ai'
  aiAssetType.value = intent.assetType || 'other'
  // Quests carry a title; lore entries and table rows carry a source label.
  const label = intent.questTitle.trim() || intent.sourceLabel?.trim() || ''
  if (label) aiName.value = label
  if (TILE_SIZES.includes(intent.tileSize as (typeof TILE_SIZES)[number])) {
    options.tileSize = intent.tileSize
  }
  aiPrompt.value = [intent.prompt, intent.style].filter(Boolean).join('，')
  if (!aiPrompt.value) fillFromStyleGuide()
}

function goQuests() {
  store.setRoute('quests')
}

function dismissQuestIntent() {
  questIntent.value = null
}

/* ---------------------------------------------------------- map handoff */

const mapIntent = ref<MapGenerateIntent | null>(null)

/** Applies a map context: AI mode, tile/FX type, size and a scene prompt. */
function adoptMapIntent(intent: MapGenerateIntent) {
  mapIntent.value = intent
  mode.value = 'ai'
  aiAssetType.value = intent.assetType
  aiName.value = `${intent.mapName} 扩展素材`
  aiPrompt.value = intent.prompt
  const tile = intent.tileSize
  if (tile && tile >= 8 && tile <= 128) aiSheet.frameSize = tile
}

function goMaps() {
  store.setRoute('maps')
}

function dismissMapIntent() {
  mapIntent.value = null
}

function setAIProvider() {
  aiProviderTouched.value = true
}

function setAISize(w: number, h: number) {
  aiWidth.value = w
  aiHeight.value = h
}

function fillFromStyleGuide() {
  const guide = project.value?.settings.styleGuide
  const typeLabel = ASSET_TYPE_LABELS[aiAssetType.value]
  const parts = [
    aiName.value.trim() || typeLabel,
    '像素风，单件素材，主体居中',
    guide?.perspective === 'top-down' ? '正俯视' : '3/4 俯视（正交）',
    `${guide?.maxColors ?? 16} 色以内`,
    guide?.outline === 'none' ? '无描边' : '1 像素深色描边',
    '透明背景，底部对齐',
  ]
  aiPrompt.value = parts.join('，')
}

/**
 * Cloud providers bill per request. Show the exact payload first; ComfyUI runs
 * locally and skips this step.
 */
async function confirmCloudSend(
  width: number,
  height: number,
  count: number,
  seed: number,
): Promise<boolean> {
  const detail = [
    '供应商：OpenAI 兼容图像接口',
    `提示词：${aiPrompt.value.trim()}`,
    `负面提示词：${aiNegative.value.trim() || '（空）'}`,
    `尺寸：${width}×${height}，数量 ${count}，种子 ${seed}`,
  ].join('\n')
  return await confirmDialog({
    title: '发送到云 API',
    message: '本次请求可能产生费用，只发送下列内容。不会发送项目文件、素材原件或本地路径。',
    detail,
    confirmText: '发送请求',
  })
}

async function submitAI() {
  if (!aiPrompt.value.trim()) {
    showNotice('请先填写提示词')
    return
  }
  const width = Math.trunc(asNumber(aiWidth.value, 512))
  const height = Math.trunc(asNumber(aiHeight.value, 512))
  const count = Math.min(4, Math.max(1, Math.trunc(asNumber(aiCount.value, 1))))
  const seed = Math.trunc(asNumber(aiSeed.value, 0))
  if (aiProvider.value !== 'comfyui' && !(await confirmCloudSend(width, height, count, seed))) return
  aiSubmitting.value = true
  try {
    const job = await store.submitAIJob({
      provider: aiProvider.value,
      prompt: aiPrompt.value,
      negativePrompt: aiNegative.value,
      width,
      height,
      count,
      seed,
      assetType: aiAssetType.value,
      name: aiName.value.trim(),
    })
    if (job) showNotice('已提交到生成队列；完成后可在下方采用为草稿素材。')
  } finally {
    aiSubmitting.value = false
  }
}

async function cancelJob(job: AIJob) {
  await store.cancelAIJob(job.id)
}

const TERMINAL_STATES: AIJobState[] = ['done', 'failed', 'canceled']
const finishedJobs = computed(() => store.aiJobs.filter((job) => TERMINAL_STATES.includes(job.state)))

type JobTab = 'all' | 'active' | 'done' | 'failed'
const JOB_TABS: Array<{ id: JobTab; label: string }> = [
  { id: 'all', label: '全部' },
  { id: 'active', label: '进行中' },
  { id: 'done', label: '已完成' },
  { id: 'failed', label: '失败' },
]
const jobTab = ref<JobTab>('all')
const failedJobs = computed(() => finishedJobs.value.filter((job) => job.state !== 'done'))
const visibleJobs = computed(() => {
  switch (jobTab.value) {
    case 'active':
      return activeJobs.value
    case 'done':
      return doneJobs.value
    case 'failed':
      return failedJobs.value
    default:
      return store.aiJobs
  }
})
const expandedJobId = ref<string | null>(null)

let knownDoneIds = new Set<string>()
watch(
  doneJobs,
  (jobs) => {
    const ids = new Set(jobs.map((job) => job.id))
    const fresh = jobs.find((job) => !knownDoneIds.has(job.id))
    const newest = [...jobs].sort(
      (a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
    )[0]
    const firstLook = knownDoneIds.size === 0
    knownDoneIds = ids
    if (firstLook) {
      expandedJobId.value = newest?.id ?? null
    } else if (fresh) {
      expandedJobId.value = fresh.id
    } else if (expandedJobId.value && !ids.has(expandedJobId.value)) {
      expandedJobId.value = newest?.id ?? null
    } else if (!expandedJobId.value && newest) {
      expandedJobId.value = newest.id
    }
  },
  { immediate: true },
)

function toggleJob(job: AIJob) {
  if (job.state !== 'done') return
  expandedJobId.value = expandedJobId.value === job.id ? null : job.id
}

/** Removes one finished job and its generated files; adopted assets stay. */
async function deleteJob(job: AIJob) {
  if (!TERMINAL_STATES.includes(job.state)) return
  const label = job.name.trim() || job.prompt.slice(0, 24)
  const ok = await confirmDialog({
    title: '删除生成任务',
    message: `「${label}」及其生成结果会从磁盘删除。已采用的素材不受影响。`,
    confirmText: '删除',
    tone: 'danger',
  })
  if (!ok) return
  await store.deleteAIJob(job.id)
}

async function clearFinishedJobs() {
  const jobs = finishedJobs.value
  if (jobs.length === 0) return
  const ok = await confirmDialog({
    title: '清空已结束任务',
    message: `${jobs.length} 个任务的生成结果会从磁盘删除。已采用的素材不受影响。`,
    confirmText: '删除',
    tone: 'danger',
  })
  if (!ok) return
  for (const job of jobs) {
    await store.deleteAIJob(job.id)
  }
}

function openAIAdopt(job: AIJob, imageIndex: number) {
  const image = job.images.find((item) => item.index === imageIndex)
  if (!image) return
  aiAdoptTarget.value = { job, image }
  aiAdoptName.value = `${job.name.trim() || 'AI 素材'} ${imageIndex + 1}`
}

function adoptedAsset(image: AIJobImage): Asset | null {
  if (!image.adoptedAssetId) return null
  return store.assets.find((asset) => asset.id === image.adoptedAssetId) ?? null
}

function isAdopted(image: AIJobImage): boolean {
  return adoptedAsset(image) !== null
}

function openAdopted(image: AIJobImage) {
  const asset = adoptedAsset(image)
  if (!asset) return
  store.requestFocus('asset', asset.id)
  store.setRoute('assets')
}

function onAIAdoptModal(open: boolean) {
  if (!open) aiAdoptTarget.value = null
}

async function confirmAIAdopt() {
  const target = aiAdoptTarget.value
  if (!target) return
  const saved = await store.adoptAIJobImage(target.job.id, target.image.index, aiAdoptName.value)
  if (!saved) return
  aiAdoptTarget.value = null
  showNotice(`已把「${saved.name}」保存为草稿素材，可在素材库中打开像素编辑器或运行后处理。`)
}

/** Number inputs yield '' when cleared; the Go binding needs real numbers. */
function asNumber(value: unknown, fallback: number): number {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function snapshotOptions(): BuildingOptions {
  return {
    ...options,
    seed: Math.trunc(asNumber(options.seed, 0)),
    width: asNumber(options.width, 4),
    depth: asNumber(options.depth, 3),
    tileSize: asNumber(options.tileSize, 32),
    floors: asNumber(options.floors, 1),
    windows: asNumber(options.windows, 0),
    weathered: Math.min(1, Math.max(0, asNumber(options.weathered, 0))),
    variants: asNumber(options.variants, 1),
  }
}

function showNotice(text: string) {
  notice.value = text
  if (noticeTimer) clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => {
    if (notice.value === text) notice.value = null
  }, 6000)
}

async function runGenerate() {
  const result = await store.generateBuildings(snapshotOptions())
  if (result) {
    drafts.value = result
    notice.value = null
  }
}

function reroll() {
  options.seed = Math.floor(Math.random() * 1_000_000_000)
  void runGenerate()
}

function openAdopt(draft: GeneratedBuilding) {
  adoptTarget.value = draft
  adoptName.value = `建筑 ${draft.seed}`
}

function onAdoptModal(open: boolean) {
  if (!open) adoptTarget.value = null
}

async function confirmAdopt() {
  const target = adoptTarget.value
  if (!target) return
  const saved = await store.adoptBuilding(snapshotOptions(), target.seed, adoptName.value)
  if (!saved) return
  adoptTarget.value = null
  showNotice(`已把「${saved.name}」保存为草稿素材，可在素材库中打开像素编辑器。`)
}

function goAssets() {
  store.setRoute('assets')
}

/** Jumps to the asset an inpaint job refines. */
function openInpaintTarget(job: AIJob) {
  if (!job.targetAssetId) return
  store.requestFocus('asset', job.targetAssetId)
  store.setRoute('assets')
}

function targetName(id?: string): string {
  if (!id) return '素材'
  return store.assets.find((asset) => asset.id === id)?.name ?? '素材'
}

onMounted(() => {
  const tile = project.value?.settings.tileSize
  if (tile && TILE_SIZES.includes(tile as (typeof TILE_SIZES)[number])) options.tileSize = tile
  const mapHandoff = store.consumeMapGenerateIntent()
  const questHandoff = store.consumeGenerateIntent()
  if (mapHandoff) adoptMapIntent(mapHandoff)
  else if (questHandoff) adoptQuestIntent(questHandoff)
  if (!store.aiSettings) void store.loadAISettings()
  void store.refreshAIJobs()
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex flex-wrap items-center gap-3">
        <h2 class="text-lg font-semibold tracking-tight text-ink">生成台</h2>
        <div
          class="flex items-center rounded-lg border border-line bg-panel-2/60 p-0.5"
          role="group"
          aria-label="生成方式"
        >
          <button
            type="button"
            class="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[11px] transition-colors"
            :class="mode === 'ai' ? 'bg-panel-3 text-ink' : 'text-muted hover:text-ink'"
            :aria-pressed="mode === 'ai'"
            @click="mode = 'ai'"
          >
            <Bot :size="13" />
            AI 生成
            <span v-if="activeJobs.length" class="rounded-full bg-accent px-1.5 font-mono text-[9px] text-canvas">
              {{ activeJobs.length }}
            </span>
          </button>
          <button
            type="button"
            class="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[11px] transition-colors"
            :class="mode === 'procedural' ? 'bg-panel-3 text-ink' : 'text-muted hover:text-ink'"
            :aria-pressed="mode === 'procedural'"
            @click="mode = 'procedural'"
          >
            <Building2 :size="13" />
            参数化建筑
          </button>
        </div>
        <span v-if="mode === 'procedural' && drafts.length" class="font-mono text-[11px] text-faint">
          {{ drafts.length }} 个变体
        </span>
        <span v-else-if="mode === 'ai' && store.aiJobs.length" class="font-mono text-[11px] text-faint">
          {{ activeJobs.length }} 进行中，共 {{ store.aiJobs.length }} 个
        </span>
      </div>
      <div v-if="mode === 'procedural'" class="flex items-center gap-2">
        <button
          type="button"
          class="pb-btn h-9 py-1"
          :disabled="store.busy"
          title="随机种子并立即重新生成"
          @click="reroll()"
        >
          <Dices :size="14" />
          换一批
        </button>
        <button
          type="button"
          class="pb-btn pb-btn-primary h-9 py-1"
          :disabled="store.busy"
          @click="runGenerate()"
        >
          <Loader2 v-if="store.busy" :size="14" class="animate-spin" />
          <Sparkles v-else :size="14" />
          生成
        </button>
      </div>
    </div>

    <div
      v-if="mapIntent"
      class="pb-fade-in flex flex-wrap items-center gap-2 rounded-lg border border-accent/30 bg-accent-soft px-3 py-2 text-[11px]"
    >
      <MapIcon :size="13" class="shrink-0 text-accent" />
      <span class="text-ink">来自地图「{{ mapIntent.mapName }}」</span>
      <span class="pb-chip font-mono">{{ mapIntent.width }}×{{ mapIntent.height }} 格</span>
      <span class="pb-chip font-mono">{{ mapIntent.tileSize }}px</span>
      <span class="pb-chip">{{ ASSET_TYPE_LABELS[mapIntent.assetType] }}</span>
      <span class="flex-1"></span>
      <button type="button" class="text-accent hover:underline" @click="goMaps()">回到地图</button>
      <button type="button" class="text-faint hover:text-ink" @click="dismissMapIntent()">清除</button>
    </div>

    <div
      v-if="questIntent"
      class="pb-fade-in flex flex-wrap items-center gap-2 rounded-lg border border-accent/30 bg-accent-soft px-3 py-2 text-[11px]"
    >
      <Wand2 :size="13" class="shrink-0 text-accent" />
      <span v-if="questIntent.sourceKind === 'entity'" class="text-ink">
        来自设定「{{ questIntent.sourceLabel }}」
      </span>
      <span v-else-if="questIntent.sourceKind === 'table'" class="text-ink">
        来自数值表「{{ questIntent.sourceLabel }}」
      </span>
      <span v-else class="text-ink">来自任务「{{ questIntent.questTitle }}」</span>
      <span class="pb-chip font-mono">
        {{ questIntent.assetType ? ASSET_TYPE_LABELS[questIntent.assetType] : '未指定类型' }}
      </span>
      <span class="pb-chip font-mono">{{ questIntent.tileSize }}px</span>
      <span v-if="questIntent.style" class="min-w-0 max-w-64 truncate text-faint" :title="questIntent.style">
        {{ questIntent.style }}
      </span>
      <span
        v-if="questIntent.aiFallback"
        class="pb-chip font-mono text-amber"
        :title="questIntent.aiFallbackDetail
          ? `AI 起稿失败：${questIntent.aiFallbackDetail}；提示词已改用记录原文`
          : 'AI 起稿不可用，提示词由记录内容直接拼成；在「设置 → AI 生成」配置文本模型后重新生成'"
      >
        本地拼装
      </span>
      <span class="flex-1"></span>
      <button
        v-if="!questIntent.sourceKind"
        type="button"
        class="text-accent hover:underline"
        @click="goQuests()"
      >
        回到任务
      </button>
      <button type="button" class="text-faint hover:text-ink" @click="dismissQuestIntent()">清除</button>
    </div>

    <div
      v-if="notice"
      class="pb-fade-in flex items-center gap-2 rounded-lg border border-line bg-panel-2 px-3 py-2 text-[11px] text-ink"
    >
      <Check :size="13" class="text-green" />
      <span class="flex-1">{{ notice }}</span>
      <button
        type="button"
        class="inline-flex items-center gap-1 text-accent hover:underline"
        @click="goAssets()"
      >
        <Images :size="12" />
        查看素材
      </button>
    </div>

    <div v-if="mode === 'procedural'" class="grid gap-4 lg:grid-cols-[320px_minmax(0,1fr)]">
      <section class="pb-card h-fit space-y-4 p-4">
        <div class="flex items-center justify-between">
          <h3 class="text-sm font-semibold text-ink">参数</h3>
          <span v-if="project" class="font-mono text-[10px] text-faint">
            项目瓦片 {{ project.settings.tileSize }}px
          </span>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <label class="flex flex-col gap-1">
            <span class="pb-label">种子</span>
            <div class="flex items-center gap-1">
              <input v-model.number="options.seed" type="number" min="0" class="pb-input h-8 py-1 text-xs" />
              <button
                type="button"
                class="pb-icon-btn size-8 shrink-0"
                title="随机种子"
                @click="options.seed = Math.floor(Math.random() * 1_000_000_000)"
              >
                <Dices :size="13" />
              </button>
            </div>
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">变体数</span>
            <input
              v-model.number="options.variants"
              type="number"
              min="1"
              max="8"
              class="pb-input h-8 py-1 text-xs"
            />
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">占地宽（格）</span>
            <input
              v-model.number="options.width"
              type="number"
              min="1"
              max="16"
              class="pb-input h-8 py-1 text-xs"
            />
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">占地深（格）</span>
            <input
              v-model.number="options.depth"
              type="number"
              min="1"
              max="16"
              class="pb-input h-8 py-1 text-xs"
            />
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">瓦片尺寸</span>
            <select v-model.number="options.tileSize" class="pb-input h-8 py-1 text-xs">
              <option v-for="size in TILE_SIZES" :key="size" :value="size">{{ size }} px</option>
            </select>
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">层数</span>
            <input
              v-model.number="options.floors"
              type="number"
              min="1"
              max="4"
              class="pb-input h-8 py-1 text-xs"
            />
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">每层窗户</span>
            <input
              v-model.number="options.windows"
              type="number"
              min="0"
              max="12"
              class="pb-input h-8 py-1 text-xs"
              title="0 表示按占地自动决定"
            />
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">做旧 0–1</span>
            <input
              v-model.number="options.weathered"
              type="range"
              min="0"
              max="1"
              step="0.05"
              class="accent-[#f2b84b]"
            />
            <span class="font-mono text-[10px] text-faint">{{ options.weathered.toFixed(2) }}</span>
          </label>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <label class="flex flex-col gap-1">
            <span class="pb-label">墙体</span>
            <div class="flex items-center gap-2">
              <input v-model="options.wallColor" type="color" class="size-8 rounded border border-line bg-canvas" />
              <span class="font-mono text-[10px] text-faint">{{ options.wallColor }}</span>
            </div>
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">屋顶</span>
            <div class="flex items-center gap-2">
              <input v-model="options.roofColor" type="color" class="size-8 rounded border border-line bg-canvas" />
              <span class="font-mono text-[10px] text-faint">{{ options.roofColor }}</span>
            </div>
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">窗框 / 描边色</span>
            <div class="flex items-center gap-2">
              <input v-model="options.trimColor" type="color" class="size-8 rounded border border-line bg-canvas" />
              <span class="font-mono text-[10px] text-faint">{{ options.trimColor }}</span>
            </div>
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">门</span>
            <div class="flex items-center gap-2">
              <input v-model="options.doorColor" type="color" class="size-8 rounded border border-line bg-canvas" />
              <span class="font-mono text-[10px] text-faint">{{ options.doorColor }}</span>
            </div>
          </label>
        </div>

        <div class="space-y-2 rounded-lg border border-line-soft bg-canvas/60 p-3">
          <label class="flex items-center gap-2 text-xs text-muted">
            <input v-model="options.sign" type="checkbox" class="size-4 accent-[#f2b84b]" />
            招牌
          </label>
          <label class="flex items-center gap-2 text-xs text-muted">
            <input v-model="options.shadow" type="checkbox" class="size-4 accent-[#f2b84b]" />
            投影（光来自左上）
          </label>
          <label class="flex items-center gap-2 text-xs text-muted">
            <input v-model="options.outline" type="checkbox" class="size-4 accent-[#f2b84b]" />
            1px 深色描边
          </label>
        </div>

        </section>

      <section class="min-w-0 space-y-3">
        <div
          v-if="!drafts.length"
          class="pb-card flex h-64 flex-col items-center justify-center gap-3 text-center"
        >
          <Sparkles :size="24" class="text-faint" />
          <p class="text-sm text-muted">还没有生成结果</p>
          <button type="button" class="pb-btn pb-btn-primary" :disabled="store.busy" @click="runGenerate()">
            生成一组
          </button>
        </div>

        <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(210px,1fr))] gap-3">
          <article v-for="draft in drafts" :key="draft.seed" class="pb-card overflow-hidden">
            <div class="flex items-center justify-between border-b border-line-soft px-3 py-1.5">
              <span class="font-mono text-[10px] text-faint">种子 {{ draft.seed }}</span>
              <span class="font-mono text-[10px] text-faint">{{ draft.width }}×{{ draft.height }}</span>
            </div>
            <div class="transparency flex h-[230px] items-center justify-center p-3">
              <img
                :src="draft.image"
                :alt="`建筑变体 ${draft.seed}`"
                class="pixelated max-h-full w-auto max-w-full"
                :style="{ imageRendering: 'pixelated' }"
              />
            </div>
            <div class="space-y-1.5 border-t border-line-soft px-3 py-2">
              <div class="flex items-center justify-between font-mono text-[10px] text-faint">
                <span>锚点 {{ draft.pivot.x }},{{ draft.pivot.y }}</span>
                <span>碰撞 {{ draft.collision.w }}×{{ draft.collision.h }}</span>
              </div>
              <button type="button" class="pb-btn h-8 w-full py-1 text-xs" @click="openAdopt(draft)">
                <Check :size="13" />
                采用为草稿素材
              </button>
            </div>
          </article>
        </div>
      </section>
    </div>

    <div v-else class="grid gap-4 lg:grid-cols-[340px_minmax(0,1fr)]">
      <section class="pb-card h-fit space-y-4 p-4">
        <div class="flex items-center justify-between">
          <h3 class="text-sm font-semibold text-ink">生成参数</h3>
          <span class="font-mono text-[10px] text-faint">
            {{ aiProvider === 'comfyui' ? '本地推理' : '云 API' }}
          </span>
        </div>

        <div class="space-y-1">
          <span class="pb-label">素材模板</span>
          <div class="grid grid-cols-2 gap-1">
            <button
              v-for="preset in AI_PRESETS"
              :key="preset.key"
              type="button"
              class="rounded-md border border-line-soft px-2 py-1 text-left text-[11px] text-muted transition-colors hover:border-accent/40 hover:text-ink"
              :title="`${preset.width}×${preset.height} · ${ASSET_TYPE_LABELS[preset.type]}`"
              @click="applyAIPreset(preset)"
            >
              {{ preset.label }}
            </button>
          </div>
          </div>

        <div class="grid grid-cols-2 gap-3">
          <label class="flex flex-col gap-1">
            <span class="pb-label">供应商</span>
            <select v-model="aiProvider" class="pb-input h-8 py-1 text-xs" @change="setAIProvider()">
              <option v-for="provider in AI_PROVIDERS" :key="provider" :value="provider">
                {{ provider === 'comfyui' ? 'ComfyUI' : 'OpenAI 兼容' }}
              </option>
            </select>
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">素材类型</span>
            <select v-model="aiAssetType" class="pb-input h-8 py-1 text-xs">
              <option v-for="type in ASSET_TYPES" :key="type" :value="type">
                {{ ASSET_TYPE_LABELS[type] }}
              </option>
            </select>
          </label>
        </div>

        <label class="flex flex-col gap-1">
          <span class="flex items-center justify-between">
            <span class="pb-label">提示词</span>
            <button type="button" class="inline-flex items-center gap-1 text-[10px] text-accent hover:underline" @click="fillFromStyleGuide()">
              <Wand2 :size="11" />
              按风格指南填充
            </button>
          </span>
          <textarea
            v-model="aiPrompt"
            rows="5"
            class="pb-input resize-y text-xs leading-relaxed"
            placeholder="例如：杂货铺，3/4 俯视像素风，木结构，暖色招牌，16 色以内，透明背景"
          />
        </label>

        <div class="space-y-1">
          <span class="pb-label">精灵表布局</span>
          <div class="flex flex-wrap gap-1">
            <button
              v-for="spec in SHEET_PRESET_OPTIONS"
              :key="spec.key"
              type="button"
              class="inline-flex items-center gap-1 rounded-md border border-line-soft px-2 py-0.5 text-[10px] text-faint transition-colors hover:text-ink"
              :title="`写入 ${spec.columns}×${spec.rows} 网格说明与推荐尺寸`"
              @click="appendSheetLayout(spec.key)"
            >
              <Film v-if="spec.key === 'fx'" :size="11" />
              <Grid3x3 v-else :size="11" />
              {{ spec.label }}
            </button>
          </div>
        </div>

        <label class="flex flex-col gap-1">
          <span class="pb-label">负面提示词</span>
          <textarea v-model="aiNegative" rows="2" class="pb-input resize-y text-[11px] leading-relaxed" />
        </label>

        <div class="grid grid-cols-2 gap-3">
          <label class="flex flex-col gap-1">
            <span class="pb-label">宽（px）</span>
            <input v-model.number="aiWidth" type="number" min="64" max="2048" step="64" class="pb-input h-8 py-1 text-xs" />
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">高（px）</span>
            <input v-model.number="aiHeight" type="number" min="64" max="2048" step="64" class="pb-input h-8 py-1 text-xs" />
          </label>
        </div>
        <div class="flex flex-wrap gap-1">
          <button
            v-for="preset in AI_SIZES"
            :key="`${preset.w}x${preset.h}`"
            type="button"
            class="rounded-md border px-2 py-0.5 font-mono text-[10px] transition-colors"
            :class="aiWidth === preset.w && aiHeight === preset.h
              ? 'border-accent/60 bg-accent/15 text-ink'
              : 'border-line-soft text-faint hover:text-ink'"
            @click="setAISize(preset.w, preset.h)"
          >
            {{ preset.label }}
          </button>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <label class="flex flex-col gap-1">
            <span class="pb-label">数量</span>
            <input v-model.number="aiCount" type="number" min="1" max="4" class="pb-input h-8 py-1 text-xs" />
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">种子</span>
            <div class="flex items-center gap-1">
              <input v-model.number="aiSeed" type="number" min="0" class="pb-input h-8 py-1 text-xs" />
              <button
                type="button"
                class="pb-icon-btn size-8 shrink-0"
                title="随机种子"
                @click="aiSeed = Math.floor(Math.random() * 1_000_000_000)"
              >
                <Dices :size="13" />
              </button>
            </div>
          </label>
        </div>

        <label class="flex flex-col gap-1">
          <span class="pb-label">素材名（可选）</span>
          <input v-model="aiName" class="pb-input h-8 py-1 text-xs" placeholder="例如：杂货铺" />
        </label>

        <button
          type="button"
          class="pb-btn pb-btn-primary h-9 w-full py-1 text-xs"
          :disabled="aiSubmitting || !aiPrompt.trim()"
          @click="submitAI()"
        >
          <Loader2 v-if="aiSubmitting" :size="14" class="animate-spin" />
          <Sparkles v-else :size="14" />
          提交到队列
        </button>

        <p v-if="aiProvider === 'openai' && !store.aiSettings?.openai.hasKey" class="text-[10px] leading-relaxed text-[#e0a05a]">
          云接口尚未保存 API Key，请先到「设置 → AI 生成」配置。
        </p>
        <p class="text-[10px] leading-relaxed text-faint">
          云调用只发送提示词、尺寸与种子，提交前会显示完整内容并确认费用。结果永远是草稿，采用后才进入素材库。
        </p>
      </section>

      <section class="min-w-0 space-y-3">
        <div v-if="store.aiJobs.length" class="pb-card overflow-hidden">
          <div class="flex items-center justify-between gap-2 px-3 py-2">
            <h3 class="text-xs font-semibold text-ink">生成队列</h3>
            <div class="flex items-center gap-2">
              <span class="font-mono text-[10px] text-faint">{{ activeJobs.length }} 进行中</span>
              <button
                v-if="finishedJobs.length"
                type="button"
                class="pb-btn px-2 py-0.5 text-[10px]"
                title="删除全部已结束任务及其生成结果"
                @click="clearFinishedJobs()"
              >
                <Trash2 :size="11" />
                清空已结束
              </button>
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-1 border-y border-line-soft bg-panel-2/30 px-2 py-1.5">
            <button
              v-for="tab in JOB_TABS"
              :key="tab.id"
              type="button"
              class="rounded-md px-2 py-1 text-[10px] transition-colors"
              :class="jobTab === tab.id ? 'bg-panel-3 text-ink' : 'text-faint hover:text-ink'"
              :aria-pressed="jobTab === tab.id"
              @click="jobTab = tab.id"
            >
              {{ tab.label }}
              <span class="ml-1 font-mono text-[9px] text-faint">
                {{
                  tab.id === 'active'
                    ? activeJobs.length
                    : tab.id === 'done'
                      ? doneJobs.length
                      : tab.id === 'failed'
                        ? failedJobs.length
                        : store.aiJobs.length
                }}
              </span>
            </button>
          </div>
          <div class="space-y-2 p-2">
            <article
              v-for="job in visibleJobs"
              :key="job.id"
              class="rounded-lg border border-line-soft bg-canvas/50 px-3 py-2"
            >
              <div class="flex flex-wrap items-center gap-2 text-[11px]">
                <button
                  type="button"
                  class="flex min-w-0 flex-1 items-center gap-2 text-left disabled:cursor-default"
                  :disabled="job.state !== 'done'"
                  :title="job.state === 'done' ? (expandedJobId === job.id ? '收起生成结果' : '展开生成结果') : ''"
                  @click="toggleJob(job)"
                >
                  <ChevronDown
                    v-if="job.state === 'done'"
                    :size="12"
                    class="shrink-0 text-faint transition-transform"
                    :class="expandedJobId === job.id ? '' : '-rotate-90'"
                  />
                  <span class="shrink-0 font-medium" :class="JOB_STATE_TONE[job.state]">
                    {{ JOB_STATE_LABEL[job.state] }}
                  </span>
                  <span class="pb-chip shrink-0 font-mono text-[9px]">
                    {{ job.provider === 'comfyui' ? 'ComfyUI' : '云 API' }}
                  </span>
                  <span class="shrink-0 font-mono text-[9px] text-faint">
                    {{ job.width }}×{{ job.height }} · ×{{ job.count }} · 种子 {{ job.seed }}
                  </span>
                  <span v-if="job.name.trim()" class="min-w-0 truncate text-ink">{{ job.name }}</span>
                  <span v-if="job.state === 'done'" class="shrink-0 font-mono text-[9px] text-faint">
                    {{ job.images.length }} 张
                  </span>
                </button>
                <div class="ml-auto flex shrink-0 items-center gap-1">
                  <button
                    v-if="job.state === 'queued' || job.state === 'running'"
                    type="button"
                    class="pb-icon-btn size-6"
                    title="取消任务"
                    @click="cancelJob(job)"
                  >
                    <Ban :size="11" />
                  </button>
                  <button
                    v-if="TERMINAL_STATES.includes(job.state)"
                    type="button"
                    class="pb-icon-btn size-6 text-[#e07a7a]"
                    title="删除任务及其生成结果（已采用的素材不受影响）"
                    @click="deleteJob(job)"
                  >
                    <Trash2 :size="11" />
                  </button>
                </div>
              </div>
              <p class="mt-1 truncate text-[10px] text-faint" :title="job.prompt">{{ job.prompt }}</p>
              <div v-if="job.state === 'running'" class="mt-1.5 flex items-center gap-2">
                <ProgressBar :value="job.progress" class="flex-1" />
                <span class="font-mono text-[9px] text-faint">{{ Math.round(job.progress) }}%</span>
                <span v-if="job.stage" class="max-w-32 truncate text-[9px] text-faint">{{ job.stage }}</span>
              </div>
              <p v-if="job.error" class="mt-1 text-[10px] leading-relaxed whitespace-pre-line text-[#e07a7a]">{{ job.error }}</p>

              <div
                v-if="job.state === 'done' && expandedJobId === job.id"
                class="mt-2 grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-2 border-t border-line-soft pt-2"
              >
  <figure
                  v-for="image in job.images"
                  :key="image.index"
                  class="overflow-hidden rounded-lg border border-line-soft"
                >
                  <button
                    type="button"
                    class="transparency relative flex h-36 w-full cursor-zoom-in items-center justify-center p-2"
                    title="点击查看原图"
                    @click="openLightbox(image.url, `${job.name.trim() || job.prompt} ${image.index + 1}`)"
                  >
                    <span
                      v-if="isAdopted(image)"
                      class="pointer-events-none absolute top-1.5 right-1.5 rounded bg-black/60 px-1.5 py-0.5 text-[9px] text-white/75"
                    >
                      已采用
                    </span>
                    <img
                      :src="image.url"
                      :alt="`AI 结果 ${image.index + 1}`"
                      class="pixelated max-h-full w-auto max-w-full"
                      :style="{ imageRendering: 'pixelated' }"
                    />
                  </button>
                  <figcaption class="flex items-center justify-between gap-1 border-t border-line-soft px-2 py-1">
                    <span class="min-w-0 truncate font-mono text-[9px] text-faint">
                      {{ image.width }}×{{ image.height }}
                    </span>
                    <div class="flex shrink-0 items-center gap-1">
                      <template v-if="job.mode === 'inpaint'">
                        <button
                          type="button"
                          class="pb-btn shrink-0 px-2 py-0.5 text-[10px] whitespace-nowrap"
                          title="回到素材详情采用为新版本"
                          @click="openInpaintTarget(job)"
                        >
                          <Check :size="11" />
                          查看素材
                        </button>
                      </template>
                      <template v-else>
                      <button
                        type="button"
                        class="pb-btn shrink-0 px-2 py-0.5 text-[10px] whitespace-nowrap"
                        title="切成 4 方向 / 8 方向 / 特效序列帧"
                        @click="openAISheet(job, image.index)"
                      >
                        <Grid3x3 :size="11" />
                        切图
                      </button>
                      <button
                        v-if="isAdopted(image)"
                        type="button"
                        class="pb-btn shrink-0 px-2 py-0.5 text-[10px] whitespace-nowrap text-green"
                        title="在素材库中打开"
                        @click="openAdopted(image)"
                      >
                        <Check :size="11" />
                        已采用
                      </button>
                      <button
                        v-else
                        type="button"
                        class="pb-btn shrink-0 px-2 py-0.5 text-[10px] whitespace-nowrap"
                        @click="openAIAdopt(job, image.index)"
                      >
                        <Check :size="11" />
                        采用
                      </button>
                      </template>
                    </div>
                  </figcaption>
                </figure>
              </div>
            </article>
            <p v-if="visibleJobs.length === 0" class="px-2 py-6 text-center text-[11px] text-faint">
              这个分类里还没有任务。
            </p>
          </div>
        </div>

        <div
          v-if="!store.aiJobs.length"
          class="pb-card flex h-64 flex-col items-center justify-center gap-3 text-center"
        >
          <Bot :size="24" class="text-faint" />
          <p class="text-sm text-muted">还没有 AI 草稿</p>
          <p class="max-w-sm text-[11px] leading-relaxed text-faint">
            先在「设置 → AI 生成」里配置 ComfyUI 或云接口，然后填写左侧提示词提交到队列。
          </p>
        </div>
        <div
          v-else-if="!doneJobs.length"
          class="pb-card flex h-40 flex-col items-center justify-center gap-2 text-center"
        >
          <Loader2 :size="20" class="text-faint" />
          <p class="text-xs text-muted">任务已提交，完成后显示结果</p>
        </div>
      </section>
    </div>

    <AppModal
      :open="adoptTarget !== null"
      title="采用建筑变体"
      @update:open="onAdoptModal"
    >
      <div v-if="adoptTarget" class="space-y-4">
        <div class="transparency flex h-56 items-center justify-center rounded-lg border border-line p-3">
          <img :src="adoptTarget.image" alt="" class="pixelated max-h-full w-auto max-w-full" />
        </div>
        <div class="grid grid-cols-2 gap-2 font-mono text-[10px] text-faint">
          <span>种子 {{ adoptTarget.seed }}</span>
          <span>{{ adoptTarget.width }}×{{ adoptTarget.height }} px</span>
          <span>锚点 {{ adoptTarget.pivot.x }},{{ adoptTarget.pivot.y }}</span>
          <span>碰撞 {{ adoptTarget.collision.x }},{{ adoptTarget.collision.y }} · {{ adoptTarget.collision.w }}×{{ adoptTarget.collision.h }}</span>
        </div>
        <label class="flex flex-col gap-1">
          <span class="pb-label">素材名称</span>
          <input v-model="adoptName" class="pb-input" placeholder="例如：杂货铺" @keydown.enter="confirmAdopt()" />
        </label>
        <div class="flex justify-end gap-2">
          <button type="button" class="pb-btn" @click="adoptTarget = null">取消</button>
          <button
            type="button"
            class="pb-btn pb-btn-primary"
            :disabled="store.busy || !adoptName.trim()"
            @click="confirmAdopt()"
          >
            <Loader2 v-if="store.busy" :size="14" class="animate-spin" />
            <Check v-else :size="14" />
            保存草稿
          </button>
        </div>
      </div>
    </AppModal>

    <AppModal
      :open="aiAdoptTarget !== null"
      title="采用 AI 草稿"
      description="保存为草稿素材后，可打开像素编辑器修整，或运行像素完美后处理。"
      @update:open="onAIAdoptModal"
    >
      <div v-if="aiAdoptTarget" class="space-y-4">
        <div class="transparency flex h-64 items-center justify-center rounded-lg border border-line p-3">
          <img
            :src="aiAdoptTarget.image.url"
            alt=""
            class="pixelated max-h-full w-auto max-w-full"
            :style="{ imageRendering: 'pixelated' }"
          />
        </div>
        <div class="grid grid-cols-2 gap-2 font-mono text-[10px] text-faint">
          <span>{{ aiAdoptTarget.image.width }}×{{ aiAdoptTarget.image.height }} px</span>
          <span>种子 {{ aiAdoptTarget.job.seed }}</span>
          <span class="col-span-2 truncate" :title="aiAdoptTarget.job.prompt">
            {{ aiAdoptTarget.job.provider === 'comfyui' ? 'ComfyUI' : '云 API' }} · {{ aiAdoptTarget.job.prompt }}
          </span>
        </div>
        <label class="flex flex-col gap-1">
          <span class="pb-label">素材名称</span>
          <input
            v-model="aiAdoptName"
            class="pb-input"
            placeholder="例如：杂货铺草稿"
            @keydown.enter="confirmAIAdopt()"
          />
        </label>
        <div class="flex justify-end gap-2">
          <button type="button" class="pb-btn" @click="aiAdoptTarget = null">取消</button>
          <button
            type="button"
            class="pb-btn pb-btn-primary"
            :disabled="store.busy || !aiAdoptName.trim()"
            @click="confirmAIAdopt()"
          >
            <Loader2 v-if="store.busy" :size="14" class="animate-spin" />
            <Check v-else :size="14" />
            保存草稿
          </button>
        </div>
      </div>
    </AppModal>

    <AppModal
      :open="aiSheetTarget !== null"
      title="切成精灵表"
      description="按网格切开 AI 图，逐帧去背景、裁边、统一缩放并量化，输出带动画元数据的草稿素材。"
      @update:open="onAISheetModal"
    >
      <div v-if="aiSheetTarget" class="space-y-4">
        <div
          ref="sheetPreviewBox"
          class="transparency relative flex h-56 items-center justify-center rounded-lg border border-line p-3"
        >
          <div
            class="relative"
            :style="sheetPreviewSize.w > 0
              ? { width: `${sheetPreviewSize.w}px`, height: `${sheetPreviewSize.h}px` }
              : {}"
          >
            <img
              :src="aiSheetTarget.image.url"
              alt=""
              class="pixelated block max-h-full max-w-full"
              :style="{ imageRendering: 'pixelated' }"
              @load="syncSheetPreview"
            />
            <div
              v-if="sheetPreviewSize.w > 0"
              class="pointer-events-none absolute inset-0"
              :style="sheetGridStyle"
            />
          </div>
          <span
            v-if="sheetPreviewSize.w > 0"
            class="pb-chip pointer-events-none absolute top-2 right-2 font-mono text-[10px]"
          >
            {{ Math.trunc(asNumber(aiSheet.columns, 1)) }} × {{ Math.trunc(asNumber(aiSheet.rows, 1)) }} 网格
          </span>
        </div>

        <div class="grid grid-cols-3 gap-1">
          <button
            v-for="spec in SHEET_PRESET_OPTIONS"
            :key="spec.key"
            type="button"
            class="flex items-center justify-center gap-1 rounded-md border px-2 py-1.5 text-[11px] transition-colors"
            :class="aiSheet.preset === spec.key
              ? 'border-accent/60 bg-accent/15 text-ink'
              : 'border-line-soft text-muted hover:text-ink'"
            @click="applySheetPreset(spec.key)"
          >
            <Film v-if="spec.key === 'fx'" :size="12" />
            <Grid3x3 v-else :size="12" />
            {{ spec.label }}
          </button>
        </div>

        <div class="grid grid-cols-4 gap-2">
          <label class="flex flex-col gap-1">
            <span class="pb-label">列</span>
            <input v-model.number="aiSheet.columns" type="number" min="1" max="16" class="pb-input h-8 py-1 font-mono text-xs" />
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">行</span>
            <input v-model.number="aiSheet.rows" type="number" min="1" max="16" class="pb-input h-8 py-1 font-mono text-xs" />
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">帧尺寸</span>
            <input v-model.number="aiSheet.frameSize" type="number" min="8" max="256" class="pb-input h-8 py-1 font-mono text-xs" />
          </label>
          <label class="flex flex-col gap-1">
            <span class="pb-label">FPS</span>
            <input v-model.number="aiSheet.fps" type="number" min="1" max="60" class="pb-input h-8 py-1 font-mono text-xs" />
          </label>
        </div>

        <p class="font-mono text-[10px] text-faint">
          {{ Math.trunc(asNumber(aiSheet.columns, 1)) * Math.trunc(asNumber(aiSheet.rows, 1)) }} 帧 ·
          每帧 {{ asNumber(aiSheet.frameSize, 32) }}px · 画布 {{ asNumber(aiSheet.frameSize, 32) + 2 }}²
        </p>

        <label class="flex flex-col gap-1">
          <span class="pb-label">素材名称</span>
          <input v-model="aiSheetName" class="pb-input" placeholder="例如：主角 8 方向行走" @keydown.enter="confirmAISheet()" />
        </label>

        <label class="flex flex-col gap-1">
          <span class="pb-label">标签（逗号分隔）</span>
          <input v-model="aiSheet.tags" class="pb-input h-8 py-1 text-xs" placeholder="方向图,行走" />
        </label>

        <div class="space-y-2 rounded-lg border border-line-soft bg-canvas/60 p-3">
          <label class="flex items-center gap-2 text-xs text-muted">
            <input v-model="aiSheet.keyBackground" type="checkbox" class="size-4 accent-[#f2b84b]" />
            去除纯色背景（从边缘泛洪）
          </label>
          <label class="flex items-center gap-2 text-xs text-muted">
            <input v-model="aiSheet.outline" type="checkbox" class="size-4 accent-[#f2b84b]" />
            1px 深色描边
          </label>
        </div>

        <p class="text-[10px] leading-relaxed text-faint">
          生成时请让主体按 {{ aiSheet.columns }}×{{ aiSheet.rows }} 等距排布并留出边距；所有帧共用同一调色板，避免动画闪烁。
        </p>

        <div class="flex justify-end gap-2">
          <button type="button" class="pb-btn" @click="aiSheetTarget = null">取消</button>
          <button
            type="button"
            class="pb-btn pb-btn-primary"
            :disabled="store.busy || !aiSheetName.trim()"
            @click="confirmAISheet()"
          >
            <Loader2 v-if="store.busy" :size="14" class="animate-spin" />
            <Check v-else :size="14" />
            切分并保存
          </button>
        </div>
      </div>
    </AppModal>
  </div>
</template>