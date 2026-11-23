<script setup lang="ts">
import { Maximize2, Pause, Play, X, ZoomIn, ZoomOut } from 'lucide-vue-next'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { closeLightbox, lightboxState } from '../lib/lightbox'

const MAX_SCALE = 8
const SPEEDS = [0.5, 1, 2]
const scale = ref(0) // 0 = fit the window, 1..MAX_SCALE = actual pixel multiples
const natural = ref({ width: 0, height: 0 })
const frameIndex = ref(0)
const playing = ref(false)
const speed = ref(1)
const viewport = ref<HTMLElement | null>(null)
let timer: number | null = null

const animated = computed(() => lightboxState.frames.length > 1)
const currentSrc = computed(() =>
  animated.value ? lightboxState.frames[frameIndex.value] : lightboxState.src,
)

function stopTimer() {
  if (timer !== null) {
    window.clearTimeout(timer)
    timer = null
  }
}

function durationFor(index: number) {
  const custom = lightboxState.frameDurations[index]
  if (custom && custom > 0) return custom
  return 1000 / lightboxState.fps
}

const totalDuration = computed(() => {
  if (!animated.value) return 0
  let total = 0
  for (let index = 0; index < lightboxState.frames.length; index += 1) total += durationFor(index)
  return total
})

const durationLabel = computed(() => {
  const ms = totalDuration.value
  return ms >= 1000 ? `${(ms / 1000).toFixed(2)} 秒` : `${Math.round(ms)} ms`
})

function schedule() {
  stopTimer()
  if (!lightboxState.open || !playing.value || !animated.value) return
  const last = lightboxState.frames.length - 1
  if (!lightboxState.loop && frameIndex.value >= last) {
    playing.value = false
    return
  }
  timer = window.setTimeout(() => {
    if (!lightboxState.loop && frameIndex.value >= last) {
      playing.value = false
      return
    }
    frameIndex.value = (frameIndex.value + 1) % lightboxState.frames.length
    schedule()
  }, Math.max(20, Math.round(durationFor(frameIndex.value) / speed.value)))
}

function togglePlay() {
  if (!animated.value) return
  if (!playing.value && !lightboxState.loop && frameIndex.value >= lightboxState.frames.length - 1) {
    frameIndex.value = 0
  }
  playing.value = !playing.value
}

function stepFrame(direction: 1 | -1) {
  if (!animated.value) return
  playing.value = false
  const total = lightboxState.frames.length
  frameIndex.value = (frameIndex.value + direction + total) % total
}

function seek(event: Event) {
  const input = event.target as HTMLInputElement
  const value = Math.round(Number(input.value))
  playing.value = false
  if (!Number.isFinite(value)) return
  frameIndex.value = Math.min(lightboxState.frames.length - 1, Math.max(0, value))
}

function zoomIn() {
  scale.value = scale.value === 0 ? 1 : Math.min(MAX_SCALE, scale.value + 1)
}

function zoomOut() {
  scale.value = Math.max(0, scale.value - 1)
}

function fit() {
  scale.value = 0
}

function onWheel(event: WheelEvent) {
  event.preventDefault()
  if (event.deltaY < 0) zoomIn()
  else zoomOut()
}

function onKeydown(event: KeyboardEvent) {
  // Controls handle their own keys; otherwise Space would toggle playback twice.
  const target = event.target as HTMLElement | null
  const onControl = target ? ['INPUT', 'SELECT', 'BUTTON'].includes(target.tagName) : false
  if (onControl && event.key !== 'Escape') return
  switch (event.key) {
    case 'Escape':
      event.preventDefault()
      closeLightbox()
      break
    case ' ':
      if (animated.value) {
        event.preventDefault()
        togglePlay()
      }
      break
    case 'ArrowRight':
      stepFrame(1)
      break
    case 'ArrowLeft':
      stepFrame(-1)
      break
    default:
      break
  }
}

function onLoad(event: Event) {
  const image = event.target as HTMLImageElement
  natural.value = { width: image.naturalWidth, height: image.naturalHeight }
}

/** Decode every frame up front so the first loop does not flicker. */
function preloadFrames() {
  for (const src of lightboxState.frames) {
    const image = new Image()
    image.src = src
  }
}

let drag: {
  x: number
  y: number
  left: number
  top: number
  moved: boolean
  onImage: boolean
} | null = null

function onPointerDown(event: PointerEvent) {
  if (event.button !== 0 || !viewport.value) return
  const onImage = event.target instanceof HTMLElement && Boolean(event.target.closest('img'))
  drag = {
    x: event.clientX,
    y: event.clientY,
    left: viewport.value.scrollLeft,
    top: viewport.value.scrollTop,
    moved: false,
    onImage,
  }
  viewport.value.setPointerCapture(event.pointerId)
}

function onPointerMove(event: PointerEvent) {
  if (!drag || !viewport.value) return
  const dx = event.clientX - drag.x
  const dy = event.clientY - drag.y
  if (Math.abs(dx) > 3 || Math.abs(dy) > 3) drag.moved = true
  viewport.value.scrollLeft = drag.left - dx
  viewport.value.scrollTop = drag.top - dy
}

