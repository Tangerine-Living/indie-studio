<script setup lang="ts">
import { Eraser, Loader2, Maximize2, Paintbrush, Sparkles, Trash2 } from 'lucide-vue-next'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import type { AIJob, Asset, AIProviderId } from '../domain/types'
import { platform } from '../platform'
import { useAppStore } from '../stores/app'
import { AppError } from '../lib/errors'
import AppModal from './AppModal.vue'

const props = defineProps<{ open: boolean; asset: Asset | null }>()
const emit = defineEmits<{ 'update:open': [boolean] }>()

const store = useAppStore()

const baseImage = ref<HTMLImageElement | null>(null)
const baseURL = ref('')
const loading = ref(false)
const loadError = ref('')

const display = ref<HTMLCanvasElement | null>(null)
let maskCanvas: HTMLCanvasElement | null = null
let tintCanvas: HTMLCanvasElement | null = null
const brushSize = ref(12)
const paintMode = ref<'paint' | 'erase'>('paint')
const painting = ref(false)
let renderHandle = 0
/** View transform for the mask canvas: Space+drag pans, wheel zooms. */
const zoom = ref(1)
const pan = reactive({ x: 0, y: 0 })
const spaceDown = ref(false)
const panning = ref<{ x: number; y: number } | null>(null)
const viewStyle = computed(() => ({
  transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom.value})`,
  transformOrigin: '0 0',
}))
const canvasCursor = computed(() => {
  if (panning.value) return 'grabbing'
  if (spaceDown.value) return 'grab'
  return 'crosshair'
})

function resetView() {
  zoom.value = 1
  pan.x = 0
  pan.y = 0
  panning.value = null
}

/** Zooms around the pointer, keeping that image point under the cursor. */
function onWheel(event: WheelEvent) {
  const el = display.value
  const container = el?.parentElement
  if (!el || !container) return
  const next = Math.min(8, Math.max(1, zoom.value * (event.deltaY < 0 ? 1.15 : 1 / 1.15)))
  if (Math.abs(next - zoom.value) < 0.001) return
  const crect = container.getBoundingClientRect()
  const rect = el.getBoundingClientRect()
  const cx = (event.clientX - rect.left) / zoom.value
  const cy = (event.clientY - rect.top) / zoom.value
  zoom.value = next
  pan.x = event.clientX - crect.left - cx * next
  pan.y = event.clientY - crect.top - cy * next
}

function onViewKeyDown(event: KeyboardEvent) {
  if (!props.open || event.code !== 'Space') return
  const target = event.target as HTMLElement | null
  if (target && /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)) return
  spaceDown.value = true
  event.preventDefault()
}

function onViewKeyUp(event: KeyboardEvent) {
  if (event.code !== 'Space') return
  spaceDown.value = false
  panning.value = null
}

function onViewBlur() {
  spaceDown.value = false
  panning.value = null
}

const prompt = ref('')
const provider = ref<AIProviderId>('comfyui')
const submitting = ref(false)
const jobId = ref<string | null>(null)
const resultURL = ref('')
const compositeURL = ref('')
const error = ref('')
const notice = ref('')

const displayWidth = computed(() => {
  const image = baseImage.value
  if (!image) return 480
  return Math.min(520, Math.max(160, image.naturalWidth))
})
const displayHeight = computed(() => {
  const image = baseImage.value
  if (!image) return 360
  const scale = displayWidth.value / Math.max(1, image.naturalWidth)
  return Math.max(120, Math.round(image.naturalHeight * scale))
})

const job = computed<AIJob | null>(() =>
  jobId.value ? store.aiJobs.find((item) => item.id === jobId.value) ?? null : null,
)
const busyJob = computed(() => job.value?.state === 'queued' || job.value?.state === 'running')

function reset() {
  baseImage.value = null
  baseURL.value = ''
  loadError.value = ''
  resultURL.value = ''
  compositeURL.value = ''
  jobId.value = null
  error.value = ''
  notice.value = ''
  paintMode.value = 'paint'
  brushSize.value = 12
  resetView()
  maskCanvas = null
  tintCanvas = null
  if (renderHandle) {
    cancelAnimationFrame(renderHandle)
    renderHandle = 0
  }
}

async function loadAsset() {
  const asset = props.asset
  if (!asset) return
  reset()
  const current = asset.currentVersion
  if (!current) {
    loadError.value = '这个素材还没有版本，先在像素编辑器里画几笔并保存。'
    return
  }
  loading.value = true
  prompt.value = `${asset.name} 局部重绘`
  provider.value = store.aiSettings?.defaultProvider ?? 'comfyui'
  try {
    const url = await platform.assetURL(asset.id, `versions/${current}/output.png`)
    baseURL.value = url
    const image = await loadImage(url)
    if (!image) {
      loadError.value = '无法读取当前版本的图片'
    } else {
      baseImage.value = image
      await nextTick()
      setupCanvases()
    }
  } catch (raw) {
    loadError.value = raw instanceof AppError ? raw.message : '无法读取当前版本的图片'
  } finally {
    loading.value = false
  }
}

function loadImage(url: string): Promise<HTMLImageElement | null> {
  return new Promise((resolve) => {
    try {
      const image = new Image()
      image.onload = () => resolve(image)
      image.onerror = () => resolve(null)
      image.src = url
    } catch {
      resolve(null)
    }
  })
}

function setupCanvases() {
  const el = display.value
  const image = baseImage.value
  if (!el || !image) return
  el.width = displayWidth.value
  el.height = displayHeight.value
  maskCanvas = document.createElement('canvas')
  maskCanvas.width = el.width
  maskCanvas.height = el.height
  // Mask pixels are transparent until painted; alpha carries the painted amount.
  tintCanvas = document.createElement('canvas')
  tintCanvas.width = el.width
  tintCanvas.height = el.height
  schedulePaint()
}

function schedulePaint() {
  if (renderHandle) return
  renderHandle = requestAnimationFrame(() => {
    renderHandle = 0
    paint()
  })
}

function paint() {
  const el = display.value
  const image = baseImage.value
  if (!el || !image) return
  const ctx = el.getContext('2d')
  const mask = maskCanvas
  const tint = tintCanvas
  if (!ctx || !mask || !tint) return
  ctx.imageSmoothingEnabled = false
  ctx.clearRect(0, 0, el.width, el.height)
  ctx.drawImage(image, 0, 0, el.width, el.height)
  const tctx = tint.getContext('2d')
  if (tctx) {
    tctx.clearRect(0, 0, tint.width, tint.height)
    tctx.drawImage(mask, 0, 0)
    // Keep the red only where the mask has alpha (painted areas).
    tctx.globalCompositeOperation = 'source-in'
    tctx.fillStyle = 'rgba(255, 93, 93, 0.9)'
    tctx.fillRect(0, 0, tint.width, tint.height)
    tctx.globalCompositeOperation = 'source-over'
    ctx.save()
    ctx.globalAlpha = 0.5
    ctx.drawImage(tint, 0, 0)
    ctx.restore()
  }
}

function displayPoint(event: PointerEvent): { x: number; y: number } | null {
  const el = display.value
  if (!el) return null
  const rect = el.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) return null
  const x = ((event.clientX - rect.left) / rect.width) * el.width
  const y = ((event.clientY - rect.top) / rect.height) * el.height
  return { x, y }
}

function paintAt(point: { x: number; y: number }, last?: { x: number; y: number }) {
  const mask = maskCanvas
  if (!mask) return
  const ctx = mask.getContext('2d')
  if (!ctx) return
  const erasing = paintMode.value === 'erase'
  ctx.globalCompositeOperation = erasing ? 'destination-out' : 'source-over'
  ctx.strokeStyle = '#fff'
  ctx.fillStyle = '#fff'
  ctx.lineWidth = brushSize.value
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  if (last) {
    ctx.beginPath()
    ctx.moveTo(last.x, last.y)
    ctx.lineTo(point.x, point.y)
    ctx.stroke()
  } else {
    ctx.beginPath()
    ctx.arc(point.x, point.y, brushSize.value / 2, 0, Math.PI * 2)
    ctx.fill()
  }
  ctx.globalCompositeOperation = 'source-over'
}

let lastPoint: { x: number; y: number } | null = null

function onPointerDown(event: PointerEvent) {
  if (!maskCanvas) return
  if (spaceDown.value || event.button === 1) {
    panning.value = { x: event.clientX, y: event.clientY }
    ;(event.currentTarget as HTMLCanvasElement).setPointerCapture(event.pointerId)
    event.preventDefault()
    return
  }
  if (event.button !== 0) return
  const point = displayPoint(event)
  if (!point) return
  ;(event.currentTarget as HTMLCanvasElement).setPointerCapture(event.pointerId)
  painting.value = true
  lastPoint = point
  paintAt(point)
  schedulePaint()
}

function onPointerMove(event: PointerEvent) {
  if (panning.value) {
    pan.x += event.clientX - panning.value.x
    pan.y += event.clientY - panning.value.y
    panning.value = { x: event.clientX, y: event.clientY }
    return
  }
  if (!painting.value) return
  const point = displayPoint(event)
  if (!point) return
  paintAt(point, lastPoint ?? point)
  lastPoint = point
  schedulePaint()
}

function onPointerUp() {
  painting.value = false
  lastPoint = null
  panning.value = null
}

function clearMask() {
  const mask = maskCanvas
  if (!mask) return
  const ctx = mask.getContext('2d')
  if (!ctx) return
  ctx.clearRect(0, 0, mask.width, mask.height)
  resultURL.value = ''
  compositeURL.value = ''
  schedulePaint()
}

/** Draws the mask into an image-sized alpha canvas for compositing. */
function alphaMaskCanvas(width: number, height: number): HTMLCanvasElement | null {
  const mask = maskCanvas
  if (!mask) return null
  const scaled = document.createElement('canvas')
  scaled.width = width
  scaled.height = height
  const sctx = scaled.getContext('2d')
  if (!sctx) return null
  sctx.imageSmoothingEnabled = false
  sctx.drawImage(mask, 0, 0, width, height)
  const data = sctx.getImageData(0, 0, width, height)
  for (let i = 0; i < data.data.length; i += 4) {
    const value = data.data[i] ?? 0
    data.data[i] = 255
    data.data[i + 1] = 255
    data.data[i + 2] = 255
    data.data[i + 3] = value
  }
  sctx.putImageData(data, 0, 0)
  return scaled
}

function baseCanvas(): HTMLCanvasElement | null {
  const image = baseImage.value
  if (!image) return null
  const canvas = document.createElement('canvas')
  canvas.width = image.naturalWidth
  canvas.height = image.naturalHeight
  const ctx = canvas.getContext('2d')
  if (!ctx) return null
  ctx.imageSmoothingEnabled = false
  ctx.drawImage(image, 0, 0)
  return canvas
}

function maskDataURL(): string | null {
  const image = baseImage.value
  const mask = maskCanvas
  if (!image || !mask) return null
  const scaled = document.createElement('canvas')
  scaled.width = image.naturalWidth
  scaled.height = image.naturalHeight
  const ctx = scaled.getContext('2d')
  if (!ctx) return null
  ctx.imageSmoothingEnabled = false
  ctx.fillStyle = '#000'
  ctx.fillRect(0, 0, scaled.width, scaled.height)
  ctx.drawImage(mask, 0, 0, scaled.width, scaled.height)
  try {
    return scaled.toDataURL('image/png')
  } catch {
    return null
  }
}

async function submit() {
  const asset = props.asset
  const image = baseImage.value
  if (!asset || !image) return
  error.value = ''
  notice.value = ''
  const base = baseCanvas()
  const mask = maskDataURL()
  if (!base || !mask) {
    error.value = '无法读取画布或蒙版'
    return
  }
  if (!prompt.value.trim()) {
    error.value = '请先填写重绘提示词'
    return
  }
  let baseData = ''
  try {
    baseData = base.toDataURL('image/png')
  } catch {
    error.value = '无法导出原图'
    return
  }
  submitting.value = true
  try {
    const job = await store.submitAIJob({
      provider: provider.value,
      prompt: prompt.value.trim(),
      negativePrompt: '',
      width: image.naturalWidth,
      height: image.naturalHeight,
      count: 1,
      seed: Math.floor(Math.random() * 1_000_000_000),
      assetType: asset.type,
      name: asset.name,
      mode: 'inpaint',
      targetAssetId: asset.id,
      baseImage: baseData,
      mask,
    })
    if (job) {
      jobId.value = job.id
      resultURL.value = ''
      compositeURL.value = ''
    }
  } finally {
    submitting.value = false
  }
}

watch(display, (el) => {
  if (el && baseImage.value) setupCanvases()
})

watch(job, async (current) => {
  if (!current) return
  if (current.state === 'failed') {
    error.value = current.error || '局部重绘失败'
    return
  }
  if (current.state !== 'done') return
  const image = current.images[0]
  if (!image) {
    error.value = '生成服务没有返回图片'
    return
  }
  resultURL.value = image.url
  await composeResult(image.url)
})

async function composeResult(resultUrl: string) {
  const base = baseCanvas()
  const image = baseImage.value
  if (!base || !image) return
  const alpha = alphaMaskCanvas(base.width, base.height)
  if (!alpha) return
  const result = await loadImage(resultUrl)
  if (!result) {
    error.value = '无法读取重绘结果'
    return
  }
  const layer = document.createElement('canvas')
  layer.width = base.width
  layer.height = base.height
  const lctx = layer.getContext('2d')
  if (!lctx) return
  lctx.imageSmoothingEnabled = false
  lctx.drawImage(result, 0, 0, layer.width, layer.height)
  lctx.globalCompositeOperation = 'destination-in'
  lctx.drawImage(alpha, 0, 0)
  lctx.globalCompositeOperation = 'source-over'

  const out = document.createElement('canvas')
  out.width = base.width
  out.height = base.height
  const octx = out.getContext('2d')
  if (!octx) return
  octx.drawImage(base, 0, 0)
  octx.drawImage(layer, 0, 0)
  try {
    compositeURL.value = out.toDataURL('image/png')
  } catch {
    error.value = '无法合成重绘结果'
  }
}

async function saveVersion() {
  const asset = props.asset
  if (!asset || !compositeURL.value) return
  submitting.value = true
  try {
    const saved = await store.saveAssetVersion(asset.id, compositeURL.value)
    if (saved) {
      notice.value = `已保存 v${saved.currentVersion} 局部重绘版本`
      close()
    }
  } finally {
    submitting.value = false
  }
}

async function cancelJob() {
  if (jobId.value && busyJob.value) {
    await store.cancelAIJob(jobId.value)
  }
  jobId.value = null
  resultURL.value = ''
  compositeURL.value = ''
}

function close() {
  if (busyJob.value && jobId.value) void store.cancelAIJob(jobId.value)
  emit('update:open', false)
}

watch(
  () => props.open,
  (open) => {
    if (open) void loadAsset()
    else reset()
  },
)

onMounted(() => {
  window.addEventListener('keydown', onViewKeyDown)
  window.addEventListener('keyup', onViewKeyUp)
  window.addEventListener('blur', onViewBlur)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onViewKeyDown)
  window.removeEventListener('keyup', onViewKeyUp)
  window.removeEventListener('blur', onViewBlur)
  if (renderHandle) cancelAnimationFrame(renderHandle)
  if (busyJob.value && jobId.value) void store.cancelAIJob(jobId.value)
})
</script>

<template>
  <AppModal
    :open="props.open"
    wide
    title="局部重绘"
    :description="props.asset ? `${props.asset.name} · 只替换涂成红色的区域` : '标注区域后重新生成，未标注的像素保持不变'"
    @update:open="(next: boolean) => (next ? emit('update:open', true) : close())"
  >
    <div v-if="loading" class="flex items-center gap-2 py-10 text-xs text-muted">
      <Loader2 :size="15" class="animate-spin" /> 正在读取素材…
    </div>
    <div v-else-if="loadError" class="rounded-lg border border-amber/30 bg-amber/10 px-3 py-2 text-xs text-amber">
      {{ loadError }}
    </div>
    <div v-else-if="baseImage" class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_280px]">
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2">
          <div class="flex rounded-lg border border-line bg-panel-2 p-0.5">
            <button
              type="button"
              class="flex items-center gap-1 rounded-md px-2 py-1 text-[11px]"
              :class="paintMode === 'paint' ? 'bg-panel-3 text-ink' : 'text-muted hover:text-ink'"
              @click="paintMode = 'paint'"
            >
              <Paintbrush :size="12" /> 涂抹
            </button>
            <button
              type="button"
              class="flex items-center gap-1 rounded-md px-2 py-1 text-[11px]"
              :class="paintMode === 'erase' ? 'bg-panel-3 text-ink' : 'text-muted hover:text-ink'"
              @click="paintMode = 'erase'"
            >
              <Eraser :size="12" /> 擦除
            </button>
          </div>
          <label class="flex items-center gap-1 text-[10px] text-faint">
            笔刷
            <input v-model.number="brushSize" type="range" min="2" max="80" step="1" class="w-24 accent-[#f2b84b]" />
            <span class="w-6 font-mono">{{ brushSize }}</span>
          </label>
          <button type="button" class="pb-btn h-7 px-2 text-[11px]" @click="clearMask()">
            <Trash2 :size="12" /> 清空
          </button>
          <span class="font-mono text-[10px] text-faint">{{ Math.round(zoom * 100) }}%</span>
          <button
            type="button"
            class="pb-btn h-7 px-2 text-[11px]"
            :disabled="zoom === 1 && pan.x === 0 && pan.y === 0"
            title="回到适应窗口的位置与缩放"
            @click="resetView()"
          >
            <Maximize2 :size="12" /> 还原视图
          </button>
        </div>
        <div
          class="relative mt-2 max-w-full overflow-hidden rounded-lg border border-line"
          :style="{ width: `${displayWidth}px`, height: `${displayHeight}px` }"
          @wheel.prevent="onWheel"
        >
          <canvas
            ref="display"
            class="absolute top-0 left-0 block touch-none"
            :style="{ width: `${displayWidth}px`, height: `${displayHeight}px`, ...viewStyle, cursor: canvasCursor }"
            @pointerdown="onPointerDown"
            @pointermove="onPointerMove"
            @pointerup="onPointerUp"
            @pointercancel="onPointerUp"
            @pointerleave="onPointerUp"
            @contextmenu.prevent
          />
        </div>
        <p class="mt-1.5 text-[10px] text-faint">
          在图上涂出要重新生成的区域（红色）；未涂抹的地方保持原样。空格拖动平移，滚轮缩放。
        </p>
      </div>

      <div class="space-y-3">
        <label class="flex flex-col gap-1">
          <span class="pb-label">重绘提示词</span>
          <textarea
            v-model="prompt"
            class="pb-input min-h-20 py-1.5 text-xs"
            placeholder="例如：把右手改成握着灯笼"
            aria-label="重绘提示词"
          ></textarea>
        </label>
        <label class="flex flex-col gap-1">
          <span class="pb-label">生成通道</span>
          <select v-model="provider" class="pb-input h-8 py-1 text-xs" aria-label="局部重绘通道">
            <option value="comfyui">ComfyUI（本地工作流）</option>
            <option value="openai">OpenAI 兼容（/images/edits）</option>
          </select>
        </label>
        <p class="text-[10px] leading-relaxed text-faint">
          ComfyUI 需要一条含 IMAGE / MASK 节点标题的局部重绘工作流；OpenAI 兼容接口要求图像模型支持
          /images/edits。MiniMax Image 等不支持蒙版编辑的模型会返回 400，请换用 gpt-image 系列或改用
          ComfyUI。
        </p>

        <div v-if="error" class="rounded-lg border border-red/25 bg-red/10 px-2.5 py-2 text-[11px] text-red">
          {{ error }}
        </div>
        <div v-if="notice" class="rounded-lg border border-green/25 bg-green/10 px-2.5 py-2 text-[11px] text-green">
          {{ notice }}
        </div>

        <div v-if="busyJob" class="rounded-lg border border-line bg-panel-2/60 px-2.5 py-2 text-[11px]">
          <div class="flex items-center gap-1.5 text-muted">
            <Loader2 :size="12" class="animate-spin" />
            {{ job?.stage || '排队中' }} · {{ job?.progress ?? 0 }}%
          </div>
          <button type="button" class="mt-1.5 text-faint hover:text-red" @click="cancelJob()">取消任务</button>
        </div>

        <div v-if="compositeURL" class="space-y-2">
          <div class="pb-label">结果预览（红色区域已替换）</div>
          <img :src="compositeURL" alt="局部重绘结果" class="max-h-52 w-full rounded-lg border border-line object-contain" />
          <div class="flex gap-2">
            <button type="button" class="pb-btn pb-btn-primary h-8 flex-1 py-1 text-xs" :disabled="submitting" @click="saveVersion()">
              <Loader2 v-if="submitting" :size="13" class="animate-spin" />
              <Sparkles v-else :size="13" />
              采用为新版本
            </button>
            <button type="button" class="pb-btn h-8 py-1 text-xs" :disabled="submitting" @click="submit()">重新生成</button>
          </div>
        </div>
        <button
          v-else
          type="button"
          class="pb-btn pb-btn-primary h-9 w-full py-1 text-xs"
          :disabled="submitting || busyJob"
          @click="submit()"
        >
          <Loader2 v-if="submitting || busyJob" :size="14" class="animate-spin" />
          <Sparkles v-else :size="14" />
          提交局部重绘
        </button>
      </div>
    </div>
  </AppModal>
</template>