function onPointerUp() {
  if (!drag) return
  const { moved, onImage } = drag
  drag = null
  if (!moved && !onImage) closeLightbox()
}

function cancelDrag() {
  drag = null
}

watch([playing, speed], schedule)

watch(
  () => lightboxState.open,
  (open) => {
    if (typeof document === 'undefined') return
    if (open) {
      scale.value = 0
      natural.value = { width: 0, height: 0 }
      frameIndex.value = 0
      speed.value = 1
      playing.value = animated.value
      schedule()
      preloadFrames()
      document.addEventListener('keydown', onKeydown)
    } else {
      stopTimer()
      playing.value = false
      document.removeEventListener('keydown', onKeydown)
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  stopTimer()
  if (typeof document === 'undefined') return
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="lightboxState.open"
      class="fixed inset-0 z-[70] flex flex-col bg-[#07090d]/95"
      role="dialog"
      aria-modal="true"
      aria-label="查看图片"
    >
      <div class="flex h-12 shrink-0 items-center gap-2 border-b border-white/10 px-4">
        <span class="min-w-0 flex-1 truncate text-xs text-white/70">{{ lightboxState.label }}</span>
        <span v-if="natural.width" class="font-mono text-[10px] text-white/40">
          {{ natural.width }} × {{ natural.height }} px
        </span>
        <button
          type="button"
          class="flex h-7 items-center rounded-md px-1.5 text-white/60 hover:bg-white/10 hover:text-white disabled:opacity-30"
          :disabled="scale === 0"
          title="缩小"
          @click="zoomOut()"
        >
          <ZoomOut :size="14" />
        </button>
        <span class="w-9 text-center font-mono text-[10px] text-white/60">
          {{ scale === 0 ? '适应' : `${scale}×` }}
        </span>
        <button
          type="button"
          class="flex h-7 items-center rounded-md px-1.5 text-white/60 hover:bg-white/10 hover:text-white disabled:opacity-30"
          :disabled="scale === MAX_SCALE"
          title="放大"
          @click="zoomIn()"
        >
          <ZoomIn :size="14" />
        </button>
        <button
          type="button"
          class="flex h-7 items-center gap-1 rounded-md px-1.5 text-white/60 hover:bg-white/10 hover:text-white"
          title="适应窗口"
          @click="fit()"
        >
          <Maximize2 :size="13" />
        </button>
        <button
          type="button"
          class="flex size-7 items-center justify-center rounded-md text-white/60 hover:bg-white/10 hover:text-white"
          aria-label="关闭"
          @click="closeLightbox()"
        >
          <X :size="15" />
        </button>
      </div>
      <div
        ref="viewport"
        class="min-h-0 flex-1 cursor-grab overflow-auto active:cursor-grabbing"
        @wheel.prevent="onWheel"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="cancelDrag()"
      >
        <div class="mx-auto flex min-h-full w-max min-w-full items-center justify-center p-6">
          <img
            :src="currentSrc"
            :alt="lightboxState.label"
            class="transparency block select-none"
            :class="scale === 0 ? (animated ? 'max-h-[calc(100vh-9rem)]' : 'max-h-[calc(100vh-7rem)]') : ''"
            :style="
              scale > 0 && natural.width
                ? { width: `${natural.width * scale}px`, imageRendering: 'pixelated' }
                : { imageRendering: 'pixelated' }
            "
            draggable="false"
            @load="onLoad"
          />
        </div>
      </div>
      <div
        v-if="animated"
        class="flex h-12 shrink-0 items-center gap-3 border-t border-white/10 px-4"
      >
        <button
          type="button"
          class="flex size-7 shrink-0 items-center justify-center rounded-md text-white/60 hover:bg-white/10 hover:text-white"
          :title="playing ? '暂停（空格）' : '播放（空格）'"
          @click="togglePlay()"
        >
          <Pause v-if="playing" :size="14" />
          <Play v-else :size="14" />
        </button>
        <span class="w-14 shrink-0 text-center font-mono text-[10px] text-white/50">
          {{ frameIndex + 1 }} / {{ lightboxState.frames.length }}
        </span>
        <input
          type="range"
          min="0"
          :max="lightboxState.frames.length - 1"
          step="1"
          class="h-1 min-w-0 flex-1 cursor-pointer accent-[#8fb7ff]"
          :value="frameIndex"
          aria-label="选择帧"
          @input="seek($event)"
        />
        <span class="shrink-0 font-mono text-[10px] text-white/40">
          {{ lightboxState.fps }} FPS · 全长 {{ durationLabel }}
        </span>
        <select
          v-model.number="speed"
          class="h-6 shrink-0 rounded-md border border-white/10 bg-white/5 px-1 text-[10px] text-white/60 outline-none"
          aria-label="播放速度"
        >
          <option v-for="value in SPEEDS" :key="value" :value="value" class="bg-[#11161d]">
            {{ value }}×
          </option>
        </select>
        <span v-if="!lightboxState.loop" class="shrink-0 text-[10px] text-white/40">单次</span>
      </div>
    </div>
  </Teleport>
</template>